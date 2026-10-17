package ops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
)

// Importing issues from another tracker in one transaction, keeping their numbers, dates, people
// and comments.

const maxImportIssues = 2000

func importOps() []*Op {
	return []*Op{
		op("import_issues", app.Write, []string{"POST /issues/import"},
			"Import issues from another tracker in one go (workspace admins). An issue can keep its number, so ENG-42 "+
				"stays ENG-42 (the number must be free in the team; gaps are fine), along with its creator, dates, status, "+
				"labels, assignee, project, links, subscribers and comments with their authors and dates. Issues point at "+
				"each other (parent, relatedTo, blocks, blockedBy, duplicateOf) by identifier, within the import or already "+
				"in Ricrac. Pass the issues inline, or upload them as a JSON file with prepare_file_upload and pass its "+
				"fileId, which keeps long descriptions out of the conversation. All or nothing, and nobody is notified. "+
				"dryRun checks everything and reports without importing.",
			importIssues).returns(ResultOf[ImportedIssues]()),
	}
}

type ImportArgs struct {
	Team   string        `json:"team" doc:"Team key, name or id the issues go to."`
	Issues []ImportIssue `json:"issues" doc:"The issues, at most 2000. Pass these or fileId."`
	FileID *string       `json:"fileId" doc:"A JSON file from prepare_file_upload holding the issues as an array, or as {\"issues\": [...]}."`
	DryRun bool          `json:"dryRun,omitempty" doc:"Check everything and report what would be imported, without importing."`
}

type ImportIssue struct {
	Number      *int            `json:"number" doc:"The number to keep, e.g. 42 for ENG-42; must be free in the team. Without it the issue takes the next one."`
	Title       string          `json:"title"`
	Description *string         `json:"description" doc:"Markdown, kept as is. Mentions don't notify anyone."`
	State       *string         `json:"state" doc:"Status name, type or id; the team's default when left out."`
	Priority    *int            `json:"priority" doc:"0 none, 1 urgent, 2 high, 3 medium, 4 low."`
	Estimate    *float64        `json:"estimate" doc:"Points on the team's scale."`
	Assignee    *string         `json:"assignee" doc:"Person (id, name, email); a member of the workspace."`
	Creator     *string         `json:"creator" doc:"Who created it (id, name, email); you when left out."`
	Labels      []string        `json:"labels" doc:"Label names or ids (Group/Child for a label in a group)."`
	Project     *string         `json:"project" doc:"Project name, slug or id."`
	Milestone   *string         `json:"milestone" doc:"Milestone name or id of the project."`
	Cycle       *string         `json:"cycle" doc:"Cycle number, name or id of the team."`
	Parent      *string         `json:"parent" doc:"The parent issue's identifier, within the import or already in Ricrac."`
	DueDate     *string         `json:"dueDate" doc:"YYYY-MM-DD."`
	SortOrder   *float64        `json:"sortOrder" doc:"Position on the team's board (lower is higher up)."`
	CreatedAt   *string         `json:"createdAt" doc:"ISO 8601; now when left out."`
	UpdatedAt   *string         `json:"updatedAt" doc:"ISO 8601; the latest of its other dates when left out."`
	StartedAt   *string         `json:"startedAt" doc:"ISO 8601; createdAt when left out and the status is started or completed."`
	CompletedAt *string         `json:"completedAt" doc:"ISO 8601; createdAt when left out and the status is completed."`
	CanceledAt  *string         `json:"canceledAt" doc:"ISO 8601; createdAt when left out and the status is canceled."`
	ArchivedAt  *string         `json:"archivedAt" doc:"ISO 8601; archives the issue."`
	Links       []ImportLink    `json:"links" doc:"Links to attach."`
	Subscribers []string        `json:"subscribers" doc:"People subscribed besides the creator and assignee."`
	RelatedTo   []string        `json:"relatedTo" doc:"Identifiers of related issues."`
	Blocks      []string        `json:"blocks" doc:"Identifiers of issues this one blocks."`
	BlockedBy   []string        `json:"blockedBy" doc:"Identifiers of issues blocking this one."`
	DuplicateOf *string         `json:"duplicateOf" doc:"Identifier of the issue this duplicates."`
	Comments    []ImportComment `json:"comments" doc:"Comments, oldest first."`
}

type ImportLink struct {
	URL      string  `json:"url"`
	Title    string  `json:"title"`
	Subtitle *string `json:"subtitle"`
}

type ImportComment struct {
	Body      string  `json:"body" doc:"Markdown."`
	Author    *string `json:"author" doc:"Person (id, name, email); you when left out."`
	CreatedAt *string `json:"createdAt" doc:"ISO 8601; the issue's createdAt when left out."`
}

func importIssues(c *app.Ctx, args ImportArgs) (any, error) {
	if err := c.RequireAdmin(); err != nil {
		return nil, err
	}
	issues := args.Issues
	switch {
	case args.Issues != nil && args.FileID == nil:
	case args.Issues == nil && args.FileID != nil:
		var err error
		if issues, err = importFromFile(c, *args.FileID); err != nil {
			return nil, err
		}
	default:
		return nil, app.BadRequest("Pass either issues or fileId")
	}
	if len(issues) == 0 || len(issues) > maxImportIssues {
		return nil, app.BadRequest("Import 1 to %d issues at a time", maxImportIssues)
	}
	team, err := resolveTeam(c, args.Team)
	if err != nil {
		return nil, err
	}
	tx, err := c.DB().Begin(c)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(c)
	key, err := app.One[string](c, tx, "SELECT upper(key) FROM teams WHERE id = $1 FOR UPDATE", team)
	if err != nil {
		return nil, err
	}
	numbers, err := assignNumbers(c, tx, team, key, issues)
	if err != nil {
		return nil, err
	}
	ids := map[int]uuid.UUID{}
	comments := 0
	for i, issue := range issues {
		id, err := insertImported(c, tx, team, numbers[i], issue)
		if err != nil {
			return nil, importedAt(key, numbers[i], err)
		}
		comments += len(issue.Comments)
		ids[numbers[i]] = id
	}
	relations := 0
	for i, issue := range issues {
		added, err := connectImported(c, tx, key, ids, ids[numbers[i]], issue)
		if err != nil {
			return nil, importedAt(key, numbers[i], err)
		}
		relations += added
	}
	count, err := app.One[int](c, tx,
		"UPDATE teams SET issue_count = greatest(issue_count, $2) WHERE id = $1 RETURNING issue_count", team, slices.Max(numbers))
	if err != nil {
		return nil, err
	}
	for _, number := range numbers {
		value, err := issueValue(c, tx, ids[number])
		if err != nil {
			return nil, err
		}
		if err := emit(c, tx, "Issue", "create", value, nil, &team); err != nil {
			return nil, err
		}
	}
	if err := audit(c, tx, "issues.imported", M{"team": key, "count": len(issues)}); err != nil {
		return nil, err
	}
	if !args.DryRun {
		if err := tx.Commit(c); err != nil {
			return nil, err
		}
	}
	identifiers := make([]string, len(numbers))
	for i, n := range numbers {
		identifiers[i] = fmt.Sprintf("%s-%d", key, n)
	}
	return M{
		"dryRun":         args.DryRun,
		"imported":       len(identifiers),
		"comments":       comments,
		"relations":      relations,
		"identifiers":    identifiers,
		"nextIdentifier": fmt.Sprintf("%s-%d", key, count+1),
	}, nil
}

func importFromFile(c *app.Ctx, reference string) ([]ImportIssue, error) {
	notFound := app.NotFound("No uploaded file %q", strings.TrimSpace(reference))
	file, ok := asUUID(reference)
	if !ok {
		return nil, notFound
	}
	data, found, err := app.Maybe[*[]byte](c, c.DB(), "SELECT data FROM files WHERE id = $1 AND workspace_id = $2", file, c.Workspace)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound
	}
	if data == nil {
		return nil, app.BadRequest("The file hasn't been uploaded yet; PUT it to its uploadUrl")
	}
	var parsed any
	if err := json.Unmarshal(*data, &parsed); err != nil {
		return nil, app.BadRequest("The file isn't JSON: %s", err)
	}
	if object, ok := parsed.(M); ok {
		if list, ok := object["issues"]; ok {
			parsed = list
		}
	}
	var issues []ImportIssue
	decoder := json.NewDecoder(bytes.NewReader(app.Marshal(parsed)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&issues); err != nil {
		return nil, app.BadRequest("The file's issues are invalid: %s", describeDecodeError(err))
	}
	return issues, nil
}

// assignNumbers is the numbers given, checked to be free, and the next ones after everything
// else for the rest.
func assignNumbers(c *app.Ctx, tx pgx.Tx, team uuid.UUID, key string, issues []ImportIssue) ([]int, error) {
	given := []int{}
	for _, issue := range issues {
		if issue.Number != nil {
			given = append(given, *issue.Number)
		}
	}
	for _, n := range given {
		if n < 1 {
			return nil, app.BadRequest("%d isn't an issue number; numbers start at 1", n)
		}
	}
	seen := map[int]bool{}
	for _, n := range given {
		if seen[n] {
			return nil, app.BadRequest("%s-%d appears twice in the import", key, n)
		}
		seen[n] = true
	}
	taken, err := app.All[int](c, tx, "SELECT number FROM issues WHERE team_id = $1 AND number = ANY($2) ORDER BY number", team, given)
	if err != nil {
		return nil, err
	}
	if len(taken) > 0 {
		list := make([]string, min(len(taken), 20))
		for i := range list {
			list[i] = fmt.Sprintf("%s-%d", key, taken[i])
		}
		which := "These numbers are"
		if len(taken) == 1 {
			which = "This number is"
		}
		return nil, app.BadRequest("%s taken in the team: %s", which, strings.Join(list, ", "))
	}
	next, err := app.One[int](c, tx, "SELECT issue_count FROM teams WHERE id = $1", team)
	if err != nil {
		return nil, err
	}
	for _, n := range given {
		next = max(next, n)
	}
	numbers := make([]int, len(issues))
	for i, issue := range issues {
		if issue.Number != nil {
			numbers[i] = *issue.Number
			continue
		}
		next++
		numbers[i] = next
	}
	return numbers, nil
}

// importedAt says which issue of the import an error is about.
func importedAt(key string, number int, err error) error {
	apiErr := app.AsError(err)
	return app.Errorf(apiErr.Status, "%s-%d: %s", key, number, apiErr.Message)
}

func optionalTime(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	at, err := parseDatetime(*value)
	if err != nil {
		return nil, err
	}
	return &at, nil
}

func optionalUser(c *app.Ctx, value *string) (*uuid.UUID, error) {
	if value == nil {
		return nil, nil
	}
	id, err := resolveUser(c, *value)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func insertImported(c *app.Ctx, tx pgx.Tx, team uuid.UUID, number int, issue ImportIssue) (uuid.UUID, error) {
	title, err := nonEmpty(issue.Title, "title")
	if err != nil {
		return uuid.Nil, err
	}
	var state uuid.UUID
	if issue.State != nil {
		state, err = resolveState(c, team, *issue.State)
	} else {
		state, err = defaultState(c, tx, team)
	}
	if err != nil {
		return uuid.Nil, err
	}
	kind, err := stateType(c, tx, state)
	if err != nil {
		return uuid.Nil, err
	}
	var estimate *float64
	if issue.Estimate != nil {
		settings, err := loadTeamSettings(c, tx, team)
		if err != nil {
			return uuid.Nil, err
		}
		e, err := checkEstimate(settings, *issue.Estimate)
		if err != nil {
			return uuid.Nil, err
		}
		estimate = &e
	}
	creator := c.User
	if found, err := optionalUser(c, issue.Creator); err != nil {
		return uuid.Nil, err
	} else if found != nil {
		creator = *found
	}
	assignee, err := optionalUser(c, issue.Assignee)
	if err != nil {
		return uuid.Nil, err
	}
	var project, milestone, cycle *uuid.UUID
	if issue.Project != nil {
		id, err := resolveProject(c, *issue.Project)
		if err != nil {
			return uuid.Nil, err
		}
		project = &id
	}
	if issue.Milestone != nil {
		id, err := resolveMilestone(c, project, *issue.Milestone)
		if err != nil {
			return uuid.Nil, err
		}
		milestone = &id
	}
	if issue.Cycle != nil {
		id, err := resolveCycle(c, &team, *issue.Cycle)
		if err != nil {
			return uuid.Nil, err
		}
		cycle = &id
	}
	created := time.Now().UTC()
	if at, err := optionalTime(issue.CreatedAt); err != nil {
		return uuid.Nil, err
	} else if at != nil {
		created = *at
	}
	stamp := func(value *string, applies bool) (*time.Time, error) {
		at, err := optionalTime(value)
		if err != nil || at != nil || !applies {
			return at, err
		}
		return &created, nil
	}
	started, err := stamp(issue.StartedAt, kind == "started" || kind == "completed")
	if err != nil {
		return uuid.Nil, err
	}
	completed, err := stamp(issue.CompletedAt, kind == "completed")
	if err != nil {
		return uuid.Nil, err
	}
	canceled, err := stamp(issue.CanceledAt, kind == "canceled")
	if err != nil {
		return uuid.Nil, err
	}
	archived, err := optionalTime(issue.ArchivedAt)
	if err != nil {
		return uuid.Nil, err
	}
	commentTimes := make([]time.Time, len(issue.Comments))
	for i, comment := range issue.Comments {
		at, err := optionalTime(comment.CreatedAt)
		if err != nil {
			return uuid.Nil, err
		}
		commentTimes[i] = created
		if at != nil {
			commentTimes[i] = *at
		}
	}
	updated, err := optionalTime(issue.UpdatedAt)
	if err != nil {
		return uuid.Nil, err
	}
	if updated == nil {
		latest := created
		for _, at := range append([]*time.Time{started, completed, canceled, archived}, pointers(commentTimes)...) {
			if at != nil && at.After(latest) {
				latest = *at
			}
		}
		updated = &latest
	}
	var due *time.Time
	if issue.DueDate != nil {
		date, err := parseDate(*issue.DueDate)
		if err != nil {
			return uuid.Nil, err
		}
		due = &date
	}
	var sortOrder float64
	if issue.SortOrder != nil {
		sortOrder = *issue.SortOrder
	} else if sortOrder, err = app.One[float64](c, tx, "SELECT coalesce(min(sort_order), 0) - 1000 FROM issues WHERE team_id = $1", team); err != nil {
		return uuid.Nil, err
	}
	priority := 0
	if issue.Priority != nil {
		priority = *issue.Priority
	}
	if priority, err = checkPriority(priority); err != nil {
		return uuid.Nil, err
	}
	id, err := app.One[uuid.UUID](c, tx,
		`INSERT INTO issues (workspace_id, team_id, number, title, description, state_id, priority, estimate,
			assignee_id, creator_id, project_id, milestone_id, cycle_id, due_date, sort_order,
			started_at, completed_at, canceled_at, archived_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, coalesce($11, (SELECT project_id FROM project_milestones WHERE id = $12)),
			$12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
		 RETURNING id`,
		c.Workspace, team, number, title, orEmpty(issue.Description), state, priority, estimate, assignee, creator,
		project, milestone, cycle, due, sortOrder, started, completed, canceled, archived, created, *updated)
	if err != nil {
		return uuid.Nil, err
	}
	if project != nil {
		if err := app.Exec(c, tx, "INSERT INTO project_teams (project_id, team_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", *project, team); err != nil {
			return uuid.Nil, err
		}
	}
	if issue.Labels != nil {
		labels, err := resolveLabels(c, &team, issue.Labels)
		if err != nil {
			return uuid.Nil, err
		}
		group, clashes, err := app.Maybe[string](c, tx,
			`SELECT p.name FROM issue_labels l JOIN issue_labels p ON p.id = l.parent_id
			 WHERE l.id = ANY($1) AND p.group_type = 'singleSelect' GROUP BY p.name HAVING count(*) > 1 LIMIT 1`, labels)
		if err != nil {
			return uuid.Nil, err
		}
		if clashes {
			return uuid.Nil, app.BadRequest("An issue takes one label of the group %q", group)
		}
		if err := app.Exec(c, tx,
			"INSERT INTO issue_label_links (issue_id, label_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING", id, labels); err != nil {
			return uuid.Nil, err
		}
	}
	subscribers := []uuid.UUID{creator}
	if assignee != nil {
		subscribers = append(subscribers, *assignee)
	}
	others, err := resolveUsers(c, issue.Subscribers)
	if err != nil {
		return uuid.Nil, err
	}
	if err := subscribe(c, tx, id, append(subscribers, others...)); err != nil {
		return uuid.Nil, err
	}
	for _, link := range issue.Links {
		if _, err := upsertAttachment(c, tx, id, link.URL, link.Title, link.Subtitle, nil); err != nil {
			return uuid.Nil, err
		}
	}
	for i, comment := range issue.Comments {
		author := c.User
		if found, err := optionalUser(c, comment.Author); err != nil {
			return uuid.Nil, err
		} else if found != nil {
			author = *found
		}
		body, err := nonEmpty(comment.Body, "comment body")
		if err != nil {
			return uuid.Nil, err
		}
		if err := app.Exec(c, tx,
			"INSERT INTO comments (workspace_id, issue_id, user_id, body, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $5)",
			c.Workspace, id, author, body, commentTimes[i]); err != nil {
			return uuid.Nil, err
		}
	}
	if err := app.Exec(c, tx,
		"INSERT INTO issue_history (issue_id, actor_id, action, changes, created_at) VALUES ($1, $2, 'created', $3, $4)",
		id, creator, M{"importedBy": c.User}, created); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func pointers(times []time.Time) []*time.Time {
	list := make([]*time.Time, len(times))
	for i := range times {
		list[i] = &times[i]
	}
	return list
}

// findImported is an issue of this import by its identifier, else one already in Ricrac.
func findImported(c *app.Ctx, key string, ids map[int]uuid.UUID, value string) (uuid.UUID, error) {
	if k, number, ok := parseIdentifier(value); ok && k == key {
		if id, found := ids[number]; found {
			return id, nil
		}
	}
	return resolveIssue(c, value)
}

// connectImported adds the parent, relations and duplicates, once every issue of the import
// exists. Returns how many relations were added.
func connectImported(c *app.Ctx, tx pgx.Tx, key string, ids map[int]uuid.UUID, id uuid.UUID, issue ImportIssue) (int, error) {
	if issue.Parent != nil {
		parent, err := findImported(c, key, ids, *issue.Parent)
		if err != nil {
			return 0, err
		}
		loops, err := app.One[bool](c, tx,
			`WITH RECURSIVE up AS (SELECT id, parent_id FROM issues WHERE id = $1
			 UNION SELECT i.id, i.parent_id FROM issues i JOIN up ON i.id = up.parent_id)
			 SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)`, parent, id)
		if err != nil {
			return 0, err
		}
		if loops {
			return 0, app.BadRequest("An issue can't be a sub-issue of itself or of its own sub-issues")
		}
		if err := app.Exec(c, tx,
			`UPDATE issues SET parent_id = $2,
				sub_issue_sort_order = (SELECT coalesce(max(sub_issue_sort_order), 0) + 1000 FROM issues WHERE parent_id = $2)
			 WHERE id = $1`, id, parent); err != nil {
			return 0, err
		}
	}
	var duplicate []string
	if issue.DuplicateOf != nil {
		duplicate = []string{*issue.DuplicateOf}
	}
	lists := []struct {
		values  []string
		kind    string
		reverse bool
	}{
		{issue.RelatedTo, "related", false},
		{issue.Blocks, "blocks", false},
		{issue.BlockedBy, "blocks", true},
		{duplicate, "duplicate", false},
	}
	added := 0
	for _, list := range lists {
		for _, value := range list.values {
			other, err := findImported(c, key, ids, value)
			if err != nil {
				return 0, err
			}
			if other == id {
				return 0, app.BadRequest("An issue can't be related to itself")
			}
			from, to := id, other
			if list.reverse {
				from, to = other, id
			}
			tag, err := tx.Exec(c,
				`INSERT INTO issue_relations (issue_id, related_issue_id, type, creator_id)
				 SELECT $1, $2, $3, $4 WHERE NOT ($3 = 'related' AND EXISTS (SELECT 1 FROM issue_relations
					WHERE type = 'related' AND issue_id = $2 AND related_issue_id = $1))
				 ON CONFLICT DO NOTHING`, from, to, list.kind, c.User)
			if err != nil {
				return 0, err
			}
			added += int(tag.RowsAffected())
		}
	}
	return added, nil
}

/* Shapes */

type ImportedIssues struct {
	DryRun         bool     `json:"dryRun" doc:"Nothing was imported, only checked."`
	Imported       int      `json:"imported" doc:"How many issues."`
	Comments       int      `json:"comments" doc:"How many comments."`
	Relations      int      `json:"relations" doc:"How many relations (relatedTo, blocks, blockedBy, duplicateOf)."`
	Identifiers    []string `json:"identifiers"`
	NextIdentifier string   `json:"nextIdentifier" doc:"The identifier the team's next issue gets."`
}
