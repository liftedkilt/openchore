package api

import (
	"context"
	"net/http"
	"strconv"

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

func UserFromContext(ctx context.Context) *model.User {
	u, _ := ctx.Value(userContextKey).(*model.User)
	return u
}
