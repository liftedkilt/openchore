package api

import (
	"log"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/liftedkilt/openchore/internal/config"
	"github.com/liftedkilt/openchore/internal/store"
	"github.com/liftedkilt/openchore/internal/webhook"
)

// writableSettings is the allowlist of setting keys that can be written via
// the admin API.  Keys not in this set are rejected with 400.
var writableSettings = map[string]bool{
	"ai_enabled":                true,
	"ai_endpoint":               true,
	"ai_model":                  true,
	"ai_auto_approve_threshold": true,
	"ai_tts_enabled":            true,
	"ai_tts_endpoint":           true,
	"ai_tts_voice":              true,
	"base_url":                  true,
	"discord_webhook_url":       true,
}

type AdminHandler struct {
	store      *store.Store
	dispatcher *webhook.Dispatcher
}

func NewAdminHandler(s *store.Store, dispatcher *webhook.Dispatcher) *AdminHandler {
	return &AdminHandler{store: s, dispatcher: dispatcher}
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

	ip := clientIP(r)
	if err := bcrypt.CompareHashAndPassword([]byte(stored), []byte(req.Passcode)); err != nil {
		log.Printf("auth: failed admin passcode attempt from %s", ip)
		if h.dispatcher != nil {
			h.dispatcher.Fire(webhook.EventAdminPasscodeFailed, map[string]any{
				"ip_address": ip,
				"user_agent": r.UserAgent(),
			})
		}
		writeError(w, http.StatusUnauthorized, "incorrect passcode")
		return
	}

	log.Printf("auth: admin passcode verified from %s", ip)
	if h.dispatcher != nil {
		h.dispatcher.Fire(webhook.EventAdminPasscodeVerified, map[string]any{
			"ip_address": ip,
			"user_agent": r.UserAgent(),
		})
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
	ip := clientIP(r)
	if err := bcrypt.CompareHashAndPassword([]byte(stored), []byte(req.OldPasscode)); err != nil {
		log.Printf("auth: failed admin passcode update attempt from %s", ip)
		if h.dispatcher != nil {
			h.dispatcher.Fire(webhook.EventAdminPasscodeFailed, map[string]any{
				"ip_address": ip,
				"user_agent": r.UserAgent(),
			})
		}
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

	caller := UserFromContext(r.Context())
	var actorID int64
	var actorName string
	if caller != nil {
		actorID = caller.ID
		actorName = caller.Name
	}
	log.Printf("auth: admin passcode changed by user %d (%s) from %s", actorID, actorName, ip)
	if h.dispatcher != nil {
		h.dispatcher.Fire(webhook.EventAdminPasscodeChanged, map[string]any{
			"actor_id":   actorID,
			"actor_name": actorName,
			"ip_address": ip,
		})
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
	if !writableSettings[key] {
		writeError(w, http.StatusBadRequest, "unknown setting key")
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

func (h *AdminHandler) ExportConfig(w http.ResponseWriter, r *http.Request) {
	allSections := []string{"users", "chores", "rewards", "streak_rewards", "settings"}
	sections := allSections
	if q := r.URL.Query().Get("sections"); q != "" {
		sections = strings.Split(q, ",")
	}

	cfg, err := config.Export(r.Context(), h.store, sections)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to export config")
		return
	}

	data, err := config.Marshal(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal config")
		return
	}

	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", `attachment; filename="config.yaml"`)
	w.Write(data)
}
