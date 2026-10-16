//! Helpers shared by the operations: optional-or-null arguments, dates, paging, ids and text
//! patches.

use base64::Engine;
use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use chrono::{DateTime, Duration, NaiveDate, Utc};
use rand::Rng;
use schemars::JsonSchema;
use serde::{Deserialize, Deserializer};
use serde_json::{Value, json};

use crate::error::{ApiError, ApiResult};

/// For `Option<Option<T>>` fields: missing stays `None`, `null` becomes `Some(None)` (clear it).
pub fn nullable<'de, T, D>(deserializer: D) -> Result<Option<Option<T>>, D::Error>
where
    T: Deserialize<'de>,
    D: Deserializer<'de>,
{
    Option::<T>::deserialize(deserializer).map(Some)
}

/// A random 12-character id for URLs (projects, documents, views, ...).
pub fn slug_id() -> String {
    const ALPHABET: &[u8] = b"0123456789abcdef";
    let mut rng = rand::thread_rng();
    (0..12).map(|_| ALPHABET[rng.gen_range(0..ALPHABET.len())] as char).collect()
}

pub fn parse_date(value: &str) -> ApiResult<NaiveDate> {
    let value = value.trim();
    if let Ok(date) = NaiveDate::parse_from_str(value, "%Y-%m-%d") {
        return Ok(date);
    }
    if let Ok(at) = parse_datetime(value) {
        return Ok(at.date_naive());
    }
    Err(ApiError::bad_request(format!("\"{value}\" isn't a date; use YYYY-MM-DD")))
}

/// An ISO 8601 date or time, or a duration relative to now such as `-P1D`, `P2W` or `-PT3H`.
pub fn parse_datetime(value: &str) -> ApiResult<DateTime<Utc>> {
    let value = value.trim();
    if let Ok(at) = DateTime::parse_from_rfc3339(value) {
        return Ok(at.with_timezone(&Utc));
    }
    if let Ok(date) = NaiveDate::parse_from_str(value, "%Y-%m-%d") {
        return Ok(date.and_hms_opt(0, 0, 0).expect("midnight exists").and_utc());
    }
    if let Some(duration) = parse_duration(value) {
        return Ok(Utc::now() + duration);
    }
    Err(ApiError::bad_request(format!(
        "\"{value}\" isn't a time; use ISO 8601 (2026-01-31 or 2026-01-31T12:00:00Z) or a duration like -P1D"
    )))
}

fn parse_duration(value: &str) -> Option<Duration> {
    let (negative, rest) = match value.strip_prefix('-') {
        Some(rest) => (true, rest),
        None => (false, value.strip_prefix('+').unwrap_or(value)),
    };
    let rest = rest.strip_prefix('P').or_else(|| rest.strip_prefix('p'))?;
    let (date_part, time_part) = match rest.split_once(['T', 't']) {
        Some((d, t)) => (d, Some(t)),
        None => (rest, None),
    };
    let mut total = Duration::zero();
    let mut parse = |part: &str, units: &[(char, i64)]| -> Option<()> {
        let mut number = String::new();
        for c in part.chars() {
            if c.is_ascii_digit() || c == '.' {
                number.push(c);
                continue;
            }
            let seconds = units.iter().find(|(u, _)| u.eq_ignore_ascii_case(&c))?.1;
            let n: f64 = number.parse().ok()?;
            total += Duration::milliseconds((n * seconds as f64 * 1000.0) as i64);
            number.clear();
        }
        number.is_empty().then_some(())
    };
    parse(date_part, &[('Y', 365 * 86400), ('M', 30 * 86400), ('W', 7 * 86400), ('D', 86400)])?;
    if let Some(time) = time_part {
        parse(time, &[('H', 3600), ('M', 60), ('S', 1)])?;
    }
    if total.is_zero() {
        return None;
    }
    Some(if negative { -total } else { total })
}

/// Paging arguments, flattened into list operations.
#[derive(Deserialize, JsonSchema, Default)]
#[serde(rename_all = "camelCase")]
pub struct Page {
    /// How many to return, 50 by default, at most 250.
    pub limit: Option<i64>,
    /// The `endCursor` of the previous page, to continue after it.
    pub cursor: Option<String>,
}

impl Page {
    pub fn limit(&self) -> i64 {
        self.limit.unwrap_or(50).clamp(1, 250)
    }

    pub fn offset(&self) -> ApiResult<i64> {
        let Some(cursor) = &self.cursor else { return Ok(0) };
        let invalid = || ApiError::bad_request("Invalid cursor; pass the endCursor of the previous page");
        let bytes = URL_SAFE_NO_PAD.decode(cursor).map_err(|_| invalid())?;
        let text = String::from_utf8(bytes).map_err(|_| invalid())?;
        text.strip_prefix("o:").and_then(|n| n.parse().ok()).ok_or_else(invalid)
    }

    /// Fetch `limit() + 1` rows from `offset()`; this trims the extra row and says whether
    /// there are more.
    pub fn wrap(&self, mut nodes: Vec<Value>) -> ApiResult<Value> {
        let limit = self.limit() as usize;
        let has_next = nodes.len() > limit;
        nodes.truncate(limit);
        let end = self.offset()? + nodes.len() as i64;
        Ok(json!({
            "nodes": nodes,
            "pageInfo": {
                "hasNextPage": has_next,
                "endCursor": has_next.then(|| URL_SAFE_NO_PAD.encode(format!("o:{end}"))),
            }
        }))
    }
}

/// One edit of a Markdown text, applied in order with the others of a `patch`.
#[derive(Deserialize, JsonSchema, Clone)]
#[serde(tag = "op", rename_all = "snake_case")]
pub enum TextEdit {
    /// Replace text that occurs exactly once (or every occurrence with replace_all).
    Replace {
        old_string: String,
        new_string: String,
        #[serde(default)]
        replace_all: bool,
    },
    /// Insert text directly before an anchor that occurs exactly once.
    InsertBefore { anchor: String, text: String },
    /// Insert text directly after an anchor that occurs exactly once.
    InsertAfter { anchor: String, text: String },
    /// Insert text at the very start.
    Prepend { text: String },
    /// Insert text at the very end.
    Append { text: String },
    /// Replace from `from` (inclusive) up to `to` (exclusive, kept), each occurring once.
    ReplaceRange { from: String, to: String, new_string: String },
}

fn find_once(content: &str, needle: &str) -> ApiResult<usize> {
    if needle.is_empty() {
        return Err(ApiError::bad_request("Patch anchors can't be empty"));
    }
    let mut matches = content.match_indices(needle);
    let Some((at, _)) = matches.next() else {
        return Err(ApiError::bad_request(format!("Patch text not found: {needle:?}")));
    };
    if matches.next().is_some() {
        return Err(ApiError::bad_request(format!("Patch text occurs more than once, add context: {needle:?}")));
    }
    Ok(at)
}

/// Applies every edit or none.
pub fn apply_patch(content: &str, edits: &[TextEdit]) -> ApiResult<String> {
    let mut text = content.to_string();
    for edit in edits {
        text = match edit {
            TextEdit::Replace { old_string, new_string, replace_all: true } => {
                if old_string.is_empty() || !text.contains(old_string.as_str()) {
                    return Err(ApiError::bad_request(format!("Patch text not found: {old_string:?}")));
                }
                text.replace(old_string.as_str(), new_string)
            }
            TextEdit::Replace { old_string, new_string, replace_all: false } => {
                let at = find_once(&text, old_string)?;
                format!("{}{new_string}{}", &text[..at], &text[at + old_string.len()..])
            }
            TextEdit::InsertBefore { anchor, text: insert } => {
                let at = find_once(&text, anchor)?;
                format!("{}{insert}{}", &text[..at], &text[at..])
            }
            TextEdit::InsertAfter { anchor, text: insert } => {
                let at = find_once(&text, anchor)? + anchor.len();
                format!("{}{insert}{}", &text[..at], &text[at..])
            }
            TextEdit::Prepend { text: insert } => format!("{insert}{text}"),
            TextEdit::Append { text: insert } => format!("{text}{insert}"),
            TextEdit::ReplaceRange { from, to, new_string } => {
                let start = find_once(&text, from)?;
                let end = start + find_once(&text[start..], to)?;
                format!("{}{new_string}{}", &text[..start], &text[end..])
            }
        };
    }
    Ok(text)
}

/// The new content from either a full replacement or a patch of the current one.
pub fn new_content(
    current: &str,
    replacement: Option<String>,
    patch: Option<&[TextEdit]>,
) -> ApiResult<Option<String>> {
    match (replacement, patch) {
        (Some(_), Some(_)) => Err(ApiError::bad_request("Pass either the full content or a patch, not both")),
        (Some(text), None) => Ok(Some(text)),
        (None, Some(edits)) => apply_patch(current, edits).map(Some),
        (None, None) => Ok(None),
    }
}

pub fn check_color(color: &str) -> ApiResult<String> {
    let hex = color.trim().trim_start_matches('#');
    if !(hex.len() == 6 || hex.len() == 3) || !hex.bytes().all(|b| b.is_ascii_hexdigit()) {
        return Err(ApiError::bad_request(format!("\"{color}\" isn't a hex color like #5e6ad2")));
    }
    Ok(format!("#{}", hex.to_lowercase()))
}

/// Colors for things created without one.
pub fn pick_color(seed: &str) -> String {
    const COLORS: [&str; 12] = [
        "#5e6ad2", "#26b5ce", "#0f783c", "#4cb782", "#f2c94c", "#f2994a", "#eb5757", "#bb87fc", "#95a2b3", "#ef4b9f",
        "#6d8a9d", "#c4a46b",
    ];
    let index = seed.bytes().fold(0usize, |acc, b| acc.wrapping_mul(31).wrapping_add(b as usize));
    COLORS[index % COLORS.len()].to_string()
}

pub fn non_empty(value: &str, what: &str) -> ApiResult<String> {
    let value = value.trim();
    if value.is_empty() {
        return Err(ApiError::bad_request(format!("{what} can't be empty")));
    }
    Ok(value.to_string())
}

pub const PRIORITY_NAMES: [&str; 5] = ["No priority", "Urgent", "High", "Medium", "Low"];

pub fn check_priority(priority: i32) -> ApiResult<i32> {
    if !(0..=4).contains(&priority) {
        return Err(ApiError::bad_request("priority is 0 (none), 1 (urgent), 2 (high), 3 (medium) or 4 (low)"));
    }
    Ok(priority)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn durations() {
        let day = parse_duration("-P1D").unwrap();
        assert_eq!(day, -Duration::days(1));
        assert_eq!(parse_duration("P2W").unwrap(), Duration::days(14));
        assert_eq!(parse_duration("-PT3H30M").unwrap(), -(Duration::hours(3) + Duration::minutes(30)));
        assert!(parse_duration("P").is_none());
        assert!(parse_duration("1D").is_none());
        assert!(parse_datetime("2026-01-31").is_ok());
        assert!(parse_datetime("2026-01-31T10:00:00+02:00").is_ok());
        assert!(parse_datetime("yesterday").is_err());
    }

    #[test]
    fn patches_apply_in_order_or_not_at_all() {
        let text = "# Title\n\nOne\n\nTwo\n";
        let edits = vec![
            TextEdit::Replace { old_string: "One".into(), new_string: "1".into(), replace_all: false },
            TextEdit::InsertAfter { anchor: "Two".into(), text: " and three".into() },
            TextEdit::Append { text: "End\n".into() },
        ];
        assert_eq!(apply_patch(text, &edits).unwrap(), "# Title\n\n1\n\nTwo and three\nEnd\n");
        let ambiguous =
            vec![TextEdit::Replace { old_string: "\n\n".into(), new_string: "".into(), replace_all: false }];
        assert!(apply_patch(text, &ambiguous).is_err());
        let range = vec![TextEdit::ReplaceRange { from: "One".into(), to: "Two".into(), new_string: "".into() }];
        assert_eq!(apply_patch(text, &range).unwrap(), "# Title\n\nTwo\n");
    }

    #[test]
    fn pages() {
        let page = Page { limit: Some(2), cursor: None };
        let wrapped = page.wrap(vec![json!(1), json!(2), json!(3)]).unwrap();
        assert_eq!(wrapped["nodes"], json!([1, 2]));
        let next = Page { limit: Some(2), cursor: wrapped["pageInfo"]["endCursor"].as_str().map(String::from) };
        assert_eq!(next.offset().unwrap(), 2);
        let last = next.wrap(vec![json!(3)]).unwrap();
        assert_eq!(last["pageInfo"]["hasNextPage"], false);
    }
}
