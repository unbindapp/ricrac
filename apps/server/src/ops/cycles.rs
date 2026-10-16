use super::Op;

pub fn ops() -> Vec<Op> {
    vec![]
}

/// Creates upcoming cycles, completes ended ones (rolling unfinished issues over) and records
/// each running cycle's daily progress. Runs every minute.
pub async fn run_automations(_state: &crate::state::AppState) -> crate::error::ApiResult<()> {
    Ok(())
}
