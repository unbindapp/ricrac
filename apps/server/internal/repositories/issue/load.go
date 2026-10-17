package issue_repo

import (
	"context"

	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/issue"
	"ricrac/ent/issuelabel"
	repository "ricrac/internal/repositories"
)

// WithAnswer loads what an issue's answer shows (see models.IssueOf).
func WithAnswer(q *ent.IssueQuery) *ent.IssueQuery {
	return q.WithWorkspace().WithTeam().WithState().WithAssignee().WithCreator().
		WithLabels(func(q *ent.IssueLabelQuery) { q.WithParent().Order(ent.Asc(issuelabel.FieldName)) }).
		WithProject().WithMilestone().WithCycle().
		WithParent(func(q *ent.IssueQuery) { q.WithTeam().WithState() })
}

// Load is the issues with what their answer shows, in the order of ids.
func (self *IssueRepository) Load(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.Issue, error) {
	issues, err := WithAnswer(self.base.Client(tx).Issue.Query().Where(issue.IDIn(ids...))).All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*ent.Issue, len(issues))
	for _, i := range issues {
		byID[i.ID] = i
	}
	ordered := make([]*ent.Issue, 0, len(ids))
	for _, id := range ids {
		if i, ok := byID[id]; ok {
			ordered = append(ordered, i)
		}
	}
	return ordered, nil
}

// SubIssueCounts is how many sub-issues not in the trash each issue has.
func (self *IssueRepository) SubIssueCounts(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	var rows []struct {
		ParentID uuid.UUID `json:"parent_id"`
		Count    int       `json:"count"`
	}
	err := self.base.Client(tx).Issue.Query().
		Where(issue.ParentIDIn(ids...), issue.TrashedAtIsNil()).
		GroupBy(issue.FieldParentID).Aggregate(ent.Count()).Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	counts := make(map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		counts[row.ParentID] = row.Count
	}
	return counts, nil
}

func (self *IssueRepository) Get(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Issue, error) {
	return self.base.Client(tx).Issue.Get(ctx, id)
}

// GetForUpdate locks the issue until the transaction ends.
func (self *IssueRepository) GetForUpdate(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Issue, error) {
	return self.base.Client(tx).Issue.Query().Where(issue.ID(id)).ForUpdate().Only(ctx)
}

// Identifier is the issue's identifier, like ENG-123.
func (self *IssueRepository) Identifier(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (string, error) {
	found, err := self.base.Client(tx).Issue.Query().Where(issue.ID(id)).WithTeam().Only(ctx)
	if err != nil {
		return "", err
	}
	return found.Edges.Team.Key + "-" + itoa(found.Number), nil
}
