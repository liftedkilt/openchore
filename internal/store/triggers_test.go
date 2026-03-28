package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

func TestCreateChoreTrigger(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	admin := createTestUser(t, s, "Admin", "admin")
	chore := createTestChore(t, s, "Test Chore", 10, admin.ID)
	child := createTestUser(t, s, "Child", "child")

	dueBy := "17:00"
	availAt := "08:00"
	trigger := &model.ChoreTrigger{
		UUID:               "abcdef1234567890abcdef1234567890",
		ChoreID:            chore.ID,
		DefaultAssignedTo:  &child.ID,
		DefaultDueBy:       &dueBy,
		DefaultAvailableAt: &availAt,
		Enabled:            true,
		CooldownMinutes:    30,
	}

	if err := s.CreateChoreTrigger(ctx, trigger); err != nil {
		t.Fatalf("CreateChoreTrigger: %v", err)
	}
	if trigger.ID == 0 {
		t.Fatal("expected trigger ID to be set")
	}

	// Verify by UUID lookup
	got, err := s.GetChoreTriggerByUUID(ctx, "abcdef1234567890abcdef1234567890")
	if err != nil {
		t.Fatalf("GetChoreTriggerByUUID: %v", err)
	}
	if got == nil {
		t.Fatal("expected trigger, got nil")
	}
	if got.UUID != trigger.UUID {
		t.Errorf("UUID mismatch: got %s, want %s", got.UUID, trigger.UUID)
	}
	if got.ChoreID != chore.ID {
		t.Errorf("ChoreID mismatch: got %d, want %d", got.ChoreID, chore.ID)
	}
	if got.DefaultAssignedTo == nil || *got.DefaultAssignedTo != child.ID {
		t.Errorf("DefaultAssignedTo mismatch")
	}
	if got.DefaultDueBy == nil || *got.DefaultDueBy != "17:00" {
		t.Errorf("DefaultDueBy mismatch")
	}
	if got.DefaultAvailableAt == nil || *got.DefaultAvailableAt != "08:00" {
		t.Errorf("DefaultAvailableAt mismatch")
	}
	if !got.Enabled {
		t.Error("expected enabled=true")
	}
	if got.CooldownMinutes != 30 {
		t.Errorf("CooldownMinutes: got %d, want 30", got.CooldownMinutes)
	}
	if got.LastTriggeredAt != nil {
		t.Error("expected LastTriggeredAt to be nil")
	}
}

func TestGetChoreTriggerByUUID_NotFound(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	got, err := s.GetChoreTriggerByUUID(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for non-existent UUID")
	}
}

func TestListChoreTriggersForChore(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	admin := createTestUser(t, s, "Admin", "admin")
	chore := createTestChore(t, s, "Test Chore", 10, admin.ID)

	// Empty list
	triggers, err := s.ListChoreTriggersForChore(ctx, chore.ID)
	if err != nil {
		t.Fatalf("ListChoreTriggersForChore: %v", err)
	}
	if len(triggers) != 0 {
		t.Fatalf("expected 0 triggers, got %d", len(triggers))
	}

	// Add two triggers
	t1 := &model.ChoreTrigger{UUID: "uuid1111111111111111111111111111", ChoreID: chore.ID, Enabled: true}
	t2 := &model.ChoreTrigger{UUID: "uuid2222222222222222222222222222", ChoreID: chore.ID, Enabled: false}
	if err := s.CreateChoreTrigger(ctx, t1); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateChoreTrigger(ctx, t2); err != nil {
		t.Fatal(err)
	}

	triggers, err = s.ListChoreTriggersForChore(ctx, chore.ID)
	if err != nil {
		t.Fatalf("ListChoreTriggersForChore: %v", err)
	}
	if len(triggers) != 2 {
		t.Fatalf("expected 2 triggers, got %d", len(triggers))
	}
	if triggers[0].UUID != t1.UUID {
		t.Errorf("first trigger UUID: got %s, want %s", triggers[0].UUID, t1.UUID)
	}
}

func TestUpdateChoreTrigger(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	admin := createTestUser(t, s, "Admin", "admin")
	chore := createTestChore(t, s, "Test Chore", 10, admin.ID)
	child := createTestUser(t, s, "Child", "child")

	trigger := &model.ChoreTrigger{UUID: "update_test_uuid_1234567890abcdef", ChoreID: chore.ID, Enabled: true}
	if err := s.CreateChoreTrigger(ctx, trigger); err != nil {
		t.Fatal(err)
	}

	dueBy := "19:00"
	trigger.DefaultAssignedTo = &child.ID
	trigger.DefaultDueBy = &dueBy
	trigger.Enabled = false
	trigger.CooldownMinutes = 60

	if err := s.UpdateChoreTrigger(ctx, trigger); err != nil {
		t.Fatalf("UpdateChoreTrigger: %v", err)
	}

	got, err := s.GetChoreTriggerByUUID(ctx, trigger.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Enabled {
		t.Error("expected enabled=false after update")
	}
	if got.CooldownMinutes != 60 {
		t.Errorf("CooldownMinutes: got %d, want 60", got.CooldownMinutes)
	}
	if got.DefaultAssignedTo == nil || *got.DefaultAssignedTo != child.ID {
		t.Error("DefaultAssignedTo not updated")
	}
}

func TestDeleteChoreTrigger(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	admin := createTestUser(t, s, "Admin", "admin")
	chore := createTestChore(t, s, "Test Chore", 10, admin.ID)

	trigger := &model.ChoreTrigger{UUID: "delete_test_uuid_1234567890abcdef", ChoreID: chore.ID, Enabled: true}
	if err := s.CreateChoreTrigger(ctx, trigger); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteChoreTrigger(ctx, trigger.ID); err != nil {
		t.Fatalf("DeleteChoreTrigger: %v", err)
	}

	got, err := s.GetChoreTriggerByUUID(ctx, trigger.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Error("expected trigger to be deleted")
	}
}

func TestUpdateTriggerLastFired(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	admin := createTestUser(t, s, "Admin", "admin")
	chore := createTestChore(t, s, "Test Chore", 10, admin.ID)

	trigger := &model.ChoreTrigger{UUID: "lastfire_test_uuid_1234567890abcd", ChoreID: chore.ID, Enabled: true}
	if err := s.CreateChoreTrigger(ctx, trigger); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	if err := s.UpdateTriggerLastFired(ctx, trigger.ID, now); err != nil {
		t.Fatalf("UpdateTriggerLastFired: %v", err)
	}

	got, err := s.GetChoreTriggerByUUID(ctx, trigger.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastTriggeredAt == nil {
		t.Fatal("expected LastTriggeredAt to be set")
	}
}

func TestGetUserByName(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	createTestUser(t, s, "Alice", "child")

	// Exact match
	u, err := s.GetUserByName(ctx, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	if u == nil || u.Name != "Alice" {
		t.Error("expected to find Alice")
	}

	// Case-insensitive
	u, err = s.GetUserByName(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if u == nil || u.Name != "Alice" {
		t.Error("expected case-insensitive match for alice")
	}

	// Not found
	u, err = s.GetUserByName(ctx, "Nobody")
	if err != nil {
		t.Fatal(err)
	}
	if u != nil {
		t.Error("expected nil for non-existent user")
	}
}
