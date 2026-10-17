package milestone_repo

import (
	"context"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/issue"
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

// Load is the milestones with their projects, in the order of ids.
func (self *MilestoneRepository) Load(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.ProjectMilestone, error) {
	milestones, err := self.base.Client(tx).ProjectMilestone.Query().Where(projectmilestone.IDIn(ids...)).WithProject().All(ctx)
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
	return self.base.Client(tx).ProjectMilestone.Query().Where(projectmilestone.ProjectID(project)).
		Modify(func(s *entsql.Selector) {
			s.Select(s.C(projectmilestone.FieldID))
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

func (self *MilestoneRepository) Create(ctx context.Context, tx repository.TxInterface, project uuid.UUID, name string) (uuid.UUID, error) {
	created, err := self.base.Client(tx).ProjectMilestone.Create().SetProjectID(project).SetName(name).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// CreateFull adds a milestone with its description, target date and position.
func (self *MilestoneRepository) CreateFull(ctx context.Context, tx repository.TxInterface, project uuid.UUID, name string, description *string, target *time.Time, sortOrder float64) (uuid.UUID, error) {
	created, err := self.base.Client(tx).ProjectMilestone.Create().SetProjectID(project).SetName(name).
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

// NameTaken says whether another milestone of the same project has this name, in any case.
func (self *MilestoneRepository) NameTaken(ctx context.Context, tx repository.TxInterface, id uuid.UUID, name string) (bool, error) {
	milestone, err := self.base.Client(tx).ProjectMilestone.Get(ctx, id)
	if err != nil {
		return false, err
	}
	return self.base.Client(tx).ProjectMilestone.Query().
		Where(projectmilestone.ProjectID(milestone.ProjectID), projectmilestone.IDNEQ(id), func(s *entsql.Selector) {
			s.Where(repository.Raw("lower("+s.C(projectmilestone.FieldName)+") = lower(?)", name))
		}).Exist(ctx)
}

func (self *MilestoneRepository) Delete(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	return self.base.Client(tx).ProjectMilestone.DeleteOneID(id).Exec(ctx)
}
