package milestone_repo

import (
	"context"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/githubmilestonelink"
	"ricrac/ent/issue"
	"ricrac/ent/predicate"
	"ricrac/ent/projectmilestone"
	"ricrac/internal/models"
	repository "ricrac/internal/repositories"
	project_repo "ricrac/internal/repositories/project"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i MilestoneRepositoryInterface -p milestone_repo -s MilestoneRepository -o milestone_repository_iface.go
type MilestoneRepository struct {
	base *repository.BaseRepository
}

func NewMilestoneRepository(db *ent.Client) *MilestoneRepository {
	return &MilestoneRepository{base: repository.NewBaseRepository(db)}
}

// Load is the milestones with their projects and GitHub milestones, in the order of ids.
func (self *MilestoneRepository) Load(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.ProjectMilestone, error) {
	milestones, err := self.base.Client(tx).ProjectMilestone.Query().Where(projectmilestone.IDIn(ids...)).WithProject().
		WithGithubMilestoneLinks(func(q *ent.GithubMilestoneLinkQuery) {
			q.WithRepoSync().Order(ent.Asc(githubmilestonelink.FieldCreatedAt))
		}).All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*ent.ProjectMilestone, len(milestones))
	for _, m := range milestones {
		byID[m.ID] = m
	}
	ordered := make([]*ent.ProjectMilestone, 0, len(ids))
	for _, id := range ids {
		if m, ok := byID[id]; ok {
			ordered = append(ordered, m)
		}
	}
	return ordered, nil
}

// Stats is each milestone's issue counts and progress, counting the issues viewer can see.
func (self *MilestoneRepository) Stats(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID, viewer uuid.UUID) (map[uuid.UUID]models.ProjectIssueStats, error) {
	return project_repo.IssueStats(ctx, self.base.Client(tx), "project_issue_stats(NULL, id, $2)", ids, viewer)
}

// OfProject is the project's milestones in order; limit 0 is all of them.
func (self *MilestoneRepository) OfProject(ctx context.Context, tx repository.TxInterface, project uuid.UUID, limit, offset int64) ([]uuid.UUID, error) {
	return self.ordered(ctx, tx, projectmilestone.ProjectID(project), limit, offset)
}

// OfWorkspace is the workspace's milestones that are in no project, open ones first, in order.
func (self *MilestoneRepository) OfWorkspace(ctx context.Context, tx repository.TxInterface, workspace uuid.UUID, limit, offset int64) ([]uuid.UUID, error) {
	return self.ordered(ctx, tx, projectmilestone.And(projectmilestone.WorkspaceID(workspace), projectmilestone.ProjectIDIsNil()), limit, offset)
}

func (self *MilestoneRepository) ordered(ctx context.Context, tx repository.TxInterface, where predicate.ProjectMilestone, limit, offset int64) ([]uuid.UUID, error) {
	return self.base.Client(tx).ProjectMilestone.Query().Where(where).
		Modify(func(s *entsql.Selector) {
			s.Select(s.C(projectmilestone.FieldID))
			s.OrderExpr(entsql.Expr(s.C(projectmilestone.FieldClosedAt) + " IS NOT NULL"))
			s.OrderBy(s.C(projectmilestone.FieldSortOrder))
			s.OrderExpr(entsql.Expr(s.C(projectmilestone.FieldTargetDate) + " NULLS LAST"))
			s.OrderBy(s.C(projectmilestone.FieldCreatedAt))
			if limit > 0 {
				s.Limit(int(limit)).Offset(int(offset))
			}
		}).IDs(ctx)
}

func (self *MilestoneRepository) Get(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.ProjectMilestone, error) {
	return self.base.Client(tx).ProjectMilestone.Get(ctx, id)
}

// Issues is the milestone's issues not in the trash, in one of teams, in their order, with their
// team, state and assignee.
func (self *MilestoneRepository) Issues(ctx context.Context, id uuid.UUID, teams []uuid.UUID) ([]*ent.Issue, error) {
	return self.base.DB.Issue.Query().
		Where(issue.MilestoneID(id), issue.TrashedAtIsNil(), issue.TeamIDIn(teams...)).
		WithTeam().WithState().WithAssignee().Order(ent.Asc(issue.FieldSortOrder)).All(ctx)
}

// Create adds a milestone to the project, or to the workspace when project is nil.
func (self *MilestoneRepository) Create(ctx context.Context, tx repository.TxInterface, workspace uuid.UUID, project *uuid.UUID, name string) (uuid.UUID, error) {
	created, err := self.base.Client(tx).ProjectMilestone.Create().SetWorkspaceID(workspace).SetNillableProjectID(project).SetName(name).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// CreateFull adds a milestone with its description, target date and position.
func (self *MilestoneRepository) CreateFull(ctx context.Context, tx repository.TxInterface, workspace, project uuid.UUID, name string, description *string, target *time.Time, sortOrder float64) (uuid.UUID, error) {
	created, err := self.base.Client(tx).ProjectMilestone.Create().SetWorkspaceID(workspace).SetProjectID(project).SetName(name).
		SetNillableDescription(description).SetNillableTargetDate(target).SetSortOrder(sortOrder).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// Update is the milestone's update builder, to set fields on.
func (self *MilestoneRepository) Update(tx repository.TxInterface, id uuid.UUID) *ent.ProjectMilestoneUpdateOne {
	return self.base.Client(tx).ProjectMilestone.UpdateOneID(id)
}

// NameTaken says whether another milestone of the same project, or of the workspace when it is
// in none, has this name, in any case.
func (self *MilestoneRepository) NameTaken(ctx context.Context, tx repository.TxInterface, id uuid.UUID, name string) (bool, error) {
	milestone, err := self.base.Client(tx).ProjectMilestone.Get(ctx, id)
	if err != nil {
		return false, err
	}
	return self.base.Client(tx).ProjectMilestone.Query().
		Where(siblingsOf(milestone), projectmilestone.IDNEQ(id), named(name)).Exist(ctx)
}

func siblingsOf(milestone *ent.ProjectMilestone) predicate.ProjectMilestone {
	if milestone.ProjectID != nil {
		return projectmilestone.ProjectID(*milestone.ProjectID)
	}
	return projectmilestone.And(projectmilestone.WorkspaceID(milestone.WorkspaceID), projectmilestone.ProjectIDIsNil())
}

func named(name string) predicate.ProjectMilestone {
	return func(s *entsql.Selector) {
		s.Where(repository.Raw("lower("+s.C(projectmilestone.FieldName)+") = lower(?)", name))
	}
}

// WorkspaceNamed is the workspace's milestone in no project with this name, in any case; nil when
// there's none.
func (self *MilestoneRepository) WorkspaceNamed(ctx context.Context, tx repository.TxInterface, workspace uuid.UUID, name string) (*ent.ProjectMilestone, error) {
	found, err := self.base.Client(tx).ProjectMilestone.Query().
		Where(projectmilestone.WorkspaceID(workspace), projectmilestone.ProjectIDIsNil(), named(name)).First(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}

func (self *MilestoneRepository) Delete(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	return self.base.Client(tx).ProjectMilestone.DeleteOneID(id).Exec(ctx)
}
