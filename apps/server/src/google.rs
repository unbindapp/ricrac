//! Sign-in with Google: the authorization code flow with PKCE. Who someone is comes from the ID
//! token Google's token endpoint returns.

use axum::Json;
use axum::extract::{Query, State};
use axum::http::HeaderMap;
use axum::http::header::SET_COOKIE;
use axum::response::{AppendHeaders, IntoResponse, Redirect, Response};
use base64::Engine;
use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use chrono::Utc;
use serde::Deserialize;
use serde_json::Value;
use sha2::{Digest, Sha256};
use uuid::Uuid;

use crate::keys::{SESSION_TTL, build_cookie, get_cookie, is_secure_request, random_secret};
use crate::state::AppState;

const AUTHORIZE_URL: &str = "https://accounts.google.com/o/oauth2/v2/auth";
const TOKEN_URL: &str = "https://oauth2.googleapis.com/token";
const ISSUERS: [&str; 2] = ["https://accounts.google.com", "accounts.google.com"];
const SIGNIN_COOKIE: &str = "rr_signin";
const SIGNIN_TTL: i64 = 10 * 60;

fn redirect_uri(state: &AppState) -> String {
    format!("{}/api/auth/callback/google", state.config.public_url)
}

/// Only paths on this site, so sign-in can't be used as an open redirect.
fn safe_return_to(return_to: Option<&str>) -> &str {
    match return_to {
        Some(path) if path.starts_with('/') && !path.starts_with("//") && !path.starts_with("/\\") => path,
        _ => "/",
    }
}

fn with_error(return_to: &str, reason: &str) -> String {
    let separator = if return_to.contains('?') { '&' } else { '?' };
    format!("{return_to}{separator}signin_error={reason}")
}

struct Pending {
    expires: i64,
    state: String,
    nonce: String,
    verifier: String,
    return_to: String,
}

impl Pending {
    fn encode(&self) -> String {
        format!("signin:{}:{}:{}:{}:{}", self.expires, self.state, self.nonce, self.verifier, self.return_to)
    }

    fn decode(payload: &str) -> Option<Self> {
        let mut parts = payload.strip_prefix("signin:")?.splitn(5, ':');
        Some(Self {
            expires: parts.next()?.parse().ok()?,
            state: parts.next()?.to_string(),
            nonce: parts.next()?.to_string(),
            verifier: parts.next()?.to_string(),
            return_to: parts.next()?.to_string(),
        })
    }
}

#[derive(Deserialize)]
pub struct SignInParams {
    return_to: Option<String>,
}

pub async fn sign_in(
    State(state): State<AppState>,
    headers: HeaderMap,
    Query(params): Query<SignInParams>,
) -> Response {
    let return_to = safe_return_to(params.return_to.as_deref()).to_string();
    let Some((client_id, _)) = state.config.google() else {
        return Redirect::to(&with_error(&return_to, "not_configured")).into_response();
    };
    let pending = Pending {
        expires: Utc::now().timestamp() + SIGNIN_TTL,
        state: random_secret(""),
        nonce: random_secret(""),
        verifier: random_secret(""),
        return_to,
    };
    let challenge = URL_SAFE_NO_PAD.encode(Sha256::digest(pending.verifier.as_bytes()));
    let url = reqwest::Url::parse_with_params(
        AUTHORIZE_URL,
        [
            ("response_type", "code"),
            ("client_id", client_id),
            ("redirect_uri", &redirect_uri(&state)),
            ("scope", "openid email profile"),
            ("state", &pending.state),
            ("nonce", &pending.nonce),
            ("code_challenge", &challenge),
            ("code_challenge_method", "S256"),
            ("prompt", "select_account"),
        ],
    )
    .expect("static URL");
    let secure = is_secure_request(&state.config, &headers);
    let cookie =
        build_cookie(&state.config.cookie_name(SIGNIN_COOKIE), &state.keys.sign(&pending.encode()), SIGNIN_TTL, secure);
    (AppendHeaders([(SET_COOKIE, cookie)]), Redirect::to(url.as_str())).into_response()
}

pub async fn sign_out(State(state): State<AppState>, headers: HeaderMap) -> Response {
    let cookie = build_cookie(&state.keys.session_cookie, "", 0, is_secure_request(&state.config, &headers));
    (AppendHeaders([(SET_COOKIE, cookie)]), Json(serde_json::json!({ "ok": true }))).into_response()
}

#[derive(Deserialize)]
pub struct CallbackParams {
    code: Option<String>,
    state: Option<String>,
    error: Option<String>,
}

pub async fn callback(
    State(state): State<AppState>,
    headers: HeaderMap,
    Query(params): Query<CallbackParams>,
) -> Response {
    let secure = is_secure_request(&state.config, &headers);
    let signin_cookie = state.config.cookie_name(SIGNIN_COOKIE);
    let clear = build_cookie(&signin_cookie, "", 0, secure);
    let fail = |return_to: &str, reason: &str| {
        (AppendHeaders([(SET_COOKIE, clear.clone())]), Redirect::to(&with_error(return_to, reason))).into_response()
    };
    let pending = get_cookie(&headers, &signin_cookie)
        .and_then(|token| state.keys.verify(&token))
        .and_then(|payload| Pending::decode(&payload));
    let Some(pending) = pending.filter(|p| p.expires > Utc::now().timestamp()) else {
        return fail("/", "expired");
    };
    if params.state.as_deref() != Some(pending.state.as_str()) {
        return fail(&pending.return_to, "expired");
    }
    if let Some(error) = params.error {
        return fail(&pending.return_to, if error == "access_denied" { "denied" } else { "failed" });
    }
    let Some(code) = params.code else { return fail(&pending.return_to, "failed") };
    let identity = match exchange(&state, &code, &pending).await {
        Ok(identity) => identity,
        Err(err) => {
            tracing::warn!("Google sign-in failed: {err}");
            return fail(&pending.return_to, "failed");
        }
    };
    if !identity.email_verified {
        return fail(&pending.return_to, "unverified");
    }
    let user = match upsert_user(&state, &identity).await {
        Ok(Some(user)) => user,
        Ok(None) => return fail(&pending.return_to, "not_allowed"),
        Err(err) => {
            tracing::error!("saving the user failed: {err}");
            return fail(&pending.return_to, "failed");
        }
    };
    let session = build_cookie(&state.keys.session_cookie, &state.keys.session_token(user), SESSION_TTL, secure);
    (AppendHeaders([(SET_COOKIE, clear), (SET_COOKIE, session)]), Redirect::to(&pending.return_to)).into_response()
}

pub struct Identity {
    pub subject: String,
    pub email: String,
    pub email_verified: bool,
    pub name: Option<String>,
    pub picture: Option<String>,
}

#[derive(Deserialize)]
struct TokenResponse {
    id_token: String,
}

async fn exchange(state: &AppState, code: &str, pending: &Pending) -> Result<Identity, String> {
    let (client_id, client_secret) = state.config.google().ok_or("Google sign-in isn't configured")?;
    let response = state
        .http
        .post(TOKEN_URL)
        .form(&[
            ("grant_type", "authorization_code"),
            ("code", code),
            ("redirect_uri", &redirect_uri(state)),
            ("client_id", client_id),
            ("client_secret", client_secret),
            ("code_verifier", &pending.verifier),
        ])
        .send()
        .await
        .map_err(|e| format!("token request failed: {e}"))?;
    if !response.status().is_success() {
        let status = response.status();
        let body = response.text().await.unwrap_or_default();
        return Err(format!("token endpoint answered {status}: {body}"));
    }
    let tokens: TokenResponse = response.json().await.map_err(|e| format!("bad token response: {e}"))?;
    identity_from_id_token(&tokens.id_token, client_id, &pending.nonce)
}

/// The ID token comes straight from Google's token endpoint over TLS, so per OIDC Core 3.1.3.7
/// its signature doesn't need checking; the claims still do.
fn identity_from_id_token(id_token: &str, client_id: &str, nonce: &str) -> Result<Identity, String> {
    let payload = id_token.split('.').nth(1).ok_or("malformed ID token")?;
    let bytes = URL_SAFE_NO_PAD.decode(payload.trim_end_matches('=')).map_err(|_| "malformed ID token")?;
    let claims: Value = serde_json::from_slice(&bytes).map_err(|_| "malformed ID token")?;
    let issuer = claims["iss"].as_str().unwrap_or_default();
    if !ISSUERS.contains(&issuer) {
        return Err(format!("unexpected issuer {issuer}"));
    }
    let audience_ok = match &claims["aud"] {
        Value::String(aud) => aud == client_id,
        Value::Array(auds) => auds.iter().any(|aud| aud == client_id),
        _ => false,
    };
    if !audience_ok {
        return Err("ID token is for another client".into());
    }
    if claims["exp"].as_i64().unwrap_or(0) < Utc::now().timestamp() {
        return Err("ID token expired".into());
    }
    if claims["nonce"].as_str() != Some(nonce) {
        return Err("nonce mismatch".into());
    }
    let text = |key: &str| claims[key].as_str().filter(|v| !v.is_empty()).map(str::to_string);
    Ok(Identity {
        subject: text("sub").ok_or("ID token has no subject")?,
        email: text("email").ok_or("ID token has no email")?.to_lowercase(),
        email_verified: claims["email_verified"] == true || claims["email_verified"] == "true",
        name: text("name"),
        picture: text("picture"),
    })
}

/// Creates or refreshes the user. New accounts need an allowed email or an invitation.
pub async fn upsert_user(state: &AppState, identity: &Identity) -> Result<Option<Uuid>, sqlx::Error> {
    let existing: Option<Uuid> = sqlx::query_scalar(
        "SELECT id FROM users WHERE google_subject = $1 OR email = $2 ORDER BY google_subject = $1 DESC LIMIT 1",
    )
    .bind(&identity.subject)
    .bind(&identity.email)
    .fetch_optional(&state.db)
    .await?;
    if let Some(id) = existing {
        sqlx::query(
            "UPDATE users SET google_subject = $2, email = $3, avatar_url = coalesce($4, avatar_url), last_seen_at = now() WHERE id = $1",
        )
        .bind(id)
        .bind(&identity.subject)
        .bind(&identity.email)
        .bind(&identity.picture)
        .execute(&state.db)
        .await?;
        return Ok(Some(id));
    }
    let invited: bool =
        sqlx::query_scalar("SELECT EXISTS (SELECT 1 FROM invitations WHERE email = $1 AND expires_at > now())")
            .bind(&identity.email)
            .fetch_one(&state.db)
            .await?;
    if !invited && !state.config.allows_signup(&identity.email) {
        return Ok(None);
    }
    let name = identity.name.clone().unwrap_or_else(|| identity.email.split('@').next().unwrap_or("").to_string());
    let display_name = display_name(&name, &identity.email);
    let id = sqlx::query_scalar(
        "INSERT INTO users (google_subject, email, name, display_name, avatar_url) VALUES ($1, $2, $3, $4, $5) RETURNING id",
    )
    .bind(&identity.subject)
    .bind(&identity.email)
    .bind(&name)
    .bind(&display_name)
    .bind(&identity.picture)
    .fetch_one(&state.db)
    .await?;
    Ok(Some(id))
}

/// A short handle for @mentions: the first name, lowercased, or the email's local part.
pub fn display_name(name: &str, email: &str) -> String {
    let first: String = name.split_whitespace().next().unwrap_or("").chars().filter(|c| c.is_alphanumeric()).collect();
    if first.is_empty() { email.split('@').next().unwrap_or("user").to_lowercase() } else { first.to_lowercase() }
}

#[cfg(test)]
mod tests {
    use serde_json::json;

    use super::*;

    fn token(claims: Value) -> String {
        format!("e30.{}.sig", URL_SAFE_NO_PAD.encode(claims.to_string()))
    }

    fn claims() -> Value {
        json!({
            "iss": "https://accounts.google.com", "aud": "client", "exp": Utc::now().timestamp() + 60, "nonce": "n",
            "sub": "123", "email": "Ann@Example.com", "email_verified": true, "name": "Ann Lee",
        })
    }

    #[test]
    fn reads_identity_and_rejects_wrong_claims() {
        let identity = identity_from_id_token(&token(claims()), "client", "n").unwrap();
        assert_eq!(identity.email, "ann@example.com");
        assert!(identity.email_verified);
        for (key, value) in
            [("iss", json!("https://evil.com")), ("aud", json!("other")), ("exp", json!(1)), ("nonce", json!("x"))]
        {
            let mut bad = claims();
            bad[key] = value;
            assert!(identity_from_id_token(&token(bad), "client", "n").is_err(), "{key}");
        }
    }

    #[test]
    fn display_names() {
        assert_eq!(display_name("Ann Lee", "a@x.com"), "ann");
        assert_eq!(display_name("", "bob.s@x.com"), "bob.s");
    }
}
