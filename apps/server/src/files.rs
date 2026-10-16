//! Uploaded files: the one-time upload URL agents `PUT` a file to, and downloads for members of
//! the file's workspace.

use axum::Json;
use axum::body::Bytes;
use axum::extract::{Path, State};
use axum::http::header::{
    AUTHORIZATION, CACHE_CONTROL, CONTENT_DISPOSITION, CONTENT_SECURITY_POLICY, CONTENT_TYPE, X_CONTENT_TYPE_OPTIONS,
};
use axum::http::{HeaderMap, StatusCode};
use axum::response::{IntoResponse, Response};
use percent_encoding::{NON_ALPHANUMERIC, utf8_percent_encode};
use serde_json::json;
use uuid::Uuid;

use crate::ctx::{Access, Caller};
use crate::error::{ApiError, ApiResult};
use crate::keys::secret_hash;
use crate::state::AppState;

/// Where a file is served from; Markdown and attachments point here.
pub fn asset_url(public_url: &str, id: Uuid) -> String {
    format!("{public_url}/files/{id}")
}

pub async fn upload(
    State(state): State<AppState>,
    Path(token): Path<String>,
    _headers: HeaderMap,
    body: Bytes,
) -> Response {
    match store(&state, &token, &body).await {
        Ok(response) => response,
        Err(e) => e.into_response(),
    }
}

async fn store(state: &AppState, token: &str, body: &[u8]) -> ApiResult<Response> {
    if body.len() > state.config.max_upload_size {
        return Err(ApiError::new(
            StatusCode::PAYLOAD_TOO_LARGE,
            format!("Files can have at most {} bytes", state.config.max_upload_size),
        ));
    }
    if body.is_empty() {
        return Err(ApiError::bad_request("The body is empty; send the file's bytes"));
    }
    let id: Option<Uuid> = sqlx::query_scalar(
        "UPDATE files SET data = $2, size = $3, upload_hash = NULL, upload_expires_at = NULL
         WHERE upload_hash = $1 AND upload_expires_at > now() AND data IS NULL RETURNING id",
    )
    .bind(secret_hash(token))
    .bind(body)
    .bind(body.len() as i64)
    .fetch_optional(&state.db)
    .await?;
    let id = id.ok_or_else(|| {
        ApiError::not_found(
            "This upload URL is unknown, expired or already used; get a new one with prepare_file_upload",
        )
    })?;
    let asset = asset_url(&state.config.public_url, id);
    Ok(Json(json!({ "fileId": id, "assetUrl": asset, "size": body.len() })).into_response())
}

pub async fn download(State(state): State<AppState>, Path(id): Path<String>, headers: HeaderMap) -> Response {
    match serve(&state, &id, &headers).await {
        Ok(response) => response,
        Err(e) => e.into_response(),
    }
}

/// A token or a dashboard session.
async fn caller(state: &AppState, headers: &HeaderMap) -> ApiResult<Caller> {
    if headers.contains_key(AUTHORIZATION) {
        let caller = crate::oauth::bearer_caller(state, headers).await.ok_or_else(ApiError::unauthorized)?;
        caller.require(Access::Read)?;
        return Ok(caller);
    }
    let user = state.keys.session_user(headers).ok_or_else(ApiError::unauthorized)?;
    Ok(Caller::session(state, user))
}

async fn serve(state: &AppState, id: &str, headers: &HeaderMap) -> ApiResult<Response> {
    let caller = caller(state, headers).await?;
    let not_found = || ApiError::not_found("No such file");
    let id = Uuid::parse_str(id).map_err(|_| not_found())?;
    let (workspace, name, content_type, data): (Uuid, String, String, Vec<u8>) =
        sqlx::query_as("SELECT workspace_id, name, content_type, data FROM files WHERE id = $1 AND data IS NOT NULL")
            .bind(id)
            .fetch_optional(&state.db)
            .await?
            .ok_or_else(not_found)?;
    if !caller.reachable().await?.iter().any(|w| w.id == workspace) {
        return Err(not_found());
    }
    let headers = [
        (CONTENT_TYPE, content_type.clone()),
        (CONTENT_DISPOSITION, disposition(&name, &content_type)),
        (X_CONTENT_TYPE_OPTIONS, "nosniff".to_string()),
        (CONTENT_SECURITY_POLICY, "sandbox".to_string()),
        (CACHE_CONTROL, "private, max-age=3600".to_string()),
    ];
    Ok((headers, data).into_response())
}

/// Types a browser would run are downloaded instead of shown.
fn disposition(name: &str, content_type: &str) -> String {
    let active = ["html", "svg", "xml", "javascript", "ecmascript"].iter().any(|t| content_type.contains(t));
    let kind = if active { "attachment" } else { "inline" };
    let ascii: String =
        name.chars().map(|c| if c.is_ascii_graphic() && c != '"' && c != '\\' || c == ' ' { c } else { '_' }).collect();
    format!("{kind}; filename=\"{ascii}\"; filename*=UTF-8''{}", utf8_percent_encode(name, NON_ALPHANUMERIC))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn dispositions() {
        assert_eq!(disposition("a b.png", "image/png"), "inline; filename=\"a b.png\"; filename*=UTF-8''a%20b%2Epng");
        assert!(disposition("x.svg", "image/svg+xml").starts_with("attachment; filename=\"x.svg\""));
        assert!(disposition("\"é\".txt", "text/plain").starts_with("inline; filename=\"___.txt\""));
    }
}
