//! Issues: listing with filters, reading, creating and changing them (with the workflow
//! automations that come with a change), trash, archive, history and sharing.

use chrono::{DateTime, NaiveDate, Utc};
use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Value, json};
use sqlx::{PgConnection, Postgres, QueryBuilder};
use uuid::Uuid;

use super::events::{self, Notice};
use super::filter;
use super::resolve::{self, as_uuid};
use super::util::{Page, TextEdit, check_priority, new_content, non_empty, nullable, parse_date, parse_datetime};
use super::{IdArgs, Op, op};
use crate::ctx::{Access, Ctx};
use crate::error::{ApiError, ApiResult};

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "list_issues",
            Access::Read,
            &[("GET", "/issues")],
            "List issues, newest update first. Narrow with the simple arguments (team, assignee \"me\", state, label, \
             project, cycle, ...) and/or a `filter` object for anything more (and/or, comparators, relations). Archived \
             and trashed issues are left out unless asked for. Descriptions are left out unless includeDescription is set.",
            list,
        ),
        op(
            "get_issue",
            Access::Read,
            &[("GET", "/issues/{id}")],
            "Read an issue by identifier (ENG-123), id or link: its fields, description, sub-issues, relations \
             (blocks, blocked by, related, duplicates), attachments, subscribers and releases. Set includeComments or \
             includeHistory to also get its comment threads or change history.",
            get,
        ),
        op(
            "save_issue",
            Access::Write,
            &[("POST", "/issues"), ("PATCH", "/issues/{id}")],
            "Create an issue (omit id; team and title are required unless a template or parent provides them) or update \
             one (pass id). Omitted fields stay as they are; null clears a field. People are given as id, name, email or \
             \"me\"; states as name, type or id; labels, projects, cycles and milestones as name or id. Use patch to edit \
             part of the description without resending it. Moving an issue to another team gives it a new identifier; \
             the old one keeps working.",
            save,
        ),
        op(
            "update_issues",
            Access::Write,
            &[("PATCH", "/issues")],
            "Apply the same change to several issues at once, e.g. move them to a state, assign them, add a label or put \
             them in a cycle. `changes` takes the fields of save_issue (without id). Stops at the first issue that fails; \
             issues before it keep their change.",
            update_many,
        ),
        op(
            "delete_issue",
            Access::Write,
            &[("DELETE", "/issues/{id}")],
            "Move an issue to the trash. It disappears from lists and is deleted for good after 30 days; restore_issue \
             brings it back before then.",
            trash,
        ),
        op(
            "restore_issue",
            Access::Write,
            &[("POST", "/issues/{id}/restore")],
            "Bring an issue back from the trash.",
            restore,
        ),
        op(
            "archive_issue",
            Access::Write,
            &[("POST", "/issues/{id}/archive")],
            "Archive an issue (and its sub-issues). Archived issues are kept and readable but left out of lists unless \
             includeArchived is set. Closed issues are archived automatically after the team's auto-archive period.",
            archive,
        ),
        op(
            "unarchive_issue",
            Access::Write,
            &[("POST", "/issues/{id}/unarchive")],
            "Bring an archived issue back.",
            unarchive,
        ),
        op(
            "get_issue_history",
            Access::Read,
            &[("GET", "/issues/{id}/history")],
            "The change history of an issue, oldest first: who changed which field from what to what, and when.",
            history,
        ),
        op(
            "share_issue",
            Access::Write,
            &[("POST", "/issues/{id}/shares")],
            "Give a workspace member who can't see the issue's team (a guest, or someone outside a private team) access \
             to this one issue. Only when the user asks to share it.",
            share,
        ),
        op(
            "unshare_issue",
            Access::Write,
            &[("DELETE", "/issues/{id}/shares/{user}")],
            "Take back access to an issue given with share_issue.",
            unshare,
        ),
    ]
}

/* ------------------------------------------------------------------ */
/*                           Representation                           */
/* ------------------------------------------------------------------ */

pub const ISSUE_FROM: &str =
    "FROM issues i JOIN teams t ON t.id = i.team_id JOIN workspaces w ON w.id = i.workspace_id";

fn sql_literal(value: &str) -> String {
    format!("'{}'", value.replace('\'', "''"))
}

/// The JSON of the issue `i` (with its team `t` and workspace `w` joined).
pub fn issue_json(ctx: &Ctx, with_description: bool) -> String {
    let base = sql_literal(ctx.public_url());
    let description = if with_description { "'description', i.description," } else { "" };
    format!(
        "(jsonb_build_object(
            'id', i.id,
            'identifier', t.key || '-' || i.number,
            'number', i.number,
            'title', i.title,
            {description}
            'url', {base} || '/' || w.url_key || '/issue/' || t.key || '-' || i.number,
            'branchName', lower(t.key) || '-' || i.number || '-' ||
                left(trim(both '-' from regexp_replace(lower(i.title), '[^a-z0-9]+', '-', 'g')), 40),
            'priority', i.priority,
            'priorityLabel', (ARRAY['No priority','Urgent','High','Medium','Low'])[i.priority + 1],
            'estimate', i.estimate,
            'state', state_ref(i.state_id),
            'team', jsonb_build_object('id', t.id, 'key', t.key, 'name', t.name),
            'assignee', user_ref(i.assignee_id),
            'creator', user_ref(i.creator_id),
            'labels', issue_label_refs(i.id),
            'project', project_ref(i.project_id),
            'projectMilestone', milestone_ref(i.milestone_id),
            'cycle', cycle_ref(i.cycle_id),
            'parent', issue_ref(i.parent_id),
            'dueDate', i.due_date
        ) || jsonb_build_object(
            'sortOrder', i.sort_order,
            'subIssueSortOrder', i.sub_issue_sort_order,
            'subIssueCount', (SELECT count(*) FROM issues c WHERE c.parent_id = i.id AND c.trashed_at IS NULL),
            'slaType', i.sla_type,
            'slaStartedAt', i.sla_started_at,
            'slaBreachesAt', i.sla_breaches_at,
            'snoozedUntil', i.snoozed_until,
            'startedAt', i.started_at,
            'completedAt', i.completed_at,
            'canceledAt', i.canceled_at,
            'startedTriageAt', i.started_triage_at,
            'triagedAt', i.triaged_at,
            'autoClosedAt', i.auto_closed_at,
            'autoArchivedAt', i.auto_archived_at,
            'archivedAt', i.archived_at,
            'trashed', i.trashed_at IS NOT NULL,
            'trashedAt', i.trashed_at,
            'previousIdentifiers', to_jsonb(i.previous_identifiers),
            'createdAt', i.created_at,
            'updatedAt', i.updated_at
        ))"
    )
}

/// One issue's JSON, with its description.
pub async fn issue_value(ctx: &Ctx, db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    let sql = format!("SELECT {} {ISSUE_FROM} WHERE i.id = $1", issue_json(ctx, true));
    let value: Option<Value> = sqlx::query_scalar(&sql).bind(id).fetch_optional(&mut *db).await?;
    value.ok_or_else(|| ApiError::not_found("No such issue"))
}

/* ------------------------------------------------------------------ */
/*                                List                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema, Default)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ListIssuesArgs {
    /// Team key, name or id.
    team: Option<String>,
    /// With team: also issues of its sub-teams.
    #[serde(default)]
    include_sub_teams: bool,
    /// Person (id, name, email or "me"), or "null" for unassigned issues.
    assignee: Option<String>,
    /// Person who created the issues (id, name, email or "me").
    creator: Option<String>,
    /// Person subscribed to the issues (id, name, email or "me").
    subscriber: Option<String>,
    /// Status name, type (triage, backlog, unstarted, started, completed, canceled) or id.
    state: Option<String>,
    /// Label name or id.
    label: Option<String>,
    /// Project name, slug or id, or "null" for issues in no project.
    project: Option<String>,
    /// Project milestone name or id (name needs project).
    milestone: Option<String>,
    /// Cycle number, name or id, or "current", "next", "previous".
    cycle: Option<String>,
    /// Release name, slug or id.
    release: Option<String>,
    /// Sub-issues of this issue (identifier or id).
    parent_id: Option<String>,
    /// 0 none, 1 urgent, 2 high, 3 medium, 4 low.
    priority: Option<i32>,
    /// Words to find in the title, description or comments.
    query: Option<String>,
    /// Created after: ISO 8601 date/time or a duration like -P1D.
    created_at: Option<String>,
    /// Updated after: ISO 8601 date/time or a duration like -P1W.
    updated_at: Option<String>,
    /// Due on or before this date (or duration from now, e.g. P1W).
    due_before: Option<String>,
    /// A saved custom view (name, slug or id) whose filter applies too.
    custom_view: Option<String>,
    #[schemars(description = "Any further conditions. See the filter help in the description of this field: \
        Fields: id, number, title, description, priority, estimate, dueDate, createdAt, updatedAt, startedAt, completedAt, \
        canceledAt, archivedAt, triagedAt, snoozedUntil, slaBreachesAt, state {id,name,type}, team {id,key,name}, \
        assignee / creator {id,email,name,displayName,isMe}, labels {some|every|none: {id,name,parent}}, project {id,name,slugId}, \
        projectMilestone {id,name}, cycle {id,number,name,isActive}, parent {id,identifier}, subscribers {some: user}, \
        hasBlockedByRelations, hasBlockingRelations, hasRelatedRelations, hasDuplicateRelations, children {some: filter}, \
        searchableContent {contains}, and, or. Comparators: eq, neq, in, nin, lt, lte, gt, gte, contains, notContains, \
        startsWith, endsWith, null. Example: {\"or\": [{\"priority\": {\"lte\": 2, \"neq\": 0}}, {\"labels\": {\"some\": {\"name\": {\"eq\": \"Bug\"}}}}]}")]
    filter: Option<Value>,
    #[serde(default)]
    include_archived: bool,
    #[serde(default)]
    include_trashed: bool,
    /// Include each issue's description.
    #[serde(default)]
    include_description: bool,
    /// updatedAt (default), createdAt, priority, dueDate, manual (the team's board order) or identifier.
    order_by: Option<String>,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

fn eq(value: impl Into<Value>) -> Value {
    json!({ "eq": value.into() })
}

fn is_null_word(value: &str) -> bool {
    matches!(value.trim().to_lowercase().as_str(), "null" | "none" | "unassigned" | "no project")
}

async fn simple_filters(ctx: &Ctx, args: &ListIssuesArgs) -> ApiResult<Vec<Value>> {
    let mut parts = Vec::new();
    let mut team = None;
    if let Some(value) = &args.team {
        let id = resolve::team(ctx, value).await?;
        team = Some(id);
        if args.include_sub_teams {
            let ids: Vec<Uuid> = sqlx::query_scalar(
                "WITH RECURSIVE tree AS (SELECT id FROM teams WHERE id = $1
                 UNION SELECT t.id FROM teams t JOIN tree ON t.parent_id = tree.id) SELECT id FROM tree",
            )
            .bind(id)
            .fetch_all(ctx.db())
            .await?;
            parts.push(json!({ "team": { "id": { "in": ids } } }));
        } else {
            parts.push(json!({ "team": { "id": eq(id.to_string()) } }));
        }
    }
    if let Some(value) = &args.assignee {
        parts.push(match is_null_word(value) {
            true => json!({ "assignee": { "null": true } }),
            false => json!({ "assignee": { "id": eq(resolve::user(ctx, value).await?.to_string()) } }),
        });
    }
    if let Some(value) = &args.creator {
        parts.push(json!({ "creator": { "id": eq(resolve::user(ctx, value).await?.to_string()) } }));
    }
    if let Some(value) = &args.subscriber {
        parts.push(json!({ "subscribers": { "some": { "id": eq(resolve::user(ctx, value).await?.to_string()) } } }));
    }
    if let Some(value) = &args.state {
        parts.push(match (team, as_uuid(value)) {
            (Some(team), _) => json!({ "state": { "id": eq(resolve::state(ctx, team, value).await?.to_string()) } }),
            (None, Some(id)) => json!({ "state": { "id": eq(id.to_string()) } }),
            (None, None) => json!({ "or": [{ "state": { "name": eq(value.as_str()) } }, { "state": { "type": eq(value.as_str()) } }] }),
        });
    }
    if let Some(value) = &args.label {
        parts.push(match as_uuid(value) {
            Some(id) => json!({ "labels": { "some": { "id": eq(id.to_string()) } } }),
            None => {
                resolve::labels(ctx, team, std::slice::from_ref(value)).await?;
                let (group, name) =
                    value.split_once('/').map_or((None, value.as_str()), |(g, n)| (Some(g.trim()), n.trim()));
                match group {
                    Some(group) => {
                        json!({ "labels": { "some": { "name": eq(name), "parent": { "name": eq(group) } } } })
                    }
                    None => json!({ "labels": { "some": { "name": eq(name) } } }),
                }
            }
        });
    }
    let mut project = None;
    if let Some(value) = &args.project {
        parts.push(match is_null_word(value) {
            true => json!({ "project": { "null": true } }),
            false => {
                let id = resolve::project(ctx, value).await?;
                project = Some(id);
                json!({ "project": { "id": eq(id.to_string()) } })
            }
        });
    }
    if let Some(value) = &args.milestone {
        let id = resolve::milestone(ctx, project, value).await?;
        parts.push(json!({ "projectMilestone": { "id": eq(id.to_string()) } }));
    }
    if let Some(value) = &args.cycle {
        let lower = value.trim().to_lowercase();
        parts.push(match (team, lower.as_str()) {
            (Some(team), _) => {
                json!({ "cycle": { "id": eq(resolve::cycle(ctx, Some(team), value).await?.to_string()) } })
            }
            (None, "current" | "active") => json!({ "cycle": { "isActive": eq(true) } }),
            (None, _) if as_uuid(value).is_some() => json!({ "cycle": { "id": eq(value.trim()) } }),
            (None, _) => match value.trim().parse::<i64>() {
                Ok(number) => json!({ "cycle": { "number": eq(number) } }),
                Err(_) => json!({ "cycle": { "name": eq(value.as_str()) } }),
            },
        });
    }
    if let Some(value) = &args.release {
        let id = resolve::release(ctx, value).await?;
        parts.push(json!({ "releases": { "some": { "id": eq(id.to_string()) } } }));
    }
    if let Some(value) = &args.parent_id {
        let id = resolve::issue(ctx, value).await?;
        parts.push(json!({ "parent": { "id": eq(id.to_string()) } }));
    }
    if let Some(priority) = args.priority {
        parts.push(json!({ "priority": eq(check_priority(priority)?) }));
    }
    for (field, value) in [("createdAt", &args.created_at), ("updatedAt", &args.updated_at)] {
        if let Some(value) = value {
            parts.push(json!({ field: { "gte": parse_datetime(value)?.to_rfc3339() } }));
        }
    }
    if let Some(value) = &args.due_before {
        parts.push(json!({ "dueDate": { "lte": parse_datetime(value)?.to_rfc3339() } }));
    }
    Ok(parts)
}

async fn list(ctx: Ctx, args: ListIssuesArgs) -> ApiResult<Value> {
    let page = Page { limit: args.limit, cursor: args.cursor.clone() };
    let mut parts = simple_filters(&ctx, &args).await?;
    if let Some(view) = &args.custom_view {
        let id = resolve::custom_view(&ctx, view).await?;
        let (model, view_filter, view_team): (String, Value, Option<Uuid>) =
            sqlx::query_as("SELECT model, filter, team_id FROM custom_views WHERE id = $1")
                .bind(id)
                .fetch_one(ctx.db())
                .await?;
        if model != "issue" {
            return Err(ApiError::bad_request(format!("That view shows {model}s, not issues")));
        }
        parts.push(view_filter);
        if let Some(team) = view_team {
            parts.push(json!({ "team": { "id": eq(team.to_string()) } }));
        }
    }
    if let Some(extra) = &args.filter {
        parts.push(extra.clone());
    }
    let order = match args.order_by.as_deref().unwrap_or("updatedAt") {
        "updatedAt" => "i.updated_at DESC",
        "createdAt" => "i.created_at DESC",
        "priority" => "CASE WHEN i.priority = 0 THEN 5 ELSE i.priority END, i.updated_at DESC",
        "dueDate" => "i.due_date ASC NULLS LAST, i.priority",
        "manual" => "i.sort_order ASC",
        "identifier" | "number" => "t.key, i.number",
        other => {
            return Err(ApiError::bad_request(format!(
                "orderBy \"{other}\" isn't one of updatedAt, createdAt, priority, dueDate, manual, identifier"
            )));
        }
    };
    let mut qb = visible_issues(&ctx, &issue_json(&ctx, args.include_description)).await?;
    if !args.include_archived {
        qb.push(" AND i.archived_at IS NULL");
    }
    if !args.include_trashed {
        qb.push(" AND i.trashed_at IS NULL");
    }
    if let Some(query) = args.query.as_deref().map(str::trim).filter(|q| !q.is_empty()) {
        if let Some((key, number)) = resolve::parse_identifier(query) {
            qb.push(" AND ((upper(t.key) = ");
            qb.push_bind(key);
            qb.push(" AND i.number = ");
            qb.push_bind(number);
            qb.push(") OR ");
            filter::push_search(&mut qb, "i", query);
            qb.push(")");
        } else {
            qb.push(" AND ");
            filter::push_search(&mut qb, "i", query);
        }
    }
    filter::push(&mut qb, &ctx, "i", &json!({ "and": parts }))?;
    qb.push(format!(" ORDER BY {order} LIMIT "));
    qb.push_bind(page.limit() + 1);
    qb.push(" OFFSET ");
    qb.push_bind(page.offset()?);
    let nodes: Vec<Value> = qb.build_query_scalar().fetch_all(ctx.db()).await?;
    page.wrap(nodes)
}

/// `SELECT <columns> FROM issues ... WHERE <visible to the caller>`, to add conditions to.
pub async fn visible_issues<'a>(ctx: &Ctx, columns: &str) -> ApiResult<QueryBuilder<'a, Postgres>> {
    let teams = ctx.team_ids().await?.to_vec();
    let mut qb = QueryBuilder::new(format!("SELECT {columns} {ISSUE_FROM} WHERE i.workspace_id = "));
    qb.push_bind(ctx.workspace);
    qb.push(" AND (i.team_id = ANY(");
    qb.push_bind(teams);
    qb.push(") OR EXISTS (SELECT 1 FROM issue_shares sh WHERE sh.issue_id = i.id AND sh.user_id = ");
    qb.push_bind(ctx.user);
    qb.push("))");
    Ok(qb)
}

/* ------------------------------------------------------------------ */
/*                                Read                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct GetIssueArgs {
    /// Identifier (ENG-123), id or link.
    id: String,
    /// Also return the comment threads.
    #[serde(default)]
    include_comments: bool,
    /// Also return the change history.
    #[serde(default)]
    include_history: bool,
}

async fn get(ctx: Ctx, args: GetIssueArgs) -> ApiResult<Value> {
    let id = resolve::issue(&ctx, &args.id).await?;
    let mut db = ctx.db().acquire().await?;
    let mut value = issue_value(&ctx, &mut db, id).await?;
    let teams = ctx.team_ids().await?.to_vec();
    let children: Vec<Value> = sqlx::query_scalar(
        "SELECT jsonb_build_object('id', c.id, 'identifier', issue_identifier(c.id), 'title', c.title,
            'state', state_ref(c.state_id), 'assignee', user_ref(c.assignee_id), 'priority', c.priority)
         FROM issues c WHERE c.parent_id = $1 AND c.trashed_at IS NULL AND c.team_id = ANY($2)
         ORDER BY c.sub_issue_sort_order NULLS LAST, c.created_at",
    )
    .bind(id)
    .bind(&teams)
    .fetch_all(&mut *db)
    .await?;
    let relations: Vec<(String, bool, Value, Uuid)> = sqlx::query_as(
        "SELECT r.type, r.issue_id = $1, issue_ref(CASE WHEN r.issue_id = $1 THEN r.related_issue_id ELSE r.issue_id END), r.id
         FROM issue_relations r WHERE r.issue_id = $1 OR r.related_issue_id = $1 ORDER BY r.created_at",
    )
    .bind(id)
    .fetch_all(&mut *db)
    .await?;
    let pick = |kind: &str, outgoing: Option<bool>| -> Vec<Value> {
        relations
            .iter()
            .filter(|(t, out, _, _)| t == kind && outgoing.is_none_or(|o| o == *out))
            .map(|(_, _, issue, _)| issue.clone())
            .collect()
    };
    value["children"] = json!(children);
    value["relations"] = json!({
        "blocks": pick("blocks", Some(true)),
        "blockedBy": pick("blocks", Some(false)),
        "related": pick("related", None),
        "duplicateOf": pick("duplicate", Some(true)).into_iter().next(),
        "duplicates": pick("duplicate", Some(false)),
    });
    value["attachments"] = sqlx::query_scalar(
        "SELECT coalesce(jsonb_agg(jsonb_build_object('id', a.id, 'title', a.title, 'subtitle', a.subtitle, 'url', a.url,
            'sourceType', a.source_type, 'metadata', a.metadata, 'creator', user_ref(a.creator_id), 'createdAt', a.created_at)
            ORDER BY a.created_at), '[]') FROM attachments a WHERE a.issue_id = $1",
    )
    .bind(id)
    .fetch_one(&mut *db)
    .await?;
    value["subscribers"] = sqlx::query_scalar(
        "SELECT coalesce(jsonb_agg(user_ref(s.user_id)), '[]') FROM issue_subscribers s WHERE s.issue_id = $1",
    )
    .bind(id)
    .fetch_one(&mut *db)
    .await?;
    value["releases"] = sqlx::query_scalar(
        "SELECT coalesce(jsonb_agg(release_ref(r.release_id)), '[]') FROM issue_releases r WHERE r.issue_id = $1",
    )
    .bind(id)
    .fetch_one(&mut *db)
    .await?;
    value["documents"] = sqlx::query_scalar(
        "SELECT coalesce(jsonb_agg(document_ref(d.id)), '[]') FROM documents d WHERE d.issue_id = $1 AND d.trashed_at IS NULL",
    )
    .bind(id)
    .fetch_one(&mut *db)
    .await?;
    value["sharedWith"] = sqlx::query_scalar(
        "SELECT coalesce(jsonb_agg(user_ref(s.user_id)), '[]') FROM issue_shares s WHERE s.issue_id = $1",
    )
    .bind(id)
    .fetch_one(&mut *db)
    .await?;
    value["reactions"] = sqlx::query_scalar(
        "SELECT coalesce(jsonb_agg(jsonb_build_object('emoji', r.emoji, 'user', user_ref(r.user_id))), '[]')
         FROM reactions r WHERE r.issue_id = $1",
    )
    .bind(id)
    .fetch_one(&mut *db)
    .await?;
    value["commentCount"] = sqlx::query_scalar("SELECT to_jsonb(count(*)) FROM comments WHERE issue_id = $1")
        .bind(id)
        .fetch_one(&mut *db)
        .await?;
    if args.include_comments {
        value["comments"] = super::comments::threads(&ctx, &mut db, "issue_id", id).await?;
    }
    if args.include_history {
        value["history"] = history_value(&mut db, id).await?;
    }
    Ok(value)
}

async fn history_value(db: &mut PgConnection, issue: Uuid) -> ApiResult<Value> {
    Ok(sqlx::query_scalar(
        "SELECT coalesce(jsonb_agg(jsonb_build_object('action', h.action, 'actor', user_ref(h.actor_id),
            'changes', h.changes, 'createdAt', h.created_at) ORDER BY h.id), '[]')
         FROM issue_history h WHERE h.issue_id = $1",
    )
    .bind(issue)
    .fetch_one(&mut *db)
    .await?)
}

async fn history(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = resolve::issue(&ctx, &args.id).await?;
    let mut db = ctx.db().acquire().await?;
    Ok(json!({ "identifier": identifier(&mut db, id).await?, "history": history_value(&mut db, id).await? }))
}

pub async fn identifier(db: &mut PgConnection, issue: Uuid) -> ApiResult<String> {
    Ok(sqlx::query_scalar("SELECT issue_identifier($1)").bind(issue).fetch_one(&mut *db).await?)
}

/* ------------------------------------------------------------------ */
/*                                Save                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema, Clone)]
#[serde(deny_unknown_fields)]
pub struct LinkArg {
    pub url: String,
    pub title: String,
}

#[derive(Deserialize, JsonSchema, Default, Clone)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SaveIssueArgs {
    /// Only to update an issue: its identifier (ENG-123), id or link. Omit to create one.
    pub id: Option<String>,
    /// Team key, name or id. Required to create (unless parentId or the template sets it); changing it moves the issue.
    pub team: Option<String>,
    pub title: Option<String>,
    /// Markdown. Mention people with @displayName or @email; they are notified and subscribed.
    pub description: Option<String>,
    /// Edits to the current description instead of the whole text, applied in order, all or none.
    pub patch: Option<Vec<TextEdit>>,
    /// Status name, type (backlog, unstarted, started, completed, canceled, triage) or id.
    pub state: Option<String>,
    /// 0 none, 1 urgent, 2 high, 3 medium, 4 low.
    pub priority: Option<i32>,
    /// Estimate points on the team's scale; null clears it. T-shirt sizes are XS=1, S=2, M=3, L=5, XL=8, XXL=13, XXXL=21.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<f64>")]
    pub estimate: Option<Option<f64>>,
    /// Person (id, name, email or "me"); null unassigns.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    pub assignee: Option<Option<String>>,
    /// Replace all labels with these (names or ids; Group/Child for a label in a group).
    pub labels: Option<Vec<String>>,
    pub add_labels: Option<Vec<String>>,
    pub remove_labels: Option<Vec<String>>,
    /// Project name, slug or id; null removes the issue from its project.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    pub project: Option<Option<String>>,
    /// Project milestone name or id; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    pub milestone: Option<Option<String>>,
    /// Cycle number, name or id, or "current" / "next"; null removes it from its cycle.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    pub cycle: Option<Option<String>>,
    /// Parent issue (identifier or id), making this a sub-issue; null detaches it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    pub parent_id: Option<Option<String>>,
    /// YYYY-MM-DD; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    pub due_date: Option<Option<String>>,
    /// Issues this one blocks (added to existing relations).
    pub blocks: Option<Vec<String>>,
    /// Issues blocking this one (added to existing relations).
    pub blocked_by: Option<Vec<String>>,
    /// Issues related to this one (added).
    pub related_to: Option<Vec<String>>,
    pub remove_blocks: Option<Vec<String>>,
    pub remove_blocked_by: Option<Vec<String>>,
    pub remove_related_to: Option<Vec<String>>,
    /// The issue this duplicates; null removes the duplicate relation. Marking a duplicate moves it to a canceled status.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    pub duplicate_of: Option<Option<String>>,
    /// Links to attach ({url, title}); a link already on the issue is updated.
    pub links: Option<Vec<LinkArg>>,
    pub add_subscribers: Option<Vec<String>>,
    pub remove_subscribers: Option<Vec<String>>,
    /// Release names, slugs or ids to add.
    pub add_releases: Option<Vec<String>>,
    pub remove_releases: Option<Vec<String>>,
    /// Replace all releases with these.
    pub set_releases: Option<Vec<String>>,
    /// Hide the issue from triage / inbox until this time (ISO 8601 or duration like P1W); null wakes it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    pub snoozed_until: Option<Option<String>>,
    /// When the SLA breaches (ISO 8601); null removes the SLA.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    pub sla_breaches_at: Option<Option<String>>,
    /// "all" or "onlyBusinessDays".
    pub sla_type: Option<String>,
    /// Position on the team's board (lower is higher up).
    pub sort_order: Option<f64>,
    /// Position among its parent's sub-issues.
    pub sub_issue_sort_order: Option<f64>,
    /// Issue template name or id, applied on create: it fills in fields and sub-issues the arguments don't set.
    pub template: Option<String>,
}

#[derive(sqlx::FromRow, Clone)]
pub struct IssueRow {
    pub id: Uuid,
    pub team_id: Uuid,
    pub number: i32,
    pub title: String,
    pub description: String,
    pub state_id: Uuid,
    pub priority: i32,
    pub estimate: Option<f64>,
    pub assignee_id: Option<Uuid>,
    pub creator_id: Option<Uuid>,
    pub project_id: Option<Uuid>,
    pub milestone_id: Option<Uuid>,
    pub cycle_id: Option<Uuid>,
    pub parent_id: Option<Uuid>,
    pub due_date: Option<NaiveDate>,
    pub sort_order: f64,
    pub sub_issue_sort_order: Option<f64>,
    pub sla_type: Option<String>,
    pub sla_breaches_at: Option<DateTime<Utc>>,
    pub snoozed_until: Option<DateTime<Utc>>,
    pub archived_at: Option<DateTime<Utc>>,
    pub trashed_at: Option<DateTime<Utc>>,
}

pub async fn load_row(db: &mut PgConnection, id: Uuid) -> ApiResult<IssueRow> {
    let row = sqlx::query_as::<_, IssueRow>("SELECT * FROM issues WHERE id = $1 FOR UPDATE")
        .bind(id)
        .fetch_optional(&mut *db)
        .await?;
    row.ok_or_else(|| ApiError::not_found("No such issue"))
}

async fn save(ctx: Ctx, args: SaveIssueArgs) -> ApiResult<Value> {
    let mut tx = ctx.db().begin().await?;
    let id = match args.id.clone() {
        Some(reference) => {
            let id = resolve::issue(&ctx, &reference).await?;
            update(&ctx, &mut tx, id, args).await?;
            id
        }
        None => create(&ctx, &mut tx, args).await?,
    };
    let value = issue_value(&ctx, &mut tx, id).await?;
    tx.commit().await?;
    Ok(value)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct UpdateManyArgs {
    /// Identifiers (ENG-123) or ids of the issues to change, at most 100.
    ids: Vec<String>,
    /// The change: an object with save_issue's fields (except id and template), e.g.
    /// {"state": "Done"}, {"assignee": "me", "addLabels": ["Bug"]} or {"cycle": "current"}.
    #[schemars(with = "serde_json::Map<String, Value>")]
    changes: Value,
}

async fn update_many(ctx: Ctx, args: UpdateManyArgs) -> ApiResult<Value> {
    if args.ids.is_empty() || args.ids.len() > 100 {
        return Err(ApiError::bad_request("Pass 1 to 100 issue ids"));
    }
    let changes: SaveIssueArgs =
        serde_json::from_value(args.changes).map_err(|e| ApiError::bad_request(format!("Invalid changes: {e}")))?;
    if changes.id.is_some() || changes.template.is_some() {
        return Err(ApiError::bad_request("changes can't have an id or a template"));
    }
    let mut updated = Vec::new();
    for reference in &args.ids {
        let id = resolve::issue(&ctx, reference).await?;
        let mut tx = ctx.db().begin().await?;
        update(&ctx, &mut tx, id, changes.clone()).await?;
        let identifier = identifier(&mut tx, id).await?;
        tx.commit().await?;
        updated.push(identifier);
    }
    Ok(json!({ "updated": updated }))
}

struct TeamSettings {
    estimate_type: String,
    estimate_allow_zero: bool,
    estimate_extended: bool,
    default_estimate: Option<f64>,
    triage_enabled: bool,
}

async fn team_settings(db: &mut PgConnection, team: Uuid) -> ApiResult<TeamSettings> {
    let row: (String, bool, bool, Option<f64>, bool) = sqlx::query_as(
        "SELECT estimate_type, estimate_allow_zero, estimate_extended, default_estimate, triage_enabled FROM teams WHERE id = $1",
    )
    .bind(team)
    .fetch_one(&mut *db)
    .await?;
    Ok(TeamSettings {
        estimate_type: row.0,
        estimate_allow_zero: row.1,
        estimate_extended: row.2,
        default_estimate: row.3,
        triage_enabled: row.4,
    })
}

/// The estimates a team's scale allows.
pub fn allowed_estimates(kind: &str, allow_zero: bool, extended: bool) -> Vec<f64> {
    let (base, more): (&[f64], &[f64]) = match kind {
        "exponential" => (&[1.0, 2.0, 4.0, 8.0, 16.0], &[32.0, 64.0]),
        "fibonacci" => (&[1.0, 2.0, 3.0, 5.0, 8.0], &[13.0, 21.0]),
        "linear" => (&[1.0, 2.0, 3.0, 4.0, 5.0], &[6.0, 7.0, 8.0, 9.0, 10.0]),
        "tShirt" => (&[1.0, 2.0, 3.0, 5.0, 8.0], &[13.0, 21.0]),
        _ => (&[], &[]),
    };
    let mut values = Vec::new();
    if allow_zero {
        values.push(0.0);
    }
    values.extend_from_slice(base);
    if extended {
        values.extend_from_slice(more);
    }
    values
}

fn check_estimate(settings: &TeamSettings, estimate: f64) -> ApiResult<f64> {
    if settings.estimate_type == "notUsed" {
        return Err(ApiError::bad_request(
            "This team doesn't use estimates; turn them on with save_team (estimateType)",
        ));
    }
    let allowed = allowed_estimates(&settings.estimate_type, settings.estimate_allow_zero, settings.estimate_extended);
    if !allowed.contains(&estimate) {
        let list: Vec<String> = allowed.iter().map(|v| v.to_string()).collect();
        return Err(ApiError::bad_request(format!(
            "{estimate} isn't on the team's {} scale; use one of {}",
            settings.estimate_type,
            list.join(", ")
        )));
    }
    Ok(estimate)
}

/// The state new issues of the team start in: its default, else the first backlog or unstarted.
pub async fn default_state(db: &mut PgConnection, team: Uuid) -> ApiResult<Uuid> {
    let id: Option<Uuid> = sqlx::query_scalar(
        "SELECT s.id FROM workflow_states s JOIN teams t ON t.id = s.team_id
         WHERE s.team_id = $1 AND s.archived_at IS NULL
         ORDER BY s.id = t.default_issue_state_id DESC, array_position(ARRAY['backlog','unstarted','started','triage','completed','canceled'], s.type), s.position
         LIMIT 1",
    )
    .bind(team)
    .fetch_optional(&mut *db)
    .await?;
    id.ok_or_else(|| ApiError::bad_request("The team has no statuses"))
}

pub async fn first_state_of_type(db: &mut PgConnection, team: Uuid, kind: &str) -> ApiResult<Option<Uuid>> {
    Ok(sqlx::query_scalar(
        "SELECT id FROM workflow_states WHERE team_id = $1 AND type = $2 AND archived_at IS NULL ORDER BY position LIMIT 1",
    )
    .bind(team)
    .bind(kind)
    .fetch_optional(&mut *db)
    .await?)
}

async fn state_type(db: &mut PgConnection, state: Uuid) -> ApiResult<String> {
    Ok(sqlx::query_scalar("SELECT type FROM workflow_states WHERE id = $1").bind(state).fetch_one(&mut *db).await?)
}

/// What a template fills in on a new issue.
#[derive(Deserialize, Default)]
#[serde(rename_all = "camelCase", default)]
struct IssueTemplate {
    title: Option<String>,
    description: Option<String>,
    team: Option<String>,
    state: Option<String>,
    priority: Option<i32>,
    estimate: Option<f64>,
    assignee: Option<String>,
    labels: Option<Vec<String>>,
    project: Option<String>,
    cycle: Option<String>,
    due_date: Option<String>,
    sub_issues: Vec<Value>,
}

async fn create(ctx: &Ctx, tx: &mut PgConnection, mut args: SaveIssueArgs) -> ApiResult<Uuid> {
    let mut sub_issues = Vec::new();
    if let Some(template) = args.template.take() {
        let id = resolve::template(ctx, "issue", &template).await?;
        let (data, team): (Value, Option<Uuid>) =
            sqlx::query_as("SELECT data, team_id FROM templates WHERE id = $1").bind(id).fetch_one(&mut *tx).await?;
        let t: IssueTemplate = serde_json::from_value(data).unwrap_or_default();
        args.team = args.team.or(t.team).or(team.map(|t| t.to_string()));
        args.title = args.title.or(t.title);
        if args.description.as_deref().is_none_or(str::is_empty) {
            args.description = t.description;
        }
        args.state = args.state.or(t.state);
        args.priority = args.priority.or(t.priority);
        args.estimate = args.estimate.or(t.estimate.map(Some));
        args.assignee = args.assignee.or(t.assignee.map(Some));
        if let Some(labels) = t.labels {
            args.add_labels.get_or_insert_with(Vec::new).extend(labels);
        }
        args.project = args.project.or(t.project.map(Some));
        args.cycle = args.cycle.or(t.cycle.map(Some));
        args.due_date = args.due_date.or(t.due_date.map(Some));
        sub_issues = t.sub_issues;
    }
    let parent = match args.parent_id.clone().flatten() {
        Some(reference) => Some(resolve::issue(ctx, &reference).await?),
        None => None,
    };
    let team = match (&args.team, parent) {
        (Some(team), _) => resolve::team(ctx, team).await?,
        (None, Some(parent)) => {
            sqlx::query_scalar("SELECT team_id FROM issues WHERE id = $1").bind(parent).fetch_one(&mut *tx).await?
        }
        (None, None) => only_team(ctx).await?,
    };
    let title = non_empty(args.title.as_deref().unwrap_or(""), "title")?;
    let settings = team_settings(tx, team).await?;
    let state = match &args.state {
        Some(state) => resolve::state(ctx, team, state).await?,
        None => default_state(tx, team).await?,
    };
    let number: i32 =
        sqlx::query_scalar("UPDATE teams SET issue_count = issue_count + 1 WHERE id = $1 RETURNING issue_count")
            .bind(team)
            .fetch_one(&mut *tx)
            .await?;
    let sort_order: f64 = match args.sort_order {
        Some(order) => order,
        None => {
            sqlx::query_scalar("SELECT coalesce(min(sort_order), 0) - 1000 FROM issues WHERE team_id = $1")
                .bind(team)
                .fetch_one(&mut *tx)
                .await?
        }
    };
    let estimate = match args.estimate.take() {
        Some(Some(estimate)) => Some(check_estimate(&settings, estimate)?),
        Some(None) => None,
        None => settings.default_estimate.filter(|_| settings.estimate_type != "notUsed"),
    };
    let id: Uuid = sqlx::query_scalar(
        "INSERT INTO issues (workspace_id, team_id, number, title, description, state_id, priority, creator_id, sort_order, estimate)
         VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $8, $9) RETURNING id",
    )
    .bind(ctx.workspace)
    .bind(team)
    .bind(number)
    .bind(&title)
    .bind(args.description.take().unwrap_or_default())
    .bind(state)
    .bind(ctx.user)
    .bind(sort_order)
    .bind(estimate)
    .fetch_one(&mut *tx)
    .await?;
    let new_type = state_type(tx, state).await?;
    sqlx::query(
        "UPDATE issues SET
            started_at = CASE WHEN $2 IN ('started', 'completed') THEN now() END,
            completed_at = CASE WHEN $2 = 'completed' THEN now() END,
            canceled_at = CASE WHEN $2 = 'canceled' THEN now() END,
            started_triage_at = CASE WHEN $2 = 'triage' THEN now() END
         WHERE id = $1",
    )
    .bind(id)
    .bind(&new_type)
    .execute(&mut *tx)
    .await?;
    events::subscribe(tx, id, &[ctx.user]).await?;
    events::history(tx, ctx, id, "created", json!({})).await?;
    args.state = None;
    args.title = None;
    args.team = None;
    apply_changes(ctx, tx, id, args, true).await?;

    if new_type == "triage" && settings.triage_enabled {
        route_to_triage(ctx, tx, id, team).await?;
    }
    for (n, sub) in sub_issues.into_iter().enumerate() {
        let mut sub_args: SaveIssueArgs = serde_json::from_value(sub)
            .map_err(|e| ApiError::bad_request(format!("The template's sub-issue {} is invalid: {e}", n + 1)))?;
        sub_args.id = None;
        sub_args.template = None;
        // The parent isn't committed yet, so resolving it by reference would miss it.
        sub_args.parent_id = None;
        sub_args.team = sub_args.team.or(Some(team.to_string()));
        let child = Box::pin(create(ctx, tx, sub_args)).await?;
        sqlx::query("UPDATE issues SET parent_id = $2, sub_issue_sort_order = $3 WHERE id = $1")
            .bind(child)
            .bind(id)
            .bind((n as f64 + 1.0) * 1000.0)
            .execute(&mut *tx)
            .await?;
    }
    let value = issue_value(ctx, tx, id).await?;
    events::emit(tx, ctx, "Issue", "create", &value, None, Some(team)).await?;
    Ok(id)
}

/// The team to use when none was named: the caller's only team.
async fn only_team(ctx: &Ctx) -> ApiResult<Uuid> {
    let teams: Vec<(Uuid, String)> =
        sqlx::query_as("SELECT id, key FROM teams WHERE id = ANY($1) AND archived_at IS NULL ORDER BY key")
            .bind(ctx.team_ids().await?)
            .fetch_all(ctx.db())
            .await?;
    match teams.as_slice() {
        [(id, _)] => Ok(*id),
        [] => Err(ApiError::bad_request("There are no teams yet; create one with save_team")),
        many => {
            let keys: Vec<_> = many.iter().map(|(_, k)| k.as_str()).collect();
            Err(ApiError::bad_request(format!("Name the team: {}", keys.join(", "))))
        }
    }
}

/// Who is on triage for the team now: with a rotation, the person whose turn it is; else everyone listed.
pub async fn triage_responsible(db: &mut PgConnection, team: Uuid) -> ApiResult<(String, Vec<Uuid>)> {
    let (action, ids, weeks, start): (String, Vec<Uuid>, Option<i32>, Option<NaiveDate>) = sqlx::query_as(
        "SELECT triage_action, triage_responsible_ids, triage_rotation_weeks, triage_rotation_start FROM teams WHERE id = $1",
    )
    .bind(team)
    .fetch_one(&mut *db)
    .await?;
    let current = match (weeks, start, ids.is_empty()) {
        (Some(weeks), Some(start), false) if weeks > 0 => {
            let elapsed = (Utc::now().date_naive() - start).num_weeks().max(0);
            let turn = (elapsed / weeks as i64) as usize % ids.len();
            vec![ids[turn]]
        }
        _ => ids,
    };
    Ok((action, current))
}

async fn route_to_triage(ctx: &Ctx, tx: &mut PgConnection, issue: Uuid, team: Uuid) -> ApiResult<()> {
    let (action, people) = triage_responsible(tx, team).await?;
    if people.is_empty() || action == "none" {
        return Ok(());
    }
    let identifier = identifier(tx, issue).await?;
    if action == "assign" {
        let assignee = people[0];
        sqlx::query("UPDATE issues SET assignee_id = $2 WHERE id = $1 AND assignee_id IS NULL")
            .bind(issue)
            .bind(assignee)
            .execute(&mut *tx)
            .await?;
        events::subscribe(tx, issue, &[assignee]).await?;
    }
    let notice = Notice {
        kind: "triageIssue",
        summary: format!("{identifier} needs triage"),
        issue: Some(issue),
        ..Default::default()
    };
    events::notify(tx, ctx, &people, &notice).await
}

async fn update(ctx: &Ctx, tx: &mut PgConnection, id: Uuid, mut args: SaveIssueArgs) -> ApiResult<()> {
    if args.template.is_some() {
        return Err(ApiError::bad_request("Templates only apply when creating an issue"));
    }
    let before_value = issue_value(ctx, tx, id).await?;
    let row = load_row(tx, id).await?;
    if let Some(team) = args.team.take() {
        let team = resolve::team(ctx, &team).await?;
        if team != row.team_id {
            move_team(ctx, tx, &row, team, args.state.is_some()).await?;
        }
    }
    let row = load_row(tx, id).await?;
    let mut changes = serde_json::Map::new();
    if let Some(title) = args.title.take() {
        let title = non_empty(&title, "title")?;
        if title != row.title {
            sqlx::query("UPDATE issues SET title = $2 WHERE id = $1").bind(id).bind(&title).execute(&mut *tx).await?;
            changes.insert("title".into(), json!({ "from": row.title, "to": title }));
        }
    }
    if let Some(description) = new_content(&row.description, args.description.take(), args.patch.take().as_deref())?
        && description != row.description
    {
        sqlx::query("UPDATE issues SET description = $2 WHERE id = $1")
            .bind(id)
            .bind(&description)
            .execute(&mut *tx)
            .await?;
        changes.insert("description".into(), json!({ "changed": true }));
        let previous = events::mentions(ctx, &row.description).await?;
        let mentioned: Vec<Uuid> =
            events::mentions(ctx, &description).await?.into_iter().filter(|u| !previous.contains(u)).collect();
        notify_mentions(ctx, tx, id, &mentioned).await?;
    }
    if let Some(state) = args.state.take() {
        let state = resolve::state(ctx, row.team_id, &state).await?;
        if state != row.state_id {
            set_state(ctx, tx, id, state, "updated").await?;
        }
    }
    if !changes.is_empty() {
        events::history(tx, ctx, id, "updated", Value::Object(changes)).await?;
    }
    apply_changes(ctx, tx, id, args, false).await?;
    sqlx::query("UPDATE issues SET updated_at = now() WHERE id = $1").bind(id).execute(&mut *tx).await?;
    let after = issue_value(ctx, tx, id).await?;
    let updated_from = changed_fields(&before_value, &after);
    if !updated_from.as_object().is_none_or(|o| o.is_empty()) {
        events::emit(tx, ctx, "Issue", "update", &after, Some(updated_from), Some(row.team_id)).await?;
    }
    Ok(())
}

/// The previous values of the fields that changed, for webhooks' `updatedFrom`.
pub fn changed_fields(before: &Value, after: &Value) -> Value {
    let (Some(before), Some(after)) = (before.as_object(), after.as_object()) else { return json!({}) };
    let changed: serde_json::Map<String, Value> = before
        .iter()
        .filter(|(k, v)| k.as_str() != "updatedAt" && after.get(k.as_str()) != Some(v))
        .map(|(k, v)| (k.clone(), v.clone()))
        .collect();
    Value::Object(changed)
}

async fn notify_mentions(ctx: &Ctx, tx: &mut PgConnection, issue: Uuid, users: &[Uuid]) -> ApiResult<()> {
    if users.is_empty() {
        return Ok(());
    }
    events::subscribe(tx, issue, users).await?;
    let identifier = identifier(tx, issue).await?;
    let notice = Notice {
        kind: "issueMention",
        summary: format!("You were mentioned in {identifier}"),
        issue: Some(issue),
        ..Default::default()
    };
    events::notify(tx, ctx, users, &notice).await
}

/// Moves the issue to another team: a new number (the old identifier keeps resolving), a status
/// of the same name or type, and only labels the new team has. Its sub-issues come along.
async fn move_team(ctx: &Ctx, tx: &mut PgConnection, row: &IssueRow, team: Uuid, state_given: bool) -> ApiResult<()> {
    let old_identifier = identifier(tx, row.id).await?;
    let number: i32 =
        sqlx::query_scalar("UPDATE teams SET issue_count = issue_count + 1 WHERE id = $1 RETURNING issue_count")
            .bind(team)
            .fetch_one(&mut *tx)
            .await?;
    let (old_name, old_type): (String, String) = sqlx::query_as("SELECT name, type FROM workflow_states WHERE id = $1")
        .bind(row.state_id)
        .fetch_one(&mut *tx)
        .await?;
    let same_name: Option<Uuid> = sqlx::query_scalar(
        "SELECT id FROM workflow_states WHERE team_id = $1 AND lower(name) = lower($2) AND archived_at IS NULL LIMIT 1",
    )
    .bind(team)
    .bind(&old_name)
    .fetch_optional(&mut *tx)
    .await?;
    let state = match (same_name, state_given) {
        (Some(id), _) => id,
        (None, _) => match first_state_of_type(tx, team, &old_type).await? {
            Some(id) => id,
            None => default_state(tx, team).await?,
        },
    };
    sqlx::query(
        "UPDATE issues SET team_id = $2, number = $3, state_id = $4, cycle_id = NULL,
            previous_identifiers = array_append(previous_identifiers, $5), updated_at = now()
         WHERE id = $1",
    )
    .bind(row.id)
    .bind(team)
    .bind(number)
    .bind(state)
    .bind(&old_identifier)
    .execute(&mut *tx)
    .await?;
    sqlx::query(
        "DELETE FROM issue_label_links l USING issue_labels lb
         WHERE l.issue_id = $1 AND lb.id = l.label_id AND lb.team_id IS NOT NULL AND lb.team_id <> $2",
    )
    .bind(row.id)
    .bind(team)
    .execute(&mut *tx)
    .await?;
    let new_identifier = identifier(tx, row.id).await?;
    let teams: (Value, Value) =
        sqlx::query_as("SELECT team_ref($1), team_ref($2)").bind(row.team_id).bind(team).fetch_one(&mut *tx).await?;
    events::history(
        tx,
        ctx,
        row.id,
        "moved",
        json!({ "team": { "from": teams.0, "to": teams.1 }, "identifier": { "from": old_identifier, "to": new_identifier } }),
    )
    .await?;
    let children: Vec<Uuid> =
        sqlx::query_scalar("SELECT id FROM issues WHERE parent_id = $1 AND team_id = $2 AND trashed_at IS NULL")
            .bind(row.id)
            .bind(row.team_id)
            .fetch_all(&mut *tx)
            .await?;
    for child in children {
        let child_row = load_row(tx, child).await?;
        Box::pin(move_team(ctx, tx, &child_row, team, false)).await?;
    }
    Ok(())
}

/// Moves the issue to `state`, stamping when it started, completed or left triage, and runs what
/// follows: notifications, the cycle it joins, and closing parents or sub-issues when the team
/// automates that.
pub async fn set_state(ctx: &Ctx, tx: &mut PgConnection, issue: Uuid, state: Uuid, action: &str) -> ApiResult<()> {
    let mut queue = vec![(issue, state, action.to_string())];
    while let Some((issue, state, action)) = queue.pop() {
        let row = load_row(tx, issue).await?;
        if row.state_id == state {
            continue;
        }
        let old_type = state_type(tx, row.state_id).await?;
        let new_type = state_type(tx, state).await?;
        sqlx::query(
            "UPDATE issues SET state_id = $2,
                started_at = CASE WHEN $3 = 'started' THEN coalesce(started_at, now())
                                  WHEN $3 IN ('completed', 'canceled') THEN started_at END,
                completed_at = CASE WHEN $3 = 'completed' THEN now() END,
                canceled_at = CASE WHEN $3 = 'canceled' THEN now() END,
                started_triage_at = CASE WHEN $3 = 'triage' THEN coalesce(started_triage_at, now()) ELSE started_triage_at END,
                triaged_at = CASE WHEN $3 <> 'triage' AND $4 = 'triage' THEN now() ELSE triaged_at END,
                auto_closed_at = NULL,
                updated_at = now()
             WHERE id = $1",
        )
        .bind(issue)
        .bind(state)
        .bind(&new_type)
        .bind(&old_type)
        .execute(&mut *tx)
        .await?;
        let refs: (Option<Value>, Option<Value>) = sqlx::query_as("SELECT state_ref($1), state_ref($2)")
            .bind(row.state_id)
            .bind(state)
            .fetch_one(&mut *tx)
            .await?;
        events::history(tx, ctx, issue, &action, json!({ "state": { "from": refs.0, "to": refs.1.clone() } })).await?;

        let identifier = identifier(tx, issue).await?;
        let state_name = refs.1.as_ref().and_then(|v| v["name"].as_str()).unwrap_or_default().to_string();
        let mut subscribers = events::issue_subscribers(tx, issue).await?;
        subscribers.extend(row.assignee_id);
        let notice = Notice {
            kind: "issueStatusChanged",
            summary: format!("{identifier} moved to {state_name}"),
            issue: Some(issue),
            ..Default::default()
        };
        events::notify(tx, ctx, &subscribers, &notice).await?;

        add_to_current_cycle(tx, &row, &new_type).await?;

        let closed = matches!(new_type.as_str(), "completed" | "canceled");
        if closed {
            notify_unblocked(ctx, tx, issue).await?;
            let close_children: bool = sqlx::query_scalar("SELECT auto_close_child_issues FROM teams WHERE id = $1")
                .bind(row.team_id)
                .fetch_one(&mut *tx)
                .await?;
            if close_children {
                let children: Vec<(Uuid, Uuid)> = sqlx::query_as(
                    "SELECT c.id, c.team_id FROM issues c JOIN workflow_states s ON s.id = c.state_id
                     WHERE c.parent_id = $1 AND c.trashed_at IS NULL AND s.type NOT IN ('completed', 'canceled')",
                )
                .bind(issue)
                .fetch_all(&mut *tx)
                .await?;
                for (child, child_team) in children {
                    if let Some(target) = first_state_of_type(tx, child_team, &new_type).await? {
                        queue.push((child, target, "auto_closed_with_parent".into()));
                    }
                }
            }
        }
        if let Some(parent) = row.parent_id {
            let (parent_team, parent_type): (Uuid, String) = sqlx::query_as(
                "SELECT i.team_id, s.type FROM issues i JOIN workflow_states s ON s.id = i.state_id WHERE i.id = $1",
            )
            .bind(parent)
            .fetch_one(&mut *tx)
            .await?;
            let close_parent: bool = sqlx::query_scalar("SELECT auto_close_parent_issues FROM teams WHERE id = $1")
                .bind(parent_team)
                .fetch_one(&mut *tx)
                .await?;
            let (open, done): (i64, i64) = sqlx::query_as(
                "SELECT count(*) FILTER (WHERE s.type NOT IN ('completed', 'canceled')), count(*) FILTER (WHERE s.type = 'completed')
                 FROM issues c JOIN workflow_states s ON s.id = c.state_id WHERE c.parent_id = $1 AND c.trashed_at IS NULL",
            )
            .bind(parent)
            .fetch_one(&mut *tx)
            .await?;
            if close_parent
                && closed
                && open == 0
                && done > 0
                && !matches!(parent_type.as_str(), "completed" | "canceled")
                && let Some(target) = first_state_of_type(tx, parent_team, "completed").await?
            {
                queue.push((parent, target, "auto_completed_by_sub_issues".into()));
            }
        }
    }
    Ok(())
}

/// Puts a started (or completed) issue without a cycle into the team's current cycle, when the
/// team does that.
async fn add_to_current_cycle(tx: &mut PgConnection, row: &IssueRow, new_type: &str) -> ApiResult<()> {
    if row.cycle_id.is_some() || !matches!(new_type, "started" | "completed") {
        return Ok(());
    }
    let column = if new_type == "started" { "cycle_auto_add_started" } else { "cycle_auto_add_completed" };
    sqlx::query(&format!(
        "UPDATE issues SET cycle_id = (SELECT c.id FROM cycles c WHERE c.team_id = $2 AND c.starts_at <= now()
            AND c.ends_at > now() AND c.archived_at IS NULL ORDER BY c.starts_at DESC LIMIT 1)
         WHERE id = $1 AND EXISTS (SELECT 1 FROM teams t WHERE t.id = $2 AND t.cycles_enabled AND t.{column})"
    ))
    .bind(row.id)
    .bind(row.team_id)
    .execute(&mut *tx)
    .await?;
    Ok(())
}

/// Tells the people on issues this one blocked that nothing blocks them anymore.
async fn notify_unblocked(ctx: &Ctx, tx: &mut PgConnection, issue: Uuid) -> ApiResult<()> {
    let freed: Vec<Uuid> = sqlx::query_scalar(
        "SELECT r.related_issue_id FROM issue_relations r WHERE r.issue_id = $1 AND r.type = 'blocks'
         AND NOT EXISTS (SELECT 1 FROM issue_relations o JOIN issues b ON b.id = o.issue_id
            JOIN workflow_states s ON s.id = b.state_id
            WHERE o.related_issue_id = r.related_issue_id AND o.type = 'blocks' AND o.issue_id <> $1
            AND s.type NOT IN ('completed', 'canceled') AND b.trashed_at IS NULL)",
    )
    .bind(issue)
    .fetch_all(&mut *tx)
    .await?;
    for blocked in freed {
        let identifier = identifier(tx, blocked).await?;
        let mut people = events::issue_subscribers(tx, blocked).await?;
        let assignee: Option<Uuid> = sqlx::query_scalar("SELECT assignee_id FROM issues WHERE id = $1")
            .bind(blocked)
            .fetch_one(&mut *tx)
            .await?;
        people.extend(assignee);
        let notice = Notice {
            kind: "issueUnblocked",
            summary: format!("{identifier} is no longer blocked"),
            issue: Some(blocked),
            ..Default::default()
        };
        events::notify(tx, ctx, &people, &notice).await?;
    }
    Ok(())
}

/// The fields that work the same when creating and updating.
async fn apply_changes(
    ctx: &Ctx,
    tx: &mut PgConnection,
    id: Uuid,
    args: SaveIssueArgs,
    creating: bool,
) -> ApiResult<()> {
    let row = load_row(tx, id).await?;
    let mut changes = serde_json::Map::new();
    let settings = team_settings(tx, row.team_id).await?;

    if let Some(priority) = args.priority {
        let priority = check_priority(priority)?;
        if priority != row.priority {
            sqlx::query("UPDATE issues SET priority = $2 WHERE id = $1")
                .bind(id)
                .bind(priority)
                .execute(&mut *tx)
                .await?;
            changes.insert("priority".into(), json!({ "from": row.priority, "to": priority }));
        }
    }
    if let Some(estimate) = args.estimate {
        let estimate = estimate.map(|e| check_estimate(&settings, e)).transpose()?;
        if estimate != row.estimate {
            sqlx::query("UPDATE issues SET estimate = $2 WHERE id = $1")
                .bind(id)
                .bind(estimate)
                .execute(&mut *tx)
                .await?;
            changes.insert("estimate".into(), json!({ "from": row.estimate, "to": estimate }));
        }
    }
    if let Some(assignee) = args.assignee.clone() {
        let assignee = match assignee {
            Some(person) if !is_null_word(&person) => Some(resolve::user(ctx, &person).await?),
            _ => None,
        };
        if assignee != row.assignee_id {
            if let Some(user) = assignee {
                check_can_see_team(ctx, tx, user, row.team_id, id).await?;
            }
            sqlx::query("UPDATE issues SET assignee_id = $2 WHERE id = $1")
                .bind(id)
                .bind(assignee)
                .execute(&mut *tx)
                .await?;
            let refs: (Option<Value>, Option<Value>) = sqlx::query_as("SELECT user_ref($1), user_ref($2)")
                .bind(row.assignee_id)
                .bind(assignee)
                .fetch_one(&mut *tx)
                .await?;
            changes.insert("assignee".into(), json!({ "from": refs.0, "to": refs.1 }));
            let identifier = identifier(tx, id).await?;
            if let Some(user) = assignee {
                events::subscribe(tx, id, &[user]).await?;
                let notice = Notice {
                    kind: "issueAssignedToYou",
                    summary: format!("{identifier} was assigned to you"),
                    issue: Some(id),
                    ..Default::default()
                };
                events::notify(tx, ctx, &[user], &notice).await?;
            }
            if let Some(previous) = row.assignee_id {
                let notice = Notice {
                    kind: "issueUnassignedFromYou",
                    summary: format!("{identifier} was unassigned from you"),
                    issue: Some(id),
                    ..Default::default()
                };
                events::notify(tx, ctx, &[previous], &notice).await?;
            }
        }
    }
    if creating {
        let description: String =
            sqlx::query_scalar("SELECT description FROM issues WHERE id = $1").bind(id).fetch_one(&mut *tx).await?;
        let mentioned = events::mentions(ctx, &description).await?;
        notify_mentions(ctx, tx, id, &mentioned).await?;
    }

    labels(ctx, tx, &row, &args, &mut changes).await?;

    let mut project = row.project_id;
    if let Some(value) = &args.project {
        project = match value {
            Some(p) if !is_null_word(p) => Some(resolve::project(ctx, p).await?),
            _ => None,
        };
        if project != row.project_id {
            sqlx::query("UPDATE issues SET project_id = $2, milestone_id = CASE WHEN $2 IS NULL OR $2 IS DISTINCT FROM project_id THEN NULL ELSE milestone_id END WHERE id = $1")
                .bind(id)
                .bind(project)
                .execute(&mut *tx)
                .await?;
            if let Some(project) = project {
                sqlx::query("INSERT INTO project_teams (project_id, team_id) VALUES ($1, $2) ON CONFLICT DO NOTHING")
                    .bind(project)
                    .bind(row.team_id)
                    .execute(&mut *tx)
                    .await?;
            }
            let refs: (Option<Value>, Option<Value>) = sqlx::query_as("SELECT project_ref($1), project_ref($2)")
                .bind(row.project_id)
                .bind(project)
                .fetch_one(&mut *tx)
                .await?;
            changes.insert("project".into(), json!({ "from": refs.0, "to": refs.1 }));
        }
    }
    if let Some(value) = &args.milestone {
        let milestone = match value {
            Some(m) if !is_null_word(m) => Some(resolve::milestone(ctx, project, m).await?),
            _ => None,
        };
        let milestone_project: Option<Uuid> = match milestone {
            Some(m) => {
                sqlx::query_scalar("SELECT project_id FROM project_milestones WHERE id = $1")
                    .bind(m)
                    .fetch_optional(&mut *tx)
                    .await?
            }
            None => None,
        };
        if let (Some(mp), Some(p)) = (milestone_project, project)
            && mp != p
        {
            return Err(ApiError::bad_request("That milestone belongs to another project than the issue's"));
        }
        sqlx::query("UPDATE issues SET milestone_id = $2, project_id = coalesce($3, project_id) WHERE id = $1")
            .bind(id)
            .bind(milestone)
            .bind(milestone_project)
            .execute(&mut *tx)
            .await?;
        if milestone != row.milestone_id {
            let refs: (Option<Value>, Option<Value>) = sqlx::query_as("SELECT milestone_ref($1), milestone_ref($2)")
                .bind(row.milestone_id)
                .bind(milestone)
                .fetch_one(&mut *tx)
                .await?;
            changes.insert("projectMilestone".into(), json!({ "from": refs.0, "to": refs.1 }));
        }
    }
    if let Some(value) = &args.cycle {
        let cycle = match value {
            Some(c) if !is_null_word(c) => Some(resolve::cycle(ctx, Some(row.team_id), c).await?),
            _ => None,
        };
        if let Some(cycle) = cycle {
            let cycle_team: Uuid =
                sqlx::query_scalar("SELECT team_id FROM cycles WHERE id = $1").bind(cycle).fetch_one(&mut *tx).await?;
            if cycle_team != row.team_id {
                return Err(ApiError::bad_request("That cycle belongs to another team"));
            }
        }
        if cycle != row.cycle_id {
            sqlx::query("UPDATE issues SET cycle_id = $2 WHERE id = $1").bind(id).bind(cycle).execute(&mut *tx).await?;
            let refs: (Option<Value>, Option<Value>) = sqlx::query_as("SELECT cycle_ref($1), cycle_ref($2)")
                .bind(row.cycle_id)
                .bind(cycle)
                .fetch_one(&mut *tx)
                .await?;
            changes.insert("cycle".into(), json!({ "from": refs.0, "to": refs.1 }));
        }
    }
    if let Some(value) = &args.parent_id {
        let parent = match value {
            Some(p) if !is_null_word(p) => Some(resolve::issue(ctx, p).await?),
            _ => None,
        };
        if let Some(parent) = parent {
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
        }
        if parent != row.parent_id {
            sqlx::query(
                "UPDATE issues SET parent_id = $2, sub_issue_sort_order = CASE WHEN $2 IS NULL THEN NULL ELSE
                    (SELECT coalesce(max(sub_issue_sort_order), 0) + 1000 FROM issues WHERE parent_id = $2) END
                 WHERE id = $1",
            )
            .bind(id)
            .bind(parent)
            .execute(&mut *tx)
            .await?;
            let refs: (Option<Value>, Option<Value>) = sqlx::query_as("SELECT issue_ref($1), issue_ref($2)")
                .bind(row.parent_id)
                .bind(parent)
                .fetch_one(&mut *tx)
                .await?;
            changes.insert("parent".into(), json!({ "from": refs.0, "to": refs.1 }));
        }
    }
    if let Some(value) = &args.due_date {
        let due = value.as_deref().filter(|d| !is_null_word(d)).map(parse_date).transpose()?;
        if due != row.due_date {
            sqlx::query("UPDATE issues SET due_date = $2 WHERE id = $1").bind(id).bind(due).execute(&mut *tx).await?;
            changes.insert("dueDate".into(), json!({ "from": row.due_date, "to": due }));
        }
    }
    if let Some(value) = &args.snoozed_until {
        let until = value.as_deref().map(parse_datetime).transpose()?;
        sqlx::query("UPDATE issues SET snoozed_until = $2, snoozed_by_id = CASE WHEN $2 IS NULL THEN NULL ELSE $3 END WHERE id = $1")
            .bind(id)
            .bind(until)
            .bind(ctx.user)
            .execute(&mut *tx)
            .await?;
        changes.insert("snoozedUntil".into(), json!({ "from": row.snoozed_until, "to": until }));
    }
    if let Some(value) = &args.sla_breaches_at {
        let breaches = value.as_deref().map(parse_datetime).transpose()?;
        let sla_type = match args.sla_type.as_deref() {
            None | Some("all") => "all",
            Some("onlyBusinessDays") => "onlyBusinessDays",
            Some(other) => {
                return Err(ApiError::bad_request(format!(
                    "slaType is \"all\" or \"onlyBusinessDays\", not \"{other}\""
                )));
            }
        };
        sqlx::query(
            "UPDATE issues SET sla_breaches_at = $2, sla_type = CASE WHEN $2 IS NULL THEN NULL ELSE $3 END,
                sla_started_at = CASE WHEN $2 IS NULL THEN NULL ELSE coalesce(sla_started_at, now()) END WHERE id = $1",
        )
        .bind(id)
        .bind(breaches)
        .bind(sla_type)
        .execute(&mut *tx)
        .await?;
        changes.insert("slaBreachesAt".into(), json!({ "from": row.sla_breaches_at, "to": breaches }));
    } else if args.sla_type.is_some() {
        return Err(ApiError::bad_request("Set slaType together with slaBreachesAt"));
    }
    if let Some(order) = args.sort_order {
        sqlx::query("UPDATE issues SET sort_order = $2 WHERE id = $1").bind(id).bind(order).execute(&mut *tx).await?;
    }
    if let Some(order) = args.sub_issue_sort_order {
        sqlx::query("UPDATE issues SET sub_issue_sort_order = $2 WHERE id = $1")
            .bind(id)
            .bind(order)
            .execute(&mut *tx)
            .await?;
    }

    relations(ctx, tx, id, &args, &mut changes).await?;

    if let Some(links) = &args.links {
        for link in links {
            super::attachments::upsert(ctx, tx, id, &link.url, &link.title, None, None).await?;
        }
    }
    if let Some(people) = &args.add_subscribers {
        let users = resolve::users(ctx, people).await?;
        events::subscribe(tx, id, &users).await?;
    }
    if let Some(people) = &args.remove_subscribers {
        let users = resolve::users(ctx, people).await?;
        sqlx::query("DELETE FROM issue_subscribers WHERE issue_id = $1 AND user_id = ANY($2)")
            .bind(id)
            .bind(&users)
            .execute(&mut *tx)
            .await?;
    }
    releases(ctx, tx, id, &args).await?;

    if !changes.is_empty() && !creating {
        events::history(tx, ctx, id, "updated", Value::Object(changes)).await?;
    }
    Ok(())
}

/// Guests and people outside a private team can't be assigned unless the issue is shared with them.
async fn check_can_see_team(ctx: &Ctx, tx: &mut PgConnection, user: Uuid, team: Uuid, issue: Uuid) -> ApiResult<()> {
    let ok: bool = sqlx::query_scalar(
        "SELECT EXISTS (SELECT 1 FROM teams t JOIN workspace_members m ON m.workspace_id = t.workspace_id AND m.user_id = $2
            WHERE t.id = $1 AND ((NOT t.private AND m.role <> 'guest')
                OR EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id = t.id AND tm.user_id = $2)))
         OR EXISTS (SELECT 1 FROM issue_shares s WHERE s.issue_id = $3 AND s.user_id = $2)",
    )
    .bind(team)
    .bind(user)
    .bind(issue)
    .fetch_one(&mut *tx)
    .await?;
    if !ok {
        let _ = ctx;
        return Err(ApiError::bad_request(
            "That person can't see this team's issues; add them to the team or share the issue first",
        ));
    }
    Ok(())
}

async fn labels(
    ctx: &Ctx,
    tx: &mut PgConnection,
    row: &IssueRow,
    args: &SaveIssueArgs,
    changes: &mut serde_json::Map<String, Value>,
) -> ApiResult<()> {
    if args.labels.is_some() && (args.add_labels.is_some() || args.remove_labels.is_some()) {
        return Err(ApiError::bad_request("Pass either labels or addLabels/removeLabels, not both"));
    }
    if args.labels.is_none() && args.add_labels.is_none() && args.remove_labels.is_none() {
        return Ok(());
    }
    let current: Vec<Uuid> = sqlx::query_scalar("SELECT label_id FROM issue_label_links WHERE issue_id = $1")
        .bind(row.id)
        .fetch_all(&mut *tx)
        .await?;
    let mut next = match &args.labels {
        Some(all) => resolve::labels(ctx, Some(row.team_id), all).await?,
        None => current.clone(),
    };
    if let Some(add) = &args.add_labels {
        let added = resolve::labels(ctx, Some(row.team_id), add).await?;
        // A label from a single-select group replaces its sibling.
        let groups: Vec<(Uuid, Option<Uuid>, String)> = sqlx::query_as(
            "SELECT l.id, l.parent_id, coalesce(p.group_type, 'multiSelect') FROM issue_labels l
             LEFT JOIN issue_labels p ON p.id = l.parent_id WHERE l.id = ANY($1)",
        )
        .bind(&added)
        .fetch_all(&mut *tx)
        .await?;
        for (label, parent, kind) in groups {
            if let (Some(parent), "singleSelect") = (parent, kind.as_str()) {
                let siblings: Vec<Uuid> =
                    sqlx::query_scalar("SELECT id FROM issue_labels WHERE parent_id = $1 AND id <> $2")
                        .bind(parent)
                        .bind(label)
                        .fetch_all(&mut *tx)
                        .await?;
                next.retain(|l| !siblings.contains(l));
            }
            if !next.contains(&label) {
                next.push(label);
            }
        }
    }
    if let Some(remove) = &args.remove_labels {
        let removed = resolve::labels(ctx, Some(row.team_id), remove).await?;
        next.retain(|l| !removed.contains(l));
    }
    let clashes: Vec<String> = sqlx::query_scalar(
        "SELECT p.name FROM issue_labels l JOIN issue_labels p ON p.id = l.parent_id
         WHERE l.id = ANY($1) AND p.group_type = 'singleSelect' GROUP BY p.name HAVING count(*) > 1",
    )
    .bind(&next)
    .fetch_all(&mut *tx)
    .await?;
    if let Some(group) = clashes.first() {
        return Err(ApiError::bad_request(format!("An issue takes one label of the group \"{group}\"")));
    }
    let mut sorted_current = current.clone();
    sorted_current.sort();
    let mut sorted_next = next.clone();
    sorted_next.sort();
    if sorted_current == sorted_next {
        return Ok(());
    }
    sqlx::query("DELETE FROM issue_label_links WHERE issue_id = $1 AND NOT (label_id = ANY($2))")
        .bind(row.id)
        .bind(&next)
        .execute(&mut *tx)
        .await?;
    sqlx::query(
        "INSERT INTO issue_label_links (issue_id, label_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING",
    )
    .bind(row.id)
    .bind(&next)
    .execute(&mut *tx)
    .await?;
    let names = |ids: Vec<Uuid>| async move {
        sqlx::query_scalar::<_, String>("SELECT name FROM issue_labels WHERE id = ANY($1) ORDER BY name")
            .bind(ids)
            .fetch_all(ctx.db())
            .await
    };
    changes.insert("labels".into(), json!({ "from": names(current).await?, "to": names(next).await? }));
    Ok(())
}

async fn relations(
    ctx: &Ctx,
    tx: &mut PgConnection,
    id: Uuid,
    args: &SaveIssueArgs,
    changes: &mut serde_json::Map<String, Value>,
) -> ApiResult<()> {
    let add =
        [(&args.blocks, "blocks", false), (&args.blocked_by, "blocks", true), (&args.related_to, "related", false)];
    for (list, kind, reverse) in add {
        for other in list.iter().flatten() {
            let other = resolve::issue(ctx, other).await?;
            if other == id {
                return Err(ApiError::bad_request("An issue can't be related to itself"));
            }
            let (from, to) = if reverse { (other, id) } else { (id, other) };
            if kind == "related" {
                let exists: bool = sqlx::query_scalar(
                    "SELECT EXISTS (SELECT 1 FROM issue_relations WHERE type = 'related'
                        AND ((issue_id = $1 AND related_issue_id = $2) OR (issue_id = $2 AND related_issue_id = $1)))",
                )
                .bind(from)
                .bind(to)
                .fetch_one(&mut *tx)
                .await?;
                if exists {
                    continue;
                }
            }
            sqlx::query("INSERT INTO issue_relations (issue_id, related_issue_id, type, creator_id) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING")
                .bind(from)
                .bind(to)
                .bind(kind)
                .bind(ctx.user)
                .execute(&mut *tx)
                .await?;
            let label = if reverse {
                "blockedBy"
            } else if kind == "blocks" {
                "blocks"
            } else {
                "relatedTo"
            };
            let other_identifier = identifier(tx, other).await?;
            changes
                .entry(format!("added_{label}"))
                .or_insert_with(|| json!([]))
                .as_array_mut()
                .expect("array")
                .push(json!(other_identifier));
        }
    }
    let remove = [
        (&args.remove_blocks, "blocks", false),
        (&args.remove_blocked_by, "blocks", true),
        (&args.remove_related_to, "related", false),
    ];
    for (list, kind, reverse) in remove {
        for other in list.iter().flatten() {
            let other = resolve::issue(ctx, other).await?;
            let (from, to) = if reverse { (other, id) } else { (id, other) };
            let symmetric = kind == "related";
            sqlx::query(
                "DELETE FROM issue_relations WHERE type = $3 AND ((issue_id = $1 AND related_issue_id = $2)
                    OR ($4 AND issue_id = $2 AND related_issue_id = $1))",
            )
            .bind(from)
            .bind(to)
            .bind(kind)
            .bind(symmetric)
            .execute(&mut *tx)
            .await?;
        }
    }
    if let Some(duplicate_of) = &args.duplicate_of {
        sqlx::query("DELETE FROM issue_relations WHERE issue_id = $1 AND type = 'duplicate'")
            .bind(id)
            .execute(&mut *tx)
            .await?;
        if let Some(other) = duplicate_of.as_deref().filter(|d| !is_null_word(d)) {
            let other = resolve::issue(ctx, other).await?;
            if other == id {
                return Err(ApiError::bad_request("An issue can't duplicate itself"));
            }
            sqlx::query("INSERT INTO issue_relations (issue_id, related_issue_id, type, creator_id) VALUES ($1, $2, 'duplicate', $3)")
                .bind(id)
                .bind(other)
                .bind(ctx.user)
                .execute(&mut *tx)
                .await?;
            let row = load_row(tx, id).await?;
            let closed: bool =
                sqlx::query_scalar("SELECT type IN ('completed', 'canceled') FROM workflow_states WHERE id = $1")
                    .bind(row.state_id)
                    .fetch_one(&mut *tx)
                    .await?;
            if !closed && let Some(canceled) = first_state_of_type(tx, row.team_id, "canceled").await? {
                set_state(ctx, tx, id, canceled, "marked_duplicate").await?;
            }
            changes.insert("duplicateOf".into(), json!(identifier(tx, other).await?));
        }
    }
    Ok(())
}

async fn releases(ctx: &Ctx, tx: &mut PgConnection, id: Uuid, args: &SaveIssueArgs) -> ApiResult<()> {
    if args.set_releases.is_some() && (args.add_releases.is_some() || args.remove_releases.is_some()) {
        return Err(ApiError::bad_request("Pass either setReleases or addReleases/removeReleases, not both"));
    }
    if let Some(all) = &args.set_releases {
        let ids = resolve::releases(ctx, all).await?;
        sqlx::query("DELETE FROM issue_releases WHERE issue_id = $1").bind(id).execute(&mut *tx).await?;
        sqlx::query(
            "INSERT INTO issue_releases (issue_id, release_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING",
        )
        .bind(id)
        .bind(&ids)
        .execute(&mut *tx)
        .await?;
    }
    if let Some(add) = &args.add_releases {
        let ids = resolve::releases(ctx, add).await?;
        sqlx::query(
            "INSERT INTO issue_releases (issue_id, release_id) SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING",
        )
        .bind(id)
        .bind(&ids)
        .execute(&mut *tx)
        .await?;
    }
    if let Some(remove) = &args.remove_releases {
        let ids = resolve::releases(ctx, remove).await?;
        sqlx::query("DELETE FROM issue_releases WHERE issue_id = $1 AND release_id = ANY($2)")
            .bind(id)
            .bind(&ids)
            .execute(&mut *tx)
            .await?;
    }
    Ok(())
}

/* ------------------------------------------------------------------ */
/*                     Trash, archive and sharing                     */
/* ------------------------------------------------------------------ */

async fn mark(ctx: &Ctx, reference: &str, sql: &str, action: &str, webhook_action: &str) -> ApiResult<Value> {
    let id = resolve::issue(ctx, reference).await?;
    let mut tx = ctx.db().begin().await?;
    let row = load_row(&mut tx, id).await?;
    let affected: Vec<Uuid> = sqlx::query_scalar(sql).bind(id).fetch_all(&mut *tx).await?;
    for issue in &affected {
        events::history(&mut tx, ctx, *issue, action, json!({})).await?;
    }
    let value = issue_value(ctx, &mut tx, id).await?;
    events::emit(&mut tx, ctx, "Issue", webhook_action, &value, None, Some(row.team_id)).await?;
    tx.commit().await?;
    Ok(value)
}

const WITH_SUB_ISSUES: &str = "WITH RECURSIVE tree AS (SELECT id FROM issues WHERE id = $1
    UNION SELECT i.id FROM issues i JOIN tree ON i.parent_id = tree.id)";

async fn trash(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let sql = format!(
        "{WITH_SUB_ISSUES} UPDATE issues SET trashed_at = now(), updated_at = now() WHERE id IN (SELECT id FROM tree) AND trashed_at IS NULL RETURNING id"
    );
    mark(&ctx, &args.id, &sql, "trashed", "remove").await
}

async fn restore(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let sql = format!(
        "{WITH_SUB_ISSUES} UPDATE issues SET trashed_at = NULL, updated_at = now() WHERE id IN (SELECT id FROM tree) AND trashed_at IS NOT NULL RETURNING id"
    );
    mark(&ctx, &args.id, &sql, "restored", "update").await
}

async fn archive(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let sql = format!(
        "{WITH_SUB_ISSUES} UPDATE issues SET archived_at = now(), updated_at = now() WHERE id IN (SELECT id FROM tree) AND archived_at IS NULL RETURNING id"
    );
    mark(&ctx, &args.id, &sql, "archived", "update").await
}

async fn unarchive(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let sql = "UPDATE issues SET archived_at = NULL, auto_archived_at = NULL, updated_at = now() WHERE id = $1 AND archived_at IS NOT NULL RETURNING id";
    mark(&ctx, &args.id, sql, "unarchived", "update").await
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ShareArgs {
    /// Identifier (ENG-123) or id of the issue.
    id: String,
    /// Workspace member to share with: id, name, email or "me".
    user: String,
}

async fn share(ctx: Ctx, args: ShareArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let id = resolve::issue(&ctx, &args.id).await?;
    let user = resolve::user(&ctx, &args.user).await?;
    let mut tx = ctx.db().begin().await?;
    sqlx::query("INSERT INTO issue_shares (issue_id, user_id, sharer_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING")
        .bind(id)
        .bind(user)
        .bind(ctx.user)
        .execute(&mut *tx)
        .await?;
    events::subscribe(&mut tx, id, &[user]).await?;
    let identifier = identifier(&mut tx, id).await?;
    let notice = Notice {
        kind: "issueShared",
        summary: format!("{identifier} was shared with you"),
        issue: Some(id),
        ..Default::default()
    };
    events::notify(&mut tx, &ctx, &[user], &notice).await?;
    events::history(&mut tx, &ctx, id, "shared", json!({ "user": user })).await?;
    tx.commit().await?;
    Ok(json!({ "shared": identifier, "with": user }))
}

async fn unshare(ctx: Ctx, args: ShareArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let id = resolve::issue(&ctx, &args.id).await?;
    let user = resolve::user(&ctx, &args.user).await?;
    sqlx::query("DELETE FROM issue_shares WHERE issue_id = $1 AND user_id = $2")
        .bind(id)
        .bind(user)
        .execute(ctx.db())
        .await?;
    Ok(json!({ "unshared": args.id, "with": user }))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn estimate_scales() {
        assert_eq!(allowed_estimates("fibonacci", false, false), vec![1.0, 2.0, 3.0, 5.0, 8.0]);
        assert_eq!(allowed_estimates("exponential", true, true), vec![0.0, 1.0, 2.0, 4.0, 8.0, 16.0, 32.0, 64.0]);
        assert!(allowed_estimates("notUsed", true, true) == vec![0.0]);
    }

    #[test]
    fn updated_from_lists_previous_values() {
        let before = json!({ "title": "a", "priority": 1, "updatedAt": "x" });
        let after = json!({ "title": "b", "priority": 1, "updatedAt": "y" });
        assert_eq!(changed_fields(&before, &after), json!({ "title": "a" }));
    }
}
