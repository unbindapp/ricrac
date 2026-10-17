package cycle_repo

import (
	"context"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"ricrac/ent"
	"ricrac/ent/cycle"
	"ricrac/ent/team"
	repository "ricrac/internal/repositories"
)

// automationLock is the key of the advisory lock that keeps replicas from running the
// automations at the same time.
const automationLock int64 = 0x5269_6372_6163_4379

// LockAutomations takes the automations' lock until the transaction ends, false when another
// run holds it.
func (self *CycleRepository) LockAutomations(ctx context.Context, tx repository.TxInterface) (bool, error) {
	rows, err := self.base.Client(tx).QueryContext(ctx, "SELECT pg_try_advisory_xact_lock($1)", automationLock)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	var locked bool
	if rows.Next() {
		err = rows.Scan(&locked)
	}
	return locked, err
}

// TeamsWithCycles is the unarchived teams with cycles on, locked until the transaction ends.
func (self *CycleRepository) TeamsWithCycles(ctx context.Context, tx repository.TxInterface) ([]*ent.Team, error) {
	return self.base.Client(tx).Team.Query().Where(team.CyclesEnabled(true), team.ArchivedAtIsNil()).ForUpdate().All(ctx)
}

// Latest is when the team's last cycle ends (nil without cycles), the highest number and how
// many unarchived cycles start after now.
func (self *CycleRepository) Latest(ctx context.Context, tx repository.TxInterface, teamID uuid.UUID, now time.Time) (*time.Time, int, int, error) {
	rows, err := self.base.Client(tx).QueryContext(ctx,
		`SELECT max(ends_at), coalesce(max(number), 0), count(*) FILTER (WHERE starts_at > $2 AND archived_at IS NULL)
		 FROM cycles WHERE team_id = $1`, teamID, now)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()
	var end *time.Time
	var number, upcoming int
	if rows.Next() {
		err = rows.Scan(&end, &number, &upcoming)
	}
	return end, number, upcoming, err
}

// Ended is the team's unarchived cycles that ended and aren't completed, the earliest first.
func (self *CycleRepository) Ended(ctx context.Context, tx repository.TxInterface, teamID uuid.UUID) ([]*ent.Cycle, error) {
	return self.base.Client(tx).Cycle.Query().
		Where(cycle.TeamID(teamID), cycle.CompletedAtIsNil(), cycle.ArchivedAtIsNil(), func(s *entsql.Selector) {
			s.Where(entsql.ExprP(s.C(cycle.FieldEndsAt) + " <= now()"))
		}).Order(ent.Asc(cycle.FieldEndsAt)).All(ctx)
}

// NextAfter is the team's first unarchived cycle that starts at or after ends and hasn't ended.
func (self *CycleRepository) NextAfter(ctx context.Context, tx repository.TxInterface, teamID uuid.UUID, ends time.Time) (*uuid.UUID, error) {
	ids, err := self.base.Client(tx).Cycle.Query().
		Where(cycle.TeamID(teamID), cycle.ArchivedAtIsNil(), cycle.StartsAtGTE(ends), func(s *entsql.Selector) {
			s.Where(entsql.ExprP(s.C(cycle.FieldEndsAt) + " > now()"))
		}).Order(ent.Asc(cycle.FieldStartsAt)).Limit(1).IDs(ctx)
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	return &ids[0], nil
}

// snapshot is the issue counts of cycle c today ($2), as a history entry.
const snapshot = `(SELECT jsonb_build_object('date', $2::text, 'scope', count(*),
		'completed', count(*) FILTER (WHERE s.type = 'completed'),
		'started', count(*) FILTER (WHERE s.type = 'started'),
		'scopeEstimate', coalesce(sum(i.estimate), 0),
		'completedEstimate', coalesce(sum(i.estimate) FILTER (WHERE s.type = 'completed'), 0))
	 FROM issues i JOIN workflow_states s ON s.id = i.state_id WHERE i.cycle_id = c.id AND i.trashed_at IS NULL)`

// Complete marks the cycle completed, replacing today's snapshot in its history with a last one.
func (self *CycleRepository) Complete(ctx context.Context, tx repository.TxInterface, id uuid.UUID, today string) error {
	_, err := self.base.Client(tx).ExecContext(ctx,
		`UPDATE cycles c SET completed_at = now(), updated_at = now(), history =
			(SELECT coalesce(jsonb_agg(e ORDER BY n), '[]') FROM jsonb_array_elements(c.history) WITH ORDINALITY h(e, n)
			 WHERE e->>'date' IS DISTINCT FROM $2::text) || jsonb_build_array(`+snapshot+`)
		 WHERE c.id = $1`, id, today)
	return err
}

// RecordProgress adds today's snapshot to the history of the running cycles of teams that don't
// have one yet.
func (self *CycleRepository) RecordProgress(ctx context.Context, tx repository.TxInterface, teams []uuid.UUID, today string) error {
	_, err := self.base.Client(tx).ExecContext(ctx,
		`UPDATE cycles c SET history = c.history || jsonb_build_array(`+snapshot+`)
		 WHERE c.team_id = ANY($1) AND c.archived_at IS NULL AND c.completed_at IS NULL
		 AND c.starts_at <= now() AND c.ends_at > now()
		 AND NOT c.history @> jsonb_build_array(jsonb_build_object('date', $2::text))`, teams, today)
	return err
}
