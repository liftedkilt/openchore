// Package hub is the friend-families coordination server: a small, separate
// service that passes snapshots between two families' OpenChore instances so
// that neither instance has to be reachable from the internet. Both sides
// only ever call out to the hub.
//
// The hub can't read what it carries. Snapshots are encrypted by the
// instances with a key that travels in the invite, never through the hub.
// All the hub knows is that a link exists, how big its two blobs are and
// when they last changed. See docs/friends.md.
package hub

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

const (
	// MaxBlob caps one family's encrypted snapshot.
	MaxBlob = 256 << 10
	// InviteTTL is how long an invite can be joined.
	InviteTTL = 7 * 24 * time.Hour
	// IdleTTL is how long a link nobody syncs is kept.
	IdleTTL = 90 * 24 * time.Hour
	// MaxLinks caps how many links the hub holds, as a backstop.
	MaxLinks = 10000
	// MaxWaitingPerCaller caps, on a hub anyone may use (no keys), how many
	// invites one address can have waiting to be joined. Without it one
	// address could fill MaxLinks in a day at the signup rate and lock
	// everyone else out. A family makes one invite at a time.
	MaxWaitingPerCaller = 20

	// Per-address request budgets, refilled every minute. An instance
	// syncs each link every 15 minutes, so these are far above honest use
	// and only slow down someone hammering the hub.
	RatePerMinute       = 120
	RateSignupPerMinute = 10 // creating and joining links
)

const schema = `
CREATE TABLE IF NOT EXISTS links (
    id           TEXT PRIMARY KEY,
    created_at   DATETIME NOT NULL,
    touched_at   DATETIME NOT NULL,
    join_hash    TEXT NOT NULL DEFAULT '',
    join_expires DATETIME,
    a_token      TEXT NOT NULL,
    b_token      TEXT NOT NULL DEFAULT '',
    a_data       BLOB,
    a_at         DATETIME,
    b_data       BLOB,
    b_at         DATETIME
);`

// migrations are added to a hub database made by an older version; an
// error that the column already exists is expected and ignored.
var migrations = []string{
	// Who made a link that's still waiting to be joined (for
	// MaxWaitingPerCaller): a keyed hash of the caller (see Hub.callerID),
	// cleared once the link is joined.
	`ALTER TABLE links ADD COLUMN created_by TEXT NOT NULL DEFAULT ''`,
}

// Hub serves the hub API.
type Hub struct {
	db   *sql.DB
	keys []string // sha256 of each key allowed to create links; empty = anyone
	now  func() time.Time
	mux  *http.ServeMux

	// TrustProxy makes the hub take the caller's address from the last
	// X-Forwarded-For entry (set it when exactly one reverse proxy sits in
	// front). Otherwise the connection's own address is used.
	TrustProxy bool
	// Log, when set, gets one line per request (never bodies or tokens).
	Log *log.Logger

	mu      sync.Mutex
	window  time.Time
	hits    map[string]int
	signups map[string]int

	// salt keys the caller hashes kept in links.created_by. It's made fresh
	// at every start, so a stored hash can't be matched back to an address
	// by trying them all; a restart only forgets who made which invite.
	salt      []byte
	proxyOnce sync.Once
}

// New prepares the database and returns the hub. keys are the secrets that
// may create links; with none, anyone who can reach the hub can.
func New(db *sql.DB, keys []string) (*Hub, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return nil, err
		}
	}
	h := &Hub{db: db, now: time.Now, mux: http.NewServeMux(), salt: make([]byte, 32)}
	if _, err := rand.Read(h.salt); err != nil {
		return nil, err
	}
	for _, k := range keys {
		if k = strings.TrimSpace(k); k != "" {
			h.keys = append(h.keys, hash(k))
		}
	}
	h.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok\n")) })
	h.mux.HandleFunc("POST /v1/links", h.create)
	h.mux.HandleFunc("POST /v1/links/{id}/join", h.join)
	h.mux.HandleFunc("POST /v1/links/{id}/exchange", h.exchange)
	h.mux.HandleFunc("DELETE /v1/links/{id}", h.remove)
	return h, nil
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	addr := h.caller(r)
	if !h.TrustProxy && r.Header.Get("X-Forwarded-For") != "" && h.Log != nil {
		h.proxyOnce.Do(func() {
			h.Log.Printf("requests come through a proxy (X-Forwarded-For) but HUB_TRUST_PROXY isn't set: every caller is counted as the proxy, so they all share one rate limit and one cap on waiting invites")
		})
	}
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	signup := r.Method == http.MethodPost && (r.URL.Path == "/v1/links" || strings.HasSuffix(r.URL.Path, "/join"))
	switch {
	case r.URL.Path == "/healthz":
		h.mux.ServeHTTP(rec, r) // probes aren't counted or logged
		return
	case !h.allow(limitKey(addr), signup):
		rec.Header().Set("Retry-After", "60")
		fail(rec, http.StatusTooManyRequests, "too many requests; try again in a minute")
	default:
		r.Body = http.MaxBytesReader(rec, r.Body, MaxBlob*2)
		h.mux.ServeHTTP(rec, r)
	}
	if h.Log != nil {
		// Link ids aren't secret (tokens are, and they're in headers).
		// The path is quoted: it's the caller's text and mustn't be able
		// to start a line of its own.
		h.Log.Printf("%s %s %q %d", addr, r.Method, r.URL.EscapedPath(), rec.status)
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// caller is the address requests are counted against.
func (h *Hub) caller(r *http.Request) string {
	if h.TrustProxy {
		// The proxy appends the address it saw, either to the client's own
		// header line or as a new line, so only the very last entry of the
		// last line is trustworthy. It must be an IP address: anything else
		// is the client's invention (and would be an unbounded map key).
		lines := r.Header.Values("X-Forwarded-For")
		if len(lines) > 0 {
			parts := strings.Split(lines[len(lines)-1], ",")
			if ip := net.ParseIP(strings.TrimSpace(parts[len(parts)-1])); ip != nil {
				return ip.String()
			}
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// limitKey is what a caller's limits are counted against: its address, or
// for IPv6 its /64, since one host is normally given a whole /64 and could
// otherwise rotate through addresses to get a fresh budget each time.
func limitKey(addr string) string {
	a, err := netip.ParseAddr(addr)
	if err != nil {
		return addr
	}
	if a = a.Unmap(); a.Is6() {
		if p, err := a.WithZone("").Prefix(64); err == nil {
			return p.String()
		}
	}
	return a.String()
}

// callerID is the keyed hash of a caller's limit key kept on a waiting link.
func (h *Hub) callerID(r *http.Request) string {
	m := hmac.New(sha256.New, h.salt)
	m.Write([]byte(limitKey(h.caller(r))))
	return hex.EncodeToString(m.Sum(nil))
}

// allow counts a request against the caller's budget for this minute.
func (h *Hub) allow(addr string, signup bool) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if now := h.now(); h.hits == nil || now.Sub(h.window) >= time.Minute {
		h.window, h.hits, h.signups = now, map[string]int{}, map[string]int{}
	}
	h.hits[addr]++
	if signup {
		h.signups[addr]++
	}
	return h.hits[addr] <= RatePerMinute && h.signups[addr] <= RateSignupPerMinute
}

// Sweep deletes invites nobody joined in time and links nobody has synced
// for IdleTTL.
func (h *Hub) Sweep(ctx context.Context) error {
	now := h.now()
	_, err := h.db.ExecContext(ctx,
		`DELETE FROM links WHERE (b_token = '' AND join_expires < ?) OR touched_at < ?`, now, now.Add(-IdleTTL))
	return err
}

func hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func random(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func bearer(r *http.Request) string {
	v, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return v
}

func same(a, b string) bool {
	return a != "" && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, msg string) {
	reply(w, status, map[string]string{"error": msg})
}

// create makes a link and its one-time join code. The caller becomes the
// link's first member.
func (h *Hub) create(w http.ResponseWriter, r *http.Request) {
	if len(h.keys) > 0 {
		got, ok := hash(bearer(r)), false
		for _, k := range h.keys {
			ok = ok || same(got, k)
		}
		if !ok {
			fail(w, http.StatusUnauthorized, "this hub needs a key to create a link")
			return
		}
	}
	ctx := r.Context()
	_ = h.Sweep(ctx)
	var n int
	if err := h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM links`).Scan(&n); err != nil {
		fail(w, http.StatusInternalServerError, "hub storage error")
		return
	}
	if n >= MaxLinks {
		fail(w, http.StatusServiceUnavailable, "this hub is full")
		return
	}
	by := h.callerID(r)
	id, token, code := random(12), random(32), random(18)
	now := h.now()
	expires := now.Add(InviteTTL)
	// One statement, so two requests at once can't both slip under the cap
	// on waiting invites (only checked on a hub without keys).
	res, err := h.db.ExecContext(ctx,
		`INSERT INTO links (id, created_at, touched_at, join_hash, join_expires, a_token, created_by)
		 SELECT ?, ?, ?, ?, ?, ?, ?
		 WHERE ? OR (SELECT COUNT(*) FROM links WHERE created_by = ? AND b_token = '') < ?`,
		id, now, now, hash(code), expires, hash(token), by, len(h.keys) > 0, by, MaxWaitingPerCaller)
	if err != nil {
		fail(w, http.StatusInternalServerError, "hub storage error")
		return
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		fail(w, http.StatusTooManyRequests, "too many invites waiting to be joined; use or let one expire first")
		return
	}
	reply(w, http.StatusCreated, map[string]any{"link": id, "token": token, "join_code": code, "expires_at": expires})
}

// join makes the caller the link's second member, using up the join code.
func (h *Hub) join(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		fail(w, http.StatusBadRequest, "code is required")
		return
	}
	token := random(32)
	res, err := h.db.ExecContext(r.Context(),
		`UPDATE links SET b_token = ?, join_hash = '', created_by = '', touched_at = ?
		 WHERE id = ? AND b_token = '' AND join_hash = ? AND join_expires >= ?`,
		hash(token), h.now(), r.PathValue("id"), hash(req.Code), h.now())
	if err != nil {
		fail(w, http.StatusInternalServerError, "hub storage error")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		fail(w, http.StatusForbidden, "this invite has been used, has expired or isn't valid")
		return
	}
	reply(w, http.StatusOK, map[string]string{"token": token})
}

var errGone = errors.New("gone")

// member finds the link and which side ("a" or "b") the bearer token is.
func (h *Hub) member(r *http.Request) (side string, joined bool, err error) {
	var a, b string
	err = h.db.QueryRowContext(r.Context(), `SELECT a_token, b_token FROM links WHERE id = ?`, r.PathValue("id")).Scan(&a, &b)
	if err == sql.ErrNoRows {
		return "", false, errGone
	}
	if err != nil {
		return "", false, err
	}
	got := hash(bearer(r))
	switch {
	case same(got, a):
		return "a", b != "", nil
	case same(got, b):
		return "b", true, nil
	}
	return "", false, errGone // don't tell a stranger the link exists
}

// exchange stores the caller's blob and returns the other member's.
func (h *Hub) exchange(w http.ResponseWriter, r *http.Request) {
	side, joined, err := h.member(r)
	if err == errGone {
		fail(w, http.StatusGone, "this link no longer exists")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "hub storage error")
		return
	}
	var req struct {
		Data []byte `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "invalid body")
		return
	}
	if len(req.Data) > MaxBlob {
		fail(w, http.StatusRequestEntityTooLarge, "snapshot too large")
		return
	}
	other := "b"
	if side == "b" {
		other = "a"
	}
	ctx, now, id := r.Context(), h.now(), r.PathValue("id")
	if _, err := h.db.ExecContext(ctx,
		`UPDATE links SET `+side+`_data = ?, `+side+`_at = ?, touched_at = ? WHERE id = ?`, req.Data, now, now, id); err != nil {
		fail(w, http.StatusInternalServerError, "hub storage error")
		return
	}
	var data []byte
	var at sql.NullTime
	if err := h.db.QueryRowContext(ctx, `SELECT `+other+`_data, `+other+`_at FROM links WHERE id = ?`, id).Scan(&data, &at); err != nil {
		fail(w, http.StatusInternalServerError, "hub storage error")
		return
	}
	out := map[string]any{"joined": joined}
	if len(data) > 0 {
		out["peer"] = map[string]any{"data": data, "updated_at": at.Time}
	}
	reply(w, http.StatusOK, out)
}

// remove deletes the link and both blobs. The other member finds out at its
// next exchange (410).
func (h *Hub) remove(w http.ResponseWriter, r *http.Request) {
	if _, _, err := h.member(r); err == errGone {
		w.WriteHeader(http.StatusNoContent) // already gone: same outcome
		return
	} else if err != nil {
		fail(w, http.StatusInternalServerError, "hub storage error")
		return
	}
	if _, err := h.db.ExecContext(r.Context(), `DELETE FROM links WHERE id = ?`, r.PathValue("id")); err != nil {
		fail(w, http.StatusInternalServerError, "hub storage error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetClock overrides the clock (tests).
func (h *Hub) SetClock(now func() time.Time) { h.now = now }

// DB exposes the database (tests).
func (h *Hub) DB() *sql.DB { return h.db }
