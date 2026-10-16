//! Workflow states (issue statuses) of a team: listing, creating, changing and archiving them.

use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Value, json};
use sqlx::PgConnection;
use uuid::Uuid;

use super::events;
use super::issues::{self, changed_fields};
use super::resolve::{self, as_uuid};
use super::teams::{STATE_ORDER, require_can_edit};
use super::util::{Page, check_color, non_empty, nullable};
use super::{Op, op};
use crate::ctx::{Access, Ctx};
use crate::error::{ApiError, ApiResult};

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "list_issue_statuses",
            Access::Read,
            &[("GET", "/teams/{team}/statuses")],
            "List a team's workflow statuses in board order (triage, backlog, unstarted, started, completed, canceled, \
             then position): name, type, color, how many issues are in each and which is the default for new issues.",
            list,
        ),
        op(
            "get_issue_status",
            Access::Read,
            &[("GET", "/statuses/{id}")],
            "Read a workflow status by id, or by name or type together with its team.",
            get,
        ),
        op(
            "save_issue_status",
            Access::Write,
            &[("POST", "/statuses"), ("PATCH", "/statuses/{id}")],
            "Create a workflow status (omit id; team, name and type are required) or change one (id, or name with team). \
             Types: triage, backlog, unstarted, started, completed, canceled. Names are unique within the team. The type \
             can only change while no issues are in the status. Omitted fields stay as they are; null clears the \
             description.",
            save,
        ),
        op(
            "delete_issue_status",
            Access::Write,
            &[("DELETE", "/statuses/{id}")],
            "Archive a workflow status. If issues are in it, pass moveIssuesTo (a status of the same team) and they move \
             there. If it is the team's default or auto-close status, that setting moves to moveIssuesTo too (which then \
             must be backlog/unstarted, or canceled). A team keeps at least one open, one completed and one canceled \
             status; turn triage off with save_team rather than deleting the Triage status.",
            delete,
        ),
    ]
}

pub const STATE_TYPES: [&str; 6] = ["triage", "backlog", "unstarted", "started", "completed", "canceled"];

fn default_color(kind: &str) -> &'static str {
    match kind {
        "triage" => "#fc7840",
        "backlog" => "#bec2c8",
        "unstarted" => "#e2e2e2",
        "started" => "#f2c94c",
        "completed" => "#5e6ad2",
        _ => "#95a2b3",
    }
}

const STATE_JSON: &str = "jsonb_build_object(
    'id', s.id,
    'name', s.name,
    'type', s.type,
    'color', s.color,
    'description', s.description,
    'position', s.position,
    'team', team_ref(s.team_id),
    'issueCount', (SELECT count(*) FROM issues i WHERE i.state_id = s.id AND i.trashed_at IS NULL),
    'isDefault', s.id IS NOT DISTINCT FROM t.default_issue_state_id,
    'isAutoCloseState', s.id IS NOT DISTINCT FROM t.auto_close_state_id,
    'archivedAt', s.archived_at,
    'createdAt', s.created_at,
    'updatedAt', s.updated_at)";

const STATE_FROM: &str = "FROM workflow_states s JOIN teams t ON t.id = s.team_id";

async fn state_value(db: &mut PgConnection, state: Uuid) -> ApiResult<Value> {
    let value: Option<Value> = sqlx::query_scalar(&format!("SELECT {STATE_JSON} {STATE_FROM} WHERE s.id = $1"))
        .bind(state)
        .fetch_optional(&mut *db)
        .await?;
    value.ok_or_else(|| ApiError::not_found("No such status"))
}

/// An active status of the team by id, name or type (the first of that type), read through `db`
/// so that statuses created in the same transaction are found.
pub async fn find_state(db: &mut PgConnection, team: Uuid, value: &str) -> ApiResult<Uuid> {
    let rows: Vec<(Uuid, String, String)> = sqlx::query_as(&format!(
        "SELECT s.id, s.name, s.type FROM workflow_states s WHERE s.team_id = $1 AND s.archived_at IS NULL ORDER BY {STATE_ORDER}"
    ))
    .bind(team)
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
    let names: Vec<String> = rows.iter().map(|r| format!("{} ({})", r.1, r.2)).collect();
    Err(ApiError::bad_request(format!("The team has no status \"{value}\". Options: {}.", names.join(", "))))
}

/// A status the caller can see, by id, or by name or type with its team. Archived statuses are
/// found by id.
async fn find(ctx: &Ctx, value: &str, team: Option<&str>) -> ApiResult<(Uuid, Uuid)> {
    if let Some(id) = as_uuid(value) {
        let team: Option<Uuid> = sqlx::query_scalar(
            "SELECT s.team_id FROM workflow_states s JOIN teams t ON t.id = s.team_id WHERE s.id = $1 AND t.workspace_id = $2",
        )
        .bind(id)
        .bind(ctx.workspace)
        .fetch_optional(ctx.db())
        .await?;
        return match team {
            Some(team) if ctx.can_see_team(team).await? => Ok((id, team)),
            _ => Err(ApiError::not_found(format!("No status {id}"))),
        };
    }
    let Some(team) = team else {
        return Err(ApiError::bad_request("Name the team too, or pass the status's id"));
    };
    let team = resolve::team(ctx, team).await?;
    let mut db = ctx.db().acquire().await?;
    Ok((find_state(&mut db, team, value).await?, team))
}

fn check_type(kind: &str) -> ApiResult<String> {
    let kind = kind.trim();
    crate::ensure!(
        STATE_TYPES.contains(&kind),
        "\"{kind}\" isn't a status type; use triage, backlog, unstarted, started, completed or canceled"
    );
    Ok(kind.to_string())
}

/// A team keeps one open, one completed and one canceled status, and its default and auto-close
/// statuses fit their purpose.
async fn check_workflow(tx: &mut PgConnection, team: Uuid) -> ApiResult<()> {
    let (open, completed, canceled, default_type, auto_close_type): (i64, i64, i64, Option<String>, Option<String>) =
        sqlx::query_as(
            "SELECT count(*) FILTER (WHERE s.type NOT IN ('completed', 'canceled')),
                count(*) FILTER (WHERE s.type = 'completed'), count(*) FILTER (WHERE s.type = 'canceled'),
                (SELECT d.type FROM workflow_states d JOIN teams t ON t.default_issue_state_id = d.id WHERE t.id = $1),
                (SELECT c.type FROM workflow_states c JOIN teams t ON t.auto_close_state_id = c.id WHERE t.id = $1)
             FROM workflow_states s WHERE s.team_id = $1 AND s.archived_at IS NULL",
        )
        .bind(team)
        .fetch_one(&mut *tx)
        .await?;
    crate::ensure!(open > 0, "A team needs at least one status that isn't completed or canceled");
    crate::ensure!(completed > 0, "A team needs at least one completed status");
    crate::ensure!(canceled > 0, "A team needs at least one canceled status");
    if let Some(kind) = default_type {
        crate::ensure!(
            matches!(kind.as_str(), "backlog" | "unstarted"),
            "The team's default status must stay a backlog or unstarted status"
        );
    }
    if let Some(kind) = auto_close_type {
        crate::ensure!(kind == "canceled", "The team's auto-close status must stay a canceled status");
    }
    Ok(())
}

/* ------------------------------------------------------------------ */
/*                                Read                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ListArgs {
    /// Team key, name or id.
    team: String,
    /// Also list archived statuses.
    #[serde(default)]
    include_archived: bool,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list(ctx: Ctx, args: ListArgs) -> ApiResult<Value> {
    let page = Page { limit: args.limit, cursor: args.cursor };
    let team = resolve::team(&ctx, &args.team).await?;
    let sql = format!(
        "SELECT {STATE_JSON} {STATE_FROM} WHERE s.team_id = $1 AND ($2 OR s.archived_at IS NULL)
         ORDER BY s.archived_at IS NOT NULL, {STATE_ORDER} LIMIT $3 OFFSET $4"
    );
    let nodes: Vec<Value> = sqlx::query_scalar(&sql)
        .bind(team)
        .bind(args.include_archived)
        .bind(page.limit() + 1)
        .bind(page.offset()?)
        .fetch_all(ctx.db())
        .await?;
    page.wrap(nodes)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct GetArgs {
    /// The status's id, or its name or type together with team.
    id: String,
    /// Team key, name or id, when id is a name or type.
    team: Option<String>,
}

async fn get(ctx: Ctx, args: GetArgs) -> ApiResult<Value> {
    let (state, _) = find(&ctx, &args.id, args.team.as_deref()).await?;
    let mut db = ctx.db().acquire().await?;
    state_value(&mut db, state).await
}

/* ------------------------------------------------------------------ */
/*                                Save                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SaveArgs {
    /// Only to change a status: its id, or its name with team. Omit to create one.
    id: Option<String>,
    /// Team key, name or id. Required to create.
    team: Option<String>,
    /// Required to create; unique within the team.
    name: Option<String>,
    /// triage, backlog, unstarted, started, completed or canceled. Required to create; changes only while no issues
    /// are in the status.
    #[serde(rename = "type")]
    kind: Option<String>,
    /// Hex color like #f2c94c; defaults to the type's color.
    color: Option<String>,
    /// Null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    description: Option<Option<String>>,
    /// Order within its type on the board; lower comes first. New statuses go last.
    position: Option<f64>,
}

async fn save(ctx: Ctx, args: SaveArgs) -> ApiResult<Value> {
    let mut tx = ctx.db().begin().await?;
    let (state, team, before) = match &args.id {
        Some(reference) => {
            let (state, team) = find(&ctx, reference, args.team.as_deref()).await?;
            require_can_edit(&ctx, &mut tx, team).await?;
            (state, team, Some(state_value(&mut tx, state).await?))
        }
        None => {
            let team = resolve::team(&ctx, args.team.as_deref().unwrap_or_default())
                .await
                .map_err(|e| ApiError::bad_request(format!("team is required to create a status. {}", e.message)))?;
            require_can_edit(&ctx, &mut tx, team).await?;
            (create(&mut tx, team, &args).await?, team, None)
        }
    };
    if before.is_some() {
        update(&mut tx, state, team, args).await?;
    }
    check_workflow(&mut tx, team).await?;
    let value = state_value(&mut tx, state).await?;
    match before {
        Some(before) => {
            let updated_from = changed_fields(&before, &value);
            if updated_from.as_object().is_some_and(|o| !o.is_empty()) {
                events::emit(&mut tx, &ctx, "WorkflowState", "update", &value, Some(updated_from), Some(team)).await?;
            }
        }
        None => events::emit(&mut tx, &ctx, "WorkflowState", "create", &value, None, Some(team)).await?,
    }
    tx.commit().await?;
    Ok(value)
}

async fn check_name_free(tx: &mut PgConnection, team: Uuid, state: Option<Uuid>, name: &str) -> ApiResult<()> {
    let taken: bool = sqlx::query_scalar(
        "SELECT EXISTS (SELECT 1 FROM workflow_states WHERE team_id = $1 AND lower(name) = lower($2)
         AND archived_at IS NULL AND id IS DISTINCT FROM $3)",
    )
    .bind(team)
    .bind(name)
    .bind(state)
    .fetch_one(&mut *tx)
    .await?;
    crate::ensure!(!taken, "The team already has a status named \"{name}\"");
    Ok(())
}

async fn check_single_triage(tx: &mut PgConnection, team: Uuid, state: Option<Uuid>) -> ApiResult<()> {
    let exists: bool = sqlx::query_scalar(
        "SELECT EXISTS (SELECT 1 FROM workflow_states WHERE team_id = $1 AND type = 'triage' AND archived_at IS NULL
         AND id IS DISTINCT FROM $2)",
    )
    .bind(team)
    .bind(state)
    .fetch_one(&mut *tx)
    .await?;
    crate::ensure!(!exists, "The team already has a triage status; a team has one");
    Ok(())
}

async fn create(tx: &mut PgConnection, team: Uuid, args: &SaveArgs) -> ApiResult<Uuid> {
    let name = non_empty(args.name.as_deref().unwrap_or_default(), "name")?;
    let Some(kind) = args.kind.as_deref() else {
        return Err(ApiError::bad_request(
            "type is required: triage, backlog, unstarted, started, completed or canceled",
        ));
    };
    let kind = check_type(kind)?;
    check_name_free(tx, team, None, &name).await?;
    if kind == "triage" {
        check_single_triage(tx, team, None).await?;
    }
    let color = match &args.color {
        Some(color) => check_color(color)?,
        None => default_color(&kind).to_string(),
    };
    let position: f64 = match args.position {
        Some(position) => position,
        None => {
            sqlx::query_scalar(
                "SELECT coalesce(max(position) FILTER (WHERE type = $2), max(position), 0) + 1
                 FROM workflow_states WHERE team_id = $1 AND archived_at IS NULL",
            )
            .bind(team)
            .bind(&kind)
            .fetch_one(&mut *tx)
            .await?
        }
    };
    let description = args.description.clone().flatten().map(|d| d.trim().to_string()).filter(|d| !d.is_empty());
    Ok(sqlx::query_scalar(
        "INSERT INTO workflow_states (team_id, name, type, color, description, position)
         VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
    )
    .bind(team)
    .bind(&name)
    .bind(&kind)
    .bind(&color)
    .bind(description)
    .bind(position)
    .fetch_one(&mut *tx)
    .await?)
}

async fn update(tx: &mut PgConnection, state: Uuid, team: Uuid, args: SaveArgs) -> ApiResult<()> {
    if let Some(name) = &args.name {
        let name = non_empty(name, "name")?;
        check_name_free(tx, team, Some(state), &name).await?;
        sqlx::query("UPDATE workflow_states SET name = $2 WHERE id = $1")
            .bind(state)
            .bind(name)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(color) = &args.color {
        sqlx::query("UPDATE workflow_states SET color = $2 WHERE id = $1")
            .bind(state)
            .bind(check_color(color)?)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(description) = args.description {
        let description = description.map(|d| d.trim().to_string()).filter(|d| !d.is_empty());
        sqlx::query("UPDATE workflow_states SET description = $2 WHERE id = $1")
            .bind(state)
            .bind(description)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(position) = args.position {
        sqlx::query("UPDATE workflow_states SET position = $2 WHERE id = $1")
            .bind(state)
            .bind(position)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(kind) = &args.kind {
        let kind = check_type(kind)?;
        let (current, issues): (String, i64) = sqlx::query_as(
            "SELECT s.type, (SELECT count(*) FROM issues i WHERE i.state_id = s.id) FROM workflow_states s WHERE s.id = $1",
        )
        .bind(state)
        .fetch_one(&mut *tx)
        .await?;
        if kind != current {
            crate::ensure!(
                issues == 0,
                "{issues} issues are in this status, so its type can't change; move them or create a new status"
            );
            if kind == "triage" {
                check_single_triage(tx, team, Some(state)).await?;
            }
            sqlx::query("UPDATE workflow_states SET type = $2 WHERE id = $1")
                .bind(state)
                .bind(&kind)
                .execute(&mut *tx)
                .await?;
        }
    }
    sqlx::query("UPDATE workflow_states SET updated_at = now() WHERE id = $1").bind(state).execute(&mut *tx).await?;
    Ok(())
}

/* ------------------------------------------------------------------ */
/*                               Delete                               */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DeleteArgs {
    /// The status's id, or its name with team.
    id: String,
    /// Team key, name or id, when id is a name.
    team: Option<String>,
    /// Status (name, type or id, same team) to move its issues to. Required when issues are in it or when it is the
    /// team's default or auto-close status.
    move_issues_to: Option<String>,
}

async fn delete(ctx: Ctx, args: DeleteArgs) -> ApiResult<Value> {
    let (state, team) = find(&ctx, &args.id, args.team.as_deref()).await?;
    let mut tx = ctx.db().begin().await?;
    require_can_edit(&ctx, &mut tx, team).await?;
    let (name, kind, archived, triage_on, is_default, is_auto_close): (String, String, bool, bool, bool, bool) =
        sqlx::query_as(
            "SELECT s.name, s.type, s.archived_at IS NOT NULL, t.triage_enabled,
                s.id IS NOT DISTINCT FROM t.default_issue_state_id, s.id IS NOT DISTINCT FROM t.auto_close_state_id
             FROM workflow_states s JOIN teams t ON t.id = s.team_id WHERE s.id = $1 FOR UPDATE",
        )
        .bind(state)
        .fetch_one(&mut *tx)
        .await?;
    crate::ensure!(!archived, "The status {name} is already archived");
    crate::ensure!(
        !(kind == "triage" && triage_on),
        "Triage is on for this team; turn it off with save_team (triageEnabled: false) instead"
    );
    let replacement = match &args.move_issues_to {
        Some(target) => {
            let target = find_state(&mut tx, team, target).await?;
            crate::ensure!(target != state, "moveIssuesTo must be another status");
            Some(target)
        }
        None => None,
    };
    let waiting: Vec<Uuid> = sqlx::query_scalar("SELECT id FROM issues WHERE state_id = $1 ORDER BY number")
        .bind(state)
        .fetch_all(&mut *tx)
        .await?;
    let need = |why: String| ApiError::bad_request(format!("{why}; pass moveIssuesTo (a status of the same team)"));
    if !waiting.is_empty() && replacement.is_none() {
        return Err(need(format!("{} issues are in {name}", waiting.len())));
    }
    if is_default {
        let Some(target) = replacement else { return Err(need(format!("{name} is the team's default status"))) };
        sqlx::query("UPDATE teams SET default_issue_state_id = $2 WHERE id = $1")
            .bind(team)
            .bind(target)
            .execute(&mut *tx)
            .await?;
    }
    if is_auto_close {
        let Some(target) = replacement else { return Err(need(format!("{name} is the team's auto-close status"))) };
        sqlx::query("UPDATE teams SET auto_close_state_id = $2 WHERE id = $1")
            .bind(team)
            .bind(target)
            .execute(&mut *tx)
            .await?;
    }
    sqlx::query("UPDATE workflow_states SET archived_at = now(), updated_at = now() WHERE id = $1")
        .bind(state)
        .execute(&mut *tx)
        .await?;
    check_workflow(&mut tx, team).await?;
    if let Some(target) = replacement {
        for issue in &waiting {
            issues::set_state(&ctx, &mut tx, *issue, target, "updated").await?;
        }
    }
    let mut value = state_value(&mut tx, state).await?;
    events::emit(&mut tx, &ctx, "WorkflowState", "remove", &value, None, Some(team)).await?;
    value["movedIssues"] = json!(waiting.len());
    value["movedTo"] = match replacement {
        Some(target) => sqlx::query_scalar("SELECT state_ref($1)").bind(target).fetch_one(&mut *tx).await?,
        None => Value::Null,
    };
    tx.commit().await?;
    Ok(value)
}
