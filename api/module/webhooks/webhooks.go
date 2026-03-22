// Package webhooks provides the webhook dispatcher service. It finds active
// webhooks subscribed to an event, creates delivery records, and delivers
// them asynchronously via the queue.
package webhooks

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/stanza-go/framework/pkg/log"
	"github.com/stanza-go/framework/pkg/queue"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/framework/pkg/webhook"
)

const queueType = "webhook.deliver"

// Dispatcher sends webhook events to subscribed endpoints.
type Dispatcher struct {
	db     *sqlite.DB
	queue  *queue.Queue
	client *webhook.Client
	logger *log.Logger
}

// NewDispatcher creates a new webhook dispatcher and registers the queue
// handler for processing deliveries.
func NewDispatcher(db *sqlite.DB, q *queue.Queue, logger *log.Logger) *Dispatcher {
	d := &Dispatcher{
		db:     db,
		queue:  q,
		client: webhook.NewClient(),
		logger: logger,
	}

	q.Register(queueType, d.processDelivery)

	return d
}

// Stats returns the underlying webhook client delivery counters.
func (d *Dispatcher) Stats() webhook.ClientStats {
	return d.client.Stats()
}

// deliveryJob is the payload enqueued for each webhook delivery.
type deliveryJob struct {
	DeliveryID int64  `json:"delivery_id"`
	WebhookID  int64  `json:"webhook_id"`
	URL        string `json:"url"`
	Secret     string `json:"secret"`
	Event      string `json:"event"`
	Payload    string `json:"payload"`
}

// Dispatch sends an event to all active webhooks that are subscribed to it.
// Delivery is asynchronous — jobs are enqueued and processed by the queue
// worker.
func (d *Dispatcher) Dispatch(ctx context.Context, event string, payload any) error {
	if d == nil {
		return nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Collect matching webhooks first, then close rows before doing any
	// writes. SQLite uses a single connection — holding rows open while
	// calling Exec would deadlock.
	type target struct {
		id     int64
		url    string
		secret string
	}

	rows, err := d.db.Query(
		"SELECT id, url, secret, events FROM webhooks WHERE is_active = 1",
	)
	if err != nil {
		return err
	}

	var targets []target
	for rows.Next() {
		var id int64
		var url, secret, eventsJSON string
		if err := rows.Scan(&id, &url, &secret, &eventsJSON); err != nil {
			continue
		}
		if matchesEvent(eventsJSON, event) {
			targets = append(targets, target{id: id, url: url, secret: secret})
		}
	}
	_ = rows.Err()
	rows.Close()

	now := time.Now().UTC().Format(time.RFC3339)

	for _, t := range targets {
		// Create delivery record.
		res, err := d.db.Exec(
			"INSERT INTO webhook_deliveries (webhook_id, event, payload, status, created_at) VALUES (?, ?, ?, 'pending', ?)",
			t.id, event, string(body), now,
		)
		if err != nil {
			d.logger.Error("webhook: create delivery record",
				log.Int64("webhook_id", t.id),
				log.Err(err),
			)
			continue
		}

		job := deliveryJob{
			DeliveryID: res.LastInsertID,
			WebhookID:  t.id,
			URL:        t.url,
			Secret:     t.secret,
			Event:      event,
			Payload:    string(body),
		}

		jobPayload, err := json.Marshal(job)
		if err != nil {
			continue
		}

		if _, err := d.queue.Enqueue(ctx, queueType, jobPayload, queue.MaxAttempts(4)); err != nil {
			d.logger.Error("webhook: enqueue delivery",
				log.Int64("webhook_id", t.id),
				log.Err(err),
			)
		}
	}

	return nil
}

// processDelivery handles a queued webhook delivery job.
func (d *Dispatcher) processDelivery(ctx context.Context, payload []byte) error {
	var job deliveryJob
	if err := json.Unmarshal(payload, &job); err != nil {
		return err
	}

	result, err := d.client.Send(ctx, &webhook.Delivery{
		URL:     job.URL,
		Secret:  job.Secret,
		Event:   job.Event,
		Payload: []byte(job.Payload),
	})

	now := time.Now().UTC().Format(time.RFC3339)

	if err != nil {
		// Update delivery record as failed.
		_, _ = d.db.Exec(
			"UPDATE webhook_deliveries SET status = 'failed', response_body = ?, attempts = attempts + 1, completed_at = ? WHERE id = ?",
			err.Error(), now, job.DeliveryID,
		)
		return err
	}

	status := "success"
	if result.StatusCode < 200 || result.StatusCode >= 300 {
		status = "failed"
	}

	respBody := result.Body
	if len(respBody) > 4096 {
		respBody = respBody[:4096]
	}

	_, _ = d.db.Exec(
		"UPDATE webhook_deliveries SET status = ?, status_code = ?, response_body = ?, delivery_id = ?, attempts = attempts + 1, completed_at = ? WHERE id = ?",
		status, result.StatusCode, respBody, result.DeliveryID, now, job.DeliveryID,
	)

	if status == "failed" {
		return &DeliveryError{StatusCode: result.StatusCode}
	}

	return nil
}

// DeliveryError is returned when a webhook delivery gets a non-2xx response,
// causing the queue to retry the job.
type DeliveryError struct {
	StatusCode int
}

func (e *DeliveryError) Error() string {
	return "webhook: delivery failed with status " + strconv.Itoa(e.StatusCode)
}

// GenerateSecret creates a random webhook secret in the format "whsec_<hex>".
func GenerateSecret() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return "whsec_" + hex.EncodeToString(b)
}

// matchesEvent checks if a JSON array of event patterns matches the given event.
// Supports "*" wildcard for all events, and prefix matching with "*" suffix
// (e.g., "user.*" matches "user.created").
func matchesEvent(eventsJSON, event string) bool {
	var events []string
	if err := json.Unmarshal([]byte(eventsJSON), &events); err != nil {
		return false
	}

	for _, pattern := range events {
		if pattern == "*" {
			return true
		}
		if pattern == event {
			return true
		}
		if strings.HasSuffix(pattern, ".*") {
			prefix := strings.TrimSuffix(pattern, ".*")
			if strings.HasPrefix(event, prefix+".") {
				return true
			}
		}
	}

	return false
}
