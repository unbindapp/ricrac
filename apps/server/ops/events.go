package ops

import (
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"ricrac/app"
)

// What happens around a change: issue history, inbox notifications, webhook deliveries and the
// audit log. Everything is written in the change's transaction, so it only exists if the change
// does.

// emit queues a webhook delivery for every enabled webhook of the workspace that listens to kind
// (Issue, Comment, Project, ...) on this team, or on every team.
func emit(c *app.Ctx, tx app.Querier, kind, action string, data any, updatedFrom any, team *uuid.UUID) error {
	hooks, err := app.All[uuid.UUID](c, tx,
		`SELECT id FROM webhooks WHERE workspace_id = $1 AND enabled AND $2 = ANY(resource_types)
		 AND (team_id IS NULL OR team_id = $3)`, c.Workspace, kind, team)
	if err != nil || len(hooks) == 0 {
		return err
	}
	var url any
	if object, ok := data.(M); ok {
		url = object["url"]
	}
	for _, hook := range hooks {
		payload := M{
			"action":      action,
			"type":        kind,
			"data":        data,
			"url":         url,
			"actor":       M{"id": c.User},
			"createdAt":   time.Now().UTC(),
			"workspaceId": c.Workspace,
			"webhookId":   hook,
		}
		if updatedFrom != nil {
			payload["updatedFrom"] = updatedFrom
		}
		if err := app.Exec(c, tx, "INSERT INTO webhook_deliveries (webhook_id, payload) VALUES ($1, $2)", hook, payload); err != nil {
			return err
		}
	}
	c.State.QueueDeliveries()
	return nil
}

// history adds an entry to an issue's history. changes maps fields to {from, to}.
func history(c *app.Ctx, tx app.Querier, issue uuid.UUID, action string, changes any) error {
	return app.Exec(c, tx, "INSERT INTO issue_history (issue_id, actor_id, action, changes) VALUES ($1, $2, $3, $4)",
		issue, c.User, action, changes)
}

func audit(c *app.Ctx, tx app.Querier, kind string, metadata any) error {
	return app.Exec(c, tx, "INSERT INTO audit_entries (workspace_id, actor_id, type, metadata) VALUES ($1, $2, $3, $4)",
		c.Workspace, c.User, kind, metadata)
}

// Notice is what an inbox notification is about. Unset targets stay empty.
type Notice struct {
	// issueAssignedToYou, issueMention, issueComment, issueCommentReply, issueStatusChanged,
	// issueNewComment, issuePriorityChanged, issueBlocking, issueUnblocked, issueDue,
	// issueCreated (triage), issueSubscribed, projectUpdateCreated, documentMention, ...
	Kind         string
	Summary      string
	Issue        *uuid.UUID
	Comment      *uuid.UUID
	Project      *uuid.UUID
	Initiative   *uuid.UUID
	Document     *uuid.UUID
	StatusUpdate *uuid.UUID
}

// notify notifies each of users once, except the caller, who doesn't need to hear about their
// own change, and anyone no longer in the workspace.
func notify(c *app.Ctx, tx app.Querier, users []uuid.UUID, notice Notice) error {
	var seen []uuid.UUID
	for _, user := range users {
		if user == c.User || slices.Contains(seen, user) {
			continue
		}
		seen = append(seen, user)
		err := app.Exec(c, tx,
			`INSERT INTO notifications (workspace_id, user_id, actor_id, type, issue_id, comment_id, project_id,
				initiative_id, document_id, status_update_id, summary)
			 SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
			 WHERE EXISTS (SELECT 1 FROM workspace_members WHERE workspace_id = $1 AND user_id = $2 AND active)`,
			c.Workspace, user, c.User, notice.Kind, notice.Issue, notice.Comment, notice.Project, notice.Initiative,
			notice.Document, notice.StatusUpdate, notice.Summary)
		if err != nil {
			return err
		}
	}
	return nil
}

func issueSubscribers(c *app.Ctx, tx app.Querier, issue uuid.UUID) ([]uuid.UUID, error) {
	return app.All[uuid.UUID](c, tx, "SELECT user_id FROM issue_subscribers WHERE issue_id = $1", issue)
}

func subscribe(c *app.Ctx, tx app.Querier, issue uuid.UUID, users []uuid.UUID) error {
	return app.Exec(c, tx,
		"INSERT INTO issue_subscribers (issue_id, user_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING",
		issue, users)
}

// mentions is the members mentioned in Markdown as @displayName, @email or @ + the part of
// their email before the @.
func mentions(c *app.Ctx, text string) ([]uuid.UUID, error) {
	handles := mentionHandles(text)
	if len(handles) == 0 {
		return nil, nil
	}
	return app.All[uuid.UUID](c, c.DB(),
		`SELECT u.id FROM users u JOIN workspace_members m ON m.user_id = u.id AND m.workspace_id = $1 AND m.active
		 WHERE lower(u.display_name) = ANY($2) OR lower(u.email) = ANY($2) OR lower(split_part(u.email, '@', 1)) = ANY($2)`,
		c.Workspace, handles)
}

func mentionHandles(text string) []string {
	var handles []string
	chars := []rune(text)
	for i, c := range chars {
		startsWord := i == 0 || !(unicode.IsLetter(chars[i-1]) || unicode.IsDigit(chars[i-1]) || chars[i-1] == '@')
		if c != '@' || !startsWord {
			continue
		}
		var handle []rune
		for _, h := range chars[i+1:] {
			if !(unicode.IsLetter(h) || unicode.IsDigit(h) || strings.ContainsRune("._-@+", h)) {
				break
			}
			handle = append(handle, h)
		}
		name := strings.ToLower(strings.TrimRight(string(handle), ".-"))
		if name != "" && !slices.Contains(handles, name) {
			handles = append(handles, name)
		}
	}
	return handles
}
