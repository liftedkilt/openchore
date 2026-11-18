package hub

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

	// An invite nobody used is gone after a week; a joined link stays.
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
