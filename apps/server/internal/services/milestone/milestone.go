// Package milestone_service is milestones: the stages of a project, or the workspace's own (like
// those synced with GitHub), each with its issues' progress.
package milestone_service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	"ricrac/internal/repositories/repositories"
	events_service "ricrac/internal/services/events"
	resolve_service "ricrac/internal/services/resolve"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i MilestoneServiceInterface -p milestone_service -s MilestoneService -o milestone_service_iface.go
type MilestoneService struct {
	repo    repositories.RepositoriesInterface
	resolve resolve_service.ResolveServiceInterface
	events  events_service.EventsServiceInterface
}

func NewMilestoneService(repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface) *MilestoneService {
	return &MilestoneService{repo: repo, resolve: resolve, events: events}
}

type ListInput struct {
	Project *string `json:"project" doc:"Project name, slug, link or id; omit for the workspace's own milestones, those in no project."`
	Limit   *int64  `json:"limit" doc:"How many to return, 50 by default, at most 250."`
	Cursor  *string `json:"cursor" doc:"The endCursor of the previous page."`
}

type MilestoneInput struct {
	ID      string  `json:"id" doc:"The milestone's id or name."`
	Project *string `json:"project" doc:"Project name, slug or id, when id names one of its milestones."`
}

type SaveInput struct {
	ID          *string               `json:"id" doc:"Only to change a milestone: its id or name. Omit to create one."`
	Project     *string               `json:"project" doc:"Project name, slug or id. To create: the project it is in; omit for one of the workspace's own."`
	Name        *string               `json:"name" doc:"Required to create; unique within the project, or among the workspace's own."`
	Description args.Nullable[string] `json:"description" doc:"Markdown; null clears it."`
	TargetDate  args.Nullable[string] `json:"targetDate" doc:"YYYY-MM-DD; null clears it."`
	SortOrder   *float64              `json:"sortOrder" doc:"Position among its sibling milestones (lower comes first; ties go by target date)."`
	Closed      *bool                 `json:"closed" doc:"true closes the milestone, false opens it again."`
}

// Values is the milestones with the progress of the issues actor sees, in the order of ids,
// read in tx.
func (self *MilestoneService) Values(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, ids []uuid.UUID) ([]models.Milestone, error) {
	milestones, err := self.repo.Milestone().Load(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	stats, err := self.repo.Milestone().Stats(ctx, tx, ids, actor.User)
	if err != nil {
		return nil, err
	}
	values := make([]models.Milestone, len(milestones))
	for i, m := range milestones {
		values[i] = models.MilestoneOf(m, stats[m.ID])
	}
	return values, nil
}

// OfProject is the project's milestones in order, read in tx.
func (self *MilestoneService) OfProject(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, project uuid.UUID) ([]models.Milestone, error) {
	ids, err := self.repo.Milestone().OfProject(ctx, tx, project, 0, 0)
	if err != nil {
		return nil, err
	}
	return self.Values(ctx, tx, actor, ids)
}

func (self *MilestoneService) value(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID) (models.Milestone, error) {
	values, err := self.Values(ctx, tx, actor, []uuid.UUID{id})
	if err != nil {
		return models.Milestone{}, err
	}
	if len(values) == 0 {
		return models.Milestone{}, errdefs.NotFound("No such milestone")
	}
	return values[0], nil
}

func (self *MilestoneService) List(ctx context.Context, actor *auth.Actor, input *ListInput) (args.PageOf[models.Milestone], error) {
	page := args.Page{Limit: input.Limit, Cursor: input.Cursor}
	offset, err := page.Offset()
	if err != nil {
		return args.PageOf[models.Milestone]{}, err
	}
	ids, err := self.listed(ctx, actor, input.Project, page.Size()+1, offset)
	if err != nil {
		return args.PageOf[models.Milestone]{}, err
	}
	values, err := self.Values(ctx, nil, actor, ids)
	if err != nil {
		return args.PageOf[models.Milestone]{}, err
	}
	return args.WrapOf(page, values)
}

func (self *MilestoneService) listed(ctx context.Context, actor *auth.Actor, project *string, limit, offset int64) ([]uuid.UUID, error) {
	if project == nil {
		return self.repo.Milestone().OfWorkspace(ctx, nil, actor.Workspace, limit, offset)
	}
	id, err := self.resolve.Project(ctx, actor, *project)
	if err != nil {
		return nil, err
	}
	return self.repo.Milestone().OfProject(ctx, nil, id, limit, offset)
}

func (self *MilestoneService) find(ctx context.Context, actor *auth.Actor, id string, project *string) (uuid.UUID, error) {
	var projectID *uuid.UUID
	if project != nil {
		found, err := self.resolve.Project(ctx, actor, *project)
		if err != nil {
			return uuid.Nil, err
		}
		projectID = &found
	}
	return self.resolve.Milestone(ctx, actor, projectID, id)
}

func (self *MilestoneService) Get(ctx context.Context, actor *auth.Actor, input *MilestoneInput) (models.MilestoneDetail, error) {
	id, err := self.find(ctx, actor, input.ID, input.Project)
	if err != nil {
		return models.MilestoneDetail{}, err
	}
	value, err := self.value(ctx, nil, actor, id)
	if err != nil {
		return models.MilestoneDetail{}, err
	}
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return models.MilestoneDetail{}, err
	}
	issues, err := self.repo.Milestone().Issues(ctx, id, teams)
	if err != nil {
		return models.MilestoneDetail{}, err
	}
	detail := models.MilestoneDetail{Milestone: value, Issues: make([]models.IssueChild, len(issues))}
	for i, issue := range issues {
		detail.Issues[i] = models.MilestoneIssueOf(issue)
	}
	return detail, nil
}

func (self *MilestoneService) Save(ctx context.Context, actor *auth.Actor, input *SaveInput) (models.Milestone, error) {
	var existing *uuid.UUID
	if input.ID != nil {
		id, err := self.find(ctx, actor, *input.ID, input.Project)
		if err != nil {
			return models.Milestone{}, err
		}
		existing = &id
	}
	var value models.Milestone
	err := self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var id uuid.UUID
		var err error
		if existing != nil {
			id = *existing
		} else if id, err = self.create(ctx, tx, actor, input); err != nil {
			return err
		}
		if err := self.apply(ctx, tx, id, input); err != nil {
			return err
		}
		if value, err = self.value(ctx, tx, actor, id); err != nil {
			return err
		}
		if existing != nil && showsOnGitHub(input) {
			if err := self.events.QueueMilestoneForGitHub(ctx, tx, actor, id, "milestone"); err != nil {
				return err
			}
		}
		team, err := self.webhookTeam(ctx, tx, id)
		if err != nil {
			return err
		}
		action := "create"
		if existing != nil {
			action = "update"
		}
		return self.events.Emit(ctx, tx, actor, "ProjectMilestone", action, value, nil, team)
	})
	if err != nil {
		return models.Milestone{}, err
	}
	return value, nil
}

// showsOnGitHub says whether the input changes what GitHub milestones have too.
func showsOnGitHub(input *SaveInput) bool {
	return input.Name != nil || input.Description.Set || input.TargetDate.Set || input.Closed != nil
}

// webhookTeam is the team of the milestone's project, nil for the workspace's own.
func (self *MilestoneService) webhookTeam(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*uuid.UUID, error) {
	milestone, err := self.repo.Milestone().Get(ctx, tx, id)
	if err != nil || milestone.ProjectID == nil {
		return nil, err
	}
	return self.repo.Project().WebhookTeam(ctx, tx, *milestone.ProjectID)
}

func (self *MilestoneService) create(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, input *SaveInput) (uuid.UUID, error) {
	var project *uuid.UUID
	if input.Project != nil {
		id, err := self.resolve.Project(ctx, actor, *input.Project)
		if err != nil {
			return uuid.Nil, err
		}
		project = &id
	}
	name, err := utils.NonEmpty(utils.OrEmpty(input.Name), "name")
	if err != nil {
		return uuid.Nil, err
	}
	return self.repo.Milestone().Create(ctx, tx, actor.Workspace, project, name)
}

func (self *MilestoneService) apply(ctx context.Context, tx repository.TxInterface, id uuid.UUID, input *SaveInput) error {
	if input.Name != nil {
		name, err := utils.NonEmpty(*input.Name, "name")
		if err != nil {
			return err
		}
		taken, err := self.repo.Milestone().NameTaken(ctx, tx, id, name)
		if err != nil {
			return err
		}
		if taken {
			return errdefs.Conflict("There is a milestone \"%s\" already", name)
		}
		if err := self.repo.Milestone().Update(tx, id).SetName(name).Exec(ctx); err != nil {
			return err
		}
	}
	if input.Description.Set {
		update := self.repo.Milestone().Update(tx, id)
		if input.Description.Valid {
			update.SetDescription(input.Description.Value)
		} else {
			update.ClearDescription()
		}
		if err := update.Exec(ctx); err != nil {
			return err
		}
	}
	if input.TargetDate.Set {
		var target *time.Time
		if input.TargetDate.Valid && !utils.IsNoneWord(input.TargetDate.Value) {
			date, err := utils.ParseDate(input.TargetDate.Value)
			if err != nil {
				return err
			}
			target = &date
		}
		update := self.repo.Milestone().Update(tx, id)
		if target == nil {
			update.ClearTargetDate()
		} else {
			update.SetTargetDate(*target)
		}
		if err := update.Exec(ctx); err != nil {
			return err
		}
	}
	if input.SortOrder != nil {
		if err := self.repo.Milestone().Update(tx, id).SetSortOrder(*input.SortOrder).Exec(ctx); err != nil {
			return err
		}
	}
	now, err := self.repo.Now(ctx, tx)
	if err != nil {
		return err
	}
	update := self.repo.Milestone().Update(tx, id).SetUpdatedAt(now)
	if input.Closed != nil {
		if *input.Closed {
			update.SetClosedAt(now)
		} else {
			update.ClearClosedAt()
		}
	}
	return update.Exec(ctx)
}

func (self *MilestoneService) Delete(ctx context.Context, actor *auth.Actor, input *MilestoneInput) (models.DeletedMilestone, error) {
	id, err := self.find(ctx, actor, input.ID, input.Project)
	if err != nil {
		return models.DeletedMilestone{}, err
	}
	var value models.Milestone
	err = self.repo.WithTx(ctx, func(tx repository.TxInterface) error {
		var err error
		if value, err = self.value(ctx, tx, actor, id); err != nil {
			return err
		}
		team, err := self.webhookTeam(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := self.events.QueueMilestoneForGitHub(ctx, tx, actor, id, "milestone_deleted"); err != nil {
			return err
		}
		if err := self.repo.Milestone().Delete(ctx, tx, id); err != nil {
			return err
		}
		return self.events.Emit(ctx, tx, actor, "ProjectMilestone", "remove", value, nil, team)
	})
	if err != nil {
		return models.DeletedMilestone{}, err
	}
	return models.DeletedMilestone{Deleted: true, ID: id.String(), Name: value.Name}, nil
}
