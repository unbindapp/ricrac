//! The REST API under `/v1`, generated from the operations: each has its routes, and every
//! operation can also be called as `POST /v1/ops/{name}` with its arguments as the body. Also
//! serves the OpenAPI document and an overview for agents at `/llms.txt`.

use std::collections::{BTreeMap, HashMap};

use axum::Router;
use axum::body::Bytes;
use axum::extract::{FromRequestParts, Path, RawQuery, State};
use axum::http::request::Parts;
use axum::http::{HeaderMap, Method, StatusCode, header};
use axum::response::{IntoResponse, Response};
use axum::routing::{MethodFilter, MethodRouter, get, post};
use serde_json::{Map, Value, json};
use uuid::Uuid;

use crate::ctx::{ALL_SCOPES, Access, Ctx};
use crate::error::{ApiError, ApiResult};
use crate::mcp::INSTRUCTIONS;
use crate::ops::{self, Op};
use crate::state::AppState;

/// The caller of a `/v1` request: a bearer token, or the dashboard's session cookie with the
/// workspace in `x-ricrac-workspace`.
pub struct Caller(pub Ctx);

impl FromRequestParts<AppState> for Caller {
    type Rejection = Response;

    async fn from_request_parts(parts: &mut Parts, state: &AppState) -> Result<Self, Self::Rejection> {
        if let Some(ctx) = crate::oauth::bearer_ctx(state, &parts.headers).await {
            return Ok(Self(ctx));
        }
        if parts.headers.contains_key(header::AUTHORIZATION) {
            return Err(crate::oauth::unauthorized(state, &parts.headers));
        }
        let Some(user) = state.keys.session_user(&parts.headers) else {
            return Err(crate::oauth::unauthorized(state, &parts.headers));
        };
        if parts.method != Method::GET && !parts.headers.contains_key("x-ricrac") {
            return Err(ApiError::forbidden("Missing x-ricrac header").into_response());
        }
        let workspace = session_workspace(state, user, &parts.headers).await.map_err(IntoResponse::into_response)?;
        let scopes = vec![Access::Read, Access::Write, Access::Admin];
        match Ctx::load(state, user, workspace, scopes).await {
            Ok(Some(ctx)) => Ok(Self(ctx)),
            Ok(None) => Err(ApiError::forbidden("You aren't a member of that workspace").into_response()),
            Err(e) => Err(e.into_response()),
        }
    }
}

/// The workspace named in `x-ricrac-workspace` (id or URL key), or the user's only one.
pub async fn session_workspace(state: &AppState, user: Uuid, headers: &HeaderMap) -> ApiResult<Uuid> {
    let named =
        headers.get("x-ricrac-workspace").and_then(|v| v.to_str().ok()).map(str::trim).filter(|v| !v.is_empty());
    let ids: Vec<Uuid> = sqlx::query_scalar(
        "SELECT w.id FROM workspaces w JOIN workspace_members m ON m.workspace_id = w.id
         WHERE m.user_id = $1 AND m.active AND ($2::text IS NULL OR w.id::text = $2 OR w.url_key = lower($2))",
    )
    .bind(user)
    .bind(named)
    .fetch_all(&state.db)
    .await?;
    match ids.as_slice() {
        [one] => Ok(*one),
        [] => Err(ApiError::forbidden("No such workspace")),
        _ => Err(ApiError::bad_request("Name the workspace in the x-ricrac-workspace header")),
    }
}

pub fn router() -> Router<AppState> {
    let mut paths: BTreeMap<&'static str, MethodRouter<AppState>> = BTreeMap::new();
    for op in ops::all() {
        for (method, path) in op.routes {
            let filter = match *method {
                "GET" => MethodFilter::GET,
                "POST" => MethodFilter::POST,
                "PATCH" => MethodFilter::PATCH,
                "PUT" => MethodFilter::PUT,
                "DELETE" => MethodFilter::DELETE,
                other => panic!("unsupported method {other}"),
            };
            let name = op.name;
            let handler = move |State(state): State<AppState>,
                                caller: Caller,
                                Path(params): Path<HashMap<String, String>>,
                                RawQuery(query): RawQuery,
                                body: Bytes| async move {
                let _ = state;
                call(name, caller.0, params, query, body).await
            };
            let entry = paths.remove(path).unwrap_or_default();
            paths.insert(path, entry.on(filter, handler));
        }
    }
    let mut router =
        Router::new().route("/", get(index)).route("/openapi.json", get(openapi)).route("/ops", get(list_ops)).route(
            "/ops/{name}",
            post(|caller: Caller, Path(name): Path<String>, body: Bytes| async move {
                call(&name, caller.0, HashMap::new(), None, body).await
            }),
        );
    for (path, method_router) in paths {
        router = router.route(path, method_router);
    }
    router.fallback(|| async { ApiError::not_found("No such endpoint; see /v1/openapi.json") })
}

async fn call(name: &str, ctx: Ctx, path: HashMap<String, String>, query: Option<String>, body: Bytes) -> Response {
    let Some(op) = ops::find(name) else {
        return ApiError::not_found(format!("No operation {name}")).into_response();
    };
    let args = match arguments(op, path, query.as_deref(), &body) {
        Ok(args) => args,
        Err(e) => return e.into_response(),
    };
    match op.call(ctx, args).await {
        Ok(value) => (StatusCode::OK, axum::Json(value)).into_response(),
        Err(e) => e.into_response(),
    }
}

/// The JSON body, then the query string, then the path parameters, merged into one object.
fn arguments(op: &Op, path: HashMap<String, String>, query: Option<&str>, body: &[u8]) -> ApiResult<Value> {
    let mut args = Map::new();
    if !body.iter().all(u8::is_ascii_whitespace) {
        match serde_json::from_slice::<Value>(body) {
            Ok(Value::Object(map)) => args = map,
            Ok(_) => return Err(ApiError::bad_request("The body must be a JSON object")),
            Err(e) => return Err(ApiError::bad_request(format!("The body isn't valid JSON: {e}"))),
        }
    }
    let properties = op.schema.get("properties").and_then(Value::as_object);
    if let Some(query) = query {
        let mut lists: HashMap<String, Vec<String>> = HashMap::new();
        for pair in query.split('&').filter(|p| !p.is_empty()) {
            let (key, value) = pair.split_once('=').unwrap_or((pair, ""));
            let decode =
                |s: &str| percent_encoding::percent_decode_str(&s.replace('+', " ")).decode_utf8_lossy().to_string();
            lists.entry(decode(key).trim_end_matches("[]").to_string()).or_default().push(decode(value));
        }
        for (key, values) in lists {
            let schema = properties.and_then(|p| p.get(&key));
            args.insert(key, coerce(values, schema));
        }
    }
    for (key, value) in path {
        args.insert(key, Value::String(value));
    }
    Ok(Value::Object(args))
}

fn types(schema: Option<&Value>) -> Vec<String> {
    let Some(schema) = schema else { return vec![] };
    let mut found = Vec::new();
    match schema.get("type") {
        Some(Value::String(t)) => found.push(t.clone()),
        Some(Value::Array(ts)) => found.extend(ts.iter().filter_map(|t| t.as_str().map(String::from))),
        _ => {}
    }
    for key in ["anyOf", "oneOf"] {
        if let Some(Value::Array(options)) = schema.get(key) {
            for option in options {
                found.extend(types(Some(option)));
            }
        }
    }
    found
}

/// Query values are text; this turns them into what the argument's schema expects.
fn coerce(values: Vec<String>, schema: Option<&Value>) -> Value {
    let kinds = types(schema);
    let has = |k: &str| kinds.iter().any(|t| t == k);
    let one = |value: &str| -> Value {
        if value == "null" && has("null") {
            return Value::Null;
        }
        if has("boolean")
            && let Ok(b) = value.parse::<bool>()
        {
            return Value::Bool(b);
        }
        if (has("integer") || has("number"))
            && !has("string")
            && let Ok(n) = serde_json::from_str::<serde_json::Number>(value)
        {
            return Value::Number(n);
        }
        if has("object")
            && let Ok(v @ Value::Object(_)) = serde_json::from_str::<Value>(value)
        {
            return v;
        }
        if kinds.is_empty()
            && let Ok(v) = serde_json::from_str::<Value>(value)
        {
            return v;
        }
        Value::String(value.to_string())
    };
    if has("array") {
        let items = schema.and_then(|s| s.get("items"));
        if let [single] = values.as_slice()
            && let Ok(v @ Value::Array(_)) = serde_json::from_str::<Value>(single)
        {
            return v;
        }
        let parts: Vec<String> =
            values.iter().flat_map(|v| v.split(',').map(|s| s.trim().to_string())).filter(|s| !s.is_empty()).collect();
        return Value::Array(parts.into_iter().map(|p| coerce(vec![p], items)).collect());
    }
    one(values.last().map(String::as_str).unwrap_or(""))
}

async fn index(State(state): State<AppState>) -> axum::Json<Value> {
    let base = &state.config.public_url;
    axum::Json(json!({
        "name": "Ricrac API",
        "openapi": format!("{base}/v1/openapi.json"),
        "guide": format!("{base}/llms.txt"),
        "mcp": state.config.mcp_url(),
        "authentication": "Authorization: Bearer <OAuth access token or API key>",
    }))
}

async fn list_ops() -> axum::Json<Value> {
    let ops: Vec<Value> = ops::all()
        .iter()
        .map(|op| {
            json!({
                "name": op.name,
                "description": op.description,
                "access": op.access.as_str(),
                "routes": op.routes.iter().map(|(m, p)| format!("{m} /v1{p}")).collect::<Vec<_>>(),
                "arguments": op.schema,
            })
        })
        .collect();
    axum::Json(json!({ "operations": ops }))
}

fn path_params(path: &str) -> Vec<&str> {
    path.split('/').filter_map(|s| s.strip_prefix('{')?.strip_suffix('}')).collect()
}

pub fn openapi_document(base: &str) -> Value {
    let mut paths: BTreeMap<String, Map<String, Value>> = BTreeMap::new();
    for op in ops::all() {
        for (n, (method, path)) in op.routes.iter().enumerate() {
            let in_path = path_params(path);
            let properties = op.schema.get("properties").and_then(Value::as_object).cloned().unwrap_or_default();
            let required: Vec<&str> = op
                .schema
                .get("required")
                .and_then(Value::as_array)
                .map(|r| r.iter().filter_map(Value::as_str).collect())
                .unwrap_or_default();
            let mut parameters: Vec<Value> = in_path
                .iter()
                .map(|p| json!({ "name": p, "in": "path", "required": true, "schema": { "type": "string" } }))
                .collect();
            let mut operation = json!({
                "operationId": if n == 0 { op.name.to_string() } else { format!("{}_{}", op.name, method.to_lowercase()) },
                "summary": op.description.split(". ").next().unwrap_or(op.description),
                "description": format!("{}\n\nAlso callable as POST /v1/ops/{} and as the MCP tool {}.", op.description, op.name, op.name),
                "x-access": op.access.as_str(),
                "responses": {
                    "200": { "description": "The result", "content": { "application/json": { "schema": { "type": "object" } } } },
                    "400": { "description": "Invalid arguments; the error says why" },
                    "401": { "description": "Missing or invalid token" },
                    "403": { "description": "Not allowed" },
                    "404": { "description": "Not found" },
                },
            });
            let rest: Map<String, Value> =
                properties.into_iter().filter(|(k, _)| !in_path.contains(&k.as_str())).collect();
            if matches!(*method, "GET" | "DELETE") {
                parameters.extend(rest.iter().map(|(name, schema)| {
                    let complex = types(Some(schema)).iter().any(|t| t == "object");
                    let mut parameter = json!({
                        "name": name,
                        "in": "query",
                        "required": required.contains(&name.as_str()),
                        "description": schema.get("description").cloned().unwrap_or(Value::Null),
                    });
                    if complex {
                        parameter["content"] = json!({ "application/json": { "schema": schema } });
                    } else {
                        parameter["schema"] = schema.clone();
                    }
                    parameter
                }));
            } else if !rest.is_empty() {
                let mut body = op.schema.clone();
                body["properties"] = Value::Object(rest);
                if let Some(Value::Array(req)) = body.get_mut("required") {
                    req.retain(|r| !r.as_str().is_some_and(|r| in_path.contains(&r)));
                }
                operation["requestBody"] =
                    json!({ "required": false, "content": { "application/json": { "schema": body } } });
            }
            operation["parameters"] = Value::Array(parameters);
            paths.entry(format!("/v1{path}")).or_default().insert(method.to_lowercase(), operation);
        }
    }
    json!({
        "openapi": "3.1.0",
        "info": {
            "title": "Ricrac",
            "version": env!("CARGO_PKG_VERSION"),
            "description": INSTRUCTIONS,
        },
        "servers": [{ "url": base }],
        "security": [{ "bearer": [] }],
        "components": {
            "securitySchemes": {
                "bearer": { "type": "http", "scheme": "bearer", "description": "An OAuth access token or an API key (rr_key_...)" },
                "oauth": {
                    "type": "oauth2",
                    "flows": { "authorizationCode": {
                        "authorizationUrl": format!("{base}/oauth/authorize"),
                        "tokenUrl": format!("{base}/oauth/token"),
                        "refreshUrl": format!("{base}/oauth/token"),
                        "scopes": { "read": "Read everything", "write": "Create and change things", "admin": "Workspace administration" },
                    } },
                },
            },
        },
        "paths": paths,
    })
}

async fn openapi(State(state): State<AppState>) -> axum::Json<Value> {
    axum::Json(openapi_document(&state.config.public_url))
}

/// A Markdown overview for agents reading the API without MCP.
pub async fn llms_txt(State(state): State<AppState>) -> Response {
    let base = &state.config.public_url;
    let mut text = format!(
        "# Ricrac\n\n{INSTRUCTIONS}\n\n## Connecting\n\n- MCP (streamable HTTP): {base}/mcp, with OAuth (discovery at \
         {base}/.well-known/oauth-authorization-server; dynamic registration and client ID metadata documents are supported).\n\
         - REST: {base}/v1, with `Authorization: Bearer <token>`. Tokens are OAuth access tokens or API keys (rr_key_...) made in \
         the dashboard at {base}. Scopes: {}.\n- OpenAPI: {base}/v1/openapi.json. Every operation can also be called as \
         `POST {base}/v1/ops/<name>` with its arguments as a JSON body.\n- Query strings take arrays as repeated or \
         comma-separated values and objects (like `filter`) as JSON.\n\n## Operations\n",
        ALL_SCOPES.join(", ")
    );
    for op in ops::all() {
        let routes: Vec<String> = op.routes.iter().map(|(m, p)| format!("`{m} /v1{p}`")).collect();
        text.push_str(&format!(
            "\n### {}\n\n{} ({}; {})\n\n{}\n",
            op.name,
            routes.join(", "),
            op.access.as_str(),
            "MCP tool of the same name",
            op.description
        ));
    }
    ([(header::CONTENT_TYPE, "text/markdown; charset=utf-8")], text).into_response()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn query_values_follow_the_schema() {
        let schema = json!({
            "properties": {
                "priority": { "type": ["integer", "null"] },
                "query": { "type": ["string", "null"] },
                "includeArchived": { "type": "boolean" },
                "labels": { "type": ["array", "null"], "items": { "type": "string" } },
                "filter": {},
            }
        });
        let p = |k: &str| schema["properties"].get(k);
        assert_eq!(coerce(vec!["2".into()], p("priority")), json!(2));
        assert_eq!(coerce(vec!["123".into()], p("query")), json!("123"));
        assert_eq!(coerce(vec!["true".into()], p("includeArchived")), json!(true));
        assert_eq!(coerce(vec!["Bug,UI".into()], p("labels")), json!(["Bug", "UI"]));
        assert_eq!(coerce(vec!["Bug".into(), "UI".into()], p("labels")), json!(["Bug", "UI"]));
        assert_eq!(coerce(vec!["{\"priority\":{\"eq\":1}}".into()], p("filter")), json!({ "priority": { "eq": 1 } }));
    }

    #[test]
    fn openapi_lists_every_route() {
        let document = openapi_document("https://r.example");
        let count: usize = document["paths"].as_object().unwrap().values().map(|p| p.as_object().unwrap().len()).sum();
        let routes: usize = ops::all().iter().map(|op| op.routes.len()).sum();
        assert_eq!(count, routes);
    }
}
