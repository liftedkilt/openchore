package hub

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func newHub(t *testing.T, keys ...string) (*Hub, *httptest.Server) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	h, err := New(db, keys)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(func() { srv.Close(); db.Close() })
	return h, srv
}

func do(t *testing.T, srv *httptest.Server, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var rd bytes.Buffer
	if body != nil {
		json.NewEncoder(&rd).Encode(body)
	}
	req, _ := http.NewRequest(method, srv.URL+path, &rd)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out := map[string]any{}
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestLinkLifecycle(t *testing.T) {
	_, srv := newHub(t, "key")

	// Creating needs the hub key.
	if code, _ := do(t, srv, "POST", "/v1/links", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("create without key = %d", code)
	}
	if code, _ := do(t, srv, "POST", "/v1/links", "nope", nil); code != http.StatusUnauthorized {
		t.Fatalf("create with wrong key = %d", code)
	}
	code, link := do(t, srv, "POST", "/v1/links", "key", nil)
	if code != http.StatusCreated {
		t.Fatalf("create = %d %v", code, link)
	}
	id, a, joinCode := link["link"].(string), link["token"].(string), link["join_code"].(string)
	path := "/v1/links/" + id

	// Before anyone joins, the creator can leave data and sees nobody.
	code, out := do(t, srv, "POST", path+"/exchange", a, map[string]any{"data": []byte("from-a")})
	if code != 200 || out["joined"] != false || out["peer"] != nil {
		t.Fatalf("exchange before join = %d %v", code, out)
	}

	// Joining needs the right code, works once, and gives a second token.
	if code, _ := do(t, srv, "POST", path+"/join", "", map[string]any{"code": "wrong"}); code != http.StatusForbidden {
		t.Fatalf("join with wrong code = %d", code)
	}
	code, out = do(t, srv, "POST", path+"/join", "", map[string]any{"code": joinCode})
	if code != 200 {
		t.Fatalf("join = %d %v", code, out)
	}
	b := out["token"].(string)
	if code, _ := do(t, srv, "POST", path+"/join", "", map[string]any{"code": joinCode}); code != http.StatusForbidden {
		t.Fatalf("second join = %d", code)
	}

	// Each side gets the other's data, never its own.
	code, out = do(t, srv, "POST", path+"/exchange", b, map[string]any{"data": []byte("from-b")})
	if code != 200 || out["joined"] != true || out["peer"].(map[string]any)["data"] != "ZnJvbS1h" { // base64 "from-a"
		t.Fatalf("B's exchange = %d %v", code, out)
	}
	code, out = do(t, srv, "POST", path+"/exchange", a, map[string]any{"data": []byte("from-a2")})
	if code != 200 || out["joined"] != true || out["peer"].(map[string]any)["data"] != "ZnJvbS1i" { // "from-b"
		t.Fatalf("A's exchange = %d %v", code, out)
	}

	// A stranger (or the hub key) can't read or delete the link, and isn't
	// told it exists.
	for _, tok := range []string{"", "key", "guess"} {
		if code, _ := do(t, srv, "POST", path+"/exchange", tok, map[string]any{}); code != http.StatusGone {
			t.Fatalf("stranger exchange with %q = %d", tok, code)
		}
		do(t, srv, "DELETE", path, tok, nil)
	}
	if code, _ := do(t, srv, "POST", path+"/exchange", a, map[string]any{"data": []byte("x")}); code != 200 {
		t.Fatalf("link was deleted by a stranger: %d", code)
	}

	// Oversized data is refused.
	if code, _ := do(t, srv, "POST", path+"/exchange", a, map[string]any{"data": make([]byte, MaxBlob+1)}); code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized = %d", code)
	}

	// Either member can end it; the other then gets 410.
	if code, _ := do(t, srv, "DELETE", path, b, nil); code != http.StatusNoContent {
		t.Fatalf("delete = %d", code)
	}
	if code, _ := do(t, srv, "POST", path+"/exchange", a, map[string]any{}); code != http.StatusGone {
		t.Fatalf("exchange after delete = %d", code)
	}
}

func TestSweepAndLimits(t *testing.T) {
	h, srv := newHub(t) // no keys: anyone may create
	now := time.Now()
	h.SetClock(func() time.Time { return now })

	_, stale := do(t, srv, "POST", "/v1/links", "", nil)  // never joined
	_, joined := do(t, srv, "POST", "/v1/links", "", nil) // joined, then idle
	jp := "/v1/links/" + joined["link"].(string)
	do(t, srv, "POST", jp+"/join", "", map[string]any{"code": joined["join_code"]})

	// An invite nobody used is gone after InviteTTL (24h); a joined link stays.
	now = now.Add(InviteTTL + time.Hour)
	if err := h.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if code, _ := do(t, srv, "POST", "/v1/links/"+stale["link"].(string)+"/exchange", stale["token"].(string), map[string]any{}); code != http.StatusGone {
		t.Fatalf("unjoined invite survived: %d", code)
	}
	if code, _ := do(t, srv, "POST", jp+"/exchange", joined["token"].(string), map[string]any{}); code != 200 {
		t.Fatalf("joined link was swept: %d", code)
	}
	// ...until nobody has synced it for IdleTTL.
	now = now.Add(IdleTTL + time.Hour)
	if err := h.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if code, _ := do(t, srv, "POST", jp+"/exchange", joined["token"].(string), map[string]any{}); code != http.StatusGone {
		t.Fatalf("idle link survived: %d", code)
	}

	// Signups are rate limited per address, and the budget refills.
	now = now.Add(2 * time.Minute)
	var last int
	for i := 0; i <= RateSignupPerMinute; i++ {
		last, _ = do(t, srv, "POST", "/v1/links", "", nil)
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("signup %d = %d, want 429", RateSignupPerMinute+1, last)
	}
	now = now.Add(time.Minute)
	if code, _ := do(t, srv, "POST", "/v1/links", "", nil); code != http.StatusCreated {
		t.Fatalf("after the minute passed = %d", code)
	}
}

// On an open hub one address can't hold more than MaxWaitingPerCaller
// unjoined invites, so it can't fill the hub; joining one frees a place.
// A hub with keys has no such cap (its key holders are trusted).
func TestOpenHubCapsWaitingInvitesPerAddress(t *testing.T) {
	h, srv := newHub(t)
	now := time.Now()
	h.SetClock(func() time.Time { return now })
	var first map[string]any
	for i := 0; i < MaxWaitingPerCaller; i++ {
		now = now.Add(time.Minute) // stay inside the signup rate
		code, out := do(t, srv, "POST", "/v1/links", "", nil)
		if code != http.StatusCreated {
			t.Fatalf("invite %d = %d", i+1, code)
		}
		if first == nil {
			first = out
		}
	}
	now = now.Add(time.Minute)
	if code, _ := do(t, srv, "POST", "/v1/links", "", nil); code != http.StatusTooManyRequests {
		t.Fatalf("invite past the cap = %d, want 429", code)
	}
	do(t, srv, "POST", "/v1/links/"+first["link"].(string)+"/join", "", map[string]any{"code": first["join_code"]})
	if code, _ := do(t, srv, "POST", "/v1/links", "", nil); code != http.StatusCreated {
		t.Fatalf("invite after one was joined = %d", code)
	}

	k, ksrv := newHub(t, "key")
	k.SetClock(func() time.Time { return now })
	for i := 0; i <= MaxWaitingPerCaller; i++ {
		now = now.Add(time.Minute)
		if code, _ := do(t, ksrv, "POST", "/v1/links", "key", nil); code != http.StatusCreated {
			t.Fatalf("keyed hub invite %d = %d", i+1, code)
		}
	}
}

// The hub doesn't describe itself to someone who just opens its address.
func TestNoLandingPage(t *testing.T) {
	_, srv := newHub(t, "key")
	for _, path := range []string{"/", "/v1", "/v1/links"} {
		resp, err := srv.Client().Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body := make([]byte, 512)
		n, _ := resp.Body.Read(body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("GET %s = %d", path, resp.StatusCode)
		}
		if bytes.Contains(bytes.ToLower(body[:n]), []byte("openchore")) {
			t.Errorf("GET %s names the software: %q", path, body[:n])
		}
	}
}

func TestCallerAddress(t *testing.T) {
	h, _ := newHub(t)
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.9:1234"
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 203.0.113.7")
	if got := h.caller(r); got != "10.0.0.9" {
		t.Fatalf("without a trusted proxy = %s", got)
	}
	h.TrustProxy = true
	// The last entry is the one our own proxy added; earlier ones are the client's claim.
	if got := h.caller(r); got != "203.0.113.7" {
		t.Fatalf("behind a proxy = %s", got)
	}
	// A proxy may add its own header line instead of appending: still the last one.
	r.Header.Add("X-Forwarded-For", "198.51.100.9")
	if got := h.caller(r); got != "198.51.100.9" {
		t.Fatalf("two header lines = %s", got)
	}
	// Something that isn't an address is ignored, not used as a key.
	r.Header.Set("X-Forwarded-For", "not-an-address")
	if got := h.caller(r); got != "10.0.0.9" {
		t.Fatalf("junk header = %s", got)
	}
}

// Limits count an IPv6 caller by its /64: one host normally holds a whole
// /64 and could otherwise rotate addresses for a fresh budget.
func TestLimitKey(t *testing.T) {
	for addr, want := range map[string]string{
		"203.0.113.7":          "203.0.113.7",
		"::ffff:203.0.113.7":   "203.0.113.7",
		"2001:db8:1:2::1":      "2001:db8:1:2::/64",
		"2001:db8:1:2:ffff::9": "2001:db8:1:2::/64",
		"2001:db8:1:3::1":      "2001:db8:1:3::/64",
		"fe80::1%eth0":         "fe80::/64",
		"not-an-address":       "not-an-address",
	} {
		if got := limitKey(addr); got != want {
			t.Errorf("limitKey(%s) = %s, want %s", addr, got, want)
		}
	}
}

// On an open hub, addresses in one IPv6 /64 share the cap on waiting
// invites; a different /64 has its own.
func TestOpenHubCapsWaitingInvitesPerIPv6Prefix(t *testing.T) {
	h, srv := newHub(t)
	h.TrustProxy = true
	now := time.Now()
	h.SetClock(func() time.Time { return now })
	create := func(from string) int {
		t.Helper()
		now = now.Add(time.Minute) // stay inside the signup rate
		req, _ := http.NewRequest("POST", srv.URL+"/v1/links", nil)
		req.Header.Set("X-Forwarded-For", from)
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	for i := 0; i < MaxWaitingPerCaller; i++ {
		if code := create(fmt.Sprintf("2001:db8:1:2::%x", i+1)); code != http.StatusCreated {
			t.Fatalf("invite %d = %d", i+1, code)
		}
	}
	if code := create("2001:db8:1:2::ffff"); code != http.StatusTooManyRequests {
		t.Fatalf("a new address in the same /64 = %d, want 429", code)
	}
	if code := create("2001:db8:1:3::1"); code != http.StatusCreated {
		t.Fatalf("another /64 = %d", code)
	}
}

// Opening a hub database made by this or an older version works, and a
// joined link no longer says who made it.
func TestHubReopensAndForgetsWhoMadeAJoinedLink(t *testing.T) {
	h, srv := newHub(t)
	if _, err := New(h.DB(), nil); err != nil {
		t.Fatalf("reopening a database that already has created_by: %v", err)
	}
	_, l := do(t, srv, "POST", "/v1/links", "", nil)
	var by string
	h.DB().QueryRow(`SELECT created_by FROM links WHERE id = ?`, l["link"]).Scan(&by)
	if by == "" || strings.Contains(by, "127.0.0.1") || by == hash("127.0.0.1") {
		t.Fatalf("created_by = %q, want a keyed hash", by)
	}
	do(t, srv, "POST", "/v1/links/"+l["link"].(string)+"/join", "", map[string]any{"code": l["join_code"]})
	h.DB().QueryRow(`SELECT created_by FROM links WHERE id = ?`, l["link"]).Scan(&by)
	if by != "" {
		t.Fatalf("a joined link still says who made it: %q", by)
	}
}

// Requests through a proxy the hub hasn't been told to trust get one warning.
func TestHubWarnsAboutAnUntrustedProxy(t *testing.T) {
	h, srv := newHub(t)
	var buf bytes.Buffer
	h.Log = log.New(&buf, "", 0)
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest("POST", srv.URL+"/v1/links", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.7")
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	if n := strings.Count(buf.String(), "HUB_TRUST_PROXY"); n != 1 {
		t.Fatalf("proxy warnings = %d:\n%s", n, buf.String())
	}
}

func TestInviteTTLDefaultAndOverride(t *testing.T) {
	if InviteTTL != 24*time.Hour {
		t.Fatalf("InviteTTL = %v, want 24h", InviteTTL)
	}

	h, srv := newHub(t, "key")
	base := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	now := base
	h.SetClock(func() time.Time { return now })

	// Default TTL is 24h.
	code, defLink := do(t, srv, "POST", "/v1/links", "key", nil)
	if code != http.StatusCreated {
		t.Fatalf("create default = %d", code)
	}
	exp, err := time.Parse(time.RFC3339Nano, defLink["expires_at"].(string))
	if err != nil {
		t.Fatalf("parse expires_at: %v", err)
	}
	if got := exp.Sub(base); got != 24*time.Hour {
		t.Fatalf("default invite TTL = %v, want 24h", got)
	}

	// Custom Hub.InviteTTL overrides the default for new invites.
	h.InviteTTL = 2 * time.Hour
	code, shortLink := do(t, srv, "POST", "/v1/links", "key", nil)
	if code != http.StatusCreated {
		t.Fatalf("create short = %d", code)
	}
	exp, err = time.Parse(time.RFC3339Nano, shortLink["expires_at"].(string))
	if err != nil {
		t.Fatalf("parse short expires_at: %v", err)
	}
	if got := exp.Sub(base); got != 2*time.Hour {
		t.Fatalf("custom invite TTL = %v, want 2h", got)
	}

	// Joined link survives past InviteTTL while waiting for parent confirmation.
	code, joinedLink := do(t, srv, "POST", "/v1/links", "key", nil)
	if code != http.StatusCreated {
		t.Fatalf("create joined = %d", code)
	}
	jp := "/v1/links/" + joinedLink["link"].(string)
	if code, _ := do(t, srv, "POST", jp+"/join", "", map[string]any{"code": joinedLink["join_code"]}); code != http.StatusOK {
		t.Fatalf("join before expiry = %d", code)
	}

	// Non-positive Hub.InviteTTL falls back to the 24h package default.
	h.InviteTTL = -time.Hour
	code, fallbackLink := do(t, srv, "POST", "/v1/links", "key", nil)
	if code != http.StatusCreated {
		t.Fatalf("create fallback = %d", code)
	}
	exp, _ = time.Parse(time.RFC3339Nano, fallbackLink["expires_at"].(string))
	if got := exp.Sub(base); got != 24*time.Hour {
		t.Fatalf("non-positive fallback TTL = %v, want 24h", got)
	}

	// Advance past 2h: shortLink can no longer be joined and is swept, while
	// joinedLink and defLink remain live.
	now = base.Add(2*time.Hour + time.Minute)
	if code, _ := do(t, srv, "POST", "/v1/links/"+shortLink["link"].(string)+"/join", "", map[string]any{"code": shortLink["join_code"]}); code != http.StatusForbidden {
		t.Fatalf("join expired short invite = %d, want 403", code)
	}
	if err := h.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if code, _ := do(t, srv, "POST", "/v1/links/"+shortLink["link"].(string)+"/exchange", shortLink["token"].(string), map[string]any{}); code != http.StatusGone {
		t.Fatalf("expired short link survived sweep = %d", code)
	}
	if code, _ := do(t, srv, "POST", jp+"/exchange", joinedLink["token"].(string), map[string]any{}); code != http.StatusOK {
		t.Fatalf("joined link did not survive past InviteTTL = %d", code)
	}
	if code, _ := do(t, srv, "POST", "/v1/links/"+defLink["link"].(string)+"/join", "", map[string]any{"code": defLink["join_code"]}); code != http.StatusOK {
		t.Fatalf("default 24h link should still be joinable at 2h = %d", code)
	}
}

