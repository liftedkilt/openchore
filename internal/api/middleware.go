package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

type contextKey string

const userContextKey contextKey = "user"

// RequireUser extracts X-User-ID header and loads the user into context.
func RequireUser(s *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			idStr := r.Header.Get("X-User-ID")
			if idStr == "" {
				writeError(w, http.StatusUnauthorized, "X-User-ID header required")
				return
			}
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid X-User-ID")
				return
			}
			user, err := s.GetUser(r.Context(), id)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to load user")
				return
			}
			if user == nil {
				writeError(w, http.StatusUnauthorized, "user not found")
				return
			}
			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil || user.Role != "admin" {
			writeError(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// apiTokenContextKey marks that the request was authenticated via API token.
const apiTokenContextKey contextKey = "api_token"

// RequireUserOrToken checks for a Bearer token first, then falls back to X-User-ID.
// A valid API token grants admin-level access (synthetic admin user in context).
func RequireUserOrToken(s *store.Store) func(http.Handler) http.Handler {
	requireUser := RequireUser(s)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
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

			// Fall back to X-User-ID header auth
			requireUser(next).ServeHTTP(w, r)
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
