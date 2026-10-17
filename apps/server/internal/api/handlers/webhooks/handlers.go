package webhooks_handler

import (
	"ricrac/internal/api/oapi"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
)

func RegisterHandlers(srv *server.Server, reg *oapi.Registry) {
	webhooks := srv.WebhookService
	oapi.Register(reg, oapi.Operation{
		Name: "list_webhooks", Access: auth.AdminAccess, Routes: []string{"GET /webhooks"},
		Description: "List the workspace's webhooks: URL, label, team, the resource types they receive and whether they are " +
			"enabled. Secrets are never shown. Admins only.",
	}, webhooks.List)
	oapi.Register(reg, oapi.Operation{
		Name: "get_webhook", Access: auth.AdminAccess, Routes: []string{"GET /webhooks/{id}"},
		Description: "Read a webhook with its last 20 deliveries (action, type, status code, error, attempts and when it was " +
			"delivered or given up). Admins only.",
	}, webhooks.Get)
	oapi.Register(reg, oapi.Operation{
		Name: "save_webhook", Access: auth.AdminAccess, Routes: []string{"POST /webhooks", "PATCH /webhooks/{id}"},
		Description: "Create a webhook (omit id; url is required) or update one (pass id). Ricrac POSTs a JSON body " +
			"{action (create, update, remove), type (the resource type), data (the entity as the API shows it), " +
			"updatedFrom (previous values of changed fields, on update), url, actor, createdAt, workspaceId, webhookId, " +
			"webhookTimestamp (ms since epoch)} for every change to the resourceTypes it listens to, on one team or " +
			"all. Headers: Ricrac-Event (the type), Ricrac-Delivery (delivery id) and Ricrac-Signature: the hex " +
			"HMAC-SHA256 of the raw request body keyed with the webhook's secret. Receivers verify it by computing " +
			"the same HMAC over the exact bytes received, comparing in constant time, and rejecting bodies whose " +
			"webhookTimestamp is more than a minute old. The secret (rr_whs_...) is returned once, on create; " +
			"rotate_webhook_secret makes a new one. Answer with a 2xx within 10 seconds; anything else is retried " +
			"after 1m, 5m, 30m, 2h and 6h, then given up. Redirects aren't followed. Admins only.",
	}, webhooks.Save)
	oapi.Register(reg, oapi.Operation{
		Name: "rotate_webhook_secret", Access: auth.AdminAccess, Routes: []string{"POST /webhooks/{id}/secret"},
		Description: "Replace a webhook's signing secret and return the new one (shown only now). Deliveries from now on are " +
			"signed with it. Admins only.",
	}, webhooks.RotateSecret)
	oapi.Register(reg, oapi.Operation{
		Name: "delete_webhook", Access: auth.AdminAccess, Routes: []string{"DELETE /webhooks/{id}"},
		Description: "Delete a webhook and its queued deliveries. Admins only.",
	}, webhooks.Delete)
	oapi.Register(reg, oapi.Operation{
		Name: "test_webhook", Access: auth.AdminAccess, Routes: []string{"POST /webhooks/{id}/test"},
		Description: "Send a ping ({action: \"test\", type: \"Ping\", data: {webhookId}}) to a webhook right away and report " +
			"what the receiver answered. A failed ping isn't retried. Admins only.",
	}, webhooks.Test)
	oapi.Register(reg, oapi.Operation{
		Name: "list_webhook_deliveries", Access: auth.AdminAccess, Routes: []string{"GET /webhooks/{id}/deliveries"},
		Description: "A webhook's deliveries, newest first: action, type, status code, error, attempts, the next attempt and " +
			"when it was delivered or given up. Deliveries are kept for 14 days. Admins only.",
	}, webhooks.ListDeliveries)
}
