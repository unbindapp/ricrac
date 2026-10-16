//! Status updates on projects and initiatives: a written update with a health (on track, at
//! risk, off track). The newest update's health is the project's or initiative's health.

use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Value, json};
use sqlx::{PgConnection, Postgres, QueryBuilder};
use uuid::Uuid;

use super::events::{self, Notice};
use super::projects::{check_health, webhook_team};
use super::resolve::{self, as_uuid};
use super::util::{non_empty, parse_datetime};
use super::{IdArgs, Op, op};
use crate::ctx::{Access, Ctx, Role};
use crate::error::{ApiError, ApiResult};

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "get_status_updates",
            Access::Read,
            &[("GET", "/status-updates")],
            "Status updates, newest first: of one project or initiative, or, without either, the recent updates across \
             the projects you can see and the initiatives. Pass id to read one update.",
            list,
        ),
        op(
            "save_status_update",
            Access::Write,
            &[("POST", "/status-updates"), ("PATCH", "/status-updates/{id}")],
            "Post a status update on a project or initiative (omit id; body is required) with its health: onTrack, \
             atRisk or offTrack (by default the previous update's, else onTrack). The newest update's health becomes \
             the project's or initiative's. Members and subscribers of the project (or the initiative's owner) are \
             notified, and so are people @mentioned. Pass id to edit your own update.",
            save,
        ),
        op(
            "delete_status_update",
            Access::Write,
            &[("DELETE", "/status-updates/{id}")],
            "Delete a status update (yours, or any as a workspace admin). The health goes back to the newest remaining \
             update's.",
            delete,
        ),
    ]
}

/// The JSON of the status update `u`.
pub const UPDATE_JSON: &str = "jsonb_build_object('id', u.id,
    'type', CASE WHEN u.project_id IS NOT NULL THEN 'project' ELSE 'initiative' END,
    'project', project_ref(u.project_id), 'initiative', initiative_ref(u.initiative_id),
    'body', u.body, 'health', u.health, 'user', user_ref(u.user_id),
    'reactions', (SELECT coalesce(jsonb_agg(jsonb_build_object('emoji', r.emoji, 'user', user_ref(r.user_id))
        ORDER BY r.created_at), '[]') FROM reactions r WHERE r.status_update_id = u.id),
    'commentCount', (SELECT count(*) FROM comments c WHERE c.status_update_id = u.id),
    'createdAt', u.created_at, 'updatedAt', u.updated_at, 'editedAt', u.edited_at)";

/// The newest update on the thing whose id is in `column` (project_id or initiative_id).
pub async fn latest(db: &mut PgConnection, column: &str, id: Uuid) -> ApiResult<Option<Value>> {
    Ok(sqlx::query_scalar(&format!(
        "SELECT {UPDATE_JSON} FROM status_updates u WHERE u.{column} = $1 ORDER BY u.created_at DESC, u.id LIMIT 1"
    ))
    .bind(id)
    .fetch_optional(&mut *db)
    .await?)
}

async fn update_value(db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    Ok(sqlx::query_scalar(&format!("SELECT {UPDATE_JSON} FROM status_updates u WHERE u.id = $1"))
        .bind(id)
        .fetch_one(&mut *db)
        .await?)
}

/// Copies the newest update's health onto the project or initiative (null when none is left).
async fn sync_health(tx: &mut PgConnection, project: Option<Uuid>, initiative: Option<Uuid>) -> ApiResult<()> {
    let (table, column, id) = match (project, initiative) {
        (Some(project), _) => ("projects", "project_id", project),
        (None, Some(initiative)) => ("initiatives", "initiative_id", initiative),
        (None, None) => return Ok(()),
    };
    sqlx::query(&format!(
        "UPDATE {table} SET health = (SELECT health FROM status_updates WHERE {column} = $1
            ORDER BY created_at DESC, id LIMIT 1) WHERE id = $1"
    ))
    .bind(id)
    .execute(&mut *tx)
    .await?;
    Ok(())
}

/* ------------------------------------------------------------------ */
/*                                List                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ListArgs {
    /// One status update by id.
    id: Option<String>,
    /// Project name, slug, link or id.
    project: Option<String>,
    /// Initiative name, slug, link or id.
    initiative: Option<String>,
    /// project or initiative updates only.
    #[serde(rename = "type")]
    kind: Option<String>,
    /// Author (id, name, email or "me").
    user: Option<String>,
    /// Created after: ISO 8601 date/time or a duration like -P1W.
    created_at: Option<String>,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list(ctx: Ctx, args: ListArgs) -> ApiResult<Value> {
    let page = super::util::Page { limit: args.limit, cursor: args.cursor.clone() };
    let teams = ctx.team_ids().await?.to_vec();
    let mut qb: QueryBuilder<Postgres> =
        QueryBuilder::new(format!("SELECT {UPDATE_JSON} FROM status_updates u WHERE u.workspace_id = "));
    qb.push_bind(ctx.workspace);
    qb.push(
        " AND ((u.project_id IS NOT NULL AND EXISTS (SELECT 1 FROM projects p WHERE p.id = u.project_id
            AND p.trashed_at IS NULL AND EXISTS (SELECT 1 FROM project_teams pt WHERE pt.project_id = p.id AND pt.team_id = ANY(",
    );
    qb.push_bind(teams);
    qb.push(")))) OR (u.initiative_id IS NOT NULL AND ");
    qb.push_bind(ctx.role != Role::Guest);
    qb.push(" AND EXISTS (SELECT 1 FROM initiatives i WHERE i.id = u.initiative_id AND i.trashed_at IS NULL)))");
    if let Some(id) = &args.id {
        let id = as_uuid(id).ok_or_else(|| ApiError::not_found(format!("No status update \"{}\"", id.trim())))?;
        qb.push(" AND u.id = ");
        qb.push_bind(id);
    }
    if let Some(project) = &args.project {
        qb.push(" AND u.project_id = ");
        qb.push_bind(resolve::project(&ctx, project).await?);
    }
    if let Some(initiative) = &args.initiative {
        qb.push(" AND u.initiative_id = ");
        qb.push_bind(resolve::initiative(&ctx, initiative).await?);
    }
    match args.kind.as_deref() {
        None => {}
        Some("project") => {
            qb.push(" AND u.project_id IS NOT NULL");
        }
        Some("initiative") => {
            qb.push(" AND u.initiative_id IS NOT NULL");
        }
        Some(other) => return Err(ApiError::bad_request(format!("type is project or initiative, not \"{other}\""))),
    }
    if let Some(user) = &args.user {
        qb.push(" AND u.user_id = ");
        qb.push_bind(resolve::user(&ctx, user).await?);
    }
    if let Some(after) = &args.created_at {
        qb.push(" AND u.created_at >= ");
        qb.push_bind(parse_datetime(after)?);
    }
    qb.push(" ORDER BY u.created_at DESC, u.id LIMIT ");
    qb.push_bind(page.limit() + 1);
    qb.push(" OFFSET ");
    qb.push_bind(page.offset()?);
    let nodes: Vec<Value> = qb.build_query_scalar().fetch_all(ctx.db()).await?;
    page.wrap(nodes)
}

/* ------------------------------------------------------------------ */
/*                                Save                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SaveArgs {
    /// Only to edit an update: its id. Omit to post one.
    id: Option<String>,
    /// project or initiative; follows from which of them is given.
    #[serde(rename = "type")]
    kind: Option<String>,
    /// Project name, slug, link or id.
    project: Option<String>,
    /// Initiative name, slug, link or id.
    initiative: Option<String>,
    /// Markdown. Mention people with @displayName or @email; they are notified. Required to post.
    body: Option<String>,
    /// onTrack, atRisk or offTrack.
    health: Option<String>,
}

struct Target {
    project: Option<Uuid>,
    initiative: Option<Uuid>,
    name: String,
}

async fn target(ctx: &Ctx, args: &SaveArgs) -> ApiResult<Target> {
    match (&args.project, &args.initiative, args.kind.as_deref()) {
        (Some(_), Some(_), _) => Err(ApiError::bad_request("Pass either project or initiative, not both")),
        (Some(project), None, None | Some("project")) => {
            let id = resolve::project(ctx, project).await?;
            let name =
                sqlx::query_scalar("SELECT name FROM projects WHERE id = $1").bind(id).fetch_one(ctx.db()).await?;
            Ok(Target { project: Some(id), initiative: None, name })
        }
        (None, Some(initiative), None | Some("initiative")) => {
            ctx.require_member()?;
            let id = resolve::initiative(ctx, initiative).await?;
            let name =
                sqlx::query_scalar("SELECT name FROM initiatives WHERE id = $1").bind(id).fetch_one(ctx.db()).await?;
            Ok(Target { project: None, initiative: Some(id), name })
        }
        (None, None, _) => Err(ApiError::bad_request("Name the project or initiative the update is about")),
        (_, _, Some(kind)) => Err(ApiError::bad_request(format!(
            "type \"{kind}\" doesn't match; it is project with project, initiative with initiative"
        ))),
    }
}

async fn save(ctx: Ctx, args: SaveArgs) -> ApiResult<Value> {
    match args.id.clone() {
        Some(id) => edit(&ctx, &id, args).await,
        None => create(&ctx, args).await,
    }
}

async fn create(ctx: &Ctx, args: SaveArgs) -> ApiResult<Value> {
    let target = target(ctx, &args).await?;
    let body = non_empty(args.body.as_deref().unwrap_or(""), "body")?;
    let mut tx = ctx.db().begin().await?;
    let health = match &args.health {
        Some(health) => check_health(health)?,
        None => {
            let previous: Option<String> = sqlx::query_scalar(
                "SELECT health FROM status_updates WHERE project_id IS NOT DISTINCT FROM $1
                 AND initiative_id IS NOT DISTINCT FROM $2 ORDER BY created_at DESC, id LIMIT 1",
            )
            .bind(target.project)
            .bind(target.initiative)
            .fetch_optional(&mut *tx)
            .await?;
            previous.unwrap_or_else(|| "onTrack".into())
        }
    };
    let id: Uuid = sqlx::query_scalar(
        "INSERT INTO status_updates (workspace_id, project_id, initiative_id, user_id, body, health)
         VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
    )
    .bind(ctx.workspace)
    .bind(target.project)
    .bind(target.initiative)
    .bind(ctx.user)
    .bind(&body)
    .bind(&health)
    .fetch_one(&mut *tx)
    .await?;
    sync_health(&mut tx, target.project, target.initiative).await?;

    let mentioned = events::mentions(ctx, &body).await?;
    notify_mentions(ctx, &mut tx, &target, id, &mentioned).await?;
    let audience: Vec<Uuid> = match target.project {
        Some(project) => {
            sqlx::query_scalar(
                "SELECT user_id FROM project_members WHERE project_id = $1
                 UNION SELECT user_id FROM project_subscribers WHERE project_id = $1
                 UNION SELECT lead_id FROM projects WHERE id = $1 AND lead_id IS NOT NULL",
            )
            .bind(project)
            .fetch_all(&mut *tx)
            .await?
        }
        None => {
            sqlx::query_scalar("SELECT owner_id FROM initiatives WHERE id = $1 AND owner_id IS NOT NULL")
                .bind(target.initiative)
                .fetch_all(&mut *tx)
                .await?
        }
    };
    let audience: Vec<Uuid> = audience.into_iter().filter(|u| !mentioned.contains(u)).collect();
    let notice = Notice {
        kind: if target.project.is_some() { "projectUpdateCreated" } else { "initiativeUpdateCreated" },
        summary: format!("New update on {}", target.name),
        project: target.project,
        initiative: target.initiative,
        status_update: Some(id),
        ..Default::default()
    };
    events::notify(&mut tx, ctx, &audience, &notice).await?;

    let value = update_value(&mut tx, id).await?;
    emit(&mut tx, ctx, &target, "create", &value).await?;
    tx.commit().await?;
    Ok(value)
}

async fn notify_mentions(
    ctx: &Ctx,
    tx: &mut PgConnection,
    target: &Target,
    update: Uuid,
    users: &[Uuid],
) -> ApiResult<()> {
    if users.is_empty() {
        return Ok(());
    }
    let notice = Notice {
        kind: "projectUpdateMention",
        summary: format!("You were mentioned in an update on {}", target.name),
        project: target.project,
        initiative: target.initiative,
        status_update: Some(update),
        ..Default::default()
    };
    events::notify(tx, ctx, users, &notice).await
}

async fn emit(tx: &mut PgConnection, ctx: &Ctx, target: &Target, action: &str, value: &Value) -> ApiResult<()> {
    match target.project {
        Some(project) => {
            let team = webhook_team(tx, project).await?;
            events::emit(tx, ctx, "ProjectUpdate", action, value, None, team).await
        }
        None => events::emit(tx, ctx, "InitiativeUpdate", action, value, None, None).await,
    }
}

/// The update by id, if the caller can see what it is about.
async fn find(ctx: &Ctx, reference: &str) -> ApiResult<(Uuid, Option<Uuid>, Target)> {
    let not_found = || ApiError::not_found(format!("No status update \"{}\"", reference.trim()));
    let id = as_uuid(reference).ok_or_else(not_found)?;
    let row: Option<(Option<Uuid>, Option<Uuid>, Option<Uuid>)> = sqlx::query_as(
        "SELECT project_id, initiative_id, user_id FROM status_updates WHERE id = $1 AND workspace_id = $2",
    )
    .bind(id)
    .bind(ctx.workspace)
    .fetch_optional(ctx.db())
    .await?;
    let (project, initiative, author) = row.ok_or_else(not_found)?;
    let lookup = SaveArgs {
        id: None,
        kind: None,
        project: project.map(|p| p.to_string()),
        initiative: initiative.map(|i| i.to_string()),
        body: None,
        health: None,
    };
    let target = target(ctx, &lookup).await.map_err(|_| not_found())?;
    Ok((id, author, target))
}

async fn edit(ctx: &Ctx, reference: &str, args: SaveArgs) -> ApiResult<Value> {
    if args.project.is_some() || args.initiative.is_some() || args.kind.is_some() {
        return Err(ApiError::bad_request("An update stays on its project or initiative; pass only body and health"));
    }
    let (id, author, target) = find(ctx, reference).await?;
    if author != Some(ctx.user) {
        return Err(ApiError::forbidden("Only the author can edit a status update"));
    }
    let mut tx = ctx.db().begin().await?;
    let old_body: String = sqlx::query_scalar("SELECT body FROM status_updates WHERE id = $1 FOR UPDATE")
        .bind(id)
        .fetch_one(&mut *tx)
        .await?;
    if let Some(body) = &args.body {
        let body = non_empty(body, "body")?;
        sqlx::query("UPDATE status_updates SET body = $2 WHERE id = $1").bind(id).bind(&body).execute(&mut *tx).await?;
        let previous = events::mentions(ctx, &old_body).await?;
        let mentioned: Vec<Uuid> =
            events::mentions(ctx, &body).await?.into_iter().filter(|u| !previous.contains(u)).collect();
        notify_mentions(ctx, &mut tx, &target, id, &mentioned).await?;
    }
    if let Some(health) = &args.health {
        sqlx::query("UPDATE status_updates SET health = $2 WHERE id = $1")
            .bind(id)
            .bind(check_health(health)?)
            .execute(&mut *tx)
            .await?;
    }
    sqlx::query("UPDATE status_updates SET edited_at = now(), updated_at = now() WHERE id = $1")
        .bind(id)
        .execute(&mut *tx)
        .await?;
    sync_health(&mut tx, target.project, target.initiative).await?;
    let value = update_value(&mut tx, id).await?;
    emit(&mut tx, ctx, &target, "update", &value).await?;
    tx.commit().await?;
    Ok(value)
}

async fn delete(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let (id, author, target) = find(&ctx, &args.id).await?;
    if author != Some(ctx.user) && ctx.role < Role::Admin {
        return Err(ApiError::forbidden("Only the author or a workspace admin can delete a status update"));
    }
    let mut tx = ctx.db().begin().await?;
    let value = update_value(&mut tx, id).await?;
    sqlx::query("DELETE FROM status_updates WHERE id = $1").bind(id).execute(&mut *tx).await?;
    sync_health(&mut tx, target.project, target.initiative).await?;
    emit(&mut tx, &ctx, &target, "remove", &value).await?;
    tx.commit().await?;
    Ok(json!({ "deleted": true, "id": id }))
}
