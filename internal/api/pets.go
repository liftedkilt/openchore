package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/pet"
	"github.com/liftedkilt/openchore/internal/store"
)

// Farm pets: an optional feature a grown-up turns on in Settings. Each
// person may adopt one farm animal; all of them live on one shared farm, so
// everyone can see whose animal is sick or hiding. Chores keep an animal
// healthy (see internal/pet); points buy food, play and accessories through
// ordinary rewards that carry a pet_action.

const (
	settingPetsEnabled = "pets_enabled"
	maxPetNameLen      = 24
)

type PetHandler struct {
	store *store.Store
	now   func() time.Time
}

func NewPetHandler(s *store.Store) *PetHandler {
	return &PetHandler{store: s, now: time.Now}
}

func petsEnabled(ctx context.Context, s *store.Store) bool {
	v, _ := s.GetSetting(ctx, settingPetsEnabled)
	return v == "true"
}

// describe fills in mood and owner details for one pet.
func (h *PetHandler) describe(ctx context.Context, p *model.Pet, owner *model.User) error {
	now := h.now()
	today := now.Format(model.DateFormat)
	yesterday := now.AddDate(0, 0, -1).Format(model.DateFormat)
	chores, err := h.store.GetScheduledChoresForUser(ctx, p.UserID, []string{today}, now)
	if err != nil {
		return err
	}
	pet.Describe(p, yesterday, pet.CaughtUp(chores))
	if owner != nil {
		p.OwnerName = owner.Name
		p.OwnerColor = owner.Color
	}
	return nil
}

// Farm returns every animal on the household farm. Anyone signed in can look.
func (h *PetHandler) Farm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	enabled := petsEnabled(ctx, h.store)
	out := map[string]any{
		"enabled": enabled,
		"species": model.PetSpecies,
		"items":   model.PetItems,
		"pets":    []model.Pet{},
	}
	if !enabled {
		writeJSON(w, http.StatusOK, out)
		return
	}
	pets, err := h.store.ListPets(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list pets")
		return
	}
	users, err := h.store.ListUsers(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list people")
		return
	}
	byID := map[int64]*model.User{}
	for i := range users {
		byID[users[i].ID] = &users[i]
	}
	for i := range pets {
		if err := h.describe(ctx, &pets[i], byID[pets[i].UserID]); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to describe pet")
			return
		}
	}
	out["pets"] = pets
	writeJSON(w, http.StatusOK, out)
}

// requirePerson returns the signed-in person, refusing API tokens (which
// have no profile and so no pet).
func requirePerson(w http.ResponseWriter, r *http.Request) *model.User {
	u := UserFromContext(r.Context())
	if u == nil {
		writeError(w, http.StatusForbidden, "sign in to a profile first")
	}
	return u
}

// Mine returns the signed-in person's pet, or {"pet": null}.
func (h *PetHandler) Mine(w http.ResponseWriter, r *http.Request) {
	u := requirePerson(w, r)
	if u == nil {
		return
	}
	ctx := r.Context()
	p, err := h.store.GetPetForUser(ctx, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load pet")
		return
	}
	if p != nil {
		if err := h.describe(ctx, p, u); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to describe pet")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": petsEnabled(ctx, h.store), "pet": p})
}

func cleanPetName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	return name, name != "" && utf8.RuneCountInString(name) <= maxPetNameLen
}

// Adopt gives the signed-in person a farm animal.
func (h *PetHandler) Adopt(w http.ResponseWriter, r *http.Request) {
	u := requirePerson(w, r)
	if u == nil {
		return
	}
	ctx := r.Context()
	if !petsEnabled(ctx, h.store) {
		writeError(w, http.StatusForbidden, "pets are turned off")
		return
	}
	var req struct {
		Name    string `json:"name"`
		Species string `json:"species"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	name, ok := cleanPetName(req.Name)
	if !ok {
		writeError(w, http.StatusBadRequest, "name must be 1-24 characters")
		return
	}
	if !model.ValidPetSpecies(req.Species) {
		writeError(w, http.StatusBadRequest, "unknown animal")
		return
	}
	p := &model.Pet{
		UserID:   u.ID,
		Name:     name,
		Species:  req.Species,
		Health:   pet.StartHealth,
		Fullness: pet.StartFullness,
		Joy:      pet.StartJoy,
		// Adoption day isn't judged: the first day checked is tomorrow.
		LastDayChecked: h.now().Format(model.DateFormat),
	}
	if err := h.store.CreatePet(ctx, p); err != nil {
		if errors.Is(err, store.ErrPetExists) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to adopt pet")
		return
	}
	p, _ = h.store.GetPetForUser(ctx, u.ID)
	if err := h.describe(ctx, p, u); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to describe pet")
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// Update renames the pet or changes which owned accessory it wears. The
// animal itself can't be swapped, and new accessories come from rewards.
func (h *PetHandler) Update(w http.ResponseWriter, r *http.Request) {
	u := requirePerson(w, r)
	if u == nil {
		return
	}
	ctx := r.Context()
	p, err := h.store.GetPetForUser(ctx, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load pet")
		return
	}
	if p == nil {
		writeError(w, http.StatusNotFound, "you don't have a pet")
		return
	}
	var req struct {
		Name      *string `json:"name"`
		Accessory *string `json:"accessory"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name != nil {
		name, ok := cleanPetName(*req.Name)
		if !ok {
			writeError(w, http.StatusBadRequest, "name must be 1-24 characters")
			return
		}
		if err := h.store.UpdatePetProfile(ctx, u.ID, name, p.Species); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to rename pet")
			return
		}
	}
	if req.Accessory != nil {
		item := *req.Accessory
		if item != "" {
			owned, err := h.store.PetOwnsItem(ctx, u.ID, item)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to check accessories")
				return
			}
			if !owned {
				writeError(w, http.StatusUnprocessableEntity, "get that from the rewards first")
				return
			}
		}
		if err := h.store.SetPetAccessory(ctx, u.ID, item); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to change accessory")
			return
		}
	}
	p, _ = h.store.GetPetForUser(ctx, u.ID)
	if err := h.describe(ctx, p, u); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to describe pet")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// defaultPetRewards are added the first time pets are turned on.
var defaultPetRewards = []model.Reward{
	{Name: "Feed my animal", Description: "A full meal for your farm animal.", Icon: "🥕", Cost: 5, PetAction: model.PetActionFeed},
	{Name: "Give a treat", Description: "A tasty treat: a little food and a lot of joy.", Icon: "🍎", Cost: 10, PetAction: model.PetActionTreat},
	{Name: "Play together", Description: "Playtime makes your animal happy.", Icon: "🎾", Cost: 5, PetAction: model.PetActionPlay},
	{Name: "Straw hat", Description: "A straw hat for your farm animal.", Icon: "👒", Cost: 25, PetAction: model.PetActionAccessory, PetItem: "straw_hat"},
	{Name: "Bandana", Description: "A bandana for your farm animal.", Icon: "🧣", Cost: 20, PetAction: model.PetActionAccessory, PetItem: "bandana"},
	{Name: "Flower crown", Description: "A flower crown for your farm animal.", Icon: "🌼", Cost: 30, PetAction: model.PetActionAccessory, PetItem: "flower_crown"},
}

// SetEnabled turns farm pets on or off (grown-ups). Turning them on the
// first time adds a starter set of pet-care rewards to the store.
func (h *PetHandler) SetEnabled(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	val := "false"
	if req.Enabled {
		val = "true"
	}
	if err := h.store.SetSetting(ctx, settingPetsEnabled, val); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save setting")
		return
	}
	seeded := 0
	if req.Enabled {
		n, err := h.store.CountPetRewards(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check rewards")
			return
		}
		if n == 0 {
			var createdBy int64
			if u := UserFromContext(ctx); u != nil {
				createdBy = u.ID
			}
			for _, rw := range defaultPetRewards {
				rw.Active = true
				rw.CreatedBy = createdBy
				if err := h.store.CreateReward(ctx, &rw); err != nil {
					writeError(w, http.StatusInternalServerError, "failed to add pet rewards")
					return
				}
				seeded++
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": req.Enabled, "rewards_added": seeded})
}

// Remove takes a person's pet away (grown-ups), e.g. to start over.
func (h *PetHandler) Remove(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := h.store.DeletePetForUser(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove pet")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// checkPetReward refuses a pet-care redemption that can't take effect,
// before any points move. It returns "" when the redemption may go ahead.
func (h *PetHandler) checkPetReward(ctx context.Context, u *model.User, rw *model.Reward) (string, error) {
	if !petsEnabled(ctx, h.store) {
		return "pets are turned off", nil
	}
	p, err := h.store.GetPetForUser(ctx, u.ID)
	if err != nil {
		return "", err
	}
	if p == nil {
		return "adopt a farm animal first", nil
	}
	switch rw.PetAction {
	case model.PetActionPlay:
		if err := h.describe(ctx, p, u); err != nil {
			return "", err
		}
		if p.Hiding {
			return p.Name + " is hiding and doesn't want to come out and play. Finish today's chores first!", nil
		}
		if !p.CanPlay {
			return p.Name + " is too sick to play. Doing your chores will help them get better.", nil
		}
	case model.PetActionAccessory:
		owned, err := h.store.PetOwnsItem(ctx, u.ID, rw.PetItem)
		if err != nil {
			return "", err
		}
		if owned {
			return p.Name + " already has that", nil
		}
	}
	return "", nil
}

// applyPetReward gives the pet what a redeemed pet-care reward bought.
func (h *PetHandler) applyPetReward(ctx context.Context, userID int64, rw *model.Reward) error {
	if rw.PetAction == model.PetActionAccessory {
		return h.store.AddPetItem(ctx, userID, rw.PetItem)
	}
	f, j := pet.Effect(rw.PetAction)
	return h.store.AdjustPetNeeds(ctx, userID, f, j)
}
