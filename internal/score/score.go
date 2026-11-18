// Package score holds the leaderboard rules: a person's score for a week is
// the share of their assigned chores' points that they earned, so a big
// chore counts for more than a small one.
//
// It is a share, never a points total. Every family sets its own point
// values, so totals can't be compared between families; the share of your
// own chores' points can. Keep docs/friends.md in step.
package score

import (
	"sort"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

// MinAssigned is how many chores a person needs in the week to be ranked.
// Below it one chore swings the percentage too far.
const MinAssigned = 5

// SummarizeDay counts one day's chores. finished says the day is over; while
// it isn't, a chore joins the count only once it's done (or has expired), so
// nobody is marked down in the morning for chores that aren't due yet.
func SummarizeDay(date string, chores []model.ScheduledChore, finished bool) model.ScoreDay {
	d := model.ScoreDay{Date: date}
	for _, c := range chores {
		status := ""
		if c.CompletionStatus != nil {
			status = *c.CompletionStatus
		}
		if status == model.StatusExcused {
			continue // excused: out of both sides
		}
		done := c.Completed && status != model.StatusRejected
		own := c.AssignmentType == "" || c.AssignmentType == model.AssignmentIndividual
		switch {
		case model.CountsTowardDay(c.Category) && own:
			// Worth the chore's own point value (a schedule's multiplier
			// isn't applied).
			pts := max(c.PointsValue, 0)
			if done {
				d.Assigned++
				d.Done++
				d.AssignedPoints += pts
				d.DonePoints += pts
			} else if finished || c.Expired {
				d.Assigned++
				d.AssignedPoints += pts
			}
		case done && (c.Category == model.CategoryBonus || model.CountsTowardDay(c.Category)):
			// Bonus chores, and family / first-come chores this person did.
			d.Bonus++
		}
	}
	return d
}

// WeekStart returns the Monday of the week containing t, as a date string.
func WeekStart(t time.Time) string {
	back := (int(t.Weekday()) + 6) % 7
	return t.AddDate(0, 0, -back).Format(model.DateFormat)
}

// WeekEnd returns the Sunday that closes the week starting on weekStart.
func WeekEnd(weekStart string) string {
	t, err := time.Parse(model.DateFormat, weekStart)
	if err != nil {
		return weekStart
	}
	return t.AddDate(0, 0, 6).Format(model.DateFormat)
}

// Total is a person's score over a set of days.
type Total struct {
	Assigned       int `json:"assigned"`
	Done           int `json:"done"`
	Bonus          int `json:"bonus"`
	AssignedPoints int `json:"assigned_points"`
	DonePoints     int `json:"done_points"`
	// Percent is the share earned, rounded: DonePoints out of
	// AssignedPoints, or Done out of Assigned when the person's chores
	// carry no points at all. 0 when nothing was assigned.
	Percent int `json:"percent"`
	// Rank is the place among ranked people (ties share one); 0 = not
	// ranked yet (fewer than MinAssigned chores).
	Rank int `json:"rank"`
}

// Sum adds up the days between from and to (inclusive), skipping paused ones.
func Sum(days []model.ScoreDay, from, to string) Total {
	var t Total
	for _, d := range days {
		if d.Paused || d.Date < from || d.Date > to {
			continue
		}
		t.Assigned += d.Assigned
		t.Done += d.Done
		t.Bonus += d.Bonus
		t.AssignedPoints += d.AssignedPoints
		t.DonePoints += d.DonePoints
	}
	if num, den := t.share(); den > 0 {
		t.Percent = (num*100 + den/2) / den
	}
	return t
}

// share is the score as a fraction: points earned out of points assigned.
// A family that doesn't use points (every chore worth 0) is scored by the
// number of chores instead.
func (t Total) share() (num, den int) {
	if t.AssignedPoints > 0 {
		return t.DonePoints, t.AssignedPoints
	}
	return t.Done, t.Assigned
}

// Ranked reports whether a total has enough chores behind it to be ranked.
func (t Total) Ranked() bool { return t.Assigned >= MinAssigned }

// less orders two totals best first: ranked before unranked, then the higher
// share of chores done (compared exactly, not by the rounded percent), then
// more bonus chores.
func less(a, b Total) bool {
	if a.Ranked() != b.Ranked() {
		return a.Ranked()
	}
	an, ad := a.share()
	bn, bd := b.share()
	if l, r := an*max(bd, 1), bn*max(ad, 1); l != r {
		return l > r
	}
	return a.Bonus > b.Bonus
}

func tied(a, b Total) bool { return !less(a, b) && !less(b, a) }

// Rank sorts totals best first and fills in Rank. get returns a pointer to
// the i-th entry's total; swap exchanges two entries.
func Rank(n int, get func(i int) *Total, swap func(i, j int)) {
	sort.Stable(sorter{n, get, swap})
	for i := 0; i < n; i++ {
		t := get(i)
		switch {
		case !t.Ranked():
			t.Rank = 0
		case i > 0 && tied(*get(i - 1), *t):
			t.Rank = get(i - 1).Rank
		default:
			t.Rank = i + 1
		}
	}
}

type sorter struct {
	n    int
	get  func(i int) *Total
	swap func(i, j int)
}

func (s sorter) Len() int           { return s.n }
func (s sorter) Less(i, j int) bool { return less(*s.get(i), *s.get(j)) }
func (s sorter) Swap(i, j int)      { s.swap(i, j) }
