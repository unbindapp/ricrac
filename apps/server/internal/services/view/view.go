// Package view_service is custom views: saved filters of issues (or projects, initiatives),
// personal or shared with the workspace, optionally for one team.
package view_service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"ricrac/config"
	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	"ricrac/internal/repositories/repositories"
	view_repo "ricrac/internal/repositories/view"
	events_service "ricrac/internal/services/events"
	issue_service "ricrac/internal/services/issue"
	resolve_service "ricrac/internal/services/resolve"
)

type M = map[string]any

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i ViewServiceInterface -p view_service -s ViewService -o view_service_iface.go
type ViewService struct {
	cfg     config.Config
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
	events  events_service.EventsServiceInterface
	issues  issue_service.IssueServiceInterface
}

func NewViewService(cfg config.Config, repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface, issues issue_service.IssueServiceInterface) *ViewService {
	return &ViewService{cfg: cfg, repo: repo, resolve: resolve, events: events, issues: issues}
}

type ListInput struct {
	Query  *string `json:"query" doc:"Words in the name or description."`
	Model  *string `json:"model" doc:"issue, project or initiative."`
	Team   *string `json:"team" doc:"Optional team key, name or id: that team's views."`
	Shared *bool   `json:"shared" doc:"true: only views shared with the workspace; false: only the caller's personal views."`
	args.Page
}

type IDInput struct {
	ID string `json:"id"`
}

type SaveInput struct {
	ID          *string               `json:"id" doc:"Only to update a view: its id, slug, link or name. Omit to create one."`
	Name        *string               `json:"name"`
	Description args.Nullable[string] `json:"description"`
	Icon        args.Nullable[string] `json:"icon"`
	Color       args.Nullable[string] `json:"color" doc:"Hex color like #5e6ad2; null clears it."`
	Model       *string               `json:"model" doc:"What the view lists: issue (default), project or initiative. Only when creating."`
	Filter      any                   `json:"filter" doc:"The view's filter object. For issue views, the filter language of list_issues: Fields: id, number, title, description, priority, estimate, dueDate, createdAt, updatedAt, startedAt, completedAt, canceledAt, archivedAt, state {id,name,type}, team {id,key,name}, assignee / creator {id,email,name,displayName,isMe}, labels {some|every|none: {id,name,parent}}, project {id,name,slugId}, cycle {id,number,name,isActive}, and, or. Comparators: eq, neq, in, nin, lt, lte, gt, gte, contains, startsWith, endsWith, null. Example: {\"assignee\": {\"isMe\": {\"eq\": true}}, \"state\": {\"type\": {\"in\": [\"started\", \"unstarted\"]}}}"`
	Display     any                   `json:"display" doc:"How to show it, e.g. {\"groupBy\": \"state\", \"orderBy\": \"priority\"}."`
	Team        args.Nullable[string] `json:"team" doc:"Optional team key, name or id the view belongs to (its issues are that team's); null makes it workspace-wide."`
	Shared      *bool                 `json:"shared" doc:"Share the view with the workspace (default false: only the caller sees it)."`
}

func (self *ViewService) value(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.CustomView, error) {
	view, err := self.repo.View().Load(ctx, tx, id)
	if ent.IsNotFound(err) {
		return models.CustomView{}, errdefs.NotFound("No such view")
	}
	if err != nil {
		return models.CustomView{}, err
	}
	return models.CustomViewOf(view, self.cfg.PublicURL), nil
}

// find is a view the caller can use, by id, slug, link or name: shared or theirs, and of a team
// they see.
func (self *ViewService) find(ctx context.Context, actor *auth.Actor, value string) (uuid.UUID, error) {
	id, err := self.resolve.CustomView(ctx, actor, value)
	if err != nil {
		return uuid.Nil, err
	}
	view, err := self.repo.View().Get(ctx, nil, id)
	if err != nil || view.TeamID == nil {
		return id, err
	}
	visible, err := actor.CanSeeTeam(ctx, *view.TeamID)
	if err != nil {
		return uuid.Nil, err
	}
	if !visible {
		return uuid.Nil, errdefs.NotFound("No view %q", strings.TrimSpace(value))
	}
	return id, nil
}

func checkModel(model string) (string, error) {
	switch model {
	case "issue", "project", "initiative":
		return model, nil
	}
	return "", errdefs.BadRequest("model %q isn't one of issue, project, initiative", model)
}

func (self *ViewService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.CustomView], error) {
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return args.PageOf[models.CustomView]{}, err
	}
	filter := view_repo.ListFilter{Workspace: actor.Workspace, Viewer: actor.User, Teams: teams, Shared: input.Shared}
	if input.Model != nil {
		model, err := checkModel(*input.Model)
		if err != nil {
			return args.PageOf[models.CustomView]{}, err
		}
		filter.Model = &model
	}
	if input.Team != nil {
		team, err := self.resolve.Team(ctx, actor, *input.Team)
		if err != nil {
			return args.PageOf[models.CustomView]{}, err
		}
		filter.Team = &team
	}
	if query := strings.TrimSpace(utils.OrEmpty(input.Query)); query != "" {
		filter.Pattern = new("%" + utils.EscapeLike(query) + "%")
	}
	if filter.Offset, err = input.Offset(); err != nil {
		return args.PageOf[models.CustomView]{}, err
	}
	filter.Limit = input.Size() + 1
	ids, err := self.repo.View().List(ctx, filter)
	if err != nil {
		return args.PageOf[models.CustomView]{}, err
	}
	views, err := self.repo.View().LoadAll(ctx, ids)
	if err != nil {
		return args.PageOf[models.CustomView]{}, err
	}
	values := make([]models.CustomView, len(views))
	for i, v := range views {
		values[i] = models.CustomViewOf(v, self.cfg.PublicURL)
	}
	return args.WrapOf(input.Page, values)
}

func (self *ViewService) Get(ctx context.Context, actor *auth.Actor, input *IDInput) (models.CustomView, error) {
	id, err := self.find(ctx, actor, input.ID)
	if err != nil {
		return models.CustomView{}, err
	}
	return self.value(ctx, nil, id)
}

func checkObject(value any, what string) error {
	if _, ok := value.(M); !ok {
		return errdefs.BadRequest("%s must be an object", what)
	}
	return nil
}

// checkCanChange lets only the owner change a view; admins also change shared ones.
func (self *ViewService) checkCanChange(ctx context.Context, actor *auth.Actor, id uuid.UUID) error {
	view, err := self.repo.View().Get(ctx, nil, id)
	if err != nil {
		return err
	}
	if view.OwnerID != nil && *view.OwnerID == actor.User || view.Shared && actor.Role >= auth.Admin {
		return nil
	}
	return errdefs.Forbidden("Only the view's owner (or an admin, for a shared view) can change it")
}

func optionalColor(color args.Nullable[string]) (*string, error) {
	if !color.Valid {
		return nil, nil
	}
	checked, err := utils.CheckColor(color.Value)
	if err != nil {
		return nil, err
	}
	return &checked, nil
}

func (self *ViewService) Save(ctx context.Context, actor *auth.Actor, input *SaveInput) (models.CustomView, error) {
	if input.Shared != nil && *input.Shared {
		if err := actor.RequireMember(); err != nil {
			return models.CustomView{}, err
		}
	}
	if input.Display != nil {
		if err := checkObject(input.Display, "display"); err != nil {
			return models.CustomView{}, err
		}
	}
	color, err := optionalColor(input.Color)
	if err != nil {
		return models.CustomView{}, err
	}
	var team *uuid.UUID
	if input.Team.Valid {
		id, err := self.resolve.Team(ctx, actor, input.Team.Value)
		if err != nil {
			return models.CustomView{}, err
		}
		team = &id
	}
	var value models.CustomView
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		id, action, err := self.createOrFind(ctx, tx, actor, input)
		if err != nil {
			return err
		}
		if err := self.apply(ctx, tx, actor, id, input, color, team); err != nil {
			return err
		}
		if value, err = self.value(ctx, tx, id); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "CustomView", action, value, nil, teamOf(value))
	})
	return value, err
}

func (self *ViewService) createOrFind(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, input *SaveInput) (uuid.UUID, string, error) {
	if input.ID != nil {
		id, err := self.find(ctx, actor, *input.ID)
		if err != nil {
			return uuid.Nil, "", err
		}
		if err := self.checkCanChange(ctx, actor, id); err != nil {
			return uuid.Nil, "", err
		}
		if input.Model != nil {
			return uuid.Nil, "", errdefs.BadRequest("A view's model can't change; create another view")
		}
		return id, "update", nil
	}
	name, err := utils.NonEmpty(utils.OrEmpty(input.Name), "name")
	if err != nil {
		return uuid.Nil, "", err
	}
	model := "issue"
	if input.Model != nil {
		model = *input.Model
	}
	if model, err = checkModel(model); err != nil {
		return uuid.Nil, "", err
	}
	id, err := self.repo.View().Create(ctx, tx, view_repo.NewView{Workspace: actor.Workspace, Owner: actor.User, SlugID: utils.SlugID(), Name: name, Model: model})
	return id, "create", err
}

func (self *ViewService) apply(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID, input *SaveInput, color *string, team *uuid.UUID) error {
	update := func(change view_repo.Change) error { return self.repo.View().Update(ctx, tx, id, change) }
	if input.Name != nil {
		name, err := utils.NonEmpty(*input.Name, "name")
		if err != nil {
			return err
		}
		if err := update(view_repo.SetName(name)); err != nil {
			return err
		}
	}
	if input.Filter != nil {
		if err := checkObject(input.Filter, "filter"); err != nil {
			return err
		}
		view, err := self.repo.View().Get(ctx, tx, id)
		if err != nil {
			return err
		}
		if view.Model == "issue" {
			if err := self.issues.CheckFilter(ctx, tx, actor, input.Filter); err != nil {
				return err
			}
		}
		if err := update(view_repo.SetFilter(json.RawMessage(errdefs.Marshal(input.Filter)))); err != nil {
			return err
		}
	}
	if input.Description.Set {
		if err := update(view_repo.SetDescription(utils.Trimmed(input.Description.Ptr()))); err != nil {
			return err
		}
	}
	if input.Icon.Set {
		if err := update(view_repo.SetIcon(utils.Trimmed(input.Icon.Ptr()))); err != nil {
			return err
		}
	}
	if input.Color.Set {
		if err := update(view_repo.SetColor(color)); err != nil {
			return err
		}
	}
	if input.Display != nil {
		if err := update(view_repo.SetDisplay(json.RawMessage(errdefs.Marshal(input.Display)))); err != nil {
			return err
		}
	}
	if input.Team.Set {
		if err := update(view_repo.SetTeam(team)); err != nil {
			return err
		}
	}
	if input.Shared != nil {
		if err := update(view_repo.SetShared(*input.Shared)); err != nil {
			return err
		}
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	return update(view_repo.Touch(now))
}

func teamOf(value models.CustomView) *uuid.UUID {
	if value.Team == nil {
		return nil
	}
	id, ok := utils.AsUUID(value.Team.ID)
	if !ok {
		return nil
	}
	return &id
}

func (self *ViewService) Delete(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DeletedCustomView, error) {
	id, err := self.find(ctx, actor, input.ID)
	if err != nil {
		return models.DeletedCustomView{}, err
	}
	if err := self.checkCanChange(ctx, actor, id); err != nil {
		return models.DeletedCustomView{}, err
	}
	var value models.CustomView
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		if value, err = self.value(ctx, tx, id); err != nil {
			return err
		}
		if err := self.repo.View().Delete(ctx, tx, id); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "CustomView", "remove", value, nil, teamOf(value))
	})
	if err != nil {
		return models.DeletedCustomView{}, err
	}
	return models.DeletedCustomView{Deleted: true, ID: id.String(), Name: value.Name}, nil
}
