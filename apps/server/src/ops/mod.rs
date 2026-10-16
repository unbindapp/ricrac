//! Every operation of the tracker, defined once. The REST API (`/v1`), the MCP tools and the
//! OpenAPI document are all generated from this list.

use std::future::Future;
use std::pin::Pin;
use std::sync::{Arc, LazyLock};

use schemars::JsonSchema;
use schemars::generate::SchemaSettings;
use serde::de::DeserializeOwned;
use serde_json::Value;

use crate::ctx::{Access, Caller, Ctx};
use crate::error::{ApiError, ApiResult};

pub mod attachments;
pub mod comments;
pub mod customers;
pub mod cycles;
pub mod documents;
pub mod events;
pub mod favorites;
pub mod filter;
pub mod initiatives;
pub mod issues;
pub mod labels;
pub mod milestones;
pub mod notifications;
pub mod projects;
pub mod releases;
pub mod resolve;
pub mod search;
pub mod states;
pub mod status_updates;
pub mod teams;
pub mod templates;
pub mod util;
pub mod views;
pub mod webhooks;
pub mod workspace;

type Running = Pin<Box<dyn Future<Output = ApiResult<Value>> + Send>>;

enum Run {
    /// Runs in one workspace.
    Workspace(Arc<dyn Fn(Ctx, Value) -> Running + Send + Sync>),
    /// Runs for the caller across the workspaces they reach.
    Account(Arc<dyn Fn(Caller, Value) -> Running + Send + Sync>),
}

pub struct Op {
    /// The MCP tool name, e.g. `save_issue`.
    pub name: &'static str,
    pub description: &'static str,
    pub access: Access,
    /// REST routes under `/v1`, e.g. `("PATCH", "/issues/{id}")`. Path parameters are passed in
    /// the arguments under the same name.
    pub routes: &'static [(&'static str, &'static str)],
    /// JSON Schema of the arguments.
    pub schema: Value,
    run: Run,
}

const WORKSPACE_ARG: &str = "The workspace to act in, by URL key (see list_workspaces) or id.";

impl Op {
    pub async fn call(&self, ctx: Ctx, args: Value) -> ApiResult<Value> {
        ctx.require(self.access)?;
        match &self.run {
            Run::Workspace(run) => run(ctx, args).await,
            Run::Account(run) => run(ctx.caller(), args).await,
        }
    }

    /// Runs the operation for a caller of the API or MCP, in the workspace named by the
    /// `workspace` argument, or else `fallback` (see `Caller::ctx`).
    pub async fn call_as(&self, caller: &Caller, mut args: Value, fallback: Option<&str>) -> ApiResult<Value> {
        let named = take_workspace(&mut args)?;
        match &self.run {
            Run::Account(run) => {
                caller.require(self.access)?;
                run(caller.clone(), args).await
            }
            Run::Workspace(_) => self.call(caller.ctx(named.as_deref().or(fallback), &args).await?, args).await,
        }
    }

    /// Whether the operation changes nothing.
    pub fn is_read_only(&self) -> bool {
        self.access == Access::Read
    }

    pub fn runs_in_workspace(&self) -> bool {
        matches!(self.run, Run::Workspace(_))
    }

    /// The schema with the `workspace` argument, required for tokens that reach several workspaces.
    pub fn schema_with_workspace(&self, required: bool) -> Value {
        let mut schema = self.schema.clone();
        if !self.runs_in_workspace() {
            return schema;
        }
        let description = if required {
            format!("{WORKSPACE_ARG} Required: this token reaches several workspaces.")
        } else {
            format!("{WORKSPACE_ARG} Needed when the token reaches several workspaces.")
        };
        schema["properties"]["workspace"] = serde_json::json!({ "type": "string", "description": description });
        if required {
            match schema.get_mut("required").and_then(Value::as_array_mut) {
                Some(list) => list.insert(0, "workspace".into()),
                None => schema["required"] = serde_json::json!(["workspace"]),
            }
        }
        schema
    }
}

/// Removes the `workspace` argument, which every operation takes besides its own.
fn take_workspace(args: &mut Value) -> ApiResult<Option<String>> {
    let Some(object) = args.as_object_mut() else { return Ok(None) };
    match object.remove("workspace") {
        None | Some(Value::Null) => Ok(None),
        Some(Value::String(key)) => Ok(Some(key)),
        Some(Value::Number(key)) => Ok(Some(key.to_string())),
        Some(_) => Err(ApiError::bad_request("workspace is a URL key or id")),
    }
}

fn parse<A: DeserializeOwned>(args: Value) -> ApiResult<A> {
    let args = if args.is_null() { Value::Object(Default::default()) } else { args };
    serde_json::from_value(args).map_err(|e| ApiError::bad_request(format!("Invalid arguments: {e}")))
}

/// Defines an operation from an async function taking the context and its arguments.
pub fn op<A, F, Fut>(
    name: &'static str,
    access: Access,
    routes: &'static [(&'static str, &'static str)],
    description: &'static str,
    handler: F,
) -> Op
where
    A: DeserializeOwned + JsonSchema + Send + 'static,
    F: Fn(Ctx, A) -> Fut + Send + Sync + Copy + 'static,
    Fut: Future<Output = ApiResult<Value>> + Send + 'static,
{
    let run = move |ctx: Ctx, args: Value| -> Running { Box::pin(async move { handler(ctx, parse(args)?).await }) };
    Op { name, description, access, routes, schema: schema_of::<A>(), run: Run::Workspace(Arc::new(run)) }
}

/// Defines an operation that isn't about one workspace, like listing the ones the caller reaches.
pub fn account_op<A, F, Fut>(
    name: &'static str,
    access: Access,
    routes: &'static [(&'static str, &'static str)],
    description: &'static str,
    handler: F,
) -> Op
where
    A: DeserializeOwned + JsonSchema + Send + 'static,
    F: Fn(Caller, A) -> Fut + Send + Sync + Copy + 'static,
    Fut: Future<Output = ApiResult<Value>> + Send + 'static,
{
    let run =
        move |caller: Caller, args: Value| -> Running { Box::pin(async move { handler(caller, parse(args)?).await }) };
    Op { name, description, access, routes, schema: schema_of::<A>(), run: Run::Account(Arc::new(run)) }
}

/// Inlined, so that clients which don't follow `$ref`s still see every field.
pub fn schema_of<A: JsonSchema>() -> Value {
    let generator = SchemaSettings::draft2020_12()
        .with(|s| {
            s.inline_subschemas = true;
            s.meta_schema = None;
        })
        .into_generator();
    let mut schema = generator.into_root_schema_for::<A>().to_value();
    if let Some(object) = schema.as_object_mut() {
        object.remove("title");
        object.entry("properties").or_insert_with(|| Value::Object(Default::default()));
    }
    schema
}

static OPS: LazyLock<Vec<Op>> = LazyLock::new(|| {
    [
        workspace::ops(),
        teams::ops(),
        states::ops(),
        labels::ops(),
        issues::ops(),
        comments::ops(),
        attachments::ops(),
        projects::ops(),
        milestones::ops(),
        status_updates::ops(),
        initiatives::ops(),
        cycles::ops(),
        documents::ops(),
        views::ops(),
        favorites::ops(),
        templates::ops(),
        notifications::ops(),
        search::ops(),
        customers::ops(),
        releases::ops(),
        webhooks::ops(),
    ]
    .into_iter()
    .flatten()
    .collect()
});

pub fn all() -> &'static [Op] {
    &OPS
}

pub fn find(name: &str) -> Option<&'static Op> {
    OPS.iter().find(|op| op.name == name)
}

/// Arguments with no fields.
#[derive(serde::Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct NoArgs {}

/// Arguments naming one thing by its id (or another reference the operation accepts).
#[derive(serde::Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct IdArgs {
    pub id: String,
}

#[cfg(test)]
mod tests {
    use std::collections::HashSet;

    use super::*;

    #[test]
    fn names_and_routes_are_unique() {
        let mut names = HashSet::new();
        let mut routes = HashSet::new();
        for op in all() {
            assert!(names.insert(op.name), "two operations are named {}", op.name);
            assert!(op.name.len() <= 64 && op.name.bytes().all(|b| b.is_ascii_lowercase() || b == b'_'), "{}", op.name);
            for route in op.routes {
                assert!(routes.insert(*route), "two operations answer {route:?}");
            }
        }
    }

    #[test]
    fn schemas_are_objects_with_path_parameters() {
        for op in all() {
            assert_eq!(op.schema["type"], "object", "{}", op.name);
            for (_, path) in op.routes {
                for param in path.split('/').filter_map(|s| s.strip_prefix('{')?.strip_suffix('}')) {
                    assert!(
                        op.schema["properties"].get(param).is_some(),
                        "{} has no argument for path parameter {param}",
                        op.name
                    );
                }
            }
        }
    }
}
