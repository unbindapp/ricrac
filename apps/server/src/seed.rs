//! What new workspaces and teams start with, as in Linear.

use sqlx::PgConnection;
use uuid::Uuid;

use crate::error::ApiResult;

/// Project statuses and a few labels.
pub async fn workspace_defaults(tx: &mut PgConnection, workspace: Uuid) -> ApiResult<()> {
    let statuses = [
        ("Backlog", "backlog", "#bec2c8"),
        ("Planned", "planned", "#e2e2e2"),
        ("In Progress", "started", "#f2c94c"),
        ("Paused", "paused", "#95a2b3"),
        ("Completed", "completed", "#5e6ad2"),
        ("Canceled", "canceled", "#95a2b3"),
    ];
    for (position, (name, kind, color)) in statuses.into_iter().enumerate() {
        sqlx::query(
            "INSERT INTO project_statuses (workspace_id, name, type, color, position) VALUES ($1, $2, $3, $4, $5)",
        )
        .bind(workspace)
        .bind(name)
        .bind(kind)
        .bind(color)
        .bind(position as f64)
        .execute(&mut *tx)
        .await?;
    }
    for (name, color) in [("Bug", "#eb5757"), ("Feature", "#bb87fc"), ("Improvement", "#4ea7fc")] {
        sqlx::query("INSERT INTO issue_labels (workspace_id, name, color) VALUES ($1, $2, $3)")
            .bind(workspace)
            .bind(name)
            .bind(color)
            .execute(&mut *tx)
            .await?;
    }
    for (position, (name, color)) in
        [("Active", "#4cb782"), ("Prospect", "#f2c94c"), ("Churned", "#95a2b3")].into_iter().enumerate()
    {
        sqlx::query("INSERT INTO customer_statuses (workspace_id, name, color, position) VALUES ($1, $2, $3, $4)")
            .bind(workspace)
            .bind(name)
            .bind(color)
            .bind(position as f64)
            .execute(&mut *tx)
            .await?;
    }
    Ok(())
}

/// The workflow every team starts with. New issues go to Backlog; issues closed by the
/// auto-close automation go to Canceled.
pub async fn team_defaults(tx: &mut PgConnection, team: Uuid) -> ApiResult<()> {
    let states = [
        ("Backlog", "backlog", "#bec2c8"),
        ("Todo", "unstarted", "#e2e2e2"),
        ("In Progress", "started", "#f2c94c"),
        ("In Review", "started", "#4cb782"),
        ("Done", "completed", "#5e6ad2"),
        ("Canceled", "canceled", "#95a2b3"),
        ("Duplicate", "canceled", "#95a2b3"),
    ];
    for (position, (name, kind, color)) in states.into_iter().enumerate() {
        sqlx::query("INSERT INTO workflow_states (team_id, name, type, color, position) VALUES ($1, $2, $3, $4, $5)")
            .bind(team)
            .bind(name)
            .bind(kind)
            .bind(color)
            .bind(position as f64)
            .execute(&mut *tx)
            .await?;
    }
    sqlx::query(
        "UPDATE teams SET
            default_issue_state_id = (SELECT id FROM workflow_states WHERE team_id = $1 AND name = 'Backlog'),
            auto_close_state_id = (SELECT id FROM workflow_states WHERE team_id = $1 AND name = 'Canceled')
         WHERE id = $1",
    )
    .bind(team)
    .execute(&mut *tx)
    .await?;
    Ok(())
}

/// Adds the Triage status a team needs once triage is turned on.
pub async fn ensure_triage_state(tx: &mut PgConnection, team: Uuid) -> ApiResult<Uuid> {
    let existing: Option<Uuid> = sqlx::query_scalar(
        "SELECT id FROM workflow_states WHERE team_id = $1 AND type = 'triage' AND archived_at IS NULL LIMIT 1",
    )
    .bind(team)
    .fetch_optional(&mut *tx)
    .await?;
    if let Some(id) = existing {
        return Ok(id);
    }
    Ok(sqlx::query_scalar(
        "INSERT INTO workflow_states (team_id, name, type, color, position)
         VALUES ($1, 'Triage', 'triage', '#fc7840', (SELECT coalesce(min(position), 0) - 1 FROM workflow_states WHERE team_id = $1))
         RETURNING id",
    )
    .bind(team)
    .fetch_one(&mut *tx)
    .await?)
}

/// A team key from a name: its initials, or its first letters, up to five, uppercase.
pub fn team_key(name: &str) -> String {
    let words: Vec<&str> = name.split_whitespace().filter(|w| w.chars().any(char::is_alphanumeric)).collect();
    let key: String = if words.len() > 1 {
        words.iter().filter_map(|w| w.chars().find(|c| c.is_ascii_alphanumeric())).take(5).collect()
    } else {
        name.chars().filter(char::is_ascii_alphanumeric).take(3).collect()
    };
    let key = key.to_uppercase();
    if key.is_empty() { "TEAM".into() } else { key }
}

/// Creates a team with the default workflow and the creator as its first member.
pub async fn create_team(
    tx: &mut PgConnection,
    workspace: Uuid,
    creator: Uuid,
    name: &str,
    key: &str,
) -> ApiResult<Uuid> {
    let team: Uuid = sqlx::query_scalar("INSERT INTO teams (workspace_id, name, key) VALUES ($1, $2, $3) RETURNING id")
        .bind(workspace)
        .bind(name)
        .bind(key.to_uppercase())
        .fetch_one(&mut *tx)
        .await?;
    team_defaults(tx, team).await?;
    sqlx::query("INSERT INTO team_members (team_id, user_id, owner) VALUES ($1, $2, true)")
        .bind(team)
        .bind(creator)
        .execute(&mut *tx)
        .await?;
    Ok(team)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn keys() {
        assert_eq!(team_key("Engineering"), "ENG");
        assert_eq!(team_key("Mobile Platform Team"), "MPT");
        assert_eq!(team_key("  "), "TEAM");
    }
}
