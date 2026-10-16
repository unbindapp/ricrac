//! Uploaded files: the one-time upload URL agents `PUT` a file to, and downloads for members of
//! the file's workspace.

use axum::body::Bytes;
use axum::extract::{Path, State};
use axum::http::HeaderMap;
use axum::response::{IntoResponse, Response};

use crate::error::ApiError;
use crate::state::AppState;

pub async fn upload(
    State(_state): State<AppState>,
    Path(_token): Path<String>,
    _headers: HeaderMap,
    _body: Bytes,
) -> Response {
    ApiError::not_found("Not implemented").into_response()
}

pub async fn download(State(_state): State<AppState>, Path(_id): Path<String>, _headers: HeaderMap) -> Response {
    ApiError::not_found("Not implemented").into_response()
}
