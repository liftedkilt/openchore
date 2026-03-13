package api

import (
	"net/http"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
	"github.com/liftedkilt/openchore/internal/webhook"
)

type ChoreHandler struct {
	store      *store.Store
	dispatcher *webhook.Dispatcher
}

func NewChoreHandler(s *store.Store, d *webhook.Dispatcher) *ChoreHandler {
	return &ChoreHandler{store: s, dispatcher: d}
}

func (h *ChoreHandler) List(w http.ResponseWriter, r *http.Request) {
	chores, err := h.store.ListChores(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list chores")
		return
	}
	if chores == nil {
		chores = []model.Chore{}
	}
	writeJSON(w, http.StatusOK, chores)
}

func (h *ChoreHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid chore id")
		return
	}
	chore, err := h.store.GetChore(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get chore")
		return
	}
	if chore == nil {
		writeError(w, http.StatusNotFound, "chore not found")
		return
	}
	writeJSON(w, http.StatusOK, chore)
}

type createChoreRequest struct {
	Title            string `json:"title"`
	Description      string `json:"description"`
	Category         string `json:"category"`
	Icon             string `json:"icon"`
	PointsValue      int    `json:"points_value"`
	EstimatedMinutes *int   `json:"estimated_minutes"`
}

func (h *ChoreHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createChoreRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if req.Category == "" {
		req.Category = "core"
	}
	if req.Category != "required" && req.Category != "core" && req.Category != "bonus" {
		writeError(w, http.StatusBadRequest, "category must be required, core, or bonus")
		return
	}

	user := UserFromContext(r.Context())
	chore := &model.Chore{
		Title:            req.Title,
		Description:      req.Description,
		Category:         req.Category,
		Icon:             req.Icon,
		PointsValue:      req.PointsValue,
		EstimatedMinutes: req.EstimatedMinutes,
		Source:           "manual",
		CreatedBy:        user.ID,
	}
	if err := h.store.CreateChore(r.Context(), chore); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create chore")
		return
	}
	writeJSON(w, http.StatusCreated, chore)
}

func (h *ChoreHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid chore id")
		return
	}
	existing, err := h.store.GetChore(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get chore")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "chore not found")
		return
	}

	var req createChoreRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title != "" {
		existing.Title = req.Title
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	if req.Category != "" {
		if req.Category != "required" && req.Category != "core" && req.Category != "bonus" {
			writeError(w, http.StatusBadRequest, "category must be required, core, or bonus")
			return
		}
		existing.Category = req.Category
	}
	if req.Icon != "" {
		existing.Icon = req.Icon
	}
	if req.PointsValue != 0 {
		existing.PointsValue = req.PointsValue
	}
	if req.EstimatedMinutes != nil {
		existing.EstimatedMinutes = req.EstimatedMinutes
	}

	if err := h.store.UpdateChore(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update chore")
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (h *ChoreHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid chore id")
		return
	}
	if err := h.store.DeleteChore(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete chore")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Schedules ---

type createScheduleRequest struct {
	AssignedTo         int64   `json:"assigned_to"`
	AssignmentType     string  `json:"assignment_type"`
	DayOfWeek          *int    `json:"day_of_week"`
	SpecificDate       *string `json:"specific_date"`
	AvailableAt        *string `json:"available_at"`
	DueBy              *string `json:"due_by"`
	ExpiryPenalty      string  `json:"expiry_penalty"`
	ExpiryPenaltyValue int     `json:"expiry_penalty_value"`
	PointsMultiplier   float64 `json:"points_multiplier"`
	StartDate          *string `json:"start_date"`
	EndDate            *string `json:"end_date"`
	RecurrenceInterval *int    `json:"recurrence_interval"`
	RecurrenceStart    *string `json:"recurrence_start"`
}

func (h *ChoreHandler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	choreID, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid chore id")
		return
	}
	chore, err := h.store.GetChore(r.Context(), choreID)
	if err != nil || chore == nil {
		writeError(w, http.StatusNotFound, "chore not found")
		return
	}

	var req createScheduleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.AssignedTo == 0 {
		writeError(w, http.StatusBadRequest, "assigned_to is required")
		return
	}
	if req.RecurrenceInterval != nil {
		if *req.RecurrenceInterval < 1 {
			writeError(w, http.StatusBadRequest, "recurrence_interval must be >= 1")
			return
		}
		if req.RecurrenceStart == nil {
			writeError(w, http.StatusBadRequest, "recurrence_start is required with recurrence_interval")
			return
		}
	} else if req.DayOfWeek == nil && req.SpecificDate == nil {
		writeError(w, http.StatusBadRequest, "day_of_week, specific_date, or recurrence_interval is required")
		return
	}
	if req.AssignmentType == "" {
		req.AssignmentType = "individual"
	}
	if req.PointsMultiplier == 0 {
		req.PointsMultiplier = 1.0
	}
	if req.ExpiryPenalty == "" {
		req.ExpiryPenalty = "block"
	}
	if req.ExpiryPenalty != "block" && req.ExpiryPenalty != "no_points" && req.ExpiryPenalty != "penalty" {
		writeError(w, http.StatusBadRequest, "expiry_penalty must be block, no_points, or penalty")
		return
	}
	if req.ExpiryPenalty == "penalty" && req.ExpiryPenaltyValue <= 0 {
		writeError(w, http.StatusBadRequest, "expiry_penalty_value must be positive for penalty mode")
		return
	}

	schedule := &model.ChoreSchedule{
		ChoreID:            choreID,
		AssignedTo:         req.AssignedTo,
		AssignmentType:     req.AssignmentType,
		DayOfWeek:          req.DayOfWeek,
		SpecificDate:       req.SpecificDate,
		AvailableAt:        req.AvailableAt,
		DueBy:              req.DueBy,
		ExpiryPenalty:      req.ExpiryPenalty,
		ExpiryPenaltyValue: req.ExpiryPenaltyValue,
		PointsMultiplier:   req.PointsMultiplier,
		StartDate:          req.StartDate,
		EndDate:            req.EndDate,
		RecurrenceInterval: req.RecurrenceInterval,
		RecurrenceStart:    req.RecurrenceStart,
	}
	if err := h.store.CreateSchedule(r.Context(), schedule); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create schedule")
		return
	}
	writeJSON(w, http.StatusCreated, schedule)
}

func (h *ChoreHandler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	choreID, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid chore id")
		return
	}
	schedules, err := h.store.ListSchedulesForChore(r.Context(), choreID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list schedules")
		return
	}
	if schedules == nil {
		schedules = []model.ChoreSchedule{}
	}
	writeJSON(w, http.StatusOK, schedules)
}

func (h *ChoreHandler) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	scheduleID, err := urlParamInt64(r, "scheduleID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule id")
		return
	}
	if err := h.store.DeleteSchedule(r.Context(), scheduleID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete schedule")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Completions ---

type completeChoreRequest struct {
	CompletedBy    int64  `json:"completed_by"`
	CompletionDate string `json:"completion_date"`
}

func (h *ChoreHandler) Complete(w http.ResponseWriter, r *http.Request) {
	scheduleID, err := urlParamInt64(r, "scheduleID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule id")
		return
	}

	var req completeChoreRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.CompletionDate == "" {
		req.CompletionDate = time.Now().Format("2006-01-02")
	}

	// Get the schedule to check time lock
	schedule, err := h.store.GetSchedule(r.Context(), scheduleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get schedule")
		return
	}
	if schedule == nil {
		writeError(w, http.StatusNotFound, "schedule not found")
		return
	}

	// Enforce time lock
	now := time.Now()
	nowTime := now.Format("15:04")
	if schedule.AvailableAt != nil && *schedule.AvailableAt != "" {
		if nowTime < *schedule.AvailableAt {
			writeError(w, http.StatusUnprocessableEntity, "this chore isn't available until "+*schedule.AvailableAt)
			return
		}
	}

	// Check expiry
	isExpired := false
	if schedule.DueBy != nil && *schedule.DueBy != "" && req.CompletionDate == now.Format("2006-01-02") {
		if nowTime > *schedule.DueBy {
			isExpired = true
		}
	}

	// Enforce expiry penalty
	if isExpired && schedule.ExpiryPenalty == "block" {
		writeError(w, http.StatusUnprocessableEntity, "this chore has expired and can no longer be completed")
		return
	}

	// Check if already completed
	existing, err := h.store.GetCompletionForScheduleDate(r.Context(), scheduleID, req.CompletionDate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check completion")
		return
	}
	if existing != nil {
		writeError(w, http.StatusConflict, "chore already completed for this date")
		return
	}

	user := UserFromContext(r.Context())
	completedBy := user.ID
	if req.CompletedBy != 0 {
		completedBy = req.CompletedBy
	}

	completion := &model.ChoreCompletion{
		ChoreScheduleID: scheduleID,
		CompletedBy:     completedBy,
		Status:          "approved",
		CompletionDate:  req.CompletionDate,
	}
	if err := h.store.CompleteChore(r.Context(), completion); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to complete chore")
		return
	}

	// Credit or penalize points based on expiry status
	pts, _ := h.store.GetChorePointsForSchedule(r.Context(), scheduleID)
	if isExpired {
		switch schedule.ExpiryPenalty {
		case "no_points":
			pts = 0
		case "penalty":
			pts = 0
			_ = h.store.DebitExpiryPenalty(r.Context(), completedBy, completion.ID, schedule.ExpiryPenaltyValue)
		}
	}
	if pts > 0 {
		_ = h.store.CreditChorePoints(r.Context(), completedBy, completion.ID, pts)
	}

	// Recalculate streak
	_ = h.store.RecalculateStreak(r.Context(), completedBy, req.CompletionDate)

	// Fire webhook
	chore, _ := h.store.GetChore(r.Context(), schedule.ChoreID)
	choreTitle := ""
	if chore != nil {
		choreTitle = chore.Title
	}
	completedByUser, _ := h.store.GetUser(r.Context(), completedBy)
	completedByName := ""
	if completedByUser != nil {
		completedByName = completedByUser.Name
	}
	h.dispatcher.Fire(webhook.EventChoreCompleted, map[string]any{
		"schedule_id":     scheduleID,
		"chore_title":     choreTitle,
		"user_id":         completedBy,
		"user_name":       completedByName,
		"completion_date": req.CompletionDate,
		"points_earned":   pts,
	})

	// Check if all chores for today are done
	go func() {
		todayChores, err := h.store.GetScheduledChoresForUser(r.Context(), completedBy, []string{req.CompletionDate}, time.Now())
		if err == nil {
			allDone := len(todayChores) > 0
			for _, c := range todayChores {
				if !c.Completed && c.Category != "bonus" {
					allDone = false
					break
				}
			}
			if allDone {
				h.dispatcher.Fire(webhook.EventDailyComplete, map[string]any{
					"user_id":   completedBy,
					"user_name": completedByName,
					"date":      req.CompletionDate,
				})
			}
		}
	}()

	writeJSON(w, http.StatusCreated, completion)
}

func (h *ChoreHandler) Uncomplete(w http.ResponseWriter, r *http.Request) {
	scheduleID, err := urlParamInt64(r, "scheduleID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule id")
		return
	}
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}

	// Get completion before deleting so we can reverse points
	existing, _ := h.store.GetCompletionForScheduleDate(r.Context(), scheduleID, dateStr)
	var completedBy int64
	if existing != nil {
		completedBy = existing.CompletedBy
		// Reverse the actual net points for this completion (handles normal credit and penalty debits)
		net, err := h.store.GetNetPointsForCompletion(r.Context(), existing.ID)
		if err == nil && net != 0 {
			_ = h.store.DebitChorePoints(r.Context(), existing.CompletedBy, existing.ID, net)
		}
	}

	if err := h.store.UncompleteChore(r.Context(), scheduleID, dateStr); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to uncomplete chore")
		return
	}

	// Recalculate streak
	if completedBy > 0 {
		_ = h.store.RecalculateStreak(r.Context(), completedBy, dateStr)
	}

	// Fire webhook
	schedule, _ := h.store.GetSchedule(r.Context(), scheduleID)
	choreTitle := ""
	if schedule != nil {
		chore, _ := h.store.GetChore(r.Context(), schedule.ChoreID)
		if chore != nil {
			choreTitle = chore.Title
		}
	}
	uncompleteUser, _ := h.store.GetUser(r.Context(), completedBy)
	uncompleteUserName := ""
	if uncompleteUser != nil {
		uncompleteUserName = uncompleteUser.Name
	}
	h.dispatcher.Fire(webhook.EventChoreUncompleted, map[string]any{
		"schedule_id": scheduleID,
		"chore_title": choreTitle,
		"user_id":     completedBy,
		"user_name":   uncompleteUserName,
		"date":        dateStr,
	})

	w.WriteHeader(http.StatusNoContent)
}
