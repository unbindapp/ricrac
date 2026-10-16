//! The workspace, its members and invitations, and the audit log.

use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Value, json};
use sqlx::PgConnection;
use uuid::Uuid;

use super::events;
use super::resolve::{self, as_uuid};
use super::util::{Page, non_empty, nullable};
use super::{IdArgs, NoArgs, Op, op};
use crate::account::check_url_key;
use crate::ctx::{Access, Ctx, Role};
use crate::error::{ApiError, ApiResult};

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "get_workspace",
            Access::Read,
            &[("GET", "/workspace")],
            "The workspace: its name, URL key and link, your role, the teams you can see (key, name, private) and \
             how many members and issues it has.",
            get_workspace,
        ),
        op(
            "update_workspace",
            Access::Admin,
            &[("PATCH", "/workspace")],
            "Rename the workspace, change its URL key (2 to 32 lowercase letters, digits and dashes; links with the old \
             key stop working) or its logo. Admins only. Omitted fields stay as they are; null clears the logo.",
            update_workspace,
        ),
        op(
            "list_users",
            Access::Read,
            &[("GET", "/users")],
            "List the workspace's members with their role, teams and whether they are you. Narrow with query (name, \
             display name or email) or team; removed members are left out unless includeInactive is set.",
            list_users,
        ),
        op(
            "get_user",
            Access::Read,
            &[("GET", "/users/{id}")],
            "Read a member by id, name, display name, email or \"me\": role, teams, timezone and how many open issues \
             are assigned to them. Removed members can be read by id.",
            get_user,
        ),
        op(
            "update_user",
            Access::Write,
            &[("PATCH", "/users/{id}")],
            "Change a member. Anyone may change their own name, displayName and timezone. Changing a role (owner, admin, \
             member, guest) needs an admin; only owners grant or take away owner, and the last owner stays one. Omitted \
             fields stay as they are; null clears the timezone.",
            update_user,
        ),
        op(
            "remove_user",
            Access::Admin,
            &[("DELETE", "/users/{id}")],
            "Remove a member from the workspace (admins only): they lose access and leave all teams; their issues, \
             comments and history stay. The last owner can't be removed. Invite them again to bring them back.",
            remove_user,
        ),
        op(
            "list_invitations",
            Access::Read,
            &[("GET", "/invitations")],
            "List pending invitations to the workspace: email, role, teams, who invited and when they expire.",
            list_invitations,
        ),
        op(
            "invite_user",
            Access::Write,
            &[("POST", "/invitations")],
            "Invite someone by email. role is member (default), admin (only admins invite admins) or guest; teams \
             (keys, names or ids) are the teams they join, required for guests, who only see those teams. The \
             invitation expires in 14 days; inviting the same email again renews it. They accept it in the dashboard.",
            invite_user,
        ),
        op(
            "delete_invitation",
            Access::Write,
            &[("DELETE", "/invitations/{id}")],
            "Withdraw a pending invitation by id. Admins can withdraw any; members the ones they sent.",
            delete_invitation,
        ),
        op(
            "get_audit_log",
            Access::Admin,
            &[("GET", "/audit-log")],
            "The workspace's audit log, newest first (admins only): role changes, removed members, invitations, \
             archived teams and workspace changes, with who did it. Filter with type, e.g. member.roleChanged.",
            get_audit_log,
        ),
    ]
}

/* ------------------------------------------------------------------ */
/*                              Workspace                             */
/* ------------------------------------------------------------------ */

async fn workspace_value(ctx: &Ctx, db: &mut PgConnection) -> ApiResult<Value> {
    let teams = ctx.team_ids().await?.to_vec();
    Ok(sqlx::query_scalar(
        "SELECT jsonb_build_object(
            'id', w.id,
            'name', w.name,
            'urlKey', w.url_key,
            'logoUrl', w.logo_url,
            'url', $3 || '/' || w.url_key,
            'createdAt', w.created_at,
            'role', $4,
            'teams', (SELECT coalesce(jsonb_agg(jsonb_build_object('id', t.id, 'key', t.key, 'name', t.name,
                'private', t.private) ORDER BY t.name), '[]')
                FROM teams t WHERE t.id = ANY($2) AND t.archived_at IS NULL),
            'memberCount', (SELECT count(*) FROM workspace_members m WHERE m.workspace_id = w.id AND m.active),
            'issueCount', (SELECT count(*) FROM issues i WHERE i.workspace_id = w.id AND i.team_id = ANY($2)
                AND i.trashed_at IS NULL))
         FROM workspaces w WHERE w.id = $1",
    )
    .bind(ctx.workspace)
    .bind(&teams)
    .bind(ctx.public_url())
    .bind(ctx.role.as_str())
    .fetch_one(&mut *db)
    .await?)
}

async fn get_workspace(ctx: Ctx, _: NoArgs) -> ApiResult<Value> {
    let mut db = ctx.db().acquire().await?;
    workspace_value(&ctx, &mut db).await
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct UpdateWorkspaceArgs {
    /// 1 to 80 characters.
    name: Option<String>,
    /// 2 to 32 lowercase letters, digits and dashes; must not be taken by another workspace.
    url_key: Option<String>,
    /// Image URL; null removes the logo.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    logo_url: Option<Option<String>>,
}

async fn update_workspace(ctx: Ctx, args: UpdateWorkspaceArgs) -> ApiResult<Value> {
    let mut tx = ctx.db().begin().await?;
    let (name, url_key, logo_url): (String, String, Option<String>) =
        sqlx::query_as("SELECT name, url_key, logo_url FROM workspaces WHERE id = $1 FOR UPDATE")
            .bind(ctx.workspace)
            .fetch_one(&mut *tx)
            .await?;
    let mut changes = serde_json::Map::new();
    if let Some(new_name) = args.name {
        let new_name = non_empty(&new_name, "name")?;
        crate::ensure!(new_name.chars().count() <= 80, "The name is 1 to 80 characters");
        if new_name != name {
            changes.insert("name".into(), json!({ "from": name, "to": new_name }));
            sqlx::query("UPDATE workspaces SET name = $2 WHERE id = $1")
                .bind(ctx.workspace)
                .bind(&new_name)
                .execute(&mut *tx)
                .await?;
        }
    }
    if let Some(new_key) = args.url_key {
        let new_key = check_url_key(&new_key)?;
        if new_key != url_key {
            let taken: bool =
                sqlx::query_scalar("SELECT EXISTS (SELECT 1 FROM workspaces WHERE url_key = $1 AND id <> $2)")
                    .bind(&new_key)
                    .bind(ctx.workspace)
                    .fetch_one(&mut *tx)
                    .await?;
            if taken {
                return Err(ApiError::conflict(format!("The URL key \"{new_key}\" is taken")));
            }
            changes.insert("urlKey".into(), json!({ "from": url_key, "to": new_key }));
            sqlx::query("UPDATE workspaces SET url_key = $2 WHERE id = $1")
                .bind(ctx.workspace)
                .bind(&new_key)
                .execute(&mut *tx)
                .await?;
        }
    }
    if let Some(logo) = args.logo_url {
        let logo = logo.map(|l| l.trim().to_string()).filter(|l| !l.is_empty());
        if logo != logo_url {
            changes.insert("logoUrl".into(), json!({ "from": logo_url, "to": logo }));
            sqlx::query("UPDATE workspaces SET logo_url = $2 WHERE id = $1")
                .bind(ctx.workspace)
                .bind(&logo)
                .execute(&mut *tx)
                .await?;
        }
    }
    if !changes.is_empty() {
        sqlx::query("UPDATE workspaces SET updated_at = now() WHERE id = $1")
            .bind(ctx.workspace)
            .execute(&mut *tx)
            .await?;
        events::audit(&mut tx, &ctx, "workspace.updated", Value::Object(changes)).await?;
    }
    let value = workspace_value(&ctx, &mut tx).await?;
    tx.commit().await?;
    Ok(value)
}

/* ------------------------------------------------------------------ */
/*                               Members                              */
/* ------------------------------------------------------------------ */

/// The JSON of member `u` with membership `m`; `$2` is the caller's visible teams, `$3` the caller.
const USER_JSON: &str = "jsonb_build_object(
    'id', u.id,
    'name', u.name,
    'displayName', u.display_name,
    'email', u.email,
    'avatarUrl', u.avatar_url,
    'timezone', u.timezone,
    'role', m.role,
    'active', m.active,
    'isMe', u.id = $3,
    'joinedAt', m.joined_at,
    'teams', (SELECT coalesce(jsonb_agg(team_ref(t.id) ORDER BY t.name), '[]')
        FROM team_members tm JOIN teams t ON t.id = tm.team_id
        WHERE tm.user_id = u.id AND t.workspace_id = m.workspace_id AND t.archived_at IS NULL AND t.id = ANY($2)))";

const MEMBER_FROM: &str = "FROM users u JOIN workspace_members m ON m.user_id = u.id";

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ListUsersArgs {
    /// Part of a name, display name or email.
    query: Option<String>,
    /// Also list removed members.
    #[serde(default)]
    include_inactive: bool,
    /// Only members of this team (key, name or id).
    team: Option<String>,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list_users(ctx: Ctx, args: ListUsersArgs) -> ApiResult<Value> {
    let page = Page { limit: args.limit, cursor: args.cursor };
    let team = match &args.team {
        Some(team) => Some(resolve::team(&ctx, team).await?),
        None => None,
    };
    let query = args.query.as_deref().map(str::trim).filter(|q| !q.is_empty()).map(|q| format!("%{q}%"));
    let teams = ctx.team_ids().await?.to_vec();
    let sql = format!(
        "SELECT {USER_JSON} {MEMBER_FROM} WHERE m.workspace_id = $1 AND ($4 OR m.active)
         AND ($5::text IS NULL OR u.name ILIKE $5 OR u.display_name ILIKE $5 OR u.email ILIKE $5)
         AND ($6::uuid IS NULL OR EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id = $6 AND tm.user_id = u.id))
         ORDER BY lower(u.name), u.email LIMIT $7 OFFSET $8"
    );
    let nodes: Vec<Value> = sqlx::query_scalar(&sql)
        .bind(ctx.workspace)
        .bind(&teams)
        .bind(ctx.user)
        .bind(args.include_inactive)
        .bind(query)
        .bind(team)
        .bind(page.limit() + 1)
        .bind(page.offset()?)
        .fetch_all(ctx.db())
        .await?;
    page.wrap(nodes)
}

/// An active member by id, name, email or "me", or a removed member by id.
async fn member(ctx: &Ctx, value: &str) -> ApiResult<Uuid> {
    let found = resolve::user(ctx, value).await;
    let Some(id) = as_uuid(value).filter(|_| found.is_err()) else { return found };
    let removed: bool =
        sqlx::query_scalar("SELECT EXISTS (SELECT 1 FROM workspace_members WHERE workspace_id = $1 AND user_id = $2)")
            .bind(ctx.workspace)
            .bind(id)
            .fetch_one(ctx.db())
            .await?;
    if removed { Ok(id) } else { found }
}

async fn user_value(ctx: &Ctx, db: &mut PgConnection, user: Uuid) -> ApiResult<Value> {
    let teams = ctx.team_ids().await?.to_vec();
    let sql = format!(
        "SELECT {USER_JSON} || jsonb_build_object(
            'openIssueCount', (SELECT count(*) FROM issues i JOIN workflow_states s ON s.id = i.state_id
                WHERE i.workspace_id = m.workspace_id AND i.assignee_id = u.id AND i.trashed_at IS NULL
                AND i.archived_at IS NULL AND s.type NOT IN ('completed', 'canceled')))
         {MEMBER_FROM} WHERE m.workspace_id = $1 AND u.id = $4"
    );
    let value: Option<Value> = sqlx::query_scalar(&sql)
        .bind(ctx.workspace)
        .bind(&teams)
        .bind(ctx.user)
        .bind(user)
        .fetch_optional(&mut *db)
        .await?;
    value.ok_or_else(|| ApiError::not_found("No such member"))
}

async fn get_user(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let user = member(&ctx, &args.id).await?;
    let mut db = ctx.db().acquire().await?;
    user_value(&ctx, &mut db, user).await
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct UpdateUserArgs {
    /// The member: id, name, email or "me".
    id: String,
    /// owner, admin, member or guest. Needs an admin; owner changes need an owner.
    role: Option<String>,
    /// Your full name.
    name: Option<String>,
    /// Your short handle, used for @mentions.
    display_name: Option<String>,
    /// Your IANA timezone, like Europe/Berlin; null clears it.
    #[serde(default, deserialize_with = "nullable")]
    #[schemars(with = "Option<String>")]
    timezone: Option<Option<String>>,
}

async fn active_owners(db: &mut PgConnection, workspace: Uuid) -> ApiResult<i64> {
    Ok(sqlx::query_scalar(
        "SELECT count(*) FROM workspace_members WHERE workspace_id = $1 AND role = 'owner' AND active",
    )
    .bind(workspace)
    .fetch_one(&mut *db)
    .await?)
}

async fn role_of(db: &mut PgConnection, workspace: Uuid, user: Uuid) -> ApiResult<Role> {
    let role: String =
        sqlx::query_scalar("SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2 FOR UPDATE")
            .bind(workspace)
            .bind(user)
            .fetch_one(&mut *db)
            .await?;
    Role::parse(&role).ok_or_else(|| ApiError::bad_request("Unknown role"))
}

async fn update_user(ctx: Ctx, args: UpdateUserArgs) -> ApiResult<Value> {
    let user = resolve::user(&ctx, &args.id).await?;
    let profile_change = args.name.is_some() || args.display_name.is_some() || args.timezone.is_some();
    if profile_change && user != ctx.user {
        return Err(ApiError::forbidden("You can only change your own name, display name and timezone"));
    }
    let mut tx = ctx.db().begin().await?;
    if let Some(role) = &args.role {
        ctx.require(Access::Admin)?;
        let new =
            Role::parse(role.trim()).ok_or_else(|| ApiError::bad_request("role is owner, admin, member or guest"))?;
        let old = role_of(&mut tx, ctx.workspace, user).await?;
        if (new == Role::Owner || old == Role::Owner) && new != old && ctx.role != Role::Owner {
            return Err(ApiError::forbidden("Only owners can make someone an owner or take it away"));
        }
        if old == Role::Owner && new != Role::Owner && active_owners(&mut tx, ctx.workspace).await? <= 1 {
            return Err(ApiError::bad_request("The workspace needs an owner; make someone else owner first"));
        }
        if new != old {
            sqlx::query("UPDATE workspace_members SET role = $3 WHERE workspace_id = $1 AND user_id = $2")
                .bind(ctx.workspace)
                .bind(user)
                .bind(new.as_str())
                .execute(&mut *tx)
                .await?;
            let metadata = json!({ "user": user, "from": old.as_str(), "to": new.as_str() });
            events::audit(&mut tx, &ctx, "member.roleChanged", metadata).await?;
        }
    }
    if let Some(name) = &args.name {
        let name = non_empty(name, "name")?;
        sqlx::query("UPDATE users SET name = $2 WHERE id = $1").bind(user).bind(name).execute(&mut *tx).await?;
    }
    if let Some(display_name) = &args.display_name {
        let display_name = non_empty(display_name.trim_start_matches('@'), "displayName")?;
        crate::ensure!(
            !display_name.contains(char::is_whitespace),
            "A display name is one word, used for @mentions, like \"ann\""
        );
        sqlx::query("UPDATE users SET display_name = $2 WHERE id = $1")
            .bind(user)
            .bind(display_name)
            .execute(&mut *tx)
            .await?;
    }
    if let Some(timezone) = &args.timezone {
        let timezone = timezone.as_deref().map(str::trim).filter(|t| !t.is_empty());
        if let Some(zone) = timezone {
            check_timezone(&mut tx, zone).await?;
        }
        sqlx::query("UPDATE users SET timezone = $2 WHERE id = $1").bind(user).bind(timezone).execute(&mut *tx).await?;
    }
    let value = user_value(&ctx, &mut tx, user).await?;
    tx.commit().await?;
    Ok(value)
}

/// Accepts IANA timezone names Postgres knows, like Europe/Berlin or UTC.
pub async fn check_timezone(db: &mut PgConnection, zone: &str) -> ApiResult<()> {
    let known: bool = sqlx::query_scalar("SELECT EXISTS (SELECT 1 FROM pg_timezone_names WHERE name = $1)")
        .bind(zone)
        .fetch_one(&mut *db)
        .await?;
    crate::ensure!(known, "\"{zone}\" isn't a timezone; use an IANA name like Europe/Berlin or UTC");
    Ok(())
}

async fn remove_user(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let user = resolve::user(&ctx, &args.id).await?;
    let mut tx = ctx.db().begin().await?;
    let role = role_of(&mut tx, ctx.workspace, user).await?;
    if role == Role::Owner {
        if ctx.role != Role::Owner {
            return Err(ApiError::forbidden("Only owners can remove an owner"));
        }
        if active_owners(&mut tx, ctx.workspace).await? <= 1 {
            return Err(ApiError::bad_request("The last owner can't be removed; make someone else owner first"));
        }
    }
    sqlx::query("UPDATE workspace_members SET active = false WHERE workspace_id = $1 AND user_id = $2")
        .bind(ctx.workspace)
        .bind(user)
        .execute(&mut *tx)
        .await?;
    sqlx::query(
        "DELETE FROM team_members tm USING teams t WHERE t.id = tm.team_id AND t.workspace_id = $1 AND tm.user_id = $2",
    )
    .bind(ctx.workspace)
    .bind(user)
    .execute(&mut *tx)
    .await?;
    sqlx::query(
        "UPDATE teams SET triage_responsible_ids = array_remove(triage_responsible_ids, $2)
         WHERE workspace_id = $1 AND $2 = ANY(triage_responsible_ids)",
    )
    .bind(ctx.workspace)
    .bind(user)
    .execute(&mut *tx)
    .await?;
    let email: String =
        sqlx::query_scalar("SELECT email FROM users WHERE id = $1").bind(user).fetch_one(&mut *tx).await?;
    events::audit(&mut tx, &ctx, "member.removed", json!({ "user": user, "email": email, "role": role.as_str() }))
        .await?;
    tx.commit().await?;
    Ok(json!({ "removed": user }))
}

/* ------------------------------------------------------------------ */
/*                             Invitations                            */
/* ------------------------------------------------------------------ */

const INVITATION_JSON: &str = "jsonb_build_object(
    'id', i.id,
    'email', i.email,
    'role', i.role,
    'teams', (SELECT coalesce(jsonb_agg(team_ref(t.id) ORDER BY t.name), '[]') FROM teams t WHERE t.id = ANY(i.team_ids)),
    'inviter', user_ref(i.inviter_id),
    'createdAt', i.created_at,
    'expiresAt', i.expires_at)";

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ListInvitationsArgs {
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list_invitations(ctx: Ctx, args: ListInvitationsArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let page = Page { limit: args.limit, cursor: args.cursor };
    let sql = format!(
        "SELECT {INVITATION_JSON} FROM invitations i WHERE i.workspace_id = $1 AND i.expires_at > now()
         ORDER BY i.created_at DESC LIMIT $2 OFFSET $3"
    );
    let nodes: Vec<Value> = sqlx::query_scalar(&sql)
        .bind(ctx.workspace)
        .bind(page.limit() + 1)
        .bind(page.offset()?)
        .fetch_all(ctx.db())
        .await?;
    page.wrap(nodes)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct InviteUserArgs {
    /// Their email address; they sign in with the Google account of this email.
    email: String,
    /// member (default), admin (only admins invite admins) or guest.
    role: Option<String>,
    /// Teams they join (keys, names or ids). Required for guests.
    #[serde(default)]
    teams: Vec<String>,
}

async fn invite_user(ctx: Ctx, args: InviteUserArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let email = args.email.trim().to_lowercase();
    let valid = email.split_once('@').is_some_and(|(local, domain)| {
        !local.is_empty() && domain.contains('.') && !domain.starts_with('.') && !domain.ends_with('.')
    });
    crate::ensure!(valid && !email.contains(char::is_whitespace), "\"{email}\" isn't an email address");
    let role = match args.role.as_deref().map(str::trim).unwrap_or("member") {
        "member" => Role::Member,
        "guest" => Role::Guest,
        "admin" => {
            ctx.require(Access::Admin).map_err(|_| ApiError::forbidden("Only admins can invite admins"))?;
            Role::Admin
        }
        other => return Err(ApiError::bad_request(format!("role is member, admin or guest, not \"{other}\""))),
    };
    let mut teams = Vec::new();
    for team in &args.teams {
        let id = resolve::team(&ctx, team).await?;
        if !teams.contains(&id) {
            teams.push(id);
        }
    }
    crate::ensure!(role != Role::Guest || !teams.is_empty(), "Guests only see their teams; name at least one in teams");
    let mut tx = ctx.db().begin().await?;
    let member: bool = sqlx::query_scalar(
        "SELECT EXISTS (SELECT 1 FROM users u JOIN workspace_members m ON m.user_id = u.id
            WHERE m.workspace_id = $1 AND m.active AND lower(u.email) = $2)",
    )
    .bind(ctx.workspace)
    .bind(&email)
    .fetch_one(&mut *tx)
    .await?;
    if member {
        return Err(ApiError::conflict(format!("{email} is already a member of this workspace")));
    }
    let id: Uuid = sqlx::query_scalar(
        "INSERT INTO invitations (workspace_id, email, role, team_ids, inviter_id, expires_at)
         VALUES ($1, $2, $3, $4, $5, now() + interval '14 days')
         ON CONFLICT (workspace_id, email) DO UPDATE SET role = excluded.role, team_ids = excluded.team_ids,
            inviter_id = excluded.inviter_id, created_at = now(), expires_at = excluded.expires_at
         RETURNING id",
    )
    .bind(ctx.workspace)
    .bind(&email)
    .bind(role.as_str())
    .bind(&teams)
    .bind(ctx.user)
    .fetch_one(&mut *tx)
    .await?;
    events::audit(&mut tx, &ctx, "invitation.created", json!({ "email": email, "role": role.as_str() })).await?;
    let value: Value = sqlx::query_scalar(&format!("SELECT {INVITATION_JSON} FROM invitations i WHERE i.id = $1"))
        .bind(id)
        .fetch_one(&mut *tx)
        .await?;
    tx.commit().await?;
    Ok(value)
}

async fn delete_invitation(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    ctx.require_member()?;
    let id = resolve::by_id(&ctx, "invitations", "invitation", &args.id).await?;
    let inviter: Option<Uuid> =
        sqlx::query_scalar("SELECT inviter_id FROM invitations WHERE id = $1").bind(id).fetch_one(ctx.db()).await?;
    if inviter != Some(ctx.user) && ctx.role < Role::Admin {
        return Err(ApiError::forbidden("Only admins can withdraw invitations someone else sent"));
    }
    sqlx::query("DELETE FROM invitations WHERE id = $1").bind(id).execute(ctx.db()).await?;
    Ok(json!({ "deleted": id }))
}

/* ------------------------------------------------------------------ */
/*                              Audit log                             */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct AuditLogArgs {
    /// Only entries of this type, e.g. member.roleChanged, member.removed, invitation.created,
    /// team.archived, workspace.updated.
    #[serde(rename = "type")]
    kind: Option<String>,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn get_audit_log(ctx: Ctx, args: AuditLogArgs) -> ApiResult<Value> {
    let page = Page { limit: args.limit, cursor: args.cursor };
    let nodes: Vec<Value> = sqlx::query_scalar(
        "SELECT jsonb_build_object('id', a.id, 'type', a.type, 'actor', user_ref(a.actor_id), 'metadata', a.metadata,
            'createdAt', a.created_at)
         FROM audit_entries a WHERE a.workspace_id = $1 AND ($2::text IS NULL OR a.type = $2)
         ORDER BY a.id DESC LIMIT $3 OFFSET $4",
    )
    .bind(ctx.workspace)
    .bind(args.kind.as_deref().map(str::trim))
    .bind(page.limit() + 1)
    .bind(page.offset()?)
    .fetch_all(ctx.db())
    .await?;
    page.wrap(nodes)
}
