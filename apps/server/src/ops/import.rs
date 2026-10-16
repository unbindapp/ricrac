//! Importing issues from another tracker in one transaction, keeping their numbers, dates,
//! people and comments.

use std::collections::{HashMap, HashSet};

use chrono::{DateTime, Utc};
use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Value, json};
use sqlx::PgConnection;
use uuid::Uuid;

use super::issues::{check_estimate, default_state, issue_value, team_settings};
use super::resolve::{self, as_uuid};
use super::util::{check_priority, non_empty, parse_date, parse_datetime};
use super::{Op, attachments, events, op};
use crate::ctx::{Access, Ctx};
use crate::error::{ApiError, ApiResult};

const MAX_ISSUES: usize = 2000;

pub fn ops() -> Vec<Op> {
    vec![op(
        "import_issues",
        Access::Write,
        &[("POST", "/issues/import")],
        "Import issues from another tracker in one go (workspace admins). An issue can keep its number, so ENG-42 \
         stays ENG-42 (the number must be free in the team; gaps are fine), along with its creator, dates, status, \
         labels, assignee, project, links, subscribers and comments with their authors and dates. Issues point at \
         each other (parent, relatedTo, blocks, blockedBy, duplicateOf) by identifier, within the import or already \
         in Ricrac. Pass the issues inline, or upload them as a JSON file with prepare_file_upload and pass its \
         fileId, which keeps long descriptions out of the conversation. All or nothing, and nobody is notified. \
         dryRun checks everything and reports without importing.",
        import,
    )]
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ImportArgs {
    /// Team key, name or id the issues go to.
    team: String,
    /// The issues, at most 2000. Pass these or fileId.
    issues: Option<Vec<ImportIssue>>,
    /// A JSON file from prepare_file_upload holding the issues as an array, or as {"issues": [...]}.
    file_id: Option<String>,
    /// Check everything and report what would be imported, without importing.
    #[serde(default)]
    dry_run: bool,
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ImportIssue {
    /// The number to keep, e.g. 42 for ENG-42; must be free in the team. Without it the issue takes the next one.
    number: Option<i32>,
    title: String,
    /// Markdown, kept as is. Mentions don't notify anyone.
    description: Option<String>,
    /// Status name, type or id; the team's default when left out.
    state: Option<String>,
    /// 0 none, 1 urgent, 2 high, 3 medium, 4 low.
    priority: Option<i32>,
    /// Points on the team's scale.
    estimate: Option<f64>,
    /// Person (id, name, email); a member of the workspace.
    assignee: Option<String>,
    /// Who created it (id, name, email); you when left out.
    creator: Option<String>,
    /// Label names or ids (Group/Child for a label in a group).
    labels: Option<Vec<String>>,
    /// Project name, slug or id.
    project: Option<String>,
    /// Milestone name or id of the project.
    milestone: Option<String>,
    /// Cycle number, name or id of the team.
    cycle: Option<String>,
    /// The parent issue's identifier, within the import or already in Ricrac.
    parent: Option<String>,
    /// YYYY-MM-DD.
    due_date: Option<String>,
    /// Position on the team's board (lower is higher up).
    sort_order: Option<f64>,
    /// ISO 8601; now when left out.
    created_at: Option<String>,
    /// ISO 8601; the latest of its other dates when left out.
    updated_at: Option<String>,
    /// ISO 8601; createdAt when left out and the status is started or completed.
    started_at: Option<String>,
    /// ISO 8601; createdAt when left out and the status is completed.
    completed_at: Option<String>,
    /// ISO 8601; createdAt when left out and the status is canceled.
    canceled_at: Option<String>,
    /// ISO 8601; archives the issue.
    archived_at: Option<String>,
    /// Links to attach.
    links: Option<Vec<ImportLink>>,
    /// People subscribed besides the creator and assignee.
    subscribers: Option<Vec<String>>,
    /// Identifiers of related issues.
    related_to: Option<Vec<String>>,
    /// Identifiers of issues this one blocks.
    blocks: Option<Vec<String>>,
    /// Identifiers of issues blocking this one.
    blocked_by: Option<Vec<String>>,
    /// Identifier of the issue this duplicates.
    duplicate_of: Option<String>,
    /// Comments, oldest first.
    comments: Option<Vec<ImportComment>>,
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ImportLink {
    url: String,
    title: String,
    subtitle: Option<String>,
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ImportComment {
    /// Markdown.
    body: String,
    /// Person (id, name, email); you when left out.
    author: Option<String>,
    /// ISO 8601; the issue's createdAt when left out.
    created_at: Option<String>,
}

async fn import(ctx: Ctx, args: ImportArgs) -> ApiResult<Value> {
    ctx.require_admin()?;
    let issues = match (args.issues, &args.file_id) {
        (Some(issues), None) => issues,
        (None, Some(file)) => from_file(&ctx, file).await?,
        _ => return Err(ApiError::bad_request("Pass either issues or fileId")),
    };
    if issues.is_empty() || issues.len() > MAX_ISSUES {
        return Err(ApiError::bad_request(format!("Import 1 to {MAX_ISSUES} issues at a time")));
    }
    let team = resolve::team(&ctx, &args.team).await?;
    let mut tx = ctx.db().begin().await?;
    let key: String = sqlx::query_scalar("SELECT upper(key) FROM teams WHERE id = $1 FOR UPDATE")
        .bind(team)
        .fetch_one(&mut *tx)
        .await?;
    let numbers = assign_numbers(&mut tx, team, &key, &issues).await?;

    let mut ids = HashMap::new();
    let mut comments = 0;
    for (issue, &number) in issues.iter().zip(&numbers) {
        let id = insert(&ctx, &mut tx, team, number, issue).await.map_err(at(&key, number))?;
        comments += issue.comments.as_ref().map_or(0, Vec::len);
        ids.insert(number, id);
    }
    let mut relations = 0;
    for (issue, &number) in issues.iter().zip(&numbers) {
        relations += connect(&ctx, &mut tx, &key, &ids, ids[&number], issue).await.map_err(at(&key, number))?;
    }
    let count: i32 = sqlx::query_scalar(
        "UPDATE teams SET issue_count = greatest(issue_count, $2) WHERE id = $1 RETURNING issue_count",
    )
    .bind(team)
    .bind(numbers.iter().max())
    .fetch_one(&mut *tx)
    .await?;
    for id in ids.values() {
        let value = issue_value(&ctx, &mut tx, *id).await?;
        events::emit(&mut tx, &ctx, "Issue", "create", &value, None, Some(team)).await?;
    }
    events::audit(&mut tx, &ctx, "issues.imported", json!({ "team": key, "count": issues.len() })).await?;

    if args.dry_run {
        tx.rollback().await?;
    } else {
        tx.commit().await?;
    }
    let identifiers: Vec<String> = numbers.iter().map(|n| format!("{key}-{n}")).collect();
    Ok(json!({
        "dryRun": args.dry_run,
        "imported": identifiers.len(),
        "comments": comments,
        "relations": relations,
        "identifiers": identifiers,
        "nextIdentifier": format!("{key}-{}", count + 1),
    }))
}

async fn from_file(ctx: &Ctx, reference: &str) -> ApiResult<Vec<ImportIssue>> {
    let not_found = || ApiError::not_found(format!("No uploaded file \"{}\"", reference.trim()));
    let file = as_uuid(reference).ok_or_else(not_found)?;
    let data: Option<Vec<u8>> = sqlx::query_scalar("SELECT data FROM files WHERE id = $1 AND workspace_id = $2")
        .bind(file)
        .bind(ctx.workspace)
        .fetch_optional(ctx.db())
        .await?
        .ok_or_else(not_found)?;
    let data =
        data.ok_or_else(|| ApiError::bad_request("The file hasn't been uploaded yet; PUT it to its uploadUrl"))?;
    let mut parsed: Value =
        serde_json::from_slice(&data).map_err(|e| ApiError::bad_request(format!("The file isn't JSON: {e}")))?;
    let issues = parsed.get_mut("issues").map(Value::take).unwrap_or(parsed);
    serde_json::from_value(issues).map_err(|e| ApiError::bad_request(format!("The file's issues are invalid: {e}")))
}

/// The numbers given, checked to be free, and the next ones after everything else for the rest.
async fn assign_numbers(tx: &mut PgConnection, team: Uuid, key: &str, issues: &[ImportIssue]) -> ApiResult<Vec<i32>> {
    let given: Vec<i32> = issues.iter().filter_map(|i| i.number).collect();
    if let Some(bad) = given.iter().find(|&&n| n < 1) {
        return Err(ApiError::bad_request(format!("{bad} isn't an issue number; numbers start at 1")));
    }
    let mut seen = HashSet::new();
    if let Some(twice) = given.iter().find(|&&n| !seen.insert(n)) {
        return Err(ApiError::bad_request(format!("{key}-{twice} appears twice in the import")));
    }
    let taken: Vec<i32> =
        sqlx::query_scalar("SELECT number FROM issues WHERE team_id = $1 AND number = ANY($2) ORDER BY number")
            .bind(team)
            .bind(&given)
            .fetch_all(&mut *tx)
            .await?;
    if !taken.is_empty() {
        let list: Vec<String> = taken.iter().take(20).map(|n| format!("{key}-{n}")).collect();
        return Err(ApiError::bad_request(format!(
            "{} taken in the team: {}",
            if taken.len() == 1 { "This number is" } else { "These numbers are" },
            list.join(", ")
        )));
    }
    let count: i32 =
        sqlx::query_scalar("SELECT issue_count FROM teams WHERE id = $1").bind(team).fetch_one(&mut *tx).await?;
    let mut next = given.iter().copied().fold(count, i32::max);
    Ok(issues
        .iter()
        .map(|issue| {
            issue.number.unwrap_or_else(|| {
                next += 1;
                next
            })
        })
        .collect())
}

/// Says which issue of the import an error is about.
fn at(key: &str, number: i32) -> impl Fn(ApiError) -> ApiError {
    let identifier = format!("{key}-{number}");
    move |e| ApiError::new(e.status, format!("{identifier}: {}", e.message))
}

fn time(value: &Option<String>) -> ApiResult<Option<DateTime<Utc>>> {
    value.as_deref().map(parse_datetime).transpose()
}

async fn person(ctx: &Ctx, value: &Option<String>) -> ApiResult<Option<Uuid>> {
    match value {
        Some(value) => Ok(Some(resolve::user(ctx, value).await?)),
        None => Ok(None),
    }
}

async fn insert(ctx: &Ctx, tx: &mut PgConnection, team: Uuid, number: i32, issue: &ImportIssue) -> ApiResult<Uuid> {
    let title = non_empty(&issue.title, "title")?;
    let state = match &issue.state {
        Some(state) => resolve::state(ctx, team, state).await?,
        None => default_state(tx, team).await?,
    };
    let kind: String =
        sqlx::query_scalar("SELECT type FROM workflow_states WHERE id = $1").bind(state).fetch_one(&mut *tx).await?;
    let estimate = match issue.estimate {
        Some(estimate) => Some(check_estimate(&team_settings(tx, team).await?, estimate)?),
        None => None,
    };
    let creator = person(ctx, &issue.creator).await?.unwrap_or(ctx.user);
    let assignee = person(ctx, &issue.assignee).await?;
    let project = match &issue.project {
        Some(project) => Some(resolve::project(ctx, project).await?),
        None => None,
    };
    let milestone = match &issue.milestone {
        Some(milestone) => Some(resolve::milestone(ctx, project, milestone).await?),
        None => None,
    };
    let cycle = match &issue.cycle {
        Some(cycle) => Some(resolve::cycle(ctx, Some(team), cycle).await?),
        None => None,
    };
    let created = time(&issue.created_at)?.unwrap_or_else(Utc::now);
    let started = time(&issue.started_at)?.or((kind == "started" || kind == "completed").then_some(created));
    let completed = time(&issue.completed_at)?.or((kind == "completed").then_some(created));
    let canceled = time(&issue.canceled_at)?.or((kind == "canceled").then_some(created));
    let archived = time(&issue.archived_at)?;
    let comment_times: Vec<DateTime<Utc>> = issue
        .comments
        .iter()
        .flatten()
        .map(|c| time(&c.created_at).map(|t| t.unwrap_or(created)))
        .collect::<ApiResult<_>>()?;
    let updated = time(&issue.updated_at)?.unwrap_or_else(|| {
        [Some(created), started, completed, canceled, archived]
            .into_iter()
            .flatten()
            .chain(comment_times.clone())
            .max()
            .unwrap_or(created)
    });
    let due = issue.due_date.as_deref().map(parse_date).transpose()?;
    let sort_order: f64 = match issue.sort_order {
        Some(order) => order,
        None => {
            sqlx::query_scalar("SELECT coalesce(min(sort_order), 0) - 1000 FROM issues WHERE team_id = $1")
                .bind(team)
                .fetch_one(&mut *tx)
                .await?
        }
    };

    let id: Uuid = sqlx::query_scalar(
        "INSERT INTO issues (workspace_id, team_id, number, title, description, state_id, priority, estimate,
            assignee_id, creator_id, project_id, milestone_id, cycle_id, due_date, sort_order,
            started_at, completed_at, canceled_at, archived_at, created_at, updated_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, coalesce($11, (SELECT project_id FROM project_milestones WHERE id = $12)),
            $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
         RETURNING id",
    )
    .bind(ctx.workspace)
    .bind(team)
    .bind(number)
    .bind(&title)
    .bind(issue.description.as_deref().unwrap_or_default())
    .bind(state)
    .bind(check_priority(issue.priority.unwrap_or(0))?)
    .bind(estimate)
    .bind(assignee)
    .bind(creator)
    .bind(project)
    .bind(milestone)
    .bind(cycle)
    .bind(due)
    .bind(sort_order)
    .bind(started)
    .bind(completed)
    .bind(canceled)
    .bind(archived)
    .bind(created)
    .bind(updated)
    .fetch_one(&mut *tx)
    .await?;

    if let Some(project) = project {
        sqlx::query("INSERT INTO project_teams (project_id, team_id) VALUES ($1, $2) ON CONFLICT DO NOTHING")
            .bind(project)
            .bind(team)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(labels) = &issue.labels {
        let labels = resolve::labels(ctx, Some(team), labels).await?;
        let clash: Option<String> = sqlx::query_scalar(
            "SELECT p.name FROM issue_labels l JOIN issue_labels p ON p.id = l.parent_id
             WHERE l.id = ANY($1) AND p.group_type = 'singleSelect' GROUP BY p.name HAVING count(*) > 1 LIMIT 1",
        )
        .bind(&labels)
        .fetch_optional(&mut *tx)
        .await?;
        if let Some(group) = clash {
            return Err(ApiError::bad_request(format!("An issue takes one label of the group \"{group}\"")));
        }
        sqlx::query(
            "INSERT INTO issue_label_links (issue_id, label_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING",
        )
        .bind(id)
        .bind(&labels)
        .execute(&mut *tx)
        .await?;
    }
    let mut subscribers = vec![creator];
    subscribers.extend(assignee);
    subscribers.extend(resolve::users(ctx, issue.subscribers.as_deref().unwrap_or_default()).await?);
    events::subscribe(tx, id, &subscribers).await?;
    for link in issue.links.iter().flatten() {
        attachments::upsert(ctx, tx, id, &link.url, &link.title, link.subtitle.as_deref(), None).await?;
    }
    for (comment, at) in issue.comments.iter().flatten().zip(comment_times) {
        let author = person(ctx, &comment.author).await?.unwrap_or(ctx.user);
        sqlx::query(
            "INSERT INTO comments (workspace_id, issue_id, user_id, body, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $5)",
        )
        .bind(ctx.workspace)
        .bind(id)
        .bind(author)
        .bind(non_empty(&comment.body, "comment body")?)
        .bind(at)
        .execute(&mut *tx)
        .await?;
    }
    sqlx::query("INSERT INTO issue_history (issue_id, actor_id, action, changes, created_at) VALUES ($1, $2, 'created', $3, $4)")
        .bind(id)
        .bind(creator)
        .bind(json!({ "importedBy": ctx.user }))
        .bind(created)
        .execute(&mut *tx)
        .await?;
    Ok(id)
}

/// An issue of this import by its identifier, else one already in Ricrac.
async fn find(ctx: &Ctx, key: &str, ids: &HashMap<i32, Uuid>, value: &str) -> ApiResult<Uuid> {
    let imported = resolve::parse_identifier(value).filter(|(k, _)| k == key).and_then(|(_, n)| ids.get(&n));
    match imported {
        Some(id) => Ok(*id),
        None => resolve::issue(ctx, value).await,
    }
}

/// Parent, relations and duplicates, once every issue of the import exists. Returns how many
/// relations were added.
async fn connect(
    ctx: &Ctx,
    tx: &mut PgConnection,
    key: &str,
    ids: &HashMap<i32, Uuid>,
    id: Uuid,
    issue: &ImportIssue,
) -> ApiResult<usize> {
    if let Some(parent) = &issue.parent {
        let parent = find(ctx, key, ids, parent).await?;
        let loops: bool = sqlx::query_scalar(
            "WITH RECURSIVE up AS (SELECT id, parent_id FROM issues WHERE id = $1
             UNION SELECT i.id, i.parent_id FROM issues i JOIN up ON i.id = up.parent_id)
             SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)",
        )
        .bind(parent)
        .bind(id)
        .fetch_one(&mut *tx)
        .await?;
        if loops {
            return Err(ApiError::bad_request("An issue can't be a sub-issue of itself or of its own sub-issues"));
        }
        sqlx::query(
            "UPDATE issues SET parent_id = $2,
                sub_issue_sort_order = (SELECT coalesce(max(sub_issue_sort_order), 0) + 1000 FROM issues WHERE parent_id = $2)
             WHERE id = $1",
        )
        .bind(id)
        .bind(parent)
        .execute(&mut *tx)
        .await?;
    }
    let lists =
        [(&issue.related_to, "related", false), (&issue.blocks, "blocks", false), (&issue.blocked_by, "blocks", true)];
    let duplicate = issue.duplicate_of.iter().cloned().collect::<Vec<_>>();
    let mut added = 0;
    for (list, kind, reverse) in lists.into_iter().chain([(&Some(duplicate), "duplicate", false)]) {
        for other in list.iter().flatten() {
            let other = find(ctx, key, ids, other).await?;
            if other == id {
                return Err(ApiError::bad_request("An issue can't be related to itself"));
            }
            let (from, to) = if reverse { (other, id) } else { (id, other) };
            let inserted = sqlx::query(
                "INSERT INTO issue_relations (issue_id, related_issue_id, type, creator_id)
                 SELECT $1, $2, $3, $4 WHERE NOT ($3 = 'related' AND EXISTS (SELECT 1 FROM issue_relations
                    WHERE type = 'related' AND issue_id = $2 AND related_issue_id = $1))
                 ON CONFLICT DO NOTHING",
            )
            .bind(from)
            .bind(to)
            .bind(kind)
            .bind(ctx.user)
            .execute(&mut *tx)
            .await?;
            added += inserted.rows_affected() as usize;
        }
    }
    Ok(added)
}
