package score

import (
	"testing"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

func chore(cat, assign string, completed bool, status string) model.ScheduledChore {
	c := model.ScheduledChore{Category: cat, AssignmentType: assign, Completed: completed, PointsValue: 5}
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
	if got.Assigned != 4 || got.Done != 2 || got.Bonus != 2 || got.AssignedPoints != 20 || got.DonePoints != 10 {
		t.Fatalf("finished day = %+v, want 2 of 4 (10 of 20 points), 2 bonus", got)
	}
	// A big chore counts for more than a small one.
	big := chore(model.CategoryCore, model.AssignmentIndividual, false, "")
	big.PointsValue = 30
	small := chore(model.CategoryCore, model.AssignmentIndividual, true, model.StatusApproved)
	got = SummarizeDay("2026-11-16", []model.ScheduledChore{big, small}, true)
	if got.Assigned != 2 || got.Done != 1 || got.AssignedPoints != 35 || got.DonePoints != 5 {
		t.Fatalf("weighted day = %+v, want 1 of 2 chores, 5 of 35 points", got)
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
		{Date: "2026-11-15", Assigned: 9, Done: 0, AssignedPoints: 90}, // last week
		{Date: "2026-11-16", Assigned: 3, Done: 3, Bonus: 1, AssignedPoints: 15, DonePoints: 15},
		{Date: "2026-11-17", Assigned: 4, Done: 4, AssignedPoints: 40, DonePoints: 40, Paused: true}, // paused: skipped
		{Date: "2026-11-18", Assigned: 3, Done: 2, AssignedPoints: 35, DonePoints: 5},                // missed the big one
	}
	got := Sum(days, "2026-11-16", "2026-11-22")
	// 5 of 6 chores, but only 20 of the 50 points: the percentage follows the points.
	if got.Assigned != 6 || got.Done != 5 || got.Bonus != 1 || got.AssignedPoints != 50 || got.DonePoints != 20 || got.Share != 4000 || got.Percent != 40 {
		t.Fatalf("Sum = %+v", got)
	}
	// The score rounds down: 100 means nothing was missed, and any work shows.
	for _, c := range []struct{ assigned, done, aPts, dPts, share, pct int }{
		{20, 19, 200, 199, 9950, 99}, // one small chore missed: not 100
		{3, 2, 3, 2, 6666, 66},       // two thirds
		{300, 1, 300, 1, 33, 1},      // a little work is not 0
		{6, 5, 10, 10, 9999, 99},     // the missed chore was worth no points: still not a full score
		{6, 6, 10, 10, 10000, 100},   // everything done
	} {
		got := Sum([]model.ScoreDay{{Date: "2026-11-16", Assigned: c.assigned, Done: c.done, AssignedPoints: c.aPts, DonePoints: c.dPts}}, "2026-11-16", "2026-11-22")
		if got.Share != c.share || got.Percent != c.pct {
			t.Errorf("%d of %d chores, %d of %d points: share %d percent %d, want %d and %d", c.done, c.assigned, c.dPts, c.aPts, got.Share, got.Percent, c.share, c.pct)
		}
	}
	// Level on share: the one who did more of their chores is ahead.
	lvl := []Total{{Assigned: 10, Done: 8, Share: 9000}, {Assigned: 10, Done: 9, Share: 9000}}
	Rank(2, func(i int) *Total { return &lvl[i] }, func(i, j int) { lvl[i], lvl[j] = lvl[j], lvl[i] })
	if lvl[0].Done != 9 || lvl[0].Rank != 1 || lvl[1].Rank != 2 {
		t.Errorf("tie on share not broken by chores done: %+v", lvl)
	}
	// A family that doesn't use points is scored by chores.
	if got := Sum([]model.ScoreDay{{Date: "2026-11-16", Assigned: 6, Done: 5}}, "2026-11-16", "2026-11-22"); got.Percent != 83 {
		t.Fatalf("no-points Sum = %+v", got)
	}

	// A friend's total is built from the share they send, and ranks against
	// a local one on that share alone.
	if ft := FromShare(20, 18, 2, 9000); ft.Percent != 90 || ft.Share != 9000 || ft.Done != 18 {
		t.Fatalf("FromShare = %+v", ft)
	}
	if ft := FromShare(3, 9, -1, 99999); ft.Done != 3 || ft.Bonus != 0 || ft.Share != ShareScale {
		t.Fatalf("FromShare out of range = %+v", ft)
	}
	totals := []Total{
		{Assigned: 4, Done: 4, Share: 10000},            // perfect, but too few chores to rank
		{Assigned: 10, Done: 9, Share: 9000},            // 90
		FromShare(20, 18, 2, 9000),                      // 90 with bonus, from a friend: ahead of the plain 90
		{Assigned: 20, Done: 18, Bonus: 2, Share: 9000}, // tie with the friend
		{Assigned: 8, Done: 8, Share: 10000},            // 100
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

// A first-come chore a sibling did only marks this copy done: it isn't an
// extra chore this person did.
func TestSummarizeDay_SiblingFCFSIsNotBonus(t *testing.T) {
	mine := chore(model.CategoryCore, model.AssignmentFCFS, true, model.StatusApproved)
	theirs := chore(model.CategoryCore, model.AssignmentFCFS, true, model.StatusApproved)
	theirs.CompletedBySibling = true
	if d := SummarizeDay("2026-03-11", []model.ScheduledChore{mine, theirs}, true); d.Bonus != 1 {
		t.Errorf("Bonus = %d, want 1 (only the first-come chore this person did)", d.Bonus)
	}
}
