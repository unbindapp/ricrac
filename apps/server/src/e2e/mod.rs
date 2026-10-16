//! End-to-end tests: each starts on a fresh database (`#[sqlx::test]`, which needs DATABASE_URL
//! pointing at the compose Postgres) with a workspace, and calls operations as the API does.

mod conversation;
mod http;
mod integrations;
mod issues;
mod planning;
mod projects;
mod teams;
mod workspace;

use serde_json::{Value, json};
use sqlx::PgPool;
use uuid::Uuid;

use crate::config::Config;
use crate::ctx::{Access, Ctx};
use crate::error::ApiError;
use crate::state::AppState;
use crate::{ops, seed};

pub fn config() -> Config {
    Config {
        port: 0,
        database_url: String::new(),
        public_url: "http://localhost:3000".into(),
        secret_key: "test-secret-key-that-is-long-enough-123".into(),
        google_client_id: None,
        google_client_secret: None,
        allowed_emails: vec![],
        max_upload_size: 1024 * 1024,
        secure_cookies: None,
    }
}

/// A workspace "acme" owned by Ann, with the team ENG.
pub struct World {
    pub state: AppState,
    pub workspace: Uuid,
    pub ann: Ctx,
}

pub async fn world(db: PgPool) -> World {
    let _ = tracing_subscriber::fmt().with_test_writer().with_env_filter("warn").try_init();
    let state = AppState::new(db, config());
    let ann = add_user(&state, "ann@acme.test", "Ann Smith").await;
    let mut tx = state.db.begin().await.unwrap();
    let workspace: Uuid =
        sqlx::query_scalar("INSERT INTO workspaces (name, url_key) VALUES ('Acme', 'acme') RETURNING id")
            .fetch_one(&mut *tx)
            .await
            .unwrap();
    sqlx::query("INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')")
        .bind(workspace)
        .bind(ann)
        .execute(&mut *tx)
        .await
        .unwrap();
    seed::workspace_defaults(&mut tx, workspace).await.unwrap();
    seed::create_team(&mut tx, workspace, ann, "Engineering", "ENG").await.unwrap();
    tx.commit().await.unwrap();
    let ann = ctx(&state, ann, workspace).await;
    World { state, workspace, ann }
}

pub async fn add_user(state: &AppState, email: &str, name: &str) -> Uuid {
    sqlx::query_scalar("INSERT INTO users (email, name, display_name) VALUES ($1, $2, $3) RETURNING id")
        .bind(email)
        .bind(name)
        .bind(crate::google::display_name(name, email))
        .fetch_one(&state.db)
        .await
        .unwrap()
}

pub async fn ctx(state: &AppState, user: Uuid, workspace: Uuid) -> Ctx {
    Ctx::load(state, user, workspace, vec![Access::Read, Access::Write, Access::Admin]).await.unwrap().unwrap()
}

impl World {
    /// Adds someone to the workspace with `role` (member, admin or guest).
    pub async fn member(&self, email: &str, name: &str, role: &str) -> Ctx {
        let user = add_user(&self.state, email, name).await;
        sqlx::query("INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)")
            .bind(self.workspace)
            .bind(user)
            .bind(role)
            .execute(&self.state.db)
            .await
            .unwrap();
        ctx(&self.state, user, self.workspace).await
    }
}

/// Calls an operation and returns its result, failing the test on an error.
pub async fn call(ctx: &Ctx, name: &str, args: Value) -> Value {
    match try_call(ctx, name, args).await {
        Ok(value) => value,
        Err(e) => panic!("{name} failed: {}", e.message),
    }
}

pub async fn try_call(ctx: &Ctx, name: &str, args: Value) -> Result<Value, ApiError> {
    let op = ops::find(name).unwrap_or_else(|| panic!("no operation {name}"));
    op.call(ctx.clone(), args).await
}

/// Calls an operation that must fail, returning the error message.
pub async fn fails(ctx: &Ctx, name: &str, args: Value) -> String {
    match try_call(ctx, name, args).await {
        Ok(value) => panic!("{name} should have failed but returned {value}"),
        Err(e) => e.message,
    }
}

pub fn identifiers(list: &Value) -> Vec<String> {
    list["nodes"].as_array().unwrap().iter().map(|n| n["identifier"].as_str().unwrap().to_string()).collect()
}

#[allow(dead_code)]
pub fn empty() -> Value {
    json!({})
}
