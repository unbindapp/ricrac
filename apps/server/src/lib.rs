#![allow(clippy::type_complexity)]

pub mod account;
pub mod app;
pub mod config;
pub mod ctx;
#[cfg(test)]
mod e2e;
pub mod error;
pub mod files;
pub mod google;
pub mod jobs;
pub mod keys;
pub mod mcp;
pub mod oauth;
pub mod ops;
pub mod rest;
pub mod seed;
pub mod spa;
pub mod state;

use std::str::FromStr;

use sqlx::postgres::{PgConnectOptions, PgPoolOptions};

pub use state::AppState;

pub async fn connect(database_url: &str) -> Result<sqlx::PgPool, sqlx::Error> {
    let options = PgConnectOptions::from_str(database_url)?.options([("timezone", "UTC")]);
    let db = PgPoolOptions::new().max_connections(20).connect_with(options).await?;
    sqlx::migrate!().run(&db).await?;
    Ok(db)
}
