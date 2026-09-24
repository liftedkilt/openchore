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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/liftedkilt/openchore/internal/store"
)

// Session tokens are stateless, HMAC-signed claims issued by the server after
// a profile proves who it is (PIN, OIDC, or tapping a profile that has no
// credential). They travel as an HttpOnly cookie for the browser and can also
// be sent as "Authorization: Bearer ocs1.…" by other clients.
//
// Revocation is per user: every token carries the user's session_version and
// is rejected once that counter moves (PIN change, unlink, role change, or an
// explicit "sign out everywhere").

const (
	sessionCookieName  = "openchore_session"
	sessionTokenPrefix = "ocs1."

	// Session methods, recorded in the token so the UI and audit log know how
	// the session was established.
	SessionMethodTap    = "tap"    // profile with no credential
	SessionMethodPin    = "pin"    // profile PIN
	SessionMethodOIDC   = "oidc"   // OIDC provider
	SessionMethodUpload = "upload" // single-chore photo upload link (QR code)

	sessionSecretSetting = "session_secret"
)

// Default lifetimes. Kiosk-style logins (tap/PIN) are short because shared
// devices are the norm; OIDC logins happen on personal devices and persist.
const (
	DefaultKioskSessionTTL  = 12 * time.Hour
	DefaultOIDCSessionTTL   = 30 * 24 * time.Hour
	DefaultUploadSessionTTL = 30 * time.Minute
)

// SessionClaims is the signed payload of a session token.
type SessionClaims struct {
	UserID   int64  `json:"uid"`
	Version  int64  `json:"ver"`
	Method   string `json:"m"`
	IssuedAt int64  `json:"iat"`
	Expires  int64  `json:"exp"`
	// Provider is the OIDC provider ID for oidc sessions.
	Provider string `json:"p,omitempty"`
	// ScheduleID restricts an upload session to completing a single chore.
	ScheduleID int64 `json:"sid,omitempty"`
}

// ExpiresAt returns the expiry as a time.Time.
func (c *SessionClaims) ExpiresAt() time.Time { return time.Unix(c.Expires, 0) }

// Persistent reports whether the session should survive the browser closing
// and skip kiosk idle-logout (personal-device OIDC logins).
func (c *SessionClaims) Persistent() bool { return c.Method == SessionMethodOIDC }

// SessionManager signs and verifies session tokens.
type SessionManager struct {
	secret    []byte
	kioskTTL  time.Duration
	oidcTTL   time.Duration
	uploadTTL time.Duration
	now       func() time.Time
}

// NewSessionManager builds a manager around an HMAC secret.
func NewSessionManager(secret []byte) *SessionManager {
	return &SessionManager{
		secret:    secret,
		kioskTTL:  DefaultKioskSessionTTL,
		oidcTTL:   DefaultOIDCSessionTTL,
		uploadTTL: DefaultUploadSessionTTL,
		now:       time.Now,
	}
}

// SetTTLs overrides session lifetimes; zero values keep the defaults.
func (m *SessionManager) SetTTLs(kiosk, oidc time.Duration) {
	if kiosk > 0 {
		m.kioskTTL = kiosk
	}
	if oidc > 0 {
		m.oidcTTL = oidc
	}
}

// LoadSessionSecret returns the HMAC secret: the OPENCHORE_SESSION_SECRET
// value when provided, otherwise one persisted in settings (generated on
// first boot so restarts don't log everyone out).
func LoadSessionSecret(ctx context.Context, s *store.Store, envValue string) ([]byte, error) {
	if envValue != "" {
		if len(envValue) < 32 {
			return nil, errors.New("OPENCHORE_SESSION_SECRET must be at least 32 characters")
		}
		return []byte(envValue), nil
	}
	stored, err := s.GetSetting(ctx, sessionSecretSetting)
	if err != nil {
		return nil, fmt.Errorf("reading session secret: %w", err)
	}
	if stored != "" {
		return hex.DecodeString(stored)
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generating session secret: %w", err)
	}
	if err := s.SetSetting(ctx, sessionSecretSetting, hex.EncodeToString(raw)); err != nil {
		return nil, fmt.Errorf("storing session secret: %w", err)
	}
	return raw, nil
}

// Issue signs a new token for the given claims, filling in timestamps.
func (m *SessionManager) Issue(c SessionClaims) (string, *SessionClaims) {
	now := m.now()
	ttl := m.kioskTTL
	switch c.Method {
	case SessionMethodOIDC:
		ttl = m.oidcTTL
	case SessionMethodUpload:
		ttl = m.uploadTTL
	}
	c.IssuedAt = now.Unix()
	c.Expires = now.Add(ttl).Unix()
	return SignSessionToken(m.secret, c), &c
}

// SignSessionToken signs arbitrary claims. Exported for tests.
func SignSessionToken(secret []byte, c SessionClaims) string {
	payload, _ := json.Marshal(c)
	enc := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(enc))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return sessionTokenPrefix + enc + "." + sig
}

var errInvalidSession = errors.New("invalid session")

// Verify checks the signature and expiry. It does not check session_version;
// the middleware does that against the freshly loaded user.
func (m *SessionManager) Verify(token string) (*SessionClaims, error) {
	if !strings.HasPrefix(token, sessionTokenPrefix) {
		return nil, errInvalidSession
	}
	body := strings.TrimPrefix(token, sessionTokenPrefix)
	enc, sig, ok := strings.Cut(body, ".")
	if !ok {
		return nil, errInvalidSession
	}
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(enc))
	want := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, want) {
		return nil, errInvalidSession
	}
	payload, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		return nil, errInvalidSession
	}
	var c SessionClaims
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, errInvalidSession
	}
	if c.UserID <= 0 || m.now().Unix() >= c.Expires {
		return nil, errInvalidSession
	}
	return &c, nil
}

// isHTTPS reports whether the client reached us over TLS, directly or via a
// reverse proxy.
func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// setSessionCookie writes the session cookie. Persistent sessions get a
// Max-Age; kiosk sessions are browser-session cookies.
func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, c *SessionClaims) {
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	}
	if c.Persistent() {
		cookie.MaxAge = int(time.Until(c.ExpiresAt()).Seconds())
	}
	http.SetCookie(w, cookie)
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// sessionTokenFromRequest returns a session token from the Authorization
// header (ocs1.* bearer) or the session cookie.
func sessionTokenFromRequest(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer "+sessionTokenPrefix) {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if c, err := r.Cookie(sessionCookieName); err == nil {
		return c.Value
	}
	return ""
}
