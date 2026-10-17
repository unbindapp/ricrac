package project_repo

import (
	"context"
	"database/sql"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/project"
	"ricrac/ent/projectstatus"
	repository "ricrac/internal/repositories"
)

// Statuses is the workspace's project statuses in use, in their order.
func (self *ProjectRepository) Statuses(ctx context.Context, workspace uuid.UUID, limit, offset int64) ([]*ent.ProjectStatus, error) {
	return self.base.DB.ProjectStatus.Query().
		Where(projectstatus.WorkspaceID(workspace), projectstatus.ArchivedAtIsNil()).
		Order(ent.Asc(projectstatus.FieldPosition), ent.Asc(projectstatus.FieldCreatedAt)).
		Limit(int(limit)).Offset(int(offset)).All(ctx)
}

func (self *ProjectRepository) Status(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.ProjectStatus, error) {
	return self.base.Client(tx).ProjectStatus.Get(ctx, id)
}

// StatusProjectCounts is how many projects not in the trash each status has.
func (self *ProjectRepository) StatusProjectCounts(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	var rows []struct {
		StatusID uuid.UUID `json:"status_id"`
		Count    int       `json:"count"`
	}
	err := self.base.Client(tx).Project.Query().
		Where(project.StatusIDIn(ids...), project.TrashedAtIsNil()).
		GroupBy(project.FieldStatusID).Aggregate(ent.Count()).Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	counts := make(map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		counts[row.StatusID] = row.Count
	}
	return counts, nil
}

// DefaultStatus is the status new projects start in: the first backlog status, else the first
// planned one, else the first.
func (self *ProjectRepository) DefaultStatus(ctx context.Context, tx repository.TxInterface, workspace uuid.UUID) (uuid.UUID, bool, error) {
	found, err := self.base.Client(tx).ProjectStatus.Query().
		Where(projectstatus.WorkspaceID(workspace), projectstatus.ArchivedAtIsNil()).
		Modify(func(s *entsql.Selector) {
			s.OrderExpr(entsql.Expr("array_position(ARRAY['backlog', 'planned'], " + s.C(projectstatus.FieldType) + ") NULLS LAST"))
			s.OrderBy(s.C(projectstatus.FieldPosition))
		}).First(ctx)
	if ent.IsNotFound(err) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return found.ID, true, nil
}

// StatusNameTaken says whether another status in use has this name, in any case.
func (self *ProjectRepository) StatusNameTaken(ctx context.Context, tx repository.TxInterface, workspace, id uuid.UUID, name string) (bool, error) {
	return self.base.Client(tx).ProjectStatus.Query().
		Where(projectstatus.WorkspaceID(workspace), projectstatus.IDNEQ(id), projectstatus.ArchivedAtIsNil(), func(s *entsql.Selector) {
			s.Where(repository.Raw("lower("+s.C(projectstatus.FieldName)+") = lower(?)", name))
		}).Exist(ctx)
}

// CreateStatus adds a status after the workspace's others.
func (self *ProjectRepository) CreateStatus(ctx context.Context, tx repository.TxInterface, workspace uuid.UUID, name, kind, color string) (uuid.UUID, error) {
	client := self.base.Client(tx)
	var last float64
	err := scan(ctx, client, func(rows *sql.Rows) error { return rows.Scan(&last) },
		"SELECT coalesce(max(position), 0) FROM project_statuses WHERE workspace_id = $1", workspace)
	if err != nil {
		return uuid.Nil, err
	}
	created, err := client.ProjectStatus.Create().SetWorkspaceID(workspace).SetName(name).SetType(kind).SetColor(color).
		SetPosition(last + 1).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// UpdateStatus is the status's update builder, to set fields on.
func (self *ProjectRepository) UpdateStatus(tx repository.TxInterface, id uuid.UUID) *ent.ProjectStatusUpdateOne {
	return self.base.Client(tx).ProjectStatus.UpdateOneID(id)
}

// OtherStatuses is how many other statuses in use the workspace has.
func (self *ProjectRepository) OtherStatuses(ctx context.Context, tx repository.TxInterface, workspace, id uuid.UUID) (int, error) {
	return self.base.Client(tx).ProjectStatus.Query().
		Where(projectstatus.WorkspaceID(workspace), projectstatus.IDNEQ(id), projectstatus.ArchivedAtIsNil()).Count(ctx)
}

// ProjectsInStatus is every project in the status, trashed ones too.
func (self *ProjectRepository) ProjectsInStatus(ctx context.Context, tx repository.TxInterface, id uuid.UUID) ([]uuid.UUID, error) {
	return self.base.Client(tx).Project.Query().Where(project.StatusID(id)).IDs(ctx)
}

// MoveProjects moves the projects in one status to another.
func (self *ProjectRepository) MoveProjects(ctx context.Context, tx repository.TxInterface, from, to uuid.UUID, now time.Time) error {
	return self.base.Client(tx).Project.Update().Where(project.StatusID(from)).SetStatusID(to).SetUpdatedAt(now).Exec(ctx)
}

func (self *ProjectRepository) DeleteStatus(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	return self.base.Client(tx).ProjectStatus.DeleteOneID(id).Exec(ctx)
}
