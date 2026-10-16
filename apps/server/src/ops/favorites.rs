//! Favorites: the caller's own shortcuts to issues, projects, views, documents and more, in
//! optional folders.

use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Value, json};
use sqlx::{PgConnection, Postgres, QueryBuilder};
use uuid::Uuid;

use super::resolve::{self, as_uuid};
use super::util::Page;
use super::{Op, op};
use crate::ctx::{Access, Ctx};
use crate::error::{ApiError, ApiResult};

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "list_favorites",
            Access::Read,
            &[("GET", "/favorites")],
            "List the caller's favorites by folder and order, each with what it points at. Favorites of things that \
             were deleted or can no longer be seen are left out.",
            list,
        ),
        op(
            "add_favorite",
            Access::Write,
            &[("POST", "/favorites")],
            "Add something to the caller's favorites: an issue, project, initiative, cycle, customView, document, \
             label, team, user or release. Adding a favorite again only updates its folder or order.",
            add,
        ),
        op(
            "remove_favorite",
            Access::Write,
            &[("DELETE", "/favorites/{id}")],
            "Remove one of the caller's favorites, by the favorite's id, or by type and target.",
            remove,
        ),
    ]
}

/// The API's types and how they are stored.
const TYPES: [(&str, &str); 10] = [
    ("issue", "issue"),
    ("project", "project"),
    ("initiative", "initiative"),
    ("cycle", "cycle"),
    ("customView", "custom_view"),
    ("document", "document"),
    ("label", "label"),
    ("team", "team"),
    ("user", "user"),
    ("release", "release"),
];

fn stored_type(kind: &str) -> ApiResult<&'static str> {
    TYPES.iter().find(|(api, _)| api.eq_ignore_ascii_case(kind.trim())).map(|(_, stored)| *stored).ok_or_else(|| {
        let names: Vec<&str> = TYPES.iter().map(|(api, _)| *api).collect();
        ApiError::bad_request(format!("type \"{kind}\" isn't one of {}", names.join(", ")))
    })
}

const FAVORITE_JSON: &str = "jsonb_build_object(
    'id', f.id,
    'type', CASE f.type WHEN 'custom_view' THEN 'customView' ELSE f.type END,
    'folder', f.folder,
    'sortOrder', f.sort_order,
    'createdAt', f.created_at,
    'target', CASE f.type
        WHEN 'issue' THEN issue_ref(f.target_id)
        WHEN 'project' THEN project_ref(f.target_id)
        WHEN 'initiative' THEN initiative_ref(f.target_id)
        WHEN 'cycle' THEN cycle_ref(f.target_id) || jsonb_build_object('team', (SELECT team_ref(c.team_id) FROM cycles c WHERE c.id = f.target_id))
        WHEN 'custom_view' THEN (SELECT jsonb_build_object('id', v.id, 'name', v.name, 'slugId', v.slug_id) FROM custom_views v WHERE v.id = f.target_id)
        WHEN 'document' THEN document_ref(f.target_id)
        WHEN 'label' THEN label_ref(f.target_id)
        WHEN 'team' THEN team_ref(f.target_id)
        WHEN 'user' THEN user_ref(f.target_id)
        WHEN 'release' THEN release_ref(f.target_id)
    END)";

/// `SELECT <favorite JSON> FROM favorites f WHERE <the caller's, still there and visible>`.
async fn visible_favorites<'a>(ctx: &Ctx) -> ApiResult<QueryBuilder<'a, Postgres>> {
    let teams = ctx.team_ids().await?.to_vec();
    let mut qb = QueryBuilder::new(format!("SELECT {FAVORITE_JSON} FROM favorites f WHERE f.workspace_id = "));
    qb.push_bind(ctx.workspace);
    qb.push(" AND f.user_id = ");
    qb.push_bind(ctx.user);
    qb.push(
        " AND CASE f.type
        WHEN 'issue' THEN EXISTS (SELECT 1 FROM issues i WHERE i.id = f.target_id AND i.trashed_at IS NULL AND (i.team_id = ANY(",
    );
    qb.push_bind(teams.clone());
    qb.push(") OR EXISTS (SELECT 1 FROM issue_shares s WHERE s.issue_id = i.id AND s.user_id = ");
    qb.push_bind(ctx.user);
    qb.push(
        "))) WHEN 'project' THEN EXISTS (SELECT 1 FROM projects p JOIN project_teams pt ON pt.project_id = p.id
            WHERE p.id = f.target_id AND p.trashed_at IS NULL AND pt.team_id = ANY(",
    );
    qb.push_bind(teams.clone());
    qb.push(
        ")) WHEN 'initiative' THEN EXISTS (SELECT 1 FROM initiatives n WHERE n.id = f.target_id AND n.trashed_at IS NULL)
        WHEN 'cycle' THEN EXISTS (SELECT 1 FROM cycles c WHERE c.id = f.target_id AND c.team_id = ANY(",
    );
    qb.push_bind(teams.clone());
    qb.push(
        ")) WHEN 'custom_view' THEN EXISTS (SELECT 1 FROM custom_views v WHERE v.id = f.target_id
            AND (v.shared OR v.owner_id = f.user_id) AND (v.team_id IS NULL OR v.team_id = ANY(",
    );
    qb.push_bind(teams.clone());
    qb.push(
        "))) WHEN 'document' THEN EXISTS (SELECT 1 FROM documents d WHERE d.id = f.target_id AND d.trashed_at IS NULL",
    );
    super::documents::push_visible(&mut qb, ctx, &teams, "d");
    qb.push(
        ") WHEN 'label' THEN EXISTS (SELECT 1 FROM issue_labels l WHERE l.id = f.target_id AND (l.team_id IS NULL OR l.team_id = ANY(",
    );
    qb.push_bind(teams.clone());
    qb.push(")))  WHEN 'team' THEN f.target_id = ANY(");
    qb.push_bind(teams);
    qb.push(
        ") WHEN 'user' THEN EXISTS (SELECT 1 FROM workspace_members m WHERE m.user_id = f.target_id
            AND m.workspace_id = f.workspace_id AND m.active)
        WHEN 'release' THEN EXISTS (SELECT 1 FROM releases r WHERE r.id = f.target_id)
        ELSE FALSE END",
    );
    Ok(qb)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ListFavoritesArgs {
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list(ctx: Ctx, args: ListFavoritesArgs) -> ApiResult<Value> {
    let page = Page { limit: args.limit, cursor: args.cursor };
    let mut qb = visible_favorites(&ctx).await?;
    qb.push(" ORDER BY f.folder NULLS FIRST, f.sort_order, f.created_at LIMIT ");
    qb.push_bind(page.limit() + 1);
    qb.push(" OFFSET ");
    qb.push_bind(page.offset()?);
    let nodes: Vec<Value> = qb.build_query_scalar().fetch_all(ctx.db()).await?;
    page.wrap(nodes)
}

/// The id of what `target` names, for a favorite of `kind` (as stored).
async fn target(ctx: &Ctx, kind: &str, target: &str, team: Option<&str>) -> ApiResult<Uuid> {
    let team = match team {
        Some(team) => Some(resolve::team(ctx, team).await?),
        None => None,
    };
    match kind {
        "issue" => resolve::issue(ctx, target).await,
        "project" => resolve::project(ctx, target).await,
        "initiative" => resolve::initiative(ctx, target).await,
        "cycle" => resolve::cycle(ctx, team, target).await,
        "custom_view" => super::views::find(ctx, target).await,
        "document" => resolve::document(ctx, target).await,
        "label" => Ok(resolve::labels(ctx, team, &[target.to_string()]).await?[0]),
        "team" => resolve::team(ctx, target).await,
        "user" => resolve::user(ctx, target).await,
        _ => resolve::release(ctx, target).await,
    }
}

async fn favorite_value(db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    let sql = format!("SELECT {FAVORITE_JSON} FROM favorites f WHERE f.id = $1");
    Ok(sqlx::query_scalar(&sql).bind(id).fetch_one(&mut *db).await?)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct AddFavoriteArgs {
    /// issue, project, initiative, cycle, customView, document, label, team, user or release.
    #[serde(rename = "type")]
    kind: String,
    /// What to favorite: its id, or the reference its type takes (ENG-123, a name, a slug, "me", ...).
    target: String,
    /// Team key, name or id, to find a cycle by number or name, or a team's label.
    team: Option<String>,
    /// Folder name to put it in.
    folder: Option<String>,
    /// Position among the favorites (lower is higher up); after the others by default.
    sort_order: Option<f64>,
}

async fn add(ctx: Ctx, args: AddFavoriteArgs) -> ApiResult<Value> {
    let kind = stored_type(&args.kind)?;
    let target = target(&ctx, kind, &args.target, args.team.as_deref()).await?;
    let folder = args.folder.as_deref().map(str::trim).filter(|f| !f.is_empty());
    let mut tx = ctx.db().begin().await?;
    let id: Uuid = sqlx::query_scalar(
        "INSERT INTO favorites (workspace_id, user_id, type, target_id, folder, sort_order)
         VALUES ($1, $2, $3, $4, $5, coalesce($6, (SELECT coalesce(max(sort_order), 0) + 1000 FROM favorites WHERE user_id = $2 AND workspace_id = $1)))
         ON CONFLICT (user_id, type, target_id) DO UPDATE SET
            folder = coalesce(EXCLUDED.folder, favorites.folder),
            sort_order = coalesce($6, favorites.sort_order)
         RETURNING id",
    )
    .bind(ctx.workspace)
    .bind(ctx.user)
    .bind(kind)
    .bind(target)
    .bind(folder)
    .bind(args.sort_order)
    .fetch_one(&mut *tx)
    .await?;
    let value = favorite_value(&mut tx, id).await?;
    tx.commit().await?;
    Ok(value)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct RemoveFavoriteArgs {
    /// The favorite's id; or, with type, what it points at.
    id: Option<String>,
    /// With target (or id): the type of what the favorite points at.
    #[serde(rename = "type")]
    kind: Option<String>,
    /// With type: what the favorite points at (id or reference).
    target: Option<String>,
    /// Team key, name or id, to find a cycle by number or name, or a team's label.
    team: Option<String>,
}

async fn remove(ctx: Ctx, args: RemoveFavoriteArgs) -> ApiResult<Value> {
    let by_id: Option<Uuid> = match args.id.as_deref().and_then(as_uuid) {
        Some(id) if args.kind.is_none() => {
            sqlx::query_scalar(
                "DELETE FROM favorites WHERE id = $1 AND user_id = $2 AND workspace_id = $3 RETURNING id",
            )
            .bind(id)
            .bind(ctx.user)
            .bind(ctx.workspace)
            .fetch_optional(ctx.db())
            .await?
        }
        _ => None,
    };
    if let Some(id) = by_id {
        return Ok(json!({ "removed": true, "id": id }));
    }
    let (Some(kind), Some(reference)) = (&args.kind, args.target.as_ref().or(args.id.as_ref())) else {
        return Err(ApiError::not_found("No such favorite; pass its id, or type and target"));
    };
    let kind = stored_type(kind)?;
    let target = target(&ctx, kind, reference, args.team.as_deref()).await?;
    let removed: Option<Uuid> = sqlx::query_scalar(
        "DELETE FROM favorites WHERE user_id = $1 AND workspace_id = $2 AND type = $3 AND target_id = $4 RETURNING id",
    )
    .bind(ctx.user)
    .bind(ctx.workspace)
    .bind(kind)
    .bind(target)
    .fetch_optional(ctx.db())
    .await?;
    match removed {
        Some(id) => Ok(json!({ "removed": true, "id": id })),
        None => Err(ApiError::not_found("That isn't one of your favorites")),
    }
}
