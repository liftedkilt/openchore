package api

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/tts"
)

// Own chores: anyone signed in to their own profile (kids included) can add
// tasks to their own list as reminders. They are always Optional and worth
// 0 points, assigned only to the person who added them, and never need a
// photo or approval. Grown-ups can still add Optional chores worth points
// through the usual Manage screens.
//
// The owner manages a chore only while it stays that way: once a grown-up
// adds points or changes its category it becomes theirs to manage, so a kid
// can't put a points chore on a schedule of their own.

const (
	maxOwnChores       = 50
	maxOwnTitleLen     = 80
	maxOwnDescLen      = 500
	maxOwnIntervalDays = 365
	// How far ahead a one-off task may be planned.
	maxOwnDaysAhead = 366
)

// ownChoreWhen says when an own chore shows up. Exactly one of SpecificDate,
// DaysOfWeek or RecurrenceInterval is set.
type ownChoreWhen struct {
	// One day, today or later.
	SpecificDate *string `json:"specific_date"`
	// Every week on these days (0 = Sunday … 6 = Saturday).
	DaysOfWeek []int `json:"days_of_week"`
	// Every N days, counting from StartDate.
	RecurrenceInterval *int `json:"recurrence_interval"`
	// First day for a repeating task (defaults to today).
	StartDate *string `json:"start_date"`
	// Optional last day for a repeating task.
	EndDate *string `json:"end_date"`
}

type ownChoreRequest struct {
	Title       *string       `json:"title"`
	Description *string       `json:"description"`
	Icon        *string       `json:"icon"`
	When        *ownChoreWhen `json:"when"`
}

// ownChoreCaller returns the signed-in profile, or writes an error. API
// tokens have no profile of their own and can't add own chores.
func ownChoreCaller(w http.ResponseWriter, r *http.Request) *model.User {
	u := UserFromContext(r.Context())
	if u == nil || u.ID == 0 {
		writeError(w, http.StatusForbidden, "sign in to a profile to manage your own tasks")
		return nil
	}
	return u
}

func ownChoreEditable(c *model.Chore) bool {
	return c.Category == model.CategoryOptional && c.PointsValue == 0 && c.MissedPenaltyValue == 0
}

func parseDay(s string) (time.Time, bool) {
	t, err := time.ParseInLocation(model.DateFormat, s, time.Local)
	return t, err == nil
}

// ownSchedules validates when and turns it into schedule rows for userID.
func ownSchedules(when *ownChoreWhen, userID int64, now time.Time) ([]model.ChoreSchedule, string) {
	if when == nil {
		return nil, "when is required"
	}
	today := now.Format(model.DateFormat)
	todayT, _ := parseDay(today)

	modes := 0
	if when.SpecificDate != nil {
		modes++
	}
	if len(when.DaysOfWeek) > 0 {
		modes++
	}
	if when.RecurrenceInterval != nil {
		modes++
	}
	if modes != 1 {
		return nil, "choose one: a date, days of the week, or every few days"
	}

	base := model.ChoreSchedule{
		AssignedTo:       userID,
		AssignmentType:   model.AssignmentIndividual,
		PointsMultiplier: 1.0,
		ExpiryPenalty:    model.ExpiryNoPoints,
	}

	if when.SpecificDate != nil {
		d, ok := parseDay(*when.SpecificDate)
		if !ok {
			return nil, "specific_date must be YYYY-MM-DD"
		}
		if *when.SpecificDate < today {
			return nil, "pick today or a day in the future"
		}
		if d.Sub(todayT) > maxOwnDaysAhead*24*time.Hour {
			return nil, "pick a day within the next year"
		}
		sc := base
		date := *when.SpecificDate
		sc.SpecificDate = &date
		return []model.ChoreSchedule{sc}, ""
	}

	start := today
	if when.StartDate != nil && *when.StartDate != "" {
		d, ok := parseDay(*when.StartDate)
		if !ok {
			return nil, "start_date must be YYYY-MM-DD"
		}
		if *when.StartDate < today {
			return nil, "start today or later"
		}
		if d.Sub(todayT) > maxOwnDaysAhead*24*time.Hour {
			return nil, "start within the next year"
		}
		start = *when.StartDate
	}
	var end *string
	if when.EndDate != nil && *when.EndDate != "" {
		if _, ok := parseDay(*when.EndDate); !ok {
			return nil, "end_date must be YYYY-MM-DD"
		}
		if *when.EndDate < start {
			return nil, "end_date can't be before the start"
		}
		e := *when.EndDate
		end = &e
	}

	if when.RecurrenceInterval != nil {
		n := *when.RecurrenceInterval
		if n < 1 || n > maxOwnIntervalDays {
			return nil, "recurrence_interval must be between 1 and 365 days"
		}
		sc := base
		interval, rs := n, start
		sc.RecurrenceInterval = &interval
		sc.RecurrenceStart = &rs
		sc.EndDate = end
		return []model.ChoreSchedule{sc}, ""
	}

	seen := map[int]bool{}
	var out []model.ChoreSchedule
	for _, d := range when.DaysOfWeek {
		if d < 0 || d > 6 {
			return nil, "days_of_week must be 0 (Sunday) to 6 (Saturday)"
		}
		if seen[d] {
			continue
		}
		seen[d] = true
		sc := base
		day, st := d, start
		sc.DayOfWeek = &day
		// Only hold a weekly task back when it starts later than today.
		if st > today {
			sc.StartDate = &st
		}
		sc.EndDate = end
		out = append(out, sc)
	}
	return out, ""
}

func activeSchedules(all []model.ChoreSchedule, today string) []model.ChoreSchedule {
	out := []model.ChoreSchedule{}
	for _, cs := range all {
		if cs.EndDate != nil && *cs.EndDate < today {
			continue
		}
		if cs.SpecificDate != nil && *cs.SpecificDate < today {
			continue
		}
		out = append(out, cs)
	}
	return out
}

// ListOwn returns the caller's own chores with their current schedules.
// GET /api/me/chores
func (h *ChoreHandler) ListOwn(w http.ResponseWriter, r *http.Request) {
	u := ownChoreCaller(w, r)
	if u == nil {
		return
	}
	chores, err := h.store.ListOwnChores(r.Context(), u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list your tasks")
		return
	}
	today := time.Now().Format(model.DateFormat)
	out := []model.OwnChore{}
	for _, c := range chores {
		schedules, err := h.store.ListSchedulesForChore(r.Context(), c.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list your tasks")
			return
		}
		out = append(out, model.OwnChore{Chore: c, Schedules: activeSchedules(schedules, today), Editable: ownChoreEditable(&c)})
	}
	writeJSON(w, http.StatusOK, out)
}

func cleanOwnText(title, desc *string) (string, string, string) {
	t, d := "", ""
	if title != nil {
		t = strings.TrimSpace(*title)
	}
	if desc != nil {
		d = strings.TrimSpace(*desc)
	}
	if utf8.RuneCountInString(t) > maxOwnTitleLen {
		return "", "", "title is too long"
	}
	if utf8.RuneCountInString(d) > maxOwnDescLen {
		return "", "", "description is too long"
	}
	return t, d, ""
}

// CreateOwn adds an Optional, 0-point chore to the caller's own list.
// POST /api/me/chores
func (h *ChoreHandler) CreateOwn(w http.ResponseWriter, r *http.Request) {
	u := ownChoreCaller(w, r)
	if u == nil {
		return
	}
	var req ownChoreRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	title, desc, msg := cleanOwnText(req.Title, req.Description)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	schedules, msg := ownSchedules(req.When, u.ID, time.Now())
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	existing, err := h.store.ListOwnChores(r.Context(), u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add your task")
		return
	}
	if len(existing) >= maxOwnChores {
		writeError(w, http.StatusUnprocessableEntity, "you have lots of tasks already; remove some first")
		return
	}
	icon := ""
	if req.Icon != nil {
		icon = strings.TrimSpace(*req.Icon)
	}

	chore := &model.Chore{
		Title:       title,
		Description: desc,
		Category:    model.CategoryOptional,
		Icon:        icon,
		PhotoSource: model.PhotoSourceChild,
		Source:      model.ChoreSourceSelf,
		CreatedBy:   u.ID,
	}
	if err := h.store.CreateChoreWithSchedules(r.Context(), chore, schedules); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add your task")
		return
	}
	if audio := h.aiSvc.Audio(); audio != nil {
		audio.GenerateAsync(*chore)
	}
	writeJSON(w, http.StatusCreated, model.OwnChore{Chore: *chore, Schedules: schedules, Editable: true})
}

// ownChoreFor loads chore {id} and checks the caller may manage it.
func (h *ChoreHandler) ownChoreFor(w http.ResponseWriter, r *http.Request, u *model.User) *model.Chore {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid chore id")
		return nil
	}
	c, err := h.store.GetChore(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get chore")
		return nil
	}
	if c == nil || c.Source != model.ChoreSourceSelf || c.CreatedBy != u.ID {
		writeError(w, http.StatusNotFound, "task not found")
		return nil
	}
	if !ownChoreEditable(c) {
		writeError(w, http.StatusForbidden, "a grown-up changed this task; ask them to change it")
		return nil
	}
	return c
}

// UpdateOwn renames an own chore and, with `when`, reschedules it.
// PUT /api/me/chores/{id}
func (h *ChoreHandler) UpdateOwn(w http.ResponseWriter, r *http.Request) {
	u := ownChoreCaller(w, r)
	if u == nil {
		return
	}
	c := h.ownChoreFor(w, r, u)
	if c == nil {
		return
	}
	var req ownChoreRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	title, desc, msg := cleanOwnText(req.Title, req.Description)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	spokenBefore := tts.SpokenText(c)
	if req.Title != nil {
		if title == "" {
			writeError(w, http.StatusBadRequest, "title is required")
			return
		}
		c.Title = title
	}
	if req.Description != nil {
		c.Description = desc
	}
	if req.Icon != nil {
		c.Icon = strings.TrimSpace(*req.Icon)
	}

	now := time.Now()
	var schedules []model.ChoreSchedule
	if req.When != nil {
		schedules, msg = ownSchedules(req.When, u.ID, now)
		if msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
	}
	if err := h.store.UpdateChore(r.Context(), c); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update your task")
		return
	}
	if req.When != nil {
		yesterday := now.AddDate(0, 0, -1).Format(model.DateFormat)
		if err := h.store.ReplaceChoreSchedules(r.Context(), c.ID, yesterday, schedules); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to reschedule your task")
			return
		}
	}
	if audio := h.aiSvc.Audio(); audio != nil && (c.TTSAudioURL == "" || tts.SpokenText(c) != spokenBefore) {
		audio.GenerateAsync(*c)
	}
	all, err := h.store.ListSchedulesForChore(r.Context(), c.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update your task")
		return
	}
	writeJSON(w, http.StatusOK, model.OwnChore{Chore: *c, Schedules: activeSchedules(all, now.Format(model.DateFormat)), Editable: true})
}

// DeleteOwn removes an own chore and its schedules.
// DELETE /api/me/chores/{id}
func (h *ChoreHandler) DeleteOwn(w http.ResponseWriter, r *http.Request) {
	u := ownChoreCaller(w, r)
	if u == nil {
		return
	}
	c := h.ownChoreFor(w, r, u)
	if c == nil {
		return
	}
	if err := h.store.DeleteChore(r.Context(), c.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove your task")
		return
	}
	tts.Remove(c.ID)
	w.WriteHeader(http.StatusNoContent)
}
