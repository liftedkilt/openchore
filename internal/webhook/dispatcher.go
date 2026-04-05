package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

// Event types
const (
	EventChoreCompleted   = "chore.completed"
	EventChoreUncompleted = "chore.uncompleted"
	EventChoreExpired     = "chore.expired"
	EventRewardRedeemed   = "reward.redeemed"
	EventDailyComplete    = "daily.complete"
	EventStreakMilestone   = "streak.milestone"
	EventPointsDecayed       = "points.decayed"
	EventChoreMissed         = "chore.missed"
	EventChoreFCFSCompleted  = "chore.fcfs_completed"
)

type Dispatcher struct {
	store  *store.Store
	client *http.Client
}

func NewDispatcher(s *store.Store) *Dispatcher {
	return &Dispatcher{
		store: s,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Payload is the standard webhook payload
type Payload struct {
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
}

// Fire sends an event to all matching webhooks asynchronously
func (d *Dispatcher) Fire(event string, data any) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		webhooks, err := d.store.GetActiveWebhooksForEvent(ctx, event)
		if err != nil {
			log.Printf("webhook: failed to get webhooks for event %s: %v", event, err)
			return
		}

		payload := Payload{
			Event:     event,
			Timestamp: time.Now().UTC(),
			Data:      data,
		}

		body, err := json.Marshal(payload)
		if err != nil {
			log.Printf("webhook: failed to marshal payload: %v", err)
			return
		}

		for _, wh := range webhooks {
			d.deliver(ctx, wh, event, body)
		}
	}()
}

func (d *Dispatcher) deliver(ctx context.Context, wh model.Webhook, event string, body []byte) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, bytes.NewReader(body))
	if err != nil {
		d.logDelivery(ctx, wh.ID, event, string(body), nil, "", err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OpenChore-Event", event)

	// HMAC signature if secret is set
	if wh.Secret != "" {
		mac := hmac.New(sha256.New, []byte(wh.Secret))
		mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-OpenChore-Signature", fmt.Sprintf("sha256=%s", sig))
	}

	resp, err := d.client.Do(req)
	if err != nil {
		d.logDelivery(ctx, wh.ID, event, string(body), nil, "", err.Error())
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	d.logDelivery(ctx, wh.ID, event, string(body), &resp.StatusCode, string(respBody), "")
}

func (d *Dispatcher) logDelivery(ctx context.Context, webhookID int64, event, payload string, statusCode *int, responseBody, errMsg string) {
	delivery := &model.WebhookDelivery{
		WebhookID:    webhookID,
		Event:        event,
		Payload:      payload,
		ResponseBody: responseBody,
		Error:        errMsg,
	}
	if statusCode != nil {
		delivery.StatusCode = statusCode
	}
	if err := d.store.LogWebhookDelivery(ctx, delivery); err != nil {
		log.Printf("webhook: failed to log delivery: %v", err)
	}
}
