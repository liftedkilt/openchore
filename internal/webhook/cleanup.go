package webhook

import (
	"context"
	"log"
	"time"

	"github.com/liftedkilt/openchore/internal/store"
)

// DeliveryCleaner periodically purges old rows from webhook_deliveries so the
// table does not grow unbounded. See issue #18.
//
// A full SQLite VACUUM is intentionally not run here: VACUUM requires that no
// other transactions be in flight, rewrites the entire database file, and the
// codebase does not otherwise use PRAGMA auto_vacuum=incremental, so
// `PRAGMA incremental_vacuum` is a no-op. Deleted pages are returned to the
// SQLite free list and reused by subsequent inserts, so disk growth is bounded
// even without VACUUM. Operators who want to reclaim disk space on an existing
// install can run `sqlite3 openchore.db 'VACUUM;'` while the server is stopped.
type DeliveryCleaner struct {
	store         *store.Store
	retention     time.Duration
	interval      time.Duration
	firstRunDelay time.Duration
}

// NewDeliveryCleaner constructs a cleaner. retentionDays <= 0 disables cleanup
// (the returned cleaner's Start will immediately return). intervalHours <= 0
// falls back to 24h.
func NewDeliveryCleaner(s *store.Store, retentionDays, intervalHours int) *DeliveryCleaner {
	if intervalHours <= 0 {
		intervalHours = 24
	}
	return &DeliveryCleaner{
		store:     s,
		retention: time.Duration(retentionDays) * 24 * time.Hour,
		interval:  time.Duration(intervalHours) * time.Hour,
		// Small delay so we don't compete with startup work on the same DB conn.
		firstRunDelay: 30 * time.Second,
	}
}

// Start runs the cleanup loop until ctx is cancelled. It performs an initial
// run after firstRunDelay, then runs every interval. Cleanup is a no-op when
// retention is non-positive.
func (c *DeliveryCleaner) Start(ctx context.Context) {
	if c.retention <= 0 {
		log.Printf("webhook-cleanup: disabled (retention <= 0)")
		return
	}

	log.Printf("webhook-cleanup: started (retention=%s, interval=%s)", c.retention, c.interval)

	// Initial run shortly after startup.
	select {
	case <-ctx.Done():
		return
	case <-time.After(c.firstRunDelay):
	}
	c.runOnce(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.runOnce(ctx)
		}
	}
}

func (c *DeliveryCleaner) runOnce(ctx context.Context) {
	cutoff := time.Now().Add(-c.retention)
	n, err := c.store.DeleteOldWebhookDeliveries(ctx, cutoff)
	if err != nil {
		log.Printf("webhook-cleanup: delete failed: %v", err)
		return
	}
	if n > 0 {
		log.Printf("webhook-cleanup: purged %d webhook_deliveries rows older than %s", n, cutoff.Format(time.RFC3339))
	}
}
