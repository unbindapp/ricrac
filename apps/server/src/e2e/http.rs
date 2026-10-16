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
    key_reaching(w, scopes, Some(&[w.workspace])).await
}

/// A key of Ann's for these workspaces, or all of hers.
async fn key_reaching(w: &World, scopes: &[&str], workspaces: Option<&[Uuid]>) -> String {
    let key = random_secret(crate::oauth::API_KEY_PREFIX);
    sqlx::query("INSERT INTO api_keys (workspace_ids, user_id, label, key_hash, key_hint, scopes) VALUES ($1, $2, 'test', $3, 'x', $4)")
        .bind(workspaces)
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
    let stranger = w.second_workspace("Beta", "beta", "OPS").await;
    sqlx::query("UPDATE workspace_members SET active = false WHERE workspace_id = $1")
        .bind(stranger)
        .execute(&w.state.db)
        .await
        .unwrap();
    for workspaces in [vec![], vec![w.workspace, stranger]] {
        assert!(crate::oauth::decide(&w.state, w.ann.user, Some(workspaces), &req, true).await.is_err());
    }
    let redirect = crate::oauth::decide(&w.state, w.ann.user, Some(vec![w.workspace]), &req, true).await.unwrap();
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

fn with_header(mut request: Request<Body>, name: &'static str, value: &str) -> Request<Body> {
    request.headers_mut().insert(name, value.parse().unwrap());
    request
}

#[sqlx::test]
async fn tokens_for_several_workspaces_name_one_per_call(db: PgPool) {
    let w = world(db).await;
    let beta = w.second_workspace("Beta", "beta", "OPS").await;
    let router = crate::app::router(w.state.clone());
    let all = key_reaching(&w, &["read", "write"], None).await;
    let (_, listed) = send(&router, request("GET", "/v1/workspaces", &all, None)).await;
    let keys: Vec<&str> = listed["nodes"].as_array().unwrap().iter().map(|n| n["urlKey"].as_str().unwrap()).collect();
    assert_eq!(keys, ["acme", "beta"]);

    let issue = json!({ "team": "ENG", "title": "In Acme" });
    let (status, error) = send(&router, request("POST", "/v1/issues", &all, Some(issue.clone()))).await;
    assert_eq!(status, StatusCode::BAD_REQUEST);
    let message = error["error"].as_str().unwrap();
    assert!(message.contains("\"acme\" (Acme)") && message.contains("\"beta\" (Beta)"), "{message}");
    let (_, created) = send(
        &router,
        request("POST", "/v1/issues", &all, Some(json!({ "workspace": "acme", "team": "ENG", "title": "In Acme" }))),
    )
    .await;
    assert_eq!(created["identifier"], "ENG-1", "{created}");
    let in_beta = request("POST", "/v1/issues", &all, Some(json!({ "team": "OPS", "title": "In Beta" })));
    let (_, created) = send(&router, with_header(in_beta, "x-ricrac-workspace", "beta")).await;
    assert_eq!(created["identifier"], "OPS-1", "{created}");
    let (_, read) = send(&router, request("GET", "/v1/issues/ENG-1?workspace=acme", &all, None)).await;
    assert_eq!(read["title"], "In Acme");
    let link = json!({ "id": "http://localhost:3000/beta/issue/OPS-1" });
    let (_, read) = send(&router, request("POST", "/v1/ops/get_issue", &all, Some(link))).await;
    assert_eq!(read["title"], "In Beta", "links name their workspace");
    let (status, _) =
        send(&router, request("POST", "/v1/ops/get_issue", &all, Some(json!({ "workspace": "nope", "id": "ENG-1" }))))
            .await;
    assert_eq!(status, StatusCode::BAD_REQUEST);

    let only_beta = key_reaching(&w, &["read"], Some(&[beta])).await;
    let (_, listed) = send(&router, request("GET", "/v1/issues", &only_beta, None)).await;
    assert_eq!(identifiers(&listed), ["OPS-1"]);
    let (status, _) = send(&router, request("GET", "/v1/issues?workspace=acme", &only_beta, None)).await;
    assert_eq!(status, StatusCode::BAD_REQUEST, "the key doesn't reach acme");
}

#[sqlx::test]
async fn mcp_tools_take_the_workspace_when_the_token_reaches_several(db: PgPool) {
    let w = world(db).await;
    let router = crate::app::router(w.state.clone());
    let all = key_reaching(&w, &["read", "write"], None).await;
    let save_issue_schema = |key: String| {
        let router = router.clone();
        async move {
            let rpc = Request::post("/mcp")
                .header("authorization", format!("Bearer {key}"))
                .header("content-type", "application/json")
                .header("accept", "application/json, text/event-stream")
                .header("host", "localhost:3000")
                .header("mcp-protocol-version", "2025-06-18")
                .body(Body::from(json!({ "jsonrpc": "2.0", "id": 1, "method": "tools/list" }).to_string()))
                .unwrap();
            let (_, tools) = send(&router, rpc).await;
            let tools = tools["result"]["tools"].as_array().unwrap().clone();
            assert!(tools.iter().any(|t| t["name"] == "list_workspaces"));
            tools.into_iter().find(|t| t["name"] == "save_issue").unwrap()["inputSchema"].clone()
        }
    };
    let schema = save_issue_schema(all.clone()).await;
    assert!(schema["properties"].get("workspace").is_none(), "one workspace needs no argument");
    w.second_workspace("Beta", "beta", "OPS").await;
    let schema = save_issue_schema(all).await;
    assert!(schema["properties"]["workspace"].is_object());
    assert!(schema["required"].as_array().unwrap().contains(&json!("workspace")));
}

#[sqlx::test]
async fn leaving_a_workspace_takes_it_out_of_keys(db: PgPool) {
    let w = world(db).await;
    let beta = w.second_workspace("Beta", "beta", "OPS").await;
    let bob = w.member("bob@acme.test", "Bob", "member").await.user;
    sqlx::query("INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'member')")
        .bind(beta)
        .bind(bob)
        .execute(&w.state.db)
        .await
        .unwrap();
    let router = crate::app::router(w.state.clone());
    let cookie = format!("{}={}", w.state.keys.session_cookie, w.state.keys.session_token(bob));
    let dashboard = |method: &str, uri: &str, body: Value| {
        Request::builder()
            .method(method)
            .uri(uri)
            .header("cookie", &cookie)
            .header("x-ricrac", "1")
            .header("content-type", "application/json")
            .body(Body::from(body.to_string()))
            .unwrap()
    };
    let mut keys = vec![];
    for workspaces in [json!([beta]), json!([w.workspace, beta]), Value::Null] {
        let body = json!({ "label": "agent", "scopes": ["read"], "workspaceIds": workspaces });
        let (status, created) = send(&router, dashboard("POST", "/api/api-keys", body)).await;
        assert_eq!(status, StatusCode::OK, "{created}");
        keys.push(created["key"]["id"].as_str().unwrap().to_string());
    }
    let admin_key = json!({ "label": "admin", "scopes": ["admin"], "workspaceIds": [beta] });
    let (status, _) = send(&router, dashboard("POST", "/api/api-keys", admin_key)).await;
    assert_eq!(status, StatusCode::BAD_REQUEST, "Bob isn't an admin in beta");

    let (status, _) = send(&router, dashboard("POST", &format!("/api/workspaces/{beta}/leave"), json!({}))).await;
    assert_eq!(status, StatusCode::OK);
    let (_, listed) = send(&router, dashboard("GET", "/api/api-keys", Value::Null)).await;
    let reach: Vec<(String, bool, Value)> = listed
        .as_array()
        .unwrap()
        .iter()
        .map(|k| {
            (k["id"].as_str().unwrap().to_string(), k["allWorkspaces"].as_bool().unwrap(), k["workspaces"].clone())
        })
        .collect();
    assert!(!reach.iter().any(|(id, ..)| *id == keys[0]), "the key for beta alone is gone");
    assert!(reach.contains(&(keys[1].clone(), false, json!(["Acme"]))));
    assert!(reach.contains(&(keys[2].clone(), true, json!([]))));
}
