//! Teams: their settings (estimates, cycles, triage, automations), members, sub-teams, archiving
//! and the triage rotation.

use chrono::{Duration, NaiveDate, Utc};
use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Value, json};
use sqlx::{PgConnection, Postgres};
use uuid::Uuid;

use super::events;
use super::issues::{self, allowed_estimates, changed_fields};
use super::resolve;
use super::states::find_state;
use super::util::{Page, check_color, non_empty, nullable, parse_date};
use super::workspace::check_timezone;
use super::{IdArgs, Op, op};
use crate::account::check_team_key;
use crate::ctx::{Access, Ctx, Role};
use crate::error::{ApiError, ApiResult};
use crate::seed;

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "list_teams",
            Access::Read,
            &[("GET", "/teams")],
            "List the teams you can see (private teams only if you're in them) with key, name, whether you're a member, \
             member and open issue counts and whether cycles and triage are on. Narrow with query (name or key) or mine.",
            list_teams,
        ),
        op(
            "get_team",
            Access::Read,
            &[("GET", "/teams/{id}")],
            "Read a team by key, name or id: all its settings (estimates, cycles, triage, auto-archive and auto-close), \
             its statuses in order, members (with owners), sub-teams, current cycle, default status and who is on triage.",
            get_team,
        ),
        op(
            "save_team",
            Access::Write,
            &[("POST", "/teams"), ("PATCH", "/teams/{id}")],
            "Create a team (omit id; name is required, key defaults to its initials) or change one (id = key, name or \
             id). Omitted fields stay as they are; null clears a nullable field. People are id, name, email or \"me\"; \
             statuses are name, type or id. Changing the key renames every issue (ENG-1 becomes NEW-1); old \
             identifiers keep working. Workspace admins, team owners and members of the team (or anyone, for a public \
             team) may change it; guests can't.",
            save_team,
        ),
        op(
            "delete_team",
            Access::Write,
            &[("DELETE", "/teams/{id}")],
            "Archive a team (workspace admins and team owners). It disappears from lists and its key becomes free; its \
             issues stay readable.",
            delete_team,
        ),
        op(
            "join_team",
            Access::Write,
            &[("POST", "/teams/{id}/join")],
            "Join a team yourself. Private teams can only be joined by workspace admins; others need a member to add them \
             (save_team addMembers).",
            join_team,
        ),
        op(
            "leave_team",
            Access::Write,
            &[("POST", "/teams/{id}/leave")],
            "Leave a team. The last member of a private team can't leave it.",
            leave_team,
        ),
        op(
            "get_triage_responsibility",
            Access::Read,
            &[("GET", "/teams/{id}/triage")],
            "Who handles the team's triage: the action (none, notify or assign), who is responsible now and, with a \
             rotation, the upcoming shifts (filter them with user).",
            get_triage_responsibility,
        ),
    ]
}

/* ------------------------------------------------------------------ */
/*                           Representation                           */
/* ------------------------------------------------------------------ */

const TEAM_FROM: &str = "FROM teams t JOIN workspaces w ON w.id = t.workspace_id";

pub const STATE_ORDER: &str =
    "array_position(ARRAY['triage','backlog','unstarted','started','completed','canceled'], s.type), s.position";

/// The JSON of team `t` (with its workspace `w`); `me` is the caller's id and `base` the public URL.
fn team_json(me: &str, base: &str) -> String {
    format!(
        "jsonb_build_object(
            'id', t.id,
            'key', t.key,
            'name', t.name,
            'description', t.description,
            'icon', t.icon,
            'color', t.color,
            'private', t.private,
            'parent', team_ref(t.parent_id),
            'url', {base} || '/' || w.url_key || '/team/' || t.key,
            'isMember', EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id = t.id AND tm.user_id = {me}),
            'memberCount', (SELECT count(*) FROM team_members tm WHERE tm.team_id = t.id),
            'issueCount', (SELECT count(*) FROM issues i JOIN workflow_states s ON s.id = i.state_id
                WHERE i.team_id = t.id AND i.trashed_at IS NULL AND i.archived_at IS NULL
                AND s.type NOT IN ('completed', 'canceled')),
            'cyclesEnabled', t.cycles_enabled,
            'triageEnabled', t.triage_enabled,
            'timezone', t.timezone,
            'archivedAt', t.archived_at,
            'createdAt', t.created_at,
            'updatedAt', t.updated_at)"
    )
}

const TEAM_SETTINGS_JSON: &str = "jsonb_build_object(
    'estimateType', t.estimate_type,
    'estimateAllowZero', t.estimate_allow_zero,
    'estimateExtended', t.estimate_extended,
    'defaultEstimate', t.default_estimate,
    'cycleDurationWeeks', t.cycle_duration_weeks,
    'cycleCooldownWeeks', t.cycle_cooldown_weeks,
    'cycleStartDay', t.cycle_start_day,
    'upcomingCycleCount', t.upcoming_cycle_count,
    'cycleAutoAddStarted', t.cycle_auto_add_started,
    'cycleAutoAddCompleted', t.cycle_auto_add_completed,
    'cycleRollover', t.cycle_rollover,
    'triageAction', t.triage_action,
    'triageResponsible', (SELECT coalesce(jsonb_agg(user_ref(r.id) ORDER BY r.n), '[]')
        FROM unnest(t.triage_responsible_ids) WITH ORDINALITY r(id, n)),
    'triageRotationWeeks', t.triage_rotation_weeks,
    'triageRotationStart', t.triage_rotation_start,
    'requirePriorityToLeaveTriage', t.require_priority_to_leave_triage,
    'autoArchiveMonths', t.auto_archive_months,
    'autoCloseMonths', t.auto_close_months,
    'autoCloseState', state_ref(t.auto_close_state_id),
    'autoCloseParentIssues', t.auto_close_parent_issues,
    'autoCloseChildIssues', t.auto_close_child_issues,
    'defaultIssueState', state_ref(t.default_issue_state_id))";

/// Everything about one team, as get_team answers.
pub async fn team_value(ctx: &Ctx, db: &mut PgConnection, team: Uuid) -> ApiResult<Value> {
    let visible = ctx.team_ids().await?.to_vec();
    let sql = format!(
        "SELECT {} || {TEAM_SETTINGS_JSON} || jsonb_build_object(
            'states', (SELECT coalesce(jsonb_agg(jsonb_build_object('id', s.id, 'name', s.name, 'type', s.type,
                'color', s.color, 'description', s.description, 'position', s.position) ORDER BY {STATE_ORDER}), '[]')
                FROM workflow_states s WHERE s.team_id = t.id AND s.archived_at IS NULL),
            'members', (SELECT coalesce(jsonb_agg(user_ref(tm.user_id) || jsonb_build_object('owner', tm.owner)
                ORDER BY lower(u.name)), '[]')
                FROM team_members tm JOIN users u ON u.id = tm.user_id WHERE tm.team_id = t.id),
            'labelCount', (SELECT count(*) FROM issue_labels l WHERE l.team_id = t.id AND l.retired_at IS NULL
                AND NOT l.is_group),
            'subTeams', (SELECT coalesce(jsonb_agg(team_ref(c.id) ORDER BY c.name), '[]') FROM teams c
                WHERE c.parent_id = t.id AND c.archived_at IS NULL AND (c.id = ANY($4) OR NOT c.private)),
            'currentCycle', (SELECT cycle_ref(c.id) FROM cycles c WHERE c.team_id = t.id AND c.archived_at IS NULL
                AND c.starts_at <= now() AND c.ends_at > now() ORDER BY c.starts_at DESC LIMIT 1))
         {TEAM_FROM} WHERE t.id = $1",
        team_json("$2", "$3")
    );
    let mut value: Value = sqlx::query_scalar(&sql)
        .bind(team)
        .bind(ctx.user)
        .bind(ctx.public_url())
        .bind(&visible)
        .fetch_optional(&mut *db)
        .await?
        .ok_or_else(|| ApiError::not_found("No such team"))?;
    let kind = value["estimateType"].as_str().unwrap_or("notUsed").to_string();
    let allow_zero = value["estimateAllowZero"].as_bool().unwrap_or(false);
    let extended = value["estimateExtended"].as_bool().unwrap_or(false);
    value["allowedEstimates"] = json!(allowed_estimates(&kind, allow_zero, extended));
    let (action, responsible) = issues::triage_responsible(db, team).await?;
    value["triage"] = json!({
        "enabled": value["triageEnabled"],
        "action": action,
        "responsible": user_refs(db, &responsible).await?,
    });
    Ok(value)
}

async fn user_refs(db: &mut PgConnection, ids: &[Uuid]) -> ApiResult<Value> {
    Ok(sqlx::query_scalar(
        "SELECT coalesce(jsonb_agg(user_ref(r.id) ORDER BY r.n), '[]') FROM unnest($1::uuid[]) WITH ORDINALITY r(id, n)",
    )
    .bind(ids)
    .fetch_one(&mut *db)
    .await?)
}

/* ------------------------------------------------------------------ */
/*                                Read                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ListTeamsArgs {
    /// Part of a team's name or key.
    query: Option<String>,
    /// Also list archived teams.
    #[serde(default)]
    include_archived: bool,
    /// Only teams you are a member of.
    #[serde(default)]
    mine: bool,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list_teams(ctx: Ctx, args: ListTeamsArgs) -> ApiResult<Value> {
    let page = Page { limit: args.limit, cursor: args.cursor };
    let visible = ctx.team_ids().await?.to_vec();
    let query = args.query.as_deref().map(str::trim).filter(|q| !q.is_empty()).map(|q| format!("%{q}%"));
    let sql = format!(
        "SELECT {} {TEAM_FROM} WHERE t.id = ANY($1) AND ($4 OR t.archived_at IS NULL)
         AND (NOT $5 OR EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id = t.id AND tm.user_id = $2))
         AND ($6::text IS NULL OR t.name ILIKE $6 OR t.key ILIKE $6)
         ORDER BY lower(t.name) LIMIT $7 OFFSET $8",
        team_json("$2", "$3")
    );
    let nodes: Vec<Value> = sqlx::query_scalar(&sql)
        .bind(&visible)
        .bind(ctx.user)
        .bind(ctx.public_url())
        .bind(args.include_archived)
        .bind(args.mine)
        .bind(query)
        .bind(page.limit() + 1)
        .bind(page.offset()?)
        .fetch_all(ctx.db())
        .await?;
    page.wrap(nodes)
}

async fn get_team(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let team = resolve::team(&ctx, &args.id).await?;
    let mut db = ctx.db().acquire().await?;
    team_value(&ctx, &mut db, team).await
}

/* ------------------------------------------------------------------ */
/*                             Permissions                            */
/* ------------------------------------------------------------------ */

struct Membership {
    private: bool,
    member: bool,
    owner: bool,
}

async fn membership(db: &mut PgConnection, team: Uuid, user: Uuid) -> ApiResult<Membership> {
    let (private, member, owner): (bool, bool, bool) = sqlx::query_as(
        "SELECT t.private, tm.user_id IS NOT NULL, coalesce(tm.owner, false)
         FROM teams t LEFT JOIN team_members tm ON tm.team_id = t.id AND tm.user_id = $2 WHERE t.id = $1",
    )
    .bind(team)
    .bind(user)
    .fetch_one(&mut *db)
    .await?;
    Ok(Membership { private, member, owner })
}

/// Workspace admins, the team's owners and members, and for a public team any member of the
/// workspace may change a team, its statuses and its settings. Guests may not.
pub async fn require_can_edit(ctx: &Ctx, db: &mut PgConnection, team: Uuid) -> ApiResult<()> {
    ctx.require_member()?;
    if ctx.role >= Role::Admin {
        return Ok(());
    }
    let m = membership(db, team, ctx.user).await?;
    if m.owner || m.member || !m.private {
        return Ok(());
    }
    Err(ApiError::forbidden("Only members of this private team can change it"))
}

/// A team by id, key or name; admins also find private teams they aren't in.
async fn find_team(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    let visible = resolve::team(ctx, value).await;
    if visible.is_ok() || ctx.role < Role::Admin {
        return visible;
    }
    let value = value.trim();
    let found: Option<Uuid> = sqlx::query_scalar(
        "SELECT id FROM teams WHERE workspace_id = $1 AND archived_at IS NULL
         AND (id::text = $2 OR upper(key) = upper($2) OR lower(name) = lower($2))
         ORDER BY id::text = $2 DESC, upper(key) = upper($2) DESC LIMIT 1",
    )
    .bind(ctx.workspace)
    .bind(value)
    .fetch_optional(ctx.db())
    .await?;
    found.map_or(visible, Ok)
}

/* ------------------------------------------------------------------ */
/*                                Save                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema, Default)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SaveTeamArgs {
    /// Only to change a team: its key, name or id. Omit to create one.
    id: Option<String>,
    /// Required to create.
    name: Option<String>,
    /// 1 to 7 letters and digits starting with a letter, like ENG. Defaults to the name's initials.
    key: Option<String>,
    /// Null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    description: Option<Option<String>>,
    /// An emoji or icon name; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    icon: Option<Option<String>>,
    /// Hex color like #5e6ad2; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    color: Option<Option<String>>,
    /// Private teams and their issues are only visible to their members.
    private: Option<bool>,
    /// Parent team (key, name or id), making this a sub-team; null makes it a top-level team.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    parent: Option<Option<String>>,
    /// IANA timezone like Europe/Berlin, used for cycles.
    timezone: Option<String>,
    /// notUsed, exponential, fibonacci, linear or tShirt.
    estimate_type: Option<String>,
    /// Allow an estimate of 0.
    estimate_allow_zero: Option<bool>,
    /// Allow the larger estimates of the scale.
    estimate_extended: Option<bool>,
    /// The estimate new issues get; null for none.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<f64>")]
    default_estimate: Option<Option<f64>>,
    cycles_enabled: Option<bool>,
    /// 1 to 8.
    cycle_duration_weeks: Option<i32>,
    /// Weeks off between cycles, 0 to 4.
    cycle_cooldown_weeks: Option<i32>,
    /// Day cycles start: 0 Sunday, 1 Monday ... 6 Saturday.
    cycle_start_day: Option<i32>,
    /// How many upcoming cycles to create ahead, 1 to 8.
    upcoming_cycle_count: Option<i32>,
    /// Put issues into the current cycle when they're started.
    cycle_auto_add_started: Option<bool>,
    /// Put issues into the current cycle when they're completed.
    cycle_auto_add_completed: Option<bool>,
    /// Move unfinished issues to the next cycle when one ends.
    cycle_rollover: Option<bool>,
    /// Route issues from other teams and integrations to a Triage status first. Turning it off moves issues in
    /// triage to the default status.
    triage_enabled: Option<bool>,
    /// What happens to new triage issues: none, notify (the responsible people) or assign (to the person on duty).
    triage_action: Option<String>,
    /// People responsible for triage (replaces the list): id, name, email or "me".
    triage_responsible: Option<Vec<String>>,
    /// With a number of weeks, the responsible people take turns, in order, for that many weeks each; null for no rotation.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<i32>")]
    triage_rotation_weeks: Option<Option<i32>>,
    /// YYYY-MM-DD the rotation starts (the first person's first shift); defaults to today.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    triage_rotation_start: Option<Option<String>>,
    /// Issues need a priority before they can leave triage.
    require_priority_to_leave_triage: Option<bool>,
    /// Archive closed issues after 1, 3, 6, 9 or 12 months; null turns it off.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<i32>")]
    auto_archive_months: Option<Option<i32>>,
    /// Close issues not updated for 1, 3, 6, 9 or 12 months; null turns it off.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<i32>")]
    auto_close_months: Option<Option<i32>>,
    /// The canceled-type status auto-closed issues go to (name or id); null for the first canceled status.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    auto_close_state: Option<Option<String>>,
    /// Complete a parent issue when all its sub-issues are done.
    auto_close_parent_issues: Option<bool>,
    /// Close the sub-issues when their parent is closed.
    auto_close_child_issues: Option<bool>,
    /// The status new issues start in (a backlog or unstarted status: name or id).
    default_issue_state: Option<String>,
    /// People to add to the team: id, name, email or "me".
    add_members: Option<Vec<String>>,
    /// People to take off the team.
    remove_members: Option<Vec<String>>,
}

async fn save_team(ctx: Ctx, args: SaveTeamArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let mut tx = ctx.db().begin().await?;
    let (team, before) = match &args.id {
        Some(reference) => {
            let team = resolve::team(&ctx, reference).await?;
            require_can_edit(&ctx, &mut tx, team).await?;
            sqlx::query("SELECT id FROM teams WHERE id = $1 FOR UPDATE").bind(team).execute(&mut *tx).await?;
            (team, Some(team_value(&ctx, &mut tx, team).await?))
        }
        None => (create(&ctx, &mut tx, &args).await?, None),
    };
    apply(&ctx, &mut tx, team, args, before.is_none()).await?;
    let value = team_value(&ctx, &mut tx, team).await?;
    match before {
        Some(before) => {
            let updated_from = changed_fields(&before, &value);
            if updated_from.as_object().is_some_and(|o| !o.is_empty()) {
                sqlx::query("UPDATE teams SET updated_at = now() WHERE id = $1").bind(team).execute(&mut *tx).await?;
                events::emit(&mut tx, &ctx, "Team", "update", &value, Some(updated_from), Some(team)).await?;
            }
        }
        None => events::emit(&mut tx, &ctx, "Team", "create", &value, None, Some(team)).await?,
    }
    tx.commit().await?;
    Ok(value)
}

async fn create(ctx: &Ctx, tx: &mut PgConnection, args: &SaveTeamArgs) -> ApiResult<Uuid> {
    let name = non_empty(args.name.as_deref().unwrap_or(""), "name")?;
    let key = check_team_key(&args.key.clone().unwrap_or_else(|| seed::team_key(&name)))?;
    check_name_free(ctx, tx, None, &name).await?;
    check_key_free(ctx, tx, None, &key).await?;
    seed::create_team(tx, ctx.workspace, ctx.user, &name, &key).await
}

async fn check_key_free(ctx: &Ctx, tx: &mut PgConnection, team: Option<Uuid>, key: &str) -> ApiResult<()> {
    let taken: Option<String> = sqlx::query_scalar(
        "SELECT name FROM teams WHERE workspace_id = $1 AND upper(key) = $2 AND archived_at IS NULL
         AND id IS DISTINCT FROM $3",
    )
    .bind(ctx.workspace)
    .bind(key)
    .bind(team)
    .fetch_optional(&mut *tx)
    .await?;
    match taken {
        Some(name) => Err(ApiError::conflict(format!("The key {key} is taken by the team {name}; choose another key"))),
        None => Ok(()),
    }
}

async fn check_name_free(ctx: &Ctx, tx: &mut PgConnection, team: Option<Uuid>, name: &str) -> ApiResult<()> {
    let taken: bool = sqlx::query_scalar(
        "SELECT EXISTS (SELECT 1 FROM teams WHERE workspace_id = $1 AND lower(name) = lower($2)
         AND archived_at IS NULL AND id IS DISTINCT FROM $3)",
    )
    .bind(ctx.workspace)
    .bind(name)
    .bind(team)
    .fetch_one(&mut *tx)
    .await?;
    crate::ensure!(!taken, "There already is a team named \"{name}\"");
    Ok(())
}

/// Sets one column of the team.
async fn set<T>(tx: &mut PgConnection, team: Uuid, column: &str, value: T) -> ApiResult<()>
where
    T: for<'q> sqlx::Encode<'q, Postgres> + sqlx::Type<Postgres> + Send + 'static,
{
    sqlx::query(&format!("UPDATE teams SET {column} = $2 WHERE id = $1"))
        .bind(team)
        .bind(value)
        .execute(&mut *tx)
        .await?;
    Ok(())
}

fn in_range(value: i32, range: std::ops::RangeInclusive<i32>, what: &str) -> ApiResult<i32> {
    crate::ensure!(range.contains(&value), "{what} is {} to {}", range.start(), range.end());
    Ok(value)
}

fn check_months(value: Option<i32>, what: &str) -> ApiResult<Option<i32>> {
    crate::ensure!(value.is_none_or(|m| [1, 3, 6, 9, 12].contains(&m)), "{what} is 1, 3, 6, 9 or 12 (or null for off)");
    Ok(value)
}

fn trimmed(value: Option<String>) -> Option<String> {
    value.map(|v| v.trim().to_string()).filter(|v| !v.is_empty())
}

async fn apply(ctx: &Ctx, tx: &mut PgConnection, team: Uuid, args: SaveTeamArgs, creating: bool) -> ApiResult<()> {
    if !creating {
        if let Some(name) = &args.name {
            let name = non_empty(name, "name")?;
            check_name_free(ctx, tx, Some(team), &name).await?;
            set(tx, team, "name", name).await?;
        }
        if let Some(key) = &args.key {
            change_key(ctx, tx, team, &check_team_key(key)?).await?;
        }
    }
    if let Some(description) = args.description.clone() {
        set(tx, team, "description", trimmed(description)).await?;
    }
    if let Some(icon) = args.icon.clone() {
        set(tx, team, "icon", trimmed(icon)).await?;
    }
    if let Some(color) = args.color.clone() {
        set(tx, team, "color", trimmed(color).map(|c| check_color(&c)).transpose()?).await?;
    }
    if let Some(private) = args.private {
        set(tx, team, "private", private).await?;
    }
    if let Some(parent) = &args.parent {
        let parent = match parent {
            Some(p) => Some(resolve::team(ctx, p).await?),
            None => None,
        };
        if let Some(parent) = parent {
            let loops: bool = sqlx::query_scalar(
                "WITH RECURSIVE up AS (SELECT id, parent_id FROM teams WHERE id = $1
                 UNION SELECT t.id, t.parent_id FROM teams t JOIN up ON t.id = up.parent_id)
                 SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)",
            )
            .bind(parent)
            .bind(team)
            .fetch_one(&mut *tx)
            .await?;
            crate::ensure!(!loops, "A team can't be a sub-team of itself or of its own sub-teams");
        }
        set(tx, team, "parent_id", parent).await?;
    }
    if let Some(timezone) = &args.timezone {
        let timezone = timezone.trim();
        check_timezone(tx, timezone).await?;
        set(tx, team, "timezone", timezone.to_string()).await?;
    }
    if let Some(kind) = &args.estimate_type {
        crate::ensure!(
            ["notUsed", "exponential", "fibonacci", "linear", "tShirt"].contains(&kind.as_str()),
            "estimateType is notUsed, exponential, fibonacci, linear or tShirt"
        );
        set(tx, team, "estimate_type", kind.clone()).await?;
    }
    for (column, value) in [
        ("estimate_allow_zero", args.estimate_allow_zero),
        ("estimate_extended", args.estimate_extended),
        ("cycles_enabled", args.cycles_enabled),
        ("cycle_auto_add_started", args.cycle_auto_add_started),
        ("cycle_auto_add_completed", args.cycle_auto_add_completed),
        ("cycle_rollover", args.cycle_rollover),
        ("require_priority_to_leave_triage", args.require_priority_to_leave_triage),
        ("auto_close_parent_issues", args.auto_close_parent_issues),
        ("auto_close_child_issues", args.auto_close_child_issues),
    ] {
        if let Some(value) = value {
            set(tx, team, column, value).await?;
        }
    }
    if let Some(estimate) = args.default_estimate {
        set(tx, team, "default_estimate", estimate).await?;
    }
    for (column, value, range, what) in [
        ("cycle_duration_weeks", args.cycle_duration_weeks, 1..=8, "cycleDurationWeeks"),
        ("cycle_cooldown_weeks", args.cycle_cooldown_weeks, 0..=4, "cycleCooldownWeeks"),
        ("cycle_start_day", args.cycle_start_day, 0..=6, "cycleStartDay"),
        ("upcoming_cycle_count", args.upcoming_cycle_count, 1..=8, "upcomingCycleCount"),
    ] {
        if let Some(value) = value {
            set(tx, team, column, in_range(value, range, what)?).await?;
        }
    }
    if let Some(months) = args.auto_archive_months {
        set(tx, team, "auto_archive_months", check_months(months, "autoArchiveMonths")?).await?;
    }
    if let Some(months) = args.auto_close_months {
        set(tx, team, "auto_close_months", check_months(months, "autoCloseMonths")?).await?;
    }
    if let Some(state) = &args.auto_close_state {
        let state = match state {
            Some(s) => Some(find_state(tx, team, s).await?),
            None => issues::first_state_of_type(tx, team, "canceled").await?,
        };
        if let Some(state) = state {
            crate::ensure!(state_type(tx, state).await? == "canceled", "autoCloseState must be a canceled-type status");
        }
        set(tx, team, "auto_close_state_id", state).await?;
    }
    if let Some(state) = &args.default_issue_state {
        let state = find_state(tx, team, state).await?;
        let kind = state_type(tx, state).await?;
        crate::ensure!(
            matches!(kind.as_str(), "backlog" | "unstarted"),
            "defaultIssueState must be a backlog or unstarted status, not {kind}"
        );
        set(tx, team, "default_issue_state_id", Some(state)).await?;
    }
    members(ctx, tx, team, &args).await?;
    triage(ctx, tx, team, &args).await?;
    check_team(tx, team).await
}

async fn state_type(tx: &mut PgConnection, state: Uuid) -> ApiResult<String> {
    Ok(sqlx::query_scalar("SELECT type FROM workflow_states WHERE id = $1").bind(state).fetch_one(&mut *tx).await?)
}

/// Gives the team a new key. Every issue keeps its old identifier in previous_identifiers.
async fn change_key(ctx: &Ctx, tx: &mut PgConnection, team: Uuid, key: &str) -> ApiResult<()> {
    let old: String = sqlx::query_scalar("SELECT key FROM teams WHERE id = $1").bind(team).fetch_one(&mut *tx).await?;
    if old == key {
        return Ok(());
    }
    check_key_free(ctx, tx, Some(team), key).await?;
    sqlx::query(
        "UPDATE issues SET previous_identifiers = array_append(previous_identifiers, $2 || '-' || number)
         WHERE team_id = $1 AND NOT ($2 || '-' || number) = ANY(previous_identifiers)",
    )
    .bind(team)
    .bind(&old)
    .execute(&mut *tx)
    .await?;
    set(tx, team, "key", key.to_string()).await
}

/// People who can see the team: its members, and for a public team every member who isn't a guest.
async fn check_can_see(tx: &mut PgConnection, team: Uuid, users: &[Uuid]) -> ApiResult<()> {
    let blind: Vec<String> = sqlx::query_scalar(
        "SELECT u.name FROM users u JOIN teams t ON t.id = $1
         JOIN workspace_members m ON m.workspace_id = t.workspace_id AND m.user_id = u.id
         WHERE u.id = ANY($2) AND NOT ((NOT t.private AND m.role <> 'guest')
            OR EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id = t.id AND tm.user_id = u.id))",
    )
    .bind(team)
    .bind(users)
    .fetch_all(&mut *tx)
    .await?;
    crate::ensure!(blind.is_empty(), "{} can't see this team; add them to it first", blind.join(", "));
    Ok(())
}

async fn members(ctx: &Ctx, tx: &mut PgConnection, team: Uuid, args: &SaveTeamArgs) -> ApiResult<()> {
    if let Some(people) = &args.add_members {
        let users = resolve::users(ctx, people).await?;
        sqlx::query("INSERT INTO team_members (team_id, user_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING")
            .bind(team)
            .bind(&users)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(people) = &args.remove_members {
        let users = resolve::users(ctx, people).await?;
        sqlx::query("DELETE FROM team_members WHERE team_id = $1 AND user_id = ANY($2)")
            .bind(team)
            .bind(&users)
            .execute(&mut *tx)
            .await?;
        drop_from_triage(tx, team, &users).await?;
    }
    Ok(())
}

async fn drop_from_triage(tx: &mut PgConnection, team: Uuid, users: &[Uuid]) -> ApiResult<()> {
    sqlx::query(
        "UPDATE teams SET triage_responsible_ids = ARRAY(SELECT r FROM unnest(triage_responsible_ids) WITH ORDINALITY x(r, n)
            WHERE NOT r = ANY($2) ORDER BY n) WHERE id = $1",
    )
    .bind(team)
    .bind(users)
    .execute(&mut *tx)
    .await?;
    Ok(())
}

async fn triage(ctx: &Ctx, tx: &mut PgConnection, team: Uuid, args: &SaveTeamArgs) -> ApiResult<()> {
    if let Some(action) = &args.triage_action {
        crate::ensure!(
            ["none", "notify", "assign"].contains(&action.as_str()),
            "triageAction is none, notify or assign"
        );
        set(tx, team, "triage_action", action.clone()).await?;
    }
    if let Some(people) = &args.triage_responsible {
        let users = resolve::users(ctx, people).await?;
        check_can_see(tx, team, &users).await?;
        set(tx, team, "triage_responsible_ids", users).await?;
    }
    if let Some(weeks) = args.triage_rotation_weeks {
        let weeks = weeks.map(|w| in_range(w, 1..=52, "triageRotationWeeks")).transpose()?;
        set(tx, team, "triage_rotation_weeks", weeks).await?;
    }
    if let Some(start) = &args.triage_rotation_start {
        let start = start.as_deref().map(parse_date).transpose()?;
        set(tx, team, "triage_rotation_start", start).await?;
    }
    sqlx::query(
        "UPDATE teams SET triage_rotation_start = (now() AT TIME ZONE 'UTC')::date
         WHERE id = $1 AND triage_rotation_weeks IS NOT NULL AND triage_rotation_start IS NULL",
    )
    .bind(team)
    .execute(&mut *tx)
    .await?;
    match args.triage_enabled {
        Some(true) => {
            sqlx::query(
                "UPDATE workflow_states SET archived_at = NULL WHERE id = (SELECT id FROM workflow_states
                    WHERE team_id = $1 AND type = 'triage' AND archived_at IS NOT NULL
                    AND NOT EXISTS (SELECT 1 FROM workflow_states WHERE team_id = $1 AND type = 'triage' AND archived_at IS NULL)
                    ORDER BY archived_at DESC LIMIT 1)",
            )
            .bind(team)
            .execute(&mut *tx)
            .await?;
            seed::ensure_triage_state(tx, team).await?;
            set(tx, team, "triage_enabled", true).await?;
        }
        Some(false) => {
            set(tx, team, "triage_enabled", false).await?;
            let triage: Vec<Uuid> = sqlx::query_scalar(
                "UPDATE workflow_states SET archived_at = now() WHERE team_id = $1 AND type = 'triage'
                 AND archived_at IS NULL RETURNING id",
            )
            .bind(team)
            .fetch_all(&mut *tx)
            .await?;
            let waiting: Vec<Uuid> = sqlx::query_scalar("SELECT id FROM issues WHERE state_id = ANY($1)")
                .bind(&triage)
                .fetch_all(&mut *tx)
                .await?;
            if !waiting.is_empty() {
                let target = issues::default_state(tx, team).await?;
                for issue in waiting {
                    issues::set_state(ctx, tx, issue, target, "updated").await?;
                }
            }
        }
        None => {}
    }
    Ok(())
}

/// What must hold after any change: private teams keep a member, the default estimate is on the
/// scale.
async fn check_team(tx: &mut PgConnection, team: Uuid) -> ApiResult<()> {
    let (private, members, kind, zero, extended, estimate): (bool, i64, String, bool, bool, Option<f64>) =
        sqlx::query_as(
            "SELECT private, (SELECT count(*) FROM team_members WHERE team_id = $1), estimate_type,
                estimate_allow_zero, estimate_extended, default_estimate FROM teams WHERE id = $1",
        )
        .bind(team)
        .fetch_one(&mut *tx)
        .await?;
    crate::ensure!(!private || members > 0, "A private team needs at least one member");
    if let Some(estimate) = estimate.filter(|_| kind != "notUsed") {
        let allowed = allowed_estimates(&kind, zero, extended);
        let list: Vec<String> = allowed.iter().map(f64::to_string).collect();
        crate::ensure!(
            allowed.contains(&estimate),
            "defaultEstimate {estimate} isn't on the {kind} scale; use one of {}",
            list.join(", ")
        );
    }
    Ok(())
}

/* ------------------------------------------------------------------ */
/*                       Archive, join and leave                      */
/* ------------------------------------------------------------------ */

async fn delete_team(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let team = resolve::team(&ctx, &args.id).await?;
    let mut tx = ctx.db().begin().await?;
    if ctx.role < Role::Admin && !membership(&mut tx, team, ctx.user).await?.owner {
        return Err(ApiError::forbidden("Only workspace admins and the team's owners can delete a team"));
    }
    let (key, name): (String, String) = sqlx::query_as(
        "UPDATE teams SET archived_at = now(), updated_at = now()
        WHERE id = $1 RETURNING key, name",
    )
    .bind(team)
    .fetch_one(&mut *tx)
    .await?;
    sqlx::query("UPDATE teams SET parent_id = NULL WHERE parent_id = $1").bind(team).execute(&mut *tx).await?;
    events::audit(&mut tx, &ctx, "team.archived", json!({ "team": team, "key": key, "name": name })).await?;
    let value = team_value(&ctx, &mut tx, team).await?;
    events::emit(&mut tx, &ctx, "Team", "remove", &value, None, Some(team)).await?;
    tx.commit().await?;
    Ok(json!({ "deleted": team, "key": key }))
}

async fn join_team(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let team = find_team(&ctx, &args.id).await?;
    let mut tx = ctx.db().begin().await?;
    let m = membership(&mut tx, team, ctx.user).await?;
    if m.private && !m.member && ctx.role < Role::Admin {
        return Err(ApiError::forbidden("Only workspace admins can join a private team; ask a member to add you"));
    }
    sqlx::query("INSERT INTO team_members (team_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING")
        .bind(team)
        .bind(ctx.user)
        .execute(&mut *tx)
        .await?;
    let value = team_value(&ctx, &mut tx, team).await?;
    tx.commit().await?;
    Ok(value)
}

async fn leave_team(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let team = resolve::team(&ctx, &args.id).await?;
    let mut tx = ctx.db().begin().await?;
    sqlx::query("SELECT id FROM teams WHERE id = $1 FOR UPDATE").bind(team).execute(&mut *tx).await?;
    let m = membership(&mut tx, team, ctx.user).await?;
    crate::ensure!(m.member, "You aren't a member of this team");
    sqlx::query("DELETE FROM team_members WHERE team_id = $1 AND user_id = $2")
        .bind(team)
        .bind(ctx.user)
        .execute(&mut *tx)
        .await?;
    drop_from_triage(&mut tx, team, &[ctx.user]).await?;
    check_team(&mut tx, team).await.map_err(|_| {
        ApiError::bad_request("You're the last member of this private team; add someone else or delete the team")
    })?;
    let team_ref: Value = sqlx::query_scalar("SELECT team_ref($1)").bind(team).fetch_one(&mut *tx).await?;
    tx.commit().await?;
    Ok(json!({ "left": team_ref }))
}

/* ------------------------------------------------------------------ */
/*                                Triage                              */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct TriageArgs {
    /// Team key, name or id.
    id: String,
    /// How many upcoming shifts to list, 10 by default, at most 100.
    limit: Option<usize>,
    /// Only this person's shifts: id, name, email or "me".
    user: Option<String>,
}

/// The rotation's shifts from the current one on: (person, start, end).
fn shifts(
    people: &[Uuid],
    weeks: i32,
    start: NaiveDate,
    today: NaiveDate,
    limit: usize,
    only: Option<Uuid>,
) -> Vec<(Uuid, NaiveDate, NaiveDate)> {
    if people.is_empty() || weeks < 1 || only.is_some_and(|u| !people.contains(&u)) {
        return vec![];
    }
    let weeks = weeks as i64;
    let mut turn = (today - start).num_weeks().max(0) / weeks;
    let mut found = Vec::new();
    while found.len() < limit {
        let person = people[turn as usize % people.len()];
        let starts = start + Duration::weeks(turn * weeks);
        if only.is_none_or(|u| u == person) {
            found.push((person, starts, starts + Duration::weeks(weeks)));
        }
        turn += 1;
    }
    found
}

async fn get_triage_responsibility(ctx: Ctx, args: TriageArgs) -> ApiResult<Value> {
    let team = resolve::team(&ctx, &args.id).await?;
    let only = match &args.user {
        Some(user) => Some(resolve::user(&ctx, user).await?),
        None => None,
    };
    let mut db = ctx.db().acquire().await?;
    let (team_ref, enabled, people, weeks, start): (Value, bool, Vec<Uuid>, Option<i32>, Option<NaiveDate>) =
        sqlx::query_as(
            "SELECT team_ref(id), triage_enabled, triage_responsible_ids, triage_rotation_weeks, triage_rotation_start
             FROM teams WHERE id = $1",
        )
        .bind(team)
        .fetch_one(&mut *db)
        .await?;
    let (action, current) = issues::triage_responsible(&mut db, team).await?;
    let mut upcoming = Vec::new();
    if let (Some(weeks), Some(start)) = (weeks, start) {
        let limit = args.limit.unwrap_or(10).clamp(1, 100);
        for (person, starts, ends) in shifts(&people, weeks, start, Utc::now().date_naive(), limit, only) {
            let user: Value = sqlx::query_scalar("SELECT user_ref($1)").bind(person).fetch_one(&mut *db).await?;
            upcoming.push(json!({ "user": user, "startsAt": starts, "endsAt": ends }));
        }
    }
    let rotation = weeks.map(|weeks| json!({ "weeks": weeks, "start": start }));
    Ok(json!({
        "team": team_ref,
        "triageEnabled": enabled,
        "action": action,
        "responsible": user_refs(&mut db, &current).await?,
        "everyone": user_refs(&mut db, &people).await?,
        "rotation": rotation,
        "upcomingShifts": upcoming,
    }))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rotation_shifts() {
        let (a, b) = (Uuid::new_v4(), Uuid::new_v4());
        let start = NaiveDate::from_ymd_opt(2026, 1, 5).unwrap();
        let today = NaiveDate::from_ymd_opt(2026, 1, 20).unwrap();
        let all = shifts(&[a, b], 1, start, today, 3, None);
        assert_eq!(all.iter().map(|s| s.0).collect::<Vec<_>>(), vec![a, b, a]);
        assert_eq!(all[0].1, NaiveDate::from_ymd_opt(2026, 1, 19).unwrap());
        let mine = shifts(&[a, b], 2, start, today, 2, Some(b));
        assert_eq!(mine[0].1, NaiveDate::from_ymd_opt(2026, 1, 19).unwrap());
        assert_eq!(mine[1].1, NaiveDate::from_ymd_opt(2026, 2, 16).unwrap());
    }
}
