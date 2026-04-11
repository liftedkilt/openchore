package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	msqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
	"github.com/liftedkilt/openchore/migrations"
	_ "modernc.org/sqlite"
)

type testEnv struct {
	db         *sql.DB
	store      *store.Store
	dispatcher *Dispatcher
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
	d := NewDispatcher(s)

	t.Cleanup(func() {
		db.Close()
	})

	return &testEnv{db: db, store: s, dispatcher: d}
}

// createParentUser creates a parent user and returns its ID.
func createParentUser(t *testing.T, env *testEnv, name string) int64 {
	t.Helper()
	u := &model.User{Name: name, Role: "admin", AvatarURL: ""}
	if err := env.store.CreateUser(context.Background(), u); err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}
	return u.ID
}

// createChildUser creates a child user and returns its ID.
func createChildUser(t *testing.T, env *testEnv, name string) int64 {
	t.Helper()
	u := &model.User{Name: name, Role: "child", AvatarURL: ""}
	if err := env.store.CreateUser(context.Background(), u); err != nil {
		t.Fatalf("failed to create child user: %v", err)
	}
	return u.ID
}

// createWebhook inserts a webhook directly into the DB and returns its ID.
func createWebhook(t *testing.T, env *testEnv, url, secret, events string, active bool) int64 {
	t.Helper()
	wh := &model.Webhook{URL: url, Secret: secret, Events: events, Active: active}
	if err := env.store.CreateWebhook(context.Background(), wh); err != nil {
		t.Fatalf("failed to create webhook: %v", err)
	}
	return wh.ID
}

// createChoreWithSchedule creates a chore and a schedule, returning both IDs.
func createChoreWithSchedule(t *testing.T, env *testEnv, parentID, childID int64, category string, dayOfWeek int, dueBy *string, missedPenalty int) (choreID, scheduleID int64) {
	t.Helper()
	c := &model.Chore{
		Title:              fmt.Sprintf("Test Chore %d", time.Now().UnixNano()),
		Description:        "test chore",
		Category:           category,
		PointsValue:        10,
		MissedPenaltyValue: missedPenalty,
		Source:             "manual",
		CreatedBy:          parentID,
	}
	if err := env.store.CreateChore(context.Background(), c); err != nil {
		t.Fatalf("failed to create chore: %v", err)
	}

	dow := dayOfWeek
	cs := &model.ChoreSchedule{
		ChoreID:          c.ID,
		AssignedTo:       childID,
		AssignmentType:   "individual",
		DayOfWeek:        &dow,
		PointsMultiplier: 1.0,
		DueBy:            dueBy,
		ExpiryPenalty:    "none",
	}
	if err := env.store.CreateSchedule(context.Background(), cs); err != nil {
		t.Fatalf("failed to create schedule: %v", err)
	}
	return c.ID, cs.ID
}

// --- Dispatcher Tests ---

func TestNewDispatcher(t *testing.T) {
	env := setupTest(t)
	d := NewDispatcher(env.store)
	if d == nil {
		t.Fatal("NewDispatcher returned nil")
	}
	if d.store != env.store {
		t.Error("dispatcher store mismatch")
	}
	if d.client == nil {
		t.Error("dispatcher client is nil")
	}
}

func TestDispatcher_Fire_SuccessfulDelivery(t *testing.T) {
	env := setupTest(t)

	var mu sync.Mutex
	var receivedPayload Payload
	var receivedEvent string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		receivedEvent = r.Header.Get("X-OpenChore-Event")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	createWebhook(t, env, ts.URL, "", "*", true)

	env.dispatcher.Fire(EventChoreCompleted, map[string]string{"chore": "dishes"})

	// Fire is async, wait for delivery
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if receivedEvent != EventChoreCompleted {
		t.Errorf("expected event header %q, got %q", EventChoreCompleted, receivedEvent)
	}
	if receivedPayload.Event != EventChoreCompleted {
		t.Errorf("expected payload event %q, got %q", EventChoreCompleted, receivedPayload.Event)
	}
}

func TestDispatcher_Fire_HMACSignature(t *testing.T) {
	env := setupTest(t)

	secret := "my-webhook-secret"
	var mu sync.Mutex
	var receivedSig string
	var receivedBody []byte

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		receivedSig = r.Header.Get("X-OpenChore-Signature")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	createWebhook(t, env, ts.URL, secret, "*", true)

	env.dispatcher.Fire(EventRewardRedeemed, map[string]string{"reward": "ice cream"})
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if receivedSig == "" {
		t.Fatal("expected HMAC signature header, got empty")
	}

	// Verify the signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(receivedBody)
	expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if receivedSig != expectedSig {
		t.Errorf("HMAC mismatch:\n  got:  %s\n  want: %s", receivedSig, expectedSig)
	}
}

func TestDispatcher_Fire_NoSignatureWithoutSecret(t *testing.T) {
	env := setupTest(t)

	var mu sync.Mutex
	var receivedSig string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		receivedSig = r.Header.Get("X-OpenChore-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	createWebhook(t, env, ts.URL, "", "*", true)

	env.dispatcher.Fire(EventChoreCompleted, map[string]string{"test": "val"})
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if receivedSig != "" {
		t.Errorf("expected no signature header when no secret, got %q", receivedSig)
	}
}

func TestDispatcher_Fire_EventFiltering(t *testing.T) {
	env := setupTest(t)

	var mu sync.Mutex
	callCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Webhook only subscribed to chore.completed
	createWebhook(t, env, ts.URL, "", EventChoreCompleted, true)

	// Fire a non-matching event
	env.dispatcher.Fire(EventRewardRedeemed, map[string]string{"reward": "toy"})
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	count := callCount
	mu.Unlock()

	if count != 0 {
		t.Errorf("expected 0 calls for non-matching event, got %d", count)
	}

	// Fire a matching event
	env.dispatcher.Fire(EventChoreCompleted, map[string]string{"chore": "dishes"})
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	count = callCount
	mu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 call for matching event, got %d", count)
	}
}

func TestDispatcher_Fire_MultipleEvents(t *testing.T) {
	env := setupTest(t)

	var mu sync.Mutex
	callCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Webhook subscribed to two events
	createWebhook(t, env, ts.URL, "", "chore.completed,reward.redeemed", true)

	env.dispatcher.Fire(EventChoreCompleted, nil)
	time.Sleep(300 * time.Millisecond)

	env.dispatcher.Fire(EventRewardRedeemed, nil)
	time.Sleep(300 * time.Millisecond)

	env.dispatcher.Fire(EventChoreExpired, nil) // should NOT match
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	count := callCount
	mu.Unlock()

	if count != 2 {
		t.Errorf("expected 2 calls for matching events, got %d", count)
	}
}

func TestDispatcher_Fire_InactiveWebhookIgnored(t *testing.T) {
	env := setupTest(t)

	var mu sync.Mutex
	callCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Inactive webhook
	createWebhook(t, env, ts.URL, "", "*", false)

	env.dispatcher.Fire(EventChoreCompleted, map[string]string{"test": "val"})
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	count := callCount
	mu.Unlock()

	if count != 0 {
		t.Errorf("expected 0 calls for inactive webhook, got %d", count)
	}
}

func TestDispatcher_Fire_FailedDelivery500(t *testing.T) {
	env := setupTest(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer ts.Close()

	whID := createWebhook(t, env, ts.URL, "", "*", true)

	env.dispatcher.Fire(EventChoreCompleted, map[string]string{"test": "fail"})
	time.Sleep(500 * time.Millisecond)

	// Verify delivery was logged with error status
	deliveries, err := env.store.ListWebhookDeliveries(context.Background(), whID, 10)
	if err != nil {
		t.Fatalf("failed to list deliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery log, got %d", len(deliveries))
	}
	if deliveries[0].StatusCode == nil || *deliveries[0].StatusCode != 500 {
		t.Errorf("expected status code 500, got %v", deliveries[0].StatusCode)
	}
	if deliveries[0].ResponseBody != "server error" {
		t.Errorf("expected response body 'server error', got %q", deliveries[0].ResponseBody)
	}
}

func TestDispatcher_Fire_UnreachableEndpoint(t *testing.T) {
	env := setupTest(t)

	// Use an endpoint that will refuse connections
	whID := createWebhook(t, env, "http://127.0.0.1:1", "", "*", true)

	env.dispatcher.Fire(EventChoreCompleted, map[string]string{"test": "unreachable"})
	time.Sleep(1 * time.Second)

	deliveries, err := env.store.ListWebhookDeliveries(context.Background(), whID, 10)
	if err != nil {
		t.Fatalf("failed to list deliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery log, got %d", len(deliveries))
	}
	if deliveries[0].Error == "" {
		t.Error("expected error message for unreachable endpoint, got empty")
	}
	if deliveries[0].StatusCode != nil {
		t.Errorf("expected nil status code for connection error, got %v", *deliveries[0].StatusCode)
	}
}

func TestDispatcher_Fire_DeliveryLogged(t *testing.T) {
	env := setupTest(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("accepted"))
	}))
	defer ts.Close()

	whID := createWebhook(t, env, ts.URL, "", "*", true)

	env.dispatcher.Fire(EventChoreCompleted, map[string]string{"key": "value"})
	time.Sleep(500 * time.Millisecond)

	deliveries, err := env.store.ListWebhookDeliveries(context.Background(), whID, 10)
	if err != nil {
		t.Fatalf("failed to list deliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliveries))
	}

	d := deliveries[0]
	if d.WebhookID != whID {
		t.Errorf("expected webhook_id %d, got %d", whID, d.WebhookID)
	}
	if d.Event != EventChoreCompleted {
		t.Errorf("expected event %q, got %q", EventChoreCompleted, d.Event)
	}
	if d.StatusCode == nil || *d.StatusCode != 200 {
		t.Errorf("expected status code 200, got %v", d.StatusCode)
	}
	if d.ResponseBody != "accepted" {
		t.Errorf("expected response body 'accepted', got %q", d.ResponseBody)
	}
	if d.Error != "" {
		t.Errorf("expected no error, got %q", d.Error)
	}

	// Verify payload is valid JSON with expected structure
	var payload Payload
	if err := json.Unmarshal([]byte(d.Payload), &payload); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if payload.Event != EventChoreCompleted {
		t.Errorf("payload event mismatch: got %q", payload.Event)
	}
}

func TestDispatcher_Fire_NoWebhooks(t *testing.T) {
	env := setupTest(t)

	// Fire with no webhooks registered -- should not panic
	env.dispatcher.Fire(EventChoreCompleted, map[string]string{"test": "nothing"})
	time.Sleep(300 * time.Millisecond)
	// Success if no panic
}

// --- ExpiryChecker Tests ---

func TestNewExpiryChecker(t *testing.T) {
	env := setupTest(t)
	ec := NewExpiryChecker(env.store, env.dispatcher)
	if ec == nil {
		t.Fatal("NewExpiryChecker returned nil")
	}
	if ec.interval != 1*time.Minute {
		t.Errorf("expected interval 1m, got %v", ec.interval)
	}
	if ec.fired == nil {
		t.Error("fired map is nil")
	}
}

func TestExpiryChecker_StartAndCancel(t *testing.T) {
	env := setupTest(t)
	ec := NewExpiryChecker(env.store, env.dispatcher)
	ec.interval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		ec.Start(ctx)
		close(done)
	}()

	// Let it tick a couple of times
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Success: Start returned after cancel
	case <-time.After(2 * time.Second):
		t.Fatal("ExpiryChecker.Start did not return after context cancel")
	}
}

func TestFiredKey(t *testing.T) {
	key := firedKey(42, "2026-03-28")
	expected := "2026-03-28:42"
	if key != expected {
		t.Errorf("firedKey(42, '2026-03-28') = %q, want %q", key, expected)
	}
}

func TestFiredKeyDeduplication(t *testing.T) {
	env := setupTest(t)

	var mu sync.Mutex
	callCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	createWebhook(t, env, ts.URL, "", "*", true)

	parentID := createParentUser(t, env, "Parent")
	childID := createChildUser(t, env, "Child")

	// Create a chore with a due_by time in the past for today
	today := time.Now()
	dow := int(today.Weekday())
	dueBy := "00:01" // Already past
	createChoreWithSchedule(t, env, parentID, childID, "required", dow, &dueBy, 5)

	ec := NewExpiryChecker(env.store, env.dispatcher)
	ec.interval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go ec.Start(ctx)

	// Wait for a few ticks
	time.Sleep(300 * time.Millisecond)
	cancel()

	mu.Lock()
	count := callCount
	mu.Unlock()

	// The expired chore should fire exactly once (deduplication)
	if count != 1 {
		t.Errorf("expected exactly 1 webhook fire for deduplicated expired chore, got %d", count)
	}
}

func TestExpiryChecker_NonExpiredChoresNotFired(t *testing.T) {
	env := setupTest(t)

	var mu sync.Mutex
	callCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	createWebhook(t, env, ts.URL, "", "*", true)

	parentID := createParentUser(t, env, "Parent")
	childID := createChildUser(t, env, "Child")

	// Create a chore with a due_by time far in the future
	today := time.Now()
	dow := int(today.Weekday())
	dueBy := "23:59"
	createChoreWithSchedule(t, env, parentID, childID, "required", dow, &dueBy, 5)

	ec := NewExpiryChecker(env.store, env.dispatcher)
	ec.interval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go ec.Start(ctx)

	time.Sleep(300 * time.Millisecond)
	cancel()

	mu.Lock()
	count := callCount
	mu.Unlock()

	if count != 0 {
		t.Errorf("expected 0 webhook fires for non-expired chore, got %d", count)
	}
}

// --- DecayChecker (Penalty) Tests ---

func TestNewDecayChecker(t *testing.T) {
	env := setupTest(t)
	dc := NewDecayChecker(env.store, env.dispatcher)
	if dc == nil {
		t.Fatal("NewDecayChecker returned nil")
	}
	if dc.interval != 15*time.Minute {
		t.Errorf("expected interval 15m, got %v", dc.interval)
	}
}

func TestDecayChecker_StartAndCancel(t *testing.T) {
	env := setupTest(t)
	dc := NewDecayChecker(env.store, env.dispatcher)
	dc.interval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		dc.Start(ctx)
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("DecayChecker.Start did not return after context cancel")
	}
}

func TestDecayChecker_PenalizeMissedRequiredChore(t *testing.T) {
	env := setupTest(t)

	var mu sync.Mutex
	var firedEvents []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		event := r.Header.Get("X-OpenChore-Event")
		firedEvents = append(firedEvents, event)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	createWebhook(t, env, ts.URL, "", "*", true)

	parentID := createParentUser(t, env, "Parent")
	childID := createChildUser(t, env, "Child")

	// Create a required chore for yesterday's day-of-week with a missed penalty
	yesterday := time.Now().AddDate(0, 0, -1)
	dow := int(yesterday.Weekday())
	_, scheduleID := createChoreWithSchedule(t, env, parentID, childID, "required", dow, nil, 5)

	dc := NewDecayChecker(env.store, env.dispatcher)

	// Directly call check
	dc.check(context.Background())

	// Wait for async webhook fire
	time.Sleep(500 * time.Millisecond)

	// Verify penalty was applied
	hasPenalty, err := env.store.HasMissedChorePenalty(context.Background(), scheduleID, yesterday.Format(model.DateFormat))
	if err != nil {
		t.Fatalf("HasMissedChorePenalty error: %v", err)
	}
	if !hasPenalty {
		t.Error("expected penalty to be recorded for missed required chore")
	}

	// Verify webhook was fired
	mu.Lock()
	events := make([]string, len(firedEvents))
	copy(events, firedEvents)
	mu.Unlock()

	found := false
	for _, e := range events {
		if e == EventChoreMissed {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %q webhook event to be fired, got events: %v", EventChoreMissed, events)
	}
}

func TestDecayChecker_NoPenaltyForBonusChore(t *testing.T) {
	env := setupTest(t)

	parentID := createParentUser(t, env, "Parent")
	childID := createChildUser(t, env, "Child")

	// Create a bonus chore for yesterday (should NOT be penalized)
	yesterday := time.Now().AddDate(0, 0, -1)
	dow := int(yesterday.Weekday())
	_, scheduleID := createChoreWithSchedule(t, env, parentID, childID, "bonus", dow, nil, 5)

	dc := NewDecayChecker(env.store, env.dispatcher)
	dc.check(context.Background())

	hasPenalty, err := env.store.HasMissedChorePenalty(context.Background(), scheduleID, yesterday.Format(model.DateFormat))
	if err != nil {
		t.Fatalf("HasMissedChorePenalty error: %v", err)
	}
	if hasPenalty {
		t.Error("bonus chores should not be penalized")
	}
}

func TestDecayChecker_NoPenaltyForZeroPenaltyValue(t *testing.T) {
	env := setupTest(t)

	parentID := createParentUser(t, env, "Parent")
	childID := createChildUser(t, env, "Child")

	// Required chore with 0 penalty value
	yesterday := time.Now().AddDate(0, 0, -1)
	dow := int(yesterday.Weekday())
	_, scheduleID := createChoreWithSchedule(t, env, parentID, childID, "required", dow, nil, 0)

	dc := NewDecayChecker(env.store, env.dispatcher)
	dc.check(context.Background())

	hasPenalty, err := env.store.HasMissedChorePenalty(context.Background(), scheduleID, yesterday.Format(model.DateFormat))
	if err != nil {
		t.Fatalf("HasMissedChorePenalty error: %v", err)
	}
	if hasPenalty {
		t.Error("chores with 0 penalty value should not be penalized")
	}
}

func TestDecayChecker_NoDuplicatePenalty(t *testing.T) {
	env := setupTest(t)

	parentID := createParentUser(t, env, "Parent")
	childID := createChildUser(t, env, "Child")

	yesterday := time.Now().AddDate(0, 0, -1)
	dow := int(yesterday.Weekday())
	_, scheduleID := createChoreWithSchedule(t, env, parentID, childID, "required", dow, nil, 5)

	dc := NewDecayChecker(env.store, env.dispatcher)

	// Run check twice
	dc.check(context.Background())
	dc.check(context.Background())

	// Count penalty transactions for this schedule
	var count int
	err := env.db.QueryRow(
		`SELECT COUNT(*) FROM point_transactions WHERE reason = 'missed_chore' AND reference_id = ?`,
		scheduleID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 penalty transaction, got %d", count)
	}
}

func TestDecayChecker_OnlyPenalizesChildren(t *testing.T) {
	env := setupTest(t)

	parentID := createParentUser(t, env, "Parent")

	// Create a required chore assigned to the parent (not a child)
	yesterday := time.Now().AddDate(0, 0, -1)
	dow := int(yesterday.Weekday())
	_, scheduleID := createChoreWithSchedule(t, env, parentID, parentID, "required", dow, nil, 5)

	dc := NewDecayChecker(env.store, env.dispatcher)
	dc.check(context.Background())

	hasPenalty, err := env.store.HasMissedChorePenalty(context.Background(), scheduleID, yesterday.Format(model.DateFormat))
	if err != nil {
		t.Fatalf("HasMissedChorePenalty error: %v", err)
	}
	if hasPenalty {
		t.Error("parents should not be penalized for missed chores")
	}
}

// --- PointsDecayChecker Tests ---

func TestNewPointsDecayChecker(t *testing.T) {
	env := setupTest(t)
	pdc := NewPointsDecayChecker(env.store, env.dispatcher)
	if pdc == nil {
		t.Fatal("NewPointsDecayChecker returned nil")
	}
	if pdc.interval != 15*time.Minute {
		t.Errorf("expected interval 15m, got %v", pdc.interval)
	}
}

func TestPointsDecayChecker_StartAndCancel(t *testing.T) {
	env := setupTest(t)
	pdc := NewPointsDecayChecker(env.store, env.dispatcher)
	pdc.interval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		pdc.Start(ctx)
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("PointsDecayChecker.Start did not return after context cancel")
	}
}

func TestPointsDecayChecker_DecaysWhenRequiredChoreMissed(t *testing.T) {
	env := setupTest(t)
	ctx := context.Background()

	var mu sync.Mutex
	var firedEvents []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		firedEvents = append(firedEvents, r.Header.Get("X-OpenChore-Event"))
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	createWebhook(t, env, ts.URL, "", "*", true)

	parentID := createParentUser(t, env, "Parent")
	childID := createChildUser(t, env, "Child")

	// Seed a balance so we can observe the debit.
	if err := env.store.AdminAdjustPoints(ctx, childID, 100, ""); err != nil {
		t.Fatalf("AdminAdjustPoints: %v", err)
	}

	// A required chore that was scheduled yesterday and NOT completed.
	yesterday := time.Now().AddDate(0, 0, -1)
	createChoreWithSchedule(t, env, parentID, childID, "required", int(yesterday.Weekday()), nil, 0)

	// Enable decay for this child at 7 points/day.
	if err := env.store.SetUserDecayConfig(ctx, &model.UserDecayConfig{
		UserID: childID, Enabled: true, DecayRate: 7, DecayIntervalHours: 24,
	}); err != nil {
		t.Fatalf("SetUserDecayConfig: %v", err)
	}

	pdc := NewPointsDecayChecker(env.store, env.dispatcher)
	pdc.check(ctx)

	balance, err := env.store.GetPointBalance(ctx, childID)
	if err != nil {
		t.Fatalf("GetPointBalance: %v", err)
	}
	if balance != 93 {
		t.Errorf("expected balance 93 after decay, got %d", balance)
	}

	// last_decay_at should be set so we don't re-decay on the next tick.
	cfg, _ := env.store.GetUserDecayConfig(ctx, childID)
	if cfg.LastDecayAt == nil {
		t.Error("expected last_decay_at to be set after decay")
	}

	// Running again immediately must not decay again (interval respected).
	pdc.check(ctx)
	balance2, _ := env.store.GetPointBalance(ctx, childID)
	if balance2 != 93 {
		t.Errorf("expected balance to remain 93 on second check, got %d", balance2)
	}

	// Webhook for points.decayed should have fired exactly once.
	time.Sleep(300 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	decayedCount := 0
	for _, e := range firedEvents {
		if e == EventPointsDecayed {
			decayedCount++
		}
	}
	if decayedCount != 1 {
		t.Errorf("expected 1 %q webhook event, got %d (events=%v)", EventPointsDecayed, decayedCount, firedEvents)
	}
}

func TestPointsDecayChecker_NoDecayWhenAllChoresComplete(t *testing.T) {
	env := setupTest(t)
	ctx := context.Background()

	parentID := createParentUser(t, env, "Parent")
	childID := createChildUser(t, env, "Child")
	if err := env.store.AdminAdjustPoints(ctx, childID, 100, ""); err != nil {
		t.Fatalf("AdminAdjustPoints: %v", err)
	}

	yesterday := time.Now().AddDate(0, 0, -1)
	_, scheduleID := createChoreWithSchedule(t, env, parentID, childID, "core", int(yesterday.Weekday()), nil, 0)

	// Mark yesterday's chore as completed.
	if err := env.store.CompleteChore(ctx, &model.ChoreCompletion{
		ChoreScheduleID: scheduleID,
		CompletedBy:     childID,
		Status:          model.StatusApproved,
		CompletionDate:  yesterday.Format(model.DateFormat),
	}); err != nil {
		t.Fatalf("CompleteChore: %v", err)
	}

	if err := env.store.SetUserDecayConfig(ctx, &model.UserDecayConfig{
		UserID: childID, Enabled: true, DecayRate: 5, DecayIntervalHours: 24,
	}); err != nil {
		t.Fatalf("SetUserDecayConfig: %v", err)
	}

	pdc := NewPointsDecayChecker(env.store, env.dispatcher)
	pdc.check(ctx)

	balance, _ := env.store.GetPointBalance(ctx, childID)
	if balance != 100 {
		t.Errorf("expected balance to stay at 100, got %d", balance)
	}
}

func TestPointsDecayChecker_IgnoresBonusChores(t *testing.T) {
	env := setupTest(t)
	ctx := context.Background()

	parentID := createParentUser(t, env, "Parent")
	childID := createChildUser(t, env, "Child")
	if err := env.store.AdminAdjustPoints(ctx, childID, 50, ""); err != nil {
		t.Fatalf("AdminAdjustPoints: %v", err)
	}

	// Only a bonus chore scheduled for yesterday, incomplete.
	yesterday := time.Now().AddDate(0, 0, -1)
	createChoreWithSchedule(t, env, parentID, childID, "bonus", int(yesterday.Weekday()), nil, 0)

	if err := env.store.SetUserDecayConfig(ctx, &model.UserDecayConfig{
		UserID: childID, Enabled: true, DecayRate: 5, DecayIntervalHours: 24,
	}); err != nil {
		t.Fatalf("SetUserDecayConfig: %v", err)
	}

	pdc := NewPointsDecayChecker(env.store, env.dispatcher)
	pdc.check(ctx)

	balance, _ := env.store.GetPointBalance(ctx, childID)
	if balance != 50 {
		t.Errorf("bonus-only misses must not trigger decay; balance=%d want 50", balance)
	}
}

func TestPointsDecayChecker_RespectsIntervalHours(t *testing.T) {
	env := setupTest(t)
	ctx := context.Background()

	parentID := createParentUser(t, env, "Parent")
	childID := createChildUser(t, env, "Child")
	if err := env.store.AdminAdjustPoints(ctx, childID, 100, ""); err != nil {
		t.Fatalf("AdminAdjustPoints: %v", err)
	}

	yesterday := time.Now().AddDate(0, 0, -1)
	createChoreWithSchedule(t, env, parentID, childID, "required", int(yesterday.Weekday()), nil, 0)

	if err := env.store.SetUserDecayConfig(ctx, &model.UserDecayConfig{
		UserID: childID, Enabled: true, DecayRate: 5, DecayIntervalHours: 24,
	}); err != nil {
		t.Fatalf("SetUserDecayConfig: %v", err)
	}

	// Simulate a decay that already ran 1 hour ago.
	recent := time.Now().Add(-1 * time.Hour)
	if err := env.store.UpdateLastDecayAt(ctx, childID, recent); err != nil {
		t.Fatalf("UpdateLastDecayAt: %v", err)
	}

	pdc := NewPointsDecayChecker(env.store, env.dispatcher)
	pdc.check(ctx)

	balance, _ := env.store.GetPointBalance(ctx, childID)
	if balance != 100 {
		t.Errorf("expected balance to stay at 100 (interval not elapsed), got %d", balance)
	}
}

func TestPointsDecayChecker_SkipsDisabledUsers(t *testing.T) {
	env := setupTest(t)
	ctx := context.Background()

	parentID := createParentUser(t, env, "Parent")
	childID := createChildUser(t, env, "Child")
	if err := env.store.AdminAdjustPoints(ctx, childID, 100, ""); err != nil {
		t.Fatalf("AdminAdjustPoints: %v", err)
	}

	yesterday := time.Now().AddDate(0, 0, -1)
	createChoreWithSchedule(t, env, parentID, childID, "required", int(yesterday.Weekday()), nil, 0)

	// Explicitly disabled.
	if err := env.store.SetUserDecayConfig(ctx, &model.UserDecayConfig{
		UserID: childID, Enabled: false, DecayRate: 5, DecayIntervalHours: 24,
	}); err != nil {
		t.Fatalf("SetUserDecayConfig: %v", err)
	}

	pdc := NewPointsDecayChecker(env.store, env.dispatcher)
	pdc.check(ctx)

	balance, _ := env.store.GetPointBalance(ctx, childID)
	if balance != 100 {
		t.Errorf("disabled user should not be decayed; balance=%d want 100", balance)
	}
}

func TestPointsDecayChecker_ClampsToNonNegativeBalance(t *testing.T) {
	env := setupTest(t)
	ctx := context.Background()

	parentID := createParentUser(t, env, "Parent")
	childID := createChildUser(t, env, "Child")

	// Only 3 points in the bank; decay rate is 10.
	if err := env.store.AdminAdjustPoints(ctx, childID, 3, ""); err != nil {
		t.Fatalf("AdminAdjustPoints: %v", err)
	}

	yesterday := time.Now().AddDate(0, 0, -1)
	createChoreWithSchedule(t, env, parentID, childID, "required", int(yesterday.Weekday()), nil, 0)

	if err := env.store.SetUserDecayConfig(ctx, &model.UserDecayConfig{
		UserID: childID, Enabled: true, DecayRate: 10, DecayIntervalHours: 24,
	}); err != nil {
		t.Fatalf("SetUserDecayConfig: %v", err)
	}

	pdc := NewPointsDecayChecker(env.store, env.dispatcher)
	pdc.check(ctx)

	balance, _ := env.store.GetPointBalance(ctx, childID)
	if balance != 0 {
		t.Errorf("expected balance clamped to 0, got %d", balance)
	}
}
