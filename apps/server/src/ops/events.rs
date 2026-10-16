//! What happens around a change: issue history, inbox notifications, webhook deliveries and the
//! audit log. Everything is written in the change's transaction, so it only exists if the change
//! does.

use serde_json::{Value, json};
use sqlx::PgConnection;
use uuid::Uuid;

use crate::ctx::Ctx;
use crate::error::ApiResult;

/// Queues a webhook delivery for every enabled webhook of the workspace that listens to `kind`
/// (`Issue`, `Comment`, `Project`, ...) on this team, or on every team.
pub async fn emit(
    tx: &mut PgConnection,
    ctx: &Ctx,
    kind: &str,
    action: &str,
    data: &Value,
    updated_from: Option<Value>,
    team: Option<Uuid>,
) -> ApiResult<()> {
    let hooks: Vec<Uuid> = sqlx::query_scalar(
        "SELECT id FROM webhooks WHERE workspace_id = $1 AND enabled AND $2 = ANY(resource_types)
         AND (team_id IS NULL OR team_id = $3)",
    )
    .bind(ctx.workspace)
    .bind(kind)
    .bind(team)
    .fetch_all(&mut *tx)
    .await?;
    if hooks.is_empty() {
        return Ok(());
    }
    let url = data.get("url").cloned().unwrap_or(Value::Null);
    for hook in hooks {
        let mut payload = json!({
            "action": action,
            "type": kind,
            "data": data,
            "url": url,
            "actor": { "id": ctx.user },
            "createdAt": chrono::Utc::now(),
            "workspaceId": ctx.workspace,
            "webhookId": hook,
        });
        if let Some(from) = &updated_from {
            payload["updatedFrom"] = from.clone();
        }
        sqlx::query("INSERT INTO webhook_deliveries (webhook_id, payload) VALUES ($1, $2)")
            .bind(hook)
            .bind(payload)
            .execute(&mut *tx)
            .await?;
    }
    ctx.state.deliveries.notify_one();
    Ok(())
}

/// Adds an entry to an issue's history. `changes` maps fields to `{from, to}`.
pub async fn history(tx: &mut PgConnection, ctx: &Ctx, issue: Uuid, action: &str, changes: Value) -> ApiResult<()> {
    sqlx::query("INSERT INTO issue_history (issue_id, actor_id, action, changes) VALUES ($1, $2, $3, $4)")
        .bind(issue)
        .bind(ctx.user)
        .bind(action)
        .bind(changes)
        .execute(&mut *tx)
        .await?;
    Ok(())
}

pub async fn audit(tx: &mut PgConnection, ctx: &Ctx, kind: &str, metadata: Value) -> ApiResult<()> {
    sqlx::query("INSERT INTO audit_entries (workspace_id, actor_id, type, metadata) VALUES ($1, $2, $3, $4)")
        .bind(ctx.workspace)
        .bind(ctx.user)
        .bind(kind)
        .bind(metadata)
        .execute(&mut *tx)
        .await?;
    Ok(())
}

/// What an inbox notification is about. Unset targets stay empty.
#[derive(Default, Clone)]
pub struct Notice {
    /// issueAssigned, issueMention, issueComment, issueCommentReply, issueStatusChanged,
    /// issueNewComment, issuePriorityChanged, issueBlocking, issueUnblocked, issueDue, issueCreated
    /// (triage), issueSubscribed, projectUpdateCreated, projectUpdateMention, documentMention,
    /// projectMention, initiativeUpdateCreated, ...
    pub kind: &'static str,
    pub summary: String,
    pub issue: Option<Uuid>,
    pub comment: Option<Uuid>,
    pub project: Option<Uuid>,
    pub initiative: Option<Uuid>,
    pub document: Option<Uuid>,
    pub status_update: Option<Uuid>,
}

/// Notifies each of `users` once, except the caller, who doesn't need to hear about their own
/// change, and anyone no longer in the workspace.
pub async fn notify(tx: &mut PgConnection, ctx: &Ctx, users: &[Uuid], notice: &Notice) -> ApiResult<()> {
    let mut seen = Vec::new();
    for &user in users {
        if user == ctx.user || seen.contains(&user) {
            continue;
        }
        seen.push(user);
        sqlx::query(
            "INSERT INTO notifications (workspace_id, user_id, actor_id, type, issue_id, comment_id, project_id,
                initiative_id, document_id, status_update_id, summary)
             SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
             WHERE EXISTS (SELECT 1 FROM workspace_members WHERE workspace_id = $1 AND user_id = $2 AND active)",
        )
        .bind(ctx.workspace)
        .bind(user)
        .bind(ctx.user)
        .bind(notice.kind)
        .bind(notice.issue)
        .bind(notice.comment)
        .bind(notice.project)
        .bind(notice.initiative)
        .bind(notice.document)
        .bind(notice.status_update)
        .bind(&notice.summary)
        .execute(&mut *tx)
        .await?;
    }
    Ok(())
}

pub async fn issue_subscribers(tx: &mut PgConnection, issue: Uuid) -> ApiResult<Vec<Uuid>> {
    Ok(sqlx::query_scalar("SELECT user_id FROM issue_subscribers WHERE issue_id = $1")
        .bind(issue)
        .fetch_all(&mut *tx)
        .await?)
}

pub async fn subscribe(tx: &mut PgConnection, issue: Uuid, users: &[Uuid]) -> ApiResult<()> {
    sqlx::query(
        "INSERT INTO issue_subscribers (issue_id, user_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING",
    )
    .bind(issue)
    .bind(users)
    .execute(&mut *tx)
    .await?;
    Ok(())
}

/// Members mentioned in Markdown as `@displayName`, `@email` or `@` + the part of their email
/// before the `@`.
pub async fn mentions(ctx: &Ctx, text: &str) -> ApiResult<Vec<Uuid>> {
    let handles = mention_handles(text);
    if handles.is_empty() {
        return Ok(vec![]);
    }
    let ids: Vec<Uuid> = sqlx::query_scalar(
        "SELECT u.id FROM users u JOIN workspace_members m ON m.user_id = u.id AND m.workspace_id = $1 AND m.active
         WHERE lower(u.display_name) = ANY($2) OR lower(u.email) = ANY($2) OR lower(split_part(u.email, '@', 1)) = ANY($2)",
    )
    .bind(ctx.workspace)
    .bind(&handles)
    .fetch_all(ctx.db())
    .await?;
    Ok(ids)
}

fn mention_handles(text: &str) -> Vec<String> {
    let mut handles = Vec::new();
    let chars: Vec<char> = text.chars().collect();
    for (i, c) in chars.iter().enumerate() {
        let starts_word = i == 0 || !(chars[i - 1].is_alphanumeric() || chars[i - 1] == '@');
        if *c != '@' || !starts_word {
            continue;
        }
        let handle: String = chars[i + 1..]
            .iter()
            .take_while(|c| c.is_alphanumeric() || matches!(c, '.' | '_' | '-' | '@' | '+'))
            .collect();
        let handle = handle.trim_end_matches(['.', '-']).to_lowercase();
        if !handle.is_empty() && !handles.contains(&handle) {
            handles.push(handle);
        }
    }
    handles
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn finds_mentions() {
        assert_eq!(mention_handles("Hey @ann, can @bob.smith look? cc @Carl."), vec!["ann", "bob.smith", "carl"]);
        assert_eq!(mention_handles("mail me at x@y.com"), Vec::<String>::new());
        assert_eq!(mention_handles("@ann@corp.com please"), vec!["ann@corp.com"]);
    }
}
