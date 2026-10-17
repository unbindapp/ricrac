package issue_repo

import (
	"context"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/cycle"
	"ricrac/ent/issue"
	"ricrac/ent/team"
	"ricrac/ent/workflowstate"
	repository "ricrac/internal/repositories"
)

func (self *IssueRepository) State(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.WorkflowState, error) {
	return self.base.Client(tx).WorkflowState.Get(ctx, id)
}

// DefaultState is the state new issues of the team start in: its default, else the first
// backlog, unstarted, started, triage, completed or canceled state.
func (self *IssueRepository) DefaultState(ctx context.Context, tx repository.TxInterface, teamID uuid.UUID) (uuid.UUID, bool, error) {
	ids, err := self.base.Client(tx).WorkflowState.Query().
		Where(workflowstate.TeamID(teamID), workflowstate.ArchivedAtIsNil()).
		Modify(func(s *entsql.Selector) {
			teams := entsql.Table(team.Table)
			s.Join(teams).On(s.C(workflowstate.FieldTeamID), teams.C(team.FieldID))
			s.Select(s.C(workflowstate.FieldID))
			s.OrderExpr(entsql.Expr(s.C(workflowstate.FieldID) + " = " + teams.C(team.FieldDefaultIssueStateID) + " DESC"))
			s.OrderExpr(entsql.Expr("array_position(ARRAY['backlog','unstarted','started','triage','completed','canceled'], " + s.C(workflowstate.FieldType) + ")"))
			s.OrderExpr(entsql.Expr(s.C(workflowstate.FieldPosition)))
			s.Limit(1)
		}).IDs(ctx)
	if err != nil || len(ids) == 0 {
		return uuid.Nil, false, err
	}
	return ids[0], true, nil
}

// FirstStateOfType is the team's first unarchived state of a type, nil when there's none.
func (self *IssueRepository) FirstStateOfType(ctx context.Context, tx repository.TxInterface, teamID uuid.UUID, kind string) (*uuid.UUID, error) {
	ids, err := self.base.Client(tx).WorkflowState.Query().
		Where(workflowstate.TeamID(teamID), workflowstate.Type(kind), workflowstate.ArchivedAtIsNil()).
		Order(ent.Asc(workflowstate.FieldPosition)).Limit(1).IDs(ctx)
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	return &ids[0], nil
}

func (self *IssueRepository) Team(ctx context.Context, tx repository.TxInterface, id uuid.UUID) (*ent.Team, error) {
	return self.base.Client(tx).Team.Get(ctx, id)
}

type StateStamps struct {
	State                                 uuid.UUID
	StartedAt, CompletedAt, CanceledAt    *time.Time
	StartedTriageAt, TriagedAt, UpdatedAt *time.Time
}

// MoveToState sets the issue's state and the times it stamps, clearing auto_closed_at.
func (self *IssueRepository) MoveToState(ctx context.Context, tx repository.TxInterface, id uuid.UUID, stamps StateStamps) error {
	update := self.base.Client(tx).Issue.UpdateOneID(id).SetStateID(stamps.State).ClearAutoClosedAt().SetUpdatedAt(*stamps.UpdatedAt)
	set := func(value *time.Time, setter func(time.Time) *ent.IssueUpdateOne, clear func() *ent.IssueUpdateOne) {
		if value == nil {
			clear()
			return
		}
		setter(*value)
	}
	set(stamps.StartedAt, update.SetStartedAt, update.ClearStartedAt)
	set(stamps.CompletedAt, update.SetCompletedAt, update.ClearCompletedAt)
	set(stamps.CanceledAt, update.SetCanceledAt, update.ClearCanceledAt)
	set(stamps.StartedTriageAt, update.SetStartedTriageAt, update.ClearStartedTriageAt)
	set(stamps.TriagedAt, update.SetTriagedAt, update.ClearTriagedAt)
	return update.Exec(ctx)
}

// AddToCurrentCycle puts the issue in its team's current cycle when the team adds issues that
// start (or complete, per autoAddColumn) to it.
func (self *IssueRepository) AddToCurrentCycle(ctx context.Context, tx repository.TxInterface, id, teamID uuid.UUID, started bool) error {
	t, err := self.base.Client(tx).Team.Get(ctx, teamID)
	if err != nil {
		return err
	}
	if !t.CyclesEnabled || started && !t.CycleAutoAddStarted || !started && !t.CycleAutoAddCompleted {
		return nil
	}
	current, err := self.base.Client(tx).Cycle.Query().
		Where(cycle.TeamID(teamID), cycle.ArchivedAtIsNil(), func(s *entsql.Selector) {
			s.Where(entsql.ExprP(s.C(cycle.FieldStartsAt) + " <= now() AND " + s.C(cycle.FieldEndsAt) + " > now()"))
		}).
		Order(ent.Desc(cycle.FieldStartsAt)).Limit(1).IDs(ctx)
	if err != nil {
		return err
	}
	update := self.base.Client(tx).Issue.UpdateOneID(id)
	if len(current) == 0 {
		return update.ClearCycleID().Exec(ctx)
	}
	return update.SetCycleID(current[0]).Exec(ctx)
}

// Unblocked is the issues the issue blocked that nothing open blocks anymore.
func (self *IssueRepository) Unblocked(ctx context.Context, tx repository.TxInterface, id uuid.UUID) ([]uuid.UUID, error) {
	rows, err := self.base.Client(tx).QueryContext(ctx,
		`SELECT r.related_issue_id FROM issue_relations r WHERE r.issue_id = $1 AND r.type = 'blocks'
		 AND NOT EXISTS (SELECT 1 FROM issue_relations o JOIN issues b ON b.id = o.issue_id
			JOIN workflow_states s ON s.id = b.state_id
			WHERE o.related_issue_id = r.related_issue_id AND o.type = 'blocks' AND o.issue_id <> $1
			AND s.type NOT IN ('completed', 'canceled') AND b.trashed_at IS NULL)`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var freed []uuid.UUID
	for rows.Next() {
		var blocked uuid.UUID
		if err := rows.Scan(&blocked); err != nil {
			return nil, err
		}
		freed = append(freed, blocked)
	}
	return freed, rows.Err()
}

// OpenChildren is the sub-issues not in the trash, completed or canceled.
func (self *IssueRepository) OpenChildren(ctx context.Context, tx repository.TxInterface, parent uuid.UUID) ([]*ent.Issue, error) {
	return self.base.Client(tx).Issue.Query().
		Where(issue.ParentID(parent), issue.TrashedAtIsNil(), issue.HasStateWith(workflowstate.TypeNotIn("completed", "canceled"))).
		All(ctx)
}

// ChildCounts is how many of the sub-issues not in the trash are open, and how many completed.
func (self *IssueRepository) ChildCounts(ctx context.Context, tx repository.TxInterface, parent uuid.UUID) (open, done int, err error) {
	rows, err := self.base.Client(tx).QueryContext(ctx,
		`SELECT count(*) FILTER (WHERE s.type NOT IN ('completed', 'canceled')), count(*) FILTER (WHERE s.type = 'completed')
		 FROM issues c JOIN workflow_states s ON s.id = c.state_id WHERE c.parent_id = $1 AND c.trashed_at IS NULL`, parent)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	if rows.Next() {
		err = rows.Scan(&open, &done)
	}
	return open, done, err
}
