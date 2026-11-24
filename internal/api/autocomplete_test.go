package api_test

import (
	"net/http"
	"testing"
)

// A chore marked auto_complete is finished by another app through the API:
// the assignee can neither tick nor untick it, a parent or an API token can.
func TestAutoCompleteChore(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	env.request(t, "POST", "/api/users", map[string]any{"name": "Kid", "role": "child"}, adminHeaders())

	resp := env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Math practice", "category": "core", "points_value": 5, "auto_complete": true,
	}, adminHeaders())
	var chore map[string]any
	decodeBody(t, resp, &chore)
	if chore["auto_complete"] != true {
		t.Fatalf("created chore auto_complete = %v, want true", chore["auto_complete"])
	}
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{"assigned_to": 2, "day_of_week": 3}, adminHeaders())

	const day = "2026-03-11" // a Wednesday
	body := map[string]any{"completion_date": day}

	// The kid sees the flag and can't tick the chore.
	resp = env.request(t, "GET", "/api/users/2/chores?date="+day, nil, childHeaders(2))
	var list []map[string]any
	decodeBody(t, resp, &list)
	if len(list) != 1 || list[0]["auto_complete"] != true {
		t.Fatalf("kid's chore list = %v, want one auto_complete chore", list)
	}
	env.expectStatus(t, "POST", "/api/schedules/1/complete", body, childHeaders(2), http.StatusForbidden)

	// An API token completes it for the kid, approved, with points.
	resp = env.request(t, "POST", "/api/admin/tokens", map[string]any{"name": "learning app"}, adminHeaders())
	var tok map[string]any
	decodeBody(t, resp, &tok)
	token := map[string]string{"Authorization": "Bearer " + tok["token"].(string)}
	resp = env.request(t, "POST", "/api/schedules/1/complete", map[string]any{"completed_by": 2, "completion_date": day}, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("token complete = %d, want 201", resp.StatusCode)
	}
	var done map[string]any
	decodeBody(t, resp, &done)
	if done["status"] != "approved" || done["completed_by"] != float64(2) {
		t.Errorf("completion = %v, want approved for user 2", done)
	}
	if got := env.balance(t, 2); got != 5 {
		t.Errorf("balance = %d, want 5", got)
	}

	// The kid can't untick it either; a parent can.
	env.expectStatus(t, "DELETE", "/api/schedules/1/complete?date="+day, nil, childHeaders(2), http.StatusForbidden)
	env.expectStatus(t, "DELETE", "/api/schedules/1/complete?date="+day, nil, adminHeaders(), http.StatusNoContent)

	// An update that doesn't mention the flag (an older client) keeps it;
	// turning it off lets the kid tick the chore again.
	env.request(t, "PUT", "/api/chores/1", map[string]any{"title": "Math"}, adminHeaders())
	env.expectStatus(t, "POST", "/api/schedules/1/complete", body, childHeaders(2), http.StatusForbidden)
	env.request(t, "PUT", "/api/chores/1", map[string]any{"auto_complete": false}, adminHeaders())
	env.expectStatus(t, "POST", "/api/schedules/1/complete", body, childHeaders(2), http.StatusCreated)
}
