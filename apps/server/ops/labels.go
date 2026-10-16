package ops

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
)

// Issue labels: workspace labels and team labels, label groups, retiring and deleting them.

func labelOps() []*Op {
	return []*Op{
		op("list_issue_labels", app.Read, []string{"GET /labels"},
			"List issue labels. With team: that team's labels and the workspace labels (the ones its issues can use); "+
				"without: workspace labels and the labels of every team you can see. Groups (isGroup) hold child labels; an "+
				"issue takes one child of a singleSelect group. Retired labels are left out unless includeRetired is set.",
			listLabels).returns(ResultOf[PageOf[IssueLabel]]()),
		op("get_issue_label", app.Read, []string{"GET /labels/{id}"},
			"Read an issue label by id or name (Group/Label for a label in a group; pass team to pick a team's label).",
			getLabel).returns(ResultOf[IssueLabel]()),
		op("save_issue_label", app.Write, []string{"POST /labels", "PATCH /labels/{id}"},
			"Create an issue label (omit id; name is required; with team it's that team's label, without it a workspace "+
				"label) or change one (id = id or name). Set isGroup to make a group that holds labels, and parent to put a "+
				"label in a group (null takes it out). Names are unique within their team (or the workspace) and group. "+
				"Omitted fields stay as they are; null clears a field.",
			saveLabel).returns(ResultOf[IssueLabel]()),
		op("retire_issue_label", app.Write, []string{"POST /labels/{id}/retire"},
			"Retire a label (and the labels of a group): it stays on the issues that have it but can't be added anymore. "+
				"restore_issue_label brings it back.",
			retireLabel).returns(ResultOf[IssueLabel]()),
		op("restore_issue_label", app.Write, []string{"POST /labels/{id}/restore"},
			"Bring back a retired label (and the labels of a group).", restoreLabel).returns(ResultOf[IssueLabel]()),
		op("delete_issue_label", app.Write, []string{"DELETE /labels/{id}"},
			"Delete a label for good and take it off every issue; deleting a group deletes its labels too. Prefer "+
				"retire_issue_label to keep it on existing issues.",
			deleteLabel).returns(ResultOf[DeletedIssueLabel]()),
	}
}

const labelJSON = `jsonb_build_object(
	'id', l.id,
	'name', l.name,
	'color', l.color,
	'description', l.description,
	'isGroup', l.is_group,
	'groupType', CASE WHEN l.is_group THEN l.group_type END,
	'parent', CASE WHEN p.id IS NOT NULL THEN jsonb_build_object('id', p.id, 'name', p.name) END,
	'team', team_ref(l.team_id),
	'children', CASE WHEN l.is_group THEN (SELECT coalesce(jsonb_agg(label_ref(c.id) ORDER BY lower(c.name)), '[]')
		FROM issue_labels c WHERE c.parent_id = l.id) END,
	'issueCount', (SELECT count(DISTINCT ll.issue_id) FROM issue_label_links ll JOIN issues i ON i.id = ll.issue_id
		WHERE i.trashed_at IS NULL AND (ll.label_id = l.id
			OR (l.is_group AND ll.label_id IN (SELECT c.id FROM issue_labels c WHERE c.parent_id = l.id)))),
	'creator', user_ref(l.creator_id),
	'retiredAt', l.retired_at,
	'createdAt', l.created_at,
	'updatedAt', l.updated_at)`

const labelFrom = "FROM issue_labels l LEFT JOIN issue_labels p ON p.id = l.parent_id"

func labelValue(c *app.Ctx, q app.Querier, label uuid.UUID) (M, error) {
	value, found, err := app.Maybe[M](c, q, fmt.Sprintf("SELECT %s %s WHERE l.id = $1", labelJSON, labelFrom), label)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, app.NotFound("No such label")
	}
	return value, nil
}

// findLabel is a label the caller can see (retired ones too) by id, name or Group/Name. With a
// team, its labels and the workspace labels are searched and the team's label wins a tie.
func findLabel(c *app.Ctx, value string, team *uuid.UUID) (uuid.UUID, error) {
	var teams []uuid.UUID
	if team != nil {
		teams = []uuid.UUID{*team}
	} else {
		var err error
		if teams, err = c.TeamIDs(); err != nil {
			return uuid.Nil, err
		}
	}
	type row struct {
		ID     uuid.UUID
		Name   string
		Parent *string
		Team   *uuid.UUID
	}
	rows, err := app.Rows[row](c, c.DB(),
		`SELECT l.id, l.name, p.name, l.team_id FROM issue_labels l LEFT JOIN issue_labels p ON p.id = l.parent_id
		 WHERE l.workspace_id = $1 AND (l.team_id IS NULL OR l.team_id = ANY($2)) ORDER BY lower(l.name)`,
		c.Workspace, teams)
	if err != nil {
		return uuid.Nil, err
	}
	value = strings.TrimSpace(value)
	if id, ok := asUUID(value); ok {
		for _, r := range rows {
			if r.ID == id {
				return r.ID, nil
			}
		}
	}
	group, name := splitGroup(value)
	var matches, teamMatches []row
	for _, r := range rows {
		if !strings.EqualFold(r.Name, name) || group != nil && (r.Parent == nil || !strings.EqualFold(*r.Parent, *group)) {
			continue
		}
		matches = append(matches, r)
		if team != nil && r.Team != nil && *r.Team == *team {
			teamMatches = append(teamMatches, r)
		}
	}
	switch {
	case len(matches) == 1:
		return matches[0].ID, nil
	case len(teamMatches) == 1:
		return teamMatches[0].ID, nil
	case len(matches) == 0:
		var names []string
		for _, r := range rows[:min(len(rows), 30)] {
			if r.Parent != nil {
				names = append(names, *r.Parent+"/"+r.Name)
			} else {
				names = append(names, r.Name)
			}
		}
		return uuid.Nil, app.BadRequest("No label %q. Options: %s.", value, strings.Join(names, ", "))
	}
	return uuid.Nil, app.BadRequest("%q matches several labels; pass team, name it as Group/%s, or use its id", value, name)
}

func labelTeam(c *app.Ctx, q app.Querier, label uuid.UUID) (*uuid.UUID, error) {
	return app.One[*uuid.UUID](c, q, "SELECT team_id FROM issue_labels WHERE id = $1", label)
}

func teamArg(c *app.Ctx, team *string) (*uuid.UUID, error) {
	if team == nil {
		return nil, nil
	}
	id, err := resolveTeam(c, *team)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

/* Read */

type ListLabelsArgs struct {
	Team           *string `json:"team" doc:"Team key, name or id: its labels and the workspace labels."`
	IncludeRetired bool    `json:"includeRetired,omitempty" doc:"Also list retired labels."`
	Query          *string `json:"query" doc:"Part of a label's name."`
	Page
}

func listLabels(c *app.Ctx, args ListLabelsArgs) (any, error) {
	team, err := teamArg(c, args.Team)
	if err != nil {
		return nil, err
	}
	var teams []uuid.UUID
	if team != nil {
		teams = []uuid.UUID{*team}
	} else if teams, err = c.TeamIDs(); err != nil {
		return nil, err
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	sql := fmt.Sprintf(`SELECT %s %s WHERE l.workspace_id = $1
		 AND (l.team_id IS NULL OR l.team_id IN (SELECT id FROM teams WHERE id = ANY($2) AND archived_at IS NULL))
		 AND ($3 OR l.retired_at IS NULL) AND ($4::text IS NULL OR l.name ILIKE $4 OR p.name ILIKE $4)
		 ORDER BY lower(coalesce(p.name, l.name)), l.parent_id IS NOT NULL, lower(l.name), l.team_id NULLS FIRST
		 LIMIT $5 OFFSET $6`, labelJSON, labelFrom)
	nodes, err := app.All[any](c, c.DB(), sql, c.Workspace, teams, args.IncludeRetired, likePattern(args.Query),
		args.limit()+1, offset)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

type LabelArgs struct {
	ID   string  `json:"id" doc:"The label's id or name (Group/Label for a label in a group)."`
	Team *string `json:"team" doc:"Team key, name or id, to pick between labels of the same name."`
}

// labelOf is the label the arguments name.
func labelOf(c *app.Ctx, args LabelArgs) (uuid.UUID, error) {
	team, err := teamArg(c, args.Team)
	if err != nil {
		return uuid.Nil, err
	}
	return findLabel(c, args.ID, team)
}

func getLabel(c *app.Ctx, args LabelArgs) (any, error) {
	label, err := labelOf(c, args)
	if err != nil {
		return nil, err
	}
	return labelValue(c, c.DB(), label)
}

/* Save */

type SaveLabelArgs struct {
	ID          *string          `json:"id" doc:"Only to change a label: its id or name (Group/Label for a label in a group). Omit to create one."`
	Team        *string          `json:"team" doc:"Team key, name or id. On create, makes it a team label (omit for a workspace label); on update, picks between labels of the same name."`
	Name        *string          `json:"name" doc:"Required to create."`
	Color       *string          `json:"color" doc:"Hex color like #eb5757; picked for you on create when omitted."`
	Description Nullable[string] `json:"description" doc:"Null clears it."`
	IsGroup     *bool            `json:"isGroup" doc:"A group holds labels and can't be put on issues itself."`
	GroupType   *string          `json:"groupType" doc:"For groups: singleSelect (an issue takes one of its labels, the default) or multiSelect."`
	Parent      Nullable[string] `json:"parent" doc:"The group (name or id) to put the label in; null takes it out of its group. A team label's group is a workspace group or a group of the same team."`
}

type labelRecord struct {
	Team      *uuid.UUID
	Parent    *uuid.UUID
	Name      string
	IsGroup   bool
	GroupType string
}

func saveLabel(c *app.Ctx, args SaveLabelArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	team, err := teamArg(c, args.Team)
	if err != nil {
		return nil, err
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		var label uuid.UUID
		var before M
		var err error
		if args.ID != nil {
			if label, err = findLabel(c, *args.ID, team); err != nil {
				return err
			}
			if before, err = labelValue(c, tx, label); err != nil {
				return err
			}
		} else if label, err = createLabel(c, tx, team, args); err != nil {
			return err
		}
		if err := applyLabel(c, tx, label, args); err != nil {
			return err
		}
		if value, err = labelValue(c, tx, label); err != nil {
			return err
		}
		labelTeamID, err := labelTeam(c, tx, label)
		if err != nil {
			return err
		}
		if before == nil {
			return emit(c, tx, "IssueLabel", "create", value, nil, labelTeamID)
		}
		updatedFrom := changedFields(before, value)
		if len(updatedFrom) == 0 {
			return nil
		}
		if err := app.Exec(c, tx, "UPDATE issue_labels SET updated_at = now() WHERE id = $1", label); err != nil {
			return err
		}
		return emit(c, tx, "IssueLabel", "update", value, updatedFrom, labelTeamID)
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func createLabel(c *app.Ctx, tx pgx.Tx, team *uuid.UUID, args SaveLabelArgs) (uuid.UUID, error) {
	name, err := nonEmpty(orEmpty(args.Name), "name")
	if err != nil {
		return uuid.Nil, err
	}
	color := pickColor(name)
	if args.Color != nil {
		color = *args.Color
	}
	if color, err = checkColor(color); err != nil {
		return uuid.Nil, err
	}
	return app.One[uuid.UUID](c, tx,
		`INSERT INTO issue_labels (workspace_id, team_id, name, color, creator_id) VALUES ($1, $2, $3, $4, $5)
		 RETURNING id`, c.Workspace, team, name, color, c.User)
}

func applyLabel(c *app.Ctx, tx pgx.Tx, label uuid.UUID, args SaveLabelArgs) error {
	current, _, err := app.Row[labelRecord](c, tx,
		"SELECT team_id, parent_id, name, is_group, group_type FROM issue_labels WHERE id = $1 FOR UPDATE", label)
	if err != nil {
		return err
	}
	name := current.Name
	if args.Name != nil {
		if name, err = nonEmpty(*args.Name, "name"); err != nil {
			return err
		}
	}
	parent := current.Parent
	if args.Parent.Set {
		parent = nil
		if args.Parent.Valid {
			id, err := checkLabelParent(c, tx, label, current.Team, args.Parent.Value)
			if err != nil {
				return err
			}
			parent = &id
		}
	}
	isGroup := current.IsGroup
	if args.IsGroup != nil {
		isGroup = *args.IsGroup
	}
	if isGroup && !current.IsGroup {
		used, err := app.One[bool](c, tx, "SELECT EXISTS (SELECT 1 FROM issue_label_links WHERE label_id = $1)", label)
		if err != nil {
			return err
		}
		if used {
			return app.BadRequest("%s is on issues, and a group can't be; create a new group instead", name)
		}
	}
	if !isGroup && current.IsGroup {
		hasChildren, err := app.One[bool](c, tx, "SELECT EXISTS (SELECT 1 FROM issue_labels WHERE parent_id = $1)", label)
		if err != nil {
			return err
		}
		if hasChildren {
			return app.BadRequest("%s still holds labels; move or delete them first", name)
		}
	}
	groupType := current.GroupType
	if args.GroupType != nil {
		if !isGroup {
			return app.BadRequest("groupType only applies to groups (isGroup: true)")
		}
		if *args.GroupType != "singleSelect" && *args.GroupType != "multiSelect" {
			return app.BadRequest("groupType is singleSelect or multiSelect")
		}
		groupType = *args.GroupType
	}
	taken, err := app.One[bool](c, tx,
		`SELECT EXISTS (SELECT 1 FROM issue_labels WHERE workspace_id = $1 AND team_id IS NOT DISTINCT FROM $2
		 AND parent_id IS NOT DISTINCT FROM $3 AND lower(name) = lower($4) AND id <> $5)`,
		c.Workspace, current.Team, parent, name, label)
	if err != nil {
		return err
	}
	if taken {
		place := "the workspace"
		if parent != nil {
			place = "this group"
		} else if current.Team != nil {
			place = "this team"
		}
		return app.BadRequest("There already is a label named %q in %s", name, place)
	}
	if err := app.Exec(c, tx,
		"UPDATE issue_labels SET name = $2, parent_id = $3, is_group = $4, group_type = $5 WHERE id = $1",
		label, name, parent, isGroup, groupType); err != nil {
		return err
	}
	if args.Color != nil {
		color, err := checkColor(*args.Color)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "UPDATE issue_labels SET color = $2 WHERE id = $1", label, color); err != nil {
			return err
		}
	}
	if args.Description.Set {
		if err := app.Exec(c, tx, "UPDATE issue_labels SET description = $2 WHERE id = $1",
			label, trimmed(args.Description.Ptr())); err != nil {
			return err
		}
	}
	if isGroup {
		if err := checkSingleSelect(c, tx, label); err != nil {
			return err
		}
	}
	if parent != nil {
		return checkSingleSelect(c, tx, *parent)
	}
	return nil
}

// checkLabelParent is the group reference names, if label may go in it: same team or a
// workspace group, and not one of its own descendants.
func checkLabelParent(c *app.Ctx, tx pgx.Tx, label uuid.UUID, team *uuid.UUID, reference string) (uuid.UUID, error) {
	parent, err := findLabel(c, reference, team)
	if err != nil {
		return uuid.Nil, err
	}
	group, _, err := app.Row[struct {
		IsGroup bool
		Team    *uuid.UUID
		Name    string
	}](c, tx, "SELECT is_group, team_id, name FROM issue_labels WHERE id = $1", parent)
	if err != nil {
		return uuid.Nil, err
	}
	if !group.IsGroup {
		return uuid.Nil, app.BadRequest("%s isn't a group; make it one with isGroup: true", group.Name)
	}
	if group.Team != nil && (team == nil || *group.Team != *team) {
		return uuid.Nil, app.BadRequest("A label's group must be a workspace group or a group of the same team")
	}
	loops, err := app.One[bool](c, tx,
		`WITH RECURSIVE up AS (SELECT id, parent_id FROM issue_labels WHERE id = $1
		 UNION SELECT l.id, l.parent_id FROM issue_labels l JOIN up ON l.id = up.parent_id)
		 SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)`, parent, label)
	if err != nil {
		return uuid.Nil, err
	}
	if loops {
		return uuid.Nil, app.BadRequest("A label can't be in its own group or in a group inside it")
	}
	return parent, nil
}

// checkSingleSelect checks that issues have at most one label of a single-select group.
func checkSingleSelect(c *app.Ctx, tx pgx.Tx, group uuid.UUID) error {
	clash, found, err := app.Maybe[string](c, tx,
		`SELECT g.name FROM issue_labels g JOIN issue_labels c ON c.parent_id = g.id
		 JOIN issue_label_links ll ON ll.label_id = c.id
		 WHERE g.id = $1 AND g.group_type = 'singleSelect' GROUP BY g.name, ll.issue_id HAVING count(*) > 1 LIMIT 1`, group)
	if err != nil {
		return err
	}
	if found {
		return app.BadRequest("Some issues would have several labels of the single-select group %s; make it multiSelect or remove them first", clash)
	}
	return nil
}

/* Retire, restore, delete */

func setRetired(c *app.Ctx, args LabelArgs, retired bool) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	label, err := labelOf(c, args)
	if err != nil {
		return nil, err
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		before, err := labelValue(c, tx, label)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx,
			`UPDATE issue_labels SET retired_at = CASE WHEN $2 THEN coalesce(retired_at, now()) END, updated_at = now()
			 WHERE id = $1 OR parent_id = $1`, label, retired); err != nil {
			return err
		}
		if value, err = labelValue(c, tx, label); err != nil {
			return err
		}
		team, err := labelTeam(c, tx, label)
		if err != nil {
			return err
		}
		return emit(c, tx, "IssueLabel", "update", value, changedFields(before, value), team)
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func retireLabel(c *app.Ctx, args LabelArgs) (any, error) {
	return setRetired(c, args, true)
}

func restoreLabel(c *app.Ctx, args LabelArgs) (any, error) {
	return setRetired(c, args, false)
}

func deleteLabel(c *app.Ctx, args LabelArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	label, err := labelOf(c, args)
	if err != nil {
		return nil, err
	}
	var issues int64
	err = inTx(c, func(tx pgx.Tx) error {
		value, err := labelValue(c, tx, label)
		if err != nil {
			return err
		}
		team, err := labelTeam(c, tx, label)
		if err != nil {
			return err
		}
		if issues, err = app.One[int64](c, tx,
			`SELECT count(DISTINCT issue_id) FROM issue_label_links
			 WHERE label_id IN (SELECT id FROM issue_labels WHERE id = $1 OR parent_id = $1)`, label); err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM issue_labels WHERE id = $1", label); err != nil {
			return err
		}
		return emit(c, tx, "IssueLabel", "remove", value, nil, team)
	})
	if err != nil {
		return nil, err
	}
	return M{"deleted": label, "removedFromIssues": issues}, nil
}

/* Shapes */

type IssueLabel struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Color       string            `json:"color"`
	Description *string           `json:"description"`
	IsGroup     bool              `json:"isGroup"`
	GroupType   *string           `json:"groupType" enum:"singleSelect,multiSelect" doc:"For groups."`
	Parent      *IssueLabelParent `json:"parent" doc:"The group it is in."`
	Team        *TeamRef          `json:"team" doc:"Null for a workspace label."`
	Children    *[]LabelRef       `json:"children" doc:"For groups: the labels in it."`
	IssueCount  int               `json:"issueCount"`
	Creator     *UserRef          `json:"creator"`
	RetiredAt   *string           `json:"retiredAt"`
	CreatedAt   string            `json:"createdAt"`
	UpdatedAt   string            `json:"updatedAt"`
}

type IssueLabelParent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DeletedIssueLabel struct {
	Deleted           string `json:"deleted"`
	RemovedFromIssues int    `json:"removedFromIssues"`
}
