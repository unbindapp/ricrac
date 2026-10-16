//! Issue labels: workspace labels and team labels, label groups, retiring and deleting them.

use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Value, json};
use sqlx::PgConnection;
use uuid::Uuid;

use super::events;
use super::issues::changed_fields;
use super::resolve::{self, as_uuid};
use super::util::{Page, check_color, non_empty, nullable, pick_color};
use super::{Op, op};
use crate::ctx::{Access, Ctx};
use crate::error::{ApiError, ApiResult};

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "list_issue_labels",
            Access::Read,
            &[("GET", "/labels")],
            "List issue labels. With team: that team's labels and the workspace labels (the ones its issues can use); \
             without: workspace labels and the labels of every team you can see. Groups (isGroup) hold child labels; an \
             issue takes one child of a singleSelect group. Retired labels are left out unless includeRetired is set.",
            list,
        ),
        op(
            "get_issue_label",
            Access::Read,
            &[("GET", "/labels/{id}")],
            "Read an issue label by id or name (Group/Label for a label in a group; pass team to pick a team's label).",
            get,
        ),
        op(
            "save_issue_label",
            Access::Write,
            &[("POST", "/labels"), ("PATCH", "/labels/{id}")],
            "Create an issue label (omit id; name is required; with team it's that team's label, without it a workspace \
             label) or change one (id = id or name). Set isGroup to make a group that holds labels, and parent to put a \
             label in a group (null takes it out). Names are unique within their team (or the workspace) and group. \
             Omitted fields stay as they are; null clears a field.",
            save,
        ),
        op(
            "retire_issue_label",
            Access::Write,
            &[("POST", "/labels/{id}/retire")],
            "Retire a label (and the labels of a group): it stays on the issues that have it but can't be added anymore. \
             restore_issue_label brings it back.",
            retire,
        ),
        op(
            "restore_issue_label",
            Access::Write,
            &[("POST", "/labels/{id}/restore")],
            "Bring back a retired label (and the labels of a group).",
            restore,
        ),
        op(
            "delete_issue_label",
            Access::Write,
            &[("DELETE", "/labels/{id}")],
            "Delete a label for good and take it off every issue; deleting a group deletes its labels too. Prefer \
             retire_issue_label to keep it on existing issues.",
            delete,
        ),
    ]
}

const LABEL_JSON: &str = "jsonb_build_object(
    'id', l.id,
    'name', l.name,
    'color', l.color,
    'description', l.description,
    'isGroup', l.is_group,
    'groupType', CASE WHEN l.is_group THEN l.group_type END,
    'parent', CASE WHEN p.id IS NOT NULL THEN jsonb_build_object('id', p.id, 'name', p.name) END,
    'team', team_ref(l.team_id),
    'children', CASE WHEN l.is_group THEN (SELECT coalesce(jsonb_agg(label_ref(c.id) ORDER BY lower(c.name)), '[]')
        FROM issue_labels c WHERE c.parent_id = l.id) END,
    'issueCount', (SELECT count(DISTINCT ll.issue_id) FROM issue_label_links ll JOIN issues i ON i.id = ll.issue_id
        WHERE i.trashed_at IS NULL AND (ll.label_id = l.id
            OR (l.is_group AND ll.label_id IN (SELECT c.id FROM issue_labels c WHERE c.parent_id = l.id)))),
    'creator', user_ref(l.creator_id),
    'retiredAt', l.retired_at,
    'createdAt', l.created_at,
    'updatedAt', l.updated_at)";

const LABEL_FROM: &str = "FROM issue_labels l LEFT JOIN issue_labels p ON p.id = l.parent_id";

async fn label_value(db: &mut PgConnection, label: Uuid) -> ApiResult<Value> {
    let value: Option<Value> = sqlx::query_scalar(&format!("SELECT {LABEL_JSON} {LABEL_FROM} WHERE l.id = $1"))
        .bind(label)
        .fetch_optional(&mut *db)
        .await?;
    value.ok_or_else(|| ApiError::not_found("No such label"))
}

struct LabelRow {
    id: Uuid,
    name: String,
    parent: Option<String>,
    team: Option<Uuid>,
}

/// A label the caller can see (retired ones too) by id, name or Group/Name. With a team, its
/// labels and the workspace labels are searched and the team's label wins a tie.
async fn find(ctx: &Ctx, value: &str, team: Option<Uuid>) -> ApiResult<Uuid> {
    let teams = match team {
        Some(team) => vec![team],
        None => ctx.team_ids().await?.to_vec(),
    };
    let rows: Vec<(Uuid, String, Option<String>, Option<Uuid>)> = sqlx::query_as(
        "SELECT l.id, l.name, p.name, l.team_id FROM issue_labels l LEFT JOIN issue_labels p ON p.id = l.parent_id
         WHERE l.workspace_id = $1 AND (l.team_id IS NULL OR l.team_id = ANY($2)) ORDER BY lower(l.name)",
    )
    .bind(ctx.workspace)
    .bind(&teams)
    .fetch_all(ctx.db())
    .await?;
    let rows: Vec<LabelRow> =
        rows.into_iter().map(|(id, name, parent, team)| LabelRow { id, name, parent, team }).collect();
    let value = value.trim();
    if let Some(row) = rows.iter().find(|r| Some(r.id) == as_uuid(value)) {
        return Ok(row.id);
    }
    let (group, name) = match value.split_once('/') {
        Some((g, n)) => (Some(g.trim()), n.trim()),
        None => (None, value),
    };
    let matches: Vec<&LabelRow> = rows
        .iter()
        .filter(|r| {
            r.name.eq_ignore_ascii_case(name)
                && group.is_none_or(|g| r.parent.as_deref().is_some_and(|p| p.eq_ignore_ascii_case(g)))
        })
        .collect();
    let team_matches: Vec<&&LabelRow> = matches.iter().filter(|r| team.is_some() && r.team == team).collect();
    match (matches.as_slice(), team_matches.as_slice()) {
        ([one], _) => Ok(one.id),
        (_, [one]) => Ok(one.id),
        ([], _) => {
            let names: Vec<String> = rows
                .iter()
                .take(30)
                .map(|r| match &r.parent {
                    Some(p) => format!("{p}/{}", r.name),
                    None => r.name.clone(),
                })
                .collect();
            Err(ApiError::bad_request(format!("No label \"{value}\". Options: {}.", names.join(", "))))
        }
        _ => Err(ApiError::bad_request(format!(
            "\"{value}\" matches several labels; pass team, name it as Group/{name}, or use its id"
        ))),
    }
}

async fn label_team(db: &mut PgConnection, label: Uuid) -> ApiResult<Option<Uuid>> {
    Ok(sqlx::query_scalar("SELECT team_id FROM issue_labels WHERE id = $1").bind(label).fetch_one(&mut *db).await?)
}

async fn team_arg(ctx: &Ctx, team: Option<&str>) -> ApiResult<Option<Uuid>> {
    match team {
        Some(team) => Ok(Some(resolve::team(ctx, team).await?)),
        None => Ok(None),
    }
}

/* ------------------------------------------------------------------ */
/*                                Read                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ListArgs {
    /// Team key, name or id: its labels and the workspace labels.
    team: Option<String>,
    /// Also list retired labels.
    #[serde(default)]
    include_retired: bool,
    /// Part of a label's name.
    query: Option<String>,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list(ctx: Ctx, args: ListArgs) -> ApiResult<Value> {
    let page = Page { limit: args.limit, cursor: args.cursor };
    let teams = match team_arg(&ctx, args.team.as_deref()).await? {
        Some(team) => vec![team],
        None => ctx.team_ids().await?.to_vec(),
    };
    let query = args.query.as_deref().map(str::trim).filter(|q| !q.is_empty()).map(|q| format!("%{q}%"));
    let sql = format!(
        "SELECT {LABEL_JSON} {LABEL_FROM} WHERE l.workspace_id = $1
         AND (l.team_id IS NULL OR l.team_id IN (SELECT id FROM teams WHERE id = ANY($2) AND archived_at IS NULL))
         AND ($3 OR l.retired_at IS NULL) AND ($4::text IS NULL OR l.name ILIKE $4 OR p.name ILIKE $4)
         ORDER BY lower(coalesce(p.name, l.name)), l.parent_id IS NOT NULL, lower(l.name), l.team_id NULLS FIRST
         LIMIT $5 OFFSET $6"
    );
    let nodes: Vec<Value> = sqlx::query_scalar(&sql)
        .bind(ctx.workspace)
        .bind(&teams)
        .bind(args.include_retired)
        .bind(query)
        .bind(page.limit() + 1)
        .bind(page.offset()?)
        .fetch_all(ctx.db())
        .await?;
    page.wrap(nodes)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct LabelArgs {
    /// The label's id or name (Group/Label for a label in a group).
    id: String,
    /// Team key, name or id, to pick between labels of the same name.
    team: Option<String>,
}

async fn get(ctx: Ctx, args: LabelArgs) -> ApiResult<Value> {
    let team = team_arg(&ctx, args.team.as_deref()).await?;
    let label = find(&ctx, &args.id, team).await?;
    let mut db = ctx.db().acquire().await?;
    label_value(&mut db, label).await
}

/* ------------------------------------------------------------------ */
/*                                Save                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SaveArgs {
    /// Only to change a label: its id or name (Group/Label for a label in a group). Omit to create one.
    id: Option<String>,
    /// Team key, name or id. On create, makes it a team label (omit for a workspace label); on update, picks between
    /// labels of the same name.
    team: Option<String>,
    /// Required to create.
    name: Option<String>,
    /// Hex color like #eb5757; picked for you on create when omitted.
    color: Option<String>,
    /// Null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    description: Option<Option<String>>,
    /// A group holds labels and can't be put on issues itself.
    is_group: Option<bool>,
    /// For groups: singleSelect (an issue takes one of its labels, the default) or multiSelect.
    group_type: Option<String>,
    /// The group (name or id) to put the label in; null takes it out of its group. A team label's group is a
    /// workspace group or a group of the same team.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    parent: Option<Option<String>>,
}

struct Label {
    team: Option<Uuid>,
    parent: Option<Uuid>,
    name: String,
    is_group: bool,
    group_type: String,
}

async fn load(db: &mut PgConnection, label: Uuid) -> ApiResult<Label> {
    let (team, parent, name, is_group, group_type): (Option<Uuid>, Option<Uuid>, String, bool, String) =
        sqlx::query_as(
            "SELECT team_id, parent_id, name, is_group, group_type FROM issue_labels WHERE id = $1 FOR UPDATE",
        )
        .bind(label)
        .fetch_one(&mut *db)
        .await?;
    Ok(Label { team, parent, name, is_group, group_type })
}

async fn save(ctx: Ctx, args: SaveArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let team = team_arg(&ctx, args.team.as_deref()).await?;
    let mut tx = ctx.db().begin().await?;
    let (label, before) = match &args.id {
        Some(reference) => {
            let label = find(&ctx, reference, team).await?;
            (label, Some(label_value(&mut tx, label).await?))
        }
        None => {
            let name = non_empty(args.name.as_deref().unwrap_or_default(), "name")?;
            let color = args.color.clone().unwrap_or_else(|| pick_color(&name));
            let label: Uuid = sqlx::query_scalar(
                "INSERT INTO issue_labels (workspace_id, team_id, name, color, creator_id) VALUES ($1, $2, $3, $4, $5)
                 RETURNING id",
            )
            .bind(ctx.workspace)
            .bind(team)
            .bind(&name)
            .bind(check_color(&color)?)
            .bind(ctx.user)
            .fetch_one(&mut *tx)
            .await?;
            (label, None)
        }
    };
    apply(&ctx, &mut tx, label, args).await?;
    let value = label_value(&mut tx, label).await?;
    let team = label_team(&mut tx, label).await?;
    match before {
        Some(before) => {
            let updated_from = changed_fields(&before, &value);
            if updated_from.as_object().is_some_and(|o| !o.is_empty()) {
                sqlx::query("UPDATE issue_labels SET updated_at = now() WHERE id = $1")
                    .bind(label)
                    .execute(&mut *tx)
                    .await?;
                events::emit(&mut tx, &ctx, "IssueLabel", "update", &value, Some(updated_from), team).await?;
            }
        }
        None => events::emit(&mut tx, &ctx, "IssueLabel", "create", &value, None, team).await?,
    }
    tx.commit().await?;
    Ok(value)
}

async fn apply(ctx: &Ctx, tx: &mut PgConnection, label: Uuid, args: SaveArgs) -> ApiResult<()> {
    let current = load(tx, label).await?;
    let name = match &args.name {
        Some(name) => non_empty(name, "name")?,
        None => current.name.clone(),
    };
    let parent = match &args.parent {
        Some(Some(reference)) => Some(check_parent(ctx, tx, label, current.team, reference).await?),
        Some(None) => None,
        None => current.parent,
    };
    let is_group = args.is_group.unwrap_or(current.is_group);
    if is_group && !current.is_group {
        let used: bool = sqlx::query_scalar("SELECT EXISTS (SELECT 1 FROM issue_label_links WHERE label_id = $1)")
            .bind(label)
            .fetch_one(&mut *tx)
            .await?;
        crate::ensure!(!used, "{name} is on issues, and a group can't be; create a new group instead");
    }
    if !is_group && current.is_group {
        let has_children: bool = sqlx::query_scalar("SELECT EXISTS (SELECT 1 FROM issue_labels WHERE parent_id = $1)")
            .bind(label)
            .fetch_one(&mut *tx)
            .await?;
        crate::ensure!(!has_children, "{name} still holds labels; move or delete them first");
    }
    let group_type = match &args.group_type {
        Some(kind) => {
            crate::ensure!(is_group, "groupType only applies to groups (isGroup: true)");
            crate::ensure!(
                matches!(kind.as_str(), "singleSelect" | "multiSelect"),
                "groupType is singleSelect or multiSelect"
            );
            kind.clone()
        }
        None => current.group_type.clone(),
    };
    let taken: bool = sqlx::query_scalar(
        "SELECT EXISTS (SELECT 1 FROM issue_labels WHERE workspace_id = $1 AND team_id IS NOT DISTINCT FROM $2
         AND parent_id IS NOT DISTINCT FROM $3 AND lower(name) = lower($4) AND id <> $5)",
    )
    .bind(ctx.workspace)
    .bind(current.team)
    .bind(parent)
    .bind(&name)
    .bind(label)
    .fetch_one(&mut *tx)
    .await?;
    let place = if parent.is_some() {
        "this group"
    } else if current.team.is_some() {
        "this team"
    } else {
        "the workspace"
    };
    crate::ensure!(!taken, "There already is a label named \"{name}\" in {place}");
    sqlx::query("UPDATE issue_labels SET name = $2, parent_id = $3, is_group = $4, group_type = $5 WHERE id = $1")
        .bind(label)
        .bind(&name)
        .bind(parent)
        .bind(is_group)
        .bind(&group_type)
        .execute(&mut *tx)
        .await?;
    if let Some(color) = &args.color {
        sqlx::query("UPDATE issue_labels SET color = $2 WHERE id = $1")
            .bind(label)
            .bind(check_color(color)?)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(description) = args.description {
        let description = description.map(|d| d.trim().to_string()).filter(|d| !d.is_empty());
        sqlx::query("UPDATE issue_labels SET description = $2 WHERE id = $1")
            .bind(label)
            .bind(description)
            .execute(&mut *tx)
            .await?;
    }
    for group in [is_group.then_some(label), parent].into_iter().flatten() {
        check_single_select(tx, group).await?;
    }
    Ok(())
}

/// The group `reference` names, if `label` may go in it: same team or a workspace group, and not
/// one of its own descendants.
async fn check_parent(
    ctx: &Ctx,
    tx: &mut PgConnection,
    label: Uuid,
    team: Option<Uuid>,
    reference: &str,
) -> ApiResult<Uuid> {
    let parent = find(ctx, reference, team).await?;
    let (is_group, parent_team, name): (bool, Option<Uuid>, String) =
        sqlx::query_as("SELECT is_group, team_id, name FROM issue_labels WHERE id = $1")
            .bind(parent)
            .fetch_one(&mut *tx)
            .await?;
    crate::ensure!(is_group, "{name} isn't a group; make it one with isGroup: true");
    crate::ensure!(
        parent_team.is_none() || parent_team == team,
        "A label's group must be a workspace group or a group of the same team"
    );
    let loops: bool = sqlx::query_scalar(
        "WITH RECURSIVE up AS (SELECT id, parent_id FROM issue_labels WHERE id = $1
         UNION SELECT l.id, l.parent_id FROM issue_labels l JOIN up ON l.id = up.parent_id)
         SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)",
    )
    .bind(parent)
    .bind(label)
    .fetch_one(&mut *tx)
    .await?;
    crate::ensure!(!loops, "A label can't be in its own group or in a group inside it");
    Ok(parent)
}

/// Issues may have at most one label of a single-select group.
async fn check_single_select(tx: &mut PgConnection, group: Uuid) -> ApiResult<()> {
    let clash: Option<String> = sqlx::query_scalar(
        "SELECT g.name FROM issue_labels g JOIN issue_labels c ON c.parent_id = g.id
         JOIN issue_label_links ll ON ll.label_id = c.id
         WHERE g.id = $1 AND g.group_type = 'singleSelect' GROUP BY g.name, ll.issue_id HAVING count(*) > 1 LIMIT 1",
    )
    .bind(group)
    .fetch_optional(&mut *tx)
    .await?;
    match clash {
        Some(group) => Err(ApiError::bad_request(format!(
            "Some issues would have several labels of the single-select group {group}; make it multiSelect or remove them first"
        ))),
        None => Ok(()),
    }
}

/* ------------------------------------------------------------------ */
/*                       Retire, restore, delete                      */
/* ------------------------------------------------------------------ */

async fn set_retired(ctx: &Ctx, args: LabelArgs, retired: bool) -> ApiResult<Value> {
    ctx.require_member()?;
    let team = team_arg(ctx, args.team.as_deref()).await?;
    let label = find(ctx, &args.id, team).await?;
    let mut tx = ctx.db().begin().await?;
    let before = label_value(&mut tx, label).await?;
    sqlx::query(
        "UPDATE issue_labels SET retired_at = CASE WHEN $2 THEN coalesce(retired_at, now()) END, updated_at = now()
         WHERE id = $1 OR parent_id = $1",
    )
    .bind(label)
    .bind(retired)
    .execute(&mut *tx)
    .await?;
    let value = label_value(&mut tx, label).await?;
    let team = label_team(&mut tx, label).await?;
    events::emit(&mut tx, ctx, "IssueLabel", "update", &value, Some(changed_fields(&before, &value)), team).await?;
    tx.commit().await?;
    Ok(value)
}

async fn retire(ctx: Ctx, args: LabelArgs) -> ApiResult<Value> {
    set_retired(&ctx, args, true).await
}

async fn restore(ctx: Ctx, args: LabelArgs) -> ApiResult<Value> {
    set_retired(&ctx, args, false).await
}

async fn delete(ctx: Ctx, args: LabelArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let team = team_arg(&ctx, args.team.as_deref()).await?;
    let label = find(&ctx, &args.id, team).await?;
    let mut tx = ctx.db().begin().await?;
    let value = label_value(&mut tx, label).await?;
    let team = label_team(&mut tx, label).await?;
    let issues: i64 = sqlx::query_scalar(
        "SELECT count(DISTINCT issue_id) FROM issue_label_links
         WHERE label_id IN (SELECT id FROM issue_labels WHERE id = $1 OR parent_id = $1)",
    )
    .bind(label)
    .fetch_one(&mut *tx)
    .await?;
    sqlx::query("DELETE FROM issue_labels WHERE id = $1").bind(label).execute(&mut *tx).await?;
    events::emit(&mut tx, &ctx, "IssueLabel", "remove", &value, None, team).await?;
    tx.commit().await?;
    Ok(json!({ "deleted": label, "removedFromIssues": issues }))
}
