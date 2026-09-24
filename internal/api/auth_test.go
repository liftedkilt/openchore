package api_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc/oidctest"
	"github.com/golang-migrate/migrate/v4"
	msqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/liftedkilt/openchore/internal/api"
	"github.com/liftedkilt/openchore/internal/config"
	"github.com/liftedkilt/openchore/internal/store"
	"github.com/liftedkilt/openchore/internal/webhook"
	"github.com/liftedkilt/openchore/migrations"
)

func hasSessionCookie(resp *http.Response) bool {
	for _, c := range resp.Cookies() {
		if c.Name == "openchore_session" && c.Value != "" && c.MaxAge >= 0 {
			return true
		}
	}
	return false
}

func sessionCookie(t *testing.T, resp *http.Response) *http.Cookie {
	t.Helper()
	for _, c := range resp.Cookies() {
		if c.Name == "openchore_session" && c.Value != "" {
			return c
		}
	}
	t.Fatal("no session cookie in response")
	return nil
}

func cookieHeaders(c *http.Cookie) map[string]string {
	return map[string]string{"Cookie": c.Name + "=" + c.Value}
}

// login signs userID in with an optional PIN and returns the response.
func (e *testEnv) login(t *testing.T, userID int, pin string, expected int) *http.Response {
	t.Helper()
	body := map[string]any{"user_id": userID}
	if pin != "" {
		body["pin"] = pin
	}
	return e.expectStatus(t, "POST", "/api/auth/login", body, nil, expected)
}

func errorCode(t *testing.T, resp *http.Response) string {
	t.Helper()
	var body map[string]any
	decodeBody(t, resp, &body)
	code, _ := body["code"].(string)
	return code
}

// =================== SESSIONS ===================

func TestLegacyUserIDHeaderIsNotTrusted(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "GET", "/api/admin/tokens", nil,
		map[string]string{"X-User-ID": "1"}, http.StatusUnauthorized)
	env.expectStatus(t, "POST", "/api/admin/tokens", map[string]any{"name": "x"},
		map[string]string{"X-User-ID": "1"}, http.StatusUnauthorized)
}

func TestLoginTapSetsSessionCookie(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")

	resp := env.login(t, kid, "", http.StatusOK)
	cookie := sessionCookie(t, resp)
	if !cookie.HttpOnly {
		t.Fatal("session cookie must be HttpOnly")
	}
	if cookie.MaxAge != 0 {
		t.Fatalf("kiosk sessions should be browser-session cookies, got MaxAge=%d", cookie.MaxAge)
	}

	resp = env.expectStatus(t, "GET", "/api/auth/me", nil, cookieHeaders(cookie), http.StatusOK)
	var me map[string]any
	decodeBody(t, resp, &me)
	if me["user"].(map[string]any)["name"] != "Kid" {
		t.Fatalf("unexpected /me user: %v", me["user"])
	}
	session := me["session"].(map[string]any)
	if session["method"] != "tap" || session["persistent"] != false {
		t.Fatalf("unexpected session: %v", session)
	}

	// A kid session can't reach admin routes.
	env.expectStatus(t, "GET", "/api/admin/tokens", nil, cookieHeaders(cookie), http.StatusForbidden)
}

func TestLoginWithPin(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/pin", kid),
		map[string]any{"new_pin": "4321"}, childHeaders(kid), http.StatusOK)

	if code := errorCode(t, env.login(t, kid, "", http.StatusUnauthorized)); code != "incorrect_pin" {
		t.Fatalf("expected incorrect_pin, got %q", code)
	}
	env.login(t, kid, "1111", http.StatusUnauthorized)
	resp := env.login(t, kid, "4321", http.StatusOK)
	if !hasSessionCookie(resp) {
		t.Fatal("expected a session cookie")
	}
	var body map[string]any
	decodeBody(t, resp, &body)
	if tok, _ := body["token"].(string); !strings.HasPrefix(tok, "ocs1.") {
		t.Fatalf("expected an ocs1 token in the body, got %v", body["token"])
	}
}

func TestLoginPinLockout(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/pin", kid),
		map[string]any{"new_pin": "4321"}, childHeaders(kid), http.StatusOK)

	for i := 0; i < 5; i++ {
		env.login(t, kid, "0000", http.StatusUnauthorized)
	}
	// Locked now, even with the right PIN.
	resp := env.login(t, kid, "4321", http.StatusTooManyRequests)
	if code := errorCode(t, resp); code != "locked_out" {
		t.Fatalf("expected locked_out, got %q", code)
	}
	// Other profiles are unaffected.
	env.login(t, 1, "", http.StatusForbidden) // admin without credential: must claim
}

func TestTamperedAndExpiredSessionsRejected(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	tok := sessionToken(1)
	tampered := tok[:len(tok)-2] + "xx"
	env.expectStatus(t, "GET", "/api/auth/me", nil,
		map[string]string{"Authorization": "Bearer " + tampered}, http.StatusUnauthorized)

	forged := api.SignSessionToken([]byte("some-other-secret-that-is-long-enough!!"), api.SessionClaims{
		UserID: 1, Method: api.SessionMethodPin, Expires: time.Now().Add(time.Hour).Unix(),
	})
	env.expectStatus(t, "GET", "/api/auth/me", nil,
		map[string]string{"Authorization": "Bearer " + forged}, http.StatusUnauthorized)

	expired := api.SignSessionToken(testSessionSecret, api.SessionClaims{
		UserID: 1, Method: api.SessionMethodPin, Expires: time.Now().Add(-time.Minute).Unix(),
	})
	env.expectStatus(t, "GET", "/api/auth/me", nil,
		map[string]string{"Authorization": "Bearer " + expired}, http.StatusUnauthorized)
}

func TestLogoutEverywhereRevokesSessions(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")

	first := sessionCookie(t, env.login(t, kid, "", http.StatusOK))
	second := sessionCookie(t, env.login(t, kid, "", http.StatusOK))

	env.expectStatus(t, "POST", "/api/auth/logout-everywhere", nil, cookieHeaders(first), http.StatusNoContent)
	env.expectStatus(t, "GET", "/api/auth/me", nil, cookieHeaders(first), http.StatusUnauthorized)
	env.expectStatus(t, "GET", "/api/auth/me", nil, cookieHeaders(second), http.StatusUnauthorized)

	// A fresh login works again.
	fresh := sessionCookie(t, env.login(t, kid, "", http.StatusOK))
	env.expectStatus(t, "GET", "/api/auth/me", nil, cookieHeaders(fresh), http.StatusOK)
}

func TestSecretSettingsNotReadable(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	env.expectStatus(t, "GET", "/api/admin/settings/session_secret", nil, adminHeaders(), http.StatusForbidden)
	env.expectStatus(t, "GET", "/api/admin/settings/admin_passcode", nil, adminHeaders(), http.StatusForbidden)
}

// =================== ADMIN PROFILES ===================

func TestAdminWithoutCredentialClaimsPinWithLegacyPasscode(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t) // no PIN, like every admin on a pre-upgrade install

	resp := env.login(t, 1, "", http.StatusForbidden)
	var body map[string]any
	decodeBody(t, resp, &body)
	if body["code"] != "admin_setup_required" || body["legacy_passcode"] != true {
		t.Fatalf("expected admin_setup_required with legacy passcode, got %v", body)
	}

	env.expectStatus(t, "POST", "/api/auth/login", map[string]any{
		"user_id": 1, "legacy_passcode": "1111", "new_pin": "2468",
	}, nil, http.StatusUnauthorized)
	env.expectStatus(t, "POST", "/api/auth/login", map[string]any{
		"user_id": 1, "legacy_passcode": "0000", "new_pin": "12",
	}, nil, http.StatusBadRequest)
	resp = env.expectStatus(t, "POST", "/api/auth/login", map[string]any{
		"user_id": 1, "legacy_passcode": "0000", "new_pin": "2468",
	}, nil, http.StatusOK)
	cookie := sessionCookie(t, resp)
	env.expectStatus(t, "GET", "/api/admin/tokens", nil, cookieHeaders(cookie), http.StatusOK)

	// The PIN now works on its own, and the household passcode is retired
	// because every admin has a credential.
	env.login(t, 1, "2468", http.StatusOK)
	passcode, err := env.store.GetSetting(t.Context(), "admin_passcode")
	if err != nil {
		t.Fatal(err)
	}
	if passcode != "" {
		t.Fatal("expected legacy admin passcode to be retired")
	}
}

func TestAdminProfilesRequireCredential(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")

	// Creating an admin without a PIN is refused.
	env.expectStatus(t, "POST", "/api/users", map[string]any{
		"name": "Parent Two", "role": "admin",
	}, adminHeaders(), http.StatusBadRequest)
	resp := env.expectStatus(t, "POST", "/api/users", map[string]any{
		"name": "Parent Two", "role": "admin", "pin": "1357",
	}, adminHeaders(), http.StatusCreated)
	var p2 map[string]any
	decodeBody(t, resp, &p2)
	if p2["has_pin"] != true {
		t.Fatal("expected new admin to have a pin")
	}

	// Promoting a kid with no PIN is refused.
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d", kid), map[string]any{"role": "admin"},
		adminHeaders(), http.StatusBadRequest)

	// Demoting the last remaining admin is refused.
	p2ID := int(p2["id"].(float64))
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d", p2ID), map[string]any{"role": "child"},
		adminHeaders(), http.StatusOK)
	env.expectStatus(t, "PUT", "/api/users/1", map[string]any{"role": "child"},
		adminHeaders(), http.StatusConflict)
}

func TestRoleChangeRevokesSessions(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	resp := env.expectStatus(t, "POST", "/api/users", map[string]any{
		"name": "Parent Two", "role": "admin", "pin": "1357",
	}, adminHeaders(), http.StatusCreated)
	var p2 map[string]any
	decodeBody(t, resp, &p2)
	p2ID := int(p2["id"].(float64))

	cookie := sessionCookie(t, env.login(t, p2ID, "1357", http.StatusOK))
	env.expectStatus(t, "GET", "/api/admin/tokens", nil, cookieHeaders(cookie), http.StatusOK)

	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d", p2ID), map[string]any{"role": "child"},
		adminHeaders(), http.StatusOK)
	env.expectStatus(t, "GET", "/api/admin/tokens", nil, cookieHeaders(cookie), http.StatusUnauthorized)
}

// =================== PARTICIPATION & COMPLETING ON BEHALF ===================

// createScheduledChore creates a chore assigned to userID every day and
// returns the schedule ID.
func (e *testEnv) createScheduledChore(t *testing.T, title string, userID int, extra map[string]any) int {
	t.Helper()
	body := map[string]any{"title": title, "category": "core", "points_value": 5}
	for k, v := range extra {
		body[k] = v
	}
	resp := e.expectStatus(t, "POST", "/api/chores", body, adminHeaders(), http.StatusCreated)
	var chore map[string]any
	decodeBody(t, resp, &chore)
	resp = e.expectStatus(t, "POST", fmt.Sprintf("/api/chores/%d/schedules", int(chore["id"].(float64))),
		map[string]any{"assigned_to": userID, "specific_date": time.Now().Format("2006-01-02")}, adminHeaders(), http.StatusCreated)
	var sched map[string]any
	decodeBody(t, resp, &sched)
	return int(sched["id"].(float64))
}

func (e *testEnv) balance(t *testing.T, userID int) int {
	t.Helper()
	resp := e.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", userID), nil, nil, http.StatusOK)
	var pts map[string]any
	decodeBody(t, resp, &pts)
	return int(pts["balance"].(float64))
}

func TestKidCannotCompleteSiblingsChore(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidA := env.createChild(t, "A")
	kidB := env.createChild(t, "B")
	sched := env.createScheduledChore(t, "Dishes", kidA, nil)

	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", sched), map[string]any{},
		childHeaders(kidB), http.StatusForbidden)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", sched), map[string]any{
		"completed_by": kidB,
	}, childHeaders(kidA), http.StatusForbidden)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", sched), map[string]any{},
		childHeaders(kidA), http.StatusCreated)
	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/schedules/%d/complete", sched), nil,
		childHeaders(kidB), http.StatusForbidden)
}

func TestParentCompletesKidsChoreOnTheirBehalf(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	sched := env.createScheduledChore(t, "Photo chore", kid, map[string]any{"requires_photo": true})

	// The kid still needs a photo (or to skip it and wait for approval)...
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", sched), map[string]any{},
		childHeaders(kid), http.StatusBadRequest)

	// ...but a parent can vouch for it without one, and the kid is credited.
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", sched), map[string]any{},
		adminHeaders(), http.StatusCreated)
	if got := env.balance(t, kid); got != 5 {
		t.Fatalf("expected kid to be credited 5, got %d", got)
	}
	if got := env.balance(t, 1); got != 0 {
		t.Fatalf("expected parent to get nothing, got %d", got)
	}

	// And a parent can undo it on the kid's behalf.
	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/schedules/%d/complete", sched), nil,
		adminHeaders(), http.StatusNoContent)
	if got := env.balance(t, kid); got != 0 {
		t.Fatalf("expected kid balance back to 0, got %d", got)
	}
}

func TestParentsTakePart(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	sched := env.createScheduledChore(t, "Mow lawn", 1, nil)

	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", sched), map[string]any{},
		adminHeaders(), http.StatusCreated)
	if got := env.balance(t, 1); got != 5 {
		t.Fatalf("expected parent to earn 5, got %d", got)
	}

	// Parents can pick a theme like everyone else.
	env.expectStatus(t, "PUT", "/api/users/1/theme", map[string]any{"theme": "galaxy"},
		adminHeaders(), http.StatusOK)
}

// =================== UPLOAD LINKS ===================

func TestUploadLinkIsScopedToOneChore(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	sibling := env.createChild(t, "Sibling")
	mine := env.createScheduledChore(t, "Mine", kid, nil)
	other := env.createScheduledChore(t, "Other", kid, nil)
	theirs := env.createScheduledChore(t, "Theirs", sibling, nil)

	env.expectStatus(t, "POST", "/api/auth/upload-link", map[string]any{"schedule_id": theirs},
		childHeaders(kid), http.StatusForbidden)

	resp := env.expectStatus(t, "POST", "/api/auth/upload-link", map[string]any{"schedule_id": mine},
		childHeaders(kid), http.StatusOK)
	var link map[string]any
	decodeBody(t, resp, &link)
	h := map[string]string{"Authorization": "Bearer " + link["token"].(string)}

	env.expectStatus(t, "GET", "/api/rewards", nil, h, http.StatusForbidden)
	env.expectStatus(t, "POST", "/api/auth/upload-link", map[string]any{"schedule_id": mine}, h, http.StatusForbidden)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", other), map[string]any{}, h, http.StatusForbidden)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", mine), map[string]any{}, h, http.StatusCreated)
}

// =================== OIDC ===================

// fakeIdP is a minimal OpenID provider: discovery and JWKS come from
// oidctest; /auth and /token implement the authorization-code flow with PKCE.
type fakeIdP struct {
	t      *testing.T
	srv    *httptest.Server
	inner  *oidctest.Server
	key    *rsa.PrivateKey
	client string

	mu       sync.Mutex
	subject  string
	email    string
	codes    map[string]fakeCode
	lastAuth url.Values
}

type fakeCode struct {
	nonce, challenge, subject, email string
}

func newFakeIdP(t *testing.T) *fakeIdP {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeIdP{
		t: t, key: key, client: "openchore-test",
		codes: map[string]fakeCode{}, subject: "sub-1", email: "one@example.com",
		inner: &oidctest.Server{PublicKeys: []oidctest.PublicKey{{PublicKey: key.Public(), KeyID: "k1", Algorithm: oidc.RS256}}},
	}
	f.srv = httptest.NewServer(f)
	f.inner.SetIssuer(f.srv.URL)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeIdP) setUser(sub, email string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subject, f.email = sub, email
}

func (f *fakeIdP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/auth":
		q := r.URL.Query()
		f.mu.Lock()
		f.lastAuth = q
		code := fmt.Sprintf("code-%d", len(f.codes)+1)
		f.codes[code] = fakeCode{nonce: q.Get("nonce"), challenge: q.Get("code_challenge"), subject: f.subject, email: f.email}
		f.mu.Unlock()
		if q.Get("code_challenge_method") != "S256" {
			http.Error(w, "pkce required", http.StatusBadRequest)
			return
		}
		back := q.Get("redirect_uri") + "?" + url.Values{"code": {code}, "state": {q.Get("state")}}.Encode()
		http.Redirect(w, r, back, http.StatusFound)
	case "/token":
		_ = r.ParseForm()
		f.mu.Lock()
		c, ok := f.codes[r.PostForm.Get("code")]
		delete(f.codes, r.PostForm.Get("code"))
		f.mu.Unlock()
		sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
		if !ok || base64.RawURLEncoding.EncodeToString(sum[:]) != c.challenge {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		claims, _ := json.Marshal(map[string]any{
			"iss": f.srv.URL, "aud": f.client, "sub": c.subject, "email": c.email,
			"email_verified": true, "name": "Test Person", "nonce": c.nonce,
			"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
		})
		idToken := oidctest.SignIDToken(f.key, "k1", oidc.RS256, string(claims))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at", "token_type": "Bearer", "expires_in": 3600, "id_token": idToken,
		})
	default:
		f.inner.ServeHTTP(w, r)
	}
}

// setupTestWithOIDC is setupTest with one OIDC provider ("pocket") pointed
// at a fake IdP.
func setupTestWithOIDC(t *testing.T, prompt string) (*testEnv, *fakeIdP) {
	t.Helper()
	idp := newFakeIdP(t)

	db, err := sql.Open("sqlite", ":memory:?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	driver, err := msqlite.WithInstance(db, &msqlite.Config{})
	if err != nil {
		t.Fatal(err)
	}
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "sqlite", driver)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatal(err)
	}

	s := store.New(db)
	d := webhook.NewDispatcher(s)
	sessions := api.NewSessionManager(testSessionSecret)
	authCfg, err := config.ResolveAuth(&config.Config{Auth: &config.AuthConfig{
		OIDC: []config.OIDCProviderConfig{{
			ID: "pocket", Name: "Pocket ID", Issuer: idp.srv.URL, ClientID: idp.client, ClientSecret: "s3cret", Prompt: prompt,
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	oidcSvc := api.NewOIDCService(s, sessions, d, authCfg)
	router, chores, _ := api.NewRouter(s, d, api.Auth{Sessions: sessions, OIDC: oidcSvc})
	server := httptest.NewServer(router)
	t.Cleanup(func() {
		server.Close()
		db.Close()
	})
	return &testEnv{server: server, db: db, store: s, chores: chores}, idp
}

// browser is an HTTP client with a cookie jar that follows redirects through
// the API and the IdP but stops at the first page of the SPA.
type browser struct {
	t      *testing.T
	env    *testEnv
	client *http.Client
}

func newBrowser(t *testing.T, env *testEnv) *browser {
	jar, _ := cookiejar.New(nil)
	b := &browser{t: t, env: env}
	b.client = &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if strings.HasPrefix(req.URL.String(), env.server.URL) && !strings.HasPrefix(req.URL.Path, "/api/") {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	return b
}

// get follows the flow and returns the final SPA location (path + query).
func (b *browser) get(path string) string {
	b.t.Helper()
	resp, err := b.client.Get(b.env.server.URL + path)
	if err != nil {
		b.t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		b.t.Fatalf("GET %s: expected a redirect to the app, got %d", path, resp.StatusCode)
	}
	return resp.Header.Get("Location")
}

func (b *browser) do(method, path string, body any) *http.Response {
	b.t.Helper()
	var rdr *strings.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rdr = strings.NewReader(string(raw))
	} else {
		rdr = strings.NewReader("")
	}
	req, _ := http.NewRequest(method, b.env.server.URL+path, rdr)
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.client.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	return resp
}

func (b *browser) expect(method, path string, body any, status int) *http.Response {
	b.t.Helper()
	resp := b.do(method, path, body)
	if resp.StatusCode != status {
		b.t.Fatalf("%s %s: expected %d, got %d", method, path, status, resp.StatusCode)
	}
	return resp
}

func TestOIDCProvidersListed(t *testing.T) {
	env, _ := setupTestWithOIDC(t, "")
	resp := env.expectStatus(t, "GET", "/api/auth/providers", nil, nil, http.StatusOK)
	var providers []map[string]any
	decodeBody(t, resp, &providers)
	if len(providers) != 1 || providers[0]["id"] != "pocket" || providers[0]["name"] != "Pocket ID" {
		t.Fatalf("unexpected providers: %v", providers)
	}
}

func TestOIDCLinkThenLogin(t *testing.T) {
	env, idp := setupTestWithOIDC(t, "login")
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")

	// Linking requires a session.
	anon := newBrowser(t, env)
	if loc := anon.get("/api/auth/oidc/pocket/start?mode=link&return=/account"); !strings.Contains(loc, "auth_error=session_required") {
		t.Fatalf("expected session_required, got %s", loc)
	}

	// Kid signs in by tapping, then links their provider account.
	b := newBrowser(t, env)
	b.expect("POST", "/api/auth/login", map[string]any{"user_id": kid}, http.StatusOK)
	loc := b.get("/api/auth/oidc/pocket/start?mode=link&return=/account")
	if loc != "/account?linked=pocket" {
		t.Fatalf("unexpected link redirect: %s", loc)
	}
	if idp.lastAuth.Get("prompt") != "login" {
		t.Fatalf("expected prompt=login to be forwarded, got %q", idp.lastAuth.Get("prompt"))
	}

	// The public listing now advertises the provider on the kid's profile.
	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d", kid), nil, nil, http.StatusOK)
	var u map[string]any
	decodeBody(t, resp, &u)
	if ps, _ := u["auth_providers"].([]any); len(ps) != 1 || ps[0] != "pocket" {
		t.Fatalf("expected auth_providers=[pocket], got %v", u["auth_providers"])
	}
	// ...without leaking the email.
	resp = env.expectStatus(t, "GET", "/api/users", nil, nil, http.StatusOK)
	var raw strings.Builder
	var list any
	decodeBody(t, resp, &list)
	_ = json.NewEncoder(&raw).Encode(list)
	if strings.Contains(raw.String(), "one@example.com") || strings.Contains(raw.String(), "sub-1") {
		t.Fatal("public user listing leaks identity details")
	}

	// Tapping the profile now requires the provider.
	resp = env.login(t, kid, "", http.StatusForbidden)
	if code := errorCode(t, resp); code != "oidc_required" {
		t.Fatalf("expected oidc_required, got %q", code)
	}

	// Login from a fresh device via the profile hint.
	phone := newBrowser(t, env)
	loc = phone.get(fmt.Sprintf("/api/auth/oidc/pocket/start?user_id=%d", kid))
	if loc != "/" {
		t.Fatalf("expected redirect to /, got %s", loc)
	}
	if idp.lastAuth.Get("login_hint") != "one@example.com" {
		t.Fatalf("expected login_hint with linked email, got %q", idp.lastAuth.Get("login_hint"))
	}
	resp = phone.expect("GET", "/api/auth/me", nil, http.StatusOK)
	var me map[string]any
	decodeBody(t, resp, &me)
	session := me["session"].(map[string]any)
	if session["method"] != "oidc" || session["persistent"] != true || session["provider"] != "pocket" {
		t.Fatalf("unexpected session: %v", session)
	}
}

func TestOIDCLoginErrors(t *testing.T) {
	env, idp := setupTestWithOIDC(t, "")
	env.createAdmin(t)
	kidA := env.createChild(t, "A")
	kidB := env.createChild(t, "B")

	// A links sub-1.
	a := newBrowser(t, env)
	a.expect("POST", "/api/auth/login", map[string]any{"user_id": kidA}, http.StatusOK)
	a.get("/api/auth/oidc/pocket/start?mode=link")

	// Tapping B but signing in as A's account is refused.
	if loc := newBrowser(t, env).get(fmt.Sprintf("/api/auth/oidc/pocket/start?user_id=%d", kidB)); !strings.Contains(loc, "auth_error=wrong_account") {
		t.Fatalf("expected wrong_account, got %s", loc)
	}

	// B can't link an account that already belongs to A.
	b := newBrowser(t, env)
	b.expect("POST", "/api/auth/login", map[string]any{"user_id": kidB}, http.StatusOK)
	if loc := b.get("/api/auth/oidc/pocket/start?mode=link&return=/account"); !strings.Contains(loc, "auth_error=linked_to_other_profile") {
		t.Fatalf("expected linked_to_other_profile, got %s", loc)
	}

	// An unknown account can't sign in.
	idp.setUser("sub-unknown", "nobody@example.com")
	if loc := newBrowser(t, env).get("/api/auth/oidc/pocket/start"); !strings.Contains(loc, "auth_error=not_linked") {
		t.Fatalf("expected not_linked, got %s", loc)
	}

	// A callback without the flow cookie (CSRF / replay) is rejected.
	if loc := newBrowser(t, env).get("/api/auth/oidc/pocket/callback?code=x&state=y"); !strings.Contains(loc, "auth_error=invalid_state") {
		t.Fatalf("expected invalid_state, got %s", loc)
	}

	// Open redirects are ignored.
	idp.setUser("sub-1", "one@example.com")
	if loc := newBrowser(t, env).get("/api/auth/oidc/pocket/start?return=//evil.example"); loc != "/" {
		t.Fatalf("expected unsafe return path to be dropped, got %s", loc)
	}

	env.expectStatus(t, "GET", "/api/auth/oidc/nope/start", nil, nil, http.StatusNotFound)
}

func TestOIDCUnlinkRevokesProviderSessions(t *testing.T) {
	env, _ := setupTestWithOIDC(t, "")
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")

	kiosk := newBrowser(t, env)
	kiosk.expect("POST", "/api/auth/login", map[string]any{"user_id": kid}, http.StatusOK)
	kiosk.get("/api/auth/oidc/pocket/start?mode=link")

	phone := newBrowser(t, env)
	phone.get(fmt.Sprintf("/api/auth/oidc/pocket/start?user_id=%d", kid))
	phone.expect("GET", "/api/auth/me", nil, http.StatusOK)

	resp := kiosk.expect("GET", fmt.Sprintf("/api/users/%d/identities", kid), nil, http.StatusOK)
	var ids []map[string]any
	decodeBody(t, resp, &ids)
	if len(ids) != 1 || ids[0]["email"] != "one@example.com" {
		t.Fatalf("unexpected identities: %v", ids)
	}
	// Siblings can't see or remove each other's linked accounts.
	sib := env.createChild(t, "Sib")
	env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/identities", kid), nil, childHeaders(sib), http.StatusForbidden)

	kiosk.expect("DELETE", fmt.Sprintf("/api/users/%d/identities/%d", kid, int(ids[0]["id"].(float64))), nil, http.StatusNoContent)

	// The phone's provider session is gone; the kiosk that unlinked stays signed in.
	phone.expect("GET", "/api/auth/me", nil, http.StatusUnauthorized)
	kiosk.expect("GET", "/api/auth/me", nil, http.StatusOK)
	env.login(t, kid, "", http.StatusOK)
}

func TestOIDCAdminCannotUnlinkOnlyCredential(t *testing.T) {
	env, _ := setupTestWithOIDC(t, "")
	resp := env.expectStatus(t, "POST", "/api/setup", map[string]any{
		"parent":   map[string]any{"name": "Robin", "pin": "2468"},
		"children": []map[string]any{{"name": "Kid"}},
	}, nil, http.StatusCreated)
	var setup map[string]any
	decodeBody(t, resp, &setup)
	parentID := int(setup["admin"].(map[string]any)["id"].(float64))

	b := newBrowser(t, env)
	b.expect("POST", "/api/auth/login", map[string]any{"user_id": parentID, "pin": "2468"}, http.StatusOK)
	b.get("/api/auth/oidc/pocket/start?mode=link")

	// With a linked account, the parent may drop their PIN...
	b.expect("DELETE", fmt.Sprintf("/api/users/%d/pin", parentID), map[string]any{"current_pin": "2468"}, http.StatusOK)
	// ...but then can't unlink their last credential.
	resp = b.expect("GET", fmt.Sprintf("/api/users/%d/identities", parentID), nil, http.StatusOK)
	var ids []map[string]any
	decodeBody(t, resp, &ids)
	b.expect("DELETE", fmt.Sprintf("/api/users/%d/identities/%d", parentID, int(ids[0]["id"].(float64))), nil, http.StatusConflict)
}

func TestCrossOriginCookieWritesBlocked(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	cookie := sessionCookie(t, env.login(t, kid, "", http.StatusOK))
	host := strings.TrimPrefix(env.server.URL, "http://")

	withOrigin := func(origin string) map[string]string {
		h := cookieHeaders(cookie)
		h["Origin"] = origin
		return h
	}
	// A sibling app on another subdomain can't write with the kid's cookie...
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/theme", kid), map[string]any{"theme": "galaxy"},
		withOrigin("http://evil.home.lan"), http.StatusForbidden)
	// ...but the app itself (same host, any port) can.
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/theme", kid), map[string]any{"theme": "galaxy"},
		withOrigin("http://"+strings.Split(host, ":")[0]+":5173"), http.StatusOK)
	// Reads and non-browser clients are unaffected.
	env.expectStatus(t, "GET", "/api/auth/me", nil, withOrigin("http://evil.home.lan"), http.StatusOK)
	h := sessionHeaders(kid)
	h["Origin"] = "http://evil.home.lan"
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/theme", kid), map[string]any{"theme": "forest"}, h, http.StatusOK)
}
