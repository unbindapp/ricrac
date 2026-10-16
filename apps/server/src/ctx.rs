//! Who is calling and in which workspace. Every operation runs with a `Ctx`, whether it came
//! through the REST API, MCP or the dashboard.

use std::sync::Arc;

use serde_json::Value;
use sqlx::PgPool;
use tokio::sync::OnceCell;
use uuid::Uuid;

use crate::error::{ApiError, ApiResult};
use crate::state::AppState;

#[derive(Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Debug)]
pub enum Role {
    Guest,
    Member,
    Admin,
    Owner,
}

impl Role {
    pub fn parse(value: &str) -> Option<Self> {
        match value {
            "guest" => Some(Self::Guest),
            "member" => Some(Self::Member),
            "admin" => Some(Self::Admin),
            "owner" => Some(Self::Owner),
            _ => None,
        }
    }

    pub fn as_str(self) -> &'static str {
        match self {
            Self::Guest => "guest",
            Self::Member => "member",
            Self::Admin => "admin",
            Self::Owner => "owner",
        }
    }
}

/// What a token may do. Sessions and keys without scopes may do everything their role allows.
#[derive(Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Debug)]
pub enum Access {
    Read,
    Write,
    Admin,
}

impl Access {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Read => "read",
            Self::Write => "write",
            Self::Admin => "admin",
        }
    }

    pub fn parse_scopes(scopes: &[String]) -> Vec<Access> {
        scopes
            .iter()
            .filter_map(|s| match s.as_str() {
                "read" => Some(Access::Read),
                "write" => Some(Access::Write),
                "admin" => Some(Access::Admin),
                _ => None,
            })
            .collect()
    }
}

pub const ALL_SCOPES: [&str; 3] = ["read", "write", "admin"];

fn scope_error(access: Access) -> ApiError {
    ApiError::forbidden(format!("This token lacks the \"{}\" scope", access.as_str()))
}

/// Who is calling and which workspaces their token reaches. Each operation then runs in one of
/// them, as a `Ctx`.
#[derive(Clone)]
pub struct Caller {
    pub state: AppState,
    pub user: Uuid,
    pub scopes: Vec<Access>,
    /// None reaches every workspace the user belongs to, including ones they join later.
    pub workspaces: Option<Vec<Uuid>>,
}

/// A workspace a caller can act in.
#[derive(sqlx::FromRow)]
pub struct Reached {
    pub id: Uuid,
    pub url_key: String,
    pub name: String,
    pub role: String,
}

impl Caller {
    /// A dashboard session: every workspace, everything the user's roles allow.
    pub fn session(state: &AppState, user: Uuid) -> Self {
        Self { state: state.clone(), user, scopes: vec![Access::Read, Access::Write, Access::Admin], workspaces: None }
    }

    pub fn require(&self, access: Access) -> ApiResult<()> {
        if !self.scopes.contains(&access) {
            return Err(scope_error(access));
        }
        Ok(())
    }

    /// The workspaces the token reaches that the user is still an active member of, by name.
    pub async fn reachable(&self) -> ApiResult<Vec<Reached>> {
        Ok(sqlx::query_as(
            "SELECT w.id, w.url_key, w.name, m.role FROM workspace_members m JOIN workspaces w ON w.id = m.workspace_id
             WHERE m.user_id = $1 AND m.active AND ($2::uuid[] IS NULL OR w.id = ANY($2)) ORDER BY lower(w.name)",
        )
        .bind(self.user)
        .bind(&self.workspaces)
        .fetch_all(&self.state.db)
        .await?)
    }

    /// The caller in one workspace: the one named (URL key or id), else the one the links among
    /// the arguments point to, else the only one the token reaches.
    pub async fn ctx(&self, named: Option<&str>, args: &Value) -> ApiResult<Ctx> {
        let reachable = self.reachable().await?;
        if reachable.is_empty() {
            return Err(ApiError::forbidden("This token doesn't reach any workspace you're a member of"));
        }
        let chosen = match named.map(str::trim).filter(|n| !n.is_empty()) {
            Some(name) => {
                let key = name.to_lowercase();
                reachable.iter().find(|w| w.url_key == key || w.id.to_string() == key).ok_or_else(|| {
                    ApiError::bad_request(format!("No workspace \"{name}\" here; use one of {}", choices(&reachable)))
                })?
            }
            None => match (linked(&reachable, args, &self.state.config.public_url), reachable.as_slice()) {
                (Some(workspace), _) | (None, [workspace]) => workspace,
                _ => {
                    return Err(ApiError::bad_request(format!(
                        "This token reaches several workspaces: pass \"workspace\" with one of {}",
                        choices(&reachable)
                    )));
                }
            },
        };
        Ok(Ctx {
            state: self.state.clone(),
            user: self.user,
            workspace: chosen.id,
            role: Role::parse(&chosen.role).unwrap_or(Role::Guest),
            scopes: self.scopes.clone(),
            teams: Arc::default(),
        })
    }
}

fn choices(reachable: &[Reached]) -> String {
    let names: Vec<String> = reachable.iter().map(|w| format!("\"{}\" ({})", w.url_key, w.name)).collect();
    names.join(", ")
}

/// The workspace of the links passed as arguments, like https://ricrac.example/acme/issue/ENG-1,
/// when they all point to the same one.
fn linked<'a>(reachable: &'a [Reached], args: &Value, base: &str) -> Option<&'a Reached> {
    let prefix = format!("{base}/");
    let mut found: Option<&Reached> = None;
    for value in args.as_object()?.values() {
        let items = match value {
            Value::Array(items) => items.iter().collect(),
            value => vec![value],
        };
        for link in items.into_iter().filter_map(Value::as_str).filter(|s| !s.contains(char::is_whitespace)) {
            let Some(key) = link.strip_prefix(&prefix).and_then(|rest| rest.split('/').next()) else { continue };
            let Some(workspace) = reachable.iter().find(|w| w.url_key == key) else { continue };
            if found.is_some_and(|f| f.id != workspace.id) {
                return None;
            }
            found = Some(workspace);
        }
    }
    found
}

#[derive(Clone)]
pub struct Ctx {
    pub state: AppState,
    pub user: Uuid,
    pub workspace: Uuid,
    pub role: Role,
    pub scopes: Vec<Access>,
    teams: Arc<OnceCell<Vec<Uuid>>>,
}

impl Ctx {
    /// The context of `user` in `workspace`, or None when they aren't an active member.
    pub async fn load(state: &AppState, user: Uuid, workspace: Uuid, scopes: Vec<Access>) -> ApiResult<Option<Self>> {
        let role: Option<String> = sqlx::query_scalar(
            "SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2 AND active",
        )
        .bind(workspace)
        .bind(user)
        .fetch_optional(&state.db)
        .await?;
        let Some(role) = role.as_deref().and_then(Role::parse) else { return Ok(None) };
        Ok(Some(Self { state: state.clone(), user, workspace, role, scopes, teams: Arc::default() }))
    }

    /// The same caller with nothing cached, as a new request would see it.
    pub fn fresh(&self) -> Self {
        Self { teams: Arc::default(), ..self.clone() }
    }

    pub fn db(&self) -> &PgPool {
        &self.state.db
    }

    pub fn public_url(&self) -> &str {
        &self.state.config.public_url
    }

    pub fn allows(&self, access: Access) -> bool {
        self.scopes.contains(&access)
    }

    /// The same caller, reaching only this workspace.
    pub fn caller(&self) -> Caller {
        Caller {
            state: self.state.clone(),
            user: self.user,
            scopes: self.scopes.clone(),
            workspaces: Some(vec![self.workspace]),
        }
    }

    pub fn require(&self, access: Access) -> ApiResult<()> {
        if !self.allows(access) {
            return Err(scope_error(access));
        }
        if access == Access::Admin {
            return self.require_admin();
        }
        Ok(())
    }

    pub fn require_admin(&self) -> ApiResult<()> {
        if self.role < Role::Admin {
            return Err(ApiError::forbidden("Only workspace admins can do this"));
        }
        Ok(())
    }

    /// Guests only work inside the teams they were added to.
    pub fn require_member(&self) -> ApiResult<()> {
        if self.role == Role::Guest {
            return Err(ApiError::forbidden("Guests can't do this"));
        }
        Ok(())
    }

    /// The teams whose issues this user can see: every public team (except for guests) and the
    /// private teams they belong to. Archived teams are included so their issues stay readable.
    pub async fn team_ids(&self) -> ApiResult<&[Uuid]> {
        let ids = self
            .teams
            .get_or_try_init(|| async {
                sqlx::query_scalar(
                    "SELECT t.id FROM teams t WHERE t.workspace_id = $1 AND (
                        ($2 AND NOT t.private)
                        OR EXISTS (SELECT 1 FROM team_members m WHERE m.team_id = t.id AND m.user_id = $3)
                     )",
                )
                .bind(self.workspace)
                .bind(self.role != Role::Guest)
                .bind(self.user)
                .fetch_all(self.db())
                .await
            })
            .await?;
        Ok(ids)
    }

    pub async fn can_see_team(&self, team: Uuid) -> ApiResult<bool> {
        Ok(self.team_ids().await?.contains(&team))
    }
}
