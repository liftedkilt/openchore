package api

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/liftedkilt/openchore/internal/friends"
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
	// Ribbons, when set, says whose animals wear the leaderboard winner's
	// ribbon this week (set by the router; the leaderboard lives elsewhere).
	Ribbons func(ctx context.Context) map[int64]bool
	// Friends, when set, handles gifts and playdates to linked families.
	Friends *friends.Service
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
	if owner == nil {
		if owner, err = h.store.GetUser(ctx, p.UserID); err != nil {
			return err
		}
	}
	pet.ShowToday(p, chores, yesterday, owner != nil && owner.Paused)
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
		"colors":  model.PetColors,
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
	ribbons := map[int64]bool{}
	if h.Ribbons != nil {
		ribbons = h.Ribbons(ctx)
	}
	for i := range pets {
		if err := h.describe(ctx, &pets[i], byID[pets[i].UserID]); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to describe pet")
			return
		}
		pets[i].Ribbon = ribbons[pets[i].UserID]
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
		Color   string `json:"color"`
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
	if req.Color != "" && !model.ValidPetColor(req.Species, req.Color) {
		writeError(w, http.StatusBadRequest, "unknown coat colour")
		return
	}
	p := &model.Pet{
		Color:    req.Color,
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
		// Sprite can only be cleared here (""): going back to the drawn
		// animal is free; a new picture comes from a "picture" reward.
		Sprite *string `json:"sprite"`
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
		if err := h.store.RenamePet(ctx, u.ID, name); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to rename pet")
			return
		}
	}
	if req.Sprite != nil {
		if *req.Sprite != "" {
			writeError(w, http.StatusUnprocessableEntity, "get a picture reward to use your own picture")
			return
		}
		if err := h.clearSprite(ctx, u.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to change picture")
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

// playdateJoy is what a day out at a friend's farm does for an animal.
const playdateJoy = 25

// defaultPetRewards are added the first time pets are turned on.
var defaultPetRewards = []model.Reward{
	{Name: "Feed my animal", Description: "A full meal for your farm animal.", Icon: "🥕", Cost: 5, PetAction: model.PetActionFeed},
	{Name: "Give a treat", Description: "A tasty treat: a little food and a lot of joy.", Icon: "🍎", Cost: 10, PetAction: model.PetActionTreat},
	{Name: "Play together", Description: "Playtime makes your animal happy.", Icon: "🎾", Cost: 5, PetAction: model.PetActionPlay},
	{Name: "Straw hat", Description: "A straw hat for your farm animal.", Icon: "👒", Cost: 25, PetAction: model.PetActionAccessory, PetItem: "straw_hat"},
	{Name: "Bandana", Description: "A bandana for your farm animal.", Icon: "🧣", Cost: 20, PetAction: model.PetActionAccessory, PetItem: "bandana"},
	{Name: "Flower crown", Description: "A flower crown for your farm animal.", Icon: "🌼", Cost: 30, PetAction: model.PetActionAccessory, PetItem: "flower_crown"},
	{Name: "Party hat", Description: "A party hat for your farm animal.", Icon: "🥳", Cost: 20, PetAction: model.PetActionAccessory, PetItem: "party_hat"},
	{Name: "Round glasses", Description: "Smart round glasses for your farm animal.", Icon: "👓", Cost: 20, PetAction: model.PetActionAccessory, PetItem: "round_glasses"},
	{Name: "Restyle coat", Description: "Give your farm animal a new coat colour.", Icon: "🎨", Cost: 30, PetAction: model.PetActionRestyle},
	{Name: "Design an accessory", Description: "Draw your own accessory, or use a picture, and your animal will wear it.", Icon: "🖍️", Cost: 40, PetAction: model.PetActionDesign},
	{Name: "Use my own picture", Description: "Make your farm animal look like a picture you drew or made.", Icon: "🖼️", Cost: 60, PetAction: model.PetActionPicture},
	{Name: "Treat for a friend's animal", Description: "Send a treat to an animal on a friend family's farm.", Icon: "🎁", Cost: 10, PetAction: model.PetActionGift},
	{Name: "Playdate", Description: "Send your animal to play on a friend family's farm for the day.", Icon: "🚜", Cost: 15, PetAction: model.PetActionPlaydate},
	{Name: "New animal", Description: "Trade for a different farm animal. Health, food and joy come along.", Icon: "🔁", Cost: 150, PetAction: model.PetActionNewAnimal},
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
	// Its pictures go with it: nothing should stay fetchable.
	old, _ := h.store.GetPetForUser(r.Context(), id)
	if err := h.store.DeletePetForUser(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove pet")
		return
	}
	removePetFiles(old)
	w.WriteHeader(http.StatusNoContent)
}

// clearSprite goes back to the drawn animal and deletes the picture.
func (h *PetHandler) clearSprite(ctx context.Context, userID int64) error {
	old, err := h.store.SetPetSprite(ctx, userID, "", 1)
	if err == nil && old != "" {
		removePetImage(old)
	}
	return err
}

// RemovePicture takes a person's own animal picture away (grown-ups), e.g.
// when it isn't suitable. The animal goes back to its drawn look.
func (h *PetHandler) RemovePicture(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	p, err := h.store.GetPetForUser(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load pet")
		return
	}
	if p == nil {
		writeError(w, http.StatusNotFound, "that person has no farm animal")
		return
	}
	if err := h.clearSprite(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove picture")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ApprovePicture records whether a grown-up has looked at a person's own
// animal picture and said it may be sent to friend families. It never
// affects the family's own farm, where the picture shows either way.
func (h *PetHandler) ApprovePicture(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var req struct {
		Approved *bool `json:"approved"`
		// Sprite is the id of the picture the parent was looking at. If
		// the kid has changed it since, nothing is approved.
		Sprite string `json:"sprite"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Approved == nil {
		writeError(w, http.StatusBadRequest, "approved (true or false) is required")
		return
	}
	p, err := h.store.GetPetForUser(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load pet")
		return
	}
	if p == nil || p.Sprite == "" {
		writeError(w, http.StatusNotFound, "that person's animal has no picture of its own")
		return
	}
	if req.Sprite == "" {
		req.Sprite = p.Sprite
		if *req.Approved {
			writeError(w, http.StatusBadRequest, "sprite (the id of the picture you looked at) is required to approve it")
			return
		}
	}
	found, err := h.store.SetPetSpriteApproved(r.Context(), id, req.Sprite, *req.Approved)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save")
		return
	}
	if !found {
		writeError(w, http.StatusConflict, "the picture has changed since you looked at it; have another look first")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "approved": *req.Approved})
}

// PetChoice is what a person picks when redeeming a "new_animal" or
// "restyle" reward.
type PetChoice struct {
	Species string `json:"species"`
	Color   string `json:"color"`
	// For "design": where it's worn and the picture as a data: URL.
	Slot  string `json:"slot"`
	Image string `json:"image"`
	// For "gift": the friend whose animal gets it, by leaderboard key
	// ("f<link>:<member>"). For "playdate": the linked family to visit.
	Friend string `json:"friend"`
	Link   int64  `json:"link"`

	send *friends.SendTarget // the checked gift or playdate

	png    []byte // the checked, shrunk picture
	frames int    // for an animal picture: how many frames it holds
	item   string // once saved: "img:<slot>:<id>" for an accessory, the bare id for an animal picture
}

// checkPetReward refuses a pet-care redemption that can't take effect,
// before any points move. It returns "" when the redemption may go ahead.
// choice (may be nil) carries the pick for new_animal / restyle; it is
// normalised in place.
func (h *PetHandler) checkPetReward(ctx context.Context, u *model.User, rw *model.Reward, choice *PetChoice) (string, error) {
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
	case model.PetActionNewAnimal:
		if choice == nil || !model.ValidPetSpecies(choice.Species) {
			return "choose an animal", nil
		}
		if choice.Color == "" {
			choice.Color = model.PetColors[choice.Species][0]
		}
		if !model.ValidPetColor(choice.Species, choice.Color) {
			return "choose a coat colour", nil
		}
		if choice.Species == p.Species && choice.Color == petColor(p) {
			return p.Name + " already looks like that", nil
		}
	case model.PetActionDesign:
		if choice == nil || !slices.Contains(model.PetItemSlots, choice.Slot) {
			return "choose where it goes: head, face or neck", nil
		}
		png, err := decodePetImage(choice.Image)
		if err != nil {
			return err.Error(), nil
		}
		choice.png = png
	case model.PetActionGift, model.PetActionPlaydate:
		if h.Friends == nil || choice == nil {
			return "choose a friend", nil
		}
		if rw.PetAction == model.PetActionPlaydate {
			if err := h.describe(ctx, p, u); err != nil {
				return "", err
			}
			if p.Hiding {
				return p.Name + " is hiding and doesn't want to go out. Finish today's chores first!", nil
			}
			if !p.CanPlay {
				return p.Name + " is too sick to go on a playdate. Doing your chores will help them get better.", nil
			}
		}
		target, msg, err := h.Friends.CheckSend(ctx, u.ID, rw.PetAction, choice.Friend, choice.Link)
		if err != nil || msg != "" {
			return msg, err
		}
		choice.send = target
	case model.PetActionPicture:
		if choice == nil || choice.Image == "" {
			return "choose a picture", nil
		}
		png, frames, err := decodeSprite(choice.Image)
		if err != nil {
			return err.Error(), nil
		}
		choice.png, choice.frames = png, frames
	case model.PetActionRestyle:
		if choice == nil || !model.ValidPetColor(p.Species, choice.Color) {
			return "choose a coat colour", nil
		}
		if choice.Color == petColor(p) {
			return p.Name + " already has that coat", nil
		}
		choice.Species = p.Species
	}
	return "", nil
}

// petColor is the pet's coat, with "" read as its species' default.
func petColor(p *model.Pet) string {
	if p.Color == "" {
		return model.PetColors[p.Species][0]
	}
	return p.Color
}

// applyPetReward gives the pet what a redeemed pet-care reward bought.
func (h *PetHandler) applyPetReward(ctx context.Context, userID int64, rw *model.Reward, choice *PetChoice) error {
	switch rw.PetAction {
	case model.PetActionAccessory:
		return h.store.AddPetItem(ctx, userID, rw.PetItem)
	case model.PetActionNewAnimal, model.PetActionRestyle:
		return h.store.SetPetLook(ctx, userID, choice.Species, choice.Color)
	case model.PetActionDesign:
		return h.store.AddPetItem(ctx, userID, choice.item)
	case model.PetActionGift:
		// The treat goes to the friend's animal, not this one. It was
		// recorded before the points moved (see Redeem).
		return nil
	case model.PetActionPlaydate:
		// A day out: the animal comes home happier.
		return h.store.AdjustPetNeeds(ctx, userID, 0, playdateJoy)
	case model.PetActionPicture:
		old, err := h.store.SetPetSprite(ctx, userID, choice.item, choice.frames)
		if err == nil && old != "" {
			removePetImage(old)
		}
		return err
	}
	f, j := pet.Effect(rw.PetAction)
	return h.store.AdjustPetNeeds(ctx, userID, f, j)
}
