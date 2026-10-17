package issue_repo

import (
	"context"
	"strings"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/issue"
	"ricrac/ent/issueshare"
	"ricrac/ent/team"
	"ricrac/internal/common/utils"
	repository "ricrac/internal/repositories"
)

// TryFilter runs the condition on no issues, so that mistakes in its SQL surface.
func (self *IssueRepository) TryFilter(ctx context.Context, tx repository.TxInterface, condition *Condition) error {
	selector := entsql.Dialect(dialect.Postgres).SelectExpr(entsql.Expr("1")).From(entsql.Table(issue.Table).As("i")).Where(entsql.False())
	if condition != nil {
		selector.Where(condition.On("i"))
	}
	query, args := selector.Limit(0).Query()
	_, err := self.base.Client(tx).ExecContext(ctx, query, args...)
	return err
}

// Orders are how issues can be listed, by name. IssueAlias stands for the issues table, t for
// the issue's team.
var Orders = map[string]string{
	"updatedAt":  "{i}.updated_at DESC",
	"createdAt":  "{i}.created_at DESC",
	"priority":   "CASE WHEN {i}.priority = 0 THEN 5 ELSE {i}.priority END, {i}.updated_at DESC",
	"dueDate":    "{i}.due_date ASC NULLS LAST, {i}.priority",
	"manual":     "{i}.sort_order ASC",
	"identifier": "t.key, {i}.number",
	"number":     "t.key, {i}.number",
}

type ListQuery struct {
	Workspace uuid.UUID
	// Issues of these teams, and those shared with Viewer.
	Teams           []uuid.UUID
	Viewer          uuid.UUID
	IncludeArchived bool
	IncludeTrashed  bool
	// Words to find, or an identifier.
	Search  string
	Filter  *Condition
	OrderBy string
	Limit   int64
	Offset  int64
}

// alias is how the query names the issues table.
func alias(s *entsql.Selector) string {
	column := s.C(issue.FieldID)
	return column[:strings.LastIndex(column, ".")]
}

// List is the ids of the issues the query finds, in its order.
func (self *IssueRepository) List(ctx context.Context, query ListQuery) ([]uuid.UUID, error) {
	return self.base.DB.Issue.Query().
		Where(issue.WorkspaceID(query.Workspace),
			issue.Or(issue.TeamIDIn(query.Teams...), issue.HasIssueSharesWith(issueshare.UserID(query.Viewer)))).
		Modify(func(s *entsql.Selector) {
			i := alias(s)
			teams := entsql.Table(team.Table).As("t")
			s.Join(teams).On(s.C(issue.FieldTeamID), teams.C(team.FieldID))
			if !query.IncludeArchived {
				s.Where(entsql.IsNull(s.C(issue.FieldArchivedAt)))
			}
			if !query.IncludeTrashed {
				s.Where(entsql.IsNull(s.C(issue.FieldTrashedAt)))
			}
			if query.Search != "" {
				s.Where(searchFor(i, query.Search))
			}
			if query.Filter != nil {
				s.Where(query.Filter.On(i))
			}
			s.Select(s.C(issue.FieldID))
			s.OrderExpr(entsql.Expr(strings.ReplaceAll(Orders[query.OrderBy], IssueAlias, i)))
			s.Limit(int(query.Limit)).Offset(int(query.Offset))
		}).IDs(ctx)
}

// searchFor finds the words, or the issue an identifier names.
func searchFor(i, query string) *entsql.Predicate {
	key, number, ok := utils.ParseIdentifier(query)
	if !ok {
		return Search(i, query).On(i)
	}
	q := &SQL{}
	q.Push("((upper(t.key) = ").Bind(key).Push(" AND " + i + ".number = ").Bind(number).Push(") OR ")
	q.Append(Search(i, query)).Push(")")
	return q.Condition().On(i)
}

// SubTeams is the team and the teams under it, at any depth.
func (self *IssueRepository) SubTeams(ctx context.Context, teamID uuid.UUID) ([]uuid.UUID, error) {
	return scanAll[uuid.UUID](ctx, self.base.DB,
		`WITH RECURSIVE tree AS (SELECT id FROM teams WHERE id = $1
		 UNION SELECT t.id FROM teams t JOIN tree ON t.parent_id = tree.id) SELECT id FROM tree`, teamID)
}

// TeamsByKey is the unarchived teams among ids, by key.
func (self *IssueRepository) TeamsByKey(ctx context.Context, ids []uuid.UUID) ([]*ent.Team, error) {
	return self.base.DB.Team.Query().Where(team.IDIn(ids...), team.ArchivedAtIsNil()).Order(team.ByKey()).All(ctx)
}

// View is a custom view's model, filter and team.
func (self *IssueRepository) View(ctx context.Context, id uuid.UUID) (*ent.CustomView, error) {
	return self.base.DB.CustomView.Get(ctx, id)
}

func scanAll[T any](ctx context.Context, client *ent.Client, query string, args ...any) ([]T, error) {
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []T
	for rows.Next() {
		var value T
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func scanOne[T any](ctx context.Context, client *ent.Client, query string, args ...any) (T, error) {
	var value T
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return value, err
	}
	defer rows.Close()
	if rows.Next() {
		err = rows.Scan(&value)
	}
	if err == nil {
		err = rows.Err()
	}
	return value, err
}
