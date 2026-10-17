package ops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
)

// Cycles: a team's time-boxed iterations. They are created ahead by the automations, which also
// complete ended cycles (rolling unfinished issues over) and record daily progress.

func cycleOps() []*Op {
	return []*Op{
		op("list_cycles", app.Read, []string{"GET /teams/{team}/cycles"},
			"List a team's cycles with their progress (issue counts by state type, estimate sums, daily history). "+
				"type narrows to the current, previous or next cycle, the upcoming ones or the completed ones. Newest "+
				"first, except upcoming (soonest first). Archived cycles are left out unless includeArchived is set.",
			listCycles).returns(ResultOf[PageOf[Cycle]]()),
		op("get_cycle", app.Read, []string{"GET /cycles/{id}"},
			"Read a cycle by id, or by number, name or \"current\" / \"next\" / \"previous\" together with team: its "+
				"dates, progress, daily history and issues (unless includeIssues is false).",
			getCycle).returns(ResultOf[CycleDetail]()),
		op("save_cycle", app.Write, []string{"POST /cycles", "PATCH /cycles/{id}"},
			"Create a cycle (omit id; team, startsAt and endsAt are required and the team must have cycles enabled) "+
				"or update one's name, description or dates (pass id). Cycles of a team can't overlap, and completed "+
				"cycles can't be moved. Teams with cycles enabled get their cycles created automatically; only create "+
				"one by hand when the user asks.",
			saveCycle).returns(ResultOf[Cycle]()),
		op("archive_cycle", app.Write, []string{"POST /cycles/{id}/archive"},
			"Archive an upcoming or completed cycle (not the current one). Issues of an archived upcoming cycle leave it.",
			archiveCycle).returns(ResultOf[Cycle]()),
	}
}

/* Representation */

const cycleFrom = `FROM cycles c JOIN teams t ON t.id = c.team_id JOIN workspaces w ON w.id = t.workspace_id
	CROSS JOIN LATERAL (SELECT count(*) AS issues,
		count(*) FILTER (WHERE s.type = 'completed') AS completed,
		count(*) FILTER (WHERE s.type = 'started') AS started,
		count(*) FILTER (WHERE s.type IN ('triage', 'backlog', 'unstarted')) AS unstarted,
		count(*) FILTER (WHERE s.type = 'canceled') AS canceled,
		coalesce(sum(i.estimate), 0) AS estimate,
		coalesce(sum(i.estimate) FILTER (WHERE s.type = 'completed'), 0) AS completed_estimate,
		coalesce(sum(i.estimate) FILTER (WHERE s.type = 'started'), 0) AS started_estimate
	 FROM issues i JOIN workflow_states s ON s.id = i.state_id WHERE i.cycle_id = c.id AND i.trashed_at IS NULL) n`

// cycleJSON is the JSON of the cycle c (with cycleFrom joined).
func cycleJSON(c *app.Ctx) string {
	return fmt.Sprintf(`jsonb_build_object(
		'id', c.id,
		'number', c.number,
		'name', c.name,
		'description', c.description,
		'url', %s || '/' || w.url_key || '/team/' || t.key || '/cycle/' || c.number,
		'startsAt', c.starts_at,
		'endsAt', c.ends_at,
		'completedAt', c.completed_at,
		'archivedAt', c.archived_at,
		'isActive', c.starts_at <= now() AND c.ends_at > now(),
		'isFuture', c.starts_at > now(),
		'isPast', c.ends_at <= now(),
		'team', jsonb_build_object('id', t.id, 'key', t.key, 'name', t.name),
		'progress', CASE WHEN n.issues = 0 THEN 0 ELSE round(n.completed::numeric / n.issues, 4) END,
		'scope', jsonb_build_object('issues', n.issues, 'completed', n.completed, 'started', n.started,
			'unstarted', n.unstarted, 'canceled', n.canceled, 'estimate', n.estimate,
			'completedEstimate', n.completed_estimate, 'startedEstimate', n.started_estimate),
		'history', c.history,
		'createdAt', c.created_at,
		'updatedAt', c.updated_at
	)`, sqlLiteral(c.PublicURL()))
}

func cycleValue(c *app.Ctx, q app.Querier, id uuid.UUID) (M, error) {
	value, found, err := app.Maybe[M](c, q, fmt.Sprintf("SELECT %s %s WHERE c.id = $1", cycleJSON(c), cycleFrom), id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, app.NotFound("No such cycle")
	}
	return value, nil
}

// refUUID is the id of the reference under key, like the team of a value: nil when there's none.
func refUUID(value M, key string) *uuid.UUID {
	ref, _ := value[key].(M)
	text, _ := ref["id"].(string)
	id, ok := asUUID(text)
	if !ok {
		return nil
	}
	return &id
}

/* Read */

type ListCyclesArgs struct {
	Team            string  `json:"team" doc:"Team key, name or id."`
	Type            *string `json:"type" doc:"current, previous, next, upcoming, completed or all (default)."`
	IncludeArchived bool    `json:"includeArchived,omitempty"`
	Page
}

func listCycles(c *app.Ctx, args ListCyclesArgs) (any, error) {
	team, err := resolveTeam(c, args.Team)
	if err != nil {
		return nil, err
	}
	kind := "all"
	if args.Type != nil {
		kind = *args.Type
	}
	kinds := map[string]struct {
		condition, order string
		single           bool
	}{
		"all":       {"TRUE", "c.starts_at DESC", false},
		"current":   {"c.starts_at <= now() AND c.ends_at > now()", "c.starts_at DESC", true},
		"previous":  {"c.ends_at <= now()", "c.ends_at DESC", true},
		"next":      {"c.starts_at > now()", "c.starts_at", true},
		"upcoming":  {"c.starts_at > now()", "c.starts_at", false},
		"completed": {"(c.completed_at IS NOT NULL OR c.ends_at <= now())", "c.starts_at DESC", false},
	}
	chosen, ok := kinds[kind]
	if !ok {
		return nil, app.BadRequest("type %q isn't one of current, previous, next, upcoming, completed, all", kind)
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	q := app.NewSQL(fmt.Sprintf("SELECT %s %s WHERE c.team_id = ", cycleJSON(c), cycleFrom)).Bind(team)
	q.Push(" AND " + chosen.condition)
	if !args.IncludeArchived {
		q.Push(" AND c.archived_at IS NULL")
	}
	limit := args.limit() + 1
	if chosen.single {
		limit = 1
	}
	q.Push(" ORDER BY " + chosen.order + " LIMIT ").Bind(limit).Push(" OFFSET ").Bind(offset)
	nodes, err := app.All[any](c, c.DB(), q.String(), q.Args...)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

type GetCycleArgs struct {
	ID            string  `json:"id" doc:"The cycle's id, or its number, name or \"current\" / \"next\" / \"previous\" (with team)."`
	Team          *string `json:"team" doc:"Team key, name or id, when id isn't the cycle's id."`
	IncludeIssues *bool   `json:"includeIssues" doc:"List the cycle's issues (default true)."`
}

func findCycle(c *app.Ctx, team *string, value string) (uuid.UUID, error) {
	if team == nil {
		return resolveCycle(c, nil, value)
	}
	id, err := resolveTeam(c, *team)
	if err != nil {
		return uuid.Nil, err
	}
	return resolveCycle(c, &id, value)
}

func getCycle(c *app.Ctx, args GetCycleArgs) (any, error) {
	id, err := findCycle(c, args.Team, args.ID)
	if err != nil {
		return nil, err
	}
	value, err := cycleValue(c, c.DB(), id)
	if err != nil {
		return nil, err
	}
	if args.IncludeIssues != nil && !*args.IncludeIssues {
		return value, nil
	}
	value["issues"], err = app.One[any](c, c.DB(),
		`SELECT coalesce(jsonb_agg(jsonb_build_object('id', i.id, 'identifier', t.key || '-' || i.number,
			'title', i.title, 'state', state_ref(i.state_id), 'assignee', user_ref(i.assignee_id),
			'estimate', i.estimate, 'priority', i.priority)
			ORDER BY array_position(ARRAY['started','unstarted','backlog','triage','completed','canceled'], s.type),
				CASE WHEN i.priority = 0 THEN 5 ELSE i.priority END, i.sort_order), '[]')
		 FROM issues i JOIN teams t ON t.id = i.team_id JOIN workflow_states s ON s.id = i.state_id
		 WHERE i.cycle_id = $1 AND i.trashed_at IS NULL`, id)
	if err != nil {
		return nil, err
	}
	return value, nil
}

/* Save */

type SaveCycleArgs struct {
	ID          *string          `json:"id" doc:"Only to update a cycle: its id (or number / name with team). Omit to create one."`
	Team        *string          `json:"team" doc:"Team key, name or id. Required to create."`
	Name        Nullable[string] `json:"name" doc:"Null clears it (the cycle is then shown as \"Cycle <number>\")."`
	Description Nullable[string] `json:"description"`
	StartsAt    *string          `json:"startsAt" doc:"ISO 8601 date or time (a date starts at 00:00 UTC). Required to create."`
	EndsAt      *string          `json:"endsAt" doc:"ISO 8601 date or time, after startsAt. Required to create."`
}

// trimmedOrNil is the trimmed text, or nil when there's none.
func trimmedOrNil(value *string) *string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return new(strings.TrimSpace(*value))
}

// chronoRFC3339 writes a time as chrono does: +00:00 and as many fractional digits as it needs.
func chronoRFC3339(at time.Time) string {
	at = at.UTC()
	nanos := at.Nanosecond()
	layout := "2006-01-02T15:04:05.000000000"
	switch {
	case nanos == 0:
		layout = "2006-01-02T15:04:05"
	case nanos%1_000_000 == 0:
		layout = "2006-01-02T15:04:05.000"
	case nanos%1_000 == 0:
		layout = "2006-01-02T15:04:05.000000"
	}
	return at.Format(layout) + "+00:00"
}

func checkNoOverlap(c *app.Ctx, tx pgx.Tx, team uuid.UUID, except *uuid.UUID, starts, ends time.Time) error {
	if !ends.After(starts) {
		return app.BadRequest("endsAt must be after startsAt")
	}
	type clash struct {
		Number int
		Starts time.Time
		Ends   time.Time
	}
	found, ok, err := app.Row[clash](c, tx,
		`SELECT number, starts_at, ends_at FROM cycles WHERE team_id = $1 AND archived_at IS NULL
		 AND id IS DISTINCT FROM $2 AND starts_at < $4 AND ends_at > $3 ORDER BY starts_at LIMIT 1`,
		team, except, starts, ends)
	if err != nil {
		return err
	}
	if ok {
		return app.BadRequest("Those dates overlap cycle %d (%s to %s); a team's cycles can't overlap",
			found.Number, chronoRFC3339(found.Starts), chronoRFC3339(found.Ends))
	}
	return nil
}

func saveCycle(c *app.Ctx, args SaveCycleArgs) (any, error) {
	var value M
	err := inTx(c, func(tx pgx.Tx) error {
		var id uuid.UUID
		var err error
		action := "create"
		if args.ID != nil {
			action = "update"
			if id, err = findCycle(c, args.Team, *args.ID); err != nil {
				return err
			}
			err = updateCycle(c, tx, id, args)
		} else {
			id, err = createCycle(c, tx, args)
		}
		if err != nil {
			return err
		}
		if value, err = cycleValue(c, tx, id); err != nil {
			return err
		}
		return emit(c, tx, "Cycle", action, value, nil, refUUID(value, "team"))
	})
	return value, err
}

func createCycle(c *app.Ctx, tx pgx.Tx, args SaveCycleArgs) (uuid.UUID, error) {
	if args.Team == nil {
		return uuid.Nil, app.BadRequest("Name the team of the new cycle")
	}
	team, err := resolveTeam(c, *args.Team)
	if err != nil {
		return uuid.Nil, err
	}
	if args.StartsAt == nil || args.EndsAt == nil {
		return uuid.Nil, app.BadRequest("startsAt and endsAt are required to create a cycle")
	}
	starts, err := parseDatetime(*args.StartsAt)
	if err != nil {
		return uuid.Nil, err
	}
	ends, err := parseDatetime(*args.EndsAt)
	if err != nil {
		return uuid.Nil, err
	}
	enabled, err := app.One[bool](c, tx, "SELECT cycles_enabled FROM teams WHERE id = $1 FOR UPDATE", team)
	if err != nil {
		return uuid.Nil, err
	}
	if !enabled {
		return uuid.Nil, app.BadRequest("This team doesn't use cycles; turn them on with save_team (cyclesEnabled) first")
	}
	if err := checkNoOverlap(c, tx, team, nil, starts, ends); err != nil {
		return uuid.Nil, err
	}
	return app.One[uuid.UUID](c, tx,
		`INSERT INTO cycles (team_id, number, name, description, starts_at, ends_at)
		 SELECT $1, coalesce(max(number), 0) + 1, $2, $3, $4, $5 FROM cycles WHERE team_id = $1 RETURNING id`,
		team, trimmedOrNil(args.Name.Ptr()), trimmedOrNil(args.Description.Ptr()), starts, ends)
}

func updateCycle(c *app.Ctx, tx pgx.Tx, id uuid.UUID, args SaveCycleArgs) error {
	type current struct {
		Team      uuid.UUID
		Starts    time.Time
		Ends      time.Time
		Completed *time.Time
	}
	row, found, err := app.Row[current](c, tx,
		"SELECT team_id, starts_at, ends_at, completed_at FROM cycles WHERE id = $1 FOR UPDATE", id)
	if err != nil {
		return err
	}
	if !found {
		return app.NotFound("No such cycle")
	}
	if args.StartsAt != nil || args.EndsAt != nil {
		if row.Completed != nil {
			return app.BadRequest("A completed cycle's dates can't change")
		}
		starts, ends := row.Starts, row.Ends
		if args.StartsAt != nil {
			if starts, err = parseDatetime(*args.StartsAt); err != nil {
				return err
			}
		}
		if args.EndsAt != nil {
			if ends, err = parseDatetime(*args.EndsAt); err != nil {
				return err
			}
		}
		if err := checkNoOverlap(c, tx, row.Team, &id, starts, ends); err != nil {
			return err
		}
		if err := app.Exec(c, tx, "UPDATE cycles SET starts_at = $2, ends_at = $3 WHERE id = $1", id, starts, ends); err != nil {
			return err
		}
	}
	if args.Name.Set {
		if err := app.Exec(c, tx, "UPDATE cycles SET name = $2 WHERE id = $1", id, trimmedOrNil(args.Name.Ptr())); err != nil {
			return err
		}
	}
	if args.Description.Set {
		if err := app.Exec(c, tx, "UPDATE cycles SET description = $2 WHERE id = $1", id, trimmedOrNil(args.Description.Ptr())); err != nil {
			return err
		}
	}
	return app.Exec(c, tx, "UPDATE cycles SET updated_at = now() WHERE id = $1", id)
}

type ArchiveCycleArgs struct {
	ID   string  `json:"id" doc:"The cycle's id, or its number or name (with team)."`
	Team *string `json:"team" doc:"Team key, name or id, when id isn't the cycle's id."`
}

func archiveCycle(c *app.Ctx, args ArchiveCycleArgs) (any, error) {
	id, err := findCycle(c, args.Team, args.ID)
	if err != nil {
		return nil, err
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		type state struct {
			Team     uuid.UUID
			Future   bool
			Done     bool
			Archived bool
		}
		row, _, err := app.Row[state](c, tx,
			`SELECT team_id, starts_at > now(), completed_at IS NOT NULL OR ends_at <= now(), archived_at IS NOT NULL
			 FROM cycles WHERE id = $1 FOR UPDATE`, id)
		if err != nil {
			return err
		}
		if !row.Archived {
			if !row.Future && !row.Done {
				return app.BadRequest("The current cycle can't be archived; wait until it is completed")
			}
			if row.Future {
				if err := app.Exec(c, tx,
					`WITH moved AS (UPDATE issues SET cycle_id = NULL, updated_at = now() WHERE cycle_id = $1 RETURNING id)
					 INSERT INTO issue_history (issue_id, actor_id, action, changes)
					 SELECT id, $2, 'updated', jsonb_build_object('cycle', jsonb_build_object('from', cycle_ref($1), 'to', NULL))
					 FROM moved`, id, c.User); err != nil {
					return err
				}
			}
			if err := app.Exec(c, tx, "UPDATE cycles SET archived_at = now(), updated_at = now() WHERE id = $1", id); err != nil {
				return err
			}
		}
		if value, err = cycleValue(c, tx, id); err != nil {
			return err
		}
		return emit(c, tx, "Cycle", "update", value, nil, &row.Team)
	})
	return value, err
}

/* Automations */

// cycleAutomationLock is the key of the advisory lock that keeps replicas from running the
// automations at the same time.
const cycleAutomationLock int64 = 0x5269_6372_6163_4379

type cycleSettings struct {
	ID                 uuid.UUID
	CycleDurationWeeks int
	CycleCooldownWeeks int
	CycleStartDay      int
	UpcomingCycleCount int
	CycleRollover      bool
}

// cycleSnapshot is the issue counts of cycle c today, as a history entry; $2 is the date.
const cycleSnapshot = `(SELECT jsonb_build_object('date', $2::text, 'scope', count(*),
		'completed', count(*) FILTER (WHERE s.type = 'completed'),
		'started', count(*) FILTER (WHERE s.type = 'started'),
		'scopeEstimate', coalesce(sum(i.estimate), 0),
		'completedEstimate', coalesce(sum(i.estimate) FILTER (WHERE s.type = 'completed'), 0))
	 FROM issues i JOIN workflow_states s ON s.id = i.state_id WHERE i.cycle_id = c.id AND i.trashed_at IS NULL)`

// RunCycleAutomations creates upcoming cycles, completes ended ones (rolling unfinished issues
// over) and records each running cycle's daily progress. Runs every minute.
func RunCycleAutomations(ctx context.Context, state *app.State) error {
	return pgx.BeginFunc(ctx, state.DB, func(tx pgx.Tx) error {
		locked, err := app.One[bool](ctx, tx, "SELECT pg_try_advisory_xact_lock($1)", cycleAutomationLock)
		if err != nil || !locked {
			return err
		}
		now, err := app.One[time.Time](ctx, tx, "SELECT now()")
		if err != nil {
			return err
		}
		now = now.UTC()
		today := now.Format(time.DateOnly)
		teams, err := app.Rows[cycleSettings](ctx, tx,
			`SELECT id, cycle_duration_weeks, cycle_cooldown_weeks, cycle_start_day, upcoming_cycle_count, cycle_rollover
			 FROM teams WHERE cycles_enabled AND archived_at IS NULL FOR UPDATE`)
		if err != nil {
			return err
		}
		ids := []uuid.UUID{}
		for _, team := range teams {
			if err := createCycles(ctx, tx, team, now); err != nil {
				return err
			}
			if err := completeEnded(ctx, tx, team, today); err != nil {
				return err
			}
			ids = append(ids, team.ID)
		}
		return app.Exec(ctx, tx, fmt.Sprintf(
			`UPDATE cycles c SET history = c.history || jsonb_build_array(%s)
			 WHERE c.team_id = ANY($1) AND c.archived_at IS NULL AND c.completed_at IS NULL
			 AND c.starts_at <= now() AND c.ends_at > now()
			 AND NOT c.history @> jsonb_build_array(jsonb_build_object('date', $2::text))`, cycleSnapshot), ids, today)
	})
}

// firstStart is 00:00 UTC of the most recent startDay (0 = Sunday) on or before today.
func firstStart(today time.Time, startDay int) time.Time {
	back := ((int(today.Weekday())-startDay)%7 + 7) % 7
	return time.Date(today.Year(), today.Month(), today.Day()-back, 0, 0, 0, 0, time.UTC)
}

const cycleWeek = 7 * 24 * time.Hour

// createCycles adds cycles after the team's last one until there is a current one (unless it is
// cooling down) and as many upcoming ones as the team wants.
func createCycles(ctx context.Context, tx pgx.Tx, team cycleSettings, now time.Time) error {
	duration := time.Duration(max(team.CycleDurationWeeks, 1)) * cycleWeek
	step := duration + time.Duration(max(team.CycleCooldownWeeks, 0))*cycleWeek
	type last struct {
		End      *time.Time
		Number   int
		Upcoming int64
	}
	row, _, err := app.Row[last](ctx, tx,
		`SELECT max(ends_at), coalesce(max(number), 0), count(*) FILTER (WHERE starts_at > $2 AND archived_at IS NULL)
		 FROM cycles WHERE team_id = $1`, team.ID, now)
	if err != nil {
		return err
	}
	number, upcoming := row.Number, row.Upcoming
	var start time.Time
	if row.End != nil {
		start = row.End.Add(step - duration)
	} else {
		start = firstStart(now, team.CycleStartDay)
	}
	for !start.Add(duration).After(now) {
		start = start.Add(step)
	}
	for {
		future := start.After(now)
		if future && upcoming >= int64(max(team.UpcomingCycleCount, 0)) {
			return nil
		}
		number++
		if err := app.Exec(ctx, tx, "INSERT INTO cycles (team_id, number, starts_at, ends_at) VALUES ($1, $2, $3, $4)",
			team.ID, number, start, start.Add(duration)); err != nil {
			return err
		}
		if future {
			upcoming++
		}
		start = start.Add(step)
	}
}

// completeEnded completes the team's ended cycles: unfinished issues move to the next cycle (or
// out of any cycle when the team doesn't roll over), with a last snapshot of the counts first.
func completeEnded(ctx context.Context, tx pgx.Tx, team cycleSettings, today string) error {
	type ended struct {
		ID   uuid.UUID
		Ends time.Time
	}
	cycles, err := app.Rows[ended](ctx, tx,
		`SELECT id, ends_at FROM cycles WHERE team_id = $1 AND completed_at IS NULL AND archived_at IS NULL
		 AND ends_at <= now() ORDER BY ends_at`, team.ID)
	if err != nil {
		return err
	}
	for _, cycle := range cycles {
		var next *uuid.UUID
		if team.CycleRollover {
			id, found, err := app.Maybe[uuid.UUID](ctx, tx,
				`SELECT id FROM cycles WHERE team_id = $1 AND archived_at IS NULL AND starts_at >= $2
				 AND ends_at > now() ORDER BY starts_at LIMIT 1`, team.ID, cycle.Ends)
			if err != nil {
				return err
			}
			if found {
				next = &id
			}
		}
		if err := app.Exec(ctx, tx, fmt.Sprintf(
			`UPDATE cycles c SET completed_at = now(), updated_at = now(), history =
				(SELECT coalesce(jsonb_agg(e ORDER BY n), '[]') FROM jsonb_array_elements(c.history) WITH ORDINALITY h(e, n)
				 WHERE e->>'date' IS DISTINCT FROM $2::text) || jsonb_build_array(%s)
			 WHERE c.id = $1`, cycleSnapshot), cycle.ID, today); err != nil {
			return err
		}
		if err := app.Exec(ctx, tx,
			`WITH moved AS (
				UPDATE issues i SET cycle_id = $2, updated_at = now() FROM workflow_states s
				WHERE s.id = i.state_id AND i.cycle_id = $1 AND i.trashed_at IS NULL
				AND s.type NOT IN ('completed', 'canceled') RETURNING i.id)
			 INSERT INTO issue_history (issue_id, action, changes)
			 SELECT id, 'cycle_rollover', jsonb_build_object('cycle', jsonb_build_object('from', cycle_ref($1), 'to', cycle_ref($2)))
			 FROM moved`, cycle.ID, next); err != nil {
			return err
		}
	}
	return nil
}

/* Shapes */

type Cycle struct {
	ID          string          `json:"id"`
	Number      int             `json:"number"`
	Name        *string         `json:"name"`
	Description *string         `json:"description"`
	URL         string          `json:"url"`
	StartsAt    string          `json:"startsAt"`
	EndsAt      string          `json:"endsAt"`
	CompletedAt *string         `json:"completedAt"`
	ArchivedAt  *string         `json:"archivedAt"`
	IsActive    bool            `json:"isActive"`
	IsFuture    bool            `json:"isFuture"`
	IsPast      bool            `json:"isPast"`
	Team        TeamRef         `json:"team"`
	Progress    float64         `json:"progress" doc:"The share of its issues completed, 0 to 1."`
	Scope       CycleScope      `json:"scope"`
	History     []CycleSnapshot `json:"history" doc:"The counts of each day the cycle ran."`
	CreatedAt   string          `json:"createdAt"`
	UpdatedAt   string          `json:"updatedAt"`
}

type CycleScope struct {
	Issues            int     `json:"issues"`
	Completed         int     `json:"completed"`
	Started           int     `json:"started"`
	Unstarted         int     `json:"unstarted"`
	Canceled          int     `json:"canceled"`
	Estimate          float64 `json:"estimate"`
	CompletedEstimate float64 `json:"completedEstimate"`
	StartedEstimate   float64 `json:"startedEstimate"`
}

type CycleSnapshot struct {
	Date              string  `json:"date"`
	Scope             int     `json:"scope"`
	Completed         int     `json:"completed"`
	Started           int     `json:"started"`
	ScopeEstimate     float64 `json:"scopeEstimate"`
	CompletedEstimate float64 `json:"completedEstimate"`
}

type CycleDetail struct {
	Cycle
	Issues []CycleIssue `json:"issues,omitempty" doc:"Unless includeIssues is false."`
}

type CycleIssue struct {
	ID         string   `json:"id"`
	Identifier string   `json:"identifier"`
	Title      string   `json:"title"`
	State      StateRef `json:"state"`
	Assignee   *UserRef `json:"assignee"`
	Estimate   *float64 `json:"estimate"`
	Priority   int      `json:"priority"`
}
