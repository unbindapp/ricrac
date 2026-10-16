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
use crate::ops;

pub const INSTRUCTIONS: &str = "Ricrac is an issue tracker for agents, with Linear's model: a workspace has teams; \
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
list_teams to learn the teams and their keys, and list_issue_statuses for a team's workflow.";

#[derive(Clone, Default)]
pub struct Mcp;

fn caller(ctx: &RequestContext<RoleServer>) -> Result<Ctx, McpError> {
    let parts = ctx
        .extensions
        .get::<axum::http::request::Parts>()
        .ok_or_else(|| McpError::internal_error("missing request", None))?;
    parts.extensions.get::<Ctx>().cloned().ok_or_else(|| McpError::invalid_request("not authenticated", None))
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
        let tools = ops::all().iter().filter(|op| visible(op, &ctx)).map(tool).collect();
        Ok(ListToolsResult::with_all_items(tools))
    }

    async fn call_tool(
        &self,
        request: CallToolRequestParams,
        context: RequestContext<RoleServer>,
    ) -> Result<CallToolResponse, McpError> {
        let ctx = caller(&context)?;
        let Some(op) = ops::find(&request.name) else {
            return Err(McpError::invalid_params(format!("Unknown tool {}", request.name), None));
        };
        let args = request.arguments.map(Value::Object).unwrap_or(Value::Null);
        let result = match op.call(ctx, args).await {
            Ok(value) => {
                CallToolResult::success(vec![ContentBlock::text(serde_json::to_string(&value).unwrap_or_default())])
            }
            Err(e) => CallToolResult::error(vec![ContentBlock::text(e.message)]),
        };
        Ok(result.into())
    }

    fn get_tool(&self, name: &str) -> Option<Tool> {
        ops::find(name).map(tool)
    }
}
