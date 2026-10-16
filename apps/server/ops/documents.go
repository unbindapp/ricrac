package ops

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
)

// Documents: Markdown pages attached to a project, initiative, team, issue, cycle or release,
// with earlier versions kept as they are rewritten.

func documentOps() []*Op {
	return []*Op{
		op("list_documents", app.Read, []string{"GET /documents"},
			"List documents, most recently updated first. Narrow to those of a project, initiative, team, issue, "+
				"cycle or release, by creator, or by words in the title or content. Contents are left out unless "+
				"includeContent is set; read one with get_document.",
			listDocuments).returns(ResultOf[PageOf[Document]]()),
		op("get_document", app.Read, []string{"GET /documents/{id}"},
			"Read a document by id, slug, link or title: its Markdown content, where it belongs and how many earlier "+
				"versions are kept.",
			getDocument).returns(ResultOf[DocumentDetail]()),
		op("save_document", app.Write, []string{"POST /documents", "PATCH /documents/{id}"},
			"Create a document (omit id; title and exactly one of project, initiative, team, issue, cycle or release "+
				"are required, unless a template gives the title) or update one (pass id). Use patch to edit part of the "+
				"content without resending it. Passing a parent on update moves the document there. People mentioned "+
				"with @displayName or @email are notified. The previous content is kept as a version.",
			saveDocument).returns(ResultOf[DocumentDetail]()),
		op("delete_document", app.Write, []string{"DELETE /documents/{id}"},
			"Move a document to the trash. It is deleted for good after 30 days; restore_document brings it back.",
			trashDocument).returns(ResultOf[DocumentDetail]()),
		op("restore_document", app.Write, []string{"POST /documents/{id}/restore"},
			"Bring a document back from the trash.",
			restoreDocument).returns(ResultOf[DocumentDetail]()),
		op("list_document_versions", app.Read, []string{"GET /documents/{id}/versions"},
			"The earlier versions of a document, newest first (without their content; read one with "+
				"get_document_version).",
			listDocumentVersions).returns(ResultOf[[]DocumentVersionSummary]()),
		op("get_document_version", app.Read, []string{"GET /document-versions/{id}"},
			"Read an earlier version of a document: its title and content as they were.",
			getDocumentVersion).returns(ResultOf[DocumentVersion]()),
	}
}

/* Representation */

const documentFrom = "FROM documents d JOIN workspaces w ON w.id = d.workspace_id"

// documentJSON is the JSON of the document d (with its workspace w joined).
func documentJSON(c *app.Ctx, withContent bool) string {
	content := ""
	if withContent {
		content = "'content', d.content,"
	}
	return fmt.Sprintf(`jsonb_build_object(
		'id', d.id,
		'slugId', d.slug_id,
		'title', d.title,
		%s
		'url', %s || '/' || w.url_key || '/document/' || concat_ws('-', nullif(trim(both '-' from left(
			trim(both '-' from regexp_replace(lower(d.title), '[^a-z0-9]+', '-', 'g')), 60)), ''), d.slug_id),
		'icon', d.icon,
		'color', d.color,
		'project', project_ref(d.project_id),
		'initiative', initiative_ref(d.initiative_id),
		'team', team_ref(d.team_id),
		'issue', issue_ref(d.issue_id),
		'cycle', cycle_ref(d.cycle_id),
		'release', release_ref(d.release_id),
		'creator', user_ref(d.creator_id),
		'updatedBy', user_ref(d.updated_by_id),
		'trashed', d.trashed_at IS NOT NULL,
		'trashedAt', d.trashed_at,
		'createdAt', d.created_at,
		'updatedAt', d.updated_at
	)`, content, sqlLiteral(c.PublicURL()))
}

func documentValue(c *app.Ctx, q app.Querier, id uuid.UUID) (M, error) {
	value, found, err := app.Maybe[M](c, q, fmt.Sprintf(
		`SELECT %s || jsonb_build_object('versionCount', (SELECT count(*) FROM document_versions v WHERE v.document_id = d.id))
		 %s WHERE d.id = $1`, documentJSON(c, true), documentFrom), id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, app.NotFound("No such document")
	}
	return value, nil
}

// pushDocumentVisible appends AND <the caller can see document d>, as canSeeDocument decides: by
// its team, project, issue or cycle; documents of initiatives and releases are visible to
// everyone.
func pushDocumentVisible(q *app.SQL, c *app.Ctx, teams []uuid.UUID, d string) {
	q.Push(fmt.Sprintf(" AND CASE WHEN %s.team_id IS NOT NULL THEN %s.team_id = ANY(", d, d)).Bind(teams)
	q.Push(fmt.Sprintf(`) WHEN %s.project_id IS NOT NULL THEN EXISTS (SELECT 1 FROM projects vp JOIN project_teams vpt ON vpt.project_id = vp.id
			WHERE vp.id = %s.project_id AND vp.trashed_at IS NULL AND vpt.team_id = ANY(`, d, d)).Bind(teams)
	q.Push(fmt.Sprintf(
		`)) WHEN %s.issue_id IS NOT NULL THEN EXISTS (SELECT 1 FROM issues vi WHERE vi.id = %s.issue_id AND (vi.team_id = ANY(`, d, d)).Bind(teams)
	q.Push(") OR EXISTS (SELECT 1 FROM issue_shares vs WHERE vs.issue_id = vi.id AND vs.user_id = ").Bind(c.User)
	q.Push(fmt.Sprintf(
		`))) WHEN %s.cycle_id IS NOT NULL THEN EXISTS (SELECT 1 FROM cycles vc WHERE vc.id = %s.cycle_id AND vc.team_id = ANY(`, d, d)).Bind(teams)
	q.Push(")) ELSE TRUE END")
}

// findDocument is a document the caller can see, by id, slug, link or title; with trashed, only
// among those in the trash.
func findDocument(c *app.Ctx, value string, trashed bool) (uuid.UUID, error) {
	if !trashed {
		return resolveDocument(c, value)
	}
	value = strings.TrimSpace(value)
	tail := strings.TrimRight(value, "/")
	tail = tail[strings.LastIndexAny(tail, "/-")+1:]
	var asID *uuid.UUID
	if id, ok := asUUID(value); ok {
		asID = &id
	}
	notFound := app.NotFound("No document %q in the trash", value)
	id, found, err := app.Maybe[uuid.UUID](c, c.DB(),
		`SELECT id FROM documents WHERE workspace_id = $1 AND trashed_at IS NOT NULL
		 AND (id = $2 OR slug_id = $3 OR lower(title) = lower($4)) ORDER BY trashed_at DESC LIMIT 1`,
		c.Workspace, asID, tail, value)
	if err != nil {
		return uuid.Nil, err
	}
	if !found {
		return uuid.Nil, notFound
	}
	visible, err := canSeeDocument(c, id)
	if err != nil {
		return uuid.Nil, err
	}
	if !visible {
		return uuid.Nil, notFound
	}
	return id, nil
}

/* Read */

type ListDocumentsArgs struct {
	Project        *string `json:"project" doc:"Project name, slug or id."`
	Initiative     *string `json:"initiative" doc:"Initiative name, slug or id."`
	Team           *string `json:"team" doc:"Team key, name or id: the team's documents (with cycle, only the team the cycle belongs to)."`
	Issue          *string `json:"issue" doc:"Issue identifier (ENG-123) or id."`
	Cycle          *string `json:"cycle" doc:"Cycle id, or its number, name or \"current\" with team."`
	Release        *string `json:"release" doc:"Release name, slug or id."`
	Creator        *string `json:"creator" doc:"Person who created the documents (id, name, email or \"me\")."`
	Query          *string `json:"query" doc:"Words to find in the title or content."`
	IncludeTrashed bool    `json:"includeTrashed,omitempty"`
	IncludeContent bool    `json:"includeContent,omitempty" doc:"Include each document's content."`
	OrderBy        *string `json:"orderBy" doc:"updatedAt (default) or createdAt, newest first."`
	Page
}

type documentParent struct {
	column string
	id     uuid.UUID
}

func listDocuments(c *app.Ctx, args ListDocumentsArgs) (any, error) {
	orderBy := "updatedAt"
	if args.OrderBy != nil {
		orderBy = *args.OrderBy
	}
	order, ok := map[string]string{"updatedAt": "d.updated_at DESC", "createdAt": "d.created_at DESC"}[orderBy]
	if !ok {
		return nil, app.BadRequest("orderBy %q isn't updatedAt or createdAt", orderBy)
	}
	var team *uuid.UUID
	if args.Team != nil {
		id, err := resolveTeam(c, *args.Team)
		if err != nil {
			return nil, err
		}
		team = &id
	}
	var parents []documentParent
	for _, p := range []struct {
		column  string
		value   *string
		resolve func(*app.Ctx, string) (uuid.UUID, error)
	}{
		{"project_id", args.Project, resolveProject},
		{"initiative_id", args.Initiative, resolveInitiative},
		{"issue_id", args.Issue, resolveIssue},
		{"release_id", args.Release, resolveRelease},
	} {
		if p.value == nil {
			continue
		}
		id, err := p.resolve(c, *p.value)
		if err != nil {
			return nil, err
		}
		parents = append(parents, documentParent{p.column, id})
	}
	switch {
	case args.Cycle != nil:
		id, err := resolveCycle(c, team, *args.Cycle)
		if err != nil {
			return nil, err
		}
		parents = append(parents, documentParent{"cycle_id", id})
	case team != nil:
		parents = append(parents, documentParent{"team_id", *team})
	}
	teams, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	q := app.NewSQL(fmt.Sprintf("SELECT %s %s WHERE d.workspace_id = ", documentJSON(c, args.IncludeContent), documentFrom)).Bind(c.Workspace)
	pushDocumentVisible(q, c, teams, "d")
	for _, p := range parents {
		q.Push(fmt.Sprintf(" AND d.%s = ", p.column)).Bind(p.id)
	}
	if args.Creator != nil {
		creator, err := resolveUser(c, *args.Creator)
		if err != nil {
			return nil, err
		}
		q.Push(" AND d.creator_id = ").Bind(creator)
	}
	if !args.IncludeTrashed {
		q.Push(" AND d.trashed_at IS NULL")
	}
	if query := strings.TrimSpace(orEmpty(args.Query)); query != "" {
		q.Push(" AND (d.search @@ websearch_to_tsquery('english', ").Bind(query)
		q.Push(") OR d.title ILIKE ").Bind("%" + escapeLike(query) + "%").Push(")")
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

func getDocument(c *app.Ctx, args IDArgs) (any, error) {
	id, err := findDocument(c, args.ID, false)
	if err != nil {
		return nil, err
	}
	return documentValue(c, c.DB(), id)
}

/* Save */

type SaveDocumentArgs struct {
	ID             *string          `json:"id" doc:"Only to update a document: its id, slug, link or title. Omit to create one."`
	Title          *string          `json:"title"`
	Content        *string          `json:"content" doc:"Markdown. Mention people with @displayName or @email; they are notified."`
	Patch          []TextEdit       `json:"patch" doc:"Edits to the current content instead of the whole text, applied in order, all or none."`
	Icon           Nullable[string] `json:"icon" doc:"An emoji or icon name; null clears it."`
	Color          Nullable[string] `json:"color" doc:"Hex color like #5e6ad2; null clears it."`
	Project        *string          `json:"project" doc:"Parent project: name, slug or id."`
	Initiative     *string          `json:"initiative" doc:"Parent initiative: name, slug or id."`
	Team           *string          `json:"team" doc:"Parent team: key, name or id (or the team of the cycle, with cycle)."`
	Issue          *string          `json:"issue" doc:"Parent issue: identifier (ENG-123) or id."`
	Cycle          *string          `json:"cycle" doc:"Parent cycle: id, or number, name or \"current\" with team."`
	Release        *string          `json:"release" doc:"Parent release: name, slug or id."`
	Template       *string          `json:"template" doc:"Document template name or id, applied on create: it gives the title and content the arguments don't."`
	RestoreVersion *int64           `json:"restoreVersion" doc:"An earlier version (its id from list_document_versions) to bring back on update: its title and content replace the current ones, which are kept as a version."`
}

type documentParents struct {
	Project, Initiative, Team, Issue, Cycle, Release *uuid.UUID
}

// parentOf is the one parent named in the arguments, or nil when none is.
func parentOf(c *app.Ctx, args SaveDocumentArgs) (*documentParents, error) {
	var team *uuid.UUID
	if args.Team != nil {
		id, err := resolveTeam(c, *args.Team)
		if err != nil {
			return nil, err
		}
		team = &id
	}
	named := []bool{
		args.Project != nil,
		args.Initiative != nil,
		args.Team != nil && args.Cycle == nil,
		args.Issue != nil,
		args.Cycle != nil,
		args.Release != nil,
	}
	switch count := len(slices.DeleteFunc(named, func(n bool) bool { return !n })); {
	case count == 0:
		return nil, nil
	case count > 1:
		return nil, app.BadRequest("A document belongs to one of project, initiative, team, issue, cycle or release; pass only one")
	}
	var parent documentParents
	var id uuid.UUID
	var err error
	switch {
	case args.Project != nil:
		id, err = resolveProject(c, *args.Project)
		parent.Project = &id
	case args.Initiative != nil:
		id, err = resolveInitiative(c, *args.Initiative)
		parent.Initiative = &id
	case args.Issue != nil:
		id, err = resolveIssue(c, *args.Issue)
		parent.Issue = &id
	case args.Cycle != nil:
		id, err = resolveCycle(c, team, *args.Cycle)
		parent.Cycle = &id
	case args.Release != nil:
		id, err = resolveRelease(c, *args.Release)
		parent.Release = &id
	default:
		parent.Team = team
	}
	if err != nil {
		return nil, err
	}
	return &parent, nil
}

func saveDocument(c *app.Ctx, args SaveDocumentArgs) (any, error) {
	var value M
	err := inTx(c, func(tx pgx.Tx) error {
		var id uuid.UUID
		var err error
		action := "create"
		if args.ID != nil {
			action = "update"
			if id, err = findDocument(c, *args.ID, false); err != nil {
				return err
			}
			err = updateDocument(c, tx, id, args)
		} else {
			id, err = createDocument(c, tx, args)
		}
		if err != nil {
			return err
		}
		if value, err = documentValue(c, tx, id); err != nil {
			return err
		}
		team, err := documentWebhookTeam(c, tx, id)
		if err != nil {
			return err
		}
		return emit(c, tx, "Document", action, value, nil, team)
	})
	return value, err
}

func documentWebhookTeam(c *app.Ctx, q app.Querier, id uuid.UUID) (*uuid.UUID, error) {
	return app.One[*uuid.UUID](c, q,
		`SELECT coalesce(d.team_id, (SELECT team_id FROM issues WHERE id = d.issue_id),
			(SELECT team_id FROM cycles WHERE id = d.cycle_id)) FROM documents d WHERE d.id = $1`, id)
}

// optionalColor checks a color argument: nil when it is null or left out.
func optionalColor(color Nullable[string]) (*string, error) {
	if !color.Valid {
		return nil, nil
	}
	checked, err := checkColor(color.Value)
	if err != nil {
		return nil, err
	}
	return &checked, nil
}

func createDocument(c *app.Ctx, tx pgx.Tx, args SaveDocumentArgs) (uuid.UUID, error) {
	parent, err := parentOf(c, args)
	if err != nil {
		return uuid.Nil, err
	}
	if parent == nil {
		return uuid.Nil, app.BadRequest("Say where the document belongs: one of project, initiative, team, issue, cycle or release")
	}
	if args.Patch != nil {
		return uuid.Nil, app.BadRequest("patch only applies when updating; pass content")
	}
	if args.RestoreVersion != nil {
		return uuid.Nil, app.BadRequest("restoreVersion only applies when updating")
	}
	title, content := args.Title, args.Content
	if args.Template != nil {
		template, err := resolveTemplate(c, "document", *args.Template)
		if err != nil {
			return uuid.Nil, err
		}
		data, err := app.One[M](c, tx, "SELECT data FROM templates WHERE id = $1", template)
		if err != nil {
			return uuid.Nil, err
		}
		text := func(key string) *string {
			value, ok := data[key].(string)
			if !ok {
				return nil
			}
			return &value
		}
		if title == nil {
			title = text("title")
		}
		if orEmpty(content) == "" {
			content = text("content")
		}
	}
	checkedTitle, err := nonEmpty(orEmpty(title), "title")
	if err != nil {
		return uuid.Nil, err
	}
	color, err := optionalColor(args.Color)
	if err != nil {
		return uuid.Nil, err
	}
	id, err := app.One[uuid.UUID](c, tx,
		`INSERT INTO documents (workspace_id, slug_id, title, content, icon, color, project_id, initiative_id, team_id,
			issue_id, cycle_id, release_id, creator_id, updated_by_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13) RETURNING id`,
		c.Workspace, slugID(), checkedTitle, orEmpty(content), args.Icon.Ptr(), color, parent.Project, parent.Initiative,
		parent.Team, parent.Issue, parent.Cycle, parent.Release, c.User)
	if err != nil {
		return uuid.Nil, err
	}
	return id, notifyDocumentMentions(c, tx, id, checkedTitle, "", orEmpty(content))
}

func updateDocument(c *app.Ctx, tx pgx.Tx, id uuid.UUID, args SaveDocumentArgs) error {
	if args.Template != nil {
		return app.BadRequest("Templates only apply when creating a document")
	}
	type current struct {
		Title     string
		Content   string
		UpdatedBy *uuid.UUID
	}
	row, _, err := app.Row[current](c, tx, "SELECT title, content, updated_by_id FROM documents WHERE id = $1 FOR UPDATE", id)
	if err != nil {
		return err
	}
	title := row.Title
	if args.Title != nil {
		if title, err = nonEmpty(*args.Title, "title"); err != nil {
			return err
		}
	}
	content := row.Content
	replaced, err := newContent(row.Content, args.Content, args.Patch)
	if err != nil {
		return err
	}
	if replaced != nil {
		content = *replaced
	}
	restoring := args.RestoreVersion != nil
	if restoring {
		if title, content, err = versionText(c, tx, id, *args.RestoreVersion, args); err != nil {
			return err
		}
	}
	if title != row.Title || content != row.Content {
		if err := keepVersion(c, tx, id, row.Title, row.Content, row.UpdatedBy, restoring); err != nil {
			return err
		}
		if err := app.Exec(c, tx, "UPDATE documents SET title = $2, content = $3 WHERE id = $1", id, title, content); err != nil {
			return err
		}
		if err := notifyDocumentMentions(c, tx, id, title, row.Content, content); err != nil {
			return err
		}
	}
	if args.Icon.Set {
		if err := app.Exec(c, tx, "UPDATE documents SET icon = $2 WHERE id = $1", id, args.Icon.Ptr()); err != nil {
			return err
		}
	}
	if args.Color.Set {
		color, err := optionalColor(args.Color)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "UPDATE documents SET color = $2 WHERE id = $1", id, color); err != nil {
			return err
		}
	}
	parent, err := parentOf(c, args)
	if err != nil {
		return err
	}
	if parent != nil {
		if err := app.Exec(c, tx,
			`UPDATE documents SET project_id = $2, initiative_id = $3, team_id = $4, issue_id = $5, cycle_id = $6,
				release_id = $7 WHERE id = $1`,
			id, parent.Project, parent.Initiative, parent.Team, parent.Issue, parent.Cycle, parent.Release); err != nil {
			return err
		}
	}
	return app.Exec(c, tx, "UPDATE documents SET updated_at = now(), updated_by_id = $2 WHERE id = $1", id, c.User)
}

// versionText is the title and content of an earlier version of the document, to restore.
func versionText(c *app.Ctx, tx pgx.Tx, id uuid.UUID, version int64, args SaveDocumentArgs) (string, string, error) {
	if args.Title != nil || args.Content != nil || args.Patch != nil {
		return "", "", app.BadRequest("restoreVersion brings back a version's title and content; don't pass title, content or patch with it")
	}
	type text struct{ Title, Content string }
	found, ok, err := app.Row[text](c, tx,
		"SELECT title, content FROM document_versions WHERE id = $1 AND document_id = $2", version, id)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", app.NotFound("No version %d of this document; list_document_versions lists them", version)
	}
	return found.Title, found.Content, nil
}

// keepVersion keeps the current title and content as a version, unless the caller wrote them and
// the last version is less than 10 minutes old: a burst of edits by one person makes one version.
// A restore always keeps one, so it can be undone.
func keepVersion(c *app.Ctx, tx pgx.Tx, id uuid.UUID, title, content string, updatedBy *uuid.UUID, always bool) error {
	recent, err := app.One[bool](c, tx,
		"SELECT EXISTS (SELECT 1 FROM document_versions WHERE document_id = $1 AND created_at > now() - interval '10 minutes')", id)
	if err != nil {
		return err
	}
	if !always && recent && updatedBy != nil && *updatedBy == c.User {
		return nil
	}
	return app.Exec(c, tx, "INSERT INTO document_versions (document_id, title, content, author_id) VALUES ($1, $2, $3, $4)",
		id, title, content, updatedBy)
}

func notifyDocumentMentions(c *app.Ctx, tx pgx.Tx, id uuid.UUID, title, before, after string) error {
	previous, err := mentions(c, before)
	if err != nil {
		return err
	}
	mentioned, err := mentions(c, after)
	if err != nil {
		return err
	}
	mentioned = slices.DeleteFunc(mentioned, func(u uuid.UUID) bool { return slices.Contains(previous, u) })
	if len(mentioned) == 0 {
		return nil
	}
	return notify(c, tx, mentioned, Notice{Kind: "documentMention", Summary: "You were mentioned in " + title, Document: &id})
}

/* Trash and versions */

func setDocumentTrashed(c *app.Ctx, id uuid.UUID, trashed bool) (any, error) {
	var value M
	err := inTx(c, func(tx pgx.Tx) error {
		if err := app.Exec(c, tx,
			`UPDATE documents SET trashed_at = CASE WHEN $2 THEN now() END, updated_at = now(), updated_by_id = $3
			 WHERE id = $1`, id, trashed, c.User); err != nil {
			return err
		}
		var err error
		if value, err = documentValue(c, tx, id); err != nil {
			return err
		}
		team, err := documentWebhookTeam(c, tx, id)
		if err != nil {
			return err
		}
		action := "update"
		if trashed {
			action = "remove"
		}
		return emit(c, tx, "Document", action, value, nil, team)
	})
	return value, err
}

func trashDocument(c *app.Ctx, args IDArgs) (any, error) {
	id, err := findDocument(c, args.ID, false)
	if err != nil {
		return nil, err
	}
	return setDocumentTrashed(c, id, true)
}

func restoreDocument(c *app.Ctx, args IDArgs) (any, error) {
	id, err := findDocument(c, args.ID, true)
	if err != nil {
		return nil, err
	}
	return setDocumentTrashed(c, id, false)
}

func listDocumentVersions(c *app.Ctx, args IDArgs) (any, error) {
	id, err := findDocument(c, args.ID, false)
	if err != nil {
		return nil, err
	}
	return app.One[any](c, c.DB(),
		`SELECT coalesce(jsonb_agg(jsonb_build_object('id', v.id, 'title', v.title, 'author', user_ref(v.author_id),
			'createdAt', v.created_at) ORDER BY v.id DESC), '[]')
		 FROM document_versions v WHERE v.document_id = $1`, id)
}

func getDocumentVersion(c *app.Ctx, args IDArgs) (any, error) {
	notFound := app.NotFound("No document version %q", strings.TrimSpace(args.ID))
	version, err := strconv.ParseInt(strings.TrimSpace(args.ID), 10, 64)
	if err != nil {
		return nil, notFound
	}
	type row struct {
		Document uuid.UUID
		Value    M
	}
	found, ok, err := app.Row[row](c, c.DB(),
		`SELECT d.id, jsonb_build_object('id', v.id, 'document', document_ref(d.id), 'title', v.title,
			'content', v.content, 'author', user_ref(v.author_id), 'createdAt', v.created_at)
		 FROM document_versions v JOIN documents d ON d.id = v.document_id WHERE v.id = $1 AND d.workspace_id = $2`,
		version, c.Workspace)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, notFound
	}
	visible, err := canSeeDocument(c, found.Document)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, notFound
	}
	return found.Value, nil
}

/* Shapes */

type Document struct {
	ID         string         `json:"id"`
	SlugID     string         `json:"slugId"`
	Title      string         `json:"title"`
	Content    string         `json:"content,omitempty" doc:"Markdown; in lists only with includeContent."`
	URL        string         `json:"url"`
	Icon       *string        `json:"icon"`
	Color      *string        `json:"color"`
	Project    *ProjectRef    `json:"project"`
	Initiative *InitiativeRef `json:"initiative"`
	Team       *TeamRef       `json:"team"`
	Issue      *IssueRef      `json:"issue"`
	Cycle      *CycleRef      `json:"cycle"`
	Release    *ReleaseRef    `json:"release"`
	Creator    *UserRef       `json:"creator"`
	UpdatedBy  *UserRef       `json:"updatedBy"`
	Trashed    bool           `json:"trashed"`
	TrashedAt  *string        `json:"trashedAt"`
	CreatedAt  string         `json:"createdAt"`
	UpdatedAt  string         `json:"updatedAt"`
}

type DocumentDetail struct {
	Document
	VersionCount int `json:"versionCount"`
}

type DocumentVersionSummary struct {
	ID        int      `json:"id"`
	Title     string   `json:"title"`
	Author    *UserRef `json:"author"`
	CreatedAt string   `json:"createdAt"`
}

type DocumentVersion struct {
	ID        int         `json:"id"`
	Document  DocumentRef `json:"document"`
	Title     string      `json:"title"`
	Content   string      `json:"content"`
	Author    *UserRef    `json:"author"`
	CreatedAt string      `json:"createdAt"`
}
