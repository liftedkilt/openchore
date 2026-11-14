package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

var clockRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// UpdateSchedule edits a schedule's timing and scoring in place:
// available_at, due_by, expiry_penalty, expiry_penalty_value,
// points_multiplier, start_date and end_date. Fields left out of the body
// are kept; a field sent as null is cleared (for the nullable ones).
//
// The assignee and the recurrence (day_of_week, specific_date, interval)
// can't change here. Ending a schedule with end_date and creating a new one
// moves a chore to a new person or days without deleting its history.
func (h *ChoreHandler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	scheduleID, err := urlParamInt64(r, "scheduleID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule id")
		return
	}
	cs, err := h.store.GetSchedule(r.Context(), scheduleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get schedule")
		return
	}
	if cs == nil {
		writeError(w, http.StatusNotFound, "schedule not found")
		return
	}

	// DATE columns can come back with a time suffix; compare and store
	// plain YYYY-MM-DD.
	for _, d := range []*string{cs.StartDate, cs.EndDate} {
		if d != nil && len(*d) > 10 {
			*d = (*d)[:10]
		}
	}

	var req map[string]json.RawMessage
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	for key := range req {
		switch key {
		case "available_at", "due_by", "start_date", "end_date",
			"expiry_penalty", "expiry_penalty_value", "points_multiplier":
		default:
			writeError(w, http.StatusBadRequest, "cannot update "+key+"; end this schedule and create a new one instead")
			return
		}
	}

	// optString applies a nullable string field: absent keeps, null clears.
	optString := func(key string, dst **string, valid func(string) bool, msg string) bool {
		raw, ok := req[key]
		if !ok {
			return true
		}
		if string(raw) == "null" {
			*dst = nil
			return true
		}
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			writeError(w, http.StatusBadRequest, key+" must be a string or null")
			return false
		}
		if v == "" {
			*dst = nil
			return true
		}
		if !valid(v) {
			writeError(w, http.StatusBadRequest, msg)
			return false
		}
		*dst = &v
		return true
	}
	isClock := func(v string) bool { return clockRe.MatchString(v) }
	isDate := func(v string) bool { _, err := time.Parse(model.DateFormat, v); return err == nil }

	if !optString("available_at", &cs.AvailableAt, isClock, "available_at must be HH:MM") ||
		!optString("due_by", &cs.DueBy, isClock, "due_by must be HH:MM") ||
		!optString("start_date", &cs.StartDate, isDate, "start_date must be YYYY-MM-DD") ||
		!optString("end_date", &cs.EndDate, isDate, "end_date must be YYYY-MM-DD") {
		return
	}
	if raw, ok := req["expiry_penalty"]; ok {
		if err := json.Unmarshal(raw, &cs.ExpiryPenalty); err != nil {
			writeError(w, http.StatusBadRequest, "expiry_penalty must be a string")
			return
		}
	}
	if raw, ok := req["expiry_penalty_value"]; ok {
		if err := json.Unmarshal(raw, &cs.ExpiryPenaltyValue); err != nil {
			writeError(w, http.StatusBadRequest, "expiry_penalty_value must be an integer")
			return
		}
	}
	if raw, ok := req["points_multiplier"]; ok {
		if err := json.Unmarshal(raw, &cs.PointsMultiplier); err != nil || cs.PointsMultiplier <= 0 {
			writeError(w, http.StatusBadRequest, "points_multiplier must be a positive number")
			return
		}
	}

	switch cs.ExpiryPenalty {
	case model.ExpiryBlock, model.ExpiryNoPoints:
	case model.ExpiryPenalty:
		if cs.ExpiryPenaltyValue <= 0 {
			writeError(w, http.StatusBadRequest, "expiry_penalty_value must be positive for penalty mode")
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "expiry_penalty must be block, no_points, or penalty")
		return
	}
	if cs.StartDate != nil && cs.EndDate != nil && *cs.EndDate < *cs.StartDate {
		writeError(w, http.StatusBadRequest, "end_date is before start_date")
		return
	}

	if err := h.store.UpdateSchedule(r.Context(), cs); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update schedule")
		return
	}
	writeJSON(w, http.StatusOK, cs)
}
