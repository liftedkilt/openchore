package api

import (
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

type SetupHandler struct {
	store    *store.Store
	sessions *SessionManager
}

func NewSetupHandler(s *store.Store, sm *SessionManager) *SetupHandler {
	return &SetupHandler{store: s, sessions: sm}
}

type setupParent struct {
	Name string `json:"name"`
	Pin  string `json:"pin"`
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
	Parent   setupParent  `json:"parent"`
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
	if !pinFormatValid(req.Parent.Pin) {
		writeError(w, http.StatusBadRequest, "parent pin must be 4-8 digits")
		return
	}
	parentName := req.Parent.Name
	if parentName == "" {
		parentName = "Parent"
	}
	pinHash, err := bcrypt.GenerateFromPassword([]byte(req.Parent.Pin), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash pin")
		return
	}

	// 1. Create the parent (admin) profile
	admin := &model.User{Name: parentName, Role: model.RoleAdmin, PinHash: string(pinHash)}
	if err := h.store.CreateUser(r.Context(), admin); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create admin user")
		return
	}

	// 2. Create children
	var createdChildren []*model.User
	for _, c := range req.Children {
		child := &model.User{
			Name:  c.Name,
			Role:  model.RoleChild,
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
			category = model.CategoryCore
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
					ExpiryPenalty:    model.ExpiryBlock,
				}
				if err := h.store.CreateSchedule(r.Context(), schedule); err != nil {
					writeError(w, http.StatusInternalServerError, "failed to create schedule")
					return
				}
			}
		}
	}

	// Sign the parent in so the wizard can drop them straight into the app.
	token, issued := h.sessions.Issue(SessionClaims{UserID: admin.ID, Version: admin.SessionVersion, Method: SessionMethodPin})
	setSessionCookie(w, r, token, issued)

	writeJSON(w, http.StatusCreated, map[string]any{
		"admin":    admin,
		"children": createdChildren,
	})
}
