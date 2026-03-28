package webhook

import (
	"context"
	"log"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

// DecayChecker (formerly PenaltyChecker) runs periodically to apply penalties for missed required chores.
// We keep the name DecayChecker for now to avoid breaking existing server initialization.
type DecayChecker struct {
	store      *store.Store
	dispatcher *Dispatcher
	interval   time.Duration
}

func NewDecayChecker(s *store.Store, d *Dispatcher) *DecayChecker {
	return &DecayChecker{
		store:      s,
		dispatcher: d,
		interval:   15 * time.Minute,
	}
}

func (pc *DecayChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(pc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pc.check(ctx)
		}
	}
}

func (pc *DecayChecker) check(ctx context.Context) {
	users, err := pc.store.ListUsers(ctx)
	if err != nil {
		log.Printf("penalty-checker: failed to list users: %v", err)
		return
	}

	now := time.Now()
	yesterday := now.AddDate(0, 0, -1).Format(model.DateFormat)

	for _, u := range users {
		if u.Role != "child" || u.Paused {
			continue
		}

		chores, err := pc.store.GetScheduledChoresForUser(ctx, u.ID, []string{yesterday}, now)
		if err != nil {
			log.Printf("penalty-checker: failed to get chores for user %d: %v", u.ID, err)
			continue
		}

		for _, c := range chores {
			// Only penalize required chores that weren't completed and have a penalty value
			if c.Category == "required" && !c.Completed && c.MissedPenaltyValue > 0 {
				// Check if already penalized to avoid double-dipping
				alreadyPenalized, err := pc.store.HasMissedChorePenalty(ctx, c.ScheduleID, yesterday)
				if err != nil {
					log.Printf("penalty-checker: failed to check existing penalty for user %d, schedule %d: %v", u.ID, c.ScheduleID, err)
					continue
				}
				if alreadyPenalized {
					continue
				}

				if err := pc.store.DebitMissedChore(ctx, u.ID, c.ScheduleID, c.MissedPenaltyValue, yesterday); err != nil {
					log.Printf("penalty-checker: failed to debit penalty for user %d, schedule %d: %v", u.ID, c.ScheduleID, err)
					continue
				}

				pc.dispatcher.Fire(EventChoreMissed, map[string]any{
					"user_id":        u.ID,
					"user_name":      u.Name,
					"chore_title":    c.Title,
					"penalty_amount": c.MissedPenaltyValue,
					"date":           yesterday,
				})
			}
		}
	}
}
