//! Projects: listing, reading, creating and changing them (teams, members, labels, dates, links,
//! initiatives, dependencies and templates), trash and archive, and the workspace's project
//! statuses and labels.

use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Map, Value, json};
use sqlx::{PgConnection, Postgres, QueryBuilder};
use uuid::Uuid;

use super::events::{self, Notice};
use super::filter::escape_like;
use super::issues::{LinkArg, changed_fields};
use super::resolve::{self, as_uuid};
use super::util::{
    Page, TextEdit, check_color, check_priority, new_content, non_empty, nullable, parse_date, parse_datetime,
    pick_color, slug_id,
};
use super::{IdArgs, Op, op};
use crate::ctx::{Access, Ctx};
use crate::error::{ApiError, ApiResult};

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "list_projects",
            Access::Read,
            &[("GET", "/projects")],
            "List projects in teams you can see, most recently updated first. Narrow with team, status (name or type: \
             backlog, planned, started, paused, completed, canceled), lead (\"me\"), member, initiative, label, health, \
             priority, query, or a saved project view (customView). Archived and trashed projects are left out unless \
             asked for; descriptions unless includeDescription is set. Each project has its progress: completed issues \
             / (all - canceled).",
            list,
        ),
        op(
            "get_project",
            Access::Read,
            &[("GET", "/projects/{id}")],
            "Read a project by id, slug, link or name: its fields and description, members, milestones with their \
             progress, latest status update, links, dependencies (blocks, blocked by, related), documents, issue counts \
             by status type and estimate sums.",
            get,
        ),
        op(
            "save_project",
            Access::Write,
            &[("POST", "/projects"), ("PATCH", "/projects/{id}")],
            "Create a project (omit id; name and at least one team via setTeams or addTeams are required) or update one \
             (pass id). Omitted fields stay as they are; null clears a field. People are given as id, name, email or \
             \"me\"; the status (state) as name, type or id. Use patch to edit part of the description. A project template \
             (create only) adds its milestones and issues. A new lead is notified; so are people @mentioned in the \
             description.",
            save,
        ),
        op(
            "delete_project",
            Access::Write,
            &[("DELETE", "/projects/{id}")],
            "Move a project to the trash. Its issues keep their project. It is deleted for good after 30 days; \
             restore_project brings it back before then.",
            trash,
        ),
        op(
            "restore_project",
            Access::Write,
            &[("POST", "/projects/{id}/restore")],
            "Bring a project back from the trash.",
            restore,
        ),
        op(
            "archive_project",
            Access::Write,
            &[("POST", "/projects/{id}/archive")],
            "Archive a project. It stays readable but is left out of lists unless includeArchived is set.",
            archive,
        ),
        op(
            "unarchive_project",
            Access::Write,
            &[("POST", "/projects/{id}/unarchive")],
            "Bring an archived project back.",
            unarchive,
        ),
        op(
            "list_project_statuses",
            Access::Read,
            &[("GET", "/project-statuses")],
            "The workspace's project statuses in order, each with its type (backlog, planned, started, paused, \
             completed, canceled) and how many projects are in it.",
            list_statuses,
        ),
        op(
            "save_project_status",
            Access::Write,
            &[("POST", "/project-statuses"), ("PATCH", "/project-statuses/{id}")],
            "Create a project status (omit id; name and type are required) or change one (pass its id or name). \
             Workspace admins only.",
            save_status,
        ),
        op(
            "delete_project_status",
            Access::Write,
            &[("DELETE", "/project-statuses/{id}")],
            "Delete a project status. Projects in it move to moveProjectsTo, which is required when there are any. \
             The workspace keeps at least one status. Workspace admins only.",
            delete_status,
        ),
        op(
            "list_project_labels",
            Access::Read,
            &[("GET", "/project-labels")],
            "The workspace's project labels and label groups, with how many projects use each. Retired labels are \
             left out unless includeRetired is set.",
            list_labels,
        ),
        op(
            "save_project_label",
            Access::Write,
            &[("POST", "/project-labels"), ("PATCH", "/project-labels/{id}")],
            "Create a project label (omit id; name is required) or change one (pass its id or name). A label can sit in \
             a group (parent, a label with isGroup); groups don't nest.",
            save_label,
        ),
        op(
            "retire_project_label",
            Access::Write,
            &[("POST", "/project-labels/{id}/retire")],
            "Retire a project label (and a group's labels): projects keep it, but it can't be added anymore.",
            retire_label,
        ),
        op(
            "restore_project_label",
            Access::Write,
            &[("POST", "/project-labels/{id}/restore")],
            "Bring a retired project label back.",
            restore_label,
        ),
        op(
            "delete_project_label",
            Access::Write,
            &[("DELETE", "/project-labels/{id}")],
            "Delete a project label for good, removing it from every project.",
            delete_label,
        ),
    ]
}

/* ------------------------------------------------------------------ */
/*                           Representation                           */
/* ------------------------------------------------------------------ */

pub const PROJECT_FROM: &str = "FROM projects p JOIN workspaces w ON w.id = p.workspace_id";

pub fn sql_literal(value: &str) -> String {
    format!("'{}'", value.replace('\'', "''"))
}

/// The JSON of the project `p` (with its workspace `w` joined).
pub fn project_json(ctx: &Ctx, with_description: bool) -> String {
    let base = sql_literal(ctx.public_url());
    let description = if with_description { "'description', p.content," } else { "" };
    format!(
        "(jsonb_build_object(
            'id', p.id,
            'slugId', p.slug_id,
            'name', p.name,
            'summary', p.summary,
            {description}
            'url', {base} || '/' || w.url_key || '/project/' || concat_ws('-', nullif(entity_slug(p.name), ''), p.slug_id),
            'icon', p.icon,
            'color', p.color,
            'status', project_status_ref(p.status_id),
            'priority', p.priority,
            'priorityLabel', (ARRAY['No priority','Urgent','High','Medium','Low'])[p.priority + 1],
            'health', p.health,
            'lead', user_ref(p.lead_id),
            'leadTeam', team_ref(p.lead_team_id),
            'teams', (SELECT coalesce(jsonb_agg(team_ref(pt.team_id) ORDER BY tt.key), '[]')
                FROM project_teams pt JOIN teams tt ON tt.id = pt.team_id WHERE pt.project_id = p.id),
            'labels', (SELECT coalesce(jsonb_agg(project_label_ref(pl.label_id) ORDER BY l.name), '[]')
                FROM project_label_links pl JOIN project_labels l ON l.id = pl.label_id WHERE pl.project_id = p.id),
            'initiatives', (SELECT coalesce(jsonb_agg(initiative_ref(ip.initiative_id) ORDER BY ini.name), '[]')
                FROM initiative_projects ip JOIN initiatives ini ON ini.id = ip.initiative_id
                WHERE ip.project_id = p.id AND ini.trashed_at IS NULL)
        ) || jsonb_build_object(
            'startDate', p.start_date,
            'startDateResolution', p.start_date_resolution,
            'targetDate', p.target_date,
            'targetDateResolution', p.target_date_resolution,
            'sortOrder', p.sort_order,
            'startedAt', p.started_at,
            'completedAt', p.completed_at,
            'canceledAt', p.canceled_at,
            'archivedAt', p.archived_at,
            'trashed', p.trashed_at IS NOT NULL,
            'trashedAt', p.trashed_at,
            'createdAt', p.created_at,
            'updatedAt', p.updated_at
        ) || project_issue_stats(p.id, NULL))"
    )
}

/// One project's JSON, with its description.
pub async fn project_value(ctx: &Ctx, db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    let sql = format!("SELECT {} {PROJECT_FROM} WHERE p.id = $1", project_json(ctx, true));
    let value: Option<Value> = sqlx::query_scalar(&sql).bind(id).fetch_optional(&mut *db).await?;
    value.ok_or_else(|| ApiError::not_found("No such project"))
}

/// The JSON array of the links on the thing whose id is in `column` (project_id or initiative_id),
/// as a subquery on `$1`.
pub fn links_sql(column: &str) -> String {
    format!(
        "(SELECT coalesce(jsonb_agg(jsonb_build_object('id', l.id, 'url', l.url, 'label', l.label,
            'creator', user_ref(l.creator_id), 'createdAt', l.created_at) ORDER BY l.sort_order, l.created_at), '[]')
          FROM entity_links l WHERE l.{column} = $1)"
    )
}

/// Everything get_project shows: the project, its people, milestones, latest update, links,
/// dependencies, documents, issue counts and estimates.
async fn project_detail(ctx: &Ctx, db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    let mut value = project_value(ctx, db, id).await?;
    let teams = ctx.team_ids().await?.to_vec();
    let visible = resolve::project_visible("o", 2);
    let sql = format!(
        "SELECT jsonb_build_object(
            'creator', (SELECT user_ref(creator_id) FROM projects WHERE id = $1),
            'members', (SELECT coalesce(jsonb_agg(user_ref(m.user_id) ORDER BY u.name), '[]')
                FROM project_members m JOIN users u ON u.id = m.user_id WHERE m.project_id = $1),
            'subscribers', (SELECT coalesce(jsonb_agg(user_ref(s.user_id) ORDER BY u.name), '[]')
                FROM project_subscribers s JOIN users u ON u.id = s.user_id WHERE s.project_id = $1),
            'links', {links},
            'relations', jsonb_build_object(
                'blocks', (SELECT coalesce(jsonb_agg(project_ref(o.id) ORDER BY r.created_at), '[]')
                    FROM project_relations r JOIN projects o ON o.id = r.related_project_id
                    WHERE r.project_id = $1 AND r.type = 'blocks' AND o.trashed_at IS NULL AND {visible}),
                'blockedBy', (SELECT coalesce(jsonb_agg(project_ref(o.id) ORDER BY r.created_at), '[]')
                    FROM project_relations r JOIN projects o ON o.id = r.project_id
                    WHERE r.related_project_id = $1 AND r.type = 'blocks' AND o.trashed_at IS NULL AND {visible}),
                'related', (SELECT coalesce(jsonb_agg(project_ref(o.id) ORDER BY r.created_at), '[]')
                    FROM project_relations r
                    JOIN projects o ON o.id = CASE WHEN r.project_id = $1 THEN r.related_project_id ELSE r.project_id END
                    WHERE (r.project_id = $1 OR r.related_project_id = $1) AND r.type = 'related'
                    AND o.trashed_at IS NULL AND {visible})
            ),
            'documents', (SELECT coalesce(jsonb_agg(document_ref(d.id) ORDER BY d.sort_order, d.created_at), '[]')
                FROM documents d WHERE d.project_id = $1 AND d.trashed_at IS NULL),
            'issueCountByStateType', (SELECT coalesce(jsonb_object_agg(x.type, x.n), '{{}}') FROM (
                SELECT s.type, count(*) AS n FROM issues i JOIN workflow_states s ON s.id = i.state_id
                WHERE i.project_id = $1 AND i.trashed_at IS NULL GROUP BY s.type) x),
            'estimate', (SELECT jsonb_build_object('total', coalesce(sum(i.estimate), 0),
                    'completed', coalesce(sum(i.estimate) FILTER (WHERE s.type = 'completed'), 0))
                FROM issues i JOIN workflow_states s ON s.id = i.state_id
                WHERE i.project_id = $1 AND i.trashed_at IS NULL AND s.type <> 'canceled')
        )",
        links = links_sql("project_id"),
    );
    let more: Value = sqlx::query_scalar(&sql).bind(id).bind(&teams).fetch_one(&mut *db).await?;
    if let (Some(target), Value::Object(more)) = (value.as_object_mut(), more) {
        target.extend(more);
    }
    value["milestones"] = super::milestones::project_milestones(db, id).await?;
    value["latestUpdate"] = super::status_updates::latest(db, "project_id", id).await?.unwrap_or(Value::Null);
    Ok(value)
}

/// The team a project's webhook events are filed under: its lead team, else its first team.
pub async fn webhook_team(db: &mut PgConnection, project: Uuid) -> ApiResult<Option<Uuid>> {
    Ok(sqlx::query_scalar(
        "SELECT coalesce(p.lead_team_id, (SELECT pt.team_id FROM project_teams pt JOIN teams t ON t.id = pt.team_id
            WHERE pt.project_id = p.id ORDER BY t.key LIMIT 1)) FROM projects p WHERE p.id = $1",
    )
    .bind(project)
    .fetch_one(&mut *db)
    .await?)
}

/* ------------------------------------------------------------------ */
/*                                List                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema, Default)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ListProjectsArgs {
    /// Team key, name or id.
    team: Option<String>,
    /// Project status name, type (backlog, planned, started, paused, completed, canceled) or id.
    status: Option<String>,
    /// Lead (id, name, email or "me"), or "null" for projects without one.
    lead: Option<String>,
    /// A project member (id, name, email or "me").
    member: Option<String>,
    /// Initiative name, slug or id.
    initiative: Option<String>,
    /// Project label name or id.
    label: Option<String>,
    /// onTrack, atRisk or offTrack.
    health: Option<String>,
    /// 0 none, 1 urgent, 2 high, 3 medium, 4 low.
    priority: Option<i32>,
    /// Words to find in the name, summary or description.
    query: Option<String>,
    /// A saved project view (name, slug or id) whose filter applies too.
    custom_view: Option<String>,
    #[schemars(description = "Further conditions, as in project views. Fields: status {id,name,type}, \
        lead {id,isMe}, team {id,key}, priority, health, name, startDate, targetDate, and, or. Comparators: eq, neq, \
        in, nin, lt, lte, gt, gte, null; dates take ISO 8601 or a duration from now like P2W. \
        Example: {\"status\": {\"type\": {\"in\": [\"planned\", \"started\"]}}, \"targetDate\": {\"lt\": \"P1M\"}}")]
    filter: Option<Value>,
    #[serde(default)]
    include_archived: bool,
    #[serde(default)]
    include_trashed: bool,
    /// Include each project's description.
    #[serde(default)]
    include_description: bool,
    /// updatedAt (default), createdAt, targetDate, priority or manual.
    order_by: Option<String>,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

const STATUS_TYPES: [&str; 6] = ["backlog", "planned", "started", "paused", "completed", "canceled"];
const HEALTHS: [&str; 3] = ["onTrack", "atRisk", "offTrack"];

pub fn check_health(value: &str) -> ApiResult<String> {
    HEALTHS
        .iter()
        .find(|h| h.eq_ignore_ascii_case(value.trim()))
        .map(|h| h.to_string())
        .ok_or_else(|| ApiError::bad_request(format!("health is onTrack, atRisk or offTrack, not \"{value}\"")))
}

pub fn is_null_word(value: &str) -> bool {
    matches!(value.trim().to_lowercase().as_str(), "null" | "none")
}

async fn list(ctx: Ctx, args: ListProjectsArgs) -> ApiResult<Value> {
    let page = Page { limit: args.limit, cursor: args.cursor.clone() };
    let order = match args.order_by.as_deref().unwrap_or("updatedAt") {
        "updatedAt" => "p.updated_at DESC",
        "createdAt" => "p.created_at DESC",
        "targetDate" => "p.target_date ASC NULLS LAST, p.updated_at DESC",
        "priority" => "CASE WHEN p.priority = 0 THEN 5 ELSE p.priority END, p.updated_at DESC",
        "manual" => "p.sort_order ASC, p.created_at",
        other => {
            return Err(ApiError::bad_request(format!(
                "orderBy \"{other}\" isn't one of updatedAt, createdAt, targetDate, priority, manual"
            )));
        }
    };
    let teams = ctx.team_ids().await?.to_vec();
    let mut qb = QueryBuilder::new(format!(
        "SELECT {} {PROJECT_FROM} WHERE p.workspace_id = ",
        project_json(&ctx, args.include_description)
    ));
    qb.push_bind(ctx.workspace);
    qb.push(" AND EXISTS (SELECT 1 FROM project_teams vt WHERE vt.project_id = p.id AND vt.team_id = ANY(");
    qb.push_bind(teams);
    qb.push("))");
    if !args.include_archived {
        qb.push(" AND p.archived_at IS NULL");
    }
    if !args.include_trashed {
        qb.push(" AND p.trashed_at IS NULL");
    }
    if let Some(team) = &args.team {
        qb.push(" AND EXISTS (SELECT 1 FROM project_teams ft WHERE ft.project_id = p.id AND ft.team_id = ");
        qb.push_bind(resolve::team(&ctx, team).await?);
        qb.push(")");
    }
    if let Some(status) = &args.status {
        let lower = status.trim().to_lowercase();
        if STATUS_TYPES.contains(&lower.as_str()) {
            qb.push(" AND EXISTS (SELECT 1 FROM project_statuses fs WHERE fs.id = p.status_id AND fs.type = ");
            qb.push_bind(lower);
            qb.push(")");
        } else {
            qb.push(" AND p.status_id = ");
            qb.push_bind(resolve::project_status(&ctx, status).await?);
        }
    }
    if let Some(lead) = &args.lead {
        if is_null_word(lead) {
            qb.push(" AND p.lead_id IS NULL");
        } else {
            qb.push(" AND p.lead_id = ");
            qb.push_bind(resolve::user(&ctx, lead).await?);
        }
    }
    if let Some(member) = &args.member {
        qb.push(" AND EXISTS (SELECT 1 FROM project_members fm WHERE fm.project_id = p.id AND fm.user_id = ");
        qb.push_bind(resolve::user(&ctx, member).await?);
        qb.push(")");
    }
    if let Some(initiative) = &args.initiative {
        qb.push(" AND EXISTS (SELECT 1 FROM initiative_projects fi WHERE fi.project_id = p.id AND fi.initiative_id = ");
        qb.push_bind(resolve::initiative(&ctx, initiative).await?);
        qb.push(")");
    }
    if let Some(label) = &args.label {
        let ids = resolve::project_labels(&ctx, std::slice::from_ref(label)).await?;
        qb.push(" AND EXISTS (SELECT 1 FROM project_label_links fl WHERE fl.project_id = p.id AND fl.label_id = ANY(");
        qb.push_bind(ids);
        qb.push("))");
    }
    if let Some(health) = &args.health {
        qb.push(" AND p.health = ");
        qb.push_bind(check_health(health)?);
    }
    if let Some(priority) = args.priority {
        qb.push(" AND p.priority = ");
        qb.push_bind(check_priority(priority)?);
    }
    if let Some(query) = args.query.as_deref().map(str::trim).filter(|q| !q.is_empty()) {
        qb.push(" AND (p.search @@ websearch_to_tsquery('english', ");
        qb.push_bind(query.to_string());
        qb.push(") OR p.name ILIKE ");
        qb.push_bind(format!("%{}%", escape_like(query)));
        qb.push(")");
    }
    if let Some(view) = &args.custom_view {
        let id = resolve::custom_view(&ctx, view).await?;
        let (model, view_filter, view_team): (String, Value, Option<Uuid>) =
            sqlx::query_as("SELECT model, filter, team_id FROM custom_views WHERE id = $1")
                .bind(id)
                .fetch_one(ctx.db())
                .await?;
        if model != "project" {
            return Err(ApiError::bad_request(format!("That view shows {model}s, not projects")));
        }
        qb.push(" AND ");
        push_filter(&mut qb, &ctx, &view_filter)?;
        if let Some(team) = view_team {
            qb.push(" AND EXISTS (SELECT 1 FROM project_teams vw WHERE vw.project_id = p.id AND vw.team_id = ");
            qb.push_bind(team);
            qb.push(")");
        }
    }
    if let Some(extra) = &args.filter {
        qb.push(" AND ");
        push_filter(&mut qb, &ctx, extra)?;
    }
    qb.push(format!(" ORDER BY {order} LIMIT "));
    qb.push_bind(page.limit() + 1);
    qb.push(" OFFSET ");
    qb.push_bind(page.offset()?);
    let nodes: Vec<Value> = qb.build_query_scalar().fetch_all(ctx.db()).await?;
    page.wrap(nodes)
}

/// Pushes the condition of a project filter (the filter of a project view) on `p`.
fn push_filter(qb: &mut QueryBuilder<'_, Postgres>, ctx: &Ctx, filter: &Value) -> ApiResult<()> {
    let object = filter.as_object().ok_or_else(|| ApiError::bad_request("A project filter must be an object"))?;
    qb.push("(TRUE");
    for (key, value) in object {
        qb.push(" AND ");
        match key.as_str() {
            "and" | "or" => {
                let list = value
                    .as_array()
                    .ok_or_else(|| ApiError::bad_request(format!("\"{key}\" takes a list of filters")))?;
                qb.push(if key == "and" { "(TRUE" } else { "(FALSE" });
                for item in list {
                    qb.push(if key == "and" { " AND " } else { " OR " });
                    push_filter(qb, ctx, item)?;
                }
                qb.push(")");
            }
            "priority" => compare(qb, "p.priority", "int", value)?,
            "health" => compare(qb, "p.health", "text", value)?,
            "name" => compare(qb, "p.name", "text", value)?,
            "startDate" => compare(qb, "p.start_date", "date", value)?,
            "targetDate" => compare(qb, "p.target_date", "date", value)?,
            "status" => {
                qb.push("EXISTS (SELECT 1 FROM project_statuses fs WHERE fs.id = p.status_id");
                nested(
                    qb,
                    key,
                    value,
                    &[("id", "fs.id", "uuid"), ("name", "fs.name", "text"), ("type", "fs.type", "text")],
                )?;
                qb.push(")");
            }
            "team" | "teams" => {
                qb.push("EXISTS (SELECT 1 FROM project_teams ft JOIN teams t ON t.id = ft.team_id WHERE ft.project_id = p.id");
                nested(
                    qb,
                    key,
                    value,
                    &[("id", "t.id", "uuid"), ("key", "t.key", "text"), ("name", "t.name", "text")],
                )?;
                qb.push(")");
            }
            "lead" => {
                let fields = value.as_object().ok_or_else(|| ApiError::bad_request("lead takes {id, isMe, null}"))?;
                qb.push("(TRUE");
                for (field, condition) in fields {
                    qb.push(" AND ");
                    match field.as_str() {
                        "id" => compare(qb, "p.lead_id", "uuid", condition)?,
                        "null" => {
                            qb.push(if condition.as_bool().unwrap_or(true) {
                                "p.lead_id IS NULL"
                            } else {
                                "p.lead_id IS NOT NULL"
                            });
                        }
                        "isMe" => {
                            let me = condition.as_bool().or_else(|| condition["eq"].as_bool()).unwrap_or(true);
                            qb.push(if me { "p.lead_id = " } else { "p.lead_id IS DISTINCT FROM " });
                            qb.push_bind(ctx.user);
                        }
                        other => {
                            return Err(ApiError::bad_request(format!(
                                "lead filters take id, isMe or null, not \"{other}\""
                            )));
                        }
                    }
                }
                qb.push(")");
            }
            other => {
                return Err(ApiError::bad_request(format!(
                    "Project filters don't have \"{other}\"; use status, lead, team, priority, health, name, startDate, \
                     targetDate, and, or"
                )));
            }
        }
    }
    qb.push(")");
    Ok(())
}

/// ` AND <column> <comparators>` for each field of a nested filter like `status: {type: {eq: "started"}}`.
fn nested(
    qb: &mut QueryBuilder<'_, Postgres>,
    key: &str,
    value: &Value,
    fields: &[(&str, &str, &str)],
) -> ApiResult<()> {
    let object = value.as_object().ok_or_else(|| ApiError::bad_request(format!("{key} takes an object")))?;
    for (field, condition) in object {
        let Some((_, column, ty)) = fields.iter().find(|(name, _, _)| name == field) else {
            let names: Vec<_> = fields.iter().map(|f| f.0).collect();
            return Err(ApiError::bad_request(format!("{key} filters take {}, not \"{field}\"", names.join(", "))));
        };
        qb.push(" AND ");
        compare(qb, column, ty, condition)?;
    }
    Ok(())
}

fn scalar(value: &Value, ty: &str) -> ApiResult<String> {
    let text = match value {
        Value::String(s) => s.clone(),
        Value::Number(n) => n.to_string(),
        Value::Bool(b) => b.to_string(),
        other => return Err(ApiError::bad_request(format!("{other} isn't a value to compare with"))),
    };
    match ty {
        "date" => Ok(parse_datetime(&text)?.date_naive().to_string()),
        "text" => Ok(text.to_lowercase()),
        _ => Ok(text),
    }
}

/// `<column> <comparators>`; a plain value means eq. Text compares without case.
fn compare(qb: &mut QueryBuilder<'_, Postgres>, column: &str, ty: &str, condition: &Value) -> ApiResult<()> {
    let column = if ty == "text" { format!("lower({column})") } else { column.to_string() };
    let Some(comparators) = condition.as_object() else {
        qb.push(format!("{column} = "));
        qb.push_bind(scalar(condition, ty)?);
        qb.push(format!("::{ty}"));
        return Ok(());
    };
    qb.push("(TRUE");
    for (comparator, value) in comparators {
        qb.push(" AND ");
        let sign = match comparator.as_str() {
            "eq" => "=",
            "neq" => "IS DISTINCT FROM",
            "lt" => "<",
            "lte" => "<=",
            "gt" => ">",
            "gte" => ">=",
            "in" | "nin" => {
                let list = value
                    .as_array()
                    .ok_or_else(|| ApiError::bad_request(format!("{comparator} takes a list")))?
                    .iter()
                    .map(|v| scalar(v, ty))
                    .collect::<ApiResult<Vec<String>>>()?;
                qb.push(if comparator == "in" { format!("{column} = ANY(") } else { format!("NOT ({column} = ANY(") });
                qb.push_bind(list);
                qb.push(format!("::{ty}[])"));
                if comparator == "nin" {
                    qb.push(")");
                }
                continue;
            }
            "null" => {
                qb.push(if value.as_bool().unwrap_or(true) {
                    format!("{column} IS NULL")
                } else {
                    format!("{column} IS NOT NULL")
                });
                continue;
            }
            other => {
                return Err(ApiError::bad_request(format!(
                    "\"{other}\" isn't a comparator; use eq, neq, in, nin, lt, lte, gt, gte or null"
                )));
            }
        };
        qb.push(format!("{column} {sign} "));
        qb.push_bind(scalar(value, ty)?);
        qb.push(format!("::{ty}"));
    }
    qb.push(")");
    Ok(())
}

/* ------------------------------------------------------------------ */
/*                                Read                                */
/* ------------------------------------------------------------------ */

async fn get(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = resolve::project(&ctx, &args.id).await?;
    let mut db = ctx.db().acquire().await?;
    project_detail(&ctx, &mut db, id).await
}

/* ------------------------------------------------------------------ */
/*                                Save                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema, Default)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SaveProjectArgs {
    /// Only to update a project: its id, slug, link or name. Omit to create one.
    id: Option<String>,
    /// Required to create.
    name: Option<String>,
    /// A one-line summary, at most 255 characters; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    summary: Option<Option<String>>,
    /// Markdown. Mention people with @displayName or @email; they are notified.
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
    /// Project status name, type (backlog, planned, started, paused, completed, canceled) or id. New projects
    /// start in the first backlog status.
    state: Option<String>,
    /// 0 none, 1 urgent, 2 high, 3 medium, 4 low.
    priority: Option<i32>,
    /// Person (id, name, email or "me"); null removes the lead.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    lead: Option<Option<String>>,
    /// One of the project's teams (key, name or id) that leads it; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    lead_team: Option<Option<String>>,
    /// Teams (keys, names or ids) to add.
    add_teams: Option<Vec<String>>,
    remove_teams: Option<Vec<String>>,
    /// Replace all teams with these. A project keeps at least one team.
    set_teams: Option<Vec<String>>,
    /// People to add as members (they are subscribed).
    add_members: Option<Vec<String>>,
    remove_members: Option<Vec<String>>,
    /// Replace all project labels with these (names or ids; Group/Label for a label in a group).
    labels: Option<Vec<String>>,
    add_labels: Option<Vec<String>>,
    remove_labels: Option<Vec<String>>,
    /// YYYY-MM-DD; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    start_date: Option<Option<String>>,
    /// How precise startDate is: month, quarter, halfYear or year; null means the exact day.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    start_date_resolution: Option<Option<String>>,
    /// YYYY-MM-DD; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    target_date: Option<Option<String>>,
    /// How precise targetDate is: month, quarter, halfYear or year; null means the exact day.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    target_date_resolution: Option<Option<String>>,
    /// Links to add ({url, title}); a link already on the project gets the new title.
    links: Option<Vec<LinkArg>>,
    /// URLs of links to remove.
    remove_links: Option<Vec<String>>,
    /// Initiatives (names, slugs or ids) to add the project to.
    add_initiatives: Option<Vec<String>>,
    remove_initiatives: Option<Vec<String>>,
    /// Replace all initiatives with these.
    set_initiatives: Option<Vec<String>>,
    /// Projects this one blocks (added to existing dependencies).
    blocks: Option<Vec<String>>,
    /// Projects blocking this one (added).
    blocked_by: Option<Vec<String>>,
    /// Projects related to this one (added).
    related_to: Option<Vec<String>>,
    remove_blocks: Option<Vec<String>>,
    remove_blocked_by: Option<Vec<String>>,
    remove_related_to: Option<Vec<String>>,
    /// Position in manually ordered lists (lower is higher up).
    sort_order: Option<f64>,
    /// Project template name or id, applied on create: it fills in what the arguments don't set and adds its
    /// milestones and issues.
    template: Option<String>,
}

/// What a project template fills in on a new project.
#[derive(Deserialize, Default)]
#[serde(rename_all = "camelCase", default)]
struct ProjectTemplate {
    name: Option<String>,
    summary: Option<String>,
    description: Option<String>,
    state: Option<String>,
    priority: Option<i32>,
    lead: Option<String>,
    labels: Option<Vec<String>>,
    teams: Option<Vec<String>>,
    milestones: Vec<TemplateMilestone>,
    issues: Vec<Value>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct TemplateMilestone {
    name: String,
    description: Option<String>,
    target_date: Option<String>,
}

#[derive(sqlx::FromRow)]
struct ProjectRow {
    name: String,
    summary: Option<String>,
    content: String,
    icon: Option<String>,
    color: Option<String>,
    status_id: Uuid,
    priority: i32,
    lead_id: Option<Uuid>,
}

async fn load_row(db: &mut PgConnection, id: Uuid) -> ApiResult<ProjectRow> {
    let row = sqlx::query_as::<_, ProjectRow>(
        "SELECT name, summary, content, icon, color, status_id, priority, lead_id FROM projects WHERE id = $1 FOR UPDATE",
    )
    .bind(id)
    .fetch_optional(&mut *db)
    .await?;
    row.ok_or_else(|| ApiError::not_found("No such project"))
}

async fn save(ctx: Ctx, args: SaveProjectArgs) -> ApiResult<Value> {
    let (id, template_issues) = match args.id.clone() {
        Some(reference) => {
            let id = resolve::project(&ctx, &reference).await?;
            let mut tx = ctx.db().begin().await?;
            update(&ctx, &mut tx, id, args).await?;
            tx.commit().await?;
            (id, vec![])
        }
        None => {
            ctx.require_member()?;
            let mut tx = ctx.db().begin().await?;
            let created = create(&ctx, &mut tx, args).await?;
            tx.commit().await?;
            created
        }
    };
    for (n, issue) in template_issues.into_iter().enumerate() {
        let created = super::find("save_issue").expect("save_issue exists").call(ctx.clone(), issue).await;
        if let Err(e) = created {
            return Err(ApiError::bad_request(format!(
                "The project was created, but the template's issue {} failed: {}",
                n + 1,
                e.message
            )));
        }
    }
    let mut db = ctx.db().acquire().await?;
    project_detail(&ctx, &mut db, id).await
}

/// The status new projects start in: the first backlog status, else the first planned one.
async fn default_status(db: &mut PgConnection, workspace: Uuid) -> ApiResult<Uuid> {
    let id: Option<Uuid> = sqlx::query_scalar(
        "SELECT id FROM project_statuses WHERE workspace_id = $1 AND archived_at IS NULL
         ORDER BY array_position(ARRAY['backlog', 'planned'], type) NULLS LAST, position LIMIT 1",
    )
    .bind(workspace)
    .fetch_optional(&mut *db)
    .await?;
    id.ok_or_else(|| ApiError::bad_request("The workspace has no project statuses; add one with save_project_status"))
}

/// Creates the project and its template milestones; returns the template's issues to create
/// (as save_issue arguments) once the project is committed.
async fn create(ctx: &Ctx, tx: &mut PgConnection, mut args: SaveProjectArgs) -> ApiResult<(Uuid, Vec<Value>)> {
    let mut template = ProjectTemplate::default();
    if let Some(reference) = args.template.take() {
        let id = resolve::template(ctx, "project", &reference).await?;
        let (data, team): (Value, Option<Uuid>) =
            sqlx::query_as("SELECT data, team_id FROM templates WHERE id = $1").bind(id).fetch_one(&mut *tx).await?;
        template = serde_json::from_value(data)
            .map_err(|e| ApiError::bad_request(format!("The project template is invalid: {e}")))?;
        args.name = args.name.or(template.name.take());
        args.summary = args.summary.or(template.summary.take().map(Some));
        if args.description.as_deref().is_none_or(str::is_empty) && args.patch.is_none() {
            args.description = template.description.take();
        }
        args.state = args.state.or(template.state.take());
        args.priority = args.priority.or(template.priority);
        args.lead = args.lead.or(template.lead.take().map(Some));
        if args.labels.is_none()
            && let Some(labels) = template.labels.take()
        {
            args.add_labels.get_or_insert_with(Vec::new).extend(labels);
        }
        if args.set_teams.is_none() && args.add_teams.as_ref().is_none_or(Vec::is_empty) {
            args.add_teams = template.teams.take().or(team.map(|t| vec![t.to_string()]));
        }
    }
    let name = non_empty(args.name.as_deref().unwrap_or(""), "name")?;
    if args.set_teams.as_ref().or(args.add_teams.as_ref()).is_none_or(Vec::is_empty) {
        return Err(ApiError::bad_request("A project needs at least one team; pass setTeams or addTeams"));
    }
    let status = match args.state.take() {
        Some(state) => resolve::project_status(ctx, &state).await?,
        None => default_status(tx, ctx.workspace).await?,
    };
    let id: Uuid = sqlx::query_scalar(
        "INSERT INTO projects (workspace_id, slug_id, name, status_id, creator_id, sort_order)
         VALUES ($1, $2, $3, $4, $5, (SELECT coalesce(max(sort_order), 0) + 1000 FROM projects WHERE workspace_id = $1))
         RETURNING id",
    )
    .bind(ctx.workspace)
    .bind(slug_id())
    .bind(&name)
    .bind(status)
    .bind(ctx.user)
    .fetch_one(&mut *tx)
    .await?;
    stamp_status(tx, id).await?;
    subscribe(tx, id, &[ctx.user]).await?;
    apply_changes(ctx, tx, id, args, true).await?;

    let mut milestones: Vec<(String, Uuid)> = Vec::new();
    for (n, milestone) in template.milestones.iter().enumerate() {
        let target = milestone.target_date.as_deref().map(parse_datetime).transpose()?.map(|t| t.date_naive());
        let milestone_id: Uuid = sqlx::query_scalar(
            "INSERT INTO project_milestones (project_id, name, description, target_date, sort_order)
             VALUES ($1, $2, $3, $4, $5) RETURNING id",
        )
        .bind(id)
        .bind(non_empty(&milestone.name, "A template milestone's name")?)
        .bind(&milestone.description)
        .bind(target)
        .bind(n as f64)
        .fetch_one(&mut *tx)
        .await?;
        milestones.push((milestone.name.trim().to_lowercase(), milestone_id));
    }
    let first_team = webhook_team(tx, id).await?;
    let mut issues = Vec::new();
    for (n, issue) in template.issues.into_iter().enumerate() {
        let Value::Object(mut issue) = issue else {
            return Err(ApiError::bad_request(format!("The template's issue {} isn't an object", n + 1)));
        };
        issue.remove("id");
        issue.insert("project".into(), json!(id.to_string()));
        if !issue.contains_key("team")
            && let Some(team) = first_team
        {
            issue.insert("team".into(), json!(team.to_string()));
        }
        if let Some(Value::String(milestone)) = issue.get("milestone")
            && let Some((_, found)) = milestones.iter().find(|(name, _)| *name == milestone.trim().to_lowercase())
        {
            issue.insert("milestone".into(), json!(found.to_string()));
        }
        issues.push(Value::Object(issue));
    }
    let value = project_value(ctx, tx, id).await?;
    events::emit(tx, ctx, "Project", "create", &value, None, first_team).await?;
    Ok((id, issues))
}

async fn update(ctx: &Ctx, tx: &mut PgConnection, id: Uuid, mut args: SaveProjectArgs) -> ApiResult<()> {
    if args.template.is_some() {
        return Err(ApiError::bad_request("Templates only apply when creating a project"));
    }
    let before = project_value(ctx, tx, id).await?;
    let row = load_row(tx, id).await?;
    if let Some(name) = args.name.take() {
        let name = non_empty(&name, "name")?;
        sqlx::query("UPDATE projects SET name = $2 WHERE id = $1").bind(id).bind(&name).execute(&mut *tx).await?;
    }
    if let Some(state) = args.state.take() {
        let status = resolve::project_status(ctx, &state).await?;
        if status != row.status_id {
            sqlx::query("UPDATE projects SET status_id = $2 WHERE id = $1")
                .bind(id)
                .bind(status)
                .execute(&mut *tx)
                .await?;
            stamp_status(tx, id).await?;
        }
    }
    apply_changes(ctx, tx, id, args, false).await?;
    sqlx::query("UPDATE projects SET updated_at = now() WHERE id = $1").bind(id).execute(&mut *tx).await?;
    let after = project_value(ctx, tx, id).await?;
    let updated_from = changed_fields(&before, &after);
    if !updated_from.as_object().is_none_or(Map::is_empty) {
        let team = webhook_team(tx, id).await?;
        events::emit(tx, ctx, "Project", "update", &after, Some(updated_from), team).await?;
    }
    Ok(())
}

/// Stamps when the project started, completed or was canceled, from the type of its status.
async fn stamp_status(tx: &mut PgConnection, id: Uuid) -> ApiResult<()> {
    sqlx::query(
        "UPDATE projects p SET
            started_at = CASE WHEN s.type = 'started' THEN coalesce(p.started_at, now())
                              WHEN s.type IN ('paused', 'completed', 'canceled') THEN p.started_at END,
            completed_at = CASE WHEN s.type = 'completed' THEN coalesce(p.completed_at, now()) END,
            canceled_at = CASE WHEN s.type = 'canceled' THEN coalesce(p.canceled_at, now()) END
         FROM project_statuses s WHERE p.id = $1 AND s.id = p.status_id",
    )
    .bind(id)
    .execute(&mut *tx)
    .await?;
    Ok(())
}

pub async fn subscribe(tx: &mut PgConnection, project: Uuid, users: &[Uuid]) -> ApiResult<()> {
    sqlx::query(
        "INSERT INTO project_subscribers (project_id, user_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING",
    )
    .bind(project)
    .bind(users)
    .execute(&mut *tx)
    .await?;
    Ok(())
}

fn check_resolution(value: Option<String>) -> ApiResult<Option<String>> {
    let Some(value) = value.filter(|v| !is_null_word(v)) else { return Ok(None) };
    ["month", "quarter", "halfYear", "year"]
        .iter()
        .find(|r| r.eq_ignore_ascii_case(value.trim()))
        .map(|r| Some(r.to_string()))
        .ok_or_else(|| {
            ApiError::bad_request(format!("A date resolution is month, quarter, halfYear or year, not \"{value}\""))
        })
}

fn optional_date(value: Option<String>) -> ApiResult<Option<chrono::NaiveDate>> {
    value.filter(|v| !is_null_word(v)).map(|v| parse_date(&v)).transpose()
}

/// The fields that work the same when creating and updating.
async fn apply_changes(
    ctx: &Ctx,
    tx: &mut PgConnection,
    id: Uuid,
    args: SaveProjectArgs,
    creating: bool,
) -> ApiResult<()> {
    let row = load_row(tx, id).await?;
    relations(ctx, tx, id, &args).await?;
    if let Some(summary) = args.summary {
        let summary = summary.map(|s| s.trim().to_string()).filter(|s| !s.is_empty());
        if summary.as_ref().is_some_and(|s| s.chars().count() > 255) {
            return Err(ApiError::bad_request("summary is at most 255 characters; put the rest in the description"));
        }
        if summary != row.summary {
            sqlx::query("UPDATE projects SET summary = $2 WHERE id = $1")
                .bind(id)
                .bind(summary)
                .execute(&mut *tx)
                .await?;
        }
    }
    let content = new_content(&row.content, args.description, args.patch.as_deref())?;
    if let Some(content) = content.filter(|c| *c != row.content) {
        sqlx::query("UPDATE projects SET content = $2 WHERE id = $1").bind(id).bind(&content).execute(&mut *tx).await?;
        let previous = if creating { vec![] } else { events::mentions(ctx, &row.content).await? };
        let mentioned: Vec<Uuid> =
            events::mentions(ctx, &content).await?.into_iter().filter(|u| !previous.contains(u)).collect();
        if !mentioned.is_empty() {
            subscribe(tx, id, &mentioned).await?;
            let notice = Notice {
                kind: "projectMention",
                summary: format!("You were mentioned in the project {}", row.name),
                project: Some(id),
                ..Default::default()
            };
            events::notify(tx, ctx, &mentioned, &notice).await?;
        }
    }
    if let Some(icon) = args.icon {
        let icon = icon.filter(|i| !i.trim().is_empty());
        if icon != row.icon {
            sqlx::query("UPDATE projects SET icon = $2 WHERE id = $1").bind(id).bind(icon).execute(&mut *tx).await?;
        }
    }
    if let Some(color) = args.color {
        let color = color.as_deref().map(check_color).transpose()?;
        if color != row.color {
            sqlx::query("UPDATE projects SET color = $2 WHERE id = $1").bind(id).bind(color).execute(&mut *tx).await?;
        }
    }
    if let Some(priority) = args.priority {
        let priority = check_priority(priority)?;
        if priority != row.priority {
            sqlx::query("UPDATE projects SET priority = $2 WHERE id = $1")
                .bind(id)
                .bind(priority)
                .execute(&mut *tx)
                .await?;
        }
    }
    if let Some(lead) = args.lead {
        let lead = match lead.filter(|l| !is_null_word(l)) {
            Some(person) => Some(resolve::user(ctx, &person).await?),
            None => None,
        };
        if lead != row.lead_id {
            sqlx::query("UPDATE projects SET lead_id = $2 WHERE id = $1").bind(id).bind(lead).execute(&mut *tx).await?;
            if let Some(lead) = lead {
                subscribe(tx, id, &[lead]).await?;
                let name: String =
                    sqlx::query_scalar("SELECT name FROM projects WHERE id = $1").bind(id).fetch_one(&mut *tx).await?;
                let notice = Notice {
                    kind: "projectAssigned",
                    summary: format!("You are now the lead of the project {name}"),
                    project: Some(id),
                    ..Default::default()
                };
                events::notify(tx, ctx, &[lead], &notice).await?;
            }
        }
    }
    teams(ctx, tx, id, &args.set_teams, &args.add_teams, &args.remove_teams).await?;
    if let Some(team) = args.lead_team {
        let team = match team.filter(|t| !is_null_word(t)) {
            Some(team) => Some(resolve::team(ctx, &team).await?),
            None => None,
        };
        let on_project: bool = sqlx::query_scalar(
            "SELECT $2::uuid IS NULL OR EXISTS (SELECT 1 FROM project_teams WHERE project_id = $1 AND team_id = $2)",
        )
        .bind(id)
        .bind(team)
        .fetch_one(&mut *tx)
        .await?;
        if !on_project {
            return Err(ApiError::bad_request(
                "The lead team must be one of the project's teams; add it with addTeams",
            ));
        }
        sqlx::query("UPDATE projects SET lead_team_id = $2 WHERE id = $1")
            .bind(id)
            .bind(team)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(people) = &args.add_members {
        let users = resolve::users(ctx, people).await?;
        sqlx::query(
            "INSERT INTO project_members (project_id, user_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING",
        )
        .bind(id)
        .bind(&users)
        .execute(&mut *tx)
        .await?;
        subscribe(tx, id, &users).await?;
    }
    if let Some(people) = &args.remove_members {
        let users = resolve::users(ctx, people).await?;
        sqlx::query("DELETE FROM project_members WHERE project_id = $1 AND user_id = ANY($2)")
            .bind(id)
            .bind(&users)
            .execute(&mut *tx)
            .await?;
    }
    labels(ctx, tx, id, &args.labels, &args.add_labels, &args.remove_labels).await?;
    dates(tx, id, "start", args.start_date, args.start_date_resolution).await?;
    dates(tx, id, "target", args.target_date, args.target_date_resolution).await?;
    save_links(
        ctx,
        tx,
        "project_id",
        id,
        args.links.as_deref().unwrap_or_default(),
        args.remove_links.as_deref().unwrap_or_default(),
    )
    .await?;
    initiatives(ctx, tx, id, &args.set_initiatives, &args.add_initiatives, &args.remove_initiatives).await?;
    if let Some(order) = args.sort_order {
        sqlx::query("UPDATE projects SET sort_order = $2 WHERE id = $1").bind(id).bind(order).execute(&mut *tx).await?;
    }
    Ok(())
}

async fn dates(
    tx: &mut PgConnection,
    id: Uuid,
    which: &str,
    date: Option<Option<String>>,
    resolution: Option<Option<String>>,
) -> ApiResult<()> {
    if let Some(date) = date {
        let date = optional_date(date)?;
        sqlx::query(&format!(
            "UPDATE projects SET {which}_date = $2,
                {which}_date_resolution = CASE WHEN $2::date IS NULL THEN NULL ELSE {which}_date_resolution END
             WHERE id = $1"
        ))
        .bind(id)
        .bind(date)
        .execute(&mut *tx)
        .await?;
    }
    if let Some(resolution) = resolution {
        let resolution = check_resolution(resolution)?;
        sqlx::query(&format!("UPDATE projects SET {which}_date_resolution = $2 WHERE id = $1"))
            .bind(id)
            .bind(resolution)
            .execute(&mut *tx)
            .await?;
    }
    Ok(())
}

async fn teams(
    ctx: &Ctx,
    tx: &mut PgConnection,
    id: Uuid,
    set: &Option<Vec<String>>,
    add: &Option<Vec<String>>,
    remove: &Option<Vec<String>>,
) -> ApiResult<()> {
    if set.is_some() && (add.is_some() || remove.is_some()) {
        return Err(ApiError::bad_request("Pass either setTeams or addTeams/removeTeams, not both"));
    }
    if set.is_none() && add.is_none() && remove.is_none() {
        return Ok(());
    }
    if let Some(all) = set {
        let ids = resolve_teams(ctx, all).await?;
        sqlx::query("DELETE FROM project_teams WHERE project_id = $1 AND NOT (team_id = ANY($2))")
            .bind(id)
            .bind(&ids)
            .execute(&mut *tx)
            .await?;
        insert_teams(tx, id, &ids).await?;
    }
    if let Some(add) = add {
        let ids = resolve_teams(ctx, add).await?;
        insert_teams(tx, id, &ids).await?;
    }
    if let Some(remove) = remove {
        let ids = resolve_teams(ctx, remove).await?;
        sqlx::query("DELETE FROM project_teams WHERE project_id = $1 AND team_id = ANY($2)")
            .bind(id)
            .bind(&ids)
            .execute(&mut *tx)
            .await?;
    }
    let count: i64 = sqlx::query_scalar("SELECT count(*) FROM project_teams WHERE project_id = $1")
        .bind(id)
        .fetch_one(&mut *tx)
        .await?;
    if count == 0 {
        return Err(ApiError::bad_request("A project keeps at least one team"));
    }
    sqlx::query(
        "UPDATE projects SET lead_team_id = NULL WHERE id = $1
         AND lead_team_id NOT IN (SELECT team_id FROM project_teams WHERE project_id = $1)",
    )
    .bind(id)
    .execute(&mut *tx)
    .await?;
    Ok(())
}

async fn resolve_teams(ctx: &Ctx, values: &[String]) -> ApiResult<Vec<Uuid>> {
    let mut ids = Vec::new();
    for value in values {
        ids.push(resolve::team(ctx, value).await?);
    }
    Ok(ids)
}

async fn insert_teams(tx: &mut PgConnection, id: Uuid, teams: &[Uuid]) -> ApiResult<()> {
    sqlx::query("INSERT INTO project_teams (project_id, team_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING")
        .bind(id)
        .bind(teams)
        .execute(&mut *tx)
        .await?;
    Ok(())
}

async fn labels(
    ctx: &Ctx,
    tx: &mut PgConnection,
    id: Uuid,
    set: &Option<Vec<String>>,
    add: &Option<Vec<String>>,
    remove: &Option<Vec<String>>,
) -> ApiResult<()> {
    if set.is_some() && (add.is_some() || remove.is_some()) {
        return Err(ApiError::bad_request("Pass either labels or addLabels/removeLabels, not both"));
    }
    if let Some(all) = set {
        let ids = resolve::project_labels(ctx, all).await?;
        sqlx::query("DELETE FROM project_label_links WHERE project_id = $1 AND NOT (label_id = ANY($2))")
            .bind(id)
            .bind(&ids)
            .execute(&mut *tx)
            .await?;
        insert_labels(tx, id, &ids).await?;
    }
    if let Some(add) = add {
        let ids = resolve::project_labels(ctx, add).await?;
        insert_labels(tx, id, &ids).await?;
    }
    if let Some(remove) = remove {
        let ids = resolve::project_labels(ctx, remove).await?;
        sqlx::query("DELETE FROM project_label_links WHERE project_id = $1 AND label_id = ANY($2)")
            .bind(id)
            .bind(&ids)
            .execute(&mut *tx)
            .await?;
    }
    Ok(())
}

async fn insert_labels(tx: &mut PgConnection, id: Uuid, labels: &[Uuid]) -> ApiResult<()> {
    sqlx::query(
        "INSERT INTO project_label_links (project_id, label_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING",
    )
    .bind(id)
    .bind(labels)
    .execute(&mut *tx)
    .await?;
    Ok(())
}

/// Adds links to a project or initiative (`column` is project_id or initiative_id); a link with
/// the same URL gets the new title. Then removes the links with the URLs in `remove`.
pub async fn save_links(
    ctx: &Ctx,
    tx: &mut PgConnection,
    column: &str,
    id: Uuid,
    links: &[LinkArg],
    remove: &[String],
) -> ApiResult<()> {
    for link in links {
        let url = non_empty(&link.url, "A link's url")?;
        let label = if link.title.trim().is_empty() { url.clone() } else { link.title.trim().to_string() };
        let updated = sqlx::query(&format!("UPDATE entity_links SET label = $3 WHERE {column} = $1 AND url = $2"))
            .bind(id)
            .bind(&url)
            .bind(&label)
            .execute(&mut *tx)
            .await?;
        if updated.rows_affected() > 0 {
            continue;
        }
        sqlx::query(&format!(
            "INSERT INTO entity_links (workspace_id, {column}, url, label, creator_id, sort_order)
             VALUES ($1, $2, $3, $4, $5, (SELECT coalesce(max(sort_order), 0) + 1 FROM entity_links WHERE {column} = $2))"
        ))
        .bind(ctx.workspace)
        .bind(id)
        .bind(&url)
        .bind(&label)
        .bind(ctx.user)
        .execute(&mut *tx)
        .await?;
    }
    if !remove.is_empty() {
        let urls: Vec<String> = remove.iter().map(|u| u.trim().to_string()).collect();
        sqlx::query(&format!("DELETE FROM entity_links WHERE {column} = $1 AND url = ANY($2)"))
            .bind(id)
            .bind(&urls)
            .execute(&mut *tx)
            .await?;
    }
    Ok(())
}

async fn initiatives(
    ctx: &Ctx,
    tx: &mut PgConnection,
    id: Uuid,
    set: &Option<Vec<String>>,
    add: &Option<Vec<String>>,
    remove: &Option<Vec<String>>,
) -> ApiResult<()> {
    if set.is_some() && (add.is_some() || remove.is_some()) {
        return Err(ApiError::bad_request("Pass either setInitiatives or addInitiatives/removeInitiatives, not both"));
    }
    if (set.is_some() || add.is_some() || remove.is_some()) && ctx.role == crate::ctx::Role::Guest {
        return Err(ApiError::forbidden("Guests can't change initiatives"));
    }
    if let Some(all) = set {
        let ids = resolve::initiatives(ctx, all).await?;
        sqlx::query("DELETE FROM initiative_projects WHERE project_id = $1 AND NOT (initiative_id = ANY($2))")
            .bind(id)
            .bind(&ids)
            .execute(&mut *tx)
            .await?;
        link_initiatives(tx, id, &ids).await?;
    }
    if let Some(add) = add {
        let ids = resolve::initiatives(ctx, add).await?;
        link_initiatives(tx, id, &ids).await?;
    }
    if let Some(remove) = remove {
        let ids = resolve::initiatives(ctx, remove).await?;
        sqlx::query("DELETE FROM initiative_projects WHERE project_id = $1 AND initiative_id = ANY($2)")
            .bind(id)
            .bind(&ids)
            .execute(&mut *tx)
            .await?;
    }
    Ok(())
}

pub async fn link_initiatives(tx: &mut PgConnection, project: Uuid, initiatives: &[Uuid]) -> ApiResult<()> {
    sqlx::query(
        "INSERT INTO initiative_projects (initiative_id, project_id, sort_order)
         SELECT i, $1, (SELECT coalesce(max(sort_order), 0) + 1 FROM initiative_projects WHERE initiative_id = i)
         FROM unnest($2::uuid[]) AS i ON CONFLICT DO NOTHING",
    )
    .bind(project)
    .bind(initiatives)
    .execute(&mut *tx)
    .await?;
    Ok(())
}

async fn relations(ctx: &Ctx, tx: &mut PgConnection, id: Uuid, args: &SaveProjectArgs) -> ApiResult<()> {
    let add =
        [(&args.blocks, "blocks", false), (&args.blocked_by, "blocks", true), (&args.related_to, "related", false)];
    for (list, kind, reverse) in add {
        for other in list.iter().flatten() {
            let other = resolve::project(ctx, other).await?;
            if other == id {
                return Err(ApiError::bad_request("A project can't depend on or relate to itself"));
            }
            let (from, to) = if reverse { (other, id) } else { (id, other) };
            sqlx::query(
                "INSERT INTO project_relations (project_id, related_project_id, type, creator_id)
                 SELECT $1, $2, $3, $4 WHERE NOT EXISTS (SELECT 1 FROM project_relations WHERE type = $3
                    AND ((project_id = $1 AND related_project_id = $2) OR ($3 = 'related' AND project_id = $2 AND related_project_id = $1)))",
            )
            .bind(from)
            .bind(to)
            .bind(kind)
            .bind(ctx.user)
            .execute(&mut *tx)
            .await?;
        }
    }
    let remove = [
        (&args.remove_blocks, "blocks", false),
        (&args.remove_blocked_by, "blocks", true),
        (&args.remove_related_to, "related", false),
    ];
    for (list, kind, reverse) in remove {
        for other in list.iter().flatten() {
            let other = resolve::project(ctx, other).await?;
            let (from, to) = if reverse { (other, id) } else { (id, other) };
            sqlx::query(
                "DELETE FROM project_relations WHERE type = $3 AND ((project_id = $1 AND related_project_id = $2)
                    OR ($3 = 'related' AND project_id = $2 AND related_project_id = $1))",
            )
            .bind(from)
            .bind(to)
            .bind(kind)
            .execute(&mut *tx)
            .await?;
        }
    }
    Ok(())
}

/* ------------------------------------------------------------------ */
/*                          Trash and archive                         */
/* ------------------------------------------------------------------ */

async fn mark(ctx: &Ctx, id: Uuid, sql: &str, action: &str) -> ApiResult<Value> {
    let mut tx = ctx.db().begin().await?;
    sqlx::query(sql).bind(id).execute(&mut *tx).await?;
    let value = project_value(ctx, &mut tx, id).await?;
    let team = webhook_team(&mut tx, id).await?;
    events::emit(&mut tx, ctx, "Project", action, &value, None, team).await?;
    tx.commit().await?;
    Ok(value)
}

async fn trash(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let id = resolve::project(&ctx, &args.id).await?;
    let sql = "UPDATE projects SET trashed_at = now(), updated_at = now() WHERE id = $1 AND trashed_at IS NULL";
    mark(&ctx, id, sql, "remove").await
}

async fn restore(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let id = find_trashed(&ctx, "projects", "project", &args.id).await?;
    let teams = ctx.team_ids().await?.to_vec();
    let visible: bool =
        sqlx::query_scalar(&format!("SELECT {} FROM projects p WHERE p.id = $1", resolve::project_visible("p", 2)))
            .bind(id)
            .bind(&teams)
            .fetch_one(ctx.db())
            .await?;
    if !visible {
        return Err(ApiError::not_found(format!("No project \"{}\" in the trash", args.id.trim())));
    }
    let sql = "UPDATE projects SET trashed_at = NULL, updated_at = now() WHERE id = $1";
    mark(&ctx, id, sql, "update").await
}

async fn archive(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = resolve::project(&ctx, &args.id).await?;
    let sql = "UPDATE projects SET archived_at = now(), updated_at = now() WHERE id = $1 AND archived_at IS NULL";
    mark(&ctx, id, sql, "update").await
}

async fn unarchive(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = resolve::project(&ctx, &args.id).await?;
    let sql = "UPDATE projects SET archived_at = NULL, updated_at = now() WHERE id = $1";
    mark(&ctx, id, sql, "update").await
}

/// Linear-style slugs end in the id: `my-project-0a1b2c3d4e5f`.
fn slug_tail(value: &str) -> &str {
    let value = value.trim();
    let segment =
        value.split(['?', '#']).next().unwrap_or(value).trim_end_matches('/').rsplit('/').next().unwrap_or(value);
    match segment.rsplit_once('-') {
        Some((_, tail)) if tail.len() == 12 && tail.bytes().all(|b| b.is_ascii_hexdigit()) => tail,
        _ => segment,
    }
}

/// A trashed project or initiative (`table`) by id, slug, link or name.
pub async fn find_trashed(ctx: &Ctx, table: &str, what: &str, value: &str) -> ApiResult<Uuid> {
    let found: Option<Uuid> = sqlx::query_scalar(&format!(
        "SELECT id FROM {table} WHERE workspace_id = $1 AND trashed_at IS NOT NULL
         AND (id::text = $2 OR slug_id = $3 OR lower(name) = lower($2)) ORDER BY trashed_at DESC LIMIT 1"
    ))
    .bind(ctx.workspace)
    .bind(value.trim())
    .bind(slug_tail(value))
    .fetch_optional(ctx.db())
    .await?;
    found.ok_or_else(|| ApiError::not_found(format!("No {what} \"{}\" in the trash", value.trim())))
}

/* ------------------------------------------------------------------ */
/*                              Statuses                              */
/* ------------------------------------------------------------------ */

const STATUS_JSON: &str = "jsonb_build_object('id', s.id, 'name', s.name, 'type', s.type, 'color', s.color,
    'description', s.description, 'position', s.position,
    'projectCount', (SELECT count(*) FROM projects sp WHERE sp.status_id = s.id AND sp.trashed_at IS NULL),
    'archivedAt', s.archived_at, 'createdAt', s.created_at, 'updatedAt', s.updated_at)";

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ListStatusesArgs {
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list_statuses(ctx: Ctx, args: ListStatusesArgs) -> ApiResult<Value> {
    let page = Page { limit: args.limit, cursor: args.cursor };
    let nodes: Vec<Value> = sqlx::query_scalar(&format!(
        "SELECT {STATUS_JSON} FROM project_statuses s WHERE s.workspace_id = $1 AND s.archived_at IS NULL
         ORDER BY s.position, s.created_at LIMIT $2 OFFSET $3"
    ))
    .bind(ctx.workspace)
    .bind(page.limit() + 1)
    .bind(page.offset()?)
    .fetch_all(ctx.db())
    .await?;
    page.wrap(nodes)
}

async fn status_value(db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    Ok(sqlx::query_scalar(&format!("SELECT {STATUS_JSON} FROM project_statuses s WHERE s.id = $1"))
        .bind(id)
        .fetch_one(&mut *db)
        .await?)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SaveStatusArgs {
    /// Only to change a status: its id or name. Omit to create one.
    id: Option<String>,
    name: Option<String>,
    /// backlog, planned, started, paused, completed or canceled. Required to create.
    #[serde(rename = "type")]
    kind: Option<String>,
    /// Hex color like #f2c94c.
    color: Option<String>,
    /// null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    description: Option<Option<String>>,
    /// Order among the statuses (lower comes first). New statuses go last.
    position: Option<f64>,
}

fn check_status_type(value: &str) -> ApiResult<String> {
    let lower = value.trim().to_lowercase();
    if !STATUS_TYPES.contains(&lower.as_str()) {
        return Err(ApiError::bad_request(format!(
            "A project status type is backlog, planned, started, paused, completed or canceled, not \"{value}\""
        )));
    }
    Ok(lower)
}

async fn save_status(ctx: Ctx, args: SaveStatusArgs) -> ApiResult<Value> {
    ctx.require_admin()?;
    let mut tx = ctx.db().begin().await?;
    let id = match &args.id {
        Some(reference) => resolve::project_status(&ctx, reference).await?,
        None => {
            let name = non_empty(args.name.as_deref().unwrap_or(""), "name")?;
            let kind =
                check_status_type(args.kind.as_deref().ok_or_else(|| ApiError::bad_request("type is required"))?)?;
            let color = match &args.color {
                Some(color) => check_color(color)?,
                None => pick_color(&name),
            };
            sqlx::query_scalar(
                "INSERT INTO project_statuses (workspace_id, name, type, color, position)
                 VALUES ($1, $2, $3, $4, (SELECT coalesce(max(position), 0) + 1 FROM project_statuses WHERE workspace_id = $1))
                 RETURNING id",
            )
            .bind(ctx.workspace)
            .bind(&name)
            .bind(&kind)
            .bind(&color)
            .fetch_one(&mut *tx)
            .await?
        }
    };
    if let Some(name) = &args.name {
        let name = non_empty(name, "name")?;
        let taken: bool = sqlx::query_scalar(
            "SELECT EXISTS (SELECT 1 FROM project_statuses WHERE workspace_id = $1 AND id <> $2
                AND lower(name) = lower($3) AND archived_at IS NULL)",
        )
        .bind(ctx.workspace)
        .bind(id)
        .bind(&name)
        .fetch_one(&mut *tx)
        .await?;
        if taken {
            return Err(ApiError::conflict(format!("There is already a project status \"{name}\"")));
        }
        sqlx::query("UPDATE project_statuses SET name = $2 WHERE id = $1")
            .bind(id)
            .bind(&name)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(kind) = &args.kind {
        sqlx::query("UPDATE project_statuses SET type = $2 WHERE id = $1")
            .bind(id)
            .bind(check_status_type(kind)?)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(color) = &args.color {
        sqlx::query("UPDATE project_statuses SET color = $2 WHERE id = $1")
            .bind(id)
            .bind(check_color(color)?)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(description) = &args.description {
        sqlx::query("UPDATE project_statuses SET description = $2 WHERE id = $1")
            .bind(id)
            .bind(description)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(position) = args.position {
        sqlx::query("UPDATE project_statuses SET position = $2 WHERE id = $1")
            .bind(id)
            .bind(position)
            .execute(&mut *tx)
            .await?;
    }
    sqlx::query("UPDATE project_statuses SET updated_at = now() WHERE id = $1").bind(id).execute(&mut *tx).await?;
    let value = status_value(&mut tx, id).await?;
    tx.commit().await?;
    Ok(value)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DeleteStatusArgs {
    /// The status's id or name.
    id: String,
    /// The status (name, type or id) its projects move to; required when projects use it.
    move_projects_to: Option<String>,
}

async fn delete_status(ctx: Ctx, args: DeleteStatusArgs) -> ApiResult<Value> {
    ctx.require_admin()?;
    let id = resolve::project_status(&ctx, &args.id).await?;
    let mut tx = ctx.db().begin().await?;
    let remaining: i64 = sqlx::query_scalar(
        "SELECT count(*) FROM project_statuses WHERE workspace_id = $1 AND id <> $2 AND archived_at IS NULL",
    )
    .bind(ctx.workspace)
    .bind(id)
    .fetch_one(&mut *tx)
    .await?;
    if remaining == 0 {
        return Err(ApiError::bad_request("The workspace keeps at least one project status"));
    }
    let projects: Vec<Uuid> =
        sqlx::query_scalar("SELECT id FROM projects WHERE status_id = $1").bind(id).fetch_all(&mut *tx).await?;
    if !projects.is_empty() {
        let Some(target) = &args.move_projects_to else {
            return Err(ApiError::bad_request(format!(
                "{} projects use this status; pass moveProjectsTo",
                projects.len()
            )));
        };
        let target = resolve::project_status(&ctx, target).await?;
        if target == id {
            return Err(ApiError::bad_request("moveProjectsTo must be another status"));
        }
        sqlx::query("UPDATE projects SET status_id = $2, updated_at = now() WHERE status_id = $1")
            .bind(id)
            .bind(target)
            .execute(&mut *tx)
            .await?;
        for project in &projects {
            stamp_status(&mut tx, *project).await?;
        }
    }
    sqlx::query("DELETE FROM project_statuses WHERE id = $1").bind(id).execute(&mut *tx).await?;
    tx.commit().await?;
    Ok(json!({ "deleted": true, "id": id, "movedProjects": projects.len() }))
}

/* ------------------------------------------------------------------ */
/*                               Labels                               */
/* ------------------------------------------------------------------ */

const LABEL_JSON: &str = "jsonb_build_object('id', l.id, 'name', l.name, 'color', l.color,
    'description', l.description, 'isGroup', l.is_group,
    'parent', (SELECT jsonb_build_object('id', g.id, 'name', g.name) FROM project_labels g WHERE g.id = l.parent_id),
    'projectCount', (SELECT count(*) FROM project_label_links pl JOIN projects lp ON lp.id = pl.project_id
        WHERE pl.label_id = l.id AND lp.trashed_at IS NULL),
    'creator', user_ref(l.creator_id), 'retiredAt', l.retired_at, 'createdAt', l.created_at, 'updatedAt', l.updated_at)";

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ListLabelsArgs {
    #[serde(default)]
    include_retired: bool,
    /// Part of the name.
    query: Option<String>,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list_labels(ctx: Ctx, args: ListLabelsArgs) -> ApiResult<Value> {
    let page = Page { limit: args.limit, cursor: args.cursor };
    let like = args.query.as_deref().map(|q| format!("%{}%", escape_like(q.trim())));
    let nodes: Vec<Value> = sqlx::query_scalar(&format!(
        "SELECT {LABEL_JSON} FROM project_labels l WHERE l.workspace_id = $1 AND ($2 OR l.retired_at IS NULL)
         AND ($3::text IS NULL OR l.name ILIKE $3)
         ORDER BY coalesce((SELECT g.name FROM project_labels g WHERE g.id = l.parent_id), l.name), l.parent_id IS NOT NULL, l.name
         LIMIT $4 OFFSET $5"
    ))
    .bind(ctx.workspace)
    .bind(args.include_retired)
    .bind(like)
    .bind(page.limit() + 1)
    .bind(page.offset()?)
    .fetch_all(ctx.db())
    .await?;
    page.wrap(nodes)
}

/// A project label or group by id, name or Group/Name, retired ones included.
async fn label_id(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    let rows: Vec<(Uuid, String, Option<String>)> = sqlx::query_as(
        "SELECT l.id, l.name, g.name FROM project_labels l LEFT JOIN project_labels g ON g.id = l.parent_id
         WHERE l.workspace_id = $1 ORDER BY l.retired_at IS NOT NULL, l.name",
    )
    .bind(ctx.workspace)
    .fetch_all(ctx.db())
    .await?;
    let value = value.trim();
    if let Some(row) = rows.iter().find(|r| Some(r.0) == as_uuid(value)) {
        return Ok(row.0);
    }
    let (group, name) = value.split_once('/').map_or((None, value), |(g, n)| (Some(g.trim()), n.trim()));
    let matches: Vec<_> = rows
        .iter()
        .filter(|r| {
            r.1.eq_ignore_ascii_case(name)
                && group.is_none_or(|g| r.2.as_deref().is_some_and(|p| p.eq_ignore_ascii_case(g)))
        })
        .collect();
    match matches.as_slice() {
        [one, ..] if matches.len() == 1 || group.is_some() => Ok(one.0),
        [] => {
            let names: Vec<String> =
                rows.iter().map(|r| r.2.as_ref().map_or(r.1.clone(), |g| format!("{g}/{}", r.1))).take(30).collect();
            Err(ApiError::bad_request(format!("No project label \"{value}\". Options: {}.", names.join(", "))))
        }
        _ => {
            Err(ApiError::bad_request(format!("\"{value}\" matches labels in several groups; name it as Group/{name}")))
        }
    }
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SaveLabelArgs {
    /// Only to change a label: its id, name or Group/Name. Omit to create one.
    id: Option<String>,
    name: Option<String>,
    /// Hex color like #bb87fc.
    color: Option<String>,
    /// null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    description: Option<Option<String>>,
    /// Whether this is a group holding other labels (projects can't have a group itself).
    is_group: Option<bool>,
    /// The group (name or id) this label sits in; null takes it out.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    parent: Option<Option<String>>,
}

async fn label_value(db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    Ok(sqlx::query_scalar(&format!("SELECT {LABEL_JSON} FROM project_labels l WHERE l.id = $1"))
        .bind(id)
        .fetch_one(&mut *db)
        .await?)
}

async fn save_label(ctx: Ctx, args: SaveLabelArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let existing = match &args.id {
        Some(reference) => Some(label_id(&ctx, reference).await?),
        None => None,
    };
    let mut tx = ctx.db().begin().await?;
    let id = match existing {
        Some(id) => id,
        None => {
            let name = non_empty(args.name.as_deref().unwrap_or(""), "name")?;
            let color = match &args.color {
                Some(color) => check_color(color)?,
                None => pick_color(&name),
            };
            sqlx::query_scalar(
                "INSERT INTO project_labels (workspace_id, name, color, creator_id) VALUES ($1, $2, $3, $4) RETURNING id",
            )
            .bind(ctx.workspace)
            .bind(&name)
            .bind(&color)
            .bind(ctx.user)
            .fetch_one(&mut *tx)
            .await?
        }
    };
    if let Some(name) = &args.name {
        sqlx::query("UPDATE project_labels SET name = $2 WHERE id = $1")
            .bind(id)
            .bind(non_empty(name, "name")?)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(color) = &args.color {
        sqlx::query("UPDATE project_labels SET color = $2 WHERE id = $1")
            .bind(id)
            .bind(check_color(color)?)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(description) = &args.description {
        sqlx::query("UPDATE project_labels SET description = $2 WHERE id = $1")
            .bind(id)
            .bind(description)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(is_group) = args.is_group {
        sqlx::query("UPDATE project_labels SET is_group = $2 WHERE id = $1")
            .bind(id)
            .bind(is_group)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(parent) = &args.parent {
        let parent = match parent.as_deref().filter(|p| !is_null_word(p)) {
            Some(parent) => Some(label_id(&ctx, parent).await?),
            None => None,
        };
        sqlx::query("UPDATE project_labels SET parent_id = $2 WHERE id = $1")
            .bind(id)
            .bind(parent)
            .execute(&mut *tx)
            .await?;
    }
    let (is_group, parent_is_group, has_children, in_use): (bool, Option<bool>, bool, bool) = sqlx::query_as(
        "SELECT l.is_group, g.is_group,
            EXISTS (SELECT 1 FROM project_labels c WHERE c.parent_id = l.id),
            EXISTS (SELECT 1 FROM project_label_links pl WHERE pl.label_id = l.id)
         FROM project_labels l LEFT JOIN project_labels g ON g.id = l.parent_id WHERE l.id = $1",
    )
    .bind(id)
    .fetch_one(&mut *tx)
    .await?;
    if parent_is_group == Some(false) {
        return Err(ApiError::bad_request("A label's parent must be a group (a label with isGroup)"));
    }
    if is_group && parent_is_group.is_some() {
        return Err(ApiError::bad_request("Groups don't nest; a group can't have a parent"));
    }
    if is_group && in_use {
        return Err(ApiError::bad_request("Projects use this label, so it can't become a group"));
    }
    if !is_group && has_children {
        return Err(ApiError::bad_request("This group has labels in it, so it must stay a group"));
    }
    let clash: bool = sqlx::query_scalar(
        "SELECT EXISTS (SELECT 1 FROM project_labels o JOIN project_labels l ON l.id = $1
            WHERE o.workspace_id = l.workspace_id AND o.id <> l.id AND lower(o.name) = lower(l.name)
            AND o.parent_id IS NOT DISTINCT FROM l.parent_id AND o.retired_at IS NULL)",
    )
    .bind(id)
    .fetch_one(&mut *tx)
    .await?;
    if clash {
        return Err(ApiError::conflict("There is already a project label with that name"));
    }
    sqlx::query("UPDATE project_labels SET updated_at = now() WHERE id = $1").bind(id).execute(&mut *tx).await?;
    let value = label_value(&mut tx, id).await?;
    let action = if existing.is_some() { "update" } else { "create" };
    events::emit(&mut tx, &ctx, "ProjectLabel", action, &value, None, None).await?;
    tx.commit().await?;
    Ok(value)
}

async fn mark_label(ctx: &Ctx, reference: &str, sql: &str) -> ApiResult<Value> {
    ctx.require_member()?;
    let id = label_id(ctx, reference).await?;
    let mut tx = ctx.db().begin().await?;
    sqlx::query(sql).bind(id).execute(&mut *tx).await?;
    let value = label_value(&mut tx, id).await?;
    events::emit(&mut tx, ctx, "ProjectLabel", "update", &value, None, None).await?;
    tx.commit().await?;
    Ok(value)
}

async fn retire_label(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let sql = "UPDATE project_labels SET retired_at = now(), updated_at = now()
               WHERE (id = $1 OR parent_id = $1) AND retired_at IS NULL";
    mark_label(&ctx, &args.id, sql).await
}

async fn restore_label(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let sql = "UPDATE project_labels SET retired_at = NULL, updated_at = now() WHERE id = $1 OR parent_id = $1";
    mark_label(&ctx, &args.id, sql).await
}

async fn delete_label(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let id = label_id(&ctx, &args.id).await?;
    let mut tx = ctx.db().begin().await?;
    let value = label_value(&mut tx, id).await?;
    sqlx::query("DELETE FROM project_labels WHERE id = $1").bind(id).execute(&mut *tx).await?;
    events::emit(&mut tx, &ctx, "ProjectLabel", "remove", &value, None, None).await?;
    tx.commit().await?;
    Ok(json!({ "deleted": true, "id": id }))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn slug_tails() {
        assert_eq!(slug_tail("http://x.dev/acme/project/launch-0a1b2c3d4e5f"), "0a1b2c3d4e5f");
        assert_eq!(slug_tail("Launch"), "Launch");
    }
}
