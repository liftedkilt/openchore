package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

// helper to create a chore via admin API and return its ID
func (e *testEnv) createChore(t *testing.T, title string) int {
	t.Helper()
	resp := e.request(t, "POST", "/api/chores", map[string]any{
		"title":    title,
		"category": "required",
	}, adminHeaders())
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to create chore: %d", resp.StatusCode)
	}
	var chore map[string]any
	decodeBody(t, resp, &chore)
	return int(chore["id"].(float64))
}

// helper to create a trigger via admin API and return the full response map
func (e *testEnv) createTrigger(t *testing.T, choreID int, data map[string]any) map[string]any {
	t.Helper()
	resp := e.request(t, "POST", fmt.Sprintf("/api/chores/%d/triggers", choreID), data, adminHeaders())
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to create trigger: %d", resp.StatusCode)
	}
	var trigger map[string]any
	decodeBody(t, resp, &trigger)
	return trigger
}

// ===== Admin CRUD Tests =====

func TestTriggerAdminCRUD(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	childID := env.createChild(t, "Alice")
	choreID := env.createChore(t, "Feed the cats")

	// List empty
	resp := env.request(t, "GET", fmt.Sprintf("/api/chores/%d/triggers", choreID), nil, adminHeaders())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var triggers []any
	decodeBody(t, resp, &triggers)
	if len(triggers) != 0 {
		t.Fatalf("expected 0 triggers, got %d", len(triggers))
	}

	// Create
	trigger := env.createTrigger(t, choreID, map[string]any{
		"default_assigned_to": childID,
		"cooldown_minutes":    15,
	})
	uuid := trigger["uuid"].(string)
	if len(uuid) != 32 {
		t.Errorf("expected 32-char UUID, got %d chars: %s", len(uuid), uuid)
	}
	if trigger["chore_id"].(float64) != float64(choreID) {
		t.Error("chore_id mismatch")
	}

	triggerID := int(trigger["id"].(float64))

	// List now has 1
	resp = env.request(t, "GET", fmt.Sprintf("/api/chores/%d/triggers", choreID), nil, adminHeaders())
	decodeBody(t, resp, &triggers)
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}

	// Update
	resp = env.request(t, "PUT", fmt.Sprintf("/api/triggers/%d", triggerID), map[string]any{
		"cooldown_minutes": 30,
		"enabled":          false,
	}, adminHeaders())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on update, got %d", resp.StatusCode)
	}

	// Delete
	resp = env.request(t, "DELETE", fmt.Sprintf("/api/triggers/%d", triggerID), nil, adminHeaders())
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 on delete, got %d", resp.StatusCode)
	}

	// Verify deleted
	resp = env.request(t, "GET", fmt.Sprintf("/api/chores/%d/triggers", choreID), nil, adminHeaders())
	decodeBody(t, resp, &triggers)
	if len(triggers) != 0 {
		t.Fatalf("expected 0 triggers after delete, got %d", len(triggers))
	}
}

func TestTriggerAdminRequiresAuth(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	choreID := env.createChore(t, "Test Chore")

	// No auth header
	resp := env.request(t, "GET", fmt.Sprintf("/api/chores/%d/triggers", choreID), nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", resp.StatusCode)
	}

	// Child user (non-admin)
	childID := env.createChild(t, "Bob")
	resp = env.request(t, "GET", fmt.Sprintf("/api/chores/%d/triggers", choreID), nil, childHeaders(childID))
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for child user, got %d", resp.StatusCode)
	}
}

func TestTriggerUUIDUnique(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	choreID := env.createChore(t, "Test Chore")

	t1 := env.createTrigger(t, choreID, nil)
	t2 := env.createTrigger(t, choreID, nil)

	if t1["uuid"].(string) == t2["uuid"].(string) {
		t.Error("expected different UUIDs for different triggers")
	}
}

// ===== Public Trigger Endpoint Tests =====

func TestFireTrigger_Success(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	childID := env.createChild(t, "Alice")
	choreID := env.createChore(t, "Feed the cats")

	trigger := env.createTrigger(t, choreID, map[string]any{
		"default_assigned_to": childID,
	})
	uuid := trigger["uuid"].(string)

	// Fire the trigger (no auth needed)
	resp := env.request(t, "POST", fmt.Sprintf("/api/hooks/trigger/%s", uuid), nil, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]any
	decodeBody(t, resp, &result)

	if result["message"] != "Chore triggered" {
		t.Errorf("unexpected message: %v", result["message"])
	}
	if result["assigned_to"] != "Alice" {
		t.Errorf("expected assigned_to=Alice, got %v", result["assigned_to"])
	}
	if result["chore"] != "Feed the cats" {
		t.Errorf("expected chore=Feed the cats, got %v", result["chore"])
	}
	if result["schedule_id"] == nil || result["schedule_id"].(float64) == 0 {
		t.Error("expected schedule_id to be set")
	}
}

func TestFireTrigger_UnknownUUID(t *testing.T) {
	env := setupTest(t)

	resp := env.request(t, "POST", "/api/hooks/trigger/nonexistent_uuid_1234567890", nil, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown UUID, got %d", resp.StatusCode)
	}
}

func TestFireTrigger_DisabledTrigger(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	childID := env.createChild(t, "Alice")
	choreID := env.createChore(t, "Test Chore")

	trigger := env.createTrigger(t, choreID, map[string]any{
		"default_assigned_to": childID,
	})
	triggerID := int(trigger["id"].(float64))
	uuid := trigger["uuid"].(string)

	// Disable trigger
	env.request(t, "PUT", fmt.Sprintf("/api/triggers/%d", triggerID), map[string]any{
		"enabled":             false,
		"default_assigned_to": childID,
	}, adminHeaders())

	// Try to fire
	resp := env.request(t, "POST", fmt.Sprintf("/api/hooks/trigger/%s", uuid), nil, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for disabled trigger, got %d", resp.StatusCode)
	}
}

func TestFireTrigger_CooldownEnforcement(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	childID := env.createChild(t, "Alice")
	choreID := env.createChore(t, "Test Chore")

	trigger := env.createTrigger(t, choreID, map[string]any{
		"default_assigned_to": childID,
		"cooldown_minutes":    60, // 60 minute cooldown
	})
	uuid := trigger["uuid"].(string)

	// First fire should succeed
	resp := env.request(t, "POST", fmt.Sprintf("/api/hooks/trigger/%s", uuid), nil, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first fire: expected 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Second fire should be rate limited
	resp = env.request(t, "POST", fmt.Sprintf("/api/hooks/trigger/%s", uuid), nil, nil)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for cooldown, got %d", resp.StatusCode)
	}
}

func TestFireTrigger_CooldownExpires(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	childID := env.createChild(t, "Alice")
	choreID := env.createChore(t, "Test Chore")

	// Create trigger with 0 cooldown
	trigger := env.createTrigger(t, choreID, map[string]any{
		"default_assigned_to": childID,
		"cooldown_minutes":    0,
	})
	uuid := trigger["uuid"].(string)

	// First fire
	resp := env.request(t, "POST", fmt.Sprintf("/api/hooks/trigger/%s", uuid), nil, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first fire: expected 201, got %d", resp.StatusCode)
	}

	// Second fire should be rejected as duplicate (same chore+user+today)
	resp = env.request(t, "POST", fmt.Sprintf("/api/hooks/trigger/%s", uuid), nil, nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("second fire (duplicate): expected 409, got %d", resp.StatusCode)
	}
}

func TestFireTrigger_AssignToOverride(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	childID := env.createChild(t, "Alice")
	env.createChild(t, "Bob")
	choreID := env.createChore(t, "Test Chore")

	trigger := env.createTrigger(t, choreID, map[string]any{
		"default_assigned_to": childID,
	})
	uuid := trigger["uuid"].(string)

	// Override with assign_to param
	resp := env.request(t, "POST", fmt.Sprintf("/api/hooks/trigger/%s?assign_to=Bob", uuid), nil, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var result map[string]any
	decodeBody(t, resp, &result)
	if result["assigned_to"] != "Bob" {
		t.Errorf("expected assigned_to=Bob, got %v", result["assigned_to"])
	}
}

func TestFireTrigger_AssignToCaseInsensitive(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	env.createChild(t, "Alice")
	choreID := env.createChore(t, "Test Chore")

	trigger := env.createTrigger(t, choreID, nil)
	uuid := trigger["uuid"].(string)

	// Case insensitive name lookup
	resp := env.request(t, "POST", fmt.Sprintf("/api/hooks/trigger/%s?assign_to=alice", uuid), nil, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for case-insensitive name, got %d", resp.StatusCode)
	}
	var result map[string]any
	decodeBody(t, resp, &result)
	if result["assigned_to"] != "Alice" {
		t.Errorf("expected assigned_to=Alice, got %v", result["assigned_to"])
	}
}

func TestFireTrigger_NoAssignee(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	choreID := env.createChore(t, "Test Chore")

	// Create trigger with no default assignee
	trigger := env.createTrigger(t, choreID, nil)
	uuid := trigger["uuid"].(string)

	// Fire without assign_to param
	resp := env.request(t, "POST", fmt.Sprintf("/api/hooks/trigger/%s", uuid), nil, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 when no assignee, got %d", resp.StatusCode)
	}
}

// ===== allow_repeat: post a chore more than once a day =====

func fireTriggerStatus(t *testing.T, env *testEnv, uuid string) int {
	t.Helper()
	resp := env.request(t, "POST", fmt.Sprintf("/api/hooks/trigger/%s", uuid), nil, nil)
	resp.Body.Close()
	return resp.StatusCode
}

func TestFireTrigger_SameDayDuplicateRejectedByDefault(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	env.createChild(t, "Alice")
	env.createChild(t, "Bob")
	choreID := env.createChore(t, "Put away laundry")

	individual := env.createTrigger(t, choreID, map[string]any{"default_assigned_to": env.createChild(t, "Cara")})
	if got := fireTriggerStatus(t, env, individual["uuid"].(string)); got != http.StatusCreated {
		t.Fatalf("individual first fire: expected 201, got %d", got)
	}
	if got := fireTriggerStatus(t, env, individual["uuid"].(string)); got != http.StatusConflict {
		t.Fatalf("individual second fire: expected 409, got %d", got)
	}

	fcfsChore := env.createChore(t, "Empty dryer")
	fcfs := env.createTrigger(t, fcfsChore, map[string]any{"assignment_type": "fcfs"})
	if got := fireTriggerStatus(t, env, fcfs["uuid"].(string)); got != http.StatusCreated {
		t.Fatalf("fcfs first fire: expected 201, got %d", got)
	}
	if got := fireTriggerStatus(t, env, fcfs["uuid"].(string)); got != http.StatusConflict {
		t.Fatalf("fcfs second fire: expected 409, got %d", got)
	}
}

func TestFireTrigger_AllowRepeat(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	aliceID := env.createChild(t, "Alice")
	env.createChild(t, "Bob")

	// FCFS: every fire makes a new group for all children
	choreID := env.createChore(t, "Put away laundry")
	fcfs := env.createTrigger(t, choreID, map[string]any{"assignment_type": "fcfs", "allow_repeat": true})
	if fcfs["allow_repeat"] != true {
		t.Fatalf("expected allow_repeat true in create response, got %v", fcfs["allow_repeat"])
	}
	for i := 0; i < 3; i++ {
		if got := fireTriggerStatus(t, env, fcfs["uuid"].(string)); got != http.StatusCreated {
			t.Fatalf("fcfs fire %d: expected 201, got %d", i+1, got)
		}
	}
	resp := env.request(t, "GET", fmt.Sprintf("/api/chores/%d/schedules", choreID), nil, adminHeaders())
	var schedules []map[string]any
	decodeBody(t, resp, &schedules)
	if len(schedules) != 6 { // 3 fires x 2 children
		t.Fatalf("expected 6 schedules, got %d", len(schedules))
	}
	groups := map[string]bool{}
	for _, s := range schedules {
		groups[s["fcfs_group_id"].(string)] = true
	}
	if len(groups) != 3 {
		t.Fatalf("expected 3 separate FCFS groups, got %d", len(groups))
	}

	// Individual: the same child can get it twice
	soloChore := env.createChore(t, "Fold towels")
	solo := env.createTrigger(t, soloChore, map[string]any{"default_assigned_to": aliceID, "allow_repeat": true})
	for i := 0; i < 2; i++ {
		if got := fireTriggerStatus(t, env, solo["uuid"].(string)); got != http.StatusCreated {
			t.Fatalf("individual fire %d: expected 201, got %d", i+1, got)
		}
	}

	// The flag survives a list and can be switched off with an update
	resp = env.request(t, "GET", fmt.Sprintf("/api/chores/%d/triggers", choreID), nil, adminHeaders())
	var listed []map[string]any
	decodeBody(t, resp, &listed)
	if len(listed) != 1 || listed[0]["allow_repeat"] != true {
		t.Fatalf("expected listed trigger with allow_repeat true, got %v", listed)
	}
	resp = env.request(t, "PUT", fmt.Sprintf("/api/triggers/%v", fcfs["id"]), map[string]any{"assignment_type": "fcfs", "allow_repeat": false}, adminHeaders())
	resp.Body.Close()
	if got := fireTriggerStatus(t, env, fcfs["uuid"].(string)); got != http.StatusConflict {
		t.Fatalf("after turning allow_repeat off: expected 409, got %d", got)
	}
}

// A first-come chore pays only the person who finishes it. The others get a
// shadow completion (approved, 0 points); opening their own core or bonus
// gate later must not pay them for it.
func TestFCFS_OnlyFinisherIsPaid(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	alice := env.createChild(t, "Alice")
	bob := env.createChild(t, "Bob")
	today := time.Now().Format(model.DateFormat)

	mkChore := func(title, category string, points int) int {
		resp := env.expectStatus(t, "POST", "/api/chores", map[string]any{
			"title": title, "category": category, "points_value": points,
		}, adminHeaders(), http.StatusCreated)
		var c map[string]any
		decodeBody(t, resp, &c)
		return int(c["id"].(float64))
	}

	// Bob has a Must-do chore today, so his core and bonus gates start closed.
	brush := mkChore("Brush teeth", "required", 0)
	resp := env.expectStatus(t, "POST", fmt.Sprintf("/api/chores/%d/schedules", brush), map[string]any{
		"assigned_to": bob, "specific_date": today,
	}, adminHeaders(), http.StatusCreated)
	var brushSched map[string]any
	decodeBody(t, resp, &brushSched)

	// Two first-come chores, offered to both kids.
	dryer := mkChore("Empty dryer", "core", 10)
	bins := mkChore("Bring in bins", "bonus", 7)
	for _, c := range []int{dryer, bins} {
		trig := env.createTrigger(t, c, map[string]any{"assignment_type": "fcfs"})
		if got := fireTriggerStatus(t, env, trig["uuid"].(string)); got != http.StatusCreated {
			t.Fatalf("fire trigger for chore %d: got %d", c, got)
		}
	}

	// Alice finishes both first.
	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/chores?view=daily&date=%s", alice, today), nil, adminHeaders(), http.StatusOK)
	var aliceChores []model.ScheduledChore
	decodeBody(t, resp, &aliceChores)
	for _, title := range []string{"Empty dryer", "Bring in bins"} {
		var id int64
		for _, c := range aliceChores {
			if c.Title == title {
				id = c.ScheduleID
			}
		}
		if id == 0 {
			t.Fatalf("Alice has no %q today", title)
		}
		env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", id), map[string]any{
			"completion_date": today,
		}, adminHeaders(), http.StatusCreated)
	}

	// Bob then does his Must-do chore, which opens his gates.
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%v/complete", brushSched["id"]), map[string]any{
		"completion_date": today,
	}, adminHeaders(), http.StatusCreated)

	if got := env.balance(t, alice); got != 17 {
		t.Errorf("Alice (finisher) balance = %d, want 17", got)
	}
	if got := env.balance(t, bob); got != 0 {
		t.Errorf("Bob (didn't finish) balance = %d, want 0", got)
	}
}

// fcfsSetup makes Alice and Bob, a first-come chore offered to both (fired
// for today), and returns each kid's copy of it.
func fcfsSetup(t *testing.T, env *testEnv, chore map[string]any) (alice, bob int, aliceSched, bobSched int64, today string) {
	t.Helper()
	env.createAdmin(t)
	alice = env.createChild(t, "Alice")
	bob = env.createChild(t, "Bob")
	today = time.Now().Format(model.DateFormat)
	resp := env.expectStatus(t, "POST", "/api/chores", chore, adminHeaders(), http.StatusCreated)
	var c map[string]any
	decodeBody(t, resp, &c)
	trig := env.createTrigger(t, int(c["id"].(float64)), map[string]any{"assignment_type": "fcfs"})
	if got := fireTriggerStatus(t, env, trig["uuid"].(string)); got != http.StatusCreated {
		t.Fatalf("fire trigger: got %d", got)
	}
	aliceSched = fcfsCopy(t, env, alice, today).ScheduleID
	bobSched = fcfsCopy(t, env, bob, today).ScheduleID
	return
}

// fcfsCopy returns the person's only chore today (their copy of the first-come chore).
func fcfsCopy(t *testing.T, env *testEnv, user int, today string) model.ScheduledChore {
	t.Helper()
	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/chores?view=daily&date=%s", user, today), nil, adminHeaders(), http.StatusOK)
	var chores []model.ScheduledChore
	decodeBody(t, resp, &chores)
	if len(chores) != 1 {
		t.Fatalf("user %d: expected 1 chore today, got %d", user, len(chores))
	}
	return chores[0]
}

// Unchecking a first-come chore from a kid who didn't finish it undoes the
// whole group, including the points the finisher got.
func TestFCFS_UncheckOtherCopyTakesBackFinishersPoints(t *testing.T) {
	env := setupTest(t)
	alice, bob, aliceSched, bobSched, today := fcfsSetup(t, env, map[string]any{
		"title": "Empty dryer", "category": "core", "points_value": 10,
	})
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", aliceSched), map[string]any{
		"completion_date": today,
	}, adminHeaders(), http.StatusCreated)
	if got := env.balance(t, alice); got != 10 {
		t.Fatalf("Alice balance after finishing = %d, want 10", got)
	}

	resp := env.request(t, "DELETE", fmt.Sprintf("/api/schedules/%d/complete?date=%s", bobSched, today), nil, adminHeaders())
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatalf("uncheck Bob's copy: got %d", resp.StatusCode)
	}
	if got := env.balance(t, alice); got != 0 {
		t.Errorf("Alice balance after unchecking = %d, want 0", got)
	}
	if got := env.balance(t, bob); got != 0 {
		t.Errorf("Bob balance = %d, want 0", got)
	}
	for _, u := range []int{alice, bob} {
		if c := fcfsCopy(t, env, u, today); c.Completed {
			t.Errorf("user %d's copy is still done after unchecking", u)
		}
	}
}

// A first-come chore that needs approval: once a parent approves the
// finisher's completion, the other kids' copies are done too (by the
// finisher, for 0 points).
func TestFCFS_ApprovalMarksOtherCopiesDone(t *testing.T) {
	env := setupTest(t)
	alice, bob, aliceSched, _, today := fcfsSetup(t, env, map[string]any{
		"title": "Empty dryer", "category": "core", "points_value": 10, "requires_approval": true,
	})
	resp := env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", aliceSched), map[string]any{
		"completion_date": today,
	}, childHeaders(alice), http.StatusCreated)
	var comp map[string]any
	decodeBody(t, resp, &comp)
	if comp["status"] != model.StatusPending {
		t.Fatalf("expected a pending completion, got %v", comp["status"])
	}
	if c := fcfsCopy(t, env, bob, today); c.Completed {
		t.Fatalf("Bob's copy is done before approval")
	}

	env.expectStatus(t, "POST", fmt.Sprintf("/api/completions/%v/approve", comp["id"]), nil, adminHeaders(), http.StatusNoContent)

	c := fcfsCopy(t, env, bob, today)
	if !c.Completed || c.CompletedByName != "Alice" {
		t.Errorf("Bob's copy after approval: completed=%v by %q, want done by Alice", c.Completed, c.CompletedByName)
	}
	if got := env.balance(t, alice); got != 10 {
		t.Errorf("Alice balance = %d, want 10", got)
	}
	if got := env.balance(t, bob); got != 0 {
		t.Errorf("Bob balance = %d, want 0", got)
	}
}

// A rejected first-come completion frees the chore: the other kids see it
// as open again and can do it themselves.
func TestFCFS_RejectionFreesChore(t *testing.T) {
	env := setupTest(t)
	alice, bob, aliceSched, bobSched, today := fcfsSetup(t, env, map[string]any{
		"title": "Empty dryer", "category": "core", "points_value": 10, "requires_approval": true,
	})
	resp := env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", aliceSched), map[string]any{
		"completion_date": today,
	}, childHeaders(alice), http.StatusCreated)
	var comp map[string]any
	decodeBody(t, resp, &comp)

	// While Alice's completion waits for approval, the chore is taken.
	if c := fcfsCopy(t, env, bob, today); !c.CompletedBySibling {
		t.Errorf("Bob's copy while Alice's waits: want it shown as taken by a sibling")
	}
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", bobSched), map[string]any{
		"completion_date": today,
	}, adminHeaders(), http.StatusConflict)

	env.expectStatus(t, "POST", fmt.Sprintf("/api/completions/%v/reject", comp["id"]), nil, adminHeaders(), http.StatusNoContent)

	if c := fcfsCopy(t, env, bob, today); c.Completed || c.CompletedBySibling {
		t.Fatalf("Bob's copy after rejection: completed=%v bySibling=%v, want open", c.Completed, c.CompletedBySibling)
	}
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", bobSched), map[string]any{
		"completion_date": today,
	}, adminHeaders(), http.StatusCreated)

	if got := env.balance(t, bob); got != 10 {
		t.Errorf("Bob balance = %d, want 10", got)
	}
	if got := env.balance(t, alice); got != 0 {
		t.Errorf("Alice balance = %d, want 0", got)
	}
	if c := fcfsCopy(t, env, alice, today); !c.Completed || c.CompletedByName != "Bob" {
		t.Errorf("Alice's copy: completed=%v by %q, want done by Bob", c.Completed, c.CompletedByName)
	}
	// Bob's own copy is his, not a sibling's (Alice's copy now holds a
	// 0-point completion by Bob next to her rejected try).
	if c := fcfsCopy(t, env, bob, today); !c.Completed || c.CompletedBySibling {
		t.Errorf("Bob's own copy: completed=%v bySibling=%v, want done by him", c.Completed, c.CompletedBySibling)
	}
}

// Excusing one kid's copy of a first-come chore doesn't take it from the others.
func TestFCFS_ExcusedCopyDoesNotTakeChore(t *testing.T) {
	env := setupTest(t)
	_, bob, aliceSched, bobSched, today := fcfsSetup(t, env, map[string]any{
		"title": "Empty dryer", "category": "core", "points_value": 10,
	})
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/excuse", aliceSched), map[string]any{
		"date": today, "reason": "sick",
	}, adminHeaders(), http.StatusCreated)

	if c := fcfsCopy(t, env, bob, today); c.CompletedBySibling {
		t.Errorf("Bob's copy shows as taken after Alice was excused")
	}
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", bobSched), map[string]any{
		"completion_date": today,
	}, adminHeaders(), http.StatusCreated)
	if got := env.balance(t, bob); got != 10 {
		t.Errorf("Bob balance = %d, want 10", got)
	}
}

// A kid can't undo a first-come chore a sibling did by unchecking their own
// copy (it would take the sibling's points); a parent still can.
func TestFCFS_KidCannotUndoSiblingsChore(t *testing.T) {
	env := setupTest(t)
	alice, bob, aliceSched, bobSched, today := fcfsSetup(t, env, map[string]any{
		"title": "Empty dryer", "category": "core", "points_value": 10,
	})
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", aliceSched), map[string]any{
		"completion_date": today,
	}, childHeaders(alice), http.StatusCreated)

	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/schedules/%d/complete?date=%s", bobSched, today), nil, childHeaders(bob), http.StatusForbidden)
	if got := env.balance(t, alice); got != 10 {
		t.Errorf("Alice balance after Bob's tap = %d, want 10", got)
	}
	if c := fcfsCopy(t, env, bob, today); !c.Completed {
		t.Errorf("Bob's copy was undone by his own tap")
	}

	// Alice's own copy isn't shown as done by a sibling (the kid app would
	// then refuse to let her undo it).
	if c := fcfsCopy(t, env, alice, today); !c.Completed || c.CompletedBySibling {
		t.Errorf("Alice's own copy: completed=%v bySibling=%v, want done by her, not a sibling", c.Completed, c.CompletedBySibling)
	}

	// Alice can undo her own.
	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/schedules/%d/complete?date=%s", aliceSched, today), nil, childHeaders(alice), http.StatusNoContent)
	if got := env.balance(t, alice); got != 0 {
		t.Errorf("Alice balance after undoing her own = %d, want 0", got)
	}
}

// After a rejection, "try again" only clears the kid's own attempt: if a
// sibling has since done the chore, it's theirs and stays done.
func TestFCFS_RetryAfterRejection(t *testing.T) {
	env := setupTest(t)
	alice, bob, aliceSched, bobSched, today := fcfsSetup(t, env, map[string]any{
		"title": "Empty dryer", "category": "core", "points_value": 10, "requires_approval": true,
	})
	try := func() any {
		resp := env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", aliceSched), map[string]any{
			"completion_date": today,
		}, childHeaders(alice), http.StatusCreated)
		var comp map[string]any
		decodeBody(t, resp, &comp)
		return comp["id"]
	}

	// Nobody else has it: Alice can clear her rejected try and go again.
	env.expectStatus(t, "POST", fmt.Sprintf("/api/completions/%v/reject", try()), nil, adminHeaders(), http.StatusNoContent)
	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/schedules/%d/complete?date=%s", aliceSched, today), nil, childHeaders(alice), http.StatusNoContent)
	second := try()

	// Rejected again, and this time Bob does it.
	env.expectStatus(t, "POST", fmt.Sprintf("/api/completions/%v/reject", second), nil, adminHeaders(), http.StatusNoContent)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", bobSched), map[string]any{
		"completion_date": today,
	}, adminHeaders(), http.StatusCreated)

	// Alice's "try again" can't undo Bob's.
	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/schedules/%d/complete?date=%s", aliceSched, today), nil, childHeaders(alice), http.StatusForbidden)
	if got := env.balance(t, bob); got != 10 {
		t.Errorf("Bob balance = %d, want 10", got)
	}
	if c := fcfsCopy(t, env, bob, today); !c.Completed {
		t.Errorf("Bob's copy was undone")
	}
}
