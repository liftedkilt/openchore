package api

import (
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"github.com/liftedkilt/openchore/internal/store"
)

type AdminHandler struct {
	store *store.Store
}

func NewAdminHandler(s *store.Store) *AdminHandler {
	return &AdminHandler{store: s}
}

type verifyPasscodeRequest struct {
	Passcode string `json:"passcode"`
}

func (h *AdminHandler) VerifyPasscode(w http.ResponseWriter, r *http.Request) {
	var req verifyPasscodeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	stored, err := h.store.GetSetting(r.Context(), "admin_passcode")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check passcode")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(stored), []byte(req.Passcode)); err != nil {
		writeError(w, http.StatusUnauthorized, "incorrect passcode")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"valid": true})
}

type updatePasscodeRequest struct {
	OldPasscode string `json:"old_passcode"`
	NewPasscode string `json:"new_passcode"`
}

func (h *AdminHandler) UpdatePasscode(w http.ResponseWriter, r *http.Request) {
	var req updatePasscodeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.NewPasscode) < 4 {
		writeError(w, http.StatusBadRequest, "passcode must be at least 4 characters")
		return
	}

	stored, err := h.store.GetSetting(r.Context(), "admin_passcode")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check passcode")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored), []byte(req.OldPasscode)); err != nil {
		writeError(w, http.StatusUnauthorized, "incorrect current passcode")
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPasscode), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash passcode")
		return
	}

	if err := h.store.SetSetting(r.Context(), "admin_passcode", string(hashed)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update passcode")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"updated": true})
}

func (h *AdminHandler) GetSetting(w http.ResponseWriter, r *http.Request) {
	key := urlParam(r, "key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "key required")
		return
	}
	val, err := h.store.GetSetting(r.Context(), key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get setting")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"key": key, "value": val})
}

func (h *AdminHandler) SetSetting(w http.ResponseWriter, r *http.Request) {
	key := urlParam(r, "key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "key required")
		return
	}
	var req struct {
		Value string `json:"value"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.store.SetSetting(r.Context(), key, req.Value); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update setting")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"key": key, "value": req.Value})
}
