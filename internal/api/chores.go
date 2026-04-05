package api

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/liftedkilt/openchore/internal/ai"
	"github.com/liftedkilt/openchore/internal/discord"
	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
	"github.com/liftedkilt/openchore/internal/webhook"
)

type ChoreHandler struct {
	store      *store.Store
	dispatcher *webhook.Dispatcher
	discord    *discord.Notifier
	reviewer   *ai.Reviewer
	ttsGen     *ai.TTSGenerator
	descGen    *ai.DescriptionGenerator
	summarizer *ai.Summarizer
}

func NewChoreHandler(s *store.Store, d *webhook.Dispatcher, dn *discord.Notifier) *ChoreHandler {
	return &ChoreHandler{store: s, dispatcher: d, discord: dn}
}

// SetAIServices sets the optional AI reviewer and TTS generator.
func (h *ChoreHandler) SetAIServices(reviewer *ai.Reviewer, ttsGen *ai.TTSGenerator) {
	h.reviewer = reviewer
	h.ttsGen = ttsGen
}

// SetAIExtras sets the optional AI description generator and summarizer.
func (h *ChoreHandler) SetAIExtras(descGen *ai.DescriptionGenerator, summarizer *ai.Summarizer) {
	h.descGen = descGen
	h.summarizer = summarizer
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
	Title              string `json:"title"`
	Description        string `json:"description"`
	Category           string `json:"category"`
	Icon               string `json:"icon"`
	PointsValue        int    `json:"points_value"`
	MissedPenaltyValue int    `json:"missed_penalty_value"`
	EstimatedMinutes   *int   `json:"estimated_minutes"`
	RequiresApproval   bool   `json:"requires_approval"`
	RequiresPhoto      bool   `json:"requires_photo"`
	PhotoSource        string `json:"photo_source"`
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
		req.Category = model.CategoryCore
	}
	if req.Category != model.CategoryRequired && req.Category != model.CategoryCore && req.Category != model.CategoryBonus {
		writeError(w, http.StatusBadRequest, "category must be required, core, or bonus")
		return
	}

	photoSource := req.PhotoSource
	if photoSource == "" {
		photoSource = model.PhotoSourceChild
	}
	if photoSource != model.PhotoSourceChild && photoSource != model.PhotoSourceExternal && photoSource != model.PhotoSourceBoth {
		writeError(w, http.StatusBadRequest, "photo_source must be child, external, or both")
		return
	}

	user := UserFromContext(r.Context())
	chore := &model.Chore{
		Title:              req.Title,
		Description:        req.Description,
		Category:           req.Category,
		Icon:               req.Icon,
		PointsValue:        req.PointsValue,
		MissedPenaltyValue: req.MissedPenaltyValue,
		EstimatedMinutes:   req.EstimatedMinutes,
		RequiresApproval:   req.RequiresApproval,
		RequiresPhoto:      req.RequiresPhoto,
		PhotoSource:        photoSource,
		Source:             "manual",
		CreatedBy:          user.ID,
	}
	if err := h.store.CreateChore(r.Context(), chore); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create chore")
		return
	}

	// Generate TTS description + audio in background if AI TTS is enabled
	if h.ttsGen != nil {
		ttsEnabled, _ := h.store.GetSetting(r.Context(), "ai_tts_enabled")
		if ttsEnabled == "true" {
			go func() {
				ctx := context.Background()
				desc, audioURL, err := h.ttsGen.GenerateAndSynthesize(ctx, chore.Title, chore.Description, chore.ID)
				if err != nil {
					log.Printf("ai: TTS generation failed for chore %d: %v", chore.ID, err)
					return
				}
				if desc != "" {
					_ = h.store.UpdateChoreTTSDescription(ctx, chore.ID, desc)
				}
				if audioURL != "" {
					_ = h.store.UpdateChoreTTSAudioURL(ctx, chore.ID, audioURL)
				}
			}()
		}
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
		if req.Category != model.CategoryRequired && req.Category != model.CategoryCore && req.Category != model.CategoryBonus {
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
	if req.MissedPenaltyValue != 0 {
		existing.MissedPenaltyValue = req.MissedPenaltyValue
	}
	if req.EstimatedMinutes != nil {
		existing.EstimatedMinutes = req.EstimatedMinutes
	}
	// Always update booleans as they might be toggled off (or we could rely on a PATCH approach, but here we just assign)
	existing.RequiresApproval = req.RequiresApproval
	existing.RequiresPhoto = req.RequiresPhoto
	if req.PhotoSource != "" {
		if req.PhotoSource != model.PhotoSourceChild && req.PhotoSource != model.PhotoSourceExternal && req.PhotoSource != model.PhotoSourceBoth {
			writeError(w, http.StatusBadRequest, "photo_source must be child, external, or both")
			return
		}
		existing.PhotoSource = req.PhotoSource
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
		req.ExpiryPenalty = model.ExpiryBlock
	}
	if req.ExpiryPenalty != model.ExpiryBlock && req.ExpiryPenalty != model.ExpiryNoPoints && req.ExpiryPenalty != model.ExpiryPenalty {
		writeError(w, http.StatusBadRequest, "expiry_penalty must be block, no_points, or penalty")
		return
	}
	if req.ExpiryPenalty == model.ExpiryPenalty && req.ExpiryPenaltyValue <= 0 {
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
	PhotoURL       string `json:"photo_url"`
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
		req.CompletionDate = time.Now().Format(model.DateFormat)
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
	if schedule.DueBy != nil && *schedule.DueBy != "" && req.CompletionDate == now.Format(model.DateFormat) {
		if nowTime > *schedule.DueBy {
			isExpired = true
		}
	}

	// Enforce expiry penalty
	if isExpired && schedule.ExpiryPenalty == model.ExpiryBlock {
		writeError(w, http.StatusUnprocessableEntity, "this chore has expired and can no longer be completed")
		return
	}

	// FCFS race condition check: if a sibling already completed this FCFS group, reject
	if schedule.AssignmentType == model.AssignmentFCFS && schedule.FcfsGroupID != nil {
		done, err := h.store.FcfsGroupCompletedForDate(r.Context(), *schedule.FcfsGroupID, req.CompletionDate)
		if err == nil && done {
			writeError(w, http.StatusConflict, "a sibling already completed this chore")
			return
		}
	}

	// Check if already completed
	existing, err := h.store.GetCompletionForScheduleDate(r.Context(), scheduleID, req.CompletionDate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check completion")
		return
	}
	if existing != nil {
		if existing.Status == model.StatusAIRejected {
			// Allow retry — delete the rejected attempt
			_ = h.store.UncompleteChore(r.Context(), scheduleID, req.CompletionDate)
		} else {
			writeError(w, http.StatusConflict, "chore already completed for this date")
			return
		}
	}

	// Fetch chore details to check category and requirements
	chore, _ := h.store.GetChore(r.Context(), schedule.ChoreID)

	// For "child" photo source, require photo at completion time.
	// For "external" or "both", photo can be attached later.
	photoSource := model.PhotoSourceChild
	if chore != nil {
		photoSource = chore.PhotoSource
		if photoSource == "" {
			photoSource = model.PhotoSourceChild
		}
	}
	if chore != nil && chore.RequiresPhoto && req.PhotoURL == "" && photoSource == model.PhotoSourceChild {
		writeError(w, http.StatusBadRequest, "a photo is required to complete this chore")
		return
	}

	// AI photo review (if enabled and photo provided)
	var aiFeedback string
	var aiConfidence float64
	if req.PhotoURL != "" && h.reviewer != nil {
		aiEnabled, _ := h.store.GetSetting(r.Context(), "ai_enabled")
		if aiEnabled == "true" {
			photoPath := req.PhotoURL
			// Convert relative URL to file path
			if len(photoPath) > 0 && photoPath[0] == '/' {
				photoPath = "data" + photoPath // /uploads/x.jpg -> data/uploads/x.jpg
			}

			thresholdStr, _ := h.store.GetSetting(r.Context(), "ai_auto_approve_threshold")
			threshold := 0.85
			if t, err := strconv.ParseFloat(thresholdStr, 64); err == nil && t > 0 {
				threshold = t
			}

			choreDesc := ""
			if chore != nil {
				choreDesc = chore.Description
			}
			result, err := h.reviewer.ReviewPhoto(r.Context(), chore.Title, choreDesc, photoPath)
			if err != nil {
				log.Printf("ai: review failed (proceeding without): %v", err)
				// Fall through to normal flow if AI is unavailable
			} else {
				aiFeedback = result.Feedback
				aiConfidence = result.Confidence

				// Reject only if the model is confident the chore is NOT done.
				// If complete=true, always approve. If complete=false but confidence
				// is below the threshold, give the kid the benefit of the doubt.
				if !result.Complete && result.Confidence >= threshold {
					// AI says not complete — save as ai_rejected with feedback
					user := UserFromContext(r.Context())
					completedBy := user.ID
					if req.CompletedBy != 0 {
						completedBy = req.CompletedBy
					}
					rejection := &model.ChoreCompletion{
						ChoreScheduleID: scheduleID,
						CompletedBy:     completedBy,
						Status:          model.StatusAIRejected,
						PhotoURL:        req.PhotoURL,
						CompletionDate:  req.CompletionDate,
						AIFeedback:      result.Feedback,
						AIConfidence:    result.Confidence,
					}
					_ = h.store.CompleteChore(r.Context(), rejection)

					// Synthesize feedback audio in background if TTS available
					var feedbackAudioURL string
					if h.ttsGen != nil {
						ttsEnabled, _ := h.store.GetSetting(r.Context(), "ai_tts_enabled")
						if ttsEnabled == "true" {
							if url, err := h.ttsGen.SynthesizeFeedback(r.Context(), result.Feedback, rejection.ID); err == nil {
								feedbackAudioURL = url
							}
						}
					}

					writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
						"error": result.Feedback,
						"ai_review": map[string]any{
							"complete":        result.Complete,
							"confidence":      result.Confidence,
							"feedback":        result.Feedback,
							"feedback_audio":  feedbackAudioURL,
						},
					})
					return
				}
			}
		}
	}

	status := model.StatusApproved
	if chore != nil && chore.RequiresApproval {
		status = model.StatusPending
	}

	user := UserFromContext(r.Context())
	completedBy := user.ID
	if req.CompletedBy != 0 {
		completedBy = req.CompletedBy
	}

	completion := &model.ChoreCompletion{
		ChoreScheduleID: scheduleID,
		CompletedBy:     completedBy,
		Status:          status,
		PhotoURL:        req.PhotoURL,
		CompletionDate:  req.CompletionDate,
		AIFeedback:      aiFeedback,
		AIConfidence:    aiConfidence,
	}
	if err := h.store.CompleteChore(r.Context(), completion); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to complete chore")
		return
	}

	var pts int
	// Only calculate points and streak if immediately approved
	if status == model.StatusApproved {
		// Credit or penalize points based on expiry status
		pts, _ = h.store.GetChorePointsForSchedule(r.Context(), scheduleID)

		// Bonus chore points only count once required + core chores are complete
		if chore != nil && chore.Category == model.CategoryBonus {
			if !h.shouldAwardBonusPoints(r.Context(), completedBy, req.CompletionDate) {
				pts = 0
			}
		}

		if isExpired {
			switch schedule.ExpiryPenalty {
			case model.ExpiryNoPoints:
				pts = 0
			case model.ExpiryPenalty:
				pts = 0
				if err := h.store.DebitExpiryPenalty(r.Context(), completedBy, completion.ID, schedule.ExpiryPenaltyValue); err != nil {
					log.Printf("error debiting expiry penalty for user %d completion %d: %v", completedBy, completion.ID, err)
				}
			}
		}
		if pts > 0 {
			if err := h.store.CreditChorePoints(r.Context(), completedBy, completion.ID, pts); err != nil {
				log.Printf("error crediting chore points for user %d completion %d: %v", completedBy, completion.ID, err)
			}
		}

		// Recalculate streak
		if err := h.store.RecalculateStreak(r.Context(), completedBy, req.CompletionDate); err != nil {
			log.Printf("error recalculating streak for user %d: %v", completedBy, err)
		}
	}

	// FCFS: complete sibling schedules with shadow completions
	if schedule.AssignmentType == model.AssignmentFCFS && schedule.FcfsGroupID != nil && status == model.StatusApproved {
		if err := h.store.CompleteFCFSSiblings(r.Context(), *schedule.FcfsGroupID, completedBy, scheduleID, req.CompletionDate); err != nil {
			log.Printf("error completing FCFS siblings: %v", err)
		}

		// Fire FCFS-specific webhook
		completedByUser, _ := h.store.GetUser(r.Context(), completedBy)
		fcfsName := ""
		if completedByUser != nil {
			fcfsName = completedByUser.Name
		}
		fcfsTitle := ""
		if chore != nil {
			fcfsTitle = chore.Title
		}
		h.dispatcher.Fire(webhook.EventChoreFCFSCompleted, map[string]any{
			"completion_id":   completion.ID,
			"schedule_id":     scheduleID,
			"fcfs_group_id":   *schedule.FcfsGroupID,
			"chore_title":     fcfsTitle,
			"user_id":         completedBy,
			"user_name":       fcfsName,
			"completion_date": req.CompletionDate,
			"points_earned":   pts,
		})
	}

	// Fire webhook
	choreTitle := ""
	if chore != nil {
		choreTitle = chore.Title
	}
	completedByUser, _ := h.store.GetUser(r.Context(), completedBy)
	completedByName := ""
	if completedByUser != nil {
		completedByName = completedByUser.Name
	}

	// Determine absolute photo URL for webhooks
	absolutePhotoURL := req.PhotoURL
	if req.PhotoURL != "" {
		baseURL, _ := h.store.GetSetting(r.Context(), "base_url")
		if baseURL != "" {
			absolutePhotoURL = baseURL + req.PhotoURL
		}
	}

	h.dispatcher.Fire(webhook.EventChoreCompleted, map[string]any{
		"completion_id":   completion.ID,
		"schedule_id":     scheduleID,
		"chore_title":     choreTitle,
		"user_id":         completedBy,
		"user_name":       completedByName,
		"completion_date": req.CompletionDate,
		"points_earned":   pts,
		"status":          status,
		"photo_url":       absolutePhotoURL,
		"photo_source":    photoSource,
	})

	// Discord notification (non-blocking)
	if status == model.StatusPending {
		h.discord.NotifyPendingApproval(completedByName, choreTitle, absolutePhotoURL)
	} else {
		h.discord.NotifyCompleted(completedByName, choreTitle, absolutePhotoURL, pts)
	}

	// Check if all chores for today are done (only if this one was approved)
	if status == model.StatusApproved {
		go func() {
			todayChores, err := h.store.GetScheduledChoresForUser(context.Background(), completedBy, []string{req.CompletionDate}, time.Now())
			if err == nil {
				allDone := len(todayChores) > 0
				for _, c := range todayChores {
					if !c.Completed && c.Category != model.CategoryBonus {
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
	}

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
		dateStr = time.Now().Format(model.DateFormat)
	}

	// Get the schedule to check for FCFS
	schedule, _ := h.store.GetSchedule(r.Context(), scheduleID)

	// Get completion before deleting so we can reverse points
	existing, _ := h.store.GetCompletionForScheduleDate(r.Context(), scheduleID, dateStr)
	var completedBy int64
	if existing != nil {
		completedBy = existing.CompletedBy
		// Reverse the actual net points for this completion (handles normal credit and penalty debits)
		net, err := h.store.GetNetPointsForCompletion(r.Context(), existing.ID)
		if err == nil && net != 0 {
			if err := h.store.DebitChorePoints(r.Context(), existing.CompletedBy, existing.ID, net); err != nil {
				log.Printf("error debiting chore points for user %d completion %d: %v", existing.CompletedBy, existing.ID, err)
			}
		}
	}

	// FCFS: uncomplete all siblings in the group
	if schedule != nil && schedule.AssignmentType == model.AssignmentFCFS && schedule.FcfsGroupID != nil {
		if err := h.store.UncompleteByFCFSGroup(r.Context(), *schedule.FcfsGroupID, dateStr); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to uncomplete FCFS group")
			return
		}
	} else {
		if err := h.store.UncompleteChore(r.Context(), scheduleID, dateStr); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to uncomplete chore")
			return
		}
	}

	// Recalculate streak
	if completedBy > 0 {
		if err := h.store.RecalculateStreak(r.Context(), completedBy, dateStr); err != nil {
			log.Printf("error recalculating streak for user %d: %v", completedBy, err)
		}
	}

	// Fire webhook
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

// --- Approvals ---

func (h *ChoreHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	pending, err := h.store.ListPendingCompletions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list pending completions")
		return
	}
	if pending == nil {
		pending = []store.PendingCompletionRow{}
	}
	writeJSON(w, http.StatusOK, pending)
}

func (h *ChoreHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid completion id")
		return
	}

	completion, err := h.store.GetCompletion(r.Context(), id)
	if err != nil || completion == nil {
		writeError(w, http.StatusNotFound, "completion not found")
		return
	}

	if completion.Status != model.StatusPending {
		writeError(w, http.StatusBadRequest, "completion is not pending")
		return
	}

	admin := UserFromContext(r.Context())
	if err := h.store.UpdateCompletionStatus(r.Context(), id, model.StatusApproved, admin.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to approve")
		return
	}

	// Calculate and award points now that it's approved
	schedule, _ := h.store.GetSchedule(r.Context(), completion.ChoreScheduleID)
	var pts int
	if schedule != nil {
		pts, _ = h.store.GetChorePointsForSchedule(r.Context(), schedule.ID)
		chore, _ := h.store.GetChore(r.Context(), schedule.ChoreID)

		// Bonus logic
		if chore != nil && chore.Category == model.CategoryBonus {
			if !h.shouldAwardBonusPoints(r.Context(), completion.CompletedBy, completion.CompletionDate) {
				pts = 0
			}
		}

		if pts > 0 {
			if err := h.store.CreditChorePoints(r.Context(), completion.CompletedBy, completion.ID, pts); err != nil {
				log.Printf("error crediting chore points for user %d completion %d: %v", completion.CompletedBy, completion.ID, err)
			}
		}
	}

	// Recalculate streak
	if err := h.store.RecalculateStreak(r.Context(), completion.CompletedBy, completion.CompletionDate); err != nil {
		log.Printf("error recalculating streak for user %d: %v", completion.CompletedBy, err)
	}

	// Discord notification for approval
	{
		userName := ""
		if u, _ := h.store.GetUser(r.Context(), completion.CompletedBy); u != nil {
			userName = u.Name
		}
		choreTitle := ""
		if schedule != nil {
			if c, _ := h.store.GetChore(r.Context(), schedule.ChoreID); c != nil {
				choreTitle = c.Title
			}
		}
		h.discord.NotifyApproved(userName, choreTitle)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ChoreHandler) Reject(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid completion id")
		return
	}

	completion, err := h.store.GetCompletion(r.Context(), id)
	if err != nil || completion == nil {
		writeError(w, http.StatusNotFound, "completion not found")
		return
	}

	if completion.Status != model.StatusPending {
		writeError(w, http.StatusBadRequest, "completion is not pending")
		return
	}

	admin := UserFromContext(r.Context())
	if err := h.store.UpdateCompletionStatus(r.Context(), id, model.StatusRejected, admin.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reject")
		return
	}

	// Discord notification for rejection
	{
		userName := ""
		if u, _ := h.store.GetUser(r.Context(), completion.CompletedBy); u != nil {
			userName = u.Name
		}
		choreTitle := ""
		if schedule, _ := h.store.GetSchedule(r.Context(), completion.ChoreScheduleID); schedule != nil {
			if c, _ := h.store.GetChore(r.Context(), schedule.ChoreID); c != nil {
				choreTitle = c.Title
			}
		}
		h.discord.NotifyRejected(userName, choreTitle)
	}

	w.WriteHeader(http.StatusNoContent)
}

// AttachPhoto allows attaching or replacing a photo on a pending completion.
// This is used by external systems (e.g. Home Assistant) to provide photo proof
// after a chore has been marked complete.
func (h *ChoreHandler) AttachPhoto(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid completion id")
		return
	}

	completion, err := h.store.GetCompletion(r.Context(), id)
	if err != nil || completion == nil {
		writeError(w, http.StatusNotFound, "completion not found")
		return
	}

	if completion.Status != model.StatusPending {
		writeError(w, http.StatusBadRequest, "completion is not pending")
		return
	}

	var req struct {
		PhotoURL string `json:"photo_url"`
	}
	if err := decodeJSON(r, &req); err != nil || req.PhotoURL == "" {
		writeError(w, http.StatusBadRequest, "photo_url is required")
		return
	}

	if err := h.store.UpdateCompletionPhoto(r.Context(), id, req.PhotoURL); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to attach photo")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":        id,
		"photo_url": req.PhotoURL,
	})
}

// TestAIReview lets admins test the AI photo review with a dummy chore name and photo.
func (h *ChoreHandler) TestAIReview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChoreTitle string `json:"chore_title"`
		PhotoURL   string `json:"photo_url"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ChoreTitle == "" || req.PhotoURL == "" {
		writeError(w, http.StatusBadRequest, "chore_title and photo_url are required")
		return
	}

	if h.reviewer == nil {
		writeError(w, http.StatusServiceUnavailable, "AI services not available")
		return
	}

	photoPath := req.PhotoURL
	if len(photoPath) > 0 && photoPath[0] == '/' {
		photoPath = "data" + photoPath
	}

	t0 := time.Now()
	result, err := h.reviewer.ReviewPhoto(r.Context(), req.ChoreTitle, "", photoPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "AI review failed: "+err.Error())
		return
	}
	log.Printf("ai: photo review took %s", time.Since(t0))

	// Synthesize feedback audio if TTS is available
	var feedbackAudioURL string
	if h.ttsGen != nil {
		ttsEnabled, _ := h.store.GetSetting(r.Context(), "ai_tts_enabled")
		if ttsEnabled == "true" {
			t1 := time.Now()
			url, err := h.ttsGen.SynthesizeFeedback(r.Context(), result.Feedback, 0)
			if err != nil {
				log.Printf("ai: TTS synthesis failed for chore checker (%s): %v", time.Since(t1), err)
			} else {
				feedbackAudioURL = url
				log.Printf("ai: TTS synthesis took %s", time.Since(t1))
			}
		} else {
			log.Printf("ai: TTS disabled in settings (ai_tts_enabled=%q)", ttsEnabled)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"complete":       result.Complete,
		"confidence":     result.Confidence,
		"feedback":       result.Feedback,
		"feedback_audio": feedbackAudioURL,
	})
}

// SynthesizeTTS lets the admin retry TTS audio synthesis for given feedback text.
func (h *ChoreHandler) SynthesizeTTS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Text == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}

	if h.ttsGen == nil {
		writeError(w, http.StatusServiceUnavailable, "AI services not available")
		return
	}

	url, err := h.ttsGen.SynthesizeFeedback(r.Context(), req.Text, 0)
	if err != nil {
		log.Printf("ai: TTS synthesis failed: %v", err)
		writeError(w, http.StatusServiceUnavailable, "TTS synthesis failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"audio_url": url})
}

// GenerateDescription lets admins generate a chore description using AI.
func (h *ChoreHandler) GenerateDescription(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title    string `json:"title"`
		Category string `json:"category"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	if h.descGen == nil {
		writeError(w, http.StatusServiceUnavailable, "AI services not available")
		return
	}

	desc, err := h.descGen.GenerateDescription(r.Context(), req.Title, req.Category)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "AI generation failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"description": desc})
}

// SuggestPoints lets admins get AI-recommended point values for a chore.
func (h *ChoreHandler) SuggestPoints(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Category    string `json:"category"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	if h.descGen == nil {
		writeError(w, http.StatusServiceUnavailable, "AI services not available")
		return
	}

	points, minutes, reasoning, err := h.descGen.SuggestPoints(r.Context(), req.Title, req.Description, req.Category)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "AI suggestion failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"points":            points,
		"estimated_minutes": minutes,
		"reasoning":         reasoning,
	})
}

// shouldAwardBonusPoints returns true if all required and core chores for the
// given user and date are complete, meaning bonus points should be awarded.
func (h *ChoreHandler) shouldAwardBonusPoints(ctx context.Context, userID int64, date string) bool {
	todayChores, err := h.store.GetScheduledChoresForUser(ctx, userID, []string{date}, time.Now())
	if err != nil {
		return false
	}
	for _, c := range todayChores {
		if !c.Completed && (c.Category == model.CategoryRequired || c.Category == model.CategoryCore) {
			return false
		}
	}
	return true
}
