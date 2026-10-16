//! OAuth 2.1 authorization server: discovery, dynamic client registration, client ID metadata
//! documents, apps registered in the dashboard, authorization code + PKCE, rotating refresh
//! tokens and revocation. Every grant is for one workspace, chosen on the consent screen.
//! Also the bearer check shared by `/v1` and `/mcp`, which takes these tokens and API keys.

use std::collections::HashMap;
use std::time::Duration;

use axum::Json;
use axum::extract::{Request, State};
use axum::http::{HeaderMap, HeaderValue, StatusCode, header};
use axum::middleware::Next;
use axum::response::{IntoResponse, Response};
use base64::Engine;
use base64::engine::general_purpose::{STANDARD, URL_SAFE_NO_PAD};
use chrono::{DateTime, Utc};
use percent_encoding::{AsciiSet, NON_ALPHANUMERIC, utf8_percent_encode};
use serde::{Deserialize, Serialize};
use serde_json::json;
use sha2::{Digest, Sha256};
use ts_rs::TS;
use uuid::Uuid;

use crate::ctx::{ALL_SCOPES, Access, Ctx};
use crate::keys::{client_ip, random_secret, secret_hash};
use crate::state::AppState;

const CODE_TTL: i64 = 10 * 60;
const ACCESS_TTL: i64 = 24 * 3600;
const REFRESH_TTL: i64 = 90 * 24 * 3600;
const CIMD_MAX_BYTES: usize = 64 * 1024;
const CIMD_CACHE_TTL: i64 = 3600;

pub const API_KEY_PREFIX: &str = "rr_key_";

fn resource_metadata_url(base: &str) -> String {
    format!("{base}/.well-known/oauth-protected-resource/mcp")
}

/* ------------------------------------------------------------------ */
/*                              Discovery                             */
/* ------------------------------------------------------------------ */

pub async fn protected_resource(State(state): State<AppState>) -> Response {
    let base = &state.config.public_url;
    Json(json!({
        "resource": state.config.mcp_url(),
        "authorization_servers": [base],
        "scopes_supported": ALL_SCOPES,
        "bearer_methods_supported": ["header"],
        "resource_name": "Ricrac",
    }))
    .into_response()
}

pub async fn authorization_server(State(state): State<AppState>) -> Response {
    let base = &state.config.public_url;
    Json(json!({
        "issuer": base,
        "authorization_endpoint": format!("{base}/oauth/authorize"),
        "token_endpoint": format!("{base}/oauth/token"),
        "registration_endpoint": format!("{base}/oauth/register"),
        "revocation_endpoint": format!("{base}/oauth/revoke"),
        "scopes_supported": ALL_SCOPES,
        "response_types_supported": ["code"],
        "grant_types_supported": ["authorization_code", "refresh_token"],
        "code_challenge_methods_supported": ["S256"],
        "token_endpoint_auth_methods_supported": ["none", "client_secret_post", "client_secret_basic"],
        "client_id_metadata_document_supported": true,
        "authorization_response_iss_parameter_supported": true,
    }))
    .into_response()
}

fn oauth_error(status: StatusCode, error: &str, description: &str) -> Response {
    let mut res = (status, Json(json!({ "error": error, "error_description": description }))).into_response();
    res.headers_mut().insert(header::CACHE_CONTROL, HeaderValue::from_static("no-store"));
    res
}

/* ------------------------------------------------------------------ */
/*                           Redirect URIs                            */
/* ------------------------------------------------------------------ */

fn strip_port(authority: &str) -> &str {
    match authority.rsplit_once(':') {
        Some((host, port)) if !port.is_empty() && port.bytes().all(|b| b.is_ascii_digit()) => host,
        _ => authority,
    }
}

fn loopback_parts(uri: &str) -> Option<(&str, &str)> {
    let rest = uri.strip_prefix("http://")?;
    let (authority, path) = rest.split_once('/').unwrap_or((rest, ""));
    let host = strip_port(authority);
    matches!(host, "localhost" | "127.0.0.1" | "[::1]").then_some((host, path))
}

/// https anywhere, http only on loopback (native clients such as Claude Code), or an app's own
/// URI scheme. PKCE or the client secret protects the code in every case.
pub fn is_allowed_redirect(uri: &str) -> bool {
    if uri.len() > 2000 || uri.contains('#') {
        return false;
    }
    let Some((scheme, rest)) = uri.split_once("://") else { return false };
    match scheme {
        "https" => !rest.is_empty(),
        "http" => loopback_parts(uri).is_some(),
        "javascript" | "data" | "file" | "vbscript" => false,
        s => !s.is_empty() && s.chars().all(|c| c.is_ascii_alphanumeric() || matches!(c, '+' | '.' | '-')),
    }
}

/// Exact match, except that loopback clients may use any port (RFC 8252 section 7.3).
fn redirect_matches(registered: &str, requested: &str) -> bool {
    if registered == requested {
        return true;
    }
    match (loopback_parts(registered), loopback_parts(requested)) {
        (Some(a), Some(b)) => a == b,
        _ => false,
    }
}

/* ------------------------------------------------------------------ */
/*                    Dynamic client registration                     */
/* ------------------------------------------------------------------ */

#[derive(Deserialize)]
pub struct RegisterRequest {
    #[serde(default)]
    redirect_uris: Vec<String>,
    client_name: Option<String>,
    token_endpoint_auth_method: Option<String>,
}

pub async fn register(State(state): State<AppState>, headers: HeaderMap, Json(req): Json<RegisterRequest>) -> Response {
    if state.limiter.hit(&format!("register:{}", client_ip(&headers))) {
        return oauth_error(
            StatusCode::TOO_MANY_REQUESTS,
            "invalid_request",
            "Too many registrations, try again later",
        );
    }
    if req.redirect_uris.is_empty() || req.redirect_uris.len() > 10 {
        return oauth_error(StatusCode::BAD_REQUEST, "invalid_redirect_uri", "Provide 1 to 10 redirect_uris");
    }
    if let Some(bad) = req.redirect_uris.iter().find(|u| !is_allowed_redirect(u)) {
        return oauth_error(
            StatusCode::BAD_REQUEST,
            "invalid_redirect_uri",
            &format!("Redirect URI not allowed: {bad}"),
        );
    }
    let method = req.token_endpoint_auth_method.as_deref().unwrap_or("none");
    let secret = match method {
        "none" => None,
        "client_secret_post" | "client_secret_basic" => Some(random_secret("rr_cs_")),
        other => {
            return oauth_error(
                StatusCode::BAD_REQUEST,
                "invalid_client_metadata",
                &format!("Unsupported auth method {other}"),
            );
        }
    };
    let name: String = req.client_name.unwrap_or_else(|| "MCP client".into()).trim().chars().take(100).collect();
    let id = format!("dcr_{}", random_secret("").chars().take(24).collect::<String>());
    let result =
        sqlx::query("INSERT INTO oauth_clients (id, secret_hash, name, redirect_uris) VALUES ($1, $2, $3, $4)")
            .bind(&id)
            .bind(secret.as_deref().map(secret_hash))
            .bind(&name)
            .bind(&req.redirect_uris)
            .execute(&state.db)
            .await;
    if let Err(e) = result {
        tracing::error!("client registration failed: {e}");
        return oauth_error(StatusCode::INTERNAL_SERVER_ERROR, "server_error", "Registration failed");
    }
    let mut body = json!({
        "client_id": id,
        "client_id_issued_at": Utc::now().timestamp(),
        "client_name": name,
        "redirect_uris": req.redirect_uris,
        "grant_types": ["authorization_code", "refresh_token"],
        "response_types": ["code"],
        "token_endpoint_auth_method": method,
    });
    if let Some(secret) = secret {
        body["client_secret"] = json!(secret);
        body["client_secret_expires_at"] = json!(0);
    }
    (StatusCode::CREATED, Json(body)).into_response()
}

/* ------------------------------------------------------------------ */
/*                     Client ID metadata documents                   */
/* ------------------------------------------------------------------ */

/// An https URL with a path on a named public host, e.g. Claude's published client identity.
fn is_client_id_url(id: &str) -> bool {
    let Some(rest) = id.strip_prefix("https://") else { return false };
    let Some((authority, path)) = rest.split_once('/') else { return false };
    let host = strip_port(authority);
    !path.is_empty()
        && !id.contains(['#', '?'])
        && host.contains('.')
        && !host.starts_with('[')
        && !host.bytes().all(|b| b.is_ascii_digit() || b == b'.')
}

#[derive(Deserialize)]
struct ClientMetadata {
    client_id: String,
    client_name: String,
    #[serde(default)]
    redirect_uris: Vec<String>,
    token_endpoint_auth_method: Option<String>,
    #[serde(default)]
    token_endpoint_auth_methods_supported: Vec<String>,
}

impl ClientMetadata {
    /// ChatGPT prefers `private_key_jwt` but lists `none` as a fallback, which is what we offer.
    fn supports_public_auth(&self) -> bool {
        self.token_endpoint_auth_method.as_deref().unwrap_or("none") == "none"
            || self.token_endpoint_auth_methods_supported.iter().any(|m| m == "none")
    }
}

async fn fetch_client_metadata(url: &str) -> Result<ClientMetadata, String> {
    let http = reqwest::Client::builder()
        .timeout(Duration::from_secs(5))
        .redirect(reqwest::redirect::Policy::none())
        .user_agent("ricrac")
        .build()
        .map_err(|e| e.to_string())?;
    let mut res = http.get(url).header(header::ACCEPT, "application/json").send().await.map_err(|e| e.to_string())?;
    if !res.status().is_success() {
        return Err(format!("HTTP {}", res.status()));
    }
    let mut body = Vec::new();
    while let Some(chunk) = res.chunk().await.map_err(|e| e.to_string())? {
        body.extend_from_slice(&chunk);
        if body.len() > CIMD_MAX_BYTES {
            return Err("document too large".into());
        }
    }
    let doc: ClientMetadata = serde_json::from_slice(&body).map_err(|e| e.to_string())?;
    if doc.client_id != url {
        return Err("client_id does not match the document URL".into());
    }
    if !doc.supports_public_auth() {
        return Err("only public clients are supported".into());
    }
    if doc.redirect_uris.is_empty() || doc.redirect_uris.len() > 10 {
        return Err("provide 1 to 10 redirect_uris".into());
    }
    if let Some(bad) = doc.redirect_uris.iter().find(|u| !is_allowed_redirect(u)) {
        return Err(format!("redirect URI not allowed: {bad}"));
    }
    Ok(doc)
}

/// Refreshes the document at most once an hour and keeps the last copy when the fetch fails.
async fn cimd_client(state: &AppState, url: &str) -> Result<Client, String> {
    let cached = load_client(state, url).await;
    if let Some(client) = &cached
        && client.fetched_at.is_some_and(|at| at.timestamp() > Utc::now().timestamp() - CIMD_CACHE_TTL)
    {
        return Ok(client.clone());
    }
    let doc = match fetch_client_metadata(url).await {
        Ok(doc) => doc,
        Err(e) => {
            tracing::warn!("fetching client metadata {url} failed: {e}");
            return cached.ok_or_else(|| "Could not read the application's identity document.".into());
        }
    };
    let name: String = doc.client_name.trim().chars().take(100).collect();
    let name = if name.is_empty() { "MCP client".to_string() } else { name };
    sqlx::query(
        "INSERT INTO oauth_clients (id, name, redirect_uris, fetched_at) VALUES ($1, $2, $3, now())
         ON CONFLICT (id) DO UPDATE SET name = excluded.name, redirect_uris = excluded.redirect_uris, fetched_at = now()",
    )
    .bind(url)
    .bind(&name)
    .bind(&doc.redirect_uris)
    .execute(&state.db)
    .await
    .map_err(|e| {
        tracing::error!("storing client metadata failed: {e}");
        "Database error".to_string()
    })?;
    Ok(Client {
        name,
        redirect_uris: doc.redirect_uris,
        secret_hash: None,
        owner_id: None,
        fetched_at: Some(Utc::now()),
    })
}

/* ------------------------------------------------------------------ */
/*                            Authorization                           */
/* ------------------------------------------------------------------ */

#[derive(Clone, sqlx::FromRow)]
struct Client {
    name: String,
    redirect_uris: Vec<String>,
    secret_hash: Option<String>,
    owner_id: Option<Uuid>,
    fetched_at: Option<DateTime<Utc>>,
}

async fn load_client(state: &AppState, id: &str) -> Option<Client> {
    sqlx::query_as("SELECT name, redirect_uris, secret_hash, owner_id, fetched_at FROM oauth_clients WHERE id = $1")
        .bind(id)
        .fetch_optional(&state.db)
        .await
        .ok()
        .flatten()
}

fn resource_matches(resource: Option<&str>, base: &str) -> bool {
    resource.is_none_or(|r| {
        let r = r.trim_end_matches('/');
        r == base || r == format!("{base}/mcp") || r == format!("{base}/v1")
    })
}

/// The query of `/oauth/authorize`, passed on by the consent page.
#[derive(Deserialize, Serialize, TS, Clone)]
#[ts(export)]
pub struct OAuthRequest {
    pub client_id: String,
    pub redirect_uri: String,
    pub response_type: String,
    #[ts(optional)]
    pub code_challenge: Option<String>,
    #[ts(optional)]
    pub code_challenge_method: Option<String>,
    #[ts(optional)]
    pub state: Option<String>,
    #[ts(optional)]
    pub scope: Option<String>,
    #[ts(optional)]
    pub resource: Option<String>,
}

#[derive(Serialize, TS)]
#[ts(export)]
pub struct OAuthClientInfo {
    pub client_name: String,
    pub redirect_host: String,
    pub scopes: Vec<String>,
    /// Registered in a dashboard by someone, rather than by the client itself.
    pub verified: bool,
}

fn requested_scopes(scope: Option<&str>) -> Result<Vec<String>, String> {
    let Some(scope) = scope.map(str::trim).filter(|s| !s.is_empty()) else {
        return Ok(ALL_SCOPES.iter().map(|s| s.to_string()).collect());
    };
    let mut scopes = Vec::new();
    for part in scope.split([' ', ',']).filter(|s| !s.is_empty()) {
        if !ALL_SCOPES.contains(&part) {
            return Err(format!("Unknown scope \"{part}\". Scopes are read, write and admin."));
        }
        if !scopes.iter().any(|s| s == part) {
            scopes.push(part.to_string());
        }
    }
    // Writing implies reading.
    if !scopes.iter().any(|s| s == "read") {
        scopes.insert(0, "read".into());
    }
    Ok(scopes)
}

/// Checks an authorization request before the consent screen. Errors are shown on the page,
/// never sent to an unverified redirect URI.
pub async fn check_request(state: &AppState, req: &OAuthRequest) -> Result<OAuthClientInfo, String> {
    let base = &state.config.public_url;
    let client = if is_client_id_url(&req.client_id) {
        cimd_client(state, &req.client_id).await?
    } else {
        load_client(state, &req.client_id).await.ok_or("Unknown application. Try connecting again.")?
    };
    if !client.redirect_uris.iter().any(|u| redirect_matches(u, &req.redirect_uri)) {
        return Err("The application's redirect address doesn't match its registration.".into());
    }
    if req.response_type != "code" {
        return Err("Unsupported response_type.".into());
    }
    match (req.code_challenge.as_deref(), req.code_challenge_method.as_deref()) {
        (Some(challenge), Some("S256")) if challenge.len() >= 43 => {}
        (None, None) if client.secret_hash.is_some() && client.owner_id.is_some() => {}
        _ => return Err("The application must use PKCE (S256).".into()),
    }
    if !resource_matches(req.resource.as_deref(), base) {
        return Err("The application asked for access to a different server.".into());
    }
    let scopes = requested_scopes(req.scope.as_deref())?;
    let redirect_host =
        req.redirect_uri.split("://").nth(1).and_then(|r| r.split(['/', '?']).next()).unwrap_or("").to_string();
    Ok(OAuthClientInfo { client_name: client.name, redirect_host, scopes, verified: client.owner_id.is_some() })
}

/// RFC 3986 unreserved characters stay as they are.
const QUERY_VALUE: &AsciiSet = &NON_ALPHANUMERIC.remove(b'-').remove(b'.').remove(b'_').remove(b'~');

fn with_params(uri: &str, params: &[(&str, &str)]) -> String {
    let query: Vec<String> =
        params.iter().map(|(k, v)| format!("{k}={}", utf8_percent_encode(v, QUERY_VALUE))).collect();
    let sep = if uri.contains('?') { '&' } else { '?' };
    format!("{uri}{sep}{}", query.join("&"))
}

/// Records the user's decision and returns where to send the browser.
pub async fn decide(
    state: &AppState,
    user: Uuid,
    workspace: Option<Uuid>,
    req: &OAuthRequest,
    allow: bool,
) -> Result<String, String> {
    let info = check_request(state, req).await?;
    let base = state.config.public_url.as_str();
    let state_param = req.state.as_deref().unwrap_or("");
    let mut params = vec![("iss", base)];
    if !state_param.is_empty() {
        params.push(("state", state_param));
    }
    if !allow {
        params.push(("error", "access_denied"));
        return Ok(with_params(&req.redirect_uri, &params));
    }
    let workspace = workspace.ok_or("Pick a workspace.")?;
    if Ctx::load(state, user, workspace, vec![]).await.map_err(|e| e.message)?.is_none() {
        return Err("You aren't a member of that workspace.".into());
    }
    let code = random_secret("rr_ac_");
    sqlx::query(
        "INSERT INTO oauth_codes (code_hash, client_id, user_id, workspace_id, scopes, redirect_uri, code_challenge, resource, expires_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now() + make_interval(secs => $9))",
    )
    .bind(secret_hash(&code))
    .bind(&req.client_id)
    .bind(user)
    .bind(workspace)
    .bind(&info.scopes)
    .bind(&req.redirect_uri)
    .bind(&req.code_challenge)
    .bind(&req.resource)
    .bind(CODE_TTL as f64)
    .execute(&state.db)
    .await
    .map_err(|e| {
        tracing::error!("storing authorization code failed: {e}");
        "Database error".to_string()
    })?;
    params.insert(0, ("code", &code));
    Ok(with_params(&req.redirect_uri, &params))
}

/* ------------------------------------------------------------------ */
/*                               Tokens                               */
/* ------------------------------------------------------------------ */

fn client_credentials(headers: &HeaderMap, form: &HashMap<String, String>) -> (Option<String>, Option<String>) {
    let basic = headers
        .get(header::AUTHORIZATION)
        .and_then(|v| v.to_str().ok())
        .and_then(|v| v.strip_prefix("Basic "))
        .and_then(|v| STANDARD.decode(v.trim()).ok())
        .and_then(|v| String::from_utf8(v).ok())
        .and_then(|v| v.split_once(':').map(|(id, secret)| (id.to_string(), secret.to_string())));
    if let Some((id, secret)) = basic {
        let decode = |s: &str| percent_encoding::percent_decode_str(s).decode_utf8_lossy().to_string();
        return (Some(decode(&id)), Some(decode(&secret)));
    }
    (form.get("client_id").cloned(), form.get("client_secret").cloned())
}

/// The client and whether it proved its secret. Clients with a secret may leave it out only when
/// the grant is protected by PKCE.
#[allow(clippy::result_large_err)]
async fn authenticate_client(
    state: &AppState,
    headers: &HeaderMap,
    form: &HashMap<String, String>,
) -> Result<(String, bool), Response> {
    let (id, secret) = client_credentials(headers, form);
    let invalid = || oauth_error(StatusCode::UNAUTHORIZED, "invalid_client", "Unknown client or bad credentials");
    let Some(id) = id else { return Err(invalid()) };
    let Some(client) = load_client(state, &id).await else { return Err(invalid()) };
    match (client.secret_hash, secret) {
        (Some(expected), Some(given)) if secret_hash(&given) == expected => Ok((id, true)),
        (Some(_), Some(_)) => Err(invalid()),
        (_, _) => Ok((id, false)),
    }
}

async fn issue_tokens(state: &AppState, grant: Uuid, scopes: &[String]) -> Result<Response, sqlx::Error> {
    let access = random_secret("rr_at_");
    let refresh = random_secret("rr_rt_");
    for (token, kind, ttl) in [(&access, "access", ACCESS_TTL), (&refresh, "refresh", REFRESH_TTL)] {
        sqlx::query(
            "INSERT INTO oauth_tokens (token_hash, kind, grant_id, expires_at) VALUES ($1, $2, $3, now() + make_interval(secs => $4))",
        )
        .bind(secret_hash(token))
        .bind(kind)
        .bind(grant)
        .bind(ttl as f64)
        .execute(&state.db)
        .await?;
    }
    let mut res = Json(json!({
        "access_token": access,
        "token_type": "Bearer",
        "expires_in": ACCESS_TTL,
        "refresh_token": refresh,
        "scope": scopes.join(" "),
    }))
    .into_response();
    res.headers_mut().insert(header::CACHE_CONTROL, HeaderValue::from_static("no-store"));
    Ok(res)
}

fn pkce_matches(verifier: &str, challenge: &str) -> bool {
    (43..=128).contains(&verifier.len()) && URL_SAFE_NO_PAD.encode(Sha256::digest(verifier.as_bytes())) == challenge
}

pub async fn token(
    State(state): State<AppState>,
    headers: HeaderMap,
    axum::Form(form): axum::Form<HashMap<String, String>>,
) -> Response {
    let (client_id, authenticated) = match authenticate_client(&state, &headers, &form).await {
        Ok(client) => client,
        Err(res) => return res,
    };
    let result = match form.get("grant_type").map(String::as_str) {
        Some("authorization_code") => exchange_code(&state, &client_id, authenticated, &form).await,
        Some("refresh_token") => refresh(&state, &client_id, &form).await,
        _ => {
            return oauth_error(
                StatusCode::BAD_REQUEST,
                "unsupported_grant_type",
                "Use authorization_code or refresh_token",
            );
        }
    };
    result.unwrap_or_else(|e| {
        tracing::error!("token endpoint failed: {e}");
        oauth_error(StatusCode::INTERNAL_SERVER_ERROR, "server_error", "Token request failed")
    })
}

type CodeRow = (String, Uuid, Uuid, Vec<String>, String, Option<String>, Option<String>, DateTime<Utc>);

async fn exchange_code(
    state: &AppState,
    client_id: &str,
    authenticated: bool,
    form: &HashMap<String, String>,
) -> Result<Response, sqlx::Error> {
    let invalid = |msg: &str| Ok(oauth_error(StatusCode::BAD_REQUEST, "invalid_grant", msg));
    let Some(code) = form.get("code") else { return invalid("Missing code") };
    // Single use: the code is deleted whether or not the exchange succeeds.
    let row: Option<CodeRow> = sqlx::query_as(
        "DELETE FROM oauth_codes WHERE code_hash = $1
         RETURNING client_id, user_id, workspace_id, scopes, redirect_uri, code_challenge, resource, expires_at",
    )
    .bind(secret_hash(code))
    .fetch_optional(&state.db)
    .await?;
    let Some((code_client, user, workspace, scopes, redirect_uri, challenge, resource, expires_at)) = row else {
        return invalid("Unknown or used code");
    };
    if expires_at < Utc::now() || code_client != client_id {
        return invalid("Expired code or wrong client");
    }
    if form.get("redirect_uri").is_some_and(|r| r != &redirect_uri) {
        return invalid("redirect_uri does not match");
    }
    match challenge {
        Some(challenge) => {
            if !form.get("code_verifier").is_some_and(|v| pkce_matches(v, &challenge)) {
                return invalid("PKCE verification failed");
            }
        }
        None if !authenticated => return invalid("This client must authenticate with its secret or use PKCE"),
        None => {}
    }
    let requested = form.get("resource").map(String::as_str).or(resource.as_deref());
    if !resource_matches(requested, &state.config.public_url) {
        return Ok(oauth_error(StatusCode::BAD_REQUEST, "invalid_target", "Tokens are only issued for this server"));
    }
    let grant: Uuid = sqlx::query_scalar(
        "INSERT INTO oauth_grants (client_id, user_id, workspace_id, scopes) VALUES ($1, $2, $3, $4) RETURNING id",
    )
    .bind(client_id)
    .bind(user)
    .bind(workspace)
    .bind(&scopes)
    .fetch_one(&state.db)
    .await?;
    issue_tokens(state, grant, &scopes).await
}

async fn refresh(state: &AppState, client_id: &str, form: &HashMap<String, String>) -> Result<Response, sqlx::Error> {
    let invalid = || Ok(oauth_error(StatusCode::BAD_REQUEST, "invalid_grant", "Invalid or expired refresh token"));
    let Some(token) = form.get("refresh_token") else { return invalid() };
    // Rotation: the old refresh token stops working as soon as it is used.
    let row: Option<(Uuid, DateTime<Utc>)> = sqlx::query_as(
        "DELETE FROM oauth_tokens WHERE token_hash = $1 AND kind = 'refresh' RETURNING grant_id, expires_at",
    )
    .bind(secret_hash(token))
    .fetch_optional(&state.db)
    .await?;
    let Some((grant, expires_at)) = row else { return invalid() };
    let found: Option<(String, Vec<String>)> =
        sqlx::query_as("SELECT client_id, scopes FROM oauth_grants WHERE id = $1")
            .bind(grant)
            .fetch_optional(&state.db)
            .await?;
    let Some((grant_client, scopes)) = found else { return invalid() };
    if expires_at < Utc::now() || grant_client != client_id {
        return invalid();
    }
    issue_tokens(state, grant, &scopes).await
}

/// RFC 7009: revoking either token of a grant ends the whole grant. Unknown tokens are fine.
pub async fn revoke(
    State(state): State<AppState>,
    headers: HeaderMap,
    axum::Form(form): axum::Form<HashMap<String, String>>,
) -> Response {
    let (client_id, _) = match authenticate_client(&state, &headers, &form).await {
        Ok(client) => client,
        Err(res) => return res,
    };
    let Some(token) = form.get("token") else {
        return oauth_error(StatusCode::BAD_REQUEST, "invalid_request", "Missing token");
    };
    let result = sqlx::query(
        "DELETE FROM oauth_grants g USING oauth_tokens t WHERE t.token_hash = $1 AND t.grant_id = g.id AND g.client_id = $2",
    )
    .bind(secret_hash(token))
    .bind(&client_id)
    .execute(&state.db)
    .await;
    if let Err(e) = result {
        tracing::error!("revoking failed: {e}");
        return oauth_error(StatusCode::INTERNAL_SERVER_ERROR, "server_error", "Revocation failed");
    }
    StatusCode::OK.into_response()
}

/* ------------------------------------------------------------------ */
/*                           Bearer checks                            */
/* ------------------------------------------------------------------ */

/// The token from `Authorization: Bearer <token>`, or a bare API key.
fn bearer(headers: &HeaderMap) -> Option<&str> {
    let value = headers.get(header::AUTHORIZATION)?.to_str().ok()?.trim();
    let token = value.strip_prefix("Bearer ").or_else(|| value.strip_prefix("bearer ")).unwrap_or(value);
    Some(token.trim()).filter(|t| !t.is_empty())
}

/// The caller of a request with an OAuth access token or an API key.
pub async fn bearer_ctx(state: &AppState, headers: &HeaderMap) -> Option<Ctx> {
    let token = bearer(headers)?;
    if token.starts_with(API_KEY_PREFIX) {
        let (id, user, workspace, scopes): (Uuid, Uuid, Uuid, Vec<String>) =
            sqlx::query_as("SELECT id, user_id, workspace_id, scopes FROM api_keys WHERE key_hash = $1")
                .bind(secret_hash(token))
                .fetch_optional(&state.db)
                .await
                .ok()??;
        let _ = sqlx::query("UPDATE api_keys SET last_used_at = now() WHERE id = $1 AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute')")
            .bind(id)
            .execute(&state.db)
            .await;
        return Ctx::load(state, user, workspace, Access::parse_scopes(&scopes)).await.ok()?;
    }
    let (grant, user, workspace, scopes, expires_at): (Uuid, Uuid, Uuid, Vec<String>, DateTime<Utc>) = sqlx::query_as(
        "SELECT g.id, g.user_id, g.workspace_id, g.scopes, t.expires_at FROM oauth_tokens t
         JOIN oauth_grants g ON g.id = t.grant_id WHERE t.token_hash = $1 AND t.kind = 'access'",
    )
    .bind(secret_hash(token))
    .fetch_optional(&state.db)
    .await
    .ok()??;
    if expires_at < Utc::now() {
        return None;
    }
    let _ = sqlx::query(
        "UPDATE oauth_grants SET last_used_at = now() WHERE id = $1 AND last_used_at < now() - interval '1 minute'",
    )
    .bind(grant)
    .execute(&state.db)
    .await;
    Ctx::load(state, user, workspace, Access::parse_scopes(&scopes)).await.ok()?
}

/// Rejects `/mcp` requests without a valid token, pointing clients at the OAuth metadata.
pub async fn require_bearer(State(state): State<AppState>, mut req: Request, next: Next) -> Response {
    let Some(ctx) = bearer_ctx(&state, req.headers()).await else {
        return unauthorized(&state, req.headers());
    };
    req.extensions_mut().insert(ctx);
    next.run(req).await
}

pub fn unauthorized(state: &AppState, headers: &HeaderMap) -> Response {
    let sent_token = headers.contains_key(header::AUTHORIZATION);
    let challenge = format!(
        "Bearer resource_metadata=\"{}\"{}",
        resource_metadata_url(&state.config.public_url),
        if sent_token { ", error=\"invalid_token\"" } else { "" }
    );
    let message = if sent_token {
        "The token is invalid or expired"
    } else {
        "Authenticate with an OAuth access token or an API key: Authorization: Bearer <token>"
    };
    let mut res = (StatusCode::UNAUTHORIZED, Json(json!({ "error": message }))).into_response();
    if let Ok(value) = HeaderValue::from_str(&challenge) {
        res.headers_mut().insert(header::WWW_AUTHENTICATE, value);
    }
    res
}

/// Drops expired codes and tokens, grants without live tokens, and registrations never used.
pub async fn cleanup(state: &AppState) -> Result<(), sqlx::Error> {
    sqlx::query("DELETE FROM oauth_codes WHERE expires_at < now()").execute(&state.db).await?;
    sqlx::query("DELETE FROM oauth_tokens WHERE expires_at < now()").execute(&state.db).await?;
    sqlx::query("DELETE FROM oauth_grants g WHERE NOT EXISTS (SELECT 1 FROM oauth_tokens t WHERE t.grant_id = g.id)")
        .execute(&state.db)
        .await?;
    sqlx::query(
        "DELETE FROM oauth_clients c WHERE c.owner_id IS NULL AND c.created_at < now() - interval '1 day'
         AND NOT EXISTS (SELECT 1 FROM oauth_grants g WHERE g.client_id = c.id)",
    )
    .execute(&state.db)
    .await?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn redirects() {
        assert!(is_allowed_redirect("https://claude.ai/api/mcp/auth_callback"));
        assert!(is_allowed_redirect("http://localhost:33418/callback"));
        assert!(is_allowed_redirect("http://[::1]:8080/cb"));
        assert!(is_allowed_redirect("motile://ricrac"));
        assert!(!is_allowed_redirect("http://evil.com/cb"));
        assert!(!is_allowed_redirect("http://localhost.evil.com/cb"));
        assert!(!is_allowed_redirect("javascript://alert(1)"));
        assert!(!is_allowed_redirect("https://x.com/cb#frag"));
        assert!(redirect_matches("http://localhost/callback", "http://localhost:3118/callback"));
        assert!(!redirect_matches("http://localhost/callback", "http://127.0.0.1:3118/callback"));
        assert!(!redirect_matches("https://a.com/cb", "https://a.com:444/cb"));
    }

    #[test]
    fn client_id_urls() {
        assert!(is_client_id_url("https://claude.ai/oauth/claude-code-client-metadata"));
        assert!(!is_client_id_url("https://claude.ai/"));
        assert!(!is_client_id_url("http://claude.ai/oauth/metadata"));
        assert!(!is_client_id_url("https://10.0.0.1/metadata"));
    }

    #[test]
    fn scopes() {
        assert_eq!(requested_scopes(None).unwrap(), vec!["read", "write", "admin"]);
        assert_eq!(requested_scopes(Some("write")).unwrap(), vec!["read", "write"]);
        assert!(requested_scopes(Some("issues:create")).is_err());
    }

    #[test]
    fn pkce() {
        assert!(pkce_matches(
            "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk",
            "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
        ));
        assert!(!pkce_matches("short", "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"));
    }
}
