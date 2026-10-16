//! Who is calling and in which workspace. Every operation runs with a `Ctx`, whether it came
//! through the REST API, MCP or the dashboard.

use std::sync::Arc;

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

    pub fn db(&self) -> &PgPool {
        &self.state.db
    }

    pub fn public_url(&self) -> &str {
        &self.state.config.public_url
    }

    pub fn allows(&self, access: Access) -> bool {
        self.scopes.contains(&access)
    }

    pub fn require(&self, access: Access) -> ApiResult<()> {
        if !self.allows(access) {
            return Err(ApiError::forbidden(format!("This token lacks the \"{}\" scope", access.as_str())));
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
