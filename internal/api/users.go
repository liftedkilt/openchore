package api

import (
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

type UserHandler struct {
	store *store.Store
}

func NewUserHandler(s *store.Store) *UserHandler {
	return &UserHandler{store: s}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	if users == nil {
		users = []model.User{}
	}
	writeJSON(w, http.StatusOK, users)
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	user, err := h.store.GetUser(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if user == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

type createUserRequest struct {
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Role      string `json:"role"`
	Age       *int   `json:"age"`
	Theme     string `json:"theme"`
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Role == "" {
		req.Role = "child"
	}
	if req.Role != "admin" && req.Role != "child" {
		writeError(w, http.StatusBadRequest, "role must be admin or child")
		return
	}

	theme := req.Theme
	if req.Role == "admin" {
		// Admin users never have a theme — the admin UI always uses the default.
		theme = ""
	}
	user := &model.User{
		Name:      req.Name,
		AvatarURL: req.AvatarURL,
		Role:      req.Role,
		Age:       req.Age,
		Theme:     theme,
	}
	if err := h.store.CreateUser(r.Context(), user); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	existing, err := h.store.GetUser(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	var req createUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.AvatarURL != "" {
		existing.AvatarURL = req.AvatarURL
	}
	if req.Role != "" {
		if req.Role != "admin" && req.Role != "child" {
			writeError(w, http.StatusBadRequest, "role must be admin or child")
			return
		}
		existing.Role = req.Role
	}
	if req.Age != nil {
		existing.Age = req.Age
	}
	if req.Theme != "" && existing.Role != "admin" {
		existing.Theme = req.Theme
	}
	if existing.Role == "admin" {
		// Admin users never have a theme — clear any stale value.
		existing.Theme = ""
	}

	if err := h.store.UpdateUser(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (h *UserHandler) UpdateTheme(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	// Users can only update their own theme
	caller := UserFromContext(r.Context())
	if caller.ID != id {
		writeError(w, http.StatusForbidden, "can only update your own theme")
		return
	}

	// Admin users do not have themes — the admin UI always uses the default.
	if caller.Role == "admin" {
		writeError(w, http.StatusForbidden, "admin users do not have themes")
		return
	}

	var req struct {
		Theme string `json:"theme"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	validThemes := map[string]bool{"default": true, "quest": true, "galaxy": true, "forest": true}
	if !validThemes[req.Theme] {
		writeError(w, http.StatusBadRequest, "invalid theme")
		return
	}

	existing, err := h.store.GetUser(r.Context(), id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	existing.Theme = req.Theme
	if err := h.store.UpdateUser(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update theme")
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (h *UserHandler) UpdateAvatar(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	caller := UserFromContext(r.Context())
	if caller.ID != id {
		writeError(w, http.StatusForbidden, "can only update your own avatar")
		return
	}

	var req struct {
		AvatarURL string `json:"avatar_url"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.AvatarURL == "" {
		writeError(w, http.StatusBadRequest, "avatar_url is required")
		return
	}

	existing, err := h.store.GetUser(r.Context(), id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	existing.AvatarURL = req.AvatarURL
	if err := h.store.UpdateUser(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update avatar")
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (h *UserHandler) UpdateLineColor(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	caller := UserFromContext(r.Context())
	if caller.ID != id {
		writeError(w, http.StatusForbidden, "can only update your own line color")
		return
	}

	var req struct {
		LineColor string `json:"line_color"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.LineColor == "" {
		writeError(w, http.StatusBadRequest, "line_color is required")
		return
	}

	existing, err := h.store.GetUser(r.Context(), id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	existing.LineColor = req.LineColor
	if err := h.store.UpdateUser(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update line color")
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (h *UserHandler) Pause(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	existing, err := h.store.GetUser(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	existing.Paused = true
	if err := h.store.UpdateUser(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to pause user")
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (h *UserHandler) Unpause(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	existing, err := h.store.GetUser(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	existing.Paused = false
	if err := h.store.UpdateUser(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unpause user")
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	// Prevent deleting the last admin
	user, err := h.store.GetUser(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if user == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if user.Role == "admin" {
		users, err := h.store.ListUsers(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list users")
			return
		}
		adminCount := 0
		for _, u := range users {
			if u.Role == "admin" {
				adminCount++
			}
		}
		if adminCount <= 1 {
			writeError(w, http.StatusConflict, "cannot delete the last admin user")
			return
		}
	}

	if err := h.store.DeleteUser(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Profile PIN ---

// pinFormatValid checks that a PIN is 4-8 numeric digits.
func pinFormatValid(pin string) bool {
	if len(pin) < 4 || len(pin) > 8 {
		return false
	}
	for _, c := range pin {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

type verifyPinRequest struct {
	Pin string `json:"pin"`
}

// VerifyPin checks a PIN attempt against the stored hash for the given user.
// This is a public endpoint: it is how a kid unlocks their own profile at the
// login screen, before any session identity exists.
func (h *UserHandler) VerifyPin(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var req verifyPinRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	hash, err := h.store.GetUserPinHash(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check pin")
		return
	}
	if hash == "" {
		// No PIN set — nothing to verify.
		writeError(w, http.StatusBadRequest, "profile has no pin")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Pin)); err != nil {
		writeError(w, http.StatusUnauthorized, "incorrect pin")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"valid": true})
}

type setPinRequest struct {
	CurrentPin string `json:"current_pin"`
	NewPin     string `json:"new_pin"`
}

// SetPin sets or updates a user's profile PIN. A user changing their own PIN
// (whether admin or child) must supply the current PIN if one is already set.
// Admins can reset another user's PIN without supplying the current value
// (used for forgotten-PIN recovery from the admin dashboard).
func (h *UserHandler) SetPin(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	caller := UserFromContext(r.Context())
	if caller == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	isAdmin := caller.Role == "admin"
	targetIsSelf := caller.ID == id
	if !isAdmin && !targetIsSelf {
		writeError(w, http.StatusForbidden, "can only change your own pin")
		return
	}
	// Only an admin acting on a *different* user may bypass the current-PIN
	// check. An admin changing their own PIN must still prove they know it.
	adminOverride := isAdmin && !targetIsSelf

	var req setPinRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !pinFormatValid(req.NewPin) {
		writeError(w, http.StatusBadRequest, "pin must be 4-8 digits")
		return
	}

	existingHash, err := h.store.GetUserPinHash(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check pin")
		return
	}
	if existingHash == "" {
		// No PIN set yet — ensure the target user actually exists.
		u, err := h.store.GetUser(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get user")
			return
		}
		if u == nil {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
	} else if !adminOverride {
		// Caller is changing their own PIN — must supply the current value.
		if err := bcrypt.CompareHashAndPassword([]byte(existingHash), []byte(req.CurrentPin)); err != nil {
			writeError(w, http.StatusUnauthorized, "incorrect current pin")
			return
		}
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPin), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash pin")
		return
	}
	if err := h.store.SetUserPin(r.Context(), id, string(newHash)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save pin")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"has_pin": true})
}

type clearPinRequest struct {
	CurrentPin string `json:"current_pin"`
}

// ClearPin removes the PIN from a user's profile. A user clearing their own
// PIN (whether admin or child) must supply the current value. Admins clearing
// another user's PIN can do so without it (used to reset a forgotten kid PIN
// from the admin dashboard).
func (h *UserHandler) ClearPin(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	caller := UserFromContext(r.Context())
	if caller == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	isAdmin := caller.Role == "admin"
	targetIsSelf := caller.ID == id
	if !isAdmin && !targetIsSelf {
		writeError(w, http.StatusForbidden, "can only change your own pin")
		return
	}
	adminOverride := isAdmin && !targetIsSelf

	existingHash, err := h.store.GetUserPinHash(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check pin")
		return
	}
	if existingHash == "" {
		// Already clear — idempotent success.
		writeJSON(w, http.StatusOK, map[string]bool{"has_pin": false})
		return
	}

	if !adminOverride {
		var req clearPinRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(existingHash), []byte(req.CurrentPin)); err != nil {
			writeError(w, http.StatusUnauthorized, "incorrect current pin")
			return
		}
	}

	if err := h.store.SetUserPin(r.Context(), id, ""); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear pin")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"has_pin": false})
}

func (h *UserHandler) GetChores(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	view := r.URL.Query().Get("view")
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format(model.DateFormat)
	}

	date, err := time.Parse(model.DateFormat, dateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date format, use YYYY-MM-DD")
		return
	}

	var dates []string
	switch view {
	case "weekly":
		// Find Monday of the week
		weekday := date.Weekday()
		offset := int(weekday - time.Monday)
		if offset < 0 {
			offset += 7
		}
		monday := date.AddDate(0, 0, -offset)
		for i := 0; i < 7; i++ {
			dates = append(dates, monday.AddDate(0, 0, i).Format(model.DateFormat))
		}
	default: // daily
		dates = []string{dateStr}
	}

	chores, err := h.store.GetScheduledChoresForUser(r.Context(), id, dates, time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get chores")
		return
	}
	if chores == nil {
		chores = []model.ScheduledChore{}
	}
	writeJSON(w, http.StatusOK, chores)
}
