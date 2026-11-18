package score

import (
	"testing"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

func chore(cat, assign string, completed bool, status string) model.ScheduledChore {
	c := model.ScheduledChore{Category: cat, AssignmentType: assign, Completed: completed}
	if status != "" {
		c.CompletionStatus = &status
	}
	return c
}

func TestSummarizeDay(t *testing.T) {
	chores := []model.ScheduledChore{
		chore(model.CategoryRequired, model.AssignmentIndividual, true, model.StatusApproved),
		chore(model.CategoryCore, model.AssignmentIndividual, true, model.StatusPending),   // waiting counts
		chore(model.CategoryCore, model.AssignmentIndividual, true, model.StatusRejected),  // rejected doesn't
		chore(model.CategoryCore, model.AssignmentIndividual, false, ""),                   // not done
		chore(model.CategoryCore, model.AssignmentIndividual, true, model.StatusExcused),   // dropped
		chore(model.CategoryBonus, model.AssignmentIndividual, true, model.StatusApproved), // bonus
		chore(model.CategoryBonus, model.AssignmentIndividual, false, ""),                  // nothing
		chore(model.CategoryCore, model.AssignmentFamily, true, model.StatusApproved),      // family: bonus
		chore(model.CategoryCore, model.AssignmentFCFS, false, ""),                         // first-come, not mine
		chore(model.CategoryOptional, model.AssignmentIndividual, true, model.StatusApproved),
	}
	got := SummarizeDay("2026-11-16", chores, true)
	if got.Assigned != 4 || got.Done != 2 || got.Bonus != 2 {
		t.Fatalf("finished day = %+v, want 2 of 4, 2 bonus", got)
	}
	// While the day is still running, chores that aren't done don't count yet.
	got = SummarizeDay("2026-11-16", chores, false)
	if got.Assigned != 2 || got.Done != 2 {
		t.Fatalf("running day = %+v, want 2 of 2", got)
	}
	// ...unless they've already expired.
	late := chore(model.CategoryCore, model.AssignmentIndividual, false, "")
	late.Expired = true
	got = SummarizeDay("2026-11-16", []model.ScheduledChore{late}, false)
	if got.Assigned != 1 || got.Done != 0 {
		t.Fatalf("expired chore = %+v, want 0 of 1", got)
	}
}

func TestWeekStart(t *testing.T) {
	for in, want := range map[string]string{
		"2026-11-16": "2026-11-16", // Monday
		"2026-11-17": "2026-11-16",
		"2026-11-22": "2026-11-16", // Sunday
		"2026-11-23": "2026-11-23",
	} {
		d, _ := time.Parse(model.DateFormat, in)
		if got := WeekStart(d); got != want {
			t.Errorf("WeekStart(%s) = %s, want %s", in, got, want)
		}
	}
	if got := WeekEnd("2026-11-16"); got != "2026-11-22" {
		t.Errorf("WeekEnd = %s", got)
	}
}

func TestSumAndRank(t *testing.T) {
	days := []model.ScoreDay{
		{Date: "2026-11-15", Assigned: 9, Done: 0}, // last week
		{Date: "2026-11-16", Assigned: 3, Done: 3, Bonus: 1},
		{Date: "2026-11-17", Assigned: 4, Done: 4, Paused: true}, // paused: skipped
		{Date: "2026-11-18", Assigned: 3, Done: 2},
	}
	got := Sum(days, "2026-11-16", "2026-11-22")
	if got.Assigned != 6 || got.Done != 5 || got.Bonus != 1 || got.Percent != 83 {
		t.Fatalf("Sum = %+v", got)
	}

	totals := []Total{
		{Assigned: 4, Done: 4},             // perfect, but too few chores to rank
		{Assigned: 10, Done: 9},            // 90%
		{Assigned: 20, Done: 18, Bonus: 2}, // 90% with bonus: ahead of the plain 90%
		{Assigned: 20, Done: 18, Bonus: 2}, // tie
		{Assigned: 8, Done: 8},             // 100%
	}
	Rank(len(totals), func(i int) *Total { return &totals[i] }, func(i, j int) { totals[i], totals[j] = totals[j], totals[i] })
	wantRanks := []int{1, 2, 2, 4, 0}
	wantDone := []int{8, 18, 18, 9, 4}
	for i := range totals {
		if totals[i].Rank != wantRanks[i] || totals[i].Done != wantDone[i] {
			t.Fatalf("place %d = %+v, want rank %d done %d", i, totals[i], wantRanks[i], wantDone[i])
		}
	}
}
