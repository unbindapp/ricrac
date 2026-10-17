package ops

import (
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
)

// Workflow states (issue statuses) of a team: listing, creating, changing and archiving them.

func stateOps() []*Op {
	return []*Op{
		op("list_issue_statuses", app.Read, []string{"GET /teams/{team}/statuses"},
			"List a team's workflow statuses in board order (triage, backlog, unstarted, started, completed, canceled, "+
				"then position): name, type, color, how many issues are in each and which is the default for new issues.",
			listStates).returns(ResultOf[PageOf[IssueStatus]]()),
		op("get_issue_status", app.Read, []string{"GET /statuses/{id}"},
			"Read a workflow status by id, or by name or type together with its team.", getState).returns(ResultOf[IssueStatus]()),
		op("save_issue_status", app.Write, []string{"POST /statuses", "PATCH /statuses/{id}"},
			"Create a workflow status (omit id; team, name and type are required) or change one (id, or name with team). "+
				"Types: triage, backlog, unstarted, started, completed, canceled. Names are unique within the team. The type "+
				"can only change while no issues are in the status. Omitted fields stay as they are; null clears the "+
				"description.",
			saveState).returns(ResultOf[IssueStatus]()),
		op("delete_issue_status", app.Write, []string{"DELETE /statuses/{id}"},
			"Archive a workflow status. If issues are in it, pass moveIssuesTo (a status of the same team) and they move "+
				"there. If it is the team's default or auto-close status, that setting moves to moveIssuesTo too (which then "+
				"must be backlog/unstarted, or canceled). A team keeps at least one open, one completed and one canceled "+
				"status; turn triage off with save_team rather than deleting the Triage status.",
			deleteState).returns(ResultOf[DeletedIssueStatus]()),
	}
}

var stateTypes = []string{"triage", "backlog", "unstarted", "started", "completed", "canceled"}

func defaultStateColor(kind string) string {
	switch kind {
	case "triage":
		return "#fc7840"
	case "backlog":
		return "#bec2c8"
	case "unstarted":
		return "#e2e2e2"
	case "started":
		return "#f2c94c"
	case "completed":
		return "#5e6ad2"
	}
	return "#95a2b3"
}

const stateJSON = `jsonb_build_object(
	'id', s.id,
	'name', s.name,
	'type', s.type,
	'color', s.color,
	'description', s.description,
	'position', s.position,
	'team', team_ref(s.team_id),
	'issueCount', (SELECT count(*) FROM issues i WHERE i.state_id = s.id AND i.trashed_at IS NULL),
	'isDefault', s.id IS NOT DISTINCT FROM t.default_issue_state_id,
	'isAutoCloseState', s.id IS NOT DISTINCT FROM t.auto_close_state_id,
	'archivedAt', s.archived_at,
	'createdAt', s.created_at,
	'updatedAt', s.updated_at)`

const stateFrom = "FROM workflow_states s JOIN teams t ON t.id = s.team_id"

func stateValue(c *app.Ctx, q app.Querier, state uuid.UUID) (M, error) {
	value, found, err := app.Maybe[M](c, q, fmt.Sprintf("SELECT %s %s WHERE s.id = $1", stateJSON, stateFrom), state)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, app.NotFound("No such status")
	}
	return value, nil
}

// findState is an active status of the team by id, name or type (the first of that type), read
// through q so that statuses created in the same transaction are found.
func findState(c *app.Ctx, q app.Querier, team uuid.UUID, value string) (uuid.UUID, error) {
	rows, err := app.Rows[typed](c, q, fmt.Sprintf(
		"SELECT s.id, s.name, s.type FROM workflow_states s WHERE s.team_id = $1 AND s.archived_at IS NULL ORDER BY %s",
		stateOrder), team)
	if err != nil {
		return uuid.Nil, err
	}
	value = strings.TrimSpace(value)
	if id, found := findTyped(rows, value); found {
		return id, nil
	}
	return uuid.Nil, app.BadRequest("The team has no status %q. Options: %s.", value, strings.Join(typedNames(rows), ", "))
}

// findStatus is a status the caller can see, by id, or by name or type with its team, and its
// team. Archived statuses are found by id.
func findStatus(c *app.Ctx, value string, team *string) (uuid.UUID, uuid.UUID, error) {
	if id, ok := asUUID(value); ok {
		teamOf, found, err := app.Maybe[uuid.UUID](c, c.DB(),
			"SELECT s.team_id FROM workflow_states s JOIN teams t ON t.id = s.team_id WHERE s.id = $1 AND t.workspace_id = $2",
			id, c.Workspace)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		if !found {
			return uuid.Nil, uuid.Nil, app.NotFound("No status %s", id)
		}
		visible, err := c.CanSeeTeam(teamOf)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		if !visible {
			return uuid.Nil, uuid.Nil, app.NotFound("No status %s", id)
		}
		return id, teamOf, nil
	}
	if team == nil {
		return uuid.Nil, uuid.Nil, app.BadRequest("Name the team too, or pass the status's id")
	}
	teamID, err := resolveTeam(c, *team)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	state, err := findState(c, c.DB(), teamID, value)
	return state, teamID, err
}

func checkStateType(kind string) (string, error) {
	kind = strings.TrimSpace(kind)
	if !slices.Contains(stateTypes, kind) {
		return "", app.BadRequest("%q isn't a status type; use triage, backlog, unstarted, started, completed or canceled", kind)
	}
	return kind, nil
}

// checkWorkflow checks that a team keeps one open, one completed and one canceled status, and
// that its default and auto-close statuses fit their purpose.
func checkWorkflow(c *app.Ctx, tx pgx.Tx, team uuid.UUID) error {
	type counts struct {
		Open, Completed, Canceled int64
		DefaultType               *string
		AutoCloseType             *string
	}
	r, _, err := app.Row[counts](c, tx,
		`SELECT count(*) FILTER (WHERE s.type NOT IN ('completed', 'canceled')),
			count(*) FILTER (WHERE s.type = 'completed'), count(*) FILTER (WHERE s.type = 'canceled'),
			(SELECT d.type FROM workflow_states d JOIN teams t ON t.default_issue_state_id = d.id WHERE t.id = $1),
			(SELECT c.type FROM workflow_states c JOIN teams t ON t.auto_close_state_id = c.id WHERE t.id = $1)
		 FROM workflow_states s WHERE s.team_id = $1 AND s.archived_at IS NULL`, team)
	if err != nil {
		return err
	}
	switch {
	case r.Open == 0:
		return app.BadRequest("A team needs at least one status that isn't completed or canceled")
	case r.Completed == 0:
		return app.BadRequest("A team needs at least one completed status")
	case r.Canceled == 0:
		return app.BadRequest("A team needs at least one canceled status")
	case r.DefaultType != nil && *r.DefaultType != "backlog" && *r.DefaultType != "unstarted":
		return app.BadRequest("The team's default status must stay a backlog or unstarted status")
	case r.AutoCloseType != nil && *r.AutoCloseType != "canceled":
		return app.BadRequest("The team's auto-close status must stay a canceled status")
	}
	return nil
}

/* Read */

type ListStatesArgs struct {
	Team            string `json:"team" doc:"Team key, name or id."`
	IncludeArchived bool   `json:"includeArchived,omitempty" doc:"Also list archived statuses."`
	Page
}

func listStates(c *app.Ctx, args ListStatesArgs) (any, error) {
	team, err := resolveTeam(c, args.Team)
	if err != nil {
		return nil, err
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	sql := fmt.Sprintf(`SELECT %s %s WHERE s.team_id = $1 AND ($2 OR s.archived_at IS NULL)
		 ORDER BY s.archived_at IS NOT NULL, %s LIMIT $3 OFFSET $4`, stateJSON, stateFrom, stateOrder)
	nodes, err := app.All[any](c, c.DB(), sql, team, args.IncludeArchived, args.limit()+1, offset)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

type GetStateArgs struct {
	ID   string  `json:"id" doc:"The status's id, or its name or type together with team."`
	Team *string `json:"team" doc:"Team key, name or id, when id is a name or type."`
}

func getState(c *app.Ctx, args GetStateArgs) (any, error) {
	state, _, err := findStatus(c, args.ID, args.Team)
	if err != nil {
		return nil, err
	}
	return stateValue(c, c.DB(), state)
}

/* Save */

type SaveStateArgs struct {
	ID          *string          `json:"id" doc:"Only to change a status: its id, or its name with team. Omit to create one."`
	Team        *string          `json:"team" doc:"Team key, name or id. Required to create."`
	Name        *string          `json:"name" doc:"Required to create; unique within the team."`
	Type        *string          `json:"type" doc:"triage, backlog, unstarted, started, completed or canceled. Required to create; changes only while no issues are in the status."`
	Color       *string          `json:"color" doc:"Hex color like #f2c94c; defaults to the type's color."`
	Description Nullable[string] `json:"description" doc:"Null clears it."`
	Position    *float64         `json:"position" doc:"Order within its type on the board; lower comes first. New statuses go last."`
}

func saveState(c *app.Ctx, args SaveStateArgs) (any, error) {
	var value M
	err := inTx(c, func(tx pgx.Tx) error {
		var state, team uuid.UUID
		var before M
		var err error
		if args.ID != nil {
			if state, team, err = findStatus(c, *args.ID, args.Team); err != nil {
				return err
			}
			if err := requireCanEdit(c, tx, team); err != nil {
				return err
			}
			if before, err = stateValue(c, tx, state); err != nil {
				return err
			}
			if err := updateState(c, tx, state, team, args); err != nil {
				return err
			}
		} else {
			if team, err = resolveTeam(c, orEmpty(args.Team)); err != nil {
				return app.BadRequest("team is required to create a status. %s", app.AsError(err).Message)
			}
			if err := requireCanEdit(c, tx, team); err != nil {
				return err
			}
			if state, err = createState(c, tx, team, args); err != nil {
				return err
			}
		}
		if err := checkWorkflow(c, tx, team); err != nil {
			return err
		}
		if value, err = stateValue(c, tx, state); err != nil {
			return err
		}
		if before == nil {
			return emit(c, tx, "WorkflowState", "create", value, nil, &team)
		}
		updatedFrom := changedFields(before, value)
		if len(updatedFrom) == 0 {
			return nil
		}
		return emit(c, tx, "WorkflowState", "update", value, updatedFrom, &team)
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func checkStateNameFree(c *app.Ctx, tx pgx.Tx, team uuid.UUID, state *uuid.UUID, name string) error {
	taken, err := app.One[bool](c, tx,
		`SELECT EXISTS (SELECT 1 FROM workflow_states WHERE team_id = $1 AND lower(name) = lower($2)
		 AND archived_at IS NULL AND id IS DISTINCT FROM $3)`, team, name, state)
	if err != nil {
		return err
	}
	if taken {
		return app.BadRequest("The team already has a status named %q", name)
	}
	return nil
}

func checkSingleTriage(c *app.Ctx, tx pgx.Tx, team uuid.UUID, state *uuid.UUID) error {
	exists, err := app.One[bool](c, tx,
		`SELECT EXISTS (SELECT 1 FROM workflow_states WHERE team_id = $1 AND type = 'triage' AND archived_at IS NULL
		 AND id IS DISTINCT FROM $2)`, team, state)
	if err != nil {
		return err
	}
	if exists {
		return app.BadRequest("The team already has a triage status; a team has one")
	}
	return nil
}

func createState(c *app.Ctx, tx pgx.Tx, team uuid.UUID, args SaveStateArgs) (uuid.UUID, error) {
	name, err := nonEmpty(orEmpty(args.Name), "name")
	if err != nil {
		return uuid.Nil, err
	}
	if args.Type == nil {
		return uuid.Nil, app.BadRequest("type is required: triage, backlog, unstarted, started, completed or canceled")
	}
	kind, err := checkStateType(*args.Type)
	if err != nil {
		return uuid.Nil, err
	}
	if err := checkStateNameFree(c, tx, team, nil, name); err != nil {
		return uuid.Nil, err
	}
	if kind == "triage" {
		if err := checkSingleTriage(c, tx, team, nil); err != nil {
			return uuid.Nil, err
		}
	}
	color := defaultStateColor(kind)
	if args.Color != nil {
		if color, err = checkColor(*args.Color); err != nil {
			return uuid.Nil, err
		}
	}
	var position float64
	if args.Position != nil {
		position = *args.Position
	} else if position, err = app.One[float64](c, tx,
		`SELECT coalesce(max(position) FILTER (WHERE type = $2), max(position), 0) + 1
		 FROM workflow_states WHERE team_id = $1 AND archived_at IS NULL`, team, kind); err != nil {
		return uuid.Nil, err
	}
	return app.One[uuid.UUID](c, tx,
		`INSERT INTO workflow_states (team_id, name, type, color, description, position)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`, team, name, kind, color, trimmed(args.Description.Ptr()), position)
}

func updateState(c *app.Ctx, tx pgx.Tx, state, team uuid.UUID, args SaveStateArgs) error {
	set := func(column string, value any) error {
		return app.Exec(c, tx, fmt.Sprintf("UPDATE workflow_states SET %s = $2 WHERE id = $1", column), state, value)
	}
	if args.Name != nil {
		name, err := nonEmpty(*args.Name, "name")
		if err != nil {
			return err
		}
		if err := checkStateNameFree(c, tx, team, &state, name); err != nil {
			return err
		}
		if err := set("name", name); err != nil {
			return err
		}
	}
	if args.Color != nil {
		color, err := checkColor(*args.Color)
		if err != nil {
			return err
		}
		if err := set("color", color); err != nil {
			return err
		}
	}
	if args.Description.Set {
		if err := set("description", trimmed(args.Description.Ptr())); err != nil {
			return err
		}
	}
	if args.Position != nil {
		if err := set("position", *args.Position); err != nil {
			return err
		}
	}
	if args.Type != nil {
		if err := changeStateType(c, tx, state, team, *args.Type); err != nil {
			return err
		}
	}
	return app.Exec(c, tx, "UPDATE workflow_states SET updated_at = now() WHERE id = $1", state)
}

func changeStateType(c *app.Ctx, tx pgx.Tx, state, team uuid.UUID, value string) error {
	kind, err := checkStateType(value)
	if err != nil {
		return err
	}
	current, _, err := app.Row[struct {
		Type   string
		Issues int64
	}](c, tx, "SELECT s.type, (SELECT count(*) FROM issues i WHERE i.state_id = s.id) FROM workflow_states s WHERE s.id = $1", state)
	if err != nil {
		return err
	}
	if kind == current.Type {
		return nil
	}
	if current.Issues != 0 {
		return app.BadRequest("%d issues are in this status, so its type can't change; move them or create a new status", current.Issues)
	}
	if kind == "triage" {
		if err := checkSingleTriage(c, tx, team, &state); err != nil {
			return err
		}
	}
	return app.Exec(c, tx, "UPDATE workflow_states SET type = $2 WHERE id = $1", state, kind)
}

/* Delete */

type DeleteStateArgs struct {
	ID           string  `json:"id" doc:"The status's id, or its name with team."`
	Team         *string `json:"team" doc:"Team key, name or id, when id is a name."`
	MoveIssuesTo *string `json:"moveIssuesTo" doc:"Status (name, type or id, same team) to move its issues to. Required when issues are in it or when it is the team's default or auto-close status."`
}

func deleteState(c *app.Ctx, args DeleteStateArgs) (any, error) {
	state, team, err := findStatus(c, args.ID, args.Team)
	if err != nil {
		return nil, err
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		if err := requireCanEdit(c, tx, team); err != nil {
			return err
		}
		type row struct {
			Name        string
			Type        string
			Archived    bool
			TriageOn    bool
			IsDefault   bool
			IsAutoClose bool
		}
		r, _, err := app.Row[row](c, tx,
			`SELECT s.name, s.type, s.archived_at IS NOT NULL, t.triage_enabled,
				s.id IS NOT DISTINCT FROM t.default_issue_state_id, s.id IS NOT DISTINCT FROM t.auto_close_state_id
			 FROM workflow_states s JOIN teams t ON t.id = s.team_id WHERE s.id = $1 FOR UPDATE`, state)
		if err != nil {
			return err
		}
		if r.Archived {
			return app.BadRequest("The status %s is already archived", r.Name)
		}
		if r.Type == "triage" && r.TriageOn {
			return app.BadRequest("Triage is on for this team; turn it off with save_team (triageEnabled: false) instead")
		}
		var replacement *uuid.UUID
		if args.MoveIssuesTo != nil {
			target, err := findState(c, tx, team, *args.MoveIssuesTo)
			if err != nil {
				return err
			}
			if target == state {
				return app.BadRequest("moveIssuesTo must be another status")
			}
			replacement = &target
		}
		waiting, err := app.All[uuid.UUID](c, tx, "SELECT id FROM issues WHERE state_id = $1 ORDER BY number", state)
		if err != nil {
			return err
		}
		need := func(why string) error {
			return app.BadRequest("%s; pass moveIssuesTo (a status of the same team)", why)
		}
		if len(waiting) > 0 && replacement == nil {
			return need(fmt.Sprintf("%d issues are in %s", len(waiting), r.Name))
		}
		for _, setting := range []struct {
			on     bool
			column string
			what   string
		}{
			{r.IsDefault, "default_issue_state_id", "default"},
			{r.IsAutoClose, "auto_close_state_id", "auto-close"},
		} {
			if !setting.on {
				continue
			}
			if replacement == nil {
				return need(fmt.Sprintf("%s is the team's %s status", r.Name, setting.what))
			}
			if err := setTeam(c, tx, team, setting.column, *replacement); err != nil {
				return err
			}
		}
		if err := app.Exec(c, tx,
			"UPDATE workflow_states SET archived_at = now(), updated_at = now() WHERE id = $1", state); err != nil {
			return err
		}
		if err := checkWorkflow(c, tx, team); err != nil {
			return err
		}
		if replacement != nil {
			for _, issue := range waiting {
				if err := setState(c, tx, issue, *replacement, "updated"); err != nil {
					return err
				}
			}
		}
		if value, err = stateValue(c, tx, state); err != nil {
			return err
		}
		if err := emit(c, tx, "WorkflowState", "remove", value, nil, &team); err != nil {
			return err
		}
		value["movedIssues"] = len(waiting)
		value["movedTo"] = nil
		if replacement != nil {
			value["movedTo"], err = app.One[any](c, tx, "SELECT state_ref($1)", *replacement)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

/* Shapes */

type IssueStatus struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Type             string  `json:"type" enum:"triage,backlog,unstarted,started,completed,canceled"`
	Color            string  `json:"color"`
	Description      *string `json:"description"`
	Position         float64 `json:"position" doc:"Order within its type on the board."`
	Team             TeamRef `json:"team"`
	IssueCount       int     `json:"issueCount"`
	IsDefault        bool    `json:"isDefault" doc:"New issues start in it."`
	IsAutoCloseState bool    `json:"isAutoCloseState" doc:"Auto-closed issues go to it."`
	ArchivedAt       *string `json:"archivedAt"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
}

type DeletedIssueStatus struct {
	IssueStatus
	MovedIssues int       `json:"movedIssues"`
	MovedTo     *StateRef `json:"movedTo"`
}
