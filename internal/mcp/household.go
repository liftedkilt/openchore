package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"
)

const dateFormat = "2006-01-02"

// These mirror the REST API's JSON (internal/model) for the fields the
// helpers read. Decoding into local structs keeps this package free of
// internal/api while still going through the API for every write.
type apiUser struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Age    *int   `json:"age,omitempty"`
	Paused bool   `json:"paused"`
}

type apiChore struct {
	ID                 int64  `json:"id"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	Category           string `json:"category"`
	Icon               string `json:"icon,omitempty"`
	PointsValue        int    `json:"points_value"`
	MissedPenaltyValue int    `json:"missed_penalty_value"`
	EstimatedMinutes   *int   `json:"estimated_minutes,omitempty"`
	RequiresApproval   bool   `json:"requires_approval"`
	RequiresPhoto      bool   `json:"requires_photo"`
	PhotoSource        string `json:"photo_source"`
	Source             string `json:"source"`
}

type apiSchedule struct {
	ID                 int64   `json:"id"`
	ChoreID            int64   `json:"chore_id"`
	AssignedTo         int64   `json:"assigned_to"`
	AssignmentType     string  `json:"assignment_type"`
	FcfsGroupID        *string `json:"fcfs_group_id,omitempty"`
	DayOfWeek          *int    `json:"day_of_week,omitempty"`
	SpecificDate       *string `json:"specific_date,omitempty"`
	AvailableAt        *string `json:"available_at,omitempty"`
	PointsMultiplier   float64 `json:"points_multiplier"`
	StartDate          *string `json:"start_date,omitempty"`
	EndDate            *string `json:"end_date,omitempty"`
	RecurrenceInterval *int    `json:"recurrence_interval,omitempty"`
	RecurrenceStart    *string `json:"recurrence_start,omitempty"`
	DueBy              *string `json:"due_by,omitempty"`
	ExpiryPenalty      string  `json:"expiry_penalty"`
	ExpiryPenaltyValue int     `json:"expiry_penalty_value"`
}

type apiTriggerable struct {
	ID       int64 `json:"id"`
	Triggers []struct {
		ID                  int64  `json:"id"`
		UUID                string `json:"uuid"`
		DefaultAssignedName string `json:"default_assigned_name,omitempty"`
		Enabled             bool   `json:"enabled"`
	} `json:"triggers"`
}

var categoryLabels = map[string]string{"required": "Must do", "core": "Every day", "bonus": "Bonus"}

var dayNames = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

var dayByName = map[string][]int{
	"sun": {0}, "sunday": {0}, "mon": {1}, "monday": {1}, "tue": {2}, "tues": {2}, "tuesday": {2},
	"wed": {3}, "wednesday": {3}, "thu": {4}, "thur": {4}, "thurs": {4}, "thursday": {4},
	"fri": {5}, "friday": {5}, "sat": {6}, "saturday": {6},
	"daily": {0, 1, 2, 3, 4, 5, 6}, "everyday": {0, 1, 2, 3, 4, 5, 6}, "every day": {0, 1, 2, 3, 4, 5, 6},
	"weekdays": {1, 2, 3, 4, 5}, "weekends": {0, 6},
}

func today() string { return time.Now().Format(dateFormat) }

func dateOnly(p *string) string {
	if p == nil {
		return ""
	}
	if len(*p) > 10 {
		return (*p)[:10]
	}
	return *p
}

// ended reports whether a schedule can no longer come up.
func (s apiSchedule) ended(today string) bool {
	if d := dateOnly(s.EndDate); d != "" && d < today {
		return true
	}
	if d := dateOnly(s.SpecificDate); d != "" && s.RecurrenceInterval == nil && d < today {
		return true
	}
	return false
}

// --- shared schedule pattern arguments ---

var patternProps = map[string]any{
	"days": map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "Weekdays it repeats on: sun, mon, tue, wed, thu, fri, sat, or the shortcuts daily, weekdays, weekends.",
	},
	"recurrence_interval":  integer("Repeat every N days instead of on weekdays (with recurrence_start)."),
	"recurrence_start":     str("First day of an every-N-days pattern, YYYY-MM-DD (default today)."),
	"specific_date":        str("A one-off date, YYYY-MM-DD, instead of a repeating pattern."),
	"available_at":         str("Earliest time it can be done, HH:MM."),
	"due_by":               str("Deadline, HH:MM."),
	"expiry_penalty":       enum("After due_by: block (can't do it late, default), no_points (late earns nothing), penalty (late costs expiry_penalty_value).", "block", "no_points", "penalty"),
	"expiry_penalty_value": integer("Points deducted when late, for penalty mode."),
	"points_multiplier":    number("Multiplier on the chore's points (default 1)."),
	"start_date":           str("First day this applies, YYYY-MM-DD (default: now)."),
	"end_date":             str("Last day this applies, YYYY-MM-DD (default: no end)."),
}

// schedulePayloads expands a pattern into the REST API's one-row-per-day
// schedule bodies for one person.
func schedulePayloads(a Args, userID int64) ([]map[string]any, error) {
	common := a.pick("available_at", "due_by", "expiry_penalty", "expiry_penalty_value", "points_multiplier", "start_date", "end_date")
	common["assigned_to"] = userID

	patterns := 0
	for _, k := range []string{"days", "recurrence_interval", "specific_date"} {
		if a.Has(k) {
			patterns++
		}
	}
	if patterns != 1 {
		return nil, fmt.Errorf("give exactly one of days, recurrence_interval or specific_date")
	}

	switch {
	case a.Has("specific_date"):
		p := clone(common)
		p["specific_date"] = a.Str("specific_date")
		return []map[string]any{p}, nil
	case a.Has("recurrence_interval"):
		p := clone(common)
		p["recurrence_interval"] = a["recurrence_interval"]
		start := a.Str("recurrence_start")
		if start == "" {
			start = today()
		}
		p["recurrence_start"] = start
		return []map[string]any{p}, nil
	}

	raw, ok := a["days"].([]any)
	if !ok || len(raw) == 0 {
		if s, ok := a["days"].(string); ok { // "weekdays" as a bare string
			raw = []any{s}
		} else {
			return nil, fmt.Errorf("days must be a list like [\"mon\", \"wed\"]")
		}
	}
	seen := map[int]bool{}
	var days []int
	for _, d := range raw {
		name := strings.ToLower(strings.TrimSpace(fmt.Sprint(d)))
		list, ok := dayByName[name]
		if !ok {
			return nil, fmt.Errorf("unknown day %q", d)
		}
		for _, n := range list {
			if !seen[n] {
				seen[n] = true
				days = append(days, n)
			}
		}
	}
	sort.Ints(days)
	out := make([]map[string]any, 0, len(days))
	for _, d := range days {
		p := clone(common)
		p["day_of_week"] = d
		out = append(out, p)
	}
	return out, nil
}

func clone(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// assign creates schedules for each person and reports what happened.
func assign(ctx context.Context, c *caller, choreID int64, userIDs []int64, a Args) (map[string]any, error) {
	if len(userIDs) == 0 {
		return nil, fmt.Errorf("user_ids must name at least one person")
	}
	created := []any{}
	var failed []string
	for _, uid := range userIDs {
		payloads, err := schedulePayloads(a, uid)
		if err != nil {
			return nil, err
		}
		for _, p := range payloads {
			out, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/chores/%d/schedules", choreID), nil, p)
			if err != nil {
				failed = append(failed, fmt.Sprintf("person %d: %v", uid, err))
				continue
			}
			created = append(created, out)
		}
	}
	res := map[string]any{"created": len(created), "schedules": created}
	if len(failed) > 0 {
		res["errors"] = failed
	}
	return res, nil
}

func householdTools() []*Tool {
	choreProps := map[string]any{
		"title":                str("Chore name, short (\"Fold laundry\")."),
		"description":          str("What to do, written for the kid (it's read aloud if audio is set up)."),
		"category":             enum("required = Must do, core = Every day (default), bonus = Bonus.", "required", "core", "bonus"),
		"icon":                 str("One emoji, e.g. 🧺."),
		"points_value":         integer("Points earned."),
		"missed_penalty_value": integer("Points lost if it ends the day undone (0 = none)."),
		"estimated_minutes":    integer("Rough time it takes."),
		"requires_approval":    boolean("A parent must approve before points are paid."),
		"requires_photo":       boolean("Needs a photo as proof (then waits for approval)."),
		"photo_source":         enum("Who provides the photo: child (default), external (a camera/automation), or both.", "child", "external", "both"),
	}

	return []*Tool{
		{
			Name:     "household_overview",
			Covers:   []string{"GET /users", "GET /chores", "GET /chores/{}/schedules", "GET /chores/triggerable"},
			Title:    "Household overview",
			ReadOnly: true,
			Description: "The best first call. Lists people, then every chore with its settings and who does it when, " +
				"grouped into plain language (\"Zoe: Mon, Wed, Fri, due 09:00\"), plus any triggers. " +
				"Also flags chores nobody is assigned and chores with near-identical names, for cleanup.",
			Schema: obj(map[string]any{
				"person_id":            integer("Only show chores assigned to this person."),
				"include_ended":        boolean("Also show schedules that have ended (default false)."),
				"include_descriptions": boolean("Include chore descriptions (default false, to keep it short)."),
			}),
			Run: overview,
		},
		{
			Name:   "create_chore",
			Covers: []string{"POST /chores", "POST /chores/{}/schedules"},
			Title:  "Create a chore",
			Description: "Create a chore and, optionally, assign it in the same step: pass user_ids plus a pattern " +
				"(days, recurrence_interval or specific_date) and any timing. Check household_overview first so you don't duplicate an existing chore.",
			Schema: obj(merge(choreProps, map[string]any{
				"user_ids": ids("People to assign it to now (optional)."),
			}, patternProps), "title"),
			Run: func(ctx context.Context, c *caller, a Args) (any, error) {
				body := a.pick("title", "description", "category", "icon", "points_value", "missed_penalty_value",
					"estimated_minutes", "requires_approval", "requires_photo", "photo_source")
				out, err := c.do(ctx, http.MethodPost, "/chores", nil, body)
				if err != nil {
					return nil, err
				}
				if !a.Has("user_ids") {
					return map[string]any{"chore": out}, nil
				}
				chore, _ := out.(map[string]any)
				id, _ := Args(chore).Int("id")
				uids, err := a.Ints("user_ids")
				if err != nil {
					return nil, err
				}
				res, err := assign(ctx, c, id, uids, a)
				if err != nil {
					return map[string]any{"chore": out, "assignment_error": err.Error()}, nil
				}
				return map[string]any{"chore": out, "assignment": res}, nil
			},
		},
		{
			Name:   "update_chore",
			Covers: []string{"GET /chores/{}", "PUT /chores/{}"},
			Title:  "Edit a chore",
			Description: "Change a chore's definition. Only the fields you pass change; everything else is kept " +
				"(including approval/photo settings). This doesn't change who does it or when (see assign_chore).",
			Schema: obj(merge(map[string]any{"id": integer("Chore id.")}, choreProps), "id"),
			Run: func(ctx context.Context, c *caller, a Args) (any, error) {
				id, _ := a.Int("id")
				// The REST update overwrites the booleans, so start from the
				// current chore and overlay what was asked for.
				var cur map[string]any
				if err := c.getInto(ctx, fmt.Sprintf("/chores/%d", id), nil, &cur); err != nil {
					return nil, err
				}
				body := map[string]any{}
				for _, k := range []string{"title", "description", "category", "icon", "points_value", "missed_penalty_value",
					"estimated_minutes", "requires_approval", "requires_photo", "photo_source"} {
					if v, ok := cur[k]; ok {
						body[k] = v
					}
				}
				for k, v := range a {
					if k != "id" {
						body[k] = v
					}
				}
				if s, _ := a["description"].(string); a.Has("description") && s == "" {
					return nil, fmt.Errorf("a description can't be cleared, only replaced")
				}
				return c.do(ctx, http.MethodPut, fmt.Sprintf("/chores/%d", id), nil, body)
			},
		},
		{
			Name:   "assign_chore",
			Covers: []string{"POST /chores/{}/schedules"},
			Title:  "Assign a chore",
			Description: "Give an existing chore to one or more people on a pattern: weekdays (days), every N days " +
				"(recurrence_interval), or one date (specific_date), with optional times and late rules. " +
				"Adds to any existing assignments; to change one, unassign_chore first.",
			Schema: obj(merge(map[string]any{
				"chore_id": integer("Chore id."),
				"user_ids": ids("People to assign it to."),
			}, patternProps), "chore_id", "user_ids"),
			Run: func(ctx context.Context, c *caller, a Args) (any, error) {
				id, _ := a.Int("chore_id")
				uids, err := a.Ints("user_ids")
				if err != nil {
					return nil, err
				}
				return assign(ctx, c, id, uids, a)
			},
		},
		{
			Name:   "unassign_chore",
			Covers: []string{"GET /chores/{}/schedules", "PUT /schedules/{}", "DELETE /chores/{}/schedules/{}"},
			Title:  "Unassign a chore",
			Description: "Stop a chore for some or all of its people. mode \"retire\" (default) ends the schedules after last_day " +
				"(default yesterday, so it's gone from today) and keeps their history and reports; schedules that never started " +
				"are simply removed. mode \"delete\" removes them and their completion history. Narrow it with user_ids or schedule_ids.",
			Schema: obj(map[string]any{
				"chore_id":     integer("Chore id."),
				"user_ids":     ids("Only these people (default everyone)."),
				"schedule_ids": ids("Only these schedule rows."),
				"mode":         enum("retire (default, keeps history) or delete.", "retire", "delete"),
				"last_day":     str("For retire: the last day it still applies, YYYY-MM-DD (default yesterday)."),
			}, "chore_id"),
			Run: unassign,
		},
	}
}

func unassign(ctx context.Context, c *caller, a Args) (any, error) {
	choreID, _ := a.Int("chore_id")
	mode := a.Str("mode")
	if mode == "" {
		mode = "retire"
	}
	if mode != "retire" && mode != "delete" {
		return nil, fmt.Errorf("mode must be retire or delete")
	}
	lastDay := a.Str("last_day")
	if lastDay == "" {
		lastDay = time.Now().AddDate(0, 0, -1).Format(dateFormat)
	} else if _, err := time.Parse(dateFormat, lastDay); err != nil {
		return nil, fmt.Errorf("last_day must be YYYY-MM-DD")
	}

	var users, scheds map[int64]bool
	if a.Has("user_ids") {
		list, err := a.Ints("user_ids")
		if err != nil {
			return nil, err
		}
		users = set(list)
	}
	if a.Has("schedule_ids") {
		list, err := a.Ints("schedule_ids")
		if err != nil {
			return nil, err
		}
		scheds = set(list)
	}

	var schedules []apiSchedule
	if err := c.getInto(ctx, fmt.Sprintf("/chores/%d/schedules", choreID), nil, &schedules); err != nil {
		return nil, err
	}

	now := today()
	retired, deleted, skipped := []int64{}, []int64{}, []int64{}
	var failed []string
	for _, s := range schedules {
		if users != nil && !users[s.AssignedTo] || scheds != nil && !scheds[s.ID] {
			continue
		}
		del := mode == "delete"
		if !del {
			switch {
			case s.ended(now):
				skipped = append(skipped, s.ID) // already over
				continue
			case dateOnly(s.SpecificDate) != "" && s.RecurrenceInterval == nil && dateOnly(s.SpecificDate) > lastDay,
				dateOnly(s.StartDate) != "" && dateOnly(s.StartDate) > lastDay:
				del = true // never came up: nothing to keep
			case dateOnly(s.SpecificDate) != "" && s.RecurrenceInterval == nil:
				skipped = append(skipped, s.ID) // a one-off on or before last_day stays as history
				continue
			}
		}
		var err error
		if del {
			_, err = c.do(ctx, http.MethodDelete, fmt.Sprintf("/chores/%d/schedules/%d", choreID, s.ID), nil, nil)
			if err == nil {
				deleted = append(deleted, s.ID)
			}
		} else {
			_, err = c.do(ctx, http.MethodPut, fmt.Sprintf("/schedules/%d", s.ID), nil, map[string]any{"end_date": lastDay})
			if err == nil {
				retired = append(retired, s.ID)
			}
		}
		if err != nil {
			failed = append(failed, fmt.Sprintf("schedule %d: %v", s.ID, err))
		}
	}
	res := map[string]any{"retired": retired, "deleted": deleted, "already_ended_or_kept": skipped}
	if mode == "retire" {
		res["last_day"] = lastDay
	}
	if len(failed) > 0 {
		res["errors"] = failed
	}
	if len(retired)+len(deleted)+len(skipped)+len(failed) == 0 {
		res["note"] = "no matching schedules"
	}
	return res, nil
}

func set(list []int64) map[int64]bool {
	m := make(map[int64]bool, len(list))
	for _, v := range list {
		m[v] = true
	}
	return m
}

func updatePersonTool() *Tool {
	return &Tool{
		Name:   "update_person",
		Covers: []string{"PUT /users/{}"},
		Title:  "Edit a person",
		Description: "Change someone's name, role, age, look or colours; omitted fields are kept. " +
			"Making someone a parent requires them to have a PIN or linked account first; the last parent can't be demoted.",
		Schema: obj(map[string]any{
			"id":         integer("Person id."),
			"name":       str("Display name."),
			"role":       enum("admin (parent) or child.", "admin", "child"),
			"age":        integer("Age in years."),
			"avatar_url": str("Avatar image URL."),
			"theme":      enum("App look.", "sunroom", "blocks", "tint"),
			"color":      enum("Person colour.", "coral", "mint", "butter", "sky", "rose", "leaf", "lilac", "sand"),
		}, "id"),
		Run: func(ctx context.Context, c *caller, a Args) (any, error) {
			id, _ := a.Int("id")
			body := a.pick("name", "role", "age", "avatar_url", "theme", "color")
			if len(body) == 0 {
				return nil, fmt.Errorf("nothing to change")
			}
			return c.do(ctx, http.MethodPut, fmt.Sprintf("/users/%d", id), nil, body)
		},
	}
}

// --- overview ---

type assignmentGroup struct {
	key         string
	PersonID    int64   `json:"person_id"`
	Person      string  `json:"person"`
	When        string  `json:"when"`
	AvailableAt string  `json:"available_at,omitempty"`
	DueBy       string  `json:"due_by,omitempty"`
	IfLate      string  `json:"if_late,omitempty"`
	Multiplier  float64 `json:"points_multiplier,omitempty"`
	From        string  `json:"from,omitempty"`
	Until       string  `json:"until,omitempty"`
	Ended       bool    `json:"ended,omitempty"`
	Source      string  `json:"source,omitempty"`
	ScheduleIDs []int64 `json:"schedule_ids"`
	days        []int
}

func overview(ctx context.Context, c *caller, a Args) (any, error) {
	var users []apiUser
	if err := c.getInto(ctx, "/users", nil, &users); err != nil {
		return nil, err
	}
	var chores []apiChore
	if err := c.getInto(ctx, "/chores", nil, &chores); err != nil {
		return nil, err
	}
	var triggerable struct {
		Chores []apiTriggerable `json:"chores"`
	}
	if err := c.getInto(ctx, "/chores/triggerable", nil, &triggerable); err != nil {
		return nil, err
	}
	trigByChore := map[int64]apiTriggerable{}
	for _, t := range triggerable.Chores {
		trigByChore[t.ID] = t
	}

	names := map[int64]string{}
	people := make([]map[string]any, 0, len(users))
	for _, u := range users {
		names[u.ID] = u.Name
		p := map[string]any{"id": u.ID, "name": u.Name, "role": map[string]string{"admin": "parent", "child": "kid"}[u.Role]}
		if u.Age != nil {
			p["age"] = *u.Age
		}
		if u.Paused {
			p["paused"] = true
		}
		people = append(people, p)
	}

	personFilter, filtered := a.Int("person_id")
	includeEnded := a.Bool("include_ended")
	now := today()

	outChores := []map[string]any{}
	unassigned := []string{}
	byKey := map[string][]string{}
	for _, ch := range chores {
		var scheds []apiSchedule
		if err := c.getInto(ctx, fmt.Sprintf("/chores/%d/schedules", ch.ID), url.Values{}, &scheds); err != nil {
			return nil, err
		}
		groups := groupSchedules(scheds, names, now, includeEnded, personFilter, filtered)

		trig := trigByChore[ch.ID]
		active := 0
		for _, s := range scheds {
			if !s.ended(now) {
				active++
			}
		}
		if active == 0 && len(trig.Triggers) == 0 {
			unassigned = append(unassigned, fmt.Sprintf("%s (id %d)", ch.Title, ch.ID))
		}
		if filtered && len(groups) == 0 {
			continue
		}
		k := normalizeTitle(ch.Title)
		byKey[k] = append(byKey[k], fmt.Sprintf("%s (id %d)", ch.Title, ch.ID))

		entry := map[string]any{
			"id":       ch.ID,
			"title":    ch.Title,
			"icon":     ch.Icon,
			"category": fmt.Sprintf("%s (%s)", categoryLabels[ch.Category], ch.Category),
			"points":   ch.PointsValue,
		}
		if ch.MissedPenaltyValue != 0 {
			entry["missed_penalty"] = ch.MissedPenaltyValue
		}
		if ch.RequiresApproval {
			entry["needs_approval"] = true
		}
		if ch.RequiresPhoto {
			entry["needs_photo"] = ch.PhotoSource
		}
		if ch.EstimatedMinutes != nil {
			entry["minutes"] = *ch.EstimatedMinutes
		}
		if a.Bool("include_descriptions") && ch.Description != "" {
			entry["description"] = ch.Description
		}
		entry["assigned"] = groups
		if len(trig.Triggers) > 0 {
			var ts []map[string]any
			for _, t := range trig.Triggers {
				m := map[string]any{"id": t.ID, "uuid": t.UUID, "enabled": t.Enabled}
				if t.DefaultAssignedName != "" {
					m["default_person"] = t.DefaultAssignedName
				}
				ts = append(ts, m)
			}
			entry["triggers"] = ts
		}
		outChores = append(outChores, entry)
	}

	dupes := [][]string{}
	for _, v := range byKey {
		if len(v) > 1 {
			sort.Strings(v)
			dupes = append(dupes, v)
		}
	}
	sort.Slice(dupes, func(i, j int) bool { return dupes[i][0] < dupes[j][0] })

	res := map[string]any{
		"today":   fmt.Sprintf("%s (%s)", now, time.Now().Weekday()),
		"people":  people,
		"chores":  outChores,
		"cleanup": map[string]any{"not_assigned_to_anyone": unassigned, "possible_duplicates": dupes},
	}
	return res, nil
}

func groupSchedules(scheds []apiSchedule, names map[int64]string, now string, includeEnded bool, person int64, filtered bool) []*assignmentGroup {
	groups := []*assignmentGroup{}
	idx := map[string]*assignmentGroup{}
	for _, s := range scheds {
		if filtered && s.AssignedTo != person {
			continue
		}
		ended := s.ended(now)
		if ended && !includeEnded {
			continue
		}
		g := &assignmentGroup{
			PersonID:    s.AssignedTo,
			Person:      names[s.AssignedTo],
			AvailableAt: deref(s.AvailableAt),
			DueBy:       deref(s.DueBy),
			From:        dateOnly(s.StartDate),
			Until:       dateOnly(s.EndDate),
			Ended:       ended,
		}
		if g.Person == "" {
			g.Person = fmt.Sprintf("person %d", s.AssignedTo)
		}
		if g.DueBy != "" {
			switch s.ExpiryPenalty {
			case "no_points":
				g.IfLate = "no points"
			case "penalty":
				g.IfLate = fmt.Sprintf("-%d points", s.ExpiryPenaltyValue)
			default:
				g.IfLate = "can't be done late"
			}
		}
		if s.PointsMultiplier != 0 && s.PointsMultiplier != 1 {
			g.Multiplier = s.PointsMultiplier
		}
		if s.FcfsGroupID != nil {
			g.Source = "trigger, first come first served"
		} else if s.AssignmentType == "fcfs" {
			g.Source = "first come first served"
		}
		kind := ""
		switch {
		case s.RecurrenceInterval != nil:
			kind = fmt.Sprintf("interval-%d", s.ID)
			g.When = fmt.Sprintf("every %d days from %s", *s.RecurrenceInterval, dateOnly(s.RecurrenceStart))
		case s.DayOfWeek != nil:
			kind = "weekly"
		case s.SpecificDate != nil:
			kind = fmt.Sprintf("date-%d", s.ID)
			g.When = "on " + dateOnly(s.SpecificDate)
		}
		g.key = fmt.Sprintf("%d|%s|%s|%s|%s|%d|%v|%s|%s|%v|%s", s.AssignedTo, kind, g.AvailableAt, g.DueBy,
			s.ExpiryPenalty, s.ExpiryPenaltyValue, s.PointsMultiplier, g.From, g.Until, ended, g.Source)

		if existing := idx[g.key]; existing != nil && kind == "weekly" {
			existing.ScheduleIDs = append(existing.ScheduleIDs, s.ID)
			existing.days = append(existing.days, *s.DayOfWeek)
			continue
		}
		g.ScheduleIDs = []int64{s.ID}
		if s.DayOfWeek != nil {
			g.days = []int{*s.DayOfWeek}
		}
		idx[g.key] = g
		groups = append(groups, g)
	}
	for _, g := range groups {
		if g.days != nil {
			g.When = describeDays(g.days)
		}
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Person < groups[j].Person })
	return groups
}

// describeDays turns weekday numbers into "every day", "weekdays",
// "Mon, Wed, Fri" and so on, listing Monday first.
func describeDays(days []int) string {
	has := map[int]bool{}
	for _, d := range days {
		has[d] = true
	}
	switch {
	case len(has) == 7:
		return "every day"
	case len(has) == 5 && !has[0] && !has[6]:
		return "weekdays"
	case len(has) == 2 && has[0] && has[6]:
		return "weekends"
	}
	var parts []string
	for _, d := range []int{1, 2, 3, 4, 5, 6, 0} {
		if has[d] {
			parts = append(parts, dayNames[d])
		}
	}
	return strings.Join(parts, ", ")
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// normalizeTitle folds a title for duplicate detection: case, punctuation,
// emoji and a plural "s" don't matter.
func normalizeTitle(t string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(t) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if len(s) > 3 {
		s = strings.TrimSuffix(s, "s")
	}
	return s
}
