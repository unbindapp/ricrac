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

// Projects: listing, reading, creating and changing them (teams, members, labels, dates, links,
// initiatives, dependencies and templates), trash and archive, and the workspace's project
// statuses and labels.

func projectOps() []*Op {
	return []*Op{
		op("list_projects", app.Read, []string{"GET /projects"},
			"List projects in teams you can see, most recently updated first. Narrow with team, status (name or type: "+
				"backlog, planned, started, paused, completed, canceled), lead (\"me\"), member, initiative, label, health, "+
				"priority, query, or a saved project view (customView). Archived and trashed projects are left out unless "+
				"asked for; descriptions unless includeDescription is set. Each project has its progress: completed issues "+
				"/ (all - canceled).",
			listProjects).returns(ResultOf[PageOf[Project]]()),
		op("get_project", app.Read, []string{"GET /projects/{id}"},
			"Read a project by id, slug, link or name: its fields and description, members, milestones with their "+
				"progress, latest status update, links, dependencies (blocks, blocked by, related), documents, issue counts "+
				"by status type and estimate sums.",
			getProject).returns(ResultOf[ProjectDetail]()),
		op("save_project", app.Write, []string{"POST /projects", "PATCH /projects/{id}"},
			"Create a project (omit id; name and at least one team via setTeams or addTeams are required) or update one "+
				"(pass id). Omitted fields stay as they are; null clears a field. People are given as id, name, email or "+
				"\"me\"; the status (state) as name, type or id. Use patch to edit part of the description. A project template "+
				"(create only) adds its milestones and issues. A new lead is notified; so are people @mentioned in the "+
				"description.",
			saveProject).returns(ResultOf[ProjectDetail]()),
		op("delete_project", app.Write, []string{"DELETE /projects/{id}"},
			"Move a project to the trash. Its issues keep their project. It is deleted for good after 30 days; "+
				"restore_project brings it back before then.",
			trashProject).returns(ResultOf[Project]()),
		op("restore_project", app.Write, []string{"POST /projects/{id}/restore"}, "Bring a project back from the trash.", restoreProject).returns(ResultOf[Project]()),
		op("archive_project", app.Write, []string{"POST /projects/{id}/archive"},
			"Archive a project. It stays readable but is left out of lists unless includeArchived is set.", archiveProject).returns(ResultOf[Project]()),
		op("unarchive_project", app.Write, []string{"POST /projects/{id}/unarchive"}, "Bring an archived project back.", unarchiveProject).returns(ResultOf[Project]()),
		op("list_project_statuses", app.Read, []string{"GET /project-statuses"},
			"The workspace's project statuses in order, each with its type (backlog, planned, started, paused, "+
				"completed, canceled) and how many projects are in it.",
			listProjectStatuses).returns(ResultOf[PageOf[ProjectStatus]]()),
		op("save_project_status", app.Write, []string{"POST /project-statuses", "PATCH /project-statuses/{id}"},
			"Create a project status (omit id; name and type are required) or change one (pass its id or name). "+
				"Workspace admins only.",
			saveProjectStatus).returns(ResultOf[ProjectStatus]()),
		op("delete_project_status", app.Write, []string{"DELETE /project-statuses/{id}"},
			"Delete a project status. Projects in it move to moveProjectsTo, which is required when there are any. "+
				"The workspace keeps at least one status. Workspace admins only.",
			deleteProjectStatus).returns(ResultOf[DeletedProjectStatus]()),
		op("list_project_labels", app.Read, []string{"GET /project-labels"},
			"The workspace's project labels and label groups, with how many projects use each. Retired labels are "+
				"left out unless includeRetired is set.",
			listProjectLabels).returns(ResultOf[PageOf[ProjectLabel]]()),
		op("save_project_label", app.Write, []string{"POST /project-labels", "PATCH /project-labels/{id}"},
			"Create a project label (omit id; name is required) or change one (pass its id or name). A label can sit in "+
				"a group (parent, a label with isGroup); groups don't nest.",
			saveProjectLabel).returns(ResultOf[ProjectLabel]()),
		op("retire_project_label", app.Write, []string{"POST /project-labels/{id}/retire"},
			"Retire a project label (and a group's labels): projects keep it, but it can't be added anymore.", retireProjectLabel).returns(ResultOf[ProjectLabel]()),
		op("restore_project_label", app.Write, []string{"POST /project-labels/{id}/restore"},
			"Bring a retired project label back.", restoreProjectLabel).returns(ResultOf[ProjectLabel]()),
		op("delete_project_label", app.Write, []string{"DELETE /project-labels/{id}"},
			"Delete a project label for good, removing it from every project.", deleteProjectLabel).returns(ResultOf[DeletedProjectLabel]()),
	}
}

/* Representation */

const projectFrom = "FROM projects p JOIN workspaces w ON w.id = p.workspace_id"

// projectJSON is the JSON of the project p (with its workspace w joined).
func projectJSON(c *app.Ctx, withDescription bool) string {
	description := ""
	if withDescription {
		description = "'description', p.content,"
	}
	return fmt.Sprintf(`(jsonb_build_object(
		'id', p.id,
		'slugId', p.slug_id,
		'name', p.name,
		'summary', p.summary,
		%s
		'url', %s || '/' || w.url_key || '/project/' || concat_ws('-', nullif(entity_slug(p.name), ''), p.slug_id),
		'icon', p.icon,
		'color', p.color,
		'status', project_status_ref(p.status_id),
		'priority', p.priority,
		'priorityLabel', (ARRAY['No priority','Urgent','High','Medium','Low'])[p.priority + 1],
		'health', p.health,
		'lead', user_ref(p.lead_id),
		'leadTeam', team_ref(p.lead_team_id),
		'teams', (SELECT coalesce(jsonb_agg(team_ref(pt.team_id) ORDER BY tt.key), '[]')
			FROM project_teams pt JOIN teams tt ON tt.id = pt.team_id WHERE pt.project_id = p.id),
		'labels', (SELECT coalesce(jsonb_agg(project_label_ref(pl.label_id) ORDER BY l.name), '[]')
			FROM project_label_links pl JOIN project_labels l ON l.id = pl.label_id WHERE pl.project_id = p.id),
		'initiatives', (SELECT coalesce(jsonb_agg(initiative_ref(ip.initiative_id) ORDER BY ini.name), '[]')
			FROM initiative_projects ip JOIN initiatives ini ON ini.id = ip.initiative_id
			WHERE ip.project_id = p.id AND ini.trashed_at IS NULL)
	) || jsonb_build_object(
		'startDate', p.start_date,
		'startDateResolution', p.start_date_resolution,
		'targetDate', p.target_date,
		'targetDateResolution', p.target_date_resolution,
		'sortOrder', p.sort_order,
		'startedAt', p.started_at,
		'completedAt', p.completed_at,
		'canceledAt', p.canceled_at,
		'archivedAt', p.archived_at,
		'trashed', p.trashed_at IS NOT NULL,
		'trashedAt', p.trashed_at,
		'createdAt', p.created_at,
		'updatedAt', p.updated_at
	) || project_issue_stats(p.id, NULL))`, description, sqlLiteral(c.PublicURL()))
}

// projectValue is one project's JSON, with its description.
func projectValue(c *app.Ctx, q app.Querier, id uuid.UUID) (M, error) {
	value, found, err := app.Maybe[M](c, q, fmt.Sprintf("SELECT %s %s WHERE p.id = $1", projectJSON(c, true), projectFrom), id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, app.NotFound("No such project")
	}
	return value, nil
}

// linksSQL is the JSON array of the links on the thing whose id is in column (project_id or
// initiative_id), as a subquery on $1.
func linksSQL(column string) string {
	return fmt.Sprintf(`(SELECT coalesce(jsonb_agg(jsonb_build_object('id', l.id, 'url', l.url, 'label', l.label,
		'creator', user_ref(l.creator_id), 'createdAt', l.created_at) ORDER BY l.sort_order, l.created_at), '[]')
	  FROM entity_links l WHERE l.%s = $1)`, column)
}

// projectDetail is everything get_project shows: the project, its people, milestones, latest
// update, links, dependencies, documents, issue counts and estimates.
func projectDetail(c *app.Ctx, q app.Querier, id uuid.UUID) (M, error) {
	value, err := projectValue(c, q, id)
	if err != nil {
		return nil, err
	}
	teams, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	visible := projectVisible("o", 2)
	sql := fmt.Sprintf(`SELECT jsonb_build_object(
		'creator', (SELECT user_ref(creator_id) FROM projects WHERE id = $1),
		'members', (SELECT coalesce(jsonb_agg(user_ref(m.user_id) ORDER BY u.name), '[]')
			FROM project_members m JOIN users u ON u.id = m.user_id WHERE m.project_id = $1),
		'subscribers', (SELECT coalesce(jsonb_agg(user_ref(s.user_id) ORDER BY u.name), '[]')
			FROM project_subscribers s JOIN users u ON u.id = s.user_id WHERE s.project_id = $1),
		'links', %[1]s,
		'relations', jsonb_build_object(
			'blocks', (SELECT coalesce(jsonb_agg(project_ref(o.id) ORDER BY r.created_at), '[]')
				FROM project_relations r JOIN projects o ON o.id = r.related_project_id
				WHERE r.project_id = $1 AND r.type = 'blocks' AND o.trashed_at IS NULL AND %[2]s),
			'blockedBy', (SELECT coalesce(jsonb_agg(project_ref(o.id) ORDER BY r.created_at), '[]')
				FROM project_relations r JOIN projects o ON o.id = r.project_id
				WHERE r.related_project_id = $1 AND r.type = 'blocks' AND o.trashed_at IS NULL AND %[2]s),
			'related', (SELECT coalesce(jsonb_agg(project_ref(o.id) ORDER BY r.created_at), '[]')
				FROM project_relations r
				JOIN projects o ON o.id = CASE WHEN r.project_id = $1 THEN r.related_project_id ELSE r.project_id END
				WHERE (r.project_id = $1 OR r.related_project_id = $1) AND r.type = 'related'
				AND o.trashed_at IS NULL AND %[2]s)
		),
		'documents', (SELECT coalesce(jsonb_agg(document_ref(d.id) ORDER BY d.sort_order, d.created_at), '[]')
			FROM documents d WHERE d.project_id = $1 AND d.trashed_at IS NULL),
		'issueCountByStateType', (SELECT coalesce(jsonb_object_agg(x.type, x.n), '{}') FROM (
			SELECT s.type, count(*) AS n FROM issues i JOIN workflow_states s ON s.id = i.state_id
			WHERE i.project_id = $1 AND i.trashed_at IS NULL GROUP BY s.type) x),
		'estimate', (SELECT jsonb_build_object('total', coalesce(sum(i.estimate), 0),
				'completed', coalesce(sum(i.estimate) FILTER (WHERE s.type = 'completed'), 0))
			FROM issues i JOIN workflow_states s ON s.id = i.state_id
			WHERE i.project_id = $1 AND i.trashed_at IS NULL AND s.type <> 'canceled')
	)`, linksSQL("project_id"), visible)
	more, err := app.One[M](c, q, sql, id, teams)
	if err != nil {
		return nil, err
	}
	for key, v := range more {
		value[key] = v
	}
	if value["milestones"], err = projectMilestones(c, q, id); err != nil {
		return nil, err
	}
	if value["latestUpdate"], err = latestStatusUpdate(c, q, "project_id", id); err != nil {
		return nil, err
	}
	return value, nil
}

// webhookTeam is the team a project's webhook events are filed under: its lead team, else its
// first team.
func webhookTeam(c *app.Ctx, q app.Querier, project uuid.UUID) (*uuid.UUID, error) {
	return app.One[*uuid.UUID](c, q,
		`SELECT coalesce(p.lead_team_id, (SELECT pt.team_id FROM project_teams pt JOIN teams t ON t.id = pt.team_id
			WHERE pt.project_id = p.id ORDER BY t.key LIMIT 1)) FROM projects p WHERE p.id = $1`, project)
}

/* List */

type ListProjectsArgs struct {
	Team               *string `json:"team" doc:"Team key, name or id."`
	Status             *string `json:"status" doc:"Project status name, type (backlog, planned, started, paused, completed, canceled) or id."`
	Lead               *string `json:"lead" doc:"Lead (id, name, email or \"me\"), or \"null\" for projects without one."`
	Member             *string `json:"member" doc:"A project member (id, name, email or \"me\")."`
	Initiative         *string `json:"initiative" doc:"Initiative name, slug or id."`
	Label              *string `json:"label" doc:"Project label name or id."`
	Health             *string `json:"health" doc:"onTrack, atRisk or offTrack."`
	Priority           *int    `json:"priority" doc:"0 none, 1 urgent, 2 high, 3 medium, 4 low."`
	Query              *string `json:"query" doc:"Words to find in the name, summary or description."`
	CustomView         *string `json:"customView" doc:"A saved project view (name, slug or id) whose filter applies too."`
	Filter             any     `json:"filter" doc:"Further conditions, as in project views. Fields: status {id,name,type}, lead {id,isMe}, team {id,key}, priority, health, name, startDate, targetDate, and, or. Comparators: eq, neq, in, nin, lt, lte, gt, gte, null; dates take ISO 8601 or a duration from now like P2W. Example: {\"status\": {\"type\": {\"in\": [\"planned\", \"started\"]}}, \"targetDate\": {\"lt\": \"P1M\"}}"`
	IncludeArchived    bool    `json:"includeArchived,omitempty"`
	IncludeTrashed     bool    `json:"includeTrashed,omitempty"`
	IncludeDescription bool    `json:"includeDescription,omitempty" doc:"Include each project's description."`
	OrderBy            *string `json:"orderBy" doc:"updatedAt (default), createdAt, targetDate, priority or manual."`
	Limit              *int64  `json:"limit" doc:"How many to return, 50 by default, at most 250."`
	Cursor             *string `json:"cursor" doc:"The endCursor of the previous page."`
}

var projectStatusTypes = []string{"backlog", "planned", "started", "paused", "completed", "canceled"}

func checkHealth(value string) (string, error) {
	for _, health := range []string{"onTrack", "atRisk", "offTrack"} {
		if strings.EqualFold(health, strings.TrimSpace(value)) {
			return health, nil
		}
	}
	return "", app.BadRequest("health is onTrack, atRisk or offTrack, not \"%s\"", value)
}

// isNoneWord is whether a value means nothing, for project and initiative fields.
func isNoneWord(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "null", "none":
		return true
	}
	return false
}

func listProjects(c *app.Ctx, args ListProjectsArgs) (any, error) {
	page := Page{Limit: args.Limit, Cursor: args.Cursor}
	orders := map[string]string{
		"updatedAt":  "p.updated_at DESC",
		"createdAt":  "p.created_at DESC",
		"targetDate": "p.target_date ASC NULLS LAST, p.updated_at DESC",
		"priority":   "CASE WHEN p.priority = 0 THEN 5 ELSE p.priority END, p.updated_at DESC",
		"manual":     "p.sort_order ASC, p.created_at",
	}
	orderBy := "updatedAt"
	if args.OrderBy != nil {
		orderBy = *args.OrderBy
	}
	order, ok := orders[orderBy]
	if !ok {
		return nil, app.BadRequest("orderBy \"%s\" isn't one of updatedAt, createdAt, targetDate, priority, manual", orderBy)
	}
	teams, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	q := app.NewSQL(fmt.Sprintf("SELECT %s %s WHERE p.workspace_id = ", projectJSON(c, args.IncludeDescription), projectFrom)).Bind(c.Workspace)
	q.Push(" AND EXISTS (SELECT 1 FROM project_teams vt WHERE vt.project_id = p.id AND vt.team_id = ANY(").Bind(teams).Push("))")
	if !args.IncludeArchived {
		q.Push(" AND p.archived_at IS NULL")
	}
	if !args.IncludeTrashed {
		q.Push(" AND p.trashed_at IS NULL")
	}
	if args.Team != nil {
		team, err := resolveTeam(c, *args.Team)
		if err != nil {
			return nil, err
		}
		q.Push(" AND EXISTS (SELECT 1 FROM project_teams ft WHERE ft.project_id = p.id AND ft.team_id = ").Bind(team).Push(")")
	}
	if args.Status != nil {
		lower := strings.ToLower(strings.TrimSpace(*args.Status))
		if slices.Contains(projectStatusTypes, lower) {
			q.Push(" AND EXISTS (SELECT 1 FROM project_statuses fs WHERE fs.id = p.status_id AND fs.type = ").Bind(lower).Push(")")
		} else {
			status, err := resolveProjectStatus(c, *args.Status)
			if err != nil {
				return nil, err
			}
			q.Push(" AND p.status_id = ").Bind(status)
		}
	}
	if args.Lead != nil {
		if isNoneWord(*args.Lead) {
			q.Push(" AND p.lead_id IS NULL")
		} else {
			lead, err := resolveUser(c, *args.Lead)
			if err != nil {
				return nil, err
			}
			q.Push(" AND p.lead_id = ").Bind(lead)
		}
	}
	if args.Member != nil {
		member, err := resolveUser(c, *args.Member)
		if err != nil {
			return nil, err
		}
		q.Push(" AND EXISTS (SELECT 1 FROM project_members fm WHERE fm.project_id = p.id AND fm.user_id = ").Bind(member).Push(")")
	}
	if args.Initiative != nil {
		initiative, err := resolveInitiative(c, *args.Initiative)
		if err != nil {
			return nil, err
		}
		q.Push(" AND EXISTS (SELECT 1 FROM initiative_projects fi WHERE fi.project_id = p.id AND fi.initiative_id = ").Bind(initiative).Push(")")
	}
	if args.Label != nil {
		ids, err := resolveProjectLabels(c, []string{*args.Label})
		if err != nil {
			return nil, err
		}
		q.Push(" AND EXISTS (SELECT 1 FROM project_label_links fl WHERE fl.project_id = p.id AND fl.label_id = ANY(").Bind(ids).Push("))")
	}
	if args.Health != nil {
		health, err := checkHealth(*args.Health)
		if err != nil {
			return nil, err
		}
		q.Push(" AND p.health = ").Bind(health)
	}
	if args.Priority != nil {
		priority, err := checkPriority(*args.Priority)
		if err != nil {
			return nil, err
		}
		q.Push(" AND p.priority = ").Bind(priority)
	}
	if query := strings.TrimSpace(orEmpty(args.Query)); query != "" {
		q.Push(" AND (p.search @@ websearch_to_tsquery('english', ").Bind(query)
		q.Push(") OR p.name ILIKE ").Bind("%" + escapeLike(query) + "%").Push(")")
	}
	if args.CustomView != nil {
		id, err := resolveCustomView(c, *args.CustomView)
		if err != nil {
			return nil, err
		}
		type view struct {
			Model  string
			Filter any
			Team   *uuid.UUID
		}
		v, _, err := app.Row[view](c, c.DB(), "SELECT model, filter, team_id FROM custom_views WHERE id = $1", id)
		if err != nil {
			return nil, err
		}
		if v.Model != "project" {
			return nil, app.BadRequest("That view shows %ss, not projects", v.Model)
		}
		q.Push(" AND ")
		if err := pushProjectFilter(q, c, v.Filter); err != nil {
			return nil, err
		}
		if v.Team != nil {
			q.Push(" AND EXISTS (SELECT 1 FROM project_teams vw WHERE vw.project_id = p.id AND vw.team_id = ").Bind(*v.Team).Push(")")
		}
	}
	if args.Filter != nil {
		q.Push(" AND ")
		if err := pushProjectFilter(q, c, args.Filter); err != nil {
			return nil, err
		}
	}
	offset, err := page.offset()
	if err != nil {
		return nil, err
	}
	q.Push(" ORDER BY " + order + " LIMIT ").Bind(page.limit() + 1).Push(" OFFSET ").Bind(offset)
	nodes, err := app.All[any](c, c.DB(), q.String(), q.Args...)
	if err != nil {
		return nil, err
	}
	return page.wrap(nodes)
}

type filterField struct{ name, column, kind string }

// pushProjectFilter pushes the condition of a project filter (the filter of a project view) on p.
func pushProjectFilter(q *app.SQL, c *app.Ctx, filter any) error {
	object, ok := filter.(M)
	if !ok {
		return app.BadRequest("A project filter must be an object")
	}
	q.Push("(TRUE")
	for _, key := range sortedKeys(object) {
		value := object[key]
		q.Push(" AND ")
		var err error
		switch key {
		case "and", "or":
			list, ok := value.([]any)
			if !ok {
				return app.BadRequest("\"%s\" takes a list of filters", key)
			}
			joiner := " OR "
			if key == "and" {
				q.Push("(TRUE")
				joiner = " AND "
			} else {
				q.Push("(FALSE")
			}
			for _, item := range list {
				q.Push(joiner)
				if err := pushProjectFilter(q, c, item); err != nil {
					return err
				}
			}
			q.Push(")")
		case "priority":
			err = compareFilter(q, "p.priority", "int", value)
		case "health":
			err = compareFilter(q, "p.health", "text", value)
		case "name":
			err = compareFilter(q, "p.name", "text", value)
		case "startDate":
			err = compareFilter(q, "p.start_date", "date", value)
		case "targetDate":
			err = compareFilter(q, "p.target_date", "date", value)
		case "status":
			q.Push("EXISTS (SELECT 1 FROM project_statuses fs WHERE fs.id = p.status_id")
			err = nestedFilter(q, key, value, []filterField{{"id", "fs.id", "uuid"}, {"name", "fs.name", "text"}, {"type", "fs.type", "text"}})
			q.Push(")")
		case "team", "teams":
			q.Push("EXISTS (SELECT 1 FROM project_teams ft JOIN teams t ON t.id = ft.team_id WHERE ft.project_id = p.id")
			err = nestedFilter(q, key, value, []filterField{{"id", "t.id", "uuid"}, {"key", "t.key", "text"}, {"name", "t.name", "text"}})
			q.Push(")")
		case "lead":
			err = leadFilter(q, c, value)
		default:
			return app.BadRequest("Project filters don't have \"%s\"; use status, lead, team, priority, health, name, startDate, "+
				"targetDate, and, or", key)
		}
		if err != nil {
			return err
		}
	}
	q.Push(")")
	return nil
}

func leadFilter(q *app.SQL, c *app.Ctx, value any) error {
	fields, ok := value.(M)
	if !ok {
		return app.BadRequest("lead takes {id, isMe, null}")
	}
	q.Push("(TRUE")
	for _, field := range sortedKeys(fields) {
		condition := fields[field]
		q.Push(" AND ")
		switch field {
		case "id":
			if err := compareFilter(q, "p.lead_id", "uuid", condition); err != nil {
				return err
			}
		case "null":
			if boolOr(condition, true) {
				q.Push("p.lead_id IS NULL")
			} else {
				q.Push("p.lead_id IS NOT NULL")
			}
		case "isMe":
			me, ok := condition.(bool)
			if !ok {
				object, _ := condition.(M)
				me = boolOr(object["eq"], true)
			}
			if me {
				q.Push("p.lead_id = ").Bind(c.User)
			} else {
				q.Push("p.lead_id IS DISTINCT FROM ").Bind(c.User)
			}
		default:
			return app.BadRequest("lead filters take id, isMe or null, not \"%s\"", field)
		}
	}
	q.Push(")")
	return nil
}

func boolOr(value any, fallback bool) bool {
	if b, ok := value.(bool); ok {
		return b
	}
	return fallback
}

// nestedFilter pushes " AND <column> <comparators>" for each field of a nested filter like
// status: {type: {eq: "started"}}.
func nestedFilter(q *app.SQL, key string, value any, fields []filterField) error {
	object, ok := value.(M)
	if !ok {
		return app.BadRequest("%s takes an object", key)
	}
	for _, name := range sortedKeys(object) {
		index := slices.IndexFunc(fields, func(f filterField) bool { return f.name == name })
		if index < 0 {
			names := make([]string, len(fields))
			for i, f := range fields {
				names[i] = f.name
			}
			return app.BadRequest("%s filters take %s, not \"%s\"", key, strings.Join(names, ", "), name)
		}
		q.Push(" AND ")
		if err := compareFilter(q, fields[index].column, fields[index].kind, object[name]); err != nil {
			return err
		}
	}
	return nil
}

func filterScalar(value any, kind string) (string, error) {
	var text string
	switch v := value.(type) {
	case string:
		text = v
	case float64:
		text = strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		text = strconv.FormatBool(v)
	default:
		return "", app.BadRequest("%s isn't a value to compare with", app.Marshal(value))
	}
	switch kind {
	case "date":
		at, err := parseDatetime(text)
		if err != nil {
			return "", err
		}
		return at.Format(time.DateOnly), nil
	case "text":
		return strings.ToLower(text), nil
	}
	return text, nil
}

// compareFilter pushes "<column> <comparators>"; a plain value means eq. Text compares without
// case.
func compareFilter(q *app.SQL, column, kind string, condition any) error {
	if kind == "text" {
		column = "lower(" + column + ")"
	}
	comparators, ok := condition.(M)
	if !ok {
		text, err := filterScalar(condition, kind)
		if err != nil {
			return err
		}
		q.Push(column + " = ").Bind(text).Push("::" + kind)
		return nil
	}
	signs := map[string]string{"eq": "=", "neq": "IS DISTINCT FROM", "lt": "<", "lte": "<=", "gt": ">", "gte": ">="}
	q.Push("(TRUE")
	for _, comparator := range sortedKeys(comparators) {
		value := comparators[comparator]
		q.Push(" AND ")
		switch comparator {
		case "in", "nin":
			items, ok := value.([]any)
			if !ok {
				return app.BadRequest("%s takes a list", comparator)
			}
			list := make([]string, len(items))
			for i, item := range items {
				text, err := filterScalar(item, kind)
				if err != nil {
					return err
				}
				list[i] = text
			}
			if comparator == "in" {
				q.Push(column + " = ANY(")
			} else {
				q.Push("NOT (" + column + " = ANY(")
			}
			q.Bind(list).Push("::text[]::" + kind + "[])")
			if comparator == "nin" {
				q.Push(")")
			}
		case "null":
			if boolOr(value, true) {
				q.Push(column + " IS NULL")
			} else {
				q.Push(column + " IS NOT NULL")
			}
		default:
			sign, ok := signs[comparator]
			if !ok {
				return app.BadRequest("\"%s\" isn't a comparator; use eq, neq, in, nin, lt, lte, gt, gte or null", comparator)
			}
			text, err := filterScalar(value, kind)
			if err != nil {
				return err
			}
			q.Push(column + " " + sign + " ").Bind(text).Push("::" + kind)
		}
	}
	q.Push(")")
	return nil
}

/* Read */

func getProject(c *app.Ctx, args IDArgs) (any, error) {
	id, err := resolveProject(c, args.ID)
	if err != nil {
		return nil, err
	}
	return projectDetail(c, c.DB(), id)
}

/* Save */

type SaveProjectArgs struct {
	ID                   *string          `json:"id" doc:"Only to update a project: its id, slug, link or name. Omit to create one."`
	Name                 *string          `json:"name" doc:"Required to create."`
	Summary              Nullable[string] `json:"summary" doc:"A one-line summary, at most 255 characters; null clears it."`
	Description          *string          `json:"description" doc:"Markdown. Mention people with @displayName or @email; they are notified."`
	Patch                []TextEdit       `json:"patch" doc:"Edits to the current description instead of the whole text, applied in order, all or none."`
	Icon                 Nullable[string] `json:"icon" doc:"An emoji or icon name; null clears it."`
	Color                Nullable[string] `json:"color" doc:"Hex color like #5e6ad2; null clears it."`
	State                *string          `json:"state" doc:"Project status name, type (backlog, planned, started, paused, completed, canceled) or id. New projects start in the first backlog status."`
	Priority             *int             `json:"priority" doc:"0 none, 1 urgent, 2 high, 3 medium, 4 low."`
	Lead                 Nullable[string] `json:"lead" doc:"Person (id, name, email or \"me\"); null removes the lead."`
	LeadTeam             Nullable[string] `json:"leadTeam" doc:"One of the project's teams (key, name or id) that leads it; null clears it."`
	AddTeams             []string         `json:"addTeams" doc:"Teams (keys, names or ids) to add."`
	RemoveTeams          []string         `json:"removeTeams"`
	SetTeams             []string         `json:"setTeams" doc:"Replace all teams with these. A project keeps at least one team."`
	AddMembers           []string         `json:"addMembers" doc:"People to add as members (they are subscribed)."`
	RemoveMembers        []string         `json:"removeMembers"`
	Labels               []string         `json:"labels" doc:"Replace all project labels with these (names or ids; Group/Label for a label in a group)."`
	AddLabels            []string         `json:"addLabels"`
	RemoveLabels         []string         `json:"removeLabels"`
	StartDate            Nullable[string] `json:"startDate" doc:"YYYY-MM-DD; null clears it."`
	StartDateResolution  Nullable[string] `json:"startDateResolution" doc:"How precise startDate is: month, quarter, halfYear or year; null means the exact day."`
	TargetDate           Nullable[string] `json:"targetDate" doc:"YYYY-MM-DD; null clears it."`
	TargetDateResolution Nullable[string] `json:"targetDateResolution" doc:"How precise targetDate is: month, quarter, halfYear or year; null means the exact day."`
	Links                []LinkArg        `json:"links" doc:"Links to add ({url, title}); a link already on the project gets the new title."`
	RemoveLinks          []string         `json:"removeLinks" doc:"URLs of links to remove."`
	AddInitiatives       []string         `json:"addInitiatives" doc:"Initiatives (names, slugs or ids) to add the project to."`
	RemoveInitiatives    []string         `json:"removeInitiatives"`
	SetInitiatives       []string         `json:"setInitiatives" doc:"Replace all initiatives with these."`
	Blocks               []string         `json:"blocks" doc:"Projects this one blocks (added to existing dependencies)."`
	BlockedBy            []string         `json:"blockedBy" doc:"Projects blocking this one (added)."`
	RelatedTo            []string         `json:"relatedTo" doc:"Projects related to this one (added)."`
	RemoveBlocks         []string         `json:"removeBlocks"`
	RemoveBlockedBy      []string         `json:"removeBlockedBy"`
	RemoveRelatedTo      []string         `json:"removeRelatedTo"`
	SortOrder            *float64         `json:"sortOrder" doc:"Position in manually ordered lists (lower is higher up)."`
	Template             *string          `json:"template" doc:"Project template name or id, applied on create: it fills in what the arguments don't set and adds its milestones and issues."`
}

// projectTemplate is what a project template fills in on a new project.
type projectTemplate struct {
	Name        *string  `json:"name"`
	Summary     *string  `json:"summary"`
	Description *string  `json:"description"`
	State       *string  `json:"state"`
	Priority    *int     `json:"priority"`
	Lead        *string  `json:"lead"`
	Labels      []string `json:"labels"`
	Teams       []string `json:"teams"`
	Milestones  []struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		TargetDate  *string `json:"targetDate"`
	} `json:"milestones"`
	Issues []any `json:"issues"`
}

type projectRow struct {
	Name     string
	Summary  *string
	Content  string
	Icon     *string
	Color    *string
	StatusID uuid.UUID
	Priority int
	LeadID   *uuid.UUID
}

func loadProjectRow(c *app.Ctx, q app.Querier, id uuid.UUID) (projectRow, error) {
	row, found, err := app.Row[projectRow](c, q,
		"SELECT name, summary, content, icon, color, status_id, priority, lead_id FROM projects WHERE id = $1 FOR UPDATE", id)
	if err != nil {
		return row, err
	}
	if !found {
		return row, app.NotFound("No such project")
	}
	return row, nil
}

func saveProject(c *app.Ctx, args SaveProjectArgs) (any, error) {
	var id uuid.UUID
	var templateIssues []M
	if args.ID != nil {
		var err error
		if id, err = resolveProject(c, *args.ID); err != nil {
			return nil, err
		}
		if err := inTx(c, func(tx pgx.Tx) error { return updateProject(c, tx, id, args) }); err != nil {
			return nil, err
		}
	} else {
		if err := c.RequireMember(); err != nil {
			return nil, err
		}
		err := inTx(c, func(tx pgx.Tx) error {
			var err error
			id, templateIssues, err = createProject(c, tx, args)
			return err
		})
		if err != nil {
			return nil, err
		}
	}
	for n, issue := range templateIssues {
		if _, err := Find("save_issue").Call(c, issue); err != nil {
			return nil, app.BadRequest("The project was created, but the template's issue %d failed: %s", n+1, app.AsError(err).Message)
		}
	}
	return projectDetail(c, c.DB(), id)
}

// defaultProjectStatus is the status new projects start in: the first backlog status, else the
// first planned one.
func defaultProjectStatus(c *app.Ctx, q app.Querier) (uuid.UUID, error) {
	id, found, err := app.Maybe[uuid.UUID](c, q,
		`SELECT id FROM project_statuses WHERE workspace_id = $1 AND archived_at IS NULL
		 ORDER BY array_position(ARRAY['backlog', 'planned'], type) NULLS LAST, position LIMIT 1`, c.Workspace)
	if err != nil {
		return uuid.Nil, err
	}
	if !found {
		return uuid.Nil, app.BadRequest("The workspace has no project statuses; add one with save_project_status")
	}
	return id, nil
}

// applyProjectTemplate fills in what the arguments leave to the template.
func applyProjectTemplate(c *app.Ctx, tx pgx.Tx, args *SaveProjectArgs) (projectTemplate, error) {
	var t projectTemplate
	id, err := resolveTemplate(c, "project", *args.Template)
	if err != nil {
		return t, err
	}
	args.Template = nil
	type stored struct {
		Data any
		Team *uuid.UUID
	}
	row, _, err := app.Row[stored](c, tx, "SELECT data, team_id FROM templates WHERE id = $1", id)
	if err != nil {
		return t, err
	}
	if err := remarshal(row.Data, &t); err != nil {
		return t, app.BadRequest("The project template is invalid: %s", strings.TrimPrefix(err.Error(), "json: "))
	}
	for _, m := range t.Milestones {
		if m.Name == nil {
			return t, app.BadRequest("The project template is invalid: missing field `name`")
		}
	}
	args.Name = firstOf(args.Name, t.Name)
	args.Summary = orNullable(args.Summary, t.Summary)
	if orEmpty(args.Description) == "" && args.Patch == nil {
		args.Description = t.Description
	}
	args.State = firstOf(args.State, t.State)
	args.Priority = firstOf(args.Priority, t.Priority)
	args.Lead = orNullable(args.Lead, t.Lead)
	if args.Labels == nil && t.Labels != nil {
		args.AddLabels = append(args.AddLabels, t.Labels...)
	}
	if args.SetTeams == nil && len(args.AddTeams) == 0 {
		switch {
		case t.Teams != nil:
			args.AddTeams = t.Teams
		case row.Team != nil:
			args.AddTeams = []string{row.Team.String()}
		default:
			args.AddTeams = nil
		}
	}
	return t, nil
}

// createProject creates the project and its template milestones; it returns the template's
// issues to create (as save_issue arguments) once the project is committed.
func createProject(c *app.Ctx, tx pgx.Tx, args SaveProjectArgs) (uuid.UUID, []M, error) {
	var t projectTemplate
	if args.Template != nil {
		var err error
		if t, err = applyProjectTemplate(c, tx, &args); err != nil {
			return uuid.Nil, nil, err
		}
	}
	name, err := nonEmpty(orEmpty(args.Name), "name")
	if err != nil {
		return uuid.Nil, nil, err
	}
	teams := args.SetTeams
	if teams == nil {
		teams = args.AddTeams
	}
	if len(teams) == 0 {
		return uuid.Nil, nil, app.BadRequest("A project needs at least one team; pass setTeams or addTeams")
	}
	var status uuid.UUID
	if args.State != nil {
		status, err = resolveProjectStatus(c, *args.State)
	} else {
		status, err = defaultProjectStatus(c, tx)
	}
	if err != nil {
		return uuid.Nil, nil, err
	}
	args.State = nil
	id, err := app.One[uuid.UUID](c, tx,
		`INSERT INTO projects (workspace_id, slug_id, name, status_id, creator_id, sort_order)
		 VALUES ($1, $2, $3, $4, $5, (SELECT coalesce(max(sort_order), 0) + 1000 FROM projects WHERE workspace_id = $1))
		 RETURNING id`, c.Workspace, slugID(), name, status, c.User)
	if err != nil {
		return uuid.Nil, nil, err
	}
	if err := stampProjectStatus(c, tx, id); err != nil {
		return uuid.Nil, nil, err
	}
	if err := subscribeProject(c, tx, id, []uuid.UUID{c.User}); err != nil {
		return uuid.Nil, nil, err
	}
	if err := applyProjectChanges(c, tx, id, args, true); err != nil {
		return uuid.Nil, nil, err
	}
	milestones := map[string]uuid.UUID{}
	for n, m := range t.Milestones {
		var target *time.Time
		if m.TargetDate != nil {
			at, err := parseDatetime(*m.TargetDate)
			if err != nil {
				return uuid.Nil, nil, err
			}
			target = new(time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC))
		}
		milestoneName, err := nonEmpty(*m.Name, "A template milestone's name")
		if err != nil {
			return uuid.Nil, nil, err
		}
		milestone, err := app.One[uuid.UUID](c, tx,
			`INSERT INTO project_milestones (project_id, name, description, target_date, sort_order)
			 VALUES ($1, $2, $3, $4, $5) RETURNING id`, id, milestoneName, m.Description, target, float64(n))
		if err != nil {
			return uuid.Nil, nil, err
		}
		key := strings.ToLower(milestoneName)
		if _, taken := milestones[key]; !taken {
			milestones[key] = milestone
		}
	}
	firstTeam, err := webhookTeam(c, tx, id)
	if err != nil {
		return uuid.Nil, nil, err
	}
	var issues []M
	for n, item := range t.Issues {
		issue, ok := item.(map[string]any)
		if !ok {
			return uuid.Nil, nil, app.BadRequest("The template's issue %d isn't an object", n+1)
		}
		delete(issue, "id")
		issue["project"] = id.String()
		if _, named := issue["team"]; !named && firstTeam != nil {
			issue["team"] = firstTeam.String()
		}
		if milestone, ok := issue["milestone"].(string); ok {
			if found, ok := milestones[strings.ToLower(strings.TrimSpace(milestone))]; ok {
				issue["milestone"] = found.String()
			}
		}
		issues = append(issues, issue)
	}
	value, err := projectValue(c, tx, id)
	if err != nil {
		return uuid.Nil, nil, err
	}
	return id, issues, emit(c, tx, "Project", "create", value, nil, firstTeam)
}

func updateProject(c *app.Ctx, tx pgx.Tx, id uuid.UUID, args SaveProjectArgs) error {
	if args.Template != nil {
		return app.BadRequest("Templates only apply when creating a project")
	}
	before, err := projectValue(c, tx, id)
	if err != nil {
		return err
	}
	row, err := loadProjectRow(c, tx, id)
	if err != nil {
		return err
	}
	if args.Name != nil {
		name, err := nonEmpty(*args.Name, "name")
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "UPDATE projects SET name = $2 WHERE id = $1", id, name); err != nil {
			return err
		}
	}
	if args.State != nil {
		status, err := resolveProjectStatus(c, *args.State)
		if err != nil {
			return err
		}
		if status != row.StatusID {
			if err := app.Exec(c, tx, "UPDATE projects SET status_id = $2 WHERE id = $1", id, status); err != nil {
				return err
			}
			if err := stampProjectStatus(c, tx, id); err != nil {
				return err
			}
		}
		args.State = nil
	}
	if err := applyProjectChanges(c, tx, id, args, false); err != nil {
		return err
	}
	if err := app.Exec(c, tx, "UPDATE projects SET updated_at = now() WHERE id = $1", id); err != nil {
		return err
	}
	after, err := projectValue(c, tx, id)
	if err != nil {
		return err
	}
	updatedFrom := changedFields(before, after)
	if len(updatedFrom) == 0 {
		return nil
	}
	team, err := webhookTeam(c, tx, id)
	if err != nil {
		return err
	}
	return emit(c, tx, "Project", "update", after, updatedFrom, team)
}

// stampProjectStatus stamps when the project started, completed or was canceled, from the type
// of its status.
func stampProjectStatus(c *app.Ctx, tx pgx.Tx, id uuid.UUID) error {
	return app.Exec(c, tx,
		`UPDATE projects p SET
			started_at = CASE WHEN s.type = 'started' THEN coalesce(p.started_at, now())
							  WHEN s.type IN ('paused', 'completed', 'canceled') THEN p.started_at END,
			completed_at = CASE WHEN s.type = 'completed' THEN coalesce(p.completed_at, now()) END,
			canceled_at = CASE WHEN s.type = 'canceled' THEN coalesce(p.canceled_at, now()) END
		 FROM project_statuses s WHERE p.id = $1 AND s.id = p.status_id`, id)
}

func subscribeProject(c *app.Ctx, tx pgx.Tx, project uuid.UUID, users []uuid.UUID) error {
	return app.Exec(c, tx,
		"INSERT INTO project_subscribers (project_id, user_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING",
		project, users)
}

// checkResolution is a date resolution, nil when null or a null word.
func checkResolution(value Nullable[string]) (*string, error) {
	if !value.Valid || isNoneWord(value.Value) {
		return nil, nil
	}
	for _, resolution := range []string{"month", "quarter", "halfYear", "year"} {
		if strings.EqualFold(resolution, strings.TrimSpace(value.Value)) {
			return &resolution, nil
		}
	}
	return nil, app.BadRequest("A date resolution is month, quarter, halfYear or year, not \"%s\"", value.Value)
}

func optionalDate(value Nullable[string]) (*time.Time, error) {
	if !value.Valid || isNoneWord(value.Value) {
		return nil, nil
	}
	date, err := parseDate(value.Value)
	if err != nil {
		return nil, err
	}
	return &date, nil
}

// optionalRef resolves a nullable reference: nil when null or a null word.
func optionalRef(c *app.Ctx, value Nullable[string], resolve func(*app.Ctx, string) (uuid.UUID, error)) (*uuid.UUID, error) {
	if !value.Valid || isNoneWord(value.Value) {
		return nil, nil
	}
	id, err := resolve(c, value.Value)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// applyProjectChanges applies the fields that work the same when creating and updating.
func applyProjectChanges(c *app.Ctx, tx pgx.Tx, id uuid.UUID, args SaveProjectArgs, creating bool) error {
	row, err := loadProjectRow(c, tx, id)
	if err != nil {
		return err
	}
	if err := projectRelations(c, tx, id, args); err != nil {
		return err
	}
	if args.Summary.Set {
		var summary *string
		if trimmed := strings.TrimSpace(args.Summary.Value); args.Summary.Valid && trimmed != "" {
			summary = &trimmed
		}
		if summary != nil && len([]rune(*summary)) > 255 {
			return app.BadRequest("summary is at most 255 characters; put the rest in the description")
		}
		if !samePtr(summary, row.Summary) {
			if err := app.Exec(c, tx, "UPDATE projects SET summary = $2 WHERE id = $1", id, summary); err != nil {
				return err
			}
		}
	}
	content, err := newContent(row.Content, args.Description, args.Patch)
	if err != nil {
		return err
	}
	if content != nil && *content != row.Content {
		if err := projectContent(c, tx, id, row, *content, creating); err != nil {
			return err
		}
	}
	if args.Icon.Set {
		var icon *string
		if args.Icon.Valid && strings.TrimSpace(args.Icon.Value) != "" {
			icon = &args.Icon.Value
		}
		if !samePtr(icon, row.Icon) {
			if err := app.Exec(c, tx, "UPDATE projects SET icon = $2 WHERE id = $1", id, icon); err != nil {
				return err
			}
		}
	}
	if args.Color.Set {
		var color *string
		if args.Color.Valid {
			checked, err := checkColor(args.Color.Value)
			if err != nil {
				return err
			}
			color = &checked
		}
		if !samePtr(color, row.Color) {
			if err := app.Exec(c, tx, "UPDATE projects SET color = $2 WHERE id = $1", id, color); err != nil {
				return err
			}
		}
	}
	if args.Priority != nil {
		priority, err := checkPriority(*args.Priority)
		if err != nil {
			return err
		}
		if priority != row.Priority {
			if err := app.Exec(c, tx, "UPDATE projects SET priority = $2 WHERE id = $1", id, priority); err != nil {
				return err
			}
		}
	}
	if args.Lead.Set {
		if err := projectLead(c, tx, id, row, args.Lead); err != nil {
			return err
		}
	}
	if err := projectTeams(c, tx, id, args.SetTeams, args.AddTeams, args.RemoveTeams); err != nil {
		return err
	}
	if args.LeadTeam.Set {
		team, err := optionalRef(c, args.LeadTeam, resolveTeam)
		if err != nil {
			return err
		}
		onProject, err := app.One[bool](c, tx,
			"SELECT $2::uuid IS NULL OR EXISTS (SELECT 1 FROM project_teams WHERE project_id = $1 AND team_id = $2)", id, team)
		if err != nil {
			return err
		}
		if !onProject {
			return app.BadRequest("The lead team must be one of the project's teams; add it with addTeams")
		}
		if err := app.Exec(c, tx, "UPDATE projects SET lead_team_id = $2 WHERE id = $1", id, team); err != nil {
			return err
		}
	}
	if args.AddMembers != nil {
		users, err := resolveUsers(c, args.AddMembers)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx,
			"INSERT INTO project_members (project_id, user_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING", id, users); err != nil {
			return err
		}
		if err := subscribeProject(c, tx, id, users); err != nil {
			return err
		}
	}
	if args.RemoveMembers != nil {
		users, err := resolveUsers(c, args.RemoveMembers)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM project_members WHERE project_id = $1 AND user_id = ANY($2)", id, users); err != nil {
			return err
		}
	}
	if err := projectLabels(c, tx, id, args.Labels, args.AddLabels, args.RemoveLabels); err != nil {
		return err
	}
	if err := projectDates(c, tx, id, "start", args.StartDate, args.StartDateResolution); err != nil {
		return err
	}
	if err := projectDates(c, tx, id, "target", args.TargetDate, args.TargetDateResolution); err != nil {
		return err
	}
	if err := saveLinks(c, tx, "project_id", id, args.Links, args.RemoveLinks); err != nil {
		return err
	}
	if err := projectInitiatives(c, tx, id, args.SetInitiatives, args.AddInitiatives, args.RemoveInitiatives); err != nil {
		return err
	}
	if args.SortOrder != nil {
		return app.Exec(c, tx, "UPDATE projects SET sort_order = $2 WHERE id = $1", id, *args.SortOrder)
	}
	return nil
}

// projectContent sets the description and notifies the people newly mentioned in it.
func projectContent(c *app.Ctx, tx pgx.Tx, id uuid.UUID, row projectRow, content string, creating bool) error {
	if err := app.Exec(c, tx, "UPDATE projects SET content = $2 WHERE id = $1", id, content); err != nil {
		return err
	}
	var previous []uuid.UUID
	if !creating {
		var err error
		if previous, err = mentions(c, row.Content); err != nil {
			return err
		}
	}
	found, err := mentions(c, content)
	if err != nil {
		return err
	}
	mentioned := slices.DeleteFunc(found, func(u uuid.UUID) bool { return slices.Contains(previous, u) })
	if len(mentioned) == 0 {
		return nil
	}
	if err := subscribeProject(c, tx, id, mentioned); err != nil {
		return err
	}
	return notify(c, tx, mentioned, Notice{Kind: "projectMention", Summary: "You were mentioned in the project " + row.Name, Project: &id})
}

func projectLead(c *app.Ctx, tx pgx.Tx, id uuid.UUID, row projectRow, value Nullable[string]) error {
	lead, err := optionalRef(c, value, resolveUser)
	if err != nil {
		return err
	}
	if samePtr(lead, row.LeadID) {
		return nil
	}
	if err := app.Exec(c, tx, "UPDATE projects SET lead_id = $2 WHERE id = $1", id, lead); err != nil {
		return err
	}
	if lead == nil {
		return nil
	}
	if err := subscribeProject(c, tx, id, []uuid.UUID{*lead}); err != nil {
		return err
	}
	name, err := app.One[string](c, tx, "SELECT name FROM projects WHERE id = $1", id)
	if err != nil {
		return err
	}
	return notify(c, tx, []uuid.UUID{*lead}, Notice{Kind: "projectAssigned", Summary: "You are now the lead of the project " + name, Project: &id})
}

func projectDates(c *app.Ctx, tx pgx.Tx, id uuid.UUID, which string, date, resolution Nullable[string]) error {
	if date.Set {
		value, err := optionalDate(date)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, fmt.Sprintf(`UPDATE projects SET %[1]s_date = $2,
				%[1]s_date_resolution = CASE WHEN $2::date IS NULL THEN NULL ELSE %[1]s_date_resolution END
			 WHERE id = $1`, which), id, value); err != nil {
			return err
		}
	}
	if !resolution.Set {
		return nil
	}
	value, err := checkResolution(resolution)
	if err != nil {
		return err
	}
	return app.Exec(c, tx, fmt.Sprintf("UPDATE projects SET %s_date_resolution = $2 WHERE id = $1", which), id, value)
}

func projectTeams(c *app.Ctx, tx pgx.Tx, id uuid.UUID, set, add, remove []string) error {
	if set != nil && (add != nil || remove != nil) {
		return app.BadRequest("Pass either setTeams or addTeams/removeTeams, not both")
	}
	if set == nil && add == nil && remove == nil {
		return nil
	}
	if set != nil {
		ids, err := resolveTeams(c, set)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM project_teams WHERE project_id = $1 AND NOT (team_id = ANY($2))", id, ids); err != nil {
			return err
		}
		if err := insertProjectTeams(c, tx, id, ids); err != nil {
			return err
		}
	}
	if add != nil {
		ids, err := resolveTeams(c, add)
		if err != nil {
			return err
		}
		if err := insertProjectTeams(c, tx, id, ids); err != nil {
			return err
		}
	}
	if remove != nil {
		ids, err := resolveTeams(c, remove)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM project_teams WHERE project_id = $1 AND team_id = ANY($2)", id, ids); err != nil {
			return err
		}
	}
	count, err := app.One[int64](c, tx, "SELECT count(*) FROM project_teams WHERE project_id = $1", id)
	if err != nil {
		return err
	}
	if count == 0 {
		return app.BadRequest("A project keeps at least one team")
	}
	return app.Exec(c, tx,
		`UPDATE projects SET lead_team_id = NULL WHERE id = $1
		 AND lead_team_id NOT IN (SELECT team_id FROM project_teams WHERE project_id = $1)`, id)
}

// resolveTeams resolves each team, keeping repeats.
func resolveTeams(c *app.Ctx, values []string) ([]uuid.UUID, error) {
	ids := []uuid.UUID{}
	for _, value := range values {
		id, err := resolveTeam(c, value)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func insertProjectTeams(c *app.Ctx, tx pgx.Tx, id uuid.UUID, teams []uuid.UUID) error {
	return app.Exec(c, tx,
		"INSERT INTO project_teams (project_id, team_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING", id, teams)
}

func projectLabels(c *app.Ctx, tx pgx.Tx, id uuid.UUID, set, add, remove []string) error {
	if set != nil && (add != nil || remove != nil) {
		return app.BadRequest("Pass either labels or addLabels/removeLabels, not both")
	}
	if set != nil {
		ids, err := resolveProjectLabels(c, set)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM project_label_links WHERE project_id = $1 AND NOT (label_id = ANY($2))", id, ids); err != nil {
			return err
		}
		if err := insertProjectLabels(c, tx, id, ids); err != nil {
			return err
		}
	}
	if add != nil {
		ids, err := resolveProjectLabels(c, add)
		if err != nil {
			return err
		}
		if err := insertProjectLabels(c, tx, id, ids); err != nil {
			return err
		}
	}
	if remove == nil {
		return nil
	}
	ids, err := resolveProjectLabels(c, remove)
	if err != nil {
		return err
	}
	return app.Exec(c, tx, "DELETE FROM project_label_links WHERE project_id = $1 AND label_id = ANY($2)", id, ids)
}

func insertProjectLabels(c *app.Ctx, tx pgx.Tx, id uuid.UUID, labels []uuid.UUID) error {
	return app.Exec(c, tx,
		"INSERT INTO project_label_links (project_id, label_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING", id, labels)
}

// saveLinks adds links to a project or initiative (column is project_id or initiative_id); a
// link with the same URL gets the new title. Then it removes the links with the URLs in remove.
func saveLinks(c *app.Ctx, tx pgx.Tx, column string, id uuid.UUID, links []LinkArg, remove []string) error {
	for _, link := range links {
		url, err := nonEmpty(link.URL, "A link's url")
		if err != nil {
			return err
		}
		label := strings.TrimSpace(link.Title)
		if label == "" {
			label = url
		}
		tag, err := tx.Exec(c, fmt.Sprintf("UPDATE entity_links SET label = $3 WHERE %s = $1 AND url = $2", column), id, url, label)
		if err != nil {
			return err
		}
		if tag.RowsAffected() > 0 {
			continue
		}
		if err := app.Exec(c, tx, fmt.Sprintf(
			`INSERT INTO entity_links (workspace_id, %[1]s, url, label, creator_id, sort_order)
			 VALUES ($1, $2, $3, $4, $5, (SELECT coalesce(max(sort_order), 0) + 1 FROM entity_links WHERE %[1]s = $2))`, column),
			c.Workspace, id, url, label, c.User); err != nil {
			return err
		}
	}
	if len(remove) == 0 {
		return nil
	}
	urls := make([]string, len(remove))
	for i, url := range remove {
		urls[i] = strings.TrimSpace(url)
	}
	return app.Exec(c, tx, fmt.Sprintf("DELETE FROM entity_links WHERE %s = $1 AND url = ANY($2)", column), id, urls)
}

func projectInitiatives(c *app.Ctx, tx pgx.Tx, id uuid.UUID, set, add, remove []string) error {
	if set != nil && (add != nil || remove != nil) {
		return app.BadRequest("Pass either setInitiatives or addInitiatives/removeInitiatives, not both")
	}
	if (set != nil || add != nil || remove != nil) && c.Role == app.Guest {
		return app.Forbidden("Guests can't change initiatives")
	}
	if set != nil {
		ids, err := resolveInitiatives(c, set)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM initiative_projects WHERE project_id = $1 AND NOT (initiative_id = ANY($2))", id, ids); err != nil {
			return err
		}
		if err := linkInitiatives(c, tx, id, ids); err != nil {
			return err
		}
	}
	if add != nil {
		ids, err := resolveInitiatives(c, add)
		if err != nil {
			return err
		}
		if err := linkInitiatives(c, tx, id, ids); err != nil {
			return err
		}
	}
	if remove == nil {
		return nil
	}
	ids, err := resolveInitiatives(c, remove)
	if err != nil {
		return err
	}
	return app.Exec(c, tx, "DELETE FROM initiative_projects WHERE project_id = $1 AND initiative_id = ANY($2)", id, ids)
}

func linkInitiatives(c *app.Ctx, tx pgx.Tx, project uuid.UUID, initiatives []uuid.UUID) error {
	return app.Exec(c, tx,
		`INSERT INTO initiative_projects (initiative_id, project_id, sort_order)
		 SELECT i, $1, (SELECT coalesce(max(sort_order), 0) + 1 FROM initiative_projects WHERE initiative_id = i)
		 FROM unnest($2::uuid[]) AS i ON CONFLICT DO NOTHING`, project, initiatives)
}

func projectRelations(c *app.Ctx, tx pgx.Tx, id uuid.UUID, args SaveProjectArgs) error {
	type change struct {
		list    []string
		kind    string
		reverse bool
	}
	ends := func(other uuid.UUID, reverse bool) (uuid.UUID, uuid.UUID) {
		if reverse {
			return other, id
		}
		return id, other
	}
	for _, add := range []change{{args.Blocks, "blocks", false}, {args.BlockedBy, "blocks", true}, {args.RelatedTo, "related", false}} {
		for _, reference := range add.list {
			other, err := resolveProject(c, reference)
			if err != nil {
				return err
			}
			if other == id {
				return app.BadRequest("A project can't depend on or relate to itself")
			}
			from, to := ends(other, add.reverse)
			if err := app.Exec(c, tx,
				`INSERT INTO project_relations (project_id, related_project_id, type, creator_id)
				 SELECT $1, $2, $3, $4 WHERE NOT EXISTS (SELECT 1 FROM project_relations WHERE type = $3
					AND ((project_id = $1 AND related_project_id = $2) OR ($3 = 'related' AND project_id = $2 AND related_project_id = $1)))`,
				from, to, add.kind, c.User); err != nil {
				return err
			}
		}
	}
	for _, remove := range []change{{args.RemoveBlocks, "blocks", false}, {args.RemoveBlockedBy, "blocks", true}, {args.RemoveRelatedTo, "related", false}} {
		for _, reference := range remove.list {
			other, err := resolveProject(c, reference)
			if err != nil {
				return err
			}
			from, to := ends(other, remove.reverse)
			if err := app.Exec(c, tx,
				`DELETE FROM project_relations WHERE type = $3 AND ((project_id = $1 AND related_project_id = $2)
					OR ($3 = 'related' AND project_id = $2 AND related_project_id = $1))`,
				from, to, remove.kind); err != nil {
				return err
			}
		}
	}
	return nil
}

/* Trash and archive */

func markProject(c *app.Ctx, id uuid.UUID, sql, action string) (any, error) {
	var value M
	err := inTx(c, func(tx pgx.Tx) error {
		if err := app.Exec(c, tx, sql, id); err != nil {
			return err
		}
		var err error
		if value, err = projectValue(c, tx, id); err != nil {
			return err
		}
		team, err := webhookTeam(c, tx, id)
		if err != nil {
			return err
		}
		return emit(c, tx, "Project", action, value, nil, team)
	})
	return value, err
}

func trashProject(c *app.Ctx, args IDArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	id, err := resolveProject(c, args.ID)
	if err != nil {
		return nil, err
	}
	return markProject(c, id, "UPDATE projects SET trashed_at = now(), updated_at = now() WHERE id = $1 AND trashed_at IS NULL", "remove")
}

func restoreProject(c *app.Ctx, args IDArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	id, err := findTrashed(c, "projects", "project", args.ID)
	if err != nil {
		return nil, err
	}
	teams, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	visible, err := app.One[bool](c, c.DB(), fmt.Sprintf("SELECT %s FROM projects p WHERE p.id = $1", projectVisible("p", 2)), id, teams)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, app.NotFound("No project \"%s\" in the trash", strings.TrimSpace(args.ID))
	}
	return markProject(c, id, "UPDATE projects SET trashed_at = NULL, updated_at = now() WHERE id = $1", "update")
}

func archiveProject(c *app.Ctx, args IDArgs) (any, error) {
	id, err := resolveProject(c, args.ID)
	if err != nil {
		return nil, err
	}
	return markProject(c, id, "UPDATE projects SET archived_at = now(), updated_at = now() WHERE id = $1 AND archived_at IS NULL", "update")
}

func unarchiveProject(c *app.Ctx, args IDArgs) (any, error) {
	id, err := resolveProject(c, args.ID)
	if err != nil {
		return nil, err
	}
	return markProject(c, id, "UPDATE projects SET archived_at = NULL, updated_at = now() WHERE id = $1", "update")
}

// trashedSlugTail is the id a slug, link or path ends in: my-project-0a1b2c3d4e5f.
func trashedSlugTail(value string) string {
	value = strings.TrimSpace(value)
	if i := strings.IndexAny(value, "?#"); i >= 0 {
		value = value[:i]
	}
	segment := strings.TrimRight(value, "/")
	segment = segment[strings.LastIndex(segment, "/")+1:]
	i := strings.LastIndex(segment, "-")
	if i < 0 {
		return segment
	}
	if tail := segment[i+1:]; len(tail) == 12 && isHex(tail) {
		return tail
	}
	return segment
}

// findTrashed is a trashed project or initiative (table) by id, slug, link or name.
func findTrashed(c *app.Ctx, table, what, value string) (uuid.UUID, error) {
	id, found, err := app.Maybe[uuid.UUID](c, c.DB(), fmt.Sprintf(
		`SELECT id FROM %s WHERE workspace_id = $1 AND trashed_at IS NOT NULL
		 AND (id::text = $2 OR slug_id = $3 OR lower(name) = lower($2)) ORDER BY trashed_at DESC LIMIT 1`, table),
		c.Workspace, strings.TrimSpace(value), trashedSlugTail(value))
	if err != nil {
		return uuid.Nil, err
	}
	if !found {
		return uuid.Nil, app.NotFound("No %s \"%s\" in the trash", what, strings.TrimSpace(value))
	}
	return id, nil
}

/* Statuses */

const projectStatusJSON = `jsonb_build_object('id', s.id, 'name', s.name, 'type', s.type, 'color', s.color,
	'description', s.description, 'position', s.position,
	'projectCount', (SELECT count(*) FROM projects sp WHERE sp.status_id = s.id AND sp.trashed_at IS NULL),
	'archivedAt', s.archived_at, 'createdAt', s.created_at, 'updatedAt', s.updated_at)`

type ListProjectStatusesArgs struct {
	Limit  *int64  `json:"limit" doc:"How many to return, 50 by default, at most 250."`
	Cursor *string `json:"cursor" doc:"The endCursor of the previous page."`
}

func listProjectStatuses(c *app.Ctx, args ListProjectStatusesArgs) (any, error) {
	page := Page{Limit: args.Limit, Cursor: args.Cursor}
	offset, err := page.offset()
	if err != nil {
		return nil, err
	}
	nodes, err := app.All[any](c, c.DB(),
		`SELECT `+projectStatusJSON+` FROM project_statuses s WHERE s.workspace_id = $1 AND s.archived_at IS NULL
		 ORDER BY s.position, s.created_at LIMIT $2 OFFSET $3`, c.Workspace, page.limit()+1, offset)
	if err != nil {
		return nil, err
	}
	return page.wrap(nodes)
}

type SaveProjectStatusArgs struct {
	ID          *string          `json:"id" doc:"Only to change a status: its id or name. Omit to create one."`
	Name        *string          `json:"name"`
	Type        *string          `json:"type" doc:"backlog, planned, started, paused, completed or canceled. Required to create."`
	Color       *string          `json:"color" doc:"Hex color like #f2c94c."`
	Description Nullable[string] `json:"description" doc:"null clears it."`
	Position    *float64         `json:"position" doc:"Order among the statuses (lower comes first). New statuses go last."`
}

func checkProjectStatusType(value string) (string, error) {
	lower := strings.ToLower(strings.TrimSpace(value))
	if !slices.Contains(projectStatusTypes, lower) {
		return "", app.BadRequest("A project status type is backlog, planned, started, paused, completed or canceled, not \"%s\"", value)
	}
	return lower, nil
}

func colorOrPick(color *string, name string) (string, error) {
	if color == nil {
		return pickColor(name), nil
	}
	return checkColor(*color)
}

func saveProjectStatus(c *app.Ctx, args SaveProjectStatusArgs) (any, error) {
	if err := c.RequireAdmin(); err != nil {
		return nil, err
	}
	var value any
	err := inTx(c, func(tx pgx.Tx) error {
		var id uuid.UUID
		var err error
		if args.ID != nil {
			id, err = resolveProjectStatus(c, *args.ID)
		} else {
			id, err = createProjectStatus(c, tx, args)
		}
		if err != nil {
			return err
		}
		if args.Name != nil {
			name, err := nonEmpty(*args.Name, "name")
			if err != nil {
				return err
			}
			taken, err := app.One[bool](c, tx,
				`SELECT EXISTS (SELECT 1 FROM project_statuses WHERE workspace_id = $1 AND id <> $2
					AND lower(name) = lower($3) AND archived_at IS NULL)`, c.Workspace, id, name)
			if err != nil {
				return err
			}
			if taken {
				return app.Conflict("There is already a project status \"%s\"", name)
			}
			if err := app.Exec(c, tx, "UPDATE project_statuses SET name = $2 WHERE id = $1", id, name); err != nil {
				return err
			}
		}
		if args.Type != nil {
			kind, err := checkProjectStatusType(*args.Type)
			if err != nil {
				return err
			}
			if err := app.Exec(c, tx, "UPDATE project_statuses SET type = $2 WHERE id = $1", id, kind); err != nil {
				return err
			}
		}
		if args.Color != nil {
			color, err := checkColor(*args.Color)
			if err != nil {
				return err
			}
			if err := app.Exec(c, tx, "UPDATE project_statuses SET color = $2 WHERE id = $1", id, color); err != nil {
				return err
			}
		}
		if args.Description.Set {
			if err := app.Exec(c, tx, "UPDATE project_statuses SET description = $2 WHERE id = $1", id, args.Description.Ptr()); err != nil {
				return err
			}
		}
		if args.Position != nil {
			if err := app.Exec(c, tx, "UPDATE project_statuses SET position = $2 WHERE id = $1", id, *args.Position); err != nil {
				return err
			}
		}
		if err := app.Exec(c, tx, "UPDATE project_statuses SET updated_at = now() WHERE id = $1", id); err != nil {
			return err
		}
		value, err = app.One[any](c, tx, "SELECT "+projectStatusJSON+" FROM project_statuses s WHERE s.id = $1", id)
		return err
	})
	return value, err
}

func createProjectStatus(c *app.Ctx, tx pgx.Tx, args SaveProjectStatusArgs) (uuid.UUID, error) {
	name, err := nonEmpty(orEmpty(args.Name), "name")
	if err != nil {
		return uuid.Nil, err
	}
	if args.Type == nil {
		return uuid.Nil, app.BadRequest("type is required")
	}
	kind, err := checkProjectStatusType(*args.Type)
	if err != nil {
		return uuid.Nil, err
	}
	color, err := colorOrPick(args.Color, name)
	if err != nil {
		return uuid.Nil, err
	}
	return app.One[uuid.UUID](c, tx,
		`INSERT INTO project_statuses (workspace_id, name, type, color, position)
		 VALUES ($1, $2, $3, $4, (SELECT coalesce(max(position), 0) + 1 FROM project_statuses WHERE workspace_id = $1))
		 RETURNING id`, c.Workspace, name, kind, color)
}

type DeleteProjectStatusArgs struct {
	ID             string  `json:"id" doc:"The status's id or name."`
	MoveProjectsTo *string `json:"moveProjectsTo" doc:"The status (name, type or id) its projects move to; required when projects use it."`
}

func deleteProjectStatus(c *app.Ctx, args DeleteProjectStatusArgs) (any, error) {
	if err := c.RequireAdmin(); err != nil {
		return nil, err
	}
	id, err := resolveProjectStatus(c, args.ID)
	if err != nil {
		return nil, err
	}
	var projects []uuid.UUID
	err = inTx(c, func(tx pgx.Tx) error {
		remaining, err := app.One[int64](c, tx,
			"SELECT count(*) FROM project_statuses WHERE workspace_id = $1 AND id <> $2 AND archived_at IS NULL", c.Workspace, id)
		if err != nil {
			return err
		}
		if remaining == 0 {
			return app.BadRequest("The workspace keeps at least one project status")
		}
		if projects, err = app.All[uuid.UUID](c, tx, "SELECT id FROM projects WHERE status_id = $1", id); err != nil {
			return err
		}
		if len(projects) > 0 {
			if args.MoveProjectsTo == nil {
				return app.BadRequest("%d projects use this status; pass moveProjectsTo", len(projects))
			}
			target, err := resolveProjectStatus(c, *args.MoveProjectsTo)
			if err != nil {
				return err
			}
			if target == id {
				return app.BadRequest("moveProjectsTo must be another status")
			}
			if err := app.Exec(c, tx, "UPDATE projects SET status_id = $2, updated_at = now() WHERE status_id = $1", id, target); err != nil {
				return err
			}
			for _, project := range projects {
				if err := stampProjectStatus(c, tx, project); err != nil {
					return err
				}
			}
		}
		return app.Exec(c, tx, "DELETE FROM project_statuses WHERE id = $1", id)
	})
	if err != nil {
		return nil, err
	}
	return M{"deleted": true, "id": id, "movedProjects": len(projects)}, nil
}

/* Labels */

const projectLabelJSON = `jsonb_build_object('id', l.id, 'name', l.name, 'color', l.color,
	'description', l.description, 'isGroup', l.is_group,
	'parent', (SELECT jsonb_build_object('id', g.id, 'name', g.name) FROM project_labels g WHERE g.id = l.parent_id),
	'projectCount', (SELECT count(*) FROM project_label_links pl JOIN projects lp ON lp.id = pl.project_id
		WHERE pl.label_id = l.id AND lp.trashed_at IS NULL),
	'creator', user_ref(l.creator_id), 'retiredAt', l.retired_at, 'createdAt', l.created_at, 'updatedAt', l.updated_at)`

type ListProjectLabelsArgs struct {
	IncludeRetired bool    `json:"includeRetired,omitempty"`
	Query          *string `json:"query" doc:"Part of the name."`
	Limit          *int64  `json:"limit" doc:"How many to return, 50 by default, at most 250."`
	Cursor         *string `json:"cursor" doc:"The endCursor of the previous page."`
}

func listProjectLabels(c *app.Ctx, args ListProjectLabelsArgs) (any, error) {
	page := Page{Limit: args.Limit, Cursor: args.Cursor}
	var like *string
	if args.Query != nil {
		like = new("%" + escapeLike(strings.TrimSpace(*args.Query)) + "%")
	}
	offset, err := page.offset()
	if err != nil {
		return nil, err
	}
	nodes, err := app.All[any](c, c.DB(),
		`SELECT `+projectLabelJSON+` FROM project_labels l WHERE l.workspace_id = $1 AND ($2 OR l.retired_at IS NULL)
		 AND ($3::text IS NULL OR l.name ILIKE $3)
		 ORDER BY coalesce((SELECT g.name FROM project_labels g WHERE g.id = l.parent_id), l.name), l.parent_id IS NOT NULL, l.name
		 LIMIT $4 OFFSET $5`, c.Workspace, args.IncludeRetired, like, page.limit()+1, offset)
	if err != nil {
		return nil, err
	}
	return page.wrap(nodes)
}

// projectLabelID is a project label or group by id, name or Group/Name, retired ones included.
func projectLabelID(c *app.Ctx, value string) (uuid.UUID, error) {
	type row struct {
		ID     uuid.UUID
		Name   string
		Parent *string
	}
	rows, err := app.Rows[row](c, c.DB(),
		`SELECT l.id, l.name, g.name FROM project_labels l LEFT JOIN project_labels g ON g.id = l.parent_id
		 WHERE l.workspace_id = $1 ORDER BY l.retired_at IS NOT NULL, l.name`, c.Workspace)
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
	var matches []row
	for _, r := range rows {
		if strings.EqualFold(r.Name, name) && (group == nil || r.Parent != nil && strings.EqualFold(*r.Parent, *group)) {
			matches = append(matches, r)
		}
	}
	switch {
	case len(matches) == 1 || len(matches) > 1 && group != nil:
		return matches[0].ID, nil
	case len(matches) == 0:
		var names []string
		for _, r := range rows[:min(len(rows), 30)] {
			if r.Parent != nil {
				names = append(names, *r.Parent+"/"+r.Name)
			} else {
				names = append(names, r.Name)
			}
		}
		return uuid.Nil, app.BadRequest("No project label \"%s\". Options: %s.", value, strings.Join(names, ", "))
	}
	return uuid.Nil, app.BadRequest("\"%s\" matches labels in several groups; name it as Group/%s", value, name)
}

type SaveProjectLabelArgs struct {
	ID          *string          `json:"id" doc:"Only to change a label: its id, name or Group/Name. Omit to create one."`
	Name        *string          `json:"name"`
	Color       *string          `json:"color" doc:"Hex color like #bb87fc."`
	Description Nullable[string] `json:"description" doc:"null clears it."`
	IsGroup     *bool            `json:"isGroup" doc:"Whether this is a group holding other labels (projects can't have a group itself)."`
	Parent      Nullable[string] `json:"parent" doc:"The group (name or id) this label sits in; null takes it out."`
}

func projectLabelValue(c *app.Ctx, q app.Querier, id uuid.UUID) (any, error) {
	return app.One[any](c, q, "SELECT "+projectLabelJSON+" FROM project_labels l WHERE l.id = $1", id)
}

func saveProjectLabel(c *app.Ctx, args SaveProjectLabelArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	var existing *uuid.UUID
	if args.ID != nil {
		id, err := projectLabelID(c, *args.ID)
		if err != nil {
			return nil, err
		}
		existing = &id
	}
	var value any
	err := inTx(c, func(tx pgx.Tx) error {
		var id uuid.UUID
		if existing != nil {
			id = *existing
		} else {
			name, err := nonEmpty(orEmpty(args.Name), "name")
			if err != nil {
				return err
			}
			color, err := colorOrPick(args.Color, name)
			if err != nil {
				return err
			}
			if id, err = app.One[uuid.UUID](c, tx,
				"INSERT INTO project_labels (workspace_id, name, color, creator_id) VALUES ($1, $2, $3, $4) RETURNING id",
				c.Workspace, name, color, c.User); err != nil {
				return err
			}
		}
		if err := applyProjectLabelChanges(c, tx, id, args); err != nil {
			return err
		}
		var err error
		if value, err = projectLabelValue(c, tx, id); err != nil {
			return err
		}
		action := "create"
		if existing != nil {
			action = "update"
		}
		return emit(c, tx, "ProjectLabel", action, value, nil, nil)
	})
	return value, err
}

func applyProjectLabelChanges(c *app.Ctx, tx pgx.Tx, id uuid.UUID, args SaveProjectLabelArgs) error {
	if args.Name != nil {
		name, err := nonEmpty(*args.Name, "name")
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "UPDATE project_labels SET name = $2 WHERE id = $1", id, name); err != nil {
			return err
		}
	}
	if args.Color != nil {
		color, err := checkColor(*args.Color)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "UPDATE project_labels SET color = $2 WHERE id = $1", id, color); err != nil {
			return err
		}
	}
	if args.Description.Set {
		if err := app.Exec(c, tx, "UPDATE project_labels SET description = $2 WHERE id = $1", id, args.Description.Ptr()); err != nil {
			return err
		}
	}
	if args.IsGroup != nil {
		if err := app.Exec(c, tx, "UPDATE project_labels SET is_group = $2 WHERE id = $1", id, *args.IsGroup); err != nil {
			return err
		}
	}
	if args.Parent.Set {
		parent, err := optionalRef(c, args.Parent, projectLabelID)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "UPDATE project_labels SET parent_id = $2 WHERE id = $1", id, parent); err != nil {
			return err
		}
	}
	type shape struct {
		IsGroup       bool
		ParentIsGroup *bool
		HasChildren   bool
		InUse         bool
	}
	s, _, err := app.Row[shape](c, tx,
		`SELECT l.is_group, g.is_group,
			EXISTS (SELECT 1 FROM project_labels c WHERE c.parent_id = l.id),
			EXISTS (SELECT 1 FROM project_label_links pl WHERE pl.label_id = l.id)
		 FROM project_labels l LEFT JOIN project_labels g ON g.id = l.parent_id WHERE l.id = $1`, id)
	if err != nil {
		return err
	}
	switch {
	case s.ParentIsGroup != nil && !*s.ParentIsGroup:
		return app.BadRequest("A label's parent must be a group (a label with isGroup)")
	case s.IsGroup && s.ParentIsGroup != nil:
		return app.BadRequest("Groups don't nest; a group can't have a parent")
	case s.IsGroup && s.InUse:
		return app.BadRequest("Projects use this label, so it can't become a group")
	case !s.IsGroup && s.HasChildren:
		return app.BadRequest("This group has labels in it, so it must stay a group")
	}
	clash, err := app.One[bool](c, tx,
		`SELECT EXISTS (SELECT 1 FROM project_labels o JOIN project_labels l ON l.id = $1
			WHERE o.workspace_id = l.workspace_id AND o.id <> l.id AND lower(o.name) = lower(l.name)
			AND o.parent_id IS NOT DISTINCT FROM l.parent_id AND o.retired_at IS NULL)`, id)
	if err != nil {
		return err
	}
	if clash {
		return app.Conflict("There is already a project label with that name")
	}
	return app.Exec(c, tx, "UPDATE project_labels SET updated_at = now() WHERE id = $1", id)
}

func markProjectLabel(c *app.Ctx, reference, sql string) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	id, err := projectLabelID(c, reference)
	if err != nil {
		return nil, err
	}
	var value any
	err = inTx(c, func(tx pgx.Tx) error {
		if err := app.Exec(c, tx, sql, id); err != nil {
			return err
		}
		var err error
		if value, err = projectLabelValue(c, tx, id); err != nil {
			return err
		}
		return emit(c, tx, "ProjectLabel", "update", value, nil, nil)
	})
	return value, err
}

func retireProjectLabel(c *app.Ctx, args IDArgs) (any, error) {
	return markProjectLabel(c, args.ID, `UPDATE project_labels SET retired_at = now(), updated_at = now()
		WHERE (id = $1 OR parent_id = $1) AND retired_at IS NULL`)
}

func restoreProjectLabel(c *app.Ctx, args IDArgs) (any, error) {
	return markProjectLabel(c, args.ID, "UPDATE project_labels SET retired_at = NULL, updated_at = now() WHERE id = $1 OR parent_id = $1")
}

func deleteProjectLabel(c *app.Ctx, args IDArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	id, err := projectLabelID(c, args.ID)
	if err != nil {
		return nil, err
	}
	err = inTx(c, func(tx pgx.Tx) error {
		value, err := projectLabelValue(c, tx, id)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM project_labels WHERE id = $1", id); err != nil {
			return err
		}
		return emit(c, tx, "ProjectLabel", "remove", value, nil, nil)
	})
	if err != nil {
		return nil, err
	}
	return M{"deleted": true, "id": id}, nil
}

/* Result shapes */

// ProjectIssueStats is what project_issue_stats adds to projects and milestones.
type ProjectIssueStats struct {
	IssueCount          int     `json:"issueCount"`
	CompletedIssueCount int     `json:"completedIssueCount"`
	CanceledIssueCount  int     `json:"canceledIssueCount"`
	Progress            float64 `json:"progress" doc:"Completed issues / (all - canceled), from 0 to 1."`
}

type Project struct {
	ID                   string            `json:"id"`
	SlugID               string            `json:"slugId"`
	Name                 string            `json:"name"`
	Summary              *string           `json:"summary"`
	Description          string            `json:"description,omitempty" doc:"Markdown; in lists only with includeDescription."`
	URL                  string            `json:"url"`
	Icon                 *string           `json:"icon"`
	Color                *string           `json:"color"`
	Status               ProjectStatusRef  `json:"status"`
	Priority             int               `json:"priority" doc:"0 none, 1 urgent, 2 high, 3 medium, 4 low."`
	PriorityLabel        string            `json:"priorityLabel"`
	Health               *string           `json:"health" enum:"onTrack,atRisk,offTrack"`
	Lead                 *UserRef          `json:"lead"`
	LeadTeam             *TeamRef          `json:"leadTeam"`
	Teams                []TeamRef         `json:"teams"`
	Labels               []ProjectLabelRef `json:"labels"`
	Initiatives          []InitiativeRef   `json:"initiatives"`
	StartDate            *string           `json:"startDate"`
	StartDateResolution  *string           `json:"startDateResolution" enum:"month,quarter,halfYear,year"`
	TargetDate           *string           `json:"targetDate"`
	TargetDateResolution *string           `json:"targetDateResolution" enum:"month,quarter,halfYear,year"`
	SortOrder            float64           `json:"sortOrder"`
	StartedAt            *string           `json:"startedAt"`
	CompletedAt          *string           `json:"completedAt"`
	CanceledAt           *string           `json:"canceledAt"`
	ArchivedAt           *string           `json:"archivedAt"`
	Trashed              bool              `json:"trashed"`
	TrashedAt            *string           `json:"trashedAt"`
	CreatedAt            string            `json:"createdAt"`
	UpdatedAt            string            `json:"updatedAt"`
	ProjectIssueStats
}

type ProjectDetail struct {
	Project
	Creator               *UserRef         `json:"creator"`
	Members               []UserRef        `json:"members"`
	Subscribers           []UserRef        `json:"subscribers"`
	Links                 []EntityLink     `json:"links"`
	Relations             ProjectRelations `json:"relations"`
	Documents             []DocumentRef    `json:"documents"`
	IssueCountByStateType map[string]int   `json:"issueCountByStateType" doc:"Issue counts by status type; types without issues are left out."`
	Estimate              ProjectEstimate  `json:"estimate"`
	Milestones            []Milestone      `json:"milestones"`
	LatestUpdate          *StatusUpdate    `json:"latestUpdate"`
}

// EntityLink is a link (resource) on a project or initiative.
type EntityLink struct {
	ID        string   `json:"id"`
	URL       string   `json:"url"`
	Label     string   `json:"label"`
	Creator   *UserRef `json:"creator"`
	CreatedAt string   `json:"createdAt"`
}

type ProjectRelations struct {
	Blocks    []ProjectRef `json:"blocks"`
	BlockedBy []ProjectRef `json:"blockedBy"`
	Related   []ProjectRef `json:"related"`
}

type ProjectEstimate struct {
	Total     float64 `json:"total" doc:"The sum of the estimates of the project's issues, canceled ones left out."`
	Completed float64 `json:"completed"`
}

type ProjectStatus struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Type         string  `json:"type" enum:"backlog,planned,started,paused,completed,canceled"`
	Color        string  `json:"color"`
	Description  *string `json:"description"`
	Position     float64 `json:"position"`
	ProjectCount int     `json:"projectCount"`
	ArchivedAt   *string `json:"archivedAt"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
}

type DeletedProjectStatus struct {
	Deleted       bool   `json:"deleted"`
	ID            string `json:"id"`
	MovedProjects int    `json:"movedProjects"`
}

type ProjectLabel struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Color        string             `json:"color"`
	Description  *string            `json:"description"`
	IsGroup      bool               `json:"isGroup"`
	Parent       *ProjectLabelGroup `json:"parent"`
	ProjectCount int                `json:"projectCount"`
	Creator      *UserRef           `json:"creator"`
	RetiredAt    *string            `json:"retiredAt"`
	CreatedAt    string             `json:"createdAt"`
	UpdatedAt    string             `json:"updatedAt"`
}

type ProjectLabelGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DeletedProjectLabel struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
}
