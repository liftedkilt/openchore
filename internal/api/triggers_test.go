package api_test

import (
	"fmt"
	"net/http"
	"testing"
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
