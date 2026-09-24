package api

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
	"github.com/liftedkilt/openchore/internal/webhook"
)

// legacyPasscodeSetting is the pre-1.0 household-wide admin passcode. It is
// only consulted to let an existing admin profile claim a PIN the first time
// it signs in after upgrading, and is deleted once every admin has a
// credential of their own.
const legacyPasscodeSetting = "admin_passcode"

// Login error codes returned alongside "error" so the UI can pick the right
// next step without string-matching messages.
const (
	authCodeIncorrectPin       = "incorrect_pin"
	authCodeLockedOut          = "locked_out"
	authCodeOIDCRequired       = "oidc_required"
	authCodeAdminSetupRequired = "admin_setup_required"
	authCodeIncorrectPasscode  = "incorrect_passcode"
)

func writeAuthError(w http.ResponseWriter, status int, code, msg string, extra map[string]any) {
	body := map[string]any{"error": msg, "code": code}
	for k, v := range extra {
		body[k] = v
	}
	writeJSON(w, status, body)
}

// loginLimiter slows down PIN guessing. After maxFree consecutive failures a
// profile is locked for an exponentially growing window (capped). Kept in
// memory: a restart resets it, which is fine for a household app.
type loginLimiter struct {
	mu      sync.Mutex
	entries map[int64]*limiterEntry
	now     func() time.Time
}

type limiterEntry struct {
	failures    int
	lockedUntil time.Time
}

const (
	limiterMaxFree  = 5
	limiterBaseLock = 30 * time.Second
	limiterMaxLock  = 15 * time.Minute
)

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{entries: map[int64]*limiterEntry{}, now: time.Now}
}

// retryAfter returns how long the key is still locked, or 0.
func (l *loginLimiter) retryAfter(key int64) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[key]
	if e == nil {
		return 0
	}
	if d := e.lockedUntil.Sub(l.now()); d > 0 {
		return d
	}
	return 0
}

func (l *loginLimiter) fail(key int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[key]
	if e == nil {
		e = &limiterEntry{}
		l.entries[key] = e
	}
	e.failures++
	if e.failures >= limiterMaxFree {
		lock := limiterBaseLock << (e.failures - limiterMaxFree)
		if lock > limiterMaxLock || lock <= 0 {
			lock = limiterMaxLock
		}
		e.lockedUntil = l.now().Add(lock)
	}
}

func (l *loginLimiter) succeed(key int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

// legacyPasscodeKey is the limiter key for household passcode attempts.
const legacyPasscodeKey int64 = -1

type AuthHandler struct {
	store      *store.Store
	sessions   *SessionManager
	dispatcher *webhook.Dispatcher
	limiter    *loginLimiter
	oidc       *OIDCService
}

func NewAuthHandler(s *store.Store, sm *SessionManager, d *webhook.Dispatcher, oidc *OIDCService) *AuthHandler {
	return &AuthHandler{store: s, sessions: sm, dispatcher: d, limiter: newLoginLimiter(), oidc: oidc}
}

func (h *AuthHandler) fire(event string, payload map[string]any) {
	if h.dispatcher != nil {
		h.dispatcher.Fire(event, payload)
	}
}

// startSession issues a session for user and sets the cookie. The token is
// also returned in the body for non-browser clients.
func (h *AuthHandler) startSession(w http.ResponseWriter, r *http.Request, user *model.User, claims SessionClaims) {
	claims.UserID = user.ID
	claims.Version = user.SessionVersion
	token, issued := h.sessions.Issue(claims)
	setSessionCookie(w, r, token, issued)
	writeJSON(w, http.StatusOK, sessionResponse(user, issued, token))
}

func sessionResponse(user *model.User, c *SessionClaims, token string) map[string]any {
	session := map[string]any{
		"method":     c.Method,
		"expires_at": c.ExpiresAt().UTC().Format(time.RFC3339),
		"persistent": c.Persistent(),
	}
	if c.Provider != "" {
		session["provider"] = c.Provider
	}
	resp := map[string]any{"user": user, "session": session}
	if token != "" {
		resp["token"] = token
	}
	return resp
}

type loginRequest struct {
	UserID int64  `json:"user_id"`
	Pin    string `json:"pin"`
	// LegacyPasscode + NewPin let an admin profile without a credential claim
	// a PIN using the old household admin passcode (one-time upgrade path).
	LegacyPasscode string `json:"legacy_passcode"`
	NewPin         string `json:"new_pin"`
}

// Login starts a session for a profile tapped on the login screen.
//
//   - Profile with a PIN: the PIN is required.
//   - Profile linked to OIDC and without a PIN: must use the provider.
//   - Admin profile with no credential at all: must set a PIN first (using
//     the legacy household passcode when one exists).
//   - Any other profile (typically a young kid): tapping is enough.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil || req.UserID <= 0 {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	user, err := h.store.GetUser(r.Context(), req.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load user")
		return
	}
	if user == nil {
		writeError(w, http.StatusUnauthorized, "user not found")
		return
	}
	ip := clientIP(r)

	switch {
	case user.HasPin:
		if d := h.limiter.retryAfter(user.ID); d > 0 {
			writeAuthError(w, http.StatusTooManyRequests, authCodeLockedOut,
				"too many attempts, try again later", map[string]any{"retry_after_seconds": int(d.Seconds()) + 1})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PinHash), []byte(req.Pin)); err != nil {
			h.limiter.fail(user.ID)
			log.Printf("auth: failed pin login for user %d (%s) from %s", user.ID, user.Name, ip)
			h.fire(webhook.EventProfilePinFailed, map[string]any{
				"user_id": user.ID, "user_name": user.Name, "ip_address": ip,
			})
			writeAuthError(w, http.StatusUnauthorized, authCodeIncorrectPin, "incorrect pin", nil)
			return
		}
		h.limiter.succeed(user.ID)
		log.Printf("auth: user %d (%s) signed in with pin from %s", user.ID, user.Name, ip)
		h.fire(webhook.EventProfilePinVerified, map[string]any{
			"user_id": user.ID, "user_name": user.Name, "ip_address": ip,
		})
		h.startSession(w, r, user, SessionClaims{Method: SessionMethodPin})

	case len(user.AuthProviders) > 0:
		writeAuthError(w, http.StatusForbidden, authCodeOIDCRequired,
			"this profile signs in with a linked account", map[string]any{"providers": user.AuthProviders})

	case user.Role == model.RoleAdmin:
		h.claimAdminPin(w, r, user, req)

	default:
		log.Printf("auth: user %d (%s) signed in (no credential) from %s", user.ID, user.Name, ip)
		h.startSession(w, r, user, SessionClaims{Method: SessionMethodTap})
	}
}

// claimAdminPin handles an admin profile that has neither a PIN nor a linked
// account. Admin profiles always need a credential, so the caller must set a
// PIN now. On upgraded installs the legacy household passcode must be
// supplied as proof; on installs without one, the first PIN set wins.
func (h *AuthHandler) claimAdminPin(w http.ResponseWriter, r *http.Request, user *model.User, req loginRequest) {
	legacy, err := h.store.GetSetting(r.Context(), legacyPasscodeSetting)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check passcode")
		return
	}
	hasLegacy := legacy != ""
	if req.NewPin == "" {
		writeAuthError(w, http.StatusForbidden, authCodeAdminSetupRequired,
			"admin profiles need a pin or linked account", map[string]any{"legacy_passcode": hasLegacy})
		return
	}
	if !pinFormatValid(req.NewPin) {
		writeError(w, http.StatusBadRequest, "pin must be 4-8 digits")
		return
	}
	ip := clientIP(r)
	if hasLegacy {
		if d := h.limiter.retryAfter(legacyPasscodeKey); d > 0 {
			writeAuthError(w, http.StatusTooManyRequests, authCodeLockedOut,
				"too many attempts, try again later", map[string]any{"retry_after_seconds": int(d.Seconds()) + 1})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(legacy), []byte(req.LegacyPasscode)); err != nil {
			h.limiter.fail(legacyPasscodeKey)
			log.Printf("auth: failed legacy admin passcode for user %d (%s) from %s", user.ID, user.Name, ip)
			h.fire(webhook.EventAdminPasscodeFailed, map[string]any{
				"user_id": user.ID, "user_name": user.Name, "ip_address": ip, "user_agent": r.UserAgent(),
			})
			writeAuthError(w, http.StatusUnauthorized, authCodeIncorrectPasscode, "incorrect passcode", nil)
			return
		}
		h.limiter.succeed(legacyPasscodeKey)
		h.fire(webhook.EventAdminPasscodeVerified, map[string]any{
			"user_id": user.ID, "user_name": user.Name, "ip_address": ip, "user_agent": r.UserAgent(),
		})
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPin), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash pin")
		return
	}
	if err := h.store.SetUserPin(r.Context(), user.ID, string(hash)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save pin")
		return
	}
	user.PinHash = string(hash)
	user.HasPin = true
	log.Printf("auth: admin user %d (%s) claimed a pin from %s", user.ID, user.Name, ip)
	h.fire(webhook.EventProfilePinChanged, map[string]any{
		"user_id": user.ID, "user_name": user.Name, "actor_id": user.ID, "actor_name": user.Name,
		"admin_override": false, "is_new": true, "ip_address": ip,
	})
	h.retireLegacyPasscode(r)
	h.startSession(w, r, user, SessionClaims{Method: SessionMethodPin})
}

// retireLegacyPasscode deletes the household passcode once every admin
// profile has a credential of its own.
func (h *AuthHandler) retireLegacyPasscode(r *http.Request) {
	_ = RetireLegacyPasscodeIfUnused(r.Context(), h.store)
}

// RetireLegacyPasscodeIfUnused clears the pre-1.0 household admin passcode
// when no admin profile still depends on it to claim a PIN. Called at
// startup and after each claim.
func RetireLegacyPasscodeIfUnused(ctx context.Context, s *store.Store) error {
	current, err := s.GetSetting(ctx, legacyPasscodeSetting)
	if err != nil || current == "" {
		return err
	}
	users, err := s.ListUsers(ctx)
	if err != nil {
		return err
	}
	for _, u := range users {
		if u.Role == model.RoleAdmin && !u.HasPin && len(u.AuthProviders) == 0 {
			return nil
		}
	}
	if err := s.SetSetting(ctx, legacyPasscodeSetting, ""); err != nil {
		return err
	}
	log.Printf("auth: every admin has a credential; legacy admin passcode retired")
	return nil
}

// Logout clears the session cookie. It is public so a stale or revoked
// cookie can always be cleared.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// LogoutEverywhere revokes every session of the caller.
func (h *AuthHandler) LogoutEverywhere(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil || user.ID == 0 {
		writeError(w, http.StatusBadRequest, "no user session")
		return
	}
	if err := h.store.BumpSessionVersion(r.Context(), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke sessions")
		return
	}
	clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// Me returns the signed-in user and session details.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	claims := SessionClaimsFromContext(r.Context())
	if user == nil || claims == nil {
		writeError(w, http.StatusUnauthorized, "sign in required")
		return
	}
	writeJSON(w, http.StatusOK, sessionResponse(user, claims, ""))
}

type uploadLinkRequest struct {
	ScheduleID int64 `json:"schedule_id"`
}

// UploadLink issues a short-lived token that lets another device (a phone
// scanning the QR code) upload a photo and complete one chore as the caller.
func (h *AuthHandler) UploadLink(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	claims := SessionClaimsFromContext(r.Context())
	if user == nil || claims == nil || claims.Method == SessionMethodUpload {
		writeError(w, http.StatusForbidden, "a signed-in profile is required")
		return
	}
	var req uploadLinkRequest
	if err := decodeJSON(r, &req); err != nil || req.ScheduleID <= 0 {
		writeError(w, http.StatusBadRequest, "schedule_id is required")
		return
	}
	schedule, err := h.store.GetSchedule(r.Context(), req.ScheduleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get schedule")
		return
	}
	if schedule == nil {
		writeError(w, http.StatusNotFound, "schedule not found")
		return
	}
	if !canActOnSchedule(user, schedule) {
		writeError(w, http.StatusForbidden, "this chore is assigned to someone else")
		return
	}
	token, issued := h.sessions.Issue(SessionClaims{
		UserID:     user.ID,
		Version:    user.SessionVersion,
		Method:     SessionMethodUpload,
		ScheduleID: schedule.ID,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"expires_at": issued.ExpiresAt().UTC().Format(time.RFC3339),
	})
}

// canActOnSchedule reports whether user may complete/uncomplete a schedule:
// its assignee, or any admin acting on someone's behalf.
func canActOnSchedule(user *model.User, schedule *model.ChoreSchedule) bool {
	return user.Role == model.RoleAdmin || schedule.AssignedTo == user.ID
}
