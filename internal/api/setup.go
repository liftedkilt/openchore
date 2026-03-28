package api

import (
	"net/http"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

type SetupHandler struct {
	store *store.Store
}

func NewSetupHandler(s *store.Store) *SetupHandler {
	return &SetupHandler{store: s}
}

type setupChild struct {
	Name  string `json:"name"`
	Theme string `json:"theme"`
}

type setupChore struct {
	Title    string `json:"title"`
	Icon     string `json:"icon"`
	Category string `json:"category"`
	Points   int    `json:"points_value"`
}

type setupRequest struct {
	Children []setupChild `json:"children"`
	Chores   []setupChore `json:"chores"`
}

func (h *SetupHandler) Setup(w http.ResponseWriter, r *http.Request) {
	// Only allow setup when no users exist
	users, err := h.store.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check users")
		return
	}
	if len(users) > 0 {
		writeError(w, http.StatusConflict, "setup already completed")
		return
	}

	var req setupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Children) == 0 {
		writeError(w, http.StatusBadRequest, "at least one child is required")
		return
	}

	// 1. Create admin user
	admin := &model.User{Name: "Parent", Role: "admin"}
	if err := h.store.CreateUser(r.Context(), admin); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create admin user")
		return
	}

	// 2. Create children
	var createdChildren []*model.User
	for _, c := range req.Children {
		child := &model.User{
			Name:  c.Name,
			Role:  "child",
			Theme: c.Theme,
		}
		if err := h.store.CreateUser(r.Context(), child); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create child")
			return
		}
		createdChildren = append(createdChildren, child)
	}

	// 3. Create chores and assign to all children for every day
	for _, ch := range req.Chores {
		category := ch.Category
		if category == "" {
			category = "core"
		}
		chore := &model.Chore{
			Title:       ch.Title,
			Category:    category,
			Icon:        ch.Icon,
			PointsValue: ch.Points,
			Source:      "manual",
			CreatedBy:   admin.ID,
		}
		if err := h.store.CreateChore(r.Context(), chore); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create chore")
			return
		}

		for _, child := range createdChildren {
			for dow := 0; dow < 7; dow++ {
				dayOfWeek := dow
				schedule := &model.ChoreSchedule{
					ChoreID:          chore.ID,
					AssignedTo:       child.ID,
					AssignmentType:   "individual",
					DayOfWeek:        &dayOfWeek,
					PointsMultiplier: 1.0,
					ExpiryPenalty:    "block",
				}
				if err := h.store.CreateSchedule(r.Context(), schedule); err != nil {
					writeError(w, http.StatusInternalServerError, "failed to create schedule")
					return
				}
			}
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"admin":    admin,
		"children": createdChildren,
	})
}
