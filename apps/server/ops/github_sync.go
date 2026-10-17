package ops

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"ricrac/app"
	"ricrac/internal/integrations/github"
)

// Issues and comments of synced repositories, from GitHub: webhook events and the import of a
// repository's existing issues. Changes are made as the GitHub user's Ricrac account when their
// login is linked, else by nobody, and marked FromGitHubIssue so they aren't sent back.

type repoSync struct {
	ID           uuid.UUID
	Workspace    uuid.UUID
	Installation int64
	FullName     string
	Team         uuid.UUID
	Project      *uuid.UUID
}

func (h *hook) syncedRepo(repository github.Repository) (*repoSync, error) {
	sync, found, err := app.Row[repoSync](h.ctx, h.state.DB,
		`SELECT id, workspace_id, installation_id, full_name, team_id, project_id FROM github_repo_syncs
		 WHERE installation_id = $1 AND repo_id = $2`, h.installation, repository.ID)
	if err != nil || !found {
		return nil, err
	}
	return &sync, nil
}

type issueLink struct {
	Issue     uuid.UUID
	Number    int
	UpdatedAt time.Time
}

func findIssueLink(ctx context.Context, q app.Querier, sync *repoSync, githubID int64) (*issueLink, error) {
	link, found, err := app.Row[issueLink](ctx, q,
		"SELECT issue_id, github_number, github_updated_at FROM github_issue_links WHERE repo_sync_id = $1 AND github_id = $2",
		sync.ID, githubID)
	if err != nil || !found {
		return nil, err
	}
	return &link, nil
}

// recordSync notes on the repository sync whether the last change with GitHub worked.
func recordSync(ctx context.Context, state *app.State, sync uuid.UUID, err error) error {
	var message *string
	if err != nil {
		slog.Warn("syncing with GitHub failed", "repoSync", sync, "err", err)
		message = new(err.Error())
	}
	if recordErr := app.Exec(ctx, state.DB,
		"UPDATE github_repo_syncs SET last_error = $2, last_synced_at = now() WHERE id = $1", sync, message); recordErr != nil {
		return errors.Join(err, recordErr)
	}
	return err
}

func (h *hook) issues(event *github.IssuesEvent) error {
	sync, err := h.syncedRepo(event.Repository)
	if sync == nil || err != nil {
		return err
	}
	link, err := findIssueLink(h.ctx, h.state.DB, sync, event.Issue.ID)
	if err != nil {
		return err
	}
	switch event.Action {
	case "deleted", "transferred":
		return recordSync(h.ctx, h.state, sync.ID, h.unlink(link, event))
	case "opened", "edited", "labeled", "unlabeled", "assigned", "unassigned", "closed", "reopened":
	default:
		return nil
	}
	if link == nil {
		_, err := importGitHubIssue(h.ctx, h.state, sync, &event.Issue)
		return recordSync(h.ctx, h.state, sync.ID, err)
	}
	return recordSync(h.ctx, h.state, sync.ID, applyGitHubIssueEvent(h.ctx, h.state, sync, link, event))
}

// unlink forgets an issue deleted on GitHub, trashing it, or moved to another repository.
func (h *hook) unlink(link *issueLink, event *github.IssuesEvent) error {
	if link == nil {
		return nil
	}
	if event.Action == "deleted" {
		c := app.SystemCtx(h.ctx, h.state, h.workspace, uuid.Nil,
			app.Origin{Label: byLine("Deleted on GitHub", uuid.Nil, event.Sender.Login), URL: event.Issue.HTMLURL, FromGitHubIssue: true})
		if _, err := trashIssue(c, IDArgs{ID: link.Issue.String()}); err != nil {
			return err
		}
	}
	return app.Exec(h.ctx, h.state.DB, "DELETE FROM github_issue_links WHERE issue_id = $1", link.Issue)
}

// memberByLogin is the member of the workspace who linked this GitHub account, or uuid.Nil.
func memberByLogin(ctx context.Context, q app.Querier, workspace uuid.UUID, account *github.Account) uuid.UUID {
	if account == nil {
		return uuid.Nil
	}
	login, ok := github.NormalizeLogin(account.Login)
	if !ok {
		return uuid.Nil
	}
	id, _, _ := app.Maybe[uuid.UUID](ctx, q,
		`SELECT u.id FROM users u JOIN workspace_members m ON m.user_id = u.id AND m.workspace_id = $1 AND m.active
		 WHERE u.github_login = $2`, workspace, login)
	return id
}

// assignable is the member if they can see the team's issues, else uuid.Nil.
func assignable(c *app.Ctx, member, team uuid.UUID) uuid.UUID {
	if member == uuid.Nil {
		return uuid.Nil
	}
	ok, err := app.One[bool](c, c.DB(),
		`SELECT EXISTS (SELECT 1 FROM teams t JOIN workspace_members m ON m.workspace_id = t.workspace_id AND m.user_id = $2
			WHERE t.id = $1 AND ((NOT t.private AND m.role <> 'guest')
				OR EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id = t.id AND tm.user_id = $2)))`, team, member)
	if err != nil || !ok {
		return uuid.Nil
	}
	return member
}

// byLine says who did it on GitHub when they have no Ricrac account to show as the actor.
func byLine(label string, actor uuid.UUID, login string) string {
	if actor != uuid.Nil || login == "" {
		return label
	}
	return label + " by @" + login
}

func clampTitle(raw string) string {
	title := strings.TrimSpace(raw)
	if title == "" {
		return "Untitled GitHub issue"
	}
	if utf8.RuneCountInString(title) > 255 {
		return string([]rune(title)[:255])
	}
	return title
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// importGitHubIssue creates the Ricrac issue of a GitHub issue, linked to it; false when another
// delivery linked it first.
func importGitHubIssue(ctx context.Context, state *app.State, sync *repoSync, issue *github.Issue) (bool, error) {
	actor := memberByLogin(ctx, state.DB, sync.Workspace, &issue.User)
	label := byLine("Synced from GitHub #"+strconv.Itoa(issue.Number), actor, issue.User.Login)
	c := app.SystemCtx(ctx, state, sync.Workspace, actor, app.Origin{Label: label, URL: issue.HTMLURL, FromGitHubIssue: true})
	status, err := statusFor(c, sync.Team, issue.Closure())
	if err != nil {
		return false, err
	}
	args := SaveIssueArgs{
		Team: new(sync.Team.String()), Title: new(clampTitle(issue.Title)), Description: &issue.Body, State: new(status.String()),
		Labels: []string{},
	}
	for _, label := range issue.Labels {
		id, err := labelFor(c, sync.Team, label)
		if err != nil {
			return false, err
		}
		args.Labels = append(args.Labels, id.String())
	}
	if assignee := assignable(c, memberByLogin(ctx, state.DB, sync.Workspace, issue.Assignee), sync.Team); assignee != uuid.Nil {
		args.Assignee = some(assignee.String())
	}
	if sync.Project != nil {
		args.Project = some(sync.Project.String())
	}
	err = inTx(c, func(tx pgx.Tx) error {
		id, err := createIssue(c, tx, args)
		if err != nil {
			return err
		}
		return app.Exec(c, tx,
			`INSERT INTO github_issue_links (issue_id, repo_sync_id, github_id, github_number, github_node_id, github_url, github_updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`, id, sync.ID, issue.ID, issue.Number, issue.NodeID, issue.HTMLURL, issue.UpdatedAt)
	})
	if isUniqueViolation(err) {
		return false, nil
	}
	return err == nil, err
}

// statusFor is the status of the team a GitHub issue in this state goes to: open issues go to
// triage when the team uses it, else the default status.
func statusFor(c *app.Ctx, team uuid.UUID, closure github.Closure) (uuid.UUID, error) {
	var kinds []string
	switch closure {
	case github.ClosedCompleted:
		kinds = []string{"completed"}
	case github.ClosedNotPlanned:
		kinds = []string{"canceled", "completed"}
	default:
		triage, err := app.One[bool](c, c.DB(), "SELECT triage_enabled FROM teams WHERE id = $1", team)
		if err != nil {
			return uuid.Nil, err
		}
		if !triage {
			return defaultState(c, c.DB(), team)
		}
		kinds = []string{"triage"}
	}
	for _, kind := range kinds {
		id, err := firstStateOfType(c, c.DB(), team, kind)
		if err != nil || id != nil {
			return derefID(id), err
		}
	}
	return defaultState(c, c.DB(), team)
}

func derefID(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}

func matchesClosure(kind string, closure github.Closure) bool {
	switch closure {
	case github.ClosedCompleted:
		return kind == "completed"
	case github.ClosedNotPlanned:
		return kind == "canceled"
	}
	return kind != "completed" && kind != "canceled"
}

// labelFor is the team's or workspace's label named like a GitHub label, created when there's
// none.
func labelFor(c *app.Ctx, team uuid.UUID, wanted github.Label) (uuid.UUID, error) {
	name := strings.TrimSpace(wanted.Name)
	find := func() (uuid.UUID, bool, error) {
		return app.Maybe[uuid.UUID](c, c.DB(),
			`SELECT id FROM issue_labels WHERE workspace_id = $1 AND (team_id IS NULL OR team_id = $2) AND NOT is_group
			 AND retired_at IS NULL AND lower(name) = lower($3) ORDER BY team_id IS NULL, parent_id IS NOT NULL, created_at LIMIT 1`,
			c.Workspace, team, name)
	}
	if id, found, err := find(); found || err != nil {
		return id, err
	}
	var id uuid.UUID
	err := inTx(c, func(tx pgx.Tx) error {
		var err error
		id, err = app.One[uuid.UUID](c, tx,
			"INSERT INTO issue_labels (workspace_id, name, color) VALUES ($1, $2, $3) RETURNING id", c.Workspace, name, github.RicracColor(wanted.Color))
		if err != nil {
			return err
		}
		value, err := labelValue(c, tx, id)
		if err != nil {
			return err
		}
		return emit(c, tx, "IssueLabel", "create", value, nil, nil)
	})
	return id, err
}

func applyGitHubIssueEvent(ctx context.Context, state *app.State, sync *repoSync, link *issueLink, event *github.IssuesEvent) error {
	issue := &event.Issue
	switch event.Action {
	case "edited", "closed", "reopened":
		if issue.UpdatedAt.Before(link.UpdatedAt) {
			return nil
		}
	}
	actor := memberByLogin(ctx, state.DB, sync.Workspace, &event.Sender)
	c := app.SystemCtx(ctx, state, sync.Workspace, actor, app.Origin{URL: issue.HTMLURL, FromGitHubIssue: true})
	row, err := loadIssueRow(c, c.DB(), link.Issue)
	if err != nil {
		return err
	}
	args, label, err := gitHubIssueChange(c, row, event)
	if err != nil || args == nil {
		return err
	}
	c.Origin.Label = byLine(label, actor, event.Sender.Login)
	return inTx(c, func(tx pgx.Tx) error {
		if err := updateIssue(c, tx, link.Issue, *args); err != nil {
			return err
		}
		return app.Exec(c, tx, "UPDATE github_issue_links SET github_updated_at = greatest(github_updated_at, $2) WHERE issue_id = $1",
			link.Issue, issue.UpdatedAt)
	})
}

// gitHubIssueChange is what an issues event changes on the Ricrac issue, nil when nothing.
func gitHubIssueChange(c *app.Ctx, row issueRow, event *github.IssuesEvent) (*SaveIssueArgs, string, error) {
	issue := &event.Issue
	args := &SaveIssueArgs{}
	switch event.Action {
	case "edited":
		title := clampTitle(issue.Title)
		if title == row.Title && issue.Body == row.Description {
			return nil, "", nil
		}
		if title != row.Title {
			args.Title = &title
		}
		if issue.Body != row.Description {
			args.Description = &issue.Body
		}
		return args, "Edited on GitHub", nil
	case "labeled":
		if event.Label == nil {
			return nil, "", nil
		}
		id, err := labelFor(c, row.TeamID, *event.Label)
		if err != nil {
			return nil, "", err
		}
		has, err := app.One[bool](c, c.DB(), "SELECT EXISTS (SELECT 1 FROM issue_label_links WHERE issue_id = $1 AND label_id = $2)", row.ID, id)
		if err != nil || has {
			return nil, "", err
		}
		args.AddLabels = []string{id.String()}
		return args, "Labeled on GitHub", nil
	case "unlabeled":
		if event.Label == nil {
			return nil, "", nil
		}
		ids, err := app.All[string](c, c.DB(),
			`SELECT l.id::text FROM issue_label_links il JOIN issue_labels l ON l.id = il.label_id
			 WHERE il.issue_id = $1 AND lower(l.name) = lower($2)`, row.ID, event.Label.Name)
		if err != nil || len(ids) == 0 {
			return nil, "", err
		}
		args.RemoveLabels = ids
		return args, "Unlabeled on GitHub", nil
	case "assigned":
		assignee := assignable(c, memberByLogin(c, c.DB(), c.Workspace, event.Assignee), row.TeamID)
		if assignee == uuid.Nil || samePtr(row.AssigneeID, &assignee) {
			return nil, "", nil
		}
		args.Assignee = some(assignee.String())
		return args, "Assigned on GitHub", nil
	case "unassigned":
		if row.AssigneeID == nil || event.Assignee == nil {
			return nil, "", nil
		}
		login, err := app.One[*string](c, c.DB(), "SELECT github_login FROM users WHERE id = $1", *row.AssigneeID)
		if err != nil || login == nil || !strings.EqualFold(*login, event.Assignee.Login) {
			return nil, "", err
		}
		args.Assignee = Nullable[string]{Set: true}
		return args, "Unassigned on GitHub", nil
	case "closed", "reopened":
		closure := issue.Closure()
		current, err := stateType(c, c.DB(), row.StateID)
		if err != nil || matchesClosure(current, closure) {
			return nil, "", err
		}
		status, err := statusFor(c, row.TeamID, closure)
		if err != nil {
			return nil, "", err
		}
		args.State = new(status.String())
		if closure == github.Open {
			return args, "Reopened on GitHub", nil
		}
		return args, "Closed on GitHub", nil
	}
	return nil, "", nil
}

/* Comments */

func (h *hook) issueComment(event *github.IssueCommentEvent) error {
	if event.Issue.IsPullRequest() {
		return nil
	}
	switch event.Action {
	case "created", "edited", "deleted":
	default:
		return nil
	}
	sync, err := h.syncedRepo(event.Repository)
	if sync == nil || err != nil {
		return err
	}
	link, err := findIssueLink(h.ctx, h.state.DB, sync, event.Issue.ID)
	if err != nil {
		return err
	}
	if link == nil && event.Action == "deleted" {
		return nil
	}
	if link == nil {
		if _, err := importGitHubIssue(h.ctx, h.state, sync, &event.Issue); err != nil {
			return recordSync(h.ctx, h.state, sync.ID, err)
		}
		if link, err = findIssueLink(h.ctx, h.state.DB, sync, event.Issue.ID); link == nil || err != nil {
			return recordSync(h.ctx, h.state, sync.ID, err)
		}
	}
	switch event.Action {
	case "deleted":
		err = h.deleteGitHubComment(event.Comment.ID)
	case "edited":
		err = h.editGitHubComment(sync, link, &event.Comment)
	default:
		_, err = importGitHubComment(h.ctx, h.state, sync, link, &event.Comment)
	}
	return recordSync(h.ctx, h.state, sync.ID, err)
}

func commentFromGitHub(actor uuid.UUID, comment *github.Comment) string {
	if actor != uuid.Nil {
		return comment.Body
	}
	return github.GitHubCommentBody(strings.ToLower(comment.User.Login), comment.Body)
}

// importGitHubComment adds a GitHub comment to the linked issue, unless the app posted it or it
// is there already.
func importGitHubComment(ctx context.Context, state *app.State, sync *repoSync, link *issueLink, comment *github.Comment) (bool, error) {
	if github.FromRicrac(comment.Body) || strings.TrimSpace(comment.Body) == "" {
		return false, nil
	}
	known, err := app.One[bool](ctx, state.DB, "SELECT EXISTS (SELECT 1 FROM github_comment_links WHERE github_id = $1)", comment.ID)
	if err != nil || known {
		return false, err
	}
	actor := memberByLogin(ctx, state.DB, sync.Workspace, &comment.User)
	c := app.SystemCtx(ctx, state, sync.Workspace, actor, app.Origin{Label: "Commented on GitHub", URL: comment.HTMLURL, FromGitHubIssue: true})
	err = inTx(c, func(tx pgx.Tx) error {
		id, _, err := createComment(c, tx, commentTarget{"issue_id", link.Issue}, nil, commentFromGitHub(actor, comment), nil)
		if err != nil {
			return err
		}
		if err := app.Exec(c, tx, "UPDATE comments SET created_at = $2, updated_at = $2 WHERE id = $1", id, comment.CreatedAt); err != nil {
			return err
		}
		return app.Exec(c, tx, "INSERT INTO github_comment_links (comment_id, issue_id, github_id) VALUES ($1, $2, $3)", id, link.Issue, comment.ID)
	})
	if isUniqueViolation(err) {
		return false, nil
	}
	return err == nil, err
}

func (h *hook) editGitHubComment(sync *repoSync, link *issueLink, comment *github.Comment) error {
	if github.FromRicrac(comment.Body) || strings.TrimSpace(comment.Body) == "" {
		return nil
	}
	id, found, err := app.Maybe[uuid.UUID](h.ctx, h.state.DB, "SELECT comment_id FROM github_comment_links WHERE github_id = $1", comment.ID)
	if err != nil {
		return err
	}
	if !found {
		_, err := importGitHubComment(h.ctx, h.state, sync, link, comment)
		return err
	}
	actor := memberByLogin(h.ctx, h.state.DB, sync.Workspace, &comment.User)
	c := app.SystemCtx(h.ctx, h.state, sync.Workspace, actor, app.Origin{Label: "Edited a comment on GitHub", URL: comment.HTMLURL, FromGitHubIssue: true})
	row, err := loadComment(c, id.String())
	if err != nil {
		return err
	}
	return inTx(c, func(tx pgx.Tx) error {
		_, err := changeComment(c, tx, row, commentFromGitHub(actor, comment))
		return err
	})
}

func (h *hook) deleteGitHubComment(githubID int64) error {
	id, found, err := app.Maybe[uuid.UUID](h.ctx, h.state.DB, "SELECT comment_id FROM github_comment_links WHERE github_id = $1", githubID)
	if err != nil || !found {
		return err
	}
	c := app.SystemCtx(h.ctx, h.state, h.workspace, uuid.Nil, app.Origin{Label: "Deleted a comment on GitHub", FromGitHubIssue: true})
	row, err := loadComment(c, id.String())
	if err != nil {
		return err
	}
	return inTx(c, func(tx pgx.Tx) error {
		_, err := removeComment(c, tx, row)
		return err
	})
}

/* Importing a repository's existing issues */

// RunGitHubBackfills imports one page of existing issues of each repository waiting for it,
// saying whether more pages are left.
func RunGitHubBackfills(ctx context.Context, state *app.State) (bool, error) {
	if state.GitHub == nil {
		return false, nil
	}
	ids, err := app.All[uuid.UUID](ctx, state.DB,
		`SELECT r.id FROM github_repo_syncs r JOIN github_installations g ON g.id = r.installation_id
		 WHERE r.backfill_status IN ('pending', 'running') AND g.suspended_at IS NULL ORDER BY r.updated_at LIMIT 5`)
	if err != nil {
		return false, err
	}
	more := false
	for _, id := range ids {
		running, err := backfillPage(ctx, state, id)
		if err != nil {
			return more, err
		}
		more = more || running
	}
	return more, nil
}

type backfillCounts struct {
	created, updated, comments, skipped int
	failed                              error
}

func backfillPage(ctx context.Context, state *app.State, id uuid.UUID) (bool, error) {
	type row struct {
		repoSync
		Page   int
		Closed bool
	}
	sync, found, err := app.Row[row](ctx, state.DB,
		`SELECT id, workspace_id, installation_id, full_name, team_id, project_id, backfill_page, backfill_closed
		 FROM github_repo_syncs WHERE id = $1`, id)
	if err != nil || !found {
		return false, err
	}
	which := "open"
	if sync.Closed {
		which = "all"
	}
	var issues []github.Issue
	token, err := state.GitHub.InstallationToken(ctx, sync.Installation)
	next := 0
	if err == nil {
		path := fmt.Sprintf("%s/issues?state=%s&per_page=100&page=%d&sort=created&direction=asc", github.RepoPath(sync.FullName), which, sync.Page)
		next, err = state.GitHub.Do(ctx, token, "GET", path, nil, &issues)
	}
	if err != nil {
		failErr := app.Exec(ctx, state.DB, "UPDATE github_repo_syncs SET backfill_status = 'failed', updated_at = now() WHERE id = $1", id)
		return false, errors.Join(recordSync(ctx, state, id, err), failErr)
	}
	var counts backfillCounts
	for i := range issues {
		if err := backfillIssue(ctx, state, &sync.repoSync, token, &issues[i], &counts); err != nil {
			counts.skipped++
			counts.failed = err
		}
	}
	status := "running"
	if next == 0 {
		status = "completed"
	}
	err = app.Exec(ctx, state.DB,
		`UPDATE github_repo_syncs SET backfill_status = $2, backfill_page = $3, issues_created = issues_created + $4,
			issues_updated = issues_updated + $5, comments_created = comments_created + $6, skipped = skipped + $7, updated_at = now()
		 WHERE id = $1`, id, status, max(next, 1), counts.created, counts.updated, counts.comments, counts.skipped)
	if err != nil {
		return false, err
	}
	if err := recordSync(ctx, state, id, counts.failed); err != nil {
		slog.Warn("some GitHub issues weren't imported", "repoSync", id, "err", err)
	}
	return next > 0, nil
}

func backfillIssue(ctx context.Context, state *app.State, sync *repoSync, token string, issue *github.Issue, counts *backfillCounts) error {
	if issue.IsPullRequest() {
		counts.skipped++
		return nil
	}
	link, err := findIssueLink(ctx, state.DB, sync, issue.ID)
	if err != nil {
		return err
	}
	if link == nil {
		created, err := importGitHubIssue(ctx, state, sync, issue)
		if err != nil {
			return err
		}
		if created {
			counts.created++
		}
		if link, err = findIssueLink(ctx, state.DB, sync, issue.ID); link == nil || err != nil {
			return err
		}
	} else {
		changed, err := reconcileGitHubIssue(ctx, state, sync, link, issue)
		if err != nil {
			return err
		}
		if !changed {
			counts.skipped++
			return nil
		}
		counts.updated++
	}
	for page := 1; page > 0 && issue.Comments > 0; {
		var comments []github.Comment
		path := fmt.Sprintf("%s/comments?per_page=100&page=%d", github.IssuePath(sync.FullName, issue.Number), page)
		if page, err = state.GitHub.Do(ctx, token, "GET", path, nil, &comments); err != nil {
			return err
		}
		for i := range comments {
			created, err := importGitHubComment(ctx, state, sync, link, &comments[i])
			if err != nil {
				return err
			}
			if created {
				counts.comments++
			}
		}
	}
	return nil
}

// reconcileGitHubIssue brings a linked issue up to date with GitHub when GitHub changed it since.
func reconcileGitHubIssue(ctx context.Context, state *app.State, sync *repoSync, link *issueLink, issue *github.Issue) (bool, error) {
	if !issue.UpdatedAt.After(link.UpdatedAt) {
		return false, nil
	}
	c := app.SystemCtx(ctx, state, sync.Workspace, uuid.Nil,
		app.Origin{Label: "Synced from GitHub #" + strconv.Itoa(issue.Number), URL: issue.HTMLURL, FromGitHubIssue: true})
	row, err := loadIssueRow(c, c.DB(), link.Issue)
	if err != nil {
		return false, err
	}
	args := SaveIssueArgs{}
	changed := false
	if title := clampTitle(issue.Title); title != row.Title {
		args.Title, changed = &title, true
	}
	if issue.Body != row.Description {
		args.Description, changed = &issue.Body, true
	}
	current, err := stateType(c, c.DB(), row.StateID)
	if err != nil {
		return false, err
	}
	if !matchesClosure(current, issue.Closure()) {
		status, err := statusFor(c, row.TeamID, issue.Closure())
		if err != nil {
			return false, err
		}
		args.State, changed = new(status.String()), true
	}
	for _, label := range issue.Labels {
		id, err := labelFor(c, row.TeamID, label)
		if err != nil {
			return false, err
		}
		has, err := app.One[bool](c, c.DB(), "SELECT EXISTS (SELECT 1 FROM issue_label_links WHERE issue_id = $1 AND label_id = $2)", row.ID, id)
		if err != nil {
			return false, err
		}
		if !has {
			args.AddLabels, changed = append(args.AddLabels, id.String()), true
		}
	}
	if assignee := assignable(c, memberByLogin(ctx, state.DB, sync.Workspace, issue.Assignee), row.TeamID); assignee != uuid.Nil && !samePtr(row.AssigneeID, &assignee) {
		args.Assignee, changed = some(assignee.String()), true
	}
	err = inTx(c, func(tx pgx.Tx) error {
		if changed {
			if err := updateIssue(c, tx, link.Issue, args); err != nil {
				return err
			}
		}
		return app.Exec(c, tx, "UPDATE github_issue_links SET github_updated_at = greatest(github_updated_at, $2) WHERE issue_id = $1",
			link.Issue, issue.UpdatedAt)
	})
	return changed, err
}
