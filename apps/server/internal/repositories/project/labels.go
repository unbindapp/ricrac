package project_repo

import (
	"context"
	"database/sql"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/projectlabel"
	repository "ricrac/internal/repositories"
)

type LabelFilter struct {
	Workspace      uuid.UUID
	IncludeRetired bool
	// An ILIKE pattern on the label's name.
	Pattern *string
	Limit   int64
	Offset  int64
}

// Labels is the workspace's project labels, each group followed by its labels, by name.
func (self *ProjectRepository) Labels(ctx context.Context, filter LabelFilter) ([]uuid.UUID, error) {
	query := self.base.DB.ProjectLabel.Query().Where(projectlabel.WorkspaceID(filter.Workspace))
	if !filter.IncludeRetired {
		query.Where(projectlabel.RetiredAtIsNil())
	}
	if filter.Pattern != nil {
		query.Where(func(s *entsql.Selector) {
			s.Where(repository.Raw(s.C(projectlabel.FieldName)+" ILIKE ?", *filter.Pattern))
		})
	}
	return query.Modify(func(s *entsql.Selector) {
		s.Select(s.C(projectlabel.FieldID))
		s.OrderExpr(entsql.Expr("coalesce((SELECT g.name FROM project_labels g WHERE g.id = " + s.C(projectlabel.FieldParentID) +
			"), " + s.C(projectlabel.FieldName) + ")"))
		s.OrderExpr(entsql.Expr(s.C(projectlabel.FieldParentID) + " IS NOT NULL"))
		s.OrderBy(s.C(projectlabel.FieldName))
		s.Limit(int(filter.Limit)).Offset(int(filter.Offset))
	}).IDs(ctx)
}

// LoadLabels is the labels with their parents and creators, in the order of ids.
func (self *ProjectRepository) LoadLabels(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.ProjectLabel, error) {
	labels, err := self.base.Client(tx).ProjectLabel.Query().Where(projectlabel.IDIn(ids...)).WithParent().WithCreator().All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*ent.ProjectLabel, len(labels))
	for _, l := range labels {
		byID[l.ID] = l
	}
	ordered := make([]*ent.ProjectLabel, 0, len(ids))
	for _, id := range ids {
		if l, ok := byID[id]; ok {
			ordered = append(ordered, l)
		}
	}
	return ordered, nil
}

// LabelProjectCounts is how many projects not in the trash have each label.
func (self *ProjectRepository) LabelProjectCounts(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	counts := map[uuid.UUID]int{}
	err := scan(ctx, self.base.Client(tx), func(rows *sql.Rows) error {
		var id uuid.UUID
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return err
		}
		counts[id] = n
		return nil
	}, `SELECT pl.label_id, count(*) FROM project_label_links pl JOIN projects lp ON lp.id = pl.project_id
		WHERE pl.label_id = ANY($1) AND lp.trashed_at IS NULL GROUP BY pl.label_id`, ids)
	return counts, err
}

type LabelRow struct {
	ID     uuid.UUID
	Name   string
	Parent *string
}

// AllLabels is every project label of the workspace with its group's name, retired ones last,
// by name.
func (self *ProjectRepository) AllLabels(ctx context.Context, workspace uuid.UUID) ([]LabelRow, error) {
	labels, err := self.base.DB.ProjectLabel.Query().Where(projectlabel.WorkspaceID(workspace)).WithParent().
		Modify(func(s *entsql.Selector) {
			s.OrderExpr(entsql.Expr(s.C(projectlabel.FieldRetiredAt) + " IS NOT NULL"))
			s.OrderBy(s.C(projectlabel.FieldName))
		}).All(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]LabelRow, len(labels))
	for i, l := range labels {
		rows[i] = LabelRow{ID: l.ID, Name: l.Name}
		if l.Edges.Parent != nil {
			rows[i].Parent = &l.Edges.Parent.Name
		}
	}
	return rows, nil
}

func (self *ProjectRepository) CreateLabel(ctx context.Context, tx repository.TxInterface, workspace uuid.UUID, name, color string, creator uuid.UUID) (uuid.UUID, error) {
	created, err := self.base.Client(tx).ProjectLabel.Create().SetWorkspaceID(workspace).SetName(name).SetColor(color).
		SetCreatorID(creator).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// UpdateLabel is the label's update builder, to set fields on.
func (self *ProjectRepository) UpdateLabel(tx repository.TxInterface, id uuid.UUID) *ent.ProjectLabelUpdateOne {
	return self.base.Client(tx).ProjectLabel.UpdateOneID(id)
}

type LabelShape struct {
	IsGroup       bool
	ParentIsGroup *bool
	HasChildren   bool
	InUse         bool
}

// LabelShape is whether the label is a group, its parent is one, it holds labels and projects
// use it.
func (self *ProjectRepository) LabelShape(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (LabelShape, error) {
	var shape LabelShape
	err := scan(ctx, self.base.Client(tx), func(rows *sql.Rows) error {
		return rows.Scan(&shape.IsGroup, &shape.ParentIsGroup, &shape.HasChildren, &shape.InUse)
	}, `SELECT l.is_group, g.is_group,
			EXISTS (SELECT 1 FROM project_labels c WHERE c.parent_id = l.id),
			EXISTS (SELECT 1 FROM project_label_links pl WHERE pl.label_id = l.id)
		 FROM project_labels l LEFT JOIN project_labels g ON g.id = l.parent_id WHERE l.id = $1`, id)
	return shape, err
}

// LabelNameClash says whether another unretired label in the same group (or outside groups) has
// the label's name, in any case.
func (self *ProjectRepository) LabelNameClash(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (bool, error) {
	label, err := self.base.Client(tx).ProjectLabel.Get(ctx, id)
	if err != nil {
		return false, err
	}
	query := self.base.Client(tx).ProjectLabel.Query().
		Where(projectlabel.WorkspaceID(label.WorkspaceID), projectlabel.IDNEQ(id), projectlabel.RetiredAtIsNil(), func(s *entsql.Selector) {
			s.Where(repository.Raw("lower("+s.C(projectlabel.FieldName)+") = lower(?)", label.Name))
		})
	if label.ParentID == nil {
		query.Where(projectlabel.ParentIDIsNil())
	} else {
		query.Where(projectlabel.ParentID(*label.ParentID))
	}
	return query.Exist(ctx)
}

// RetireLabel retires the label and its group's labels that aren't retired yet.
func (self *ProjectRepository) RetireLabel(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now time.Time) error {
	return self.base.Client(tx).ProjectLabel.Update().
		Where(projectlabel.Or(projectlabel.ID(id), projectlabel.ParentID(id)), projectlabel.RetiredAtIsNil()).
		SetRetiredAt(now).SetUpdatedAt(now).Exec(ctx)
}

// RestoreLabel brings back the label and its group's labels.
func (self *ProjectRepository) RestoreLabel(ctx context.Context, tx repository.TxInterface, id uuid.UUID, now time.Time) error {
	return self.base.Client(tx).ProjectLabel.Update().
		Where(projectlabel.Or(projectlabel.ID(id), projectlabel.ParentID(id))).
		ClearRetiredAt().SetUpdatedAt(now).Exec(ctx)
}

func (self *ProjectRepository) DeleteLabel(ctx context.Context, tx repository.TxInterface, id uuid.UUID) error {
	return self.base.Client(tx).ProjectLabel.DeleteOneID(id).Exec(ctx)
}
