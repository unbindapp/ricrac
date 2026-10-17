// Package template_service is templates: what new issues, projects and documents start from, for
// the workspace or one team, and the recurring issues they create.
package template_service

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/opcall"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	"ricrac/internal/repositories/repositories"
	access_service "ricrac/internal/services/access"
	events_service "ricrac/internal/services/events"
	resolve_service "ricrac/internal/services/resolve"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i TemplateServiceInterface -p template_service -s TemplateService -o template_service_iface.go
type TemplateService struct {
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
	events  events_service.EventsServiceInterface
	access  access_service.AccessServiceInterface
	ops     opcall.Caller
}

func NewTemplateService(repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface, access access_service.AccessServiceInterface, ops opcall.Caller) *TemplateService {
	return &TemplateService{repo: repo, resolve: resolve, events: events, access: access, ops: ops}
}

type ListInput struct {
	Type *string `json:"type" doc:"issue, project or document."`
	Team *string `json:"team" doc:"Team key, name or id: the team's templates and the workspace-wide ones."`
	args.Page
}

type GetInput struct {
	ID   string  `json:"id" doc:"The template's id or name."`
	Type *string `json:"type" doc:"issue, project or document, when templates of several types share the name."`
}

// Recurrence is when a recurring issue template creates its issues.
type Recurrence struct {
	Every    uint32  `json:"every"`
	Unit     string  `json:"unit"`
	StartsAt *string `json:"startsAt"`
}

func (r *Recurrence) UnmarshalJSON(data []byte) error {
	type plain Recurrence
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode((*plain)(r))
}

type SaveInput struct {
	ID          *string                   `json:"id" doc:"Only to update a template: its id or name. Omit to create one."`
	Name        *string                   `json:"name"`
	Description args.Nullable[string]     `json:"description"`
	Type        *string                   `json:"type" doc:"issue, project or document. Required to create; can't change."`
	Team        args.Nullable[string]     `json:"team" doc:"Team key, name or id the template belongs to; null makes it workspace-wide."`
	Data        any                       `json:"data" doc:"What the template fills in; the keys depend on the type (see the description)."`
	Recurrence  args.Nullable[Recurrence] `json:"recurrence" doc:"Issue templates only: create an issue from the template on a schedule, e.g. {\"every\": 1, \"unit\": \"week\",\n\"startsAt\": \"2026-11-02T09:00:00Z\"}. unit is day, week or month; startsAt (ISO 8601, default now) is the\nfirst one. null stops it."`
}

type DeleteInput struct {
	ID   string  `json:"id" doc:"The template's id or name."`
	Type *string `json:"type" doc:"issue, project or document, when templates of several types share the name."`
}

// value is the template as the API answers it, with its data, read in tx.
func (self *TemplateService) value(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.TemplateDetail, error) {
	templates, err := self.repo.Template().Load(ctx, tx, []uuid.UUID{id})
	if err != nil {
		return models.TemplateDetail{}, err
	}
	if len(templates) == 0 {
		return models.TemplateDetail{}, errdefs.NotFound("No such template")
	}
	return models.TemplateDetailOf(templates[0]), nil
}

func checkType(kind string) (string, error) {
	switch kind {
	case "issue", "project", "document":
		return kind, nil
	}
	return "", errdefs.BadRequest("type %q isn't one of issue, project, document", kind)
}

// find is a template the caller can see by id or name, of kind when given.
func (self *TemplateService) find(ctx context.Context, actor *auth.Actor, kind *string, value string) (uuid.UUID, error) {
	if kind != nil {
		checked, err := checkType(*kind)
		if err != nil {
			return uuid.Nil, err
		}
		return self.resolve.Template(ctx, actor, checked, value)
	}
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	var asID *uuid.UUID
	if id, ok := utils.AsUUID(value); ok {
		asID = &id
	}
	name := strings.TrimSpace(value)
	rows, err := self.repo.Template().Named(ctx, actor.Workspace, teams, asID, name)
	if err != nil {
		return uuid.Nil, err
	}
	switch {
	case len(rows) == 0:
		return uuid.Nil, errdefs.NotFound("No template %q; list_templates shows them", name)
	case len(rows) == 1, asID != nil && *asID == rows[0].ID:
		return rows[0].ID, nil
	}
	types := make([]string, len(rows))
	for i, r := range rows {
		types[i] = r.Type
	}
	return uuid.Nil, errdefs.BadRequest("Several templates are named %q (%s); pass type too", name, strings.Join(types, ", "))
}

func teamOf(value models.TemplateDetail) *uuid.UUID {
	if value.Team == nil {
		return nil
	}
	id, err := uuid.Parse(value.Team.ID)
	if err != nil {
		return nil
	}
	return &id
}
