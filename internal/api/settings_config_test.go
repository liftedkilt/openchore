package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// --- AI connection settings ---

func (e *testEnv) aiConfig(t *testing.T) map[string]map[string]any {
	t.Helper()
	resp := e.expectStatus(t, "GET", "/api/admin/ai/config", nil, adminHeaders(), http.StatusOK)
	var cfg map[string]map[string]any
	decodeBody(t, resp, &cfg)
	return cfg
}

func (e *testEnv) aiStatus(t *testing.T) map[string]map[string]any {
	t.Helper()
	resp := e.expectStatus(t, "GET", "/api/admin/ai/status", nil, adminHeaders(), http.StatusOK)
	var st map[string]map[string]any
	decodeBody(t, resp, &st)
	return st
}

func TestAIConfigFromUI(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("AI_BASE_URL", "")
	t.Setenv("TTS_BASE_URL", "")
	env := setupTest(t)
	env.createAdmin(t)
	f := newFakeAI(t)

	if st := env.aiStatus(t); st["ai"]["configured"] != false || st["tts"]["configured"] != false {
		t.Fatalf("expected nothing configured, got %v", st)
	}
	env.expectStatus(t, "POST", "/api/admin/ai/generate-description", map[string]any{"title": "Dishes"}, adminHeaders(), http.StatusServiceUnavailable)

	// Only admins can change it.
	kid := env.createChild(t, "Kid")
	env.expectStatus(t, "PUT", "/api/admin/ai/config", map[string]any{"ai": map[string]any{"base_url": f.server.URL + "/v1", "model": "m"}}, childHeaders(kid), http.StatusForbidden)

	// A model is required, and URLs must be http(s).
	env.expectStatus(t, "PUT", "/api/admin/ai/config", map[string]any{"ai": map[string]any{"base_url": f.server.URL + "/v1"}}, adminHeaders(), http.StatusBadRequest)
	env.expectStatus(t, "PUT", "/api/admin/ai/config", map[string]any{"ai": map[string]any{"base_url": "ftp://x", "model": "m"}}, adminHeaders(), http.StatusBadRequest)

	env.expectStatus(t, "PUT", "/api/admin/ai/config", map[string]any{
		"ai":  map[string]any{"base_url": f.server.URL + "/v1", "model": "ui-model", "api_key": "sk-secret"},
		"tts": map[string]any{"base_url": f.server.URL + "/v1", "model": ""},
	}, adminHeaders(), http.StatusOK)

	// Takes effect without a restart.
	st := env.aiStatus(t)
	if st["ai"]["configured"] != true || st["ai"]["model"] != "ui-model" || st["tts"]["configured"] != true {
		t.Fatalf("expected AI and TTS configured, got %v", st)
	}
	env.expectStatus(t, "POST", "/api/admin/ai/generate-description", map[string]any{"title": "Dishes"}, adminHeaders(), http.StatusOK)

	// The key is write-only.
	cfg := env.aiConfig(t)
	if cfg["ai"]["api_key_set"] != true || cfg["ai"]["from_env"] != false || cfg["ai"]["base_url"] != f.server.URL+"/v1" {
		t.Fatalf("unexpected config: %v", cfg)
	}
	if _, ok := cfg["ai"]["api_key"]; ok {
		t.Fatal("api key must not be returned")
	}
	env.expectStatus(t, "GET", "/api/admin/settings/ai_api_key", nil, adminHeaders(), http.StatusForbidden)
	env.expectStatus(t, "PUT", "/api/admin/settings/ai_api_key", map[string]any{"value": "x"}, adminHeaders(), http.StatusBadRequest)
	resp := env.expectStatus(t, "GET", "/api/admin/export-config", nil, adminHeaders(), http.StatusOK)
	var export strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		export.Write(buf[:n])
		if err != nil {
			break
		}
	}
	resp.Body.Close()
	if strings.Contains(export.String(), "sk-secret") {
		t.Fatal("config export leaks the AI API key")
	}

	// Omitting the key keeps it; "" clears it.
	env.expectStatus(t, "PUT", "/api/admin/ai/config", map[string]any{"ai": map[string]any{"base_url": f.server.URL + "/v1", "model": "other"}}, adminHeaders(), http.StatusOK)
	if cfg := env.aiConfig(t); cfg["ai"]["api_key_set"] != true || cfg["ai"]["model"] != "other" {
		t.Fatalf("expected key kept and model updated, got %v", cfg)
	}
	env.expectStatus(t, "PUT", "/api/admin/ai/config", map[string]any{"ai": map[string]any{"base_url": f.server.URL + "/v1", "model": "other", "api_key": ""}}, adminHeaders(), http.StatusOK)
	if cfg := env.aiConfig(t); cfg["ai"]["api_key_set"] != false {
		t.Fatalf("expected key cleared, got %v", cfg)
	}

	// A chore created now gets read-aloud audio from the new service.
	env.createScheduledChore(t, "Feed the cat", kid, nil)
	deadline := time.Now().Add(5 * time.Second)
	for !containsString(f.spoken(), "Feed the cat") {
		if time.Now().After(deadline) {
			t.Fatalf("expected audio to be recorded, spoken: %v", f.spoken())
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Clearing the base URL turns AI off again.
	env.expectStatus(t, "PUT", "/api/admin/ai/config", map[string]any{"ai": map[string]any{"base_url": ""}}, adminHeaders(), http.StatusOK)
	if st := env.aiStatus(t); st["ai"]["configured"] != false || st["tts"]["configured"] != true {
		t.Fatalf("expected AI off and TTS still on, got %v", st)
	}
}

func TestAIConfigEnvironmentWins(t *testing.T) {
	t.Chdir(t.TempDir())
	env := setupTest(t)
	env.createAdmin(t)
	t.Setenv("AI_BASE_URL", "http://env-ai:8080/v1")
	t.Setenv("AI_MODEL", "env-model")
	t.Setenv("TTS_BASE_URL", "")

	cfg := env.aiConfig(t)
	if cfg["ai"]["from_env"] != true || cfg["ai"]["model"] != "env-model" || cfg["tts"]["from_env"] != false {
		t.Fatalf("unexpected config: %v", cfg)
	}
	env.expectStatus(t, "PUT", "/api/admin/ai/config", map[string]any{"ai": map[string]any{"base_url": "http://other/v1", "model": "m"}}, adminHeaders(), http.StatusConflict)
	// The unlocked section can still be changed.
	env.expectStatus(t, "PUT", "/api/admin/ai/config", map[string]any{"tts": map[string]any{"base_url": "http://kokoro:8880/v1"}}, adminHeaders(), http.StatusOK)
	if cfg := env.aiConfig(t); cfg["tts"]["base_url"] != "http://kokoro:8880/v1" || cfg["ai"]["model"] != "env-model" {
		t.Fatalf("unexpected config after TTS update: %v", cfg)
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if strings.Contains(v, s) {
			return true
		}
	}
	return false
}

// --- Sign-in settings ---

type authConfigResp struct {
	Providers []struct {
		ID              string `json:"id"`
		Name            string `json:"name"`
		Issuer          string `json:"issuer"`
		ClientSecretSet bool   `json:"client_secret_set"`
		Scopes          string `json:"scopes"`
		Prompt          string `json:"prompt"`
		Source          string `json:"source"`
		LinkedAccounts  int    `json:"linked_accounts"`
		RedirectURI     string `json:"redirect_uri"`
	} `json:"providers"`
	KioskSession struct {
		Hours      float64 `json:"hours"`
		FromConfig bool    `json:"from_config"`
	} `json:"kiosk_session"`
	PersonalSession struct {
		Hours float64 `json:"hours"`
	} `json:"personal_session"`
}

func (e *testEnv) authConfig(t *testing.T) authConfigResp {
	t.Helper()
	resp := e.expectStatus(t, "GET", "/api/admin/auth/config", nil, adminHeaders(), http.StatusOK)
	var cfg authConfigResp
	decodeBody(t, resp, &cfg)
	return cfg
}

func TestOIDCProviderAddedFromUI(t *testing.T) {
	env := setupTest(t)
	idp := newFakeIdP(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")

	provider := map[string]any{
		"id": "pocket", "name": "Pocket ID", "issuer": idp.srv.URL + "/", "client_id": idp.client,
		"client_secret": "s3cret", "prompt": "login",
	}
	env.expectStatus(t, "POST", "/api/admin/auth/providers", provider, childHeaders(kid), http.StatusForbidden)
	for _, bad := range []map[string]any{
		{"id": "Bad ID", "issuer": idp.srv.URL, "client_id": "c"},
		{"id": "ok", "issuer": "not a url", "client_id": "c"},
		{"id": "ok", "issuer": idp.srv.URL, "client_id": ""},
		{"id": "ok", "issuer": idp.srv.URL, "client_id": "c", "prompt": "weird"},
	} {
		env.expectStatus(t, "POST", "/api/admin/auth/providers", bad, adminHeaders(), http.StatusBadRequest)
	}
	env.expectStatus(t, "POST", "/api/admin/auth/providers", provider, adminHeaders(), http.StatusCreated)
	env.expectStatus(t, "POST", "/api/admin/auth/providers", provider, adminHeaders(), http.StatusConflict)

	cfg := env.authConfig(t)
	if len(cfg.Providers) != 1 {
		t.Fatalf("expected one provider, got %+v", cfg.Providers)
	}
	p := cfg.Providers[0]
	if p.Source != "settings" || !p.ClientSecretSet || p.Issuer != idp.srv.URL || p.Scopes != "openid profile email" ||
		!strings.HasSuffix(p.RedirectURI, "/api/auth/oidc/pocket/callback") {
		t.Fatalf("unexpected provider: %+v", p)
	}

	// Public listing picks it up without a restart.
	resp := env.expectStatus(t, "GET", "/api/auth/providers", nil, nil, http.StatusOK)
	var listed []map[string]any
	decodeBody(t, resp, &listed)
	if len(listed) != 1 || listed[0]["name"] != "Pocket ID" {
		t.Fatalf("unexpected public providers: %v", listed)
	}
	env.expectStatus(t, "POST", "/api/admin/auth/providers/pocket/test", nil, adminHeaders(), http.StatusOK)

	// Link and sign in through it.
	b := newBrowser(t, env)
	b.expect("POST", "/api/auth/login", map[string]any{"user_id": kid}, http.StatusOK)
	if loc := b.get("/api/auth/oidc/pocket/start?mode=link&return=/account"); loc != "/account?linked=pocket" {
		t.Fatalf("unexpected link redirect: %s", loc)
	}
	if idp.lastAuth.Get("prompt") != "login" {
		t.Fatalf("expected prompt=login, got %q", idp.lastAuth.Get("prompt"))
	}
	phone := newBrowser(t, env)
	if loc := phone.get(fmt.Sprintf("/api/auth/oidc/pocket/start?user_id=%d", kid)); loc != "/" {
		t.Fatalf("expected login redirect to /, got %s", loc)
	}
	if cfg := env.authConfig(t); cfg.Providers[0].LinkedAccounts != 1 {
		t.Fatalf("expected 1 linked account, got %+v", cfg.Providers[0])
	}

	// Editing without a secret keeps it.
	env.expectStatus(t, "PUT", "/api/admin/auth/providers/pocket", map[string]any{
		"name": "Family ID", "issuer": idp.srv.URL, "client_id": idp.client, "scopes": "profile,email",
	}, adminHeaders(), http.StatusOK)
	p = env.authConfig(t).Providers[0]
	if p.Name != "Family ID" || !p.ClientSecretSet || p.Prompt != "" || p.Scopes != "openid profile email" {
		t.Fatalf("unexpected provider after edit: %+v", p)
	}
	env.expectStatus(t, "PUT", "/api/admin/auth/providers/nope", map[string]any{
		"issuer": idp.srv.URL, "client_id": "c",
	}, adminHeaders(), http.StatusNotFound)

	// The kid has no PIN, so removing their only way in is refused.
	resp = env.expectStatus(t, "DELETE", "/api/admin/auth/providers/pocket", nil, adminHeaders(), http.StatusConflict)
	var errBody map[string]string
	decodeBody(t, resp, &errBody)
	if !strings.Contains(errBody["error"], "Kid") {
		t.Fatalf("expected the stranded profile to be named, got %q", errBody["error"])
	}
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/pin", kid), map[string]any{"new_pin": "4321"}, adminHeaders(), http.StatusOK)
	env.expectStatus(t, "DELETE", "/api/admin/auth/providers/pocket", nil, adminHeaders(), http.StatusOK)

	resp = env.expectStatus(t, "GET", "/api/auth/providers", nil, nil, http.StatusOK)
	decodeBody(t, resp, &listed)
	if len(listed) != 0 {
		t.Fatalf("expected no providers after delete, got %v", listed)
	}
	env.expectStatus(t, "GET", "/api/auth/oidc/pocket/start", nil, nil, http.StatusNotFound)
}

func TestOIDCConfigProvidersAreReadOnly(t *testing.T) {
	env, idp := setupTestWithOIDC(t, "")
	env.createAdmin(t)

	cfg := env.authConfig(t)
	if len(cfg.Providers) != 1 || cfg.Providers[0].Source != "config" {
		t.Fatalf("expected the config provider, got %+v", cfg.Providers)
	}
	body := map[string]any{"id": "pocket", "issuer": idp.srv.URL, "client_id": "c"}
	env.expectStatus(t, "POST", "/api/admin/auth/providers", body, adminHeaders(), http.StatusConflict)
	env.expectStatus(t, "PUT", "/api/admin/auth/providers/pocket", body, adminHeaders(), http.StatusConflict)
	env.expectStatus(t, "DELETE", "/api/admin/auth/providers/pocket", nil, adminHeaders(), http.StatusConflict)

	// UI providers are listed after config ones.
	env.expectStatus(t, "POST", "/api/admin/auth/providers", map[string]any{
		"id": "google", "name": "Google", "issuer": "https://accounts.google.com", "client_id": "g",
	}, adminHeaders(), http.StatusCreated)
	resp := env.expectStatus(t, "GET", "/api/auth/providers", nil, nil, http.StatusOK)
	var listed []map[string]any
	decodeBody(t, resp, &listed)
	if len(listed) != 2 || listed[0]["id"] != "pocket" || listed[1]["id"] != "google" {
		t.Fatalf("unexpected providers: %v", listed)
	}
}

func TestSessionLengthsFromUI(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")

	cfg := env.authConfig(t)
	if cfg.KioskSession.Hours != 12 || cfg.PersonalSession.Hours != 720 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	env.expectStatus(t, "PUT", "/api/admin/auth/sessions", map[string]any{"kiosk_session_hours": 0.01}, adminHeaders(), http.StatusBadRequest)
	env.expectStatus(t, "PUT", "/api/admin/auth/sessions", map[string]any{"kiosk_session_hours": 2, "personal_session_hours": 168}, adminHeaders(), http.StatusOK)
	cfg = env.authConfig(t)
	if cfg.KioskSession.Hours != 2 || cfg.PersonalSession.Hours != 168 {
		t.Fatalf("unexpected lengths: %+v", cfg)
	}

	// New sessions use it straight away.
	resp := env.login(t, kid, "", http.StatusOK)
	var body map[string]any
	decodeBody(t, resp, &body)
	expires, err := time.Parse(time.RFC3339, body["session"].(map[string]any)["expires_at"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Until(expires); d > 2*time.Hour+time.Minute || d < 2*time.Hour-time.Minute {
		t.Fatalf("expected a 2h session, got %s", d)
	}

	// null restores the default.
	env.expectStatus(t, "PUT", "/api/admin/auth/sessions", map[string]any{"kiosk_session_hours": nil}, adminHeaders(), http.StatusOK)
	if cfg := env.authConfig(t); cfg.KioskSession.Hours != 12 {
		t.Fatalf("expected default restored, got %+v", cfg.KioskSession)
	}
}
