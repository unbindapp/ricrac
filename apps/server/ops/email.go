package ops

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
	"ricrac/mail"
)

// Emails are queued in the transaction of the change that causes them and sent by the job loop,
// so they only go out when the change happened. Notifications become digests: those still unread
// after a couple of minutes are gathered per person into one email.

const (
	digestDelay    = 2 * time.Minute
	digestMaxAge   = 24 * time.Hour
	digestMaxItems = 20
	sendAttempts   = 5
)

// queueEmail queues an email to go out once the transaction commits.
func queueEmail(ctx context.Context, tx app.Querier, to string, content mail.Content) error {
	return app.Exec(ctx, tx, "INSERT INTO emails (recipient, content) VALUES ($1, $2)", to, content)
}

// SendEmails gathers due notification digests and sends the queued emails. Without a mailer it
// does nothing, and notifications stay unemailed.
func SendEmails(ctx context.Context, state *app.State) error {
	if state.Mailer == nil {
		return nil
	}
	if err := queueDigests(ctx, state); err != nil {
		return err
	}
	return sendQueued(ctx, state)
}

type pendingNotice struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Email       string
	Kind        string
	Summary     string
	ActorID     *uuid.UUID
	ActorName   *string
	WorkspaceID uuid.UUID
	Workspace   string
	URLKey      string
	Subject     *uuid.UUID
	Identifier  *string
	Title       *string
	URL         *string
}

func queueDigests(ctx context.Context, state *app.State) error {
	return pgx.BeginFunc(ctx, state.DB, func(tx pgx.Tx) error {
		cutoff := time.Now().Add(-digestDelay)
		pending, err := app.Rows[pendingNotice](ctx, tx,
			`SELECT n.id, n.user_id, u.email, n.type, n.summary, n.actor_id, a.name, n.workspace_id, w.name, w.url_key,
				coalesce(n.issue_id, n.project_id, n.initiative_id, n.document_id),
				issue_identifier(n.issue_id),
				coalesce(i.title, p.name, ini.name, d.title),
				CASE
					WHEN n.issue_id IS NOT NULL THEN $2 || '/' || w.url_key || '/issue/' || issue_identifier(n.issue_id)
					WHEN n.project_id IS NOT NULL THEN $2 || '/' || w.url_key || '/project/' || concat_ws('-', nullif(entity_slug(p.name), ''), p.slug_id)
					WHEN n.initiative_id IS NOT NULL THEN $2 || '/' || w.url_key || '/initiative/' || concat_ws('-', nullif(entity_slug(ini.name), ''), ini.slug_id)
					WHEN n.document_id IS NOT NULL THEN $2 || '/' || w.url_key || '/document/' || concat_ws('-', nullif(entity_slug(d.title), ''), d.slug_id)
				END
			 FROM notifications n
			 JOIN users u ON u.id = n.user_id
			 JOIN workspaces w ON w.id = n.workspace_id
			 LEFT JOIN users a ON a.id = n.actor_id
			 LEFT JOIN issues i ON i.id = n.issue_id
			 LEFT JOIN projects p ON p.id = n.project_id
			 LEFT JOIN initiatives ini ON ini.id = n.initiative_id
			 LEFT JOIN documents d ON d.id = n.document_id
			 WHERE n.emailed_at IS NULL AND n.created_at <= $1 AND n.created_at > $3
			   AND n.read_at IS NULL AND n.archived_at IS NULL AND (n.snoozed_until IS NULL OR n.snoozed_until <= now())
			   AND u.email_notifications
			 ORDER BY n.user_id, n.created_at
			 FOR UPDATE OF n SKIP LOCKED`,
			cutoff, state.Config.PublicURL, time.Now().Add(-digestMaxAge))
		if err != nil {
			return err
		}
		var batch []pendingNotice
		flush := func() error {
			if len(batch) == 0 {
				return nil
			}
			content := digest(batch, state.Config.PublicURL)
			batch = nil
			return queueEmail(ctx, tx, content.to, content.Content)
		}
		for _, notice := range pending {
			if len(batch) > 0 && batch[0].UserID != notice.UserID {
				if err := flush(); err != nil {
					return err
				}
			}
			batch = append(batch, notice)
		}
		if err := flush(); err != nil {
			return err
		}
		return app.Exec(ctx, tx, "UPDATE notifications SET emailed_at = now() WHERE emailed_at IS NULL AND created_at <= $1", cutoff)
	})
}

type addressed struct {
	mail.Content
	to string
}

func digest(batch []pendingNotice, publicURL string) addressed {
	count := len(batch)
	var items []mail.Item
	index := map[string]int{}
	for _, notice := range batch[:min(count, digestMaxItems)] {
		key := notice.ID.String()
		if notice.Subject != nil {
			key = notice.Subject.String()
		}
		position, seen := index[key]
		if !seen {
			position = len(items)
			index[key] = position
			items = append(items, digestItem(notice))
		}
		items[position].Lines = append(items[position].Lines, digestLine(notice))
	}
	heading := fmt.Sprintf("You have %d unread notifications", count)
	if count == 1 {
		heading = "You have an unread notification"
	}
	sameWorkspace := true
	for _, notice := range batch {
		sameWorkspace = sameWorkspace && notice.WorkspaceID == batch[0].WorkspaceID
	}
	inbox := publicURL
	if sameWorkspace {
		heading += " in " + batch[0].Workspace
		inbox = publicURL + "/" + batch[0].URLKey + "/inbox"
	}
	subject := fmt.Sprintf("%d unread notifications", count)
	if count == 1 {
		subject = strings.TrimSpace(items[0].Identifier+" "+items[0].Title) + ": " + lineSentence(items[0].Lines[0])
	}
	content := mail.Content{
		Subject:   subject,
		Preheader: lineSentence(items[0].Lines[0]),
		Heading:   heading + ".",
		Items:     items,
		Button:    mail.Button{Label: "Open your inbox", URL: inbox},
		Footer: mail.Footer{
			InstanceURL: publicURL,
			ManageURL:   publicURL + "/settings/profile",
			ManageLabel: "Notification settings",
		},
	}
	if count > digestMaxItems {
		content.Note = fmt.Sprintf("And %d more in your inbox.", count-digestMaxItems)
	}
	return addressed{content, batch[0].Email}
}

func digestItem(notice pendingNotice) mail.Item {
	item := mail.Item{Title: notice.Summary, URL: orEmpty(notice.URL)}
	if notice.Title != nil {
		item.Title = *notice.Title
	}
	if notice.Identifier != nil {
		item.Identifier = *notice.Identifier
	}
	return item
}

// actorSentences say what the actor of a notification did, by its kind.
var actorSentences = map[string]string{
	"issueAssignedToYou":      "assigned the issue to you",
	"issueUnassignedFromYou":  "unassigned you from the issue",
	"issueMention":            "mentioned you",
	"issueShared":             "shared the issue with you",
	"projectMention":          "mentioned you in the project",
	"projectAssigned":         "made you the project lead",
	"documentMention":         "mentioned you in the document",
	"projectUpdateCreated":    "posted an update",
	"projectUpdateMention":    "mentioned you in an update",
	"initiativeUpdateCreated": "posted an update",
}

func digestLine(notice pendingNotice) mail.Line {
	sentence, known := actorSentences[notice.Kind]
	switch {
	case strings.HasSuffix(notice.Kind, "CommentMention"):
		sentence, known = "mentioned you in a comment", true
	case strings.HasSuffix(notice.Kind, "CommentReply"):
		sentence, known = "replied to a thread you're in", true
	case strings.HasSuffix(notice.Kind, "NewComment"):
		sentence, known = "commented", true
	case notice.Kind == "issueStatusChanged":
		if _, state, found := strings.Cut(notice.Summary, " moved to "); found {
			sentence, known = "moved the issue to "+state, true
		}
	}
	if !known || notice.ActorID == nil || notice.ActorName == nil {
		text := notice.Summary
		if notice.Identifier != nil {
			text = upperFirst(strings.TrimSpace(strings.TrimPrefix(text, *notice.Identifier)))
		}
		return mail.Line{Text: text}
	}
	return mail.ActorLine(notice.ActorID.String(), *notice.ActorName, sentence)
}

func lineSentence(line mail.Line) string {
	return strings.TrimSpace(line.ActorName + " " + line.Text)
}

func upperFirst(text string) string {
	if text == "" {
		return text
	}
	return strings.ToUpper(text[:1]) + text[1:]
}

type queuedEmail struct {
	ID        uuid.UUID
	Recipient string
	Content   mail.Content
	Attempts  int
}

func sendQueued(ctx context.Context, state *app.State) error {
	due, err := app.Rows[queuedEmail](ctx, state.DB,
		`SELECT id, recipient, content, attempts FROM emails
		 WHERE sent_at IS NULL AND attempts < $1 AND send_after <= now() ORDER BY created_at LIMIT 100`, sendAttempts)
	if err != nil {
		return err
	}
	for _, email := range due {
		message, err := mail.Render(email.Recipient, email.Content)
		if err == nil {
			err = state.Mailer.Send(ctx, message)
		}
		if err == nil {
			if err := app.Exec(ctx, state.DB, "UPDATE emails SET sent_at = now(), attempts = attempts + 1, error = NULL WHERE id = $1", email.ID); err != nil {
				return err
			}
			continue
		}
		slog.Warn("email failed", "to", email.Recipient, "attempt", email.Attempts+1, "err", err)
		backoff := time.Duration(1<<(2*email.Attempts)) * time.Minute
		if err := app.Exec(ctx, state.DB, "UPDATE emails SET attempts = attempts + 1, error = $2, send_after = now() + make_interval(secs => $3) WHERE id = $1",
			email.ID, err.Error(), backoff.Seconds()); err != nil {
			return err
		}
	}
	return nil
}
