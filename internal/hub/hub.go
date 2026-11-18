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
}

// New prepares the database and returns the hub. keys are the secrets that
// may create links; with none, anyone who can reach the hub can.
func New(db *sql.DB, keys []string) (*Hub, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	h := &Hub{db: db, now: time.Now, mux: http.NewServeMux()}
	for _, k := range keys {
		if k = strings.TrimSpace(k); k != "" {
			h.keys = append(h.keys, hash(k))
		}
	}
	h.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok\n")) })
	h.mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("OpenChore coordination server.\n\n" +
			"This passes encrypted updates between families who have linked their OpenChore apps.\n" +
			"It can't read them, and there is nothing to see or sign in to here.\n"))
	})
	h.mux.HandleFunc("POST /v1/links", h.create)
	h.mux.HandleFunc("POST /v1/links/{id}/join", h.join)
	h.mux.HandleFunc("POST /v1/links/{id}/exchange", h.exchange)
	h.mux.HandleFunc("DELETE /v1/links/{id}", h.remove)
	return h, nil
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	addr := h.caller(r)
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	signup := r.Method == http.MethodPost && (r.URL.Path == "/v1/links" || strings.HasSuffix(r.URL.Path, "/join"))
	switch {
	case r.URL.Path == "/healthz":
		h.mux.ServeHTTP(rec, r) // probes aren't counted or logged
		return
	case !h.allow(addr, signup):
		rec.Header().Set("Retry-After", "60")
		fail(rec, http.StatusTooManyRequests, "too many requests; try again in a minute")
	default:
		r.Body = http.MaxBytesReader(rec, r.Body, MaxBlob*2)
		h.mux.ServeHTTP(rec, r)
	}
	if h.Log != nil {
		// Link ids aren't secret (tokens are, and they're in headers).
		h.Log.Printf("%s %s %s %d", addr, r.Method, r.URL.Path, rec.status)
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
		if parts := strings.Split(r.Header.Get("X-Forwarded-For"), ","); len(parts) > 0 {
			if ip := strings.TrimSpace(parts[len(parts)-1]); ip != "" {
				return ip
			}
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
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
	id, token, code := random(12), random(32), random(18)
	now := h.now()
	expires := now.Add(InviteTTL)
	if _, err := h.db.ExecContext(ctx,
		`INSERT INTO links (id, created_at, touched_at, join_hash, join_expires, a_token) VALUES (?, ?, ?, ?, ?, ?)`,
		id, now, now, hash(code), expires, hash(token)); err != nil {
		fail(w, http.StatusInternalServerError, "hub storage error")
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
		`UPDATE links SET b_token = ?, join_hash = '', touched_at = ?
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
