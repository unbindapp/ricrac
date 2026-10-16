package ops

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
)

// Teams: their settings (estimates, cycles, triage, automations), members, sub-teams, archiving
// and the triage rotation.

func teamOps() []*Op {
	return []*Op{
		op("list_teams", app.Read, []string{"GET /teams"},
			"List the teams you can see (private teams only if you're in them) with key, name, whether you're a member, "+
				"member and open issue counts and whether cycles and triage are on. Narrow with query (name or key) or mine.",
			listTeams),
		op("get_team", app.Read, []string{"GET /teams/{id}"},
			"Read a team by key, name or id: all its settings (estimates, cycles, triage, auto-archive and auto-close), "+
				"its statuses in order, members (with owners), sub-teams, current cycle, default status and who is on triage.",
			getTeam),
		op("save_team", app.Write, []string{"POST /teams", "PATCH /teams/{id}"},
			"Create a team (omit id; name is required, key defaults to its initials) or change one (id = key, name or "+
				"id). Omitted fields stay as they are; null clears a nullable field. People are id, name, email or \"me\"; "+
				"statuses are name, type or id. Changing the key renames every issue (ENG-1 becomes NEW-1); old "+
				"identifiers keep working. Workspace admins, team owners and members of the team (or anyone, for a public "+
				"team) may change it; guests can't.",
			saveTeam),
		op("delete_team", app.Write, []string{"DELETE /teams/{id}"},
			"Archive a team (workspace admins and team owners). It disappears from lists and its key becomes free; its "+
				"issues stay readable.",
			deleteTeam),
		op("join_team", app.Write, []string{"POST /teams/{id}/join"},
			"Join a team yourself. Private teams can only be joined by workspace admins; others need a member to add them "+
				"(save_team addMembers).",
			joinTeam),
		op("leave_team", app.Write, []string{"POST /teams/{id}/leave"},
			"Leave a team. The last member of a private team can't leave it.", leaveTeam),
		op("get_triage_responsibility", app.Read, []string{"GET /teams/{id}/triage"},
			"Who handles the team's triage: the action (none, notify or assign), who is responsible now and, with a "+
				"rotation, the upcoming shifts (filter them with user).",
			getTriageResponsibility),
	}
}

/* Representation */

const teamFrom = "FROM teams t JOIN workspaces w ON w.id = t.workspace_id"

const stateOrder = "array_position(ARRAY['triage','backlog','unstarted','started','completed','canceled'], s.type), s.position"

// teamJSON is the JSON of team t (with its workspace w); me is the caller's id and base the
// public URL, as SQL.
func teamJSON(me, base string) string {
	return fmt.Sprintf(`jsonb_build_object(
		'id', t.id,
		'key', t.key,
		'name', t.name,
		'description', t.description,
		'icon', t.icon,
		'color', t.color,
		'private', t.private,
		'parent', team_ref(t.parent_id),
		'url', %[2]s || '/' || w.url_key || '/team/' || t.key,
		'isMember', EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id = t.id AND tm.user_id = %[1]s),
		'memberCount', (SELECT count(*) FROM team_members tm WHERE tm.team_id = t.id),
		'issueCount', (SELECT count(*) FROM issues i JOIN workflow_states s ON s.id = i.state_id
			WHERE i.team_id = t.id AND i.trashed_at IS NULL AND i.archived_at IS NULL
			AND s.type NOT IN ('completed', 'canceled')),
		'cyclesEnabled', t.cycles_enabled,
		'triageEnabled', t.triage_enabled,
		'timezone', t.timezone,
		'archivedAt', t.archived_at,
		'createdAt', t.created_at,
		'updatedAt', t.updated_at)`, me, base)
}

const teamSettingsJSON = `jsonb_build_object(
	'estimateType', t.estimate_type,
	'estimateAllowZero', t.estimate_allow_zero,
	'estimateExtended', t.estimate_extended,
	'defaultEstimate', t.default_estimate,
	'cycleDurationWeeks', t.cycle_duration_weeks,
	'cycleCooldownWeeks', t.cycle_cooldown_weeks,
	'cycleStartDay', t.cycle_start_day,
	'upcomingCycleCount', t.upcoming_cycle_count,
	'cycleAutoAddStarted', t.cycle_auto_add_started,
	'cycleAutoAddCompleted', t.cycle_auto_add_completed,
	'cycleRollover', t.cycle_rollover,
	'triageAction', t.triage_action,
	'triageResponsible', (SELECT coalesce(jsonb_agg(user_ref(r.id) ORDER BY r.n), '[]')
		FROM unnest(t.triage_responsible_ids) WITH ORDINALITY r(id, n)),
	'triageRotationWeeks', t.triage_rotation_weeks,
	'triageRotationStart', t.triage_rotation_start,
	'requirePriorityToLeaveTriage', t.require_priority_to_leave_triage,
	'autoArchiveMonths', t.auto_archive_months,
	'autoCloseMonths', t.auto_close_months,
	'autoCloseState', state_ref(t.auto_close_state_id),
	'autoCloseParentIssues', t.auto_close_parent_issues,
	'autoCloseChildIssues', t.auto_close_child_issues,
	'defaultIssueState', state_ref(t.default_issue_state_id))`

// teamValue is everything about one team, as get_team answers.
func teamValue(c *app.Ctx, q app.Querier, team uuid.UUID) (M, error) {
	visible, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	sql := fmt.Sprintf(`SELECT %s || %s || jsonb_build_object(
			'states', (SELECT coalesce(jsonb_agg(jsonb_build_object('id', s.id, 'name', s.name, 'type', s.type,
				'color', s.color, 'description', s.description, 'position', s.position) ORDER BY %s), '[]')
				FROM workflow_states s WHERE s.team_id = t.id AND s.archived_at IS NULL),
			'members', (SELECT coalesce(jsonb_agg(user_ref(tm.user_id) || jsonb_build_object('owner', tm.owner)
				ORDER BY lower(u.name)), '[]')
				FROM team_members tm JOIN users u ON u.id = tm.user_id WHERE tm.team_id = t.id),
			'labelCount', (SELECT count(*) FROM issue_labels l WHERE l.team_id = t.id AND l.retired_at IS NULL
				AND NOT l.is_group),
			'subTeams', (SELECT coalesce(jsonb_agg(team_ref(c.id) ORDER BY c.name), '[]') FROM teams c
				WHERE c.parent_id = t.id AND c.archived_at IS NULL AND (c.id = ANY($4) OR NOT c.private)),
			'currentCycle', (SELECT cycle_ref(c.id) FROM cycles c WHERE c.team_id = t.id AND c.archived_at IS NULL
				AND c.starts_at <= now() AND c.ends_at > now() ORDER BY c.starts_at DESC LIMIT 1))
		 %s WHERE t.id = $1`, teamJSON("$2", "$3"), teamSettingsJSON, stateOrder, teamFrom)
	value, found, err := app.Maybe[M](c, q, sql, team, c.User, c.PublicURL(), visible)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, app.NotFound("No such team")
	}
	kind, ok := value["estimateType"].(string)
	if !ok {
		kind = "notUsed"
	}
	allowZero, _ := value["estimateAllowZero"].(bool)
	extended, _ := value["estimateExtended"].(bool)
	value["allowedEstimates"] = allowedEstimates(kind, allowZero, extended)
	action, responsible, err := triageResponsible(c, q, team)
	if err != nil {
		return nil, err
	}
	people, err := userRefs(c, q, responsible)
	if err != nil {
		return nil, err
	}
	value["triage"] = M{"enabled": value["triageEnabled"], "action": action, "responsible": people}
	return value, nil
}

func userRefs(c *app.Ctx, q app.Querier, ids []uuid.UUID) (any, error) {
	return app.One[any](c, q,
		"SELECT coalesce(jsonb_agg(user_ref(r.id) ORDER BY r.n), '[]') FROM unnest($1::uuid[]) WITH ORDINALITY r(id, n)", ids)
}

/* Read */

type ListTeamsArgs struct {
	Query           *string `json:"query" doc:"Part of a team's name or key."`
	IncludeArchived bool    `json:"includeArchived,omitempty" doc:"Also list archived teams."`
	Mine            bool    `json:"mine,omitempty" doc:"Only teams you are a member of."`
	Page
}

// likePattern is %query% for ILIKE, or nil when the query is empty.
func likePattern(query *string) *string {
	trimmed := strings.TrimSpace(orEmpty(query))
	if trimmed == "" {
		return nil
	}
	return new("%" + trimmed + "%")
}

func listTeams(c *app.Ctx, args ListTeamsArgs) (any, error) {
	visible, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	sql := fmt.Sprintf(`SELECT %s %s WHERE t.id = ANY($1) AND ($4 OR t.archived_at IS NULL)
		 AND (NOT $5 OR EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id = t.id AND tm.user_id = $2))
		 AND ($6::text IS NULL OR t.name ILIKE $6 OR t.key ILIKE $6)
		 ORDER BY lower(t.name) LIMIT $7 OFFSET $8`, teamJSON("$2", "$3"), teamFrom)
	nodes, err := app.All[any](c, c.DB(), sql, visible, c.User, c.PublicURL(), args.IncludeArchived, args.Mine,
		likePattern(args.Query), args.limit()+1, offset)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

func getTeam(c *app.Ctx, args IDArgs) (any, error) {
	team, err := resolveTeam(c, args.ID)
	if err != nil {
		return nil, err
	}
	return teamValue(c, c.DB(), team)
}

/* Permissions */

type membership struct {
	Private bool
	Member  bool
	Owner   bool
}

func loadMembership(c *app.Ctx, q app.Querier, team, user uuid.UUID) (membership, error) {
	m, _, err := app.Row[membership](c, q,
		`SELECT t.private, tm.user_id IS NOT NULL, coalesce(tm.owner, false)
		 FROM teams t LEFT JOIN team_members tm ON tm.team_id = t.id AND tm.user_id = $2 WHERE t.id = $1`, team, user)
	return m, err
}

// requireCanEdit lets workspace admins, the team's owners and members, and for a public team any
// member of the workspace change a team, its statuses and its settings. Guests may not.
func requireCanEdit(c *app.Ctx, q app.Querier, team uuid.UUID) error {
	if err := c.RequireMember(); err != nil {
		return err
	}
	if c.Role >= app.Admin {
		return nil
	}
	m, err := loadMembership(c, q, team, c.User)
	if err != nil {
		return err
	}
	if m.Owner || m.Member || !m.Private {
		return nil
	}
	return app.Forbidden("Only members of this private team can change it")
}

// findTeam is a team by id, key or name; admins also find private teams they aren't in.
func findTeam(c *app.Ctx, value string) (uuid.UUID, error) {
	visible, err := resolveTeam(c, value)
	if err == nil || c.Role < app.Admin {
		return visible, err
	}
	found, ok, findErr := app.Maybe[uuid.UUID](c, c.DB(),
		`SELECT id FROM teams WHERE workspace_id = $1 AND archived_at IS NULL
		 AND (id::text = $2 OR upper(key) = upper($2) OR lower(name) = lower($2))
		 ORDER BY id::text = $2 DESC, upper(key) = upper($2) DESC LIMIT 1`, c.Workspace, strings.TrimSpace(value))
	if findErr != nil {
		return uuid.Nil, findErr
	}
	if !ok {
		return visible, err
	}
	return found, nil
}

/* Save */

type SaveTeamArgs struct {
	ID                           *string           `json:"id" doc:"Only to change a team: its key, name or id. Omit to create one."`
	Name                         *string           `json:"name" doc:"Required to create."`
	Key                          *string           `json:"key" doc:"1 to 7 letters and digits starting with a letter, like ENG. Defaults to the name's initials."`
	Description                  Nullable[string]  `json:"description" doc:"Null clears it."`
	Icon                         Nullable[string]  `json:"icon" doc:"An emoji or icon name; null clears it."`
	Color                        Nullable[string]  `json:"color" doc:"Hex color like #5e6ad2; null clears it."`
	Private                      *bool             `json:"private" doc:"Private teams and their issues are only visible to their members."`
	Parent                       Nullable[string]  `json:"parent" doc:"Parent team (key, name or id), making this a sub-team; null makes it a top-level team."`
	Timezone                     *string           `json:"timezone" doc:"IANA timezone like Europe/Berlin, used for cycles."`
	EstimateType                 *string           `json:"estimateType" doc:"notUsed, exponential, fibonacci, linear or tShirt."`
	EstimateAllowZero            *bool             `json:"estimateAllowZero" doc:"Allow an estimate of 0."`
	EstimateExtended             *bool             `json:"estimateExtended" doc:"Allow the larger estimates of the scale."`
	DefaultEstimate              Nullable[float64] `json:"defaultEstimate" doc:"The estimate new issues get; null for none."`
	CyclesEnabled                *bool             `json:"cyclesEnabled"`
	CycleDurationWeeks           *int              `json:"cycleDurationWeeks" doc:"1 to 8."`
	CycleCooldownWeeks           *int              `json:"cycleCooldownWeeks" doc:"Weeks off between cycles, 0 to 4."`
	CycleStartDay                *int              `json:"cycleStartDay" doc:"Day cycles start: 0 Sunday, 1 Monday ... 6 Saturday."`
	UpcomingCycleCount           *int              `json:"upcomingCycleCount" doc:"How many upcoming cycles to create ahead, 1 to 8."`
	CycleAutoAddStarted          *bool             `json:"cycleAutoAddStarted" doc:"Put issues into the current cycle when they're started."`
	CycleAutoAddCompleted        *bool             `json:"cycleAutoAddCompleted" doc:"Put issues into the current cycle when they're completed."`
	CycleRollover                *bool             `json:"cycleRollover" doc:"Move unfinished issues to the next cycle when one ends."`
	TriageEnabled                *bool             `json:"triageEnabled" doc:"Route issues from other teams and integrations to a Triage status first. Turning it off moves issues in triage to the default status."`
	TriageAction                 *string           `json:"triageAction" doc:"What happens to new triage issues: none, notify (the responsible people) or assign (to the person on duty)."`
	TriageResponsible            []string          `json:"triageResponsible" doc:"People responsible for triage (replaces the list): id, name, email or \"me\"."`
	TriageRotationWeeks          Nullable[int]     `json:"triageRotationWeeks" doc:"With a number of weeks, the responsible people take turns, in order, for that many weeks each; null for no rotation."`
	TriageRotationStart          Nullable[string]  `json:"triageRotationStart" doc:"YYYY-MM-DD the rotation starts (the first person's first shift); defaults to today."`
	RequirePriorityToLeaveTriage *bool             `json:"requirePriorityToLeaveTriage" doc:"Issues need a priority before they can leave triage."`
	AutoArchiveMonths            Nullable[int]     `json:"autoArchiveMonths" doc:"Archive closed issues after 1, 3, 6, 9 or 12 months; null turns it off."`
	AutoCloseMonths              Nullable[int]     `json:"autoCloseMonths" doc:"Close issues not updated for 1, 3, 6, 9 or 12 months; null turns it off."`
	AutoCloseState               Nullable[string]  `json:"autoCloseState" doc:"The canceled-type status auto-closed issues go to (name or id); null for the first canceled status."`
	AutoCloseParentIssues        *bool             `json:"autoCloseParentIssues" doc:"Complete a parent issue when all its sub-issues are done."`
	AutoCloseChildIssues         *bool             `json:"autoCloseChildIssues" doc:"Close the sub-issues when their parent is closed."`
	DefaultIssueState            *string           `json:"defaultIssueState" doc:"The status new issues start in (a backlog or unstarted status: name or id)."`
	AddMembers                   []string          `json:"addMembers" doc:"People to add to the team: id, name, email or \"me\"."`
	RemoveMembers                []string          `json:"removeMembers" doc:"People to take off the team."`
}

func saveTeam(c *app.Ctx, args SaveTeamArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	var value M
	err := inTx(c, func(tx pgx.Tx) error {
		var team uuid.UUID
		var before M
		var err error
		if args.ID != nil {
			if team, err = resolveTeam(c, *args.ID); err != nil {
				return err
			}
			if err := requireCanEdit(c, tx, team); err != nil {
				return err
			}
			if err := app.Exec(c, tx, "SELECT id FROM teams WHERE id = $1 FOR UPDATE", team); err != nil {
				return err
			}
			if before, err = teamValue(c, tx, team); err != nil {
				return err
			}
		} else if team, err = createTeam(c, tx, args); err != nil {
			return err
		}
		if err := applyTeam(c, tx, team, args, before == nil); err != nil {
			return err
		}
		if value, err = teamValue(c, tx, team); err != nil {
			return err
		}
		if before == nil {
			return emit(c, tx, "Team", "create", value, nil, &team)
		}
		updatedFrom := changedFields(before, value)
		if len(updatedFrom) == 0 {
			return nil
		}
		if err := app.Exec(c, tx, "UPDATE teams SET updated_at = now() WHERE id = $1", team); err != nil {
			return err
		}
		return emit(c, tx, "Team", "update", value, updatedFrom, &team)
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func createTeam(c *app.Ctx, tx pgx.Tx, args SaveTeamArgs) (uuid.UUID, error) {
	name, err := nonEmpty(orEmpty(args.Name), "name")
	if err != nil {
		return uuid.Nil, err
	}
	key := TeamKey(name)
	if args.Key != nil {
		key = *args.Key
	}
	if key, err = CheckTeamKey(key); err != nil {
		return uuid.Nil, err
	}
	if err := checkTeamNameFree(c, tx, nil, name); err != nil {
		return uuid.Nil, err
	}
	if err := checkTeamKeyFree(c, tx, nil, key); err != nil {
		return uuid.Nil, err
	}
	return CreateTeam(c, tx, c.Workspace, c.User, name, key)
}

func checkTeamKeyFree(c *app.Ctx, tx pgx.Tx, team *uuid.UUID, key string) error {
	taken, found, err := app.Maybe[string](c, tx,
		`SELECT name FROM teams WHERE workspace_id = $1 AND upper(key) = $2 AND archived_at IS NULL
		 AND id IS DISTINCT FROM $3`, c.Workspace, key, team)
	if err != nil {
		return err
	}
	if found {
		return app.Conflict("The key %s is taken by the team %s; choose another key", key, taken)
	}
	return nil
}

func checkTeamNameFree(c *app.Ctx, tx pgx.Tx, team *uuid.UUID, name string) error {
	taken, err := app.One[bool](c, tx,
		`SELECT EXISTS (SELECT 1 FROM teams WHERE workspace_id = $1 AND lower(name) = lower($2)
		 AND archived_at IS NULL AND id IS DISTINCT FROM $3)`, c.Workspace, name, team)
	if err != nil {
		return err
	}
	if taken {
		return app.BadRequest("There already is a team named %q", name)
	}
	return nil
}

// setTeam sets one column of the team.
func setTeam(c *app.Ctx, tx pgx.Tx, team uuid.UUID, column string, value any) error {
	return app.Exec(c, tx, fmt.Sprintf("UPDATE teams SET %s = $2 WHERE id = $1", column), team, value)
}

func inRange(value, low, high int, what string) (int, error) {
	if value < low || value > high {
		return 0, app.BadRequest("%s is %d to %d", what, low, high)
	}
	return value, nil
}

func checkMonths(value *int, what string) (*int, error) {
	if value != nil && !slices.Contains([]int{1, 3, 6, 9, 12}, *value) {
		return nil, app.BadRequest("%s is 1, 3, 6, 9 or 12 (or null for off)", what)
	}
	return value, nil
}

// trimmed is the trimmed text, or nil when it is null or empty.
func trimmed(value *string) *string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return new(strings.TrimSpace(*value))
}

func applyTeam(c *app.Ctx, tx pgx.Tx, team uuid.UUID, args SaveTeamArgs, creating bool) error {
	if !creating {
		if args.Name != nil {
			name, err := nonEmpty(*args.Name, "name")
			if err != nil {
				return err
			}
			if err := checkTeamNameFree(c, tx, &team, name); err != nil {
				return err
			}
			if err := setTeam(c, tx, team, "name", name); err != nil {
				return err
			}
		}
		if args.Key != nil {
			key, err := CheckTeamKey(*args.Key)
			if err != nil {
				return err
			}
			if err := changeKey(c, tx, team, key); err != nil {
				return err
			}
		}
	}
	for _, field := range []struct {
		column string
		value  Nullable[string]
	}{{"description", args.Description}, {"icon", args.Icon}} {
		if !field.value.Set {
			continue
		}
		if err := setTeam(c, tx, team, field.column, trimmed(field.value.Ptr())); err != nil {
			return err
		}
	}
	if args.Color.Set {
		var color *string
		if text := trimmed(args.Color.Ptr()); text != nil {
			checked, err := checkColor(*text)
			if err != nil {
				return err
			}
			color = &checked
		}
		if err := setTeam(c, tx, team, "color", color); err != nil {
			return err
		}
	}
	if args.Private != nil {
		if err := setTeam(c, tx, team, "private", *args.Private); err != nil {
			return err
		}
	}
	if args.Parent.Set {
		if err := setParent(c, tx, team, args.Parent.Ptr()); err != nil {
			return err
		}
	}
	if args.Timezone != nil {
		timezone := strings.TrimSpace(*args.Timezone)
		if err := checkTimezone(c, tx, timezone); err != nil {
			return err
		}
		if err := setTeam(c, tx, team, "timezone", timezone); err != nil {
			return err
		}
	}
	if args.EstimateType != nil {
		if !slices.Contains([]string{"notUsed", "exponential", "fibonacci", "linear", "tShirt"}, *args.EstimateType) {
			return app.BadRequest("estimateType is notUsed, exponential, fibonacci, linear or tShirt")
		}
		if err := setTeam(c, tx, team, "estimate_type", *args.EstimateType); err != nil {
			return err
		}
	}
	for _, flag := range []struct {
		column string
		value  *bool
	}{
		{"estimate_allow_zero", args.EstimateAllowZero},
		{"estimate_extended", args.EstimateExtended},
		{"cycles_enabled", args.CyclesEnabled},
		{"cycle_auto_add_started", args.CycleAutoAddStarted},
		{"cycle_auto_add_completed", args.CycleAutoAddCompleted},
		{"cycle_rollover", args.CycleRollover},
		{"require_priority_to_leave_triage", args.RequirePriorityToLeaveTriage},
		{"auto_close_parent_issues", args.AutoCloseParentIssues},
		{"auto_close_child_issues", args.AutoCloseChildIssues},
	} {
		if flag.value == nil {
			continue
		}
		if err := setTeam(c, tx, team, flag.column, *flag.value); err != nil {
			return err
		}
	}
	if args.DefaultEstimate.Set {
		if err := setTeam(c, tx, team, "default_estimate", args.DefaultEstimate.Ptr()); err != nil {
			return err
		}
	}
	for _, number := range []struct {
		column    string
		value     *int
		low, high int
		what      string
	}{
		{"cycle_duration_weeks", args.CycleDurationWeeks, 1, 8, "cycleDurationWeeks"},
		{"cycle_cooldown_weeks", args.CycleCooldownWeeks, 0, 4, "cycleCooldownWeeks"},
		{"cycle_start_day", args.CycleStartDay, 0, 6, "cycleStartDay"},
		{"upcoming_cycle_count", args.UpcomingCycleCount, 1, 8, "upcomingCycleCount"},
	} {
		if number.value == nil {
			continue
		}
		value, err := inRange(*number.value, number.low, number.high, number.what)
		if err != nil {
			return err
		}
		if err := setTeam(c, tx, team, number.column, value); err != nil {
			return err
		}
	}
	for _, months := range []struct {
		column string
		value  Nullable[int]
		what   string
	}{
		{"auto_archive_months", args.AutoArchiveMonths, "autoArchiveMonths"},
		{"auto_close_months", args.AutoCloseMonths, "autoCloseMonths"},
	} {
		if !months.value.Set {
			continue
		}
		value, err := checkMonths(months.value.Ptr(), months.what)
		if err != nil {
			return err
		}
		if err := setTeam(c, tx, team, months.column, value); err != nil {
			return err
		}
	}
	if args.AutoCloseState.Set {
		if err := setAutoCloseState(c, tx, team, args.AutoCloseState.Ptr()); err != nil {
			return err
		}
	}
	if args.DefaultIssueState != nil {
		state, err := findState(c, tx, team, *args.DefaultIssueState)
		if err != nil {
			return err
		}
		kind, err := stateType(c, tx, state)
		if err != nil {
			return err
		}
		if kind != "backlog" && kind != "unstarted" {
			return app.BadRequest("defaultIssueState must be a backlog or unstarted status, not %s", kind)
		}
		if err := setTeam(c, tx, team, "default_issue_state_id", state); err != nil {
			return err
		}
	}
	if err := teamMembers(c, tx, team, args); err != nil {
		return err
	}
	if err := teamTriage(c, tx, team, args); err != nil {
		return err
	}
	return checkTeam(c, tx, team)
}

func setParent(c *app.Ctx, tx pgx.Tx, team uuid.UUID, reference *string) error {
	var parent *uuid.UUID
	if reference != nil {
		id, err := resolveTeam(c, *reference)
		if err != nil {
			return err
		}
		loops, err := app.One[bool](c, tx,
			`WITH RECURSIVE up AS (SELECT id, parent_id FROM teams WHERE id = $1
			 UNION SELECT t.id, t.parent_id FROM teams t JOIN up ON t.id = up.parent_id)
			 SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)`, id, team)
		if err != nil {
			return err
		}
		if loops {
			return app.BadRequest("A team can't be a sub-team of itself or of its own sub-teams")
		}
		parent = &id
	}
	return setTeam(c, tx, team, "parent_id", parent)
}

func setAutoCloseState(c *app.Ctx, tx pgx.Tx, team uuid.UUID, reference *string) error {
	var state *uuid.UUID
	if reference != nil {
		id, err := findState(c, tx, team, *reference)
		if err != nil {
			return err
		}
		state = &id
	} else {
		var err error
		if state, err = firstStateOfType(c, tx, team, "canceled"); err != nil {
			return err
		}
	}
	if state != nil {
		kind, err := stateType(c, tx, *state)
		if err != nil {
			return err
		}
		if kind != "canceled" {
			return app.BadRequest("autoCloseState must be a canceled-type status")
		}
	}
	return setTeam(c, tx, team, "auto_close_state_id", state)
}

// changeKey gives the team a new key. Every issue keeps its old identifier in
// previous_identifiers.
func changeKey(c *app.Ctx, tx pgx.Tx, team uuid.UUID, key string) error {
	old, err := app.One[string](c, tx, "SELECT key FROM teams WHERE id = $1", team)
	if err != nil {
		return err
	}
	if old == key {
		return nil
	}
	if err := checkTeamKeyFree(c, tx, &team, key); err != nil {
		return err
	}
	if err := app.Exec(c, tx,
		`UPDATE issues SET previous_identifiers = array_append(previous_identifiers, $2 || '-' || number)
		 WHERE team_id = $1 AND NOT ($2 || '-' || number) = ANY(previous_identifiers)`, team, old); err != nil {
		return err
	}
	return setTeam(c, tx, team, "key", key)
}

// checkCanSee checks that users can see the team: its members, and for a public team every
// member who isn't a guest.
func checkCanSee(c *app.Ctx, tx pgx.Tx, team uuid.UUID, users []uuid.UUID) error {
	blind, err := app.All[string](c, tx,
		`SELECT u.name FROM users u JOIN teams t ON t.id = $1
		 JOIN workspace_members m ON m.workspace_id = t.workspace_id AND m.user_id = u.id
		 WHERE u.id = ANY($2) AND NOT ((NOT t.private AND m.role <> 'guest')
			OR EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id = t.id AND tm.user_id = u.id))`, team, users)
	if err != nil {
		return err
	}
	if len(blind) > 0 {
		return app.BadRequest("%s can't see this team; add them to it first", strings.Join(blind, ", "))
	}
	return nil
}

func teamMembers(c *app.Ctx, tx pgx.Tx, team uuid.UUID, args SaveTeamArgs) error {
	if args.AddMembers != nil {
		users, err := resolveUsers(c, args.AddMembers)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx,
			"INSERT INTO team_members (team_id, user_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING", team, users); err != nil {
			return err
		}
	}
	if args.RemoveMembers == nil {
		return nil
	}
	users, err := resolveUsers(c, args.RemoveMembers)
	if err != nil {
		return err
	}
	if err := app.Exec(c, tx, "DELETE FROM team_members WHERE team_id = $1 AND user_id = ANY($2)", team, users); err != nil {
		return err
	}
	return dropFromTriage(c, tx, team, users)
}

func dropFromTriage(c *app.Ctx, tx pgx.Tx, team uuid.UUID, users []uuid.UUID) error {
	return app.Exec(c, tx,
		`UPDATE teams SET triage_responsible_ids = ARRAY(SELECT r FROM unnest(triage_responsible_ids) WITH ORDINALITY x(r, n)
			WHERE NOT r = ANY($2) ORDER BY n) WHERE id = $1`, team, users)
}

func teamTriage(c *app.Ctx, tx pgx.Tx, team uuid.UUID, args SaveTeamArgs) error {
	if args.TriageAction != nil {
		if !slices.Contains([]string{"none", "notify", "assign"}, *args.TriageAction) {
			return app.BadRequest("triageAction is none, notify or assign")
		}
		if err := setTeam(c, tx, team, "triage_action", *args.TriageAction); err != nil {
			return err
		}
	}
	if args.TriageResponsible != nil {
		users, err := resolveUsers(c, args.TriageResponsible)
		if err != nil {
			return err
		}
		if err := checkCanSee(c, tx, team, users); err != nil {
			return err
		}
		if err := setTeam(c, tx, team, "triage_responsible_ids", users); err != nil {
			return err
		}
	}
	if args.TriageRotationWeeks.Set {
		var weeks *int
		if args.TriageRotationWeeks.Valid {
			value, err := inRange(args.TriageRotationWeeks.Value, 1, 52, "triageRotationWeeks")
			if err != nil {
				return err
			}
			weeks = &value
		}
		if err := setTeam(c, tx, team, "triage_rotation_weeks", weeks); err != nil {
			return err
		}
	}
	if args.TriageRotationStart.Set {
		var start *time.Time
		if args.TriageRotationStart.Valid {
			date, err := parseDate(args.TriageRotationStart.Value)
			if err != nil {
				return err
			}
			start = &date
		}
		if err := setTeam(c, tx, team, "triage_rotation_start", start); err != nil {
			return err
		}
	}
	if err := app.Exec(c, tx,
		`UPDATE teams SET triage_rotation_start = (now() AT TIME ZONE 'UTC')::date
		 WHERE id = $1 AND triage_rotation_weeks IS NOT NULL AND triage_rotation_start IS NULL`, team); err != nil {
		return err
	}
	if args.TriageEnabled == nil {
		return nil
	}
	if *args.TriageEnabled {
		return enableTriage(c, tx, team)
	}
	return disableTriage(c, tx, team)
}

func enableTriage(c *app.Ctx, tx pgx.Tx, team uuid.UUID) error {
	if err := app.Exec(c, tx,
		`UPDATE workflow_states SET archived_at = NULL WHERE id = (SELECT id FROM workflow_states
			WHERE team_id = $1 AND type = 'triage' AND archived_at IS NOT NULL
			AND NOT EXISTS (SELECT 1 FROM workflow_states WHERE team_id = $1 AND type = 'triage' AND archived_at IS NULL)
			ORDER BY archived_at DESC LIMIT 1)`, team); err != nil {
		return err
	}
	if _, err := EnsureTriageState(c, tx, team); err != nil {
		return err
	}
	return setTeam(c, tx, team, "triage_enabled", true)
}

// disableTriage turns triage off and moves the issues waiting in it to the default status.
func disableTriage(c *app.Ctx, tx pgx.Tx, team uuid.UUID) error {
	if err := setTeam(c, tx, team, "triage_enabled", false); err != nil {
		return err
	}
	triage, err := app.All[uuid.UUID](c, tx,
		`UPDATE workflow_states SET archived_at = now() WHERE team_id = $1 AND type = 'triage'
		 AND archived_at IS NULL RETURNING id`, team)
	if err != nil {
		return err
	}
	waiting, err := app.All[uuid.UUID](c, tx, "SELECT id FROM issues WHERE state_id = ANY($1)", triage)
	if err != nil || len(waiting) == 0 {
		return err
	}
	target, err := defaultState(c, tx, team)
	if err != nil {
		return err
	}
	for _, issue := range waiting {
		if err := setState(c, tx, issue, target, "updated"); err != nil {
			return err
		}
	}
	return nil
}

// checkTeam checks what must hold after any change: private teams keep a member, the default
// estimate is on the scale.
func checkTeam(c *app.Ctx, tx pgx.Tx, team uuid.UUID) error {
	type row struct {
		Private  bool
		Members  int64
		Kind     string
		Zero     bool
		Extended bool
		Estimate *float64
	}
	r, _, err := app.Row[row](c, tx,
		`SELECT private, (SELECT count(*) FROM team_members WHERE team_id = $1), estimate_type,
			estimate_allow_zero, estimate_extended, default_estimate FROM teams WHERE id = $1`, team)
	if err != nil {
		return err
	}
	if r.Private && r.Members == 0 {
		return app.BadRequest("A private team needs at least one member")
	}
	if r.Estimate == nil || r.Kind == "notUsed" {
		return nil
	}
	allowed := allowedEstimates(r.Kind, r.Zero, r.Extended)
	if slices.Contains(allowed, *r.Estimate) {
		return nil
	}
	list := make([]string, len(allowed))
	for i, v := range allowed {
		list[i] = strconv.FormatFloat(v, 'f', -1, 64)
	}
	return app.BadRequest("defaultEstimate %s isn't on the %s scale; use one of %s",
		strconv.FormatFloat(*r.Estimate, 'f', -1, 64), r.Kind, strings.Join(list, ", "))
}

/* Archive, join and leave */

func deleteTeam(c *app.Ctx, args IDArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	team, err := resolveTeam(c, args.ID)
	if err != nil {
		return nil, err
	}
	var key string
	err = inTx(c, func(tx pgx.Tx) error {
		if c.Role < app.Admin {
			m, err := loadMembership(c, tx, team, c.User)
			if err != nil {
				return err
			}
			if !m.Owner {
				return app.Forbidden("Only workspace admins and the team's owners can delete a team")
			}
		}
		archived, _, err := app.Row[struct{ Key, Name string }](c, tx,
			"UPDATE teams SET archived_at = now(), updated_at = now() WHERE id = $1 RETURNING key, name", team)
		if err != nil {
			return err
		}
		key = archived.Key
		if err := app.Exec(c, tx, "UPDATE teams SET parent_id = NULL WHERE parent_id = $1", team); err != nil {
			return err
		}
		if err := audit(c, tx, "team.archived", M{"team": team, "key": archived.Key, "name": archived.Name}); err != nil {
			return err
		}
		value, err := teamValue(c, tx, team)
		if err != nil {
			return err
		}
		return emit(c, tx, "Team", "remove", value, nil, &team)
	})
	if err != nil {
		return nil, err
	}
	return M{"deleted": team, "key": key}, nil
}

func joinTeam(c *app.Ctx, args IDArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	team, err := findTeam(c, args.ID)
	if err != nil {
		return nil, err
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		m, err := loadMembership(c, tx, team, c.User)
		if err != nil {
			return err
		}
		if m.Private && !m.Member && c.Role < app.Admin {
			return app.Forbidden("Only workspace admins can join a private team; ask a member to add you")
		}
		if err := app.Exec(c, tx,
			"INSERT INTO team_members (team_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", team, c.User); err != nil {
			return err
		}
		value, err = teamValue(c, tx, team)
		return err
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func leaveTeam(c *app.Ctx, args IDArgs) (any, error) {
	team, err := resolveTeam(c, args.ID)
	if err != nil {
		return nil, err
	}
	var ref any
	err = inTx(c, func(tx pgx.Tx) error {
		if err := app.Exec(c, tx, "SELECT id FROM teams WHERE id = $1 FOR UPDATE", team); err != nil {
			return err
		}
		m, err := loadMembership(c, tx, team, c.User)
		if err != nil {
			return err
		}
		if !m.Member {
			return app.BadRequest("You aren't a member of this team")
		}
		if err := app.Exec(c, tx, "DELETE FROM team_members WHERE team_id = $1 AND user_id = $2", team, c.User); err != nil {
			return err
		}
		if err := dropFromTriage(c, tx, team, []uuid.UUID{c.User}); err != nil {
			return err
		}
		if err := checkTeam(c, tx, team); err != nil {
			return app.BadRequest("You're the last member of this private team; add someone else or delete the team")
		}
		ref, err = app.One[any](c, tx, "SELECT team_ref($1)", team)
		return err
	})
	if err != nil {
		return nil, err
	}
	return M{"left": ref}, nil
}

/* Triage */

type TriageArgs struct {
	ID    string  `json:"id" doc:"Team key, name or id."`
	Limit *int    `json:"limit" doc:"How many upcoming shifts to list, 10 by default, at most 100."`
	User  *string `json:"user" doc:"Only this person's shifts: id, name, email or \"me\"."`
}

type shift struct {
	person       uuid.UUID
	starts, ends time.Time
}

// shifts is the rotation's shifts from the current one on.
func shifts(people []uuid.UUID, weeks int, start, today time.Time, limit int, only *uuid.UUID) []shift {
	if len(people) == 0 || weeks < 1 || only != nil && !slices.Contains(people, *only) {
		return nil
	}
	week := 7 * 24 * time.Hour
	turn := max(int(today.Sub(start)/week), 0) / weeks
	var found []shift
	for len(found) < limit {
		person := people[turn%len(people)]
		starts := start.Add(time.Duration(turn*weeks) * week)
		if only == nil || *only == person {
			found = append(found, shift{person, starts, starts.Add(time.Duration(weeks) * week)})
		}
		turn++
	}
	return found
}

func getTriageResponsibility(c *app.Ctx, args TriageArgs) (any, error) {
	team, err := resolveTeam(c, args.ID)
	if err != nil {
		return nil, err
	}
	var only *uuid.UUID
	if args.User != nil {
		user, err := resolveUser(c, *args.User)
		if err != nil {
			return nil, err
		}
		only = &user
	}
	db := c.DB()
	type row struct {
		Team    any
		Enabled bool
		People  []uuid.UUID
		Weeks   *int
		Start   *time.Time
	}
	r, _, err := app.Row[row](c, db,
		`SELECT team_ref(id), triage_enabled, triage_responsible_ids, triage_rotation_weeks, triage_rotation_start
		 FROM teams WHERE id = $1`, team)
	if err != nil {
		return nil, err
	}
	action, current, err := triageResponsible(c, db, team)
	if err != nil {
		return nil, err
	}
	upcoming := []any{}
	if r.Weeks != nil && r.Start != nil {
		limit := 10
		if args.Limit != nil {
			limit = min(max(*args.Limit, 1), 100)
		}
		today := time.Now().UTC().Truncate(24 * time.Hour)
		for _, s := range shifts(r.People, *r.Weeks, *r.Start, today, limit, only) {
			user, err := app.One[any](c, db, "SELECT user_ref($1)", s.person)
			if err != nil {
				return nil, err
			}
			upcoming = append(upcoming, M{"user": user, "startsAt": s.starts.Format(time.DateOnly), "endsAt": s.ends.Format(time.DateOnly)})
		}
	}
	var rotation any
	if r.Weeks != nil {
		var start any
		if r.Start != nil {
			start = r.Start.Format(time.DateOnly)
		}
		rotation = M{"weeks": *r.Weeks, "start": start}
	}
	responsible, err := userRefs(c, db, current)
	if err != nil {
		return nil, err
	}
	everyone, err := userRefs(c, db, r.People)
	if err != nil {
		return nil, err
	}
	return M{
		"team":           r.Team,
		"triageEnabled":  r.Enabled,
		"action":         action,
		"responsible":    responsible,
		"everyone":       everyone,
		"rotation":       rotation,
		"upcomingShifts": upcoming,
	}, nil
}
