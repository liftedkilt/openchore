package pet

import (
	"testing"

	"github.com/liftedkilt/openchore/internal/model"
)

func ptr(s string) *string { return &s }

func newPet() *model.Pet {
	return &model.Pet{Health: StartHealth, Fullness: StartFullness, Joy: StartJoy}
}

func TestApplyDay(t *testing.T) {
	p := newPet()
	ApplyDay(p, "2026-11-16", DayResult{Counted: 3})
	if p.Health != 95 || p.Fullness != 50 || p.Joy != 55 || p.LastMissedDate != nil {
		t.Fatalf("good day: %+v", p)
	}
	ApplyDay(p, "2026-11-17", DayResult{Counted: 3, Missed: 1})
	if p.Health != 85 || p.LastMissedDate == nil || *p.LastMissedDate != "2026-11-17" {
		t.Fatalf("missed day: %+v", p)
	}
	ApplyDay(p, "2026-11-18", DayResult{Counted: 5, Missed: 5})
	if p.Health != 55 {
		t.Fatalf("a bad day is capped at -%d: %+v", MaxMissHealth, p)
	}
	// Paused: nothing changes.
	before := *p
	ApplyDay(p, "2026-11-19", DayResult{Counted: 2, Missed: 2, Paused: true})
	if p.Health != before.Health || p.Fullness != before.Fullness {
		t.Fatalf("paused day changed the pet: %+v", p)
	}
	// Starving costs health on top.
	p = newPet()
	p.Fullness = 10
	ApplyDay(p, "2026-11-20", DayResult{})
	if p.Fullness != 0 || p.Health != StartHealth+FreeDayHealth-StarveHealth {
		t.Fatalf("starving: %+v", p)
	}
}

func TestDescribe(t *testing.T) {
	p := newPet()
	p.LastMissedDate = ptr("2026-11-16")
	Describe(p, "2026-11-16", false)
	if !p.Hiding || p.Mood != "hiding" || p.CanPlay {
		t.Fatalf("missed yesterday, today not done: %+v", p)
	}
	Describe(p, "2026-11-16", true)
	if p.Hiding || !p.CanPlay {
		t.Fatalf("today's chores coax it out: %+v", p)
	}
	Describe(p, "2026-11-17", false)
	if p.Hiding {
		t.Fatalf("an older miss doesn't hide it: %+v", p)
	}
	p.Health = 10
	Describe(p, "2026-11-17", true)
	if p.Mood != "very_sick" || !p.Sick || p.CanPlay {
		t.Fatalf("very sick: %+v", p)
	}
}

func TestSummarizeCountsPendingAndExcusedAsDone(t *testing.T) {
	st := func(s string) *string { return &s }
	chores := []model.ScheduledChore{
		{Category: "required", Completed: true, CompletionStatus: st(model.StatusApproved)},
		{Category: "core", Completed: true, CompletionStatus: st(model.StatusPending)},
		{Category: "core", Completed: true, CompletionStatus: st(model.StatusExcused)},
		{Category: "core", Completed: true, CompletionStatus: st(model.StatusRejected)},
		{Category: "core"},
		{Category: "core", CompletedBySibling: true},
		{Category: "bonus"},
		{Category: "optional"},
	}
	d := Summarize(chores)
	if d.Counted != 6 || d.Missed != 2 {
		t.Fatalf("got %+v", d)
	}
}

func TestCaughtUpIgnoresLaterChores(t *testing.T) {
	chores := []model.ScheduledChore{
		{Category: "core", Available: true, Completed: true},
		{Category: "core", Available: false},
		{Category: "bonus", Available: true},
	}
	if !CaughtUp(chores) {
		t.Fatal("a chore that isn't available yet held the pet in hiding")
	}
	chores[1].Available = true
	if CaughtUp(chores) {
		t.Fatal("caught up with an available chore undone")
	}
}

func TestShowToday(t *testing.T) {
	done, approved := true, model.StatusApproved
	chore := func(cat string, completed bool) model.ScheduledChore {
		c := model.ScheduledChore{Category: cat, Completed: completed, Available: true}
		if completed {
			c.CompletionStatus = &approved
		}
		return c
	}
	_ = done
	sick := func() *model.Pet { return &model.Pet{Health: 34, Fullness: 80, Joy: 80} }

	// Everything that counts is done: the gain shows now, and the pet is no longer sick.
	p := sick()
	ShowToday(p, []model.ScheduledChore{chore(model.CategoryRequired, true), chore(model.CategoryCore, true), chore(model.CategoryBonus, false)})
	Describe(p, "2026-11-16", true)
	if p.Health != 34+GoodDayHealth || !p.HealedToday || p.Sick {
		t.Fatalf("all done: %+v", p)
	}
	// One chore still open (even one that isn't available yet): no preview.
	p = sick()
	late := chore(model.CategoryCore, false)
	late.Available = false
	ShowToday(p, []model.ScheduledChore{chore(model.CategoryRequired, true), late})
	Describe(p, "2026-11-16", true)
	if p.Health != 34 || p.HealedToday || !p.Sick {
		t.Fatalf("one open: %+v", p)
	}
	// A day with nothing that counts gives no preview (tonight's small gain isn't shown early).
	p = sick()
	ShowToday(p, []model.ScheduledChore{chore(model.CategoryBonus, true)})
	if p.Health != 34 || p.HealedToday {
		t.Fatalf("nothing counted: %+v", p)
	}
	// Food runs out tonight: the preview matches tonight's check (+15, -10 for
	// starving), not a gain the animal will lose again by morning.
	for _, full := range []int{0, DailyHunger} {
		p = &model.Pet{Health: 20, Fullness: full}
		ShowToday(p, []model.ScheduledChore{chore(model.CategoryCore, true)})
		night := &model.Pet{Health: 20, Fullness: full}
		ApplyDay(night, "2026-11-16", DayResult{Counted: 1})
		if p.Health != night.Health || p.Health != 20+GoodDayHealth-StarveHealth || !p.HealedToday {
			t.Fatalf("starving fullness %d: preview %d, tonight %d", full, p.Health, night.Health)
		}
	}
	// Fed enough to last the night: the full gain.
	p = &model.Pet{Health: 20, Fullness: DailyHunger + 1}
	ShowToday(p, []model.ScheduledChore{chore(model.CategoryCore, true)})
	if p.Health != 20+GoodDayHealth {
		t.Fatalf("fed: %+v", p)
	}
	// Never past full.
	p = &model.Pet{Health: 95, Fullness: 80}
	ShowToday(p, []model.ScheduledChore{chore(model.CategoryCore, true)})
	if p.Health != 100 {
		t.Fatalf("clamp: %+v", p)
	}
}
