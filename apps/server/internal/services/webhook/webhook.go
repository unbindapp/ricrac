// Package webhook_service is webhooks: workspace admins register URLs that receive a signed POST
// for every change to the resource types they listen to. Deliveries are queued by Emit inside the
// change's transaction and sent here, retrying with backoff.
package webhook_service

import (
	"context"
	"encoding/json"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	"ricrac/internal/repositories/repositories"
	webhook_repo "ricrac/internal/repositories/webhook"
	events_service "ricrac/internal/services/events"
	resolve_service "ricrac/internal/services/resolve"
)

var resourceTypes = []string{
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
	"Discussion",
	"DiscussionThread",
	"Team",
	"WorkflowState",
	"Customer",
	"CustomerNeed",
	"Release",
	"ReleasePipeline",
	"ReleaseNote",
}

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i WebhookServiceInterface -p webhook_service -s WebhookService -o webhook_service_iface.go
type WebhookService struct {
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
	events  events_service.EventsServiceInterface
	sender  *sender
}

func NewWebhookService(repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface) *WebhookService {
	return &WebhookService{repo: repo, resolve: resolve, events: events, sender: newSender()}
}

type IDInput struct {
	ID string `json:"id"`
}

type ListInput struct {
	args.Page
}

type ListDeliveriesInput struct {
	ID string `json:"id" doc:"The webhook's id."`
	args.Page
}

type SaveInput struct {
	ID            *string               `json:"id" doc:"Only to update a webhook: its id. Omit to create one."`
	URL           *string               `json:"url" doc:"Where to POST: https, or http only for localhost / 127.0.0.1. Required to create."`
	Label         args.Nullable[string] `json:"label" doc:"A name to recognize it by; null clears it."`
	ResourceTypes []string              `json:"resourceTypes" doc:"What to receive: Issue, Comment, Reaction, Attachment, IssueLabel, IssueRelation, Project, ProjectLabel, ProjectMilestone, ProjectUpdate, Initiative, InitiativeUpdate, Cycle, Document, Team, WorkflowState, Customer, CustomerNeed, Release, ReleasePipeline, ReleaseNote. Issue and Comment by default."`
	Team          args.Nullable[string] `json:"team" doc:"Only changes in this team (key, name or id); null for every team."`
	Enabled       *bool                 `json:"enabled" doc:"Disabled webhooks receive nothing; queued deliveries are given up."`
}

func (self *WebhookService) webhooks() webhook_repo.WebhookRepositoryInterface {
	return self.repo.Webhook()
}

func (self *WebhookService) value(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.Webhook, error) {
	hook, err := self.webhooks().Load(ctx, tx, id)
	if err != nil {
		return models.Webhook{}, err
	}
	if hook == nil {
		return models.Webhook{}, errdefs.NotFound("No such webhook")
	}
	return models.WebhookOf(hook), nil
}

func (self *WebhookService) webhookID(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	return self.resolve.ByID(ctx, actor, "webhooks", "webhook", value)
}

func (self *WebhookService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.Webhook], error) {
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.Webhook]{}, err
	}
	hooks, err := self.webhooks().List(ctx, actor.Workspace, input.Size()+1, offset)
	if err != nil {
		return args.PageOf[models.Webhook]{}, err
	}
	values := make([]models.Webhook, len(hooks))
	for i, h := range hooks {
		values[i] = models.WebhookOf(h)
	}
	return args.WrapOf(input.Page, values)
}

func (self *WebhookService) Get(ctx context.Context, actor *auth.Actor, input *IDInput) (models.WebhookDetail, error) {
	id, err := self.webhookID(ctx, actor, input.ID)
	if err != nil {
		return models.WebhookDetail{}, err
	}
	hook, err := self.value(ctx, nil, id)
	if err != nil {
		return models.WebhookDetail{}, err
	}
	deliveries, err := self.webhooks().LastDeliveries(ctx, id, 20)
	if err != nil {
		return models.WebhookDetail{}, err
	}
	detail := models.WebhookDetail{Webhook: hook, Deliveries: []models.WebhookDelivery{}}
	for _, d := range deliveries {
		detail.Deliveries = append(detail.Deliveries, models.WebhookDeliveryOf(d))
	}
	return detail, nil
}

func (self *WebhookService) ListDeliveries(ctx context.Context, actor *auth.Actor, input *ListDeliveriesInput) (args.PageOf[models.WebhookDelivery], error) {
	id, err := self.webhookID(ctx, actor, input.ID)
	if err != nil {
		return args.PageOf[models.WebhookDelivery]{}, err
	}
	offset, err := input.Offset()
	if err != nil {
		return args.PageOf[models.WebhookDelivery]{}, err
	}
	deliveries, err := self.webhooks().Deliveries(ctx, id, input.Size()+1, offset)
	if err != nil {
		return args.PageOf[models.WebhookDelivery]{}, err
	}
	values := make([]models.WebhookDelivery, len(deliveries))
	for i, d := range deliveries {
		values[i] = models.WebhookDeliveryOf(d)
	}
	return args.WrapOf(input.Page, values)
}

func checkURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	invalid := errdefs.BadRequest("%q isn't a valid webhook URL; use https:// (http:// only for localhost or 127.0.0.1)", value)
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
		return nil, errdefs.BadRequest("Pass at least one resource type")
	}
	types := []string{}
	for _, value := range values {
		index := slices.IndexFunc(resourceTypes, func(t string) bool { return strings.EqualFold(t, strings.TrimSpace(value)) })
		if index < 0 {
			return nil, errdefs.BadRequest("%q isn't a resource type. Options: %s.", value, strings.Join(resourceTypes, ", "))
		}
		if !slices.Contains(types, resourceTypes[index]) {
			types = append(types, resourceTypes[index])
		}
	}
	return types, nil
}

// trimNullable trims a nullable text, turning a blank one into null.
func trimNullable(value args.Nullable[string]) args.Nullable[string] {
	if !value.Valid {
		return value
	}
	if trimmed := strings.TrimSpace(value.Value); trimmed != "" {
		return args.Nullable[string]{Set: true, Valid: true, Value: trimmed}
	}
	return args.Nullable[string]{Set: true}
}

func (self *WebhookService) Save(ctx context.Context, actor *auth.Actor, input *SaveInput) (models.SavedWebhook, error) {
	team := args.Nullable[uuid.UUID]{Set: input.Team.Set}
	if input.Team.Valid {
		id, err := self.resolve.Team(ctx, actor, input.Team.Value)
		if err != nil {
			return models.SavedWebhook{}, err
		}
		team = args.Nullable[uuid.UUID]{Set: true, Valid: true, Value: id}
	}
	var hookURL *string
	if input.URL != nil {
		checked, err := checkURL(*input.URL)
		if err != nil {
			return models.SavedWebhook{}, err
		}
		hookURL = &checked
	}
	var types []string
	if input.ResourceTypes != nil {
		var err error
		if types, err = checkResourceTypes(input.ResourceTypes); err != nil {
			return models.SavedWebhook{}, err
		}
	}
	label := trimNullable(input.Label)
	if input.ID == nil {
		return self.create(ctx, actor, hookURL, label, types, team, input.Enabled)
	}
	id, err := self.webhookID(ctx, actor, *input.ID)
	if err != nil {
		return models.SavedWebhook{}, err
	}
	var value models.Webhook
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		err = self.webhooks().Update(ctx, tx, id, webhook_repo.UpdateInput{
			URL: hookURL, Label: label, ResourceTypes: types, Team: team, Enabled: input.Enabled,
		}, now)
		if err != nil {
			return err
		}
		if err := self.events.Audit(ctx, tx, actor, "webhook.updated", models.M{"webhookId": id}); err != nil {
			return err
		}
		value, err = self.value(ctx, tx, id)
		return err
	})
	if err != nil {
		return models.SavedWebhook{}, err
	}
	return models.SavedWebhook{Webhook: value}, nil
}

func (self *WebhookService) create(ctx context.Context, actor *auth.Actor, hookURL *string, label args.Nullable[string], types []string, team args.Nullable[uuid.UUID], enabled *bool) (models.SavedWebhook, error) {
	if hookURL == nil {
		return models.SavedWebhook{}, errdefs.BadRequest("url is required to create a webhook")
	}
	secret := auth.RandomSecret("rr_whs_")
	if types == nil {
		types = []string{"Issue", "Comment"}
	}
	var value models.Webhook
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		id, err := self.webhooks().Create(ctx, tx, webhook_repo.CreateInput{
			Workspace: actor.Workspace, Team: team.Ptr(), URL: *hookURL, Label: label.Ptr(), Secret: secret,
			ResourceTypes: types, Enabled: enabled == nil || *enabled, Creator: actor.User,
		})
		if err != nil {
			return err
		}
		if err := self.events.Audit(ctx, tx, actor, "webhook.created", models.M{"webhookId": id, "url": *hookURL}); err != nil {
			return err
		}
		value, err = self.value(ctx, tx, id)
		return err
	})
	if err != nil {
		return models.SavedWebhook{}, err
	}
	return models.SavedWebhook{Webhook: value, Secret: secret}, nil
}

func (self *WebhookService) RotateSecret(ctx context.Context, actor *auth.Actor, input *IDInput) (models.WebhookSecret, error) {
	id, err := self.webhookID(ctx, actor, input.ID)
	if err != nil {
		return models.WebhookSecret{}, err
	}
	secret := auth.RandomSecret("rr_whs_")
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		if err := self.webhooks().SetSecret(ctx, tx, id, secret, now); err != nil {
			return err
		}
		return self.events.Audit(ctx, tx, actor, "webhook.secretRotated", models.M{"webhookId": id})
	})
	if err != nil {
		return models.WebhookSecret{}, err
	}
	return models.WebhookSecret{ID: id.String(), Secret: secret}, nil
}

func (self *WebhookService) Delete(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DeletedWebhook, error) {
	id, err := self.webhookID(ctx, actor, input.ID)
	if err != nil {
		return models.DeletedWebhook{}, err
	}
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if err := self.webhooks().Delete(ctx, tx, id); err != nil {
			return err
		}
		return self.events.Audit(ctx, tx, actor, "webhook.deleted", models.M{"webhookId": id})
	})
	if err != nil {
		return models.DeletedWebhook{}, err
	}
	return models.DeletedWebhook{ID: id.String(), Deleted: true}, nil
}

func (self *WebhookService) Test(ctx context.Context, actor *auth.Actor, input *IDInput) (models.WebhookPing, error) {
	id, err := self.webhookID(ctx, actor, input.ID)
	if err != nil {
		return models.WebhookPing{}, err
	}
	payload := models.M{
		"action":      "test",
		"type":        "Ping",
		"data":        models.M{"webhookId": id},
		"url":         nil,
		"actor":       models.M{"id": actor.User},
		"createdAt":   time.Now().UTC(),
		"workspaceId": actor.Workspace,
		"webhookId":   id,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return models.WebhookPing{}, err
	}
	delivery, err := self.webhooks().QueuePing(ctx, id, encoded)
	if err != nil {
		return models.WebhookPing{}, err
	}
	hook, err := self.webhooks().Get(ctx, id)
	if err != nil {
		return models.WebhookPing{}, err
	}
	due := webhook_repo.Due{ID: delivery, URL: hook.URL, Secret: hook.Secret, Enabled: true}
	outcome := self.sender.send(ctx, due, payload)
	if err := self.record(ctx, due, outcome, false); err != nil {
		return models.WebhookPing{}, err
	}
	return models.WebhookPing{DeliveryID: delivery.String(), Delivered: outcome.OK, StatusCode: outcome.Status, Error: outcome.Error}, nil
}
