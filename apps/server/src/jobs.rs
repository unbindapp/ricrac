//! Work that runs on its own: workspace automations every minute and webhook deliveries as soon
//! as they are queued.

use std::time::Duration;

use crate::error::ApiResult;
use crate::ops;
use crate::state::AppState;

pub fn spawn(state: AppState) {
    let minutely = state.clone();
    tokio::spawn(async move {
        let mut interval = tokio::time::interval(Duration::from_secs(60));
        loop {
            interval.tick().await;
            if let Err(e) = every_minute(&minutely).await {
                tracing::warn!("background job failed: {}", e.message);
            }
        }
    });
    tokio::spawn(async move {
        loop {
            match ops::webhooks::deliver_due(&state).await {
                Ok(n) if n > 0 => continue,
                Ok(_) => {}
                Err(e) => tracing::warn!("webhook delivery failed: {}", e.message),
            }
            let _ = tokio::time::timeout(Duration::from_secs(10), state.deliveries.notified()).await;
            tokio::time::sleep(Duration::from_millis(200)).await;
        }
    });
}

pub async fn every_minute(state: &AppState) -> ApiResult<()> {
    state.limiter.prune();
    crate::oauth::cleanup(state).await?;
    ops::cycles::run_automations(state).await?;
    ops::templates::run_recurring(state).await?;
    automations(state).await
}

/// Auto-archive, auto-close, the trash, snoozes, due dates and expired uploads.
pub async fn automations(state: &AppState) -> ApiResult<()> {
    let db = &state.db;
    sqlx::query(
        "WITH archived AS (
            UPDATE issues i SET archived_at = now(), auto_archived_at = now()
            FROM teams t, workflow_states s
            WHERE t.id = i.team_id AND s.id = i.state_id AND i.archived_at IS NULL AND i.trashed_at IS NULL
            AND t.auto_archive_months IS NOT NULL AND s.type IN ('completed', 'canceled')
            AND coalesce(i.completed_at, i.canceled_at, i.updated_at) < now() - make_interval(months => t.auto_archive_months)
            RETURNING i.id)
         INSERT INTO issue_history (issue_id, action) SELECT id, 'auto_archived' FROM archived",
    )
    .execute(db)
    .await?;
    sqlx::query(
        "WITH closed AS (
            UPDATE issues i SET state_id = t.auto_close_state_id, canceled_at = now(), auto_closed_at = now(), updated_at = now()
            FROM teams t, workflow_states s
            WHERE t.id = i.team_id AND s.id = i.state_id AND i.archived_at IS NULL AND i.trashed_at IS NULL
            AND t.auto_close_months IS NOT NULL AND t.auto_close_state_id IS NOT NULL
            AND s.type IN ('triage', 'backlog', 'unstarted')
            AND i.updated_at < now() - make_interval(months => t.auto_close_months)
            RETURNING i.id, s.id AS from_state, t.auto_close_state_id AS to_state)
         INSERT INTO issue_history (issue_id, action, changes)
         SELECT id, 'auto_closed', jsonb_build_object('state', jsonb_build_object('from', state_ref(from_state), 'to', state_ref(to_state)))
         FROM closed",
    )
    .execute(db)
    .await?;
    for table in ["issues", "projects", "initiatives", "documents"] {
        sqlx::query(&format!("DELETE FROM {table} WHERE trashed_at < now() - interval '30 days'")).execute(db).await?;
    }
    sqlx::query("UPDATE notifications SET snoozed_until = NULL, read_at = NULL WHERE snoozed_until <= now()")
        .execute(db)
        .await?;
    sqlx::query("UPDATE issues SET snoozed_until = NULL, snoozed_by_id = NULL WHERE snoozed_until <= now()")
        .execute(db)
        .await?;
    sqlx::query(
        "INSERT INTO notifications (workspace_id, user_id, type, issue_id, summary)
         SELECT i.workspace_id, i.assignee_id, 'issueDue', i.id,
            issue_identifier(i.id) || CASE WHEN i.due_date < current_date THEN ' is overdue' ELSE ' is due today' END
         FROM issues i JOIN workflow_states s ON s.id = i.state_id
         WHERE i.assignee_id IS NOT NULL AND i.due_date <= current_date AND i.trashed_at IS NULL AND i.archived_at IS NULL
         AND s.type NOT IN ('completed', 'canceled')
         AND NOT EXISTS (SELECT 1 FROM notifications n WHERE n.issue_id = i.id AND n.user_id = i.assignee_id
            AND n.type = 'issueDue' AND n.created_at > now() - interval '1 day')",
    )
    .execute(db)
    .await?;
    sqlx::query("DELETE FROM files WHERE data IS NULL AND upload_expires_at < now()").execute(db).await?;
    sqlx::query("DELETE FROM webhook_deliveries WHERE created_at < now() - interval '14 days'").execute(db).await?;
    Ok(())
}
