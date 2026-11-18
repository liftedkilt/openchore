package api

import (
	"log"
	"net/http"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
	"github.com/liftedkilt/openchore/internal/webhook"
)

type RewardHandler struct {
	store      *store.Store
	dispatcher *webhook.Dispatcher
	pets       *PetHandler
}

func NewRewardHandler(s *store.Store, d *webhook.Dispatcher, pets *PetHandler) *RewardHandler {
	return &RewardHandler{store: s, dispatcher: d, pets: pets}
}

// validatePetReward checks a reward's pet-care fields.
func validatePetReward(rw *model.Reward) string {
	if !model.ValidPetAction(rw.PetAction) {
		return "unknown pet_action"
	}
	if rw.PetAction == model.PetActionAccessory {
		if !model.ValidPetItem(rw.PetItem) {
			return "unknown pet_item"
		}
	} else {
		rw.PetItem = ""
	}
	if rw.PetAction != "" && rw.Shareable {
		return "pet-care rewards can't be shared"
	}
	return ""
}

func (h *RewardHandler) List(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	rewards, err := h.store.ListRewardsForUser(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rewards")
		return
	}
	// Pet-care rewards only show while farm pets are on.
	if !petsEnabled(r.Context(), h.store) {
		kept := rewards[:0]
		for _, rw := range rewards {
			if rw.PetAction == "" {
				kept = append(kept, rw)
			}
		}
		rewards = kept
	}
	// Gifts and playdates only show when there's a friend family's animal to send them to.
	if h.pets == nil || h.pets.Friends == nil || !h.pets.Friends.HasFriendAnimals(r.Context()) {
		kept := rewards[:0]
		for _, rw := range rewards {
			if rw.PetAction != model.PetActionGift && rw.PetAction != model.PetActionPlaydate {
				kept = append(kept, rw)
			}
		}
		rewards = kept
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
		PetAction   string `json:"pet_action"`
		PetItem     string `json:"pet_item"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Cost <= 0 {
		writeError(w, http.StatusBadRequest, "name and positive cost required")
		return
	}
	createdBy, err := recordedBy(r.Context(), h.store, UserFromContext(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create reward")
		return
	}
	reward := &model.Reward{
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		Cost:        req.Cost,
		Stock:       req.Stock,
		Active:      true,
		Shareable:   req.Shareable,
		PetAction:   req.PetAction,
		PetItem:     req.PetItem,
		CreatedBy:   createdBy,
	}
	if msg := validatePetReward(reward); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
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
		PetAction   *string `json:"pet_action"`
		PetItem     *string `json:"pet_item"`
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
	if req.PetAction != nil {
		existing.PetAction = *req.PetAction
	}
	if req.PetItem != nil {
		existing.PetItem = *req.PetItem
	}
	if msg := validatePetReward(existing); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
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
	if user == nil {
		writeError(w, http.StatusForbidden, "sign in to a profile first")
		return
	}
	// Pet care: refuse before any points move if it can't take effect
	// (no pet, hiding, too sick to play, accessory already owned).
	petReward, err := h.store.GetReward(r.Context(), rewardID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load reward")
		return
	}
	if petReward != nil && petReward.PetAction == "" {
		petReward = nil
	}
	// New animal / restyle carry the person's pick in the body.
	var choice *PetChoice
	if petReward != nil && (petReward.PetAction == model.PetActionNewAnimal || petReward.PetAction == model.PetActionRestyle || petReward.PetAction == model.PetActionDesign || petReward.PetAction == model.PetActionPicture ||
		petReward.PetAction == model.PetActionGift || petReward.PetAction == model.PetActionPlaydate) {
		r.Body = http.MaxBytesReader(w, r.Body, 5<<20) // room for a drawn picture
		var body struct {
			Pet *PetChoice `json:"pet"`
		}
		if err := decodeJSON(r, &body); err == nil {
			choice = body.Pet
		}
	}
	if petReward != nil {
		msg, err := h.pets.checkPetReward(r.Context(), user, petReward, choice)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check pet")
			return
		}
		if msg != "" {
			writeError(w, http.StatusUnprocessableEntity, msg)
			return
		}
	}
	// A drawn accessory or animal picture is saved first and removed again if paying fails.
	if choice != nil && choice.png != nil {
		id, err := savePetImage(choice.png)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save picture")
			return
		}
		choice.item = "img:" + choice.Slot + ":" + id
		if petReward.PetAction == model.PetActionPicture {
			choice.item = id
		}
		defer func() {
			if choice.item == "" {
				removePetImage(id)
			}
		}()
	}
	redemption, err := h.store.RedeemReward(r.Context(), user.ID, rewardID)
	if err != nil {
		if choice != nil {
			choice.item = ""
		}
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if petReward != nil {
		if err := h.pets.applyPetReward(r.Context(), user.ID, petReward, choice); err != nil {
			log.Printf("pets: redemption %d paid but effect failed: %v", redemption.ID, err)
		}
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
	if rw, _ := h.store.GetReward(r.Context(), rewardID); rw != nil && rw.PetAction != "" {
		writeError(w, http.StatusUnprocessableEntity, "pet-care rewards can't be saved for")
		return
	}
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
