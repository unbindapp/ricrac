//! Webhooks: workspace admins register URLs that receive a signed POST for every change to the
//! resource types they listen to. Deliveries are queued by `events::emit` inside the change's
//! transaction and sent here, retrying with backoff.

use std::sync::LazyLock;
use std::time::Duration;

use chrono::Utc;
use hmac::{Hmac, Mac};
use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Value, json};
use sha2::Sha256;
use sqlx::PgConnection;
use uuid::Uuid;

use super::events;
use super::resolve;
use super::util::{Page, nullable};
use super::{IdArgs, Op, op};
use crate::ctx::{Access, Ctx};
use crate::error::{ApiError, ApiResult};
use crate::keys::random_secret;
use crate::state::AppState;

pub const RESOURCE_TYPES: [&str; 21] = [
    "Issue",
    "Comment",
    "Reaction",
    "Attachment",
    "IssueLabel",
    "IssueRelation",
    "Project",
    "ProjectLabel",
    "ProjectMilestone",
    "ProjectUpdate",
    "Initiative",
    "InitiativeUpdate",
    "Cycle",
    "Document",
    "Team",
    "WorkflowState",
    "Customer",
    "CustomerNeed",
    "Release",
    "ReleasePipeline",
    "ReleaseNote",
];

/// Waits before the 2nd, 3rd, ... attempt. After the last one the delivery is given up.
const BACKOFF_MINUTES: [i64; 5] = [1, 5, 30, 120, 360];
const MAX_ATTEMPTS: i32 = BACKOFF_MINUTES.len() as i32 + 1;

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "list_webhooks",
            Access::Admin,
            &[("GET", "/webhooks")],
            "List the workspace's webhooks: URL, label, team, the resource types they receive and whether they are \
             enabled. Secrets are never shown. Admins only.",
            list,
        ),
        op(
            "get_webhook",
            Access::Admin,
            &[("GET", "/webhooks/{id}")],
            "Read a webhook with its last 20 deliveries (action, type, status code, error, attempts and when it was \
             delivered or given up). Admins only.",
            get,
        ),
        op(
            "save_webhook",
            Access::Admin,
            &[("POST", "/webhooks"), ("PATCH", "/webhooks/{id}")],
            "Create a webhook (omit id; url is required) or update one (pass id). Ricrac POSTs a JSON body \
             {action (create, update, remove), type (the resource type), data (the entity as the API shows it), \
             updatedFrom (previous values of changed fields, on update), url, actor, createdAt, workspaceId, webhookId, \
             webhookTimestamp (ms since epoch)} for every change to the resourceTypes it listens to, on one team or \
             all. Headers: Ricrac-Event (the type), Ricrac-Delivery (delivery id) and Ricrac-Signature: the hex \
             HMAC-SHA256 of the raw request body keyed with the webhook's secret. Receivers verify it by computing \
             the same HMAC over the exact bytes received, comparing in constant time, and rejecting bodies whose \
             webhookTimestamp is more than a minute old. The secret (rr_whs_...) is returned once, on create; \
             rotate_webhook_secret makes a new one. Answer with a 2xx within 10 seconds; anything else is retried \
             after 1m, 5m, 30m, 2h and 6h, then given up. Redirects aren't followed. Admins only.",
            save,
        ),
        op(
            "rotate_webhook_secret",
            Access::Admin,
            &[("POST", "/webhooks/{id}/secret")],
            "Replace a webhook's signing secret and return the new one (shown only now). Deliveries from now on are \
             signed with it. Admins only.",
            rotate_secret,
        ),
        op(
            "delete_webhook",
            Access::Admin,
            &[("DELETE", "/webhooks/{id}")],
            "Delete a webhook and its queued deliveries. Admins only.",
            delete,
        ),
        op(
            "test_webhook",
            Access::Admin,
            &[("POST", "/webhooks/{id}/test")],
            "Send a ping ({action: \"test\", type: \"Ping\", data: {webhookId}}) to a webhook right away and report \
             what the receiver answered. A failed ping isn't retried. Admins only.",
            test,
        ),
        op(
            "list_webhook_deliveries",
            Access::Admin,
            &[("GET", "/webhooks/{id}/deliveries")],
            "A webhook's deliveries, newest first: action, type, status code, error, attempts, the next attempt and \
             when it was delivered or given up. Deliveries are kept for 14 days. Admins only.",
            list_deliveries,
        ),
    ]
}

/* ------------------------------------------------------------------ */
/*                           Representation                           */
/* ------------------------------------------------------------------ */

const WEBHOOK_JSON: &str = "jsonb_build_object(
    'id', h.id,
    'url', h.url,
    'label', h.label,
    'team', team_ref(h.team_id),
    'resourceTypes', to_jsonb(h.resource_types),
    'enabled', h.enabled,
    'creator', user_ref(h.creator_id),
    'createdAt', h.created_at,
    'updatedAt', h.updated_at
)";

const DELIVERY_JSON: &str = "jsonb_build_object(
    'id', d.id,
    'action', d.payload->>'action',
    'type', d.payload->>'type',
    'statusCode', d.status_code,
    'error', d.error,
    'attempts', d.attempts,
    'nextAttemptAt', CASE WHEN d.delivered_at IS NULL AND d.failed_at IS NULL THEN d.next_attempt_at END,
    'deliveredAt', d.delivered_at,
    'failedAt', d.failed_at,
    'createdAt', d.created_at
)";

async fn webhook_value(db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    let value: Option<Value> = sqlx::query_scalar(&format!("SELECT {WEBHOOK_JSON} FROM webhooks h WHERE h.id = $1"))
        .bind(id)
        .fetch_optional(&mut *db)
        .await?;
    value.ok_or_else(|| ApiError::not_found("No such webhook"))
}

async fn webhook_id(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    resolve::by_id(ctx, "webhooks", "webhook", value).await
}

/* ------------------------------------------------------------------ */
/*                                Read                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct ListArgs {
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list(ctx: Ctx, args: ListArgs) -> ApiResult<Value> {
    let page = Page { limit: args.limit, cursor: args.cursor };
    let nodes: Vec<Value> = sqlx::query_scalar(&format!(
        "SELECT {WEBHOOK_JSON} FROM webhooks h WHERE h.workspace_id = $1 ORDER BY h.created_at, h.id LIMIT $2 OFFSET $3"
    ))
    .bind(ctx.workspace)
    .bind(page.limit() + 1)
    .bind(page.offset()?)
    .fetch_all(ctx.db())
    .await?;
    page.wrap(nodes)
}

async fn get(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = webhook_id(&ctx, &args.id).await?;
    let mut db = ctx.db().acquire().await?;
    let mut value = webhook_value(&mut db, id).await?;
    value["deliveries"] = sqlx::query_scalar(&format!(
        "SELECT coalesce(jsonb_agg(x.v ORDER BY x.created_at DESC), '[]') FROM (
            SELECT {DELIVERY_JSON} AS v, d.created_at FROM webhook_deliveries d WHERE d.webhook_id = $1
            ORDER BY d.created_at DESC LIMIT 20) x"
    ))
    .bind(id)
    .fetch_one(&mut *db)
    .await?;
    Ok(value)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct ListDeliveriesArgs {
    /// The webhook's id.
    id: String,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list_deliveries(ctx: Ctx, args: ListDeliveriesArgs) -> ApiResult<Value> {
    let id = webhook_id(&ctx, &args.id).await?;
    let page = Page { limit: args.limit, cursor: args.cursor };
    let nodes: Vec<Value> = sqlx::query_scalar(&format!(
        "SELECT {DELIVERY_JSON} FROM webhook_deliveries d WHERE d.webhook_id = $1
         ORDER BY d.created_at DESC, d.id LIMIT $2 OFFSET $3"
    ))
    .bind(id)
    .bind(page.limit() + 1)
    .bind(page.offset()?)
    .fetch_all(ctx.db())
    .await?;
    page.wrap(nodes)
}

/* ------------------------------------------------------------------ */
/*                                Save                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct SaveArgs {
    /// Only to update a webhook: its id. Omit to create one.
    id: Option<String>,
    /// Where to POST: https, or http only for localhost / 127.0.0.1. Required to create.
    url: Option<String>,
    /// A name to recognize it by; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    label: Option<Option<String>>,
    /// What to receive: Issue, Comment, Reaction, Attachment, IssueLabel, IssueRelation, Project, ProjectLabel,
    /// ProjectMilestone, ProjectUpdate, Initiative, InitiativeUpdate, Cycle, Document, Team, WorkflowState,
    /// Customer, CustomerNeed, Release, ReleasePipeline, ReleaseNote. Issue and Comment by default.
    resource_types: Option<Vec<String>>,
    /// Only changes in this team (key, name or id); null for every team.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    team: Option<Option<String>>,
    /// Disabled webhooks receive nothing; queued deliveries are given up.
    enabled: Option<bool>,
}

fn check_url(value: &str) -> ApiResult<String> {
    let value = value.trim();
    let invalid = || {
        ApiError::bad_request(format!(
            "\"{value}\" isn't a valid webhook URL; use https:// (http:// only for localhost or 127.0.0.1)"
        ))
    };
    let url = reqwest::Url::parse(value).map_err(|_| invalid())?;
    let local = matches!(url.host_str(), Some("localhost" | "127.0.0.1"));
    match url.scheme() {
        "https" if url.host_str().is_some() => Ok(value.to_string()),
        "http" if local => Ok(value.to_string()),
        _ => Err(invalid()),
    }
}

fn resource_types(values: &[String]) -> ApiResult<Vec<String>> {
    if values.is_empty() {
        return Err(ApiError::bad_request("Pass at least one resource type"));
    }
    let mut types = Vec::new();
    for value in values {
        let Some(found) = RESOURCE_TYPES.iter().find(|t| t.eq_ignore_ascii_case(value.trim())) else {
            return Err(ApiError::bad_request(format!(
                "\"{value}\" isn't a resource type. Options: {}.",
                RESOURCE_TYPES.join(", ")
            )));
        };
        if !types.iter().any(|t| t == found) {
            types.push(found.to_string());
        }
    }
    Ok(types)
}

async fn save(ctx: Ctx, args: SaveArgs) -> ApiResult<Value> {
    let team = match &args.team {
        Some(Some(team)) => Some(Some(resolve::team(&ctx, team).await?)),
        Some(None) => Some(None),
        None => None,
    };
    let url = args.url.as_deref().map(check_url).transpose()?;
    let types = args.resource_types.as_deref().map(resource_types).transpose()?;
    let label = args.label.map(|l| l.map(|l| l.trim().to_string()).filter(|l| !l.is_empty()));
    let mut tx = ctx.db().begin().await?;
    let Some(reference) = args.id else {
        let url = url.ok_or_else(|| ApiError::bad_request("url is required to create a webhook"))?;
        let secret = random_secret("rr_whs_");
        let types = types.unwrap_or_else(|| vec!["Issue".into(), "Comment".into()]);
        let id: Uuid = sqlx::query_scalar(
            "INSERT INTO webhooks (workspace_id, team_id, url, label, secret, resource_types, enabled, creator_id)
             VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id",
        )
        .bind(ctx.workspace)
        .bind(team.flatten())
        .bind(&url)
        .bind(label.flatten())
        .bind(&secret)
        .bind(&types)
        .bind(args.enabled.unwrap_or(true))
        .bind(ctx.user)
        .fetch_one(&mut *tx)
        .await?;
        events::audit(&mut tx, &ctx, "webhook.created", json!({ "webhookId": id, "url": url })).await?;
        let mut value = webhook_value(&mut tx, id).await?;
        tx.commit().await?;
        value["secret"] = json!(secret);
        return Ok(value);
    };
    let id = webhook_id(&ctx, &reference).await?;
    sqlx::query(
        "UPDATE webhooks SET
            url = coalesce($2, url),
            label = CASE WHEN $3 THEN $4 ELSE label END,
            resource_types = coalesce($5, resource_types),
            team_id = CASE WHEN $6 THEN $7 ELSE team_id END,
            enabled = coalesce($8, enabled),
            updated_at = now()
         WHERE id = $1",
    )
    .bind(id)
    .bind(&url)
    .bind(label.is_some())
    .bind(label.flatten())
    .bind(&types)
    .bind(team.is_some())
    .bind(team.flatten())
    .bind(args.enabled)
    .execute(&mut *tx)
    .await?;
    events::audit(&mut tx, &ctx, "webhook.updated", json!({ "webhookId": id })).await?;
    let value = webhook_value(&mut tx, id).await?;
    tx.commit().await?;
    Ok(value)
}

async fn rotate_secret(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = webhook_id(&ctx, &args.id).await?;
    let secret = random_secret("rr_whs_");
    let mut tx = ctx.db().begin().await?;
    sqlx::query("UPDATE webhooks SET secret = $2, updated_at = now() WHERE id = $1")
        .bind(id)
        .bind(&secret)
        .execute(&mut *tx)
        .await?;
    events::audit(&mut tx, &ctx, "webhook.secretRotated", json!({ "webhookId": id })).await?;
    tx.commit().await?;
    Ok(json!({ "id": id, "secret": secret }))
}

async fn delete(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = webhook_id(&ctx, &args.id).await?;
    let mut tx = ctx.db().begin().await?;
    sqlx::query("DELETE FROM webhooks WHERE id = $1").bind(id).execute(&mut *tx).await?;
    events::audit(&mut tx, &ctx, "webhook.deleted", json!({ "webhookId": id })).await?;
    tx.commit().await?;
    Ok(json!({ "id": id, "deleted": true }))
}

async fn test(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = webhook_id(&ctx, &args.id).await?;
    let payload = json!({
        "action": "test",
        "type": "Ping",
        "data": { "webhookId": id },
        "url": null,
        "actor": { "id": ctx.user },
        "createdAt": Utc::now(),
        "workspaceId": ctx.workspace,
        "webhookId": id,
    });
    let delivery: Uuid =
        sqlx::query_scalar("INSERT INTO webhook_deliveries (webhook_id, payload, next_attempt_at) VALUES ($1, $2, 'infinity') RETURNING id")
            .bind(id)
            .bind(&payload)
            .fetch_one(ctx.db())
            .await?;
    let (url, secret): (String, String) =
        sqlx::query_as("SELECT url, secret FROM webhooks WHERE id = $1").bind(id).fetch_one(ctx.db()).await?;
    let due = Due { id: delivery, payload, attempts: 0, url, secret, enabled: true };
    let outcome = send(&due).await;
    record(&ctx.state, &due, &outcome, false).await?;
    Ok(json!({
        "deliveryId": delivery,
        "delivered": outcome.ok,
        "statusCode": outcome.status,
        "error": outcome.error,
    }))
}

/* ------------------------------------------------------------------ */
/*                              Delivery                              */
/* ------------------------------------------------------------------ */

static CLIENT: LazyLock<reqwest::Client> = LazyLock::new(|| {
    reqwest::Client::builder()
        .timeout(Duration::from_secs(10))
        .redirect(reqwest::redirect::Policy::none())
        .user_agent("Ricrac-Webhook/1.0")
        .build()
        .expect("webhook HTTP client")
});

#[derive(sqlx::FromRow)]
struct Due {
    id: Uuid,
    payload: Value,
    attempts: i32,
    url: String,
    secret: String,
    enabled: bool,
}

struct Outcome {
    ok: bool,
    status: Option<i32>,
    error: Option<String>,
}

/// The hex HMAC-SHA256 of `body` keyed with `secret`, sent as `Ricrac-Signature`.
pub fn signature(secret: &str, body: &[u8]) -> String {
    let mut mac = Hmac::<Sha256>::new_from_slice(secret.as_bytes()).expect("hmac accepts any key size");
    mac.update(body);
    hex::encode(mac.finalize().into_bytes())
}

fn truncate(text: &str) -> String {
    text.chars().take(500).collect()
}

async fn send(due: &Due) -> Outcome {
    let mut payload = due.payload.clone();
    payload["webhookTimestamp"] = json!(Utc::now().timestamp_millis());
    let body = payload.to_string().into_bytes();
    let event = payload["type"].as_str().unwrap_or_default().to_string();
    let response = CLIENT
        .post(&due.url)
        .header("Content-Type", "application/json")
        .header("Ricrac-Signature", signature(&due.secret, &body))
        .header("Ricrac-Event", event)
        .header("Ricrac-Delivery", due.id.to_string())
        .body(body)
        .send()
        .await;
    match response {
        Ok(response) if response.status().is_success() => {
            Outcome { ok: true, status: Some(response.status().as_u16() as i32), error: None }
        }
        Ok(response) => {
            let status = response.status();
            let text = response.text().await.unwrap_or_default();
            let error = match text.trim() {
                "" => format!("HTTP {status}"),
                text => format!("HTTP {status}: {text}"),
            };
            Outcome { ok: false, status: Some(status.as_u16() as i32), error: Some(truncate(&error)) }
        }
        Err(e) => Outcome { ok: false, status: None, error: Some(truncate(&e.to_string())) },
    }
}

/// Saves the result of an attempt: delivered, retried later with backoff, or given up.
async fn record(state: &AppState, due: &Due, outcome: &Outcome, retry: bool) -> ApiResult<()> {
    let attempts = due.attempts + 1;
    if outcome.ok {
        sqlx::query(
            "UPDATE webhook_deliveries SET delivered_at = now(), status_code = $2, error = NULL, attempts = $3 WHERE id = $1",
        )
        .bind(due.id)
        .bind(outcome.status)
        .bind(attempts)
        .execute(&state.db)
        .await?;
        return Ok(());
    }
    let wait = BACKOFF_MINUTES.get(attempts as usize - 1).copied().filter(|_| retry && attempts < MAX_ATTEMPTS);
    sqlx::query(
        "UPDATE webhook_deliveries SET attempts = $2, status_code = $3, error = $4,
            next_attempt_at = CASE WHEN $5::bigint IS NULL THEN next_attempt_at ELSE now() + make_interval(mins => $5::int) END,
            failed_at = CASE WHEN $5::bigint IS NULL THEN now() END
         WHERE id = $1",
    )
    .bind(due.id)
    .bind(attempts)
    .bind(outcome.status)
    .bind(&outcome.error)
    .bind(wait)
    .execute(&state.db)
    .await?;
    Ok(())
}

/// Sends queued webhook deliveries that are due, retrying failures with backoff. Returns how many
/// were attempted.
pub async fn deliver_due(state: &AppState) -> ApiResult<usize> {
    let mut tx = state.db.begin().await?;
    sqlx::query(
        "UPDATE webhook_deliveries d SET failed_at = now(), error = 'The webhook is disabled'
         FROM webhooks h WHERE h.id = d.webhook_id AND NOT h.enabled AND d.delivered_at IS NULL AND d.failed_at IS NULL",
    )
    .execute(&mut *tx)
    .await?;
    let due: Vec<Due> = sqlx::query_as(
        "WITH due AS (
            SELECT id FROM webhook_deliveries
            WHERE delivered_at IS NULL AND failed_at IS NULL AND next_attempt_at <= now()
            ORDER BY next_attempt_at LIMIT 20 FOR UPDATE SKIP LOCKED)
         UPDATE webhook_deliveries d SET next_attempt_at = now() + interval '5 minutes'
         FROM due, webhooks h WHERE d.id = due.id AND h.id = d.webhook_id
         RETURNING d.id, d.payload, d.attempts, h.url, h.secret, h.enabled",
    )
    .fetch_all(&mut *tx)
    .await?;
    tx.commit().await?;
    let due: Vec<Due> = due.into_iter().filter(|d| d.enabled).collect();
    let results = futures::future::join_all(due.iter().map(|due| async move {
        let outcome = send(due).await;
        record(state, due, &outcome, true).await
    }))
    .await;
    results.into_iter().collect::<ApiResult<Vec<()>>>()?;
    Ok(due.len())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn urls_must_be_https_except_on_this_machine() {
        assert!(check_url("https://hooks.example.com/ricrac").is_ok());
        assert!(check_url("http://localhost:8080/hook").is_ok());
        assert!(check_url("http://127.0.0.1:9/x").is_ok());
        assert!(check_url("http://hooks.example.com").is_err());
        assert!(check_url("ftp://localhost").is_err());
        assert!(check_url("not a url").is_err());
    }

    #[test]
    fn resource_types_are_canonical() {
        assert_eq!(
            resource_types(&["issue".into(), "CUSTOMERNEED".into(), "Issue".into()]).unwrap(),
            ["Issue", "CustomerNeed"]
        );
        assert!(resource_types(&["Issues".into()]).unwrap_err().message.contains("ReleaseNote"));
    }
}
