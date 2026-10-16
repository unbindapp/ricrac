//! The JSON API behind the dashboard (`/api`): sign-in, workspaces and invitations, API keys,
//! OAuth apps, connected apps and the OAuth consent screen. Writes need the `x-ricrac` header,
//! which cross-site forms can't send.

use axum::extract::{FromRequestParts, Path, Query, Request, State};
use axum::http::request::Parts;
use axum::http::{HeaderMap, Method};
use axum::middleware::Next;
use axum::response::{IntoResponse, Response};
use axum::routing::{delete, get, post};
use axum::{Json, Router};
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use serde_json::{Value, json};
use ts_rs::TS;
use uuid::Uuid;

use crate::ctx::{ALL_SCOPES, Ctx, Role};
use crate::error::{ApiError, ApiResult};
use crate::keys::{random_secret, secret_hash};
use crate::oauth::{self, API_KEY_PREFIX, OAuthClientInfo, OAuthRequest};
use crate::rest::session_workspace;
use crate::state::AppState;
use crate::{google, seed};

/// The signed-in user.
pub struct SignedIn(pub Uuid);

impl FromRequestParts<AppState> for SignedIn {
    type Rejection = ApiError;

    async fn from_request_parts(parts: &mut Parts, state: &AppState) -> Result<Self, Self::Rejection> {
        let user = state.keys.session_user(&parts.headers).ok_or_else(ApiError::unauthorized)?;
        let exists: bool = sqlx::query_scalar("SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)")
            .bind(user)
            .fetch_one(&state.db)
            .await?;
        if !exists {
            return Err(ApiError::unauthorized());
        }
        Ok(Self(user))
    }
}

/// The signed-in user in the workspace of `x-ricrac-workspace`.
pub struct Member(pub Ctx);

impl FromRequestParts<AppState> for Member {
    type Rejection = ApiError;

    async fn from_request_parts(parts: &mut Parts, state: &AppState) -> Result<Self, Self::Rejection> {
        let SignedIn(user) = SignedIn::from_request_parts(parts, state).await?;
        let workspace = session_workspace(state, user, &parts.headers).await?;
        let scopes = crate::ctx::Access::parse_scopes(&ALL_SCOPES.map(String::from));
        let ctx =
            Ctx::load(state, user, workspace, scopes).await?.ok_or_else(|| ApiError::forbidden("Not a member"))?;
        Ok(Self(ctx))
    }
}

async fn require_app_header(req: Request, next: Next) -> Response {
    let exempt = req.uri().path().starts_with("/api/auth/");
    if req.method() != Method::GET && !exempt && !req.headers().contains_key("x-ricrac") {
        return ApiError::forbidden("Missing x-ricrac header").into_response();
    }
    next.run(req).await
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/config", get(config))
        .route("/me", get(me).patch(update_me))
        .route("/auth/sign-in", get(google::sign_in))
        .route("/auth/callback/google", get(google::callback))
        .route("/auth/sign-out", post(google::sign_out))
        .route("/workspaces", post(create_workspace))
        .route("/invitations/{id}/accept", post(accept_invitation))
        .route("/invitations/{id}", delete(decline_invitation))
        .route("/api-keys", get(list_api_keys).post(create_api_key))
        .route("/api-keys/{id}", delete(delete_api_key))
        .route("/oauth-apps", get(list_oauth_apps).post(create_oauth_app))
        .route("/oauth-apps/{id}", axum::routing::patch(update_oauth_app).delete(delete_oauth_app))
        .route("/oauth-apps/{id}/secret", post(rotate_oauth_secret))
        .route("/connected-apps", get(connected_apps))
        .route("/connected-apps/{id}", delete(revoke_connected_app))
        .route("/oauth/client", get(oauth_client))
        .route("/oauth/decide", post(oauth_decide))
        .fallback(|| async { ApiError::not_found("Not found") })
        .layer(axum::middleware::from_fn(require_app_header))
}

#[derive(Serialize, TS)]
#[ts(export)]
#[serde(rename_all = "camelCase")]
pub struct AppConfig {
    pub google_configured: bool,
    pub public_url: String,
    pub mcp_url: String,
}

async fn config(State(state): State<AppState>) -> Json<AppConfig> {
    Json(AppConfig {
        google_configured: state.config.google().is_some(),
        public_url: state.config.public_url.clone(),
        mcp_url: state.config.mcp_url(),
    })
}

#[derive(Serialize, TS, sqlx::FromRow)]
#[ts(export)]
#[serde(rename_all = "camelCase")]
pub struct WorkspaceSummary {
    pub id: Uuid,
    pub name: String,
    pub url_key: String,
    pub role: String,
}

#[derive(Serialize, TS, sqlx::FromRow)]
#[ts(export)]
#[serde(rename_all = "camelCase")]
pub struct PendingInvitation {
    pub id: Uuid,
    pub workspace_name: String,
    pub role: String,
    pub inviter: Option<String>,
}

#[derive(Serialize, TS)]
#[ts(export)]
#[serde(rename_all = "camelCase")]
pub struct Me {
    pub id: Uuid,
    pub email: String,
    pub name: String,
    pub display_name: String,
    pub avatar_url: Option<String>,
    pub workspaces: Vec<WorkspaceSummary>,
    pub invitations: Vec<PendingInvitation>,
}

/// The signed-in user, or null for visitors.
async fn me(State(state): State<AppState>, headers: HeaderMap) -> ApiResult<Json<Option<Me>>> {
    let Some(id) = state.keys.session_user(&headers) else { return Ok(Json(None)) };
    let user: Option<(String, String, String, Option<String>)> =
        sqlx::query_as("SELECT email, name, display_name, avatar_url FROM users WHERE id = $1")
            .bind(id)
            .fetch_optional(&state.db)
            .await?;
    let Some((email, name, display_name, avatar_url)) = user else { return Ok(Json(None)) };
    let workspaces = sqlx::query_as(
        "SELECT w.id, w.name, w.url_key, m.role FROM workspaces w JOIN workspace_members m ON m.workspace_id = w.id
         WHERE m.user_id = $1 AND m.active ORDER BY w.name",
    )
    .bind(id)
    .fetch_all(&state.db)
    .await?;
    let invitations = sqlx::query_as(
        "SELECT i.id, w.name AS workspace_name, i.role, u.name AS inviter FROM invitations i
         JOIN workspaces w ON w.id = i.workspace_id LEFT JOIN users u ON u.id = i.inviter_id
         WHERE i.email = $1 AND i.expires_at > now()
         AND NOT EXISTS (SELECT 1 FROM workspace_members m WHERE m.workspace_id = i.workspace_id AND m.user_id = $2 AND m.active)",
    )
    .bind(&email)
    .bind(id)
    .fetch_all(&state.db)
    .await?;
    Ok(Json(Some(Me { id, email, name, display_name, avatar_url, workspaces, invitations })))
}

#[derive(Deserialize, TS)]
#[ts(export)]
#[serde(rename_all = "camelCase")]
pub struct UpdateMe {
    #[ts(optional)]
    pub name: Option<String>,
    #[ts(optional)]
    pub display_name: Option<String>,
}

async fn update_me(
    State(state): State<AppState>,
    SignedIn(user): SignedIn,
    Json(body): Json<UpdateMe>,
) -> ApiResult<Json<Value>> {
    if let Some(name) = body.name.as_deref().map(str::trim) {
        crate::ensure!(!name.is_empty() && name.len() <= 100, "Name must be 1 to 100 characters");
        sqlx::query("UPDATE users SET name = $2 WHERE id = $1").bind(user).bind(name).execute(&state.db).await?;
    }
    if let Some(handle) = body.display_name.as_deref().map(str::trim) {
        crate::ensure!(
            !handle.is_empty()
                && handle.len() <= 40
                && handle.chars().all(|c| c.is_alphanumeric() || matches!(c, '.' | '_' | '-')),
            "Display names are 1 to 40 letters, digits, dots, dashes or underscores"
        );
        sqlx::query("UPDATE users SET display_name = $2 WHERE id = $1")
            .bind(user)
            .bind(handle)
            .execute(&state.db)
            .await?;
    }
    Ok(Json(json!({ "ok": true })))
}

#[derive(Deserialize, TS)]
#[ts(export)]
#[serde(rename_all = "camelCase")]
pub struct CreateWorkspace {
    pub name: String,
    /// Lowercase letters, digits and dashes, used in links.
    pub url_key: String,
    /// The first team; named after the workspace when left out.
    #[ts(optional)]
    pub team_name: Option<String>,
    #[ts(optional)]
    pub team_key: Option<String>,
}

pub fn check_url_key(key: &str) -> ApiResult<String> {
    let key = key.trim().to_lowercase();
    crate::ensure!(
        (2..=32).contains(&key.len())
            && key.bytes().all(|b| b.is_ascii_lowercase() || b.is_ascii_digit() || b == b'-')
            && !key.starts_with('-')
            && !key.ends_with('-'),
        "The URL key is 2 to 32 lowercase letters, digits and dashes"
    );
    const RESERVED: [&str; 12] =
        ["api", "v1", "mcp", "oauth", "assets", "files", "settings", "login", "admin", "docs", "llms.txt", "uploads"];
    crate::ensure!(!RESERVED.contains(&key.as_str()), "\"{key}\" is reserved");
    Ok(key)
}

pub fn check_team_key(key: &str) -> ApiResult<String> {
    let key = key.trim().to_uppercase();
    crate::ensure!(
        (1..=7).contains(&key.len())
            && key.bytes().all(|b| b.is_ascii_uppercase() || b.is_ascii_digit())
            && key.as_bytes()[0].is_ascii_uppercase(),
        "A team key is 1 to 7 letters and digits, starting with a letter, like ENG"
    );
    Ok(key)
}

async fn create_workspace(
    State(state): State<AppState>,
    SignedIn(user): SignedIn,
    Json(body): Json<CreateWorkspace>,
) -> ApiResult<Json<WorkspaceSummary>> {
    let name = body.name.trim().to_string();
    crate::ensure!(!name.is_empty() && name.len() <= 80, "The name is 1 to 80 characters");
    let url_key = check_url_key(&body.url_key)?;
    let team_name = body.team_name.as_deref().map(str::trim).filter(|n| !n.is_empty()).unwrap_or(&name).to_string();
    let team_key = check_team_key(&body.team_key.clone().unwrap_or_else(|| seed::team_key(&team_name)))?;
    let mut tx = state.db.begin().await?;
    let taken: bool = sqlx::query_scalar("SELECT EXISTS (SELECT 1 FROM workspaces WHERE url_key = $1)")
        .bind(&url_key)
        .fetch_one(&mut *tx)
        .await?;
    if taken {
        return Err(ApiError::conflict(format!("The URL key \"{url_key}\" is taken")));
    }
    let id: Uuid = sqlx::query_scalar("INSERT INTO workspaces (name, url_key) VALUES ($1, $2) RETURNING id")
        .bind(&name)
        .bind(&url_key)
        .fetch_one(&mut *tx)
        .await?;
    sqlx::query("INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')")
        .bind(id)
        .bind(user)
        .execute(&mut *tx)
        .await?;
    seed::workspace_defaults(&mut tx, id).await?;
    seed::create_team(&mut tx, id, user, &team_name, &team_key).await?;
    tx.commit().await?;
    Ok(Json(WorkspaceSummary { id, name, url_key, role: "owner".into() }))
}

async fn accept_invitation(
    State(state): State<AppState>,
    SignedIn(user): SignedIn,
    Path(id): Path<Uuid>,
) -> ApiResult<Json<WorkspaceSummary>> {
    let mut tx = state.db.begin().await?;
    let invitation: Option<(Uuid, String, Vec<Uuid>)> = sqlx::query_as(
        "DELETE FROM invitations i USING users u WHERE i.id = $1 AND u.id = $2 AND i.email = u.email AND i.expires_at > now()
         RETURNING i.workspace_id, i.role, i.team_ids",
    )
    .bind(id)
    .bind(user)
    .fetch_optional(&mut *tx)
    .await?;
    let Some((workspace, role, teams)) = invitation else {
        return Err(ApiError::not_found("No such invitation, or it expired"));
    };
    sqlx::query(
        "INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)
         ON CONFLICT (workspace_id, user_id) DO UPDATE SET active = true, role = excluded.role",
    )
    .bind(workspace)
    .bind(user)
    .bind(&role)
    .execute(&mut *tx)
    .await?;
    sqlx::query("INSERT INTO team_members (team_id, user_id) SELECT unnest($1::uuid[]), $2 ON CONFLICT DO NOTHING")
        .bind(&teams)
        .bind(user)
        .execute(&mut *tx)
        .await?;
    let summary = sqlx::query_as(
        "SELECT w.id, w.name, w.url_key, m.role FROM workspaces w JOIN workspace_members m ON m.workspace_id = w.id
         WHERE w.id = $1 AND m.user_id = $2",
    )
    .bind(workspace)
    .bind(user)
    .fetch_one(&mut *tx)
    .await?;
    tx.commit().await?;
    Ok(Json(summary))
}

async fn decline_invitation(
    State(state): State<AppState>,
    SignedIn(user): SignedIn,
    Path(id): Path<Uuid>,
) -> ApiResult<Json<Value>> {
    sqlx::query("DELETE FROM invitations i USING users u WHERE i.id = $1 AND u.id = $2 AND i.email = u.email")
        .bind(id)
        .bind(user)
        .execute(&state.db)
        .await?;
    Ok(Json(json!({ "ok": true })))
}

/* ------------------------------------------------------------------ */
/*                              API keys                              */
/* ------------------------------------------------------------------ */

#[derive(Serialize, TS, sqlx::FromRow)]
#[ts(export)]
#[serde(rename_all = "camelCase")]
pub struct ApiKey {
    pub id: Uuid,
    pub label: String,
    pub key_hint: String,
    pub scopes: Vec<String>,
    pub created_at: DateTime<Utc>,
    pub last_used_at: Option<DateTime<Utc>>,
}

async fn list_api_keys(State(state): State<AppState>, Member(ctx): Member) -> ApiResult<Json<Vec<ApiKey>>> {
    let keys = sqlx::query_as(
        "SELECT id, label, key_hint, scopes, created_at, last_used_at FROM api_keys
         WHERE user_id = $1 AND workspace_id = $2 ORDER BY created_at DESC",
    )
    .bind(ctx.user)
    .bind(ctx.workspace)
    .fetch_all(&state.db)
    .await?;
    Ok(Json(keys))
}

#[derive(Deserialize, TS)]
#[ts(export)]
pub struct CreateApiKey {
    pub label: String,
    /// read, write and/or admin; all of them when left out.
    #[ts(optional)]
    pub scopes: Option<Vec<String>>,
}

#[derive(Serialize, TS)]
#[ts(export)]
pub struct CreatedApiKey {
    pub key: ApiKey,
    /// Shown once.
    pub secret: String,
}

fn check_scopes(scopes: Option<Vec<String>>) -> ApiResult<Vec<String>> {
    let scopes = scopes.unwrap_or_else(|| ALL_SCOPES.map(String::from).to_vec());
    crate::ensure!(!scopes.is_empty(), "Pick at least one scope");
    for scope in &scopes {
        crate::ensure!(ALL_SCOPES.contains(&scope.as_str()), "Unknown scope \"{scope}\"");
    }
    let mut scopes: Vec<String> =
        ALL_SCOPES.iter().filter(|s| scopes.iter().any(|g| g == *s)).map(|s| s.to_string()).collect();
    if !scopes.iter().any(|s| s == "read") {
        scopes.insert(0, "read".into());
    }
    Ok(scopes)
}

async fn create_api_key(
    State(state): State<AppState>,
    Member(ctx): Member,
    Json(body): Json<CreateApiKey>,
) -> ApiResult<Json<CreatedApiKey>> {
    let label = body.label.trim().to_string();
    crate::ensure!(!label.is_empty() && label.len() <= 80, "The label is 1 to 80 characters");
    let scopes = check_scopes(body.scopes)?;
    if scopes.iter().any(|s| s == "admin") {
        crate::ensure!(ctx.role >= Role::Admin, "Only workspace admins can make keys with the admin scope");
    }
    let secret = random_secret(API_KEY_PREFIX);
    let hint = format!("{}…{}", &secret[..API_KEY_PREFIX.len() + 4], &secret[secret.len() - 4..]);
    let key = sqlx::query_as(
        "INSERT INTO api_keys (workspace_id, user_id, label, key_hash, key_hint, scopes) VALUES ($1, $2, $3, $4, $5, $6)
         RETURNING id, label, key_hint, scopes, created_at, last_used_at",
    )
    .bind(ctx.workspace)
    .bind(ctx.user)
    .bind(&label)
    .bind(secret_hash(&secret))
    .bind(&hint)
    .bind(&scopes)
    .fetch_one(&state.db)
    .await?;
    Ok(Json(CreatedApiKey { key, secret }))
}

async fn delete_api_key(
    State(state): State<AppState>,
    SignedIn(user): SignedIn,
    Path(id): Path<Uuid>,
) -> ApiResult<Json<Value>> {
    sqlx::query("DELETE FROM api_keys WHERE id = $1 AND user_id = $2").bind(id).bind(user).execute(&state.db).await?;
    Ok(Json(json!({ "ok": true })))
}

/* ------------------------------------------------------------------ */
/*                             OAuth apps                             */
/* ------------------------------------------------------------------ */

#[derive(Serialize, TS, sqlx::FromRow)]
#[ts(export)]
#[serde(rename_all = "camelCase")]
pub struct OAuthApp {
    pub id: String,
    pub name: String,
    pub description: Option<String>,
    pub homepage_url: Option<String>,
    pub redirect_uris: Vec<String>,
    pub created_at: DateTime<Utc>,
    /// People who authorized it.
    pub users: i64,
}

#[derive(Deserialize, TS)]
#[ts(export)]
#[serde(rename_all = "camelCase")]
pub struct SaveOAuthApp {
    pub name: String,
    #[ts(optional)]
    pub description: Option<String>,
    #[ts(optional)]
    pub homepage_url: Option<String>,
    pub redirect_uris: Vec<String>,
}

#[derive(Serialize, TS)]
#[ts(export)]
#[serde(rename_all = "camelCase")]
pub struct OAuthAppSecret {
    pub app: OAuthApp,
    /// Shown once.
    pub client_secret: String,
}

const APP_COLUMNS: &str = "c.id, c.name, c.description, c.homepage_url, c.redirect_uris, c.created_at,
    (SELECT count(DISTINCT g.user_id) FROM oauth_grants g WHERE g.client_id = c.id) AS users";

fn check_app(body: &SaveOAuthApp) -> ApiResult<()> {
    let name = body.name.trim();
    crate::ensure!(!name.is_empty() && name.len() <= 80, "The name is 1 to 80 characters");
    crate::ensure!(!body.redirect_uris.is_empty() && body.redirect_uris.len() <= 10, "Add 1 to 10 callback URLs");
    for uri in &body.redirect_uris {
        crate::ensure!(
            oauth::is_allowed_redirect(uri.trim()),
            "Callback URL not allowed: {uri} (https, http on localhost, or an app scheme)"
        );
    }
    if let Some(url) = body.homepage_url.as_deref().filter(|u| !u.trim().is_empty()) {
        crate::ensure!(url.starts_with("https://") || url.starts_with("http://"), "The homepage is an http(s) URL");
    }
    Ok(())
}

async fn list_oauth_apps(State(state): State<AppState>, Member(ctx): Member) -> ApiResult<Json<Vec<OAuthApp>>> {
    let apps = sqlx::query_as(&format!(
        "SELECT {APP_COLUMNS} FROM oauth_clients c WHERE c.workspace_id = $1 ORDER BY c.created_at DESC"
    ))
    .bind(ctx.workspace)
    .fetch_all(&state.db)
    .await?;
    Ok(Json(apps))
}

async fn create_oauth_app(
    State(state): State<AppState>,
    Member(ctx): Member,
    Json(body): Json<SaveOAuthApp>,
) -> ApiResult<Json<OAuthAppSecret>> {
    ctx.require_member()?;
    check_app(&body)?;
    let id = random_secret("").chars().filter(char::is_ascii_alphanumeric).take(32).collect::<String>().to_lowercase();
    let secret = random_secret("rr_cs_");
    sqlx::query(
        "INSERT INTO oauth_clients (id, secret_hash, name, description, homepage_url, redirect_uris, owner_id, workspace_id)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
    )
    .bind(&id)
    .bind(secret_hash(&secret))
    .bind(body.name.trim())
    .bind(body.description.as_deref().map(str::trim).filter(|d| !d.is_empty()))
    .bind(body.homepage_url.as_deref().map(str::trim).filter(|d| !d.is_empty()))
    .bind(body.redirect_uris.iter().map(|u| u.trim().to_string()).collect::<Vec<_>>())
    .bind(ctx.user)
    .bind(ctx.workspace)
    .execute(&state.db)
    .await?;
    let app = load_app(&state, &ctx, &id).await?;
    Ok(Json(OAuthAppSecret { app, client_secret: secret }))
}

async fn load_app(state: &AppState, ctx: &Ctx, id: &str) -> ApiResult<OAuthApp> {
    let app =
        sqlx::query_as(&format!("SELECT {APP_COLUMNS} FROM oauth_clients c WHERE c.id = $1 AND c.workspace_id = $2"))
            .bind(id)
            .bind(ctx.workspace)
            .fetch_optional(&state.db)
            .await?;
    app.ok_or_else(|| ApiError::not_found("No such app"))
}

/// The app's creator and workspace admins may change it.
async fn check_app_owner(state: &AppState, ctx: &Ctx, id: &str) -> ApiResult<()> {
    let owner: Option<Option<Uuid>> =
        sqlx::query_scalar("SELECT owner_id FROM oauth_clients WHERE id = $1 AND workspace_id = $2")
            .bind(id)
            .bind(ctx.workspace)
            .fetch_optional(&state.db)
            .await?;
    let Some(owner) = owner else { return Err(ApiError::not_found("No such app")) };
    if owner != Some(ctx.user) && ctx.role < Role::Admin {
        return Err(ApiError::forbidden("Only the app's creator or an admin can change it"));
    }
    Ok(())
}

async fn update_oauth_app(
    State(state): State<AppState>,
    Member(ctx): Member,
    Path(id): Path<String>,
    Json(body): Json<SaveOAuthApp>,
) -> ApiResult<Json<OAuthApp>> {
    check_app_owner(&state, &ctx, &id).await?;
    check_app(&body)?;
    sqlx::query(
        "UPDATE oauth_clients SET name = $2, description = $3, homepage_url = $4, redirect_uris = $5 WHERE id = $1",
    )
    .bind(&id)
    .bind(body.name.trim())
    .bind(body.description.as_deref().map(str::trim).filter(|d| !d.is_empty()))
    .bind(body.homepage_url.as_deref().map(str::trim).filter(|d| !d.is_empty()))
    .bind(body.redirect_uris.iter().map(|u| u.trim().to_string()).collect::<Vec<_>>())
    .execute(&state.db)
    .await?;
    Ok(Json(load_app(&state, &ctx, &id).await?))
}

async fn rotate_oauth_secret(
    State(state): State<AppState>,
    Member(ctx): Member,
    Path(id): Path<String>,
) -> ApiResult<Json<OAuthAppSecret>> {
    check_app_owner(&state, &ctx, &id).await?;
    let secret = random_secret("rr_cs_");
    sqlx::query("UPDATE oauth_clients SET secret_hash = $2 WHERE id = $1")
        .bind(&id)
        .bind(secret_hash(&secret))
        .execute(&state.db)
        .await?;
    let app = load_app(&state, &ctx, &id).await?;
    Ok(Json(OAuthAppSecret { app, client_secret: secret }))
}

async fn delete_oauth_app(
    State(state): State<AppState>,
    Member(ctx): Member,
    Path(id): Path<String>,
) -> ApiResult<Json<Value>> {
    check_app_owner(&state, &ctx, &id).await?;
    sqlx::query("DELETE FROM oauth_clients WHERE id = $1").bind(&id).execute(&state.db).await?;
    Ok(Json(json!({ "ok": true })))
}

/* ------------------------------------------------------------------ */
/*                           Connected apps                           */
/* ------------------------------------------------------------------ */

#[derive(Serialize, TS, sqlx::FromRow)]
#[ts(export)]
#[serde(rename_all = "camelCase")]
pub struct ConnectedApp {
    pub id: Uuid,
    pub name: String,
    pub workspace_name: String,
    pub scopes: Vec<String>,
    pub created_at: DateTime<Utc>,
    pub last_used_at: DateTime<Utc>,
}

async fn connected_apps(State(state): State<AppState>, SignedIn(user): SignedIn) -> ApiResult<Json<Vec<ConnectedApp>>> {
    let apps = sqlx::query_as(
        "SELECT g.id, c.name, w.name AS workspace_name, g.scopes, g.created_at, g.last_used_at FROM oauth_grants g
         JOIN oauth_clients c ON c.id = g.client_id JOIN workspaces w ON w.id = g.workspace_id
         WHERE g.user_id = $1 AND EXISTS (SELECT 1 FROM oauth_tokens t WHERE t.grant_id = g.id AND t.expires_at > now())
         ORDER BY g.last_used_at DESC",
    )
    .bind(user)
    .fetch_all(&state.db)
    .await?;
    Ok(Json(apps))
}

async fn revoke_connected_app(
    State(state): State<AppState>,
    SignedIn(user): SignedIn,
    Path(id): Path<Uuid>,
) -> ApiResult<Json<Value>> {
    sqlx::query("DELETE FROM oauth_grants WHERE id = $1 AND user_id = $2")
        .bind(id)
        .bind(user)
        .execute(&state.db)
        .await?;
    Ok(Json(json!({ "ok": true })))
}

/* ------------------------------------------------------------------ */
/*                           OAuth consent                            */
/* ------------------------------------------------------------------ */

async fn oauth_client(
    State(state): State<AppState>,
    Query(req): Query<OAuthRequest>,
) -> ApiResult<Json<OAuthClientInfo>> {
    oauth::check_request(&state, &req).await.map(Json).map_err(ApiError::bad_request)
}

#[derive(Deserialize, TS)]
#[ts(export)]
#[serde(rename_all = "camelCase")]
pub struct OAuthDecision {
    pub request: OAuthRequest,
    #[ts(optional)]
    pub workspace_id: Option<Uuid>,
    pub allow: bool,
}

#[derive(Serialize, TS)]
#[ts(export)]
pub struct OAuthRedirect {
    pub redirect: String,
}

async fn oauth_decide(
    State(state): State<AppState>,
    SignedIn(user): SignedIn,
    Json(body): Json<OAuthDecision>,
) -> ApiResult<Json<OAuthRedirect>> {
    let redirect = oauth::decide(&state, user, body.workspace_id, &body.request, body.allow)
        .await
        .map_err(ApiError::bad_request)?;
    Ok(Json(OAuthRedirect { redirect }))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn keys_are_checked() {
        assert_eq!(check_url_key(" Acme-Co ").unwrap(), "acme-co");
        assert!(check_url_key("-acme").is_err());
        assert!(check_url_key("api").is_err());
        assert_eq!(check_team_key("eng").unwrap(), "ENG");
        assert!(check_team_key("1ENG").is_err());
        assert!(check_team_key("ENGINEERS").is_err());
    }
}
