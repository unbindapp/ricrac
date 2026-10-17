package webhook_repo

import (
	"context"
	"encoding/json"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent/webhook"
	"ricrac/ent/webhookdelivery"
	repository "ricrac/internal/repositories"
)

// GiveUpDisabled gives up the pending deliveries of disabled webhooks.
func (self *WebhookRepository) GiveUpDisabled(ctx context.Context, tx repository.TxInterface, now time.Time) error {
	return self.base.Client(tx).WebhookDelivery.Update().
		Where(webhookdelivery.HasWebhookWith(webhook.Enabled(false)), webhookdelivery.DeliveredAtIsNil(), webhookdelivery.FailedAtIsNil()).
		SetFailedAt(now).SetError("The webhook is disabled").Exec(ctx)
}

// Due is a delivery to attempt, with where and how to send it.
type Due struct {
	ID       uuid.UUID
	Payload  json.RawMessage
	Attempts int
	URL      string
	Secret   string
	Enabled  bool
}

// ClaimDue locks up to limit pending deliveries that are due, oldest first, skipping ones
// another sender holds, and puts their next attempt off by five minutes.
func (self *WebhookRepository) ClaimDue(ctx context.Context, tx repository.TxInterface, limit int, now time.Time) ([]Due, error) {
	client := self.base.Client(tx)
	deliveries, err := client.WebhookDelivery.Query().
		Where(webhookdelivery.DeliveredAtIsNil(), webhookdelivery.FailedAtIsNil(), webhookdelivery.NextAttemptAtLTE(now)).
		Order(webhookdelivery.ByNextAttemptAt()).Limit(limit).
		ForUpdate(entsql.WithLockAction(entsql.SkipLocked)).
		All(ctx)
	if err != nil || len(deliveries) == 0 {
		return nil, err
	}
	ids := make([]uuid.UUID, len(deliveries))
	hooks := make([]uuid.UUID, len(deliveries))
	for i, d := range deliveries {
		ids[i], hooks[i] = d.ID, d.WebhookID
	}
	err = client.WebhookDelivery.Update().Where(webhookdelivery.IDIn(ids...)).SetNextAttemptAt(now.Add(5 * time.Minute)).Exec(ctx)
	if err != nil {
		return nil, err
	}
	targets, err := client.Webhook.Query().Where(webhook.IDIn(hooks...)).All(ctx)
	if err != nil {
		return nil, err
	}
	due := make([]Due, 0, len(deliveries))
	for _, d := range deliveries {
		for _, h := range targets {
			if h.ID == d.WebhookID {
				due = append(due, Due{ID: d.ID, Payload: json.RawMessage(d.Payload), Attempts: d.Attempts, URL: h.URL, Secret: h.Secret, Enabled: h.Enabled})
			}
		}
	}
	return due, nil
}

// Outcome is what an attempt got: a status code and an error, unless it was delivered.
type Outcome struct {
	OK     bool
	Status *int
	Error  *string
}

func (self *WebhookRepository) MarkDelivered(ctx context.Context, id uuid.UUID, status *int, attempts int, now time.Time) error {
	return self.base.DB.WebhookDelivery.Update().Where(webhookdelivery.ID(id)).
		SetDeliveredAt(now).SetNillableStatusCode(status).ClearError().SetAttempts(attempts).Exec(ctx)
}

// MarkFailed records a failed attempt: retried at retryAt, or given up without one.
func (self *WebhookRepository) MarkFailed(ctx context.Context, id uuid.UUID, outcome Outcome, attempts int, retryAt *time.Time, now time.Time) error {
	update := self.base.DB.WebhookDelivery.Update().Where(webhookdelivery.ID(id)).SetAttempts(attempts)
	if outcome.Status == nil {
		update.ClearStatusCode()
	} else {
		update.SetStatusCode(*outcome.Status)
	}
	if outcome.Error == nil {
		update.ClearError()
	} else {
		update.SetError(*outcome.Error)
	}
	if retryAt == nil {
		update.SetFailedAt(now)
	} else {
		update.SetNextAttemptAt(*retryAt).ClearFailedAt()
	}
	return update.Exec(ctx)
}
