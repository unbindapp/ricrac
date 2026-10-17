// Package statusupdate_service is status updates on projects and initiatives: a written update
// with a health (on track, at risk, off track). The newest update's health is the project's or
// initiative's health.
package statusupdate_service

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	"ricrac/internal/repositories/repositories"
	statusupdate_repo "ricrac/internal/repositories/statusupdate"
	events_service "ricrac/internal/services/events"
	resolve_service "ricrac/internal/services/resolve"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i StatusUpdateServiceInterface -p statusupdate_service -s StatusUpdateService -o statusupdate_service_iface.go
type StatusUpdateService struct {
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
	events  events_service.EventsServiceInterface
}

func NewStatusUpdateService(repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface) *StatusUpdateService {
	return &StatusUpdateService{repo: repo, resolve: resolve, events: events}
}

type ListInput struct {
	ID         *string `json:"id" doc:"One status update by id."`
	Project    *string `json:"project" doc:"Project name, slug, link or id."`
	Initiative *string `json:"initiative" doc:"Initiative name, slug, link or id."`
	Type       *string `json:"type" doc:"project or initiative updates only."`
	User       *string `json:"user" doc:"Author (id, name, email or \"me\")."`
	CreatedAt  *string `json:"createdAt" doc:"Created after: ISO 8601 date/time or a duration like -P1W."`
	Limit      *int64  `json:"limit" doc:"How many to return, 50 by default, at most 250."`
	Cursor     *string `json:"cursor" doc:"The endCursor of the previous page."`
}

type SaveInput struct {
	ID         *string `json:"id" doc:"Only to edit an update: its id. Omit to post one."`
	Type       *string `json:"type" doc:"project or initiative; follows from which of them is given."`
	Project    *string `json:"project" doc:"Project name, slug, link or id."`
	Initiative *string `json:"initiative" doc:"Initiative name, slug, link or id."`
	Body       *string `json:"body" doc:"Markdown. Mention people with @displayName or @email; they are notified. Required to post."`
	Health     *string `json:"health" doc:"onTrack, atRisk or offTrack."`
}

type IDInput struct {
	ID string `json:"id"`
}

// Values is the updates as the API answers them, in the order of ids, read in tx.
func (self *StatusUpdateService) Values(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]models.StatusUpdate, error) {
	updates, err := self.repo.StatusUpdate().Load(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	counts, err := self.repo.StatusUpdate().CommentCounts(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	values := make([]models.StatusUpdate, len(updates))
	for i, u := range updates {
		values[i] = models.StatusUpdateOf(u, counts[u.ID])
	}
	return values, nil
}

func (self *StatusUpdateService) value(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (models.StatusUpdate, error) {
	values, err := self.Values(ctx, tx, []uuid.UUID{id})
	if err != nil {
		return models.StatusUpdate{}, err
	}
	if len(values) == 0 {
		return models.StatusUpdate{}, errdefs.NotFound("No such status update")
	}
	return values[0], nil
}

// Latest is the newest update on the project or initiative, or nil.
func (self *StatusUpdateService) Latest(ctx context.Context, tx repository.TxInterface, project, initiative *uuid.UUID) (*models.StatusUpdate, error) {
	latest, err := self.repo.StatusUpdate().Latest(ctx, tx, project, initiative)
	if err != nil || latest == nil {
		return nil, err
	}
	value, err := self.value(ctx, tx, latest.ID)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func (self *StatusUpdateService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.StatusUpdate], error) {
	page := args.Page{Limit: input.Limit, Cursor: input.Cursor}
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return args.PageOf[models.StatusUpdate]{}, err
	}
	filter := statusupdate_repo.ListFilter{Workspace: actor.Workspace, Teams: teams, SeesInitiatives: actor.Role != auth.Guest}
	if input.ID != nil {
		id, ok := utils.AsUUID(*input.ID)
		if !ok {
			return args.PageOf[models.StatusUpdate]{}, errdefs.NotFound("No status update \"%s\"", strings.TrimSpace(*input.ID))
		}
		filter.ID = &id
	}
	if input.Project != nil {
		project, err := self.resolve.Project(ctx, actor, *input.Project)
		if err != nil {
			return args.PageOf[models.StatusUpdate]{}, err
		}
		filter.Project = &project
	}
	if input.Initiative != nil {
		initiative, err := self.resolve.Initiative(ctx, actor, *input.Initiative)
		if err != nil {
			return args.PageOf[models.StatusUpdate]{}, err
		}
		filter.Initiative = &initiative
	}
	if input.Type != nil {
		if *input.Type != "project" && *input.Type != "initiative" {
			return args.PageOf[models.StatusUpdate]{}, errdefs.BadRequest("type is project or initiative, not \"%s\"", *input.Type)
		}
		filter.Type = input.Type
	}
	if input.User != nil {
		user, err := self.resolve.User(ctx, actor, *input.User)
		if err != nil {
			return args.PageOf[models.StatusUpdate]{}, err
		}
		filter.User = &user
	}
	if input.CreatedAt != nil {
		after, err := utils.ParseDatetime(*input.CreatedAt)
		if err != nil {
			return args.PageOf[models.StatusUpdate]{}, err
		}
		filter.CreatedAt = &after
	}
	if filter.Offset, err = page.Offset(); err != nil {
		return args.PageOf[models.StatusUpdate]{}, err
	}
	filter.Limit = page.Size() + 1
	ids, err := self.repo.StatusUpdate().List(ctx, filter)
	if err != nil {
		return args.PageOf[models.StatusUpdate]{}, err
	}
	values, err := self.Values(ctx, nil, ids)
	if err != nil {
		return args.PageOf[models.StatusUpdate]{}, err
	}
	return args.WrapOf(page, values)
}

// target is the project or initiative an update is about.
type target struct {
	project    *uuid.UUID
	initiative *uuid.UUID
	name       string
}

func (self *StatusUpdateService) findTarget(ctx context.Context, actor *auth.Actor, input *SaveInput) (target, error) {
	kind := utils.OrEmpty(input.Type)
	switch {
	case input.Project != nil && input.Initiative != nil:
		return target{}, errdefs.BadRequest("Pass either project or initiative, not both")
	case input.Project != nil && (input.Type == nil || kind == "project"):
		id, err := self.resolve.Project(ctx, actor, *input.Project)
		if err != nil {
			return target{}, err
		}
		project, err := self.repo.Project().Get(ctx, nil, id)
		if err != nil {
			return target{}, err
		}
		return target{project: &id, name: project.Name}, nil
	case input.Initiative != nil && (input.Type == nil || kind == "initiative"):
		if err := actor.RequireMember(); err != nil {
			return target{}, err
		}
		id, err := self.resolve.Initiative(ctx, actor, *input.Initiative)
		if err != nil {
			return target{}, err
		}
		initiative, err := self.repo.Initiative().Get(ctx, nil, id)
		if err != nil {
			return target{}, err
		}
		return target{initiative: &id, name: initiative.Name}, nil
	case input.Project == nil && input.Initiative == nil:
		return target{}, errdefs.BadRequest("Name the project or initiative the update is about")
	}
	return target{}, errdefs.BadRequest("type \"%s\" doesn't match; it is project with project, initiative with initiative", kind)
}

func (self *StatusUpdateService) Save(ctx context.Context, actor *auth.Actor, input *SaveInput) (models.StatusUpdate, error) {
	if input.ID != nil {
		return self.edit(ctx, actor, *input.ID, input)
	}
	return self.create(ctx, actor, input)
}

func (self *StatusUpdateService) create(ctx context.Context, actor *auth.Actor, input *SaveInput) (models.StatusUpdate, error) {
	on, err := self.findTarget(ctx, actor, input)
	if err != nil {
		return models.StatusUpdate{}, err
	}
	body, err := utils.NonEmpty(utils.OrEmpty(input.Body), "body")
	if err != nil {
		return models.StatusUpdate{}, err
	}
	var value models.StatusUpdate
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		health, err := self.health(ctx, tx, on, input.Health)
		if err != nil {
			return err
		}
		id, err := self.repo.StatusUpdate().Create(ctx, tx, statusupdate_repo.CreateInput{
			Workspace: actor.Workspace, Project: on.project, Initiative: on.initiative, User: actor.User, Body: body, Health: health,
		})
		if err != nil {
			return err
		}
		if err := self.repo.StatusUpdate().SyncHealth(ctx, tx, on.project, on.initiative); err != nil {
			return err
		}
		mentioned, err := self.events.Mentions(ctx, actor, body)
		if err != nil {
			return err
		}
		if err := self.notifyMentions(ctx, tx, actor, on, id, mentioned); err != nil {
			return err
		}
		var audience []uuid.UUID
		if on.project != nil {
			audience, err = self.repo.StatusUpdate().ProjectAudience(ctx, tx, *on.project)
		} else {
			audience, err = self.repo.StatusUpdate().InitiativeOwner(ctx, tx, *on.initiative)
		}
		if err != nil {
			return err
		}
		audience = slices.DeleteFunc(audience, func(u uuid.UUID) bool { return slices.Contains(mentioned, u) })
		kind := "initiativeUpdateCreated"
		if on.project != nil {
			kind = "projectUpdateCreated"
		}
		notice := events_service.Notice{Kind: kind, Summary: "New update on " + on.name, Project: on.project, Initiative: on.initiative, StatusUpdate: &id}
		if err := self.events.Notify(ctx, tx, actor, audience, notice); err != nil {
			return err
		}
		if value, err = self.value(ctx, tx, id); err != nil {
			return err
		}
		return self.emit(ctx, tx, actor, on, "create", value)
	})
	if err != nil {
		return models.StatusUpdate{}, err
	}
	return value, nil
}

// health is the health a new update gets: the one asked for, else the previous update's, else
// onTrack.
func (self *StatusUpdateService) health(ctx context.Context, tx repository.TxInterface, on target, asked *string) (string, error) {
	if asked != nil {
		return utils.CheckHealth(*asked)
	}
	previous, err := self.repo.StatusUpdate().Latest(ctx, tx, on.project, on.initiative)
	if err != nil {
		return "", err
	}
	if previous == nil {
		return "onTrack", nil
	}
	return previous.Health, nil
}

func (self *StatusUpdateService) notifyMentions(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, on target, update uuid.UUID, users []uuid.UUID) error {
	if len(users) == 0 {
		return nil
	}
	return self.events.Notify(ctx, tx, actor, users, events_service.Notice{Kind: "projectUpdateMention",
		Summary: "You were mentioned in an update on " + on.name, Project: on.project, Initiative: on.initiative, StatusUpdate: &update})
}

func (self *StatusUpdateService) emit(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, on target, action string, value models.StatusUpdate) error {
	if on.project == nil {
		return self.events.Emit(ctx, tx, actor, "InitiativeUpdate", action, value, nil, nil)
	}
	team, err := self.repo.Project().WebhookTeam(ctx, tx, *on.project)
	if err != nil {
		return err
	}
	return self.events.Emit(ctx, tx, actor, "ProjectUpdate", action, value, nil, team)
}

// find is the update by id, if the caller can see what it is about, with its author.
func (self *StatusUpdateService) find(ctx context.Context, actor *auth.Actor, reference string) (uuid.UUID, *uuid.UUID, target, error) {
	notFound := errdefs.NotFound("No status update \"%s\"", strings.TrimSpace(reference))
	id, ok := utils.AsUUID(reference)
	if !ok {
		return uuid.Nil, nil, target{}, notFound
	}
	update, err := self.repo.StatusUpdate().Get(ctx, actor.Workspace, id)
	if err != nil {
		return uuid.Nil, nil, target{}, err
	}
	if update == nil {
		return uuid.Nil, nil, target{}, notFound
	}
	var lookup SaveInput
	if update.ProjectID != nil {
		lookup.Project = new(update.ProjectID.String())
	}
	if update.InitiativeID != nil {
		lookup.Initiative = new(update.InitiativeID.String())
	}
	on, err := self.findTarget(ctx, actor, &lookup)
	if err != nil {
		return uuid.Nil, nil, target{}, notFound
	}
	return id, update.UserID, on, nil
}

func (self *StatusUpdateService) edit(ctx context.Context, actor *auth.Actor, reference string, input *SaveInput) (models.StatusUpdate, error) {
	if input.Project != nil || input.Initiative != nil || input.Type != nil {
		return models.StatusUpdate{}, errdefs.BadRequest("An update stays on its project or initiative; pass only body and health")
	}
	id, author, on, err := self.find(ctx, actor, reference)
	if err != nil {
		return models.StatusUpdate{}, err
	}
	if author == nil || *author != actor.User {
		return models.StatusUpdate{}, errdefs.Forbidden("Only the author can edit a status update")
	}
	var value models.StatusUpdate
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		current, err := self.repo.StatusUpdate().GetForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if input.Body != nil {
			if err := self.editBody(ctx, tx, actor, on, id, current.Body, *input.Body); err != nil {
				return err
			}
		}
		if input.Health != nil {
			health, err := utils.CheckHealth(*input.Health)
			if err != nil {
				return err
			}
			if err := self.repo.StatusUpdate().Update(tx, id).SetHealth(health).Exec(ctx); err != nil {
				return err
			}
		}
		now, err := self.repo.Now(ctx, tx)
		if err != nil {
			return err
		}
		if err := self.repo.StatusUpdate().Update(tx, id).SetEditedAt(now).SetUpdatedAt(now).Exec(ctx); err != nil {
			return err
		}
		if err := self.repo.StatusUpdate().SyncHealth(ctx, tx, on.project, on.initiative); err != nil {
			return err
		}
		if value, err = self.value(ctx, tx, id); err != nil {
			return err
		}
		return self.emit(ctx, tx, actor, on, "update", value)
	})
	if err != nil {
		return models.StatusUpdate{}, err
	}
	return value, nil
}

func (self *StatusUpdateService) editBody(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, on target, id uuid.UUID, oldBody, newBody string) error {
	body, err := utils.NonEmpty(newBody, "body")
	if err != nil {
		return err
	}
	if err := self.repo.StatusUpdate().Update(tx, id).SetBody(body).Exec(ctx); err != nil {
		return err
	}
	previous, err := self.events.Mentions(ctx, actor, oldBody)
	if err != nil {
		return err
	}
	found, err := self.events.Mentions(ctx, actor, body)
	if err != nil {
		return err
	}
	mentioned := slices.DeleteFunc(found, func(u uuid.UUID) bool { return slices.Contains(previous, u) })
	return self.notifyMentions(ctx, tx, actor, on, id, mentioned)
}

func (self *StatusUpdateService) Delete(ctx context.Context, actor *auth.Actor, input *IDInput) (models.DeletedStatusUpdate, error) {
	id, author, on, err := self.find(ctx, actor, input.ID)
	if err != nil {
		return models.DeletedStatusUpdate{}, err
	}
	if (author == nil || *author != actor.User) && actor.Role < auth.Admin {
		return models.DeletedStatusUpdate{}, errdefs.Forbidden("Only the author or a workspace admin can delete a status update")
	}
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		value, err := self.value(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := self.repo.StatusUpdate().Delete(ctx, tx, id); err != nil {
			return err
		}
		if err := self.repo.StatusUpdate().SyncHealth(ctx, tx, on.project, on.initiative); err != nil {
			return err
		}
		return self.emit(ctx, tx, actor, on, "remove", value)
	})
	if err != nil {
		return models.DeletedStatusUpdate{}, err
	}
	return models.DeletedStatusUpdate{Deleted: true, ID: id.String()}, nil
}
