package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

type TokenHandler struct {
	store *store.Store
}

func NewTokenHandler(s *store.Store) *TokenHandler {
	return &TokenHandler{store: s}
}

type createTokenRequest struct {
	Name string `json:"name"`
}

// Create generates a new API token and returns the plaintext token once.
func (h *TokenHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createTokenRequest
	if err := decodeJSON(r, &req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	// Generate 32 random bytes -> 64-char hex token
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	plaintext := hex.EncodeToString(raw)

	hash := sha256.Sum256([]byte(plaintext))
	tokenHash := hex.EncodeToString(hash[:])

	token := &model.APIToken{
		Name:      req.Name,
		TokenHash: tokenHash,
	}
	if err := h.store.CreateAPIToken(r.Context(), token); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":    token.ID,
		"name":  token.Name,
		"token": plaintext,
	})
}

// List returns all API tokens (without hashes).
func (h *TokenHandler) List(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.store.ListAPITokens(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tokens")
		return
	}
	if tokens == nil {
		tokens = []model.APIToken{}
	}
	writeJSON(w, http.StatusOK, tokens)
}

// Revoke soft-deletes an API token.
func (h *TokenHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid token id")
		return
	}
	if err := h.store.RevokeAPIToken(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// recordedBy is the profile an admin action is stored against. A session's
// own user, normally; an API token has no profile (its synthetic user is id
// 0), so its actions are recorded against the household's first parent.
// Columns such as chores.created_by and chore_completions.approved_by
// reference users, and foreign keys are enforced.
func recordedBy(ctx context.Context, s *store.Store, caller *model.User) (int64, error) {
	if caller != nil && caller.ID != 0 {
		return caller.ID, nil
	}
	users, err := s.ListUsers(ctx)
	if err != nil {
		return 0, err
	}
	var first int64
	for _, u := range users {
		if u.Role == model.RoleAdmin && (first == 0 || u.ID < first) {
			first = u.ID
		}
	}
	if first == 0 {
		return 0, errors.New("no parent profile to record this action against")
	}
	return first, nil
}
