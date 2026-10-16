//! Templates: what new issues, projects and documents start from, for the workspace or one team.

use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Value, json};
use sqlx::{PgConnection, Postgres, QueryBuilder};
use uuid::Uuid;

use super::events;
use super::issues::SaveIssueArgs;
use super::resolve::{self, as_uuid};
use super::util::{Page, non_empty, nullable};
use super::{Op, op};
use crate::ctx::{Access, Ctx};
use crate::error::{ApiError, ApiResult};

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "list_templates",
            Access::Read,
            &[("GET", "/templates")],
            "List the issue, project and document templates (without their data; read one with get_template). \
             save_issue, save_project and save_document take a template's name or id.",
            list,
        ),
        op(
            "get_template",
            Access::Read,
            &[("GET", "/templates/{id}")],
            "Read a template by id or name (with type when names repeat across types), including the data it fills in.",
            get,
        ),
        op(
            "save_template",
            Access::Write,
            &[("POST", "/templates"), ("PATCH", "/templates/{id}")],
            "Create a template (omit id; name, type and data are required) or update one (pass id). data holds what \
             it fills in, by type. issue: title, description, team, state, priority, estimate, assignee, labels, \
             project, cycle, dueDate and subIssues (a list of objects with save_issue's fields). project: name, \
             summary, description, state, priority, lead, labels, teams, milestones ([{name, description, \
             targetDate}]) and issues (save_issue objects). document: title, content.",
            save,
        ),
        op(
            "delete_template",
            Access::Write,
            &[("DELETE", "/templates/{id}")],
            "Delete a template. Things made from it keep what it filled in.",
            delete,
        ),
    ]
}

const TEMPLATE_JSON: &str = "jsonb_build_object(
    'id', tp.id,
    'name', tp.name,
    'description', tp.description,
    'type', tp.type,
    'team', team_ref(tp.team_id),
    'creator', user_ref(tp.creator_id),
    'createdAt', tp.created_at,
    'updatedAt', tp.updated_at
)";

async fn template_value(db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    let sql =
        format!("SELECT {TEMPLATE_JSON} || jsonb_build_object('data', tp.data) FROM templates tp WHERE tp.id = $1");
    let value: Option<Value> = sqlx::query_scalar(&sql).bind(id).fetch_optional(&mut *db).await?;
    value.ok_or_else(|| ApiError::not_found("No such template"))
}

fn check_type(kind: &str) -> ApiResult<&str> {
    match kind {
        "issue" | "project" | "document" => Ok(kind),
        other => Err(ApiError::bad_request(format!("type \"{other}\" isn't one of issue, project, document"))),
    }
}

/// A template the caller can see by id or name, of `kind` when given.
async fn find(ctx: &Ctx, kind: Option<&str>, value: &str) -> ApiResult<Uuid> {
    if let Some(kind) = kind {
        return resolve::template(ctx, check_type(kind)?, value).await;
    }
    let teams = ctx.team_ids().await?.to_vec();
    let rows: Vec<(Uuid, String)> = sqlx::query_as(
        "SELECT id, type FROM templates WHERE workspace_id = $1 AND (team_id IS NULL OR team_id = ANY($2))
         AND (id = $3 OR lower(name) = lower($4)) ORDER BY id = $3 DESC",
    )
    .bind(ctx.workspace)
    .bind(&teams)
    .bind(as_uuid(value))
    .bind(value.trim())
    .fetch_all(ctx.db())
    .await?;
    match rows.as_slice() {
        [] => Err(ApiError::not_found(format!("No template \"{}\"; list_templates shows them", value.trim()))),
        [(id, _)] => Ok(*id),
        [(id, _), ..] if as_uuid(value) == Some(*id) => Ok(*id),
        many => {
            let types: Vec<&str> = many.iter().map(|(_, t)| t.as_str()).collect();
            Err(ApiError::bad_request(format!(
                "Several templates are named \"{}\" ({}); pass type too",
                value.trim(),
                types.join(", ")
            )))
        }
    }
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ListTemplatesArgs {
    /// issue, project or document.
    #[serde(rename = "type")]
    kind: Option<String>,
    /// Team key, name or id: the team's templates and the workspace-wide ones.
    team: Option<String>,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list(ctx: Ctx, args: ListTemplatesArgs) -> ApiResult<Value> {
    let page = Page { limit: args.limit, cursor: args.cursor };
    let teams = ctx.team_ids().await?.to_vec();
    let mut qb: QueryBuilder<Postgres> =
        QueryBuilder::new(format!("SELECT {TEMPLATE_JSON} FROM templates tp WHERE tp.workspace_id = "));
    qb.push_bind(ctx.workspace);
    qb.push(" AND (tp.team_id IS NULL OR tp.team_id = ANY(");
    qb.push_bind(teams);
    qb.push("))");
    if let Some(kind) = &args.kind {
        qb.push(" AND tp.type = ");
        qb.push_bind(check_type(kind)?.to_string());
    }
    if let Some(team) = &args.team {
        qb.push(" AND (tp.team_id IS NULL OR tp.team_id = ");
        qb.push_bind(resolve::team(&ctx, team).await?);
        qb.push(")");
    }
    qb.push(" ORDER BY tp.type, lower(tp.name) LIMIT ");
    qb.push_bind(page.limit() + 1);
    qb.push(" OFFSET ");
    qb.push_bind(page.offset()?);
    let nodes: Vec<Value> = qb.build_query_scalar().fetch_all(ctx.db()).await?;
    page.wrap(nodes)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct GetTemplateArgs {
    /// The template's id or name.
    id: String,
    /// issue, project or document, when templates of several types share the name.
    #[serde(rename = "type")]
    kind: Option<String>,
}

async fn get(ctx: Ctx, args: GetTemplateArgs) -> ApiResult<Value> {
    let id = find(&ctx, args.kind.as_deref(), &args.id).await?;
    let mut db = ctx.db().acquire().await?;
    template_value(&mut db, id).await
}

/* ------------------------------------------------------------------ */
/*                                Save                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
#[allow(dead_code)]
struct IssueData {
    title: Option<String>,
    description: Option<String>,
    team: Option<String>,
    state: Option<String>,
    priority: Option<i32>,
    estimate: Option<f64>,
    assignee: Option<String>,
    labels: Option<Vec<String>>,
    project: Option<String>,
    cycle: Option<String>,
    due_date: Option<String>,
    sub_issues: Option<Vec<SaveIssueArgs>>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
#[allow(dead_code)]
struct MilestoneData {
    name: String,
    description: Option<String>,
    target_date: Option<String>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
#[allow(dead_code)]
struct ProjectData {
    name: Option<String>,
    summary: Option<String>,
    description: Option<String>,
    state: Option<String>,
    priority: Option<i32>,
    lead: Option<String>,
    labels: Option<Vec<String>>,
    teams: Option<Vec<String>>,
    milestones: Option<Vec<MilestoneData>>,
    issues: Option<Vec<SaveIssueArgs>>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
#[allow(dead_code)]
struct DocumentData {
    title: Option<String>,
    content: Option<String>,
}

/// Checks that `data` only holds what a template of `kind` fills in.
fn check_data(kind: &str, data: &Value) -> ApiResult<()> {
    if !data.is_object() {
        return Err(ApiError::bad_request("data must be an object"));
    }
    let parsed = match kind {
        "issue" => serde_json::from_value::<IssueData>(data.clone()).map(drop),
        "project" => serde_json::from_value::<ProjectData>(data.clone()).map(drop),
        _ => serde_json::from_value::<DocumentData>(data.clone()).map(drop),
    };
    parsed.map_err(|e| ApiError::bad_request(format!("Invalid {kind} template data: {e}")))
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SaveTemplateArgs {
    /// Only to update a template: its id or name. Omit to create one.
    id: Option<String>,
    name: Option<String>,
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    description: Option<Option<String>>,
    /// issue, project or document. Required to create; can't change.
    #[serde(rename = "type")]
    kind: Option<String>,
    /// Team key, name or id the template belongs to; null makes it workspace-wide.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    team: Option<Option<String>>,
    /// What the template fills in; the keys depend on the type (see the description).
    data: Option<Value>,
}

async fn save(ctx: Ctx, args: SaveTemplateArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let team = match &args.team {
        Some(Some(team)) => Some(Some(resolve::team(&ctx, team).await?)),
        Some(None) => Some(None),
        None => None,
    };
    let mut tx = ctx.db().begin().await?;
    let (id, action) = match &args.id {
        Some(reference) => {
            let id = find(&ctx, args.kind.as_deref(), reference).await?;
            (id, "update")
        }
        None => {
            let Some(kind) = &args.kind else {
                return Err(ApiError::bad_request("type is required: issue, project or document"));
            };
            let kind = check_type(kind)?;
            let name = non_empty(args.name.as_deref().unwrap_or(""), "name")?;
            if args.data.is_none() {
                return Err(ApiError::bad_request("data is required: what the template fills in"));
            }
            let id: Uuid = sqlx::query_scalar(
                "INSERT INTO templates (workspace_id, type, name, creator_id) VALUES ($1, $2, $3, $4) RETURNING id",
            )
            .bind(ctx.workspace)
            .bind(kind)
            .bind(&name)
            .bind(ctx.user)
            .fetch_one(&mut *tx)
            .await?;
            (id, "create")
        }
    };
    let kind: String =
        sqlx::query_scalar("SELECT type FROM templates WHERE id = $1 FOR UPDATE").bind(id).fetch_one(&mut *tx).await?;
    if let Some(name) = &args.name {
        let name = non_empty(name, "name")?;
        sqlx::query("UPDATE templates SET name = $2 WHERE id = $1").bind(id).bind(name).execute(&mut *tx).await?;
    }
    if let Some(description) = &args.description {
        let description = description.as_deref().map(str::trim).filter(|d| !d.is_empty());
        sqlx::query("UPDATE templates SET description = $2 WHERE id = $1")
            .bind(id)
            .bind(description)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(team) = team {
        sqlx::query("UPDATE templates SET team_id = $2 WHERE id = $1").bind(id).bind(team).execute(&mut *tx).await?;
    }
    if let Some(data) = &args.data {
        check_data(&kind, data)?;
        sqlx::query("UPDATE templates SET data = $2 WHERE id = $1").bind(id).bind(data).execute(&mut *tx).await?;
    }
    sqlx::query("UPDATE templates SET updated_at = now() WHERE id = $1").bind(id).execute(&mut *tx).await?;
    let value = template_value(&mut tx, id).await?;
    let team = value["team"]["id"].as_str().and_then(as_uuid);
    events::emit(&mut tx, &ctx, "Template", action, &value, None, team).await?;
    tx.commit().await?;
    Ok(value)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DeleteTemplateArgs {
    /// The template's id or name.
    id: String,
    /// issue, project or document, when templates of several types share the name.
    #[serde(rename = "type")]
    kind: Option<String>,
}

async fn delete(ctx: Ctx, args: DeleteTemplateArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let id = find(&ctx, args.kind.as_deref(), &args.id).await?;
    let mut tx = ctx.db().begin().await?;
    let value = template_value(&mut tx, id).await?;
    sqlx::query("DELETE FROM templates WHERE id = $1").bind(id).execute(&mut *tx).await?;
    let team = value["team"]["id"].as_str().and_then(as_uuid);
    events::emit(&mut tx, &ctx, "Template", "remove", &value, None, team).await?;
    tx.commit().await?;
    Ok(json!({ "deleted": true, "id": id, "name": value["name"] }))
}
