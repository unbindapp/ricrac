package ops

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
)

// Comments: threads on issues, projects, initiatives, documents, milestones and status updates,
// with replies, resolving, and reactions (also on issues and status updates).

func commentOps() []*Op {
	return []*Op{
		op("list_comments", app.Read, []string{"GET /comments"},
			"List the comment threads on one issue, project, initiative, document, milestone or status update, oldest "+
				"first. Each thread is its first comment with its replies, reactions and whether it was resolved.",
			listComments),
		op("save_comment", app.Write, []string{"POST /comments", "PATCH /comments/{id}"},
			"Comment on an issue, project, initiative, document, milestone or status update (pass exactly one of them), "+
				"or reply to a comment with parentId. Pass id and body to edit your own comment. The body is Markdown; "+
				"mention people with @displayName or @email to notify them. Commenting on an issue subscribes you to it and "+
				"notifies its subscribers.",
			saveComment),
		op("delete_comment", app.Write, []string{"DELETE /comments/{id}"},
			"Delete a comment (your own, or any as a workspace admin). Deleting a thread's first comment deletes its replies.",
			deleteComment),
		op("resolve_comment", app.Write, []string{"POST /comments/{id}/resolve"},
			"Mark a comment thread as resolved. Pass the thread's first comment.",
			func(c *app.Ctx, args IDArgs) (any, error) { return setResolved(c, args.ID, true) }),
		op("unresolve_comment", app.Write, []string{"POST /comments/{id}/unresolve"},
			"Reopen a resolved comment thread.",
			func(c *app.Ctx, args IDArgs) (any, error) { return setResolved(c, args.ID, false) }),
		op("add_reaction", app.Write, []string{"POST /reactions"},
			"React with an emoji to an issue, a comment or a status update (exactly one). The emoji is a unicode emoji "+
				"(👍) or a shortcode (+1, :tada:). Reacting twice with the same emoji changes nothing.",
			addReaction),
		op("remove_reaction", app.Write, []string{"DELETE /reactions/{id}", "POST /reactions/remove"},
			"Take back one of your reactions: by its id, or by emoji and the issue, comment or status update.",
			removeReaction),
	}
}

var commentTargets = []string{"issue_id", "project_id", "initiative_id", "document_id", "milestone_id", "status_update_id"}

func commentJSON(c string) string {
	return strings.ReplaceAll(`jsonb_build_object(
		'id', C.id,
		'body', C.body,
		'user', user_ref(C.user_id),
		'quotedText', C.quoted_text,
		'reactions', (SELECT coalesce(jsonb_agg(jsonb_build_object('id', r.id, 'emoji', r.emoji, 'user', user_ref(r.user_id))
			ORDER BY r.created_at), '[]') FROM reactions r WHERE r.comment_id = C.id),
		'createdAt', C.created_at,
		'updatedAt', C.updated_at,
		'editedAt', C.edited_at,
		'resolvedAt', C.resolved_at,
		'resolvingUser', user_ref(C.resolving_user_id)
	)`, "C.", c+".")
}

func threadJSON() string {
	return fmt.Sprintf(`(%s || jsonb_build_object('replies', (SELECT coalesce(jsonb_agg(%s ORDER BY rc.created_at, rc.id), '[]')
		FROM comments rc WHERE rc.parent_id = c.id)))`, commentJSON("c"), commentJSON("rc"))
}

// commentThreads is the comment threads on the thing whose id is in column (e.g. issue_id),
// oldest first, each with its replies.
func commentThreads(c *app.Ctx, q app.Querier, column string, id uuid.UUID) (any, error) {
	threads, err := threadList(c, q, column, id, nil, 0)
	return nonNil(threads), err
}

func threadList(c *app.Ctx, q app.Querier, column string, id uuid.UUID, limit *int64, offset int64) ([]any, error) {
	if !slices.Contains(commentTargets, column) {
		return nil, app.BadRequest("Comments can't be on %q", column)
	}
	return app.All[any](c, q, fmt.Sprintf(
		`SELECT %s FROM comments c WHERE c.%s = $1 AND c.parent_id IS NULL
		 ORDER BY c.created_at, c.id LIMIT $2 OFFSET $3`, threadJSON(), column), id, limit, offset)
}

func commentValue(c *app.Ctx, q app.Querier, id uuid.UUID) (M, error) {
	value, found, err := app.Maybe[M](c, q, fmt.Sprintf(
		`SELECT %s || jsonb_build_object(
			'parentId', c.parent_id,
			'issue', issue_ref(c.issue_id),
			'project', project_ref(c.project_id),
			'initiative', initiative_ref(c.initiative_id),
			'document', document_ref(c.document_id),
			'projectMilestone', milestone_ref(c.milestone_id),
			'statusUpdate', CASE WHEN c.status_update_id IS NOT NULL THEN jsonb_build_object('id', c.status_update_id) END,
			'url', CASE WHEN c.issue_id IS NOT NULL
				THEN $2 || '/' || w.url_key || '/issue/' || issue_identifier(c.issue_id) || '#comment-' || c.id END)
		 FROM comments c JOIN workspaces w ON w.id = c.workspace_id WHERE c.id = $1`, commentJSON("c")), id, c.PublicURL())
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, app.NotFound("No such comment")
	}
	return value, nil
}

/* What comments are on */

type commentTarget struct {
	column string
	id     uuid.UUID
}

type targetArgs struct {
	IssueID        *string `json:"issueId" doc:"Issue identifier (ENG-123), id or link."`
	ProjectID      *string `json:"projectId" doc:"Project name, slug or id."`
	InitiativeID   *string `json:"initiativeId" doc:"Initiative name, slug or id."`
	DocumentID     *string `json:"documentId" doc:"Document title, slug or id."`
	MilestoneID    *string `json:"milestoneId" doc:"Project milestone id."`
	StatusUpdateID *string `json:"statusUpdateId" doc:"Status update id."`
}

func (t targetArgs) given() []*string {
	return []*string{t.IssueID, t.ProjectID, t.InitiativeID, t.DocumentID, t.MilestoneID, t.StatusUpdateID}
}

// namedTarget is the one thing named among the arguments, checked visible, or nil when none is
// named.
func namedTarget(c *app.Ctx, args targetArgs) (*commentTarget, error) {
	var column, value string
	count := 0
	for i, v := range args.given() {
		if v != nil {
			column, value = commentTargets[i], *v
			count++
		}
	}
	if count == 0 {
		return nil, nil
	}
	if count > 1 {
		return nil, app.BadRequest("Pass only one of issueId, projectId, initiativeId, documentId, milestoneId and statusUpdateId")
	}
	var id uuid.UUID
	var err error
	switch column {
	case "issue_id":
		id, err = resolveIssue(c, value)
	case "project_id":
		id, err = resolveProject(c, value)
	case "initiative_id":
		id, err = resolveInitiative(c, value)
	case "document_id":
		id, err = resolveDocument(c, value)
	case "milestone_id":
		id, err = resolveMilestone(c, nil, value)
	default:
		if id, err = resolveByID(c, "status_updates", "status update", value); err == nil {
			err = checkTargetVisible(c, commentTarget{"status_update_id", id})
		}
	}
	if err != nil {
		return nil, err
	}
	return &commentTarget{column, id}, nil
}

func checkTargetVisible(c *app.Ctx, target commentTarget) error {
	id := target.id.String()
	var visible bool
	var err error
	switch target.column {
	case "issue_id":
		visible, err = canSeeIssue(c, target.id)
	case "project_id":
		_, resolveErr := resolveProject(c, id)
		visible = resolveErr == nil
	case "initiative_id":
		_, resolveErr := resolveInitiative(c, id)
		visible = resolveErr == nil
	case "document_id":
		visible, err = canSeeDocument(c, target.id)
	case "milestone_id":
		_, resolveErr := resolveMilestone(c, nil, id)
		visible = resolveErr == nil
	default:
		type owners struct{ Project, Initiative *uuid.UUID }
		row, _, rowErr := app.Row[owners](c, c.DB(), "SELECT project_id, initiative_id FROM status_updates WHERE id = $1", target.id)
		err = rowErr
		switch {
		case row.Project != nil:
			_, resolveErr := resolveProject(c, row.Project.String())
			visible = resolveErr == nil
		case row.Initiative != nil:
			_, resolveErr := resolveInitiative(c, row.Initiative.String())
			visible = resolveErr == nil
		}
	}
	if err != nil {
		return err
	}
	if !visible {
		return app.NotFound("Not found, or you can't see it")
	}
	return nil
}

type commentRow struct {
	ID       uuid.UUID
	Targets  [6]*uuid.UUID
	ParentID *uuid.UUID
	UserID   *uuid.UUID
	Body     string
}

func (r commentRow) target() (commentTarget, error) {
	for i, id := range r.Targets {
		if id != nil {
			return commentTarget{commentTargets[i], *id}, nil
		}
	}
	return commentTarget{}, app.NotFound("That comment is on nothing")
}

// loadComment is a comment of the workspace by id, on something the caller can see.
func loadComment(c *app.Ctx, value string) (commentRow, error) {
	var row commentRow
	notFound := app.NotFound("No comment %q", strings.TrimSpace(value))
	id, ok := asUUID(value)
	if !ok {
		return row, notFound
	}
	err := c.DB().QueryRow(c,
		`SELECT id, issue_id, project_id, initiative_id, document_id, milestone_id, status_update_id, parent_id, user_id, body
		 FROM comments WHERE id = $1 AND workspace_id = $2`, id, c.Workspace).
		Scan(&row.ID, &row.Targets[0], &row.Targets[1], &row.Targets[2], &row.Targets[3], &row.Targets[4], &row.Targets[5],
			&row.ParentID, &row.UserID, &row.Body)
	if app.NoRows(err) {
		return row, notFound
	}
	if err != nil {
		return row, err
	}
	target, err := row.target()
	if err != nil {
		return row, err
	}
	if checkTargetVisible(c, target) != nil {
		return row, notFound
	}
	return row, nil
}

/* List */

type ListCommentsArgs struct {
	targetArgs
	Limit  *int64  `json:"limit" doc:"How many threads to return, 50 by default, at most 250."`
	Cursor *string `json:"cursor" doc:"The endCursor of the previous page."`
}

func listComments(c *app.Ctx, args ListCommentsArgs) (any, error) {
	target, err := namedTarget(c, args.targetArgs)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, app.BadRequest("Pass one of issueId, projectId, initiativeId, documentId, milestoneId or statusUpdateId")
	}
	page := Page{Limit: args.Limit, Cursor: args.Cursor}
	offset, err := page.offset()
	if err != nil {
		return nil, err
	}
	nodes, err := threadList(c, c.DB(), target.column, target.id, new(page.limit()+1), offset)
	if err != nil {
		return nil, err
	}
	return page.wrap(nodes)
}

/* Save */

type SaveCommentArgs struct {
	ID   *string `json:"id" doc:"Only to edit one of your comments: its id. Omit to create a comment."`
	Body *string `json:"body" doc:"Markdown. Mention people with @displayName or @email; they are notified."`
	targetArgs
	ParentID   *string `json:"parentId" doc:"The comment to reply to; the reply joins its thread."`
	QuotedText *string `json:"quotedText" doc:"The passage of the document or description the comment is about."`
}

func saveComment(c *app.Ctx, args SaveCommentArgs) (any, error) {
	body, err := nonEmpty(orEmpty(args.Body), "body")
	if err != nil {
		return nil, err
	}
	target, err := namedTarget(c, args.targetArgs)
	if err != nil {
		return nil, err
	}
	if args.ID != nil {
		if target != nil || args.ParentID != nil || args.QuotedText != nil {
			return nil, app.BadRequest("Only the body of a comment can be changed")
		}
		return editComment(c, *args.ID, body)
	}
	var root *uuid.UUID
	if args.ParentID != nil {
		parent, err := loadComment(c, *args.ParentID)
		if err != nil {
			return nil, err
		}
		parentTarget, err := parent.target()
		if err != nil {
			return nil, err
		}
		if target != nil && *target != parentTarget {
			return nil, app.BadRequest("The parent comment is on something else")
		}
		target = &parentTarget
		root = parent.ParentID
		if root == nil {
			root = &parent.ID
		}
	}
	if target == nil {
		return nil, app.BadRequest("Pass what to comment on (issueId, projectId, initiativeId, documentId, milestoneId or statusUpdateId) or parentId")
	}
	var quoted *string
	if args.QuotedText != nil && strings.TrimSpace(*args.QuotedText) != "" {
		quoted = new(strings.TrimSpace(*args.QuotedText))
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		id, err := app.One[uuid.UUID](c, tx, fmt.Sprintf(
			`INSERT INTO comments (workspace_id, %s, parent_id, user_id, body, quoted_text)
			 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`, target.column), c.Workspace, target.id, root, c.User, body, quoted)
		if err != nil {
			return err
		}
		about, err := commentAbout(c, tx, *target)
		if err != nil {
			return err
		}
		mentioned, err := mentions(c, body)
		if err != nil {
			return err
		}
		var participants []uuid.UUID
		if root != nil {
			if participants, err = app.All[uuid.UUID](c, tx,
				"SELECT DISTINCT user_id FROM comments WHERE (id = $1 OR parent_id = $1) AND user_id IS NOT NULL", *root); err != nil {
				return err
			}
		}
		var audience []uuid.UUID
		switch target.column {
		case "issue_id":
			if err := subscribe(c, tx, target.id, []uuid.UUID{c.User}); err != nil {
				return err
			}
			if err := subscribe(c, tx, target.id, mentioned); err != nil {
				return err
			}
			if err := app.Exec(c, tx, "UPDATE issues SET updated_at = now() WHERE id = $1", target.id); err != nil {
				return err
			}
			if audience, err = issueSubscribers(c, tx, target.id); err != nil {
				return err
			}
		case "status_update_id":
			if audience, err = app.All[uuid.UUID](c, tx,
				"SELECT user_id FROM status_updates WHERE id = $1 AND user_id IS NOT NULL", target.id); err != nil {
				return err
			}
		}
		name, err := actorName(c)
		if err != nil {
			return err
		}
		replied := slices.DeleteFunc(participants, func(u uuid.UUID) bool { return slices.Contains(mentioned, u) })
		others := slices.DeleteFunc(audience, func(u uuid.UUID) bool {
			return slices.Contains(mentioned, u) || slices.Contains(replied, u)
		})
		for _, n := range []struct {
			users   []uuid.UUID
			kind    string
			summary string
		}{
			{mentioned, about.kinds[2], fmt.Sprintf("%s mentioned you in a comment on %s", name, about.label)},
			{replied, about.kinds[1], fmt.Sprintf("%s replied to a thread on %s", name, about.label)},
			{others, about.kinds[0], fmt.Sprintf("%s commented on %s", name, about.label)},
		} {
			notice := about.notice
			notice.Kind, notice.Summary, notice.Comment = n.kind, n.summary, &id
			if err := notify(c, tx, n.users, notice); err != nil {
				return err
			}
		}
		if value, err = commentValue(c, tx, id); err != nil {
			return err
		}
		return emit(c, tx, "Comment", "create", value, nil, about.team)
	})
	return value, err
}

func editComment(c *app.Ctx, reference, body string) (any, error) {
	row, err := loadComment(c, reference)
	if err != nil {
		return nil, err
	}
	if row.UserID == nil || *row.UserID != c.User {
		return nil, app.Forbidden("Only its author can edit a comment")
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		before, err := commentValue(c, tx, row.ID)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "UPDATE comments SET body = $2, edited_at = now(), updated_at = now() WHERE id = $1", row.ID, body); err != nil {
			return err
		}
		previous, err := mentions(c, row.Body)
		if err != nil {
			return err
		}
		current, err := mentions(c, body)
		if err != nil {
			return err
		}
		mentioned := slices.DeleteFunc(current, func(u uuid.UUID) bool { return slices.Contains(previous, u) })
		target, err := row.target()
		if err != nil {
			return err
		}
		about, err := commentAbout(c, tx, target)
		if err != nil {
			return err
		}
		if target.column == "issue_id" {
			if err := subscribe(c, tx, target.id, mentioned); err != nil {
				return err
			}
		}
		name, err := actorName(c)
		if err != nil {
			return err
		}
		notice := about.notice
		notice.Kind = about.kinds[2]
		notice.Summary = fmt.Sprintf("%s mentioned you in a comment on %s", name, about.label)
		notice.Comment = &row.ID
		if err := notify(c, tx, mentioned, notice); err != nil {
			return err
		}
		if value, err = commentValue(c, tx, row.ID); err != nil {
			return err
		}
		updatedFrom := M{"body": before["body"], "editedAt": before["editedAt"]}
		return emit(c, tx, "Comment", "update", value, updatedFrom, about.team)
	})
	return value, err
}

// about is how notifications about comments on a target read and what they point at.
type about struct {
	// The kinds for a new comment, a reply in your thread, and a mention.
	kinds  [3]string
	label  string
	notice Notice
	team   *uuid.UUID
}

func commentAbout(c *app.Ctx, tx pgx.Tx, target commentTarget) (about, error) {
	id := target.id
	label := func(sql string) (string, error) { return app.One[string](c, tx, sql, id) }
	projectKinds := [3]string{"projectNewComment", "projectCommentReply", "projectCommentMention"}
	switch target.column {
	case "issue_id":
		name, err := label("SELECT issue_identifier($1)")
		if err != nil {
			return about{}, err
		}
		team, err := app.One[uuid.UUID](c, tx, "SELECT team_id FROM issues WHERE id = $1", id)
		return about{[3]string{"issueNewComment", "issueCommentReply", "issueCommentMention"}, name, Notice{Issue: &id}, &team}, err
	case "project_id":
		name, err := label("SELECT name FROM projects WHERE id = $1")
		return about{projectKinds, name, Notice{Project: &id}, nil}, err
	case "milestone_id":
		row, _, err := app.Row[struct {
			Project uuid.UUID
			Label   string
		}](c, tx, `SELECT p.id, m.name || ' (' || p.name || ')' FROM project_milestones m
			JOIN projects p ON p.id = m.project_id WHERE m.id = $1`, id)
		return about{projectKinds, row.Label, Notice{Project: &row.Project}, nil}, err
	case "initiative_id":
		name, err := label("SELECT name FROM initiatives WHERE id = $1")
		return about{[3]string{"initiativeNewComment", "initiativeCommentReply", "initiativeCommentMention"}, name, Notice{Initiative: &id}, nil}, err
	case "document_id":
		name, err := label("SELECT title FROM documents WHERE id = $1")
		return about{[3]string{"documentNewComment", "documentCommentReply", "documentCommentMention"}, name, Notice{Document: &id}, nil}, err
	}
	row, _, err := app.Row[struct {
		Project, Initiative *uuid.UUID
		Name                string
	}](c, tx, `SELECT u.project_id, u.initiative_id, coalesce(p.name, i.name) FROM status_updates u
		LEFT JOIN projects p ON p.id = u.project_id LEFT JOIN initiatives i ON i.id = u.initiative_id
		WHERE u.id = $1`, id)
	kinds := [3]string{"initiativeUpdateNewComment", "initiativeUpdateCommentReply", "initiativeUpdateCommentMention"}
	if row.Project != nil {
		kinds = [3]string{"projectUpdateNewComment", "projectUpdateCommentReply", "projectUpdateCommentMention"}
	}
	return about{kinds, "the update on " + row.Name, Notice{StatusUpdate: &id, Project: row.Project, Initiative: row.Initiative}, nil}, err
}

func actorName(c *app.Ctx) (string, error) {
	return app.One[string](c, c.DB(), "SELECT name FROM users WHERE id = $1", c.User)
}

/* Delete, resolve, unresolve */

func teamOfTarget(c *app.Ctx, q app.Querier, target commentTarget) (*uuid.UUID, error) {
	if target.column != "issue_id" {
		return nil, nil
	}
	team, found, err := app.Maybe[uuid.UUID](c, q, "SELECT team_id FROM issues WHERE id = $1", target.id)
	if err != nil || !found {
		return nil, err
	}
	return &team, nil
}

func deleteComment(c *app.Ctx, args IDArgs) (any, error) {
	row, err := loadComment(c, args.ID)
	if err != nil {
		return nil, err
	}
	if (row.UserID == nil || *row.UserID != c.User) && c.Role < app.Admin {
		return nil, app.Forbidden("Only its author or a workspace admin can delete a comment")
	}
	var replies int64
	err = inTx(c, func(tx pgx.Tx) error {
		value, err := commentValue(c, tx, row.ID)
		if err != nil {
			return err
		}
		if replies, err = app.One[int64](c, tx, "SELECT count(*) FROM comments WHERE parent_id = $1", row.ID); err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM comments WHERE id = $1", row.ID); err != nil {
			return err
		}
		target, err := row.target()
		if err != nil {
			return err
		}
		team, err := teamOfTarget(c, tx, target)
		if err != nil {
			return err
		}
		return emit(c, tx, "Comment", "remove", value, nil, team)
	})
	if err != nil {
		return nil, err
	}
	return M{"deleted": row.ID, "deletedReplies": replies}, nil
}

func setResolved(c *app.Ctx, reference string, resolved bool) (any, error) {
	row, err := loadComment(c, reference)
	if err != nil {
		return nil, err
	}
	if row.ParentID != nil {
		return nil, app.BadRequest("Only a thread is resolved; pass the thread's first comment (the parentId)")
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		if err := app.Exec(c, tx,
			`UPDATE comments SET resolved_at = CASE WHEN $2 THEN coalesce(resolved_at, now()) END,
				resolving_user_id = CASE WHEN $2 THEN coalesce(resolving_user_id, $3) END, updated_at = now()
			 WHERE id = $1`, row.ID, resolved, c.User); err != nil {
			return err
		}
		if value, err = commentValue(c, tx, row.ID); err != nil {
			return err
		}
		target, err := row.target()
		if err != nil {
			return err
		}
		team, err := teamOfTarget(c, tx, target)
		if err != nil {
			return err
		}
		return emit(c, tx, "Comment", "update", value, nil, team)
	})
	return value, err
}

/* Reactions */

type ReactionArgs struct {
	Emoji          string  `json:"emoji" doc:"A unicode emoji (👍) or a shortcode (+1, :tada:)."`
	IssueID        *string `json:"issueId" doc:"Issue identifier (ENG-123), id or link."`
	CommentID      *string `json:"commentId" doc:"Comment id."`
	StatusUpdateID *string `json:"statusUpdateId" doc:"Status update id."`
}

type RemoveReactionArgs struct {
	ID             *string `json:"id" doc:"The reaction's id. Or pass emoji and what it is on."`
	Emoji          *string `json:"emoji"`
	IssueID        *string `json:"issueId"`
	CommentID      *string `json:"commentId"`
	StatusUpdateID *string `json:"statusUpdateId"`
}

// normalizeEmoji keeps shortcodes without colons, lowercase; unicode emoji as they are.
func normalizeEmoji(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	name := trimmed
	if inner, ok := strings.CutPrefix(trimmed, ":"); ok {
		if inner, ok := strings.CutSuffix(inner, ":"); ok {
			name = inner
		}
	}
	shortcode := len(name) <= 50
	ascii := true
	for _, r := range name {
		if r >= utf8.RuneSelf {
			ascii = false
		}
		if !(r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) || r == '_' || r == '+' || r == '-') {
			shortcode = false
		}
	}
	isUnicode := !ascii && utf8.RuneCountInString(name) <= 16 && !strings.ContainsFunc(name, unicode.IsSpace)
	if name == "" || !(shortcode || isUnicode) {
		return "", app.BadRequest("%q isn't an emoji; pass one like 👍 or a shortcode like +1 or :tada:", value)
	}
	if shortcode {
		return strings.ToLower(name), nil
	}
	return name, nil
}

// reactionTarget is the column and id a reaction is on, checked visible, and the issue's team
// for webhooks.
func reactionTarget(c *app.Ctx, issue, comment, statusUpdate *string) (string, uuid.UUID, *uuid.UUID, error) {
	var column string
	var id uuid.UUID
	var on *commentTarget
	switch {
	case issue != nil && comment == nil && statusUpdate == nil:
		issueID, err := resolveIssue(c, *issue)
		if err != nil {
			return "", uuid.Nil, nil, err
		}
		column, id, on = "issue_id", issueID, &commentTarget{"issue_id", issueID}
	case issue == nil && comment != nil && statusUpdate == nil:
		row, err := loadComment(c, *comment)
		if err != nil {
			return "", uuid.Nil, nil, err
		}
		target, err := row.target()
		if err != nil {
			return "", uuid.Nil, nil, err
		}
		column, id, on = "comment_id", row.ID, &target
	case issue == nil && comment == nil && statusUpdate != nil:
		updateID, err := resolveByID(c, "status_updates", "status update", *statusUpdate)
		if err != nil {
			return "", uuid.Nil, nil, err
		}
		if err := checkTargetVisible(c, commentTarget{"status_update_id", updateID}); err != nil {
			return "", uuid.Nil, nil, err
		}
		column, id = "status_update_id", updateID
	default:
		return "", uuid.Nil, nil, app.BadRequest("Pass exactly one of issueId, commentId and statusUpdateId")
	}
	if on == nil {
		return column, id, nil, nil
	}
	team, err := teamOfTarget(c, c.DB(), *on)
	return column, id, team, err
}

const reactionJSON = `jsonb_build_object('id', r.id, 'emoji', r.emoji, 'user', user_ref(r.user_id),
	'issue', issue_ref(r.issue_id),
	'comment', CASE WHEN r.comment_id IS NOT NULL THEN jsonb_build_object('id', r.comment_id) END,
	'statusUpdate', CASE WHEN r.status_update_id IS NOT NULL THEN jsonb_build_object('id', r.status_update_id) END,
	'createdAt', r.created_at)`

func addReaction(c *app.Ctx, args ReactionArgs) (any, error) {
	emoji, err := normalizeEmoji(args.Emoji)
	if err != nil {
		return nil, err
	}
	column, id, team, err := reactionTarget(c, args.IssueID, args.CommentID, args.StatusUpdateID)
	if err != nil {
		return nil, err
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		_, inserted, err := app.Maybe[uuid.UUID](c, tx, fmt.Sprintf(
			`INSERT INTO reactions (workspace_id, user_id, emoji, %s) VALUES ($1, $2, $3, $4)
			 ON CONFLICT DO NOTHING RETURNING id`, column), c.Workspace, c.User, emoji, id)
		if err != nil {
			return err
		}
		if value, err = app.One[M](c, tx, fmt.Sprintf(
			"SELECT %s FROM reactions r WHERE r.user_id = $1 AND r.emoji = $2 AND r.%s = $3", reactionJSON, column),
			c.User, emoji, id); err != nil {
			return err
		}
		if !inserted {
			return nil
		}
		return emit(c, tx, "Reaction", "create", value, nil, team)
	})
	return value, err
}

func removeReaction(c *app.Ctx, args RemoveReactionArgs) (any, error) {
	type found struct {
		ID   uuid.UUID
		Team *uuid.UUID
	}
	var reaction found
	var exists bool
	switch {
	case args.ID != nil && args.Emoji == nil:
		id, ok := asUUID(*args.ID)
		if !ok {
			return nil, app.NotFound("No reaction %q", *args.ID)
		}
		var err error
		reaction, exists, err = app.Row[found](c, c.DB(),
			`SELECT r.id, coalesce(i.team_id, ci.team_id) FROM reactions r
			 LEFT JOIN issues i ON i.id = r.issue_id
			 LEFT JOIN comments c ON c.id = r.comment_id LEFT JOIN issues ci ON ci.id = c.issue_id
			 WHERE r.id = $1 AND r.workspace_id = $2 AND r.user_id = $3`, id, c.Workspace, c.User)
		if err != nil {
			return nil, err
		}
	case args.ID == nil && args.Emoji != nil:
		emoji, err := normalizeEmoji(*args.Emoji)
		if err != nil {
			return nil, err
		}
		column, id, team, err := reactionTarget(c, args.IssueID, args.CommentID, args.StatusUpdateID)
		if err != nil {
			return nil, err
		}
		reaction.Team = team
		reaction.ID, exists, err = app.Maybe[uuid.UUID](c, c.DB(), fmt.Sprintf(
			"SELECT id FROM reactions WHERE user_id = $1 AND emoji = $2 AND %s = $3", column), c.User, emoji, id)
		if err != nil {
			return nil, err
		}
	default:
		return nil, app.BadRequest("Pass the reaction's id, or emoji and what it is on")
	}
	if !exists {
		return nil, app.NotFound("You have no such reaction")
	}
	err := inTx(c, func(tx pgx.Tx) error {
		value, err := app.One[M](c, tx, fmt.Sprintf("SELECT %s FROM reactions r WHERE r.id = $1", reactionJSON), reaction.ID)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "DELETE FROM reactions WHERE id = $1", reaction.ID); err != nil {
			return err
		}
		return emit(c, tx, "Reaction", "remove", value, nil, reaction.Team)
	})
	if err != nil {
		return nil, err
	}
	return M{"removed": reaction.ID}, nil
}
