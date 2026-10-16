//! Every operation of the tracker, defined once. The REST API (`/v1`), the MCP tools and the
//! OpenAPI document are all generated from this list.

use std::future::Future;
use std::pin::Pin;
use std::sync::{Arc, LazyLock};

use schemars::JsonSchema;
use schemars::generate::SchemaSettings;
use serde::de::DeserializeOwned;
use serde_json::Value;

use crate::ctx::{Access, Ctx};
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

type Runner = dyn Fn(Ctx, Value) -> Pin<Box<dyn Future<Output = ApiResult<Value>> + Send>> + Send + Sync;

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
    run: Arc<Runner>,
}

impl Op {
    pub async fn call(&self, ctx: Ctx, args: Value) -> ApiResult<Value> {
        ctx.require(self.access)?;
        (self.run)(ctx, args).await
    }

    /// Whether the operation changes nothing.
    pub fn is_read_only(&self) -> bool {
        self.access == Access::Read
    }
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
    let run = move |ctx: Ctx, args: Value| -> Pin<Box<dyn Future<Output = ApiResult<Value>> + Send>> {
        Box::pin(async move {
            let args = if args.is_null() { Value::Object(Default::default()) } else { args };
            let args: A =
                serde_json::from_value(args).map_err(|e| ApiError::bad_request(format!("Invalid arguments: {e}")))?;
            handler(ctx, args).await
        })
    };
    Op { name, description, access, routes, schema: schema_of::<A>(), run: Arc::new(run) }
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
