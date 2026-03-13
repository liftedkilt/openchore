package webhook

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/liftedkilt/openchore/internal/store"
)

// ExpiryChecker runs a periodic check for expired chores and fires webhook events.
type ExpiryChecker struct {
	store      *store.Store
	dispatcher *Dispatcher
	interval   time.Duration

	// Track which chores we've already fired events for today (schedule_id -> date)
	fired   map[string]bool
	firedMu sync.Mutex
}

func NewExpiryChecker(s *store.Store, d *Dispatcher) *ExpiryChecker {
	return &ExpiryChecker{
		store:      s,
		dispatcher: d,
		interval:   1 * time.Minute,
		fired:      make(map[string]bool),
	}
}

func (ec *ExpiryChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(ec.interval)
	defer ticker.Stop()

	// Reset fired map at midnight
	lastDate := time.Now().Format("2006-01-02")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			today := now.Format("2006-01-02")

			// Reset fired map on new day
			if today != lastDate {
				ec.firedMu.Lock()
				ec.fired = make(map[string]bool)
				ec.firedMu.Unlock()
				lastDate = today
			}

			currentTime := now.Format("15:04")
			expired, err := ec.store.GetExpiredChores(ctx, today, currentTime)
			if err != nil {
				log.Printf("expiry-checker: failed to get expired chores: %v", err)
				continue
			}

			for _, e := range expired {
				key := firedKey(e.ScheduleID, today)
				ec.firedMu.Lock()
				alreadyFired := ec.fired[key]
				if !alreadyFired {
					ec.fired[key] = true
				}
				ec.firedMu.Unlock()

				if !alreadyFired {
					ec.dispatcher.Fire(EventChoreExpired, map[string]any{
						"schedule_id": e.ScheduleID,
						"chore_title": e.ChoreTitle,
						"user_id":     e.UserID,
						"user_name":   e.UserName,
						"due_by":      e.DueBy,
						"date":        today,
					})
				}
			}
		}
	}
}

func firedKey(scheduleID int64, date string) string {
	return date + ":" + itoa(scheduleID)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	result := ""
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	if neg {
		result = "-" + result
	}
	return result
}
