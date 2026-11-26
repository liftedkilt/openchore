package store_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

// --- Empty data tests ---

func TestReportKidSummaries_Empty(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	rows, err := s.ReportKidSummaries(ctx, "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("ReportKidSummaries: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows with no children, got %d", len(rows))
	}
}

func TestReportMostMissed_Empty(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	rows, err := s.ReportMostMissed(ctx, "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("ReportMostMissed: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}
}

func TestReportCompletionTrend_Empty(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	rows, err := s.ReportCompletionTrend(ctx, "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("ReportCompletionTrend: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}
}

func TestReportCategoryBreakdown_Empty(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	rows, err := s.ReportCategoryBreakdown(ctx, "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("ReportCategoryBreakdown: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}
}

func TestReportPointsSummary_Empty(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	rows, err := s.ReportPointsSummary(ctx, "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("ReportPointsSummary: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows with no children, got %d", len(rows))
	}
}

func TestReportDayOfWeek_Empty(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	rows, err := s.ReportDayOfWeek(ctx, "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("ReportDayOfWeek: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}
}

// --- Tests with data ---
// Note: DB constraint allows status IN ('pending', 'approved', 'rejected').
// Report queries treat 'approved' as completed and 'rejected' as missed.

func TestReportKidSummaries_WithData(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	now := time.Now()
	today := now.Format("2006-01-02")
	startOfMonth := now.Format("2006-01") + "-01"
	endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

	admin := createTestUser(t, s, "Admin", "admin")
	kid1 := createTestUser(t, s, "Alice", "child")
	kid2 := createTestUser(t, s, "Bob", "child")

	chore1 := createTestChore(t, s, "Dishes", 10, admin.ID)

	sched1k1 := createTestSchedule(t, s, chore1.ID, kid1.ID, 1)
	_ = createTestSchedule(t, s, chore1.ID, kid2.ID, 1)

	// Alice approved (counts as completed in reports)
	cc1 := &model.ChoreCompletion{
		ChoreScheduleID: sched1k1.ID,
		CompletedBy:     kid1.ID,
		Status:          "approved",
		CompletionDate:  today,
	}
	if err := s.CompleteChore(ctx, cc1); err != nil {
		t.Fatalf("CompleteChore: %v", err)
	}

	// Credit points for Alice
	if err := s.CreditChorePoints(ctx, kid1.ID, cc1.ID, 10); err != nil {
		t.Fatalf("CreditChorePoints: %v", err)
	}

	rows, err := s.ReportKidSummaries(ctx, startOfMonth, endOfMonth)
	if err != nil {
		t.Fatalf("ReportKidSummaries: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 kids, got %d", len(rows))
	}

	// Rows are ordered by name: Alice, Bob
	alice := rows[0]
	if alice.Name != "Alice" {
		t.Errorf("expected Alice first, got %s", alice.Name)
	}
	if alice.TotalCompleted != 1 {
		t.Errorf("Alice completed = %d, want 1", alice.TotalCompleted)
	}
	if alice.TotalAssigned != 1 {
		t.Errorf("Alice assigned = %d, want 1", alice.TotalAssigned)
	}
	if alice.PointsEarned != 10 {
		t.Errorf("Alice points = %d, want 10", alice.PointsEarned)
	}

	bob := rows[1]
	if bob.Name != "Bob" {
		t.Errorf("expected Bob second, got %s", bob.Name)
	}
	if bob.TotalCompleted != 0 {
		t.Errorf("Bob completed = %d, want 0", bob.TotalCompleted)
	}
	// Bob has no completions at all, so assigned = completed + missed = 0
	if bob.TotalAssigned != 0 {
		t.Errorf("Bob assigned = %d, want 0", bob.TotalAssigned)
	}
}

func TestReportKidSummaries_ExcludesAdmins(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	createTestUser(t, s, "Admin", "admin")
	createTestUser(t, s, "Alice", "child")

	rows, err := s.ReportKidSummaries(ctx, "2026-01-01", "2026-12-31")
	if err != nil {
		t.Fatalf("ReportKidSummaries: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 child row, got %d", len(rows))
	}
	if rows[0].Name != "Alice" {
		t.Errorf("expected Alice, got %s", rows[0].Name)
	}
}

func TestReportMostMissed_WithRejectedCompletions(t *testing.T) {
	// Rejected completions count as "missed" in reports.
	s := setupStore(t)
	ctx := context.Background()

	admin := createTestUser(t, s, "Admin", "admin")
	kid := createTestUser(t, s, "Alice", "child")
	chore := createTestChore(t, s, "Dishes", 10, admin.ID)
	sched := createTestSchedule(t, s, chore.ID, kid.ID, 1)

	cc := &model.ChoreCompletion{
		ChoreScheduleID: sched.ID,
		CompletedBy:     kid.ID,
		Status:          "rejected",
		CompletionDate:  "2026-03-23",
	}
	if err := s.CompleteChore(ctx, cc); err != nil {
		t.Fatalf("CompleteChore: %v", err)
	}

	rows, err := s.ReportMostMissed(ctx, "2026-03-01", "2026-03-31")
	if err != nil {
		t.Fatalf("ReportMostMissed: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 missed chore row, got %d", len(rows))
	}
	if rows[0].ChoreName != "Dishes" {
		t.Errorf("expected chore name Dishes, got %s", rows[0].ChoreName)
	}
	if rows[0].MissCount != 1 {
		t.Errorf("expected miss count 1, got %d", rows[0].MissCount)
	}
}

func TestReportCompletionTrend_WithData(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	admin := createTestUser(t, s, "Admin", "admin")
	kid := createTestUser(t, s, "Alice", "child")
	chore := createTestChore(t, s, "Dishes", 10, admin.ID)
	sched := createTestSchedule(t, s, chore.ID, kid.ID, 1)

	// Day 1: 1 approved (counts as completed)
	cc1 := &model.ChoreCompletion{
		ChoreScheduleID: sched.ID, CompletedBy: kid.ID,
		Status: "approved", CompletionDate: "2026-03-23",
	}
	if err := s.CompleteChore(ctx, cc1); err != nil {
		t.Fatalf("CompleteChore: %v", err)
	}

	// Day 2: 1 rejected (does not count as completed)
	sched2 := createTestSchedule(t, s, chore.ID, kid.ID, 2)
	cc2 := &model.ChoreCompletion{
		ChoreScheduleID: sched2.ID, CompletedBy: kid.ID,
		Status: "rejected", CompletionDate: "2026-03-24",
	}
	if err := s.CompleteChore(ctx, cc2); err != nil {
		t.Fatalf("CompleteChore: %v", err)
	}

	rows, err := s.ReportCompletionTrend(ctx, "2026-03-23", "2026-03-24")
	if err != nil {
		t.Fatalf("ReportCompletionTrend: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 trend days, got %d", len(rows))
	}

	// Day 1: approved counts as completed
	if !strings.HasPrefix(rows[0].Date, "2026-03-23") {
		t.Errorf("row 0 date = %s, want prefix 2026-03-23", rows[0].Date)
	}
	if rows[0].Completed != 1 {
		t.Errorf("row 0 completed = %d, want 1", rows[0].Completed)
	}
	if rows[0].Assigned != 1 {
		t.Errorf("row 0 assigned = %d, want 1", rows[0].Assigned)
	}

	// Day 2: rejected does not count as completed
	if !strings.HasPrefix(rows[1].Date, "2026-03-24") {
		t.Errorf("row 1 date = %s, want prefix 2026-03-24", rows[1].Date)
	}
	if rows[1].Completed != 0 {
		t.Errorf("row 1 completed = %d, want 0", rows[1].Completed)
	}
	if rows[1].Assigned != 1 {
		t.Errorf("row 1 assigned = %d, want 1", rows[1].Assigned)
	}
}

func TestReportCategoryBreakdown_WithData(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	admin := createTestUser(t, s, "Admin", "admin")
	kid := createTestUser(t, s, "Alice", "child")

	// "required" category chore
	chore1 := createTestChore(t, s, "Dishes", 10, admin.ID) // category = "required"
	// "core" category chore
	chore2 := &model.Chore{
		Title: "Homework", Description: "desc", Category: "core",
		PointsValue: 5, Source: "manual", CreatedBy: admin.ID,
	}
	if err := s.CreateChore(ctx, chore2); err != nil {
		t.Fatalf("CreateChore: %v", err)
	}

	sched1 := createTestSchedule(t, s, chore1.ID, kid.ID, 1)
	sched2 := createTestSchedule(t, s, chore2.ID, kid.ID, 2)

	// Approve the "required" chore (counts as completed)
	cc1 := &model.ChoreCompletion{
		ChoreScheduleID: sched1.ID, CompletedBy: kid.ID,
		Status: "approved", CompletionDate: "2026-03-23",
	}
	if err := s.CompleteChore(ctx, cc1); err != nil {
		t.Fatalf("CompleteChore: %v", err)
	}

	// Reject the "core" chore (does not count as completed)
	cc2 := &model.ChoreCompletion{
		ChoreScheduleID: sched2.ID, CompletedBy: kid.ID,
		Status: "rejected", CompletionDate: "2026-03-24",
	}
	if err := s.CompleteChore(ctx, cc2); err != nil {
		t.Fatalf("CompleteChore: %v", err)
	}

	rows, err := s.ReportCategoryBreakdown(ctx, "2026-03-01", "2026-03-31")
	if err != nil {
		t.Fatalf("ReportCategoryBreakdown: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(rows))
	}

	// Categories sorted alphabetically: core, required
	core := rows[0]
	if core.Category != "core" {
		t.Errorf("expected core first, got %s", core.Category)
	}
	if core.TotalAssigned != 1 || core.TotalCompleted != 0 {
		t.Errorf("core: assigned=%d completed=%d, want 1/0", core.TotalAssigned, core.TotalCompleted)
	}

	required := rows[1]
	if required.Category != "required" {
		t.Errorf("expected required second, got %s", required.Category)
	}
	if required.TotalAssigned != 1 || required.TotalCompleted != 1 {
		t.Errorf("required: assigned=%d completed=%d, want 1/1", required.TotalAssigned, required.TotalCompleted)
	}
}

func TestReportPointsSummary_WithData(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	now := time.Now()
	today := now.Format("2006-01-02")
	startOfMonth := now.Format("2006-01") + "-01"
	endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

	admin := createTestUser(t, s, "Admin", "admin")
	kid := createTestUser(t, s, "Alice", "child")
	chore := createTestChore(t, s, "Dishes", 10, admin.ID)
	sched := createTestSchedule(t, s, chore.ID, kid.ID, 1)

	cc := &model.ChoreCompletion{
		ChoreScheduleID: sched.ID, CompletedBy: kid.ID,
		Status: "approved", CompletionDate: today,
	}
	if err := s.CompleteChore(ctx, cc); err != nil {
		t.Fatalf("CompleteChore: %v", err)
	}

	// Credit points
	if err := s.CreditChorePoints(ctx, kid.ID, cc.ID, 10); err != nil {
		t.Fatalf("CreditChorePoints: %v", err)
	}

	rows, err := s.ReportPointsSummary(ctx, startOfMonth, endOfMonth)
	if err != nil {
		t.Fatalf("ReportPointsSummary: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 kid, got %d", len(rows))
	}

	if rows[0].Name != "Alice" {
		t.Errorf("expected Alice, got %s", rows[0].Name)
	}
	if rows[0].PointsEarned != 10 {
		t.Errorf("points earned = %d, want 10", rows[0].PointsEarned)
	}
	if rows[0].PointsDecayed != 0 {
		t.Errorf("points decayed = %d, want 0", rows[0].PointsDecayed)
	}
	if rows[0].PointsSpent != 0 {
		t.Errorf("points spent = %d, want 0", rows[0].PointsSpent)
	}
}

func TestReportPointsSummary_ChildWithNoTransactions(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	createTestUser(t, s, "Alice", "child")

	rows, err := s.ReportPointsSummary(ctx, "2026-03-01", "2026-03-31")
	if err != nil {
		t.Fatalf("ReportPointsSummary: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 kid, got %d", len(rows))
	}
	if rows[0].PointsEarned != 0 || rows[0].PointsDecayed != 0 || rows[0].PointsSpent != 0 {
		t.Errorf("expected all zeros, got earned=%d decayed=%d spent=%d",
			rows[0].PointsEarned, rows[0].PointsDecayed, rows[0].PointsSpent)
	}
}

func TestReportDayOfWeek_WithData(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	admin := createTestUser(t, s, "Admin", "admin")
	kid := createTestUser(t, s, "Alice", "child")
	chore := createTestChore(t, s, "Dishes", 10, admin.ID)

	sched1 := createTestSchedule(t, s, chore.ID, kid.ID, 1)
	sched2 := createTestSchedule(t, s, chore.ID, kid.ID, 2)

	// Monday: approved (counts as completed)
	cc1 := &model.ChoreCompletion{
		ChoreScheduleID: sched1.ID, CompletedBy: kid.ID,
		Status: "approved", CompletionDate: "2026-03-23", // Monday
	}
	if err := s.CompleteChore(ctx, cc1); err != nil {
		t.Fatalf("CompleteChore: %v", err)
	}

	// Tuesday: rejected (does not count as completed)
	cc2 := &model.ChoreCompletion{
		ChoreScheduleID: sched2.ID, CompletedBy: kid.ID,
		Status: "rejected", CompletionDate: "2026-03-24", // Tuesday
	}
	if err := s.CompleteChore(ctx, cc2); err != nil {
		t.Fatalf("CompleteChore: %v", err)
	}

	rows, err := s.ReportDayOfWeek(ctx, "2026-03-23", "2026-03-24")
	if err != nil {
		t.Fatalf("ReportDayOfWeek: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 days, got %d", len(rows))
	}

	// SQLite %w: 0=Sun, 1=Mon, 2=Tue
	mon := rows[0]
	if mon.DayOfWeek != 1 {
		t.Errorf("row 0 dow = %d, want 1 (Monday)", mon.DayOfWeek)
	}
	if mon.TotalAssigned != 1 || mon.TotalCompleted != 1 {
		t.Errorf("Monday: assigned=%d completed=%d, want 1/1", mon.TotalAssigned, mon.TotalCompleted)
	}

	tue := rows[1]
	if tue.DayOfWeek != 2 {
		t.Errorf("row 1 dow = %d, want 2 (Tuesday)", tue.DayOfWeek)
	}
	if tue.TotalAssigned != 1 || tue.TotalCompleted != 0 {
		t.Errorf("Tuesday: assigned=%d completed=%d, want 1/0", tue.TotalAssigned, tue.TotalCompleted)
	}
}

func TestReportDateRangeFiltering(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	admin := createTestUser(t, s, "Admin", "admin")
	kid := createTestUser(t, s, "Alice", "child")
	chore := createTestChore(t, s, "Dishes", 10, admin.ID)

	sched1 := createTestSchedule(t, s, chore.ID, kid.ID, 1)
	sched2 := createTestSchedule(t, s, chore.ID, kid.ID, 2)

	// Completion inside range
	cc1 := &model.ChoreCompletion{
		ChoreScheduleID: sched1.ID, CompletedBy: kid.ID,
		Status: "approved", CompletionDate: "2026-03-25",
	}
	if err := s.CompleteChore(ctx, cc1); err != nil {
		t.Fatalf("CompleteChore: %v", err)
	}

	// Completion outside range
	cc2 := &model.ChoreCompletion{
		ChoreScheduleID: sched2.ID, CompletedBy: kid.ID,
		Status: "approved", CompletionDate: "2026-04-05",
	}
	if err := s.CompleteChore(ctx, cc2); err != nil {
		t.Fatalf("CompleteChore: %v", err)
	}

	// Query only March
	trend, err := s.ReportCompletionTrend(ctx, "2026-03-01", "2026-03-31")
	if err != nil {
		t.Fatalf("ReportCompletionTrend: %v", err)
	}
	if len(trend) != 1 {
		t.Fatalf("expected 1 trend day in range, got %d", len(trend))
	}
	if !strings.HasPrefix(trend[0].Date, "2026-03-25") {
		t.Errorf("expected date prefix 2026-03-25, got %s", trend[0].Date)
	}
}

// The Points report adds up the same way as a kid's balance plus savings:
// chores in, penalties and decay out, removed penalties cancelled, goal
// savings not counted as lost. Mirrors Gabe's Nov 16-18 history.
func TestReportPointsSummary_NetBreakdown(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	today := time.Now().Format("2006-01-02")

	admin := createTestUser(t, s, "Admin", "admin")
	kid := createTestUser(t, s, "Gabe", "child")
	chore := createTestChore(t, s, "Feed Hondo", 5, admin.ID)
	sched := createTestSchedule(t, s, chore.ID, kid.ID, 1)
	cc := &model.ChoreCompletion{ChoreScheduleID: sched.ID, CompletedBy: kid.ID, Status: "approved", CompletionDate: today}
	if err := s.CompleteChore(ctx, cc); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 11; i++ {
		if err := s.CreditChorePoints(ctx, kid.ID, cc.ID, 5); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 4; i++ {
		if err := s.DebitExpiryPenalty(ctx, kid.ID, cc.ID, 5); err != nil {
			t.Fatal(err)
		}
	}
	// One of the four penalties is removed again.
	txs, err := s.ListPointTransactions(ctx, kid.ID, 50)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReversePointTransaction(ctx, txs[0].ID); err != nil {
		t.Fatal(err)
	}
	// A +5 / -5 pair of manual adjustments cancels out.
	if err := s.AdminAdjustPoints(ctx, kid.ID, 5, "oops"); err != nil {
		t.Fatal(err)
	}
	if err := s.AdminAdjustPoints(ctx, kid.ID, -5, "Undo: oops"); err != nil {
		t.Fatal(err)
	}
	// Saving toward a goal moves points but doesn't lose them.
	r := &model.Reward{Name: "Gift Card", Cost: 50, Active: true, CreatedBy: admin.ID}
	if err := s.CreateReward(ctx, r); err != nil {
		t.Fatal(err)
	}
	c, err := s.CreateCommitment(ctx, kid.ID, r.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ContributeToCommitment(ctx, kid.ID, c.ID, 40); err != nil {
		t.Fatal(err)
	}

	rows, err := s.ReportPointsSummary(ctx, today, today)
	if err != nil {
		t.Fatal(err)
	}
	var got *store.PointsSummaryRow
	for i := range rows {
		if rows[i].UserID == kid.ID {
			got = &rows[i]
		}
	}
	if got == nil {
		t.Fatal("kid missing from report")
	}
	want := store.PointsSummaryRow{UserID: kid.ID, Name: "Gabe", PointsEarned: 55, PointsAdjusted: 0, PointsPenalties: 15, PointsNet: 40}
	if *got != want {
		t.Errorf("got %+v, want %+v", *got, want)
	}
}

// Reports bucket points by the server's local day, like completion dates.
// created_at is stored in UTC, so points earned just after local midnight
// or just before the next one can sit on a different UTC date; both must
// still count toward today. (Before the fix this failed whenever the local
// timezone wasn't UTC: in the evening west of UTC, in the morning east of it.)
func TestReportPoints_UseLocalDay(t *testing.T) {
	s, db := setupStoreWithDB(t)
	ctx := context.Background()
	now := time.Now()
	today := now.Format("2006-01-02")
	// Built from the calendar, not by adding hours to midnight: a
	// daylight-saving day is 23 or 25 hours long.
	at := func(day, hour, min int) time.Time {
		return time.Date(now.Year(), now.Month(), now.Day()+day, hour, min, 0, 0, time.Local)
	}

	kid := createTestUser(t, s, "Gabe", "child")
	for _, tx := range []struct {
		at     time.Time
		amount int
	}{
		{at(0, 0, 1), 5},      // 00:01 today
		{at(0, 23, 59), 5},    // 23:59 today
		{at(-1, 23, 59), 100}, // 23:59 yesterday: not today
	} {
		if _, err := db.Exec(
			`INSERT INTO point_transactions (user_id, amount, reason, created_at) VALUES (?, ?, 'chore_complete', ?)`,
			kid.ID, tx.amount, tx.at.UTC().Format("2006-01-02 15:04:05"),
		); err != nil {
			t.Fatal(err)
		}
	}

	points, err := s.ReportPointsSummary(ctx, today, today)
	if err != nil {
		t.Fatal(err)
	}
	kids, err := s.ReportKidSummaries(ctx, today, today)
	if err != nil {
		t.Fatal(err)
	}
	earned := func(report string, n int, row func(i int) (int64, int)) {
		t.Helper()
		for i := 0; i < n; i++ {
			if id, pts := row(i); id == kid.ID {
				if pts != 10 {
					t.Errorf("%s earned = %d, want 10", report, pts)
				}
				return
			}
		}
		t.Errorf("%s has no row for the kid", report)
	}
	earned("ReportPointsSummary", len(points), func(i int) (int64, int) { return points[i].UserID, points[i].PointsEarned })
	earned("ReportKidSummaries", len(kids), func(i int) (int64, int) { return kids[i].UserID, kids[i].PointsEarned })
}

// A first-come chore done once counts once in the reports, for whoever did
// it: not again for the 0-point copies it makes on everyone else's schedule.
func TestReports_FCFSCopiesCountOnce(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	today := time.Now().Format("2006-01-02")

	admin := createTestUser(t, s, "Admin", "admin")
	alice := createTestUser(t, s, "Alice", "child")
	bob := createTestUser(t, s, "Bob", "child")
	cara := createTestUser(t, s, "Cara", "child")
	chore := createTestChore(t, s, "Empty dryer", 10, admin.ID)
	group := "g1"
	var aliceSched int64
	for _, kid := range []*model.User{alice, bob, cara} {
		cs := &model.ChoreSchedule{ChoreID: chore.ID, AssignedTo: kid.ID, AssignmentType: model.AssignmentFCFS,
			FcfsGroupID: &group, SpecificDate: &today, PointsMultiplier: 1, ExpiryPenalty: "none"}
		if err := s.CreateSchedule(ctx, cs); err != nil {
			t.Fatal(err)
		}
		if kid == alice {
			aliceSched = cs.ID
		}
	}
	if err := s.CompleteChore(ctx, &model.ChoreCompletion{ChoreScheduleID: aliceSched, CompletedBy: alice.ID, Status: "approved", CompletionDate: today}); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteFCFSSiblings(ctx, group, alice.ID, aliceSched, today); err != nil {
		t.Fatal(err)
	}

	kids, err := s.ReportKidSummaries(ctx, today, today)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range kids {
		want := 0
		if r.UserID == alice.ID {
			want, found = 1, true
		}
		if r.TotalCompleted != want {
			t.Errorf("%s TotalCompleted = %d, want %d", r.Name, r.TotalCompleted, want)
		}
	}
	if !found {
		t.Error("the kid summary has no row for Alice")
	}
	trend, err := s.ReportCompletionTrend(ctx, today, today)
	if err != nil {
		t.Fatal(err)
	}
	if len(trend) != 1 || trend[0].Completed != 1 || trend[0].Assigned != 1 {
		t.Errorf("trend = %+v, want one day with 1 completed of 1", trend)
	}
	cats, err := s.ReportCategoryBreakdown(ctx, today, today)
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 1 || cats[0].TotalCompleted != 1 {
		t.Errorf("categories = %+v, want 1 completed", cats)
	}
	dows, err := s.ReportDayOfWeek(ctx, today, today)
	if err != nil {
		t.Fatal(err)
	}
	if len(dows) != 1 || dows[0].TotalCompleted != 1 {
		t.Errorf("day of week = %+v, want 1 completed", dows)
	}
}
