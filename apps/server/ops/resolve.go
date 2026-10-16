package ops

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"ricrac/app"
)

// Resolvers turn what agents pass (ids, names, keys, identifiers like ENG-123, "me", links) into
// ids, checking the caller may see the thing. Errors list what would have matched.

func asUUID(value string) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(value))
	return id, err == nil
}

// lastSegment is the last path segment of a link, or the value itself.
func lastSegment(value string) string {
	value = strings.TrimSpace(value)
	if !strings.Contains(value, "://") {
		return value
	}
	path := value
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	path = strings.TrimRight(path, "/")
	return path[strings.LastIndex(path, "/")+1:]
}

// slugTail is the id slugs end in: my-project-0a1b2c3d4e5f.
func slugTail(value string) string {
	segment := lastSegment(value)
	i := strings.LastIndex(segment, "-")
	if i < 0 {
		return segment
	}
	tail := segment[i+1:]
	if len(tail) == 12 && isHex(tail) {
		return tail
	}
	return segment
}

func isHex(value string) bool {
	for _, c := range value {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

func options(names []string) string {
	if len(names) == 0 {
		return "There are none."
	}
	shown := names[:min(len(names), 30)]
	more := ""
	if len(names) > len(shown) {
		more = fmt.Sprintf(" and %d more", len(names)-len(shown))
	}
	return fmt.Sprintf("Options: %s%s.", strings.Join(shown, ", "), more)
}

func appendUnique(ids []uuid.UUID, id uuid.UUID) []uuid.UUID {
	if slices.Contains(ids, id) {
		return ids
	}
	return append(ids, id)
}

// resolveAll resolves each value with one, without repeats.
func resolveAll(c *app.Ctx, values []string, one func(*app.Ctx, string) (uuid.UUID, error)) ([]uuid.UUID, error) {
	ids := []uuid.UUID{}
	for _, value := range values {
		id, err := one(c, value)
		if err != nil {
			return nil, err
		}
		ids = appendUnique(ids, id)
	}
	return ids, nil
}

type member struct {
	ID          uuid.UUID
	Name        string
	DisplayName string
	Email       string
}

// resolveUser is a member of the workspace by id, email, display name, name, the part of their
// email before the @, first name, or "me".
func resolveUser(c *app.Ctx, value string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "me") {
		return c.User, nil
	}
	rows, err := app.Rows[member](c, c.DB(),
		`SELECT u.id, u.name, u.display_name, u.email FROM users u
		 JOIN workspace_members m ON m.user_id = u.id AND m.workspace_id = $1 AND m.active`, c.Workspace)
	if err != nil {
		return uuid.Nil, err
	}
	needle := strings.ToLower(strings.TrimPrefix(value, "@"))
	id, isID := asUUID(value)
	tests := []func(member) bool{
		func(r member) bool { return isID && r.ID == id },
		func(r member) bool { return strings.ToLower(r.Email) == needle },
		func(r member) bool { return strings.ToLower(r.DisplayName) == needle },
		func(r member) bool { return strings.ToLower(r.Name) == needle },
		func(r member) bool { local, _, _ := strings.Cut(r.Email, "@"); return strings.ToLower(local) == needle },
		func(r member) bool {
			first := strings.Fields(strings.ToLower(r.Name))
			return len(first) > 0 && first[0] == needle
		},
	}
	describe := func(rows []member) []string {
		names := make([]string, len(rows))
		for i, r := range rows {
			names[i] = fmt.Sprintf("%s <%s>", r.Name, r.Email)
		}
		return names
	}
	for _, test := range tests {
		var matches []member
		for _, r := range rows {
			if test(r) {
				matches = append(matches, r)
			}
		}
		switch len(matches) {
		case 0:
			continue
		case 1:
			return matches[0].ID, nil
		default:
			return uuid.Nil, app.BadRequest("%q matches more than one person; use their email. %s", value, options(describe(matches)))
		}
	}
	return uuid.Nil, app.BadRequest("No member of this workspace matches %q. %s", value, options(describe(rows)))
}

func resolveUsers(c *app.Ctx, values []string) ([]uuid.UUID, error) {
	return resolveAll(c, values, resolveUser)
}

type keyed struct {
	ID   uuid.UUID
	Key  string
	Name string
}

// resolveTeam is a team the caller can see, by id, key or name.
func resolveTeam(c *app.Ctx, value string) (uuid.UUID, error) {
	visible, err := c.TeamIDs()
	if err != nil {
		return uuid.Nil, err
	}
	rows, err := app.Rows[keyed](c, c.DB(),
		"SELECT id, key, name FROM teams WHERE id = ANY($1) AND archived_at IS NULL ORDER BY name", visible)
	if err != nil {
		return uuid.Nil, err
	}
	value = strings.TrimSpace(value)
	id, isID := asUUID(value)
	for _, match := range []func(keyed) bool{
		func(r keyed) bool { return isID && r.ID == id },
		func(r keyed) bool { return strings.EqualFold(r.Key, value) },
		func(r keyed) bool { return strings.EqualFold(r.Name, value) },
	} {
		for _, r := range rows {
			if match(r) {
				return r.ID, nil
			}
		}
	}
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = fmt.Sprintf("%s (%s)", r.Key, r.Name)
	}
	return uuid.Nil, app.BadRequest("No team matches %q. %s", value, options(names))
}

type typed struct {
	ID   uuid.UUID
	Name string
	Type string
}

func findTyped(rows []typed, value string, extra ...func(typed) bool) (uuid.UUID, bool) {
	id, isID := asUUID(value)
	tests := append([]func(typed) bool{
		func(r typed) bool { return isID && r.ID == id },
		func(r typed) bool { return strings.EqualFold(r.Name, value) },
		func(r typed) bool { return strings.EqualFold(r.Type, value) },
	}, extra...)
	for _, test := range tests {
		for _, r := range rows {
			if test(r) {
				return r.ID, true
			}
		}
	}
	return uuid.Nil, false
}

func typedNames(rows []typed) []string {
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = fmt.Sprintf("%s (%s)", r.Name, r.Type)
	}
	return names
}

// resolveState is a workflow state of the team, by id, name or type (the first of that type).
func resolveState(c *app.Ctx, team uuid.UUID, value string) (uuid.UUID, error) {
	rows, err := app.Rows[typed](c, c.DB(),
		"SELECT id, name, type FROM workflow_states WHERE team_id = $1 AND archived_at IS NULL ORDER BY position", team)
	if err != nil {
		return uuid.Nil, err
	}
	value = strings.TrimSpace(value)
	id, found := findTyped(rows, value,
		func(r typed) bool { return strings.EqualFold(value, "todo") && r.Type == "unstarted" },
		func(r typed) bool { return strings.EqualFold(value, "done") && r.Type == "completed" },
	)
	if found {
		return id, nil
	}
	return uuid.Nil, app.BadRequest("The team has no status %q. %s", value, options(typedNames(rows)))
}

// parseIdentifier splits ENG-123 into the team key and number.
func parseIdentifier(value string) (string, int, bool) {
	value = strings.TrimSpace(value)
	i := strings.LastIndex(value, "-")
	if i < 0 {
		return "", 0, false
	}
	key, digits := value[:i], value[i+1:]
	number, err := strconv.Atoi(digits)
	if err != nil || digits == "" || strings.ContainsAny(digits, "+-") {
		return "", 0, false
	}
	if key == "" || len(key) > 10 {
		return "", 0, false
	}
	for _, c := range key {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
			return "", 0, false
		}
	}
	return strings.ToUpper(key), number, true
}

// resolveIssue is an issue the caller can see, by id, identifier (current or earlier) or link.
// Trashed issues are found too.
func resolveIssue(c *app.Ctx, value string) (uuid.UUID, error) {
	raw := strings.TrimSpace(value)
	candidate := raw
	if strings.Contains(raw, "://") {
		candidate = lastSegment(raw)
		for _, segment := range strings.Split(raw, "/") {
			if _, _, ok := parseIdentifier(segment); ok {
				candidate = segment
				break
			}
		}
	}
	notFound := app.NotFound("No issue %q; use an identifier like ENG-123 or an id", raw)
	var id uuid.UUID
	var found bool
	var err error
	if uid, ok := asUUID(candidate); ok {
		id, found, err = app.Maybe[uuid.UUID](c, c.DB(), "SELECT id FROM issues WHERE id = $1 AND workspace_id = $2", uid, c.Workspace)
	} else if key, number, ok := parseIdentifier(candidate); ok {
		id, found, err = app.Maybe[uuid.UUID](c, c.DB(),
			`SELECT i.id FROM issues i JOIN teams t ON t.id = i.team_id
			 WHERE i.workspace_id = $1 AND upper(t.key) = $2 AND i.number = $3
			 ORDER BY t.archived_at IS NOT NULL LIMIT 1`, c.Workspace, key, number)
		if err == nil && !found {
			id, found, err = app.Maybe[uuid.UUID](c, c.DB(),
				"SELECT id FROM issues WHERE workspace_id = $1 AND $2 = ANY(previous_identifiers)",
				c.Workspace, fmt.Sprintf("%s-%d", key, number))
		}
	}
	if err != nil {
		return uuid.Nil, err
	}
	if !found {
		return uuid.Nil, notFound
	}
	visible, err := canSeeIssue(c, id)
	if err != nil {
		return uuid.Nil, err
	}
	if !visible {
		return uuid.Nil, notFound
	}
	return id, nil
}

func canSeeIssue(c *app.Ctx, issue uuid.UUID) (bool, error) {
	teams, err := c.TeamIDs()
	if err != nil {
		return false, err
	}
	return app.One[bool](c, c.DB(),
		`SELECT EXISTS (SELECT 1 FROM issues i WHERE i.id = $1 AND (i.team_id = ANY($2)
			OR EXISTS (SELECT 1 FROM issue_shares s WHERE s.issue_id = i.id AND s.user_id = $3)))`,
		issue, teams, c.User)
}

func resolveIssues(c *app.Ctx, values []string) ([]uuid.UUID, error) {
	return resolveAll(c, values, resolveIssue)
}

type labelRow struct {
	ID      uuid.UUID
	Name    string
	Parent  *string
	TeamID  *uuid.UUID
	IsGroup bool
}

func splitGroup(value string) (group *string, name string) {
	g, n, found := strings.Cut(value, "/")
	if !found {
		return nil, value
	}
	g = strings.TrimSpace(g)
	return &g, strings.TrimSpace(n)
}

func labelMatches(id uuid.UUID, name string, parent *string, value string) bool {
	if uid, ok := asUUID(value); ok && uid == id {
		return true
	}
	group, wanted := splitGroup(value)
	if !strings.EqualFold(name, wanted) {
		return false
	}
	return group == nil || parent != nil && strings.EqualFold(*parent, *group)
}

// resolveLabels is issue labels by id or name, among the workspace's labels and the team's. A
// child of a group can be named Group/Child.
func resolveLabels(c *app.Ctx, team *uuid.UUID, values []string) ([]uuid.UUID, error) {
	ids := []uuid.UUID{}
	if len(values) == 0 {
		return ids, nil
	}
	rows, err := app.Rows[labelRow](c, c.DB(),
		`SELECT l.id, l.name, p.name, l.team_id, l.is_group FROM issue_labels l
		 LEFT JOIN issue_labels p ON p.id = l.parent_id
		 WHERE l.workspace_id = $1 AND l.retired_at IS NULL AND (l.team_id IS NULL OR l.team_id = $2)`,
		c.Workspace, team)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		var matches []labelRow
		for _, r := range rows {
			if !r.IsGroup && labelMatches(r.ID, r.Name, r.Parent, value) {
				matches = append(matches, r)
			}
		}
		var chosen labelRow
		switch len(matches) {
		case 0:
			var names []string
			for _, r := range rows {
				if r.IsGroup {
					continue
				}
				if r.Parent != nil {
					names = append(names, *r.Parent+"/"+r.Name)
				} else {
					names = append(names, r.Name)
				}
			}
			return nil, app.BadRequest("No label %q. %s", value, options(names))
		case 1:
			chosen = matches[0]
		default:
			// A team label wins over a workspace label of the same name.
			var teamLabels []labelRow
			for _, r := range matches {
				if r.TeamID != nil {
					teamLabels = append(teamLabels, r)
				}
			}
			if len(teamLabels) != 1 {
				_, name := splitGroup(value)
				return nil, app.BadRequest("%q matches labels in several groups; name it as Group/%s", value, name)
			}
			chosen = teamLabels[0]
		}
		ids = appendUnique(ids, chosen.ID)
	}
	return ids, nil
}

// projectVisible is SQL that is true when project alias is visible: it belongs to a team the
// caller sees. Bind the visible team ids as $param.
func projectVisible(alias string, param int) string {
	return fmt.Sprintf("EXISTS (SELECT 1 FROM project_teams pt WHERE pt.project_id = %s.id AND pt.team_id = ANY($%d))", alias, param)
}

type named struct {
	ID   uuid.UUID
	Name string
}

// resolveProject is a project by id, slug, link or name.
func resolveProject(c *app.Ctx, value string) (uuid.UUID, error) {
	teams, err := c.TeamIDs()
	if err != nil {
		return uuid.Nil, err
	}
	value = strings.TrimSpace(value)
	rows, err := app.Rows[named](c, c.DB(), fmt.Sprintf(
		`SELECT p.id, p.name FROM projects p WHERE p.workspace_id = $1 AND %s AND p.trashed_at IS NULL
		 ORDER BY p.archived_at IS NOT NULL, p.updated_at DESC`, projectVisible("p", 2)), c.Workspace, teams)
	if err != nil {
		return uuid.Nil, err
	}
	visible := func(id uuid.UUID) bool {
		return slices.ContainsFunc(rows, func(r named) bool { return r.ID == id })
	}
	if id, ok := asUUID(value); ok && visible(id) {
		return id, nil
	}
	bySlug, found, err := app.Maybe[uuid.UUID](c, c.DB(),
		"SELECT id FROM projects WHERE workspace_id = $1 AND slug_id = $2", c.Workspace, slugTail(value))
	if err != nil {
		return uuid.Nil, err
	}
	if found && visible(bySlug) {
		return bySlug, nil
	}
	for _, r := range rows {
		if strings.EqualFold(r.Name, value) {
			return r.ID, nil
		}
	}
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Name
	}
	return uuid.Nil, app.BadRequest("No project %q. %s", value, options(names))
}

func resolveProjects(c *app.Ctx, values []string) ([]uuid.UUID, error) {
	return resolveAll(c, values, resolveProject)
}

// resolveMilestone is a milestone by id, or by name within the project.
func resolveMilestone(c *app.Ctx, project *uuid.UUID, value string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if id, ok := asUUID(value); ok {
		projectOf, found, err := app.Maybe[uuid.UUID](c, c.DB(),
			`SELECT m.project_id FROM project_milestones m JOIN projects p ON p.id = m.project_id
			 WHERE m.id = $1 AND p.workspace_id = $2`, id, c.Workspace)
		if err != nil {
			return uuid.Nil, err
		}
		if found {
			if _, err := resolveProject(c, projectOf.String()); err != nil {
				return uuid.Nil, err
			}
			return id, nil
		}
	}
	if project == nil {
		return uuid.Nil, app.BadRequest("Name the project too, or pass the milestone's id")
	}
	rows, err := app.Rows[named](c, c.DB(),
		"SELECT id, name FROM project_milestones WHERE project_id = $1 ORDER BY sort_order", *project)
	if err != nil {
		return uuid.Nil, err
	}
	names := make([]string, len(rows))
	for i, r := range rows {
		if strings.EqualFold(r.Name, value) {
			return r.ID, nil
		}
		names[i] = r.Name
	}
	return uuid.Nil, app.BadRequest("The project has no milestone %q. %s", value, options(names))
}

// resolveCycle is a cycle of the team by id, number, name, or "current", "next" or "previous".
func resolveCycle(c *app.Ctx, team *uuid.UUID, value string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if id, ok := asUUID(value); ok {
		teamOf, found, err := app.Maybe[uuid.UUID](c, c.DB(), "SELECT team_id FROM cycles WHERE id = $1", id)
		if err != nil {
			return uuid.Nil, err
		}
		if found {
			visible, err := c.CanSeeTeam(teamOf)
			if err != nil {
				return uuid.Nil, err
			}
			if visible {
				return id, nil
			}
		}
		return uuid.Nil, app.NotFound("No cycle %s", value)
	}
	if team == nil {
		return uuid.Nil, app.BadRequest("Name the team too, or pass the cycle's id")
	}
	relative := map[string]string{
		"current":  "starts_at <= now() AND ends_at > now() ORDER BY starts_at DESC",
		"active":   "starts_at <= now() AND ends_at > now() ORDER BY starts_at DESC",
		"next":     "starts_at > now() ORDER BY starts_at",
		"upcoming": "starts_at > now() ORDER BY starts_at",
		"previous": "ends_at <= now() ORDER BY ends_at DESC",
		"last":     "ends_at <= now() ORDER BY ends_at DESC",
	}
	var id uuid.UUID
	var found bool
	var err error
	if clause, ok := relative[strings.ToLower(value)]; ok {
		id, found, err = app.Maybe[uuid.UUID](c, c.DB(),
			"SELECT id FROM cycles WHERE team_id = $1 AND archived_at IS NULL AND "+clause+" LIMIT 1", *team)
	} else if number, numErr := strconv.Atoi(strings.TrimSpace(strings.TrimLeft(value, "#Cc"))); numErr == nil {
		id, found, err = app.Maybe[uuid.UUID](c, c.DB(), "SELECT id FROM cycles WHERE team_id = $1 AND number = $2", *team, number)
	} else {
		id, found, err = app.Maybe[uuid.UUID](c, c.DB(),
			"SELECT id FROM cycles WHERE team_id = $1 AND lower(name) = lower($2) LIMIT 1", *team, value)
	}
	if err != nil {
		return uuid.Nil, err
	}
	if !found {
		return uuid.Nil, app.BadRequest("The team has no cycle %q; use its number, name, id, or current/next/previous", value)
	}
	return id, nil
}

type slugged struct {
	ID   uuid.UUID
	Name string
	Slug string
}

// resolveNamed is something with a workspace, a slug and a name or title, by id, slug, link or
// name.
func resolveNamed(c *app.Ctx, table, nameColumn, extra, what, value string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	rows, err := app.Rows[slugged](c, c.DB(), fmt.Sprintf(
		"SELECT id, %s, slug_id FROM %s WHERE workspace_id = $1 %s ORDER BY updated_at DESC", nameColumn, table, extra),
		c.Workspace)
	if err != nil {
		return uuid.Nil, err
	}
	id, isID := asUUID(value)
	slug := slugTail(value)
	for _, test := range []func(slugged) bool{
		func(r slugged) bool { return isID && r.ID == id },
		func(r slugged) bool { return r.Slug == slug },
		func(r slugged) bool { return strings.EqualFold(r.Name, value) },
	} {
		for _, r := range rows {
			if test(r) {
				return r.ID, nil
			}
		}
	}
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Name
	}
	return uuid.Nil, app.BadRequest("No %s %q. %s", what, value, options(names))
}

func resolveInitiative(c *app.Ctx, value string) (uuid.UUID, error) {
	return resolveNamed(c, "initiatives", "name", "AND trashed_at IS NULL", "initiative", value)
}

func resolveInitiatives(c *app.Ctx, values []string) ([]uuid.UUID, error) {
	return resolveAll(c, values, resolveInitiative)
}

// resolveDocument is a document by id, slug, link or title. Documents of projects, teams and
// issues the caller can't see are left out.
func resolveDocument(c *app.Ctx, value string) (uuid.UUID, error) {
	id, err := resolveNamed(c, "documents", "title", "AND trashed_at IS NULL", "document", value)
	if err != nil {
		return uuid.Nil, err
	}
	visible, err := canSeeDocument(c, id)
	if err != nil {
		return uuid.Nil, err
	}
	if !visible {
		return uuid.Nil, app.NotFound("No document %q", value)
	}
	return id, nil
}

func canSeeDocument(c *app.Ctx, id uuid.UUID) (bool, error) {
	type owners struct{ Team, Project, Issue, Cycle *uuid.UUID }
	row, found, err := app.Row[owners](c, c.DB(),
		"SELECT team_id, project_id, issue_id, cycle_id FROM documents WHERE id = $1", id)
	if err != nil || !found {
		return false, err
	}
	switch {
	case row.Team != nil:
		return c.CanSeeTeam(*row.Team)
	case row.Project != nil:
		_, err := resolveProject(c, row.Project.String())
		return err == nil, nil
	case row.Issue != nil:
		return canSeeIssue(c, *row.Issue)
	case row.Cycle != nil:
		_, err := resolveCycle(c, nil, row.Cycle.String())
		return err == nil, nil
	}
	return true, nil
}

func resolveProjectStatus(c *app.Ctx, value string) (uuid.UUID, error) {
	rows, err := app.Rows[typed](c, c.DB(),
		"SELECT id, name, type FROM project_statuses WHERE workspace_id = $1 AND archived_at IS NULL ORDER BY position",
		c.Workspace)
	if err != nil {
		return uuid.Nil, err
	}
	value = strings.TrimSpace(value)
	if id, found := findTyped(rows, value); found {
		return id, nil
	}
	return uuid.Nil, app.BadRequest("No project status %q. %s", value, options(typedNames(rows)))
}

func resolveProjectLabels(c *app.Ctx, values []string) ([]uuid.UUID, error) {
	ids := []uuid.UUID{}
	if len(values) == 0 {
		return ids, nil
	}
	type row struct {
		ID     uuid.UUID
		Name   string
		Parent *string
	}
	rows, err := app.Rows[row](c, c.DB(),
		`SELECT l.id, l.name, p.name FROM project_labels l LEFT JOIN project_labels p ON p.id = l.parent_id
		 WHERE l.workspace_id = $1 AND l.retired_at IS NULL AND NOT l.is_group`, c.Workspace)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		index := slices.IndexFunc(rows, func(r row) bool { return labelMatches(r.ID, r.Name, r.Parent, value) })
		if index < 0 {
			names := make([]string, len(rows))
			for i, r := range rows {
				names[i] = r.Name
			}
			return nil, app.BadRequest("No project label %q. %s", value, options(names))
		}
		ids = appendUnique(ids, rows[index].ID)
	}
	return ids, nil
}

// pipelineVisible is SQL that is true when the pipeline in column has no teams or one the caller
// sees. The team ids are UUIDs, so inlining them is safe.
func pipelineVisible(c *app.Ctx, column string) (string, error) {
	teams, err := c.TeamIDs()
	if err != nil {
		return "", err
	}
	ids := make([]string, len(teams))
	for i, t := range teams {
		ids[i] = t.String()
	}
	return fmt.Sprintf(
		`AND (NOT EXISTS (SELECT 1 FROM release_pipeline_teams pt WHERE pt.pipeline_id = %s)
		  OR EXISTS (SELECT 1 FROM release_pipeline_teams pt WHERE pt.pipeline_id = %s
		             AND pt.team_id = ANY('{%s}'::uuid[])))`, column, column, strings.Join(ids, ",")), nil
}

func resolveRelease(c *app.Ctx, value string) (uuid.UUID, error) {
	extra, err := pipelineVisible(c, "releases.pipeline_id")
	if err != nil {
		return uuid.Nil, err
	}
	return resolveNamed(c, "releases", "name", extra, "release", value)
}

func resolveReleases(c *app.Ctx, values []string) ([]uuid.UUID, error) {
	return resolveAll(c, values, resolveRelease)
}

func resolvePipeline(c *app.Ctx, value string) (uuid.UUID, error) {
	extra, err := pipelineVisible(c, "release_pipelines.id")
	if err != nil {
		return uuid.Nil, err
	}
	return resolveNamed(c, "release_pipelines", "name", extra, "release pipeline", value)
}

func resolveCustomer(c *app.Ctx, value string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	id, found, err := app.Maybe[uuid.UUID](c, c.DB(),
		`SELECT id FROM customers WHERE workspace_id = $1 AND (lower($2) = ANY(domains) OR $2 = ANY(external_ids)) LIMIT 1`,
		c.Workspace, value)
	if err != nil {
		return uuid.Nil, err
	}
	if found {
		return id, nil
	}
	return resolveNamed(c, "customers", "name", "", "customer", value)
}

func resolveCustomView(c *app.Ctx, value string) (uuid.UUID, error) {
	return resolveNamed(c, "custom_views", "name", fmt.Sprintf("AND (shared OR owner_id = '%s')", c.User), "view", value)
}

// resolveTemplate is a template by id or name, of the given type (issue, project or document).
func resolveTemplate(c *app.Ctx, kind, value string) (uuid.UUID, error) {
	type row struct {
		ID   uuid.UUID
		Name string
		Team *uuid.UUID
	}
	rows, err := app.Rows[row](c, c.DB(),
		"SELECT id, name, team_id FROM templates WHERE workspace_id = $1 AND type = $2 ORDER BY name", c.Workspace, kind)
	if err != nil {
		return uuid.Nil, err
	}
	visible, err := c.TeamIDs()
	if err != nil {
		return uuid.Nil, err
	}
	rows = slices.DeleteFunc(rows, func(r row) bool { return r.Team != nil && !slices.Contains(visible, *r.Team) })
	value = strings.TrimSpace(value)
	id, isID := asUUID(value)
	for _, r := range rows {
		if isID && r.ID == id {
			return r.ID, nil
		}
	}
	names := make([]string, len(rows))
	for i, r := range rows {
		if strings.EqualFold(r.Name, value) {
			return r.ID, nil
		}
		names[i] = r.Name
	}
	return uuid.Nil, app.BadRequest("No %s template %q. %s", kind, value, options(names))
}

// resolveByID is a row of table in the caller's workspace by id, or a 404 naming what.
func resolveByID(c *app.Ctx, table, what, value string) (uuid.UUID, error) {
	notFound := app.NotFound("No %s %q", what, strings.TrimSpace(value))
	id, ok := asUUID(lastSegment(value))
	if !ok {
		return uuid.Nil, notFound
	}
	exists, err := app.One[bool](c, c.DB(),
		fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM %s WHERE id = $1 AND workspace_id = $2)", table), id, c.Workspace)
	if err != nil {
		return uuid.Nil, err
	}
	if !exists {
		return uuid.Nil, notFound
	}
	return id, nil
}
