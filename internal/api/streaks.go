package api

import (
	"net/http"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

type StreakHandler struct {
	store *store.Store
}

func NewStreakHandler(s *store.Store) *StreakHandler {
	return &StreakHandler{store: s}
}

func (h *StreakHandler) GetUserStreak(w http.ResponseWriter, r *http.Request) {
	userID, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	streak, err := h.store.GetUserStreak(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get streak")
		return
	}
	rewards, err := h.store.ListStreakRewards(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get streak rewards")
		return
	}

	// Compute earned and next reward
	var earned []model.StreakReward
	var next *struct {
		model.StreakReward
		DaysRemaining int `json:"days_remaining"`
	}
	for _, rw := range rewards {
		if rw.StreakDays <= streak.CurrentStreak {
			earned = append(earned, rw)
		} else if next == nil {
			next = &struct {
				model.StreakReward
				DaysRemaining int `json:"days_remaining"`
			}{rw, rw.StreakDays - streak.CurrentStreak}
		}
	}

	resp := map[string]any{
		"current_streak":     streak.CurrentStreak,
		"longest_streak":     streak.LongestStreak,
		"streak_start_date":  streak.StreakStartDate,
		"earned_rewards":     earned,
	}
	if next != nil {
		resp["next_reward"] = map[string]any{
			"streak_days":    next.StreakDays,
			"bonus_points":   next.BonusPoints,
			"label":          next.Label,
			"days_remaining": next.DaysRemaining,
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *StreakHandler) ListRewards(w http.ResponseWriter, r *http.Request) {
	rewards, err := h.store.ListStreakRewards(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list streak rewards")
		return
	}
	if rewards == nil {
		rewards = []model.StreakReward{}
	}
	writeJSON(w, http.StatusOK, rewards)
}

func (h *StreakHandler) CreateReward(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StreakDays  int    `json:"streak_days"`
		BonusPoints int   `json:"bonus_points"`
		Label      string `json:"label"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.StreakDays <= 0 || req.BonusPoints <= 0 {
		writeError(w, http.StatusBadRequest, "streak_days and bonus_points must be positive")
		return
	}
	reward := &model.StreakReward{
		StreakDays:  req.StreakDays,
		BonusPoints: req.BonusPoints,
		Label:      req.Label,
	}
	if err := h.store.CreateStreakReward(r.Context(), reward); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create streak reward")
		return
	}
	writeJSON(w, http.StatusCreated, reward)
}

func (h *StreakHandler) DeleteReward(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.store.DeleteStreakReward(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete streak reward")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
