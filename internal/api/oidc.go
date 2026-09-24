package api

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/liftedkilt/openchore/internal/config"
	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
	"github.com/liftedkilt/openchore/internal/webhook"
)

// OIDC sign-in and account linking.
//
// Profiles are never created from an identity provider: a household member
// signs in to their profile once (PIN or tap), links their provider account
// from their profile settings, and from then on can use "Continue with …"
// after tapping their profile — or skip the picker entirely on their own
// phone, where the session persists.
//
// Flow state (state, nonce, PKCE verifier, mode, profile hint) lives in a
// short-lived HMAC-signed cookie, so no server-side storage is needed.

const (
	oidcFlowCookieName = "openchore_oidc_flow"
	oidcFlowTTL        = 10 * time.Minute

	oidcModeLogin = "login"
	oidcModeLink  = "link"
)

// Error codes appended to /login?auth_error=… (or the return path when
// linking) so the UI can explain what went wrong.
const (
	oidcErrProvider     = "provider_error"
	oidcErrState        = "invalid_state"
	oidcErrExchange     = "exchange_failed"
	oidcErrNotLinked    = "not_linked"
	oidcErrWrongAccount = "wrong_account"
	oidcErrLinkedOther  = "linked_to_other_profile"
	oidcErrSession      = "session_required"
	oidcErrUnavailable  = "provider_unavailable"
)

type oidcProvider struct {
	cfg config.OIDCProviderConfig

	mu       sync.Mutex
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
}

// discover lazily performs OIDC discovery and caches the result, so a
// provider that is down at startup doesn't break the server.
func (p *oidcProvider) discover(ctx context.Context) (*oidc.Provider, *oidc.IDTokenVerifier, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.provider != nil {
		return p.provider, p.verifier, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	prov, err := oidc.NewProvider(ctx, p.cfg.Issuer)
	if err != nil {
		return nil, nil, err
	}
	p.provider = prov
	p.verifier = prov.Verifier(&oidc.Config{ClientID: p.cfg.ClientID})
	return p.provider, p.verifier, nil
}

// OIDCService holds the configured providers and handles the redirect flow.
type OIDCService struct {
	store      *store.Store
	sessions   *SessionManager
	dispatcher *webhook.Dispatcher
	publicURL  string
	providers  map[string]*oidcProvider
	order      []string
	flowKey    []byte
	httpClient *http.Client
}

// NewOIDCService builds the service. It performs no network I/O.
func NewOIDCService(s *store.Store, sm *SessionManager, d *webhook.Dispatcher, auth *config.AuthConfig) *OIDCService {
	svc := &OIDCService{
		store:      s,
		sessions:   sm,
		dispatcher: d,
		providers:  map[string]*oidcProvider{},
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
	mac := hmac.New(sha256.New, sm.secret)
	mac.Write([]byte("openchore-oidc-flow"))
	svc.flowKey = mac.Sum(nil)
	if auth != nil {
		svc.publicURL = auth.PublicURL
		for _, pc := range auth.OIDC {
			svc.providers[pc.ID] = &oidcProvider{cfg: pc}
			svc.order = append(svc.order, pc.ID)
		}
	}
	return svc
}

// SetHTTPClient overrides the client used to talk to providers (tests).
func (o *OIDCService) SetHTTPClient(c *http.Client) { o.httpClient = c }

func (o *OIDCService) ctx(ctx context.Context) context.Context {
	return oidc.ClientContext(ctx, o.httpClient)
}

type providerInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Providers lists the configured providers (public, for the login screen).
func (o *OIDCService) Providers(w http.ResponseWriter, r *http.Request) {
	out := []providerInfo{}
	for _, id := range o.order {
		out = append(out, providerInfo{ID: id, Name: o.providers[id].cfg.Name})
	}
	writeJSON(w, http.StatusOK, out)
}

// oidcFlow is the signed state carried across the provider redirect.
type oidcFlow struct {
	Provider   string `json:"p"`
	State      string `json:"s"`
	Nonce      string `json:"n"`
	Verifier   string `json:"v"`
	Mode       string `json:"m"`
	HintUserID int64  `json:"h,omitempty"`
	LinkUserID int64  `json:"l,omitempty"`
	Return     string `json:"r,omitempty"`
	Expires    int64  `json:"e"`
}

func (o *OIDCService) sealFlow(f oidcFlow) string {
	payload, _ := json.Marshal(f)
	enc := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, o.flowKey)
	mac.Write([]byte(enc))
	return enc + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (o *OIDCService) openFlow(v string) (*oidcFlow, error) {
	enc, sig, ok := strings.Cut(v, ".")
	if !ok {
		return nil, errors.New("malformed flow")
	}
	mac := hmac.New(sha256.New, o.flowKey)
	mac.Write([]byte(enc))
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, mac.Sum(nil)) {
		return nil, errors.New("bad flow signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		return nil, err
	}
	var f oidcFlow
	if err := json.Unmarshal(payload, &f); err != nil {
		return nil, err
	}
	if time.Now().Unix() >= f.Expires {
		return nil, errors.New("flow expired")
	}
	return &f, nil
}

func randomString() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// safeReturn only allows same-origin absolute paths.
func safeReturn(p string) string {
	if p == "" || !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.Contains(p, `\`) {
		return ""
	}
	return p
}

// baseURL returns the externally visible origin used for redirect URIs.
func (o *OIDCService) baseURL(r *http.Request) string {
	if o.publicURL != "" {
		return o.publicURL
	}
	if v, _ := o.store.GetSetting(r.Context(), "base_url"); v != "" {
		return strings.TrimRight(v, "/")
	}
	scheme := "http"
	if isHTTPS(r) {
		scheme = "https"
	}
	host := r.Host
	if fh := r.Header.Get("X-Forwarded-Host"); fh != "" {
		host = strings.TrimSpace(strings.Split(fh, ",")[0])
	}
	return scheme + "://" + host
}

func (o *OIDCService) oauthConfig(r *http.Request, p *oidcProvider, prov *oidc.Provider) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.cfg.ClientID,
		ClientSecret: p.cfg.ClientSecret,
		Endpoint:     prov.Endpoint(),
		RedirectURL:  o.baseURL(r) + "/api/auth/oidc/" + p.cfg.ID + "/callback",
		Scopes:       p.cfg.Scopes,
	}
}

// redirectError sends the browser back to the UI with an error code.
func redirectError(w http.ResponseWriter, r *http.Request, mode, returnPath, provider, code string) {
	target := "/login"
	if mode == oidcModeLink && returnPath != "" {
		target = returnPath
	}
	q := url.Values{"auth_error": {code}, "provider": {provider}}
	sep := "?"
	if strings.Contains(target, "?") {
		sep = "&"
	}
	http.Redirect(w, r, target+sep+q.Encode(), http.StatusFound)
}

// Start begins a login or link flow.
//
//	GET /api/auth/oidc/{provider}/start?mode=login&user_id=7&return=/
//	GET /api/auth/oidc/{provider}/start?mode=link&return=/?view=account
//
// user_id is the profile tapped on the login screen: the callback refuses an
// identity linked to a different profile, and the provider receives a
// login_hint with the linked email. Linking requires an existing session.
func (o *OIDCService) Start(w http.ResponseWriter, r *http.Request) {
	id := urlParam(r, "provider")
	p := o.providers[id]
	if p == nil {
		writeError(w, http.StatusNotFound, "unknown provider")
		return
	}
	q := r.URL.Query()
	mode := q.Get("mode")
	if mode == "" {
		mode = oidcModeLogin
	}
	if mode != oidcModeLogin && mode != oidcModeLink {
		writeError(w, http.StatusBadRequest, "mode must be login or link")
		return
	}
	returnPath := safeReturn(q.Get("return"))

	flow := oidcFlow{
		Provider: id,
		State:    randomString(),
		Nonce:    randomString(),
		Verifier: oauth2.GenerateVerifier(),
		Mode:     mode,
		Return:   returnPath,
		Expires:  time.Now().Add(oidcFlowTTL).Unix(),
	}

	var loginHint string
	switch mode {
	case oidcModeLink:
		user, claims, err := o.currentSession(r)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load session")
			return
		}
		if user == nil || claims.Method == SessionMethodUpload {
			redirectError(w, r, mode, returnPath, id, oidcErrSession)
			return
		}
		flow.LinkUserID = user.ID
	case oidcModeLogin:
		if v := q.Get("user_id"); v != "" {
			uid, err := strconv.ParseInt(v, 10, 64)
			if err != nil || uid <= 0 {
				writeError(w, http.StatusBadRequest, "invalid user_id")
				return
			}
			flow.HintUserID = uid
			if ids, err := o.store.ListIdentitiesForUser(r.Context(), uid); err == nil {
				for _, ident := range ids {
					if ident.Provider == id && ident.Email != "" {
						loginHint = ident.Email
						break
					}
				}
			}
		}
	}

	prov, _, err := p.discover(o.ctx(r.Context()))
	if err != nil {
		log.Printf("oidc: discovery for %q failed: %v", id, err)
		redirectError(w, r, mode, returnPath, id, oidcErrUnavailable)
		return
	}

	opts := []oauth2.AuthCodeOption{
		oidc.Nonce(flow.Nonce),
		oauth2.S256ChallengeOption(flow.Verifier),
	}
	if p.cfg.Prompt != "" {
		opts = append(opts, oauth2.SetAuthURLParam("prompt", p.cfg.Prompt))
	}
	if loginHint != "" {
		opts = append(opts, oauth2.SetAuthURLParam("login_hint", loginHint))
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oidcFlowCookieName,
		Value:    o.sealFlow(flow),
		Path:     "/api/auth/oidc/",
		MaxAge:   int(oidcFlowTTL.Seconds()),
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, o.oauthConfig(r, p, prov).AuthCodeURL(flow.State, opts...), http.StatusFound)
}

func (o *OIDCService) currentSession(r *http.Request) (*model.User, *SessionClaims, error) {
	token := sessionTokenFromRequest(r)
	if token == "" {
		return nil, nil, nil
	}
	return loadSessionUser(r.Context(), o.store, o.sessions, token)
}

type idClaims struct {
	Email             string `json:"email"`
	EmailVerified     *bool  `json:"email_verified"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
}

// Callback completes the flow started by Start.
func (o *OIDCService) Callback(w http.ResponseWriter, r *http.Request) {
	id := urlParam(r, "provider")
	p := o.providers[id]
	if p == nil {
		writeError(w, http.StatusNotFound, "unknown provider")
		return
	}

	// The flow cookie is single-use.
	http.SetCookie(w, &http.Cookie{
		Name: oidcFlowCookieName, Value: "", Path: "/api/auth/oidc/", MaxAge: -1,
		HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode,
	})
	c, err := r.Cookie(oidcFlowCookieName)
	if err != nil {
		redirectError(w, r, oidcModeLogin, "", id, oidcErrState)
		return
	}
	flow, err := o.openFlow(c.Value)
	if err != nil || flow.Provider != id {
		redirectError(w, r, oidcModeLogin, "", id, oidcErrState)
		return
	}
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		log.Printf("oidc: provider %q returned error %q: %s", id, e, q.Get("error_description"))
		redirectError(w, r, flow.Mode, flow.Return, id, oidcErrProvider)
		return
	}
	if !subtleEq(q.Get("state"), flow.State) {
		redirectError(w, r, flow.Mode, flow.Return, id, oidcErrState)
		return
	}

	ctx := o.ctx(r.Context())
	prov, verifier, err := p.discover(ctx)
	if err != nil {
		redirectError(w, r, flow.Mode, flow.Return, id, oidcErrUnavailable)
		return
	}
	tok, err := o.oauthConfig(r, p, prov).Exchange(ctx, q.Get("code"), oauth2.VerifierOption(flow.Verifier))
	if err != nil {
		log.Printf("oidc: code exchange with %q failed: %v", id, err)
		redirectError(w, r, flow.Mode, flow.Return, id, oidcErrExchange)
		return
	}
	rawID, _ := tok.Extra("id_token").(string)
	if rawID == "" {
		log.Printf("oidc: provider %q returned no id_token", id)
		redirectError(w, r, flow.Mode, flow.Return, id, oidcErrExchange)
		return
	}
	idToken, err := verifier.Verify(ctx, rawID)
	if err != nil || !subtleEq(idToken.Nonce, flow.Nonce) {
		log.Printf("oidc: id_token from %q failed verification: %v", id, err)
		redirectError(w, r, flow.Mode, flow.Return, id, oidcErrExchange)
		return
	}
	var claims idClaims
	_ = idToken.Claims(&claims)
	displayName := claims.Name
	if displayName == "" {
		displayName = claims.PreferredUsername
	}

	existing, err := o.store.GetIdentity(r.Context(), id, idToken.Subject)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to look up identity")
		return
	}

	if flow.Mode == oidcModeLink {
		o.finishLink(w, r, flow, idToken.Subject, claims.Email, displayName, existing)
		return
	}

	if existing == nil {
		log.Printf("oidc: %q subject has no linked profile", id)
		redirectError(w, r, flow.Mode, flow.Return, id, oidcErrNotLinked)
		return
	}
	if flow.HintUserID != 0 && flow.HintUserID != existing.UserID {
		log.Printf("oidc: %q identity belongs to user %d, not tapped profile %d", id, existing.UserID, flow.HintUserID)
		redirectError(w, r, flow.Mode, flow.Return, id, oidcErrWrongAccount)
		return
	}
	user, err := o.store.GetUser(r.Context(), existing.UserID)
	if err != nil || user == nil {
		redirectError(w, r, flow.Mode, flow.Return, id, oidcErrNotLinked)
		return
	}
	_ = o.store.TouchIdentityLogin(r.Context(), existing.ID, claims.Email, displayName)

	token, issued := o.sessions.Issue(SessionClaims{
		UserID: user.ID, Version: user.SessionVersion, Method: SessionMethodOIDC, Provider: id,
	})
	setSessionCookie(w, r, token, issued)
	log.Printf("auth: user %d (%s) signed in with %q from %s", user.ID, user.Name, id, clientIP(r))
	if o.dispatcher != nil {
		o.dispatcher.Fire(webhook.EventOIDCLogin, map[string]any{
			"user_id": user.ID, "user_name": user.Name, "provider": id, "ip_address": clientIP(r),
		})
	}
	dest := flow.Return
	if dest == "" {
		dest = "/"
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

func (o *OIDCService) finishLink(w http.ResponseWriter, r *http.Request, flow *oidcFlow, subject, email, displayName string, existing *model.UserIdentity) {
	// The browser must still be signed in as the profile that started linking.
	user, claims, err := o.currentSession(r)
	if err != nil || user == nil || claims.Method == SessionMethodUpload || user.ID != flow.LinkUserID {
		redirectError(w, r, flow.Mode, flow.Return, flow.Provider, oidcErrSession)
		return
	}
	if existing != nil && existing.UserID != user.ID {
		redirectError(w, r, flow.Mode, flow.Return, flow.Provider, oidcErrLinkedOther)
		return
	}
	if existing == nil {
		ident := &model.UserIdentity{
			UserID: user.ID, Provider: flow.Provider, Subject: subject, Email: email, DisplayName: displayName,
		}
		if err := o.store.LinkIdentity(r.Context(), ident); err != nil {
			log.Printf("oidc: linking %q to user %d failed: %v", flow.Provider, user.ID, err)
			redirectError(w, r, flow.Mode, flow.Return, flow.Provider, oidcErrLinkedOther)
			return
		}
		log.Printf("auth: user %d (%s) linked %q", user.ID, user.Name, flow.Provider)
		if o.dispatcher != nil {
			o.dispatcher.Fire(webhook.EventIdentityLinked, map[string]any{
				"user_id": user.ID, "user_name": user.Name, "provider": flow.Provider, "ip_address": clientIP(r),
			})
		}
	}
	dest := flow.Return
	if dest == "" {
		dest = "/"
	}
	sep := "?"
	if strings.Contains(dest, "?") {
		sep = "&"
	}
	http.Redirect(w, r, dest+sep+"linked="+url.QueryEscape(flow.Provider), http.StatusFound)
}

func subtleEq(a, b string) bool {
	return a != "" && hmac.Equal([]byte(a), []byte(b))
}

// --- Linked identity management (authenticated) ---

// ListIdentities returns the identities linked to a profile (self or admin).
func (o *OIDCService) ListIdentities(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	caller := UserFromContext(r.Context())
	if caller.Role != model.RoleAdmin && caller.ID != id {
		writeError(w, http.StatusForbidden, "can only view your own linked accounts")
		return
	}
	ids, err := o.store.ListIdentitiesForUser(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list linked accounts")
		return
	}
	writeJSON(w, http.StatusOK, ids)
}

// UnlinkIdentity removes a linked identity (self or admin). Refuses to leave
// an admin profile with no way to sign in. Unlinking revokes the target's
// sessions, so a lost phone signed in via that provider is signed out.
func (o *OIDCService) UnlinkIdentity(w http.ResponseWriter, r *http.Request) {
	userID, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	identityID, err := urlParamInt64(r, "identityID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid identity id")
		return
	}
	caller := UserFromContext(r.Context())
	if caller.Role != model.RoleAdmin && caller.ID != userID {
		writeError(w, http.StatusForbidden, "can only unlink your own accounts")
		return
	}
	target, err := o.store.GetUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if target == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if target.Role == model.RoleAdmin && !target.HasPin && len(target.AuthProviders) <= 1 {
		writeError(w, http.StatusConflict, "admin profiles need a pin or linked account; set a pin first")
		return
	}
	ok, err := o.store.UnlinkIdentity(r.Context(), userID, identityID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unlink account")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "linked account not found")
		return
	}
	if err := o.store.BumpSessionVersion(r.Context(), userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke sessions")
		return
	}
	// Keep the caller signed in when they unlinked their own account.
	if caller.ID == userID {
		if claims := SessionClaimsFromContext(r.Context()); claims != nil {
			fresh := *claims
			fresh.Version = target.SessionVersion + 1
			if fresh.Method == SessionMethodOIDC {
				// The session came from a provider; downgrade to a kiosk
				// session rather than keeping a persistent one alive.
				fresh.Method = SessionMethodPin
				fresh.Provider = ""
			}
			token, issued := o.sessions.Issue(fresh)
			setSessionCookie(w, r, token, issued)
		}
	}
	log.Printf("auth: identity %d unlinked from user %d by user %d", identityID, userID, caller.ID)
	if o.dispatcher != nil {
		o.dispatcher.Fire(webhook.EventIdentityUnlinked, map[string]any{
			"user_id": userID, "user_name": target.Name, "actor_id": caller.ID, "actor_name": caller.Name,
		})
	}
	w.WriteHeader(http.StatusNoContent)
}
