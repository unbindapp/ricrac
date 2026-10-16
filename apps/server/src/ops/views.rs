//! Custom views: saved filters of issues (or projects, initiatives), personal or shared with the
//! workspace, optionally for one team.

use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Value, json};
use sqlx::{PgConnection, Postgres, QueryBuilder};
use uuid::Uuid;

use super::events;
use super::filter::{self, FILTER_HELP, escape_like};
use super::resolve;
use super::util::{Page, check_color, non_empty, nullable, slug_id};
use super::{IdArgs, Op, op};
use crate::ctx::{Access, Ctx, Role};
use crate::error::{ApiError, ApiResult};

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "list_custom_views",
            Access::Read,
            &[("GET", "/views")],
            "List the custom views the caller can use: those shared with the workspace and their own. Issue views \
             apply to list_issues with customView.",
            list,
        ),
        op(
            "get_custom_view",
            Access::Read,
            &[("GET", "/views/{id}")],
            "Read a custom view by id, slug, link or name: its filter and display settings.",
            get,
        ),
        op(
            "save_custom_view",
            Access::Write,
            &[("POST", "/views"), ("PATCH", "/views/{id}")],
            "Create a custom view (omit id; name and filter are required) or update one (pass id). Issue views take \
             the filter of list_issues, which is checked when saving. Views are personal unless shared; only their \
             owner (or an admin, for shared views) can change them.",
            save,
        ),
        op(
            "delete_custom_view",
            Access::Write,
            &[("DELETE", "/views/{id}")],
            "Delete a custom view. Only its owner (or an admin, for a shared view) can.",
            delete,
        ),
    ]
}

const VIEW_FROM: &str = "FROM custom_views v JOIN workspaces w ON w.id = v.workspace_id";

/// The JSON of the view `v` (with its workspace `w` joined).
fn view_json(ctx: &Ctx) -> String {
    let base = format!("'{}'", ctx.public_url().replace('\'', "''"));
    format!(
        "jsonb_build_object(
            'id', v.id,
            'slugId', v.slug_id,
            'name', v.name,
            'description', v.description,
            'icon', v.icon,
            'color', v.color,
            'model', v.model,
            'filter', v.filter,
            'display', v.display,
            'shared', v.shared,
            'team', team_ref(v.team_id),
            'owner', user_ref(v.owner_id),
            'url', {base} || '/' || w.url_key || '/view/' || concat_ws('-', nullif(trim(both '-' from left(
                trim(both '-' from regexp_replace(lower(v.name), '[^a-z0-9]+', '-', 'g')), 60)), ''), v.slug_id),
            'createdAt', v.created_at,
            'updatedAt', v.updated_at
        )"
    )
}

async fn view_value(ctx: &Ctx, db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    let sql = format!("SELECT {} {VIEW_FROM} WHERE v.id = $1", view_json(ctx));
    let value: Option<Value> = sqlx::query_scalar(&sql).bind(id).fetch_optional(&mut *db).await?;
    value.ok_or_else(|| ApiError::not_found("No such view"))
}

/// A view the caller can use, by id, slug, link or name: shared or theirs, and of a team they see.
pub async fn find(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    let id = resolve::custom_view(ctx, value).await?;
    let team: Option<Uuid> =
        sqlx::query_scalar("SELECT team_id FROM custom_views WHERE id = $1").bind(id).fetch_one(ctx.db()).await?;
    if let Some(team) = team
        && !ctx.can_see_team(team).await?
    {
        return Err(ApiError::not_found(format!("No view \"{}\"", value.trim())));
    }
    Ok(id)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ListViewsArgs {
    /// Words in the name or description.
    query: Option<String>,
    /// issue, project or initiative.
    model: Option<String>,
    /// Team key, name or id: that team's views.
    team: Option<String>,
    /// true: only views shared with the workspace; false: only the caller's personal views.
    shared: Option<bool>,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list(ctx: Ctx, args: ListViewsArgs) -> ApiResult<Value> {
    let page = Page { limit: args.limit, cursor: args.cursor };
    let teams = ctx.team_ids().await?.to_vec();
    let mut qb: QueryBuilder<Postgres> =
        QueryBuilder::new(format!("SELECT {} {VIEW_FROM} WHERE v.workspace_id = ", view_json(&ctx)));
    qb.push_bind(ctx.workspace);
    qb.push(" AND (v.shared OR v.owner_id = ");
    qb.push_bind(ctx.user);
    qb.push(") AND (v.team_id IS NULL OR v.team_id = ANY(");
    qb.push_bind(teams);
    qb.push("))");
    if let Some(model) = &args.model {
        qb.push(" AND v.model = ");
        qb.push_bind(check_model(model)?);
    }
    if let Some(team) = &args.team {
        qb.push(" AND v.team_id = ");
        qb.push_bind(resolve::team(&ctx, team).await?);
    }
    match args.shared {
        Some(true) => {
            qb.push(" AND v.shared");
        }
        Some(false) => {
            qb.push(" AND NOT v.shared AND v.owner_id = ");
            qb.push_bind(ctx.user);
        }
        None => {}
    }
    if let Some(query) = args.query.as_deref().map(str::trim).filter(|q| !q.is_empty()) {
        let like = format!("%{}%", escape_like(query));
        qb.push(" AND (v.name ILIKE ");
        qb.push_bind(like.clone());
        qb.push(" OR v.description ILIKE ");
        qb.push_bind(like);
        qb.push(")");
    }
    qb.push(" ORDER BY lower(v.name), v.created_at LIMIT ");
    qb.push_bind(page.limit() + 1);
    qb.push(" OFFSET ");
    qb.push_bind(page.offset()?);
    let nodes: Vec<Value> = qb.build_query_scalar().fetch_all(ctx.db()).await?;
    page.wrap(nodes)
}

async fn get(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = find(&ctx, &args.id).await?;
    let mut db = ctx.db().acquire().await?;
    view_value(&ctx, &mut db, id).await
}

fn check_model(model: &str) -> ApiResult<String> {
    match model {
        "issue" | "project" | "initiative" => Ok(model.to_string()),
        other => Err(ApiError::bad_request(format!("model \"{other}\" isn't one of issue, project, initiative"))),
    }
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SaveViewArgs {
    /// Only to update a view: its id, slug, link or name. Omit to create one.
    id: Option<String>,
    name: Option<String>,
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    description: Option<Option<String>>,
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    icon: Option<Option<String>>,
    /// Hex color like #5e6ad2; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    color: Option<Option<String>>,
    /// What the view lists: issue (default), project or initiative. Only when creating.
    model: Option<String>,
    #[schemars(description = "The view's filter object. For issue views, the filter language of list_issues: \
        Fields: id, number, title, description, priority, estimate, dueDate, createdAt, updatedAt, startedAt, completedAt, \
        canceledAt, archivedAt, state {id,name,type}, team {id,key,name}, assignee / creator {id,email,name,displayName,isMe}, \
        labels {some|every|none: {id,name,parent}}, project {id,name,slugId}, cycle {id,number,name,isActive}, and, or. \
        Comparators: eq, neq, in, nin, lt, lte, gt, gte, contains, startsWith, endsWith, null. \
        Example: {\"assignee\": {\"isMe\": {\"eq\": true}}, \"state\": {\"type\": {\"in\": [\"started\", \"unstarted\"]}}}")]
    filter: Option<Value>,
    /// How to show it, e.g. {"groupBy": "state", "orderBy": "priority"}.
    display: Option<Value>,
    /// Team key, name or id the view belongs to (its issues are that team's); null makes it workspace-wide.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    team: Option<Option<String>>,
    /// Share the view with the workspace (default false: only the caller sees it).
    shared: Option<bool>,
}

/// Compiles and runs the filter, so that mistakes in it surface now rather than when the view is used.
async fn check_issue_filter(ctx: &Ctx, db: &mut PgConnection, filter: &Value) -> ApiResult<()> {
    let mut qb: QueryBuilder<Postgres> = QueryBuilder::new("SELECT 1 FROM issues i WHERE FALSE");
    filter::push(&mut qb, ctx, "i", filter)?;
    qb.push(" LIMIT 0");
    qb.build().execute(&mut *db).await.map_err(|e| match e.as_database_error() {
        Some(db) => ApiError::bad_request(format!("The filter doesn't work: {}. {FILTER_HELP}", db.message())),
        None => e.into(),
    })?;
    Ok(())
}

fn check_object(value: &Value, what: &str) -> ApiResult<()> {
    if !value.is_object() {
        return Err(ApiError::bad_request(format!("{what} must be an object")));
    }
    Ok(())
}

/// Only the owner changes a view; admins also change shared ones.
async fn check_can_change(ctx: &Ctx, id: Uuid) -> ApiResult<()> {
    let (owner, shared): (Option<Uuid>, bool) =
        sqlx::query_as("SELECT owner_id, shared FROM custom_views WHERE id = $1").bind(id).fetch_one(ctx.db()).await?;
    if owner == Some(ctx.user) || (shared && ctx.role >= Role::Admin) {
        return Ok(());
    }
    Err(ApiError::forbidden("Only the view's owner (or an admin, for a shared view) can change it"))
}

async fn save(ctx: Ctx, args: SaveViewArgs) -> ApiResult<Value> {
    if args.shared == Some(true) {
        ctx.require_member()?;
    }
    if let Some(display) = &args.display {
        check_object(display, "display")?;
    }
    let color = args.color.clone().map(|c| c.as_deref().map(check_color).transpose()).transpose()?;
    let team = match &args.team {
        Some(Some(team)) => Some(Some(resolve::team(&ctx, team).await?)),
        Some(None) => Some(None),
        None => None,
    };
    let mut tx = ctx.db().begin().await?;
    let (id, action) = match &args.id {
        Some(reference) => {
            let id = find(&ctx, reference).await?;
            check_can_change(&ctx, id).await?;
            if args.model.is_some() {
                return Err(ApiError::bad_request("A view's model can't change; create another view"));
            }
            (id, "update")
        }
        None => {
            let name = non_empty(args.name.as_deref().unwrap_or(""), "name")?;
            let model = check_model(args.model.as_deref().unwrap_or("issue"))?;
            let id: Uuid = sqlx::query_scalar(
                "INSERT INTO custom_views (workspace_id, owner_id, slug_id, name, model) VALUES ($1, $2, $3, $4, $5) RETURNING id",
            )
            .bind(ctx.workspace)
            .bind(ctx.user)
            .bind(slug_id())
            .bind(&name)
            .bind(&model)
            .fetch_one(&mut *tx)
            .await?;
            (id, "create")
        }
    };
    if let Some(name) = &args.name {
        let name = non_empty(name, "name")?;
        sqlx::query("UPDATE custom_views SET name = $2 WHERE id = $1").bind(id).bind(name).execute(&mut *tx).await?;
    }
    if let Some(filter) = &args.filter {
        check_object(filter, "filter")?;
        let model: String =
            sqlx::query_scalar("SELECT model FROM custom_views WHERE id = $1").bind(id).fetch_one(&mut *tx).await?;
        if model == "issue" {
            check_issue_filter(&ctx, &mut tx, filter).await?;
        }
        sqlx::query("UPDATE custom_views SET filter = $2 WHERE id = $1")
            .bind(id)
            .bind(filter)
            .execute(&mut *tx)
            .await?;
    }
    let text = |value: &Option<Option<String>>| {
        value.as_ref().map(|v| v.as_deref().map(str::trim).filter(|v| !v.is_empty()).map(String::from))
    };
    let columns = [("description", text(&args.description)), ("icon", text(&args.icon)), ("color", color)];
    for (column, value) in columns {
        if let Some(value) = value {
            sqlx::query(&format!("UPDATE custom_views SET {column} = $2 WHERE id = $1"))
                .bind(id)
                .bind(value)
                .execute(&mut *tx)
                .await?;
        }
    }
    if let Some(display) = &args.display {
        sqlx::query("UPDATE custom_views SET display = $2 WHERE id = $1")
            .bind(id)
            .bind(display)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(team) = team {
        sqlx::query("UPDATE custom_views SET team_id = $2 WHERE id = $1").bind(id).bind(team).execute(&mut *tx).await?;
    }
    if let Some(shared) = args.shared {
        sqlx::query("UPDATE custom_views SET shared = $2 WHERE id = $1")
            .bind(id)
            .bind(shared)
            .execute(&mut *tx)
            .await?;
    }
    sqlx::query("UPDATE custom_views SET updated_at = now() WHERE id = $1").bind(id).execute(&mut *tx).await?;
    let value = view_value(&ctx, &mut tx, id).await?;
    let team = value["team"]["id"].as_str().and_then(resolve::as_uuid);
    events::emit(&mut tx, &ctx, "CustomView", action, &value, None, team).await?;
    tx.commit().await?;
    Ok(value)
}

async fn delete(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = find(&ctx, &args.id).await?;
    check_can_change(&ctx, id).await?;
    let mut tx = ctx.db().begin().await?;
    let value = view_value(&ctx, &mut tx, id).await?;
    sqlx::query("DELETE FROM favorites WHERE type = 'custom_view' AND target_id = $1")
        .bind(id)
        .execute(&mut *tx)
        .await?;
    sqlx::query("DELETE FROM custom_views WHERE id = $1").bind(id).execute(&mut *tx).await?;
    let team = value["team"]["id"].as_str().and_then(resolve::as_uuid);
    events::emit(&mut tx, &ctx, "CustomView", "remove", &value, None, team).await?;
    tx.commit().await?;
    Ok(json!({ "deleted": true, "id": id, "name": value["name"] }))
}
