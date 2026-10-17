// Package project_service is projects: listing, reading, creating and changing them (teams,
// members, labels, dates, links, initiatives, dependencies and templates), trash and archive, and
// the workspace's project statuses and labels.
package project_service

import (
	"context"

	"github.com/google/uuid"

	"ricrac/config"
	"ricrac/ent/entitylink"
	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/opcall"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	"ricrac/internal/repositories/repositories"
	events_service "ricrac/internal/services/events"
	milestone_service "ricrac/internal/services/milestone"
	resolve_service "ricrac/internal/services/resolve"
	statusupdate_service "ricrac/internal/services/statusupdate"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i ProjectServiceInterface -p project_service -s ProjectService -o project_service_iface.go
type ProjectService struct {
	cfg        config.Config
	repo       repositories.RepositoriesInterface
	resolve    resolve_service.ResolveServiceInterface
	events     events_service.EventsServiceInterface
	milestones milestone_service.MilestoneServiceInterface
	updates    statusupdate_service.StatusUpdateServiceInterface
	ops        opcall.Caller
}

func NewProjectService(cfg config.Config, repo repositories.RepositoriesInterface, resolve resolve_service.ResolveServiceInterface, events events_service.EventsServiceInterface, milestones milestone_service.MilestoneServiceInterface, updates statusupdate_service.StatusUpdateServiceInterface, ops opcall.Caller) *ProjectService {
	return &ProjectService{cfg: cfg, repo: repo, resolve: resolve, events: events, milestones: milestones, updates: updates, ops: ops}
}

type IDInput struct {
	ID string `json:"id"`
}

type LinkInput struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// Values is the projects as lists show them, with the progress of the issues actor sees, in the
// order of ids, read in tx.
func (self *ProjectService) Values(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, ids []uuid.UUID, withDescription bool) ([]models.Project, error) {
	projects, err := self.repo.Project().Load(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	stats, err := self.repo.Project().Stats(ctx, tx, ids, actor.User)
	if err != nil {
		return nil, err
	}
	values := make([]models.Project, len(projects))
	for i, p := range projects {
		values[i] = models.ProjectOf(p, self.cfg.PublicURL, stats[p.ID], withDescription)
	}
	return values, nil
}

// value is one project with its description, read in tx.
func (self *ProjectService) value(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID) (models.Project, error) {
	values, err := self.Values(ctx, tx, actor, []uuid.UUID{id}, true)
	if err != nil {
		return models.Project{}, err
	}
	if len(values) == 0 {
		return models.Project{}, errdefs.NotFound("No such project")
	}
	return values[0], nil
}

// Links is the links on the project or initiative whose id is in column.
func (self *ProjectService) Links(ctx context.Context, tx repository.TxInterface, column string, id uuid.UUID) ([]models.EntityLink, error) {
	links, err := self.repo.Project().Links(ctx, tx, column, id)
	if err != nil {
		return nil, err
	}
	values := make([]models.EntityLink, len(links))
	for i, l := range links {
		values[i] = models.EntityLinkOf(l)
	}
	return values, nil
}

// Documents is the documents of the project or initiative whose id is in column.
func (self *ProjectService) Documents(ctx context.Context, tx repository.TxInterface, column string, id uuid.UUID) ([]models.DocumentRef, error) {
	documents, err := self.repo.Project().Documents(ctx, tx, column, id)
	if err != nil {
		return nil, err
	}
	refs := make([]models.DocumentRef, len(documents))
	for i, d := range documents {
		refs[i] = *models.DocumentRefOf(d)
	}
	return refs, nil
}

// detail is everything get_project shows: the project, its people, milestones, latest update,
// links, dependencies, documents, issue counts and estimates.
func (self *ProjectService) detail(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, id uuid.UUID) (models.ProjectDetail, error) {
	value, err := self.value(ctx, tx, actor, id)
	if err != nil {
		return models.ProjectDetail{}, err
	}
	teams, err := actor.TeamIDs(ctx)
	if err != nil {
		return models.ProjectDetail{}, err
	}
	detail := models.ProjectDetail{Project: value}
	creator, err := self.repo.Project().Creator(ctx, tx, id)
	if err != nil {
		return detail, err
	}
	detail.Creator = models.UserRefOf(creator)
	members, err := self.repo.Project().Members(ctx, tx, id)
	if err != nil {
		return detail, err
	}
	detail.Members = make([]models.UserRef, len(members))
	for i, u := range members {
		detail.Members[i] = *models.UserRefOf(u)
	}
	subscribers, err := self.repo.Project().Subscribers(ctx, tx, id)
	if err != nil {
		return detail, err
	}
	detail.Subscribers = make([]models.UserRef, len(subscribers))
	for i, u := range subscribers {
		detail.Subscribers[i] = *models.UserRefOf(u)
	}
	if detail.Links, err = self.Links(ctx, tx, entitylink.FieldProjectID, id); err != nil {
		return detail, err
	}
	relations, err := self.repo.Project().Relations(ctx, tx, id, teams)
	if err != nil {
		return detail, err
	}
	detail.Relations = models.ProjectRelations{
		Blocks: projectRefs(relations.Blocks), BlockedBy: projectRefs(relations.BlockedBy), Related: projectRefs(relations.Related),
	}
	if detail.Documents, err = self.Documents(ctx, tx, entitylink.FieldProjectID, id); err != nil {
		return detail, err
	}
	if detail.IssueCountByStateType, err = self.repo.Project().IssueCountByStateType(ctx, tx, id, teams); err != nil {
		return detail, err
	}
	if detail.Estimate, err = self.repo.Project().Estimate(ctx, tx, id, teams); err != nil {
		return detail, err
	}
	if detail.Milestones, err = self.milestones.OfProject(ctx, tx, actor, id); err != nil {
		return detail, err
	}
	if detail.LatestUpdate, err = self.updates.Latest(ctx, tx, &id, nil); err != nil {
		return detail, err
	}
	return detail, nil
}

func (self *ProjectService) Get(ctx context.Context, actor *auth.Actor, input *IDInput) (models.ProjectDetail, error) {
	id, err := self.resolve.Project(ctx, actor, input.ID)
	if err != nil {
		return models.ProjectDetail{}, err
	}
	return self.detail(ctx, nil, actor, id)
}

func (self *ProjectService) list(ctx context.Context, actor *auth.Actor, page args.Page, ids []uuid.UUID, withDescription bool) (args.PageOf[models.Project], error) {
	values, err := self.Values(ctx, nil, actor, ids, withDescription)
	if err != nil {
		return args.PageOf[models.Project]{}, err
	}
	return args.WrapOf(page, values)
}
