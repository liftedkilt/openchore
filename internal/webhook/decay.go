package webhook

import (
	"context"
	"log"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

// PointsDecayChecker runs periodically and debits points from kids whose
// non-bonus chores were not all completed the previous day, according to the
// per-user settings in user_decay_config.
//
// This is distinct from DecayChecker in penalty.go, which (despite its name)
// handles per-chore "missed required chore" penalties.
type PointsDecayChecker struct {
	store      *store.Store
	dispatcher *Dispatcher
	interval   time.Duration
}

func NewPointsDecayChecker(s *store.Store, d *Dispatcher) *PointsDecayChecker {
	return &PointsDecayChecker{
		store:      s,
		dispatcher: d,
		interval:   15 * time.Minute,
	}
}

func (pdc *PointsDecayChecker) Start(ctx context.Context) {
	// Run an immediate check on startup so decays are applied promptly after
	// a restart rather than having to wait a full tick.
	pdc.check(ctx)

	ticker := time.NewTicker(pdc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pdc.check(ctx)
		}
	}
}

func (pdc *PointsDecayChecker) check(ctx context.Context) {
	configs, err := pdc.store.ListDecayConfigsEnabled(ctx)
	if err != nil {
		log.Printf("points-decay: failed to list decay configs: %v", err)
		return
	}

	now := time.Now()
	yesterday := now.AddDate(0, 0, -1).Format(model.DateFormat)

	for _, cfg := range configs {
		// Respect the per-user decay interval: skip users who were decayed
		// more recently than decay_interval_hours ago.
		if cfg.LastDecayAt != nil {
			elapsed := now.Sub(*cfg.LastDecayAt)
			if elapsed < time.Duration(cfg.DecayIntervalHours)*time.Hour {
				continue
			}
		}

		user, err := pdc.store.GetUser(ctx, cfg.UserID)
		if err != nil {
			log.Printf("points-decay: failed to load user %d: %v", cfg.UserID, err)
			continue
		}
		if user == nil || user.Role != "child" || user.Paused {
			continue
		}

		// The rule (see README): decay only if any non-bonus chore was not
		// completed yesterday. Bonus chores are ignored.
		chores, err := pdc.store.GetScheduledChoresForUser(ctx, cfg.UserID, []string{yesterday}, now)
		if err != nil {
			log.Printf("points-decay: failed to list chores for user %d: %v", cfg.UserID, err)
			continue
		}

		nonBonusCount := 0
		missedAny := false
		for _, c := range chores {
			if c.Category == model.CategoryBonus {
				continue
			}
			nonBonusCount++
			if !c.Completed {
				missedAny = true
			}
		}

		// No non-bonus chores scheduled or everything was completed: no
		// decay, but still advance the timer so we don't keep re-checking.
		if nonBonusCount == 0 || !missedAny {
			if err := pdc.store.UpdateLastDecayAt(ctx, cfg.UserID, now); err != nil {
				log.Printf("points-decay: failed to update last_decay_at for user %d: %v", cfg.UserID, err)
			}
			continue
		}

		// Clamp the debit so decay never pushes the balance negative.
		balance, err := pdc.store.GetPointBalance(ctx, cfg.UserID)
		if err != nil {
			log.Printf("points-decay: failed to get balance for user %d: %v", cfg.UserID, err)
			continue
		}
		debit := cfg.DecayRate
		if debit > balance {
			debit = balance
		}

		if debit > 0 {
			if err := pdc.store.DebitDecay(ctx, cfg.UserID, debit); err != nil {
				log.Printf("points-decay: failed to debit user %d: %v", cfg.UserID, err)
				continue
			}
			log.Printf("points-decay: debited %d points from user %d (%s) for missed chores on %s", debit, user.ID, user.Name, yesterday)

			pdc.dispatcher.Fire(EventPointsDecayed, map[string]any{
				"user_id":   user.ID,
				"user_name": user.Name,
				"amount":    debit,
				"date":      yesterday,
			})
		}

		if err := pdc.store.UpdateLastDecayAt(ctx, cfg.UserID, now); err != nil {
			log.Printf("points-decay: failed to update last_decay_at for user %d: %v", cfg.UserID, err)
		}
	}
}
