package api_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

type pointsView struct {
	Balance      int              `json:"balance"`
	Committed    int              `json:"committed"`
	Transactions []map[string]any `json:"transactions"`
}

func getPoints(t *testing.T, env *testEnv, kidID int) pointsView {
	t.Helper()
	resp := env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/points", kidID), nil, adminHeaders(), http.StatusOK)
	var pv pointsView
	decodeBody(t, resp, &pv)
	return pv
}

// newest returns the newest visible history row with this reason, or nil.
func (pv pointsView) newest(reason string) map[string]any {
	for _, tx := range pv.Transactions {
		if tx["reason"] == reason {
			return tx
		}
	}
	return nil
}

func (pv pointsView) reasons() []string {
	var out []string
	for _, tx := range pv.Transactions {
		out = append(out, fmt.Sprintf("%v %v", tx["reason"], tx["amount"]))
	}
	return out
}

func undoURL(tx map[string]any) string {
	return fmt.Sprintf("/api/points/transactions/%d/undo", int64(tx["id"].(float64)))
}

func seedPoints(t *testing.T, env *testEnv, kidID, amount int) {
	t.Helper()
	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{
		"user_id": kidID, "amount": amount, "note": "seed",
	}, adminHeaders(), http.StatusNoContent)
}

// Undoing a late penalty gives the points back once and both rows leave the
// history; unchecking the chore afterwards doesn't refund it again.
func TestUndoExpiryPenalty(t *testing.T) {
	ensureAfterMidnight(t)
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")
	seedPoints(t, env, kidID, 50)

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Feed the dog", "category": "bonus", "points_value": 10,
	}, adminHeaders())
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID, "day_of_week": int(time.Now().Weekday()),
		"due_by": "00:01", "expiry_penalty": "penalty", "expiry_penalty_value": 5,
	}, adminHeaders())
	today := time.Now().Format(model.DateFormat)
	env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by": kidID, "completion_date": today,
	}, adminHeaders(), http.StatusCreated)

	pv := getPoints(t, env, kidID)
	penalty := pv.newest(model.ReasonExpiryPenalty)
	if pv.Balance != 45 || penalty == nil {
		t.Fatalf("after late completion: balance %d, history %v", pv.Balance, pv.reasons())
	}

	env.expectStatus(t, "POST", undoURL(penalty), nil, adminHeaders(), http.StatusNoContent)
	pv = getPoints(t, env, kidID)
	if pv.Balance != 50 {
		t.Fatalf("after undo: balance %d, want 50", pv.Balance)
	}
	if pv.newest(model.ReasonExpiryPenalty) != nil || len(pv.Transactions) != 1 {
		t.Fatalf("undone penalty should leave the history, got %v", pv.reasons())
	}

	env.expectStatus(t, "POST", undoURL(penalty), nil, adminHeaders(), http.StatusConflict)

	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/schedules/1/complete?date=%s", today), nil, adminHeaders(), http.StatusNoContent)
	if pv = getPoints(t, env, kidID); pv.Balance != 50 {
		t.Fatalf("after unchecking: balance %d, want 50", pv.Balance)
	}
}

// An undone missed-chore penalty isn't charged again by the checker, and
// excusing the chore doesn't refund it a second time (in either order).
func TestUndoMissedChorePenalty(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")
	today := time.Now().Format(model.DateFormat)
	ctx := context.Background()
	seedPoints(t, env, kidID, 20)

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Clean Garage", "category": "required", "points_value": 10, "missed_penalty_value": 5,
	}, adminHeaders())
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID, "specific_date": today,
	}, adminHeaders())
	if err := env.store.DebitMissedChore(ctx, int64(kidID), 1, 5, today); err != nil {
		t.Fatal(err)
	}
	penalty := getPoints(t, env, kidID).newest(model.ReasonMissedChore)
	if penalty == nil {
		t.Fatal("expected a missed-chore penalty")
	}
	env.expectStatus(t, "POST", undoURL(penalty), nil, adminHeaders(), http.StatusNoContent)

	if err := env.store.DebitMissedChore(ctx, int64(kidID), 1, 5, today); err != nil {
		t.Fatal(err)
	}
	env.expectStatus(t, "POST", "/api/schedules/1/excuse", map[string]any{
		"date": today, "reason": "sick",
	}, adminHeaders(), http.StatusCreated)
	if pv := getPoints(t, env, kidID); pv.Balance != 20 || len(pv.Transactions) != 1 {
		t.Fatalf("after undo, re-check and excuse: balance %d, history %v; want 20 and only the seed", pv.Balance, pv.reasons())
	}

	// Excused first: the refund hides the penalty, and Undo is refused.
	if err := env.store.DebitMissedChore(ctx, int64(kidID), 1, 5, "2000-01-01"); err != nil {
		t.Fatal(err)
	}
	old := getPoints(t, env, kidID).newest(model.ReasonMissedChore)
	env.expectStatus(t, "POST", "/api/schedules/1/excuse", map[string]any{
		"date": "2000-01-01", "reason": "sick",
	}, adminHeaders(), http.StatusCreated)
	if pv := getPoints(t, env, kidID); pv.Balance != 20 || len(pv.Transactions) != 1 {
		t.Fatalf("after excusing: balance %d, history %v; want 20 and only the seed", pv.Balance, pv.reasons())
	}
	env.expectStatus(t, "POST", undoURL(old), nil, adminHeaders(), http.StatusConflict)
}

// Undoing a manual adjustment removes it without an "Undo:" row.
func TestUndoAdjustment(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")
	seedPoints(t, env, kidID, 20)
	seedPoints(t, env, kidID, -5)

	pv := getPoints(t, env, kidID)
	adj := pv.Transactions[0]
	env.expectStatus(t, "POST", undoURL(adj), nil, adminHeaders(), http.StatusNoContent)
	pv = getPoints(t, env, kidID)
	if pv.Balance != 20 || len(pv.Transactions) != 1 {
		t.Fatalf("balance %d, history %v; want 20 and only the seed", pv.Balance, pv.reasons())
	}
	env.expectStatus(t, "POST", undoURL(adj), nil, adminHeaders(), http.StatusConflict)
	env.expectStatus(t, "POST", undoURL(adj), nil, childHeaders(kidID), http.StatusForbidden)
	env.expectStatus(t, "POST", "/api/points/transactions/99999/undo", nil, adminHeaders(), http.StatusNotFound)
}

// Undoing chore points unchecks the chore, which also returns its auto-save;
// every row for it leaves the history. Checking it again brings them back.
func TestUndoChorePointsUnchecksChore(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Dishes", "category": "required", "points_value": 10,
	}, adminHeaders())
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID, "day_of_week": int(time.Now().Weekday()),
	}, adminHeaders())
	env.request(t, "POST", "/api/rewards", map[string]any{"name": "Toy", "cost": 100}, adminHeaders())
	env.expectStatus(t, "POST", "/api/rewards/1/commit", map[string]any{
		"auto_contribute_percent": 50,
	}, childHeaders(kidID), http.StatusCreated)
	today := time.Now().Format(model.DateFormat)
	env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by": kidID, "completion_date": today,
	}, adminHeaders(), http.StatusCreated)

	pv := getPoints(t, env, kidID)
	if pv.Balance != 5 || pv.Committed != 5 || len(pv.Transactions) != 2 {
		t.Fatalf("after completing: balance %d, saved %d, history %v", pv.Balance, pv.Committed, pv.reasons())
	}
	credit := pv.newest(model.ReasonChoreComplete)
	env.expectStatus(t, "POST", undoURL(credit), nil, adminHeaders(), http.StatusNoContent)

	pv = getPoints(t, env, kidID)
	if pv.Balance != 0 || pv.Committed != 0 || len(pv.Transactions) != 0 {
		t.Fatalf("after undo: balance %d, saved %d, history %v; want all zero and empty", pv.Balance, pv.Committed, pv.reasons())
	}
	env.expectStatus(t, "POST", undoURL(credit), nil, adminHeaders(), http.StatusConflict)

	// The chore is unchecked: checking it again pays out again.
	env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by": kidID, "completion_date": today,
	}, adminHeaders(), http.StatusCreated)
	if pv = getPoints(t, env, kidID); pv.Balance != 5 || pv.Committed != 5 || len(pv.Transactions) != 2 {
		t.Fatalf("after rechecking: balance %d, saved %d, history %v", pv.Balance, pv.Committed, pv.reasons())
	}
}

// Undoing a goal saving puts the points back in the balance and out of the
// goal; unchecking the chore later doesn't return the auto-save twice.
func TestUndoGoalSaving(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")

	env.request(t, "POST", "/api/chores", map[string]any{
		"title": "Dishes", "category": "required", "points_value": 10,
	}, adminHeaders())
	env.request(t, "POST", "/api/chores/1/schedules", map[string]any{
		"assigned_to": kidID, "day_of_week": int(time.Now().Weekday()),
	}, adminHeaders())
	env.request(t, "POST", "/api/rewards", map[string]any{"name": "Toy", "cost": 100}, adminHeaders())
	env.expectStatus(t, "POST", "/api/rewards/1/commit", map[string]any{
		"auto_contribute_percent": 100,
	}, childHeaders(kidID), http.StatusCreated)
	today := time.Now().Format(model.DateFormat)
	env.expectStatus(t, "POST", "/api/schedules/1/complete", map[string]any{
		"completed_by": kidID, "completion_date": today,
	}, adminHeaders(), http.StatusCreated)

	saving := getPoints(t, env, kidID).newest(model.ReasonCommitToGoal)
	if saving == nil {
		t.Fatal("expected an auto-save row")
	}
	env.expectStatus(t, "POST", undoURL(saving), nil, adminHeaders(), http.StatusNoContent)
	pv := getPoints(t, env, kidID)
	if pv.Balance != 10 || pv.Committed != 0 || len(pv.Transactions) != 1 {
		t.Fatalf("after undoing the saving: balance %d, saved %d, history %v", pv.Balance, pv.Committed, pv.reasons())
	}

	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/schedules/1/complete?date=%s", today), nil, adminHeaders(), http.StatusNoContent)
	pv = getPoints(t, env, kidID)
	if pv.Balance != 0 || pv.Committed != 0 {
		t.Fatalf("after unchecking: balance %d, saved %d; want 0 and 0", pv.Balance, pv.Committed)
	}
}

// Rows that are themselves an undo can't be undone.
func TestUndoRefusesUndoRows(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")
	env.request(t, "POST", "/api/rewards", map[string]any{"name": "Toy", "cost": 100}, adminHeaders())
	seedPoints(t, env, kidID, 10)
	env.expectStatus(t, "POST", "/api/rewards/1/commit", nil, childHeaders(kidID), http.StatusCreated)
	env.expectStatus(t, "POST", "/api/commitments/1/contribute", map[string]any{"amount": 10}, childHeaders(kidID), http.StatusOK)
	env.expectStatus(t, "DELETE", "/api/commitments/1", nil, childHeaders(kidID), http.StatusNoContent)

	pv := getPoints(t, env, kidID)
	returned := pv.newest(model.ReasonGoalBreak)
	saved := pv.newest(model.ReasonCommitToGoal)
	if returned == nil || saved == nil {
		t.Fatalf("expected saved and returned rows, got %v", pv.reasons())
	}
	env.expectStatus(t, "POST", undoURL(returned), nil, adminHeaders(), http.StatusUnprocessableEntity)
	// The goal was cancelled, so its saving can't be taken back out.
	env.expectStatus(t, "POST", undoURL(saved), nil, adminHeaders(), http.StatusUnprocessableEntity)
}
