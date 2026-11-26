// Package pet holds the rules for the optional pet companion: how a day of
// chores changes it, what pet-care rewards do, and the mood it shows.
//
// The one rule that shapes the rest: chores keep the pet healthy, points only
// buy food and play. Health can't be bought, so the pet never becomes a way
// to pay your way out of chores.
package pet

import (
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

// Tunables. Keep docs/pets.md in step.
const (
	StartHealth   = 80
	StartFullness = 70
	StartJoy      = 70

	DailyHunger   = 20 // fullness lost every day
	DailyBoredom  = 15 // joy lost every day
	GoodDayHealth = 15 // all of the day's chores done
	FreeDayHealth = 5  // a day with no chores
	MissHealth    = 10 // per missed chore...
	MaxMissHealth = 30 // ...up to this much in one day
	StarveHealth  = 10 // fullness ran out

	SickBelow     = 40 // also what counts as a sick day for pet deaths
	VerySickBelow = 15
	HungryBelow   = 30
	BoredBelow    = 30
	HappyAtLeast  = 70

	// CatchUpDays caps how many unchecked days one nightly run applies
	// (e.g. after the server was down), so a long outage can't flatten a pet.
	CatchUpDays = 7
)

// DayResult is what a person's chores looked like on one day.
type DayResult struct {
	Counted int  // chores that count toward the day
	Missed  int  // of those, not done (not completed, pending or excused)
	Paused  bool // the person was paused: the pet waits with them
}

// ApplyDay changes the pet for one finished day.
func ApplyDay(p *model.Pet, date string, d DayResult) {
	if d.Paused {
		return
	}
	p.Fullness = clamp(p.Fullness - DailyHunger)
	p.Joy = clamp(p.Joy - DailyBoredom)
	switch {
	case d.Missed > 0:
		p.Health -= min(MaxMissHealth, MissHealth*d.Missed)
		dd := date
		p.LastMissedDate = &dd
	case d.Counted > 0:
		p.Health += GoodDayHealth
	default:
		p.Health += FreeDayHealth
	}
	if p.Fullness == 0 {
		p.Health -= StarveHealth
	}
	p.Health = clamp(p.Health)
	// Pet deaths count neglect, not slow recovery: only a night that ends
	// sick after missed chores counts. A day with every chore done clears the
	// count even if the animal is still sick, so doing the chores always
	// takes the danger away (and food, which points buy, never decides it).
	// A day with no chores leaves the count as it is.
	switch {
	case p.Health >= SickBelow:
		p.SickDays = 0
	case d.Missed > 0:
		p.SickDays++
	case d.Counted > 0:
		p.SickDays = 0
	}
}

// Dies reports whether a pet has stayed sick long enough to die, given the
// household's setting (deathDays; 0 = animals never die). Call after ApplyDay.
func Dies(p *model.Pet, deathDays int) bool {
	return deathDays > 0 && p.SickDays >= deathDays
}

// DaysLeft is how many more nights with missed chores the pet can end sick
// before it dies (1 = it dies tonight if today's chores aren't all done), or
// 0 when it isn't in danger: deaths are off, it isn't sick, or today's chores
// are all done (that clears the count tonight). Call after ShowToday, so a
// kid who has done today's chores sees the danger lift straight away.
func DaysLeft(p *model.Pet, deathDays int) int {
	if deathDays <= 0 || p.Health >= SickBelow || p.HealedToday {
		return 0
	}
	return max(1, deathDays-p.SickDays)
}

// ShowToday previews today's health gain as soon as it is earned: once every
// chore that counts toward today is done, the pet is shown with the
// GoodDayHealth that tonight's check will give it, so a kid who has done
// everything sees their animal perk up now rather than tomorrow.
//
// It only changes what is shown. The stored health is untouched, tonight's
// ApplyDay is still what makes it real, and un-ticking a chore (or a
// rejection) takes the preview away again. Call it before Describe.
//
// The gain shown is what tonight's ApplyDay will give for a done day. If the
// food will run out tonight, the StarveHealth it costs is taken off too, so
// the animal doesn't look better now than it will tomorrow morning. Days the
// nightly check hasn't reached yet (just after midnight, or after downtime)
// will eat DailyHunger each first, so they count toward running out. A
// paused owner gets no preview: ApplyDay leaves their animal as it is.
// yesterday is the day before today.
func ShowToday(p *model.Pet, today []model.ScheduledChore, yesterday string, paused bool) {
	if paused {
		return
	}
	d := Summarize(today)
	if d.Counted == 0 || d.Missed > 0 {
		return
	}
	fullness := p.Fullness - DailyHunger*pendingDays(p.LastDayChecked, yesterday)
	gain := GoodDayHealth
	if clamp(fullness-DailyHunger) == 0 {
		gain -= StarveHealth
	}
	p.Health = clamp(p.Health + gain)
	p.HealedToday = true
}

// pendingDays is how many finished days after lastChecked, up to and
// including yesterday, the nightly check still has to apply (at most
// CatchUpDays, as it does).
func pendingDays(lastChecked, yesterday string) int {
	last, err1 := time.Parse(model.DateFormat, lastChecked)
	end, err2 := time.Parse(model.DateFormat, yesterday)
	if err1 != nil || err2 != nil || !end.After(last) {
		return 0
	}
	return min(CatchUpDays, int(end.Sub(last).Hours()/24+0.5))
}

// Effect is what a pet-care reward does to fullness and joy.
func Effect(action string) (fullness, joy int) {
	switch action {
	case model.PetActionFeed:
		return 40, 5
	case model.PetActionTreat:
		return 15, 20
	case model.PetActionPlay:
		return -10, 35
	}
	return 0, 0
}

// Describe fills in the computed fields. yesterday is the day before today;
// todayDone is whether every chore counting toward today that can be done
// yet is done (see CaughtUp).
func Describe(p *model.Pet, yesterday string, todayDone bool) {
	p.Hiding = p.LastMissedDate != nil && *p.LastMissedDate == yesterday && !todayDone
	p.Sick = p.Health < SickBelow
	p.CanPlay = !p.Hiding && p.Health >= VerySickBelow
	switch {
	case p.Hiding:
		p.Mood = "hiding"
	case p.Health < VerySickBelow:
		p.Mood = "very_sick"
	case p.Health < SickBelow:
		p.Mood = "sick"
	case p.Fullness < HungryBelow:
		p.Mood = "hungry"
	case p.Joy < BoredBelow:
		p.Mood = "bored"
	case p.Joy >= HappyAtLeast && p.Fullness >= 50:
		p.Mood = "happy"
	default:
		p.Mood = "content"
	}
}

// IsDone reports whether a scheduled chore counts as done for the pet:
// completed (approved or waiting for a grown-up), excused, or a first come,
// first served chore a sibling got to first. A rejected try doesn't count.
func IsDone(c model.ScheduledChore) bool {
	if c.CompletedBySibling {
		return true
	}
	if !c.Completed {
		return false
	}
	return c.CompletionStatus == nil || *c.CompletionStatus != model.StatusRejected
}

// Summarize turns a day's chores into a DayResult (Paused left false).
func Summarize(chores []model.ScheduledChore) DayResult {
	var d DayResult
	for _, c := range chores {
		if !model.CountsTowardDay(c.Category) {
			continue
		}
		d.Counted++
		if !IsDone(c) {
			d.Missed++
		}
	}
	return d
}

// CaughtUp reports whether every chore counting toward today that is
// available yet is done: a hiding pet comes out as soon as you've caught up,
// without waiting for chores that unlock this evening.
func CaughtUp(today []model.ScheduledChore) bool {
	for _, c := range today {
		if model.CountsTowardDay(c.Category) && c.Available && !IsDone(c) {
			return false
		}
	}
	return true
}

func clamp(v int) int {
	return max(0, min(100, v))
}
