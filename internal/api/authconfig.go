package api

import (
	"context"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/liftedkilt/openchore/internal/config"
	"github.com/liftedkilt/openchore/internal/model"
)

// Sign-in settings editable under Manage → Settings → Sign-in. Anything set
// in config.yaml (auth:) or the OIDC_* environment variables takes
// precedence and is shown read-only.

const (
	settingKioskSessionTTL    = "kiosk_session_ttl"
	settingPersonalSessionTTL = "personal_session_ttl"

	minSessionTTL = 5 * time.Minute
	maxSessionTTL = 365 * 24 * time.Hour
)

// oidcPrompts are the prompt values offered in the admin UI.
var oidcPrompts = map[string]bool{"": true, "login": true, "consent": true, "select_account": true}

type sessionTTL struct {
	value      time.Duration // 0 = default
	fromConfig bool
}

// sessionTTLs resolves the session lifetimes: config.yaml, then settings.
func (o *OIDCService) sessionTTLs(ctx context.Context) (kiosk, personal sessionTTL) {
	resolve := func(static, key string) sessionTTL {
		if static != "" {
			d, _ := time.ParseDuration(static)
			return sessionTTL{value: d, fromConfig: true}
		}
		v, _ := o.store.GetSetting(ctx, key)
		d, err := time.ParseDuration(v)
		if err != nil || d < minSessionTTL || d > maxSessionTTL {
			return sessionTTL{}
		}
		return sessionTTL{value: d}
	}
	return resolve(o.staticTTL.kiosk, settingKioskSessionTTL), resolve(o.staticTTL.personal, settingPersonalSessionTTL)
}

type sessionTTLView struct {
	Hours        float64 `json:"hours"`
	DefaultHours float64 `json:"default_hours"`
	FromConfig   bool    `json:"from_config"`
}

func (t sessionTTL) view(def time.Duration) sessionTTLView {
	d := t.value
	if d <= 0 {
		d = def
	}
	return sessionTTLView{Hours: d.Hours(), DefaultHours: def.Hours(), FromConfig: t.fromConfig}
}

type adminProviderView struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Issuer          string `json:"issuer"`
	ClientID        string `json:"client_id"`
	ClientSecretSet bool   `json:"client_secret_set"`
	Scopes          string `json:"scopes"`
	Prompt          string `json:"prompt"`
	// Source is "config" (config.yaml / environment, read-only) or
	// "settings" (added here).
	Source         string `json:"source"`
	LinkedAccounts int    `json:"linked_accounts"`
	RedirectURI    string `json:"redirect_uri"`
}

// AdminConfig returns everything the Sign-in settings card shows.
func (o *OIDCService) AdminConfig(w http.ResponseWriter, r *http.Request) {
	o.writeAdminConfig(w, r, http.StatusOK)
}

func (o *OIDCService) writeAdminConfig(w http.ResponseWriter, r *http.Request, status int) {
	ctx := r.Context()
	linked, err := o.store.CountIdentitiesByProvider(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count linked accounts")
		return
	}
	base := o.baseURL(r)

	o.mu.RLock()
	providers := make([]adminProviderView, 0, len(o.order))
	for _, id := range o.order {
		p := o.providers[id].cfg
		source := "config"
		if o.fromDB[id] {
			source = "settings"
		}
		providers = append(providers, adminProviderView{
			ID:              p.ID,
			Name:            p.Name,
			Issuer:          p.Issuer,
			ClientID:        p.ClientID,
			ClientSecretSet: p.ClientSecret != "",
			Scopes:          strings.Join(p.Scopes, " "),
			Prompt:          p.Prompt,
			Source:          source,
			LinkedAccounts:  linked[id],
			RedirectURI:     base + "/api/auth/oidc/" + p.ID + "/callback",
		})
	}
	o.mu.RUnlock()

	kiosk, personal := o.sessionTTLs(ctx)
	writeJSON(w, status, map[string]any{
		"providers":              providers,
		"callback_base":          base + "/api/auth/oidc/",
		"public_url_from_config": o.publicURL != "",
		"kiosk_session":          kiosk.view(DefaultKioskSessionTTL),
		"personal_session":       personal.view(DefaultOIDCSessionTTL),
	})
}

// UpdateSessionTTLs sets how long tap/PIN and linked-account sessions last.
// A null or 0 value restores the default.
func (o *OIDCService) UpdateSessionTTLs(w http.ResponseWriter, r *http.Request) {
	var req struct {
		KioskHours    *float64 `json:"kiosk_session_hours"`
		PersonalHours *float64 `json:"personal_session_hours"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx := r.Context()
	for _, f := range []struct {
		hours      *float64
		key, label string
		fromConfig bool
	}{
		{req.KioskHours, settingKioskSessionTTL, "tap and PIN session length", o.staticTTL.kiosk != ""},
		{req.PersonalHours, settingPersonalSessionTTL, "linked-account session length", o.staticTTL.personal != ""},
	} {
		if f.fromConfig {
			continue
		}
		value := ""
		if f.hours != nil && *f.hours != 0 {
			h := *f.hours
			d := time.Duration(math.Round(h * float64(time.Hour)))
			if math.IsNaN(h) || d < minSessionTTL || d > maxSessionTTL {
				writeError(w, http.StatusBadRequest, f.label+" must be between 5 minutes and 365 days")
				return
			}
			value = d.String()
		}
		if err := o.store.SetSetting(ctx, f.key, value); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save setting")
			return
		}
	}
	if err := o.Reload(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to apply sign-in settings")
		return
	}
	o.AdminConfig(w, r)
}

// providerRequest is the body of POST and PUT /api/admin/auth/providers.
// On update, a nil ClientSecret keeps the stored one.
type providerRequest struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Issuer       string  `json:"issuer"`
	ClientID     string  `json:"client_id"`
	ClientSecret *string `json:"client_secret"`
	Scopes       string  `json:"scopes"`
	Prompt       string  `json:"prompt"`
}

// validate trims the request and returns a user-facing problem, if any.
func (req *providerRequest) validate() string {
	req.ID = strings.TrimSpace(req.ID)
	req.Name = strings.TrimSpace(req.Name)
	req.Issuer = strings.TrimRight(strings.TrimSpace(req.Issuer), "/")
	req.ClientID = strings.TrimSpace(req.ClientID)
	req.Scopes = strings.Join(config.ParseScopes(req.Scopes), " ")
	req.Prompt = strings.TrimSpace(req.Prompt)
	switch {
	case !config.ValidProviderID(req.ID):
		return "id must be 1-32 lowercase letters, digits, '-' or '_', starting with a letter or digit"
	case !isHTTPURL(req.Issuer):
		return "issuer must be an http(s) URL"
	case req.ClientID == "":
		return "client_id is required"
	case !oidcPrompts[req.Prompt]:
		return "prompt must be empty, login, consent or select_account"
	}
	return ""
}

func (o *OIDCService) isStatic(id string) bool {
	for _, p := range o.static {
		if p.ID == id {
			return true
		}
	}
	return false
}

// CreateProvider adds a sign-in provider.
func (o *OIDCService) CreateProvider(w http.ResponseWriter, r *http.Request) {
	var req providerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := req.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	ctx := r.Context()
	if o.isStatic(req.ID) {
		writeError(w, http.StatusConflict, "a provider with this id is set in config.yaml or the environment")
		return
	}
	if existing, err := o.store.GetOIDCProvider(ctx, req.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check provider")
		return
	} else if existing != nil {
		writeError(w, http.StatusConflict, "a provider with this id already exists")
		return
	}
	p := &model.OIDCProvider{
		ID: req.ID, Name: req.Name, Issuer: req.Issuer, ClientID: req.ClientID,
		Scopes: req.Scopes, Prompt: req.Prompt,
	}
	if req.ClientSecret != nil {
		p.ClientSecret = strings.TrimSpace(*req.ClientSecret)
	}
	if err := o.store.CreateOIDCProvider(ctx, p); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save provider")
		return
	}
	o.afterProviderChange(w, r, http.StatusCreated)
}

// UpdateProvider edits a provider added from the admin UI. Its id can't
// change: it is stored with linked accounts and is part of the callback URL.
func (o *OIDCService) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	id := urlParam(r, "id")
	var req providerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.ID = id
	if msg := req.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	ctx := r.Context()
	if o.isStatic(id) {
		writeError(w, http.StatusConflict, "this provider is set in config.yaml or the environment; change it there")
		return
	}
	p, err := o.store.GetOIDCProvider(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get provider")
		return
	}
	if p == nil {
		writeError(w, http.StatusNotFound, "provider not found")
		return
	}
	p.Name, p.Issuer, p.ClientID, p.Scopes, p.Prompt = req.Name, req.Issuer, req.ClientID, req.Scopes, req.Prompt
	if req.ClientSecret != nil {
		p.ClientSecret = strings.TrimSpace(*req.ClientSecret)
	}
	if err := o.store.UpdateOIDCProvider(ctx, p); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save provider")
		return
	}
	o.afterProviderChange(w, r, http.StatusOK)
}

// DeleteProvider removes a provider added from the admin UI. Linked accounts
// are kept (re-adding the same id restores them), but it refuses when that
// would leave someone with no way to sign in.
func (o *OIDCService) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	id := urlParam(r, "id")
	ctx := r.Context()
	if o.isStatic(id) {
		writeError(w, http.StatusConflict, "this provider is set in config.yaml or the environment; remove it there")
		return
	}
	stranded, err := o.store.UsersSignedInOnlyWith(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check linked accounts")
		return
	}
	if len(stranded) > 0 {
		writeError(w, http.StatusConflict, "can't remove: "+strings.Join(stranded, ", ")+
			" can only sign in with this provider. Give them a PIN first.")
		return
	}
	ok, err := o.store.DeleteOIDCProvider(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete provider")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "provider not found")
		return
	}
	o.afterProviderChange(w, r, http.StatusOK)
}

func (o *OIDCService) afterProviderChange(w http.ResponseWriter, r *http.Request, status int) {
	if err := o.Reload(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to apply sign-in settings")
		return
	}
	o.writeAdminConfig(w, r, status)
}

// TestProvider runs OIDC discovery against a provider's issuer so a parent
// can check the settings before anyone tries to sign in.
func (o *OIDCService) TestProvider(w http.ResponseWriter, r *http.Request) {
	p := o.provider(urlParam(r, "id"))
	if p == nil {
		writeError(w, http.StatusNotFound, "provider not found")
		return
	}
	if _, _, err := p.discover(o.ctx(r.Context())); err != nil {
		writeError(w, http.StatusBadGateway, "couldn't reach the issuer: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
