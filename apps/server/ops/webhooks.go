package ops

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
)

// Webhooks: workspace admins register URLs that receive a signed POST for every change to the
// resource types they listen to. Deliveries are queued by emit inside the change's transaction
// and sent here, retrying with backoff.

var webhookResourceTypes = []string{
	"Issue",
	"Comment",
	"Reaction",
	"Attachment",
	"IssueLabel",
	"IssueRelation",
	"Project",
	"ProjectLabel",
	"ProjectMilestone",
	"ProjectUpdate",
	"Initiative",
	"InitiativeUpdate",
	"Cycle",
	"Document",
	"Team",
	"WorkflowState",
	"Customer",
	"CustomerNeed",
	"Release",
	"ReleasePipeline",
	"ReleaseNote",
}

// backoffMinutes is the wait before the 2nd, 3rd, ... attempt. After the last one the delivery is
// given up.
var backoffMinutes = []int64{1, 5, 30, 120, 360}

func webhookOps() []*Op {
	return []*Op{
		op("list_webhooks", app.AdminAccess, []string{"GET /webhooks"},
			"List the workspace's webhooks: URL, label, team, the resource types they receive and whether they are "+
				"enabled. Secrets are never shown. Admins only.",
			listWebhooks).returns(ResultOf[PageOf[Webhook]]()),
		op("get_webhook", app.AdminAccess, []string{"GET /webhooks/{id}"},
			"Read a webhook with its last 20 deliveries (action, type, status code, error, attempts and when it was "+
				"delivered or given up). Admins only.",
			getWebhook).returns(ResultOf[WebhookDetail]()),
		op("save_webhook", app.AdminAccess, []string{"POST /webhooks", "PATCH /webhooks/{id}"},
			"Create a webhook (omit id; url is required) or update one (pass id). Ricrac POSTs a JSON body "+
				"{action (create, update, remove), type (the resource type), data (the entity as the API shows it), "+
				"updatedFrom (previous values of changed fields, on update), url, actor, createdAt, workspaceId, webhookId, "+
				"webhookTimestamp (ms since epoch)} for every change to the resourceTypes it listens to, on one team or "+
				"all. Headers: Ricrac-Event (the type), Ricrac-Delivery (delivery id) and Ricrac-Signature: the hex "+
				"HMAC-SHA256 of the raw request body keyed with the webhook's secret. Receivers verify it by computing "+
				"the same HMAC over the exact bytes received, comparing in constant time, and rejecting bodies whose "+
				"webhookTimestamp is more than a minute old. The secret (rr_whs_...) is returned once, on create; "+
				"rotate_webhook_secret makes a new one. Answer with a 2xx within 10 seconds; anything else is retried "+
				"after 1m, 5m, 30m, 2h and 6h, then given up. Redirects aren't followed. Admins only.",
			saveWebhook).returns(ResultOf[SavedWebhook]()),
		op("rotate_webhook_secret", app.AdminAccess, []string{"POST /webhooks/{id}/secret"},
			"Replace a webhook's signing secret and return the new one (shown only now). Deliveries from now on are "+
				"signed with it. Admins only.",
			rotateWebhookSecret).returns(ResultOf[WebhookSecret]()),
		op("delete_webhook", app.AdminAccess, []string{"DELETE /webhooks/{id}"},
			"Delete a webhook and its queued deliveries. Admins only.",
			deleteWebhook).returns(ResultOf[DeletedWebhook]()),
		op("test_webhook", app.AdminAccess, []string{"POST /webhooks/{id}/test"},
			"Send a ping ({action: \"test\", type: \"Ping\", data: {webhookId}}) to a webhook right away and report "+
				"what the receiver answered. A failed ping isn't retried. Admins only.",
			testWebhook).returns(ResultOf[WebhookPing]()),
		op("list_webhook_deliveries", app.AdminAccess, []string{"GET /webhooks/{id}/deliveries"},
			"A webhook's deliveries, newest first: action, type, status code, error, attempts, the next attempt and "+
				"when it was delivered or given up. Deliveries are kept for 14 days. Admins only.",
			listDeliveries).returns(ResultOf[PageOf[WebhookDelivery]]()),
	}
}

/* Representation */

const webhookJSON = `jsonb_build_object(
	'id', h.id,
	'url', h.url,
	'label', h.label,
	'team', team_ref(h.team_id),
	'resourceTypes', to_jsonb(h.resource_types),
	'enabled', h.enabled,
	'creator', user_ref(h.creator_id),
	'createdAt', h.created_at,
	'updatedAt', h.updated_at
)`

const deliveryJSON = `jsonb_build_object(
	'id', d.id,
	'action', d.payload->>'action',
	'type', d.payload->>'type',
	'statusCode', d.status_code,
	'error', d.error,
	'attempts', d.attempts,
	'nextAttemptAt', CASE WHEN d.delivered_at IS NULL AND d.failed_at IS NULL THEN d.next_attempt_at END,
	'deliveredAt', d.delivered_at,
	'failedAt', d.failed_at,
	'createdAt', d.created_at
)`

func webhookValue(c *app.Ctx, q app.Querier, id uuid.UUID) (M, error) {
	value, found, err := app.Maybe[M](c, q, "SELECT "+webhookJSON+" FROM webhooks h WHERE h.id = $1", id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, app.NotFound("No such webhook")
	}
	return value, nil
}

func webhookID(c *app.Ctx, value string) (uuid.UUID, error) {
	return resolveByID(c, "webhooks", "webhook", value)
}

/* Read */

type ListWebhooksArgs struct {
	Page
}

func listWebhooks(c *app.Ctx, args ListWebhooksArgs) (any, error) {
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	nodes, err := app.All[any](c, c.DB(),
		"SELECT "+webhookJSON+" FROM webhooks h WHERE h.workspace_id = $1 ORDER BY h.created_at, h.id LIMIT $2 OFFSET $3",
		c.Workspace, args.limit()+1, offset)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

func getWebhook(c *app.Ctx, args IDArgs) (any, error) {
	id, err := webhookID(c, args.ID)
	if err != nil {
		return nil, err
	}
	value, err := webhookValue(c, c.DB(), id)
	if err != nil {
		return nil, err
	}
	if value["deliveries"], err = app.One[any](c, c.DB(),
		`SELECT coalesce(jsonb_agg(x.v ORDER BY x.created_at DESC), '[]') FROM (
			SELECT `+deliveryJSON+` AS v, d.created_at FROM webhook_deliveries d WHERE d.webhook_id = $1
			ORDER BY d.created_at DESC LIMIT 20) x`, id); err != nil {
		return nil, err
	}
	return value, nil
}

type ListDeliveriesArgs struct {
	ID string `json:"id" doc:"The webhook's id."`
	Page
}

func listDeliveries(c *app.Ctx, args ListDeliveriesArgs) (any, error) {
	id, err := webhookID(c, args.ID)
	if err != nil {
		return nil, err
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	nodes, err := app.All[any](c, c.DB(),
		`SELECT `+deliveryJSON+` FROM webhook_deliveries d WHERE d.webhook_id = $1
		 ORDER BY d.created_at DESC, d.id LIMIT $2 OFFSET $3`, id, args.limit()+1, offset)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

/* Save */

type SaveWebhookArgs struct {
	ID            *string          `json:"id" doc:"Only to update a webhook: its id. Omit to create one."`
	URL           *string          `json:"url" doc:"Where to POST: https, or http only for localhost / 127.0.0.1. Required to create."`
	Label         Nullable[string] `json:"label" doc:"A name to recognize it by; null clears it."`
	ResourceTypes []string         `json:"resourceTypes" doc:"What to receive: Issue, Comment, Reaction, Attachment, IssueLabel, IssueRelation, Project, ProjectLabel, ProjectMilestone, ProjectUpdate, Initiative, InitiativeUpdate, Cycle, Document, Team, WorkflowState, Customer, CustomerNeed, Release, ReleasePipeline, ReleaseNote. Issue and Comment by default."`
	Team          Nullable[string] `json:"team" doc:"Only changes in this team (key, name or id); null for every team."`
	Enabled       *bool            `json:"enabled" doc:"Disabled webhooks receive nothing; queued deliveries are given up."`
}

func checkWebhookURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	invalid := app.BadRequest("%q isn't a valid webhook URL; use https:// (http:// only for localhost or 127.0.0.1)", value)
	parsed, err := url.Parse(value)
	if err != nil {
		return "", invalid
	}
	host := strings.ToLower(parsed.Hostname())
	switch strings.ToLower(parsed.Scheme) {
	case "https":
		if host != "" {
			return value, nil
		}
	case "http":
		if host == "localhost" || host == "127.0.0.1" {
			return value, nil
		}
	}
	return "", invalid
}

func checkResourceTypes(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, app.BadRequest("Pass at least one resource type")
	}
	types := []string{}
	for _, value := range values {
		index := slices.IndexFunc(webhookResourceTypes, func(t string) bool { return strings.EqualFold(t, strings.TrimSpace(value)) })
		if index < 0 {
			return nil, app.BadRequest("%q isn't a resource type. Options: %s.", value, strings.Join(webhookResourceTypes, ", "))
		}
		if !slices.Contains(types, webhookResourceTypes[index]) {
			types = append(types, webhookResourceTypes[index])
		}
	}
	return types, nil
}

func saveWebhook(c *app.Ctx, args SaveWebhookArgs) (any, error) {
	team, err := resolveNullable(args.Team, func(v string) (uuid.UUID, error) { return resolveTeam(c, v) })
	if err != nil {
		return nil, err
	}
	var hookURL *string
	if args.URL != nil {
		checked, err := checkWebhookURL(*args.URL)
		if err != nil {
			return nil, err
		}
		hookURL = &checked
	}
	var types []string
	if args.ResourceTypes != nil {
		if types, err = checkResourceTypes(args.ResourceTypes); err != nil {
			return nil, err
		}
	}
	label := trimNullable(args.Label)
	if args.ID == nil {
		return createWebhook(c, hookURL, label, types, team, args.Enabled)
	}
	id, err := webhookID(c, *args.ID)
	if err != nil {
		return nil, err
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		err := app.Exec(c, tx,
			`UPDATE webhooks SET
				url = coalesce($2, url),
				label = CASE WHEN $3 THEN $4 ELSE label END,
				resource_types = coalesce($5, resource_types),
				team_id = CASE WHEN $6 THEN $7 ELSE team_id END,
				enabled = coalesce($8, enabled),
				updated_at = now()
			 WHERE id = $1`,
			id, hookURL, label.Set, label.Ptr(), types, team.Set, team.Ptr(), args.Enabled)
		if err != nil {
			return err
		}
		if err := audit(c, tx, "webhook.updated", M{"webhookId": id}); err != nil {
			return err
		}
		value, err = webhookValue(c, tx, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func createWebhook(c *app.Ctx, hookURL *string, label Nullable[string], types []string, team Nullable[uuid.UUID], enabled *bool) (any, error) {
	if hookURL == nil {
		return nil, app.BadRequest("url is required to create a webhook")
	}
	secret := app.RandomSecret("rr_whs_")
	if types == nil {
		types = []string{"Issue", "Comment"}
	}
	var value M
	err := inTx(c, func(tx pgx.Tx) error {
		id, err := app.One[uuid.UUID](c, tx,
			`INSERT INTO webhooks (workspace_id, team_id, url, label, secret, resource_types, enabled, creator_id)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
			c.Workspace, team.Ptr(), *hookURL, label.Ptr(), secret, types, enabled == nil || *enabled, c.User)
		if err != nil {
			return err
		}
		if err := audit(c, tx, "webhook.created", M{"webhookId": id, "url": *hookURL}); err != nil {
			return err
		}
		value, err = webhookValue(c, tx, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	value["secret"] = secret
	return value, nil
}

func rotateWebhookSecret(c *app.Ctx, args IDArgs) (any, error) {
	id, err := webhookID(c, args.ID)
	if err != nil {
		return nil, err
	}
	secret := app.RandomSecret("rr_whs_")
	err = inTx(c, func(tx pgx.Tx) error {
		if err := app.Exec(c, tx, "UPDATE webhooks SET secret = $2, updated_at = now() WHERE id = $1", id, secret); err != nil {
			return err
		}
		return audit(c, tx, "webhook.secretRotated", M{"webhookId": id})
	})
	if err != nil {
		return nil, err
	}
	return M{"id": id, "secret": secret}, nil
}

func deleteWebhook(c *app.Ctx, args IDArgs) (any, error) {
	id, err := webhookID(c, args.ID)
	if err != nil {
		return nil, err
	}
	err = inTx(c, func(tx pgx.Tx) error {
		if err := app.Exec(c, tx, "DELETE FROM webhooks WHERE id = $1", id); err != nil {
			return err
		}
		return audit(c, tx, "webhook.deleted", M{"webhookId": id})
	})
	if err != nil {
		return nil, err
	}
	return M{"id": id, "deleted": true}, nil
}

func testWebhook(c *app.Ctx, args IDArgs) (any, error) {
	id, err := webhookID(c, args.ID)
	if err != nil {
		return nil, err
	}
	payload := M{
		"action":      "test",
		"type":        "Ping",
		"data":        M{"webhookId": id},
		"url":         nil,
		"actor":       M{"id": c.User},
		"createdAt":   time.Now().UTC(),
		"workspaceId": c.Workspace,
		"webhookId":   id,
	}
	delivery, err := app.One[uuid.UUID](c, c.DB(),
		"INSERT INTO webhook_deliveries (webhook_id, payload, next_attempt_at) VALUES ($1, $2, 'infinity') RETURNING id", id, payload)
	if err != nil {
		return nil, err
	}
	type target struct{ URL, Secret string }
	hook, _, err := app.Row[target](c, c.DB(), "SELECT url, secret FROM webhooks WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	due := dueDelivery{ID: delivery, Payload: payload, URL: hook.URL, Secret: hook.Secret, Enabled: true}
	outcome := sendDelivery(c, c.State, due)
	if err := recordDelivery(c, c.State, due, outcome, false); err != nil {
		return nil, err
	}
	return M{"deliveryId": delivery, "delivered": outcome.ok, "statusCode": outcome.status, "error": outcome.error}, nil
}

/* Delivery */

type dueDelivery struct {
	ID       uuid.UUID
	Payload  M
	Attempts int32
	URL      string
	Secret   string
	Enabled  bool
}

type deliveryOutcome struct {
	ok     bool
	status *int32
	error  *string
}

// webhookSignature is the hex HMAC-SHA256 of body keyed with secret, sent as Ricrac-Signature.
func webhookSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func truncateError(text string) *string {
	runes := []rune(text)
	return new(string(runes[:min(len(runes), 500)]))
}

// webhookClient is the server's client, waiting at most 10 seconds and not following redirects.
func webhookClient(state *app.State) *http.Client {
	client := *state.HTTP
	client.Timeout = 10 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &client
}

func sendDelivery(ctx context.Context, state *app.State, due dueDelivery) deliveryOutcome {
	payload := maps.Clone(due.Payload)
	payload["webhookTimestamp"] = time.Now().UnixMilli()
	body := app.Marshal(payload)
	event, _ := payload["type"].(string)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, due.URL, bytes.NewReader(body))
	if err != nil {
		return deliveryOutcome{error: truncateError(err.Error())}
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Ricrac-Webhook/1.0")
	request.Header.Set("Ricrac-Signature", webhookSignature(due.Secret, body))
	request.Header.Set("Ricrac-Event", event)
	request.Header.Set("Ricrac-Delivery", due.ID.String())
	response, err := webhookClient(state).Do(request)
	if err != nil {
		return deliveryOutcome{error: truncateError(err.Error())}
	}
	defer response.Body.Close()
	status := int32(response.StatusCode)
	if status >= 200 && status < 300 {
		return deliveryOutcome{ok: true, status: &status}
	}
	text, _ := io.ReadAll(response.Body)
	message := "HTTP " + response.Status
	if trimmed := strings.TrimSpace(string(text)); trimmed != "" {
		message += ": " + trimmed
	}
	return deliveryOutcome{status: &status, error: truncateError(message)}
}

// recordDelivery saves the result of an attempt: delivered, retried later with backoff, or given
// up.
func recordDelivery(ctx context.Context, state *app.State, due dueDelivery, outcome deliveryOutcome, retry bool) error {
	attempts := due.Attempts + 1
	if outcome.ok {
		return app.Exec(ctx, state.DB,
			"UPDATE webhook_deliveries SET delivered_at = now(), status_code = $2, error = NULL, attempts = $3 WHERE id = $1",
			due.ID, outcome.status, attempts)
	}
	var wait *int64
	if retry && int(attempts) <= len(backoffMinutes) {
		wait = &backoffMinutes[attempts-1]
	}
	return app.Exec(ctx, state.DB,
		`UPDATE webhook_deliveries SET attempts = $2, status_code = $3, error = $4,
			next_attempt_at = CASE WHEN $5::bigint IS NULL THEN next_attempt_at ELSE now() + make_interval(mins => $5::int) END,
			failed_at = CASE WHEN $5::bigint IS NULL THEN now() END
		 WHERE id = $1`,
		due.ID, attempts, outcome.status, outcome.error, wait)
}

// DeliverDueWebhooks sends queued webhook deliveries that are due, retrying failures with backoff.
// Returns how many were attempted.
func DeliverDueWebhooks(ctx context.Context, state *app.State) (int, error) {
	var due []dueDelivery
	err := pgx.BeginFunc(ctx, state.DB, func(tx pgx.Tx) error {
		if err := app.Exec(ctx, tx,
			`UPDATE webhook_deliveries d SET failed_at = now(), error = 'The webhook is disabled'
			 FROM webhooks h WHERE h.id = d.webhook_id AND NOT h.enabled AND d.delivered_at IS NULL AND d.failed_at IS NULL`); err != nil {
			return err
		}
		var err error
		due, err = app.Rows[dueDelivery](ctx, tx,
			`WITH due AS (
				SELECT id FROM webhook_deliveries
				WHERE delivered_at IS NULL AND failed_at IS NULL AND next_attempt_at <= now()
				ORDER BY next_attempt_at LIMIT 20 FOR UPDATE SKIP LOCKED)
			 UPDATE webhook_deliveries d SET next_attempt_at = now() + interval '5 minutes'
			 FROM due, webhooks h WHERE d.id = due.id AND h.id = d.webhook_id
			 RETURNING d.id, d.payload, d.attempts, h.url, h.secret, h.enabled`)
		return err
	})
	if err != nil {
		return 0, err
	}
	due = slices.DeleteFunc(due, func(d dueDelivery) bool { return !d.Enabled })
	errs := make([]error, len(due))
	var wg sync.WaitGroup
	for i, delivery := range due {
		wg.Go(func() {
			outcome := sendDelivery(ctx, state, delivery)
			errs[i] = recordDelivery(ctx, state, delivery, outcome, true)
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return 0, err
	}
	return len(due), nil
}

/* Shapes */

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
