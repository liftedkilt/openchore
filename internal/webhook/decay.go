package webhook

import (
	"context"
	"log"
	"time"

	"github.com/liftedkilt/openchore/internal/store"
)

// DecayChecker runs periodically to apply points decay for users who haven't completed their chores.
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

func (dc *DecayChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(dc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dc.check(ctx)
		}
	}
}

func (dc *DecayChecker) check(ctx context.Context) {
	configs, err := dc.store.ListDecayConfigsEnabled(ctx)
	if err != nil {
		log.Printf("decay-checker: failed to list configs: %v", err)
		return
	}

	now := time.Now()

	for _, cfg := range configs {
		// Determine if decay is due
		intervalDuration := time.Duration(cfg.DecayIntervalHours) * time.Hour
		if cfg.LastDecayAt != nil && now.Sub(*cfg.LastDecayAt) < intervalDuration {
			continue
		}

		// Check if user completed all non-bonus chores yesterday
		yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
		chores, err := dc.store.GetScheduledChoresForUser(ctx, cfg.UserID, []string{yesterday}, now)
		if err != nil {
			log.Printf("decay-checker: failed to get chores for user %d: %v", cfg.UserID, err)
			continue
		}

		// If there were no chores yesterday, skip decay
		hasChores := false
		allDone := true
		for _, c := range chores {
			if c.Category == "bonus" {
				continue
			}
			hasChores = true
			if !c.Completed {
				allDone = false
				break
			}
		}

		if !hasChores || allDone {
			// Update timestamp even when no decay, so we don't re-check
			_ = dc.store.UpdateLastDecayAt(ctx, cfg.UserID, now)
			continue
		}

		// Check current balance — don't decay below 0
		balance, err := dc.store.GetPointBalance(ctx, cfg.UserID)
		if err != nil {
			log.Printf("decay-checker: failed to get balance for user %d: %v", cfg.UserID, err)
			continue
		}
		if balance <= 0 {
			_ = dc.store.UpdateLastDecayAt(ctx, cfg.UserID, now)
			continue
		}

		decayAmount := cfg.DecayRate
		if decayAmount > balance {
			decayAmount = balance
		}

		if err := dc.store.DebitDecay(ctx, cfg.UserID, decayAmount); err != nil {
			log.Printf("decay-checker: failed to debit decay for user %d: %v", cfg.UserID, err)
			continue
		}
		_ = dc.store.UpdateLastDecayAt(ctx, cfg.UserID, now)

		// Look up user name for webhook
		user, _ := dc.store.GetUser(ctx, cfg.UserID)
		userName := ""
		if user != nil {
			userName = user.Name
		}

		dc.dispatcher.Fire(EventPointsDecayed, map[string]any{
			"user_id":      cfg.UserID,
			"user_name":    userName,
			"decay_amount": decayAmount,
			"new_balance":  balance - decayAmount,
		})
	}
}
