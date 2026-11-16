package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func today() string { return time.Now().Format("2006-01-02") }

func daysFromNow(n int) string { return time.Now().AddDate(0, 0, n).Format("2006-01-02") }

func dailyChores(t *testing.T, env *testEnv, userID int, date string) []map[string]any {
	t.Helper()
	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/chores?view=daily&date=%s", userID, date), nil, childHeaders(userID), http.StatusOK)
	var chores []map[string]any
	decodeBody(t, resp, &chores)
	return chores
}

func balance(t *testing.T, env *testEnv, userID int) float64 {
	t.Helper()
	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", userID), nil, adminHeaders(), http.StatusOK)
	var pts map[string]any
	decodeBody(t, resp, &pts)
	return pts["balance"].(float64)
}

func TestOptionalCategoryAccepted(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	resp := env.expectStatus(t, "POST", "/api/chores", map[string]any{
		"title": "Practice piano", "category": "optional", "points_value": 5,
	}, adminHeaders(), http.StatusCreated)
	var c map[string]any
	decodeBody(t, resp, &c)
	if c["category"] != "optional" || c["points_value"].(float64) != 5 {
		t.Fatalf("unexpected chore: %v", c)
	}
}

// Optional points are paid straight away, without waiting for the day's
// Must do chores, and an undone optional chore doesn't block the bonus gate.
func TestOptionalChorePointsAndGates(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	date := today()

	mk := func(title, cat string, pts int) int {
		resp := env.expectStatus(t, "POST", "/api/chores", map[string]any{"title": title, "category": cat, "points_value": pts}, adminHeaders(), http.StatusCreated)
		var c map[string]any
		decodeBody(t, resp, &c)
		resp = env.expectStatus(t, "POST", fmt.Sprintf("/api/chores/%d/schedules", int(c["id"].(float64))), map[string]any{
			"assigned_to": kid, "specific_date": date,
		}, adminHeaders(), http.StatusCreated)
		var s map[string]any
		decodeBody(t, resp, &s)
		return int(s["id"].(float64))
	}
	required := mk("Brush teeth", "required", 1)
	optional := mk("Practice piano", "optional", 5)
	bonus := mk("Water plants", "bonus", 3)
	_ = mk("Read a book", "optional", 0) // left undone

	// Optional pays even though Must do isn't done.
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", optional), map[string]any{"completion_date": date}, childHeaders(kid), http.StatusCreated)
	if b := balance(t, env, kid); b != 5 {
		t.Fatalf("optional points: want 5, got %v", b)
	}

	// Finishing Must do opens the bonus gate even with an optional chore undone.
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", required), map[string]any{"completion_date": date}, childHeaders(kid), http.StatusCreated)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", bonus), map[string]any{"completion_date": date}, childHeaders(kid), http.StatusCreated)
	if b := balance(t, env, kid); b != 9 {
		t.Fatalf("after bonus: want 9, got %v", b)
	}

	// The day counts as complete for the streak.
	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/streak", kid), nil, childHeaders(kid), http.StatusOK)
	var streak map[string]any
	decodeBody(t, resp, &streak)
	if streak["current_streak"].(float64) != 1 {
		t.Fatalf("streak: want 1, got %v", streak["current_streak"])
	}
}

func TestOwnChoreLifecycle(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	h := childHeaders(kid)

	// Points and category in the body are ignored: always Optional, 0 points.
	in3Weeks := daysFromNow(21)
	resp := env.expectStatus(t, "POST", "/api/me/chores", map[string]any{
		"title": "  Pack for camp ", "icon": "🎒", "points_value": 100, "category": "required",
		"when": map[string]any{"specific_date": in3Weeks},
	}, h, http.StatusCreated)
	var own map[string]any
	decodeBody(t, resp, &own)
	id := int(own["id"].(float64))
	if own["title"] != "Pack for camp" || own["category"] != "optional" || own["points_value"].(float64) != 0 || own["source"] != "self" {
		t.Fatalf("unexpected own chore: %v", own)
	}

	// It shows on that day, not today.
	if got := dailyChores(t, env, kid, in3Weeks); len(got) != 1 || got[0]["title"] != "Pack for camp" {
		t.Fatalf("expected task on %s, got %v", in3Weeks, got)
	}
	if got := dailyChores(t, env, kid, today()); len(got) != 0 {
		t.Fatalf("expected nothing today, got %v", got)
	}

	// Listed back.
	resp = env.expectStatus(t, "GET", "/api/me/chores", nil, h, http.StatusOK)
	var list []map[string]any
	decodeBody(t, resp, &list)
	if len(list) != 1 || len(list[0]["schedules"].([]any)) != 1 || list[0]["editable"] != true {
		t.Fatalf("unexpected list: %v", list)
	}

	// Reschedule to every day starting today.
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/me/chores/%d", id), map[string]any{
		"title": "Pack bag", "when": map[string]any{"days_of_week": []int{0, 1, 2, 3, 4, 5, 6}},
	}, h, http.StatusOK)
	got := dailyChores(t, env, kid, today())
	if len(got) != 1 || got[0]["title"] != "Pack bag" {
		t.Fatalf("expected weekly task today, got %v", got)
	}
	if got := dailyChores(t, env, kid, in3Weeks); len(got) != 1 {
		t.Fatalf("old one-off should be gone and weekly should show, got %v", got)
	}

	// Completing it earns nothing.
	sid := int(got[0]["schedule_id"].(float64))
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", sid), map[string]any{"completion_date": today()}, h, http.StatusCreated)
	if b := balance(t, env, kid); b != 0 {
		t.Fatalf("own chore paid points: %v", b)
	}

	// Another kid can't see, change or delete it.
	other := env.createChild(t, "Sib")
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/me/chores/%d", id), map[string]any{"title": "x"}, childHeaders(other), http.StatusNotFound)
	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/me/chores/%d", id), nil, childHeaders(other), http.StatusNotFound)
	resp = env.expectStatus(t, "GET", "/api/me/chores", nil, childHeaders(other), http.StatusOK)
	decodeBody(t, resp, &list)
	if len(list) != 0 {
		t.Fatalf("sibling sees own chores: %v", list)
	}

	// Kids still can't use the admin chore routes.
	env.expectStatus(t, "POST", "/api/chores", map[string]any{"title": "x", "category": "optional"}, h, http.StatusForbidden)

	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/me/chores/%d", id), nil, h, http.StatusNoContent)
	if got := dailyChores(t, env, kid, today()); len(got) != 0 {
		t.Fatalf("deleted task still shows: %v", got)
	}
}

func TestOwnChoreRepeatEveryFewDays(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	start := daysFromNow(2)
	env.expectStatus(t, "POST", "/api/me/chores", map[string]any{
		"title": "Water cactus", "when": map[string]any{"recurrence_interval": 3, "start_date": start},
	}, childHeaders(kid), http.StatusCreated)
	for offset, want := range map[int]int{0: 0, 2: 1, 3: 0, 5: 1, 8: 1} {
		if got := dailyChores(t, env, kid, daysFromNow(offset)); len(got) != want {
			t.Fatalf("day +%d: want %d, got %d", offset, want, len(got))
		}
	}
}

func TestOwnChoreWeeklyFutureStart(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	dow := int(time.Now().Weekday())
	env.expectStatus(t, "POST", "/api/me/chores", map[string]any{
		"title": "Swim", "when": map[string]any{"days_of_week": []int{dow}, "start_date": daysFromNow(7)},
	}, childHeaders(kid), http.StatusCreated)
	if got := dailyChores(t, env, kid, today()); len(got) != 0 {
		t.Fatalf("weekly task showed before its start: %v", got)
	}
	if got := dailyChores(t, env, kid, daysFromNow(7)); len(got) != 1 {
		t.Fatalf("weekly task missing on start day: %v", got)
	}
}

func TestOwnChoreValidation(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	h := childHeaders(kid)
	bad := []map[string]any{
		{"when": map[string]any{"specific_date": today()}}, // no title
		{"title": "x"}, // no when
		{"title": "x", "when": map[string]any{"specific_date": daysFromNow(-1)}},                   // past
		{"title": "x", "when": map[string]any{"specific_date": "next tuesday"}},                    // bad date
		{"title": "x", "when": map[string]any{"days_of_week": []int{7}}},                           // bad day
		{"title": "x", "when": map[string]any{"recurrence_interval": 0, "start_date": today()}},    // bad interval
		{"title": "x", "when": map[string]any{"specific_date": today(), "days_of_week": []int{1}}}, // two modes
		{"title": "x", "when": map[string]any{"days_of_week": []int{1}, "start_date": daysFromNow(5), "end_date": daysFromNow(2)}},
	}
	for i, body := range bad {
		resp := env.request(t, "POST", "/api/me/chores", body, h)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("case %d: want 400, got %d", i, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// Once a grown-up adds points, the kid can no longer manage the chore (so
// they can't put a points chore on their own schedule).
func TestOwnChoreLockedAfterParentAddsPoints(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	h := childHeaders(kid)
	resp := env.expectStatus(t, "POST", "/api/me/chores", map[string]any{
		"title": "Clean bike", "when": map[string]any{"specific_date": today()},
	}, h, http.StatusCreated)
	var own map[string]any
	decodeBody(t, resp, &own)
	id := int(own["id"].(float64))

	// Parents see kid-added chores and can give them points.
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/chores/%d", id), map[string]any{"points_value": 10}, adminHeaders(), http.StatusOK)

	env.expectStatus(t, "PUT", fmt.Sprintf("/api/me/chores/%d", id), map[string]any{
		"when": map[string]any{"days_of_week": []int{0, 1, 2, 3, 4, 5, 6}},
	}, h, http.StatusForbidden)
	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/me/chores/%d", id), nil, h, http.StatusForbidden)
	resp = env.expectStatus(t, "GET", "/api/me/chores", nil, h, http.StatusOK)
	var list []map[string]any
	decodeBody(t, resp, &list)
	if len(list) != 1 || list[0]["editable"] != false {
		t.Fatalf("expected locked chore in list: %v", list)
	}
}
