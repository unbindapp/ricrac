//! Documents: Markdown pages attached to a project, initiative, team, issue, cycle or release,
//! with earlier versions kept as they are rewritten.

use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::Value;
use sqlx::{PgConnection, Postgres, QueryBuilder};
use uuid::Uuid;

use super::events::{self, Notice};
use super::filter::escape_like;
use super::resolve::{self, as_uuid};
use super::util::{Page, TextEdit, check_color, new_content, non_empty, nullable, slug_id};
use super::{IdArgs, Op, op};
use crate::ctx::{Access, Ctx};
use crate::error::{ApiError, ApiResult};

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "list_documents",
            Access::Read,
            &[("GET", "/documents")],
            "List documents, most recently updated first. Narrow to those of a project, initiative, team, issue, \
             cycle or release, by creator, or by words in the title or content. Contents are left out unless \
             includeContent is set; read one with get_document.",
            list,
        ),
        op(
            "get_document",
            Access::Read,
            &[("GET", "/documents/{id}")],
            "Read a document by id, slug, link or title: its Markdown content, where it belongs and how many earlier \
             versions are kept.",
            get,
        ),
        op(
            "save_document",
            Access::Write,
            &[("POST", "/documents"), ("PATCH", "/documents/{id}")],
            "Create a document (omit id; title and exactly one of project, initiative, team, issue, cycle or release \
             are required, unless a template gives the title) or update one (pass id). Use patch to edit part of the \
             content without resending it. Passing a parent on update moves the document there. People mentioned \
             with @displayName or @email are notified. The previous content is kept as a version.",
            save,
        ),
        op(
            "delete_document",
            Access::Write,
            &[("DELETE", "/documents/{id}")],
            "Move a document to the trash. It is deleted for good after 30 days; restore_document brings it back.",
            trash,
        ),
        op(
            "restore_document",
            Access::Write,
            &[("POST", "/documents/{id}/restore")],
            "Bring a document back from the trash.",
            restore,
        ),
        op(
            "list_document_versions",
            Access::Read,
            &[("GET", "/documents/{id}/versions")],
            "The earlier versions of a document, newest first (without their content; read one with \
             get_document_version).",
            versions,
        ),
        op(
            "get_document_version",
            Access::Read,
            &[("GET", "/document-versions/{id}")],
            "Read an earlier version of a document: its title and content as they were.",
            get_version,
        ),
    ]
}

/* ------------------------------------------------------------------ */
/*                           Representation                           */
/* ------------------------------------------------------------------ */

const DOCUMENT_FROM: &str = "FROM documents d JOIN workspaces w ON w.id = d.workspace_id";

/// The JSON of the document `d` (with its workspace `w` joined).
fn document_json(ctx: &Ctx, with_content: bool) -> String {
    let base = format!("'{}'", ctx.public_url().replace('\'', "''"));
    let content = if with_content { "'content', d.content," } else { "" };
    format!(
        "jsonb_build_object(
            'id', d.id,
            'slugId', d.slug_id,
            'title', d.title,
            {content}
            'url', {base} || '/' || w.url_key || '/document/' || concat_ws('-', nullif(trim(both '-' from left(
                trim(both '-' from regexp_replace(lower(d.title), '[^a-z0-9]+', '-', 'g')), 60)), ''), d.slug_id),
            'icon', d.icon,
            'color', d.color,
            'project', project_ref(d.project_id),
            'initiative', initiative_ref(d.initiative_id),
            'team', team_ref(d.team_id),
            'issue', issue_ref(d.issue_id),
            'cycle', cycle_ref(d.cycle_id),
            'release', release_ref(d.release_id),
            'creator', user_ref(d.creator_id),
            'updatedBy', user_ref(d.updated_by_id),
            'trashed', d.trashed_at IS NOT NULL,
            'trashedAt', d.trashed_at,
            'createdAt', d.created_at,
            'updatedAt', d.updated_at
        )"
    )
}

async fn document_value(ctx: &Ctx, db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    let sql = format!(
        "SELECT {} || jsonb_build_object('versionCount', (SELECT count(*) FROM document_versions v WHERE v.document_id = d.id))
         {DOCUMENT_FROM} WHERE d.id = $1",
        document_json(ctx, true)
    );
    let value: Option<Value> = sqlx::query_scalar(&sql).bind(id).fetch_optional(&mut *db).await?;
    value.ok_or_else(|| ApiError::not_found("No such document"))
}

/// Appends `AND <the caller can see document d>`, as `resolve::can_see_document` decides: by its
/// team, project, issue or cycle; documents of initiatives and releases are visible to everyone.
pub fn push_visible(qb: &mut QueryBuilder<Postgres>, ctx: &Ctx, teams: &[Uuid], d: &str) {
    qb.push(format!(" AND CASE WHEN {d}.team_id IS NOT NULL THEN {d}.team_id = ANY("));
    qb.push_bind(teams.to_vec());
    qb.push(format!(
        ") WHEN {d}.project_id IS NOT NULL THEN EXISTS (SELECT 1 FROM projects vp JOIN project_teams vpt ON vpt.project_id = vp.id
            WHERE vp.id = {d}.project_id AND vp.trashed_at IS NULL AND vpt.team_id = ANY("
    ));
    qb.push_bind(teams.to_vec());
    qb.push(format!(
        ")) WHEN {d}.issue_id IS NOT NULL THEN EXISTS (SELECT 1 FROM issues vi WHERE vi.id = {d}.issue_id AND (vi.team_id = ANY("
    ));
    qb.push_bind(teams.to_vec());
    qb.push(") OR EXISTS (SELECT 1 FROM issue_shares vs WHERE vs.issue_id = vi.id AND vs.user_id = ");
    qb.push_bind(ctx.user);
    qb.push(format!(
        "))) WHEN {d}.cycle_id IS NOT NULL THEN EXISTS (SELECT 1 FROM cycles vc WHERE vc.id = {d}.cycle_id AND vc.team_id = ANY("
    ));
    qb.push_bind(teams.to_vec());
    qb.push(")) ELSE TRUE END");
}

/// A document the caller can see, by id, slug, link or title; with `trashed`, only among those in
/// the trash.
async fn find(ctx: &Ctx, value: &str, trashed: bool) -> ApiResult<Uuid> {
    if !trashed {
        return resolve::document(ctx, value).await;
    }
    let value = value.trim();
    let tail = value.trim_end_matches('/').rsplit(['/', '-']).next().unwrap_or(value);
    let id: Option<Uuid> = sqlx::query_scalar(
        "SELECT id FROM documents WHERE workspace_id = $1 AND trashed_at IS NOT NULL
         AND (id = $2 OR slug_id = $3 OR lower(title) = lower($4)) ORDER BY trashed_at DESC LIMIT 1",
    )
    .bind(ctx.workspace)
    .bind(as_uuid(value))
    .bind(tail)
    .bind(value)
    .fetch_optional(ctx.db())
    .await?;
    match id {
        Some(id) if resolve::can_see_document(ctx, id).await? => Ok(id),
        _ => Err(ApiError::not_found(format!("No document \"{value}\" in the trash"))),
    }
}

/* ------------------------------------------------------------------ */
/*                                Read                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ListDocumentsArgs {
    /// Project name, slug or id.
    project: Option<String>,
    /// Initiative name, slug or id.
    initiative: Option<String>,
    /// Team key, name or id: the team's documents (with cycle, only the team the cycle belongs to).
    team: Option<String>,
    /// Issue identifier (ENG-123) or id.
    issue: Option<String>,
    /// Cycle id, or its number, name or "current" with team.
    cycle: Option<String>,
    /// Release name, slug or id.
    release: Option<String>,
    /// Person who created the documents (id, name, email or "me").
    creator: Option<String>,
    /// Words to find in the title or content.
    query: Option<String>,
    #[serde(default)]
    include_trashed: bool,
    /// Include each document's content.
    #[serde(default)]
    include_content: bool,
    /// updatedAt (default) or createdAt, newest first.
    order_by: Option<String>,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list(ctx: Ctx, args: ListDocumentsArgs) -> ApiResult<Value> {
    let page = Page { limit: args.limit, cursor: args.cursor.clone() };
    let order = match args.order_by.as_deref().unwrap_or("updatedAt") {
        "updatedAt" => "d.updated_at DESC",
        "createdAt" => "d.created_at DESC",
        other => return Err(ApiError::bad_request(format!("orderBy \"{other}\" isn't updatedAt or createdAt"))),
    };
    let team = match &args.team {
        Some(team) => Some(resolve::team(&ctx, team).await?),
        None => None,
    };
    let mut parents: Vec<(&str, Uuid)> = Vec::new();
    if let Some(value) = &args.project {
        parents.push(("project_id", resolve::project(&ctx, value).await?));
    }
    if let Some(value) = &args.initiative {
        parents.push(("initiative_id", resolve::initiative(&ctx, value).await?));
    }
    if let Some(value) = &args.issue {
        parents.push(("issue_id", resolve::issue(&ctx, value).await?));
    }
    if let Some(value) = &args.release {
        parents.push(("release_id", resolve::release(&ctx, value).await?));
    }
    match (&args.cycle, team) {
        (Some(value), team) => parents.push(("cycle_id", resolve::cycle(&ctx, team, value).await?)),
        (None, Some(team)) => parents.push(("team_id", team)),
        (None, None) => {}
    }
    let teams = ctx.team_ids().await?.to_vec();
    let mut qb = QueryBuilder::new(format!(
        "SELECT {} {DOCUMENT_FROM} WHERE d.workspace_id = ",
        document_json(&ctx, args.include_content)
    ));
    qb.push_bind(ctx.workspace);
    push_visible(&mut qb, &ctx, &teams, "d");
    for (column, id) in parents {
        qb.push(format!(" AND d.{column} = "));
        qb.push_bind(id);
    }
    if let Some(creator) = &args.creator {
        qb.push(" AND d.creator_id = ");
        qb.push_bind(resolve::user(&ctx, creator).await?);
    }
    if !args.include_trashed {
        qb.push(" AND d.trashed_at IS NULL");
    }
    if let Some(query) = args.query.as_deref().map(str::trim).filter(|q| !q.is_empty()) {
        qb.push(" AND (d.search @@ websearch_to_tsquery('english', ");
        qb.push_bind(query.to_string());
        qb.push(") OR d.title ILIKE ");
        qb.push_bind(format!("%{}%", escape_like(query)));
        qb.push(")");
    }
    qb.push(format!(" ORDER BY {order} LIMIT "));
    qb.push_bind(page.limit() + 1);
    qb.push(" OFFSET ");
    qb.push_bind(page.offset()?);
    let nodes: Vec<Value> = qb.build_query_scalar().fetch_all(ctx.db()).await?;
    page.wrap(nodes)
}

async fn get(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = find(&ctx, &args.id, false).await?;
    let mut db = ctx.db().acquire().await?;
    document_value(&ctx, &mut db, id).await
}

/* ------------------------------------------------------------------ */
/*                                Save                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SaveDocumentArgs {
    /// Only to update a document: its id, slug, link or title. Omit to create one.
    id: Option<String>,
    title: Option<String>,
    /// Markdown. Mention people with @displayName or @email; they are notified.
    content: Option<String>,
    /// Edits to the current content instead of the whole text, applied in order, all or none.
    patch: Option<Vec<TextEdit>>,
    /// An emoji or icon name; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    icon: Option<Option<String>>,
    /// Hex color like #5e6ad2; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    color: Option<Option<String>>,
    /// Parent project: name, slug or id.
    project: Option<String>,
    /// Parent initiative: name, slug or id.
    initiative: Option<String>,
    /// Parent team: key, name or id (or the team of the cycle, with cycle).
    team: Option<String>,
    /// Parent issue: identifier (ENG-123) or id.
    issue: Option<String>,
    /// Parent cycle: id, or number, name or "current" with team.
    cycle: Option<String>,
    /// Parent release: name, slug or id.
    release: Option<String>,
    /// Document template name or id, applied on create: it gives the title and content the arguments don't.
    template: Option<String>,
}

#[derive(Default)]
struct Parent {
    project: Option<Uuid>,
    initiative: Option<Uuid>,
    team: Option<Uuid>,
    issue: Option<Uuid>,
    cycle: Option<Uuid>,
    release: Option<Uuid>,
}

/// The one parent named in the arguments, or None when none is.
async fn parent(ctx: &Ctx, args: &SaveDocumentArgs) -> ApiResult<Option<Parent>> {
    let team = match &args.team {
        Some(team) => Some(resolve::team(ctx, team).await?),
        None => None,
    };
    let named = [
        args.project.is_some(),
        args.initiative.is_some(),
        args.team.is_some() && args.cycle.is_none(),
        args.issue.is_some(),
        args.cycle.is_some(),
        args.release.is_some(),
    ];
    match named.iter().filter(|n| **n).count() {
        0 => return Ok(None),
        1 => {}
        _ => {
            return Err(ApiError::bad_request(
                "A document belongs to one of project, initiative, team, issue, cycle or release; pass only one",
            ));
        }
    }
    let mut parent = Parent::default();
    if let Some(value) = &args.project {
        parent.project = Some(resolve::project(ctx, value).await?);
    } else if let Some(value) = &args.initiative {
        parent.initiative = Some(resolve::initiative(ctx, value).await?);
    } else if let Some(value) = &args.issue {
        parent.issue = Some(resolve::issue(ctx, value).await?);
    } else if let Some(value) = &args.cycle {
        parent.cycle = Some(resolve::cycle(ctx, team, value).await?);
    } else if let Some(value) = &args.release {
        parent.release = Some(resolve::release(ctx, value).await?);
    } else {
        parent.team = team;
    }
    Ok(Some(parent))
}

async fn save(ctx: Ctx, args: SaveDocumentArgs) -> ApiResult<Value> {
    let mut tx = ctx.db().begin().await?;
    let (id, action) = match args.id.clone() {
        Some(reference) => {
            let id = find(&ctx, &reference, false).await?;
            update(&ctx, &mut tx, id, args).await?;
            (id, "update")
        }
        None => (create(&ctx, &mut tx, args).await?, "create"),
    };
    let value = document_value(&ctx, &mut tx, id).await?;
    let team = webhook_team(&mut tx, id).await?;
    events::emit(&mut tx, &ctx, "Document", action, &value, None, team).await?;
    tx.commit().await?;
    Ok(value)
}

async fn webhook_team(db: &mut PgConnection, id: Uuid) -> ApiResult<Option<Uuid>> {
    Ok(sqlx::query_scalar(
        "SELECT coalesce(d.team_id, (SELECT team_id FROM issues WHERE id = d.issue_id),
            (SELECT team_id FROM cycles WHERE id = d.cycle_id)) FROM documents d WHERE d.id = $1",
    )
    .bind(id)
    .fetch_one(&mut *db)
    .await?)
}

fn optional_color(color: Option<Option<String>>) -> ApiResult<Option<Option<String>>> {
    color.map(|c| c.as_deref().map(check_color).transpose()).transpose()
}

async fn create(ctx: &Ctx, tx: &mut PgConnection, args: SaveDocumentArgs) -> ApiResult<Uuid> {
    let Some(parent) = parent(ctx, &args).await? else {
        return Err(ApiError::bad_request(
            "Say where the document belongs: one of project, initiative, team, issue, cycle or release",
        ));
    };
    if args.patch.is_some() {
        return Err(ApiError::bad_request("patch only applies when updating; pass content"));
    }
    let (mut title, mut content) = (args.title.clone(), args.content.clone());
    if let Some(template) = &args.template {
        let id = resolve::template(ctx, "document", template).await?;
        let data: Value =
            sqlx::query_scalar("SELECT data FROM templates WHERE id = $1").bind(id).fetch_one(&mut *tx).await?;
        let text = |key: &str| data.get(key).and_then(Value::as_str).map(String::from);
        title = title.or_else(|| text("title"));
        content = content.filter(|c| !c.is_empty()).or_else(|| text("content"));
    }
    let title = non_empty(title.as_deref().unwrap_or(""), "title")?;
    let content = content.unwrap_or_default();
    let color = optional_color(args.color)?.flatten();
    let id: Uuid = sqlx::query_scalar(
        "INSERT INTO documents (workspace_id, slug_id, title, content, icon, color, project_id, initiative_id, team_id,
            issue_id, cycle_id, release_id, creator_id, updated_by_id)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13) RETURNING id",
    )
    .bind(ctx.workspace)
    .bind(slug_id())
    .bind(&title)
    .bind(&content)
    .bind(args.icon.flatten())
    .bind(color)
    .bind(parent.project)
    .bind(parent.initiative)
    .bind(parent.team)
    .bind(parent.issue)
    .bind(parent.cycle)
    .bind(parent.release)
    .bind(ctx.user)
    .fetch_one(&mut *tx)
    .await?;
    notify_mentions(ctx, tx, id, &title, "", &content).await?;
    Ok(id)
}

async fn update(ctx: &Ctx, tx: &mut PgConnection, id: Uuid, args: SaveDocumentArgs) -> ApiResult<()> {
    if args.template.is_some() {
        return Err(ApiError::bad_request("Templates only apply when creating a document"));
    }
    let (title, content, updated_by): (String, String, Option<Uuid>) =
        sqlx::query_as("SELECT title, content, updated_by_id FROM documents WHERE id = $1 FOR UPDATE")
            .bind(id)
            .fetch_one(&mut *tx)
            .await?;
    let new_title = args.title.as_deref().map(|t| non_empty(t, "title")).transpose()?.unwrap_or(title.clone());
    let new_content = new_content(&content, args.content.clone(), args.patch.as_deref())?.unwrap_or(content.clone());
    if new_title != title || new_content != content {
        keep_version(ctx, tx, id, &title, &content, updated_by).await?;
        sqlx::query("UPDATE documents SET title = $2, content = $3 WHERE id = $1")
            .bind(id)
            .bind(&new_title)
            .bind(&new_content)
            .execute(&mut *tx)
            .await?;
        notify_mentions(ctx, tx, id, &new_title, &content, &new_content).await?;
    }
    if let Some(icon) = &args.icon {
        sqlx::query("UPDATE documents SET icon = $2 WHERE id = $1").bind(id).bind(icon).execute(&mut *tx).await?;
    }
    if let Some(color) = optional_color(args.color.clone())? {
        sqlx::query("UPDATE documents SET color = $2 WHERE id = $1").bind(id).bind(color).execute(&mut *tx).await?;
    }
    if let Some(parent) = parent(ctx, &args).await? {
        sqlx::query(
            "UPDATE documents SET project_id = $2, initiative_id = $3, team_id = $4, issue_id = $5, cycle_id = $6,
                release_id = $7 WHERE id = $1",
        )
        .bind(id)
        .bind(parent.project)
        .bind(parent.initiative)
        .bind(parent.team)
        .bind(parent.issue)
        .bind(parent.cycle)
        .bind(parent.release)
        .execute(&mut *tx)
        .await?;
    }
    sqlx::query("UPDATE documents SET updated_at = now(), updated_by_id = $2 WHERE id = $1")
        .bind(id)
        .bind(ctx.user)
        .execute(&mut *tx)
        .await?;
    Ok(())
}

/// Keeps the current title and content as a version, unless the caller wrote them and the last
/// version is less than 10 minutes old: a burst of edits by one person makes one version.
async fn keep_version(
    ctx: &Ctx,
    tx: &mut PgConnection,
    id: Uuid,
    title: &str,
    content: &str,
    updated_by: Option<Uuid>,
) -> ApiResult<()> {
    let recent: bool = sqlx::query_scalar(
        "SELECT EXISTS (SELECT 1 FROM document_versions WHERE document_id = $1 AND created_at > now() - interval '10 minutes')",
    )
    .bind(id)
    .fetch_one(&mut *tx)
    .await?;
    if recent && updated_by == Some(ctx.user) {
        return Ok(());
    }
    sqlx::query("INSERT INTO document_versions (document_id, title, content, author_id) VALUES ($1, $2, $3, $4)")
        .bind(id)
        .bind(title)
        .bind(content)
        .bind(updated_by)
        .execute(&mut *tx)
        .await?;
    Ok(())
}

async fn notify_mentions(
    ctx: &Ctx,
    tx: &mut PgConnection,
    id: Uuid,
    title: &str,
    before: &str,
    after: &str,
) -> ApiResult<()> {
    let previous = events::mentions(ctx, before).await?;
    let mentioned: Vec<Uuid> =
        events::mentions(ctx, after).await?.into_iter().filter(|u| !previous.contains(u)).collect();
    if mentioned.is_empty() {
        return Ok(());
    }
    let notice = Notice {
        kind: "documentMention",
        summary: format!("You were mentioned in {title}"),
        document: Some(id),
        ..Default::default()
    };
    events::notify(tx, ctx, &mentioned, &notice).await
}

/* ------------------------------------------------------------------ */
/*                          Trash and versions                        */
/* ------------------------------------------------------------------ */

async fn set_trashed(ctx: &Ctx, id: Uuid, trashed: bool) -> ApiResult<Value> {
    let mut tx = ctx.db().begin().await?;
    sqlx::query(
        "UPDATE documents SET trashed_at = CASE WHEN $2 THEN now() END, updated_at = now(), updated_by_id = $3
         WHERE id = $1",
    )
    .bind(id)
    .bind(trashed)
    .bind(ctx.user)
    .execute(&mut *tx)
    .await?;
    let value = document_value(ctx, &mut tx, id).await?;
    let team = webhook_team(&mut tx, id).await?;
    let action = if trashed { "remove" } else { "update" };
    events::emit(&mut tx, ctx, "Document", action, &value, None, team).await?;
    tx.commit().await?;
    Ok(value)
}

async fn trash(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = find(&ctx, &args.id, false).await?;
    set_trashed(&ctx, id, true).await
}

async fn restore(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = find(&ctx, &args.id, true).await?;
    set_trashed(&ctx, id, false).await
}

async fn versions(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = find(&ctx, &args.id, false).await?;
    Ok(sqlx::query_scalar(
        "SELECT coalesce(jsonb_agg(jsonb_build_object('id', v.id, 'title', v.title, 'author', user_ref(v.author_id),
            'createdAt', v.created_at) ORDER BY v.id DESC), '[]')
         FROM document_versions v WHERE v.document_id = $1",
    )
    .bind(id)
    .fetch_one(ctx.db())
    .await?)
}

async fn get_version(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let not_found = || ApiError::not_found(format!("No document version \"{}\"", args.id.trim()));
    let version: i64 = args.id.trim().parse().map_err(|_| not_found())?;
    let row: Option<(Uuid, Value)> = sqlx::query_as(
        "SELECT d.id, jsonb_build_object('id', v.id, 'document', document_ref(d.id), 'title', v.title,
            'content', v.content, 'author', user_ref(v.author_id), 'createdAt', v.created_at)
         FROM document_versions v JOIN documents d ON d.id = v.document_id WHERE v.id = $1 AND d.workspace_id = $2",
    )
    .bind(version)
    .bind(ctx.workspace)
    .fetch_optional(ctx.db())
    .await?;
    match row {
        Some((document, value)) if resolve::can_see_document(&ctx, document).await? => Ok(value),
        _ => Err(not_found()),
    }
}
