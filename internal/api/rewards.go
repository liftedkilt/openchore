package api

import (
	"net/http"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
	"github.com/liftedkilt/openchore/internal/webhook"
)

type RewardHandler struct {
	store      *store.Store
	dispatcher *webhook.Dispatcher
}

func NewRewardHandler(s *store.Store, d *webhook.Dispatcher) *RewardHandler {
	return &RewardHandler{store: s, dispatcher: d}
}

func (h *RewardHandler) List(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	rewards, err := h.store.ListRewardsForUser(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rewards")
		return
	}
	if rewards == nil {
		rewards = []model.Reward{}
	}
	writeJSON(w, http.StatusOK, rewards)
}

func (h *RewardHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	rewards, err := h.store.ListRewardsWithAssignments(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rewards")
		return
	}
	if rewards == nil {
		rewards = []model.Reward{}
	}
	writeJSON(w, http.StatusOK, rewards)
}

func (h *RewardHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
		Cost        int    `json:"cost"`
		Stock       *int   `json:"stock"`
		Shareable   bool   `json:"shareable"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Cost <= 0 {
		writeError(w, http.StatusBadRequest, "name and positive cost required")
		return
	}
	user := UserFromContext(r.Context())
	reward := &model.Reward{
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		Cost:        req.Cost,
		Stock:       req.Stock,
		Active:      true,
		Shareable:   req.Shareable,
		CreatedBy:   user.ID,
	}
	if err := h.store.CreateReward(r.Context(), reward); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create reward")
		return
	}
	writeJSON(w, http.StatusCreated, reward)
}

func (h *RewardHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid reward id")
		return
	}
	existing, err := h.store.GetReward(r.Context(), id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "reward not found")
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Icon        *string `json:"icon"`
		Cost        *int    `json:"cost"`
		Stock       *int    `json:"stock"`
		Active      *bool   `json:"active"`
		Shareable   *bool   `json:"shareable"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.Icon != nil {
		existing.Icon = *req.Icon
	}
	if req.Cost != nil {
		existing.Cost = *req.Cost
	}
	if req.Stock != nil {
		existing.Stock = req.Stock
	}
	if req.Active != nil {
		existing.Active = *req.Active
	}
	if req.Shareable != nil {
		existing.Shareable = *req.Shareable
	}
	if err := h.store.UpdateReward(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update reward")
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (h *RewardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid reward id")
		return
	}
	if err := h.store.DeleteReward(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete reward")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RewardHandler) SetAssignments(w http.ResponseWriter, r *http.Request) {
	rewardID, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid reward id")
		return
	}
	var req struct {
		Assignments []struct {
			UserID     int64 `json:"user_id"`
			CustomCost *int  `json:"custom_cost"`
		} `json:"assignments"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	assignments := make([]model.RewardAssignment, len(req.Assignments))
	for i, a := range req.Assignments {
		assignments[i] = model.RewardAssignment{
			RewardID:   rewardID,
			UserID:     a.UserID,
			CustomCost: a.CustomCost,
		}
	}

	if err := h.store.SetRewardAssignments(r.Context(), rewardID, assignments); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update assignments")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RewardHandler) Redeem(w http.ResponseWriter, r *http.Request) {
	rewardID, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid reward id")
		return
	}
	user := UserFromContext(r.Context())
	redemption, err := h.store.RedeemReward(r.Context(), user.ID, rewardID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// Fire webhook
	reward, _ := h.store.GetReward(r.Context(), rewardID)
	rewardName := ""
	if reward != nil {
		rewardName = reward.Name
	}
	h.dispatcher.Fire(webhook.EventRewardRedeemed, map[string]any{
		"user_id":      user.ID,
		"user_name":    user.Name,
		"reward_id":    rewardID,
		"reward_name":  rewardName,
		"points_spent": redemption.PointsSpent,
	})

	writeJSON(w, http.StatusCreated, redemption)
}

func (h *RewardHandler) UndoRedemption(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "redemptionID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid redemption id")
		return
	}
	if err := h.store.UndoRedemption(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to undo redemption")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RewardHandler) ListRedemptions(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	redemptions, err := h.store.ListRedemptionsForUser(r.Context(), id, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list redemptions")
		return
	}
	if redemptions == nil {
		redemptions = []store.RedemptionHistoryRow{}
	}
	writeJSON(w, http.StatusOK, redemptions)
}

// --- Commitments ---

func (h *RewardHandler) ListCommitments(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	commitments, err := h.store.ListCommitmentsForUser(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list commitments")
		return
	}
	if commitments == nil {
		commitments = []model.RewardCommitment{}
	}
	writeJSON(w, http.StatusOK, commitments)
}

func (h *RewardHandler) Commit(w http.ResponseWriter, r *http.Request) {
	rewardID, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid reward id")
		return
	}
	var req struct {
		AutoContributePercent int `json:"auto_contribute_percent"`
	}
	if err := decodeJSON(r, &req); err != nil {
		// Body is optional; default to 0%.
		req.AutoContributePercent = 0
	}
	user := UserFromContext(r.Context())
	c, err := h.store.CreateCommitment(r.Context(), user.ID, rewardID, req.AutoContributePercent)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (h *RewardHandler) Contribute(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid commitment id")
		return
	}
	var req struct {
		Amount int `json:"amount"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "amount must be positive")
		return
	}
	user := UserFromContext(r.Context())
	if err := h.store.ContributeToCommitment(r.Context(), user.ID, id, req.Amount); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	c, err := h.store.GetCommitment(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reload commitment")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *RewardHandler) SetAutoContribute(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid commitment id")
		return
	}
	var req struct {
		Percent int `json:"percent"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user := UserFromContext(r.Context())
	if err := h.store.SetCommitmentAutoContributePercent(r.Context(), user.ID, id, req.Percent); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	c, err := h.store.GetCommitment(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reload commitment")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *RewardHandler) BreakCommitment(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid commitment id")
		return
	}
	user := UserFromContext(r.Context())
	if err := h.store.BreakCommitment(r.Context(), user.ID, id); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetSharedPool returns the up-to-date pool details: target, total saved,
// and per-contributor breakdown for the leaderboard. Used by kids' clients
// to refresh after a sibling adds points.
func (h *RewardHandler) GetSharedPool(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid pool id")
		return
	}
	pool, err := h.store.GetSharedPool(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load pool")
		return
	}
	if pool == nil {
		writeError(w, http.StatusNotFound, "pool not found")
		return
	}
	writeJSON(w, http.StatusOK, pool)
}
