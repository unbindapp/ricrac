// Package customer_service is customers: the companies a workspace builds for, with their
// status, tier and owner, and customer needs (requests) linking them to issues and projects.
// Guests can't see them.
package customer_service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	customer_repo "ricrac/internal/repositories/customer"
	"ricrac/internal/repositories/repositories"
	events_service "ricrac/internal/services/events"
	resolve_service "ricrac/internal/services/resolve"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i CustomerServiceInterface -p customer_service -s CustomerService -o customer_service_iface.go
type CustomerService struct {
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
	events  events_service.EventsServiceInterface
}

func NewCustomerService(repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface) *CustomerService {
	return &CustomerService{repo: repo, resolve: resolve, events: events}
}

type IDInput struct {
	ID string `json:"id"`
}

type ListInput struct {
	Status          *string `json:"status" doc:"Customer status name or id."`
	Tier            *string `json:"tier" doc:"Customer tier name or id."`
	Owner           *string `json:"owner" doc:"Owner: id, name, email or \"me\"."`
	Query           *string `json:"query" doc:"Words in the name or a domain."`
	IncludeArchived bool    `json:"includeArchived,omitempty"`
	args.Page
}

type SaveInput struct {
	ID          *string               `json:"id" doc:"Only to update a customer: id, slug, name or domain. Omit to create one."`
	Name        *string               `json:"name" doc:"Required to create."`
	Domains     []string              `json:"domains" doc:"Replaces the domains, e.g. [\"acme.com\"]. Each domain belongs to one customer."`
	ExternalIDs []string              `json:"externalIds" doc:"Replaces the ids of the customer in other systems (CRM, billing)."`
	LogoURL     args.Nullable[string] `json:"logoUrl" doc:"Logo image URL; null clears it."`
	Owner       args.Nullable[string] `json:"owner" doc:"Person responsible for the customer (id, name, email or \"me\"); null clears it."`
	Status      args.Nullable[string] `json:"status" doc:"Customer status name or id; null clears it."`
	Tier        args.Nullable[string] `json:"tier" doc:"Customer tier name or id; null clears it."`
	Revenue     args.Nullable[int64]  `json:"revenue" doc:"Annual revenue, a whole number; null clears it."`
	Size        args.Nullable[int32]  `json:"size" doc:"Number of employees; null clears it."`
}

type ListOptionsInput struct {
	args.Page
}

type SaveOptionInput struct {
	ID          *string               `json:"id" doc:"Only to update one: its id or name. Omit to create one."`
	Name        *string               `json:"name" doc:"Required to create."`
	Color       *string               `json:"color" doc:"Hex color like #4cb782."`
	Description args.Nullable[string] `json:"description" doc:"null clears it."`
	Position    *float64              `json:"position" doc:"Order among the others, lowest first. New ones go last."`
}

type ListNeedsInput struct {
	Customer *string `json:"customer" doc:"Customer id, slug, name or domain."`
	Issue    *string `json:"issue" doc:"Issue identifier (ENG-123) or id."`
	Project  *string `json:"project" doc:"Project name, slug or id."`
	args.Page
}

type SaveNeedInput struct {
	ID            *string               `json:"id" doc:"Only to update a need: its id. Omit to create one."`
	Customer      *string               `json:"customer" doc:"Customer id, slug, name or domain. Required to create."`
	Issue         *string               `json:"issue" doc:"The issue (ENG-123 or id) the customer needs. Pass this or project."`
	Project       *string               `json:"project" doc:"The project (name, slug or id) the customer needs. Pass this or issue."`
	Body          *string               `json:"body" doc:"What the customer said, in Markdown."`
	Priority      *int32                `json:"priority" doc:"0 normal, 1 important."`
	AttachmentURL args.Nullable[string] `json:"attachmentUrl" doc:"Link to where the request came from (an email, a support ticket); null clears it."`
}

func (self *CustomerService) customers() customer_repo.CustomerRepositoryInterface {
	return self.repo.Customer()
}

func (self *CustomerService) value(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.Customer, error) {
	values, err := self.values(ctx, tx, []uuid.UUID{id})
	if err != nil {
		return models.Customer{}, err
	}
	if len(values) == 0 {
		return models.Customer{}, errdefs.NotFound("No such customer")
	}
	return values[0], nil
}

func (self *CustomerService) values(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]models.Customer, error) {
	customers, err := self.customers().Load(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	counts, err := self.customers().NeedCounts(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	values := make([]models.Customer, len(customers))
	for i, c := range customers {
		values[i] = models.CustomerOf(c, counts[c.ID])
	}
	return values, nil
}

func (self *CustomerService) needValue(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.CustomerNeed, error) {
	needs, err := self.needValues(ctx, tx, []uuid.UUID{id})
	if err != nil {
		return models.CustomerNeed{}, err
	}
	if len(needs) == 0 {
		return models.CustomerNeed{}, errdefs.NotFound("No such customer need")
	}
	return needs[0], nil
}

func (self *CustomerService) needValues(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]models.CustomerNeed, error) {
	needs, err := self.customers().LoadNeeds(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	values := make([]models.CustomerNeed, len(needs))
	for i, n := range needs {
		values[i] = models.CustomerNeedOf(n)
	}
	return values, nil
}

func (self *CustomerService) viewer(ctx context.Context, actor *auth.Actor) (customer_repo.Viewer, error) {
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return customer_repo.Viewer{}, err
	}
	return customer_repo.Viewer{Teams: teams, User: actor.User}, nil
}

func (self *CustomerService) customerID(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	if err := actor.RequireMember(); err != nil {
		return uuid.Nil, err
	}
	return self.resolve.Customer(ctx, actor, value)
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

// resolveNullable resolves a nullable reference: left out stays unset, null stays null.
func resolveNullable(value args.Nullable[string], resolve func(string) (uuid.UUID, error)) (args.Nullable[uuid.UUID], error) {
	if !value.Valid {
		return args.Nullable[uuid.UUID]{Set: value.Set}, nil
	}
	id, err := resolve(value.Value)
	if err != nil {
		return args.Nullable[uuid.UUID]{}, err
	}
	return args.Nullable[uuid.UUID]{Set: true, Valid: true, Value: id}, nil
}

// emitCreatedOrChanged emits create without a previous value, else update when something changed.
func (self *CustomerService) emitCreatedOrChanged(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, kind string, before *any, value any, team *uuid.UUID) error {
	if before == nil {
		return self.events.Emit(ctx, tx, actor, kind, "create", value, nil, team)
	}
	from := events_service.ChangedFields(*before, value)
	if len(from) == 0 {
		return nil
	}
	return self.events.Emit(ctx, tx, actor, kind, "update", value, from, team)
}
