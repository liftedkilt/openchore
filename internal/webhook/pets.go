package webhook

import (
	"context"
	"log"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/pet"
	"github.com/liftedkilt/openchore/internal/store"
)

// PetChecker applies each finished day's chores to every pet: done days heal
// it, missed chores make it sick and send it into hiding. It runs whether or
// not pets are switched on, so turning them off and on again doesn't replay
// the gap (pets are only stored when the feature was on).
type PetChecker struct {
	store    *store.Store
	interval time.Duration
	now      func() time.Time
}

func NewPetChecker(s *store.Store) *PetChecker {
	return &PetChecker{store: s, interval: 15 * time.Minute, now: time.Now}
}

// CheckAt runs one pass as if it were now (tests).
func (pc *PetChecker) CheckAt(ctx context.Context, now time.Time) {
	pc.check(ctx, now)
}

func (pc *PetChecker) Start(ctx context.Context) {
	pc.check(ctx, pc.now())
	ticker := time.NewTicker(pc.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pc.check(ctx, pc.now())
		}
	}
}

func (pc *PetChecker) check(ctx context.Context, now time.Time) {
	pets, err := pc.store.ListPets(ctx)
	if err != nil {
		log.Printf("pets: failed to list pets: %v", err)
		return
	}
	yesterday := now.AddDate(0, 0, -1).Format(model.DateFormat)
	for i := range pets {
		if err := pc.catchUp(ctx, &pets[i], yesterday, now); err != nil {
			log.Printf("pets: user %d: %v", pets[i].UserID, err)
		}
	}
}

func (pc *PetChecker) catchUp(ctx context.Context, p *model.Pet, yesterday string, now time.Time) error {
	if p.LastDayChecked >= yesterday {
		return nil
	}
	last, err := time.ParseInLocation(model.DateFormat, p.LastDayChecked, now.Location())
	if err != nil {
		return err
	}
	end, _ := time.ParseInLocation(model.DateFormat, yesterday, now.Location())
	start := last.AddDate(0, 0, 1)
	if skip := end.AddDate(0, 0, -(pet.CatchUpDays - 1)); start.Before(skip) {
		start = skip
	}

	user, err := pc.store.GetUser(ctx, p.UserID)
	if err != nil || user == nil {
		return err
	}
	prev := p.LastDayChecked
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		date := d.Format(model.DateFormat)
		var res pet.DayResult
		if user.Paused {
			// Paused is a current flag with no history; treat the whole
			// catch-up window as paused, the same way the penalty checker does.
			res.Paused = true
		} else {
			chores, err := pc.store.GetScheduledChoresForUser(ctx, p.UserID, []string{date}, now)
			if err != nil {
				return err
			}
			res = pet.Summarize(chores)
		}
		pet.ApplyDay(p, date, res)
		p.LastDayChecked = date
	}
	_, err = pc.store.SavePetDay(ctx, p, prev)
	return err
}
