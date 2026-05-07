package store_test

import (
	"context"
	"database/sql"
	"strings"
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

func setupStore(t *testing.T) *store.Store {
	t.Helper()
	s, _ := setupStoreWithDB(t)
	return s
}

// setupStoreWithDB returns both the Store and the underlying *sql.DB so tests
// can perform direct SQL manipulation (e.g. backdating timestamps) that isn't
// exposed through the Store API.
func setupStoreWithDB(t *testing.T) (*store.Store, *sql.DB) {
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
	return store.New(db), db
}

func createTestUser(t *testing.T, s *store.Store, name, role string) *model.User {
	t.Helper()
	u := &model.User{Name: name, Role: role}
	if err := s.CreateUser(context.Background(), u); err != nil {
		t.Fatalf("CreateUser(%s): %v", name, err)
	}
	return u
}

func createTestChore(t *testing.T, s *store.Store, title string, points int, createdBy int64) *model.Chore {
	t.Helper()
	c := &model.Chore{
		Title:       title,
		Description: "desc",
		Category:    "required",
		PointsValue: points,
		Source:      "manual",
		CreatedBy:   createdBy,
	}
	if err := s.CreateChore(context.Background(), c); err != nil {
		t.Fatalf("CreateChore(%s): %v", title, err)
	}
	return c
}

func createTestSchedule(t *testing.T, s *store.Store, choreID, assignedTo int64, dow int) *model.ChoreSchedule {
	t.Helper()
	d := dow
	cs := &model.ChoreSchedule{
		ChoreID:          choreID,
		AssignedTo:       assignedTo,
		AssignmentType:   "individual",
		DayOfWeek:        &d,
		PointsMultiplier: 1.0,
		ExpiryPenalty:    "none",
	}
	if err := s.CreateSchedule(context.Background(), cs); err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}
	return cs
}

// ===== User CRUD =====

func TestCreateUser(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := &model.User{Name: "Alice", Role: "child", AvatarURL: "/img/alice.png"}
	err := s.CreateUser(ctx, u)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.ID == 0 {
		t.Fatal("expected non-zero ID after CreateUser")
	}
}

func TestGetUser(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Bob", "admin")

	got, err := s.GetUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got == nil {
		t.Fatal("expected user, got nil")
	}
	if got.Name != "Bob" || got.Role != "admin" {
		t.Errorf("got Name=%q Role=%q, want Bob/admin", got.Name, got.Role)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	s := setupStore(t)
	got, err := s.GetUser(context.Background(), 9999)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for non-existent user, got %+v", got)
	}
}

func TestListUsers(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	createTestUser(t, s, "Charlie", "child")
	createTestUser(t, s, "Alice", "admin")

	users, err := s.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	// Ordered by name
	if users[0].Name != "Alice" || users[1].Name != "Charlie" {
		t.Errorf("users not in expected order: %v, %v", users[0].Name, users[1].Name)
	}
}

func TestUpdateUser(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Dave", "child")
	u.Name = "David"
	u.Role = "admin"
	if err := s.UpdateUser(ctx, u); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	got, _ := s.GetUser(ctx, u.ID)
	if got.Name != "David" || got.Role != "admin" {
		t.Errorf("got Name=%q Role=%q, want David/admin", got.Name, got.Role)
	}
}

func TestDeleteUser(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Eve", "child")
	if err := s.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	got, _ := s.GetUser(ctx, u.ID)
	if got != nil {
		t.Errorf("expected nil after delete, got %+v", got)
	}
}

// ===== Chore CRUD =====

func TestCreateChore(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Parent", "admin")
	c := &model.Chore{
		Title:            "Wash dishes",
		Description:      "Clean all dishes",
		Category:         "required",
		PointsValue:      10,
		RequiresApproval: true,
		RequiresPhoto:    true,
		Source:           "manual",
		CreatedBy:        u.ID,
	}
	err := s.CreateChore(ctx, c)
	if err != nil {
		t.Fatalf("CreateChore: %v", err)
	}
	if c.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
}

func TestGetChore(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Parent", "admin")
	c := &model.Chore{
		Title:            "Vacuum",
		Description:      "Vacuum the house",
		Category:         "core",
		PointsValue:      15,
		RequiresApproval: true,
		RequiresPhoto:    false,
		Source:           "manual",
		CreatedBy:        u.ID,
	}
	s.CreateChore(ctx, c)

	got, err := s.GetChore(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetChore: %v", err)
	}
	if got == nil {
		t.Fatal("expected chore, got nil")
	}
	if got.Title != "Vacuum" || got.PointsValue != 15 {
		t.Errorf("unexpected chore: %+v", got)
	}
	if !got.RequiresApproval {
		t.Error("expected RequiresApproval=true")
	}
	if got.RequiresPhoto {
		t.Error("expected RequiresPhoto=false")
	}
}

func TestGetChore_NotFound(t *testing.T) {
	s := setupStore(t)
	got, err := s.GetChore(context.Background(), 9999)
	if err != nil {
		t.Fatalf("GetChore: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestListChores(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Parent", "admin")
	createTestChore(t, s, "Zebra chore", 5, u.ID)
	createTestChore(t, s, "Apple chore", 10, u.ID)

	chores, err := s.ListChores(ctx)
	if err != nil {
		t.Fatalf("ListChores: %v", err)
	}
	if len(chores) != 2 {
		t.Fatalf("expected 2 chores, got %d", len(chores))
	}
	// Ordered by title
	if chores[0].Title != "Apple chore" {
		t.Errorf("expected first chore 'Apple chore', got %q", chores[0].Title)
	}
}

func TestUpdateChore(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Parent", "admin")
	c := createTestChore(t, s, "Old Title", 5, u.ID)
	c.Title = "New Title"
	c.PointsValue = 20
	if err := s.UpdateChore(ctx, c); err != nil {
		t.Fatalf("UpdateChore: %v", err)
	}

	got, _ := s.GetChore(ctx, c.ID)
	if got.Title != "New Title" || got.PointsValue != 20 {
		t.Errorf("update not reflected: %+v", got)
	}
}

func TestDeleteChore(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Parent", "admin")
	c := createTestChore(t, s, "To Delete", 5, u.ID)
	if err := s.DeleteChore(ctx, c.ID); err != nil {
		t.Fatalf("DeleteChore: %v", err)
	}

	got, _ := s.GetChore(ctx, c.ID)
	if got != nil {
		t.Errorf("expected nil after delete, got %+v", got)
	}
}

// ===== Schedule CRUD =====

func TestCreateSchedule(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Feed cat", 5, u.ID)

	dow := 1
	cs := &model.ChoreSchedule{
		ChoreID:          c.ID,
		AssignedTo:       u.ID,
		AssignmentType:   "individual",
		DayOfWeek:        &dow,
		PointsMultiplier: 1.5,
		ExpiryPenalty:    "none",
	}
	err := s.CreateSchedule(ctx, cs)
	if err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}
	if cs.ID == 0 {
		t.Fatal("expected non-zero schedule ID")
	}
}

func TestGetSchedule(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Walk dog", 10, u.ID)
	cs := createTestSchedule(t, s, c.ID, u.ID, 3)

	got, err := s.GetSchedule(ctx, cs.ID)
	if err != nil {
		t.Fatalf("GetSchedule: %v", err)
	}
	if got == nil {
		t.Fatal("expected schedule, got nil")
	}
	if got.ChoreID != c.ID || got.AssignedTo != u.ID {
		t.Errorf("unexpected schedule: %+v", got)
	}
}

func TestGetSchedule_NotFound(t *testing.T) {
	s := setupStore(t)
	got, err := s.GetSchedule(context.Background(), 9999)
	if err != nil {
		t.Fatalf("GetSchedule: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestListSchedulesForChore(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u1 := createTestUser(t, s, "Child1", "child")
	u2 := createTestUser(t, s, "Child2", "child")
	c := createTestChore(t, s, "Sweep", 5, u1.ID)

	createTestSchedule(t, s, c.ID, u1.ID, 1)
	createTestSchedule(t, s, c.ID, u2.ID, 2)

	schedules, err := s.ListSchedulesForChore(ctx, c.ID)
	if err != nil {
		t.Fatalf("ListSchedulesForChore: %v", err)
	}
	if len(schedules) != 2 {
		t.Fatalf("expected 2 schedules, got %d", len(schedules))
	}
}

func TestDeleteSchedule(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Mop", 5, u.ID)
	cs := createTestSchedule(t, s, c.ID, u.ID, 4)

	if err := s.DeleteSchedule(ctx, cs.ID); err != nil {
		t.Fatalf("DeleteSchedule: %v", err)
	}

	got, _ := s.GetSchedule(ctx, cs.ID)
	if got != nil {
		t.Errorf("expected nil after delete, got %+v", got)
	}
}

// ===== Chore Completions =====

func TestCompleteAndGetCompletion(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Laundry", 10, u.ID)
	cs := createTestSchedule(t, s, c.ID, u.ID, 1)

	cc := &model.ChoreCompletion{
		ChoreScheduleID: cs.ID,
		CompletedBy:     u.ID,
		Status:          "approved",
		CompletionDate:  "2026-03-28",
	}
	err := s.CompleteChore(ctx, cc)
	if err != nil {
		t.Fatalf("CompleteChore: %v", err)
	}
	if cc.ID == 0 {
		t.Fatal("expected non-zero completion ID")
	}

	// GetCompletion by ID
	got, err := s.GetCompletion(ctx, cc.ID)
	if err != nil {
		t.Fatalf("GetCompletion: %v", err)
	}
	if got == nil {
		t.Fatal("expected completion, got nil")
	}
	if got.Status != "approved" {
		t.Errorf("unexpected status: %q", got.Status)
	}
}

func TestGetCompletionForScheduleDate(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Dishes", 5, u.ID)
	cs := createTestSchedule(t, s, c.ID, u.ID, 2)

	cc := &model.ChoreCompletion{
		ChoreScheduleID: cs.ID,
		CompletedBy:     u.ID,
		Status:          "approved",
		CompletionDate:  "2026-03-28",
	}
	s.CompleteChore(ctx, cc)

	got, err := s.GetCompletionForScheduleDate(ctx, cs.ID, "2026-03-28")
	if err != nil {
		t.Fatalf("GetCompletionForScheduleDate: %v", err)
	}
	if got == nil {
		t.Fatal("expected completion")
	}
	if got.ID != cc.ID {
		t.Errorf("expected ID %d, got %d", cc.ID, got.ID)
	}

	// Not found for different date
	got2, _ := s.GetCompletionForScheduleDate(ctx, cs.ID, "2026-03-29")
	if got2 != nil {
		t.Errorf("expected nil for wrong date, got %+v", got2)
	}
}

func TestUncompleteChore(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Trash", 5, u.ID)
	cs := createTestSchedule(t, s, c.ID, u.ID, 3)

	cc := &model.ChoreCompletion{
		ChoreScheduleID: cs.ID,
		CompletedBy:     u.ID,
		Status:          "approved",
		CompletionDate:  "2026-03-28",
	}
	s.CompleteChore(ctx, cc)

	err := s.UncompleteChore(ctx, cs.ID, "2026-03-28")
	if err != nil {
		t.Fatalf("UncompleteChore: %v", err)
	}

	// Approved completions are SOFT-deleted (preserves photo/AI metadata
	// so the kid can re-check without losing the approval for the day).
	// The row should still be present with uncompleted_at set.
	got, _ := s.GetCompletionForScheduleDate(ctx, cs.ID, "2026-03-28")
	if got == nil {
		t.Fatalf("expected row to survive as soft-deleted, got nil")
	}
	if got.UncompletedAt == nil {
		t.Errorf("expected uncompleted_at to be set after uncomplete, got nil")
	}

	// Revive: completion should come back with uncompleted_at cleared.
	if err := s.ReviveCompletion(ctx, got.ID); err != nil {
		t.Fatalf("ReviveCompletion: %v", err)
	}
	revived, _ := s.GetCompletionForScheduleDate(ctx, cs.ID, "2026-03-28")
	if revived == nil || revived.UncompletedAt != nil {
		t.Errorf("expected row revived with uncompleted_at=nil, got %+v", revived)
	}
}

func TestUncompleteChore_AIRejectedHardDeletes(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Trash", 5, u.ID)
	cs := createTestSchedule(t, s, c.ID, u.ID, 3)

	cc := &model.ChoreCompletion{
		ChoreScheduleID: cs.ID,
		CompletedBy:     u.ID,
		Status:          model.StatusAIRejected,
		CompletionDate:  "2026-03-28",
	}
	s.CompleteChore(ctx, cc)

	if err := s.UncompleteChore(ctx, cs.ID, "2026-03-28"); err != nil {
		t.Fatalf("UncompleteChore: %v", err)
	}

	// ai_rejected rows should be hard-deleted so the retry flow works.
	got, _ := s.GetCompletionForScheduleDate(ctx, cs.ID, "2026-03-28")
	if got != nil {
		t.Errorf("expected ai_rejected row to be hard-deleted, got %+v", got)
	}
}

func TestUpdateCompletionStatus(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	child := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Room", 10, parent.ID)
	cs := createTestSchedule(t, s, c.ID, child.ID, 1)

	cc := &model.ChoreCompletion{
		ChoreScheduleID: cs.ID,
		CompletedBy:     child.ID,
		Status:          "pending",
		CompletionDate:  "2026-03-28",
	}
	s.CompleteChore(ctx, cc)

	// Approve
	err := s.UpdateCompletionStatus(ctx, cc.ID, "approved", parent.ID)
	if err != nil {
		t.Fatalf("UpdateCompletionStatus: %v", err)
	}

	got, _ := s.GetCompletion(ctx, cc.ID)
	if got.Status != "approved" {
		t.Errorf("expected status=approved, got %q", got.Status)
	}
	if got.ApprovedBy == nil || *got.ApprovedBy != parent.ID {
		t.Errorf("expected ApprovedBy=%d", parent.ID)
	}

	// Reject (non-approved status does not set approved_by)
	cc2 := &model.ChoreCompletion{
		ChoreScheduleID: cs.ID,
		CompletedBy:     child.ID,
		Status:          "pending",
		CompletionDate:  "2026-03-29",
	}
	s.CompleteChore(ctx, cc2)
	s.UpdateCompletionStatus(ctx, cc2.ID, "rejected", parent.ID)

	got2, _ := s.GetCompletion(ctx, cc2.ID)
	if got2.Status != "rejected" {
		t.Errorf("expected status=rejected, got %q", got2.Status)
	}
}

// ===== Points & Transactions =====

func TestGetPointBalance_Empty(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	balance, err := s.GetPointBalance(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetPointBalance: %v", err)
	}
	if balance != 0 {
		t.Errorf("expected 0 balance, got %d", balance)
	}
}

func TestCreditAndDebitChorePoints(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Sweep", 10, u.ID)
	cs := createTestSchedule(t, s, c.ID, u.ID, 1)

	cc := &model.ChoreCompletion{
		ChoreScheduleID: cs.ID,
		CompletedBy:     u.ID,
		Status:          "approved",
		CompletionDate:  "2026-03-28",
	}
	s.CompleteChore(ctx, cc)

	// Credit
	err := s.CreditChorePoints(ctx, u.ID, cc.ID, 10)
	if err != nil {
		t.Fatalf("CreditChorePoints: %v", err)
	}
	balance, _ := s.GetPointBalance(ctx, u.ID)
	if balance != 10 {
		t.Errorf("expected balance=10, got %d", balance)
	}

	// Debit
	err = s.DebitChorePoints(ctx, u.ID, cc.ID, 10)
	if err != nil {
		t.Fatalf("DebitChorePoints: %v", err)
	}
	balance, _ = s.GetPointBalance(ctx, u.ID)
	if balance != 0 {
		t.Errorf("expected balance=0 after debit, got %d", balance)
	}
}

func TestAdminAdjustPoints(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")

	err := s.AdminAdjustPoints(ctx, u.ID, 50, "bonus for being awesome")
	if err != nil {
		t.Fatalf("AdminAdjustPoints: %v", err)
	}
	balance, _ := s.GetPointBalance(ctx, u.ID)
	if balance != 50 {
		t.Errorf("expected balance=50, got %d", balance)
	}

	// Negative adjustment
	s.AdminAdjustPoints(ctx, u.ID, -20, "penalty")
	balance, _ = s.GetPointBalance(ctx, u.ID)
	if balance != 30 {
		t.Errorf("expected balance=30, got %d", balance)
	}
}

func TestListPointTransactions(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	s.AdminAdjustPoints(ctx, u.ID, 10, "first")
	s.AdminAdjustPoints(ctx, u.ID, 20, "second")
	s.AdminAdjustPoints(ctx, u.ID, 30, "third")

	txs, err := s.ListPointTransactions(ctx, u.ID, 2)
	if err != nil {
		t.Fatalf("ListPointTransactions: %v", err)
	}
	if len(txs) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(txs))
	}
	// Most recent first
	if txs[0].Amount != 30 {
		t.Errorf("expected first tx amount=30, got %d", txs[0].Amount)
	}
}

func TestGetAllPointBalances(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u1 := createTestUser(t, s, "Alice", "child")
	u2 := createTestUser(t, s, "Bob", "child")
	s.AdminAdjustPoints(ctx, u1.ID, 100, "")
	s.AdminAdjustPoints(ctx, u2.ID, 50, "")

	balances, err := s.GetAllPointBalances(ctx)
	if err != nil {
		t.Fatalf("GetAllPointBalances: %v", err)
	}
	if len(balances) != 2 {
		t.Fatalf("expected 2 balances, got %d", len(balances))
	}
}

func TestGetChorePointsForSchedule(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Clean", 20, u.ID)

	dow := 1
	cs := &model.ChoreSchedule{
		ChoreID:          c.ID,
		AssignedTo:       u.ID,
		AssignmentType:   "individual",
		DayOfWeek:        &dow,
		PointsMultiplier: 2.0,
		ExpiryPenalty:    "none",
	}
	s.CreateSchedule(ctx, cs)

	pts, err := s.GetChorePointsForSchedule(ctx, cs.ID)
	if err != nil {
		t.Fatalf("GetChorePointsForSchedule: %v", err)
	}
	if pts != 40 {
		t.Errorf("expected 40 (20*2.0), got %d", pts)
	}
}

// ===== Rewards =====

func TestCreateAndGetReward(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Parent", "admin")
	r := &model.Reward{
		Name:        "Ice Cream",
		Description: "One scoop",
		Icon:        "icecream",
		Cost:        50,
		Active:      true,
		CreatedBy:   u.ID,
	}
	err := s.CreateReward(ctx, r)
	if err != nil {
		t.Fatalf("CreateReward: %v", err)
	}
	if r.ID == 0 {
		t.Fatal("expected non-zero reward ID")
	}

	got, err := s.GetReward(ctx, r.ID)
	if err != nil {
		t.Fatalf("GetReward: %v", err)
	}
	if got.Name != "Ice Cream" || got.Cost != 50 || !got.Active {
		t.Errorf("unexpected reward: %+v", got)
	}
}

func TestGetReward_NotFound(t *testing.T) {
	s := setupStore(t)
	got, err := s.GetReward(context.Background(), 9999)
	if err != nil {
		t.Fatalf("GetReward: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestListRewards(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Parent", "admin")

	r1 := &model.Reward{Name: "Expensive", Cost: 100, Active: true, CreatedBy: u.ID}
	r2 := &model.Reward{Name: "Cheap", Cost: 10, Active: true, CreatedBy: u.ID}
	r3 := &model.Reward{Name: "Inactive", Cost: 50, Active: false, CreatedBy: u.ID}
	s.CreateReward(ctx, r1)
	s.CreateReward(ctx, r2)
	s.CreateReward(ctx, r3)

	// All rewards
	all, err := s.ListRewards(ctx, false)
	if err != nil {
		t.Fatalf("ListRewards(false): %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 rewards, got %d", len(all))
	}

	// Active only
	active, err := s.ListRewards(ctx, true)
	if err != nil {
		t.Fatalf("ListRewards(true): %v", err)
	}
	if len(active) != 2 {
		t.Errorf("expected 2 active rewards, got %d", len(active))
	}
	// Ordered by cost
	if active[0].Name != "Cheap" {
		t.Errorf("expected first reward 'Cheap', got %q", active[0].Name)
	}
}

func TestUpdateReward(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Parent", "admin")
	r := &model.Reward{Name: "Old", Cost: 10, Active: true, CreatedBy: u.ID}
	s.CreateReward(ctx, r)

	r.Name = "Updated"
	r.Cost = 99
	r.Active = false
	err := s.UpdateReward(ctx, r)
	if err != nil {
		t.Fatalf("UpdateReward: %v", err)
	}

	got, _ := s.GetReward(ctx, r.ID)
	if got.Name != "Updated" || got.Cost != 99 || got.Active {
		t.Errorf("update not reflected: %+v", got)
	}
}

func TestDeleteReward(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Parent", "admin")
	r := &model.Reward{Name: "ToDelete", Cost: 10, Active: true, CreatedBy: u.ID}
	s.CreateReward(ctx, r)

	err := s.DeleteReward(ctx, r.ID)
	if err != nil {
		t.Fatalf("DeleteReward: %v", err)
	}
	got, _ := s.GetReward(ctx, r.ID)
	if got != nil {
		t.Errorf("expected nil after delete, got %+v", got)
	}
}

func TestRedeemReward(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	child := createTestUser(t, s, "Child", "child")

	// Give child some points
	s.AdminAdjustPoints(ctx, child.ID, 100, "starting balance")

	r := &model.Reward{Name: "Movie Night", Cost: 30, Active: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, r)

	redemption, err := s.RedeemReward(ctx, child.ID, r.ID)
	if err != nil {
		t.Fatalf("RedeemReward: %v", err)
	}
	if redemption.PointsSpent != 30 {
		t.Errorf("expected 30 points spent, got %d", redemption.PointsSpent)
	}

	balance, _ := s.GetPointBalance(ctx, child.ID)
	if balance != 70 {
		t.Errorf("expected balance=70, got %d", balance)
	}
}

func TestRedeemReward_InsufficientPoints(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	child := createTestUser(t, s, "Child", "child")
	s.AdminAdjustPoints(ctx, child.ID, 5, "small balance")

	r := &model.Reward{Name: "Expensive", Cost: 100, Active: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, r)

	_, err := s.RedeemReward(ctx, child.ID, r.ID)
	if err == nil {
		t.Fatal("expected error for insufficient points")
	}
}

func TestRedeemReward_InactiveReward(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	child := createTestUser(t, s, "Child", "child")
	s.AdminAdjustPoints(ctx, child.ID, 100, "")

	r := &model.Reward{Name: "Inactive", Cost: 10, Active: false, CreatedBy: parent.ID}
	s.CreateReward(ctx, r)

	_, err := s.RedeemReward(ctx, child.ID, r.ID)
	if err == nil {
		t.Fatal("expected error for inactive reward")
	}
}

func TestRedeemReward_OutOfStock(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	child := createTestUser(t, s, "Child", "child")
	s.AdminAdjustPoints(ctx, child.ID, 100, "")

	stock := 0
	r := &model.Reward{Name: "Sold Out", Cost: 10, Stock: &stock, Active: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, r)

	_, err := s.RedeemReward(ctx, child.ID, r.ID)
	if err == nil {
		t.Fatal("expected error for out of stock reward")
	}
}

func TestRedeemReward_DecrementsStock(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	child := createTestUser(t, s, "Child", "child")
	s.AdminAdjustPoints(ctx, child.ID, 100, "")

	stock := 3
	r := &model.Reward{Name: "Limited", Cost: 10, Stock: &stock, Active: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, r)

	s.RedeemReward(ctx, child.ID, r.ID)

	got, _ := s.GetReward(ctx, r.ID)
	if got.Stock == nil || *got.Stock != 2 {
		t.Errorf("expected stock=2 after redemption, got %v", got.Stock)
	}
}

func TestUndoRedemption(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	child := createTestUser(t, s, "Child", "child")
	s.AdminAdjustPoints(ctx, child.ID, 100, "")

	stock := 5
	r := &model.Reward{Name: "Undoable", Cost: 25, Stock: &stock, Active: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, r)

	redemption, _ := s.RedeemReward(ctx, child.ID, r.ID)

	err := s.UndoRedemption(ctx, redemption.ID)
	if err != nil {
		t.Fatalf("UndoRedemption: %v", err)
	}

	// Points restored
	balance, _ := s.GetPointBalance(ctx, child.ID)
	if balance != 100 {
		t.Errorf("expected balance=100 after undo, got %d", balance)
	}

	// Stock restored
	got, _ := s.GetReward(ctx, r.ID)
	if got.Stock == nil || *got.Stock != 5 {
		t.Errorf("expected stock=5 after undo, got %v", got.Stock)
	}
}

// ===== Streaks =====

func TestGetUserStreak_Empty(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	st, err := s.GetUserStreak(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetUserStreak: %v", err)
	}
	if st.CurrentStreak != 0 || st.LongestStreak != 0 {
		t.Errorf("expected zero streaks, got current=%d longest=%d", st.CurrentStreak, st.LongestStreak)
	}
}

func TestRecalculateStreak(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	child := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Daily task", 10, child.ID)

	// Saturday 2026-03-28 = day_of_week 6
	// Create schedule for Saturday (day 6) - today
	csSat := createTestSchedule(t, s, c.ID, child.ID, 6)

	// Complete today's chore
	ccSat := &model.ChoreCompletion{
		ChoreScheduleID: csSat.ID,
		CompletedBy:     child.ID,
		Status:          "approved",
		CompletionDate:  "2026-03-28",
	}
	s.CompleteChore(ctx, ccSat)

	err := s.RecalculateStreak(ctx, child.ID, "2026-03-28")
	if err != nil {
		t.Fatalf("RecalculateStreak: %v", err)
	}

	st, _ := s.GetUserStreak(ctx, child.ID)
	// Today fully completed = streak of 1
	if st.CurrentStreak != 1 {
		t.Errorf("expected streak=1, got %d", st.CurrentStreak)
	}
	if st.LongestStreak != 1 {
		t.Errorf("expected longest_streak=1, got %d", st.LongestStreak)
	}
}

func TestRecalculateStreak_NoChores(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	child := createTestUser(t, s, "Child", "child")

	// No schedules at all - should result in 0 streak
	err := s.RecalculateStreak(ctx, child.ID, "2026-03-28")
	if err != nil {
		t.Fatalf("RecalculateStreak: %v", err)
	}

	st, _ := s.GetUserStreak(ctx, child.ID)
	if st.CurrentStreak != 0 {
		t.Errorf("expected streak=0 with no chores, got %d", st.CurrentStreak)
	}
}

// ===== Settings =====

func TestGetSetting_Empty(t *testing.T) {
	s := setupStore(t)
	val, err := s.GetSetting(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty string for nonexistent key, got %q", val)
	}
}

func TestSetAndGetSetting(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	err := s.SetSetting(ctx, "theme", "dark")
	if err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	val, err := s.GetSetting(ctx, "theme")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if val != "dark" {
		t.Errorf("expected 'dark', got %q", val)
	}

	// Update existing setting
	s.SetSetting(ctx, "theme", "light")
	val, _ = s.GetSetting(ctx, "theme")
	if val != "light" {
		t.Errorf("expected 'light' after update, got %q", val)
	}
}

// ===== Webhooks =====

func TestCreateAndListWebhooks(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	w := &model.Webhook{
		URL:    "https://example.com/hook",
		Secret: "mysecret",
		Events: "*",
		Active: true,
	}
	err := s.CreateWebhook(ctx, w)
	if err != nil {
		t.Fatalf("CreateWebhook: %v", err)
	}
	if w.ID == 0 {
		t.Fatal("expected non-zero webhook ID")
	}

	webhooks, err := s.ListWebhooks(ctx)
	if err != nil {
		t.Fatalf("ListWebhooks: %v", err)
	}
	if len(webhooks) != 1 {
		t.Fatalf("expected 1 webhook, got %d", len(webhooks))
	}
	if webhooks[0].URL != "https://example.com/hook" || !webhooks[0].Active {
		t.Errorf("unexpected webhook: %+v", webhooks[0])
	}
}

func TestUpdateWebhook(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	w := &model.Webhook{URL: "https://old.com", Events: "*", Active: true}
	s.CreateWebhook(ctx, w)

	w.URL = "https://new.com"
	w.Active = false
	w.Events = "chore.complete"
	err := s.UpdateWebhook(ctx, w)
	if err != nil {
		t.Fatalf("UpdateWebhook: %v", err)
	}

	webhooks, _ := s.ListWebhooks(ctx)
	if webhooks[0].URL != "https://new.com" || webhooks[0].Active {
		t.Errorf("update not reflected: %+v", webhooks[0])
	}
}

func TestDeleteWebhook(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	w := &model.Webhook{URL: "https://delete.me", Events: "*", Active: true}
	s.CreateWebhook(ctx, w)

	err := s.DeleteWebhook(ctx, w.ID)
	if err != nil {
		t.Fatalf("DeleteWebhook: %v", err)
	}

	webhooks, _ := s.ListWebhooks(ctx)
	if len(webhooks) != 0 {
		t.Errorf("expected 0 webhooks after delete, got %d", len(webhooks))
	}
}

func TestGetActiveWebhooksForEvent(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	// Wildcard webhook
	w1 := &model.Webhook{URL: "https://all.com", Events: "*", Active: true}
	s.CreateWebhook(ctx, w1)

	// Specific event webhook
	w2 := &model.Webhook{URL: "https://specific.com", Events: "chore.complete,reward.redeem", Active: true}
	s.CreateWebhook(ctx, w2)

	// Inactive webhook
	w3 := &model.Webhook{URL: "https://inactive.com", Events: "*", Active: false}
	s.CreateWebhook(ctx, w3)

	hooks, err := s.GetActiveWebhooksForEvent(ctx, "chore.complete")
	if err != nil {
		t.Fatalf("GetActiveWebhooksForEvent: %v", err)
	}
	if len(hooks) != 2 {
		t.Errorf("expected 2 active webhooks for chore.complete, got %d", len(hooks))
	}

	hooks2, _ := s.GetActiveWebhooksForEvent(ctx, "user.created")
	// Only wildcard matches
	if len(hooks2) != 1 {
		t.Errorf("expected 1 active webhook for user.created, got %d", len(hooks2))
	}
}

// ===== Streak Rewards =====

func TestStreakRewards(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	sr := &model.StreakReward{StreakDays: 7, BonusPoints: 50, Label: "Weekly Streak"}
	err := s.CreateStreakReward(ctx, sr)
	if err != nil {
		t.Fatalf("CreateStreakReward: %v", err)
	}
	if sr.ID == 0 {
		t.Fatal("expected non-zero streak reward ID")
	}

	rewards, err := s.ListStreakRewards(ctx)
	if err != nil {
		t.Fatalf("ListStreakRewards: %v", err)
	}
	if len(rewards) != 1 || rewards[0].StreakDays != 7 {
		t.Errorf("unexpected streak rewards: %+v", rewards)
	}

	err = s.DeleteStreakReward(ctx, sr.ID)
	if err != nil {
		t.Fatalf("DeleteStreakReward: %v", err)
	}
	rewards, _ = s.ListStreakRewards(ctx)
	if len(rewards) != 0 {
		t.Errorf("expected 0 streak rewards after delete, got %d", len(rewards))
	}
}

// ===== Decay Config =====

func TestDecayConfig(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")

	// Default config
	cfg, err := s.GetUserDecayConfig(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetUserDecayConfig: %v", err)
	}
	if cfg.Enabled || cfg.DecayRate != 5 || cfg.DecayIntervalHours != 24 {
		t.Errorf("unexpected default config: %+v", cfg)
	}

	// Set config
	cfg.Enabled = true
	cfg.DecayRate = 10
	cfg.DecayIntervalHours = 12
	err = s.SetUserDecayConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("SetUserDecayConfig: %v", err)
	}

	got, _ := s.GetUserDecayConfig(ctx, u.ID)
	if !got.Enabled || got.DecayRate != 10 || got.DecayIntervalHours != 12 {
		t.Errorf("config not saved properly: %+v", got)
	}
}

func TestListDecayConfigsEnabled(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u1 := createTestUser(t, s, "Child1", "child")
	u2 := createTestUser(t, s, "Child2", "child")

	s.SetUserDecayConfig(ctx, &model.UserDecayConfig{UserID: u1.ID, Enabled: true, DecayRate: 5, DecayIntervalHours: 24})
	s.SetUserDecayConfig(ctx, &model.UserDecayConfig{UserID: u2.ID, Enabled: false, DecayRate: 5, DecayIntervalHours: 24})

	configs, err := s.ListDecayConfigsEnabled(ctx)
	if err != nil {
		t.Fatalf("ListDecayConfigsEnabled: %v", err)
	}
	if len(configs) != 1 {
		t.Errorf("expected 1 enabled config, got %d", len(configs))
	}
}

func TestUpdateLastDecayAt(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	s.SetUserDecayConfig(ctx, &model.UserDecayConfig{UserID: u.ID, Enabled: true, DecayRate: 5, DecayIntervalHours: 24})

	now := time.Now().UTC().Truncate(time.Second)
	err := s.UpdateLastDecayAt(ctx, u.ID, now)
	if err != nil {
		t.Fatalf("UpdateLastDecayAt: %v", err)
	}

	cfg, _ := s.GetUserDecayConfig(ctx, u.ID)
	if cfg.LastDecayAt == nil {
		t.Fatal("expected LastDecayAt to be set")
	}
}

// ===== Penalty Methods =====

func TestDebitExpiryPenalty(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	s.AdminAdjustPoints(ctx, u.ID, 100, "")

	err := s.DebitExpiryPenalty(ctx, u.ID, 1, 10)
	if err != nil {
		t.Fatalf("DebitExpiryPenalty: %v", err)
	}

	balance, _ := s.GetPointBalance(ctx, u.ID)
	if balance != 90 {
		t.Errorf("expected balance=90, got %d", balance)
	}
}

func TestDebitDecay(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	s.AdminAdjustPoints(ctx, u.ID, 100, "")

	err := s.DebitDecay(ctx, u.ID, 5, "2026-04-11", "Points decay for 2026-04-11 — missed: Test")
	if err != nil {
		t.Fatalf("DebitDecay: %v", err)
	}

	balance, _ := s.GetPointBalance(ctx, u.ID)
	if balance != 95 {
		t.Errorf("expected balance=95, got %d", balance)
	}
}

func TestDebitMissedChoreAndHasPenalty(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Missed", 10, u.ID)
	cs := createTestSchedule(t, s, c.ID, u.ID, 1)

	s.AdminAdjustPoints(ctx, u.ID, 100, "")

	// No penalty yet
	has, err := s.HasMissedChorePenalty(ctx, cs.ID, "2026-03-28")
	if err != nil {
		t.Fatalf("HasMissedChorePenalty: %v", err)
	}
	if has {
		t.Error("expected no missed penalty initially")
	}

	err = s.DebitMissedChore(ctx, u.ID, cs.ID, 10, "2026-03-28")
	if err != nil {
		t.Fatalf("DebitMissedChore: %v", err)
	}

	has, _ = s.HasMissedChorePenalty(ctx, cs.ID, "2026-03-28")
	if !has {
		t.Error("expected missed penalty to exist after debit")
	}

	balance, _ := s.GetPointBalance(ctx, u.ID)
	if balance != 90 {
		t.Errorf("expected balance=90, got %d", balance)
	}
}

// ===== Reward Assignments =====

func TestRewardAssignments(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	child1 := createTestUser(t, s, "Child1", "child")
	child2 := createTestUser(t, s, "Child2", "child")

	r := &model.Reward{Name: "Assigned Reward", Cost: 50, Active: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, r)

	customCost := 30
	assignments := []model.RewardAssignment{
		{RewardID: r.ID, UserID: child1.ID, CustomCost: &customCost},
		{RewardID: r.ID, UserID: child2.ID},
	}
	err := s.SetRewardAssignments(ctx, r.ID, assignments)
	if err != nil {
		t.Fatalf("SetRewardAssignments: %v", err)
	}

	got, err := s.GetRewardAssignments(ctx, r.ID)
	if err != nil {
		t.Fatalf("GetRewardAssignments: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(got))
	}
}

func TestListRewardsForUser(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	child1 := createTestUser(t, s, "Child1", "child")
	child2 := createTestUser(t, s, "Child2", "child")

	// Reward with no assignments (available to all)
	r1 := &model.Reward{Name: "For All", Cost: 10, Active: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, r1)

	// Reward assigned only to child1
	r2 := &model.Reward{Name: "For Child1", Cost: 20, Active: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, r2)
	customCost := 15
	s.SetRewardAssignments(ctx, r2.ID, []model.RewardAssignment{
		{RewardID: r2.ID, UserID: child1.ID, CustomCost: &customCost},
	})

	// Child1 sees both rewards
	rewards1, _ := s.ListRewardsForUser(ctx, child1.ID)
	if len(rewards1) != 2 {
		t.Errorf("child1: expected 2 rewards, got %d", len(rewards1))
	}
	// Check effective cost is custom for assigned reward
	for _, r := range rewards1 {
		if r.Name == "For Child1" && r.EffectiveCost != 15 {
			t.Errorf("expected EffectiveCost=15 for assigned reward, got %d", r.EffectiveCost)
		}
	}

	// Child2 sees only the unassigned reward
	rewards2, _ := s.ListRewardsForUser(ctx, child2.ID)
	if len(rewards2) != 1 {
		t.Errorf("child2: expected 1 reward, got %d", len(rewards2))
	}
}

// ===== Pending Completions =====

func TestListPendingCompletions(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	child := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Clean Room", 10, parent.ID)
	cs := createTestSchedule(t, s, c.ID, child.ID, 6)

	// Pending completion
	cc := &model.ChoreCompletion{
		ChoreScheduleID: cs.ID,
		CompletedBy:     child.ID,
		Status:          "pending",
		CompletionDate:  "2026-03-28",
	}
	s.CompleteChore(ctx, cc)

	pending, err := s.ListPendingCompletions(ctx)
	if err != nil {
		t.Fatalf("ListPendingCompletions: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending completion, got %d", len(pending))
	}
	if pending[0].ChoreTitle != "Clean Room" || pending[0].ChildName != "Child" {
		t.Errorf("unexpected pending: %+v", pending[0])
	}
	if pending[0].AssignedUserID != child.ID {
		t.Errorf("expected AssignedUserID=%d, got %d", child.ID, pending[0].AssignedUserID)
	}
}

// TestListPendingCompletions_SiblingCompleter verifies that when a sibling
// clicks "complete" on another kid's chore, AssignedUserID is the chore's
// owner (the assignee) while ChildName is the sibling who did the click.
func TestListPendingCompletions_SiblingCompleter(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	alex := createTestUser(t, s, "Alex", "child")
	sam := createTestUser(t, s, "Sam", "child")
	c := createTestChore(t, s, "Clean Room", 10, parent.ID)
	cs := createTestSchedule(t, s, c.ID, alex.ID, 6)

	// Sam completes a chore assigned to Alex.
	cc := &model.ChoreCompletion{
		ChoreScheduleID: cs.ID,
		CompletedBy:     sam.ID,
		Status:          "pending",
		CompletionDate:  "2026-03-28",
	}
	s.CompleteChore(ctx, cc)

	pending, err := s.ListPendingCompletions(ctx)
	if err != nil {
		t.Fatalf("ListPendingCompletions: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending completion, got %d", len(pending))
	}
	if pending[0].ChildName != "Sam" {
		t.Errorf("expected ChildName=Sam (completer), got %q", pending[0].ChildName)
	}
	if pending[0].AssignedUserID != alex.ID {
		t.Errorf("expected AssignedUserID=%d (Alex), got %d", alex.ID, pending[0].AssignedUserID)
	}
}

// ===== Webhook Deliveries =====

func TestLogAndListWebhookDeliveries(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	w := &model.Webhook{URL: "https://hook.example.com", Events: "*", Active: true}
	s.CreateWebhook(ctx, w)

	sc := 200
	d := &model.WebhookDelivery{
		WebhookID:    w.ID,
		Event:        "chore.complete",
		Payload:      `{"test": true}`,
		StatusCode:   &sc,
		ResponseBody: "OK",
	}
	err := s.LogWebhookDelivery(ctx, d)
	if err != nil {
		t.Fatalf("LogWebhookDelivery: %v", err)
	}

	deliveries, err := s.ListWebhookDeliveries(ctx, w.ID, 10)
	if err != nil {
		t.Fatalf("ListWebhookDeliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliveries))
	}
	if deliveries[0].Event != "chore.complete" {
		t.Errorf("unexpected event: %q", deliveries[0].Event)
	}
}

func TestDeleteOldWebhookDeliveries(t *testing.T) {
	s, db := setupStoreWithDB(t)
	ctx := context.Background()

	w := &model.Webhook{URL: "https://hook.example.com", Events: "*", Active: true}
	if err := s.CreateWebhook(ctx, w); err != nil {
		t.Fatalf("CreateWebhook: %v", err)
	}

	// Insert three rows: two "old" (will be backdated), one "fresh" at now.
	sc := 200
	d := func(event string) *model.WebhookDelivery {
		return &model.WebhookDelivery{
			WebhookID:    w.ID,
			Event:        event,
			Payload:      `{}`,
			StatusCode:   &sc,
			ResponseBody: "OK",
		}
	}
	for _, e := range []string{"old1", "old2", "fresh"} {
		if err := s.LogWebhookDelivery(ctx, d(e)); err != nil {
			t.Fatalf("LogWebhookDelivery %q: %v", e, err)
		}
	}

	// Backdate two rows to 60 days ago via direct SQL (test-only).
	backdated := time.Now().Add(-60 * 24 * time.Hour).UTC().Format("2006-01-02 15:04:05")
	if _, err := db.ExecContext(ctx,
		`UPDATE webhook_deliveries SET created_at = ? WHERE event IN ('old1','old2')`,
		backdated); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	// Purge rows older than 30 days: should remove both backdated rows.
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	n, err := s.DeleteOldWebhookDeliveries(ctx, cutoff)
	if err != nil {
		t.Fatalf("DeleteOldWebhookDeliveries: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 rows deleted, got %d", n)
	}

	remaining, err := s.ListWebhookDeliveries(ctx, w.ID, 10)
	if err != nil {
		t.Fatalf("ListWebhookDeliveries: %v", err)
	}
	if len(remaining) != 1 || remaining[0].Event != "fresh" {
		t.Errorf("expected only 'fresh' to remain, got %+v", remaining)
	}

	// Second call with same cutoff should delete nothing.
	n, err = s.DeleteOldWebhookDeliveries(ctx, cutoff)
	if err != nil {
		t.Fatalf("DeleteOldWebhookDeliveries (second): %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 rows deleted on second call, got %d", n)
	}
}

// ===== Redemption History =====

func TestListRedemptionsForUser(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	child := createTestUser(t, s, "Child", "child")
	s.AdminAdjustPoints(ctx, child.ID, 200, "")

	r := &model.Reward{Name: "Prize", Cost: 10, Active: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, r)

	s.RedeemReward(ctx, child.ID, r.ID)
	s.RedeemReward(ctx, child.ID, r.ID)

	history, err := s.ListRedemptionsForUser(ctx, child.ID, 10)
	if err != nil {
		t.Fatalf("ListRedemptionsForUser: %v", err)
	}
	if len(history) != 2 {
		t.Errorf("expected 2 redemptions, got %d", len(history))
	}
}

// ===== GetNetPointsForCompletion =====

func TestGetNetPointsForCompletion(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Test", 10, u.ID)
	cs := createTestSchedule(t, s, c.ID, u.ID, 1)

	cc := &model.ChoreCompletion{
		ChoreScheduleID: cs.ID,
		CompletedBy:     u.ID,
		Status:          "approved",
		CompletionDate:  "2026-03-28",
	}
	s.CompleteChore(ctx, cc)
	s.CreditChorePoints(ctx, u.ID, cc.ID, 10)

	net, err := s.GetNetPointsForCompletion(ctx, cc.ID)
	if err != nil {
		t.Fatalf("GetNetPointsForCompletion: %v", err)
	}
	if net != 10 {
		t.Errorf("expected net=10, got %d", net)
	}

	// Apply expiry penalty
	s.DebitExpiryPenalty(ctx, u.ID, cc.ID, 3)
	net, _ = s.GetNetPointsForCompletion(ctx, cc.ID)
	if net != 7 {
		t.Errorf("expected net=7 after penalty, got %d", net)
	}
}

// ===== Scheduled Chores For User =====

func TestGetScheduledChoresForUser(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	child := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Saturday Task", 10, child.ID)

	// Saturday 2026-03-28 = day_of_week 6
	cs := createTestSchedule(t, s, c.ID, child.ID, 6)

	now, _ := time.Parse(model.DateFormat, "2026-03-28")
	chores, err := s.GetScheduledChoresForUser(ctx, child.ID, []string{"2026-03-28"}, now)
	if err != nil {
		t.Fatalf("GetScheduledChoresForUser: %v", err)
	}
	if len(chores) != 1 {
		t.Fatalf("expected 1 scheduled chore, got %d", len(chores))
	}
	if chores[0].Title != "Saturday Task" || chores[0].ScheduleID != cs.ID {
		t.Errorf("unexpected chore: %+v", chores[0])
	}
	if chores[0].Completed {
		t.Error("expected not completed")
	}

	// Complete the chore and check again
	cc := &model.ChoreCompletion{
		ChoreScheduleID: cs.ID,
		CompletedBy:     child.ID,
		Status:          "approved",
		CompletionDate:  "2026-03-28",
	}
	if err := s.CompleteChore(ctx, cc); err != nil {
		t.Fatalf("CompleteChore: %v", err)
	}

	chores, err = s.GetScheduledChoresForUser(ctx, child.ID, []string{"2026-03-28"}, now)
	if err != nil {
		t.Fatalf("GetScheduledChoresForUser after completion: %v", err)
	}
	if len(chores) == 0 {
		t.Fatal("no chores returned after completion")
	}
	if !chores[0].Completed {
		t.Error("expected completed after completion")
	}

	// Wrong day returns no chores
	chores, _ = s.GetScheduledChoresForUser(ctx, child.ID, []string{"2026-03-29"}, now)
	if len(chores) != 0 {
		t.Errorf("expected 0 chores for Sunday, got %d", len(chores))
	}
}

// ===== User LineColor =====

func TestUserLineColorPersistence(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := &model.User{Name: "ColorKid", Role: "child", LineColor: "#ff5733"}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Verify line_color persists through GetUser
	got, err := s.GetUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.LineColor != "#ff5733" {
		t.Errorf("expected LineColor '#ff5733', got %q", got.LineColor)
	}
}

func TestUserLineColorUpdate(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Kid", "child")

	// Initially empty
	got, _ := s.GetUser(ctx, u.ID)
	if got.LineColor != "" {
		t.Errorf("expected empty LineColor initially, got %q", got.LineColor)
	}

	// Update line_color
	u.LineColor = "#00ff00"
	if err := s.UpdateUser(ctx, u); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	got, _ = s.GetUser(ctx, u.ID)
	if got.LineColor != "#00ff00" {
		t.Errorf("expected LineColor '#00ff00' after update, got %q", got.LineColor)
	}
}

func TestUserLineColorInListUsers(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u1 := &model.User{Name: "Alice", Role: "child", LineColor: "#aaa"}
	s.CreateUser(ctx, u1)
	u2 := &model.User{Name: "Bob", Role: "child", LineColor: "#bbb"}
	s.CreateUser(ctx, u2)

	users, err := s.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	// Users ordered by name: Alice, Bob
	if users[0].LineColor != "#aaa" {
		t.Errorf("expected Alice LineColor '#aaa', got %q", users[0].LineColor)
	}
	if users[1].LineColor != "#bbb" {
		t.Errorf("expected Bob LineColor '#bbb', got %q", users[1].LineColor)
	}
}

func TestUserLineColorPreservedOnOtherFieldUpdate(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := &model.User{Name: "Kid", Role: "child", LineColor: "#123456"}
	s.CreateUser(ctx, u)

	// Update name only (simulating admin update flow: get, modify name, save)
	got, _ := s.GetUser(ctx, u.ID)
	got.Name = "Updated Kid"
	if err := s.UpdateUser(ctx, got); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	final, _ := s.GetUser(ctx, u.ID)
	if final.Name != "Updated Kid" {
		t.Errorf("expected name 'Updated Kid', got %q", final.Name)
	}
	if final.LineColor != "#123456" {
		t.Errorf("expected LineColor preserved as '#123456', got %q", final.LineColor)
	}
}

// ===== One-Off Schedule (specific_date) =====

func TestCreateScheduleWithSpecificDate(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "One-off task", 10, u.ID)

	date := "2026-04-10"
	cs := &model.ChoreSchedule{
		ChoreID:          c.ID,
		AssignedTo:       u.ID,
		AssignmentType:   "individual",
		SpecificDate:     &date,
		PointsMultiplier: 1.0,
		ExpiryPenalty:    "none",
	}
	err := s.CreateSchedule(ctx, cs)
	if err != nil {
		t.Fatalf("CreateSchedule with specific_date: %v", err)
	}
	if cs.ID == 0 {
		t.Fatal("expected non-zero schedule ID")
	}

	// Retrieve and verify
	got, err := s.GetSchedule(ctx, cs.ID)
	if err != nil {
		t.Fatalf("GetSchedule: %v", err)
	}
	if got.SpecificDate == nil {
		t.Error("expected SpecificDate to contain '2026-04-10', got nil")
	} else if !strings.HasPrefix(*got.SpecificDate, "2026-04-10") {
		t.Errorf("expected SpecificDate to start with '2026-04-10', got %q", *got.SpecificDate)
	}
	if got.DayOfWeek != nil {
		t.Errorf("expected DayOfWeek nil for one-off, got %v", got.DayOfWeek)
	}
}

func TestOneOffScheduleAppearsForCorrectDate(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	child := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Special task", 15, child.ID)

	date := "2026-04-15"
	cs := &model.ChoreSchedule{
		ChoreID:          c.ID,
		AssignedTo:       child.ID,
		AssignmentType:   "individual",
		SpecificDate:     &date,
		PointsMultiplier: 1.0,
		ExpiryPenalty:    "none",
	}
	s.CreateSchedule(ctx, cs)

	now, _ := time.Parse(model.DateFormat, "2026-04-15")

	// Should appear on the correct date
	chores, err := s.GetScheduledChoresForUser(ctx, child.ID, []string{"2026-04-15"}, now)
	if err != nil {
		t.Fatalf("GetScheduledChoresForUser: %v", err)
	}
	if len(chores) != 1 {
		t.Fatalf("expected 1 chore on 2026-04-15, got %d", len(chores))
	}
	if chores[0].Title != "Special task" {
		t.Errorf("expected 'Special task', got %q", chores[0].Title)
	}

	// Should NOT appear on a different date
	chores, _ = s.GetScheduledChoresForUser(ctx, child.ID, []string{"2026-04-16"}, now)
	if len(chores) != 0 {
		t.Errorf("expected 0 chores on 2026-04-16, got %d", len(chores))
	}
}

func TestOneOffAndRecurringSchedulesTogether(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	child := createTestUser(t, s, "Child", "child")
	c1 := createTestChore(t, s, "Recurring chore", 5, child.ID)
	c2 := createTestChore(t, s, "One-off chore", 10, child.ID)

	// 2026-04-08 is a Wednesday (day_of_week = 3)
	createTestSchedule(t, s, c1.ID, child.ID, 3)

	date := "2026-04-08"
	cs := &model.ChoreSchedule{
		ChoreID:          c2.ID,
		AssignedTo:       child.ID,
		AssignmentType:   "individual",
		SpecificDate:     &date,
		PointsMultiplier: 1.0,
		ExpiryPenalty:    "none",
	}
	s.CreateSchedule(ctx, cs)

	now, _ := time.Parse(model.DateFormat, "2026-04-08")

	// Both should appear on 2026-04-08
	chores, err := s.GetScheduledChoresForUser(ctx, child.ID, []string{"2026-04-08"}, now)
	if err != nil {
		t.Fatalf("GetScheduledChoresForUser: %v", err)
	}
	if len(chores) != 2 {
		t.Fatalf("expected 2 chores (recurring + one-off), got %d", len(chores))
	}

	// On Thursday, only recurring if it matches — since recurring is Wednesday only, expect 0
	chores, _ = s.GetScheduledChoresForUser(ctx, child.ID, []string{"2026-04-09"}, now)
	if len(chores) != 0 {
		t.Errorf("expected 0 chores on Thursday, got %d", len(chores))
	}
}

func TestScheduleExistsForDate(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	child := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Task", 5, child.ID)

	date := "2026-04-20"
	cs := &model.ChoreSchedule{
		ChoreID:          c.ID,
		AssignedTo:       child.ID,
		AssignmentType:   "individual",
		SpecificDate:     &date,
		PointsMultiplier: 1.0,
		ExpiryPenalty:    "none",
	}
	s.CreateSchedule(ctx, cs)

	exists, err := s.ScheduleExistsForDate(ctx, c.ID, child.ID, "2026-04-20")
	if err != nil {
		t.Fatalf("ScheduleExistsForDate: %v", err)
	}
	if !exists {
		t.Error("expected schedule to exist for date 2026-04-20")
	}

	exists, _ = s.ScheduleExistsForDate(ctx, c.ID, child.ID, "2026-04-21")
	if exists {
		t.Error("expected no schedule for date 2026-04-21")
	}
}

// ===== AI Verification Fields =====

func TestCompleteChoreWithAIFields(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Clean Room", 10, u.ID)
	cs := createTestSchedule(t, s, c.ID, u.ID, 1) // Monday

	cc := &model.ChoreCompletion{
		ChoreScheduleID: cs.ID,
		CompletedBy:     u.ID,
		Status:          "approved",
		PhotoURL:        "/uploads/test.jpg",
		CompletionDate:  "2026-03-30",
		AIFeedback:      "Great job! The room looks very clean.",
		AIConfidence:    0.92,
	}
	err := s.CompleteChore(ctx, cc)
	if err != nil {
		t.Fatalf("CompleteChore with AI fields: %v", err)
	}
	if cc.ID == 0 {
		t.Fatal("expected non-zero completion ID")
	}

	// Verify via GetCompletion
	got, err := s.GetCompletion(ctx, cc.ID)
	if err != nil {
		t.Fatalf("GetCompletion: %v", err)
	}
	if got.AIFeedback != "Great job! The room looks very clean." {
		t.Errorf("expected AI feedback preserved, got %q", got.AIFeedback)
	}
	if got.AIConfidence != 0.92 {
		t.Errorf("expected AI confidence 0.92, got %f", got.AIConfidence)
	}

	// Verify via GetCompletionForScheduleDate
	got2, err := s.GetCompletionForScheduleDate(ctx, cs.ID, "2026-03-30")
	if err != nil {
		t.Fatalf("GetCompletionForScheduleDate: %v", err)
	}
	if got2.AIFeedback != "Great job! The room looks very clean." {
		t.Errorf("expected AI feedback preserved, got %q", got2.AIFeedback)
	}
	if got2.AIConfidence != 0.92 {
		t.Errorf("expected AI confidence 0.92, got %f", got2.AIConfidence)
	}
}

func TestCompleteChoreAIRejected(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Make Bed", 5, u.ID)
	cs := createTestSchedule(t, s, c.ID, u.ID, 2) // Tuesday

	cc := &model.ChoreCompletion{
		ChoreScheduleID: cs.ID,
		CompletedBy:     u.ID,
		Status:          "ai_rejected",
		PhotoURL:        "/uploads/bed.jpg",
		CompletionDate:  "2026-03-31",
		AIFeedback:      "Almost! The pillows need to be straightened.",
		AIConfidence:    0.35,
	}
	err := s.CompleteChore(ctx, cc)
	if err != nil {
		t.Fatalf("CompleteChore ai_rejected: %v", err)
	}

	got, err := s.GetCompletion(ctx, cc.ID)
	if err != nil {
		t.Fatalf("GetCompletion: %v", err)
	}
	if got.Status != "ai_rejected" {
		t.Errorf("expected status ai_rejected, got %q", got.Status)
	}
	if got.AIFeedback != "Almost! The pillows need to be straightened." {
		t.Errorf("unexpected AI feedback: %q", got.AIFeedback)
	}
}

func TestGetScheduledChoresIncludesAIFields(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	child := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Sweep Floor", 10, child.ID)
	// Saturday 2026-04-04 = day_of_week 6
	cs := createTestSchedule(t, s, c.ID, child.ID, 6)

	// Complete with ai_rejected status
	cc := &model.ChoreCompletion{
		ChoreScheduleID: cs.ID,
		CompletedBy:     child.ID,
		Status:          "ai_rejected",
		PhotoURL:        "/uploads/floor.jpg",
		CompletionDate:  "2026-04-04",
		AIFeedback:      "There is still dirt in the corner.",
		AIConfidence:    0.25,
	}
	if err := s.CompleteChore(ctx, cc); err != nil {
		t.Fatalf("CompleteChore: %v", err)
	}

	now, _ := time.Parse(model.DateFormat, "2026-04-04")
	chores, err := s.GetScheduledChoresForUser(ctx, child.ID, []string{"2026-04-04"}, now)
	if err != nil {
		t.Fatalf("GetScheduledChoresForUser: %v", err)
	}
	if len(chores) != 1 {
		t.Fatalf("expected 1 scheduled chore, got %d", len(chores))
	}

	sc := chores[0]

	// ai_rejected completions should NOT be considered "completed"
	if sc.Completed {
		t.Error("expected Completed=false for ai_rejected completion")
	}

	// CompletionStatus should be set to ai_rejected
	if sc.CompletionStatus == nil || *sc.CompletionStatus != "ai_rejected" {
		t.Errorf("expected CompletionStatus=ai_rejected, got %v", sc.CompletionStatus)
	}

	// AIFeedback should be populated
	if sc.AIFeedback == nil || *sc.AIFeedback != "There is still dirt in the corner." {
		t.Errorf("expected AI feedback to be set, got %v", sc.AIFeedback)
	}

	// CompletionID should still be set (so the frontend knows there's a record)
	if sc.CompletionID == nil {
		t.Error("expected CompletionID to be set even for ai_rejected")
	}
}

func TestGetScheduledChoresApprovedIncludesAIFeedback(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	child := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Dishes", 10, child.ID)
	// Saturday 2026-04-04 = day_of_week 6
	cs := createTestSchedule(t, s, c.ID, child.ID, 6)

	cc := &model.ChoreCompletion{
		ChoreScheduleID: cs.ID,
		CompletedBy:     child.ID,
		Status:          "approved",
		PhotoURL:        "/uploads/dishes.jpg",
		CompletionDate:  "2026-04-04",
		AIFeedback:      "Looks great, nice and clean!",
		AIConfidence:    0.95,
	}
	if err := s.CompleteChore(ctx, cc); err != nil {
		t.Fatalf("CompleteChore: %v", err)
	}

	now, _ := time.Parse(model.DateFormat, "2026-04-04")
	chores, err := s.GetScheduledChoresForUser(ctx, child.ID, []string{"2026-04-04"}, now)
	if err != nil {
		t.Fatalf("GetScheduledChoresForUser: %v", err)
	}
	if len(chores) != 1 {
		t.Fatalf("expected 1 chore, got %d", len(chores))
	}

	sc := chores[0]
	// Approved completion should be "completed"
	if !sc.Completed {
		t.Error("expected Completed=true for approved completion")
	}
	// AI feedback should still be available
	if sc.AIFeedback == nil || *sc.AIFeedback != "Looks great, nice and clean!" {
		t.Errorf("expected AI feedback for approved completion, got %v", sc.AIFeedback)
	}
}

func TestUpdateChoreTTSDescription(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Parent", "admin")
	c := createTestChore(t, s, "Feed Cat", 5, u.ID)

	// Initially empty
	got, _ := s.GetChore(ctx, c.ID)
	if got.TTSDescription != "" {
		t.Errorf("expected empty TTS description initially, got %q", got.TTSDescription)
	}

	// Update TTS description
	err := s.UpdateChoreTTSDescription(ctx, c.ID, "Time to feed the kitty cat! Give them their food and fresh water.")
	if err != nil {
		t.Fatalf("UpdateChoreTTSDescription: %v", err)
	}

	// Verify it persists
	got, _ = s.GetChore(ctx, c.ID)
	if got.TTSDescription != "Time to feed the kitty cat! Give them their food and fresh water." {
		t.Errorf("TTS description not updated, got %q", got.TTSDescription)
	}

	// Update again to empty
	err = s.UpdateChoreTTSDescription(ctx, c.ID, "")
	if err != nil {
		t.Fatalf("UpdateChoreTTSDescription to empty: %v", err)
	}
	got, _ = s.GetChore(ctx, c.ID)
	if got.TTSDescription != "" {
		t.Errorf("expected empty TTS description, got %q", got.TTSDescription)
	}
}

func TestTTSDescriptionInScheduledChores(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	child := createTestUser(t, s, "Child", "child")

	// Create chore with TTS description set via the model
	c := &model.Chore{
		Title:          "Brush Teeth",
		Description:    "Brush your teeth for two minutes",
		Category:       "required",
		PointsValue:    5,
		Source:         "manual",
		TTSDescription: "Time to brush your teeth! Make sure to brush for two whole minutes.",
		CreatedBy:      child.ID,
	}
	if err := s.CreateChore(ctx, c); err != nil {
		t.Fatalf("CreateChore with TTS: %v", err)
	}

	// Saturday 2026-04-04 = day_of_week 6
	cs := createTestSchedule(t, s, c.ID, child.ID, 6)
	_ = cs

	now, _ := time.Parse(model.DateFormat, "2026-04-04")
	chores, err := s.GetScheduledChoresForUser(ctx, child.ID, []string{"2026-04-04"}, now)
	if err != nil {
		t.Fatalf("GetScheduledChoresForUser: %v", err)
	}
	if len(chores) != 1 {
		t.Fatalf("expected 1 chore, got %d", len(chores))
	}
	if chores[0].TTSDescription != "Time to brush your teeth! Make sure to brush for two whole minutes." {
		t.Errorf("expected TTS description in scheduled chore, got %q", chores[0].TTSDescription)
	}
}

func TestCompleteChoreWithEmptyAIFields(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Walk Dog", 10, u.ID)
	cs := createTestSchedule(t, s, c.ID, u.ID, 1)

	// Complete without AI fields (normal non-AI flow)
	cc := &model.ChoreCompletion{
		ChoreScheduleID: cs.ID,
		CompletedBy:     u.ID,
		Status:          "approved",
		CompletionDate:  "2026-03-30",
	}
	err := s.CompleteChore(ctx, cc)
	if err != nil {
		t.Fatalf("CompleteChore without AI fields: %v", err)
	}

	got, err := s.GetCompletion(ctx, cc.ID)
	if err != nil {
		t.Fatalf("GetCompletion: %v", err)
	}
	if got.AIFeedback != "" {
		t.Errorf("expected empty AI feedback, got %q", got.AIFeedback)
	}
	if got.AIConfidence != 0 {
		t.Errorf("expected zero AI confidence, got %f", got.AIConfidence)
	}
}

// ===== Missed-chore penalty idempotency (issue #17) =====

// TestDebitMissedChoreIsIdempotent asserts that applying a missed-chore
// penalty twice for the same (user, schedule, date) inserts exactly one
// point_transactions row and deducts points exactly once. The check lives
// in a UNIQUE partial index on idempotency_key; the second call is expected
// to succeed as a no-op rather than return an error.
func TestDebitMissedChoreIsIdempotent(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Laundry", 5, u.ID)
	cs := createTestSchedule(t, s, c.ID, u.ID, 1)

	const date = "2026-04-10"
	const penalty = 3

	if err := s.DebitMissedChore(ctx, u.ID, cs.ID, penalty, date); err != nil {
		t.Fatalf("first DebitMissedChore: %v", err)
	}
	if err := s.DebitMissedChore(ctx, u.ID, cs.ID, penalty, date); err != nil {
		t.Fatalf("second DebitMissedChore (should be no-op, not error): %v", err)
	}

	// Exactly one penalty row should exist for this schedule+date.
	txs, err := s.ListPointTransactions(ctx, u.ID, 10)
	if err != nil {
		t.Fatalf("ListPointTransactions: %v", err)
	}
	var penaltyCount int
	for _, tx := range txs {
		if tx.Reason == "missed_chore" && tx.ReferenceID != nil && *tx.ReferenceID == cs.ID {
			penaltyCount++
			if tx.IdempotencyKey == nil {
				t.Errorf("penalty row has NULL idempotency_key")
			} else if want := "missed_chore_penalty:"; !strings.HasPrefix(*tx.IdempotencyKey, want) {
				t.Errorf("idempotency key = %q, want prefix %q", *tx.IdempotencyKey, want)
			}
		}
	}
	if penaltyCount != 1 {
		t.Errorf("expected 1 penalty transaction, got %d", penaltyCount)
	}

	// Balance should reflect exactly one -3 debit, not -6.
	bal, err := s.GetPointBalance(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetPointBalance: %v", err)
	}
	if bal != -penalty {
		t.Errorf("balance = %d, want %d (debited once)", bal, -penalty)
	}
}

// TestHasMissedChorePenaltyExactMatch verifies HasMissedChorePenalty returns
// true only for the exact (schedule, date) pair that was penalized — i.e.
// the lookup is an exact key match, not a substring search that could
// accidentally match an adjacent date or schedule.
func TestHasMissedChorePenaltyExactMatch(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Dishes", 5, u.ID)
	cs1 := createTestSchedule(t, s, c.ID, u.ID, 1)
	cs2 := createTestSchedule(t, s, c.ID, u.ID, 2)

	// Before any penalty, nothing is flagged.
	for _, cs := range []*model.ChoreSchedule{cs1, cs2} {
		got, err := s.HasMissedChorePenalty(ctx, cs.ID, "2026-04-10")
		if err != nil {
			t.Fatalf("HasMissedChorePenalty: %v", err)
		}
		if got {
			t.Errorf("schedule %d: unexpected penalty reported before any debit", cs.ID)
		}
	}

	// Apply a penalty for (cs1, 2026-04-10) only.
	if err := s.DebitMissedChore(ctx, u.ID, cs1.ID, 2, "2026-04-10"); err != nil {
		t.Fatalf("DebitMissedChore: %v", err)
	}

	cases := []struct {
		name       string
		scheduleID int64
		date       string
		want       bool
	}{
		{"exact match", cs1.ID, "2026-04-10", true},
		{"same schedule, different date", cs1.ID, "2026-04-11", false},
		{"different schedule, same date", cs2.ID, "2026-04-10", false},
		{"unknown schedule", 99999, "2026-04-10", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.HasMissedChorePenalty(ctx, tc.scheduleID, tc.date)
			if err != nil {
				t.Fatalf("HasMissedChorePenalty: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestDebitMissedChoreDistinctKeysCoexist proves the UNIQUE index scopes
// idempotency to the exact (user, schedule, date) tuple — penalties for
// different dates or different schedules are NOT blocked.
func TestDebitMissedChoreDistinctKeysCoexist(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Sweep", 10, u.ID)
	cs1 := createTestSchedule(t, s, c.ID, u.ID, 1)
	cs2 := createTestSchedule(t, s, c.ID, u.ID, 2)

	calls := []struct {
		scheduleID int64
		date       string
	}{
		{cs1.ID, "2026-04-10"},
		{cs1.ID, "2026-04-11"}, // same schedule, different date
		{cs2.ID, "2026-04-10"}, // different schedule, same date
	}
	for _, call := range calls {
		if err := s.DebitMissedChore(ctx, u.ID, call.scheduleID, 2, call.date); err != nil {
			t.Fatalf("DebitMissedChore(%d, %s): %v", call.scheduleID, call.date, err)
		}
	}

	bal, err := s.GetPointBalance(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetPointBalance: %v", err)
	}
	if want := -6; bal != want {
		t.Errorf("balance = %d, want %d (three distinct penalties)", bal, want)
	}
}

// TestNullIdempotencyKeysAreNotUnique asserts that non-idempotent
// transactions (e.g. chore credits) with NULL idempotency_key do not
// collide with each other under the partial UNIQUE index. Without the
// `WHERE idempotency_key IS NOT NULL` predicate, the index would treat
// all NULLs as identical and reject the second insert.
func TestNullIdempotencyKeysAreNotUnique(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	u := createTestUser(t, s, "Child", "child")
	c := createTestChore(t, s, "Trash", 5, u.ID)
	cs := createTestSchedule(t, s, c.ID, u.ID, 3)

	// Two chore completions → two credit rows, both with NULL idempotency_key.
	cc1 := &model.ChoreCompletion{ChoreScheduleID: cs.ID, CompletedBy: u.ID, CompletionDate: "2026-04-10", Status: "approved"}
	cc2 := &model.ChoreCompletion{ChoreScheduleID: cs.ID, CompletedBy: u.ID, CompletionDate: "2026-04-11", Status: "approved"}
	if err := s.CompleteChore(ctx, cc1); err != nil {
		t.Fatalf("CompleteChore 1: %v", err)
	}
	if err := s.CompleteChore(ctx, cc2); err != nil {
		t.Fatalf("CompleteChore 2: %v", err)
	}
	if err := s.CreditChorePoints(ctx, u.ID, cc1.ID, 5); err != nil {
		t.Fatalf("CreditChorePoints 1 (NULL key): %v", err)
	}
	if err := s.CreditChorePoints(ctx, u.ID, cc2.ID, 5); err != nil {
		t.Fatalf("CreditChorePoints 2 (NULL key): %v", err)
	}

	txs, err := s.ListPointTransactions(ctx, u.ID, 10)
	if err != nil {
		t.Fatalf("ListPointTransactions: %v", err)
	}
	var creditCount int
	for _, tx := range txs {
		if tx.Reason == model.ReasonChoreComplete {
			creditCount++
			if tx.IdempotencyKey != nil {
				t.Errorf("chore credit unexpectedly has idempotency_key = %q", *tx.IdempotencyKey)
			}
		}
	}
	if creditCount != 2 {
		t.Errorf("expected 2 chore-credit rows with NULL keys, got %d", creditCount)
	}
}

// ===== Reward Commitments =====

func TestCommitmentManualSaveAndRedeem(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	kid := createTestUser(t, s, "Kid", "child")

	// Big-ticket reward.
	reward := &model.Reward{Name: "LEGO set", Cost: 500, Active: true, CreatedBy: parent.ID}
	if err := s.CreateReward(ctx, reward); err != nil {
		t.Fatalf("CreateReward: %v", err)
	}

	// Seed the kid with 700 points via an admin adjust so we don't need a chore.
	if err := s.AdminAdjustPoints(ctx, kid.ID, 700, "test seed"); err != nil {
		t.Fatalf("AdminAdjustPoints: %v", err)
	}

	c, err := s.CreateCommitment(ctx, kid.ID, reward.ID, 0)
	if err != nil {
		t.Fatalf("CreateCommitment: %v", err)
	}
	if c.TargetCost != 500 {
		t.Errorf("expected target_cost=500, got %d", c.TargetCost)
	}
	if c.AmountSaved != 0 {
		t.Errorf("expected amount_saved=0 on fresh commitment, got %d", c.AmountSaved)
	}

	// A second active commitment for the same kid should be rejected.
	if _, err := s.CreateCommitment(ctx, kid.ID, reward.ID, 0); err != store.ErrActiveCommitmentExists {
		t.Errorf("expected ErrActiveCommitmentExists, got %v", err)
	}

	// Contribute 200 manually. Spendable should drop, saved should rise.
	if err := s.ContributeToCommitment(ctx, kid.ID, c.ID, 200); err != nil {
		t.Fatalf("ContributeToCommitment 200: %v", err)
	}
	bal, _ := s.GetPointBalance(ctx, kid.ID)
	if bal != 500 {
		t.Errorf("expected spendable 500 after 200 contribution, got %d", bal)
	}
	c, err = s.GetActiveCommitmentForUser(ctx, kid.ID)
	if err != nil || c == nil {
		t.Fatalf("GetActiveCommitmentForUser: %v / %v", err, c)
	}
	if c.AmountSaved != 200 {
		t.Errorf("expected amount_saved=200, got %d", c.AmountSaved)
	}

	// Try to redeem before fully funded — should fail.
	if _, err := s.RedeemReward(ctx, kid.ID, reward.ID); err == nil {
		t.Errorf("expected redeem to fail before commitment is fully funded")
	}

	// Top up to target (300 more) — store should cap if asked for too much.
	if err := s.ContributeToCommitment(ctx, kid.ID, c.ID, 1000); err != nil {
		t.Fatalf("ContributeToCommitment top-up: %v", err)
	}
	c, _ = s.GetActiveCommitmentForUser(ctx, kid.ID)
	if c.AmountSaved != 500 {
		t.Errorf("expected saved capped at 500, got %d", c.AmountSaved)
	}
	bal, _ = s.GetPointBalance(ctx, kid.ID)
	if bal != 200 {
		t.Errorf("expected spendable 200 after full save, got %d", bal)
	}

	// Redeem now — should mark commitment redeemed and net spendable change to -500 across the
	// whole flow (700 seeded → 200 spendable + 500 saved → 200 spendable + 0 saved + 1 redemption).
	red, err := s.RedeemReward(ctx, kid.ID, reward.ID)
	if err != nil {
		t.Fatalf("RedeemReward: %v", err)
	}
	if red.PointsSpent != 500 {
		t.Errorf("expected points_spent=500, got %d", red.PointsSpent)
	}
	bal, _ = s.GetPointBalance(ctx, kid.ID)
	if bal != 200 {
		t.Errorf("expected spendable 200 after redemption (unchanged), got %d", bal)
	}
	if got, _ := s.GetActiveCommitmentForUser(ctx, kid.ID); got != nil {
		t.Errorf("expected no active commitment after redeem, got %+v", got)
	}
}

func TestCommitmentBreakReturnsSavedToSpendable(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	kid := createTestUser(t, s, "Kid", "child")
	reward := &model.Reward{Name: "Trip", Cost: 100, Active: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, reward)
	s.AdminAdjustPoints(ctx, kid.ID, 80, "seed")

	c, err := s.CreateCommitment(ctx, kid.ID, reward.ID, 0)
	if err != nil {
		t.Fatalf("CreateCommitment: %v", err)
	}
	if err := s.ContributeToCommitment(ctx, kid.ID, c.ID, 60); err != nil {
		t.Fatalf("ContributeToCommitment: %v", err)
	}

	bal, _ := s.GetPointBalance(ctx, kid.ID)
	if bal != 20 {
		t.Errorf("expected spendable 20 after committing 60, got %d", bal)
	}

	if err := s.BreakCommitment(ctx, kid.ID, c.ID); err != nil {
		t.Fatalf("BreakCommitment: %v", err)
	}
	bal, _ = s.GetPointBalance(ctx, kid.ID)
	if bal != 80 {
		t.Errorf("expected spendable 80 after break, got %d", bal)
	}
	if got, _ := s.GetActiveCommitmentForUser(ctx, kid.ID); got != nil {
		t.Errorf("expected no active commitment after break, got %+v", got)
	}
	// A new commitment should now be allowed.
	if _, err := s.CreateCommitment(ctx, kid.ID, reward.ID, 0); err != nil {
		t.Errorf("expected to be able to create a new commitment, got %v", err)
	}
}

func TestAutoContributeOnChoreCredit(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	kid := createTestUser(t, s, "Kid", "child")
	reward := &model.Reward{Name: "Bike", Cost: 1000, Active: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, reward)

	c, err := s.CreateCommitment(ctx, kid.ID, reward.ID, 25)
	if err != nil {
		t.Fatalf("CreateCommitment: %v", err)
	}

	// Simulate a chore completion paying 40 points. 25% should auto-contribute.
	chore := createTestChore(t, s, "Big chore", 40, parent.ID)
	cs := createTestSchedule(t, s, chore.ID, kid.ID, 1)
	cc := &model.ChoreCompletion{ChoreScheduleID: cs.ID, CompletedBy: kid.ID, Status: model.StatusApproved, CompletionDate: "2026-05-06"}
	if err := s.CompleteChore(ctx, cc); err != nil {
		t.Fatalf("CompleteChore: %v", err)
	}
	if err := s.CreditChorePoints(ctx, kid.ID, cc.ID, 40); err != nil {
		t.Fatalf("CreditChorePoints: %v", err)
	}

	bal, _ := s.GetPointBalance(ctx, kid.ID)
	if bal != 30 {
		t.Errorf("expected spendable 30 (40 credit - 10 auto), got %d", bal)
	}
	got, err := s.GetActiveCommitmentForUser(ctx, kid.ID)
	if err != nil || got == nil {
		t.Fatalf("GetActiveCommitmentForUser: %v / %v", err, got)
	}
	if got.AmountSaved != 10 {
		t.Errorf("expected amount_saved=10 (25%% of 40), got %d", got.AmountSaved)
	}

	// Uncomplete the chore — both the credit and the auto-contribution should reverse.
	if err := s.DebitChorePoints(ctx, kid.ID, cc.ID, 40); err != nil {
		t.Fatalf("DebitChorePoints: %v", err)
	}
	bal, _ = s.GetPointBalance(ctx, kid.ID)
	if bal != 0 {
		t.Errorf("expected spendable 0 after uncomplete, got %d", bal)
	}
	got, _ = s.GetActiveCommitmentForUser(ctx, kid.ID)
	if got.AmountSaved != 0 {
		t.Errorf("expected amount_saved=0 after auto-revert, got %d", got.AmountSaved)
	}

	// Idempotency: calling DebitChorePoints again must not over-credit savings back.
	if err := s.DebitChorePoints(ctx, kid.ID, cc.ID, 40); err != nil {
		t.Fatalf("DebitChorePoints again: %v", err)
	}
	got, _ = s.GetActiveCommitmentForUser(ctx, kid.ID)
	if got.AmountSaved != 0 {
		t.Errorf("expected amount_saved still 0 after duplicate uncomplete, got %d", got.AmountSaved)
	}
	_ = c
}

func TestAutoContributeCapsAtTargetCost(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	kid := createTestUser(t, s, "Kid", "child")
	reward := &model.Reward{Name: "Sticker", Cost: 5, Active: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, reward)
	c, _ := s.CreateCommitment(ctx, kid.ID, reward.ID, 100)

	chore := createTestChore(t, s, "Huge chore", 100, parent.ID)
	cs := createTestSchedule(t, s, chore.ID, kid.ID, 1)
	cc := &model.ChoreCompletion{ChoreScheduleID: cs.ID, CompletedBy: kid.ID, Status: model.StatusApproved, CompletionDate: "2026-05-06"}
	s.CompleteChore(ctx, cc)
	if err := s.CreditChorePoints(ctx, kid.ID, cc.ID, 100); err != nil {
		t.Fatalf("CreditChorePoints: %v", err)
	}

	got, _ := s.GetActiveCommitmentForUser(ctx, kid.ID)
	if got.AmountSaved != 5 {
		t.Errorf("expected auto-contribute capped at target_cost 5, got %d", got.AmountSaved)
	}
	bal, _ := s.GetPointBalance(ctx, kid.ID)
	if bal != 95 {
		t.Errorf("expected spendable 95 after 5 was siphoned, got %d", bal)
	}
	_ = c
}

func TestCommitmentSnapshotsTargetCost(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	kid := createTestUser(t, s, "Kid", "child")
	reward := &model.Reward{Name: "Toy", Cost: 100, Active: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, reward)

	c, err := s.CreateCommitment(ctx, kid.ID, reward.ID, 0)
	if err != nil {
		t.Fatalf("CreateCommitment: %v", err)
	}
	if c.TargetCost != 100 {
		t.Errorf("expected snapshotted target=100, got %d", c.TargetCost)
	}

	// Admin raises the price after the kid started saving.
	reward.Cost = 200
	if err := s.UpdateReward(ctx, reward); err != nil {
		t.Fatalf("UpdateReward: %v", err)
	}

	// Refetch commitment — target_cost should still be the snapshotted value.
	got, _ := s.GetActiveCommitmentForUser(ctx, kid.ID)
	if got.TargetCost != 100 {
		t.Errorf("expected target_cost still 100 (snapshotted), got %d", got.TargetCost)
	}
}

// ===== Shared / Family Goals =====

func TestSharedPoolJoinAndContribute(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	alice := createTestUser(t, s, "Alice", "child")
	bob := createTestUser(t, s, "Bob", "child")

	reward := &model.Reward{Name: "Minecraft", Cost: 600, Active: true, Shareable: true, CreatedBy: parent.ID}
	if err := s.CreateReward(ctx, reward); err != nil {
		t.Fatalf("CreateReward: %v", err)
	}
	s.AdminAdjustPoints(ctx, alice.ID, 500, "seed")
	s.AdminAdjustPoints(ctx, bob.ID, 300, "seed")

	// Alice joins first — creates the pool.
	a, err := s.CreateCommitment(ctx, alice.ID, reward.ID, 0)
	if err != nil {
		t.Fatalf("Alice CreateCommitment: %v", err)
	}
	if a.SharedPoolID == nil {
		t.Fatalf("expected SharedPoolID populated for shareable reward")
	}
	if a.Pool == nil || a.Pool.TargetCost != 600 {
		t.Fatalf("expected pool target=600, got %+v", a.Pool)
	}

	// Bob joins same pool — should reuse not create a new one.
	b, err := s.CreateCommitment(ctx, bob.ID, reward.ID, 0)
	if err != nil {
		t.Fatalf("Bob CreateCommitment: %v", err)
	}
	if b.SharedPoolID == nil || *a.SharedPoolID != *b.SharedPoolID {
		t.Errorf("expected Bob to join same pool as Alice (alice=%v bob=%v)", a.SharedPoolID, b.SharedPoolID)
	}

	// Each kid contributes their share. Pool total should sum across kids.
	if err := s.ContributeToCommitment(ctx, alice.ID, a.ID, 400); err != nil {
		t.Fatalf("Alice contribute: %v", err)
	}
	if err := s.ContributeToCommitment(ctx, bob.ID, b.ID, 200); err != nil {
		t.Fatalf("Bob contribute: %v", err)
	}

	pool, _ := s.GetSharedPool(ctx, *a.SharedPoolID)
	if pool.AmountSaved != 600 {
		t.Errorf("expected pool saved=600, got %d", pool.AmountSaved)
	}
	if len(pool.Contributors) != 2 {
		t.Errorf("expected 2 contributors, got %d", len(pool.Contributors))
	}

	// Personal commitment should be unaffected by shared participation.
	if existing, _ := s.GetActiveCommitmentForUser(ctx, alice.ID); existing != nil {
		t.Errorf("expected no active personal commitment for alice, got %+v", existing)
	}
	// Alice should still be able to start a personal goal.
	cheap := &model.Reward{Name: "LEGO", Cost: 50, Active: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, cheap)
	if _, err := s.CreateCommitment(ctx, alice.ID, cheap.ID, 0); err != nil {
		t.Errorf("expected personal commitment alongside shared share, got error: %v", err)
	}
}

func TestSharedPoolContributeCapsAtPoolRemaining(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	alice := createTestUser(t, s, "Alice", "child")
	bob := createTestUser(t, s, "Bob", "child")

	reward := &model.Reward{Name: "Family Trip", Cost: 100, Active: true, Shareable: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, reward)
	s.AdminAdjustPoints(ctx, alice.ID, 200, "seed")
	s.AdminAdjustPoints(ctx, bob.ID, 200, "seed")

	a, _ := s.CreateCommitment(ctx, alice.ID, reward.ID, 0)
	b, _ := s.CreateCommitment(ctx, bob.ID, reward.ID, 0)

	// Alice fills 90 of the pool first.
	if err := s.ContributeToCommitment(ctx, alice.ID, a.ID, 90); err != nil {
		t.Fatalf("Alice contribute: %v", err)
	}
	// Bob asks for 50 — should be capped at 10 (pool has 10 left).
	if err := s.ContributeToCommitment(ctx, bob.ID, b.ID, 50); err != nil {
		t.Fatalf("Bob contribute: %v", err)
	}
	pool, _ := s.GetSharedPool(ctx, *a.SharedPoolID)
	if pool.AmountSaved != 100 {
		t.Errorf("expected pool capped at 100, got %d", pool.AmountSaved)
	}
	bobBal, _ := s.GetPointBalance(ctx, bob.ID)
	if bobBal != 190 {
		t.Errorf("expected bob spendable 190 (only 10 contributed), got %d", bobBal)
	}
}

func TestSharedPoolRedemptionDebitsEachContributor(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	alice := createTestUser(t, s, "Alice", "child")
	bob := createTestUser(t, s, "Bob", "child")

	reward := &model.Reward{Name: "Minecraft", Cost: 300, Active: true, Shareable: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, reward)
	s.AdminAdjustPoints(ctx, alice.ID, 250, "seed")
	s.AdminAdjustPoints(ctx, bob.ID, 250, "seed")

	a, _ := s.CreateCommitment(ctx, alice.ID, reward.ID, 0)
	b, _ := s.CreateCommitment(ctx, bob.ID, reward.ID, 0)
	s.ContributeToCommitment(ctx, alice.ID, a.ID, 200)
	s.ContributeToCommitment(ctx, bob.ID, b.ID, 100)

	// Bob hits redeem (any contributor can).
	if _, err := s.RedeemReward(ctx, bob.ID, reward.ID); err != nil {
		t.Fatalf("RedeemReward: %v", err)
	}

	// Each kid loses exactly their contribution from total balance, spendable
	// unchanged net (saved → spent in one tx).
	if bal, _ := s.GetPointBalance(ctx, alice.ID); bal != 50 {
		t.Errorf("expected alice spendable 50 (250 seed - 200 contributed), got %d", bal)
	}
	if bal, _ := s.GetPointBalance(ctx, bob.ID); bal != 150 {
		t.Errorf("expected bob spendable 150 (250 seed - 100 contributed), got %d", bal)
	}

	pool, _ := s.GetSharedPool(ctx, *a.SharedPoolID)
	if pool.Status != model.CommitmentRedeemed {
		t.Errorf("expected pool redeemed, got %s", pool.Status)
	}

	// Both kids should have a redemption history entry sized to their share.
	aliceHist, _ := s.ListRedemptionsForUser(ctx, alice.ID, 10)
	if len(aliceHist) != 1 || aliceHist[0].PointsSpent != 200 {
		t.Errorf("expected alice redemption=200, got %+v", aliceHist)
	}
	bobHist, _ := s.ListRedemptionsForUser(ctx, bob.ID, 10)
	if len(bobHist) != 1 || bobHist[0].PointsSpent != 100 {
		t.Errorf("expected bob redemption=100, got %+v", bobHist)
	}
}

func TestSharedPoolRedeemFailsWhenNotFunded(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	alice := createTestUser(t, s, "Alice", "child")
	reward := &model.Reward{Name: "Big thing", Cost: 1000, Active: true, Shareable: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, reward)
	s.AdminAdjustPoints(ctx, alice.ID, 500, "seed")

	a, _ := s.CreateCommitment(ctx, alice.ID, reward.ID, 0)
	s.ContributeToCommitment(ctx, alice.ID, a.ID, 200)

	if _, err := s.RedeemReward(ctx, alice.ID, reward.ID); err == nil {
		t.Errorf("expected redemption to fail when pool isn't fully funded")
	}
}

func TestSharedPoolBreakRefundsOnlyOwnContribution(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	alice := createTestUser(t, s, "Alice", "child")
	bob := createTestUser(t, s, "Bob", "child")
	reward := &model.Reward{Name: "Game", Cost: 200, Active: true, Shareable: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, reward)
	s.AdminAdjustPoints(ctx, alice.ID, 100, "seed")
	s.AdminAdjustPoints(ctx, bob.ID, 100, "seed")

	a, _ := s.CreateCommitment(ctx, alice.ID, reward.ID, 0)
	b, _ := s.CreateCommitment(ctx, bob.ID, reward.ID, 0)
	s.ContributeToCommitment(ctx, alice.ID, a.ID, 60)
	s.ContributeToCommitment(ctx, bob.ID, b.ID, 40)

	if err := s.BreakCommitment(ctx, alice.ID, a.ID); err != nil {
		t.Fatalf("Alice break: %v", err)
	}
	if bal, _ := s.GetPointBalance(ctx, alice.ID); bal != 100 {
		t.Errorf("expected alice refunded to 100, got %d", bal)
	}
	if bal, _ := s.GetPointBalance(ctx, bob.ID); bal != 60 {
		t.Errorf("expected bob still committed 40 (spendable 60), got %d", bal)
	}
	// Pool should still be active so Bob (and others) can keep saving.
	pool, _ := s.GetSharedPool(ctx, *a.SharedPoolID)
	if pool.Status != model.CommitmentActive {
		t.Errorf("expected pool to remain active after one kid leaves, got %s", pool.Status)
	}
	if pool.AmountSaved != 40 {
		t.Errorf("expected pool to retain bob's 40, got %d", pool.AmountSaved)
	}
}

func TestSharedPoolAutoContributeStacksWithPersonal(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	kid := createTestUser(t, s, "Kid", "child")

	personal := &model.Reward{Name: "LEGO", Cost: 200, Active: true, CreatedBy: parent.ID}
	family := &model.Reward{Name: "Minecraft", Cost: 600, Active: true, Shareable: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, personal)
	s.CreateReward(ctx, family)

	if _, err := s.CreateCommitment(ctx, kid.ID, personal.ID, 50); err != nil {
		t.Fatalf("personal commitment: %v", err)
	}
	if _, err := s.CreateCommitment(ctx, kid.ID, family.ID, 25); err != nil {
		t.Fatalf("shared commitment: %v", err)
	}

	chore := createTestChore(t, s, "Big chore", 100, parent.ID)
	cs := createTestSchedule(t, s, chore.ID, kid.ID, 1)
	cc := &model.ChoreCompletion{ChoreScheduleID: cs.ID, CompletedBy: kid.ID, Status: model.StatusApproved, CompletionDate: "2026-05-06"}
	s.CompleteChore(ctx, cc)
	if err := s.CreditChorePoints(ctx, kid.ID, cc.ID, 100); err != nil {
		t.Fatalf("CreditChorePoints: %v", err)
	}

	// Expect 50 to personal, 25 to shared, 25 left spendable.
	bal, _ := s.GetPointBalance(ctx, kid.ID)
	if bal != 25 {
		t.Errorf("expected spendable 25 (100 credit - 50 personal - 25 shared), got %d", bal)
	}

	commitments, err := s.ListActiveCommitmentsForUser(ctx, kid.ID)
	if err != nil {
		t.Fatalf("ListActiveCommitmentsForUser: %v", err)
	}
	if len(commitments) != 2 {
		t.Fatalf("expected 2 active commitments, got %d", len(commitments))
	}
	var personalSaved, sharedSaved int
	for _, c := range commitments {
		if c.SharedPoolID == nil {
			personalSaved = c.AmountSaved
		} else {
			sharedSaved = c.AmountSaved
		}
	}
	if personalSaved != 50 {
		t.Errorf("expected personal saved=50, got %d", personalSaved)
	}
	if sharedSaved != 25 {
		t.Errorf("expected shared saved=25, got %d", sharedSaved)
	}
}

// Bob's dashboard fetches via ListActiveCommitmentsForUser(Bob), which
// embeds the pool. Verify Alice's contribution is reflected in Bob's view
// without Bob having to do anything — this is the path the bug report
// implicates.
func TestListActiveCommitmentsForUserSeesSiblingsContributions(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	alice := createTestUser(t, s, "Alice", "child")
	bob := createTestUser(t, s, "Bob", "child")

	reward := &model.Reward{Name: "Minecraft", Cost: 600, Active: true, Shareable: true, CreatedBy: parent.ID}
	if err := s.CreateReward(ctx, reward); err != nil {
		t.Fatalf("CreateReward: %v", err)
	}
	s.AdminAdjustPoints(ctx, alice.ID, 500, "seed")
	s.AdminAdjustPoints(ctx, bob.ID, 500, "seed")

	a, _ := s.CreateCommitment(ctx, alice.ID, reward.ID, 0)
	b, _ := s.CreateCommitment(ctx, bob.ID, reward.ID, 0)

	// Alice contributes first.
	if err := s.ContributeToCommitment(ctx, alice.ID, a.ID, 300); err != nil {
		t.Fatalf("Alice contribute: %v", err)
	}

	// Bob loads his dashboard.
	bobCommitments, err := s.ListActiveCommitmentsForUser(ctx, bob.ID)
	if err != nil {
		t.Fatalf("ListActiveCommitmentsForUser(Bob): %v", err)
	}
	if len(bobCommitments) != 1 {
		t.Fatalf("expected 1 commitment, got %d", len(bobCommitments))
	}
	bobsView := bobCommitments[0]
	if bobsView.Pool == nil {
		t.Fatalf("expected pool populated on Bob's shared share")
	}
	if bobsView.Pool.AmountSaved != 300 {
		t.Errorf("expected pool.amount_saved=300 (Alice's contribution), got %d", bobsView.Pool.AmountSaved)
	}
	if len(bobsView.Pool.Contributors) != 2 {
		t.Errorf("expected 2 contributors visible to Bob, got %d", len(bobsView.Pool.Contributors))
	}
	var aliceContrib, bobContrib int
	for _, c := range bobsView.Pool.Contributors {
		if c.UserID == alice.ID {
			aliceContrib = c.AmountSaved
		}
		if c.UserID == bob.ID {
			bobContrib = c.AmountSaved
		}
	}
	if aliceContrib != 300 {
		t.Errorf("expected Alice's contribution=300 visible to Bob, got %d", aliceContrib)
	}
	if bobContrib != 0 {
		t.Errorf("expected Bob's contribution=0 (he hasn't put any in yet), got %d", bobContrib)
	}

	// Bob's own AmountSaved on the row should still be 0 (he hasn't contributed).
	if bobsView.AmountSaved != 0 {
		t.Errorf("expected Bob's row amount_saved=0, got %d", bobsView.AmountSaved)
	}

	// Now Bob contributes — pool total should be 500, and Alice's view via
	// ListActiveCommitmentsForUser should reflect Bob's contribution too.
	if err := s.ContributeToCommitment(ctx, bob.ID, b.ID, 200); err != nil {
		t.Fatalf("Bob contribute: %v", err)
	}
	aliceCommitments, err := s.ListActiveCommitmentsForUser(ctx, alice.ID)
	if err != nil {
		t.Fatalf("ListActiveCommitmentsForUser(Alice): %v", err)
	}
	if aliceCommitments[0].Pool.AmountSaved != 500 {
		t.Errorf("expected Alice to see pool=500, got %d", aliceCommitments[0].Pool.AmountSaved)
	}
}

// Toggling shareable on AFTER kids have already committed personally must
// migrate those commitments into a fresh shared pool. Otherwise siblings end
// up in disconnected silos (the real-world bug: parent flipped the flag
// after kids had been saving toward Minecraft individually).
func TestUpdateRewardShareableTogglesMergesExistingCommits(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	alice := createTestUser(t, s, "Alice", "child")
	bob := createTestUser(t, s, "Bob", "child")

	reward := &model.Reward{Name: "Game", Cost: 200, Active: true, CreatedBy: parent.ID}
	if err := s.CreateReward(ctx, reward); err != nil {
		t.Fatalf("CreateReward: %v", err)
	}
	s.AdminAdjustPoints(ctx, alice.ID, 100, "seed")
	s.AdminAdjustPoints(ctx, bob.ID, 100, "seed")

	// Both kids start saving personally.
	a, _ := s.CreateCommitment(ctx, alice.ID, reward.ID, 0)
	b, _ := s.CreateCommitment(ctx, bob.ID, reward.ID, 0)
	s.ContributeToCommitment(ctx, alice.ID, a.ID, 40)
	s.ContributeToCommitment(ctx, bob.ID, b.ID, 30)

	// Parent toggles shareable on.
	reward.Shareable = true
	if err := s.UpdateReward(ctx, reward); err != nil {
		t.Fatalf("UpdateReward shareable=true: %v", err)
	}

	// Both commitments should now reference the same shared pool.
	commitsA, _ := s.ListActiveCommitmentsForUser(ctx, alice.ID)
	commitsB, _ := s.ListActiveCommitmentsForUser(ctx, bob.ID)
	if len(commitsA) != 1 || commitsA[0].SharedPoolID == nil {
		t.Fatalf("expected Alice's commitment migrated to a pool, got %+v", commitsA)
	}
	if len(commitsB) != 1 || commitsB[0].SharedPoolID == nil {
		t.Fatalf("expected Bob's commitment migrated to a pool, got %+v", commitsB)
	}
	if *commitsA[0].SharedPoolID != *commitsB[0].SharedPoolID {
		t.Errorf("expected both kids in same pool after toggle, got %d vs %d", *commitsA[0].SharedPoolID, *commitsB[0].SharedPoolID)
	}

	// The pool should reflect both kids' prior savings (40 + 30 = 70).
	pool, _ := s.GetSharedPool(ctx, *commitsA[0].SharedPoolID)
	if pool.AmountSaved != 70 {
		t.Errorf("expected pool to retain both kids' savings = 70, got %d", pool.AmountSaved)
	}
	if len(pool.Contributors) != 2 {
		t.Errorf("expected 2 contributors after migration, got %d", len(pool.Contributors))
	}
}

func TestUpdateRewardShareableOffRefusesWhenPoolHasContributors(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	alice := createTestUser(t, s, "Alice", "child")
	reward := &model.Reward{Name: "Trip", Cost: 100, Active: true, Shareable: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, reward)
	s.AdminAdjustPoints(ctx, alice.ID, 50, "seed")
	a, _ := s.CreateCommitment(ctx, alice.ID, reward.ID, 0)
	s.ContributeToCommitment(ctx, alice.ID, a.ID, 25)

	reward.Shareable = false
	if err := s.UpdateReward(ctx, reward); err == nil {
		t.Errorf("expected toggle-off to refuse while pool has active contributors")
	}
}

func TestAutoContribute100PercentPersonal(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	kid := createTestUser(t, s, "Kid", "child")
	reward := &model.Reward{Name: "LEGO", Cost: 200, Active: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, reward)
	if _, err := s.CreateCommitment(ctx, kid.ID, reward.ID, 100); err != nil {
		t.Fatalf("CreateCommitment: %v", err)
	}

	chore := createTestChore(t, s, "Big chore", 30, parent.ID)
	cs := createTestSchedule(t, s, chore.ID, kid.ID, 1)
	cc := &model.ChoreCompletion{ChoreScheduleID: cs.ID, CompletedBy: kid.ID, Status: model.StatusApproved, CompletionDate: "2026-05-06"}
	if err := s.CompleteChore(ctx, cc); err != nil {
		t.Fatalf("CompleteChore: %v", err)
	}
	if err := s.CreditChorePoints(ctx, kid.ID, cc.ID, 30); err != nil {
		t.Fatalf("CreditChorePoints: %v", err)
	}

	bal, _ := s.GetPointBalance(ctx, kid.ID)
	if bal != 0 {
		t.Errorf("expected spendable 0 (all 30 auto-saved at 100%%), got %d", bal)
	}
	got, _ := s.GetActiveCommitmentForUser(ctx, kid.ID)
	if got == nil || got.AmountSaved != 30 {
		t.Errorf("expected 30 auto-saved on goal, got %+v", got)
	}
}

// Mirror the actual UI flow: create commitment at 0% (default), then bump
// auto-contribute to 100% via SetCommitmentAutoContributePercent (the slider
// path). Then complete a chore. The bug report claims this doesn't auto-save.
func TestAutoContributeAfterSliderUpdate(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	parent := createTestUser(t, s, "Parent", "admin")
	kid := createTestUser(t, s, "Kid", "child")
	reward := &model.Reward{Name: "LEGO", Cost: 200, Active: true, CreatedBy: parent.ID}
	s.CreateReward(ctx, reward)
	c, err := s.CreateCommitment(ctx, kid.ID, reward.ID, 0)
	if err != nil {
		t.Fatalf("CreateCommitment: %v", err)
	}

	if err := s.SetCommitmentAutoContributePercent(ctx, kid.ID, c.ID, 100); err != nil {
		t.Fatalf("SetCommitmentAutoContributePercent: %v", err)
	}

	chore := createTestChore(t, s, "Big chore", 25, parent.ID)
	cs := createTestSchedule(t, s, chore.ID, kid.ID, 1)
	cc := &model.ChoreCompletion{ChoreScheduleID: cs.ID, CompletedBy: kid.ID, Status: model.StatusApproved, CompletionDate: "2026-05-06"}
	if err := s.CompleteChore(ctx, cc); err != nil {
		t.Fatalf("CompleteChore: %v", err)
	}
	if err := s.CreditChorePoints(ctx, kid.ID, cc.ID, 25); err != nil {
		t.Fatalf("CreditChorePoints: %v", err)
	}

	bal, _ := s.GetPointBalance(ctx, kid.ID)
	if bal != 0 {
		t.Errorf("expected spendable 0 after 100%% auto-save of 25 pts, got %d", bal)
	}
	got, _ := s.GetActiveCommitmentForUser(ctx, kid.ID)
	if got == nil || got.AmountSaved != 25 {
		t.Errorf("expected 25 auto-saved, got %+v", got)
	}
}
