package api_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	msqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/liftedkilt/openchore/internal/api"
	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
	"github.com/liftedkilt/openchore/internal/webhook"
	"github.com/liftedkilt/openchore/migrations"
)

type testEnv struct {
	server *httptest.Server
	db     *sql.DB
}

func setupTest(t *testing.T) *testEnv {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	db.SetMaxOpenConns(1)

	driver, err := msqlite.WithInstance(db, &msqlite.Config{})
	if err != nil {
		t.Fatalf("failed to create migration driver: %v", err)
	}
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatalf("failed to create migration source: %v", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "sqlite", driver)
	if err != nil {
		t.Fatalf("failed to create migrator: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("failed to run migrations: %v", err)
	}

	s := store.New(db)
	d := webhook.NewDispatcher(s)
	router, _, _ := api.NewRouter(s, d)
	server := httptest.NewServer(router)

	t.Cleanup(func() {
		server.Close()
		db.Close()
	})

	return &testEnv{server: server, db: db}
}

func (e *testEnv) request(t *testing.T, method, path string, body any, headers map[string]string) *http.Response {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.server.URL+path, reqBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}

func decodeBody(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

func (e *testEnv) createAdmin(t *testing.T) map[string]any {
	t.Helper()
	// Insert admin directly to bootstrap
	_, err := e.db.Exec(`INSERT INTO users (name, avatar_url, role) VALUES ('Admin', '', 'admin')`)
	if err != nil {
		t.Fatalf("failed to create admin: %v", err)
	}
	return map[string]any{"id": float64(1), "name": "Admin", "role": "admin"}
}

func adminHeaders() map[string]string {
	return map[string]string{"X-User-ID": "1"}
}

func childHeaders(id int) map[string]string {
	return map[string]string{"X-User-ID": fmt.Sprintf("%d", id)}
}

func (e *testEnv) createChild(t *testing.T, name string) int {
	t.Helper()
	resp := e.request(t, "POST", "/api/users", map[string]any{
		"name": name,
		"role": "child",
	}, adminHeaders())
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to create child: %d", resp.StatusCode)
	}
	var user map[string]any
	decodeBody(t, resp, &user)
	return int(user["id"].(float64))
}

// expectStatus is a helper that checks response status and returns it for further use.
func (e *testEnv) expectStatus(t *testing.T, method, path string, body any, headers map[string]string, expected int) *http.Response {
	t.Helper()
	resp := e.request(t, method, path, body, headers)
	if resp.StatusCode != expected {
		t.Fatalf("%s %s: expected %d, got %d", method, path, expected, resp.StatusCode)
	}
	return resp
}

func TestListUsersEmpty(t *testing.T) {
	env := setupTest(t)
	resp := env.request(t, "GET", "/api/users", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var users []any
	decodeBody(t, resp, &users)
	if len(users) != 0 {
		t.Fatalf("expected empty list, got %d users", len(users))
	}
}

func TestCreateAndGetUser(t *testing.T) {
	env := setupTest(t)
	admin := env.createAdmin(t)
	_ = admin

	resp := env.request(t, "POST", "/api/users", map[string]any{
		"name": "Kid One",
		"role": "child",
	}, adminHeaders())

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var user map[string]any
	decodeBody(t, resp, &user)
	if user["name"] != "Kid One" {
		t.Fatalf("expected name 'Kid One', got %v", user["name"])
	}
	if user["role"] != "child" {
		t.Fatalf("expected role 'child', got %v", user["role"])
	}

	// Verify we can get the user back
	resp = env.request(t, "GET", "/api/users/2", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestCreateUserRequiresAdmin(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Create a child user first
	env.request(t, "POST", "/api/users", map[string]any{
		"name": "Kid",
		"role": "child",
	}, adminHeaders())

	// Try to create user as child (user ID 2)
	resp := env.request(t, "POST", "/api/users", map[string]any{
		"name": "Another Kid",
		"role": "child",
	}, map[string]string{"X-User-ID": "2"})

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestChoreCRUD(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Create
	resp := env.request(t, "POST", "/api/chores", map[string]any{
		"title":    "Feed the cats",
		"category": "required",
		"icon":     "cat",
	}, adminHeaders())
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var chore map[string]any
	decodeBody(t, resp, &chore)
	if chore["title"] != "Feed the cats" {
		t.Fatalf("unexpected title: %v", chore["title"])
	}

	// List
	resp = env.request(t, "GET", "/api/chores", nil, adminHeaders())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var chores []any
	decodeBody(t, resp, &chores)
	if len(chores) != 1 {
		t.Fatalf("expected 1 chore, got %d", len(chores))
	}

	// Update
	resp = env.request(t, "PUT", "/api/chores/1", map[string]any{
		"title": "Feed the cats (morning)",
	}, adminHeaders())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var updated map[string]any
	decodeBody(t, resp, &updated)
	if updated["title"] != "Feed the cats (morning)" {
		t.Fatalf("title not updated: %v", updated["title"])
	}

	// Delete
	resp = env.request(t, "DELETE", "/api/chores/1", nil, adminHeaders())
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Verify deleted
	resp = env.request(t, "GET", "/api/chores", nil, adminHeaders())
	decodeBody(t, resp, &chores)
	if len(chores) != 0 {
		t.Fatalf("expected 0 chores after delete, got %d", len(chores))
	}
}

// TestChoreUpdatePointsAndPenaltyZeroing verifies that points_value and
// missed_penalty_value can be explicitly set to 0 via a PUT, and that a
// partial update that omits those fields leaves them untouched.
func TestChoreUpdatePointsAndPenaltyZeroing(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Create a chore with non-zero points and penalty.
	resp := env.request(t, "POST", "/api/chores", map[string]any{
		"title":                "Take out trash",
		"category":             "core",
		"points_value":         10,
		"missed_penalty_value": 5,
	}, adminHeaders())
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var chore map[string]any
	decodeBody(t, resp, &chore)
	choreID := int(chore["id"].(float64))

	// Partial update: title only. Points and penalty should be preserved.
	resp = env.request(t, "PUT", fmt.Sprintf("/api/chores/%d", choreID), map[string]any{
		"title": "Take out trash (evening)",
	}, adminHeaders())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("partial update: expected 200, got %d", resp.StatusCode)
	}
	decodeBody(t, resp, &chore)
	if chore["points_value"].(float64) != 10 {
		t.Errorf("partial update clobbered points_value: got %v, want 10", chore["points_value"])
	}
	if chore["missed_penalty_value"].(float64) != 5 {
		t.Errorf("partial update clobbered missed_penalty_value: got %v, want 5", chore["missed_penalty_value"])
	}

	// Explicit zero: both fields should be cleared.
	resp = env.request(t, "PUT", fmt.Sprintf("/api/chores/%d", choreID), map[string]any{
		"points_value":         0,
		"missed_penalty_value": 0,
	}, adminHeaders())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("zero update: expected 200, got %d", resp.StatusCode)
	}
	decodeBody(t, resp, &chore)
	if chore["points_value"].(float64) != 0 {
		t.Errorf("explicit zero was ignored for points_value: got %v, want 0", chore["points_value"])
	}
	if chore["missed_penalty_value"].(float64) != 0 {
		t.Errorf("explicit zero was ignored for missed_penalty_value: got %v, want 0", chore["missed_penalty_value"])
	}
}

func TestScheduleAndComplete(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Create a child
	env.request(t, "POST", "/api/users", map[string]any{
		"name": "Kid",
		"role": "child",
	}, adminHeaders())

	// Create a chore
	env.request(t, "POST", "/api/chores", map[string]any{
		"title":    "Take out trash",
		"category": "core",
	}, adminHeaders())

	// Schedule it for Wednesday (day 3) for Kid (user 2)
	resp := env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": 2,
		"day_of_week": 3,
	}, adminHeaders())
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var schedule map[string]any
	decodeBody(t, resp, &schedule)
	scheduleID := schedule["id"].(float64)

	// Complete it (as admin acting for kid)
	resp = env.request(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    2,
		"completion_date": "2026-03-11", // a Wednesday
	}, adminHeaders())
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	// Try completing again — should conflict
	resp = env.request(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    2,
		"completion_date": "2026-03-11",
	}, adminHeaders())
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}

	// Uncomplete
	resp = env.request(t, "DELETE", "/api/schedules/1/complete?date=2026-03-11", nil, adminHeaders())
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	_ = scheduleID
}

func TestTimeLockEnforcement(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Create child
	env.request(t, "POST", "/api/users", map[string]any{
		"name": "Kid",
		"role": "child",
	}, adminHeaders())

	// Create chore
	env.request(t, "POST", "/api/chores", map[string]any{
		"title":    "Feed cats evening meal",
		"category": "required",
	}, adminHeaders())

	// Schedule with available_at far in the future (23:59)
	resp := env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to":  2,
		"day_of_week":  3,
		"available_at": "23:59",
	}, adminHeaders())
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	// Try to complete — should be rejected due to time lock
	resp = env.request(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    2,
		"completion_date": "2026-03-11",
	}, adminHeaders())
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for time-locked chore, got %d", resp.StatusCode)
	}

	var errResp map[string]string
	decodeBody(t, resp, &errResp)
	if errResp["error"] == "" {
		t.Fatal("expected error message for time lock")
	}
}

func TestTimeLockAllowsWhenPast(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.request(t, "POST", "/api/users", map[string]any{
		"name": "Kid",
		"role": "child",
	}, adminHeaders())

	env.request(t, "POST", "/api/chores", map[string]any{
		"title":    "Morning chore",
		"category": "core",
	}, adminHeaders())

	// Schedule with available_at in the past (00:00)
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to":  2,
		"day_of_week":  3,
		"available_at": "00:00",
	}, adminHeaders())

	// Should succeed since 00:00 is always in the past
	resp := env.request(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    2,
		"completion_date": "2026-03-11",
	}, adminHeaders())
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for past time lock, got %d", resp.StatusCode)
	}
}

func TestGetUserChoresDaily(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Create child
	env.request(t, "POST", "/api/users", map[string]any{
		"name": "Kid",
		"role": "child",
	}, adminHeaders())

	// Create two chores
	env.request(t, "POST", "/api/chores", map[string]any{
		"title":    "Chore A",
		"category": "required",
	}, adminHeaders())
	env.request(t, "POST", "/api/chores", map[string]any{
		"title":    "Chore B",
		"category": "bonus",
	}, adminHeaders())

	// Schedule Chore A for Wednesday (2026-03-11 is a Wednesday, day_of_week=3)
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": 2,
		"day_of_week": 3,
	}, adminHeaders())

	// Schedule Chore B as one-off on 2026-03-11
	env.request(t, "POST", "/api/chores/2/schedules", map[string]any{
		"assigned_to":   2,
		"specific_date": "2026-03-11",
	}, adminHeaders())

	// Get daily view for that Wednesday
	resp := env.request(t, "GET", "/api/users/2/chores?view=daily&date=2026-03-11", nil, adminHeaders())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var chores []map[string]any
	decodeBody(t, resp, &chores)
	if len(chores) != 2 {
		t.Fatalf("expected 2 chores for Wednesday, got %d", len(chores))
	}

	// Different day should only show recurring chore if it matches
	resp = env.request(t, "GET", "/api/users/2/chores?view=daily&date=2026-03-12", nil, adminHeaders())
	decodeBody(t, resp, &chores)
	if len(chores) != 0 {
		t.Fatalf("expected 0 chores for Thursday, got %d", len(chores))
	}
}

func TestGetUserChoresWeekly(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.request(t, "POST", "/api/users", map[string]any{
		"name": "Kid",
		"role": "child",
	}, adminHeaders())

	env.request(t, "POST", "/api/chores", map[string]any{
		"title":    "Daily chore",
		"category": "core",
	}, adminHeaders())

	// Schedule for Monday (1) and Friday (5)
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": 2,
		"day_of_week": 1,
	}, adminHeaders())
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": 2,
		"day_of_week": 5,
	}, adminHeaders())

	// Weekly view for week of 2026-03-09 (Monday)
	resp := env.request(t, "GET", "/api/users/2/chores?view=weekly&date=2026-03-09", nil, adminHeaders())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var chores []map[string]any
	decodeBody(t, resp, &chores)
	if len(chores) != 2 {
		t.Fatalf("expected 2 chores (Mon+Fri), got %d", len(chores))
	}
}

func TestNoAuthRequired(t *testing.T) {
	env := setupTest(t)

	// List users doesn't require auth
	resp := env.request(t, "GET", "/api/users", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for public endpoint, got %d", resp.StatusCode)
	}

	// Creating chores requires auth
	resp = env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Test",
	}, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for protected endpoint without auth, got %d", resp.StatusCode)
	}
}

func TestInvalidCategory(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	resp := env.request(t, "POST", "/api/chores", map[string]any{
		"title":    "Bad chore",
		"category": "invalid",
	}, adminHeaders())
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid category, got %d", resp.StatusCode)
	}
}

func TestCompletionShowsInDailyView(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.request(t, "POST", "/api/users", map[string]any{
		"name": "Kid",
		"role": "child",
	}, adminHeaders())

	env.request(t, "POST", "/api/chores", map[string]any{
		"title":    "Sweep floor",
		"category": "core",
	}, adminHeaders())

	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": 2,
		"day_of_week": 3, // Wednesday
	}, adminHeaders())

	// Complete it
	env.request(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    2,
		"completion_date": "2026-03-11",
	}, adminHeaders())

	// Daily view should show completed=true
	resp := env.request(t, "GET", "/api/users/2/chores?view=daily&date=2026-03-11", nil, adminHeaders())
	var chores []map[string]any
	decodeBody(t, resp, &chores)
	if len(chores) != 1 {
		t.Fatalf("expected 1 chore, got %d", len(chores))
	}
	if chores[0]["completed"] != true {
		t.Fatal("expected chore to show as completed")
	}
	if chores[0]["completion_id"] == nil {
		t.Fatal("expected completion_id to be set")
	}
}

// =================== POINTS TESTS ===================

func TestPointsCreditedOnCompletion(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Create a chore worth 10 points
	env.request(t, "POST", "/api/chores", map[string]any{
		"title":        "Wash dishes",
		"category":     "bonus",
		"points_value": 10,
	}, adminHeaders())

	// Schedule for Wednesday
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID,
		"day_of_week": 3,
	}, adminHeaders())

	// Complete it
	env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    kidID,
		"completion_date": "2026-03-11",
	}, adminHeaders(), http.StatusCreated)

	// Check points balance
	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, adminHeaders(), http.StatusOK)
	var pts map[string]any
	decodeBody(t, resp, &pts)
	if pts["balance"].(float64) != 10 {
		t.Fatalf("expected balance 10, got %v", pts["balance"])
	}

	// Check transactions list
	txs := pts["transactions"].([]any)
	if len(txs) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(txs))
	}
}

func TestPointsDebitedOnUncomplete(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title":        "Chore",
		"category":     "bonus",
		"points_value": 5,
	}, adminHeaders())
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID,
		"day_of_week": 3,
	}, adminHeaders())

	// Complete then uncomplete
	env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    kidID,
		"completion_date": "2026-03-11",
	}, adminHeaders(), http.StatusCreated)
	env.expectStatus(t, "DELETE", "/api/schedules/1/complete?date=2026-03-11", nil, adminHeaders(), http.StatusNoContent)

	// Balance should be 0
	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, adminHeaders(), http.StatusOK)
	var pts map[string]any
	decodeBody(t, resp, &pts)
	if pts["balance"].(float64) != 0 {
		t.Fatalf("expected balance 0 after uncomplete, got %v", pts["balance"])
	}
}

func TestAdminPointsAdjustment(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Admin adjusts points
	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{
		"user_id": kidID,
		"amount":  25,
		"note":    "bonus for being great",
	}, adminHeaders(), http.StatusNoContent)

	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, adminHeaders(), http.StatusOK)
	var pts map[string]any
	decodeBody(t, resp, &pts)
	if pts["balance"].(float64) != 25 {
		t.Fatalf("expected 25, got %v", pts["balance"])
	}

	// Negative adjustment
	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{
		"user_id": kidID,
		"amount":  -10,
		"note":    "penalty",
	}, adminHeaders(), http.StatusNoContent)

	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &pts)
	if pts["balance"].(float64) != 15 {
		t.Fatalf("expected 15, got %v", pts["balance"])
	}
}

func TestAdminPointsAdjustValidation(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Zero amount should fail
	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{
		"user_id": 1,
		"amount":  0,
	}, adminHeaders(), http.StatusBadRequest)

	// Missing user_id
	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{
		"amount": 10,
	}, adminHeaders(), http.StatusBadRequest)
}

func TestGetAllBalances(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid1 := env.createChild(t, "Kid1")
	kid2 := env.createChild(t, "Kid2")

	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{
		"user_id": kid1, "amount": 100, "note": "test",
	}, adminHeaders(), http.StatusNoContent)
	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{
		"user_id": kid2, "amount": 50, "note": "test",
	}, adminHeaders(), http.StatusNoContent)

	resp := env.expectStatus(t, "GET", "/api/points/balances", nil, adminHeaders(), http.StatusOK)
	var balances []map[string]any
	decodeBody(t, resp, &balances)
	if len(balances) < 2 {
		t.Fatalf("expected at least 2 balances, got %d", len(balances))
	}
}

// =================== REWARDS TESTS ===================

func TestRewardCRUD(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Create
	resp := env.expectStatus(t, "POST", "/api/rewards", map[string]any{
		"name": "Extra Screen Time",
		"cost": 50,
		"icon": "📺",
	}, adminHeaders(), http.StatusCreated)
	var reward map[string]any
	decodeBody(t, resp, &reward)
	if reward["name"] != "Extra Screen Time" {
		t.Fatalf("unexpected name: %v", reward["name"])
	}
	if reward["cost"].(float64) != 50 {
		t.Fatalf("unexpected cost: %v", reward["cost"])
	}

	// List all (admin)
	resp = env.expectStatus(t, "GET", "/api/rewards/all", nil, adminHeaders(), http.StatusOK)
	var rewards []map[string]any
	decodeBody(t, resp, &rewards)
	if len(rewards) != 1 {
		t.Fatalf("expected 1 reward, got %d", len(rewards))
	}

	// Update
	resp = env.expectStatus(t, "PUT", "/api/rewards/1", map[string]any{
		"cost": 75,
	}, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &reward)
	if reward["cost"].(float64) != 75 {
		t.Fatalf("expected updated cost 75, got %v", reward["cost"])
	}

	// Delete
	env.expectStatus(t, "DELETE", "/api/rewards/1", nil, adminHeaders(), http.StatusNoContent)
	resp = env.expectStatus(t, "GET", "/api/rewards/all", nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &rewards)
	if len(rewards) != 0 {
		t.Fatalf("expected 0 rewards after delete, got %d", len(rewards))
	}
}

func TestRewardValidation(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Missing name
	env.expectStatus(t, "POST", "/api/rewards", map[string]any{
		"cost": 10,
	}, adminHeaders(), http.StatusBadRequest)

	// Zero cost
	env.expectStatus(t, "POST", "/api/rewards", map[string]any{
		"name": "Free",
		"cost": 0,
	}, adminHeaders(), http.StatusBadRequest)

	// Negative cost
	env.expectStatus(t, "POST", "/api/rewards", map[string]any{
		"name": "Negative",
		"cost": -5,
	}, adminHeaders(), http.StatusBadRequest)
}

func TestRedeemReward(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Give the kid some points
	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{
		"user_id": kidID, "amount": 100, "note": "seed",
	}, adminHeaders(), http.StatusNoContent)

	// Create a reward
	env.expectStatus(t, "POST", "/api/rewards", map[string]any{
		"name": "Ice Cream",
		"cost": 30,
		"icon": "🍦",
	}, adminHeaders(), http.StatusCreated)

	// Redeem as child
	resp := env.expectStatus(t, "POST", "/api/rewards/1/redeem", nil, childHeaders(kidID), http.StatusCreated)
	var redemption map[string]any
	decodeBody(t, resp, &redemption)
	if redemption["points_spent"].(float64) != 30 {
		t.Fatalf("expected 30 points spent, got %v", redemption["points_spent"])
	}

	// Check balance decreased
	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, childHeaders(kidID), http.StatusOK)
	var pts map[string]any
	decodeBody(t, resp, &pts)
	if pts["balance"].(float64) != 70 {
		t.Fatalf("expected balance 70, got %v", pts["balance"])
	}
}

func TestRedeemInsufficientPoints(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Kid has 0 points
	env.expectStatus(t, "POST", "/api/rewards", map[string]any{
		"name": "Expensive",
		"cost": 1000,
	}, adminHeaders(), http.StatusCreated)

	env.expectStatus(t, "POST", "/api/rewards/1/redeem", nil, childHeaders(kidID), http.StatusUnprocessableEntity)
}

func TestRedeemOutOfStock(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{
		"user_id": kidID, "amount": 1000, "note": "seed",
	}, adminHeaders(), http.StatusNoContent)

	stock := 1
	env.expectStatus(t, "POST", "/api/rewards", map[string]any{
		"name":  "Limited Edition",
		"cost":  10,
		"stock": stock,
	}, adminHeaders(), http.StatusCreated)

	// First redeem works
	env.expectStatus(t, "POST", "/api/rewards/1/redeem", nil, childHeaders(kidID), http.StatusCreated)

	// Second redeem fails — out of stock
	env.expectStatus(t, "POST", "/api/rewards/1/redeem", nil, childHeaders(kidID), http.StatusUnprocessableEntity)
}

func TestUndoRedemption(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{
		"user_id": kidID, "amount": 100, "note": "seed",
	}, adminHeaders(), http.StatusNoContent)

	stock := 5
	env.expectStatus(t, "POST", "/api/rewards", map[string]any{
		"name":  "Sticker",
		"cost":  20,
		"stock": stock,
	}, adminHeaders(), http.StatusCreated)

	// Redeem
	resp := env.expectStatus(t, "POST", "/api/rewards/1/redeem", nil, childHeaders(kidID), http.StatusCreated)
	var redemption map[string]any
	decodeBody(t, resp, &redemption)
	redemptionID := int(redemption["id"].(float64))

	// Balance should be 80
	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, childHeaders(kidID), http.StatusOK)
	var pts map[string]any
	decodeBody(t, resp, &pts)
	if pts["balance"].(float64) != 80 {
		t.Fatalf("expected 80 after redeem, got %v", pts["balance"])
	}

	// Undo the redemption
	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/redemptions/%d", redemptionID), nil, adminHeaders(), http.StatusNoContent)

	// Balance should be back to 100
	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, childHeaders(kidID), http.StatusOK)
	decodeBody(t, resp, &pts)
	if pts["balance"].(float64) != 100 {
		t.Fatalf("expected 100 after undo, got %v", pts["balance"])
	}

	// Redemption history should be empty
	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/redemptions", kidID), nil, childHeaders(kidID), http.StatusOK)
	var history []any
	decodeBody(t, resp, &history)
	if len(history) != 0 {
		t.Fatalf("expected 0 redemptions after undo, got %d", len(history))
	}
}

func TestRedemptionHistory(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{
		"user_id": kidID, "amount": 200, "note": "seed",
	}, adminHeaders(), http.StatusNoContent)

	env.expectStatus(t, "POST", "/api/rewards", map[string]any{
		"name": "Treat", "cost": 10, "icon": "🍬",
	}, adminHeaders(), http.StatusCreated)

	// Redeem twice
	env.expectStatus(t, "POST", "/api/rewards/1/redeem", nil, childHeaders(kidID), http.StatusCreated)
	env.expectStatus(t, "POST", "/api/rewards/1/redeem", nil, childHeaders(kidID), http.StatusCreated)

	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/redemptions", kidID), nil, childHeaders(kidID), http.StatusOK)
	var history []map[string]any
	decodeBody(t, resp, &history)
	if len(history) != 2 {
		t.Fatalf("expected 2 redemptions, got %d", len(history))
	}
	if history[0]["reward_name"] != "Treat" {
		t.Fatalf("unexpected reward name: %v", history[0]["reward_name"])
	}
	if history[0]["points_spent"].(float64) != 10 {
		t.Fatalf("unexpected points spent: %v", history[0]["points_spent"])
	}
}

func TestRewardAssignments(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid1 := env.createChild(t, "Kid1")
	kid2 := env.createChild(t, "Kid2")

	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{
		"user_id": kid1, "amount": 200, "note": "seed",
	}, adminHeaders(), http.StatusNoContent)
	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{
		"user_id": kid2, "amount": 200, "note": "seed",
	}, adminHeaders(), http.StatusNoContent)

	// Create reward assigned only to kid1
	env.expectStatus(t, "POST", "/api/rewards", map[string]any{
		"name": "Special", "cost": 10,
	}, adminHeaders(), http.StatusCreated)
	env.expectStatus(t, "PUT", "/api/rewards/1/assignments", map[string]any{
		"assignments": []map[string]any{
			{"user_id": kid1},
		},
	}, adminHeaders(), http.StatusNoContent)

	// Kid1 can redeem
	env.expectStatus(t, "POST", "/api/rewards/1/redeem", nil, childHeaders(kid1), http.StatusCreated)

	// Kid2 cannot redeem — not assigned
	env.expectStatus(t, "POST", "/api/rewards/1/redeem", nil, childHeaders(kid2), http.StatusUnprocessableEntity)
}

func TestRewardCustomCost(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{
		"user_id": kidID, "amount": 200, "note": "seed",
	}, adminHeaders(), http.StatusNoContent)

	// Create reward with base cost 50, assign to kid with custom cost 25
	env.expectStatus(t, "POST", "/api/rewards", map[string]any{
		"name": "Custom Cost", "cost": 50,
	}, adminHeaders(), http.StatusCreated)
	env.expectStatus(t, "PUT", "/api/rewards/1/assignments", map[string]any{
		"assignments": []map[string]any{
			{"user_id": kidID, "custom_cost": 25},
		},
	}, adminHeaders(), http.StatusNoContent)

	resp := env.expectStatus(t, "POST", "/api/rewards/1/redeem", nil, childHeaders(kidID), http.StatusCreated)
	var redemption map[string]any
	decodeBody(t, resp, &redemption)
	if redemption["points_spent"].(float64) != 25 {
		t.Fatalf("expected custom cost 25, got %v", redemption["points_spent"])
	}
}

// =================== ADMIN PASSCODE TESTS ===================

func TestAdminPasscodeVerify(t *testing.T) {
	env := setupTest(t)

	// Default passcode is "0000"
	resp := env.expectStatus(t, "POST", "/api/admin/verify", map[string]any{
		"passcode": "0000",
	}, nil, http.StatusOK)
	var result map[string]any
	decodeBody(t, resp, &result)
	if result["valid"] != true {
		t.Fatal("expected valid=true for correct passcode")
	}

	// Wrong passcode
	env.expectStatus(t, "POST", "/api/admin/verify", map[string]any{
		"passcode": "9999",
	}, nil, http.StatusUnauthorized)
}

func TestAdminPasscodeUpdate(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Update passcode
	env.expectStatus(t, "PUT", "/api/admin/passcode", map[string]any{
		"old_passcode": "0000",
		"new_passcode": "1234",
	}, adminHeaders(), http.StatusOK)

	// Old passcode should fail
	env.expectStatus(t, "POST", "/api/admin/verify", map[string]any{
		"passcode": "0000",
	}, nil, http.StatusUnauthorized)

	// New passcode should work
	env.expectStatus(t, "POST", "/api/admin/verify", map[string]any{
		"passcode": "1234",
	}, nil, http.StatusOK)
}

func TestAdminPasscodeTooShort(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "PUT", "/api/admin/passcode", map[string]any{
		"old_passcode": "0000",
		"new_passcode": "12",
	}, adminHeaders(), http.StatusBadRequest)
}

func TestAdminPasscodeWrongOld(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "PUT", "/api/admin/passcode", map[string]any{
		"old_passcode": "wrong",
		"new_passcode": "5678",
	}, adminHeaders(), http.StatusUnauthorized)
}

// =================== STREAK TESTS ===================

func TestStreakRewardCRUD(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Create
	resp := env.expectStatus(t, "POST", "/api/admin/streak-rewards", map[string]any{
		"streak_days":  7,
		"bonus_points": 50,
		"label":        "Week Warrior",
	}, adminHeaders(), http.StatusCreated)
	var reward map[string]any
	decodeBody(t, resp, &reward)
	if reward["streak_days"].(float64) != 7 {
		t.Fatalf("expected streak_days 7, got %v", reward["streak_days"])
	}

	// List
	resp = env.expectStatus(t, "GET", "/api/admin/streak-rewards", nil, adminHeaders(), http.StatusOK)
	var rewards []map[string]any
	decodeBody(t, resp, &rewards)
	if len(rewards) != 1 {
		t.Fatalf("expected 1 streak reward, got %d", len(rewards))
	}

	// Delete
	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/admin/streak-rewards/%d", int(reward["id"].(float64))), nil, adminHeaders(), http.StatusNoContent)

	resp = env.expectStatus(t, "GET", "/api/admin/streak-rewards", nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &rewards)
	if len(rewards) != 0 {
		t.Fatalf("expected 0 after delete, got %d", len(rewards))
	}
}

func TestStreakRewardValidation(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Zero streak_days
	env.expectStatus(t, "POST", "/api/admin/streak-rewards", map[string]any{
		"streak_days":  0,
		"bonus_points": 10,
	}, adminHeaders(), http.StatusBadRequest)

	// Negative bonus
	env.expectStatus(t, "POST", "/api/admin/streak-rewards", map[string]any{
		"streak_days":  5,
		"bonus_points": -1,
	}, adminHeaders(), http.StatusBadRequest)
}

func TestGetUserStreak(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/streak", kidID), nil, childHeaders(kidID), http.StatusOK)
	var streak map[string]any
	decodeBody(t, resp, &streak)
	if streak["current_streak"].(float64) != 0 {
		t.Fatalf("expected 0 streak for new user, got %v", streak["current_streak"])
	}
}

// =================== WEBHOOK TESTS ===================

func TestWebhookCRUD(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Create
	resp := env.expectStatus(t, "POST", "/api/admin/webhooks", map[string]any{
		"url":    "https://example.com/hook",
		"secret": "mysecret",
		"events": "chore.completed,reward.redeemed",
	}, adminHeaders(), http.StatusCreated)
	var wh map[string]any
	decodeBody(t, resp, &wh)
	if wh["url"] != "https://example.com/hook" {
		t.Fatalf("unexpected url: %v", wh["url"])
	}
	if wh["events"] != "chore.completed,reward.redeemed" {
		t.Fatalf("unexpected events: %v", wh["events"])
	}
	if wh["active"] != true {
		t.Fatal("expected active=true")
	}

	// List
	resp = env.expectStatus(t, "GET", "/api/admin/webhooks", nil, adminHeaders(), http.StatusOK)
	var webhooks []map[string]any
	decodeBody(t, resp, &webhooks)
	if len(webhooks) != 1 {
		t.Fatalf("expected 1 webhook, got %d", len(webhooks))
	}

	// Update
	active := false
	resp = env.expectStatus(t, "PUT", fmt.Sprintf("/api/admin/webhooks/%d", int(wh["id"].(float64))), map[string]any{
		"active": active,
	}, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &wh)
	if wh["active"] != false {
		t.Fatal("expected active=false after update")
	}

	// Delete
	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/admin/webhooks/%d", int(wh["id"].(float64))), nil, adminHeaders(), http.StatusNoContent)

	resp = env.expectStatus(t, "GET", "/api/admin/webhooks", nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &webhooks)
	if len(webhooks) != 0 {
		t.Fatalf("expected 0 after delete, got %d", len(webhooks))
	}
}

func TestWebhookRequiresURL(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "POST", "/api/admin/webhooks", map[string]any{
		"secret": "test",
	}, adminHeaders(), http.StatusBadRequest)
}

func TestWebhookDefaultEvents(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	resp := env.expectStatus(t, "POST", "/api/admin/webhooks", map[string]any{
		"url": "https://example.com/hook",
	}, adminHeaders(), http.StatusCreated)
	var wh map[string]any
	decodeBody(t, resp, &wh)
	if wh["events"] != "*" {
		t.Fatalf("expected default events '*', got %v", wh["events"])
	}
}

func TestWebhookDeliveries(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Create webhook
	resp := env.expectStatus(t, "POST", "/api/admin/webhooks", map[string]any{
		"url": "https://example.com/hook",
	}, adminHeaders(), http.StatusCreated)
	var wh map[string]any
	decodeBody(t, resp, &wh)
	whID := int(wh["id"].(float64))

	// List deliveries (should be empty)
	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/admin/webhooks/%d/deliveries", whID), nil, adminHeaders(), http.StatusOK)
	var deliveries []any
	decodeBody(t, resp, &deliveries)
	if len(deliveries) != 0 {
		t.Fatalf("expected 0 deliveries, got %d", len(deliveries))
	}
}

func TestWebhookNotFoundOnUpdate(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "PUT", "/api/admin/webhooks/999", map[string]any{
		"url": "https://example.com/new",
	}, adminHeaders(), http.StatusNotFound)
}

// =================== SCHEDULE TESTS ===================

func TestScheduleRequiresAssignedTo(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Test", "category": "core",
	}, adminHeaders())

	env.expectStatus(t, "POST", "/api/chores/1/schedules", map[string]any{
		"day_of_week": 3,
	}, adminHeaders(), http.StatusBadRequest)
}

func TestScheduleRequiresDayOrDate(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Test", "category": "core",
	}, adminHeaders())

	// Neither day_of_week nor specific_date nor recurrence
	env.expectStatus(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID,
	}, adminHeaders(), http.StatusBadRequest)
}

func TestScheduleWithRecurrence(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Test", "category": "core",
	}, adminHeaders())

	// Recurrence without start should fail
	env.expectStatus(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to":         kidID,
		"recurrence_interval": 3,
	}, adminHeaders(), http.StatusBadRequest)

	// Valid recurrence
	env.expectStatus(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to":         kidID,
		"recurrence_interval": 3,
		"recurrence_start":    "2026-03-01",
	}, adminHeaders(), http.StatusCreated)
}

func TestScheduleWithDueBy(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Test", "category": "core",
	}, adminHeaders())

	resp := env.expectStatus(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID,
		"day_of_week": 3,
		"due_by":      "17:00",
	}, adminHeaders(), http.StatusCreated)

	var schedule map[string]any
	decodeBody(t, resp, &schedule)
	if schedule["due_by"] != "17:00" {
		t.Fatalf("expected due_by '17:00', got %v", schedule["due_by"])
	}
}

func TestListSchedulesForChore(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Test", "category": "core",
	}, adminHeaders())

	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID, "day_of_week": 1,
	}, adminHeaders())
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID, "day_of_week": 5,
	}, adminHeaders())

	resp := env.expectStatus(t, "GET", "/api/chores/1/schedules", nil, adminHeaders(), http.StatusOK)
	var schedules []any
	decodeBody(t, resp, &schedules)
	if len(schedules) != 2 {
		t.Fatalf("expected 2 schedules, got %d", len(schedules))
	}
}

func TestDeleteSchedule(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Test", "category": "core",
	}, adminHeaders())
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID, "day_of_week": 1,
	}, adminHeaders())

	env.expectStatus(t, "DELETE", "/api/chores/1/schedules/1", nil, adminHeaders(), http.StatusNoContent)

	resp := env.expectStatus(t, "GET", "/api/chores/1/schedules", nil, adminHeaders(), http.StatusOK)
	var schedules []any
	decodeBody(t, resp, &schedules)
	if len(schedules) != 0 {
		t.Fatalf("expected 0 after delete, got %d", len(schedules))
	}
}

func TestSchedulePointsMultiplier(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Test", "category": "core",
	}, adminHeaders())

	resp := env.expectStatus(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to":      kidID,
		"day_of_week":      3,
		"points_multiplier": 2.0,
	}, adminHeaders(), http.StatusCreated)
	var schedule map[string]any
	decodeBody(t, resp, &schedule)
	if schedule["points_multiplier"].(float64) != 2.0 {
		t.Fatalf("expected multiplier 2.0, got %v", schedule["points_multiplier"])
	}
}

// =================== USER TESTS ===================

func TestUpdateUser(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	resp := env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d", kidID), map[string]any{
		"name": "Updated Kid",
	}, adminHeaders(), http.StatusOK)
	var user map[string]any
	decodeBody(t, resp, &user)
	if user["name"] != "Updated Kid" {
		t.Fatalf("expected updated name, got %v", user["name"])
	}
}

func TestDeleteUser(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/users/%d", kidID), nil, adminHeaders(), http.StatusNoContent)

	env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d", kidID), nil, nil, http.StatusNotFound)
}

func TestDeleteLastAdminBlocked(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t) // admin ID = 1

	// Attempt to delete the only admin — should be blocked
	resp := env.expectStatus(t, "DELETE", "/api/users/1", nil, adminHeaders(), http.StatusConflict)
	var body map[string]string
	decodeBody(t, resp, &body)
	if body["error"] != "cannot delete the last admin user" {
		t.Errorf("expected last-admin error, got %q", body["error"])
	}

	// Admin should still exist
	env.expectStatus(t, "GET", "/api/users/1", nil, nil, http.StatusOK)
}

func TestUserThemeUpdate(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Kid updates own theme
	resp := env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/theme", kidID), map[string]any{
		"theme": "galaxy",
	}, childHeaders(kidID), http.StatusOK)
	var user map[string]any
	decodeBody(t, resp, &user)
	if user["theme"] != "galaxy" {
		t.Fatalf("expected galaxy theme, got %v", user["theme"])
	}
}

func TestUserThemeUpdateForbiddenForOthers(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid1 := env.createChild(t, "Kid1")
	kid2 := env.createChild(t, "Kid2")
	_ = kid1

	// Kid2 tries to update Kid1's theme
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/theme", kid1), map[string]any{
		"theme": "quest",
	}, childHeaders(kid2), http.StatusForbidden)
}

func TestUserInvalidTheme(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/theme", kidID), map[string]any{
		"theme": "nonexistent",
	}, childHeaders(kidID), http.StatusBadRequest)
}

func TestUserAvatarUpdate(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	resp := env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/avatar", kidID), map[string]any{
		"avatar_url": "https://api.dicebear.com/9.x/glass/svg?seed=test",
	}, childHeaders(kidID), http.StatusOK)
	var user map[string]any
	decodeBody(t, resp, &user)
	if user["avatar_url"] != "https://api.dicebear.com/9.x/glass/svg?seed=test" {
		t.Fatalf("avatar not updated: %v", user["avatar_url"])
	}
}

func TestUserAvatarUpdateForbiddenForOthers(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid1 := env.createChild(t, "Kid1")
	kid2 := env.createChild(t, "Kid2")
	_ = kid1

	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/avatar", kid1), map[string]any{
		"avatar_url": "https://example.com/avatar.png",
	}, childHeaders(kid2), http.StatusForbidden)
}

func TestUserCreateValidation(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Missing name
	env.expectStatus(t, "POST", "/api/users", map[string]any{
		"role": "child",
	}, adminHeaders(), http.StatusBadRequest)

	// Invalid role
	env.expectStatus(t, "POST", "/api/users", map[string]any{
		"name": "Test",
		"role": "superuser",
	}, adminHeaders(), http.StatusBadRequest)
}

func TestUserDefaultRole(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	resp := env.expectStatus(t, "POST", "/api/users", map[string]any{
		"name": "Default Role",
	}, adminHeaders(), http.StatusCreated)
	var user map[string]any
	decodeBody(t, resp, &user)
	if user["role"] != "child" {
		t.Fatalf("expected default role 'child', got %v", user["role"])
	}
}

// =================== CHORE VALIDATION TESTS ===================

func TestChoreRequiresTitle(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "POST", "/api/chores", map[string]any{
		"category": "core",
	}, adminHeaders(), http.StatusBadRequest)
}

func TestChoreDefaultCategory(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	resp := env.expectStatus(t, "POST", "/api/chores", map[string]any{
		"title": "No category",
	}, adminHeaders(), http.StatusCreated)
	var chore map[string]any
	decodeBody(t, resp, &chore)
	if chore["category"] != "core" {
		t.Fatalf("expected default category 'core', got %v", chore["category"])
	}
}

func TestChoreGetNotFound(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "GET", "/api/chores/999", nil, adminHeaders(), http.StatusNotFound)
}

func TestChoreUpdateNotFound(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "PUT", "/api/chores/999", map[string]any{
		"title": "Nope",
	}, adminHeaders(), http.StatusNotFound)
}

func TestChoreUpdateInvalidCategory(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Test", "category": "core",
	}, adminHeaders())

	env.expectStatus(t, "PUT", "/api/chores/1", map[string]any{
		"category": "invalid",
	}, adminHeaders(), http.StatusBadRequest)
}

// =================== ADMIN AUTH EDGE CASES ===================

func TestWebhooksRequireAdmin(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.expectStatus(t, "GET", "/api/admin/webhooks", nil, childHeaders(kidID), http.StatusForbidden)
	env.expectStatus(t, "POST", "/api/admin/webhooks", map[string]any{
		"url": "https://example.com",
	}, childHeaders(kidID), http.StatusForbidden)
}

func TestRewardsAdminEndpointsRequireAdmin(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.expectStatus(t, "POST", "/api/rewards", map[string]any{
		"name": "Test", "cost": 10,
	}, childHeaders(kidID), http.StatusForbidden)

	env.expectStatus(t, "GET", "/api/rewards/all", nil, childHeaders(kidID), http.StatusForbidden)
}

func TestPointsAdjustRequiresAdmin(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{
		"user_id": kidID, "amount": 100, "note": "hack",
	}, childHeaders(kidID), http.StatusForbidden)
}

func TestRedemptionUndoRequiresAdmin(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.expectStatus(t, "DELETE", "/api/redemptions/1", nil, childHeaders(kidID), http.StatusForbidden)
}

// =================== EXPIRY PENALTY TESTS ===================

func TestExpiryPenaltyBlock(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Test", "category": "core", "points_value": 10,
	}, adminHeaders())

	// Schedule with due_by in the past (00:01) and block penalty
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to":    kidID,
		"day_of_week":    3,
		"due_by":         "00:01",
		"expiry_penalty": "block",
	}, adminHeaders())

	// Try to complete — should be blocked because it's past 00:01
	env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    kidID,
		"completion_date": time.Now().Format(model.DateFormat),
	}, adminHeaders(), http.StatusUnprocessableEntity)
}

func TestExpiryPenaltyNoPoints(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Test", "category": "bonus", "points_value": 10,
	}, adminHeaders())

	// Schedule with due_by in the past and no_points penalty
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to":    kidID,
		"day_of_week":    int(time.Now().Weekday()),
		"due_by":         "00:01",
		"expiry_penalty": "no_points",
	}, adminHeaders())

	// Should allow completion
	env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    kidID,
		"completion_date": time.Now().Format(model.DateFormat),
	}, adminHeaders(), http.StatusCreated)

	// But should earn 0 points
	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, adminHeaders(), http.StatusOK)
	var pts map[string]any
	decodeBody(t, resp, &pts)
	if pts["balance"].(float64) != 0 {
		t.Fatalf("expected 0 points for no_points penalty, got %v", pts["balance"])
	}
}

func TestExpiryPenaltyDeduction(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Give kid some points first
	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{
		"user_id": kidID, "amount": 50, "note": "seed",
	}, adminHeaders(), http.StatusNoContent)

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Test", "category": "bonus", "points_value": 10,
	}, adminHeaders())

	// Schedule with penalty of 5 points
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to":          kidID,
		"day_of_week":          int(time.Now().Weekday()),
		"due_by":               "00:01",
		"expiry_penalty":       "penalty",
		"expiry_penalty_value": 5,
	}, adminHeaders())

	// Complete late — should deduct 5 points
	env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    kidID,
		"completion_date": time.Now().Format(model.DateFormat),
	}, adminHeaders(), http.StatusCreated)

	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, adminHeaders(), http.StatusOK)
	var pts map[string]any
	decodeBody(t, resp, &pts)
	// Started with 50, penalty of -5 = 45
	if pts["balance"].(float64) != 45 {
		t.Fatalf("expected 45 after penalty, got %v", pts["balance"])
	}
}

func TestExpiryPenaltyNotExpired(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Test", "category": "bonus", "points_value": 10,
	}, adminHeaders())

	// Schedule with due_by far in the future — penalty configured but shouldn't apply
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to":          kidID,
		"day_of_week":          int(time.Now().Weekday()),
		"due_by":               "23:59",
		"expiry_penalty":       "penalty",
		"expiry_penalty_value": 100,
	}, adminHeaders())

	env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    kidID,
		"completion_date": time.Now().Format(model.DateFormat),
	}, adminHeaders(), http.StatusCreated)

	// Should get full 10 points, no penalty applied
	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, adminHeaders(), http.StatusOK)
	var pts map[string]any
	decodeBody(t, resp, &pts)
	if pts["balance"].(float64) != 10 {
		t.Fatalf("expected 10 points (not expired), got %v", pts["balance"])
	}
}

func TestExpiryPenaltyValidation(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Test", "category": "core",
	}, adminHeaders())

	// Invalid penalty type
	env.expectStatus(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to":    kidID,
		"day_of_week":    3,
		"expiry_penalty": "invalid",
	}, adminHeaders(), http.StatusBadRequest)

	// Penalty mode without value
	env.expectStatus(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to":    kidID,
		"day_of_week":    3,
		"expiry_penalty": "penalty",
	}, adminHeaders(), http.StatusBadRequest)
}

func TestScheduleExpiryPenaltyStored(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Test", "category": "core",
	}, adminHeaders())

	resp := env.expectStatus(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to":          kidID,
		"day_of_week":          3,
		"due_by":               "17:00",
		"expiry_penalty":       "penalty",
		"expiry_penalty_value": 15,
	}, adminHeaders(), http.StatusCreated)

	var schedule map[string]any
	decodeBody(t, resp, &schedule)
	if schedule["expiry_penalty"] != "penalty" {
		t.Fatalf("expected expiry_penalty 'penalty', got %v", schedule["expiry_penalty"])
	}
	if schedule["expiry_penalty_value"].(float64) != 15 {
		t.Fatalf("expected expiry_penalty_value 15, got %v", schedule["expiry_penalty_value"])
	}
}

// =================== DECAY CONFIG TESTS ===================

func TestDecayConfigCRUD(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Get default config
	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/admin/users/%d/decay", kidID), nil, adminHeaders(), http.StatusOK)
	var cfg map[string]any
	decodeBody(t, resp, &cfg)
	if cfg["enabled"] != false {
		t.Fatal("expected decay disabled by default")
	}
	if cfg["decay_rate"].(float64) != 5 {
		t.Fatalf("expected default decay_rate 5, got %v", cfg["decay_rate"])
	}

	// Enable decay
	resp = env.expectStatus(t, "PUT", fmt.Sprintf("/api/admin/users/%d/decay", kidID), map[string]any{
		"enabled":              true,
		"decay_rate":           10,
		"decay_interval_hours": 12,
	}, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &cfg)
	if cfg["enabled"] != true {
		t.Fatal("expected decay enabled")
	}
	if cfg["decay_rate"].(float64) != 10 {
		t.Fatalf("expected decay_rate 10, got %v", cfg["decay_rate"])
	}
	if cfg["decay_interval_hours"].(float64) != 12 {
		t.Fatalf("expected interval 12, got %v", cfg["decay_interval_hours"])
	}

	// Read back
	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/admin/users/%d/decay", kidID), nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &cfg)
	if cfg["enabled"] != true {
		t.Fatal("expected decay still enabled on re-read")
	}
}

func TestDecayConfigRequiresAdmin(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.expectStatus(t, "GET", fmt.Sprintf("/api/admin/users/%d/decay", kidID), nil, childHeaders(kidID), http.StatusForbidden)
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/admin/users/%d/decay", kidID), map[string]any{
		"enabled": true, "decay_rate": 5,
	}, childHeaders(kidID), http.StatusForbidden)
}

func TestDecayConfigValidation(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Negative decay rate
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/admin/users/%d/decay", kidID), map[string]any{
		"enabled":    true,
		"decay_rate": -5,
	}, adminHeaders(), http.StatusBadRequest)
}

func TestUncompleteExpiryPenaltyRefund(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Give kid 50 points
	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{
		"user_id": kidID, "amount": 50, "note": "seed",
	}, adminHeaders(), http.StatusNoContent)

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Test", "category": "bonus", "points_value": 10,
	}, adminHeaders())

	// Schedule with penalty of 5 points
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to":          kidID,
		"day_of_week":          int(time.Now().Weekday()),
		"due_by":               "00:01",
		"expiry_penalty":       "penalty",
		"expiry_penalty_value": 5,
	}, adminHeaders())

	today := time.Now().Format(model.DateFormat)

	// Complete late — penalty of -5 applied, balance should be 45
	env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    kidID,
		"completion_date": today,
	}, adminHeaders(), http.StatusCreated)

	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, adminHeaders(), http.StatusOK)
	var pts map[string]any
	decodeBody(t, resp, &pts)
	if pts["balance"].(float64) != 45 {
		t.Fatalf("expected 45 after penalty, got %v", pts["balance"])
	}

	// Uncomplete — penalty should be refunded, balance back to 50
	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/schedules/1/complete?date=%s", today), nil, adminHeaders(), http.StatusNoContent)

	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &pts)
	if pts["balance"].(float64) != 50 {
		t.Fatalf("expected 50 after uncomplete refund, got %v", pts["balance"])
	}
}

func TestUncompleteNormalPointsReversed(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Test", "category": "bonus", "points_value": 10,
	}, adminHeaders())

	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to":          kidID,
		"day_of_week":          int(time.Now().Weekday()),
		"due_by":               "23:59",
		"expiry_penalty":       "penalty",
		"expiry_penalty_value": 5,
	}, adminHeaders())

	today := time.Now().Format(model.DateFormat)

	// Complete on time — earns 10 points
	env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    kidID,
		"completion_date": today,
	}, adminHeaders(), http.StatusCreated)

	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, adminHeaders(), http.StatusOK)
	var pts map[string]any
	decodeBody(t, resp, &pts)
	if pts["balance"].(float64) != 10 {
		t.Fatalf("expected 10, got %v", pts["balance"])
	}

	// Uncomplete — should reverse the 10 point credit
	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/schedules/1/complete?date=%s", today), nil, adminHeaders(), http.StatusNoContent)

	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &pts)
	if pts["balance"].(float64) != 0 {
		t.Fatalf("expected 0 after uncomplete, got %v", pts["balance"])
	}
}

// =================== BCRYPT PASSCODE TESTS ===================

func TestBcryptPasscodeRoundTrip(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Default passcode "0000" should work (stored as bcrypt hash in migration)
	resp := env.expectStatus(t, "POST", "/api/admin/verify", map[string]any{
		"passcode": "0000",
	}, nil, http.StatusOK)
	var result map[string]any
	decodeBody(t, resp, &result)
	if result["valid"] != true {
		t.Fatal("expected valid=true for correct default passcode")
	}

	// Update passcode to "abcd1234"
	env.expectStatus(t, "PUT", "/api/admin/passcode", map[string]any{
		"old_passcode": "0000",
		"new_passcode": "abcd1234",
	}, adminHeaders(), http.StatusOK)

	// Old passcode should fail
	env.expectStatus(t, "POST", "/api/admin/verify", map[string]any{
		"passcode": "0000",
	}, nil, http.StatusUnauthorized)

	// New passcode should work
	resp = env.expectStatus(t, "POST", "/api/admin/verify", map[string]any{
		"passcode": "abcd1234",
	}, nil, http.StatusOK)
	decodeBody(t, resp, &result)
	if result["valid"] != true {
		t.Fatal("expected valid=true for new passcode")
	}

	// Verify the stored value is a bcrypt hash (starts with $2a$)
	var stored string
	err := env.db.QueryRow(`SELECT value FROM app_settings WHERE key = 'admin_passcode'`).Scan(&stored)
	if err != nil {
		t.Fatalf("failed to read stored passcode: %v", err)
	}
	if len(stored) < 4 || stored[:4] != "$2a$" {
		t.Fatalf("expected bcrypt hash starting with $2a$, got %q", stored)
	}
}

// =================== UPLOAD MIME VALIDATION TESTS ===================

func TestUploadRejectsNonImage(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Create a multipart form with a text file
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("photo", "malicious.txt")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("This is not an image file, just plain text content."))
	writer.Close()

	req, err := http.NewRequest("POST", env.server.URL+"/api/upload", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-ID", "1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-image upload, got %d", resp.StatusCode)
	}
}

func TestUploadAcceptsImage(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Create a minimal valid PNG (1x1 pixel)
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("photo", "test.png")
	if err != nil {
		t.Fatal(err)
	}
	// Minimal valid PNG file bytes
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, // bit depth, color type, etc.
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, // IDAT chunk
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x01, 0xE2, 0x21, 0xBC,
		0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
		0x44, 0xAE, 0x42, 0x60, 0x82, // IEND chunk
	}
	part.Write(pngData)
	writer.Close()

	req, err := http.NewRequest("POST", env.server.URL+"/api/upload", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-ID", "1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 for valid PNG upload, got %d: %s", resp.StatusCode, body)
	}
}

// =================== SETUP ENDPOINT TESTS ===================

func TestSetupCreatesAdminAndChildren(t *testing.T) {
	env := setupTest(t)

	resp := env.expectStatus(t, "POST", "/api/setup", map[string]any{
		"children": []map[string]any{
			{"name": "Alice", "theme": "galaxy"},
			{"name": "Bob", "theme": "forest"},
		},
		"chores": []map[string]any{
			{"title": "Feed cats", "icon": "cat", "category": "required", "points_value": 5},
		},
	}, nil, http.StatusCreated)
	var result map[string]any
	decodeBody(t, resp, &result)

	admin := result["admin"].(map[string]any)
	if admin["name"] != "Parent" {
		t.Fatalf("expected admin name 'Parent', got %v", admin["name"])
	}
	if admin["role"] != "admin" {
		t.Fatalf("expected admin role, got %v", admin["role"])
	}

	children := result["children"].([]any)
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
}

func TestSetupFailsWhenUsersExist(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "POST", "/api/setup", map[string]any{
		"children": []map[string]any{
			{"name": "Alice"},
		},
	}, nil, http.StatusConflict)
}

func TestSetupRequiresChildren(t *testing.T) {
	env := setupTest(t)

	env.expectStatus(t, "POST", "/api/setup", map[string]any{
		"children": []map[string]any{},
	}, nil, http.StatusBadRequest)
}

func TestSetupInvalidBody(t *testing.T) {
	env := setupTest(t)

	// Send invalid JSON
	req, _ := http.NewRequest("POST", env.server.URL+"/api/setup", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid body, got %d", resp.StatusCode)
	}
}

// =================== ADMIN SETTINGS TESTS ===================

func TestAdminGetAndSetSetting(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Set a valid setting from the allowlist
	resp := env.expectStatus(t, "PUT", "/api/admin/settings/base_url", map[string]any{
		"value": "https://example.com",
	}, adminHeaders(), http.StatusOK)
	var setting map[string]any
	decodeBody(t, resp, &setting)
	if setting["key"] != "base_url" {
		t.Fatalf("expected key 'base_url', got %v", setting["key"])
	}
	if setting["value"] != "https://example.com" {
		t.Fatalf("expected value 'https://example.com', got %v", setting["value"])
	}

	// Get it back
	resp = env.expectStatus(t, "GET", "/api/admin/settings/base_url", nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &setting)
	if setting["value"] != "https://example.com" {
		t.Fatalf("expected value 'https://example.com', got %v", setting["value"])
	}
}

func TestAdminSetSettingRejectsUnknownKey(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "PUT", "/api/admin/settings/not_a_real_key", map[string]any{
		"value": "foo",
	}, adminHeaders(), http.StatusBadRequest)
}

func TestAdminSettingsRequireAdmin(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.expectStatus(t, "GET", "/api/admin/settings/base_url", nil, childHeaders(kidID), http.StatusForbidden)
	env.expectStatus(t, "PUT", "/api/admin/settings/base_url", map[string]any{
		"value": "test",
	}, childHeaders(kidID), http.StatusForbidden)
}

// =================== APPROVAL WORKFLOW TESTS ===================

func TestListPendingCompletions(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Create a chore that requires approval
	env.request(t, "POST", "/api/chores", map[string]any{
		"title":             "Clean room",
		"category":          "core",
		"points_value":      10,
		"requires_approval": true,
	}, adminHeaders())

	// Schedule for today's weekday
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID,
		"day_of_week": int(time.Now().Weekday()),
	}, adminHeaders())

	// Complete it (should become pending)
	env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    kidID,
		"completion_date": time.Now().Format(model.DateFormat),
	}, childHeaders(kidID), http.StatusCreated)

	// List pending
	resp := env.expectStatus(t, "GET", "/api/completions/pending", nil, adminHeaders(), http.StatusOK)
	var pending []map[string]any
	decodeBody(t, resp, &pending)
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending completion, got %d", len(pending))
	}
}

func TestApproveCompletion(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title":             "Clean room",
		"category":          "core",
		"points_value":      10,
		"requires_approval": true,
	}, adminHeaders())

	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID,
		"day_of_week": int(time.Now().Weekday()),
	}, adminHeaders())

	resp := env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    kidID,
		"completion_date": time.Now().Format(model.DateFormat),
	}, childHeaders(kidID), http.StatusCreated)
	var completion map[string]any
	decodeBody(t, resp, &completion)
	completionID := int(completion["id"].(float64))

	// Points should be 0 before approval
	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, childHeaders(kidID), http.StatusOK)
	var pts map[string]any
	decodeBody(t, resp, &pts)
	if pts["balance"].(float64) != 0 {
		t.Fatalf("expected 0 points before approval, got %v", pts["balance"])
	}

	// Approve
	env.expectStatus(t, "POST", fmt.Sprintf("/api/completions/%d/approve", completionID), nil, adminHeaders(), http.StatusNoContent)

	// Points should be credited after approval
	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, childHeaders(kidID), http.StatusOK)
	decodeBody(t, resp, &pts)
	if pts["balance"].(float64) != 10 {
		t.Fatalf("expected 10 points after approval, got %v", pts["balance"])
	}
}

func TestRejectCompletion(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title":             "Clean room",
		"category":          "core",
		"points_value":      10,
		"requires_approval": true,
	}, adminHeaders())

	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID,
		"day_of_week": int(time.Now().Weekday()),
	}, adminHeaders())

	resp := env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    kidID,
		"completion_date": time.Now().Format(model.DateFormat),
	}, childHeaders(kidID), http.StatusCreated)
	var completion map[string]any
	decodeBody(t, resp, &completion)
	completionID := int(completion["id"].(float64))

	// Reject
	env.expectStatus(t, "POST", fmt.Sprintf("/api/completions/%d/reject", completionID), nil, adminHeaders(), http.StatusNoContent)

	// Points should remain 0
	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, childHeaders(kidID), http.StatusOK)
	var pts map[string]any
	decodeBody(t, resp, &pts)
	if pts["balance"].(float64) != 0 {
		t.Fatalf("expected 0 points after rejection, got %v", pts["balance"])
	}
}

func TestApproveNotFoundCompletion(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "POST", "/api/completions/999/approve", nil, adminHeaders(), http.StatusNotFound)
}

func TestRejectNotFoundCompletion(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "POST", "/api/completions/999/reject", nil, adminHeaders(), http.StatusNotFound)
}

func TestApproveAlreadyApproved(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title":             "Clean room",
		"category":          "core",
		"requires_approval": true,
	}, adminHeaders())

	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID,
		"day_of_week": int(time.Now().Weekday()),
	}, adminHeaders())

	resp := env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    kidID,
		"completion_date": time.Now().Format(model.DateFormat),
	}, childHeaders(kidID), http.StatusCreated)
	var completion map[string]any
	decodeBody(t, resp, &completion)
	completionID := int(completion["id"].(float64))

	// Approve once
	env.expectStatus(t, "POST", fmt.Sprintf("/api/completions/%d/approve", completionID), nil, adminHeaders(), http.StatusNoContent)

	// Approve again should fail
	env.expectStatus(t, "POST", fmt.Sprintf("/api/completions/%d/approve", completionID), nil, adminHeaders(), http.StatusBadRequest)
}

// =================== REWARDS USER LIST TESTS ===================

func TestRewardsListForUser(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Create reward
	env.expectStatus(t, "POST", "/api/rewards", map[string]any{
		"name": "Prize", "cost": 10, "icon": "star",
	}, adminHeaders(), http.StatusCreated)

	// User-facing list endpoint
	resp := env.expectStatus(t, "GET", "/api/rewards", nil, childHeaders(kidID), http.StatusOK)
	var rewards []map[string]any
	decodeBody(t, resp, &rewards)
	// Without assignments, reward should be visible to all
	if len(rewards) < 1 {
		t.Fatalf("expected at least 1 reward for user, got %d", len(rewards))
	}
}

func TestRewardUpdateNotFound(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "PUT", "/api/rewards/999", map[string]any{
		"name": "nope",
	}, adminHeaders(), http.StatusNotFound)
}

func TestRewardDeleteInvalidID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "DELETE", "/api/rewards/abc", nil, adminHeaders(), http.StatusBadRequest)
}

func TestRewardSetAssignmentsInvalidID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "PUT", "/api/rewards/abc/assignments", map[string]any{
		"assignments": []map[string]any{},
	}, adminHeaders(), http.StatusBadRequest)
}

func TestRewardRedeemInvalidID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "POST", "/api/rewards/abc/redeem", nil, adminHeaders(), http.StatusBadRequest)
}

func TestUndoRedemptionInvalidID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "DELETE", "/api/redemptions/abc", nil, adminHeaders(), http.StatusBadRequest)
}

func TestListRedemptionsInvalidUserID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "GET", "/api/users/abc/redemptions", nil, adminHeaders(), http.StatusBadRequest)
}

// =================== MIDDLEWARE EDGE CASE TESTS ===================

func TestInvalidUserIDHeader(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Non-numeric X-User-ID
	env.expectStatus(t, "GET", "/api/users/1/chores", nil, map[string]string{
		"X-User-ID": "not-a-number",
	}, http.StatusBadRequest)
}

func TestNonExistentUserIDHeader(t *testing.T) {
	env := setupTest(t)

	// User ID that doesn't exist in DB
	env.expectStatus(t, "GET", "/api/users/1/chores", nil, map[string]string{
		"X-User-ID": "9999",
	}, http.StatusUnauthorized)
}

// =================== USER EDGE CASE TESTS ===================

func TestGetUserNotFound(t *testing.T) {
	env := setupTest(t)

	env.expectStatus(t, "GET", "/api/users/999", nil, nil, http.StatusNotFound)
}

func TestGetUserInvalidID(t *testing.T) {
	env := setupTest(t)

	env.expectStatus(t, "GET", "/api/users/abc", nil, nil, http.StatusBadRequest)
}

func TestUpdateUserNotFound(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "PUT", "/api/users/999", map[string]any{
		"name": "Nope",
	}, adminHeaders(), http.StatusNotFound)
}

func TestUpdateUserInvalidRole(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d", kidID), map[string]any{
		"role": "superuser",
	}, adminHeaders(), http.StatusBadRequest)
}

func TestAvatarUpdateMissingURL(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/avatar", kidID), map[string]any{
		"avatar_url": "",
	}, childHeaders(kidID), http.StatusBadRequest)
}

func TestDeleteUserInvalidID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "DELETE", "/api/users/abc", nil, adminHeaders(), http.StatusBadRequest)
}

// =================== POINTS EDGE CASE TESTS ===================

func TestGetUserPointsInvalidID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "GET", "/api/users/abc/points", nil, adminHeaders(), http.StatusBadRequest)
}

func TestDecayConfigDefaultInterval(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Setting decay_interval_hours to 0 should default to 24
	resp := env.expectStatus(t, "PUT", fmt.Sprintf("/api/admin/users/%d/decay", kidID), map[string]any{
		"enabled":              true,
		"decay_rate":           5,
		"decay_interval_hours": 0,
	}, adminHeaders(), http.StatusOK)
	var cfg map[string]any
	decodeBody(t, resp, &cfg)
	if cfg["decay_interval_hours"].(float64) != 24 {
		t.Fatalf("expected default interval 24, got %v", cfg["decay_interval_hours"])
	}
}

func TestDecayConfigInvalidUserID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "GET", "/api/admin/users/abc/decay", nil, adminHeaders(), http.StatusBadRequest)
	env.expectStatus(t, "PUT", "/api/admin/users/abc/decay", map[string]any{
		"enabled": true, "decay_rate": 5,
	}, adminHeaders(), http.StatusBadRequest)
}

// =================== WEBHOOK EDGE CASE TESTS ===================

func TestWebhookDeleteInvalidID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "DELETE", "/api/admin/webhooks/abc", nil, adminHeaders(), http.StatusBadRequest)
}

func TestWebhookUpdateInvalidID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "PUT", "/api/admin/webhooks/abc", map[string]any{
		"url": "https://example.com",
	}, adminHeaders(), http.StatusBadRequest)
}

func TestWebhookDeliveriesInvalidID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "GET", "/api/admin/webhooks/abc/deliveries", nil, adminHeaders(), http.StatusBadRequest)
}

// =================== STREAK EDGE CASE TESTS ===================

func TestGetUserStreakInvalidID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "GET", "/api/users/abc/streak", nil, adminHeaders(), http.StatusBadRequest)
}

func TestStreakRewardDeleteInvalidID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "DELETE", "/api/admin/streak-rewards/abc", nil, adminHeaders(), http.StatusBadRequest)
}

func TestGetUserStreakWithRewards(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Create streak rewards
	env.expectStatus(t, "POST", "/api/admin/streak-rewards", map[string]any{
		"streak_days":  3,
		"bonus_points": 10,
		"label":        "3 Day Streak",
	}, adminHeaders(), http.StatusCreated)
	env.expectStatus(t, "POST", "/api/admin/streak-rewards", map[string]any{
		"streak_days":  7,
		"bonus_points": 50,
		"label":        "Week Warrior",
	}, adminHeaders(), http.StatusCreated)

	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/streak", kidID), nil, childHeaders(kidID), http.StatusOK)
	var streak map[string]any
	decodeBody(t, resp, &streak)
	// New user has 0 streak, so next_reward should be the first reward
	if streak["next_reward"] == nil {
		t.Fatal("expected next_reward to be set for new user with streak rewards configured")
	}
	nextReward := streak["next_reward"].(map[string]any)
	if nextReward["streak_days"].(float64) != 3 {
		t.Fatalf("expected next reward at 3 days, got %v", nextReward["streak_days"])
	}
}

// =================== CHORE EDGE CASE TESTS ===================

func TestChoreGetInvalidID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "GET", "/api/chores/abc", nil, adminHeaders(), http.StatusBadRequest)
}

func TestChoreUpdateInvalidID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "PUT", "/api/chores/abc", map[string]any{
		"title": "test",
	}, adminHeaders(), http.StatusBadRequest)
}

func TestChoreDeleteInvalidID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "DELETE", "/api/chores/abc", nil, adminHeaders(), http.StatusBadRequest)
}

func TestCompleteScheduleNotFound(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "POST", "/api/schedules/999/complete", map[string]any{
		"completed_by":    1,
		"completion_date": "2026-03-11",
	}, adminHeaders(), http.StatusNotFound)
}

func TestCompleteInvalidScheduleID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "POST", "/api/schedules/abc/complete", map[string]any{
		"completed_by":    1,
		"completion_date": "2026-03-11",
	}, adminHeaders(), http.StatusBadRequest)
}

func TestUncompleteInvalidScheduleID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "DELETE", "/api/schedules/abc/complete?date=2026-03-11", nil, adminHeaders(), http.StatusBadRequest)
}

func TestGetUserChoresInvalidID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "GET", "/api/users/abc/chores", nil, adminHeaders(), http.StatusBadRequest)
}

func TestGetUserChoresInvalidDate(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/chores?date=not-a-date", kidID), nil, childHeaders(kidID), http.StatusBadRequest)
}

func TestUploadNoPhotoField(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Send multipart with wrong field name
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("wrong_field", "test.png")
	part.Write([]byte("some data"))
	writer.Close()

	req, _ := http.NewRequest("POST", env.server.URL+"/api/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-ID", "1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing photo field, got %d", resp.StatusCode)
	}
}

func TestRequiresPhotoChoreCompletion(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Create a chore that requires photo
	env.request(t, "POST", "/api/chores", map[string]any{
		"title":          "Photo chore",
		"category":       "core",
		"requires_photo": true,
	}, adminHeaders())

	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID,
		"day_of_week": int(time.Now().Weekday()),
	}, adminHeaders())

	// Try to complete without photo
	env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    kidID,
		"completion_date": time.Now().Format(model.DateFormat),
	}, childHeaders(kidID), http.StatusBadRequest)

	// Complete with photo should work
	env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    kidID,
		"completion_date": time.Now().Format(model.DateFormat),
		"photo_url":       "/uploads/test.png",
	}, childHeaders(kidID), http.StatusCreated)
}

func TestListSchedulesInvalidChoreID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "GET", "/api/chores/abc/schedules", nil, adminHeaders(), http.StatusBadRequest)
}

func TestDeleteScheduleInvalidScheduleID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Only scheduleID is validated in the handler
	env.expectStatus(t, "DELETE", "/api/chores/1/schedules/abc", nil, adminHeaders(), http.StatusBadRequest)
}

func TestCreateScheduleInvalidChoreID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.expectStatus(t, "POST", "/api/chores/abc/schedules", map[string]any{
		"assigned_to": kidID,
		"day_of_week": 3,
	}, adminHeaders(), http.StatusBadRequest)
}

// =================== LINE COLOR TESTS ===================

func TestUserLineColorUpdate(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Kid updates own line color
	resp := env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/line-color", kidID), map[string]any{
		"line_color": "#ff5733",
	}, childHeaders(kidID), http.StatusOK)
	var user map[string]any
	decodeBody(t, resp, &user)
	if user["line_color"] != "#ff5733" {
		t.Fatalf("expected line_color '#ff5733', got %v", user["line_color"])
	}

	// Verify it persists via GET
	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d", kidID), nil, nil, http.StatusOK)
	decodeBody(t, resp, &user)
	if user["line_color"] != "#ff5733" {
		t.Fatalf("expected line_color to persist, got %v", user["line_color"])
	}
}

func TestUserLineColorForbiddenForOthers(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid1 := env.createChild(t, "Kid1")
	kid2 := env.createChild(t, "Kid2")
	_ = kid1

	// Kid2 tries to update Kid1's line color
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/line-color", kid1), map[string]any{
		"line_color": "#aabbcc",
	}, childHeaders(kid2), http.StatusForbidden)
}

func TestUserLineColorRequiresValue(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Empty line_color should fail
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/line-color", kidID), map[string]any{
		"line_color": "",
	}, childHeaders(kidID), http.StatusBadRequest)
}

func TestUserLineColorNotClobberedByAdminUpdate(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Kid sets line color
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/line-color", kidID), map[string]any{
		"line_color": "#00ff00",
	}, childHeaders(kidID), http.StatusOK)

	// Admin updates the user's name (should not clobber line_color)
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d", kidID), map[string]any{
		"name": "Renamed Kid",
	}, adminHeaders(), http.StatusOK)

	// Verify line_color is preserved
	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d", kidID), nil, nil, http.StatusOK)
	var user map[string]any
	decodeBody(t, resp, &user)
	if user["name"] != "Renamed Kid" {
		t.Fatalf("expected name 'Renamed Kid', got %v", user["name"])
	}
	if user["line_color"] != "#00ff00" {
		t.Fatalf("expected line_color preserved as '#00ff00', got %v", user["line_color"])
	}
}

func TestUserLineColorInvalidUserID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	env.expectStatus(t, "PUT", "/api/users/abc/line-color", map[string]any{
		"line_color": "#ff0000",
	}, adminHeaders(), http.StatusBadRequest)
}

func TestUserLineColorVisibleInListUsers(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Set line color
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/line-color", kidID), map[string]any{
		"line_color": "#123456",
	}, childHeaders(kidID), http.StatusOK)

	// List users should include line_color
	resp := env.expectStatus(t, "GET", "/api/users", nil, nil, http.StatusOK)
	var users []map[string]any
	decodeBody(t, resp, &users)

	found := false
	for _, u := range users {
		if u["name"] == "Kid" {
			if u["line_color"] != "#123456" {
				t.Fatalf("expected line_color '#123456' in list, got %v", u["line_color"])
			}
			found = true
		}
	}
	if !found {
		t.Fatal("Kid not found in user list")
	}
}

// =================== ONE-OFF SCHEDULE (QUICK ASSIGN) TESTS ===================

func TestOneOffScheduleCreation(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Create chore
	env.request(t, "POST", "/api/chores", map[string]any{
		"title":    "One-off task",
		"category": "core",
	}, adminHeaders())

	// Schedule with specific_date only (no day_of_week)
	resp := env.expectStatus(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to":   kidID,
		"specific_date": "2026-04-10",
	}, adminHeaders(), http.StatusCreated)

	var schedule map[string]any
	decodeBody(t, resp, &schedule)
	if schedule["specific_date"] != "2026-04-10" {
		t.Fatalf("expected specific_date '2026-04-10', got %v", schedule["specific_date"])
	}
	if schedule["day_of_week"] != nil {
		t.Fatalf("expected day_of_week nil for one-off, got %v", schedule["day_of_week"])
	}
}

func TestOneOffScheduleAppearsOnCorrectDate(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title":    "Special task",
		"category": "bonus",
	}, adminHeaders())

	// Schedule as one-off on 2026-04-15
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to":   kidID,
		"specific_date": "2026-04-15",
	}, adminHeaders())

	// Should appear on 2026-04-15
	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/chores?view=daily&date=2026-04-15", kidID), nil, adminHeaders(), http.StatusOK)
	var chores []map[string]any
	decodeBody(t, resp, &chores)
	if len(chores) != 1 {
		t.Fatalf("expected 1 chore on 2026-04-15, got %d", len(chores))
	}
	if chores[0]["title"] != "Special task" {
		t.Fatalf("expected 'Special task', got %v", chores[0]["title"])
	}

	// Should NOT appear on 2026-04-16
	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/chores?view=daily&date=2026-04-16", kidID), nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &chores)
	if len(chores) != 0 {
		t.Fatalf("expected 0 chores on 2026-04-16, got %d", len(chores))
	}

	// Should NOT appear on 2026-04-14
	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/chores?view=daily&date=2026-04-14", kidID), nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &chores)
	if len(chores) != 0 {
		t.Fatalf("expected 0 chores on 2026-04-14, got %d", len(chores))
	}
}

func TestOneOffScheduleCompletionAndPoints(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title":        "Quick assign chore",
		"category":     "bonus",
		"points_value": 15,
	}, adminHeaders())

	// Schedule as one-off
	resp := env.expectStatus(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to":   kidID,
		"specific_date": "2026-04-20",
	}, adminHeaders(), http.StatusCreated)
	var schedule map[string]any
	decodeBody(t, resp, &schedule)

	// Complete the one-off chore
	env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    kidID,
		"completion_date": "2026-04-20",
	}, adminHeaders(), http.StatusCreated)

	// Check points were credited
	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, adminHeaders(), http.StatusOK)
	var pts map[string]any
	decodeBody(t, resp, &pts)
	if pts["balance"].(float64) != 15 {
		t.Fatalf("expected 15 points for one-off chore, got %v", pts["balance"])
	}

	// Verify shows as completed in daily view
	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/chores?view=daily&date=2026-04-20", kidID), nil, adminHeaders(), http.StatusOK)
	var chores []map[string]any
	decodeBody(t, resp, &chores)
	if len(chores) != 1 {
		t.Fatalf("expected 1 chore, got %d", len(chores))
	}
	if chores[0]["completed"] != true {
		t.Fatal("expected one-off chore to show as completed")
	}
}

func TestOneOffScheduleInWeeklyView(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title":    "Weekly one-off",
		"category": "core",
	}, adminHeaders())

	// Schedule one-off for 2026-04-08 (Wednesday)
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to":   kidID,
		"specific_date": "2026-04-08",
	}, adminHeaders())

	// Weekly view for week of 2026-04-06 (Monday) should include it
	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/chores?view=weekly&date=2026-04-06", kidID), nil, adminHeaders(), http.StatusOK)
	var chores []map[string]any
	decodeBody(t, resp, &chores)
	if len(chores) != 1 {
		t.Fatalf("expected 1 chore in weekly view, got %d", len(chores))
	}

	// Weekly view for a different week should NOT include it
	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/chores?view=weekly&date=2026-04-13", kidID), nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &chores)
	if len(chores) != 0 {
		t.Fatalf("expected 0 chores in different week, got %d", len(chores))
	}
}

// =================== AI VERIFICATION TESTS ===================

func TestCompleteChoreAIRejectedAllowsRetry(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Create chore
	env.request(t, "POST", "/api/chores", map[string]any{
		"title":    "Clean Room",
		"category": "core",
	}, adminHeaders())

	// Schedule for Wednesday (day 3)
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID,
		"day_of_week": 3,
	}, adminHeaders())

	// Insert an ai_rejected completion directly into the DB to simulate AI rejection
	_, err := env.db.Exec(
		`INSERT INTO chore_completions (chore_schedule_id, completed_by, status, photo_url, completion_date, ai_feedback, ai_confidence)
		 VALUES (?, ?, 'ai_rejected', '/uploads/test.jpg', '2026-03-11', 'The room still has toys on the floor.', 0.3)`,
		1, kidID)
	if err != nil {
		t.Fatalf("failed to insert ai_rejected completion: %v", err)
	}

	// Retry the completion — should succeed because ai_rejected allows retry
	resp := env.request(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    kidID,
		"completion_date": "2026-03-11",
	}, adminHeaders())
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for retry after ai_rejected, got %d", resp.StatusCode)
	}

	// Verify the completion is now approved (since no AI reviewer is configured in test)
	var completion map[string]any
	decodeBody(t, resp, &completion)
	if completion["status"] != "approved" {
		t.Errorf("expected status=approved for retry, got %v", completion["status"])
	}
}

func TestCompleteChoreNormalRejectDoesNotAllowRetry(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title":    "Sweep Floor",
		"category": "core",
	}, adminHeaders())

	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID,
		"day_of_week": 3,
	}, adminHeaders())

	// Complete normally first
	env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    kidID,
		"completion_date": "2026-03-11",
	}, adminHeaders(), http.StatusCreated)

	// Try to complete again — should conflict (status is approved, not ai_rejected)
	resp := env.request(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by":    kidID,
		"completion_date": "2026-03-11",
	}, adminHeaders())
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate completion, got %d", resp.StatusCode)
	}
}

func TestScheduledChoresIncludeAIFeedback(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title":    "Make Bed",
		"category": "required",
	}, adminHeaders())

	// Schedule for Wednesday (day 3)
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID,
		"day_of_week": 3,
	}, adminHeaders())

	// Insert an ai_rejected completion with feedback
	_, err := env.db.Exec(
		`INSERT INTO chore_completions (chore_schedule_id, completed_by, status, photo_url, completion_date, ai_feedback, ai_confidence)
		 VALUES (?, ?, 'ai_rejected', '/uploads/bed.jpg', '2026-03-11', 'Almost there! Straighten the pillows.', 0.4)`,
		1, kidID)
	if err != nil {
		t.Fatalf("failed to insert completion: %v", err)
	}

	// Get daily chores — should include AI feedback fields
	resp := env.expectStatus(t, "GET",
		fmt.Sprintf("/api/users/%d/chores?view=daily&date=2026-03-11", kidID),
		nil, adminHeaders(), http.StatusOK)

	var chores []map[string]any
	decodeBody(t, resp, &chores)
	if len(chores) != 1 {
		t.Fatalf("expected 1 chore, got %d", len(chores))
	}

	chore := chores[0]

	// ai_rejected should NOT be "completed" from the kid's perspective
	if chore["completed"] != false {
		t.Error("expected completed=false for ai_rejected chore")
	}

	// completion_status should be present
	if chore["completion_status"] != "ai_rejected" {
		t.Errorf("expected completion_status=ai_rejected, got %v", chore["completion_status"])
	}

	// ai_feedback should be present
	if chore["ai_feedback"] != "Almost there! Straighten the pillows." {
		t.Errorf("expected ai_feedback to be set, got %v", chore["ai_feedback"])
	}
}

func TestScheduledChoresIncludeTTSDescription(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	// Create chore
	env.request(t, "POST", "/api/chores", map[string]any{
		"title":    "Feed Cat",
		"category": "required",
	}, adminHeaders())

	// Set TTS description directly in DB (simulating AI TTS generation)
	_, err := env.db.Exec(`UPDATE chores SET tts_description = ? WHERE id = 1`,
		"Time to feed the kitty! Give them fresh food and water.")
	if err != nil {
		t.Fatalf("failed to set tts_description: %v", err)
	}

	// Schedule for Wednesday (day 3)
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID,
		"day_of_week": 3,
	}, adminHeaders())

	// Get daily chores
	resp := env.expectStatus(t, "GET",
		fmt.Sprintf("/api/users/%d/chores?view=daily&date=2026-03-11", kidID),
		nil, adminHeaders(), http.StatusOK)

	var chores []map[string]any
	decodeBody(t, resp, &chores)
	if len(chores) != 1 {
		t.Fatalf("expected 1 chore, got %d", len(chores))
	}

	// TTS description should be included in the scheduled chore response
	if chores[0]["tts_description"] != "Time to feed the kitty! Give them fresh food and water." {
		t.Errorf("expected tts_description to be set, got %v", chores[0]["tts_description"])
	}
}

func TestChoreGetIncludesTTSDescription(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)

	// Create chore
	env.expectStatus(t, "POST", "/api/chores", map[string]any{
		"title":    "Brush Teeth",
		"category": "required",
	}, adminHeaders(), http.StatusCreated)

	// Set TTS description directly in DB (simulating AI TTS generation)
	_, err := env.db.Exec(`UPDATE chores SET tts_description = ? WHERE id = 1`,
		"Brush those pearly whites!")
	if err != nil {
		t.Fatalf("failed to set tts_description: %v", err)
	}

	// Get chore via API should include tts_description
	resp := env.expectStatus(t, "GET", "/api/chores/1", nil, adminHeaders(), http.StatusOK)
	var chore map[string]any
	decodeBody(t, resp, &chore)
	if chore["tts_description"] != "Brush those pearly whites!" {
		t.Errorf("expected tts_description on get, got %v", chore["tts_description"])
	}
}
