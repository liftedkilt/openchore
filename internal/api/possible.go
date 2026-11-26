package api

import (
	"net/http"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

// PossiblePoints returns, for each person who isn't paused, the most they can
// earn on ?date= (default today) and across that Monday-to-Sunday week.
//
// It is the assigned total, for checking that the schedule gives everyone a
// fair chance: nothing is taken off for chores that were excused, missed or
// done late. Each occurrence counts at its payout value (points_value x the
// schedule's points_multiplier, truncated like GetChorePointsForSchedule). A
// first-come chore counts for everyone it's offered to, since any of them
// could earn it.
func (h *ChoreHandler) PossiblePoints(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format(model.DateFormat)
	}
	date, err := time.Parse(model.DateFormat, dateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date format, use YYYY-MM-DD")
		return
	}
	ctx := r.Context()

	users, err := h.store.ListUsers(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	schedules, err := h.store.ListAllSchedules(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list schedules")
		return
	}
	multiplier := make(map[int64]float64, len(schedules))
	for _, s := range schedules {
		multiplier[s.ID] = s.PointsMultiplier
	}

	dates := weekDates(date)
	now := time.Now()
	out := []model.PossiblePoints{}
	for _, u := range users {
		if u.Paused {
			continue
		}
		chores, err := h.store.GetScheduledChoresForUser(ctx, u.ID, dates, now)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get chores")
			return
		}
		p := model.PossiblePoints{UserID: u.ID}
		for _, c := range chores {
			m, ok := multiplier[c.ScheduleID]
			if !ok {
				m = 1
			}
			pts := int(float64(c.PointsValue) * m)
			p.Week += pts
			if c.Date == dateStr {
				p.Today += pts
			}
		}
		out = append(out, p)
	}
	writeJSON(w, http.StatusOK, out)
}
