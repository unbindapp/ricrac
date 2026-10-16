//! Project milestones: the stages of a project, each with its issues' progress.

use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Value, json};
use sqlx::PgConnection;
use uuid::Uuid;

use super::events;
use super::projects::{is_null_word, webhook_team};
use super::resolve;
use super::util::{Page, non_empty, nullable, parse_date};
use super::{Op, op};
use crate::ctx::{Access, Ctx};
use crate::error::{ApiError, ApiResult};

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "list_milestones",
            Access::Read,
            &[("GET", "/projects/{project}/milestones")],
            "List a project's milestones in order, each with its issue counts and progress (completed issues / (all - \
             canceled)).",
            list,
        ),
        op(
            "get_milestone",
            Access::Read,
            &[("GET", "/milestones/{id}")],
            "Read a milestone by id (or by name with its project), with its progress and its issues.",
            get,
        ),
        op(
            "save_milestone",
            Access::Write,
            &[("POST", "/milestones"), ("PATCH", "/milestones/{id}")],
            "Create a milestone in a project (omit id; project and name are required) or change one (pass its id, or \
             its name with the project). Names are unique within a project. Put issues in it with save_issue \
             (milestone).",
            save,
        ),
        op(
            "delete_milestone",
            Access::Write,
            &[("DELETE", "/milestones/{id}")],
            "Delete a milestone. Its issues stay in the project, without a milestone.",
            delete,
        ),
    ]
}

/// The JSON of the milestone `m`, with its progress.
pub const MILESTONE_JSON: &str = "(jsonb_build_object('id', m.id, 'name', m.name, 'description', m.description,
    'targetDate', m.target_date, 'sortOrder', m.sort_order, 'project', project_ref(m.project_id),
    'createdAt', m.created_at, 'updatedAt', m.updated_at) || project_issue_stats(NULL, m.id))";

const ORDER: &str = "m.sort_order, m.target_date NULLS LAST, m.created_at";

/// A project's milestones in order, as a JSON array.
pub async fn project_milestones(db: &mut PgConnection, project: Uuid) -> ApiResult<Value> {
    Ok(sqlx::query_scalar(&format!(
        "SELECT coalesce(jsonb_agg({MILESTONE_JSON} ORDER BY {ORDER}), '[]') FROM project_milestones m WHERE m.project_id = $1"
    ))
    .bind(project)
    .fetch_one(&mut *db)
    .await?)
}

async fn milestone_value(db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    let value: Option<Value> =
        sqlx::query_scalar(&format!("SELECT {MILESTONE_JSON} FROM project_milestones m WHERE m.id = $1"))
            .bind(id)
            .fetch_optional(&mut *db)
            .await?;
    value.ok_or_else(|| ApiError::not_found("No such milestone"))
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ListArgs {
    /// Project name, slug, link or id.
    project: String,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list(ctx: Ctx, args: ListArgs) -> ApiResult<Value> {
    let project = resolve::project(&ctx, &args.project).await?;
    let page = Page { limit: args.limit, cursor: args.cursor };
    let nodes: Vec<Value> = sqlx::query_scalar(&format!(
        "SELECT {MILESTONE_JSON} FROM project_milestones m WHERE m.project_id = $1 ORDER BY {ORDER} LIMIT $2 OFFSET $3"
    ))
    .bind(project)
    .bind(page.limit() + 1)
    .bind(page.offset()?)
    .fetch_all(ctx.db())
    .await?;
    page.wrap(nodes)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct MilestoneArgs {
    /// The milestone's id, or its name together with project.
    id: String,
    /// Project name, slug or id; needed when id is a name.
    project: Option<String>,
}

async fn find(ctx: &Ctx, id: &str, project: Option<&str>) -> ApiResult<Uuid> {
    let project = match project {
        Some(project) => Some(resolve::project(ctx, project).await?),
        None => None,
    };
    resolve::milestone(ctx, project, id).await
}

async fn get(ctx: Ctx, args: MilestoneArgs) -> ApiResult<Value> {
    let id = find(&ctx, &args.id, args.project.as_deref()).await?;
    let mut db = ctx.db().acquire().await?;
    let mut value = milestone_value(&mut db, id).await?;
    let teams = ctx.team_ids().await?.to_vec();
    value["issues"] = sqlx::query_scalar(
        "SELECT coalesce(jsonb_agg(issue_ref(i.id) || jsonb_build_object('assignee', user_ref(i.assignee_id),
            'priority', i.priority) ORDER BY i.sort_order), '[]')
         FROM issues i WHERE i.milestone_id = $1 AND i.trashed_at IS NULL AND i.team_id = ANY($2)",
    )
    .bind(id)
    .bind(&teams)
    .fetch_one(&mut *db)
    .await?;
    Ok(value)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SaveArgs {
    /// Only to change a milestone: its id, or its name together with project. Omit to create one.
    id: Option<String>,
    /// Project name, slug or id. Required to create.
    project: Option<String>,
    /// Required to create; unique within the project.
    name: Option<String>,
    /// Markdown; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    description: Option<Option<String>>,
    /// YYYY-MM-DD; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    target_date: Option<Option<String>>,
    /// Position among the project's milestones (lower comes first; ties go by target date).
    sort_order: Option<f64>,
}

async fn save(ctx: Ctx, args: SaveArgs) -> ApiResult<Value> {
    let existing = match &args.id {
        Some(id) => Some(find(&ctx, id, args.project.as_deref()).await?),
        None => None,
    };
    let mut tx = ctx.db().begin().await?;
    let id = match existing {
        Some(id) => id,
        None => {
            let Some(project) = &args.project else {
                return Err(ApiError::bad_request("project is required to create a milestone"));
            };
            let project = resolve::project(&ctx, project).await?;
            let name = non_empty(args.name.as_deref().unwrap_or(""), "name")?;
            sqlx::query_scalar("INSERT INTO project_milestones (project_id, name) VALUES ($1, $2) RETURNING id")
                .bind(project)
                .bind(&name)
                .fetch_one(&mut *tx)
                .await?
        }
    };
    if let Some(name) = &args.name {
        let name = non_empty(name, "name")?;
        let taken: bool = sqlx::query_scalar(
            "SELECT EXISTS (SELECT 1 FROM project_milestones o JOIN project_milestones m ON m.id = $1
                WHERE o.project_id = m.project_id AND o.id <> m.id AND lower(o.name) = lower($2))",
        )
        .bind(id)
        .bind(&name)
        .fetch_one(&mut *tx)
        .await?;
        if taken {
            return Err(ApiError::conflict(format!("The project already has a milestone \"{name}\"")));
        }
        sqlx::query("UPDATE project_milestones SET name = $2 WHERE id = $1")
            .bind(id)
            .bind(&name)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(description) = &args.description {
        sqlx::query("UPDATE project_milestones SET description = $2 WHERE id = $1")
            .bind(id)
            .bind(description)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(target) = &args.target_date {
        let target = target.as_deref().filter(|d| !is_null_word(d)).map(parse_date).transpose()?;
        sqlx::query("UPDATE project_milestones SET target_date = $2 WHERE id = $1")
            .bind(id)
            .bind(target)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(order) = args.sort_order {
        sqlx::query("UPDATE project_milestones SET sort_order = $2 WHERE id = $1")
            .bind(id)
            .bind(order)
            .execute(&mut *tx)
            .await?;
    }
    sqlx::query("UPDATE project_milestones SET updated_at = now() WHERE id = $1").bind(id).execute(&mut *tx).await?;
    let value = milestone_value(&mut tx, id).await?;
    let project: Uuid = sqlx::query_scalar("SELECT project_id FROM project_milestones WHERE id = $1")
        .bind(id)
        .fetch_one(&mut *tx)
        .await?;
    let team = webhook_team(&mut tx, project).await?;
    let action = if existing.is_some() { "update" } else { "create" };
    events::emit(&mut tx, &ctx, "ProjectMilestone", action, &value, None, team).await?;
    tx.commit().await?;
    Ok(value)
}

async fn delete(ctx: Ctx, args: MilestoneArgs) -> ApiResult<Value> {
    let id = find(&ctx, &args.id, args.project.as_deref()).await?;
    let mut tx = ctx.db().begin().await?;
    let value = milestone_value(&mut tx, id).await?;
    let project: Uuid = sqlx::query_scalar("SELECT project_id FROM project_milestones WHERE id = $1")
        .bind(id)
        .fetch_one(&mut *tx)
        .await?;
    sqlx::query("DELETE FROM project_milestones WHERE id = $1").bind(id).execute(&mut *tx).await?;
    let team = webhook_team(&mut tx, project).await?;
    events::emit(&mut tx, &ctx, "ProjectMilestone", "remove", &value, None, team).await?;
    tx.commit().await?;
    Ok(json!({ "deleted": true, "id": id, "name": value["name"] }))
}
