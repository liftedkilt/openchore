package webhook

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
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

// SetInterval overrides how often the checker ticks. The 15 minute default
// is right for production; the server lowers it from POINTS_DECAY_INTERVAL so
// the e2e suite can observe a decay without waiting a quarter of an hour.
// Non-positive durations are ignored.
func (pdc *PointsDecayChecker) SetInterval(d time.Duration) {
	if d > 0 {
		pdc.interval = d
	}
}

// CheckNow runs one decay pass synchronously. Start already does an immediate
// pass on startup; this is the same work without the goroutine, so callers
// that need to observe the result (tests, mainly) don't have to race a ticker.
func (pdc *PointsDecayChecker) CheckNow(ctx context.Context) {
	pdc.check(ctx)
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
		var missedTitles []string
		for _, c := range chores {
			if c.Category == model.CategoryBonus {
				continue
			}
			nonBonusCount++
			if !c.Completed {
				missedTitles = append(missedTitles, c.Title)
			}
		}

		// No non-bonus chores scheduled or everything was completed: no
		// decay, but still advance the timer so we don't keep re-checking.
		if nonBonusCount == 0 || len(missedTitles) == 0 {
			if err := pdc.store.UpdateLastDecayAt(ctx, cfg.UserID, now); err != nil {
				log.Printf("points-decay: failed to update last_decay_at for user %d: %v", cfg.UserID, err)
			}
			continue
		}

		// Decay draws on the spendable balance first and then on points
		// already committed to savings goals. Goal savings are deliberately
		// not a safe harbour: without the clawback a kid could shelter every
		// point they earn from decay by parking it in an expensive goal they
		// never redeem, and cherry-pick which chores to do. The store clamps
		// the debit so it can never push the balance negative.
		note := fmt.Sprintf("Points decay for %s — missed: %s", yesterday, strings.Join(missedTitles, ", "))
		debit, clawbacks, err := pdc.store.DebitDecayWithClawback(ctx, cfg.UserID, cfg.DecayRate, yesterday, note)
		switch {
		case errors.Is(err, store.ErrDecayAlreadyApplied):
			// This date was already debited — fall through to update
			// last_decay_at so we don't keep retrying.
			log.Printf("points-decay: already debited user %d for %s (idempotency), advancing timer", cfg.UserID, yesterday)
		case err != nil:
			log.Printf("points-decay: failed to debit user %d: %v", cfg.UserID, err)
			continue
		case debit > 0:
			reclaimed := 0
			goalNames := make([]string, 0, len(clawbacks))
			for _, cb := range clawbacks {
				reclaimed += cb.Amount
				goalNames = append(goalNames, cb.RewardName)
			}
			if reclaimed > 0 {
				log.Printf("points-decay: debited %d points from user %d (%s) for missed chores on %s (missed: %s; %d reclaimed from goals: %s)",
					debit, user.ID, user.Name, yesterday, strings.Join(missedTitles, ", "),
					reclaimed, strings.Join(goalNames, ", "))
			} else {
				log.Printf("points-decay: debited %d points from user %d (%s) for missed chores on %s (missed: %s)",
					debit, user.ID, user.Name, yesterday, strings.Join(missedTitles, ", "))
			}

			pdc.dispatcher.Fire(EventPointsDecayed, map[string]any{
				"user_id":              user.ID,
				"user_name":            user.Name,
				"amount":               debit,
				"date":                 yesterday,
				"missed":               missedTitles,
				"reclaimed_from_goals": reclaimed,
				"goal_clawbacks":       clawbacks,
			})
		}

		if err := pdc.store.UpdateLastDecayAt(ctx, cfg.UserID, now); err != nil {
			log.Printf("points-decay: failed to update last_decay_at for user %d: %v", cfg.UserID, err)
		}
	}
}
