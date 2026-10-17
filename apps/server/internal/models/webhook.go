package models

import (
	"encoding/json"

	"ricrac/ent"
)

type Webhook struct {
	ID            string   `json:"id"`
	URL           string   `json:"url"`
	Label         *string  `json:"label"`
	Team          *TeamRef `json:"team" doc:"Only changes in this team; null for every team."`
	ResourceTypes []string `json:"resourceTypes"`
	Enabled       bool     `json:"enabled"`
	Creator       *UserRef `json:"creator"`
	CreatedAt     string   `json:"createdAt"`
	UpdatedAt     string   `json:"updatedAt"`
}

type SavedWebhook struct {
	Webhook
	Secret string `json:"secret,omitempty" doc:"The signing secret, only when the webhook is created."`
}

type WebhookDetail struct {
	Webhook
	Deliveries []WebhookDelivery `json:"deliveries" doc:"The last 20, newest first."`
}

type WebhookDelivery struct {
	ID            string  `json:"id"`
	Action        string  `json:"action" enum:"create,update,remove,test"`
	Type          string  `json:"type" doc:"The resource type, or Ping for test_webhook."`
	StatusCode    *int    `json:"statusCode"`
	Error         *string `json:"error"`
	Attempts      int     `json:"attempts"`
	NextAttemptAt *string `json:"nextAttemptAt" doc:"While it is still pending."`
	DeliveredAt   *string `json:"deliveredAt"`
	FailedAt      *string `json:"failedAt" doc:"When it was given up."`
	CreatedAt     string  `json:"createdAt"`
}

type WebhookSecret struct {
	ID     string `json:"id"`
	Secret string `json:"secret"`
}

type WebhookPing struct {
	DeliveryID string  `json:"deliveryId"`
	Delivered  bool    `json:"delivered"`
	StatusCode *int    `json:"statusCode"`
	Error      *string `json:"error"`
}

type DeletedWebhook struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// WebhookOf needs the webhook's team and creator loaded.
func WebhookOf(h *ent.Webhook) Webhook {
	return Webhook{
		ID: ID(h.ID), URL: h.URL, Label: h.Label, Team: TeamRefOf(h.Edges.Team), ResourceTypes: nonNilStrings(h.ResourceTypes),
		Enabled: h.Enabled, Creator: UserRefOf(h.Edges.Creator), CreatedAt: Time(h.CreatedAt), UpdatedAt: Time(h.UpdatedAt),
	}
}

func WebhookDeliveryOf(d *ent.WebhookDelivery) WebhookDelivery {
	var payload struct{ Action, Type string }
	_ = json.Unmarshal(d.Payload, &payload)
	delivery := WebhookDelivery{
		ID: ID(d.ID), Action: payload.Action, Type: payload.Type, StatusCode: d.StatusCode, Error: d.Error,
		Attempts: d.Attempts, DeliveredAt: TimePtr(d.DeliveredAt), FailedAt: TimePtr(d.FailedAt), CreatedAt: Time(d.CreatedAt),
	}
	if d.DeliveredAt == nil && d.FailedAt == nil {
		delivery.NextAttemptAt = TimePtr(&d.NextAttemptAt)
	}
	return delivery
}
