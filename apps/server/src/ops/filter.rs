//! The issue filter language, after Linear's `IssueFilter`: an object of fields with comparators,
//! combined with `and` / `or`, e.g.
//! `{"state": {"type": {"in": ["started", "unstarted"]}}, "assignee": {"isMe": {"eq": true}}}`.
//! Custom views store their filter in this form too. It compiles to SQL with bound values.

use serde_json::{Map, Value};
use sqlx::{Postgres, QueryBuilder};
use uuid::Uuid;

use super::resolve::as_uuid;
use super::util::parse_datetime;
use crate::ctx::Ctx;
use crate::error::{ApiError, ApiResult};

pub const FILTER_HELP: &str = "A filter object, as in Linear's IssueFilter. Fields: id, number, title, description, \
priority, estimate, dueDate, createdAt, updatedAt, startedAt, completedAt, canceledAt, archivedAt, triagedAt, \
snoozedUntil, slaBreachesAt, state {id,name,type}, team {id,key,name}, assignee / creator {id,email,name,displayName,isMe}, \
labels {some|every|none: {id,name,parent}}, project {id,name,slugId}, projectMilestone {id,name}, cycle {id,number,name,isActive}, \
parent {id,identifier}, subscribers {some: user}, hasBlockedByRelations, hasBlockingRelations, hasRelatedRelations, \
hasDuplicateRelations, children {some: filter}, searchableContent {contains}, and, or. Comparators: eq, neq, in, nin, \
lt, lte, gt, gte, contains, notContains, startsWith, endsWith, null (true for empty). Dates take ISO 8601 or durations like \
-P2W. Text compares ignore case. Example: {\"or\": [{\"priority\": {\"lte\": 2, \"neq\": 0}}, {\"labels\": {\"some\": {\"name\": {\"eq\": \"Bug\"}}}}]}";

/// Appends `AND (<filter>)` for the issue at `alias`.
pub fn push(qb: &mut QueryBuilder<Postgres>, ctx: &Ctx, alias: &str, filter: &Value) -> ApiResult<()> {
    let object = as_object(filter, "filter")?;
    if object.is_empty() {
        return Ok(());
    }
    qb.push(" AND ");
    issue(qb, ctx, alias, object)
}

fn as_object<'a>(value: &'a Value, what: &str) -> ApiResult<&'a Map<String, Value>> {
    value.as_object().ok_or_else(|| ApiError::bad_request(format!("{what} must be an object")))
}

fn bad(field: &str, message: &str) -> ApiError {
    ApiError::bad_request(format!("Invalid filter on {field}: {message}"))
}

/// Every field of the object must hold; `and` and `or` nest.
fn issue(qb: &mut QueryBuilder<Postgres>, ctx: &Ctx, i: &str, filter: &Map<String, Value>) -> ApiResult<()> {
    qb.push("(TRUE");
    for (field, value) in filter {
        qb.push(" AND ");
        match field.as_str() {
            "and" | "or" => {
                let parts = value.as_array().ok_or_else(|| bad(field, "expected a list of filters"))?;
                let joiner = if field == "and" { " AND " } else { " OR " };
                qb.push("(");
                if parts.is_empty() {
                    qb.push("TRUE");
                }
                for (n, part) in parts.iter().enumerate() {
                    if n > 0 {
                        qb.push(joiner);
                    }
                    issue(qb, ctx, i, as_object(part, field)?)?;
                }
                qb.push(")");
            }
            "id" => ids(qb, &format!("{i}.id"), field, value)?,
            "number" => number(qb, &format!("{i}.number"), field, value)?,
            "title" => text(qb, &format!("{i}.title"), field, value)?,
            "description" => text(qb, &format!("{i}.description"), field, value)?,
            "priority" => number(qb, &format!("{i}.priority"), field, value)?,
            "estimate" => number(qb, &format!("{i}.estimate"), field, value)?,
            "dueDate" => date(qb, &format!("{i}.due_date"), field, value, true)?,
            "createdAt" | "updatedAt" | "startedAt" | "completedAt" | "canceledAt" | "archivedAt" | "triagedAt"
            | "snoozedUntil" | "slaBreachesAt" | "autoClosedAt" | "startedTriageAt" => {
                let column = snake(field);
                date(qb, &format!("{i}.{column}"), field, value, false)?
            }
            "state" => {
                let f = as_object(value, field)?;
                qb.push(format!("EXISTS (SELECT 1 FROM workflow_states s WHERE s.id = {i}.state_id AND "));
                fields(
                    qb,
                    field,
                    f,
                    &[("id", Kind::Id, "s.id"), ("name", Kind::Text, "s.name"), ("type", Kind::Text, "s.type")],
                )?;
                qb.push(")");
            }
            "team" => {
                let f = as_object(value, field)?;
                qb.push(format!("EXISTS (SELECT 1 FROM teams t2 WHERE t2.id = {i}.team_id AND "));
                fields(
                    qb,
                    field,
                    f,
                    &[("id", Kind::Id, "t2.id"), ("key", Kind::Text, "t2.key"), ("name", Kind::Text, "t2.name")],
                )?;
                qb.push(")");
            }
            "assignee" | "creator" | "snoozedBy" => {
                let column = match field.as_str() {
                    "assignee" => "assignee_id",
                    "creator" => "creator_id",
                    _ => "snoozed_by_id",
                };
                user(qb, ctx, &format!("{i}.{column}"), field, as_object(value, field)?)?
            }
            "subscribers" => {
                let f = as_object(value, field)?;
                let inner = f.get("some").ok_or_else(|| bad(field, "use {\"some\": {...user filter}}"))?;
                qb.push(format!("EXISTS (SELECT 1 FROM issue_subscribers sub WHERE sub.issue_id = {i}.id AND "));
                user(qb, ctx, "sub.user_id", field, as_object(inner, field)?)?;
                qb.push(")");
            }
            "labels" => labels(qb, i, field, as_object(value, field)?)?,
            "project" => {
                let f = as_object(value, field)?;
                nullable_ref(qb, &format!("{i}.project_id"), field, f, |qb, f| {
                    qb.push(format!("EXISTS (SELECT 1 FROM projects p2 WHERE p2.id = {i}.project_id AND "));
                    fields(
                        qb,
                        field,
                        f,
                        &[
                            ("id", Kind::Id, "p2.id"),
                            ("name", Kind::Text, "p2.name"),
                            ("slugId", Kind::Text, "p2.slug_id"),
                            ("priority", Kind::Number, "p2.priority"),
                        ],
                    )?;
                    qb.push(")");
                    Ok(())
                })?
            }
            "projectMilestone" | "milestone" => {
                let f = as_object(value, field)?;
                nullable_ref(qb, &format!("{i}.milestone_id"), field, f, |qb, f| {
                    qb.push(format!("EXISTS (SELECT 1 FROM project_milestones m2 WHERE m2.id = {i}.milestone_id AND "));
                    fields(qb, field, f, &[("id", Kind::Id, "m2.id"), ("name", Kind::Text, "m2.name")])?;
                    qb.push(")");
                    Ok(())
                })?
            }
            "cycle" => {
                let f = as_object(value, field)?;
                nullable_ref(qb, &format!("{i}.cycle_id"), field, f, |qb, f| {
                    qb.push(format!("EXISTS (SELECT 1 FROM cycles c2 WHERE c2.id = {i}.cycle_id AND "));
                    let mut rest = f.clone();
                    if let Some(active) = rest.remove("isActive") {
                        let want = bool_eq(&active).ok_or_else(|| bad(field, "isActive takes {\"eq\": true|false}"))?;
                        qb.push(if want {
                            "(c2.starts_at <= now() AND c2.ends_at > now()) AND "
                        } else {
                            "NOT (c2.starts_at <= now() AND c2.ends_at > now()) AND "
                        });
                    }
                    fields(
                        qb,
                        field,
                        &rest,
                        &[
                            ("id", Kind::Id, "c2.id"),
                            ("number", Kind::Number, "c2.number"),
                            ("name", Kind::Text, "c2.name"),
                        ],
                    )?;
                    qb.push(")");
                    Ok(())
                })?
            }
            "parent" => {
                let f = as_object(value, field)?;
                nullable_ref(qb, &format!("{i}.parent_id"), field, f, |qb, f| {
                    let mut rest = f.clone();
                    if let Some(identifier) = rest.remove("identifier") {
                        let v = identifier
                            .get("eq")
                            .and_then(Value::as_str)
                            .ok_or_else(|| bad(field, "identifier takes {\"eq\": \"ENG-1\"}"))?;
                        qb.push(format!("issue_identifier({i}.parent_id) = upper("));
                        qb.push_bind(v.to_string());
                        qb.push(") AND ");
                    }
                    fields(qb, field, &rest, &[("id", Kind::Id, &format!("{i}.parent_id"))])
                })?
            }
            "children" => {
                let f = as_object(value, field)?;
                let (mode, inner) = collection(field, f)?;
                let sub = format!(
                    "EXISTS (SELECT 1 FROM issues ch WHERE ch.parent_id = {i}.id AND ch.trashed_at IS NULL AND "
                );
                match mode {
                    "some" => {
                        qb.push(sub);
                        issue(qb, ctx, "ch", inner)?;
                        qb.push(")");
                    }
                    "none" => {
                        qb.push(format!("NOT {sub}"));
                        issue(qb, ctx, "ch", inner)?;
                        qb.push(")");
                    }
                    _ => {
                        qb.push(format!("NOT EXISTS (SELECT 1 FROM issues ch WHERE ch.parent_id = {i}.id AND ch.trashed_at IS NULL AND NOT "));
                        issue(qb, ctx, "ch", inner)?;
                        qb.push(")");
                    }
                }
            }
            "hasBlockedByRelations"
            | "hasBlockingRelations"
            | "hasRelatedRelations"
            | "hasDuplicateRelations"
            | "hasSuggestedRelatedIssues" => {
                let want = bool_eq(value).ok_or_else(|| bad(field, "takes {\"eq\": true|false}"))?;
                let exists = match field.as_str() {
                    "hasBlockedByRelations" => format!(
                        "EXISTS (SELECT 1 FROM issue_relations r WHERE r.related_issue_id = {i}.id AND r.type = 'blocks')"
                    ),
                    "hasBlockingRelations" => format!(
                        "EXISTS (SELECT 1 FROM issue_relations r WHERE r.issue_id = {i}.id AND r.type = 'blocks')"
                    ),
                    "hasDuplicateRelations" => format!(
                        "EXISTS (SELECT 1 FROM issue_relations r WHERE (r.issue_id = {i}.id OR r.related_issue_id = {i}.id) AND r.type = 'duplicate')"
                    ),
                    _ => format!(
                        "EXISTS (SELECT 1 FROM issue_relations r WHERE (r.issue_id = {i}.id OR r.related_issue_id = {i}.id) AND r.type = 'related')"
                    ),
                };
                qb.push(if want { exists } else { format!("NOT {exists}") });
            }
            "searchableContent" => {
                let f = as_object(value, field)?;
                let q = f
                    .get("contains")
                    .and_then(Value::as_str)
                    .ok_or_else(|| bad(field, "takes {\"contains\": \"text\"}"))?;
                push_search(qb, i, q);
            }
            "release" | "releases" => {
                let f = as_object(value, field)?;
                let inner = f.get("some").and_then(Value::as_object).unwrap_or(f);
                qb.push(format!("EXISTS (SELECT 1 FROM issue_releases ir JOIN releases r2 ON r2.id = ir.release_id WHERE ir.issue_id = {i}.id AND "));
                fields(
                    qb,
                    field,
                    inner,
                    &[
                        ("id", Kind::Id, "r2.id"),
                        ("name", Kind::Text, "r2.name"),
                        ("version", Kind::Text, "r2.version"),
                    ],
                )?;
                qb.push(")");
            }
            other => {
                return Err(ApiError::bad_request(format!("Unknown filter field \"{other}\". {FILTER_HELP}")));
            }
        }
    }
    qb.push(")");
    Ok(())
}

/// Matches the issue's title, description and comments against `query`, as words.
pub fn push_search(qb: &mut QueryBuilder<Postgres>, i: &str, query: &str) {
    qb.push(format!("({i}.search @@ websearch_to_tsquery('english', "));
    qb.push_bind(query.to_string());
    qb.push(format!(") OR {i}.title ILIKE "));
    qb.push_bind(format!("%{}%", escape_like(query)));
    qb.push(format!(" OR EXISTS (SELECT 1 FROM comments sc WHERE sc.issue_id = {i}.id AND sc.search @@ websearch_to_tsquery('english', "));
    qb.push_bind(query.to_string());
    qb.push(")))");
}

pub fn escape_like(value: &str) -> String {
    value.replace('\\', "\\\\").replace('%', "\\%").replace('_', "\\_")
}

fn snake(field: &str) -> String {
    let mut out = String::new();
    for c in field.chars() {
        if c.is_ascii_uppercase() {
            out.push('_');
            out.push(c.to_ascii_lowercase());
        } else {
            out.push(c);
        }
    }
    out
}

fn bool_eq(value: &Value) -> Option<bool> {
    value.as_bool().or_else(|| value.get("eq")?.as_bool())
}

/// `{"null": true}` (unset), `{"null": false}` (set), or the related thing's fields.
fn nullable_ref(
    qb: &mut QueryBuilder<Postgres>,
    column: &str,
    field: &str,
    f: &Map<String, Value>,
    rest: impl FnOnce(&mut QueryBuilder<Postgres>, &Map<String, Value>) -> ApiResult<()>,
) -> ApiResult<()> {
    let mut f = f.clone();
    let null = f.remove("null");
    qb.push("(TRUE");
    if let Some(null) = null {
        let null = null.as_bool().ok_or_else(|| bad(field, "null takes true or false"))?;
        qb.push(format!(" AND {column} IS {}NULL", if null { "" } else { "NOT " }));
    }
    if !f.is_empty() {
        qb.push(" AND ");
        rest(qb, &f)?;
    }
    qb.push(")");
    Ok(())
}

fn collection<'a>(field: &str, f: &'a Map<String, Value>) -> ApiResult<(&'static str, &'a Map<String, Value>)> {
    for mode in ["some", "every", "none"] {
        if let Some(inner) = f.get(mode) {
            return Ok((mode, as_object(inner, field)?));
        }
    }
    Ok(("some", f))
}

fn labels(qb: &mut QueryBuilder<Postgres>, i: &str, field: &str, f: &Map<String, Value>) -> ApiResult<()> {
    if let Some(null) = f.get("null") {
        let null = null.as_bool().ok_or_else(|| bad(field, "null takes true or false"))?;
        let exists = format!("EXISTS (SELECT 1 FROM issue_label_links ll WHERE ll.issue_id = {i}.id)");
        qb.push(if null { format!("NOT {exists}") } else { exists });
        return Ok(());
    }
    let (mode, inner) = collection(field, f)?;
    let label_fields = |qb: &mut QueryBuilder<Postgres>, inner: &Map<String, Value>| -> ApiResult<()> {
        let mut rest = inner.clone();
        if let Some(parent) = rest.remove("parent") {
            qb.push("EXISTS (SELECT 1 FROM issue_labels lp WHERE lp.id = lb.parent_id AND ");
            fields(
                qb,
                field,
                as_object(&parent, field)?,
                &[("id", Kind::Id, "lp.id"), ("name", Kind::Text, "lp.name")],
            )?;
            qb.push(") AND ");
        }
        fields(qb, field, &rest, &[("id", Kind::Id, "lb.id"), ("name", Kind::Text, "lb.name")])
    };
    let join = format!(
        "SELECT 1 FROM issue_label_links ll JOIN issue_labels lb ON lb.id = ll.label_id WHERE ll.issue_id = {i}.id AND "
    );
    match mode {
        "some" => qb.push(format!("EXISTS ({join}")),
        "none" => qb.push(format!("NOT EXISTS ({join}")),
        _ => qb.push(format!(
            "EXISTS (SELECT 1 FROM issue_label_links lx WHERE lx.issue_id = {i}.id) AND NOT EXISTS ({join} NOT "
        )),
    };
    if mode == "every" {
        qb.push("(");
        label_fields(qb, inner)?;
        qb.push(")");
    } else {
        label_fields(qb, inner)?;
    }
    qb.push(")");
    Ok(())
}

fn user(
    qb: &mut QueryBuilder<Postgres>,
    ctx: &Ctx,
    column: &str,
    field: &str,
    f: &Map<String, Value>,
) -> ApiResult<()> {
    let mut rest = f.clone();
    qb.push("(TRUE");
    if let Some(null) = rest.remove("null") {
        let null = null.as_bool().ok_or_else(|| bad(field, "null takes true or false"))?;
        qb.push(format!(" AND {column} IS {}NULL", if null { "" } else { "NOT " }));
    }
    if let Some(is_me) = rest.remove("isMe") {
        let want = bool_eq(&is_me).ok_or_else(|| bad(field, "isMe takes {\"eq\": true|false}"))?;
        qb.push(format!(" AND {column} IS {}DISTINCT FROM ", if want { "NOT " } else { "" }));
        qb.push_bind(ctx.user);
    }
    if !rest.is_empty() {
        // `{"id": {"eq": "me"}}` means the caller too.
        if let Some(Value::Object(id)) = rest.get_mut("id") {
            for (_, v) in id.iter_mut() {
                match v {
                    Value::String(s) if s.eq_ignore_ascii_case("me") => *s = ctx.user.to_string(),
                    Value::Array(items) => items.iter_mut().for_each(|item| {
                        if item.as_str().is_some_and(|s| s.eq_ignore_ascii_case("me")) {
                            *item = Value::String(ctx.user.to_string());
                        }
                    }),
                    _ => {}
                }
            }
        }
        qb.push(format!(" AND EXISTS (SELECT 1 FROM users uf WHERE uf.id = {column} AND "));
        fields(
            qb,
            field,
            &rest,
            &[
                ("id", Kind::Id, "uf.id"),
                ("email", Kind::Text, "uf.email"),
                ("name", Kind::Text, "uf.name"),
                ("displayName", Kind::Text, "uf.display_name"),
            ],
        )?;
        qb.push(")");
    }
    qb.push(")");
    Ok(())
}

#[derive(Clone, Copy)]
enum Kind {
    Id,
    Text,
    Number,
}

/// Each named field with its comparator; unknown fields are an error.
fn fields(
    qb: &mut QueryBuilder<Postgres>,
    parent: &str,
    f: &Map<String, Value>,
    known: &[(&str, Kind, &str)],
) -> ApiResult<()> {
    qb.push("(TRUE");
    for (name, value) in f {
        let Some((_, kind, column)) = known.iter().find(|(k, _, _)| k == name) else {
            let names: Vec<_> = known.iter().map(|(k, _, _)| *k).collect();
            return Err(bad(parent, &format!("unknown field \"{name}\"; use {}", names.join(", "))));
        };
        qb.push(" AND ");
        let field = format!("{parent}.{name}");
        match kind {
            Kind::Id => ids(qb, column, &field, value)?,
            Kind::Text => text(qb, column, &field, value)?,
            Kind::Number => number(qb, column, &field, value)?,
        }
    }
    qb.push(")");
    Ok(())
}

fn comparators<'a>(field: &str, value: &'a Value) -> ApiResult<Vec<(&'a str, &'a Value)>> {
    match value {
        Value::Object(map) => Ok(map.iter().map(|(k, v)| (k.as_str(), v)).collect()),
        Value::Array(_) => Ok(vec![("in", value)]),
        Value::Null => Err(bad(field, "use {\"null\": true} for unset values")),
        _ => Ok(vec![("eq", value)]),
    }
}

fn list<'a>(field: &str, value: &'a Value) -> ApiResult<&'a Vec<Value>> {
    value.as_array().ok_or_else(|| bad(field, "in / nin take a list"))
}

fn ids(qb: &mut QueryBuilder<Postgres>, column: &str, field: &str, value: &Value) -> ApiResult<()> {
    let parse =
        |v: &Value| -> ApiResult<Uuid> { v.as_str().and_then(as_uuid).ok_or_else(|| bad(field, "ids must be UUIDs")) };
    qb.push("(TRUE");
    for (op, v) in comparators(field, value)? {
        qb.push(" AND ");
        match op {
            "eq" => {
                qb.push(format!("{column} = "));
                qb.push_bind(parse(v)?);
            }
            "neq" => {
                qb.push(format!("{column} IS DISTINCT FROM "));
                qb.push_bind(parse(v)?);
            }
            "in" | "nin" => {
                let values = list(field, v)?.iter().map(parse).collect::<ApiResult<Vec<_>>>()?;
                qb.push(format!("{}{column} = ANY(", if op == "nin" { "NOT " } else { "" }));
                qb.push_bind(values);
                qb.push(")");
            }
            "null" => {
                let null = v.as_bool().ok_or_else(|| bad(field, "null takes true or false"))?;
                qb.push(format!("{column} IS {}NULL", if null { "" } else { "NOT " }));
            }
            other => return Err(bad(field, &format!("ids compare with eq, neq, in, nin or null, not {other}"))),
        }
    }
    qb.push(")");
    Ok(())
}

fn text(qb: &mut QueryBuilder<Postgres>, column: &str, field: &str, value: &Value) -> ApiResult<()> {
    let string =
        |v: &Value| -> ApiResult<String> { v.as_str().map(str::to_string).ok_or_else(|| bad(field, "expected text")) };
    qb.push("(TRUE");
    for (op, v) in comparators(field, value)? {
        qb.push(" AND ");
        let like = |qb: &mut QueryBuilder<Postgres>, negate: bool, pattern: String| {
            qb.push(format!("{column} {}ILIKE ", if negate { "NOT " } else { "" }));
            qb.push_bind(pattern);
        };
        match op {
            "eq" | "eqIgnoreCase" => {
                qb.push(format!("lower({column}) = lower("));
                qb.push_bind(string(v)?);
                qb.push(")");
            }
            "neq" | "neqIgnoreCase" => {
                qb.push(format!("lower({column}) IS DISTINCT FROM lower("));
                qb.push_bind(string(v)?);
                qb.push(")");
            }
            "in" | "nin" => {
                let values = list(field, v)?
                    .iter()
                    .map(|v| string(v).map(|s| s.to_lowercase()))
                    .collect::<ApiResult<Vec<_>>>()?;
                qb.push(format!("{}coalesce(lower({column}), '') = ANY(", if op == "nin" { "NOT " } else { "" }));
                qb.push_bind(values);
                qb.push(")");
            }
            "contains" | "containsIgnoreCase" => like(qb, false, format!("%{}%", escape_like(&string(v)?))),
            "notContains" | "notContainsIgnoreCase" => like(qb, true, format!("%{}%", escape_like(&string(v)?))),
            "startsWith" | "startsWithIgnoreCase" => like(qb, false, format!("{}%", escape_like(&string(v)?))),
            "notStartsWith" => like(qb, true, format!("{}%", escape_like(&string(v)?))),
            "endsWith" => like(qb, false, format!("%{}", escape_like(&string(v)?))),
            "notEndsWith" => like(qb, true, format!("%{}", escape_like(&string(v)?))),
            "null" => {
                let null = v.as_bool().ok_or_else(|| bad(field, "null takes true or false"))?;
                qb.push(if null {
                    format!("coalesce({column}, '') = ''")
                } else {
                    format!("coalesce({column}, '') <> ''")
                });
            }
            other => return Err(bad(field, &format!("unknown comparator {other}"))),
        }
    }
    qb.push(")");
    Ok(())
}

fn number(qb: &mut QueryBuilder<Postgres>, column: &str, field: &str, value: &Value) -> ApiResult<()> {
    let num = |v: &Value| -> ApiResult<f64> { v.as_f64().ok_or_else(|| bad(field, "expected a number")) };
    qb.push("(TRUE");
    for (op, v) in comparators(field, value)? {
        qb.push(" AND ");
        let sql_op = match op {
            "eq" => "=",
            "neq" => "IS DISTINCT FROM",
            "lt" => "<",
            "lte" => "<=",
            "gt" => ">",
            "gte" => ">=",
            "in" | "nin" => {
                let values = list(field, v)?.iter().map(num).collect::<ApiResult<Vec<_>>>()?;
                qb.push(format!("{}{column}::float8 = ANY(", if op == "nin" { "NOT " } else { "" }));
                qb.push_bind(values);
                qb.push(")");
                continue;
            }
            "null" => {
                let null = v.as_bool().ok_or_else(|| bad(field, "null takes true or false"))?;
                qb.push(format!("{column} IS {}NULL", if null { "" } else { "NOT " }));
                continue;
            }
            other => return Err(bad(field, &format!("unknown comparator {other}"))),
        };
        qb.push(format!("{column}::float8 {sql_op} "));
        qb.push_bind(num(v)?);
    }
    qb.push(")");
    Ok(())
}

fn date(qb: &mut QueryBuilder<Postgres>, column: &str, field: &str, value: &Value, is_date: bool) -> ApiResult<()> {
    let at = |v: &Value| -> ApiResult<chrono::DateTime<chrono::Utc>> {
        parse_datetime(v.as_str().ok_or_else(|| bad(field, "expected an ISO date or duration"))?)
    };
    let cast = if is_date { "::timestamptz" } else { "" };
    qb.push("(TRUE");
    for (op, v) in comparators(field, value)? {
        qb.push(" AND ");
        let sql_op = match op {
            "eq" => {
                if is_date {
                    qb.push(format!("{column} = "));
                    qb.push_bind(at(v)?.date_naive());
                } else {
                    qb.push(format!("date_trunc('day', {column}) = date_trunc('day', "));
                    qb.push_bind(at(v)?);
                    qb.push("::timestamptz)");
                }
                continue;
            }
            "neq" => "IS DISTINCT FROM",
            "lt" => "<",
            "lte" => "<=",
            "gt" => ">",
            "gte" => ">=",
            "null" => {
                let null = v.as_bool().ok_or_else(|| bad(field, "null takes true or false"))?;
                qb.push(format!("{column} IS {}NULL", if null { "" } else { "NOT " }));
                continue;
            }
            other => {
                return Err(bad(field, &format!("dates compare with eq, neq, lt, lte, gt, gte or null, not {other}")));
            }
        };
        qb.push(format!("{column}{cast} {sql_op} "));
        qb.push_bind(at(v)?);
    }
    qb.push(")");
    Ok(())
}
