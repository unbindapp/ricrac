use super::Op;

pub fn ops() -> Vec<Op> {
    vec![]
}

/// The comment threads on the thing whose id is in `column` (e.g. `issue_id`), oldest first,
/// each with its replies.
pub async fn threads(
    _ctx: &crate::ctx::Ctx,
    _db: &mut sqlx::PgConnection,
    _column: &str,
    _id: uuid::Uuid,
) -> crate::error::ApiResult<serde_json::Value> {
    Ok(serde_json::json!([]))
}
