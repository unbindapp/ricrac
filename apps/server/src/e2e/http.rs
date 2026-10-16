//! The REST API, OAuth and MCP through the real router.

use axum::body::Body;
use axum::http::{Request, StatusCode};
use base64::Engine;
use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use serde_json::{Value, json};
use sha2::{Digest, Sha256};
use sqlx::PgPool;
use tower::ServiceExt;

use super::*;
use crate::keys::{random_secret, secret_hash};

async fn send(router: &axum::Router, request: Request<Body>) -> (StatusCode, Value) {
    let response = router.clone().oneshot(request).await.unwrap();
    let status = response.status();
    let bytes = axum::body::to_bytes(response.into_body(), usize::MAX).await.unwrap();
    let value =
        serde_json::from_slice(&bytes).unwrap_or_else(|_| Value::String(String::from_utf8_lossy(&bytes).into()));
    (status, value)
}

async fn api_key(w: &World, scopes: &[&str]) -> String {
    let key = random_secret(crate::oauth::API_KEY_PREFIX);
    sqlx::query("INSERT INTO api_keys (workspace_id, user_id, label, key_hash, key_hint, scopes) VALUES ($1, $2, 'test', $3, 'x', $4)")
        .bind(w.workspace)
        .bind(w.ann.user)
        .bind(secret_hash(&key))
        .bind(scopes.iter().map(|s| s.to_string()).collect::<Vec<_>>())
        .execute(&w.state.db)
        .await
        .unwrap();
    key
}

fn request(method: &str, uri: &str, token: &str, body: Option<Value>) -> Request<Body> {
    let builder = Request::builder().method(method).uri(uri).header("authorization", format!("Bearer {token}"));
    match body {
        Some(body) => builder.header("content-type", "application/json").body(Body::from(body.to_string())).unwrap(),
        None => builder.body(Body::empty()).unwrap(),
    }
}

#[sqlx::test]
async fn rest_routes_and_scopes(db: PgPool) {
    let w = world(db).await;
    let router = crate::app::router(w.state.clone());
    let key = api_key(&w, &["read", "write"]).await;
    let (status, created) =
        send(&router, request("POST", "/v1/issues", &key, Some(json!({ "team": "ENG", "title": "From REST" })))).await;
    assert_eq!(status, StatusCode::OK, "{created}");
    assert_eq!(created["identifier"], "ENG-1");
    let (_, updated) = send(&router, request("PATCH", "/v1/issues/ENG-1", &key, Some(json!({ "priority": 2 })))).await;
    assert_eq!(updated["priority"], 2);
    let (_, listed) =
        send(&router, request("GET", "/v1/issues?priority=2&includeDescription=true&labels=", &key, None)).await;
    assert!(listed.get("error").is_some(), "unknown query arguments are refused: {listed}");
    let (_, listed) = send(&router, request("GET", "/v1/issues?priority=2&includeDescription=true", &key, None)).await;
    assert_eq!(listed["nodes"][0]["identifier"], "ENG-1");
    let filter = urlencode(&json!({ "title": { "contains": "rest" } }).to_string());
    let (_, filtered) = send(&router, request("GET", &format!("/v1/issues?filter={filter}"), &key, None)).await;
    assert_eq!(filtered["nodes"].as_array().unwrap().len(), 1);
    let (_, by_name) = send(&router, request("POST", "/v1/ops/get_issue", &key, Some(json!({ "id": "ENG-1" })))).await;
    assert_eq!(by_name["title"], "From REST");
    let read_only = api_key(&w, &["read"]).await;
    let (status, _) = send(&router, request("POST", "/v1/issues", &read_only, Some(json!({ "title": "x" })))).await;
    assert_eq!(status, StatusCode::FORBIDDEN);
    let (status, _) = send(&router, request("GET", "/v1/issues", "rr_key_wrong", None)).await;
    assert_eq!(status, StatusCode::UNAUTHORIZED);
    let (status, document) = send(&router, Request::get("/v1/openapi.json").body(Body::empty()).unwrap()).await;
    assert_eq!(status, StatusCode::OK);
    assert!(document["paths"]["/v1/issues/{id}"]["patch"].is_object());
}

fn urlencode(value: &str) -> String {
    percent_encoding::utf8_percent_encode(value, percent_encoding::NON_ALPHANUMERIC).to_string()
}

fn form(pairs: &[(&str, &str)]) -> Body {
    Body::from(pairs.iter().map(|(k, v)| format!("{k}={}", urlencode(v))).collect::<Vec<_>>().join("&"))
}

#[sqlx::test]
async fn oauth_flow_gives_a_token_for_the_chosen_workspace(db: PgPool) {
    let w = world(db).await;
    let router = crate::app::router(w.state.clone());
    let (status, client) = send(
        &router,
        Request::post("/oauth/register")
            .header("content-type", "application/json")
            .body(Body::from(
                json!({ "client_name": "Claude", "redirect_uris": ["http://localhost/callback"] }).to_string(),
            ))
            .unwrap(),
    )
    .await;
    assert_eq!(status, StatusCode::CREATED);
    let client_id = client["client_id"].as_str().unwrap().to_string();
    let verifier = random_secret("");
    let challenge = URL_SAFE_NO_PAD.encode(Sha256::digest(verifier.as_bytes()));
    let req = crate::oauth::OAuthRequest {
        client_id: client_id.clone(),
        redirect_uri: "http://localhost:4567/callback".into(),
        response_type: "code".into(),
        code_challenge: Some(challenge),
        code_challenge_method: Some("S256".into()),
        state: Some("xyz".into()),
        scope: None,
        resource: Some("http://localhost:3000/mcp".into()),
    };
    let redirect = crate::oauth::decide(&w.state, w.ann.user, Some(w.workspace), &req, true).await.unwrap();
    assert!(redirect.starts_with("http://localhost:4567/callback?code=") && redirect.contains("state=xyz"));
    let code = redirect.split("code=").nth(1).unwrap().split('&').next().unwrap();
    let code = percent_encoding::percent_decode_str(code).decode_utf8().unwrap().to_string();
    let token_request = |body| {
        Request::post("/oauth/token").header("content-type", "application/x-www-form-urlencoded").body(body).unwrap()
    };
    let (status, tokens) = send(
        &router,
        token_request(form(&[
            ("grant_type", "authorization_code"),
            ("code", &code),
            ("client_id", &client_id),
            ("redirect_uri", "http://localhost:4567/callback"),
            ("code_verifier", &verifier),
        ])),
    )
    .await;
    assert_eq!(status, StatusCode::OK, "{tokens}");
    let access = tokens["access_token"].as_str().unwrap();
    let (_, workspace) = send(&router, request("POST", "/v1/ops/list_issues", access, Some(json!({})))).await;
    assert!(workspace["nodes"].is_array(), "{workspace}");
    let (status, _) = send(
        &router,
        token_request(form(&[
            ("grant_type", "authorization_code"),
            ("code", &code),
            ("client_id", &client_id),
            ("code_verifier", &verifier),
        ])),
    )
    .await;
    assert_eq!(status, StatusCode::BAD_REQUEST, "codes are single use");
    let (status, refreshed) = send(
        &router,
        token_request(form(&[
            ("grant_type", "refresh_token"),
            ("refresh_token", tokens["refresh_token"].as_str().unwrap()),
            ("client_id", &client_id),
        ])),
    )
    .await;
    assert_eq!(status, StatusCode::OK);
    let (status, _) = send(
        &router,
        token_request(form(&[
            ("grant_type", "refresh_token"),
            ("refresh_token", tokens["refresh_token"].as_str().unwrap()),
            ("client_id", &client_id),
        ])),
    )
    .await;
    assert_eq!(status, StatusCode::BAD_REQUEST, "refresh tokens rotate");
    let (status, _) = send(
        &router,
        Request::post("/oauth/revoke")
            .header("content-type", "application/x-www-form-urlencoded")
            .body(form(&[("token", refreshed["access_token"].as_str().unwrap()), ("client_id", &client_id)]))
            .unwrap(),
    )
    .await;
    assert_eq!(status, StatusCode::OK);
    let (status, _) =
        send(&router, request("GET", "/v1/issues", refreshed["access_token"].as_str().unwrap(), None)).await;
    assert_eq!(status, StatusCode::UNAUTHORIZED);
}

#[sqlx::test]
async fn mcp_lists_and_calls_tools(db: PgPool) {
    let w = world(db).await;
    let router = crate::app::router(w.state.clone());
    let key = api_key(&w, &["read", "write", "admin"]).await;
    let (status, _) = send(&router, Request::post("/mcp").body(Body::empty()).unwrap()).await;
    assert_eq!(status, StatusCode::UNAUTHORIZED);
    let rpc_at = |uri: &'static str, body: Value| {
        Request::post(uri)
            .header("authorization", format!("Bearer {key}"))
            .header("content-type", "application/json")
            .header("accept", "application/json, text/event-stream")
            .header("host", "localhost:3000")
            .header("mcp-protocol-version", "2025-06-18")
            .body(Body::from(body.to_string()))
            .unwrap()
    };
    let rpc = |body: Value| rpc_at("/mcp", body);
    let (_, everything) =
        send(&router, rpc_at("/mcp?tools=all", json!({ "jsonrpc": "2.0", "id": 7, "method": "tools/list" }))).await;
    assert!(everything["result"]["tools"].as_array().unwrap().iter().any(|t| t["name"] == "save_webhook"));
    let (status, init) = send(
        &router,
        rpc(json!({ "jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {
        "protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": { "name": "test", "version": "1" } } })),
    )
    .await;
    assert_eq!(status, StatusCode::OK, "{init}");
    assert!(init["result"]["instructions"].as_str().unwrap().contains("issue tracker"));
    let (_, tools) = send(&router, rpc(json!({ "jsonrpc": "2.0", "id": 2, "method": "tools/list" }))).await;
    let names: Vec<&str> =
        tools["result"]["tools"].as_array().unwrap().iter().map(|t| t["name"].as_str().unwrap()).collect();
    assert!(names.contains(&"save_issue") && names.contains(&"find_operations"), "{names:?}");
    assert!(!names.contains(&"save_webhook"), "rarer operations are found through find_operations");
    let (_, found) = send(
        &router,
        rpc(json!({ "jsonrpc": "2.0", "id": 5, "method": "tools/call", "params": {
        "name": "find_operations", "arguments": { "query": "webhook" } } })),
    )
    .await;
    let text = found["result"]["content"][0]["text"].as_str().unwrap();
    assert!(text.contains("save_webhook") && text.contains("resourceTypes"), "{text}");
    let (_, called) = send(
        &router,
        rpc(json!({ "jsonrpc": "2.0", "id": 6, "method": "tools/call", "params": {
        "name": "call_operation", "arguments": { "name": "list_webhooks", "arguments": {} } } })),
    )
    .await;
    assert_eq!(called["result"]["isError"], false, "{called}");
    let (_, result) = send(
        &router,
        rpc(json!({ "jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {
        "name": "save_issue", "arguments": { "team": "ENG", "title": "From MCP" } } })),
    )
    .await;
    let text = result["result"]["content"][0]["text"].as_str().unwrap();
    assert!(text.contains("ENG-1"), "{result}");
    let (_, error) = send(
        &router,
        rpc(json!({ "jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": {
        "name": "save_issue", "arguments": { "team": "NOPE", "title": "x" } } })),
    )
    .await;
    assert_eq!(error["result"]["isError"], true, "{error}");
}
