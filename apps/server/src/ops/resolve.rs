//! Turns what agents pass (ids, names, keys, identifiers like ENG-123, "me", links) into ids,
//! checking the caller may see the thing. Errors list what would have matched.

use sqlx::PgPool;
use uuid::Uuid;

use crate::ctx::Ctx;
use crate::error::{ApiError, ApiResult};

pub fn as_uuid(value: &str) -> Option<Uuid> {
    Uuid::parse_str(value.trim()).ok()
}

/// The last path segment of a link, or the value itself.
fn last_segment(value: &str) -> &str {
    let value = value.trim();
    if !value.contains("://") {
        return value;
    }
    let path = value.split(['?', '#']).next().unwrap_or(value).trim_end_matches('/');
    path.rsplit('/').next().unwrap_or(value)
}

/// Linear-style slugs end in the id: `my-project-0a1b2c3d4e5f`.
fn slug_tail(value: &str) -> &str {
    let segment = last_segment(value);
    match segment.rsplit_once('-') {
        Some((_, tail)) if tail.len() == 12 && tail.bytes().all(|b| b.is_ascii_hexdigit()) => tail,
        _ => segment,
    }
}

fn options(names: Vec<String>) -> String {
    if names.is_empty() {
        return "There are none.".into();
    }
    let shown: Vec<_> = names.iter().take(30).cloned().collect();
    let more =
        if names.len() > shown.len() { format!(" and {} more", names.len() - shown.len()) } else { String::new() };
    format!("Options: {}{more}.", shown.join(", "))
}

pub async fn user(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    let value = value.trim();
    if value.eq_ignore_ascii_case("me") {
        return Ok(ctx.user);
    }
    let rows: Vec<(Uuid, String, String, String)> = sqlx::query_as(
        "SELECT u.id, u.name, u.display_name, u.email FROM users u
         JOIN workspace_members m ON m.user_id = u.id AND m.workspace_id = $1 AND m.active",
    )
    .bind(ctx.workspace)
    .fetch_all(ctx.db())
    .await?;
    let needle = value.trim_start_matches('@').to_lowercase();
    let by =
        |pred: &dyn Fn(&(Uuid, String, String, String)) -> bool| rows.iter().filter(|r| pred(r)).collect::<Vec<_>>();
    let candidates = [
        by(&|r| Some(r.0) == as_uuid(value)),
        by(&|r| r.3.to_lowercase() == needle),
        by(&|r| r.2.to_lowercase() == needle),
        by(&|r| r.1.to_lowercase() == needle),
        by(&|r| r.3.split('@').next().is_some_and(|local| local.to_lowercase() == needle)),
        by(&|r| r.1.to_lowercase().split_whitespace().next() == Some(needle.as_str())),
    ];
    for matches in candidates {
        match matches.as_slice() {
            [one] => return Ok(one.0),
            [] => continue,
            many => {
                let names = many.iter().map(|r| format!("{} <{}>", r.1, r.3)).collect();
                return Err(ApiError::bad_request(format!(
                    "\"{value}\" matches more than one person; use their email. {}",
                    options(names)
                )));
            }
        }
    }
    let names = rows.iter().map(|r| format!("{} <{}>", r.1, r.3)).collect();
    Err(ApiError::bad_request(format!("No member of this workspace matches \"{value}\". {}", options(names))))
}

pub async fn users(ctx: &Ctx, values: &[String]) -> ApiResult<Vec<Uuid>> {
    let mut ids = Vec::new();
    for value in values {
        let id = user(ctx, value).await?;
        if !ids.contains(&id) {
            ids.push(id);
        }
    }
    Ok(ids)
}

/// A team the caller can see, by id, key or name.
pub async fn team(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    let visible = ctx.team_ids().await?.to_vec();
    let rows: Vec<(Uuid, String, String)> =
        sqlx::query_as("SELECT id, key, name FROM teams WHERE id = ANY($1) AND archived_at IS NULL ORDER BY name")
            .bind(&visible)
            .fetch_all(ctx.db())
            .await?;
    let value = value.trim();
    let found = rows.iter().find(|r| Some(r.0) == as_uuid(value)).or_else(|| {
        rows.iter()
            .find(|r| r.1.eq_ignore_ascii_case(value))
            .or_else(|| rows.iter().find(|r| r.2.eq_ignore_ascii_case(value)))
    });
    if let Some(row) = found {
        return Ok(row.0);
    }
    let names = rows.iter().map(|r| format!("{} ({})", r.1, r.2)).collect();
    Err(ApiError::bad_request(format!("No team matches \"{value}\". {}", options(names))))
}

/// A workflow state of the team, by id, name or type (the first of that type).
pub async fn state(ctx: &Ctx, team: Uuid, value: &str) -> ApiResult<Uuid> {
    let rows: Vec<(Uuid, String, String)> = sqlx::query_as(
        "SELECT id, name, type FROM workflow_states WHERE team_id = $1 AND archived_at IS NULL ORDER BY position",
    )
    .bind(team)
    .fetch_all(ctx.db())
    .await?;
    let value = value.trim();
    let found = rows
        .iter()
        .find(|r| Some(r.0) == as_uuid(value))
        .or_else(|| rows.iter().find(|r| r.1.eq_ignore_ascii_case(value)))
        .or_else(|| rows.iter().find(|r| r.2.eq_ignore_ascii_case(value)))
        .or_else(|| rows.iter().find(|r| value.eq_ignore_ascii_case("todo") && r.2 == "unstarted"))
        .or_else(|| rows.iter().find(|r| value.eq_ignore_ascii_case("done") && r.2 == "completed"));
    if let Some(row) = found {
        return Ok(row.0);
    }
    let names = rows.iter().map(|r| format!("{} ({})", r.1, r.2)).collect();
    Err(ApiError::bad_request(format!("The team has no status \"{value}\". {}", options(names))))
}

/// Splits `ENG-123` into the team key and number.
pub fn parse_identifier(value: &str) -> Option<(String, i32)> {
    let (key, number) = value.trim().rsplit_once('-')?;
    let number = number.parse().ok()?;
    let valid_key = !key.is_empty() && key.len() <= 10 && key.bytes().all(|b| b.is_ascii_alphanumeric() || b == b'_');
    valid_key.then(|| (key.to_uppercase(), number))
}

/// An issue the caller can see, by id, identifier (current or earlier) or link. Trashed issues
/// are found too.
pub async fn issue(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    let raw = value.trim();
    let candidate = if raw.contains("://") {
        raw.split('/').find(|s| parse_identifier(s).is_some()).unwrap_or_else(|| last_segment(raw))
    } else {
        raw
    };
    let id: Option<Uuid> = if let Some(id) = as_uuid(candidate) {
        sqlx::query_scalar("SELECT id FROM issues WHERE id = $1 AND workspace_id = $2")
            .bind(id)
            .bind(ctx.workspace)
            .fetch_optional(ctx.db())
            .await?
    } else if let Some((key, number)) = parse_identifier(candidate) {
        let current: Option<Uuid> = sqlx::query_scalar(
            "SELECT i.id FROM issues i JOIN teams t ON t.id = i.team_id
             WHERE i.workspace_id = $1 AND upper(t.key) = $2 AND i.number = $3",
        )
        .bind(ctx.workspace)
        .bind(&key)
        .bind(number)
        .fetch_optional(ctx.db())
        .await?;
        match current {
            Some(id) => Some(id),
            None => {
                sqlx::query_scalar("SELECT id FROM issues WHERE workspace_id = $1 AND $2 = ANY(previous_identifiers)")
                    .bind(ctx.workspace)
                    .bind(format!("{key}-{number}"))
                    .fetch_optional(ctx.db())
                    .await?
            }
        }
    } else {
        None
    };
    let not_found = || ApiError::not_found(format!("No issue \"{raw}\"; use an identifier like ENG-123 or an id"));
    let id = id.ok_or_else(not_found)?;
    if !can_see_issue(ctx, id).await? {
        return Err(not_found());
    }
    Ok(id)
}

pub async fn can_see_issue(ctx: &Ctx, issue: Uuid) -> ApiResult<bool> {
    let teams = ctx.team_ids().await?.to_vec();
    let visible: bool = sqlx::query_scalar(
        "SELECT EXISTS (SELECT 1 FROM issues i WHERE i.id = $1 AND (i.team_id = ANY($2)
            OR EXISTS (SELECT 1 FROM issue_shares s WHERE s.issue_id = i.id AND s.user_id = $3)))",
    )
    .bind(issue)
    .bind(&teams)
    .bind(ctx.user)
    .fetch_one(ctx.db())
    .await?;
    Ok(visible)
}

/// Issue labels by id or name, among the workspace's labels and the team's. A child of a group
/// can be named `Group/Child`.
pub async fn labels(ctx: &Ctx, team: Option<Uuid>, values: &[String]) -> ApiResult<Vec<Uuid>> {
    if values.is_empty() {
        return Ok(vec![]);
    }
    let rows: Vec<(Uuid, String, Option<String>, Option<Uuid>, bool)> = sqlx::query_as(
        "SELECT l.id, l.name, p.name, l.team_id, l.is_group FROM issue_labels l
         LEFT JOIN issue_labels p ON p.id = l.parent_id
         WHERE l.workspace_id = $1 AND l.retired_at IS NULL AND (l.team_id IS NULL OR l.team_id = $2)",
    )
    .bind(ctx.workspace)
    .bind(team)
    .fetch_all(ctx.db())
    .await?;
    let mut ids = Vec::new();
    for value in values {
        let value = value.trim();
        let (group, name) = match value.split_once('/') {
            Some((g, n)) => (Some(g.trim()), n.trim()),
            None => (None, value),
        };
        let matches: Vec<_> = rows
            .iter()
            .filter(|r| !r.4)
            .filter(|r| {
                Some(r.0) == as_uuid(value)
                    || (r.1.eq_ignore_ascii_case(name)
                        && group.is_none_or(|g| r.2.as_deref().is_some_and(|p| p.eq_ignore_ascii_case(g))))
            })
            .collect();
        // A team label wins over a workspace label of the same name.
        let chosen = match matches.as_slice() {
            [one] => one,
            [] => {
                let names = rows
                    .iter()
                    .filter(|r| !r.4)
                    .map(|r| match &r.2 {
                        Some(p) => format!("{p}/{}", r.1),
                        None => r.1.clone(),
                    })
                    .collect();
                return Err(ApiError::bad_request(format!("No label \"{value}\". {}", options(names))));
            }
            many => match many.iter().find(|r| r.3.is_some()) {
                Some(team_label) if many.iter().filter(|r| r.3.is_some()).count() == 1 => team_label,
                _ => {
                    return Err(ApiError::bad_request(format!(
                        "\"{value}\" matches labels in several groups; name it as Group/{name}"
                    )));
                }
            },
        };
        if !ids.contains(&chosen.0) {
            ids.push(chosen.0);
        }
    }
    Ok(ids)
}

/// SQL that is true when project `p` is visible: it belongs to a team the caller sees. Bind the
/// visible team ids as `$n`.
pub fn project_visible(alias: &str, teams_param: usize) -> String {
    format!(
        "EXISTS (SELECT 1 FROM project_teams pt WHERE pt.project_id = {alias}.id AND pt.team_id = ANY(${teams_param}))"
    )
}

/// A project by id, slug, link or name.
pub async fn project(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    let teams = ctx.team_ids().await?.to_vec();
    let value = value.trim();
    let slug = slug_tail(value);
    let sql = format!(
        "SELECT p.id, p.name FROM projects p WHERE p.workspace_id = $1 AND {} AND p.trashed_at IS NULL
         ORDER BY p.archived_at IS NOT NULL, p.updated_at DESC",
        project_visible("p", 2)
    );
    let rows: Vec<(Uuid, String)> = sqlx::query_as(&sql).bind(ctx.workspace).bind(&teams).fetch_all(ctx.db()).await?;
    if let Some(id) = as_uuid(value).filter(|id| rows.iter().any(|r| r.0 == *id)) {
        return Ok(id);
    }
    let by_slug: Option<Uuid> = sqlx::query_scalar("SELECT id FROM projects WHERE workspace_id = $1 AND slug_id = $2")
        .bind(ctx.workspace)
        .bind(slug)
        .fetch_optional(ctx.db())
        .await?;
    if let Some(id) = by_slug.filter(|id| rows.iter().any(|r| r.0 == *id)) {
        return Ok(id);
    }
    let named: Vec<_> = rows.iter().filter(|r| r.1.eq_ignore_ascii_case(value)).collect();
    if let Some(first) = named.first() {
        return Ok(first.0);
    }
    let names = rows.iter().map(|r| r.1.clone()).collect();
    Err(ApiError::bad_request(format!("No project \"{value}\". {}", options(names))))
}

pub async fn projects(ctx: &Ctx, values: &[String]) -> ApiResult<Vec<Uuid>> {
    let mut ids = Vec::new();
    for value in values {
        let id = project(ctx, value).await?;
        if !ids.contains(&id) {
            ids.push(id);
        }
    }
    Ok(ids)
}

/// A milestone by id, or by name within the project.
pub async fn milestone(ctx: &Ctx, project: Option<Uuid>, value: &str) -> ApiResult<Uuid> {
    let value = value.trim();
    if let Some(id) = as_uuid(value) {
        let project_of: Option<Uuid> = sqlx::query_scalar(
            "SELECT m.project_id FROM project_milestones m JOIN projects p ON p.id = m.project_id
             WHERE m.id = $1 AND p.workspace_id = $2",
        )
        .bind(id)
        .bind(ctx.workspace)
        .fetch_optional(ctx.db())
        .await?;
        if let Some(project_of) = project_of {
            self::project(ctx, &project_of.to_string()).await?;
            return Ok(id);
        }
    }
    let Some(project) = project else {
        return Err(ApiError::bad_request("Name the project too, or pass the milestone's id"));
    };
    let rows: Vec<(Uuid, String)> =
        sqlx::query_as("SELECT id, name FROM project_milestones WHERE project_id = $1 ORDER BY sort_order")
            .bind(project)
            .fetch_all(ctx.db())
            .await?;
    if let Some(row) = rows.iter().find(|r| r.1.eq_ignore_ascii_case(value)) {
        return Ok(row.0);
    }
    let names = rows.iter().map(|r| r.1.clone()).collect();
    Err(ApiError::bad_request(format!("The project has no milestone \"{value}\". {}", options(names))))
}

/// A cycle of the team by id, number, name, or "current", "next" or "previous".
pub async fn cycle(ctx: &Ctx, team: Option<Uuid>, value: &str) -> ApiResult<Uuid> {
    let value = value.trim();
    if let Some(id) = as_uuid(value) {
        let team_of: Option<Uuid> =
            sqlx::query_scalar("SELECT team_id FROM cycles WHERE id = $1").bind(id).fetch_optional(ctx.db()).await?;
        if let Some(team_of) = team_of
            && ctx.can_see_team(team_of).await?
        {
            return Ok(id);
        }
        return Err(ApiError::not_found(format!("No cycle {value}")));
    }
    let Some(team) = team else {
        return Err(ApiError::bad_request("Name the team too, or pass the cycle's id"));
    };
    let relative = match value.to_lowercase().as_str() {
        "current" | "active" => Some("starts_at <= now() AND ends_at > now() ORDER BY starts_at DESC"),
        "next" | "upcoming" => Some("starts_at > now() ORDER BY starts_at"),
        "previous" | "last" => Some("ends_at <= now() ORDER BY ends_at DESC"),
        _ => None,
    };
    let found: Option<Uuid> = if let Some(clause) = relative {
        sqlx::query_scalar(&format!(
            "SELECT id FROM cycles WHERE team_id = $1 AND archived_at IS NULL AND {clause} LIMIT 1"
        ))
        .bind(team)
        .fetch_optional(ctx.db())
        .await?
    } else if let Ok(number) = value.trim_start_matches(['#', 'C', 'c']).trim().parse::<i32>() {
        sqlx::query_scalar("SELECT id FROM cycles WHERE team_id = $1 AND number = $2")
            .bind(team)
            .bind(number)
            .fetch_optional(ctx.db())
            .await?
    } else {
        sqlx::query_scalar("SELECT id FROM cycles WHERE team_id = $1 AND lower(name) = lower($2) LIMIT 1")
            .bind(team)
            .bind(value)
            .fetch_optional(ctx.db())
            .await?
    };
    found.ok_or_else(|| {
        ApiError::bad_request(format!(
            "The team has no cycle \"{value}\"; use its number, name, id, or current/next/previous"
        ))
    })
}

/// Something with a workspace, a slug and a name or title, by id, slug, link or name.
async fn named(
    db: &PgPool,
    workspace: Uuid,
    table: &str,
    name_column: &str,
    extra: &str,
    what: &str,
    value: &str,
) -> ApiResult<Uuid> {
    let value = value.trim();
    let sql = format!(
        "SELECT id, {name_column}, slug_id FROM {table} WHERE workspace_id = $1 {extra} ORDER BY updated_at DESC"
    );
    let rows: Vec<(Uuid, String, String)> = sqlx::query_as(&sql).bind(workspace).fetch_all(db).await?;
    let slug = slug_tail(value);
    let found = rows
        .iter()
        .find(|r| Some(r.0) == as_uuid(value))
        .or_else(|| rows.iter().find(|r| r.2 == slug))
        .or_else(|| rows.iter().find(|r| r.1.eq_ignore_ascii_case(value)));
    if let Some(row) = found {
        return Ok(row.0);
    }
    let names = rows.iter().map(|r| r.1.clone()).collect();
    Err(ApiError::bad_request(format!("No {what} \"{value}\". {}", options(names))))
}

pub async fn initiative(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    named(ctx.db(), ctx.workspace, "initiatives", "name", "AND trashed_at IS NULL", "initiative", value).await
}

pub async fn initiatives(ctx: &Ctx, values: &[String]) -> ApiResult<Vec<Uuid>> {
    let mut ids = Vec::new();
    for value in values {
        let id = initiative(ctx, value).await?;
        if !ids.contains(&id) {
            ids.push(id);
        }
    }
    Ok(ids)
}

/// A document by id, slug, link or title. Documents of projects, teams and issues the caller
/// can't see are left out.
pub async fn document(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    let id = named(ctx.db(), ctx.workspace, "documents", "title", "AND trashed_at IS NULL", "document", value).await?;
    if !can_see_document(ctx, id).await? {
        return Err(ApiError::not_found(format!("No document \"{value}\"")));
    }
    Ok(id)
}

pub async fn can_see_document(ctx: &Ctx, id: Uuid) -> ApiResult<bool> {
    let row: Option<(Option<Uuid>, Option<Uuid>, Option<Uuid>, Option<Uuid>)> =
        sqlx::query_as("SELECT team_id, project_id, issue_id, cycle_id FROM documents WHERE id = $1")
            .bind(id)
            .fetch_optional(ctx.db())
            .await?;
    let Some((team, project, issue, cycle)) = row else { return Ok(false) };
    if let Some(team) = team {
        return ctx.can_see_team(team).await;
    }
    if let Some(project) = project {
        return Ok(self::project(ctx, &project.to_string()).await.is_ok());
    }
    if let Some(issue) = issue {
        return can_see_issue(ctx, issue).await;
    }
    if let Some(cycle) = cycle {
        return Ok(self::cycle(ctx, None, &cycle.to_string()).await.is_ok());
    }
    Ok(true)
}

pub async fn project_status(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    let rows: Vec<(Uuid, String, String)> = sqlx::query_as(
        "SELECT id, name, type FROM project_statuses WHERE workspace_id = $1 AND archived_at IS NULL ORDER BY position",
    )
    .bind(ctx.workspace)
    .fetch_all(ctx.db())
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
    let names = rows.iter().map(|r| format!("{} ({})", r.1, r.2)).collect();
    Err(ApiError::bad_request(format!("No project status \"{value}\". {}", options(names))))
}

pub async fn project_labels(ctx: &Ctx, values: &[String]) -> ApiResult<Vec<Uuid>> {
    if values.is_empty() {
        return Ok(vec![]);
    }
    let rows: Vec<(Uuid, String, Option<String>)> = sqlx::query_as(
        "SELECT l.id, l.name, p.name FROM project_labels l LEFT JOIN project_labels p ON p.id = l.parent_id
         WHERE l.workspace_id = $1 AND l.retired_at IS NULL AND NOT l.is_group",
    )
    .bind(ctx.workspace)
    .fetch_all(ctx.db())
    .await?;
    let mut ids = Vec::new();
    for value in values {
        let value = value.trim();
        let (group, name) = match value.split_once('/') {
            Some((g, n)) => (Some(g.trim()), n.trim()),
            None => (None, value),
        };
        let found = rows.iter().find(|r| {
            Some(r.0) == as_uuid(value)
                || (r.1.eq_ignore_ascii_case(name)
                    && group.is_none_or(|g| r.2.as_deref().is_some_and(|p| p.eq_ignore_ascii_case(g))))
        });
        let Some(found) = found else {
            let names = rows.iter().map(|r| r.1.clone()).collect();
            return Err(ApiError::bad_request(format!("No project label \"{value}\". {}", options(names))));
        };
        if !ids.contains(&found.0) {
            ids.push(found.0);
        }
    }
    Ok(ids)
}

pub async fn release(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    named(ctx.db(), ctx.workspace, "releases", "name", "", "release", value).await
}

pub async fn releases(ctx: &Ctx, values: &[String]) -> ApiResult<Vec<Uuid>> {
    let mut ids = Vec::new();
    for value in values {
        let id = release(ctx, value).await?;
        if !ids.contains(&id) {
            ids.push(id);
        }
    }
    Ok(ids)
}

pub async fn pipeline(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    named(ctx.db(), ctx.workspace, "release_pipelines", "name", "", "release pipeline", value).await
}

pub async fn customer(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    let value = value.trim();
    let by_domain: Option<Uuid> = sqlx::query_scalar(
        "SELECT id FROM customers WHERE workspace_id = $1 AND (lower($2) = ANY(domains) OR $2 = ANY(external_ids)) LIMIT 1",
    )
    .bind(ctx.workspace)
    .bind(value)
    .fetch_optional(ctx.db())
    .await?;
    if let Some(id) = by_domain {
        return Ok(id);
    }
    named(ctx.db(), ctx.workspace, "customers", "name", "", "customer", value).await
}

pub async fn custom_view(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    let extra = format!("AND (shared OR owner_id = '{}')", ctx.user);
    named(ctx.db(), ctx.workspace, "custom_views", "name", &extra, "view", value).await
}

/// A template by id or name, of the given type (issue, project or document).
pub async fn template(ctx: &Ctx, kind: &str, value: &str) -> ApiResult<Uuid> {
    let rows: Vec<(Uuid, String, Option<Uuid>)> =
        sqlx::query_as("SELECT id, name, team_id FROM templates WHERE workspace_id = $1 AND type = $2 ORDER BY name")
            .bind(ctx.workspace)
            .bind(kind)
            .fetch_all(ctx.db())
            .await?;
    let value = value.trim();
    let visible = ctx.team_ids().await?.to_vec();
    let rows: Vec<_> = rows.into_iter().filter(|r| r.2.is_none_or(|t| visible.contains(&t))).collect();
    let found = rows
        .iter()
        .find(|r| Some(r.0) == as_uuid(value))
        .or_else(|| rows.iter().find(|r| r.1.eq_ignore_ascii_case(value)));
    if let Some(row) = found {
        return Ok(row.0);
    }
    let names = rows.iter().map(|r| r.1.clone()).collect();
    Err(ApiError::bad_request(format!("No {kind} template \"{value}\". {}", options(names))))
}

/// A row of `table` in the caller's workspace by id, or a 404 naming `what`.
pub async fn by_id(ctx: &Ctx, table: &str, what: &str, value: &str) -> ApiResult<Uuid> {
    let not_found = || ApiError::not_found(format!("No {what} \"{}\"", value.trim()));
    let id = as_uuid(last_segment(value)).ok_or_else(not_found)?;
    let exists: bool =
        sqlx::query_scalar(&format!("SELECT EXISTS (SELECT 1 FROM {table} WHERE id = $1 AND workspace_id = $2)"))
            .bind(id)
            .bind(ctx.workspace)
            .fetch_one(ctx.db())
            .await?;
    if !exists {
        return Err(not_found());
    }
    Ok(id)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn identifiers() {
        assert_eq!(parse_identifier("eng-123"), Some(("ENG".into(), 123)));
        assert_eq!(parse_identifier("ENG2-7"), Some(("ENG2".into(), 7)));
        assert_eq!(parse_identifier("ENG-"), None);
        assert_eq!(parse_identifier("123"), None);
        assert_eq!(parse_identifier("my-project-0a1b2c3d4e5f"), None);
    }

    #[test]
    fn slugs_and_links() {
        assert_eq!(slug_tail("https://x.dev/acme/project/my-project-0a1b2c3d4e5f"), "0a1b2c3d4e5f");
        assert_eq!(slug_tail("0a1b2c3d4e5f"), "0a1b2c3d4e5f");
        assert_eq!(slug_tail("Launch"), "Launch");
        assert_eq!(last_segment("https://x.dev/acme/view/abc/?q=1"), "abc");
    }
}
