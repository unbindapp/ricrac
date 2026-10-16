//! The inbox: notifications about what happened to the things a person is involved in, which
//! they read, archive or snooze.

use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Value, json};
use uuid::Uuid;

use super::resolve::{self, as_uuid};
use super::util::{Page, parse_datetime};
use super::{Op, op};
use crate::ctx::{Access, Ctx};
use crate::error::{ApiError, ApiResult};

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "get_notifications",
            Access::Read,
            &[("GET", "/notifications")],
            "Your inbox, newest first: assignments, mentions, comments, replies, status changes and more, with how many \
             are unread. Archived notifications and snoozed ones (until they wake) are left out unless asked for.",
            get,
        ),
        op(
            "mark_notification",
            Access::Write,
            &[("POST", "/notifications/mark")],
            "Mark one or more of your notifications read or unread, archive or unarchive them, or snooze them until a \
             time (it comes back unread then) or unsnooze them.",
            mark,
        ),
        op(
            "mark_all_notifications_read",
            Access::Write,
            &[("POST", "/notifications/read-all")],
            "Mark all your unread notifications read, optionally only those before a time or about one issue.",
            mark_all_read,
        ),
    ]
}

const NOTIFICATION_JSON: &str = "jsonb_build_object(
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
)";

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct GetNotificationsArgs {
    /// Only unread ones.
    #[serde(default)]
    unread_only: bool,
    /// Also archived ones.
    #[serde(default)]
    include_archived: bool,
    /// Also ones snoozed until later.
    #[serde(default)]
    include_snoozed: bool,
    /// Only this type, e.g. issueAssignedToYou, issueMention, issueNewComment, issueCommentReply, issueStatusChanged.
    #[serde(rename = "type")]
    kind: Option<String>,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn get(ctx: Ctx, args: GetNotificationsArgs) -> ApiResult<Value> {
    let page = Page { limit: args.limit, cursor: args.cursor };
    let nodes: Vec<Value> = sqlx::query_scalar(&format!(
        "SELECT {NOTIFICATION_JSON} FROM notifications n WHERE n.user_id = $1 AND n.workspace_id = $2
         AND (NOT $3 OR n.read_at IS NULL)
         AND ($4 OR n.archived_at IS NULL)
         AND ($5 OR n.snoozed_until IS NULL OR n.snoozed_until <= now())
         AND ($6::text IS NULL OR n.type = $6)
         ORDER BY n.created_at DESC, n.id LIMIT $7 OFFSET $8"
    ))
    .bind(ctx.user)
    .bind(ctx.workspace)
    .bind(args.unread_only)
    .bind(args.include_archived)
    .bind(args.include_snoozed)
    .bind(args.kind.as_deref().map(str::trim).filter(|k| !k.is_empty()))
    .bind(page.limit() + 1)
    .bind(page.offset()?)
    .fetch_all(ctx.db())
    .await?;
    let unread: i64 = sqlx::query_scalar(
        "SELECT count(*) FROM notifications WHERE user_id = $1 AND workspace_id = $2 AND read_at IS NULL
         AND archived_at IS NULL AND (snoozed_until IS NULL OR snoozed_until <= now())",
    )
    .bind(ctx.user)
    .bind(ctx.workspace)
    .fetch_one(ctx.db())
    .await?;
    let mut value = page.wrap(nodes)?;
    value["unreadCount"] = json!(unread);
    Ok(value)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct MarkArgs {
    /// One notification's id.
    id: Option<String>,
    /// Several notifications' ids.
    ids: Option<Vec<String>>,
    /// read, unread, archive, unarchive, snooze or unsnooze.
    action: String,
    /// For snooze: until when, as ISO 8601 or a duration from now like PT3H or P1D.
    until: Option<String>,
}

async fn mark(ctx: Ctx, args: MarkArgs) -> ApiResult<Value> {
    let mut ids = Vec::new();
    for value in args.id.iter().chain(args.ids.iter().flatten()) {
        let id = as_uuid(value).ok_or_else(|| ApiError::not_found(format!("No notification \"{value}\"")))?;
        if !ids.contains(&id) {
            ids.push(id);
        }
    }
    if ids.is_empty() || ids.len() > 250 {
        return Err(ApiError::bad_request("Pass id or ids (at most 250)"));
    }
    let until = match (args.action.as_str(), &args.until) {
        ("snooze", Some(until)) => {
            let until = parse_datetime(until)?;
            if until <= chrono::Utc::now() {
                return Err(ApiError::bad_request("until must be in the future"));
            }
            Some(until)
        }
        ("snooze", None) => return Err(ApiError::bad_request("Pass until to snooze, e.g. PT3H, P1D or an ISO time")),
        (_, Some(_)) => return Err(ApiError::bad_request("until is only for snooze")),
        _ => None,
    };
    let set = match args.action.as_str() {
        "read" => "read_at = coalesce(read_at, now())",
        "unread" => "read_at = NULL",
        "archive" => "archived_at = coalesce(archived_at, now())",
        "unarchive" => "archived_at = NULL",
        "snooze" => "snoozed_until = $4",
        "unsnooze" => "snoozed_until = NULL",
        other => {
            return Err(ApiError::bad_request(format!(
                "action \"{other}\" isn't one of read, unread, archive, unarchive, snooze, unsnooze"
            )));
        }
    };
    let mut tx = ctx.db().begin().await?;
    let found: i64 = sqlx::query_scalar(
        "SELECT count(*) FROM notifications WHERE id = ANY($1) AND user_id = $2 AND workspace_id = $3",
    )
    .bind(&ids)
    .bind(ctx.user)
    .bind(ctx.workspace)
    .fetch_one(&mut *tx)
    .await?;
    if found as usize != ids.len() {
        return Err(ApiError::not_found("No such notification in your inbox"));
    }
    let sql = format!("UPDATE notifications SET {set} WHERE id = ANY($1) AND user_id = $2 AND workspace_id = $3");
    let mut update = sqlx::query(&sql).bind(&ids).bind(ctx.user).bind(ctx.workspace);
    if let Some(until) = until {
        update = update.bind(until);
    }
    update.execute(&mut *tx).await?;
    let nodes: Vec<Value> = sqlx::query_scalar(&format!(
        "SELECT {NOTIFICATION_JSON} FROM notifications n WHERE n.id = ANY($1) ORDER BY n.created_at DESC, n.id"
    ))
    .bind(&ids)
    .fetch_all(&mut *tx)
    .await?;
    tx.commit().await?;
    Ok(json!({ "nodes": nodes }))
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct MarkAllArgs {
    /// Only notifications created before this time (ISO 8601 or a duration like -P1D).
    before: Option<String>,
    /// Only notifications about this issue (identifier, id or link).
    issue: Option<String>,
}

async fn mark_all_read(ctx: Ctx, args: MarkAllArgs) -> ApiResult<Value> {
    let before = args.before.as_deref().map(parse_datetime).transpose()?;
    let issue: Option<Uuid> = match &args.issue {
        Some(issue) => Some(resolve::issue(&ctx, issue).await?),
        None => None,
    };
    let marked = sqlx::query(
        "UPDATE notifications SET read_at = now() WHERE user_id = $1 AND workspace_id = $2 AND read_at IS NULL
         AND ($3::timestamptz IS NULL OR created_at < $3) AND ($4::uuid IS NULL OR issue_id = $4)",
    )
    .bind(ctx.user)
    .bind(ctx.workspace)
    .bind(before)
    .bind(issue)
    .execute(ctx.db())
    .await?
    .rows_affected();
    Ok(json!({ "marked": marked }))
}
