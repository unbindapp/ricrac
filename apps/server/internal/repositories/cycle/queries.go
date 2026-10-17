package cycle_repo

import (
	"context"
	"fmt"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/cycle"
	"ricrac/ent/issue"
	"ricrac/ent/team"
	repository "ricrac/internal/repositories"
)

// Load is the cycles with their team and its workspace, in the order of ids.
func (self *CycleRepository) Load(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) ([]*ent.Cycle, error) {
	cycles, err := self.base.Client(tx).Cycle.Query().Where(cycle.IDIn(ids...)).
		WithTeam(func(q *ent.TeamQuery) { q.WithWorkspace() }).All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*ent.Cycle, len(cycles))
	for _, c := range cycles {
		byID[c.ID] = c
	}
	ordered := make([]*ent.Cycle, 0, len(ids))
	for _, id := range ids {
		if c, ok := byID[id]; ok {
			ordered = append(ordered, c)
		}
	}
	return ordered, nil
}

// Scope is a cycle's issues not in the trash, counted by state type, with their estimates.
type Scope struct {
	Issues, Completed, Started, Unstarted, Canceled int
	Estimate, CompletedEstimate, StartedEstimate    float64
}

func (self *CycleRepository) Scopes(ctx context.Context, tx repository.TxInterface, ids []uuid.UUID) (map[uuid.UUID]Scope, error) {
	rows, err := self.base.Client(tx).QueryContext(ctx,
		`SELECT c.id, n.issues, n.completed, n.started, n.unstarted, n.canceled, n.estimate, n.completed_estimate, n.started_estimate
		 FROM cycles c CROSS JOIN LATERAL (SELECT count(*) AS issues,
			count(*) FILTER (WHERE s.type = 'completed') AS completed,
			count(*) FILTER (WHERE s.type = 'started') AS started,
			count(*) FILTER (WHERE s.type IN ('triage', 'backlog', 'unstarted')) AS unstarted,
			count(*) FILTER (WHERE s.type = 'canceled') AS canceled,
			coalesce(sum(i.estimate), 0) AS estimate,
			coalesce(sum(i.estimate) FILTER (WHERE s.type = 'completed'), 0) AS completed_estimate,
			coalesce(sum(i.estimate) FILTER (WHERE s.type = 'started'), 0) AS started_estimate
		 FROM issues i JOIN workflow_states s ON s.id = i.state_id WHERE i.cycle_id = c.id AND i.trashed_at IS NULL) n
		 WHERE c.id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	scopes := map[uuid.UUID]Scope{}
	for rows.Next() {
		var id uuid.UUID
		var s Scope
		if err := rows.Scan(&id, &s.Issues, &s.Completed, &s.Started, &s.Unstarted, &s.Canceled, &s.Estimate,
			&s.CompletedEstimate, &s.StartedEstimate); err != nil {
			return nil, err
		}
		scopes[id] = s
	}
	return scopes, rows.Err()
}

type kind struct {
	condition, order string
}

var kinds = map[string]kind{
	"all":       {"TRUE", "starts_at DESC"},
	"current":   {"starts_at <= now() AND ends_at > now()", "starts_at DESC"},
	"previous":  {"ends_at <= now()", "ends_at DESC"},
	"next":      {"starts_at > now()", "starts_at"},
	"upcoming":  {"starts_at > now()", "starts_at"},
	"completed": {"(completed_at IS NOT NULL OR ends_at <= now())", "starts_at DESC"},
}

type ListFilter struct {
	Team uuid.UUID
	// all, current, previous, next, upcoming or completed.
	Kind            string
	IncludeArchived bool
	Limit           int64
	Offset          int64
}

// List is the team's cycles of a kind, in the kind's order.
func (self *CycleRepository) List(ctx context.Context, filter ListFilter) ([]uuid.UUID, error) {
	chosen, ok := kinds[filter.Kind]
	if !ok {
		return nil, fmt.Errorf("no cycle kind %q", filter.Kind)
	}
	query := self.base.DB.Cycle.Query().Where(cycle.TeamID(filter.Team), func(s *entsql.Selector) {
		s.Where(entsql.ExprP(chosen.condition))
	})
	if !filter.IncludeArchived {
		query = query.Where(cycle.ArchivedAtIsNil())
	}
	return query.Order(func(s *entsql.Selector) { s.OrderExpr(entsql.Expr(chosen.order)) }).
		Limit(int(filter.Limit)).Offset(int(filter.Offset)).IDs(ctx)
}

// Issues is the cycle's issues not in the trash, with their team, state and assignee: started
// first, then by priority (none last) and board order.
func (self *CycleRepository) Issues(ctx context.Context, tx repository.TxInterface, id uuid.UUID) ([]*ent.Issue, error) {
	return self.base.Client(tx).Issue.Query().Where(issue.CycleID(id), issue.TrashedAtIsNil()).
		WithTeam().WithState().WithAssignee().
		Order(func(s *entsql.Selector) {
			s.OrderExpr(entsql.Expr("array_position(ARRAY['started','unstarted','backlog','triage','completed','canceled'], " +
				"(SELECT ws.type FROM workflow_states ws WHERE ws.id = " + s.C(issue.FieldStateID) + "))"))
			s.OrderExpr(entsql.Expr("CASE WHEN " + s.C(issue.FieldPriority) + " = 0 THEN 5 ELSE " + s.C(issue.FieldPriority) + " END"))
			s.OrderExpr(entsql.Expr(s.C(issue.FieldSortOrder)))
		}).All(ctx)
}

func (self *CycleRepository) Get(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Cycle, error) {
	return self.base.Client(tx).Cycle.Get(ctx, id)
}

// GetForUpdate locks the cycle until the transaction ends.
func (self *CycleRepository) GetForUpdate(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Cycle, error) {
	return self.base.Client(tx).Cycle.Query().Where(cycle.ID(id)).ForUpdate().Only(ctx)
}

// TeamUsesCycles locks the team until the transaction ends and says whether it has cycles on.
func (self *CycleRepository) TeamUsesCycles(ctx context.Context, tx repository.TxInterface, teamID uuid.UUID) (bool, error) {
	found, err := self.base.Client(tx).Team.Query().Where(team.ID(teamID)).ForUpdate().Only(ctx)
	if err != nil {
		return false, err
	}
	return found.CyclesEnabled, nil
}

// Overlapping is the first unarchived cycle of the team other than except that overlaps starts
// to ends, nil when there's none.
func (self *CycleRepository) Overlapping(ctx context.Context, tx repository.TxInterface, teamID uuid.UUID, except *uuid.UUID, starts, ends time.Time) (*ent.Cycle, error) {
	query := self.base.Client(tx).Cycle.Query().
		Where(cycle.TeamID(teamID), cycle.ArchivedAtIsNil(), cycle.StartsAtLT(ends), cycle.EndsAtGT(starts))
	if except != nil {
		query = query.Where(cycle.IDNEQ(*except))
	}
	found, err := query.Order(ent.Asc(cycle.FieldStartsAt)).First(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	return found, err
}
