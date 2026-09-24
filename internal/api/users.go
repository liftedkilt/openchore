package api

import (
	"log"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
	"github.com/liftedkilt/openchore/internal/webhook"
)

type UserHandler struct {
	store      *store.Store
	dispatcher *webhook.Dispatcher
}

func NewUserHandler(s *store.Store, dispatcher *webhook.Dispatcher) *UserHandler {
	return &UserHandler{store: s, dispatcher: dispatcher}
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
	Color     string `json:"color"`
	// Pin sets an initial profile PIN on create. Required for admin profiles,
	// which must always have a PIN or a linked account.
	Pin string `json:"pin"`
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
		req.Role = model.RoleChild
	}
	if req.Role != model.RoleAdmin && req.Role != model.RoleChild {
		writeError(w, http.StatusBadRequest, "role must be admin or child")
		return
	}
	if req.Pin != "" && !pinFormatValid(req.Pin) {
		writeError(w, http.StatusBadRequest, "pin must be 4-8 digits")
		return
	}
	if msg := validateThemeAndColor(req.Theme, req.Color); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if req.Role == model.RoleAdmin && req.Pin == "" {
		writeError(w, http.StatusBadRequest, "admin profiles need a pin")
		return
	}

	user := &model.User{
		Name:      req.Name,
		AvatarURL: req.AvatarURL,
		Role:      req.Role,
		Age:       req.Age,
		Theme:     req.Theme,
		Color:     req.Color, // empty: the store assigns the next free colour
	}
	if req.Pin != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Pin), bcrypt.DefaultCost)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to hash pin")
			return
		}
		user.PinHash = string(hash)
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
	if msg := validateThemeAndColor(req.Theme, req.Color); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.AvatarURL != "" {
		existing.AvatarURL = req.AvatarURL
	}
	roleChanged := false
	if req.Role != "" && req.Role != existing.Role {
		if req.Role != model.RoleAdmin && req.Role != model.RoleChild {
			writeError(w, http.StatusBadRequest, "role must be admin or child")
			return
		}
		if req.Role == model.RoleAdmin && !existing.HasPin && len(existing.AuthProviders) == 0 {
			writeError(w, http.StatusBadRequest, "set a pin on this profile before making it an admin")
			return
		}
		if existing.Role == model.RoleAdmin {
			last, err := h.isLastAdmin(r, existing.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to list users")
				return
			}
			if last {
				writeError(w, http.StatusConflict, "cannot remove the last admin")
				return
			}
		}
		existing.Role = req.Role
		roleChanged = true
	}
	if req.Age != nil {
		existing.Age = req.Age
	}
	if req.Theme != "" {
		existing.Theme = req.Theme
	}
	if req.Color != "" {
		existing.Color = req.Color
	}

	if err := h.store.UpdateUser(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}
	if roleChanged {
		// Sessions carry no role, but revoke them anyway so a demoted admin's
		// open admin screens stop working immediately and the change is
		// clearly effective.
		if err := h.store.BumpSessionVersion(r.Context(), existing.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to revoke sessions")
			return
		}
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

	var req struct {
		Theme string `json:"theme"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !model.ValidTheme(req.Theme) {
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

// UpdateColor sets the caller's own person colour (a key from
// model.PersonColors). Admins change other people's colour via Update.
func (h *UserHandler) UpdateColor(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	caller := UserFromContext(r.Context())
	if caller.ID != id {
		writeError(w, http.StatusForbidden, "can only update your own color")
		return
	}

	var req struct {
		Color string `json:"color"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !model.ValidPersonColor(req.Color) {
		writeError(w, http.StatusBadRequest, "invalid color")
		return
	}

	existing, err := h.store.GetUser(r.Context(), id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	existing.Color = req.Color
	if err := h.store.UpdateUser(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update color")
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

// validateThemeAndColor checks optional theme/color fields on create and
// update requests. Empty values are allowed (they mean "leave as is" or
// "assign a default"). It returns an error message, or "" when valid.
func validateThemeAndColor(theme, color string) string {
	if theme != "" && !model.ValidTheme(theme) {
		return "invalid theme"
	}
	if color != "" && !model.ValidPersonColor(color) {
		return "invalid color"
	}
	return ""
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
	if user.Role == model.RoleAdmin {
		last, err := h.isLastAdmin(r, user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list users")
			return
		}
		if last {
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
	isAdmin := caller.Role == model.RoleAdmin
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

	ip := clientIP(r)
	targetUser, err := h.store.GetUser(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if targetUser == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	targetName := targetUser.Name

	existingHash, err := h.store.GetUserPinHash(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check pin")
		return
	}
	if existingHash != "" && !adminOverride {
		// Caller is changing their own PIN — must supply the current value.
		if err := bcrypt.CompareHashAndPassword([]byte(existingHash), []byte(req.CurrentPin)); err != nil {
			log.Printf("auth: failed pin change attempt for user %d (%s) by user %d (%s) from %s", id, targetName, caller.ID, caller.Name, ip)
			if h.dispatcher != nil {
				h.dispatcher.Fire(webhook.EventProfilePinFailed, map[string]any{
					"user_id":    id,
					"user_name":  targetName,
					"actor_id":   caller.ID,
					"actor_name": caller.Name,
					"ip_address": ip,
				})
			}
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

	log.Printf("auth: user %d (%s) pin set by user %d (%s) from %s (admin_override=%v)", id, targetName, caller.ID, caller.Name, ip, adminOverride)
	if h.dispatcher != nil {
		h.dispatcher.Fire(webhook.EventProfilePinChanged, map[string]any{
			"user_id":        id,
			"user_name":      targetName,
			"actor_id":       caller.ID,
			"actor_name":     caller.Name,
			"admin_override": adminOverride,
			"is_new":         existingHash == "",
			"ip_address":     ip,
		})
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
	isAdmin := caller.Role == model.RoleAdmin
	targetIsSelf := caller.ID == id
	if !isAdmin && !targetIsSelf {
		writeError(w, http.StatusForbidden, "can only change your own pin")
		return
	}
	adminOverride := isAdmin && !targetIsSelf

	ip := clientIP(r)
	targetUser, err := h.store.GetUser(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if targetUser == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	targetName := targetUser.Name

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
			log.Printf("auth: failed pin clear attempt for user %d (%s) by user %d (%s) from %s", id, targetName, caller.ID, caller.Name, ip)
			if h.dispatcher != nil {
				h.dispatcher.Fire(webhook.EventProfilePinFailed, map[string]any{
					"user_id":    id,
					"user_name":  targetName,
					"actor_id":   caller.ID,
					"actor_name": caller.Name,
					"ip_address": ip,
				})
			}
			writeError(w, http.StatusUnauthorized, "incorrect current pin")
			return
		}
	}

	if targetUser.Role == model.RoleAdmin && len(targetUser.AuthProviders) == 0 {
		writeError(w, http.StatusConflict, "admin profiles need a pin or linked account; link an account before removing the pin")
		return
	}

	if err := h.store.SetUserPin(r.Context(), id, ""); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear pin")
		return
	}

	log.Printf("auth: user %d (%s) pin cleared by user %d (%s) from %s (admin_override=%v)", id, targetName, caller.ID, caller.Name, ip, adminOverride)
	if h.dispatcher != nil {
		h.dispatcher.Fire(webhook.EventProfilePinCleared, map[string]any{
			"user_id":        id,
			"user_name":      targetName,
			"actor_id":       caller.ID,
			"actor_name":     caller.Name,
			"admin_override": adminOverride,
			"ip_address":     ip,
		})
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

// isLastAdmin reports whether userID is the only admin profile.
func (h *UserHandler) isLastAdmin(r *http.Request, userID int64) (bool, error) {
	users, err := h.store.ListUsers(r.Context())
	if err != nil {
		return false, err
	}
	for _, u := range users {
		if u.Role == model.RoleAdmin && u.ID != userID {
			return false, nil
		}
	}
	return true, nil
}
