use axum::Json;
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use serde_json::json;

/// An error the caller can act on: the message says what was wrong. Database failures are logged
/// and reported without detail.
#[derive(Debug)]
pub struct ApiError {
    pub status: StatusCode,
    pub message: String,
}

impl ApiError {
    pub fn new(status: StatusCode, message: impl Into<String>) -> Self {
        Self { status, message: message.into() }
    }

    pub fn bad_request(message: impl Into<String>) -> Self {
        Self::new(StatusCode::BAD_REQUEST, message)
    }

    pub fn not_found(message: impl Into<String>) -> Self {
        Self::new(StatusCode::NOT_FOUND, message)
    }

    pub fn forbidden(message: impl Into<String>) -> Self {
        Self::new(StatusCode::FORBIDDEN, message)
    }

    pub fn conflict(message: impl Into<String>) -> Self {
        Self::new(StatusCode::CONFLICT, message)
    }

    pub fn unauthorized() -> Self {
        Self::new(StatusCode::UNAUTHORIZED, "Not signed in")
    }
}

impl From<sqlx::Error> for ApiError {
    fn from(err: sqlx::Error) -> Self {
        if let Some(db) = err.as_database_error() {
            if db.is_unique_violation() {
                return Self::conflict(format!("Already exists ({})", db.constraint().unwrap_or("unique")));
            }
            if db.is_foreign_key_violation() {
                return Self::bad_request("Refers to something that doesn't exist or is still in use");
            }
        }
        tracing::error!("database error: {err}");
        Self::new(StatusCode::INTERNAL_SERVER_ERROR, "Something went wrong")
    }
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        (self.status, Json(json!({ "error": self.message }))).into_response()
    }
}

pub type ApiResult<T> = Result<T, ApiError>;

/// Returns early with a 400 when the condition doesn't hold.
#[macro_export]
macro_rules! ensure {
    ($cond:expr, $($msg:tt)+) => {
        if !$cond {
            return Err($crate::error::ApiError::bad_request(format!($($msg)+)));
        }
    };
}
