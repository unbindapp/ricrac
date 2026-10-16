package ops

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/google/uuid"

	"ricrac/app"
	"ricrac/github"
)

// Changes made in Ricrac to issues linked to GitHub issues, and to their comments, go to GitHub:
// they are queued in the change's transaction and sent by PushGitHubChanges with the
// installation's token. Changes that came from GitHub issues aren't queued.

// pushedFields are the issue fields GitHub has too.
var pushedFields = []string{"title", "description", "state", "labels", "assignee"}

// pushBackoff is the wait before the 2nd, 3rd, ... attempt; after the last the change is dropped.
var pushBackoff = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour}

func mirrorsGitHub(c *app.Ctx) bool {
	return c.Origin != nil && c.Origin.FromGitHubIssue
}

func queueIssueForGitHub(c *app.Ctx, tx app.Querier, issue uuid.UUID, changes M) error {
	pushed := slices.ContainsFunc(pushedFields, func(field string) bool { _, ok := changes[field]; return ok })
	if mirrorsGitHub(c) || !pushed {
		return nil
	}
	tag, err := tx.Exec(c,
		`INSERT INTO github_outbox (repo_sync_id, github_number, issue_id, kind, changes)
		 SELECT repo_sync_id, github_number, issue_id, 'issue', $2 FROM github_issue_links WHERE issue_id = $1`, issue, changes)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		c.State.QueueGitHub()
	}
	return nil
}

// queueCommentForGitHub queues a new comment on a linked issue, or the edit or deletion of a
// comment the app posted (with its replies).
func queueCommentForGitHub(c *app.Ctx, tx app.Querier, issue, comment uuid.UUID, kind string) error {
	if mirrorsGitHub(c) {
		return nil
	}
	sql := `INSERT INTO github_outbox (repo_sync_id, github_number, issue_id, comment_id, kind)
		SELECT repo_sync_id, github_number, issue_id, $2, $3 FROM github_issue_links WHERE issue_id = $1`
	if kind != "comment_created" {
		sql = `INSERT INTO github_outbox (repo_sync_id, github_number, issue_id, comment_id, github_comment_id, kind)
			SELECT l.repo_sync_id, l.github_number, l.issue_id, cl.comment_id, cl.github_id, $3
			FROM github_comment_links cl JOIN github_issue_links l ON l.issue_id = cl.issue_id
			WHERE l.issue_id = $1 AND cl.posted AND cl.comment_id IN (SELECT id FROM comments WHERE id = $2 OR parent_id = $2)`
	}
	tag, err := tx.Exec(c, sql, issue, comment, kind)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		c.State.QueueGitHub()
	}
	return nil
}

type outboxEntry struct {
	ID            int64
	RepoSync      uuid.UUID
	Number        int
	Issue         *uuid.UUID
	Comment       *uuid.UUID
	GitHubComment *int64
	Kind          string
	Changes       M
	Attempts      int
	Installation  int64
	FullName      string
}

// PushGitHubChanges sends the queued changes that are due, answering how many it tried.
func PushGitHubChanges(ctx context.Context, state *app.State) (int, error) {
	if state.GitHub == nil {
		return 0, nil
	}
	entries, err := app.Rows[outboxEntry](ctx, state.DB,
		`SELECT o.id, o.repo_sync_id, o.github_number, o.issue_id, o.comment_id, o.github_comment_id, o.kind, o.changes,
			o.attempts, r.installation_id, r.full_name
		 FROM github_outbox o JOIN github_repo_syncs r ON r.id = o.repo_sync_id
		 JOIN github_installations g ON g.id = r.installation_id
		 WHERE o.next_attempt_at <= now() AND g.suspended_at IS NULL ORDER BY o.id LIMIT 50`)
	if err != nil {
		return 0, err
	}
	for _, entry := range entries {
		pushErr := pushEntry(ctx, state, entry)
		var err error
		if pushErr == nil || entry.Attempts >= len(pushBackoff) {
			err = app.Exec(ctx, state.DB, "DELETE FROM github_outbox WHERE id = $1", entry.ID)
		} else {
			err = app.Exec(ctx, state.DB,
				"UPDATE github_outbox SET attempts = attempts + 1, next_attempt_at = now() + make_interval(secs => $2) WHERE id = $1",
				entry.ID, pushBackoff[entry.Attempts].Seconds())
		}
		if err != nil {
			return len(entries), err
		}
		_ = recordSync(ctx, state, entry.RepoSync, pushErr)
	}
	return len(entries), nil
}

func pushEntry(ctx context.Context, state *app.State, entry outboxEntry) error {
	token, err := state.GitHub.InstallationToken(ctx, entry.Installation)
	if err != nil {
		return err
	}
	switch entry.Kind {
	case "issue":
		return pushIssue(ctx, state, token, entry)
	case "comment_created":
		return pushNewComment(ctx, state, token, entry)
	case "comment_edited":
		body, found, err := commentForGitHub(ctx, state, entry.Comment)
		if err != nil || !found || entry.GitHubComment == nil {
			return err
		}
		_, err = state.GitHub.Do(ctx, token, http.MethodPatch, github.CommentPath(entry.FullName, *entry.GitHubComment), M{"body": body}, nil)
		return err
	case "comment_deleted":
		if entry.GitHubComment == nil {
			return nil
		}
		_, err := state.GitHub.Do(ctx, token, http.MethodDelete, github.CommentPath(entry.FullName, *entry.GitHubComment), nil, nil)
		if github.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

// commentForGitHub is a comment's body as the app posts it, with its author's name.
func commentForGitHub(ctx context.Context, state *app.State, comment *uuid.UUID) (string, bool, error) {
	if comment == nil {
		return "", false, nil
	}
	type row struct {
		Body   string
		Author *string
	}
	found, ok, err := app.Row[row](ctx, state.DB,
		"SELECT c.body, u.name FROM comments c LEFT JOIN users u ON u.id = c.user_id WHERE c.id = $1", *comment)
	if err != nil || !ok {
		return "", false, err
	}
	author := "Someone"
	if found.Author != nil {
		author = *found.Author
	}
	return github.RicracCommentBody(author, found.Body), true, nil
}

func pushNewComment(ctx context.Context, state *app.State, token string, entry outboxEntry) error {
	body, found, err := commentForGitHub(ctx, state, entry.Comment)
	if err != nil || !found {
		return err
	}
	posted, err := app.One[bool](ctx, state.DB, "SELECT EXISTS (SELECT 1 FROM github_comment_links WHERE comment_id = $1)", *entry.Comment)
	if err != nil || posted {
		return err
	}
	var created github.Comment
	_, err = state.GitHub.Do(ctx, token, http.MethodPost, github.IssuePath(entry.FullName, entry.Number)+"/comments", M{"body": body}, &created)
	if err != nil {
		return err
	}
	return app.Exec(ctx, state.DB,
		`INSERT INTO github_comment_links (comment_id, issue_id, github_id, posted)
		 SELECT $1, issue_id, $3, true FROM github_issue_links WHERE issue_id = $2 ON CONFLICT DO NOTHING`,
		*entry.Comment, entry.Issue, created.ID)
}

// gitHubState is how a status type reads on GitHub: open, or closed as completed or not planned.
func gitHubState(kind string) (string, string) {
	switch kind {
	case "completed":
		return "closed", "completed"
	case "canceled":
		return "closed", "not_planned"
	}
	return "open", ""
}

// change reads a history change's from and to.
func change(changes M, field string) (any, any, bool) {
	value, ok := changes[field].(M)
	if !ok {
		return nil, nil, false
	}
	return value["from"], value["to"], true
}

func refField(ref any, key string) string {
	object, _ := ref.(M)
	text, _ := object[key].(string)
	return text
}

func names(list any) []string {
	items, _ := list.([]any)
	var all []string
	for _, item := range items {
		if text, ok := item.(string); ok {
			all = append(all, text)
		}
	}
	return all
}

func pushIssue(ctx context.Context, state *app.State, token string, entry outboxEntry) error {
	if entry.Issue == nil {
		return nil
	}
	type current struct {
		Title       string
		Description string
	}
	issue, found, err := app.Row[current](ctx, state.DB, "SELECT title, description FROM issues WHERE id = $1", *entry.Issue)
	if err != nil || !found {
		return err
	}
	patch := M{}
	if _, ok := entry.Changes["title"]; ok {
		patch["title"] = issue.Title
	}
	if _, ok := entry.Changes["description"]; ok {
		patch["body"] = issue.Description
	}
	if from, to, ok := change(entry.Changes, "state"); ok {
		before, beforeReason := gitHubState(refField(from, "type"))
		after, reason := gitHubState(refField(to, "type"))
		if before != after || beforeReason != reason {
			patch["state"] = after
			if reason != "" {
				patch["state_reason"] = reason
			}
		}
	}
	path := github.IssuePath(entry.FullName, entry.Number)
	if len(patch) > 0 {
		var updated github.Issue
		if _, err := state.GitHub.Do(ctx, token, http.MethodPatch, path, patch, &updated); err != nil {
			return err
		}
		err := app.Exec(ctx, state.DB,
			"UPDATE github_issue_links SET github_updated_at = greatest(github_updated_at, $2) WHERE issue_id = $1", *entry.Issue, updated.UpdatedAt)
		if err != nil {
			return err
		}
	}
	if from, to, ok := change(entry.Changes, "labels"); ok {
		before, after := names(from), names(to)
		added := slices.DeleteFunc(slices.Clone(after), func(name string) bool { return slices.Contains(before, name) })
		if len(added) > 0 {
			if _, err := state.GitHub.Do(ctx, token, http.MethodPost, path+"/labels", M{"labels": added}, nil); err != nil {
				return err
			}
		}
		for _, name := range before {
			if slices.Contains(after, name) {
				continue
			}
			_, err := state.GitHub.Do(ctx, token, http.MethodDelete, path+"/labels/"+url.PathEscape(name), nil, nil)
			if err != nil && !github.IsNotFound(err) {
				return err
			}
		}
	}
	if from, to, ok := change(entry.Changes, "assignee"); ok {
		for _, step := range []struct {
			method string
			user   any
		}{{http.MethodDelete, from}, {http.MethodPost, to}} {
			login, err := app.One[*string](ctx, state.DB, "SELECT github_login FROM users WHERE id::text = $1", refField(step.user, "id"))
			if app.NoRows(err) || (err == nil && login == nil) {
				continue
			}
			if err != nil {
				return err
			}
			if _, err := state.GitHub.Do(ctx, token, step.method, path+"/assignees", M{"assignees": []string{*login}}, nil); err != nil {
				return err
			}
		}
	}
	return nil
}
