//! Releases: pipelines (continuous or scheduled, with stages from planned to completed), the
//! releases that move through them with the issues they ship, and release notes. Pipelines can be
//! limited to teams; a pipeline with teams is only visible to those who see one of them.

use chrono::{DateTime, NaiveDate, Utc};
use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Value, json};
use sqlx::{PgConnection, Postgres, QueryBuilder};
use uuid::Uuid;

use super::events;
use super::issues::changed_fields;
use super::resolve::{self, as_uuid};
use super::util::{Page, TextEdit, check_color, new_content, non_empty, nullable, parse_date, parse_datetime, slug_id};
use super::{IdArgs, Op, op};
use crate::ctx::{Access, Ctx};
use crate::error::{ApiError, ApiResult};

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "list_release_pipelines",
            Access::Read,
            &[("GET", "/release-pipelines")],
            "List release pipelines by name: type (continuous: every merge ships; scheduled: releases are planned \
             ahead), whether they deploy to production and how many releases they have. Set includeStages / \
             includeTeams to also get their stages and teams.",
            list_pipelines,
        ),
        op(
            "get_release_pipeline",
            Access::Read,
            &[("GET", "/release-pipelines/{id}")],
            "Read a release pipeline by id, slug or name, with its stages (in order) and teams.",
            get_pipeline,
        ),
        op(
            "save_release_pipeline",
            Access::Write,
            &[("POST", "/release-pipelines"), ("PATCH", "/release-pipelines/{id}")],
            "Create a release pipeline (omit id; name is required) or update one (pass id, slug or name). New \
             pipelines get the stages Planned, In Progress, Released and Canceled; change them with \
             save_release_stage. Teams are given as key, name or id.",
            save_pipeline,
        ),
        op(
            "delete_release_pipeline",
            Access::Write,
            &[("DELETE", "/release-pipelines/{id}")],
            "Archive a release pipeline. Its releases stay readable.",
            delete_pipeline,
        ),
        op(
            "save_release_stage",
            Access::Write,
            &[("POST", "/release-stages"), ("PATCH", "/release-stages/{id}")],
            "Add a stage to a release pipeline (omit id; pipeline, name and type are required) or change one (pass its \
             id). The type (planned, started, completed, canceled) decides which timestamps releases get when they \
             move into it.",
            save_stage,
        ),
        op(
            "delete_release_stage",
            Access::Write,
            &[("DELETE", "/release-stages/{id}")],
            "Delete a release stage. When releases are in it, moveReleasesTo (another stage of the pipeline: name, \
             type or id) is required.",
            delete_stage,
        ),
        op(
            "list_releases",
            Access::Read,
            &[("GET", "/releases")],
            "List releases, newest first: name, version, pipeline, stage, dates, commit and how many issues they ship. \
             Narrow by pipeline, stage (a type like started works across pipelines; a stage name needs pipeline) or a \
             query on the name, version or commit. Archived releases are left out unless includeArchived is set.",
            list_releases,
        ),
        op(
            "get_release",
            Access::Read,
            &[("GET", "/releases/{id}")],
            "Read a release by id, slug or name, with the issues in it (with their status) and the release notes \
             that cover it.",
            get_release,
        ),
        op(
            "save_release",
            Access::Write,
            &[("POST", "/releases"), ("PATCH", "/releases/{id}")],
            "Create a release (omit id; name and pipeline are required) or update one (pass id, slug or name). \
             Omitted fields stay as they are; null clears a field. New releases start in the pipeline's first planned \
             stage unless stage is given. Moving to a started, completed or canceled stage stamps startedAt, \
             completedAt or canceledAt unless you pass them; moving back to planned clears them. Add issues to a \
             release with save_issue (addReleases, removeReleases, setReleases).",
            save_release,
        ),
        op(
            "delete_release",
            Access::Write,
            &[("DELETE", "/releases/{id}")],
            "Archive a release. It stays readable and keeps its issues.",
            delete_release,
        ),
        op(
            "list_release_notes",
            Access::Read,
            &[("GET", "/release-notes")],
            "List release notes, newest first, optionally of one pipeline: title, pipeline, the releases they cover \
             and who wrote them. Read the content with get_release_note.",
            list_notes,
        ),
        op(
            "get_release_note",
            Access::Read,
            &[("GET", "/release-notes/{id}")],
            "Read a release note by id or slug: title, Markdown content, pipeline and the releases it covers.",
            get_note,
        ),
        op(
            "save_release_note",
            Access::Write,
            &[("POST", "/release-notes"), ("PATCH", "/release-notes/{id}")],
            "Create release notes (omit id; pipeline is required) or update them (pass id or slug). Name the releases \
             they cover with releases, or with rangeFromRelease and rangeToRelease for every release of the pipeline \
             created between the two, inclusive. The title defaults to the newest release's name and version. Use \
             patch to edit part of the content without resending it.",
            save_note,
        ),
        op(
            "delete_release_note",
            Access::Write,
            &[("DELETE", "/release-notes/{id}")],
            "Delete release notes. The releases stay.",
            delete_note,
        ),
    ]
}

const PIPELINE_TYPES: [&str; 2] = ["continuous", "scheduled"];
const STAGE_TYPES: [&str; 4] = ["planned", "started", "completed", "canceled"];
const DEFAULT_STAGES: [(&str, &str, &str); 4] = [
    ("Planned", "planned", "#bec2c8"),
    ("In Progress", "started", "#f2c94c"),
    ("Released", "completed", "#5e6ad2"),
    ("Canceled", "canceled", "#95a2b3"),
];

/* ------------------------------------------------------------------ */
/*                       Representation, access                       */
/* ------------------------------------------------------------------ */

const PIPELINE_JSON: &str = "jsonb_build_object(
    'id', p.id,
    'slugId', p.slug_id,
    'name', p.name,
    'type', p.type,
    'isProduction', p.is_production,
    'releaseCount', (SELECT count(*) FROM releases r WHERE r.pipeline_id = p.id AND r.archived_at IS NULL),
    'archivedAt', p.archived_at,
    'createdAt', p.created_at,
    'updatedAt', p.updated_at
)";

const STAGE_JSON: &str =
    "jsonb_build_object('id', s.id, 'name', s.name, 'type', s.type, 'color', s.color, 'position', s.position)";

fn pipeline_json(stages: bool, teams: bool) -> String {
    let mut json = format!("({PIPELINE_JSON}");
    if stages {
        json.push_str(&format!(
            " || jsonb_build_object('stages', (SELECT coalesce(jsonb_agg({STAGE_JSON} ORDER BY s.position), '[]')
                FROM release_stages s WHERE s.pipeline_id = p.id AND s.archived_at IS NULL))"
        ));
    }
    if teams {
        json.push_str(
            " || jsonb_build_object('teams', (SELECT coalesce(jsonb_agg(team_ref(pt.team_id)), '[]')
                FROM release_pipeline_teams pt WHERE pt.pipeline_id = p.id))",
        );
    }
    json.push(')');
    json
}

const RELEASE_JSON: &str = "jsonb_build_object(
    'id', r.id,
    'slugId', r.slug_id,
    'name', r.name,
    'version', r.version,
    'description', r.description,
    'commitSha', r.commit_sha,
    'pipeline', release_pipeline_ref(r.pipeline_id),
    'stage', release_stage_ref(r.stage_id),
    'startDate', r.start_date,
    'targetDate', r.target_date,
    'startedAt', r.started_at,
    'completedAt', r.completed_at,
    'canceledAt', r.canceled_at,
    'issueCount', (SELECT count(*) FROM issue_releases ir JOIN issues i ON i.id = ir.issue_id
        WHERE ir.release_id = r.id AND i.trashed_at IS NULL),
    'creator', user_ref(r.creator_id),
    'archivedAt', r.archived_at,
    'createdAt', r.created_at,
    'updatedAt', r.updated_at
)";

fn note_json(with_content: bool) -> String {
    let content = if with_content { "'content', n.content," } else { "" };
    format!(
        "jsonb_build_object(
            'id', n.id,
            'slugId', n.slug_id,
            'title', n.title,
            {content}
            'pipeline', release_pipeline_ref(n.pipeline_id),
            'releases', (SELECT coalesce(jsonb_agg(release_ref(r.id) ORDER BY r.created_at), '[]')
                FROM releases r WHERE r.id = ANY(n.release_ids)),
            'creator', user_ref(n.creator_id),
            'createdAt', n.created_at,
            'updatedAt', n.updated_at
        )"
    )
}

/// Adds the condition that pipeline `{column}` has no teams or a team the caller sees.
async fn push_pipeline_visible(qb: &mut QueryBuilder<'_, Postgres>, ctx: &Ctx, column: &str) -> ApiResult<()> {
    qb.push(format!(
        " AND (NOT EXISTS (SELECT 1 FROM release_pipeline_teams x WHERE x.pipeline_id = {column})
            OR EXISTS (SELECT 1 FROM release_pipeline_teams x WHERE x.pipeline_id = {column} AND x.team_id = ANY("
    ));
    qb.push_bind(ctx.team_ids().await?.to_vec());
    qb.push(")))");
    Ok(())
}

async fn check_pipeline_visible(ctx: &Ctx, pipeline: Uuid, what: &str, value: &str) -> ApiResult<()> {
    let mut qb = QueryBuilder::new("SELECT EXISTS (SELECT 1 FROM release_pipelines p WHERE p.id = ");
    qb.push_bind(pipeline);
    push_pipeline_visible(&mut qb, ctx, "p.id").await?;
    qb.push(")");
    let visible: bool = qb.build_query_scalar().fetch_one(ctx.db()).await?;
    if !visible {
        return Err(ApiError::not_found(format!("No {what} \"{}\"", value.trim())));
    }
    Ok(())
}

async fn pipeline_id(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    let id = resolve::pipeline(ctx, value).await?;
    check_pipeline_visible(ctx, id, "release pipeline", value).await?;
    Ok(id)
}

async fn release_id(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    let id = resolve::release(ctx, value).await?;
    let pipeline: Uuid =
        sqlx::query_scalar("SELECT pipeline_id FROM releases WHERE id = $1").bind(id).fetch_one(ctx.db()).await?;
    check_pipeline_visible(ctx, pipeline, "release", value).await?;
    Ok(id)
}

/// The id or slug in a value that may be a link or a `title-<slug>` path segment.
fn id_or_slug(value: &str) -> &str {
    let value = value.trim().trim_end_matches('/');
    let segment = value.rsplit('/').next().unwrap_or(value);
    match segment.rsplit_once('-') {
        Some((_, tail)) if tail.len() == 12 && tail.bytes().all(|b| b.is_ascii_hexdigit()) => tail,
        _ => segment,
    }
}

async fn note_id(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    let key = id_or_slug(value);
    let found: Option<(Uuid, Uuid)> = sqlx::query_as(
        "SELECT id, pipeline_id FROM release_notes WHERE workspace_id = $1 AND (id = $2 OR slug_id = $3)",
    )
    .bind(ctx.workspace)
    .bind(as_uuid(key))
    .bind(key)
    .fetch_optional(ctx.db())
    .await?;
    let Some((id, pipeline)) = found else {
        return Err(ApiError::not_found(format!("No release note \"{}\"; use its id or slug", value.trim())));
    };
    check_pipeline_visible(ctx, pipeline, "release note", value).await?;
    Ok(id)
}

async fn pipeline_value(db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    let sql = format!("SELECT {} FROM release_pipelines p WHERE p.id = $1", pipeline_json(true, true));
    Ok(sqlx::query_scalar(&sql).bind(id).fetch_one(&mut *db).await?)
}

async fn release_value(db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    Ok(sqlx::query_scalar(&format!("SELECT {RELEASE_JSON} FROM releases r WHERE r.id = $1"))
        .bind(id)
        .fetch_one(&mut *db)
        .await?)
}

async fn note_value(db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    Ok(sqlx::query_scalar(&format!("SELECT {} FROM release_notes n WHERE n.id = $1", note_json(true)))
        .bind(id)
        .fetch_one(&mut *db)
        .await?)
}

fn one_of(value: &str, options: &[&str], what: &str) -> ApiResult<String> {
    let value = value.trim().to_lowercase();
    if !options.contains(&value.as_str()) {
        return Err(ApiError::bad_request(format!("{what} is one of {}", options.join(", "))));
    }
    Ok(value)
}

fn escape_like(value: &str) -> String {
    format!("%{}%", value.replace('\\', "\\\\").replace('%', "\\%").replace('_', "\\_"))
}

/* ------------------------------------------------------------------ */
/*                              Pipelines                             */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct ListPipelinesArgs {
    /// Words in the name.
    query: Option<String>,
    /// continuous or scheduled.
    #[serde(rename = "type")]
    kind: Option<String>,
    is_production: Option<bool>,
    /// Pipelines of this team (key, name or id).
    team: Option<String>,
    #[serde(default)]
    include_archived: bool,
    /// Include each pipeline's stages.
    #[serde(default)]
    include_stages: bool,
    /// Include each pipeline's teams.
    #[serde(default)]
    include_teams: bool,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list_pipelines(ctx: Ctx, args: ListPipelinesArgs) -> ApiResult<Value> {
    let page = Page { limit: args.limit, cursor: args.cursor.clone() };
    let json = pipeline_json(args.include_stages, args.include_teams);
    let mut qb = QueryBuilder::new(format!("SELECT {json} FROM release_pipelines p WHERE p.workspace_id = "));
    qb.push_bind(ctx.workspace);
    push_pipeline_visible(&mut qb, &ctx, "p.id").await?;
    if !args.include_archived {
        qb.push(" AND p.archived_at IS NULL");
    }
    if let Some(query) = args.query.as_deref().map(str::trim).filter(|q| !q.is_empty()) {
        qb.push(" AND p.name ILIKE ");
        qb.push_bind(escape_like(query));
    }
    if let Some(kind) = &args.kind {
        qb.push(" AND p.type = ");
        qb.push_bind(one_of(kind, &PIPELINE_TYPES, "type")?);
    }
    if let Some(production) = args.is_production {
        qb.push(" AND p.is_production = ");
        qb.push_bind(production);
    }
    if let Some(team) = &args.team {
        qb.push(" AND EXISTS (SELECT 1 FROM release_pipeline_teams t WHERE t.pipeline_id = p.id AND t.team_id = ");
        qb.push_bind(resolve::team(&ctx, team).await?);
        qb.push(")");
    }
    qb.push(" ORDER BY lower(p.name), p.id LIMIT ");
    qb.push_bind(page.limit() + 1);
    qb.push(" OFFSET ");
    qb.push_bind(page.offset()?);
    let nodes: Vec<Value> = qb.build_query_scalar().fetch_all(ctx.db()).await?;
    page.wrap(nodes)
}

async fn get_pipeline(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = pipeline_id(&ctx, &args.id).await?;
    pipeline_value(&mut *ctx.db().acquire().await?, id).await
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct SavePipelineArgs {
    /// Only to update a pipeline: its id, slug or name. Omit to create one.
    id: Option<String>,
    /// Required to create.
    name: Option<String>,
    /// continuous (default: every deploy is a release) or scheduled (releases planned ahead).
    #[serde(rename = "type")]
    kind: Option<String>,
    /// Whether it deploys to production.
    is_production: Option<bool>,
    /// Teams to add (key, name or id).
    add_teams: Option<Vec<String>>,
    remove_teams: Option<Vec<String>>,
    /// Replace all teams with these.
    set_teams: Option<Vec<String>>,
}

async fn teams(ctx: &Ctx, values: &Option<Vec<String>>) -> ApiResult<Option<Vec<Uuid>>> {
    let Some(values) = values else { return Ok(None) };
    let mut ids = Vec::new();
    for value in values {
        ids.push(resolve::team(ctx, value).await?);
    }
    Ok(Some(ids))
}

async fn save_pipeline(ctx: Ctx, args: SavePipelineArgs) -> ApiResult<Value> {
    if args.set_teams.is_some() && (args.add_teams.is_some() || args.remove_teams.is_some()) {
        return Err(ApiError::bad_request("Pass setTeams or addTeams/removeTeams, not both"));
    }
    let existing = match &args.id {
        Some(reference) => Some(pipeline_id(&ctx, reference).await?),
        None => None,
    };
    let name = args.name.as_deref().map(|n| non_empty(n, "name")).transpose()?;
    let kind = args.kind.as_deref().map(|k| one_of(k, &PIPELINE_TYPES, "type")).transpose()?;
    let (set, add, remove) = (
        teams(&ctx, &args.set_teams).await?,
        teams(&ctx, &args.add_teams).await?,
        teams(&ctx, &args.remove_teams).await?,
    );
    let mut tx = ctx.db().begin().await?;
    let (id, before) = match existing {
        Some(id) => (id, Some(pipeline_value(&mut tx, id).await?)),
        None => {
            let name = name.clone().ok_or_else(|| ApiError::bad_request("name is required to create a pipeline"))?;
            let id: Uuid = sqlx::query_scalar(
                "INSERT INTO release_pipelines (workspace_id, slug_id, name) VALUES ($1, $2, $3) RETURNING id",
            )
            .bind(ctx.workspace)
            .bind(slug_id())
            .bind(&name)
            .fetch_one(&mut *tx)
            .await?;
            for (position, (name, kind, color)) in DEFAULT_STAGES.into_iter().enumerate() {
                sqlx::query(
                    "INSERT INTO release_stages (pipeline_id, name, type, color, position) VALUES ($1, $2, $3, $4, $5)",
                )
                .bind(id)
                .bind(name)
                .bind(kind)
                .bind(color)
                .bind(position as f64)
                .execute(&mut *tx)
                .await?;
            }
            (id, None)
        }
    };
    sqlx::query(
        "UPDATE release_pipelines SET name = coalesce($2, name), type = coalesce($3, type),
            is_production = coalesce($4, is_production), updated_at = now()
         WHERE id = $1",
    )
    .bind(id)
    .bind(&name)
    .bind(&kind)
    .bind(args.is_production)
    .execute(&mut *tx)
    .await?;
    if set.is_some() {
        sqlx::query("DELETE FROM release_pipeline_teams WHERE pipeline_id = $1").bind(id).execute(&mut *tx).await?;
    }
    if let Some(add) = set.as_ref().or(add.as_ref()) {
        sqlx::query(
            "INSERT INTO release_pipeline_teams (pipeline_id, team_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING",
        )
        .bind(id)
        .bind(add)
        .execute(&mut *tx)
        .await?;
    }
    if let Some(remove) = &remove {
        sqlx::query("DELETE FROM release_pipeline_teams WHERE pipeline_id = $1 AND team_id = ANY($2)")
            .bind(id)
            .bind(remove)
            .execute(&mut *tx)
            .await?;
    }
    let value = pipeline_value(&mut tx, id).await?;
    emit(&mut tx, &ctx, "ReleasePipeline", before, &value).await?;
    tx.commit().await?;
    Ok(value)
}

/// Emits `create` without a previous value, else `update` when something changed.
async fn emit(tx: &mut PgConnection, ctx: &Ctx, kind: &str, before: Option<Value>, value: &Value) -> ApiResult<()> {
    let Some(before) = before else {
        return events::emit(tx, ctx, kind, "create", value, None, None).await;
    };
    let from = changed_fields(&before, value);
    if from.as_object().is_none_or(|o| o.is_empty()) {
        return Ok(());
    }
    events::emit(tx, ctx, kind, "update", value, Some(from), None).await
}

async fn delete_pipeline(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = pipeline_id(&ctx, &args.id).await?;
    let mut tx = ctx.db().begin().await?;
    sqlx::query(
        "UPDATE release_pipelines SET archived_at = now(), updated_at = now() WHERE id = $1 AND archived_at IS NULL",
    )
    .bind(id)
    .execute(&mut *tx)
    .await?;
    let value = pipeline_value(&mut tx, id).await?;
    events::emit(&mut tx, &ctx, "ReleasePipeline", "remove", &value, None, None).await?;
    tx.commit().await?;
    Ok(value)
}

/* ------------------------------------------------------------------ */
/*                               Stages                               */
/* ------------------------------------------------------------------ */

/// A stage of the pipeline by id, name or type (the first of that type).
async fn stage(db: &mut PgConnection, pipeline: Uuid, value: &str) -> ApiResult<Uuid> {
    let rows: Vec<(Uuid, String, String)> = sqlx::query_as(
        "SELECT id, name, type FROM release_stages WHERE pipeline_id = $1 AND archived_at IS NULL ORDER BY position",
    )
    .bind(pipeline)
    .fetch_all(&mut *db)
    .await?;
    let value = value.trim();
    let found = rows
        .iter()
        .find(|r| Some(r.0) == as_uuid(value))
        .or_else(|| rows.iter().find(|r| r.1.eq_ignore_ascii_case(value)))
        .or_else(|| rows.iter().find(|r| r.2.eq_ignore_ascii_case(value)));
    if let Some(row) = found {
        return Ok(row.0);
    }
    let names: Vec<_> = rows.iter().map(|r| format!("{} ({})", r.1, r.2)).collect();
    Err(ApiError::bad_request(format!("The pipeline has no stage \"{value}\". Options: {}.", names.join(", "))))
}

/// A stage by id, with its pipeline, checking the caller sees the pipeline.
async fn stage_by_id(ctx: &Ctx, value: &str) -> ApiResult<(Uuid, Uuid)> {
    let not_found = || ApiError::not_found(format!("No release stage \"{}\"; pass its id", value.trim()));
    let id = as_uuid(value).ok_or_else(not_found)?;
    let pipeline: Option<Uuid> = sqlx::query_scalar(
        "SELECT s.pipeline_id FROM release_stages s JOIN release_pipelines p ON p.id = s.pipeline_id
         WHERE s.id = $1 AND p.workspace_id = $2",
    )
    .bind(id)
    .bind(ctx.workspace)
    .fetch_optional(ctx.db())
    .await?;
    let pipeline = pipeline.ok_or_else(not_found)?;
    check_pipeline_visible(ctx, pipeline, "release stage", value).await?;
    Ok((id, pipeline))
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct SaveStageArgs {
    /// Only to update a stage: its id. Omit to create one.
    id: Option<String>,
    /// The pipeline (id, slug or name). Required to create.
    pipeline: Option<String>,
    /// Required to create.
    name: Option<String>,
    /// planned, started, completed or canceled. Required to create.
    #[serde(rename = "type")]
    kind: Option<String>,
    /// Hex color like #f2c94c.
    color: Option<String>,
    /// Order in the pipeline, lowest first. New stages go last.
    position: Option<f64>,
}

async fn save_stage(ctx: Ctx, args: SaveStageArgs) -> ApiResult<Value> {
    let name = args.name.as_deref().map(|n| non_empty(n, "name")).transpose()?;
    let kind = args.kind.as_deref().map(|k| one_of(k, &STAGE_TYPES, "type")).transpose()?;
    let color = args.color.as_deref().map(check_color).transpose()?;
    let (id, pipeline) = match (&args.id, &args.pipeline) {
        (Some(id), _) => {
            let (id, pipeline) = stage_by_id(&ctx, id).await?;
            if let Some(other) = &args.pipeline
                && pipeline_id(&ctx, other).await? != pipeline
            {
                return Err(ApiError::bad_request("A stage can't move to another pipeline"));
            }
            (Some(id), pipeline)
        }
        (None, Some(pipeline)) => (None, pipeline_id(&ctx, pipeline).await?),
        (None, None) => return Err(ApiError::bad_request("pipeline is required to create a stage")),
    };
    let mut tx = ctx.db().begin().await?;
    let id = match id {
        Some(id) => id,
        None => {
            let name = name.clone().ok_or_else(|| ApiError::bad_request("name is required to create a stage"))?;
            let kind = kind.clone().ok_or_else(|| ApiError::bad_request("type is required to create a stage"))?;
            let default_color = DEFAULT_STAGES.iter().find(|s| s.1 == kind).map_or("#95a2b3", |s| s.2);
            sqlx::query_scalar(
                "INSERT INTO release_stages (pipeline_id, name, type, color, position)
                 VALUES ($1, $2, $3, $4, (SELECT coalesce(max(position), -1) + 1 FROM release_stages WHERE pipeline_id = $1))
                 RETURNING id",
            )
            .bind(pipeline)
            .bind(&name)
            .bind(&kind)
            .bind(color.clone().unwrap_or_else(|| default_color.to_string()))
            .fetch_one(&mut *tx)
            .await?
        }
    };
    sqlx::query(
        "UPDATE release_stages SET name = coalesce($2, name), type = coalesce($3, type), color = coalesce($4, color),
            position = coalesce($5, position)
         WHERE id = $1",
    )
    .bind(id)
    .bind(&name)
    .bind(&kind)
    .bind(&color)
    .bind(args.position)
    .execute(&mut *tx)
    .await?;
    let mut value: Value = sqlx::query_scalar(&format!("SELECT {STAGE_JSON} FROM release_stages s WHERE s.id = $1"))
        .bind(id)
        .fetch_one(&mut *tx)
        .await?;
    value["pipeline"] =
        sqlx::query_scalar("SELECT release_pipeline_ref($1)").bind(pipeline).fetch_one(&mut *tx).await?;
    tx.commit().await?;
    Ok(value)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct DeleteStageArgs {
    /// The stage's id.
    id: String,
    /// Where its releases go: another stage of the pipeline (name, type or id). Required when releases are in it.
    move_releases_to: Option<String>,
}

async fn delete_stage(ctx: Ctx, args: DeleteStageArgs) -> ApiResult<Value> {
    let (id, pipeline) = stage_by_id(&ctx, &args.id).await?;
    let mut tx = ctx.db().begin().await?;
    let (stages, releases): (i64, i64) = sqlx::query_as(
        "SELECT (SELECT count(*) FROM release_stages WHERE pipeline_id = $1 AND archived_at IS NULL),
                (SELECT count(*) FROM releases WHERE stage_id = $2)",
    )
    .bind(pipeline)
    .bind(id)
    .fetch_one(&mut *tx)
    .await?;
    if stages <= 1 {
        return Err(ApiError::bad_request("A pipeline needs at least one stage"));
    }
    match &args.move_releases_to {
        Some(target) => {
            let target = stage(&mut tx, pipeline, target).await?;
            if target == id {
                return Err(ApiError::bad_request("moveReleasesTo must be another stage"));
            }
            sqlx::query("UPDATE releases SET stage_id = $2, updated_at = now() WHERE stage_id = $1")
                .bind(id)
                .bind(target)
                .execute(&mut *tx)
                .await?;
        }
        None if releases > 0 => {
            return Err(ApiError::bad_request(format!(
                "{releases} release(s) are in this stage; pass moveReleasesTo with another stage of the pipeline"
            )));
        }
        None => {}
    }
    sqlx::query("DELETE FROM release_stages WHERE id = $1").bind(id).execute(&mut *tx).await?;
    tx.commit().await?;
    Ok(
        json!({ "id": id, "deleted": true, "releasesMoved": if args.move_releases_to.is_some() { releases } else { 0 } }),
    )
}

/* ------------------------------------------------------------------ */
/*                              Releases                              */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct ListReleasesArgs {
    /// Pipeline id, slug or name.
    pipeline: Option<String>,
    /// Stage id, type (planned, started, completed, canceled) or name (with pipeline).
    stage: Option<String>,
    /// Words in the name or version, or the start of the commit SHA.
    query: Option<String>,
    #[serde(default)]
    include_archived: bool,
    /// createdAt (default), updatedAt or targetDate.
    order_by: Option<String>,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list_releases(ctx: Ctx, args: ListReleasesArgs) -> ApiResult<Value> {
    let page = Page { limit: args.limit, cursor: args.cursor.clone() };
    let order = match args.order_by.as_deref().unwrap_or("createdAt") {
        "createdAt" => "r.created_at DESC",
        "updatedAt" => "r.updated_at DESC",
        "targetDate" => "r.target_date ASC NULLS LAST, r.created_at DESC",
        other => {
            return Err(ApiError::bad_request(format!(
                "orderBy \"{other}\" isn't one of createdAt, updatedAt, targetDate"
            )));
        }
    };
    let pipeline = match &args.pipeline {
        Some(pipeline) => Some(pipeline_id(&ctx, pipeline).await?),
        None => None,
    };
    let mut qb = QueryBuilder::new(format!("SELECT {RELEASE_JSON} FROM releases r WHERE r.workspace_id = "));
    qb.push_bind(ctx.workspace);
    push_pipeline_visible(&mut qb, &ctx, "r.pipeline_id").await?;
    if !args.include_archived {
        qb.push(" AND r.archived_at IS NULL");
    }
    if let Some(pipeline) = pipeline {
        qb.push(" AND r.pipeline_id = ");
        qb.push_bind(pipeline);
    }
    if let Some(value) = &args.stage {
        let lower = value.trim().to_lowercase();
        match (pipeline, as_uuid(value)) {
            (Some(pipeline), _) => {
                qb.push(" AND r.stage_id = ");
                qb.push_bind(stage(&mut *ctx.db().acquire().await?, pipeline, value).await?);
            }
            (None, Some(id)) => {
                qb.push(" AND r.stage_id = ");
                qb.push_bind(id);
            }
            (None, None) if STAGE_TYPES.contains(&lower.as_str()) => {
                qb.push(" AND EXISTS (SELECT 1 FROM release_stages s WHERE s.id = r.stage_id AND s.type = ");
                qb.push_bind(lower);
                qb.push(")");
            }
            (None, None) => {
                return Err(ApiError::bad_request(
                    "Name the pipeline too to filter by stage name, or pass a stage type or id",
                ));
            }
        }
    }
    if let Some(query) = args.query.as_deref().map(str::trim).filter(|q| !q.is_empty()) {
        let pattern = escape_like(query);
        qb.push(" AND (r.name ILIKE ");
        qb.push_bind(pattern.clone());
        qb.push(" OR r.version ILIKE ");
        qb.push_bind(pattern);
        qb.push(" OR r.commit_sha ILIKE ");
        qb.push_bind(format!("{}%", query.replace('%', "")));
        qb.push(")");
    }
    qb.push(format!(" ORDER BY {order}, r.id LIMIT "));
    qb.push_bind(page.limit() + 1);
    qb.push(" OFFSET ");
    qb.push_bind(page.offset()?);
    let nodes: Vec<Value> = qb.build_query_scalar().fetch_all(ctx.db()).await?;
    page.wrap(nodes)
}

async fn get_release(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = release_id(&ctx, &args.id).await?;
    let mut db = ctx.db().acquire().await?;
    let mut value = release_value(&mut db, id).await?;
    value["issues"] = sqlx::query_scalar(
        "SELECT coalesce(jsonb_agg(issue_ref(i.id) || jsonb_build_object('assignee', user_ref(i.assignee_id))
            ORDER BY t.key, i.number), '[]')
         FROM issue_releases ir JOIN issues i ON i.id = ir.issue_id JOIN teams t ON t.id = i.team_id
         WHERE ir.release_id = $1 AND i.trashed_at IS NULL AND (i.team_id = ANY($2)
            OR EXISTS (SELECT 1 FROM issue_shares sh WHERE sh.issue_id = i.id AND sh.user_id = $3))",
    )
    .bind(id)
    .bind(ctx.team_ids().await?)
    .bind(ctx.user)
    .fetch_one(&mut *db)
    .await?;
    value["releaseNotes"] = sqlx::query_scalar(
        "SELECT coalesce(jsonb_agg(jsonb_build_object('id', n.id, 'slugId', n.slug_id, 'title', n.title,
            'createdAt', n.created_at) ORDER BY n.created_at DESC), '[]')
         FROM release_notes n WHERE $1 = ANY(n.release_ids)",
    )
    .bind(id)
    .fetch_one(&mut *db)
    .await?;
    Ok(value)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct SaveReleaseArgs {
    /// Only to update a release: its id, slug or name. Omit to create one.
    id: Option<String>,
    /// Required to create.
    name: Option<String>,
    /// The pipeline (id, slug or name). Required to create; releases can't move between pipelines.
    pipeline: Option<String>,
    /// e.g. 1.4.0; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    version: Option<Option<String>>,
    /// Markdown; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    description: Option<Option<String>>,
    /// The commit the release was built from; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    commit_sha: Option<Option<String>>,
    /// Stage of the pipeline: name, type (planned, started, completed, canceled) or id.
    stage: Option<String>,
    /// YYYY-MM-DD; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    start_date: Option<Option<String>>,
    /// YYYY-MM-DD; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    target_date: Option<Option<String>>,
    /// ISO 8601 time it started; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    started_at: Option<Option<String>>,
    /// ISO 8601 time it shipped; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    completed_at: Option<Option<String>>,
    /// When it was created (ISO 8601), for imports. Only on create.
    created_at: Option<String>,
}

fn trimmed(value: Option<Option<String>>) -> Option<Option<String>> {
    value.map(|v| v.map(|v| v.trim().to_string()).filter(|v| !v.is_empty()))
}

fn dates(value: &Option<Option<String>>) -> ApiResult<Option<Option<NaiveDate>>> {
    match value {
        Some(Some(date)) => Ok(Some(Some(parse_date(date)?))),
        Some(None) => Ok(Some(None)),
        None => Ok(None),
    }
}

fn times(value: &Option<Option<String>>) -> ApiResult<Option<Option<DateTime<Utc>>>> {
    match value {
        Some(Some(at)) => Ok(Some(Some(parse_datetime(at)?))),
        Some(None) => Ok(Some(None)),
        None => Ok(None),
    }
}

async fn save_release(ctx: Ctx, args: SaveReleaseArgs) -> ApiResult<Value> {
    let existing = match &args.id {
        Some(reference) => Some(release_id(&ctx, reference).await?),
        None => None,
    };
    if existing.is_some() && args.created_at.is_some() {
        return Err(ApiError::bad_request("createdAt can only be set when creating a release"));
    }
    let pipeline = match (&args.pipeline, existing) {
        (Some(value), None) => pipeline_id(&ctx, value).await?,
        (None, None) => return Err(ApiError::bad_request("pipeline is required to create a release")),
        (value, Some(id)) => {
            let current: Uuid = sqlx::query_scalar("SELECT pipeline_id FROM releases WHERE id = $1")
                .bind(id)
                .fetch_one(ctx.db())
                .await?;
            if let Some(value) = value
                && pipeline_id(&ctx, value).await? != current
            {
                return Err(ApiError::bad_request("A release can't move to another pipeline"));
            }
            current
        }
    };
    let name = args.name.as_deref().map(|n| non_empty(n, "name")).transpose()?;
    let created_at = args.created_at.as_deref().map(parse_datetime).transpose()?;
    let (start_date, target_date) = (dates(&args.start_date)?, dates(&args.target_date)?);
    let (started_at, completed_at) = (times(&args.started_at)?, times(&args.completed_at)?);
    let (version, commit_sha) = (trimmed(args.version), trimmed(args.commit_sha));
    let mut tx = ctx.db().begin().await?;
    let new_stage = match (&args.stage, existing) {
        (Some(value), _) => Some(stage(&mut tx, pipeline, value).await?),
        (None, None) => Some(first_planned_stage(&mut tx, pipeline).await?),
        (None, Some(_)) => None,
    };
    let (id, before) = match existing {
        Some(id) => (id, Some(release_value(&mut tx, id).await?)),
        None => {
            let name = name.clone().ok_or_else(|| ApiError::bad_request("name is required to create a release"))?;
            let id: Uuid = sqlx::query_scalar(
                "INSERT INTO releases (workspace_id, pipeline_id, slug_id, name, creator_id, created_at, updated_at)
                 VALUES ($1, $2, $3, $4, $5, coalesce($6, now()), coalesce($6, now())) RETURNING id",
            )
            .bind(ctx.workspace)
            .bind(pipeline)
            .bind(slug_id())
            .bind(&name)
            .bind(ctx.user)
            .bind(created_at)
            .fetch_one(&mut *tx)
            .await?;
            (id, None)
        }
    };
    if let Some(new_stage) = new_stage {
        move_to_stage(&mut tx, id, new_stage).await?;
    }
    sqlx::query(
        "UPDATE releases SET
            name = coalesce($2, name),
            version = CASE WHEN $3 THEN $4 ELSE version END,
            description = CASE WHEN $5 THEN $6 ELSE description END,
            commit_sha = CASE WHEN $7 THEN $8 ELSE commit_sha END,
            start_date = CASE WHEN $9 THEN $10 ELSE start_date END,
            target_date = CASE WHEN $11 THEN $12 ELSE target_date END,
            started_at = CASE WHEN $13 THEN $14 ELSE started_at END,
            completed_at = CASE WHEN $15 THEN $16 ELSE completed_at END,
            updated_at = CASE WHEN $17 THEN now() ELSE updated_at END
         WHERE id = $1",
    )
    .bind(id)
    .bind(&name)
    .bind(version.is_some())
    .bind(version.flatten())
    .bind(args.description.is_some())
    .bind(args.description.flatten())
    .bind(commit_sha.is_some())
    .bind(commit_sha.flatten())
    .bind(start_date.is_some())
    .bind(start_date.flatten())
    .bind(target_date.is_some())
    .bind(target_date.flatten())
    .bind(started_at.is_some())
    .bind(started_at.flatten())
    .bind(completed_at.is_some())
    .bind(completed_at.flatten())
    .bind(existing.is_some())
    .execute(&mut *tx)
    .await?;
    let value = release_value(&mut tx, id).await?;
    emit(&mut tx, &ctx, "Release", before, &value).await?;
    tx.commit().await?;
    Ok(value)
}

async fn first_planned_stage(db: &mut PgConnection, pipeline: Uuid) -> ApiResult<Uuid> {
    let id: Option<Uuid> = sqlx::query_scalar(
        "SELECT id FROM release_stages WHERE pipeline_id = $1 AND archived_at IS NULL
         ORDER BY type <> 'planned', position LIMIT 1",
    )
    .bind(pipeline)
    .fetch_optional(&mut *db)
    .await?;
    id.ok_or_else(|| ApiError::bad_request("The pipeline has no stages; add one with save_release_stage"))
}

/// Moves a release to a stage, stamping or clearing startedAt, completedAt and canceledAt by the
/// stage's type. Staying in the same stage changes nothing.
async fn move_to_stage(db: &mut PgConnection, release: Uuid, stage: Uuid) -> ApiResult<()> {
    sqlx::query(
        "UPDATE releases r SET stage_id = s.id,
            started_at = CASE WHEN s.type = 'planned' THEN NULL
                WHEN s.type IN ('started', 'completed') THEN coalesce(r.started_at, now()) ELSE r.started_at END,
            completed_at = CASE WHEN s.type = 'completed' THEN coalesce(r.completed_at, now()) END,
            canceled_at = CASE WHEN s.type = 'canceled' THEN coalesce(r.canceled_at, now()) END
         FROM release_stages s
         WHERE r.id = $1 AND s.id = $2 AND r.stage_id IS DISTINCT FROM s.id",
    )
    .bind(release)
    .bind(stage)
    .execute(&mut *db)
    .await?;
    Ok(())
}

async fn delete_release(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = release_id(&ctx, &args.id).await?;
    let mut tx = ctx.db().begin().await?;
    sqlx::query("UPDATE releases SET archived_at = now(), updated_at = now() WHERE id = $1 AND archived_at IS NULL")
        .bind(id)
        .execute(&mut *tx)
        .await?;
    let value = release_value(&mut tx, id).await?;
    events::emit(&mut tx, &ctx, "Release", "remove", &value, None, None).await?;
    tx.commit().await?;
    Ok(value)
}

/* ------------------------------------------------------------------ */
/*                            Release notes                           */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct ListNotesArgs {
    /// Pipeline id, slug or name.
    pipeline: Option<String>,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list_notes(ctx: Ctx, args: ListNotesArgs) -> ApiResult<Value> {
    let page = Page { limit: args.limit, cursor: args.cursor.clone() };
    let mut qb = QueryBuilder::new(format!("SELECT {} FROM release_notes n WHERE n.workspace_id = ", note_json(false)));
    qb.push_bind(ctx.workspace);
    push_pipeline_visible(&mut qb, &ctx, "n.pipeline_id").await?;
    if let Some(pipeline) = &args.pipeline {
        qb.push(" AND n.pipeline_id = ");
        qb.push_bind(pipeline_id(&ctx, pipeline).await?);
    }
    qb.push(" ORDER BY n.created_at DESC, n.id LIMIT ");
    qb.push_bind(page.limit() + 1);
    qb.push(" OFFSET ");
    qb.push_bind(page.offset()?);
    let nodes: Vec<Value> = qb.build_query_scalar().fetch_all(ctx.db()).await?;
    page.wrap(nodes)
}

async fn get_note(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = note_id(&ctx, &args.id).await?;
    note_value(&mut *ctx.db().acquire().await?, id).await
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct SaveNoteArgs {
    /// Only to update release notes: their id or slug. Omit to create them.
    id: Option<String>,
    /// The pipeline (id, slug or name). Required to create.
    pipeline: Option<String>,
    /// Defaults to the newest release's name and version.
    title: Option<String>,
    /// Markdown.
    content: Option<String>,
    /// Edits to the current content instead of the whole text, applied in order, all or none.
    patch: Option<Vec<TextEdit>>,
    /// The releases covered (id, slug or name), replacing the current ones.
    releases: Option<Vec<String>>,
    /// With rangeToRelease: cover every release of the pipeline created from this one to that one, inclusive.
    range_from_release: Option<String>,
    range_to_release: Option<String>,
}

/// The releases a note covers, oldest first, checking they are in the pipeline.
async fn covered_releases(ctx: &Ctx, pipeline: Uuid, args: &SaveNoteArgs) -> ApiResult<Option<Vec<Uuid>>> {
    let in_pipeline = |value: &str| {
        let value = value.to_string();
        async move {
            let id = release_id(ctx, &value).await?;
            let of: Uuid = sqlx::query_scalar("SELECT pipeline_id FROM releases WHERE id = $1")
                .bind(id)
                .fetch_one(ctx.db())
                .await?;
            if of != pipeline {
                return Err(ApiError::bad_request(format!("The release \"{value}\" is in another pipeline")));
            }
            Ok(id)
        }
    };
    let range = match (&args.range_from_release, &args.range_to_release) {
        (Some(from), Some(to)) => Some((in_pipeline(from).await?, in_pipeline(to).await?)),
        (None, None) => None,
        _ => return Err(ApiError::bad_request("Pass both rangeFromRelease and rangeToRelease")),
    };
    let ids = match (&args.releases, range) {
        (Some(_), Some(_)) => return Err(ApiError::bad_request("Pass releases or a range, not both")),
        (None, None) => return Ok(None),
        (Some(values), None) => {
            let mut ids = Vec::new();
            for value in values {
                let id = in_pipeline(value).await?;
                if !ids.contains(&id) {
                    ids.push(id);
                }
            }
            ids
        }
        (None, Some((from, to))) => sqlx::query_scalar(
            "WITH bounds AS (SELECT min(created_at) AS low, max(created_at) AS high FROM releases WHERE id IN ($2, $3))
                 SELECT r.id FROM releases r, bounds b
                 WHERE r.pipeline_id = $1 AND r.created_at BETWEEN b.low AND b.high
                    AND (r.archived_at IS NULL OR r.id IN ($2, $3))
                 ORDER BY r.created_at",
        )
        .bind(pipeline)
        .bind(from)
        .bind(to)
        .fetch_all(ctx.db())
        .await?,
    };
    let ordered: Vec<Uuid> = sqlx::query_scalar("SELECT id FROM releases WHERE id = ANY($1) ORDER BY created_at, name")
        .bind(&ids)
        .fetch_all(ctx.db())
        .await?;
    Ok(Some(ordered))
}

async fn default_title(db: &mut PgConnection, releases: &[Uuid]) -> ApiResult<Option<String>> {
    let newest: Option<(String, Option<String>)> =
        sqlx::query_as("SELECT name, version FROM releases WHERE id = ANY($1) ORDER BY created_at DESC LIMIT 1")
            .bind(releases)
            .fetch_optional(&mut *db)
            .await?;
    Ok(newest.map(|(name, version)| match version {
        Some(version) if !name.contains(&version) => format!("{name} ({version})"),
        _ => name,
    }))
}

async fn save_note(ctx: Ctx, args: SaveNoteArgs) -> ApiResult<Value> {
    let existing = match &args.id {
        Some(reference) => Some(note_id(&ctx, reference).await?),
        None => None,
    };
    let pipeline = match (&args.pipeline, existing) {
        (Some(value), None) => pipeline_id(&ctx, value).await?,
        (None, None) => return Err(ApiError::bad_request("pipeline is required to create release notes")),
        (value, Some(id)) => {
            let current: Uuid = sqlx::query_scalar("SELECT pipeline_id FROM release_notes WHERE id = $1")
                .bind(id)
                .fetch_one(ctx.db())
                .await?;
            if let Some(value) = value
                && pipeline_id(&ctx, value).await? != current
            {
                return Err(ApiError::bad_request("Release notes can't move to another pipeline"));
            }
            current
        }
    };
    let releases = covered_releases(&ctx, pipeline, &args).await?;
    let mut title = args.title.as_deref().map(|t| non_empty(t, "title")).transpose()?;
    let mut tx = ctx.db().begin().await?;
    let (id, before, current_content) = match existing {
        Some(id) => {
            let before = note_value(&mut tx, id).await?;
            let content = before["content"].as_str().unwrap_or_default().to_string();
            (id, Some(before), content)
        }
        None => {
            if title.is_none() {
                title = default_title(&mut tx, releases.as_deref().unwrap_or_default()).await?;
            }
            let title = title.clone().ok_or_else(|| {
                ApiError::bad_request("Pass a title, or the releases the notes cover to name them after the newest")
            })?;
            let id: Uuid = sqlx::query_scalar(
                "INSERT INTO release_notes (workspace_id, pipeline_id, slug_id, title, creator_id)
                 VALUES ($1, $2, $3, $4, $5) RETURNING id",
            )
            .bind(ctx.workspace)
            .bind(pipeline)
            .bind(slug_id())
            .bind(&title)
            .bind(ctx.user)
            .fetch_one(&mut *tx)
            .await?;
            (id, None, String::new())
        }
    };
    let content = new_content(&current_content, args.content.clone(), args.patch.as_deref())?;
    sqlx::query(
        "UPDATE release_notes SET title = coalesce($2, title), content = coalesce($3, content),
            release_ids = coalesce($4, release_ids), updated_at = now()
         WHERE id = $1",
    )
    .bind(id)
    .bind(&title)
    .bind(&content)
    .bind(&releases)
    .execute(&mut *tx)
    .await?;
    let value = note_value(&mut tx, id).await?;
    emit(&mut tx, &ctx, "ReleaseNote", before, &value).await?;
    tx.commit().await?;
    Ok(value)
}

async fn delete_note(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = note_id(&ctx, &args.id).await?;
    let mut tx = ctx.db().begin().await?;
    let value = note_value(&mut tx, id).await?;
    sqlx::query("DELETE FROM release_notes WHERE id = $1").bind(id).execute(&mut *tx).await?;
    events::emit(&mut tx, &ctx, "ReleaseNote", "remove", &value, None, None).await?;
    tx.commit().await?;
    Ok(json!({ "id": id, "deleted": true }))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn note_references() {
        assert_eq!(id_or_slug("https://x.dev/acme/release-note/spring-launch-0a1b2c3d4e5f/"), "0a1b2c3d4e5f");
        assert_eq!(id_or_slug("0a1b2c3d4e5f"), "0a1b2c3d4e5f");
    }
}
