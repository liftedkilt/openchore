package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

type contextKey string

const userContextKey contextKey = "user"

const sessionClaimsContextKey contextKey = "session_claims"

// uploadCompletePath matches the only chore-completion route an upload
// session may call.
var uploadCompletePath = regexp.MustCompile(`^/api/schedules/(\d+)/complete$`)

// uploadSessionAllowed restricts single-chore upload sessions (issued for the
// photo QR code) to uploading a photo and completing that one schedule.
func uploadSessionAllowed(r *http.Request, c *SessionClaims) bool {
	if r.Method != http.MethodPost {
		return false
	}
	if r.URL.Path == "/api/upload" {
		return true
	}
	m := uploadCompletePath.FindStringSubmatch(r.URL.Path)
	if m == nil {
		return false
	}
	id, err := strconv.ParseInt(m[1], 10, 64)
	return err == nil && id == c.ScheduleID
}

// loadSessionUser validates a session token and returns the user it names.
// It returns (nil, nil, nil) when the token is invalid, expired, revoked, or
// the user no longer exists.
func loadSessionUser(ctx context.Context, s *store.Store, sm *SessionManager, token string) (*model.User, *SessionClaims, error) {
	claims, err := sm.Verify(token)
	if err != nil {
		return nil, nil, nil
	}
	user, err := s.GetUser(ctx, claims.UserID)
	if err != nil {
		return nil, nil, err
	}
	if user == nil || user.SessionVersion != claims.Version {
		return nil, nil, nil
	}
	return user, claims, nil
}

// RequireSession authenticates a request with a server-issued session (cookie
// or "Bearer ocs1.…"). The legacy X-User-ID header is not trusted.
func RequireSession(s *store.Store, sm *SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := sessionTokenFromRequest(r)
			if token == "" {
				writeError(w, http.StatusUnauthorized, "sign in required")
				return
			}
			user, claims, err := loadSessionUser(r.Context(), s, sm, token)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to load user")
				return
			}
			if user == nil {
				clearSessionCookie(w, r)
				writeError(w, http.StatusUnauthorized, "session expired or revoked")
				return
			}
			if claims.Method == SessionMethodUpload && !uploadSessionAllowed(r, claims) {
				writeError(w, http.StatusForbidden, "upload link cannot be used for this action")
				return
			}
			ctx := context.WithValue(r.Context(), userContextKey, user)
			ctx = context.WithValue(ctx, sessionClaimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SessionClaimsFromContext returns the claims of the session that
// authenticated the request, or nil for API-token requests.
func SessionClaimsFromContext(ctx context.Context) *SessionClaims {
	c, _ := ctx.Value(sessionClaimsContextKey).(*SessionClaims)
	return c
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil || user.Role != model.RoleAdmin {
			writeError(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// apiTokenContextKey marks that the request was authenticated via API token.
const apiTokenContextKey contextKey = "api_token"

// RequireUserOrToken accepts either an API token ("Bearer <hex>") or a
// session (see RequireSession). A valid API token grants admin-level access
// (synthetic admin user in context).
func RequireUserOrToken(s *store.Store, sm *SessionManager) func(http.Handler) http.Handler {
	requireSession := RequireSession(s, sm)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") && !strings.HasPrefix(auth, "Bearer "+sessionTokenPrefix) {
				token := strings.TrimPrefix(auth, "Bearer ")
				hash := sha256.Sum256([]byte(token))
				tokenHash := hex.EncodeToString(hash[:])

				apiToken, err := s.ValidateAPIToken(r.Context(), tokenHash)
				if err != nil {
					writeError(w, http.StatusInternalServerError, "failed to validate token")
					return
				}
				if apiToken == nil {
					writeError(w, http.StatusUnauthorized, "invalid or revoked API token")
					return
				}

				// Update last_used_at in the background (non-blocking)
				go func() { _ = s.UpdateTokenLastUsed(context.Background(), apiToken.ID) }()

				// Inject a synthetic admin user so RequireAdmin passes
				syntheticAdmin := &model.User{
					ID:   0,
					Name: "api:" + apiToken.Name,
					Role: "admin",
				}
				ctx := context.WithValue(r.Context(), userContextKey, syntheticAdmin)
				ctx = context.WithValue(ctx, apiTokenContextKey, apiToken)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			requireSession(next).ServeHTTP(w, r)
		})
	}
}

func UserFromContext(ctx context.Context) *model.User {
	u, _ := ctx.Value(userContextKey).(*model.User)
	return u
}

func APITokenFromContext(ctx context.Context) *model.APIToken {
	t, _ := ctx.Value(apiTokenContextKey).(*model.APIToken)
	return t
}
