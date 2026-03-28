package api_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	msqlite "github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/mattn/go-sqlite3"

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

	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on&_busy_timeout=5000")
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
	m, err := migrate.NewWithInstance("iofs", source, "sqlite3", driver)
	if err != nil {
		t.Fatalf("failed to create migrator: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("failed to run migrations: %v", err)
	}

	s := store.New(db)
	d := webhook.NewDispatcher(s)
	router := api.NewRouter(s, d)
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
