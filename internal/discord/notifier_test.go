package discord

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	msqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/liftedkilt/openchore/internal/store"
	"github.com/liftedkilt/openchore/migrations"
	_ "modernc.org/sqlite"
)

// setupTestStore creates an in-memory SQLite store with migrations applied.
func setupTestStore(t *testing.T) *store.Store {
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

	t.Cleanup(func() { db.Close() })
	return store.New(db)
}

// capturedRequest holds details of a request received by the test server.
type capturedRequest struct {
	Method      string
	ContentType string
	Payload     discordPayload
}

// setupWebhookServer creates an httptest server that captures incoming requests.
// Returns the server and a function to retrieve captured requests.
func setupWebhookServer(t *testing.T) (*httptest.Server, func() []capturedRequest) {
	t.Helper()
	var mu sync.Mutex
	var captured []capturedRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload discordPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("failed to decode request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		captured = append(captured, capturedRequest{
			Method:      r.Method,
			ContentType: r.Header.Get("Content-Type"),
			Payload:     payload,
		})
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))

	t.Cleanup(srv.Close)
	return srv, func() []capturedRequest {
		mu.Lock()
		defer mu.Unlock()
		c := make([]capturedRequest, len(captured))
		copy(c, captured)
		return c
	}
}

// newTestNotifier creates a Notifier backed by the given store, with the
// webhook URL already configured to point at the test server.
func newTestNotifier(t *testing.T, s *store.Store, webhookURL string) *Notifier {
	t.Helper()
	ctx := context.Background()
	if err := s.SetSetting(ctx, SettingWebhookURL, webhookURL); err != nil {
		t.Fatalf("failed to set webhook URL: %v", err)
	}
	return NewNotifier(s)
}

// waitForRequests polls until the expected number of requests arrive, or times out.
func waitForRequests(getReqs func() []capturedRequest, count int, timeout time.Duration) []capturedRequest {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		reqs := getReqs()
		if len(reqs) >= count {
			return reqs
		}
		time.Sleep(10 * time.Millisecond)
	}
	return getReqs()
}

// --- Tests for no-op when webhook is not configured ---

func TestNotifier_NoOp_WhenNoWebhookConfigured(t *testing.T) {
	s := setupTestStore(t)
	// Do NOT set any webhook URL in the store.
	n := NewNotifier(s)

	srv, getReqs := setupWebhookServer(t)
	defer srv.Close()

	// Call each notification method; none should send any HTTP request.
	n.send(discordEmbed{Title: "test"})
	// send is synchronous when called directly (not via goroutine).
	// Wait briefly to be sure.
	time.Sleep(50 * time.Millisecond)

	reqs := getReqs()
	if len(reqs) != 0 {
		t.Errorf("expected 0 requests when no webhook configured, got %d", len(reqs))
	}
}

// --- Tests for message formatting ---

func TestNotifyPendingApproval_MessageFormat(t *testing.T) {
	s := setupTestStore(t)
	srv, getReqs := setupWebhookServer(t)
	n := newTestNotifier(t, s, srv.URL)

	n.NotifyPendingApproval("Alice", "Wash Dishes", "https://example.com/photo.jpg")

	reqs := waitForRequests(getReqs, 1, 2*time.Second)
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}

	payload := reqs[0].Payload
	if len(payload.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(payload.Embeds))
	}
	embed := payload.Embeds[0]

	if embed.Title != "Chore Submitted for Approval" {
		t.Errorf("expected title 'Chore Submitted for Approval', got %q", embed.Title)
	}
	if !strings.Contains(embed.Description, "Alice") {
		t.Errorf("description should contain user name, got %q", embed.Description)
	}
	if !strings.Contains(embed.Description, "Wash Dishes") {
		t.Errorf("description should contain chore title, got %q", embed.Description)
	}
	if embed.Color != ColorYellow {
		t.Errorf("expected color %d (yellow), got %d", ColorYellow, embed.Color)
	}
	if embed.Thumbnail == nil || embed.Thumbnail.URL != "https://example.com/photo.jpg" {
		t.Errorf("expected thumbnail URL to be set, got %+v", embed.Thumbnail)
	}
	if len(embed.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(embed.Fields))
	}
	if embed.Fields[0].Name != "Kid" || embed.Fields[0].Value != "Alice" {
		t.Errorf("unexpected field 0: %+v", embed.Fields[0])
	}
	if embed.Fields[1].Name != "Chore" || embed.Fields[1].Value != "Wash Dishes" {
		t.Errorf("unexpected field 1: %+v", embed.Fields[1])
	}
	if embed.Timestamp == "" {
		t.Error("expected timestamp to be set")
	}
}

func TestNotifyPendingApproval_NoPhoto(t *testing.T) {
	s := setupTestStore(t)
	srv, getReqs := setupWebhookServer(t)
	n := newTestNotifier(t, s, srv.URL)

	n.NotifyPendingApproval("Bob", "Take Out Trash", "")

	reqs := waitForRequests(getReqs, 1, 2*time.Second)
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}

	embed := reqs[0].Payload.Embeds[0]
	if embed.Thumbnail != nil {
		t.Errorf("expected no thumbnail when photo URL is empty, got %+v", embed.Thumbnail)
	}
}

func TestNotifyApproved_MessageFormat(t *testing.T) {
	s := setupTestStore(t)
	srv, getReqs := setupWebhookServer(t)
	n := newTestNotifier(t, s, srv.URL)

	n.NotifyApproved("Alice", "Wash Dishes")

	reqs := waitForRequests(getReqs, 1, 2*time.Second)
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}

	embed := reqs[0].Payload.Embeds[0]
	if embed.Title != "Chore Approved" {
		t.Errorf("expected title 'Chore Approved', got %q", embed.Title)
	}
	if !strings.Contains(embed.Description, "Alice") || !strings.Contains(embed.Description, "Wash Dishes") {
		t.Errorf("description missing user or chore: %q", embed.Description)
	}
	if embed.Color != ColorGreen {
		t.Errorf("expected color %d (green), got %d", ColorGreen, embed.Color)
	}
}

func TestNotifyRejected_MessageFormat(t *testing.T) {
	s := setupTestStore(t)
	srv, getReqs := setupWebhookServer(t)
	n := newTestNotifier(t, s, srv.URL)

	n.NotifyRejected("Charlie", "Mow Lawn")

	reqs := waitForRequests(getReqs, 1, 2*time.Second)
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}

	embed := reqs[0].Payload.Embeds[0]
	if embed.Title != "Chore Rejected" {
		t.Errorf("expected title 'Chore Rejected', got %q", embed.Title)
	}
	if !strings.Contains(embed.Description, "Charlie") || !strings.Contains(embed.Description, "Mow Lawn") {
		t.Errorf("description missing user or chore: %q", embed.Description)
	}
	if embed.Color != ColorRed {
		t.Errorf("expected color %d (red), got %d", ColorRed, embed.Color)
	}
}

func TestNotifyCompleted_MessageFormat(t *testing.T) {
	s := setupTestStore(t)
	srv, getReqs := setupWebhookServer(t)
	n := newTestNotifier(t, s, srv.URL)

	n.NotifyCompleted("Dana", "Vacuum", "https://example.com/proof.png", 10)

	reqs := waitForRequests(getReqs, 1, 2*time.Second)
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}

	embed := reqs[0].Payload.Embeds[0]
	if embed.Title != "Chore Completed" {
		t.Errorf("expected title 'Chore Completed', got %q", embed.Title)
	}
	if !strings.Contains(embed.Description, "Dana") || !strings.Contains(embed.Description, "Vacuum") {
		t.Errorf("description missing user or chore: %q", embed.Description)
	}
	if !strings.Contains(embed.Description, "+10 pts") {
		t.Errorf("description should contain points: %q", embed.Description)
	}
	if embed.Color != ColorGreen {
		t.Errorf("expected color %d (green), got %d", ColorGreen, embed.Color)
	}
	if embed.Thumbnail == nil || embed.Thumbnail.URL != "https://example.com/proof.png" {
		t.Errorf("expected thumbnail URL, got %+v", embed.Thumbnail)
	}
}

func TestNotifyCompleted_ZeroPoints(t *testing.T) {
	s := setupTestStore(t)
	srv, getReqs := setupWebhookServer(t)
	n := newTestNotifier(t, s, srv.URL)

	n.NotifyCompleted("Eve", "Read Book", "", 0)

	reqs := waitForRequests(getReqs, 1, 2*time.Second)
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}

	embed := reqs[0].Payload.Embeds[0]
	if strings.Contains(embed.Description, "pts") {
		t.Errorf("description should not contain points when zero, got %q", embed.Description)
	}
	if embed.Thumbnail != nil {
		t.Errorf("expected no thumbnail when no photo, got %+v", embed.Thumbnail)
	}
}

// --- Tests for HTTP request formation ---

func TestSend_HTTPRequestIsCorrectlyFormed(t *testing.T) {
	s := setupTestStore(t)
	srv, getReqs := setupWebhookServer(t)
	n := newTestNotifier(t, s, srv.URL)

	// Call send directly (synchronous, no goroutine).
	n.send(discordEmbed{
		Title:       "Test Title",
		Description: "Test description",
		Color:       ColorGreen,
	})

	reqs := getReqs()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}

	req := reqs[0]
	if req.Method != http.MethodPost {
		t.Errorf("expected POST method, got %s", req.Method)
	}
	if req.ContentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", req.ContentType)
	}
	if len(req.Payload.Embeds) != 1 {
		t.Fatalf("expected 1 embed in payload, got %d", len(req.Payload.Embeds))
	}
	if req.Payload.Embeds[0].Title != "Test Title" {
		t.Errorf("expected embed title 'Test Title', got %q", req.Payload.Embeds[0].Title)
	}
}

func TestSend_NoOpWhenWebhookURLEmpty(t *testing.T) {
	s := setupTestStore(t)
	// Explicitly set an empty webhook URL.
	if err := s.SetSetting(context.Background(), SettingWebhookURL, ""); err != nil {
		t.Fatalf("failed to set setting: %v", err)
	}

	srv, getReqs := setupWebhookServer(t)
	defer srv.Close()

	n := NewNotifier(s)
	n.send(discordEmbed{Title: "should not arrive"})

	time.Sleep(50 * time.Millisecond)
	reqs := getReqs()
	if len(reqs) != 0 {
		t.Errorf("expected 0 requests when webhook URL is empty string, got %d", len(reqs))
	}
}

func TestSend_HandlesServerError(t *testing.T) {
	s := setupTestStore(t)

	// Server that returns 500.
	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer errSrv.Close()

	n := newTestNotifier(t, s, errSrv.URL)
	// Should not panic on server error.
	n.send(discordEmbed{Title: "error test"})
}

// --- Test multiple notifications in sequence ---

func TestMultipleNotifications(t *testing.T) {
	s := setupTestStore(t)
	srv, getReqs := setupWebhookServer(t)
	n := newTestNotifier(t, s, srv.URL)

	// Fire multiple notifications (they spawn goroutines).
	n.NotifyPendingApproval("Alice", "Chore1", "")
	n.NotifyApproved("Bob", "Chore2")
	n.NotifyRejected("Charlie", "Chore3")
	n.NotifyCompleted("Dana", "Chore4", "", 5)

	reqs := waitForRequests(getReqs, 4, 5*time.Second)
	if len(reqs) != 4 {
		t.Fatalf("expected 4 requests, got %d", len(reqs))
	}

	// Verify each request was a POST with JSON content type.
	for i, req := range reqs {
		if req.Method != http.MethodPost {
			t.Errorf("request %d: expected POST, got %s", i, req.Method)
		}
		if req.ContentType != "application/json" {
			t.Errorf("request %d: expected application/json, got %s", i, req.ContentType)
		}
	}
}
