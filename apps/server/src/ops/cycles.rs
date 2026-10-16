//! Cycles: a team's time-boxed iterations. They are created ahead by the automations, which also
//! complete ended cycles (rolling unfinished issues over) and record daily progress.

use chrono::{DateTime, Datelike, Duration, NaiveDate, Utc};
use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::Value;
use sqlx::{PgConnection, Postgres, QueryBuilder};
use uuid::Uuid;

use super::events;
use super::resolve;
use super::util::{Page, nullable, parse_datetime};
use super::{Op, op};
use crate::ctx::{Access, Ctx};
use crate::error::{ApiError, ApiResult};
use crate::state::AppState;

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "list_cycles",
            Access::Read,
            &[("GET", "/teams/{team}/cycles")],
            "List a team's cycles with their progress (issue counts by state type, estimate sums, daily history). \
             type narrows to the current, previous or next cycle, the upcoming ones or the completed ones. Newest \
             first, except upcoming (soonest first). Archived cycles are left out unless includeArchived is set.",
            list,
        ),
        op(
            "get_cycle",
            Access::Read,
            &[("GET", "/cycles/{id}")],
            "Read a cycle by id, or by number, name or \"current\" / \"next\" / \"previous\" together with team: its \
             dates, progress, daily history and issues (unless includeIssues is false).",
            get,
        ),
        op(
            "save_cycle",
            Access::Write,
            &[("POST", "/cycles"), ("PATCH", "/cycles/{id}")],
            "Create a cycle (omit id; team, startsAt and endsAt are required and the team must have cycles enabled) \
             or update one's name, description or dates (pass id). Cycles of a team can't overlap, and completed \
             cycles can't be moved. Teams with cycles enabled get their cycles created automatically; only create \
             one by hand when the user asks.",
            save,
        ),
        op(
            "archive_cycle",
            Access::Write,
            &[("POST", "/cycles/{id}/archive")],
            "Archive an upcoming or completed cycle (not the current one). Issues of an archived upcoming cycle leave it.",
            archive,
        ),
    ]
}

/* ------------------------------------------------------------------ */
/*                           Representation                           */
/* ------------------------------------------------------------------ */

const CYCLE_FROM: &str = "FROM cycles c JOIN teams t ON t.id = c.team_id JOIN workspaces w ON w.id = t.workspace_id
    CROSS JOIN LATERAL (SELECT count(*) AS issues,
        count(*) FILTER (WHERE s.type = 'completed') AS completed,
        count(*) FILTER (WHERE s.type = 'started') AS started,
        count(*) FILTER (WHERE s.type IN ('triage', 'backlog', 'unstarted')) AS unstarted,
        count(*) FILTER (WHERE s.type = 'canceled') AS canceled,
        coalesce(sum(i.estimate), 0) AS estimate,
        coalesce(sum(i.estimate) FILTER (WHERE s.type = 'completed'), 0) AS completed_estimate,
        coalesce(sum(i.estimate) FILTER (WHERE s.type = 'started'), 0) AS started_estimate
     FROM issues i JOIN workflow_states s ON s.id = i.state_id WHERE i.cycle_id = c.id AND i.trashed_at IS NULL) n";

/// The JSON of the cycle `c` (with `CYCLE_FROM` joined).
fn cycle_json(ctx: &Ctx) -> String {
    let base = format!("'{}'", ctx.public_url().replace('\'', "''"));
    format!(
        "jsonb_build_object(
            'id', c.id,
            'number', c.number,
            'name', c.name,
            'description', c.description,
            'url', {base} || '/' || w.url_key || '/team/' || t.key || '/cycle/' || c.number,
            'startsAt', c.starts_at,
            'endsAt', c.ends_at,
            'completedAt', c.completed_at,
            'archivedAt', c.archived_at,
            'isActive', c.starts_at <= now() AND c.ends_at > now(),
            'isFuture', c.starts_at > now(),
            'isPast', c.ends_at <= now(),
            'team', jsonb_build_object('id', t.id, 'key', t.key, 'name', t.name),
            'progress', CASE WHEN n.issues = 0 THEN 0 ELSE round(n.completed::numeric / n.issues, 4) END,
            'scope', jsonb_build_object('issues', n.issues, 'completed', n.completed, 'started', n.started,
                'unstarted', n.unstarted, 'canceled', n.canceled, 'estimate', n.estimate,
                'completedEstimate', n.completed_estimate, 'startedEstimate', n.started_estimate),
            'history', c.history,
            'createdAt', c.created_at,
            'updatedAt', c.updated_at
        )"
    )
}

async fn cycle_value(ctx: &Ctx, db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    let sql = format!("SELECT {} {CYCLE_FROM} WHERE c.id = $1", cycle_json(ctx));
    let value: Option<Value> = sqlx::query_scalar(&sql).bind(id).fetch_optional(&mut *db).await?;
    value.ok_or_else(|| ApiError::not_found("No such cycle"))
}

/* ------------------------------------------------------------------ */
/*                                Read                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ListCyclesArgs {
    /// Team key, name or id.
    team: String,
    /// current, previous, next, upcoming, completed or all (default).
    #[serde(rename = "type")]
    kind: Option<String>,
    #[serde(default)]
    include_archived: bool,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list(ctx: Ctx, args: ListCyclesArgs) -> ApiResult<Value> {
    let page = Page { limit: args.limit, cursor: args.cursor };
    let team = resolve::team(&ctx, &args.team).await?;
    let (condition, order, single) = match args.kind.as_deref().unwrap_or("all") {
        "all" => ("TRUE", "c.starts_at DESC", false),
        "current" => ("c.starts_at <= now() AND c.ends_at > now()", "c.starts_at DESC", true),
        "previous" => ("c.ends_at <= now()", "c.ends_at DESC", true),
        "next" => ("c.starts_at > now()", "c.starts_at", true),
        "upcoming" => ("c.starts_at > now()", "c.starts_at", false),
        "completed" => ("(c.completed_at IS NOT NULL OR c.ends_at <= now())", "c.starts_at DESC", false),
        other => {
            return Err(ApiError::bad_request(format!(
                "type \"{other}\" isn't one of current, previous, next, upcoming, completed, all"
            )));
        }
    };
    let mut qb: QueryBuilder<Postgres> =
        QueryBuilder::new(format!("SELECT {} {CYCLE_FROM} WHERE c.team_id = ", cycle_json(&ctx)));
    qb.push_bind(team);
    qb.push(format!(" AND {condition}"));
    if !args.include_archived {
        qb.push(" AND c.archived_at IS NULL");
    }
    qb.push(format!(" ORDER BY {order} LIMIT "));
    qb.push_bind(if single { 1 } else { page.limit() + 1 });
    qb.push(" OFFSET ");
    qb.push_bind(page.offset()?);
    let nodes: Vec<Value> = qb.build_query_scalar().fetch_all(ctx.db()).await?;
    page.wrap(nodes)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct GetCycleArgs {
    /// The cycle's id, or its number, name or "current" / "next" / "previous" (with team).
    id: String,
    /// Team key, name or id, when id isn't the cycle's id.
    team: Option<String>,
    /// List the cycle's issues (default true).
    include_issues: Option<bool>,
}

async fn find(ctx: &Ctx, team: Option<&str>, value: &str) -> ApiResult<Uuid> {
    let team = match team {
        Some(team) => Some(resolve::team(ctx, team).await?),
        None => None,
    };
    resolve::cycle(ctx, team, value).await
}

async fn get(ctx: Ctx, args: GetCycleArgs) -> ApiResult<Value> {
    let id = find(&ctx, args.team.as_deref(), &args.id).await?;
    let mut db = ctx.db().acquire().await?;
    let mut value = cycle_value(&ctx, &mut db, id).await?;
    if args.include_issues.unwrap_or(true) {
        value["issues"] = sqlx::query_scalar(
            "SELECT coalesce(jsonb_agg(jsonb_build_object('id', i.id, 'identifier', t.key || '-' || i.number,
                'title', i.title, 'state', state_ref(i.state_id), 'assignee', user_ref(i.assignee_id),
                'estimate', i.estimate, 'priority', i.priority)
                ORDER BY array_position(ARRAY['started','unstarted','backlog','triage','completed','canceled'], s.type),
                    CASE WHEN i.priority = 0 THEN 5 ELSE i.priority END, i.sort_order), '[]')
             FROM issues i JOIN teams t ON t.id = i.team_id JOIN workflow_states s ON s.id = i.state_id
             WHERE i.cycle_id = $1 AND i.trashed_at IS NULL",
        )
        .bind(id)
        .fetch_one(&mut *db)
        .await?;
    }
    Ok(value)
}

/* ------------------------------------------------------------------ */
/*                                Save                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SaveCycleArgs {
    /// Only to update a cycle: its id (or number / name with team). Omit to create one.
    id: Option<String>,
    /// Team key, name or id. Required to create.
    team: Option<String>,
    /// Null clears it (the cycle is then shown as "Cycle <number>").
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    name: Option<Option<String>>,
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    description: Option<Option<String>>,
    /// ISO 8601 date or time (a date starts at 00:00 UTC). Required to create.
    starts_at: Option<String>,
    /// ISO 8601 date or time, after startsAt. Required to create.
    ends_at: Option<String>,
}

fn clean(value: Option<String>) -> Option<String> {
    value.map(|v| v.trim().to_string()).filter(|v| !v.is_empty())
}

async fn check_no_overlap(
    db: &mut PgConnection,
    team: Uuid,
    except: Option<Uuid>,
    starts: DateTime<Utc>,
    ends: DateTime<Utc>,
) -> ApiResult<()> {
    if ends <= starts {
        return Err(ApiError::bad_request("endsAt must be after startsAt"));
    }
    let clash: Option<(i32, DateTime<Utc>, DateTime<Utc>)> = sqlx::query_as(
        "SELECT number, starts_at, ends_at FROM cycles WHERE team_id = $1 AND archived_at IS NULL
         AND id IS DISTINCT FROM $2 AND starts_at < $4 AND ends_at > $3 ORDER BY starts_at LIMIT 1",
    )
    .bind(team)
    .bind(except)
    .bind(starts)
    .bind(ends)
    .fetch_optional(&mut *db)
    .await?;
    if let Some((number, s, e)) = clash {
        return Err(ApiError::bad_request(format!(
            "Those dates overlap cycle {number} ({} to {}); a team's cycles can't overlap",
            s.to_rfc3339(),
            e.to_rfc3339()
        )));
    }
    Ok(())
}

async fn save(ctx: Ctx, args: SaveCycleArgs) -> ApiResult<Value> {
    let mut tx = ctx.db().begin().await?;
    let (id, action) = match &args.id {
        Some(reference) => {
            let id = find(&ctx, args.team.as_deref(), reference).await?;
            update(&mut tx, id, args).await?;
            (id, "update")
        }
        None => (create(&ctx, &mut tx, args).await?, "create"),
    };
    let value = cycle_value(&ctx, &mut tx, id).await?;
    let team = value["team"]["id"].as_str().and_then(resolve::as_uuid);
    events::emit(&mut tx, &ctx, "Cycle", action, &value, None, team).await?;
    tx.commit().await?;
    Ok(value)
}

async fn create(ctx: &Ctx, tx: &mut PgConnection, args: SaveCycleArgs) -> ApiResult<Uuid> {
    let Some(team) = &args.team else { return Err(ApiError::bad_request("Name the team of the new cycle")) };
    let team = resolve::team(ctx, team).await?;
    let (Some(starts), Some(ends)) = (&args.starts_at, &args.ends_at) else {
        return Err(ApiError::bad_request("startsAt and endsAt are required to create a cycle"));
    };
    let (starts, ends) = (parse_datetime(starts)?, parse_datetime(ends)?);
    let enabled: bool = sqlx::query_scalar("SELECT cycles_enabled FROM teams WHERE id = $1 FOR UPDATE")
        .bind(team)
        .fetch_one(&mut *tx)
        .await?;
    if !enabled {
        return Err(ApiError::bad_request(
            "This team doesn't use cycles; turn them on with save_team (cyclesEnabled) first",
        ));
    }
    check_no_overlap(tx, team, None, starts, ends).await?;
    let id = sqlx::query_scalar(
        "INSERT INTO cycles (team_id, number, name, description, starts_at, ends_at)
         SELECT $1, coalesce(max(number), 0) + 1, $2, $3, $4, $5 FROM cycles WHERE team_id = $1 RETURNING id",
    )
    .bind(team)
    .bind(clean(args.name.flatten()))
    .bind(clean(args.description.flatten()))
    .bind(starts)
    .bind(ends)
    .fetch_one(&mut *tx)
    .await?;
    Ok(id)
}

async fn update(tx: &mut PgConnection, id: Uuid, args: SaveCycleArgs) -> ApiResult<()> {
    let (team, starts, ends, completed): (Uuid, DateTime<Utc>, DateTime<Utc>, Option<DateTime<Utc>>) =
        sqlx::query_as("SELECT team_id, starts_at, ends_at, completed_at FROM cycles WHERE id = $1 FOR UPDATE")
            .bind(id)
            .fetch_one(&mut *tx)
            .await?;
    if args.starts_at.is_some() || args.ends_at.is_some() {
        if completed.is_some() {
            return Err(ApiError::bad_request("A completed cycle's dates can't change"));
        }
        let new_starts = args.starts_at.as_deref().map(parse_datetime).transpose()?.unwrap_or(starts);
        let new_ends = args.ends_at.as_deref().map(parse_datetime).transpose()?.unwrap_or(ends);
        check_no_overlap(tx, team, Some(id), new_starts, new_ends).await?;
        sqlx::query("UPDATE cycles SET starts_at = $2, ends_at = $3 WHERE id = $1")
            .bind(id)
            .bind(new_starts)
            .bind(new_ends)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(name) = args.name {
        sqlx::query("UPDATE cycles SET name = $2 WHERE id = $1").bind(id).bind(clean(name)).execute(&mut *tx).await?;
    }
    if let Some(description) = args.description {
        sqlx::query("UPDATE cycles SET description = $2 WHERE id = $1")
            .bind(id)
            .bind(clean(description))
            .execute(&mut *tx)
            .await?;
    }
    sqlx::query("UPDATE cycles SET updated_at = now() WHERE id = $1").bind(id).execute(&mut *tx).await?;
    Ok(())
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ArchiveCycleArgs {
    /// The cycle's id, or its number or name (with team).
    id: String,
    /// Team key, name or id, when id isn't the cycle's id.
    team: Option<String>,
}

async fn archive(ctx: Ctx, args: ArchiveCycleArgs) -> ApiResult<Value> {
    let id = find(&ctx, args.team.as_deref(), &args.id).await?;
    let mut tx = ctx.db().begin().await?;
    let (team, future, done, archived): (Uuid, bool, bool, bool) = sqlx::query_as(
        "SELECT team_id, starts_at > now(), completed_at IS NOT NULL OR ends_at <= now(), archived_at IS NOT NULL
         FROM cycles WHERE id = $1 FOR UPDATE",
    )
    .bind(id)
    .fetch_one(&mut *tx)
    .await?;
    if !archived {
        if !future && !done {
            return Err(ApiError::bad_request("The current cycle can't be archived; wait until it is completed"));
        }
        if future {
            sqlx::query(
                "WITH moved AS (UPDATE issues SET cycle_id = NULL, updated_at = now() WHERE cycle_id = $1 RETURNING id)
                 INSERT INTO issue_history (issue_id, actor_id, action, changes)
                 SELECT id, $2, 'updated', jsonb_build_object('cycle', jsonb_build_object('from', cycle_ref($1), 'to', NULL))
                 FROM moved",
            )
            .bind(id)
            .bind(ctx.user)
            .execute(&mut *tx)
            .await?;
        }
        sqlx::query("UPDATE cycles SET archived_at = now(), updated_at = now() WHERE id = $1")
            .bind(id)
            .execute(&mut *tx)
            .await?;
    }
    let value = cycle_value(&ctx, &mut tx, id).await?;
    events::emit(&mut tx, &ctx, "Cycle", "update", &value, None, Some(team)).await?;
    tx.commit().await?;
    Ok(value)
}

/* ------------------------------------------------------------------ */
/*                             Automations                            */
/* ------------------------------------------------------------------ */

/// Key of the advisory lock that keeps replicas from running the automations at the same time.
const AUTOMATION_LOCK: i64 = 0x5269_6372_6163_4379;

#[derive(sqlx::FromRow)]
struct CycleSettings {
    id: Uuid,
    cycle_duration_weeks: i32,
    cycle_cooldown_weeks: i32,
    cycle_start_day: i32,
    upcoming_cycle_count: i32,
    cycle_rollover: bool,
}

/// The issue counts of cycle `c` today, as a history entry; `$2` is the date.
const SNAPSHOT: &str = "(SELECT jsonb_build_object('date', $2::text, 'scope', count(*),
        'completed', count(*) FILTER (WHERE s.type = 'completed'),
        'started', count(*) FILTER (WHERE s.type = 'started'),
        'scopeEstimate', coalesce(sum(i.estimate), 0),
        'completedEstimate', coalesce(sum(i.estimate) FILTER (WHERE s.type = 'completed'), 0))
     FROM issues i JOIN workflow_states s ON s.id = i.state_id WHERE i.cycle_id = c.id AND i.trashed_at IS NULL)";

/// Creates upcoming cycles, completes ended ones (rolling unfinished issues over) and records
/// each running cycle's daily progress. Runs every minute.
pub async fn run_automations(state: &AppState) -> ApiResult<()> {
    let mut tx = state.db.begin().await?;
    let locked: bool =
        sqlx::query_scalar("SELECT pg_try_advisory_xact_lock($1)").bind(AUTOMATION_LOCK).fetch_one(&mut *tx).await?;
    if !locked {
        return Ok(());
    }
    let now: DateTime<Utc> = sqlx::query_scalar("SELECT now()").fetch_one(&mut *tx).await?;
    let today = now.date_naive().to_string();
    let teams: Vec<CycleSettings> = sqlx::query_as(
        "SELECT id, cycle_duration_weeks, cycle_cooldown_weeks, cycle_start_day, upcoming_cycle_count, cycle_rollover
         FROM teams WHERE cycles_enabled AND archived_at IS NULL FOR UPDATE",
    )
    .fetch_all(&mut *tx)
    .await?;
    for team in &teams {
        create_cycles(&mut tx, team, now).await?;
        complete_ended(&mut tx, team, &today).await?;
    }
    let ids: Vec<Uuid> = teams.iter().map(|t| t.id).collect();
    sqlx::query(&format!(
        "UPDATE cycles c SET history = c.history || jsonb_build_array({SNAPSHOT})
         WHERE c.team_id = ANY($1) AND c.archived_at IS NULL AND c.completed_at IS NULL
         AND c.starts_at <= now() AND c.ends_at > now()
         AND NOT c.history @> jsonb_build_array(jsonb_build_object('date', $2::text))"
    ))
    .bind(&ids)
    .bind(&today)
    .execute(&mut *tx)
    .await?;
    tx.commit().await?;
    Ok(())
}

/// 00:00 UTC of the most recent `start_day` (0 = Sunday) on or before `today`.
fn first_start(today: NaiveDate, start_day: i32) -> DateTime<Utc> {
    let back = (today.weekday().num_days_from_sunday() as i64 - start_day as i64).rem_euclid(7);
    (today - Duration::days(back)).and_hms_opt(0, 0, 0).expect("midnight exists").and_utc()
}

/// Adds cycles after the team's last one until there is a current one (unless it is cooling
/// down) and as many upcoming ones as the team wants.
async fn create_cycles(tx: &mut PgConnection, team: &CycleSettings, now: DateTime<Utc>) -> ApiResult<()> {
    let duration = Duration::weeks(team.cycle_duration_weeks.max(1) as i64);
    let step = duration + Duration::weeks(team.cycle_cooldown_weeks.max(0) as i64);
    let (last_end, mut number, mut upcoming): (Option<DateTime<Utc>>, i32, i64) = sqlx::query_as(
        "SELECT max(ends_at), coalesce(max(number), 0), count(*) FILTER (WHERE starts_at > $2 AND archived_at IS NULL)
         FROM cycles WHERE team_id = $1",
    )
    .bind(team.id)
    .bind(now)
    .fetch_one(&mut *tx)
    .await?;
    let mut start = match last_end {
        Some(end) => end + (step - duration),
        None => first_start(now.date_naive(), team.cycle_start_day),
    };
    while start + duration <= now {
        start += step;
    }
    loop {
        let future = start > now;
        if future && upcoming >= team.upcoming_cycle_count.max(0) as i64 {
            return Ok(());
        }
        number += 1;
        sqlx::query("INSERT INTO cycles (team_id, number, starts_at, ends_at) VALUES ($1, $2, $3, $4)")
            .bind(team.id)
            .bind(number)
            .bind(start)
            .bind(start + duration)
            .execute(&mut *tx)
            .await?;
        if future {
            upcoming += 1;
        }
        start += step;
    }
}

/// Completes the team's ended cycles: unfinished issues move to the next cycle (or out of any
/// cycle when the team doesn't roll over), with a last snapshot of the counts first.
async fn complete_ended(tx: &mut PgConnection, team: &CycleSettings, today: &str) -> ApiResult<()> {
    let ended: Vec<(Uuid, DateTime<Utc>)> = sqlx::query_as(
        "SELECT id, ends_at FROM cycles WHERE team_id = $1 AND completed_at IS NULL AND archived_at IS NULL
         AND ends_at <= now() ORDER BY ends_at",
    )
    .bind(team.id)
    .fetch_all(&mut *tx)
    .await?;
    for (cycle, ends) in ended {
        let next: Option<Uuid> = match team.cycle_rollover {
            true => {
                sqlx::query_scalar(
                    "SELECT id FROM cycles WHERE team_id = $1 AND archived_at IS NULL AND starts_at >= $2
                     AND ends_at > now() ORDER BY starts_at LIMIT 1",
                )
                .bind(team.id)
                .bind(ends)
                .fetch_optional(&mut *tx)
                .await?
            }
            false => None,
        };
        sqlx::query(&format!(
            "UPDATE cycles c SET completed_at = now(), updated_at = now(), history =
                (SELECT coalesce(jsonb_agg(e ORDER BY n), '[]') FROM jsonb_array_elements(c.history) WITH ORDINALITY h(e, n)
                 WHERE e->>'date' IS DISTINCT FROM $2::text) || jsonb_build_array({SNAPSHOT})
             WHERE c.id = $1"
        ))
        .bind(cycle)
        .bind(today)
        .execute(&mut *tx)
        .await?;
        sqlx::query(
            "WITH moved AS (
                UPDATE issues i SET cycle_id = $2, updated_at = now() FROM workflow_states s
                WHERE s.id = i.state_id AND i.cycle_id = $1 AND i.trashed_at IS NULL
                AND s.type NOT IN ('completed', 'canceled') RETURNING i.id)
             INSERT INTO issue_history (issue_id, action, changes)
             SELECT id, 'cycle_rollover', jsonb_build_object('cycle', jsonb_build_object('from', cycle_ref($1), 'to', cycle_ref($2)))
             FROM moved",
        )
        .bind(cycle)
        .bind(next)
        .execute(&mut *tx)
        .await?;
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn first_cycle_starts_on_the_start_day() {
        let thursday = NaiveDate::from_ymd_opt(2026, 10, 15).unwrap();
        assert_eq!(first_start(thursday, 1).to_rfc3339(), "2026-10-12T00:00:00+00:00");
        assert_eq!(first_start(thursday, 4).to_rfc3339(), "2026-10-15T00:00:00+00:00");
        assert_eq!(first_start(thursday, 5).to_rfc3339(), "2026-10-09T00:00:00+00:00");
    }
}
