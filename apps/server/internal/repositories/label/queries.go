package label_repo

import (
	"context"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/issuelabel"
	"ricrac/ent/issuelabellink"
	"ricrac/ent/team"
	repository "ricrac/internal/repositories"
)

func byLowerName(s *entsql.Selector) {
	s.OrderExpr(entsql.Expr("lower(" + s.C(issuelabel.FieldName) + ")"))
}

// Load is the labels with what their answer shows (parent, team, creator, children by name),
// in the order of ids.
func (self *LabelRepository) Load(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.IssueLabel, error) {
	labels, err := self.base.Client(tx).IssueLabel.Query().
		Where(issuelabel.IDIn(ids...)).
		WithParent().WithTeam().WithCreator().
		WithChildren(func(q *ent.IssueLabelQuery) { q.Order(byLowerName) }).
		All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*ent.IssueLabel, len(labels))
	for _, l := range labels {
		byID[l.ID] = l
	}
	ordered := make([]*ent.IssueLabel, 0, len(ids))
	for _, id := range ids {
		if l, ok := byID[id]; ok {
			ordered = append(ordered, l)
		}
	}
	return ordered, nil
}

// IssueCounts is how many issues not in the trash have each label, or for a group, one of its
// labels.
func (self *LabelRepository) IssueCounts(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	rows, err := self.base.Client(tx).QueryContext(ctx,
		`SELECT l.id, (SELECT count(DISTINCT ll.issue_id) FROM issue_label_links ll JOIN issues i ON i.id = ll.issue_id
			WHERE i.trashed_at IS NULL AND (ll.label_id = l.id
				OR (l.is_group AND ll.label_id IN (SELECT c.id FROM issue_labels c WHERE c.parent_id = l.id))))
		 FROM issue_labels l WHERE l.id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[uuid.UUID]int{}
	for rows.Next() {
		var id uuid.UUID
		var count int
		if err := rows.Scan(&id, &count); err != nil {
			return nil, err
		}
		counts[id] = count
	}
	return counts, rows.Err()
}

func (self *LabelRepository) Get(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.IssueLabel, error) {
	return self.base.Client(tx).IssueLabel.Get(ctx, id)
}

// GetForUpdate locks the label until the transaction ends.
func (self *LabelRepository) GetForUpdate(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.IssueLabel, error) {
	return self.base.Client(tx).IssueLabel.Query().Where(issuelabel.ID(id)).ForUpdate().Only(ctx)
}

type ListFilter struct {
	Workspace      uuid.UUID
	Teams          []uuid.UUID
	IncludeRetired bool
	// An ILIKE pattern on the label's or its group's name.
	Pattern *string
	Limit   int64
	Offset  int64
}

// List is the labels of the workspace and of the unarchived teams among Teams, each group
// followed by its labels, by name.
func (self *LabelRepository) List(ctx context.Context, filter ListFilter) ([]uuid.UUID, error) {
	return self.base.DB.IssueLabel.Query().
		Where(issuelabel.WorkspaceID(filter.Workspace),
			issuelabel.Or(issuelabel.TeamIDIsNil(), issuelabel.HasTeamWith(team.IDIn(filter.Teams...), team.ArchivedAtIsNil()))).
		Modify(func(s *entsql.Selector) {
			parents := entsql.Table(issuelabel.Table).As("p")
			s.LeftJoin(parents).On(parents.C(issuelabel.FieldID), s.C(issuelabel.FieldParentID))
			if !filter.IncludeRetired {
				s.Where(entsql.IsNull(s.C(issuelabel.FieldRetiredAt)))
			}
			if filter.Pattern != nil {
				s.Where(repository.Raw(s.C(issuelabel.FieldName)+" ILIKE ? OR "+parents.C(issuelabel.FieldName)+" ILIKE ?", *filter.Pattern, *filter.Pattern))
			}
			s.Select(s.C(issuelabel.FieldID))
			s.OrderExpr(entsql.Expr("lower(coalesce(" + parents.C(issuelabel.FieldName) + ", " + s.C(issuelabel.FieldName) + "))"))
			s.OrderExpr(entsql.Expr(s.C(issuelabel.FieldParentID) + " IS NOT NULL"))
			s.OrderExpr(entsql.Expr("lower(" + s.C(issuelabel.FieldName) + ")"))
			s.OrderExpr(entsql.Expr(s.C(issuelabel.FieldTeamID) + " NULLS FIRST"))
			s.Limit(int(filter.Limit)).Offset(int(filter.Offset))
		}).IDs(ctx)
}

// NameTaken says whether another label of the same team (or the workspace) and group has this
// name, in any case.
func (self *LabelRepository) NameTaken(ctx context.Context, tx repository.TxInterface, workspace uuid.UUID, teamID, parent *uuid.UUID, name string, except uuid.UUID) (bool, error) {
	query := self.base.Client(tx).IssueLabel.Query().
		Where(issuelabel.WorkspaceID(workspace), issuelabel.IDNEQ(except), func(s *entsql.Selector) {
			s.Where(repository.Raw("lower("+s.C(issuelabel.FieldName)+") = lower(?)", name))
		})
	if teamID == nil {
		query = query.Where(issuelabel.TeamIDIsNil())
	} else {
		query = query.Where(issuelabel.TeamID(*teamID))
	}
	if parent == nil {
		query = query.Where(issuelabel.ParentIDIsNil())
	} else {
		query = query.Where(issuelabel.ParentID(*parent))
	}
	return query.Exist(ctx)
}

// OnIssues says whether the label is on any issue.
func (self *LabelRepository) OnIssues(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (bool, error) {
	return self.base.Client(tx).IssueLabelLink.Query().Where(issuelabellink.LabelID(id)).Exist(ctx)
}

func (self *LabelRepository) HasChildren(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (bool, error) {
	return self.base.Client(tx).IssueLabel.Query().Where(issuelabel.ParentID(id)).Exist(ctx)
}

// InOwnTree says whether label is group or one of its ancestors.
func (self *LabelRepository) InOwnTree(ctx context.Context, tx repository.TxInterface, group, label uuid.UUID) (bool, error) {
	rows, err := self.base.Client(tx).QueryContext(ctx,
		`WITH RECURSIVE up AS (SELECT id, parent_id FROM issue_labels WHERE id = $1
		 UNION SELECT l.id, l.parent_id FROM issue_labels l JOIN up ON l.id = up.parent_id)
		 SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)`, group, label)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	var loops bool
	if rows.Next() {
		err = rows.Scan(&loops)
	}
	return loops, err
}

// SingleSelectClash is the name of the single-select group when an issue has several of its
// labels.
func (self *LabelRepository) SingleSelectClash(ctx context.Context, tx repository.TxInterface, group uuid.UUID) (string, bool, error) {
	rows, err := self.base.Client(tx).QueryContext(ctx,
		`SELECT g.name FROM issue_labels g JOIN issue_labels c ON c.parent_id = g.id
		 JOIN issue_label_links ll ON ll.label_id = c.id
		 WHERE g.id = $1 AND g.group_type = 'singleSelect' GROUP BY g.name, ll.issue_id HAVING count(*) > 1 LIMIT 1`, group)
	if err != nil {
		return "", false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return "", false, rows.Err()
	}
	var name string
	err = rows.Scan(&name)
	return name, err == nil, err
}

// IssuesWithLabelOrGroup is how many issues have the label or one of its group's labels.
func (self *LabelRepository) IssuesWithLabelOrGroup(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (int, error) {
	rows, err := self.base.Client(tx).QueryContext(ctx,
		`SELECT count(DISTINCT issue_id) FROM issue_label_links
		 WHERE label_id IN (SELECT id FROM issue_labels WHERE id = $1 OR parent_id = $1)`, id)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var count int
	if rows.Next() {
		err = rows.Scan(&count)
	}
	return count, err
}
