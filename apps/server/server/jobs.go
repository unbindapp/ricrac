package server

import (
	"context"
	"log/slog"
	"time"

	"ricrac/app"
	"ricrac/ops"
)

// StartJobs runs the work that happens on its own until ctx ends: workspace automations every
// minute, webhook deliveries as soon as they are queued, and the GitHub sync.
func StartJobs(ctx context.Context, state *app.State) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			if err := EveryMinute(ctx, state); err != nil && ctx.Err() == nil {
				slog.Warn("background job failed", "err", app.AsError(err).Message)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	go func() {
		for ctx.Err() == nil {
			sent, err := ops.DeliverDueWebhooks(ctx, state)
			if err != nil && ctx.Err() == nil {
				slog.Warn("webhook delivery failed", "err", app.AsError(err).Message)
			}
			if sent > 0 {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case <-state.Deliveries:
			case <-time.After(10 * time.Second):
			}
			time.Sleep(200 * time.Millisecond)
		}
	}()
	if state.GitHub != nil {
		go func() {
			for ctx.Err() == nil {
				busy, err := GitHubJobs(ctx, state)
				if err != nil && ctx.Err() == nil {
					slog.Warn("GitHub sync failed", "err", app.AsError(err).Message)
				}
				if busy && err == nil {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case <-state.GitHubWork:
				case <-time.After(30 * time.Second):
				}
				time.Sleep(200 * time.Millisecond)
			}
		}()
	}
}

// GitHubJobs sends the changes queued for GitHub and imports a page of each repository's existing
// issues, saying whether there's more to do right away.
func GitHubJobs(ctx context.Context, state *app.State) (bool, error) {
	pushed, err := ops.PushGitHubChanges(ctx, state)
	if err != nil {
		return false, err
	}
	more, err := ops.RunGitHubBackfills(ctx, state)
	return more || pushed > 0, err
}

func EveryMinute(ctx context.Context, state *app.State) error {
	state.Limiter.Prune()
	if err := cleanupOAuth(ctx, state); err != nil {
		return err
	}
	if err := ops.RunCycleAutomations(ctx, state); err != nil {
		return err
	}
	if err := ops.RunRecurringTemplates(ctx, state); err != nil {
		return err
	}
	if err := Automations(ctx, state); err != nil {
		return err
	}
	return ops.SendEmails(ctx, state)
}

// Automations runs auto-archive, auto-close, the trash, snoozes, due dates and expired uploads.
func Automations(ctx context.Context, state *app.State) error {
	for _, statement := range []string{
		`WITH archived AS (
			UPDATE issues i SET archived_at = now(), auto_archived_at = now()
			FROM teams t, workflow_states s
			WHERE t.id = i.team_id AND s.id = i.state_id AND i.archived_at IS NULL AND i.trashed_at IS NULL
			AND t.auto_archive_months IS NOT NULL AND s.type IN ('completed', 'canceled')
			AND coalesce(i.completed_at, i.canceled_at, i.updated_at) < now() - make_interval(months => t.auto_archive_months)
			RETURNING i.id)
		 INSERT INTO issue_history (issue_id, action) SELECT id, 'auto_archived' FROM archived`,
		`WITH closed AS (
			UPDATE issues i SET state_id = t.auto_close_state_id, canceled_at = now(), auto_closed_at = now(), updated_at = now()
			FROM teams t, workflow_states s
			WHERE t.id = i.team_id AND s.id = i.state_id AND i.archived_at IS NULL AND i.trashed_at IS NULL
			AND t.auto_close_months IS NOT NULL AND t.auto_close_state_id IS NOT NULL
			AND s.type IN ('triage', 'backlog', 'unstarted')
			AND i.updated_at < now() - make_interval(months => t.auto_close_months)
			RETURNING i.id, s.id AS from_state, t.auto_close_state_id AS to_state)
		 INSERT INTO issue_history (issue_id, action, changes)
		 SELECT id, 'auto_closed', jsonb_build_object('state', jsonb_build_object('from', state_ref(from_state), 'to', state_ref(to_state)))
		 FROM closed`,
		"DELETE FROM issues WHERE trashed_at < now() - interval '30 days'",
		"DELETE FROM projects WHERE trashed_at < now() - interval '30 days'",
		"DELETE FROM initiatives WHERE trashed_at < now() - interval '30 days'",
		"DELETE FROM documents WHERE trashed_at < now() - interval '30 days'",
		"UPDATE notifications SET snoozed_until = NULL, read_at = NULL WHERE snoozed_until <= now()",
		"UPDATE issues SET snoozed_until = NULL, snoozed_by_id = NULL WHERE snoozed_until <= now()",
		`INSERT INTO notifications (workspace_id, user_id, type, issue_id, summary)
		 SELECT i.workspace_id, i.assignee_id, 'issueDue', i.id,
			issue_identifier(i.id) || CASE WHEN i.due_date < current_date THEN ' is overdue' ELSE ' is due today' END
		 FROM issues i JOIN workflow_states s ON s.id = i.state_id
		 WHERE i.assignee_id IS NOT NULL AND i.due_date <= current_date AND i.trashed_at IS NULL AND i.archived_at IS NULL
		 AND s.type NOT IN ('completed', 'canceled')
		 AND NOT EXISTS (SELECT 1 FROM notifications n WHERE n.issue_id = i.id AND n.user_id = i.assignee_id
			AND n.type = 'issueDue' AND n.created_at > now() - interval '1 day')`,
		"DELETE FROM files WHERE data IS NULL AND upload_expires_at < now()",
		"DELETE FROM webhook_deliveries WHERE created_at < now() - interval '14 days'",
		"DELETE FROM emails WHERE created_at < now() - interval '30 days'",
	} {
		if err := app.Exec(ctx, state.DB, statement); err != nil {
			return err
		}
	}
	return nil
}
