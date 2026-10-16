//! Serves the web app from `apps/web/dist`, embedded into the binary at build time.

use axum::http::header::{CACHE_CONTROL, CONTENT_SECURITY_POLICY, CONTENT_TYPE};
use axum::http::{StatusCode, Uri};
use axum::response::{IntoResponse, Response};
use rust_embed::RustEmbed;

#[derive(RustEmbed)]
#[folder = "../web/dist/"]
#[allow_missing = true]
struct Assets;

const CSP: &str = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https: blob:; \
    media-src 'self'; connect-src 'self'; font-src 'self'; frame-src 'none'; frame-ancestors 'none'; \
    base-uri 'none'; form-action 'self'";

pub async fn serve(uri: Uri) -> Response {
    let path = uri.path().trim_start_matches('/');
    if let Some(file) = Assets::get(path).filter(|_| !path.is_empty()) {
        let mime = mime_guess::from_path(path).first_or_octet_stream();
        // Vite puts content-hashed files under assets/; everything else may change between deploys.
        let cache = if path.starts_with("assets/") { "public, max-age=31536000, immutable" } else { "no-cache" };
        return ([(CONTENT_TYPE, mime.as_ref()), (CACHE_CONTROL, cache)], file.data).into_response();
    }
    if path.starts_with("assets/") {
        return StatusCode::NOT_FOUND.into_response();
    }
    let Some(index) = Assets::get("index.html") else {
        return (StatusCode::SERVICE_UNAVAILABLE, "The dashboard is not built. Run `bun run build` in apps/web.")
            .into_response();
    };
    (
        [
            (CONTENT_TYPE, "text/html; charset=utf-8".to_string()),
            (CACHE_CONTROL, "no-cache".to_string()),
            (CONTENT_SECURITY_POLICY, CSP.to_string()),
        ],
        index.data,
    )
        .into_response()
}
