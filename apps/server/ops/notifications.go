package ops

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
)

// The inbox: notifications about what happened to the things a person is involved in, which they
// read, archive or snooze.

func notificationOps() []*Op {
	return []*Op{
		op("get_notifications", app.Read, []string{"GET /notifications"},
			"Your inbox, newest first: assignments, mentions, comments, replies, status changes and more, with how many "+
				"are unread. Archived notifications and snoozed ones (until they wake) are left out unless asked for.",
			getNotifications),
		op("mark_notification", app.Write, []string{"POST /notifications/mark"},
			"Mark one or more of your notifications read or unread, archive or unarchive them, or snooze them until a "+
				"time (it comes back unread then) or unsnooze them.",
			markNotification),
		op("mark_all_notifications_read", app.Write, []string{"POST /notifications/read-all"},
			"Mark all your unread notifications read, optionally only those before a time or about one issue.",
			markAllNotificationsRead),
	}
}

const notificationJSON = `jsonb_build_object(
	'id', n.id,
	'type', n.type,
	'summary', n.summary,
	'actor', user_ref(n.actor_id),
	'issue', issue_ref(n.issue_id),
	'comment', (SELECT jsonb_build_object('id', c.id, 'body', left(c.body, 200)) FROM comments c WHERE c.id = n.comment_id),
	'project', project_ref(n.project_id),
	'initiative', initiative_ref(n.initiative_id),
	'document', document_ref(n.document_id),
	'statusUpdate', CASE WHEN n.status_update_id IS NOT NULL THEN jsonb_build_object('id', n.status_update_id) END,
	'readAt', n.read_at,
	'snoozedUntil', n.snoozed_until,
	'archivedAt', n.archived_at,
	'createdAt', n.created_at
)`

type GetNotificationsArgs struct {
	UnreadOnly      bool    `json:"unreadOnly,omitempty" doc:"Only unread ones."`
	IncludeArchived bool    `json:"includeArchived,omitempty" doc:"Also archived ones."`
	IncludeSnoozed  bool    `json:"includeSnoozed,omitempty" doc:"Also ones snoozed until later."`
	Type            *string `json:"type" doc:"Only this type, e.g. issueAssignedToYou, issueMention, issueNewComment, issueCommentReply, issueStatusChanged."`
	Page
}

func getNotifications(c *app.Ctx, args GetNotificationsArgs) (any, error) {
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	nodes, err := app.All[any](c, c.DB(),
		`SELECT `+notificationJSON+` FROM notifications n WHERE n.user_id = $1 AND n.workspace_id = $2
		 AND (NOT $3 OR n.read_at IS NULL)
		 AND ($4 OR n.archived_at IS NULL)
		 AND ($5 OR n.snoozed_until IS NULL OR n.snoozed_until <= now())
		 AND ($6::text IS NULL OR n.type = $6)
		 ORDER BY n.created_at DESC, n.id LIMIT $7 OFFSET $8`,
		c.User, c.Workspace, args.UnreadOnly, args.IncludeArchived, args.IncludeSnoozed, trimmedOrNil(args.Type),
		args.limit()+1, offset)
	if err != nil {
		return nil, err
	}
	unread, err := app.One[int64](c, c.DB(),
		`SELECT count(*) FROM notifications WHERE user_id = $1 AND workspace_id = $2 AND read_at IS NULL
		 AND archived_at IS NULL AND (snoozed_until IS NULL OR snoozed_until <= now())`, c.User, c.Workspace)
	if err != nil {
		return nil, err
	}
	value, err := args.wrap(nodes)
	if err != nil {
		return nil, err
	}
	value["unreadCount"] = unread
	return value, nil
}

type MarkNotificationArgs struct {
	ID     *string  `json:"id" doc:"One notification's id."`
	IDs    []string `json:"ids" doc:"Several notifications' ids."`
	Action string   `json:"action" doc:"read, unread, archive, unarchive, snooze or unsnooze."`
	Until  *string  `json:"until" doc:"For snooze: until when, as ISO 8601 or a duration from now like PT3H or P1D."`
}

func markNotification(c *app.Ctx, args MarkNotificationArgs) (any, error) {
	values := args.IDs
	if args.ID != nil {
		values = append([]string{*args.ID}, values...)
	}
	ids := []uuid.UUID{}
	for _, value := range values {
		id, ok := asUUID(value)
		if !ok {
			return nil, app.NotFound("No notification %q", value)
		}
		ids = appendUnique(ids, id)
	}
	if len(ids) == 0 || len(ids) > 250 {
		return nil, app.BadRequest("Pass id or ids (at most 250)")
	}
	var until *time.Time
	switch {
	case args.Action == "snooze" && args.Until != nil:
		at, err := parseDatetime(*args.Until)
		if err != nil {
			return nil, err
		}
		if !at.After(time.Now()) {
			return nil, app.BadRequest("until must be in the future")
		}
		until = &at
	case args.Action == "snooze":
		return nil, app.BadRequest("Pass until to snooze, e.g. PT3H, P1D or an ISO time")
	case args.Until != nil:
		return nil, app.BadRequest("until is only for snooze")
	}
	set, ok := map[string]string{
		"read":      "read_at = coalesce(read_at, now())",
		"unread":    "read_at = NULL",
		"archive":   "archived_at = coalesce(archived_at, now())",
		"unarchive": "archived_at = NULL",
		"snooze":    "snoozed_until = $4",
		"unsnooze":  "snoozed_until = NULL",
	}[args.Action]
	if !ok {
		return nil, app.BadRequest("action %q isn't one of read, unread, archive, unarchive, snooze, unsnooze", args.Action)
	}
	var nodes []any
	err := inTx(c, func(tx pgx.Tx) error {
		found, err := app.One[int64](c, tx,
			"SELECT count(*) FROM notifications WHERE id = ANY($1) AND user_id = $2 AND workspace_id = $3", ids, c.User, c.Workspace)
		if err != nil {
			return err
		}
		if int(found) != len(ids) {
			return app.NotFound("No such notification in your inbox")
		}
		params := []any{ids, c.User, c.Workspace}
		if until != nil {
			params = append(params, *until)
		}
		if err := app.Exec(c, tx, "UPDATE notifications SET "+set+" WHERE id = ANY($1) AND user_id = $2 AND workspace_id = $3",
			params...); err != nil {
			return err
		}
		nodes, err = app.All[any](c, tx,
			"SELECT "+notificationJSON+" FROM notifications n WHERE n.id = ANY($1) ORDER BY n.created_at DESC, n.id", ids)
		return err
	})
	if err != nil {
		return nil, err
	}
	return M{"nodes": nonNil(nodes)}, nil
}

type MarkAllNotificationsReadArgs struct {
	Before *string `json:"before" doc:"Only notifications created before this time (ISO 8601 or a duration like -P1D)."`
	Issue  *string `json:"issue" doc:"Only notifications about this issue (identifier, id or link)."`
}

func markAllNotificationsRead(c *app.Ctx, args MarkAllNotificationsReadArgs) (any, error) {
	var before *time.Time
	if args.Before != nil {
		at, err := parseDatetime(*args.Before)
		if err != nil {
			return nil, err
		}
		before = &at
	}
	var issue *uuid.UUID
	if args.Issue != nil {
		id, err := resolveIssue(c, *args.Issue)
		if err != nil {
			return nil, err
		}
		issue = &id
	}
	tag, err := c.DB().Exec(c,
		`UPDATE notifications SET read_at = now() WHERE user_id = $1 AND workspace_id = $2 AND read_at IS NULL
		 AND ($3::timestamptz IS NULL OR created_at < $3) AND ($4::uuid IS NULL OR issue_id = $4)`,
		c.User, c.Workspace, before, issue)
	if err != nil {
		return nil, err
	}
	return M{"marked": tag.RowsAffected()}, nil
}
