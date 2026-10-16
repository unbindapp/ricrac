//! Builds the router. `main` serves it; the end-to-end tests start it too.

use axum::Router;
use axum::extract::DefaultBodyLimit;
use axum::http::HeaderValue;
use axum::http::header::{REFERRER_POLICY, X_CONTENT_TYPE_OPTIONS, X_FRAME_OPTIONS};
use axum::routing::{get, post, put};
use rmcp::transport::streamable_http_server::{
    StreamableHttpServerConfig, StreamableHttpService, session::local::LocalSessionManager,
};
use tower_http::compression::CompressionLayer;
use tower_http::compression::predicate::{NotForContentType, Predicate, SizeAbove};
use tower_http::cors::{Any, CorsLayer};
use tower_http::set_header::SetResponseHeaderLayer;
use tower_http::trace::TraceLayer;

use crate::mcp::Mcp;
use crate::state::AppState;
use crate::{account, files, oauth, rest, spa};

pub fn router(state: AppState) -> Router {
    let compression = CompressionLayer::new()
        .compress_when(SizeAbove::new(1024).and(NotForContentType::const_new("text/event-stream")));
    // MCP, OAuth and the REST API are called from other origins with bearer tokens, never cookies
    // alone, so any origin may call them.
    let cors = CorsLayer::new()
        .allow_origin(Any)
        .allow_methods(Any)
        .allow_headers(Any)
        .expose_headers([axum::http::header::WWW_AUTHENTICATE, axum::http::HeaderName::from_static("mcp-session-id")]);
    let mut mcp_config = StreamableHttpServerConfig::default().with_legacy_session_mode(false).with_json_response(true);
    if let Some(host) = state.config.public_url.split("://").nth(1) {
        mcp_config = mcp_config.with_allowed_hosts([host.to_string(), "localhost".into(), "127.0.0.1".into()]);
    }
    let mcp_service = StreamableHttpService::new(|| Ok(Mcp), LocalSessionManager::default().into(), mcp_config);
    let mcp_router = Router::new()
        .nest_service("/mcp", mcp_service)
        .layer(axum::middleware::from_fn_with_state(state.clone(), oauth::require_bearer));
    let public = Router::new()
        .route("/.well-known/oauth-protected-resource", get(oauth::protected_resource))
        .route("/.well-known/oauth-protected-resource/mcp", get(oauth::protected_resource))
        .route("/.well-known/oauth-authorization-server", get(oauth::authorization_server))
        .route("/.well-known/oauth-authorization-server/mcp", get(oauth::authorization_server))
        .route("/oauth/register", post(oauth::register))
        .route("/oauth/token", post(oauth::token))
        .route("/oauth/revoke", post(oauth::revoke))
        .route("/llms.txt", get(rest::llms_txt))
        .nest("/v1", rest::router())
        .route("/files/{id}", get(files::download))
        .route("/uploads/{token}", put(files::upload).layer(DefaultBodyLimit::max(state.config.max_upload_size)))
        .merge(mcp_router)
        .layer(cors);
    let header = |name, value| SetResponseHeaderLayer::if_not_present(name, HeaderValue::from_static(value));
    Router::new()
        .merge(public)
        .nest("/api", account::router())
        .route("/healthz", get(|| async { "ok" }))
        .fallback(spa::serve)
        .layer(DefaultBodyLimit::max(4 * 1024 * 1024))
        .layer(header(X_CONTENT_TYPE_OPTIONS, "nosniff"))
        .layer(header(X_FRAME_OPTIONS, "DENY"))
        .layer(header(REFERRER_POLICY, "strict-origin-when-cross-origin"))
        .layer(compression)
        .layer(TraceLayer::new_for_http())
        .with_state(state)
}
