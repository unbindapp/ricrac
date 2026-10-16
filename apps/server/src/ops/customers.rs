//! Customers: the companies a workspace builds for, with their status, tier and owner, and
//! customer needs (requests) linking them to issues and projects. Guests can't see them.

use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Value, json};
use sqlx::{PgConnection, Postgres, QueryBuilder};
use uuid::Uuid;

use super::events;
use super::issues::changed_fields;
use super::resolve::{self, as_uuid};
use super::util::{Page, check_color, non_empty, nullable, pick_color, slug_id};
use super::{IdArgs, Op, op};
use crate::ctx::{Access, Ctx};
use crate::error::{ApiError, ApiResult};

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "list_customers",
            Access::Read,
            &[("GET", "/customers")],
            "List customers by name: domains, external ids, owner, status, tier, revenue, size and how many needs \
             (requests) they have. Narrow by status, tier, owner or a query matching the name or a domain. Archived \
             customers are left out unless includeArchived is set.",
            list,
        ),
        op(
            "get_customer",
            Access::Read,
            &[("GET", "/customers/{id}")],
            "Read a customer by id, slug, name, domain or external id, with their needs: what they asked for, linked to \
             an issue or a project.",
            get,
        ),
        op(
            "save_customer",
            Access::Write,
            &[("POST", "/customers"), ("PATCH", "/customers/{id}")],
            "Create a customer (omit id; name is required) or update one (pass id: id, slug, name or domain). Omitted \
             fields stay as they are; null clears a field. Domains are unique within the workspace. Status and tier \
             take a name or id (see list_customer_statuses and list_customer_tiers); owner a person (id, name, email \
             or \"me\").",
            save,
        ),
        op(
            "delete_customer",
            Access::Write,
            &[("DELETE", "/customers/{id}")],
            "Archive a customer. Their needs stay on the issues and projects they are linked to.",
            delete,
        ),
        op(
            "list_customer_statuses",
            Access::Read,
            &[("GET", "/customer-statuses")],
            "The customer statuses of the workspace (e.g. Active, Prospect, Churned) in order.",
            list_statuses,
        ),
        op(
            "save_customer_status",
            Access::Admin,
            &[("POST", "/customer-statuses"), ("PATCH", "/customer-statuses/{id}")],
            "Create a customer status (omit id; name is required) or update one (pass its id or name). Admins only.",
            save_status,
        ),
        op(
            "delete_customer_status",
            Access::Admin,
            &[("DELETE", "/customer-statuses/{id}")],
            "Delete a customer status (id or name). Customers with it are left without a status. Admins only.",
            delete_status,
        ),
        op(
            "list_customer_tiers",
            Access::Read,
            &[("GET", "/customer-tiers")],
            "The customer tiers of the workspace (e.g. Enterprise, Growth) in order.",
            list_tiers,
        ),
        op(
            "save_customer_tier",
            Access::Admin,
            &[("POST", "/customer-tiers"), ("PATCH", "/customer-tiers/{id}")],
            "Create a customer tier (omit id; name is required) or update one (pass its id or name). Admins only.",
            save_tier,
        ),
        op(
            "delete_customer_tier",
            Access::Admin,
            &[("DELETE", "/customer-tiers/{id}")],
            "Delete a customer tier (id or name). Customers in it are left without a tier. Admins only.",
            delete_tier,
        ),
        op(
            "list_customer_needs",
            Access::Read,
            &[("GET", "/customer-needs")],
            "List customer needs (requests), newest first, for a customer, an issue or a project: the customer, the \
             issue or project, the request text, priority (0 normal, 1 important) and a link to the source.",
            list_needs,
        ),
        op(
            "save_customer_need",
            Access::Write,
            &[("POST", "/customer-needs"), ("PATCH", "/customer-needs/{id}")],
            "Record what a customer asked for (omit id; customer and exactly one of issue or project are required) or \
             change a need (pass id). Issues take an identifier like ENG-123; projects a name, slug or id.",
            save_need,
        ),
        op(
            "delete_customer_need",
            Access::Write,
            &[("DELETE", "/customer-needs/{id}")],
            "Delete a customer need.",
            delete_need,
        ),
    ]
}

/* ------------------------------------------------------------------ */
/*                           Representation                           */
/* ------------------------------------------------------------------ */

const CUSTOMER_JSON: &str = "jsonb_build_object(
    'id', c.id,
    'slugId', c.slug_id,
    'name', c.name,
    'domains', to_jsonb(c.domains),
    'externalIds', to_jsonb(c.external_ids),
    'logoUrl', c.logo_url,
    'owner', user_ref(c.owner_id),
    'status', customer_status_ref(c.status_id),
    'tier', customer_tier_ref(c.tier_id),
    'revenue', c.revenue,
    'size', c.size,
    'needCount', (SELECT count(*) FROM customer_needs n WHERE n.customer_id = c.id AND n.archived_at IS NULL),
    'archivedAt', c.archived_at,
    'createdAt', c.created_at,
    'updatedAt', c.updated_at
)";

const NEED_JSON: &str = "jsonb_build_object(
    'id', n.id,
    'customer', customer_ref(n.customer_id),
    'issue', issue_ref(n.issue_id),
    'project', project_ref(n.project_id),
    'body', n.body,
    'priority', n.priority,
    'attachmentUrl', n.attachment_url,
    'creator', user_ref(n.creator_id),
    'createdAt', n.created_at,
    'updatedAt', n.updated_at
)";

/// Adds the condition that need `n` points at an issue or project the caller can see.
async fn push_need_visible(qb: &mut QueryBuilder<'_, Postgres>, ctx: &Ctx) -> ApiResult<()> {
    let teams = ctx.team_ids().await?.to_vec();
    qb.push(
        " AND (n.issue_id IS NULL OR EXISTS (SELECT 1 FROM issues i WHERE i.id = n.issue_id AND i.trashed_at IS NULL \
         AND (i.team_id = ANY(",
    );
    qb.push_bind(teams.clone());
    qb.push(") OR EXISTS (SELECT 1 FROM issue_shares sh WHERE sh.issue_id = i.id AND sh.user_id = ");
    qb.push_bind(ctx.user);
    qb.push(
        ")))) AND (n.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p JOIN project_teams pt ON pt.project_id = p.id \
         WHERE p.id = n.project_id AND p.trashed_at IS NULL AND pt.team_id = ANY(",
    );
    qb.push_bind(teams);
    qb.push(")))");
    Ok(())
}

async fn customer_value(db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    let value: Option<Value> = sqlx::query_scalar(&format!("SELECT {CUSTOMER_JSON} FROM customers c WHERE c.id = $1"))
        .bind(id)
        .fetch_optional(&mut *db)
        .await?;
    value.ok_or_else(|| ApiError::not_found("No such customer"))
}

async fn need_value(db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    let value: Option<Value> = sqlx::query_scalar(&format!("SELECT {NEED_JSON} FROM customer_needs n WHERE n.id = $1"))
        .bind(id)
        .fetch_optional(&mut *db)
        .await?;
    value.ok_or_else(|| ApiError::not_found("No such customer need"))
}

async fn customer_id(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    ctx.require_member()?;
    resolve::customer(ctx, value).await
}

/* ------------------------------------------------------------------ */
/*                              Customers                             */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct ListArgs {
    /// Customer status name or id.
    status: Option<String>,
    /// Customer tier name or id.
    tier: Option<String>,
    /// Owner: id, name, email or "me".
    owner: Option<String>,
    /// Words in the name or a domain.
    query: Option<String>,
    #[serde(default)]
    include_archived: bool,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list(ctx: Ctx, args: ListArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let page = Page { limit: args.limit, cursor: args.cursor.clone() };
    let mut qb = QueryBuilder::new(format!("SELECT {CUSTOMER_JSON} FROM customers c WHERE c.workspace_id = "));
    qb.push_bind(ctx.workspace);
    if !args.include_archived {
        qb.push(" AND c.archived_at IS NULL");
    }
    if let Some(status) = &args.status {
        qb.push(" AND c.status_id = ");
        qb.push_bind(option_id(&ctx, STATUSES, status).await?);
    }
    if let Some(tier) = &args.tier {
        qb.push(" AND c.tier_id = ");
        qb.push_bind(option_id(&ctx, TIERS, tier).await?);
    }
    if let Some(owner) = &args.owner {
        qb.push(" AND c.owner_id = ");
        qb.push_bind(resolve::user(&ctx, owner).await?);
    }
    if let Some(query) = args.query.as_deref().map(str::trim).filter(|q| !q.is_empty()) {
        let pattern = format!("%{}%", query.replace('\\', "\\\\").replace('%', "\\%").replace('_', "\\_"));
        qb.push(" AND (c.name ILIKE ");
        qb.push_bind(pattern.clone());
        qb.push(" OR EXISTS (SELECT 1 FROM unnest(c.domains) d WHERE d ILIKE ");
        qb.push_bind(pattern);
        qb.push("))");
    }
    qb.push(" ORDER BY lower(c.name), c.id LIMIT ");
    qb.push_bind(page.limit() + 1);
    qb.push(" OFFSET ");
    qb.push_bind(page.offset()?);
    let nodes: Vec<Value> = qb.build_query_scalar().fetch_all(ctx.db()).await?;
    page.wrap(nodes)
}

async fn get(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = customer_id(&ctx, &args.id).await?;
    let mut db = ctx.db().acquire().await?;
    let mut value = customer_value(&mut db, id).await?;
    let mut qb = QueryBuilder::new(format!(
        "SELECT coalesce(jsonb_agg({NEED_JSON} ORDER BY n.created_at DESC), '[]') FROM customer_needs n \
         WHERE n.archived_at IS NULL AND n.customer_id = "
    ));
    qb.push_bind(id);
    push_need_visible(&mut qb, &ctx).await?;
    value["needs"] = qb.build_query_scalar().fetch_one(&mut *db).await?;
    Ok(value)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct SaveArgs {
    /// Only to update a customer: id, slug, name or domain. Omit to create one.
    id: Option<String>,
    /// Required to create.
    name: Option<String>,
    /// Replaces the domains, e.g. ["acme.com"]. Each domain belongs to one customer.
    domains: Option<Vec<String>>,
    /// Replaces the ids of the customer in other systems (CRM, billing).
    external_ids: Option<Vec<String>>,
    /// Logo image URL; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    logo_url: Option<Option<String>>,
    /// Person responsible for the customer (id, name, email or "me"); null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    owner: Option<Option<String>>,
    /// Customer status name or id; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    status: Option<Option<String>>,
    /// Customer tier name or id; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    tier: Option<Option<String>>,
    /// Annual revenue, a whole number; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<i64>")]
    revenue: Option<Option<i64>>,
    /// Number of employees; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<i32>")]
    size: Option<Option<i32>>,
}

fn domain(value: &str) -> ApiResult<String> {
    let lower = value.trim().to_lowercase();
    let host = lower.strip_prefix("https://").or_else(|| lower.strip_prefix("http://")).unwrap_or(&lower);
    let host = host.split('/').next().unwrap_or_default();
    if !host.contains('.') || host.starts_with('.') || host.ends_with('.') || host.contains(char::is_whitespace) {
        return Err(ApiError::bad_request(format!("\"{value}\" isn't a domain like acme.com")));
    }
    Ok(host.to_string())
}

async fn domains(
    ctx: &Ctx,
    db: &mut PgConnection,
    customer: Option<Uuid>,
    values: &[String],
) -> ApiResult<Vec<String>> {
    let mut domains: Vec<String> = Vec::new();
    for value in values {
        let domain = domain(value)?;
        if !domains.contains(&domain) {
            domains.push(domain);
        }
    }
    let taken: Option<(String, String)> = sqlx::query_as(
        "SELECT c.name, d FROM customers c, unnest(c.domains) d
         WHERE c.workspace_id = $1 AND c.id IS DISTINCT FROM $2 AND d = ANY($3) LIMIT 1",
    )
    .bind(ctx.workspace)
    .bind(customer)
    .bind(&domains)
    .fetch_optional(&mut *db)
    .await?;
    if let Some((name, domain)) = taken {
        return Err(ApiError::conflict(format!("{domain} already belongs to the customer {name}")));
    }
    Ok(domains)
}

async fn save(ctx: Ctx, args: SaveArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let existing = match &args.id {
        Some(reference) => Some(customer_id(&ctx, reference).await?),
        None => None,
    };
    let owner = match &args.owner {
        Some(Some(owner)) => Some(Some(resolve::user(&ctx, owner).await?)),
        Some(None) => Some(None),
        None => None,
    };
    let status = match &args.status {
        Some(Some(status)) => Some(Some(option_id(&ctx, STATUSES, status).await?)),
        Some(None) => Some(None),
        None => None,
    };
    let tier = match &args.tier {
        Some(Some(tier)) => Some(Some(option_id(&ctx, TIERS, tier).await?)),
        Some(None) => Some(None),
        None => None,
    };
    let name = args.name.as_deref().map(|n| non_empty(n, "name")).transpose()?;
    let external_ids: Option<Vec<String>> = args.external_ids.map(|ids| {
        let mut unique: Vec<String> = Vec::new();
        for id in ids.into_iter().map(|id| id.trim().to_string()).filter(|id| !id.is_empty()) {
            if !unique.contains(&id) {
                unique.push(id);
            }
        }
        unique
    });
    let logo_url = args.logo_url.map(|url| url.map(|u| u.trim().to_string()).filter(|u| !u.is_empty()));
    for (value, what) in [(args.revenue.flatten(), "revenue"), (args.size.flatten().map(i64::from), "size")] {
        if value.is_some_and(|v| v < 0) {
            return Err(ApiError::bad_request(format!("{what} can't be negative")));
        }
    }
    let mut tx = ctx.db().begin().await?;
    let domains = match &args.domains {
        Some(values) => Some(domains(&ctx, &mut tx, existing, values).await?),
        None => None,
    };
    let (id, before) = match existing {
        Some(id) => (id, Some(customer_value(&mut tx, id).await?)),
        None => {
            let name = name.clone().ok_or_else(|| ApiError::bad_request("name is required to create a customer"))?;
            let id: Uuid = sqlx::query_scalar(
                "INSERT INTO customers (workspace_id, slug_id, name) VALUES ($1, $2, $3) RETURNING id",
            )
            .bind(ctx.workspace)
            .bind(slug_id())
            .bind(&name)
            .fetch_one(&mut *tx)
            .await?;
            (id, None)
        }
    };
    sqlx::query(
        "UPDATE customers SET
            name = coalesce($2, name),
            domains = coalesce($3, domains),
            external_ids = coalesce($4, external_ids),
            logo_url = CASE WHEN $5 THEN $6 ELSE logo_url END,
            owner_id = CASE WHEN $7 THEN $8 ELSE owner_id END,
            status_id = CASE WHEN $9 THEN $10 ELSE status_id END,
            tier_id = CASE WHEN $11 THEN $12 ELSE tier_id END,
            revenue = CASE WHEN $13 THEN $14 ELSE revenue END,
            size = CASE WHEN $15 THEN $16 ELSE size END,
            updated_at = now()
         WHERE id = $1",
    )
    .bind(id)
    .bind(&name)
    .bind(&domains)
    .bind(&external_ids)
    .bind(logo_url.is_some())
    .bind(logo_url.flatten())
    .bind(owner.is_some())
    .bind(owner.flatten())
    .bind(status.is_some())
    .bind(status.flatten())
    .bind(tier.is_some())
    .bind(tier.flatten())
    .bind(args.revenue.is_some())
    .bind(args.revenue.flatten())
    .bind(args.size.is_some())
    .bind(args.size.flatten())
    .execute(&mut *tx)
    .await?;
    let value = customer_value(&mut tx, id).await?;
    match before {
        Some(before) => {
            let from = changed_fields(&before, &value);
            if !from.as_object().is_none_or(|o| o.is_empty()) {
                events::emit(&mut tx, &ctx, "Customer", "update", &value, Some(from), None).await?;
            }
        }
        None => events::emit(&mut tx, &ctx, "Customer", "create", &value, None, None).await?,
    }
    tx.commit().await?;
    Ok(value)
}

async fn delete(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = customer_id(&ctx, &args.id).await?;
    let mut tx = ctx.db().begin().await?;
    sqlx::query("UPDATE customers SET archived_at = now(), updated_at = now() WHERE id = $1 AND archived_at IS NULL")
        .bind(id)
        .execute(&mut *tx)
        .await?;
    let value = customer_value(&mut tx, id).await?;
    events::emit(&mut tx, &ctx, "Customer", "remove", &value, None, None).await?;
    tx.commit().await?;
    Ok(value)
}

/* ------------------------------------------------------------------ */
/*                         Statuses and tiers                         */
/* ------------------------------------------------------------------ */

/// Customer statuses and tiers have the same shape; this says which table.
#[derive(Clone, Copy)]
struct Options {
    table: &'static str,
    what: &'static str,
    column: &'static str,
}

const STATUSES: Options = Options { table: "customer_statuses", what: "customer status", column: "status_id" };
const TIERS: Options = Options { table: "customer_tiers", what: "customer tier", column: "tier_id" };

fn option_json(kind: Options) -> String {
    format!(
        "jsonb_build_object('id', o.id, 'name', o.name, 'color', o.color, 'description', o.description,
            'position', o.position,
            'customerCount', (SELECT count(*) FROM customers c WHERE c.{} = o.id AND c.archived_at IS NULL))",
        kind.column
    )
}

async fn option_id(ctx: &Ctx, kind: Options, value: &str) -> ApiResult<Uuid> {
    let rows: Vec<(Uuid, String)> =
        sqlx::query_as(&format!("SELECT id, name FROM {} WHERE workspace_id = $1 ORDER BY position", kind.table))
            .bind(ctx.workspace)
            .fetch_all(ctx.db())
            .await?;
    let value = value.trim();
    let found = rows
        .iter()
        .find(|r| Some(r.0) == as_uuid(value))
        .or_else(|| rows.iter().find(|r| r.1.eq_ignore_ascii_case(value)));
    if let Some(row) = found {
        return Ok(row.0);
    }
    let names: Vec<_> = rows.iter().map(|r| r.1.as_str()).collect();
    let options = if names.is_empty() { "There are none.".into() } else { format!("Options: {}.", names.join(", ")) };
    Err(ApiError::bad_request(format!("No {} \"{value}\". {options}", kind.what)))
}

async fn option_value(db: &mut PgConnection, kind: Options, id: Uuid) -> ApiResult<Value> {
    Ok(sqlx::query_scalar(&format!("SELECT {} FROM {} o WHERE o.id = $1", option_json(kind), kind.table))
        .bind(id)
        .fetch_one(&mut *db)
        .await?)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct ListOptionsArgs {
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list_options(ctx: Ctx, kind: Options, args: ListOptionsArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let page = Page { limit: args.limit, cursor: args.cursor };
    let nodes: Vec<Value> = sqlx::query_scalar(&format!(
        "SELECT {} FROM {} o WHERE o.workspace_id = $1 ORDER BY o.position, o.name LIMIT $2 OFFSET $3",
        option_json(kind),
        kind.table
    ))
    .bind(ctx.workspace)
    .bind(page.limit() + 1)
    .bind(page.offset()?)
    .fetch_all(ctx.db())
    .await?;
    page.wrap(nodes)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct SaveOptionArgs {
    /// Only to update one: its id or name. Omit to create one.
    id: Option<String>,
    /// Required to create.
    name: Option<String>,
    /// Hex color like #4cb782.
    color: Option<String>,
    /// null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    description: Option<Option<String>>,
    /// Order among the others, lowest first. New ones go last.
    position: Option<f64>,
}

async fn save_option(ctx: Ctx, kind: Options, args: SaveOptionArgs) -> ApiResult<Value> {
    let existing = match &args.id {
        Some(reference) => Some(option_id(&ctx, kind, reference).await?),
        None => None,
    };
    let name = args.name.as_deref().map(|n| non_empty(n, "name")).transpose()?;
    let color = args.color.as_deref().map(check_color).transpose()?;
    let mut tx = ctx.db().begin().await?;
    if let Some(name) = &name {
        let taken: bool = sqlx::query_scalar(&format!(
            "SELECT EXISTS (SELECT 1 FROM {} WHERE workspace_id = $1 AND lower(name) = lower($2) AND id IS DISTINCT FROM $3)",
            kind.table
        ))
        .bind(ctx.workspace)
        .bind(name)
        .bind(existing)
        .fetch_one(&mut *tx)
        .await?;
        if taken {
            return Err(ApiError::conflict(format!("There already is a {} named {name}", kind.what)));
        }
    }
    let id = match existing {
        Some(id) => id,
        None => {
            let name = name.clone().ok_or_else(|| ApiError::bad_request("name is required to create one"))?;
            sqlx::query_scalar(&format!(
                "INSERT INTO {0} (workspace_id, name, color, position)
                 VALUES ($1, $2, $3, (SELECT coalesce(max(position), -1) + 1 FROM {0} WHERE workspace_id = $1))
                 RETURNING id",
                kind.table
            ))
            .bind(ctx.workspace)
            .bind(&name)
            .bind(color.clone().unwrap_or_else(|| pick_color(&name)))
            .fetch_one(&mut *tx)
            .await?
        }
    };
    let description = args.description.map(|d| d.map(|d| d.trim().to_string()).filter(|d| !d.is_empty()));
    sqlx::query(&format!(
        "UPDATE {} SET name = coalesce($2, name), color = coalesce($3, color),
            description = CASE WHEN $4 THEN $5 ELSE description END, position = coalesce($6, position)
         WHERE id = $1",
        kind.table
    ))
    .bind(id)
    .bind(&name)
    .bind(&color)
    .bind(description.is_some())
    .bind(description.flatten())
    .bind(args.position)
    .execute(&mut *tx)
    .await?;
    let value = option_value(&mut tx, kind, id).await?;
    tx.commit().await?;
    Ok(value)
}

async fn delete_option(ctx: Ctx, kind: Options, args: IdArgs) -> ApiResult<Value> {
    let id = option_id(&ctx, kind, &args.id).await?;
    let mut tx = ctx.db().begin().await?;
    let cleared: i64 = sqlx::query_scalar(&format!("SELECT count(*) FROM customers WHERE {} = $1", kind.column))
        .bind(id)
        .fetch_one(&mut *tx)
        .await?;
    sqlx::query(&format!("DELETE FROM {} WHERE id = $1", kind.table)).bind(id).execute(&mut *tx).await?;
    tx.commit().await?;
    Ok(json!({ "id": id, "deleted": true, "customersCleared": cleared }))
}

async fn list_statuses(ctx: Ctx, args: ListOptionsArgs) -> ApiResult<Value> {
    list_options(ctx, STATUSES, args).await
}

async fn save_status(ctx: Ctx, args: SaveOptionArgs) -> ApiResult<Value> {
    save_option(ctx, STATUSES, args).await
}

async fn delete_status(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    delete_option(ctx, STATUSES, args).await
}

async fn list_tiers(ctx: Ctx, args: ListOptionsArgs) -> ApiResult<Value> {
    list_options(ctx, TIERS, args).await
}

async fn save_tier(ctx: Ctx, args: SaveOptionArgs) -> ApiResult<Value> {
    save_option(ctx, TIERS, args).await
}

async fn delete_tier(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    delete_option(ctx, TIERS, args).await
}

/* ------------------------------------------------------------------ */
/*                                Needs                               */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct ListNeedsArgs {
    /// Customer id, slug, name or domain.
    customer: Option<String>,
    /// Issue identifier (ENG-123) or id.
    issue: Option<String>,
    /// Project name, slug or id.
    project: Option<String>,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list_needs(ctx: Ctx, args: ListNeedsArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let page = Page { limit: args.limit, cursor: args.cursor.clone() };
    let mut qb = QueryBuilder::new(format!(
        "SELECT {NEED_JSON} FROM customer_needs n WHERE n.archived_at IS NULL AND n.workspace_id = "
    ));
    qb.push_bind(ctx.workspace);
    push_need_visible(&mut qb, &ctx).await?;
    if let Some(customer) = &args.customer {
        qb.push(" AND n.customer_id = ");
        qb.push_bind(customer_id(&ctx, customer).await?);
    }
    if let Some(issue) = &args.issue {
        qb.push(" AND n.issue_id = ");
        qb.push_bind(resolve::issue(&ctx, issue).await?);
    }
    if let Some(project) = &args.project {
        qb.push(" AND n.project_id = ");
        qb.push_bind(resolve::project(&ctx, project).await?);
    }
    qb.push(" ORDER BY n.created_at DESC, n.id LIMIT ");
    qb.push_bind(page.limit() + 1);
    qb.push(" OFFSET ");
    qb.push_bind(page.offset()?);
    let nodes: Vec<Value> = qb.build_query_scalar().fetch_all(ctx.db()).await?;
    page.wrap(nodes)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct SaveNeedArgs {
    /// Only to update a need: its id. Omit to create one.
    id: Option<String>,
    /// Customer id, slug, name or domain. Required to create.
    customer: Option<String>,
    /// The issue (ENG-123 or id) the customer needs. Pass this or project.
    issue: Option<String>,
    /// The project (name, slug or id) the customer needs. Pass this or issue.
    project: Option<String>,
    /// What the customer said, in Markdown.
    body: Option<String>,
    /// 0 normal, 1 important.
    priority: Option<i32>,
    /// Link to where the request came from (an email, a support ticket); null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    attachment_url: Option<Option<String>>,
}

async fn need_id(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    ctx.require_member()?;
    let id = resolve::by_id(ctx, "customer_needs", "customer need", value).await?;
    let mut qb = QueryBuilder::new("SELECT EXISTS (SELECT 1 FROM customer_needs n WHERE n.id = ");
    qb.push_bind(id);
    push_need_visible(&mut qb, ctx).await?;
    qb.push(")");
    let visible: bool = qb.build_query_scalar().fetch_one(ctx.db()).await?;
    if !visible {
        return Err(ApiError::not_found(format!("No customer need \"{}\"", value.trim())));
    }
    Ok(id)
}

async fn need_team(db: &mut PgConnection, need: Uuid) -> ApiResult<Option<Uuid>> {
    Ok(sqlx::query_scalar("SELECT i.team_id FROM customer_needs n JOIN issues i ON i.id = n.issue_id WHERE n.id = $1")
        .bind(need)
        .fetch_optional(&mut *db)
        .await?)
}

async fn save_need(ctx: Ctx, args: SaveNeedArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let existing = match &args.id {
        Some(reference) => Some(need_id(&ctx, reference).await?),
        None => None,
    };
    if args.issue.is_some() && args.project.is_some() {
        return Err(ApiError::bad_request("A need is for an issue or a project, not both"));
    }
    if existing.is_none() && args.issue.is_none() && args.project.is_none() {
        return Err(ApiError::bad_request("Pass the issue or the project the customer needs"));
    }
    let customer = match &args.customer {
        Some(customer) => Some(customer_id(&ctx, customer).await?),
        None if existing.is_none() => return Err(ApiError::bad_request("customer is required to create a need")),
        None => None,
    };
    let issue = match &args.issue {
        Some(issue) => Some(resolve::issue(&ctx, issue).await?),
        None => None,
    };
    let project = match &args.project {
        Some(project) => Some(resolve::project(&ctx, project).await?),
        None => None,
    };
    if args.priority.is_some_and(|p| !(0..=1).contains(&p)) {
        return Err(ApiError::bad_request("priority is 0 (normal) or 1 (important)"));
    }
    let attachment_url = args.attachment_url.map(|url| url.map(|u| u.trim().to_string()).filter(|u| !u.is_empty()));
    let mut tx = ctx.db().begin().await?;
    let (id, before) = match existing {
        Some(id) => (id, Some(need_value(&mut tx, id).await?)),
        None => {
            let id: Uuid = sqlx::query_scalar(
                "INSERT INTO customer_needs (workspace_id, creator_id) VALUES ($1, $2) RETURNING id",
            )
            .bind(ctx.workspace)
            .bind(ctx.user)
            .fetch_one(&mut *tx)
            .await?;
            (id, None)
        }
    };
    let target_given = issue.is_some() || project.is_some();
    sqlx::query(
        "UPDATE customer_needs SET
            customer_id = coalesce($2, customer_id),
            issue_id = CASE WHEN $3 THEN $4 ELSE issue_id END,
            project_id = CASE WHEN $3 THEN $5 ELSE project_id END,
            body = coalesce($6, body),
            priority = coalesce($7, priority),
            attachment_url = CASE WHEN $8 THEN $9 ELSE attachment_url END,
            updated_at = now()
         WHERE id = $1",
    )
    .bind(id)
    .bind(customer)
    .bind(target_given)
    .bind(issue)
    .bind(project)
    .bind(&args.body)
    .bind(args.priority)
    .bind(attachment_url.is_some())
    .bind(attachment_url.flatten())
    .execute(&mut *tx)
    .await?;
    let value = need_value(&mut tx, id).await?;
    let team = need_team(&mut tx, id).await?;
    match before {
        Some(before) => {
            let from = changed_fields(&before, &value);
            if !from.as_object().is_none_or(|o| o.is_empty()) {
                events::emit(&mut tx, &ctx, "CustomerNeed", "update", &value, Some(from), team).await?;
            }
        }
        None => events::emit(&mut tx, &ctx, "CustomerNeed", "create", &value, None, team).await?,
    }
    tx.commit().await?;
    Ok(value)
}

async fn delete_need(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let id = need_id(&ctx, &args.id).await?;
    let mut tx = ctx.db().begin().await?;
    let value = need_value(&mut tx, id).await?;
    let team = need_team(&mut tx, id).await?;
    sqlx::query("DELETE FROM customer_needs WHERE id = $1").bind(id).execute(&mut *tx).await?;
    events::emit(&mut tx, &ctx, "CustomerNeed", "remove", &value, None, team).await?;
    tx.commit().await?;
    Ok(json!({ "id": id, "deleted": true }))
}
