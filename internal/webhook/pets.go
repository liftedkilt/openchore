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
//
// With pet deaths on (the pets_death_days setting), an animal that ends that
// many nights in a row sick after missed chores dies: it's removed and a
// grave with its name is left on the farm.
type PetChecker struct {
	store    *store.Store
	interval time.Duration
	now      func() time.Time
	// OnDeath, when set, runs after an animal has died and been buried,
	// e.g. to delete its picture files.
	OnDeath func(p *model.Pet)
}

// SettingPetDeathDays is how many days in a row an animal can end sick
// after missed chores before it dies; "" or "0" means animals never die.
const SettingPetDeathDays = store.SettingPetDeathDays

// PetDeathDays reads the pet deaths setting (0 = off).
func PetDeathDays(ctx context.Context, s *store.Store) int {
	v, _ := s.GetSetting(ctx, SettingPetDeathDays)
	return store.ParsePetDeathDays(v)
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
	deathDays := 0
	if on, _ := pc.store.GetSetting(ctx, store.SettingPetsEnabled); on == "true" {
		deathDays = PetDeathDays(ctx, pc.store)
	}
	for i := range pets {
		if err := pc.catchUp(ctx, &pets[i], yesterday, now, deathDays); err != nil {
			log.Printf("pets: user %d: %v", pets[i].UserID, err)
		}
	}
}

func (pc *PetChecker) catchUp(ctx context.Context, p *model.Pet, yesterday string, now time.Time, deathDays int) error {
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
	prev, prevSick := p.LastDayChecked, p.SickDays
	// One night's check can be fatal; a catch-up over several days (after
	// downtime) can't, so the owner always sees the "dies tonight" warning
	// for at least one real day first.
	single := start.Equal(end)
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
	if single && pet.Dies(p, deathDays) {
		// BuryPet re-checks the setting and the stored count, so a parent
		// turning deaths off or lengthening them meanwhile wins.
		buried, err := pc.store.BuryPet(ctx, p, prev, prevSick, p.LastDayChecked)
		if err != nil {
			return err
		}
		if buried {
			log.Printf("pets: %s (user %d) died on %s after %d days sick", p.Name, p.UserID, p.LastDayChecked, p.SickDays)
			if pc.OnDeath != nil {
				pc.OnDeath(p)
			}
			return nil
		}
	}
	if !single && deathDays > 0 {
		p.SickDays = min(p.SickDays, deathDays-1)
	}
	// Guarded on the count this run started from: if a parent capped it
	// meanwhile, nothing is written and the next tick starts afresh.
	_, err = pc.store.SavePetDay(ctx, p, prev, prevSick)
	return err
}
