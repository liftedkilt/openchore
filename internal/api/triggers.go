package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

type TriggerHandler struct {
	store *store.Store
}

func NewTriggerHandler(s *store.Store) *TriggerHandler {
	return &TriggerHandler{store: s}
}

// generateUUID produces a 32-char hex string using crypto/rand.
func generateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// FireTrigger handles POST /api/hooks/trigger/{uuid} — public endpoint.
func (h *TriggerHandler) FireTrigger(w http.ResponseWriter, r *http.Request) {
	uuid := urlParam(r, "uuid")
	if uuid == "" {
		writeError(w, http.StatusBadRequest, "uuid is required")
		return
	}

	ctx := r.Context()

	trigger, err := h.store.GetChoreTriggerByUUID(ctx, uuid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to look up trigger")
		return
	}
	if trigger == nil {
		writeError(w, http.StatusNotFound, "trigger not found")
		return
	}
	if !trigger.Enabled {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "trigger is disabled",
		})
		return
	}

	// Check cooldown
	if trigger.CooldownMinutes > 0 && trigger.LastTriggeredAt != nil {
		lastFired, parseErr := parseFlexibleTime(*trigger.LastTriggeredAt)
		if parseErr == nil {
			cooldownEnd := lastFired.Add(time.Duration(trigger.CooldownMinutes) * time.Minute)
			if time.Now().UTC().Before(cooldownEnd) {
				writeJSON(w, http.StatusTooManyRequests, map[string]string{
					"error": fmt.Sprintf("Trigger is in cooldown, try again after %s", cooldownEnd.Format("15:04:05 UTC")),
				})
				return
			}
		}
	}

	// Resolve assigned user
	var assignedUserID int64
	var assignedUserName string

	assignToParam := r.URL.Query().Get("assign_to")
	if assignToParam != "" {
		user, err := h.store.GetUserByName(ctx, assignToParam)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to look up user")
			return
		}
		if user == nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("user %q not found", assignToParam))
			return
		}
		assignedUserID = user.ID
		assignedUserName = user.Name
	} else if trigger.DefaultAssignedTo != nil {
		user, err := h.store.GetUser(ctx, *trigger.DefaultAssignedTo)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to look up default user")
			return
		}
		if user == nil {
			writeError(w, http.StatusBadRequest, "default assigned user no longer exists")
			return
		}
		assignedUserID = user.ID
		assignedUserName = user.Name
	} else {
		writeError(w, http.StatusBadRequest, "no assignee: set assign_to param or configure a default")
		return
	}

	// Check if assigned user is paused
	assignedUser, err := h.store.GetUser(ctx, assignedUserID)
	if err == nil && assignedUser != nil && assignedUser.Paused {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("user %q is paused and cannot be assigned chores", assignedUserName))
		return
	}

	// Resolve due_by and available_at (query param overrides default)
	dueBy := r.URL.Query().Get("due_by")
	if dueBy == "" && trigger.DefaultDueBy != nil {
		dueBy = *trigger.DefaultDueBy
	}
	availableAt := r.URL.Query().Get("available_at")
	if availableAt == "" && trigger.DefaultAvailableAt != nil {
		availableAt = *trigger.DefaultAvailableAt
	}

	// Get chore title
	chore, err := h.store.GetChore(ctx, trigger.ChoreID)
	if err != nil || chore == nil {
		writeError(w, http.StatusInternalServerError, "chore not found")
		return
	}

	// Check for duplicate: same chore + user + today
	today := time.Now().Format(model.DateFormat)
	exists, err := h.store.ScheduleExistsForDate(ctx, trigger.ChoreID, assignedUserID, today)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check existing schedules")
		return
	}
	if exists {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": fmt.Sprintf("%s is already assigned to %s for today", chore.Title, assignedUserName),
		})
		return
	}

	// Create one-off schedule for today
	schedule := &model.ChoreSchedule{
		ChoreID:        trigger.ChoreID,
		AssignedTo:     assignedUserID,
		AssignmentType: "individual",
		SpecificDate:   &today,
		AvailableAt:    strPtrOrNil(availableAt),
		DueBy:          strPtrOrNil(dueBy),
		PointsMultiplier: 1.0,
		ExpiryPenalty:  "block",
	}

	if err := h.store.CreateSchedule(ctx, schedule); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create schedule")
		return
	}

	// Update last_triggered_at
	if err := h.store.UpdateTriggerLastFired(ctx, trigger.ID, time.Now()); err != nil {
		// Non-fatal, schedule was already created
		fmt.Printf("WARN: failed to update last_triggered_at for trigger %d: %v\n", trigger.ID, err)
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"message":      "Chore triggered",
		"schedule_id":  schedule.ID,
		"chore_id":     chore.ID,
		"chore":        chore.Title,
		"assigned_to":  assignedUserName,
		"date":         today,
		"available_at": schedule.AvailableAt,
		"due_by":       schedule.DueBy,
	})
}

// ListTriggerable returns all chores that have enabled triggers, with user list.
// Designed for integration discovery (e.g. Home Assistant).
func (h *TriggerHandler) ListTriggerable(w http.ResponseWriter, r *http.Request) {
	chores, err := h.store.ListTriggersWithChores(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list triggerable chores")
		return
	}

	// Also include the user list so integrations can build assignment UIs
	users, err := h.store.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	type userInfo struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Role string `json:"role"`
	}
	userList := make([]userInfo, len(users))
	for i, u := range users {
		userList[i] = userInfo{ID: u.ID, Name: u.Name, Role: u.Role}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"chores": chores,
		"users":  userList,
	})
}

// --- Admin CRUD ---

func (h *TriggerHandler) ListForChore(w http.ResponseWriter, r *http.Request) {
	choreID, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid chore id")
		return
	}
	triggers, err := h.store.ListChoreTriggersForChore(r.Context(), choreID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list triggers")
		return
	}
	if triggers == nil {
		triggers = []model.ChoreTrigger{}
	}
	writeJSON(w, http.StatusOK, triggers)
}

func (h *TriggerHandler) Create(w http.ResponseWriter, r *http.Request) {
	choreID, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid chore id")
		return
	}

	var req struct {
		DefaultAssignedTo  *int64  `json:"default_assigned_to"`
		DefaultDueBy       *string `json:"default_due_by"`
		DefaultAvailableAt *string `json:"default_available_at"`
		Enabled            *bool   `json:"enabled"`
		CooldownMinutes    int     `json:"cooldown_minutes"`
	}
	if err := decodeJSON(r, &req); err != nil {
		// Allow empty body — all fields are optional
		req = struct {
			DefaultAssignedTo  *int64  `json:"default_assigned_to"`
			DefaultDueBy       *string `json:"default_due_by"`
			DefaultAvailableAt *string `json:"default_available_at"`
			Enabled            *bool   `json:"enabled"`
			CooldownMinutes    int     `json:"cooldown_minutes"`
		}{}
	}

	uuid, err := generateUUID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate uuid")
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	trigger := &model.ChoreTrigger{
		UUID:               uuid,
		ChoreID:            choreID,
		DefaultAssignedTo:  req.DefaultAssignedTo,
		DefaultDueBy:       req.DefaultDueBy,
		DefaultAvailableAt: req.DefaultAvailableAt,
		Enabled:            enabled,
		CooldownMinutes:    req.CooldownMinutes,
	}

	if err := h.store.CreateChoreTrigger(r.Context(), trigger); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create trigger")
		return
	}

	writeJSON(w, http.StatusCreated, trigger)
}

func (h *TriggerHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid trigger id")
		return
	}

	var req struct {
		DefaultAssignedTo  *int64  `json:"default_assigned_to"`
		DefaultDueBy       *string `json:"default_due_by"`
		DefaultAvailableAt *string `json:"default_available_at"`
		Enabled            *bool   `json:"enabled"`
		CooldownMinutes    *int    `json:"cooldown_minutes"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	trigger := &model.ChoreTrigger{ID: id}
	trigger.DefaultAssignedTo = req.DefaultAssignedTo
	trigger.DefaultDueBy = req.DefaultDueBy
	trigger.DefaultAvailableAt = req.DefaultAvailableAt
	if req.Enabled != nil {
		trigger.Enabled = *req.Enabled
	} else {
		trigger.Enabled = true
	}
	if req.CooldownMinutes != nil {
		trigger.CooldownMinutes = *req.CooldownMinutes
	}

	if err := h.store.UpdateChoreTrigger(r.Context(), trigger); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update trigger")
		return
	}

	writeJSON(w, http.StatusOK, trigger)
}

func (h *TriggerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid trigger id")
		return
	}
	if err := h.store.DeleteChoreTrigger(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete trigger")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// parseFlexibleTime parses a datetime string in common SQLite formats.
func parseFlexibleTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time: %s", s)
}
