//! The MCP server at `/mcp`: one tool per operation, acting as the user and workspace of the
//! request's token.

use std::sync::Arc;

use rmcp::model::{
    CallToolRequestParams, CallToolResponse, CallToolResult, ContentBlock, Implementation, ListToolsResult,
    PaginatedRequestParams, ServerCapabilities, ServerConfig, Tool, ToolAnnotations,
};
use rmcp::service::RequestContext;
use rmcp::{ErrorData as McpError, RoleServer, ServerHandler};
use serde_json::Value;

use crate::ctx::{Access, Ctx, Role};
use crate::error::{ApiError, ApiResult};
use crate::ops;

/// The tools listed by default. Every other operation is reached through find_operations and
/// call_operation, so a session doesn't start with ~140 tool definitions; `/mcp?tools=all` lists
/// them all instead.
const CORE: &[&str] = &[
    "get_workspace",
    "list_users",
    "get_user",
    "list_teams",
    "get_team",
    "list_issue_statuses",
    "list_issue_labels",
    "save_issue_label",
    "list_issues",
    "get_issue",
    "save_issue",
    "update_issues",
    "delete_issue",
    "get_issue_history",
    "list_comments",
    "save_comment",
    "delete_comment",
    "add_reaction",
    "create_attachment",
    "prepare_file_upload",
    "upload_file",
    "list_projects",
    "get_project",
    "save_project",
    "list_project_statuses",
    "list_milestones",
    "save_milestone",
    "get_status_updates",
    "save_status_update",
    "list_initiatives",
    "get_initiative",
    "save_initiative",
    "list_cycles",
    "get_cycle",
    "list_documents",
    "get_document",
    "save_document",
    "list_custom_views",
    "list_templates",
    "get_template",
    "get_notifications",
    "mark_notification",
    "search",
];

#[derive(serde::Deserialize, schemars::JsonSchema)]
#[serde(deny_unknown_fields)]
struct FindOperationsArgs {
    /// Words to look for in operation names and descriptions, e.g. "webhook", "customer", "release note",
    /// "favorite", "archive project". Leave out to list every operation by name.
    query: Option<String>,
}

#[derive(serde::Deserialize, schemars::JsonSchema)]
#[serde(deny_unknown_fields)]
struct CallOperationArgs {
    /// The operation's name, from find_operations.
    name: String,
    /// Its arguments, as described by find_operations.
    #[serde(default)]
    #[schemars(with = "serde_json::Map<String, Value>")]
    arguments: Value,
}

const FIND_DESCRIPTION: &str = "Find operations beyond the tools listed here: webhooks, customers and customer needs, \
releases, pipelines and release notes, favorites, document versions, templates and custom views (saving/deleting), \
project and issue labels and statuses (saving/deleting), cycles (saving/archiving), teams and members (saving, \
inviting, roles), archiving/restoring, sharing, the audit log, and more. With a query, returns the matching operations \
with their argument schemas; call them with call_operation.";

const CALL_DESCRIPTION: &str = "Run any Ricrac operation by name with its arguments (see find_operations). Returns \
the operation's result.";

pub const INSTRUCTIONS: &str = "Ricrac is an issue tracker for agents: a workspace has teams; \
teams have issues (identifiers like ENG-123), workflow statuses (triage, backlog, unstarted, started, completed, \
canceled), labels, cycles and templates; projects (with milestones, status updates and documents) span teams and \
roll up into initiatives. There are also comments, attachments, custom views, favorites, an inbox of notifications, \
customers, releases and webhooks.

Referencing things: wherever a tool takes a person, team, status, label, project, cycle or milestone you may pass a \
name, key or identifier instead of an id. People also take an email or \"me\". Issues take their identifier (ENG-123), \
id or link. Errors list the valid options when a name doesn't match.

Writing: descriptions, comments, documents and updates are Markdown; write literal newlines, not escape sequences. \
Mention people with @displayName or @email: they are notified and subscribed. To change part of a long description or \
document, pass `patch` edits instead of resending it all.

Saving: save_* tools create when you omit `id` and update when you pass it; omitted fields stay as they are and null \
clears a field. List tools page with `limit` and `cursor` (pass pageInfo.endCursor). Start with get_workspace or \
list_teams to learn the teams and their keys, and list_issue_statuses for a team's workflow.

More operations: the tools listed cover everyday work. Everything else (webhooks, customers, releases, favorites, \
templates and views management, team and member administration, archiving, sharing, ...) is found with \
find_operations and run with call_operation.";

#[derive(Clone, Default)]
pub struct Mcp;

fn caller(ctx: &RequestContext<RoleServer>) -> Result<Ctx, McpError> {
    let parts = ctx
        .extensions
        .get::<axum::http::request::Parts>()
        .ok_or_else(|| McpError::internal_error("missing request", None))?;
    parts.extensions.get::<Ctx>().cloned().ok_or_else(|| McpError::invalid_request("not authenticated", None))
}

fn lists_all_tools(context: &RequestContext<RoleServer>) -> bool {
    let query = context.extensions.get::<axum::http::request::Parts>().and_then(|parts| parts.uri.query());
    query.is_some_and(|q| q.split('&').any(|pair| pair == "tools=all"))
}

fn summary(description: &str) -> &str {
    description.split_once(". ").map_or(description, |(first, _)| first)
}

fn find_operations(ctx: &Ctx, args: FindOperationsArgs) -> Value {
    let candidates = ops::all().iter().filter(|op| visible(op, ctx));
    let Some(query) = args.query.map(|q| q.to_lowercase()).filter(|q| !q.trim().is_empty()) else {
        let names: Vec<Value> =
            candidates.map(|op| serde_json::json!({ "name": op.name, "summary": summary(op.description) })).collect();
        return serde_json::json!({ "operations": names, "hint": "Search with a query to get argument schemas." });
    };
    let words: Vec<String> = query.split_whitespace().map(|w| w.trim_end_matches('s').to_string()).collect();
    let mut scored: Vec<(usize, &ops::Op)> = candidates
        .map(|op| {
            let name = op.name.replace('_', " ");
            let text = op.description.to_lowercase();
            let score = words
                .iter()
                .map(|w| {
                    if name.contains(w.as_str()) {
                        3
                    } else if text.contains(w.as_str()) {
                        1
                    } else {
                        0
                    }
                })
                .sum();
            (score, op)
        })
        .filter(|(score, _)| *score > 0)
        .collect();
    scored.sort_by_key(|s| std::cmp::Reverse(s.0));
    let found: Vec<Value> = scored
        .into_iter()
        .take(8)
        .map(|(_, op)| serde_json::json!({ "name": op.name, "description": op.description, "arguments": op.schema }))
        .collect();
    serde_json::json!({ "operations": found })
}

async fn call_operation(ctx: Ctx, args: CallOperationArgs) -> ApiResult<Value> {
    let Some(op) = ops::find(args.name.trim()) else {
        return Err(ApiError::bad_request(format!("No operation \"{}\"; use find_operations", args.name)));
    };
    op.call(ctx, args.arguments).await
}

/// Tools the caller may use: write tools need the write scope, admin tools an admin.
fn visible(op: &ops::Op, ctx: &Ctx) -> bool {
    ctx.allows(op.access) && (op.access != Access::Admin || ctx.role >= Role::Admin)
}

fn tool(op: &ops::Op) -> Tool {
    let schema = match &op.schema {
        Value::Object(map) => map.clone(),
        _ => Default::default(),
    };
    let annotations = ToolAnnotations::from_raw(
        None,
        Some(op.is_read_only()),
        Some(op.name.starts_with("delete_") || op.name.starts_with("remove_")),
        Some(op.is_read_only()),
        Some(false),
    );
    Tool::new_with_raw(op.name, Some(op.description.into()), Arc::new(schema)).with_annotations(annotations)
}

fn meta_tools() -> Vec<Tool> {
    let schema = |value: Value| Arc::new(value.as_object().cloned().unwrap_or_default());
    let read = ToolAnnotations::from_raw(None, Some(true), Some(false), Some(true), Some(false));
    vec![
        Tool::new_with_raw(
            "find_operations",
            Some(FIND_DESCRIPTION.into()),
            schema(ops::schema_of::<FindOperationsArgs>()),
        )
        .with_annotations(read),
        Tool::new_with_raw(
            "call_operation",
            Some(CALL_DESCRIPTION.into()),
            schema(ops::schema_of::<CallOperationArgs>()),
        ),
    ]
}

impl ServerHandler for Mcp {
    fn get_info(&self) -> ServerConfig {
        ServerConfig::new(ServerCapabilities::builder().enable_tools().build())
            .with_server_info(Implementation::new("ricrac", env!("CARGO_PKG_VERSION")))
            .with_instructions(INSTRUCTIONS)
    }

    async fn list_tools(
        &self,
        _request: Option<PaginatedRequestParams>,
        context: RequestContext<RoleServer>,
    ) -> Result<ListToolsResult, McpError> {
        let ctx = caller(&context)?;
        let all = lists_all_tools(&context);
        let mut tools: Vec<Tool> =
            ops::all().iter().filter(|op| visible(op, &ctx) && (all || CORE.contains(&op.name))).map(tool).collect();
        if !all {
            tools.extend(meta_tools());
        }
        Ok(ListToolsResult::with_all_items(tools))
    }

    async fn call_tool(
        &self,
        request: CallToolRequestParams,
        context: RequestContext<RoleServer>,
    ) -> Result<CallToolResponse, McpError> {
        let ctx = caller(&context)?;
        let args = request.arguments.map(Value::Object).unwrap_or(Value::Null);
        let outcome = match request.name.as_ref() {
            "find_operations" => serde_json::from_value::<FindOperationsArgs>(args)
                .map(|a| find_operations(&ctx, a))
                .map_err(|e| ApiError::bad_request(format!("Invalid arguments: {e}"))),
            "call_operation" => match serde_json::from_value::<CallOperationArgs>(args) {
                Ok(a) => call_operation(ctx, a).await,
                Err(e) => Err(ApiError::bad_request(format!("Invalid arguments: {e}"))),
            },
            name => match ops::find(name) {
                Some(op) => op.call(ctx, args).await,
                None => return Err(McpError::invalid_params(format!("Unknown tool {name}"), None)),
            },
        };
        let result = match outcome {
            Ok(value) => {
                CallToolResult::success(vec![ContentBlock::text(serde_json::to_string(&value).unwrap_or_default())])
            }
            Err(e) => CallToolResult::error(vec![ContentBlock::text(e.message)]),
        };
        Ok(result.into())
    }

    fn get_tool(&self, name: &str) -> Option<Tool> {
        ops::find(name).map(tool).or_else(|| meta_tools().into_iter().find(|t| t.name == name))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn core_tools_exist() {
        for name in CORE {
            assert!(ops::find(name).is_some(), "{name} is not an operation");
        }
    }
}
