//! Initiatives: company goals that group projects (and other initiatives), with their progress,
//! health and status. Guests can't see them.

use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Map, Value};
use sqlx::{PgConnection, Postgres, QueryBuilder};
use uuid::Uuid;

use super::events;
use super::filter::escape_like;
use super::issues::{LinkArg, changed_fields};
use super::projects::{find_trashed, is_null_word, link_initiatives, links_sql, save_links, sql_literal};
use super::resolve;
use super::util::{Page, TextEdit, check_color, new_content, non_empty, nullable, parse_date, slug_id};
use super::{IdArgs, Op, op};
use crate::ctx::{Access, Ctx};
use crate::error::{ApiError, ApiResult};

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "list_initiatives",
            Access::Read,
            &[("GET", "/initiatives")],
            "List initiatives, most recently updated first, each with its project count and progress (the average of \
             its projects'). Narrow with status (planned, active, completed), owner, parent or query. Archived ones are \
             left out unless includeArchived is set.",
            list,
        ),
        op(
            "get_initiative",
            Access::Read,
            &[("GET", "/initiatives/{id}")],
            "Read an initiative by id, slug, link or name: its fields and description, projects (with status, \
             progress, health and lead), sub-initiatives, latest status update, links and documents.",
            get,
        ),
        op(
            "save_initiative",
            Access::Write,
            &[("POST", "/initiatives"), ("PATCH", "/initiatives/{id}")],
            "Create an initiative (omit id; name is required) or update one (pass id). Omitted fields stay as they are; \
             null clears a field. Add projects with addProjects or setProjects; nest it under another initiative with \
             parent.",
            save,
        ),
        op(
            "delete_initiative",
            Access::Write,
            &[("DELETE", "/initiatives/{id}")],
            "Move an initiative to the trash; its projects stay. It is deleted for good after 30 days; \
             restore_initiative brings it back before then.",
            trash,
        ),
        op(
            "restore_initiative",
            Access::Write,
            &[("POST", "/initiatives/{id}/restore")],
            "Bring an initiative back from the trash.",
            restore,
        ),
        op(
            "archive_initiative",
            Access::Write,
            &[("POST", "/initiatives/{id}/archive")],
            "Archive an initiative. It stays readable but is left out of lists unless includeArchived is set.",
            archive,
        ),
        op(
            "unarchive_initiative",
            Access::Write,
            &[("POST", "/initiatives/{id}/unarchive")],
            "Bring an archived initiative back.",
            unarchive,
        ),
    ]
}

const INITIATIVE_FROM: &str = "FROM initiatives i JOIN workspaces w ON w.id = i.workspace_id";

/// The JSON of the initiative `i` (with its workspace `w` joined).
fn initiative_json(ctx: &Ctx, with_description: bool) -> String {
    let base = sql_literal(ctx.public_url());
    let description = if with_description { "'description', i.content," } else { "" };
    format!(
        "jsonb_build_object(
            'id', i.id,
            'slugId', i.slug_id,
            'name', i.name,
            'summary', i.summary,
            {description}
            'url', {base} || '/' || w.url_key || '/initiative/' || concat_ws('-', nullif(entity_slug(i.name), ''), i.slug_id),
            'icon', i.icon,
            'color', i.color,
            'status', i.status,
            'health', i.health,
            'owner', user_ref(i.owner_id),
            'creator', user_ref(i.creator_id),
            'targetDate', i.target_date,
            'targetDateResolution', i.target_date_resolution,
            'parent', initiative_ref(i.parent_id),
            'projectCount', (SELECT count(*) FROM initiative_projects ip JOIN projects ipp ON ipp.id = ip.project_id
                WHERE ip.initiative_id = i.id AND ipp.trashed_at IS NULL),
            'progress', initiative_progress(i.id),
            'sortOrder', i.sort_order,
            'startedAt', i.started_at,
            'completedAt', i.completed_at,
            'archivedAt', i.archived_at,
            'trashed', i.trashed_at IS NOT NULL,
            'trashedAt', i.trashed_at,
            'createdAt', i.created_at,
            'updatedAt', i.updated_at
        )"
    )
}

async fn initiative_value(ctx: &Ctx, db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    let sql = format!("SELECT {} {INITIATIVE_FROM} WHERE i.id = $1", initiative_json(ctx, true));
    let value: Option<Value> = sqlx::query_scalar(&sql).bind(id).fetch_optional(&mut *db).await?;
    value.ok_or_else(|| ApiError::not_found("No such initiative"))
}

/* ------------------------------------------------------------------ */
/*                             List, read                             */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ListArgs {
    /// planned, active or completed.
    status: Option<String>,
    /// Owner (id, name, email or "me").
    owner: Option<String>,
    /// Sub-initiatives of this initiative (name, slug or id), or "null" for top-level ones.
    parent: Option<String>,
    /// Words in the name, summary or description.
    query: Option<String>,
    #[serde(default)]
    include_archived: bool,
    #[serde(default)]
    include_trashed: bool,
    /// Include each initiative's description.
    #[serde(default)]
    include_description: bool,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list(ctx: Ctx, args: ListArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let page = Page { limit: args.limit, cursor: args.cursor.clone() };
    let mut qb: QueryBuilder<Postgres> = QueryBuilder::new(format!(
        "SELECT {} {INITIATIVE_FROM} WHERE i.workspace_id = ",
        initiative_json(&ctx, args.include_description)
    ));
    qb.push_bind(ctx.workspace);
    if !args.include_archived {
        qb.push(" AND i.archived_at IS NULL");
    }
    if !args.include_trashed {
        qb.push(" AND i.trashed_at IS NULL");
    }
    if let Some(status) = &args.status {
        qb.push(" AND i.status = ");
        qb.push_bind(check_status(status)?);
    }
    if let Some(owner) = &args.owner {
        qb.push(" AND i.owner_id = ");
        qb.push_bind(resolve::user(&ctx, owner).await?);
    }
    if let Some(parent) = &args.parent {
        if is_null_word(parent) {
            qb.push(" AND i.parent_id IS NULL");
        } else {
            qb.push(" AND i.parent_id = ");
            qb.push_bind(resolve::initiative(&ctx, parent).await?);
        }
    }
    if let Some(query) = args.query.as_deref().map(str::trim).filter(|q| !q.is_empty()) {
        let like = format!("%{}%", escape_like(query));
        qb.push(" AND (i.name ILIKE ");
        qb.push_bind(like.clone());
        qb.push(" OR i.summary ILIKE ");
        qb.push_bind(like.clone());
        qb.push(" OR i.content ILIKE ");
        qb.push_bind(like);
        qb.push(")");
    }
    qb.push(" ORDER BY i.updated_at DESC LIMIT ");
    qb.push_bind(page.limit() + 1);
    qb.push(" OFFSET ");
    qb.push_bind(page.offset()?);
    let nodes: Vec<Value> = qb.build_query_scalar().fetch_all(ctx.db()).await?;
    page.wrap(nodes)
}

async fn get(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let id = resolve::initiative(&ctx, &args.id).await?;
    let mut db = ctx.db().acquire().await?;
    let mut value = initiative_value(&ctx, &mut db, id).await?;
    let teams = ctx.team_ids().await?.to_vec();
    let sql = format!(
        "SELECT jsonb_build_object(
            'projects', (SELECT coalesce(jsonb_agg(project_ref(p.id) || jsonb_build_object(
                    'status', project_status_ref(p.status_id), 'health', p.health, 'lead', user_ref(p.lead_id),
                    'targetDate', p.target_date, 'progress', project_issue_stats(p.id, NULL)->'progress')
                ORDER BY ip.sort_order, p.name), '[]')
                FROM initiative_projects ip JOIN projects p ON p.id = ip.project_id
                WHERE ip.initiative_id = $1 AND p.trashed_at IS NULL AND {visible}),
            'subInitiatives', (SELECT coalesce(jsonb_agg(initiative_ref(c.id) || jsonb_build_object(
                    'status', c.status, 'health', c.health, 'owner', user_ref(c.owner_id),
                    'progress', initiative_progress(c.id)) ORDER BY c.sort_order, c.name), '[]')
                FROM initiatives c WHERE c.parent_id = $1 AND c.trashed_at IS NULL),
            'links', {links},
            'documents', (SELECT coalesce(jsonb_agg(document_ref(d.id) ORDER BY d.sort_order, d.created_at), '[]')
                FROM documents d WHERE d.initiative_id = $1 AND d.trashed_at IS NULL)
        )",
        visible = resolve::project_visible("p", 2),
        links = links_sql("initiative_id"),
    );
    let more: Value = sqlx::query_scalar(&sql).bind(id).bind(&teams).fetch_one(&mut *db).await?;
    if let (Some(target), Value::Object(more)) = (value.as_object_mut(), more) {
        target.extend(more);
    }
    value["latestUpdate"] = super::status_updates::latest(&mut db, "initiative_id", id).await?.unwrap_or(Value::Null);
    Ok(value)
}

/* ------------------------------------------------------------------ */
/*                                Save                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SaveArgs {
    /// Only to update an initiative: its id, slug, link or name. Omit to create one.
    id: Option<String>,
    /// Required to create.
    name: Option<String>,
    /// A one-line summary, at most 255 characters; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    summary: Option<Option<String>>,
    /// Markdown.
    description: Option<String>,
    /// Edits to the current description instead of the whole text, applied in order, all or none.
    patch: Option<Vec<TextEdit>>,
    /// An emoji or icon name; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    icon: Option<Option<String>>,
    /// Hex color like #5e6ad2; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    color: Option<Option<String>>,
    /// planned (the default), active or completed.
    status: Option<String>,
    /// Person (id, name, email or "me"); null removes the owner.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    owner: Option<Option<String>>,
    /// YYYY-MM-DD; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    target_date: Option<Option<String>>,
    /// How precise targetDate is: month, quarter, halfYear or year; null means the exact day.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    target_date_resolution: Option<Option<String>>,
    /// The initiative (name, slug or id) this one is part of; null makes it top-level.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    parent: Option<Option<String>>,
    /// Projects (names, slugs or ids) to add.
    add_projects: Option<Vec<String>>,
    remove_projects: Option<Vec<String>>,
    /// Replace all projects with these.
    set_projects: Option<Vec<String>>,
    /// Links to add ({url, title}); a link already on the initiative gets the new title.
    links: Option<Vec<LinkArg>>,
    /// URLs of links to remove.
    remove_links: Option<Vec<String>>,
    /// Position in manually ordered lists.
    sort_order: Option<f64>,
}

fn check_status(value: &str) -> ApiResult<String> {
    let lower = value.trim().to_lowercase();
    if !["planned", "active", "completed"].contains(&lower.as_str()) {
        return Err(ApiError::bad_request(format!(
            "An initiative's status is planned, active or completed, not \"{value}\""
        )));
    }
    Ok(lower)
}

async fn save(ctx: Ctx, args: SaveArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let existing = match &args.id {
        Some(reference) => Some(resolve::initiative(&ctx, reference).await?),
        None => None,
    };
    let mut tx = ctx.db().begin().await?;
    let before = match existing {
        Some(id) => Some(initiative_value(&ctx, &mut tx, id).await?),
        None => None,
    };
    let id = match existing {
        Some(id) => id,
        None => {
            let name = non_empty(args.name.as_deref().unwrap_or(""), "name")?;
            sqlx::query_scalar(
                "INSERT INTO initiatives (workspace_id, slug_id, name, creator_id, sort_order)
                 VALUES ($1, $2, $3, $4, (SELECT coalesce(max(sort_order), 0) + 1000 FROM initiatives WHERE workspace_id = $1))
                 RETURNING id",
            )
            .bind(ctx.workspace)
            .bind(slug_id())
            .bind(&name)
            .bind(ctx.user)
            .fetch_one(&mut *tx)
            .await?
        }
    };
    apply_changes(&ctx, &mut tx, id, args).await?;
    sqlx::query("UPDATE initiatives SET updated_at = now() WHERE id = $1").bind(id).execute(&mut *tx).await?;
    let value = initiative_value(&ctx, &mut tx, id).await?;
    match before {
        Some(before) => {
            let updated_from = changed_fields(&before, &value);
            if !updated_from.as_object().is_none_or(Map::is_empty) {
                events::emit(&mut tx, &ctx, "Initiative", "update", &value, Some(updated_from), None).await?;
            }
        }
        None => events::emit(&mut tx, &ctx, "Initiative", "create", &value, None, None).await?,
    }
    tx.commit().await?;
    Ok(value)
}

async fn apply_changes(ctx: &Ctx, tx: &mut PgConnection, id: Uuid, args: SaveArgs) -> ApiResult<()> {
    let content: String = sqlx::query_scalar("SELECT content FROM initiatives WHERE id = $1 FOR UPDATE")
        .bind(id)
        .fetch_one(&mut *tx)
        .await?;
    if let Some(name) = &args.name {
        sqlx::query("UPDATE initiatives SET name = $2 WHERE id = $1")
            .bind(id)
            .bind(non_empty(name, "name")?)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(summary) = args.summary {
        let summary = summary.map(|s| s.trim().to_string()).filter(|s| !s.is_empty());
        if summary.as_ref().is_some_and(|s| s.chars().count() > 255) {
            return Err(ApiError::bad_request("summary is at most 255 characters; put the rest in the description"));
        }
        sqlx::query("UPDATE initiatives SET summary = $2 WHERE id = $1")
            .bind(id)
            .bind(summary)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(content) = new_content(&content, args.description, args.patch.as_deref())? {
        sqlx::query("UPDATE initiatives SET content = $2 WHERE id = $1")
            .bind(id)
            .bind(content)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(icon) = args.icon {
        let icon = icon.filter(|i| !i.trim().is_empty());
        sqlx::query("UPDATE initiatives SET icon = $2 WHERE id = $1").bind(id).bind(icon).execute(&mut *tx).await?;
    }
    if let Some(color) = args.color {
        let color = color.as_deref().map(check_color).transpose()?;
        sqlx::query("UPDATE initiatives SET color = $2 WHERE id = $1").bind(id).bind(color).execute(&mut *tx).await?;
    }
    if let Some(status) = &args.status {
        sqlx::query(
            "UPDATE initiatives SET status = $2,
                started_at = CASE WHEN $2 = 'planned' THEN NULL WHEN $2 = 'active' THEN coalesce(started_at, now())
                                  ELSE started_at END,
                completed_at = CASE WHEN $2 = 'completed' THEN coalesce(completed_at, now()) END
             WHERE id = $1",
        )
        .bind(id)
        .bind(check_status(status)?)
        .execute(&mut *tx)
        .await?;
    }
    if let Some(owner) = args.owner {
        let owner = match owner.filter(|o| !is_null_word(o)) {
            Some(person) => Some(resolve::user(ctx, &person).await?),
            None => None,
        };
        sqlx::query("UPDATE initiatives SET owner_id = $2 WHERE id = $1")
            .bind(id)
            .bind(owner)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(date) = args.target_date {
        let date = date.filter(|d| !is_null_word(d)).map(|d| parse_date(&d)).transpose()?;
        sqlx::query(
            "UPDATE initiatives SET target_date = $2,
                target_date_resolution = CASE WHEN $2::date IS NULL THEN NULL ELSE target_date_resolution END
             WHERE id = $1",
        )
        .bind(id)
        .bind(date)
        .execute(&mut *tx)
        .await?;
    }
    if let Some(resolution) = args.target_date_resolution {
        let resolution = match resolution.filter(|r| !is_null_word(r)) {
            Some(r) => Some(
                ["month", "quarter", "halfYear", "year"]
                    .iter()
                    .find(|x| x.eq_ignore_ascii_case(r.trim()))
                    .ok_or_else(|| {
                        ApiError::bad_request(format!(
                            "A date resolution is month, quarter, halfYear or year, not \"{r}\""
                        ))
                    })?
                    .to_string(),
            ),
            None => None,
        };
        sqlx::query("UPDATE initiatives SET target_date_resolution = $2 WHERE id = $1")
            .bind(id)
            .bind(resolution)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(parent) = args.parent {
        let parent = match parent.filter(|p| !is_null_word(p)) {
            Some(parent) => Some(resolve::initiative(ctx, &parent).await?),
            None => None,
        };
        if let Some(parent) = parent {
            let loops: bool = sqlx::query_scalar(
                "WITH RECURSIVE up AS (SELECT id, parent_id FROM initiatives WHERE id = $1
                 UNION SELECT i.id, i.parent_id FROM initiatives i JOIN up ON i.id = up.parent_id)
                 SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)",
            )
            .bind(parent)
            .bind(id)
            .fetch_one(&mut *tx)
            .await?;
            if loops {
                return Err(ApiError::bad_request(
                    "An initiative can't be part of itself or of its own sub-initiatives",
                ));
            }
        }
        sqlx::query("UPDATE initiatives SET parent_id = $2 WHERE id = $1")
            .bind(id)
            .bind(parent)
            .execute(&mut *tx)
            .await?;
    }
    projects(ctx, tx, id, &args.set_projects, &args.add_projects, &args.remove_projects).await?;
    save_links(
        ctx,
        tx,
        "initiative_id",
        id,
        args.links.as_deref().unwrap_or_default(),
        args.remove_links.as_deref().unwrap_or_default(),
    )
    .await?;
    if let Some(order) = args.sort_order {
        sqlx::query("UPDATE initiatives SET sort_order = $2 WHERE id = $1")
            .bind(id)
            .bind(order)
            .execute(&mut *tx)
            .await?;
    }
    Ok(())
}

async fn projects(
    ctx: &Ctx,
    tx: &mut PgConnection,
    id: Uuid,
    set: &Option<Vec<String>>,
    add: &Option<Vec<String>>,
    remove: &Option<Vec<String>>,
) -> ApiResult<()> {
    if set.is_some() && (add.is_some() || remove.is_some()) {
        return Err(ApiError::bad_request("Pass either setProjects or addProjects/removeProjects, not both"));
    }
    if let Some(all) = set {
        let ids = resolve::projects(ctx, all).await?;
        sqlx::query("DELETE FROM initiative_projects WHERE initiative_id = $1 AND NOT (project_id = ANY($2))")
            .bind(id)
            .bind(&ids)
            .execute(&mut *tx)
            .await?;
        for project in ids {
            link_initiatives(tx, project, &[id]).await?;
        }
    }
    if let Some(add) = add {
        for project in resolve::projects(ctx, add).await? {
            link_initiatives(tx, project, &[id]).await?;
        }
    }
    if let Some(remove) = remove {
        let ids = resolve::projects(ctx, remove).await?;
        sqlx::query("DELETE FROM initiative_projects WHERE initiative_id = $1 AND project_id = ANY($2)")
            .bind(id)
            .bind(&ids)
            .execute(&mut *tx)
            .await?;
    }
    Ok(())
}

/* ------------------------------------------------------------------ */
/*                          Trash and archive                         */
/* ------------------------------------------------------------------ */

async fn mark(ctx: &Ctx, id: Uuid, sql: &str, action: &str) -> ApiResult<Value> {
    let mut tx = ctx.db().begin().await?;
    sqlx::query(sql).bind(id).execute(&mut *tx).await?;
    let value = initiative_value(ctx, &mut tx, id).await?;
    events::emit(&mut tx, ctx, "Initiative", action, &value, None, None).await?;
    tx.commit().await?;
    Ok(value)
}

async fn trash(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let id = resolve::initiative(&ctx, &args.id).await?;
    let sql = "UPDATE initiatives SET trashed_at = now(), updated_at = now() WHERE id = $1 AND trashed_at IS NULL";
    mark(&ctx, id, sql, "remove").await
}

async fn restore(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let id = find_trashed(&ctx, "initiatives", "initiative", &args.id).await?;
    let sql = "UPDATE initiatives SET trashed_at = NULL, updated_at = now() WHERE id = $1";
    mark(&ctx, id, sql, "update").await
}

async fn archive(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let id = resolve::initiative(&ctx, &args.id).await?;
    let sql = "UPDATE initiatives SET archived_at = now(), updated_at = now() WHERE id = $1 AND archived_at IS NULL";
    mark(&ctx, id, sql, "update").await
}

async fn unarchive(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let id = resolve::initiative(&ctx, &args.id).await?;
    let sql = "UPDATE initiatives SET archived_at = NULL, updated_at = now() WHERE id = $1";
    mark(&ctx, id, sql, "update").await
}
