package ops

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
)

// Issues: listing with filters, reading, creating and changing them (with the workflow
// automations that come with a change), trash, archive, history and sharing.

func issueOps() []*Op {
	return []*Op{
		op("list_issues", app.Read, []string{"GET /issues"},
			"List issues, newest update first. Narrow with the simple arguments (team, assignee \"me\", state, label, "+
				"project, cycle, ...) and/or a `filter` object for anything more (and/or, comparators, relations). Archived "+
				"and trashed issues are left out unless asked for. Descriptions are left out unless includeDescription is set.",
			listIssues),
		op("get_issue", app.Read, []string{"GET /issues/{id}"},
			"Read an issue by identifier (ENG-123), id or link: its fields, description, sub-issues, relations "+
				"(blocks, blocked by, related, duplicates), attachments, subscribers and releases. Set includeComments or "+
				"includeHistory to also get its comment threads or change history.",
			getIssue),
		op("save_issue", app.Write, []string{"POST /issues", "PATCH /issues/{id}"},
			"Create an issue (omit id; team and title are required unless a template or parent provides them) or update "+
				"one (pass id). Omitted fields stay as they are; null clears a field. People are given as id, name, email or "+
				"\"me\"; states as name, type or id; labels, projects, cycles and milestones as name or id. Use patch to edit "+
				"part of the description without resending it. Moving an issue to another team gives it a new identifier; "+
				"the old one keeps working. Returns the issue as get_issue does, with relations and sub-issues.",
			saveIssue),
		op("update_issues", app.Write, []string{"PATCH /issues"},
			"Apply the same change to several issues at once, e.g. move them to a state, assign them, add a label or put "+
				"them in a cycle. `changes` takes the fields of save_issue (without id). Stops at the first issue that fails; "+
				"issues before it keep their change.",
			updateIssues),
		op("delete_issue", app.Write, []string{"DELETE /issues/{id}"},
			"Move an issue to the trash. It disappears from lists and is deleted for good after 30 days; restore_issue "+
				"brings it back before then.",
			trashIssue),
		op("restore_issue", app.Write, []string{"POST /issues/{id}/restore"}, "Bring an issue back from the trash.", restoreIssue),
		op("archive_issue", app.Write, []string{"POST /issues/{id}/archive"},
			"Archive an issue (and its sub-issues). Archived issues are kept and readable but left out of lists unless "+
				"includeArchived is set. Closed issues are archived automatically after the team's auto-archive period.",
			archiveIssue),
		op("unarchive_issue", app.Write, []string{"POST /issues/{id}/unarchive"}, "Bring an archived issue back.", unarchiveIssue),
		op("get_issue_history", app.Read, []string{"GET /issues/{id}/history"},
			"The change history of an issue, oldest first: who changed which field from what to what, and when.",
			getIssueHistory),
		op("share_issue", app.Write, []string{"POST /issues/{id}/shares"},
			"Give a workspace member who can't see the issue's team (a guest, or someone outside a private team) access "+
				"to this one issue. Only when the user asks to share it.",
			shareIssue),
		op("unshare_issue", app.Write, []string{"DELETE /issues/{id}/shares/{user}"},
			"Take back access to an issue given with share_issue.", unshareIssue),
	}
}

/* Representation */

const issueFrom = "FROM issues i JOIN teams t ON t.id = i.team_id JOIN workspaces w ON w.id = i.workspace_id"

func sqlLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

// issueJSON is the JSON of the issue i (with its team t and workspace w joined).
func issueJSON(c *app.Ctx, withDescription bool) string {
	base := sqlLiteral(c.PublicURL())
	description := ""
	if withDescription {
		description = "'description', i.description,"
	}
	return fmt.Sprintf(`(jsonb_build_object(
		'id', i.id,
		'identifier', t.key || '-' || i.number,
		'number', i.number,
		'title', i.title,
		%s
		'url', %s || '/' || w.url_key || '/issue/' || t.key || '-' || i.number,
		'branchName', lower(t.key) || '-' || i.number || '-' ||
			left(trim(both '-' from regexp_replace(lower(i.title), '[^a-z0-9]+', '-', 'g')), 40),
		'priority', i.priority,
		'priorityLabel', (ARRAY['No priority','Urgent','High','Medium','Low'])[i.priority + 1],
		'estimate', i.estimate,
		'state', state_ref(i.state_id),
		'team', jsonb_build_object('id', t.id, 'key', t.key, 'name', t.name),
		'assignee', user_ref(i.assignee_id),
		'creator', user_ref(i.creator_id),
		'labels', issue_label_refs(i.id),
		'project', project_ref(i.project_id),
		'projectMilestone', milestone_ref(i.milestone_id),
		'cycle', cycle_ref(i.cycle_id),
		'parent', issue_ref(i.parent_id),
		'dueDate', i.due_date
	) || jsonb_build_object(
		'sortOrder', i.sort_order,
		'subIssueSortOrder', i.sub_issue_sort_order,
		'subIssueCount', (SELECT count(*) FROM issues c WHERE c.parent_id = i.id AND c.trashed_at IS NULL),
		'slaType', i.sla_type,
		'slaStartedAt', i.sla_started_at,
		'slaBreachesAt', i.sla_breaches_at,
		'snoozedUntil', i.snoozed_until,
		'startedAt', i.started_at,
		'completedAt', i.completed_at,
		'canceledAt', i.canceled_at,
		'startedTriageAt', i.started_triage_at,
		'triagedAt', i.triaged_at,
		'autoClosedAt', i.auto_closed_at,
		'autoArchivedAt', i.auto_archived_at,
		'archivedAt', i.archived_at,
		'trashed', i.trashed_at IS NOT NULL,
		'trashedAt', i.trashed_at,
		'previousIdentifiers', to_jsonb(i.previous_identifiers),
		'createdAt', i.created_at,
		'updatedAt', i.updated_at
	))`, description, base)
}

// issueValue is one issue's JSON, with its description.
func issueValue(c *app.Ctx, q app.Querier, id uuid.UUID) (M, error) {
	value, found, err := app.Maybe[M](c, q, fmt.Sprintf("SELECT %s %s WHERE i.id = $1", issueJSON(c, true), issueFrom), id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, app.NotFound("No such issue")
	}
	return value, nil
}

/* List */

type ListIssuesArgs struct {
	Team               *string `json:"team" doc:"Team key, name or id."`
	IncludeSubTeams    bool    `json:"includeSubTeams,omitempty" doc:"With team: also issues of its sub-teams."`
	Assignee           *string `json:"assignee" doc:"Person (id, name, email or \"me\"), or \"null\" for unassigned issues."`
	Creator            *string `json:"creator" doc:"Person who created the issues (id, name, email or \"me\")."`
	Subscriber         *string `json:"subscriber" doc:"Person subscribed to the issues (id, name, email or \"me\")."`
	State              *string `json:"state" doc:"Status name, type (triage, backlog, unstarted, started, completed, canceled) or id."`
	Label              *string `json:"label" doc:"Label name or id."`
	Project            *string `json:"project" doc:"Project name, slug or id, or \"null\" for issues in no project."`
	Milestone          *string `json:"milestone" doc:"Project milestone name or id (name needs project)."`
	Cycle              *string `json:"cycle" doc:"Cycle number, name or id, or \"current\", \"next\", \"previous\"."`
	Release            *string `json:"release" doc:"Release name, slug or id."`
	ParentID           *string `json:"parentId" doc:"Sub-issues of this issue (identifier or id)."`
	Priority           *int    `json:"priority" doc:"0 none, 1 urgent, 2 high, 3 medium, 4 low."`
	Query              *string `json:"query" doc:"Words to find in the title, description or comments."`
	CreatedAt          *string `json:"createdAt" doc:"Created after: ISO 8601 date/time or a duration like -P1D."`
	UpdatedAt          *string `json:"updatedAt" doc:"Updated after: ISO 8601 date/time or a duration like -P1W."`
	DueBefore          *string `json:"dueBefore" doc:"Due on or before this date (or duration from now, e.g. P1W)."`
	CustomView         *string `json:"customView" doc:"A saved custom view (name, slug or id) whose filter applies too."`
	Filter             any     `json:"filter" doc:"Any further conditions. See the filter help in the description of this field: Fields: id, number, title, description, priority, estimate, dueDate, createdAt, updatedAt, startedAt, completedAt, canceledAt, archivedAt, triagedAt, snoozedUntil, slaBreachesAt, state {id,name,type}, team {id,key,name}, assignee / creator {id,email,name,displayName,isMe}, labels {some|every|none: {id,name,parent}}, project {id,name,slugId}, projectMilestone {id,name}, cycle {id,number,name,isActive}, parent {id,identifier}, subscribers {some: user}, hasBlockedByRelations, hasBlockingRelations, hasRelatedRelations, hasDuplicateRelations, children {some: filter}, searchableContent {contains}, and, or. Comparators: eq, neq, in, nin, lt, lte, gt, gte, contains, notContains, startsWith, endsWith, null. Example: {\"or\": [{\"priority\": {\"lte\": 2, \"neq\": 0}}, {\"labels\": {\"some\": {\"name\": {\"eq\": \"Bug\"}}}}]}"`
	IncludeArchived    bool    `json:"includeArchived,omitempty"`
	IncludeTrashed     bool    `json:"includeTrashed,omitempty"`
	IncludeDescription bool    `json:"includeDescription,omitempty" doc:"Include each issue's description."`
	OrderBy            *string `json:"orderBy" doc:"updatedAt (default), createdAt, priority, dueDate, manual (the team's board order) or identifier."`
	Page
}

func eq(value any) M {
	return M{"eq": value}
}

func isNullWord(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "null", "none", "unassigned", "no project":
		return true
	}
	return false
}

func simpleFilters(c *app.Ctx, args ListIssuesArgs) ([]any, error) {
	var parts []any
	var team *uuid.UUID
	if args.Team != nil {
		id, err := resolveTeam(c, *args.Team)
		if err != nil {
			return nil, err
		}
		team = &id
		if args.IncludeSubTeams {
			ids, err := app.All[uuid.UUID](c, c.DB(),
				`WITH RECURSIVE tree AS (SELECT id FROM teams WHERE id = $1
				 UNION SELECT t.id FROM teams t JOIN tree ON t.parent_id = tree.id) SELECT id FROM tree`, id)
			if err != nil {
				return nil, err
			}
			list := make([]any, len(ids))
			for i, id := range ids {
				list[i] = id.String()
			}
			parts = append(parts, M{"team": M{"id": M{"in": list}}})
		} else {
			parts = append(parts, M{"team": M{"id": eq(id.String())}})
		}
	}
	userPart := func(value *string, wrap func(M) M) error {
		if value == nil {
			return nil
		}
		user, err := resolveUser(c, *value)
		if err != nil {
			return err
		}
		parts = append(parts, wrap(M{"id": eq(user.String())}))
		return nil
	}
	if args.Assignee != nil && isNullWord(*args.Assignee) {
		parts = append(parts, M{"assignee": M{"null": true}})
	} else if err := userPart(args.Assignee, func(f M) M { return M{"assignee": f} }); err != nil {
		return nil, err
	}
	if err := userPart(args.Creator, func(f M) M { return M{"creator": f} }); err != nil {
		return nil, err
	}
	if err := userPart(args.Subscriber, func(f M) M { return M{"subscribers": M{"some": f}} }); err != nil {
		return nil, err
	}
	if args.State != nil {
		value := *args.State
		id, isID := asUUID(value)
		switch {
		case team != nil:
			state, err := resolveState(c, *team, value)
			if err != nil {
				return nil, err
			}
			parts = append(parts, M{"state": M{"id": eq(state.String())}})
		case isID:
			parts = append(parts, M{"state": M{"id": eq(id.String())}})
		default:
			parts = append(parts, M{"or": []any{M{"state": M{"name": eq(value)}}, M{"state": M{"type": eq(value)}}}})
		}
	}
	if args.Label != nil {
		value := *args.Label
		if id, ok := asUUID(value); ok {
			parts = append(parts, M{"labels": M{"some": M{"id": eq(id.String())}}})
		} else {
			if _, err := resolveLabels(c, team, []string{value}); err != nil {
				return nil, err
			}
			group, name := splitGroup(value)
			if group != nil {
				parts = append(parts, M{"labels": M{"some": M{"name": eq(name), "parent": M{"name": eq(*group)}}}})
			} else {
				parts = append(parts, M{"labels": M{"some": M{"name": eq(name)}}})
			}
		}
	}
	var project *uuid.UUID
	if args.Project != nil {
		if isNullWord(*args.Project) {
			parts = append(parts, M{"project": M{"null": true}})
		} else {
			id, err := resolveProject(c, *args.Project)
			if err != nil {
				return nil, err
			}
			project = &id
			parts = append(parts, M{"project": M{"id": eq(id.String())}})
		}
	}
	if args.Milestone != nil {
		id, err := resolveMilestone(c, project, *args.Milestone)
		if err != nil {
			return nil, err
		}
		parts = append(parts, M{"projectMilestone": M{"id": eq(id.String())}})
	}
	if args.Cycle != nil {
		value := strings.TrimSpace(*args.Cycle)
		lower := strings.ToLower(value)
		_, isID := asUUID(value)
		number, numErr := strconv.ParseInt(value, 10, 64)
		switch {
		case team != nil:
			id, err := resolveCycle(c, team, value)
			if err != nil {
				return nil, err
			}
			parts = append(parts, M{"cycle": M{"id": eq(id.String())}})
		case lower == "current" || lower == "active":
			parts = append(parts, M{"cycle": M{"isActive": eq(true)}})
		case isID:
			parts = append(parts, M{"cycle": M{"id": eq(value)}})
		case numErr == nil:
			parts = append(parts, M{"cycle": M{"number": eq(float64(number))}})
		default:
			parts = append(parts, M{"cycle": M{"name": eq(*args.Cycle)}})
		}
	}
	if args.Release != nil {
		id, err := resolveRelease(c, *args.Release)
		if err != nil {
			return nil, err
		}
		parts = append(parts, M{"releases": M{"some": M{"id": eq(id.String())}}})
	}
	if args.ParentID != nil {
		id, err := resolveIssue(c, *args.ParentID)
		if err != nil {
			return nil, err
		}
		parts = append(parts, M{"parent": M{"id": eq(id.String())}})
	}
	if args.Priority != nil {
		priority, err := checkPriority(*args.Priority)
		if err != nil {
			return nil, err
		}
		parts = append(parts, M{"priority": eq(float64(priority))})
	}
	for _, f := range []struct {
		field string
		value *string
		op    string
	}{{"createdAt", args.CreatedAt, "gte"}, {"updatedAt", args.UpdatedAt, "gte"}, {"dueDate", args.DueBefore, "lte"}} {
		if f.value == nil {
			continue
		}
		at, err := parseDatetime(*f.value)
		if err != nil {
			return nil, err
		}
		parts = append(parts, M{f.field: M{f.op: at.Format(time.RFC3339Nano)}})
	}
	return parts, nil
}

func listIssues(c *app.Ctx, args ListIssuesArgs) (any, error) {
	parts, err := simpleFilters(c, args)
	if err != nil {
		return nil, err
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
		if v.Model != "issue" {
			return nil, app.BadRequest("That view shows %ss, not issues", v.Model)
		}
		parts = append(parts, v.Filter)
		if v.Team != nil {
			parts = append(parts, M{"team": M{"id": eq(v.Team.String())}})
		}
	}
	if args.Filter != nil {
		parts = append(parts, args.Filter)
	}
	orders := map[string]string{
		"updatedAt":  "i.updated_at DESC",
		"createdAt":  "i.created_at DESC",
		"priority":   "CASE WHEN i.priority = 0 THEN 5 ELSE i.priority END, i.updated_at DESC",
		"dueDate":    "i.due_date ASC NULLS LAST, i.priority",
		"manual":     "i.sort_order ASC",
		"identifier": "t.key, i.number",
		"number":     "t.key, i.number",
	}
	orderBy := "updatedAt"
	if args.OrderBy != nil {
		orderBy = *args.OrderBy
	}
	order, ok := orders[orderBy]
	if !ok {
		return nil, app.BadRequest("orderBy %q isn't one of updatedAt, createdAt, priority, dueDate, manual, identifier", orderBy)
	}
	q, err := visibleIssues(c, issueJSON(c, args.IncludeDescription))
	if err != nil {
		return nil, err
	}
	if !args.IncludeArchived {
		q.Push(" AND i.archived_at IS NULL")
	}
	if !args.IncludeTrashed {
		q.Push(" AND i.trashed_at IS NULL")
	}
	if query := strings.TrimSpace(orEmpty(args.Query)); query != "" {
		if key, number, ok := parseIdentifier(query); ok {
			q.Push(" AND ((upper(t.key) = ").Bind(key).Push(" AND i.number = ").Bind(number).Push(") OR ")
			pushSearch(q, "i", query)
			q.Push(")")
		} else {
			q.Push(" AND ")
			pushSearch(q, "i", query)
		}
	}
	if err := pushFilter(q, c, "i", M{"and": parts}); err != nil {
		return nil, err
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	q.Push(" ORDER BY " + order + " LIMIT ").Bind(args.limit() + 1).Push(" OFFSET ").Bind(offset)
	nodes, err := app.All[any](c, c.DB(), q.String(), q.Args...)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

// visibleIssues is SELECT <columns> FROM issues ... WHERE <visible to the caller>, to add
// conditions to.
func visibleIssues(c *app.Ctx, columns string) (*app.SQL, error) {
	teams, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	q := app.NewSQL(fmt.Sprintf("SELECT %s %s WHERE i.workspace_id = ", columns, issueFrom)).Bind(c.Workspace)
	q.Push(" AND (i.team_id = ANY(").Bind(teams)
	q.Push(") OR EXISTS (SELECT 1 FROM issue_shares sh WHERE sh.issue_id = i.id AND sh.user_id = ").Bind(c.User).Push("))")
	return q, nil
}

/* Read */

type GetIssueArgs struct {
	ID              string `json:"id" doc:"Identifier (ENG-123), id or link."`
	IncludeComments bool   `json:"includeComments,omitempty" doc:"Also return the comment threads."`
	IncludeHistory  bool   `json:"includeHistory,omitempty" doc:"Also return the change history."`
}

func getIssue(c *app.Ctx, args GetIssueArgs) (any, error) {
	id, err := resolveIssue(c, args.ID)
	if err != nil {
		return nil, err
	}
	return issueDetail(c, id, args.IncludeComments, args.IncludeHistory)
}

func issueDetail(c *app.Ctx, id uuid.UUID, withComments, withHistory bool) (M, error) {
	db := c.DB()
	value, err := issueValue(c, db, id)
	if err != nil {
		return nil, err
	}
	teams, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	children, err := app.All[any](c, db,
		`SELECT jsonb_build_object('id', c.id, 'identifier', issue_identifier(c.id), 'title', c.title,
			'state', state_ref(c.state_id), 'assignee', user_ref(c.assignee_id), 'priority', c.priority)
		 FROM issues c WHERE c.parent_id = $1 AND c.trashed_at IS NULL AND c.team_id = ANY($2)
		 ORDER BY c.sub_issue_sort_order NULLS LAST, c.created_at`, id, teams)
	if err != nil {
		return nil, err
	}
	type relation struct {
		Type     string
		Outgoing bool
		Issue    any
		ID       uuid.UUID
	}
	relations, err := app.Rows[relation](c, db,
		`SELECT r.type, r.issue_id = $1, issue_ref(CASE WHEN r.issue_id = $1 THEN r.related_issue_id ELSE r.issue_id END), r.id
		 FROM issue_relations r WHERE r.issue_id = $1 OR r.related_issue_id = $1 ORDER BY r.created_at`, id)
	if err != nil {
		return nil, err
	}
	pick := func(kind string, outgoing *bool) []any {
		list := []any{}
		for _, r := range relations {
			if r.Type == kind && (outgoing == nil || *outgoing == r.Outgoing) {
				list = append(list, r.Issue)
			}
		}
		return list
	}
	var duplicateOf any
	if list := pick("duplicate", new(true)); len(list) > 0 {
		duplicateOf = list[0]
	}
	value["children"] = nonNil(children)
	value["relations"] = M{
		"blocks":      pick("blocks", new(true)),
		"blockedBy":   pick("blocks", new(false)),
		"related":     pick("related", nil),
		"duplicateOf": duplicateOf,
		"duplicates":  pick("duplicate", new(false)),
	}
	extras := []struct{ key, sql string }{
		{"attachments", `SELECT coalesce(jsonb_agg(jsonb_build_object('id', a.id, 'title', a.title, 'subtitle', a.subtitle, 'url', a.url,
			'sourceType', a.source_type, 'metadata', a.metadata, 'creator', user_ref(a.creator_id), 'createdAt', a.created_at)
			ORDER BY a.created_at), '[]') FROM attachments a WHERE a.issue_id = $1`},
		{"subscribers", "SELECT coalesce(jsonb_agg(user_ref(s.user_id)), '[]') FROM issue_subscribers s WHERE s.issue_id = $1"},
		{"releases", "SELECT coalesce(jsonb_agg(release_ref(r.release_id)), '[]') FROM issue_releases r WHERE r.issue_id = $1"},
		{"documents", "SELECT coalesce(jsonb_agg(document_ref(d.id)), '[]') FROM documents d WHERE d.issue_id = $1 AND d.trashed_at IS NULL"},
		{"sharedWith", "SELECT coalesce(jsonb_agg(user_ref(s.user_id)), '[]') FROM issue_shares s WHERE s.issue_id = $1"},
		{"reactions", `SELECT coalesce(jsonb_agg(jsonb_build_object('emoji', r.emoji, 'user', user_ref(r.user_id))), '[]')
			FROM reactions r WHERE r.issue_id = $1`},
		{"commentCount", "SELECT to_jsonb(count(*)) FROM comments WHERE issue_id = $1"},
	}
	for _, extra := range extras {
		if value[extra.key], err = app.One[any](c, db, extra.sql, id); err != nil {
			return nil, err
		}
	}
	if withComments {
		if value["comments"], err = commentThreads(c, db, "issue_id", id); err != nil {
			return nil, err
		}
	}
	if withHistory {
		if value["history"], err = historyValue(c, db, id); err != nil {
			return nil, err
		}
	}
	return value, nil
}

func nonNil(list []any) []any {
	if list == nil {
		return []any{}
	}
	return list
}

func historyValue(c *app.Ctx, q app.Querier, issue uuid.UUID) (any, error) {
	return app.One[any](c, q,
		`SELECT coalesce(jsonb_agg(jsonb_build_object('action', h.action, 'actor', user_ref(h.actor_id),
			'changes', h.changes, 'createdAt', h.created_at) ORDER BY h.id), '[]')
		 FROM issue_history h WHERE h.issue_id = $1`, issue)
}

func getIssueHistory(c *app.Ctx, args IDArgs) (any, error) {
	id, err := resolveIssue(c, args.ID)
	if err != nil {
		return nil, err
	}
	ident, err := issueIdentifier(c, c.DB(), id)
	if err != nil {
		return nil, err
	}
	hist, err := historyValue(c, c.DB(), id)
	if err != nil {
		return nil, err
	}
	return M{"identifier": ident, "history": hist}, nil
}

func issueIdentifier(c *app.Ctx, q app.Querier, issue uuid.UUID) (string, error) {
	return app.One[string](c, q, "SELECT issue_identifier($1)", issue)
}

/* Save */

type LinkArg struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type SaveIssueArgs struct {
	ID                *string           `json:"id" doc:"Only to update an issue: its identifier (ENG-123), id or link. Omit to create one."`
	Team              *string           `json:"team" doc:"Team key, name or id. Required to create (unless parentId or the template sets it); changing it moves the issue."`
	Title             *string           `json:"title"`
	Description       *string           `json:"description" doc:"Markdown. Mention people with @displayName or @email; they are notified and subscribed."`
	Patch             []TextEdit        `json:"patch" doc:"Edits to the current description instead of the whole text, applied in order, all or none."`
	State             *string           `json:"state" doc:"Status name, type (backlog, unstarted, started, completed, canceled, triage) or id."`
	Priority          *int              `json:"priority" doc:"1 urgent, 2 high, 3 medium, 4 low; 0 means no priority (not the highest)."`
	Estimate          Nullable[float64] `json:"estimate" doc:"Estimate points on the team's scale; null clears it. T-shirt sizes are XS=1, S=2, M=3, L=5, XL=8, XXL=13, XXXL=21."`
	Assignee          Nullable[string]  `json:"assignee" doc:"Person (id, name, email or \"me\"); null unassigns."`
	Labels            []string          `json:"labels" doc:"Replace all labels with these (names or ids; Group/Child for a label in a group)."`
	AddLabels         []string          `json:"addLabels"`
	RemoveLabels      []string          `json:"removeLabels"`
	Project           Nullable[string]  `json:"project" doc:"Project name, slug or id; null removes the issue from its project."`
	Milestone         Nullable[string]  `json:"milestone" doc:"Project milestone name or id; null clears it."`
	Cycle             Nullable[string]  `json:"cycle" doc:"Cycle number, name or id, or \"current\" / \"next\"; null removes it from its cycle."`
	ParentID          Nullable[string]  `json:"parentId" doc:"Parent issue (identifier or id), making this a sub-issue; null detaches it."`
	DueDate           Nullable[string]  `json:"dueDate" doc:"YYYY-MM-DD; null clears it."`
	Blocks            []string          `json:"blocks" doc:"Issues this one blocks (added to existing relations)."`
	BlockedBy         []string          `json:"blockedBy" doc:"Issues blocking this one (added to existing relations)."`
	RelatedTo         []string          `json:"relatedTo" doc:"Issues related to this one (added)."`
	RemoveBlocks      []string          `json:"removeBlocks"`
	RemoveBlockedBy   []string          `json:"removeBlockedBy"`
	RemoveRelatedTo   []string          `json:"removeRelatedTo"`
	DuplicateOf       Nullable[string]  `json:"duplicateOf" doc:"The issue this duplicates; null removes the duplicate relation. Marking a duplicate moves it to a canceled status."`
	Links             []LinkArg         `json:"links" doc:"Links to attach ({url, title}); a link already on the issue is updated."`
	AddSubscribers    []string          `json:"addSubscribers"`
	RemoveSubscribers []string          `json:"removeSubscribers"`
	AddReleases       []string          `json:"addReleases" doc:"Release names, slugs or ids to add."`
	RemoveReleases    []string          `json:"removeReleases"`
	SetReleases       []string          `json:"setReleases" doc:"Replace all releases with these."`
	SnoozedUntil      Nullable[string]  `json:"snoozedUntil" doc:"Hide the issue from triage / inbox until this time (ISO 8601 or duration like P1W); null wakes it."`
	SLABreachesAt     Nullable[string]  `json:"slaBreachesAt" doc:"When the SLA breaches (ISO 8601); null removes the SLA."`
	SLAType           *string           `json:"slaType" doc:"\"all\" or \"onlyBusinessDays\"."`
	SortOrder         *float64          `json:"sortOrder" doc:"Position on the team's board (lower is higher up)."`
	SubIssueSortOrder *float64          `json:"subIssueSortOrder" doc:"Position among its parent's sub-issues."`
	Template          *string           `json:"template" doc:"Issue template name or id, applied on create: it fills in fields and sub-issues the arguments don't set."`
}

type issueRow struct {
	ID                uuid.UUID
	TeamID            uuid.UUID
	Number            int
	Title             string
	Description       string
	StateID           uuid.UUID
	Priority          int
	Estimate          *float64
	AssigneeID        *uuid.UUID
	CreatorID         *uuid.UUID
	ProjectID         *uuid.UUID
	MilestoneID       *uuid.UUID
	CycleID           *uuid.UUID
	ParentID          *uuid.UUID
	DueDate           *string
	SortOrder         float64
	SubIssueSortOrder *float64
	SLAType           *string
	SLABreachesAt     *time.Time
	SnoozedUntil      *time.Time
	ArchivedAt        *time.Time
	TrashedAt         *time.Time
}

func loadIssueRow(c *app.Ctx, q app.Querier, id uuid.UUID) (issueRow, error) {
	row, found, err := app.Row[issueRow](c, q,
		`SELECT id, team_id, number, title, description, state_id, priority, estimate, assignee_id, creator_id,
			project_id, milestone_id, cycle_id, parent_id, due_date::text, sort_order, sub_issue_sort_order, sla_type,
			sla_breaches_at, snoozed_until, archived_at, trashed_at
		 FROM issues WHERE id = $1 FOR UPDATE`, id)
	if err != nil {
		return row, err
	}
	if !found {
		return row, app.NotFound("No such issue")
	}
	return row, nil
}

// inTx runs fn in a transaction, committing when it succeeds.
func inTx(c *app.Ctx, fn func(tx pgx.Tx) error) error {
	return pgx.BeginFunc(c, c.DB(), fn)
}

func saveIssue(c *app.Ctx, args SaveIssueArgs) (any, error) {
	var id uuid.UUID
	err := inTx(c, func(tx pgx.Tx) error {
		if args.ID != nil {
			var err error
			if id, err = resolveIssue(c, *args.ID); err != nil {
				return err
			}
			return updateIssue(c, tx, id, args)
		}
		var err error
		id, err = createIssue(c, tx, args)
		return err
	})
	if err != nil {
		return nil, err
	}
	return issueDetail(c.Fresh(), id, false, false)
}

type UpdateIssuesArgs struct {
	IDs     []string `json:"ids" doc:"Identifiers (ENG-123) or ids of the issues to change, at most 100."`
	Changes M        `json:"changes" doc:"The change: an object with save_issue's fields (except id and template), e.g. {\"state\": \"Done\"}, {\"assignee\": \"me\", \"addLabels\": [\"Bug\"]} or {\"cycle\": \"current\"}."`
}

func updateIssues(c *app.Ctx, args UpdateIssuesArgs) (any, error) {
	if len(args.IDs) == 0 || len(args.IDs) > 100 {
		return nil, app.BadRequest("Pass 1 to 100 issue ids")
	}
	if args.Changes == nil {
		return nil, app.BadRequest("Invalid arguments: missing field `changes`")
	}
	changes, err := Parse[SaveIssueArgs](args.Changes)
	if err != nil {
		return nil, app.BadRequest("Invalid changes: %s", strings.TrimPrefix(err.Error(), "Invalid arguments: "))
	}
	if changes.ID != nil || changes.Template != nil {
		return nil, app.BadRequest("changes can't have an id or a template")
	}
	updated := []string{}
	for _, reference := range args.IDs {
		id, err := resolveIssue(c, reference)
		if err != nil {
			return nil, err
		}
		var ident string
		err = inTx(c, func(tx pgx.Tx) error {
			if err := updateIssue(c, tx, id, changes); err != nil {
				return err
			}
			ident, err = issueIdentifier(c, tx, id)
			return err
		})
		if err != nil {
			return nil, err
		}
		updated = append(updated, ident)
	}
	return M{"updated": updated}, nil
}

type teamSettings struct {
	EstimateType      string
	EstimateAllowZero bool
	EstimateExtended  bool
	DefaultEstimate   *float64
	TriageEnabled     bool
}

func loadTeamSettings(c *app.Ctx, q app.Querier, team uuid.UUID) (teamSettings, error) {
	settings, _, err := app.Row[teamSettings](c, q,
		"SELECT estimate_type, estimate_allow_zero, estimate_extended, default_estimate, triage_enabled FROM teams WHERE id = $1", team)
	return settings, err
}

// allowedEstimates is the estimates a team's scale allows.
func allowedEstimates(kind string, allowZero, extended bool) []float64 {
	scales := map[string][2][]float64{
		"exponential": {{1, 2, 4, 8, 16}, {32, 64}},
		"fibonacci":   {{1, 2, 3, 5, 8}, {13, 21}},
		"linear":      {{1, 2, 3, 4, 5}, {6, 7, 8, 9, 10}},
		"tShirt":      {{1, 2, 3, 5, 8}, {13, 21}},
	}
	values := []float64{}
	if allowZero {
		values = append(values, 0)
	}
	values = append(values, scales[kind][0]...)
	if extended {
		values = append(values, scales[kind][1]...)
	}
	return values
}

func checkEstimate(settings teamSettings, estimate float64) (float64, error) {
	if settings.EstimateType == "notUsed" {
		return 0, app.BadRequest("This team doesn't use estimates; turn them on with save_team (estimateType)")
	}
	allowed := allowedEstimates(settings.EstimateType, settings.EstimateAllowZero, settings.EstimateExtended)
	if !slices.Contains(allowed, estimate) {
		list := make([]string, len(allowed))
		for i, v := range allowed {
			list[i] = strconv.FormatFloat(v, 'f', -1, 64)
		}
		return 0, app.BadRequest("%s isn't on the team's %s scale; use one of %s",
			strconv.FormatFloat(estimate, 'f', -1, 64), settings.EstimateType, strings.Join(list, ", "))
	}
	return estimate, nil
}

// defaultState is the state new issues of the team start in: its default, else the first backlog
// or unstarted.
func defaultState(c *app.Ctx, q app.Querier, team uuid.UUID) (uuid.UUID, error) {
	id, found, err := app.Maybe[uuid.UUID](c, q,
		`SELECT s.id FROM workflow_states s JOIN teams t ON t.id = s.team_id
		 WHERE s.team_id = $1 AND s.archived_at IS NULL
		 ORDER BY s.id = t.default_issue_state_id DESC, array_position(ARRAY['backlog','unstarted','started','triage','completed','canceled'], s.type), s.position
		 LIMIT 1`, team)
	if err != nil {
		return uuid.Nil, err
	}
	if !found {
		return uuid.Nil, app.BadRequest("The team has no statuses")
	}
	return id, nil
}

func firstStateOfType(c *app.Ctx, q app.Querier, team uuid.UUID, kind string) (*uuid.UUID, error) {
	id, found, err := app.Maybe[uuid.UUID](c, q,
		"SELECT id FROM workflow_states WHERE team_id = $1 AND type = $2 AND archived_at IS NULL ORDER BY position LIMIT 1", team, kind)
	if err != nil || !found {
		return nil, err
	}
	return &id, nil
}

func stateType(c *app.Ctx, q app.Querier, state uuid.UUID) (string, error) {
	return app.One[string](c, q, "SELECT type FROM workflow_states WHERE id = $1", state)
}

// issueTemplate is what a template fills in on a new issue.
type issueTemplate struct {
	Title       *string  `json:"title"`
	Description *string  `json:"description"`
	Team        *string  `json:"team"`
	State       *string  `json:"state"`
	Priority    *int     `json:"priority"`
	Estimate    *float64 `json:"estimate"`
	Assignee    *string  `json:"assignee"`
	Labels      []string `json:"labels"`
	Project     *string  `json:"project"`
	Cycle       *string  `json:"cycle"`
	DueDate     *string  `json:"dueDate"`
	SubIssues   []M      `json:"subIssues"`
}

func some[T any](value T) Nullable[T] {
	return Nullable[T]{Set: true, Valid: true, Value: value}
}

func orNullable[T any](current Nullable[T], fallback *T) Nullable[T] {
	if current.Set || fallback == nil {
		return current
	}
	return some(*fallback)
}

func firstOf[T any](values ...*T) *T {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

func createIssue(c *app.Ctx, tx pgx.Tx, args SaveIssueArgs) (uuid.UUID, error) {
	var subIssues []M
	if args.Template != nil {
		templateID, err := resolveTemplate(c, "issue", *args.Template)
		if err != nil {
			return uuid.Nil, err
		}
		type stored struct {
			Data M
			Team *uuid.UUID
		}
		row, _, err := app.Row[stored](c, tx, "SELECT data, team_id FROM templates WHERE id = $1", templateID)
		if err != nil {
			return uuid.Nil, err
		}
		var t issueTemplate
		_ = remarshal(row.Data, &t)
		var templateTeam *string
		if row.Team != nil {
			templateTeam = new(row.Team.String())
		}
		args.Template = nil
		args.Team = firstOf(args.Team, t.Team, templateTeam)
		args.Title = firstOf(args.Title, t.Title)
		if orEmpty(args.Description) == "" {
			args.Description = t.Description
		}
		args.State = firstOf(args.State, t.State)
		args.Priority = firstOf(args.Priority, t.Priority)
		args.Estimate = orNullable(args.Estimate, t.Estimate)
		args.Assignee = orNullable(args.Assignee, t.Assignee)
		if t.Labels != nil {
			args.AddLabels = append(args.AddLabels, t.Labels...)
		}
		args.Project = orNullable(args.Project, t.Project)
		args.Cycle = orNullable(args.Cycle, t.Cycle)
		args.DueDate = orNullable(args.DueDate, t.DueDate)
		subIssues = t.SubIssues
	}
	var parent *uuid.UUID
	if args.ParentID.Valid {
		id, err := resolveIssue(c, args.ParentID.Value)
		if err != nil {
			return uuid.Nil, err
		}
		parent = &id
	}
	var team uuid.UUID
	var err error
	switch {
	case args.Team != nil:
		team, err = resolveTeam(c, *args.Team)
	case parent != nil:
		team, err = app.One[uuid.UUID](c, tx, "SELECT team_id FROM issues WHERE id = $1", *parent)
	default:
		team, err = onlyTeam(c)
	}
	if err != nil {
		return uuid.Nil, err
	}
	title, err := nonEmpty(orEmpty(args.Title), "title")
	if err != nil {
		return uuid.Nil, err
	}
	settings, err := loadTeamSettings(c, tx, team)
	if err != nil {
		return uuid.Nil, err
	}
	var state uuid.UUID
	if args.State != nil {
		state, err = resolveState(c, team, *args.State)
	} else {
		state, err = defaultState(c, tx, team)
	}
	if err != nil {
		return uuid.Nil, err
	}
	number, err := app.One[int](c, tx, "UPDATE teams SET issue_count = issue_count + 1 WHERE id = $1 RETURNING issue_count", team)
	if err != nil {
		return uuid.Nil, err
	}
	var sortOrder float64
	if args.SortOrder != nil {
		sortOrder = *args.SortOrder
	} else if sortOrder, err = app.One[float64](c, tx, "SELECT coalesce(min(sort_order), 0) - 1000 FROM issues WHERE team_id = $1", team); err != nil {
		return uuid.Nil, err
	}
	var estimate *float64
	switch {
	case args.Estimate.Valid:
		e, err := checkEstimate(settings, args.Estimate.Value)
		if err != nil {
			return uuid.Nil, err
		}
		estimate = &e
	case !args.Estimate.Set && settings.EstimateType != "notUsed":
		estimate = settings.DefaultEstimate
	}
	args.Estimate = Nullable[float64]{}
	id, err := app.One[uuid.UUID](c, tx,
		`INSERT INTO issues (workspace_id, team_id, number, title, description, state_id, priority, creator_id, sort_order, estimate)
		 VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $8, $9) RETURNING id`,
		c.Workspace, team, number, title, orEmpty(args.Description), state, c.User, sortOrder, estimate)
	if err != nil {
		return uuid.Nil, err
	}
	args.Description = nil
	newType, err := stateType(c, tx, state)
	if err != nil {
		return uuid.Nil, err
	}
	if err := app.Exec(c, tx,
		`UPDATE issues SET
			started_at = CASE WHEN $2 IN ('started', 'completed') THEN now() END,
			completed_at = CASE WHEN $2 = 'completed' THEN now() END,
			canceled_at = CASE WHEN $2 = 'canceled' THEN now() END,
			started_triage_at = CASE WHEN $2 = 'triage' THEN now() END
		 WHERE id = $1`, id, newType); err != nil {
		return uuid.Nil, err
	}
	if err := subscribe(c, tx, id, []uuid.UUID{c.User}); err != nil {
		return uuid.Nil, err
	}
	if err := history(c, tx, id, "created", M{}); err != nil {
		return uuid.Nil, err
	}
	args.State, args.Title, args.Team = nil, nil, nil
	if err := applyIssueChanges(c, tx, id, args, true); err != nil {
		return uuid.Nil, err
	}
	if newType == "triage" && settings.TriageEnabled {
		if err := routeToTriage(c, tx, id, team); err != nil {
			return uuid.Nil, err
		}
	}
	for n, sub := range subIssues {
		subArgs, err := Parse[SaveIssueArgs](sub)
		if err != nil {
			return uuid.Nil, app.BadRequest("The template's sub-issue %d is invalid: %s", n+1, strings.TrimPrefix(err.Error(), "Invalid arguments: "))
		}
		subArgs.ID, subArgs.Template = nil, nil
		// The parent isn't committed yet, so resolving it by reference would miss it.
		subArgs.ParentID = Nullable[string]{}
		if subArgs.Team == nil {
			subArgs.Team = new(team.String())
		}
		child, err := createIssue(c, tx, subArgs)
		if err != nil {
			return uuid.Nil, err
		}
		if err := app.Exec(c, tx, "UPDATE issues SET parent_id = $2, sub_issue_sort_order = $3 WHERE id = $1",
			child, id, float64(n+1)*1000); err != nil {
			return uuid.Nil, err
		}
	}
	value, err := issueValue(c, tx, id)
	if err != nil {
		return uuid.Nil, err
	}
	return id, emit(c, tx, "Issue", "create", value, nil, &team)
}

// remarshal converts a JSON value into a typed one.
func remarshal(value any, target any) error {
	return json.Unmarshal(app.Marshal(value), target)
}

// onlyTeam is the team to use when none was named: the caller's only team.
func onlyTeam(c *app.Ctx) (uuid.UUID, error) {
	visible, err := c.TeamIDs()
	if err != nil {
		return uuid.Nil, err
	}
	teams, err := app.Rows[keyed](c, c.DB(),
		"SELECT id, key, name FROM teams WHERE id = ANY($1) AND archived_at IS NULL ORDER BY key", visible)
	if err != nil {
		return uuid.Nil, err
	}
	switch len(teams) {
	case 1:
		return teams[0].ID, nil
	case 0:
		return uuid.Nil, app.BadRequest("There are no teams yet; create one with save_team")
	}
	keys := make([]string, len(teams))
	for i, t := range teams {
		keys[i] = t.Key
	}
	return uuid.Nil, app.BadRequest("Name the team: %s", strings.Join(keys, ", "))
}

// triageResponsible is who is on triage for the team now: with a rotation, the person whose
// turn it is; else everyone listed.
func triageResponsible(c *app.Ctx, q app.Querier, team uuid.UUID) (string, []uuid.UUID, error) {
	type triage struct {
		Action string
		IDs    []uuid.UUID
		Weeks  *int
		Start  *time.Time
	}
	row, _, err := app.Row[triage](c, q,
		"SELECT triage_action, triage_responsible_ids, triage_rotation_weeks, triage_rotation_start FROM teams WHERE id = $1", team)
	if err != nil {
		return "", nil, err
	}
	if row.Weeks == nil || row.Start == nil || len(row.IDs) == 0 || *row.Weeks <= 0 {
		return row.Action, row.IDs, nil
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	elapsed := max(int(today.Sub(*row.Start).Hours()/24)/7, 0)
	turn := (elapsed / *row.Weeks) % len(row.IDs)
	return row.Action, []uuid.UUID{row.IDs[turn]}, nil
}

func routeToTriage(c *app.Ctx, tx pgx.Tx, issue, team uuid.UUID) error {
	action, people, err := triageResponsible(c, tx, team)
	if err != nil || len(people) == 0 || action == "none" {
		return err
	}
	ident, err := issueIdentifier(c, tx, issue)
	if err != nil {
		return err
	}
	if action == "assign" {
		assignee := people[0]
		if err := app.Exec(c, tx, "UPDATE issues SET assignee_id = $2 WHERE id = $1 AND assignee_id IS NULL", issue, assignee); err != nil {
			return err
		}
		if err := subscribe(c, tx, issue, []uuid.UUID{assignee}); err != nil {
			return err
		}
	}
	return notify(c, tx, people, Notice{Kind: "triageIssue", Summary: ident + " needs triage", Issue: &issue})
}

func updateIssue(c *app.Ctx, tx pgx.Tx, id uuid.UUID, args SaveIssueArgs) error {
	if args.Template != nil {
		return app.BadRequest("Templates only apply when creating an issue")
	}
	before, err := issueValue(c, tx, id)
	if err != nil {
		return err
	}
	row, err := loadIssueRow(c, tx, id)
	if err != nil {
		return err
	}
	if args.Team != nil {
		team, err := resolveTeam(c, *args.Team)
		if err != nil {
			return err
		}
		if team != row.TeamID {
			if err := moveTeam(c, tx, row, team); err != nil {
				return err
			}
		}
		args.Team = nil
	}
	if row, err = loadIssueRow(c, tx, id); err != nil {
		return err
	}
	changes := M{}
	if args.Title != nil {
		title, err := nonEmpty(*args.Title, "title")
		if err != nil {
			return err
		}
		if title != row.Title {
			if err := app.Exec(c, tx, "UPDATE issues SET title = $2 WHERE id = $1", id, title); err != nil {
				return err
			}
			changes["title"] = M{"from": row.Title, "to": title}
		}
		args.Title = nil
	}
	description, err := newContent(row.Description, args.Description, args.Patch)
	if err != nil {
		return err
	}
	args.Description, args.Patch = nil, nil
	if description != nil && *description != row.Description {
		if err := app.Exec(c, tx, "UPDATE issues SET description = $2 WHERE id = $1", id, *description); err != nil {
			return err
		}
		changes["description"] = M{"changed": true}
		previous, err := mentions(c, row.Description)
		if err != nil {
			return err
		}
		current, err := mentions(c, *description)
		if err != nil {
			return err
		}
		mentioned := slices.DeleteFunc(current, func(u uuid.UUID) bool { return slices.Contains(previous, u) })
		if err := notifyMentions(c, tx, id, mentioned); err != nil {
			return err
		}
	}
	if args.State != nil {
		state, err := resolveState(c, row.TeamID, *args.State)
		if err != nil {
			return err
		}
		if state != row.StateID {
			if err := setState(c, tx, id, state, "updated"); err != nil {
				return err
			}
		}
		args.State = nil
	}
	if len(changes) > 0 {
		if err := history(c, tx, id, "updated", changes); err != nil {
			return err
		}
	}
	if err := applyIssueChanges(c, tx, id, args, false); err != nil {
		return err
	}
	if err := app.Exec(c, tx, "UPDATE issues SET updated_at = now() WHERE id = $1", id); err != nil {
		return err
	}
	after, err := issueValue(c, tx, id)
	if err != nil {
		return err
	}
	updatedFrom := changedFields(before, after)
	if len(updatedFrom) == 0 {
		return nil
	}
	return emit(c, tx, "Issue", "update", after, updatedFrom, &row.TeamID)
}

// changedFields is the previous values of the fields that changed, for webhooks' updatedFrom.
func changedFields(before, after M) M {
	changed := M{}
	for key, value := range before {
		if key == "updatedAt" {
			continue
		}
		if next, ok := after[key]; !ok || string(app.Marshal(next)) != string(app.Marshal(value)) {
			changed[key] = value
		}
	}
	return changed
}

func notifyMentions(c *app.Ctx, tx pgx.Tx, issue uuid.UUID, users []uuid.UUID) error {
	if len(users) == 0 {
		return nil
	}
	if err := subscribe(c, tx, issue, users); err != nil {
		return err
	}
	ident, err := issueIdentifier(c, tx, issue)
	if err != nil {
		return err
	}
	return notify(c, tx, users, Notice{Kind: "issueMention", Summary: "You were mentioned in " + ident, Issue: &issue})
}

// moveTeam moves the issue to another team: a new number (the old identifier keeps resolving), a
// status of the same name or type, and only labels the new team has. Its sub-issues come along.
func moveTeam(c *app.Ctx, tx pgx.Tx, row issueRow, team uuid.UUID) error {
	oldIdentifier, err := issueIdentifier(c, tx, row.ID)
	if err != nil {
		return err
	}
	number, err := app.One[int](c, tx, "UPDATE teams SET issue_count = issue_count + 1 WHERE id = $1 RETURNING issue_count", team)
	if err != nil {
		return err
	}
	old, _, err := app.Row[struct{ Name, Type string }](c, tx, "SELECT name, type FROM workflow_states WHERE id = $1", row.StateID)
	if err != nil {
		return err
	}
	state, found, err := app.Maybe[uuid.UUID](c, tx,
		"SELECT id FROM workflow_states WHERE team_id = $1 AND lower(name) = lower($2) AND archived_at IS NULL LIMIT 1", team, old.Name)
	if err != nil {
		return err
	}
	if !found {
		sameType, err := firstStateOfType(c, tx, team, old.Type)
		if err != nil {
			return err
		}
		if sameType != nil {
			state = *sameType
		} else if state, err = defaultState(c, tx, team); err != nil {
			return err
		}
	}
	if err := app.Exec(c, tx,
		`UPDATE issues SET team_id = $2, number = $3, state_id = $4, cycle_id = NULL,
			previous_identifiers = array_append(previous_identifiers, $5), updated_at = now()
		 WHERE id = $1`, row.ID, team, number, state, oldIdentifier); err != nil {
		return err
	}
	if err := app.Exec(c, tx,
		`DELETE FROM issue_label_links l USING issue_labels lb
		 WHERE l.issue_id = $1 AND lb.id = l.label_id AND lb.team_id IS NOT NULL AND lb.team_id <> $2`, row.ID, team); err != nil {
		return err
	}
	newIdentifier, err := issueIdentifier(c, tx, row.ID)
	if err != nil {
		return err
	}
	from, to, err := refs(c, tx, "team_ref", row.TeamID, team)
	if err != nil {
		return err
	}
	if err := history(c, tx, row.ID, "moved", M{
		"team":       M{"from": from, "to": to},
		"identifier": M{"from": oldIdentifier, "to": newIdentifier},
	}); err != nil {
		return err
	}
	children, err := app.All[uuid.UUID](c, tx,
		"SELECT id FROM issues WHERE parent_id = $1 AND team_id = $2 AND trashed_at IS NULL", row.ID, row.TeamID)
	if err != nil {
		return err
	}
	for _, child := range children {
		childRow, err := loadIssueRow(c, tx, child)
		if err != nil {
			return err
		}
		if err := moveTeam(c, tx, childRow, team); err != nil {
			return err
		}
	}
	return nil
}

// refs is the short JSON form of two things (either may be nil), by a *_ref SQL function.
func refs(c *app.Ctx, q app.Querier, function string, a, b any) (any, any, error) {
	type pair struct{ From, To any }
	row, _, err := app.Row[pair](c, q, fmt.Sprintf("SELECT %[1]s($1), %[1]s($2)", function), a, b)
	return row.From, row.To, err
}

type stateChange struct {
	issue, state uuid.UUID
	action       string
}

// setState moves the issue to state, stamping when it started, completed or left triage, and runs
// what follows: notifications, the cycle it joins, and closing parents or sub-issues when the
// team automates that.
func setState(c *app.Ctx, tx pgx.Tx, issue, state uuid.UUID, action string) error {
	queue := []stateChange{{issue, state, action}}
	for len(queue) > 0 {
		next := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		row, err := loadIssueRow(c, tx, next.issue)
		if err != nil {
			return err
		}
		if row.StateID == next.state {
			continue
		}
		oldType, err := stateType(c, tx, row.StateID)
		if err != nil {
			return err
		}
		newType, err := stateType(c, tx, next.state)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx,
			`UPDATE issues SET state_id = $2,
				started_at = CASE WHEN $3 = 'started' THEN coalesce(started_at, now())
				                  WHEN $3 IN ('completed', 'canceled') THEN started_at END,
				completed_at = CASE WHEN $3 = 'completed' THEN now() END,
				canceled_at = CASE WHEN $3 = 'canceled' THEN now() END,
				started_triage_at = CASE WHEN $3 = 'triage' THEN coalesce(started_triage_at, now()) ELSE started_triage_at END,
				triaged_at = CASE WHEN $3 <> 'triage' AND $4 = 'triage' THEN now() ELSE triaged_at END,
				auto_closed_at = NULL,
				updated_at = now()
			 WHERE id = $1`, next.issue, next.state, newType, oldType); err != nil {
			return err
		}
		from, to, err := refs(c, tx, "state_ref", row.StateID, next.state)
		if err != nil {
			return err
		}
		if err := history(c, tx, next.issue, next.action, M{"state": M{"from": from, "to": to}}); err != nil {
			return err
		}
		ident, err := issueIdentifier(c, tx, next.issue)
		if err != nil {
			return err
		}
		stateName, _ := to.(M)["name"].(string)
		subscribers, err := issueSubscribers(c, tx, next.issue)
		if err != nil {
			return err
		}
		if row.AssigneeID != nil {
			subscribers = append(subscribers, *row.AssigneeID)
		}
		if err := notify(c, tx, subscribers, Notice{Kind: "issueStatusChanged", Summary: ident + " moved to " + stateName, Issue: &next.issue}); err != nil {
			return err
		}
		if err := addToCurrentCycle(c, tx, row, newType); err != nil {
			return err
		}
		closed := newType == "completed" || newType == "canceled"
		if closed {
			if err := notifyUnblocked(c, tx, next.issue); err != nil {
				return err
			}
			closeChildren, err := app.One[bool](c, tx, "SELECT auto_close_child_issues FROM teams WHERE id = $1", row.TeamID)
			if err != nil {
				return err
			}
			if closeChildren {
				type child struct{ ID, Team uuid.UUID }
				children, err := app.Rows[child](c, tx,
					`SELECT c.id, c.team_id FROM issues c JOIN workflow_states s ON s.id = c.state_id
					 WHERE c.parent_id = $1 AND c.trashed_at IS NULL AND s.type NOT IN ('completed', 'canceled')`, next.issue)
				if err != nil {
					return err
				}
				for _, ch := range children {
					target, err := firstStateOfType(c, tx, ch.Team, newType)
					if err != nil {
						return err
					}
					if target != nil {
						queue = append(queue, stateChange{ch.ID, *target, "auto_closed_with_parent"})
					}
				}
			}
		}
		if row.ParentID == nil {
			continue
		}
		parent := *row.ParentID
		type parentRow struct {
			Team uuid.UUID
			Type string
		}
		p, _, err := app.Row[parentRow](c, tx,
			"SELECT i.team_id, s.type FROM issues i JOIN workflow_states s ON s.id = i.state_id WHERE i.id = $1", parent)
		if err != nil {
			return err
		}
		closeParent, err := app.One[bool](c, tx, "SELECT auto_close_parent_issues FROM teams WHERE id = $1", p.Team)
		if err != nil {
			return err
		}
		counts, _, err := app.Row[struct{ Open, Done int64 }](c, tx,
			`SELECT count(*) FILTER (WHERE s.type NOT IN ('completed', 'canceled')), count(*) FILTER (WHERE s.type = 'completed')
			 FROM issues c JOIN workflow_states s ON s.id = c.state_id WHERE c.parent_id = $1 AND c.trashed_at IS NULL`, parent)
		if err != nil {
			return err
		}
		if !closeParent || !closed || counts.Open != 0 || counts.Done == 0 || p.Type == "completed" || p.Type == "canceled" {
			continue
		}
		target, err := firstStateOfType(c, tx, p.Team, "completed")
		if err != nil {
			return err
		}
		if target != nil {
			queue = append(queue, stateChange{parent, *target, "auto_completed_by_sub_issues"})
		}
	}
	return nil
}

// addToCurrentCycle puts a started (or completed) issue without a cycle into the team's current
// cycle, when the team does that.
func addToCurrentCycle(c *app.Ctx, tx pgx.Tx, row issueRow, newType string) error {
	if row.CycleID != nil || (newType != "started" && newType != "completed") {
		return nil
	}
	column := "cycle_auto_add_completed"
	if newType == "started" {
		column = "cycle_auto_add_started"
	}
	return app.Exec(c, tx, fmt.Sprintf(
		`UPDATE issues SET cycle_id = (SELECT c.id FROM cycles c WHERE c.team_id = $2 AND c.starts_at <= now()
			AND c.ends_at > now() AND c.archived_at IS NULL ORDER BY c.starts_at DESC LIMIT 1)
		 WHERE id = $1 AND EXISTS (SELECT 1 FROM teams t WHERE t.id = $2 AND t.cycles_enabled AND t.%s)`, column),
		row.ID, row.TeamID)
}

// notifyUnblocked tells the people on issues this one blocked that nothing blocks them anymore.
func notifyUnblocked(c *app.Ctx, tx pgx.Tx, issue uuid.UUID) error {
	freed, err := app.All[uuid.UUID](c, tx,
		`SELECT r.related_issue_id FROM issue_relations r WHERE r.issue_id = $1 AND r.type = 'blocks'
		 AND NOT EXISTS (SELECT 1 FROM issue_relations o JOIN issues b ON b.id = o.issue_id
			JOIN workflow_states s ON s.id = b.state_id
			WHERE o.related_issue_id = r.related_issue_id AND o.type = 'blocks' AND o.issue_id <> $1
			AND s.type NOT IN ('completed', 'canceled') AND b.trashed_at IS NULL)`, issue)
	if err != nil {
		return err
	}
	for _, blocked := range freed {
		ident, err := issueIdentifier(c, tx, blocked)
		if err != nil {
			return err
		}
		people, err := issueSubscribers(c, tx, blocked)
		if err != nil {
			return err
		}
		assignee, err := app.One[*uuid.UUID](c, tx, "SELECT assignee_id FROM issues WHERE id = $1", blocked)
		if err != nil {
			return err
		}
		if assignee != nil {
			people = append(people, *assignee)
		}
		if err := notify(c, tx, people, Notice{Kind: "issueUnblocked", Summary: ident + " is no longer blocked", Issue: &blocked}); err != nil {
			return err
		}
	}
	return nil
}

// resolveOptional resolves a nullable reference: nil when null or a null word.
func resolveOptional(value Nullable[string], resolve func(string) (uuid.UUID, error)) (*uuid.UUID, error) {
	if !value.Valid || isNullWord(value.Value) {
		return nil, nil
	}
	id, err := resolve(value.Value)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func samePtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// applyIssueChanges applies the fields that work the same when creating and updating.
func applyIssueChanges(c *app.Ctx, tx pgx.Tx, id uuid.UUID, args SaveIssueArgs, creating bool) error {
	row, err := loadIssueRow(c, tx, id)
	if err != nil {
		return err
	}
	changes := M{}
	settings, err := loadTeamSettings(c, tx, row.TeamID)
	if err != nil {
		return err
	}
	if args.Priority != nil {
		priority, err := checkPriority(*args.Priority)
		if err != nil {
			return err
		}
		if priority != row.Priority {
			if err := app.Exec(c, tx, "UPDATE issues SET priority = $2 WHERE id = $1", id, priority); err != nil {
				return err
			}
			changes["priority"] = M{"from": row.Priority, "to": priority}
		}
	}
	if args.Estimate.Set {
		var estimate *float64
		if args.Estimate.Valid {
			e, err := checkEstimate(settings, args.Estimate.Value)
			if err != nil {
				return err
			}
			estimate = &e
		}
		if !samePtr(estimate, row.Estimate) {
			if err := app.Exec(c, tx, "UPDATE issues SET estimate = $2 WHERE id = $1", id, estimate); err != nil {
				return err
			}
			changes["estimate"] = M{"from": row.Estimate, "to": estimate}
		}
	}
	if args.Assignee.Set {
		assignee, err := resolveOptional(args.Assignee, func(v string) (uuid.UUID, error) { return resolveUser(c, v) })
		if err != nil {
			return err
		}
		if !samePtr(assignee, row.AssigneeID) {
			if assignee != nil {
				if err := checkCanSeeTeam(c, tx, *assignee, row.TeamID, id); err != nil {
					return err
				}
			}
			if err := app.Exec(c, tx, "UPDATE issues SET assignee_id = $2 WHERE id = $1", id, assignee); err != nil {
				return err
			}
			from, to, err := refs(c, tx, "user_ref", row.AssigneeID, assignee)
			if err != nil {
				return err
			}
			changes["assignee"] = M{"from": from, "to": to}
			ident, err := issueIdentifier(c, tx, id)
			if err != nil {
				return err
			}
			if assignee != nil {
				if err := subscribe(c, tx, id, []uuid.UUID{*assignee}); err != nil {
					return err
				}
				if err := notify(c, tx, []uuid.UUID{*assignee}, Notice{Kind: "issueAssignedToYou", Summary: ident + " was assigned to you", Issue: &id}); err != nil {
					return err
				}
			}
			if row.AssigneeID != nil {
				if err := notify(c, tx, []uuid.UUID{*row.AssigneeID}, Notice{Kind: "issueUnassignedFromYou", Summary: ident + " was unassigned from you", Issue: &id}); err != nil {
					return err
				}
			}
		}
	}
	if creating {
		description, err := app.One[string](c, tx, "SELECT description FROM issues WHERE id = $1", id)
		if err != nil {
			return err
		}
		mentioned, err := mentions(c, description)
		if err != nil {
			return err
		}
		if err := notifyMentions(c, tx, id, mentioned); err != nil {
			return err
		}
	}
	if err := applyLabels(c, tx, row, args, changes); err != nil {
		return err
	}
	project := row.ProjectID
	if args.Project.Set {
		if project, err = resolveOptional(args.Project, func(v string) (uuid.UUID, error) { return resolveProject(c, v) }); err != nil {
			return err
		}
		if !samePtr(project, row.ProjectID) {
			if err := app.Exec(c, tx,
				"UPDATE issues SET project_id = $2, milestone_id = CASE WHEN $2 IS NULL OR $2 IS DISTINCT FROM project_id THEN NULL ELSE milestone_id END WHERE id = $1",
				id, project); err != nil {
				return err
			}
			if project != nil {
				if err := app.Exec(c, tx, "INSERT INTO project_teams (project_id, team_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", *project, row.TeamID); err != nil {
					return err
				}
			}
			from, to, err := refs(c, tx, "project_ref", row.ProjectID, project)
			if err != nil {
				return err
			}
			changes["project"] = M{"from": from, "to": to}
		}
	}
	if args.Milestone.Set {
		milestone, err := resolveOptional(args.Milestone, func(v string) (uuid.UUID, error) { return resolveMilestone(c, project, v) })
		if err != nil {
			return err
		}
		var milestoneProject *uuid.UUID
		if milestone != nil {
			mp, found, err := app.Maybe[uuid.UUID](c, tx, "SELECT project_id FROM project_milestones WHERE id = $1", *milestone)
			if err != nil {
				return err
			}
			if found {
				milestoneProject = &mp
			}
		}
		if milestoneProject != nil && project != nil && *milestoneProject != *project {
			return app.BadRequest("That milestone belongs to another project than the issue's")
		}
		if err := app.Exec(c, tx, "UPDATE issues SET milestone_id = $2, project_id = coalesce($3, project_id) WHERE id = $1",
			id, milestone, milestoneProject); err != nil {
			return err
		}
		if !samePtr(milestone, row.MilestoneID) {
			from, to, err := refs(c, tx, "milestone_ref", row.MilestoneID, milestone)
			if err != nil {
				return err
			}
			changes["projectMilestone"] = M{"from": from, "to": to}
		}
	}
	if args.Cycle.Set {
		cycle, err := resolveOptional(args.Cycle, func(v string) (uuid.UUID, error) { return resolveCycle(c, &row.TeamID, v) })
		if err != nil {
			return err
		}
		if cycle != nil {
			cycleTeam, err := app.One[uuid.UUID](c, tx, "SELECT team_id FROM cycles WHERE id = $1", *cycle)
			if err != nil {
				return err
			}
			if cycleTeam != row.TeamID {
				return app.BadRequest("That cycle belongs to another team")
			}
		}
		if !samePtr(cycle, row.CycleID) {
			if err := app.Exec(c, tx, "UPDATE issues SET cycle_id = $2 WHERE id = $1", id, cycle); err != nil {
				return err
			}
			from, to, err := refs(c, tx, "cycle_ref", row.CycleID, cycle)
			if err != nil {
				return err
			}
			changes["cycle"] = M{"from": from, "to": to}
		}
	}
	if args.ParentID.Set {
		parent, err := resolveOptional(args.ParentID, func(v string) (uuid.UUID, error) { return resolveIssue(c, v) })
		if err != nil {
			return err
		}
		if parent != nil {
			loops, err := app.One[bool](c, tx,
				`WITH RECURSIVE up AS (SELECT id, parent_id FROM issues WHERE id = $1
				 UNION SELECT i.id, i.parent_id FROM issues i JOIN up ON i.id = up.parent_id)
				 SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)`, *parent, id)
			if err != nil {
				return err
			}
			if loops {
				return app.BadRequest("An issue can't be a sub-issue of itself or of its own sub-issues")
			}
		}
		if !samePtr(parent, row.ParentID) {
			if err := app.Exec(c, tx,
				`UPDATE issues SET parent_id = $2, sub_issue_sort_order = CASE WHEN $2::uuid IS NULL THEN NULL ELSE
					(SELECT coalesce(max(sub_issue_sort_order), 0) + 1000 FROM issues WHERE parent_id = $2) END
				 WHERE id = $1`, id, parent); err != nil {
				return err
			}
			from, to, err := refs(c, tx, "issue_ref", row.ParentID, parent)
			if err != nil {
				return err
			}
			changes["parent"] = M{"from": from, "to": to}
		}
	}
	if args.DueDate.Set {
		var due *string
		if args.DueDate.Valid && !isNullWord(args.DueDate.Value) {
			date, err := parseDate(args.DueDate.Value)
			if err != nil {
				return err
			}
			due = new(date.Format(time.DateOnly))
		}
		if !samePtr(due, row.DueDate) {
			if err := app.Exec(c, tx, "UPDATE issues SET due_date = $2::date WHERE id = $1", id, due); err != nil {
				return err
			}
			changes["dueDate"] = M{"from": row.DueDate, "to": due}
		}
	}
	if args.SnoozedUntil.Set {
		var until *time.Time
		if args.SnoozedUntil.Valid {
			at, err := parseDatetime(args.SnoozedUntil.Value)
			if err != nil {
				return err
			}
			until = &at
		}
		if err := app.Exec(c, tx,
			"UPDATE issues SET snoozed_until = $2, snoozed_by_id = CASE WHEN $2::timestamptz IS NULL THEN NULL ELSE $3::uuid END WHERE id = $1",
			id, until, c.User); err != nil {
			return err
		}
		changes["snoozedUntil"] = M{"from": row.SnoozedUntil, "to": until}
	}
	if args.SLABreachesAt.Set {
		var breaches *time.Time
		if args.SLABreachesAt.Valid {
			at, err := parseDatetime(args.SLABreachesAt.Value)
			if err != nil {
				return err
			}
			breaches = &at
		}
		slaType := "all"
		if args.SLAType != nil {
			switch *args.SLAType {
			case "all", "onlyBusinessDays":
				slaType = *args.SLAType
			default:
				return app.BadRequest("slaType is \"all\" or \"onlyBusinessDays\", not %q", *args.SLAType)
			}
		}
		if err := app.Exec(c, tx,
			`UPDATE issues SET sla_breaches_at = $2, sla_type = CASE WHEN $2::timestamptz IS NULL THEN NULL ELSE $3 END,
				sla_started_at = CASE WHEN $2::timestamptz IS NULL THEN NULL ELSE coalesce(sla_started_at, now()) END WHERE id = $1`,
			id, breaches, slaType); err != nil {
			return err
		}
		changes["slaBreachesAt"] = M{"from": row.SLABreachesAt, "to": breaches}
	} else if args.SLAType != nil {
		return app.BadRequest("Set slaType together with slaBreachesAt")
	}
	if args.SortOrder != nil {
		if err := app.Exec(c, tx, "UPDATE issues SET sort_order = $2 WHERE id = $1", id, *args.SortOrder); err != nil {
			return err
		}
	}
	if args.SubIssueSortOrder != nil {
		if err := app.Exec(c, tx, "UPDATE issues SET sub_issue_sort_order = $2 WHERE id = $1", id, *args.SubIssueSortOrder); err != nil {
			return err
		}
	}
	if err := applyRelations(c, tx, id, args, changes); err != nil {
		return err
	}
	for _, link := range args.Links {
		if _, err := upsertAttachment(c, tx, id, link.URL, link.Title, nil, nil); err != nil {
			return err
		}
	}
	if args.AddSubscribers != nil {
		users, err := resolveUsers(c, args.AddSubscribers)
		if err != nil {
			return err
		}
		if err := subscribe(c, tx, id, users); err != nil {
			return err
		}
	}
	if args.RemoveSubscribers != nil {
		users, err := resolveUsers(c, args.RemoveSubscribers)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM issue_subscribers WHERE issue_id = $1 AND user_id = ANY($2)", id, users); err != nil {
			return err
		}
	}
	if err := applyReleases(c, tx, id, args); err != nil {
		return err
	}
	if len(changes) > 0 && !creating {
		return history(c, tx, id, "updated", changes)
	}
	return nil
}

// checkCanSeeTeam keeps guests and people outside a private team from being assigned unless the
// issue is shared with them.
func checkCanSeeTeam(c *app.Ctx, tx pgx.Tx, user, team, issue uuid.UUID) error {
	ok, err := app.One[bool](c, tx,
		`SELECT EXISTS (SELECT 1 FROM teams t JOIN workspace_members m ON m.workspace_id = t.workspace_id AND m.user_id = $2
			WHERE t.id = $1 AND ((NOT t.private AND m.role <> 'guest')
				OR EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id = t.id AND tm.user_id = $2)))
		 OR EXISTS (SELECT 1 FROM issue_shares s WHERE s.issue_id = $3 AND s.user_id = $2)`, team, user, issue)
	if err != nil {
		return err
	}
	if !ok {
		return app.BadRequest("That person can't see this team's issues; add them to the team or share the issue first")
	}
	return nil
}

func applyLabels(c *app.Ctx, tx pgx.Tx, row issueRow, args SaveIssueArgs, changes M) error {
	if args.Labels != nil && (args.AddLabels != nil || args.RemoveLabels != nil) {
		return app.BadRequest("Pass either labels or addLabels/removeLabels, not both")
	}
	if args.Labels == nil && args.AddLabels == nil && args.RemoveLabels == nil {
		return nil
	}
	current, err := app.All[uuid.UUID](c, tx, "SELECT label_id FROM issue_label_links WHERE issue_id = $1", row.ID)
	if err != nil {
		return err
	}
	next := slices.Clone(current)
	if args.Labels != nil {
		if next, err = resolveLabels(c, &row.TeamID, args.Labels); err != nil {
			return err
		}
	}
	if args.AddLabels != nil {
		added, err := resolveLabels(c, &row.TeamID, args.AddLabels)
		if err != nil {
			return err
		}
		// A label from a single-select group replaces its sibling.
		type group struct {
			Label  uuid.UUID
			Parent *uuid.UUID
			Kind   string
		}
		groups, err := app.Rows[group](c, tx,
			`SELECT l.id, l.parent_id, coalesce(p.group_type, 'multiSelect') FROM issue_labels l
			 LEFT JOIN issue_labels p ON p.id = l.parent_id WHERE l.id = ANY($1)`, added)
		if err != nil {
			return err
		}
		for _, g := range groups {
			if g.Parent != nil && g.Kind == "singleSelect" {
				siblings, err := app.All[uuid.UUID](c, tx, "SELECT id FROM issue_labels WHERE parent_id = $1 AND id <> $2", *g.Parent, g.Label)
				if err != nil {
					return err
				}
				next = slices.DeleteFunc(next, func(l uuid.UUID) bool { return slices.Contains(siblings, l) })
			}
			next = appendUnique(next, g.Label)
		}
	}
	if args.RemoveLabels != nil {
		removed, err := resolveLabels(c, &row.TeamID, args.RemoveLabels)
		if err != nil {
			return err
		}
		next = slices.DeleteFunc(next, func(l uuid.UUID) bool { return slices.Contains(removed, l) })
	}
	clashes, err := app.All[string](c, tx,
		`SELECT p.name FROM issue_labels l JOIN issue_labels p ON p.id = l.parent_id
		 WHERE l.id = ANY($1) AND p.group_type = 'singleSelect' GROUP BY p.name HAVING count(*) > 1`, next)
	if err != nil {
		return err
	}
	if len(clashes) > 0 {
		return app.BadRequest("An issue takes one label of the group %q", clashes[0])
	}
	sortedCurrent := slices.SortedFunc(slices.Values(current), compareUUID)
	sortedNext := slices.SortedFunc(slices.Values(next), compareUUID)
	if slices.Equal(sortedCurrent, sortedNext) {
		return nil
	}
	if next == nil {
		next = []uuid.UUID{}
	}
	if err := app.Exec(c, tx, "DELETE FROM issue_label_links WHERE issue_id = $1 AND NOT (label_id = ANY($2))", row.ID, next); err != nil {
		return err
	}
	if err := app.Exec(c, tx, "INSERT INTO issue_label_links (issue_id, label_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING", row.ID, next); err != nil {
		return err
	}
	names := func(ids []uuid.UUID) ([]string, error) {
		list, err := app.All[string](c, tx, "SELECT name FROM issue_labels WHERE id = ANY($1) ORDER BY name", ids)
		if list == nil {
			list = []string{}
		}
		return list, err
	}
	from, err := names(current)
	if err != nil {
		return err
	}
	to, err := names(next)
	if err != nil {
		return err
	}
	changes["labels"] = M{"from": from, "to": to}
	return nil
}

func compareUUID(a, b uuid.UUID) int {
	return strings.Compare(a.String(), b.String())
}

func applyRelations(c *app.Ctx, tx pgx.Tx, id uuid.UUID, args SaveIssueArgs, changes M) error {
	additions := []struct {
		list    []string
		kind    string
		reverse bool
	}{{args.Blocks, "blocks", false}, {args.BlockedBy, "blocks", true}, {args.RelatedTo, "related", false}}
	for _, add := range additions {
		for _, reference := range add.list {
			other, err := resolveIssue(c, reference)
			if err != nil {
				return err
			}
			if other == id {
				return app.BadRequest("An issue can't be related to itself")
			}
			from, to := id, other
			if add.reverse {
				from, to = other, id
			}
			if add.kind == "related" {
				exists, err := app.One[bool](c, tx,
					`SELECT EXISTS (SELECT 1 FROM issue_relations WHERE type = 'related'
						AND ((issue_id = $1 AND related_issue_id = $2) OR (issue_id = $2 AND related_issue_id = $1)))`, from, to)
				if err != nil {
					return err
				}
				if exists {
					continue
				}
			}
			if err := app.Exec(c, tx,
				"INSERT INTO issue_relations (issue_id, related_issue_id, type, creator_id) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING",
				from, to, add.kind, c.User); err != nil {
				return err
			}
			label := "relatedTo"
			if add.reverse {
				label = "blockedBy"
			} else if add.kind == "blocks" {
				label = "blocks"
			}
			otherIdentifier, err := issueIdentifier(c, tx, other)
			if err != nil {
				return err
			}
			key := "added_" + label
			list, _ := changes[key].([]any)
			changes[key] = append(list, otherIdentifier)
		}
	}
	removals := []struct {
		list    []string
		kind    string
		reverse bool
	}{{args.RemoveBlocks, "blocks", false}, {args.RemoveBlockedBy, "blocks", true}, {args.RemoveRelatedTo, "related", false}}
	for _, remove := range removals {
		for _, reference := range remove.list {
			other, err := resolveIssue(c, reference)
			if err != nil {
				return err
			}
			from, to := id, other
			if remove.reverse {
				from, to = other, id
			}
			if err := app.Exec(c, tx,
				`DELETE FROM issue_relations WHERE type = $3 AND ((issue_id = $1 AND related_issue_id = $2)
					OR ($4 AND issue_id = $2 AND related_issue_id = $1))`, from, to, remove.kind, remove.kind == "related"); err != nil {
				return err
			}
		}
	}
	if !args.DuplicateOf.Set {
		return nil
	}
	if err := app.Exec(c, tx, "DELETE FROM issue_relations WHERE issue_id = $1 AND type = 'duplicate'", id); err != nil {
		return err
	}
	if !args.DuplicateOf.Valid || isNullWord(args.DuplicateOf.Value) {
		return nil
	}
	other, err := resolveIssue(c, args.DuplicateOf.Value)
	if err != nil {
		return err
	}
	if other == id {
		return app.BadRequest("An issue can't duplicate itself")
	}
	if err := app.Exec(c, tx,
		"INSERT INTO issue_relations (issue_id, related_issue_id, type, creator_id) VALUES ($1, $2, 'duplicate', $3)", id, other, c.User); err != nil {
		return err
	}
	row, err := loadIssueRow(c, tx, id)
	if err != nil {
		return err
	}
	closed, err := app.One[bool](c, tx, "SELECT type IN ('completed', 'canceled') FROM workflow_states WHERE id = $1", row.StateID)
	if err != nil {
		return err
	}
	if !closed {
		canceled, err := firstStateOfType(c, tx, row.TeamID, "canceled")
		if err != nil {
			return err
		}
		if canceled != nil {
			if err := setState(c, tx, id, *canceled, "marked_duplicate"); err != nil {
				return err
			}
		}
	}
	otherIdentifier, err := issueIdentifier(c, tx, other)
	if err != nil {
		return err
	}
	changes["duplicateOf"] = otherIdentifier
	return nil
}

func applyReleases(c *app.Ctx, tx pgx.Tx, id uuid.UUID, args SaveIssueArgs) error {
	if args.SetReleases != nil && (args.AddReleases != nil || args.RemoveReleases != nil) {
		return app.BadRequest("Pass either setReleases or addReleases/removeReleases, not both")
	}
	if args.SetReleases != nil {
		ids, err := resolveReleases(c, args.SetReleases)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM issue_releases WHERE issue_id = $1", id); err != nil {
			return err
		}
		if err := app.Exec(c, tx, "INSERT INTO issue_releases (issue_id, release_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING", id, ids); err != nil {
			return err
		}
	}
	if args.AddReleases != nil {
		ids, err := resolveReleases(c, args.AddReleases)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "INSERT INTO issue_releases (issue_id, release_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING", id, ids); err != nil {
			return err
		}
	}
	if args.RemoveReleases != nil {
		ids, err := resolveReleases(c, args.RemoveReleases)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM issue_releases WHERE issue_id = $1 AND release_id = ANY($2)", id, ids); err != nil {
			return err
		}
	}
	return nil
}

/* Trash, archive and sharing */

func markIssue(c *app.Ctx, reference, sql, action, webhookAction string) (any, error) {
	id, err := resolveIssue(c, reference)
	if err != nil {
		return nil, err
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		row, err := loadIssueRow(c, tx, id)
		if err != nil {
			return err
		}
		affected, err := app.All[uuid.UUID](c, tx, sql, id)
		if err != nil {
			return err
		}
		for _, issue := range affected {
			if err := history(c, tx, issue, action, M{}); err != nil {
				return err
			}
		}
		if value, err = issueValue(c, tx, id); err != nil {
			return err
		}
		return emit(c, tx, "Issue", webhookAction, value, nil, &row.TeamID)
	})
	return value, err
}

const withSubIssues = `WITH RECURSIVE tree AS (SELECT id FROM issues WHERE id = $1
	UNION SELECT i.id FROM issues i JOIN tree ON i.parent_id = tree.id)`

func trashIssue(c *app.Ctx, args IDArgs) (any, error) {
	return markIssue(c, args.ID, withSubIssues+
		" UPDATE issues SET trashed_at = now(), updated_at = now() WHERE id IN (SELECT id FROM tree) AND trashed_at IS NULL RETURNING id",
		"trashed", "remove")
}

func restoreIssue(c *app.Ctx, args IDArgs) (any, error) {
	return markIssue(c, args.ID, withSubIssues+
		" UPDATE issues SET trashed_at = NULL, updated_at = now() WHERE id IN (SELECT id FROM tree) AND trashed_at IS NOT NULL RETURNING id",
		"restored", "update")
}

func archiveIssue(c *app.Ctx, args IDArgs) (any, error) {
	return markIssue(c, args.ID, withSubIssues+
		" UPDATE issues SET archived_at = now(), updated_at = now() WHERE id IN (SELECT id FROM tree) AND archived_at IS NULL RETURNING id",
		"archived", "update")
}

func unarchiveIssue(c *app.Ctx, args IDArgs) (any, error) {
	return markIssue(c, args.ID,
		"UPDATE issues SET archived_at = NULL, auto_archived_at = NULL, updated_at = now() WHERE id = $1 AND archived_at IS NOT NULL RETURNING id",
		"unarchived", "update")
}

type ShareIssueArgs struct {
	ID   string `json:"id" doc:"Identifier (ENG-123) or id of the issue."`
	User string `json:"user" doc:"Workspace member to share with: id, name, email or \"me\"."`
}

func shareIssue(c *app.Ctx, args ShareIssueArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	id, err := resolveIssue(c, args.ID)
	if err != nil {
		return nil, err
	}
	user, err := resolveUser(c, args.User)
	if err != nil {
		return nil, err
	}
	var ident string
	err = inTx(c, func(tx pgx.Tx) error {
		if err := app.Exec(c, tx, "INSERT INTO issue_shares (issue_id, user_id, sharer_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING", id, user, c.User); err != nil {
			return err
		}
		if err := subscribe(c, tx, id, []uuid.UUID{user}); err != nil {
			return err
		}
		if ident, err = issueIdentifier(c, tx, id); err != nil {
			return err
		}
		if err := notify(c, tx, []uuid.UUID{user}, Notice{Kind: "issueShared", Summary: ident + " was shared with you", Issue: &id}); err != nil {
			return err
		}
		return history(c, tx, id, "shared", M{"user": user})
	})
	if err != nil {
		return nil, err
	}
	return M{"shared": ident, "with": user}, nil
}

func unshareIssue(c *app.Ctx, args ShareIssueArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	id, err := resolveIssue(c, args.ID)
	if err != nil {
		return nil, err
	}
	user, err := resolveUser(c, args.User)
	if err != nil {
		return nil, err
	}
	if err := app.Exec(c, c.DB(), "DELETE FROM issue_shares WHERE issue_id = $1 AND user_id = $2", id, user); err != nil {
		return nil, err
	}
	return M{"unshared": args.ID, "with": user}, nil
}
