//! Full-text search across issues, projects, documents, initiatives and comments, limited to
//! what the caller can see.

use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Value, json};
use std::cmp::Ordering;
use uuid::Uuid;

use super::filter::escape_like;
use super::resolve;
use super::util::non_empty;
use super::{Op, op};
use crate::ctx::{Access, Ctx};
use crate::error::{ApiError, ApiResult};

pub fn ops() -> Vec<Op> {
    vec![op(
        "search",
        Access::Read,
        &[("GET", "/search")],
        "Search issues, projects, documents and initiatives (and comments, when asked for) by words, best matches \
         first, titles before text. An issue identifier like ENG-12 finds that issue. Each result has its type, title, \
         link and a snippet of the matching text with matches in **bold**.",
        search,
    )]
}

const TYPES: [&str; 5] = ["issue", "project", "document", "initiative", "comment"];

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SearchArgs {
    /// Words to find; "quoted phrases", OR and -excluded words work too.
    query: String,
    /// What to search: issue, project, document, initiative, comment. All but comment by default.
    types: Option<Vec<String>>,
    /// Only things of this team (key, name or id).
    team: Option<String>,
    /// Also archived issues, projects and initiatives.
    #[serde(default)]
    include_archived: bool,
    /// How many results, 20 by default, at most 50.
    limit: Option<i64>,
}

/// Every query starts with this, so the arguments are bound the same way for all of them.
const ARGS: &str = "WITH a AS (SELECT $1::uuid AS ws, $2::uuid[] AS teams, $3::uuid AS me, $4::uuid AS team,
    $5::bool AS archived, $6::text AS base, $7::text AS pattern, $8::text AS key, $9::int AS num,
    websearch_to_tsquery('english', $10) || websearch_to_tsquery('simple', $10) AS ts)";

const HEADLINE: &str = "'MaxFragments=1, MaxWords=30, MinWords=10, StartSel=**, StopSel=**'";

fn slug(name: &str) -> String {
    format!("trim(both '-' from left(trim(both '-' from regexp_replace(lower({name}), '[^a-z0-9]+', '-', 'g')), 60))")
}

fn issue_visible(i: &str) -> String {
    format!(
        "({i}.workspace_id = a.ws AND {i}.trashed_at IS NULL AND ({i}.team_id = ANY(a.teams)
            OR EXISTS (SELECT 1 FROM issue_shares sh WHERE sh.issue_id = {i}.id AND sh.user_id = a.me)))"
    )
}

fn project_visible(p: &str) -> String {
    format!(
        "({p}.trashed_at IS NULL AND EXISTS (SELECT 1 FROM project_teams pt WHERE pt.project_id = {p}.id AND pt.team_id = ANY(a.teams)))"
    )
}

fn issue_sql() -> String {
    format!(
        "SELECT jsonb_build_object('type', 'issue', 'id', i.id, 'title', i.title, 'identifier', t.key || '-' || i.number,
            'url', a.base || '/' || w.url_key || '/issue/' || t.key || '-' || i.number,
            'snippet', left(ts_headline('english', coalesce(nullif(i.description, ''), i.title), a.ts, {HEADLINE}), 200),
            'rank', ts_rank(i.search, a.ts), 'state', state_ref(i.state_id), 'team', team_ref(i.team_id),
            'updatedAt', i.updated_at) AS v,
            coalesce(upper(t.key) = a.key AND i.number = a.num, false) AS exact,
            (i.title ILIKE a.pattern OR to_tsvector('simple', i.title) @@ a.ts) AS title_match
         FROM a, issues i JOIN teams t ON t.id = i.team_id JOIN workspaces w ON w.id = i.workspace_id
         WHERE {} AND (a.archived OR i.archived_at IS NULL) AND (a.team IS NULL OR i.team_id = a.team)
         AND (i.search @@ a.ts OR i.title ILIKE a.pattern OR (upper(t.key) = a.key AND i.number = a.num))",
        issue_visible("i")
    )
}

fn project_sql() -> String {
    format!(
        "SELECT jsonb_build_object('type', 'project', 'id', p.id, 'title', p.name,
            'url', a.base || '/' || w.url_key || '/project/' || {} || '-' || p.slug_id,
            'snippet', left(ts_headline('english', coalesce(nullif(p.content, ''), nullif(p.summary, ''), p.name), a.ts, {HEADLINE}), 200),
            'rank', ts_rank(p.search, a.ts), 'status', project_status_ref(p.status_id), 'updatedAt', p.updated_at) AS v,
            false AS exact, (p.name ILIKE a.pattern OR to_tsvector('simple', p.name) @@ a.ts) AS title_match
         FROM a, projects p JOIN workspaces w ON w.id = p.workspace_id
         WHERE p.workspace_id = a.ws AND {} AND (a.archived OR p.archived_at IS NULL)
         AND (a.team IS NULL OR EXISTS (SELECT 1 FROM project_teams pt WHERE pt.project_id = p.id AND pt.team_id = a.team))
         AND (p.search @@ a.ts OR p.name ILIKE a.pattern)",
        slug("p.name"),
        project_visible("p")
    )
}

fn document_sql() -> String {
    format!(
        "SELECT jsonb_build_object('type', 'document', 'id', d.id, 'title', d.title,
            'url', a.base || '/' || w.url_key || '/document/' || {} || '-' || d.slug_id,
            'snippet', left(ts_headline('english', coalesce(nullif(d.content, ''), d.title), a.ts, {HEADLINE}), 200),
            'rank', ts_rank(d.search, a.ts), 'project', project_ref(d.project_id), 'updatedAt', d.updated_at) AS v,
            false AS exact, (d.title ILIKE a.pattern OR to_tsvector('simple', d.title) @@ a.ts) AS title_match
         FROM a, documents d JOIN workspaces w ON w.id = d.workspace_id
         WHERE d.workspace_id = a.ws AND d.trashed_at IS NULL
         AND CASE WHEN d.team_id IS NOT NULL THEN d.team_id = ANY(a.teams)
             WHEN d.project_id IS NOT NULL THEN EXISTS (SELECT 1 FROM projects p WHERE p.id = d.project_id AND {})
             WHEN d.issue_id IS NOT NULL THEN EXISTS (SELECT 1 FROM issues i WHERE i.id = d.issue_id AND {})
             WHEN d.cycle_id IS NOT NULL THEN EXISTS (SELECT 1 FROM cycles c WHERE c.id = d.cycle_id AND c.team_id = ANY(a.teams))
             ELSE true END
         AND (a.team IS NULL OR d.team_id = a.team
             OR EXISTS (SELECT 1 FROM project_teams pt WHERE pt.project_id = d.project_id AND pt.team_id = a.team)
             OR EXISTS (SELECT 1 FROM issues i WHERE i.id = d.issue_id AND i.team_id = a.team))
         AND (d.search @@ a.ts OR d.title ILIKE a.pattern)",
        slug("d.title"),
        project_visible("p"),
        issue_visible("i")
    )
}

fn initiative_sql() -> String {
    let vector = "(setweight(to_tsvector('simple', n.name), 'A') ||
        setweight(to_tsvector('english', coalesce(n.summary, '') || ' ' || n.content), 'B'))";
    format!(
        "SELECT jsonb_build_object('type', 'initiative', 'id', n.id, 'title', n.name,
            'url', a.base || '/' || w.url_key || '/initiative/' || {} || '-' || n.slug_id,
            'snippet', left(ts_headline('english', coalesce(nullif(n.content, ''), nullif(n.summary, ''), n.name), a.ts, {HEADLINE}), 200),
            'rank', ts_rank({vector}, a.ts), 'status', n.status, 'updatedAt', n.updated_at) AS v,
            false AS exact, (n.name ILIKE a.pattern OR to_tsvector('simple', n.name) @@ a.ts) AS title_match
         FROM a, initiatives n JOIN workspaces w ON w.id = n.workspace_id
         WHERE n.workspace_id = a.ws AND n.trashed_at IS NULL AND (a.archived OR n.archived_at IS NULL)
         AND (a.team IS NULL OR EXISTS (SELECT 1 FROM initiative_projects ip JOIN project_teams pt ON pt.project_id = ip.project_id
             WHERE ip.initiative_id = n.id AND pt.team_id = a.team))
         AND ({vector} @@ a.ts OR n.name ILIKE a.pattern)",
        slug("n.name")
    )
}

fn comment_sql() -> String {
    format!(
        "SELECT jsonb_build_object('type', 'comment', 'id', c.id, 'title', i.title, 'identifier', t.key || '-' || i.number,
            'url', a.base || '/' || w.url_key || '/issue/' || t.key || '-' || i.number || '#comment-' || c.id,
            'snippet', left(ts_headline('english', c.body, a.ts, {HEADLINE}), 200),
            'rank', ts_rank(c.search, a.ts), 'issue', issue_ref(i.id), 'user', user_ref(c.user_id), 'updatedAt', c.updated_at) AS v,
            false AS exact, false AS title_match
         FROM a, comments c JOIN issues i ON i.id = c.issue_id JOIN teams t ON t.id = i.team_id
            JOIN workspaces w ON w.id = i.workspace_id
         WHERE {} AND (a.archived OR i.archived_at IS NULL) AND (a.team IS NULL OR i.team_id = a.team)
         AND (c.search @@ a.ts OR c.body ILIKE a.pattern)",
        issue_visible("i")
    )
}

fn parse_types(types: Option<Vec<String>>) -> ApiResult<Vec<&'static str>> {
    let Some(types) = types.filter(|t| !t.is_empty()) else { return Ok(TYPES[..4].to_vec()) };
    let mut chosen = Vec::new();
    for kind in &types {
        let lower = kind.trim().to_lowercase();
        let found = TYPES.iter().find(|t| **t == lower || format!("{t}s") == lower).ok_or_else(|| {
            ApiError::bad_request(format!("\"{kind}\" isn't a type to search; use {}", TYPES.join(", ")))
        })?;
        if !chosen.contains(found) {
            chosen.push(*found);
        }
    }
    Ok(chosen)
}

async fn search(ctx: Ctx, args: SearchArgs) -> ApiResult<Value> {
    let query = non_empty(&args.query, "query")?;
    let types = parse_types(args.types)?;
    let team: Option<Uuid> = match &args.team {
        Some(team) => Some(resolve::team(&ctx, team).await?),
        None => None,
    };
    let limit = args.limit.unwrap_or(20).clamp(1, 50);
    let teams = ctx.team_ids().await?.to_vec();
    let identifier = resolve::parse_identifier(&query);
    let mut found: Vec<(Value, bool, bool)> = Vec::new();
    for kind in types {
        let sql = match kind {
            "issue" => issue_sql(),
            "project" => project_sql(),
            "document" => document_sql(),
            "initiative" => initiative_sql(),
            _ => comment_sql(),
        };
        let sql = format!(
            "{ARGS} SELECT v, exact, title_match FROM ({sql}) s
             ORDER BY exact DESC, title_match DESC, (v->>'rank')::float8 DESC LIMIT $11"
        );
        let rows: Vec<(Value, bool, bool)> = sqlx::query_as(&sql)
            .bind(ctx.workspace)
            .bind(&teams)
            .bind(ctx.user)
            .bind(team)
            .bind(args.include_archived)
            .bind(ctx.public_url())
            .bind(format!("%{}%", escape_like(&query)))
            .bind(identifier.as_ref().map(|(key, _)| key.clone()))
            .bind(identifier.as_ref().map(|(_, number)| *number))
            .bind(&query)
            .bind(limit)
            .fetch_all(ctx.db())
            .await?;
        found.extend(rows);
    }
    let rank = |v: &Value| v["rank"].as_f64().unwrap_or(0.0);
    found.sort_by(|a, b| {
        (b.1, b.2)
            .cmp(&(a.1, a.2))
            .then_with(|| rank(&b.0).partial_cmp(&rank(&a.0)).unwrap_or(Ordering::Equal))
            .then_with(|| b.0["updatedAt"].as_str().cmp(&a.0["updatedAt"].as_str()))
    });
    let nodes: Vec<Value> = found.into_iter().take(limit as usize).map(|(value, _, _)| value).collect();
    Ok(json!({ "nodes": nodes }))
}
