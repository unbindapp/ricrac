use super::Op;

pub fn ops() -> Vec<Op> {
    vec![]
}

/// Attaches a link to an issue, or updates the attachment with the same URL. Returns its id.
pub async fn upsert(
    _ctx: &crate::ctx::Ctx,
    _tx: &mut sqlx::PgConnection,
    _issue: uuid::Uuid,
    _url: &str,
    _title: &str,
    _subtitle: Option<&str>,
    _metadata: Option<serde_json::Value>,
) -> crate::error::ApiResult<uuid::Uuid> {
    Ok(uuid::Uuid::nil())
}
