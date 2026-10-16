//! Attachments (links on issues, unique per URL) and file uploads, which agents embed in
//! Markdown or attach to issues.

use base64::Engine;
use base64::engine::general_purpose::{STANDARD, STANDARD_NO_PAD};
use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Value, json};
use sqlx::PgConnection;
use uuid::Uuid;

use super::events;
use super::resolve::{self, as_uuid};
use super::util::{Page, non_empty};
use super::{IdArgs, Op, op};
use crate::ctx::{Access, Ctx};
use crate::error::{ApiError, ApiResult};
use crate::files::asset_url;
use crate::keys::{random_secret, secret_hash};

const INLINE_LIMIT: usize = 5 * 1024 * 1024;

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "create_attachment",
            Access::Write,
            &[("POST", "/attachments")],
            "Attach a link (pull request, design, doc, error report, ...) to an issue. Attaching a URL the issue already \
             has updates that attachment instead. The source (github, figma, slack, ...) is recognized from the URL.",
            create,
        ),
        op("get_attachment", Access::Read, &[("GET", "/attachments/{id}")], "Read an attachment by id.", get),
        op(
            "list_attachments",
            Access::Read,
            &[("GET", "/issues/{issue}/attachments")],
            "List an issue's attachments, oldest first.",
            list,
        ),
        op(
            "delete_attachment",
            Access::Write,
            &[("DELETE", "/attachments/{id}")],
            "Remove an attachment from its issue.",
            delete,
        ),
        op(
            "prepare_file_upload",
            Access::Write,
            &[("POST", "/files")],
            "Get a one-time URL to upload a file (screenshot, log, recording, ...) to, valid for 15 minutes. Upload it \
             by running the returned `curl` command with <file> replaced by the local path (an HTTP PUT of the raw bytes). \
             Then embed the assetUrl in Markdown (`![name](assetUrl)` for images, `[name](assetUrl)` otherwise) in a \
             description or comment, or attach it to an issue with create_attachment_from_upload. Agents that can't run \
             commands use upload_file instead.",
            prepare_upload,
        ),
        op(
            "upload_file",
            Access::Write,
            &[("POST", "/files/inline")],
            "Upload a small file (up to 5 MB) as base64 in the arguments, for agents that can't run commands; otherwise \
             prefer prepare_file_upload. Embed the returned assetUrl in Markdown (`![name](assetUrl)`) or attach it to an \
             issue with create_attachment_from_upload.",
            upload_inline,
        ),
        op(
            "create_attachment_from_upload",
            Access::Write,
            &[("POST", "/attachments/from-upload")],
            "Attach an uploaded file (from prepare_file_upload or upload_file) to an issue.",
            create_from_upload,
        ),
    ]
}

/* ------------------------------------------------------------------ */
/*                           Representation                           */
/* ------------------------------------------------------------------ */

const ATTACHMENT_JSON: &str = "jsonb_build_object('id', a.id, 'url', a.url, 'title', a.title, 'subtitle', a.subtitle,
    'sourceType', a.source_type, 'metadata', a.metadata, 'iconUrl', a.icon_url, 'creator', user_ref(a.creator_id),
    'issue', issue_ref(a.issue_id), 'createdAt', a.created_at, 'updatedAt', a.updated_at)";

async fn attachment_value(db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    let value: Option<Value> =
        sqlx::query_scalar(&format!("SELECT {ATTACHMENT_JSON} FROM attachments a WHERE a.id = $1"))
            .bind(id)
            .fetch_optional(&mut *db)
            .await?;
    value.ok_or_else(|| ApiError::not_found("No such attachment"))
}

/// The host of a URL, lowercase, without credentials or port.
fn host(url: &str) -> Option<String> {
    let rest = url.split_once("://")?.1;
    let authority = rest.split(['/', '?', '#']).next()?;
    let host = authority.rsplit('@').next()?;
    let host = match host.strip_prefix('[') {
        Some(ipv6) => ipv6.split(']').next()?,
        None => host.split(':').next()?,
    };
    (!host.is_empty()).then(|| host.to_lowercase())
}

fn source_type(url: &str, public_url: &str) -> &'static str {
    let Some(name) = host(url) else { return "link" };
    let rest = url.split_once("://").map_or("", |(_, r)| r);
    let path = rest.find(['/', '?', '#']).map_or("", |i| &rest[i..]);
    if host(public_url).as_deref() == Some(name.as_str()) && path.starts_with("/files/") {
        return "upload";
    }
    let is = |domain: &str| name == domain || name.ends_with(&format!(".{domain}"));
    let sources = [
        ("github.com", "github"),
        ("githubusercontent.com", "github"),
        ("gitlab.com", "gitlab"),
        ("figma.com", "figma"),
        ("slack.com", "slack"),
        ("sentry.io", "sentry"),
        ("notion.so", "notion"),
        ("notion.site", "notion"),
        ("loom.com", "loom"),
        ("linear.app", "linear"),
    ];
    if let Some((_, source)) = sources.iter().find(|(domain, _)| is(domain)) {
        return source;
    }
    if name.starts_with("gitlab.") {
        return "gitlab";
    }
    "link"
}

/* ------------------------------------------------------------------ */
/*                             Attachments                            */
/* ------------------------------------------------------------------ */

/// Attaches a link to an issue, or updates the attachment with the same URL. Returns its id.
pub async fn upsert(
    ctx: &Ctx,
    tx: &mut PgConnection,
    issue: Uuid,
    url: &str,
    title: &str,
    subtitle: Option<&str>,
    metadata: Option<Value>,
) -> ApiResult<Uuid> {
    save(ctx, tx, issue, url, title, subtitle, metadata, None).await
}

#[allow(clippy::too_many_arguments)]
async fn save(
    ctx: &Ctx,
    tx: &mut PgConnection,
    issue: Uuid,
    url: &str,
    title: &str,
    subtitle: Option<&str>,
    metadata: Option<Value>,
    icon_url: Option<&str>,
) -> ApiResult<Uuid> {
    let url = url.trim();
    if !(url.starts_with("https://") || url.starts_with("http://")) || host(url).is_none() {
        return Err(ApiError::bad_request(format!("\"{url}\" isn't a link; attachments need an http(s) URL")));
    }
    let title = non_empty(title, "title")?;
    if metadata.as_ref().is_some_and(|m| !m.is_object()) {
        return Err(ApiError::bad_request("metadata must be an object"));
    }
    let (id, created): (Uuid, bool) = sqlx::query_as(
        "INSERT INTO attachments (workspace_id, issue_id, url, title, subtitle, metadata, icon_url, source_type, creator_id)
         VALUES ($1, $2, $3, $4, $5, coalesce($6, '{}'), $7, $8, $9)
         ON CONFLICT (issue_id, url) DO UPDATE SET title = EXCLUDED.title,
            subtitle = coalesce($5, attachments.subtitle), metadata = coalesce($6, attachments.metadata),
            icon_url = coalesce($7, attachments.icon_url), updated_at = now()
         RETURNING id, xmax = 0",
    )
    .bind(ctx.workspace)
    .bind(issue)
    .bind(url)
    .bind(&title)
    .bind(subtitle.map(str::trim).filter(|s| !s.is_empty()))
    .bind(metadata)
    .bind(icon_url)
    .bind(source_type(url, ctx.public_url()))
    .bind(ctx.user)
    .fetch_one(&mut *tx)
    .await?;
    let value = attachment_value(tx, id).await?;
    let team: Uuid =
        sqlx::query_scalar("SELECT team_id FROM issues WHERE id = $1").bind(issue).fetch_one(&mut *tx).await?;
    events::emit(tx, ctx, "Attachment", if created { "create" } else { "update" }, &value, None, Some(team)).await?;
    Ok(id)
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct CreateAttachmentArgs {
    /// Issue identifier (ENG-123), id or link.
    issue: String,
    /// The link to attach.
    url: String,
    title: String,
    /// A second line, e.g. a PR's state or a file's size.
    subtitle: Option<String>,
    /// Any JSON object to keep with the attachment.
    metadata: Option<Value>,
    /// Link to an icon to show with it.
    icon_url: Option<String>,
}

async fn create(ctx: Ctx, args: CreateAttachmentArgs) -> ApiResult<Value> {
    let issue = resolve::issue(&ctx, &args.issue).await?;
    let mut tx = ctx.db().begin().await?;
    let id = save(
        &ctx,
        &mut tx,
        issue,
        &args.url,
        &args.title,
        args.subtitle.as_deref(),
        args.metadata,
        args.icon_url.as_deref(),
    )
    .await?;
    let value = attachment_value(&mut tx, id).await?;
    tx.commit().await?;
    Ok(value)
}

/// An attachment of the workspace on an issue the caller can see.
async fn find(ctx: &Ctx, value: &str) -> ApiResult<(Uuid, Uuid)> {
    let id = resolve::by_id(ctx, "attachments", "attachment", value).await?;
    let issue: Uuid =
        sqlx::query_scalar("SELECT issue_id FROM attachments WHERE id = $1").bind(id).fetch_one(ctx.db()).await?;
    if !resolve::can_see_issue(ctx, issue).await? {
        return Err(ApiError::not_found(format!("No attachment \"{}\"", value.trim())));
    }
    Ok((id, issue))
}

async fn get(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let (id, _) = find(&ctx, &args.id).await?;
    attachment_value(&mut *ctx.db().acquire().await?, id).await
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ListAttachmentsArgs {
    /// Issue identifier (ENG-123), id or link.
    issue: String,
    /// How many to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list(ctx: Ctx, args: ListAttachmentsArgs) -> ApiResult<Value> {
    let issue = resolve::issue(&ctx, &args.issue).await?;
    let page = Page { limit: args.limit, cursor: args.cursor };
    let nodes: Vec<Value> = sqlx::query_scalar(&format!(
        "SELECT {ATTACHMENT_JSON} FROM attachments a WHERE a.issue_id = $1 ORDER BY a.created_at, a.id LIMIT $2 OFFSET $3"
    ))
    .bind(issue)
    .bind(page.limit() + 1)
    .bind(page.offset()?)
    .fetch_all(ctx.db())
    .await?;
    page.wrap(nodes)
}

async fn delete(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let (id, issue) = find(&ctx, &args.id).await?;
    let mut tx = ctx.db().begin().await?;
    let value = attachment_value(&mut tx, id).await?;
    sqlx::query("DELETE FROM attachments WHERE id = $1").bind(id).execute(&mut *tx).await?;
    let team: Uuid =
        sqlx::query_scalar("SELECT team_id FROM issues WHERE id = $1").bind(issue).fetch_one(&mut *tx).await?;
    events::emit(&mut tx, &ctx, "Attachment", "remove", &value, None, Some(team)).await?;
    tx.commit().await?;
    Ok(json!({ "deleted": id }))
}

/* ------------------------------------------------------------------ */
/*                               Uploads                              */
/* ------------------------------------------------------------------ */

fn check_name(name: &str) -> ApiResult<String> {
    let name: String = non_empty(name, "name")?.chars().filter(|c| !c.is_control()).collect();
    let name = name.rsplit(['/', '\\']).next().unwrap_or_default().trim().to_string();
    if name.is_empty() || name.chars().count() > 255 {
        return Err(ApiError::bad_request("name is a file name of 1 to 255 characters"));
    }
    Ok(name)
}

fn check_content_type(value: &str) -> ApiResult<String> {
    let value = value.trim().to_lowercase();
    let part = |p: &str| !p.is_empty() && p.bytes().all(|b| b.is_ascii_alphanumeric() || b"!#$&^_.+-".contains(&b));
    match value.split_once('/') {
        Some((kind, sub)) if value.len() <= 127 && part(kind) && part(sub) => Ok(value),
        _ => Err(ApiError::bad_request(format!(
            "\"{value}\" isn't a content type; use one like image/png, text/plain or application/pdf"
        ))),
    }
}

fn markdown(name: &str, content_type: &str, url: &str) -> String {
    let label = name.replace(['[', ']'], "");
    if content_type.starts_with("image/") { format!("![{label}]({url})") } else { format!("[{label}]({url})") }
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct PrepareUploadArgs {
    /// The file name, e.g. screenshot.png.
    name: String,
    /// Its MIME type, e.g. image/png.
    content_type: String,
    /// Its size in bytes.
    size: i64,
}

async fn prepare_upload(ctx: Ctx, args: PrepareUploadArgs) -> ApiResult<Value> {
    let name = check_name(&args.name)?;
    let content_type = check_content_type(&args.content_type)?;
    let max = ctx.state.config.max_upload_size;
    if args.size <= 0 || args.size as u64 > max as u64 {
        return Err(ApiError::bad_request(format!("size is the file's size in bytes, at most {max}")));
    }
    let token = random_secret("rr_up_");
    let (id, expires_at): (Uuid, chrono::DateTime<chrono::Utc>) = sqlx::query_as(
        "INSERT INTO files (workspace_id, uploader_id, name, content_type, size, upload_hash, upload_expires_at)
         VALUES ($1, $2, $3, $4, $5, $6, now() + interval '15 minutes') RETURNING id, upload_expires_at",
    )
    .bind(ctx.workspace)
    .bind(ctx.user)
    .bind(&name)
    .bind(&content_type)
    .bind(args.size)
    .bind(secret_hash(&token))
    .fetch_one(ctx.db())
    .await?;
    let upload_url = format!("{}/uploads/{token}", ctx.public_url());
    let asset = asset_url(ctx.public_url(), id);
    Ok(json!({
        "fileId": id,
        "uploadUrl": upload_url,
        "assetUrl": asset,
        "markdown": markdown(&name, &content_type, &asset),
        "expiresAt": expires_at,
        "curl": format!(
            "curl -sS --fail-with-body -X PUT -H 'content-type: {content_type}' --data-binary @<file> '{upload_url}'"
        ),
    }))
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct UploadInlineArgs {
    /// The file name, e.g. screenshot.png.
    name: String,
    /// Its MIME type, e.g. image/png.
    content_type: String,
    /// The file's bytes in base64 (a data: URL works too).
    base64: String,
}

fn decode_base64(value: &str) -> ApiResult<Vec<u8>> {
    let data = value.trim();
    let data = data.split_once(";base64,").filter(|(head, _)| head.starts_with("data:")).map_or(data, |(_, d)| d);
    let cleaned: String = data.chars().filter(|c| !c.is_whitespace()).collect();
    let cleaned = cleaned.replace('-', "+").replace('_', "/");
    STANDARD
        .decode(&cleaned)
        .or_else(|_| STANDARD_NO_PAD.decode(cleaned.trim_end_matches('=')))
        .map_err(|_| ApiError::bad_request("base64 isn't valid base64"))
}

async fn upload_inline(ctx: Ctx, args: UploadInlineArgs) -> ApiResult<Value> {
    let name = check_name(&args.name)?;
    let content_type = check_content_type(&args.content_type)?;
    let bytes = decode_base64(&args.base64)?;
    let max = INLINE_LIMIT.min(ctx.state.config.max_upload_size);
    if bytes.is_empty() || bytes.len() > max {
        return Err(ApiError::bad_request(format!(
            "The file must have 1 to {max} bytes; upload bigger ones with prepare_file_upload"
        )));
    }
    let id: Uuid = sqlx::query_scalar(
        "INSERT INTO files (workspace_id, uploader_id, name, content_type, size, data)
         VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
    )
    .bind(ctx.workspace)
    .bind(ctx.user)
    .bind(&name)
    .bind(&content_type)
    .bind(bytes.len() as i64)
    .bind(&bytes)
    .fetch_one(ctx.db())
    .await?;
    let asset = asset_url(ctx.public_url(), id);
    Ok(json!({
        "fileId": id,
        "assetUrl": asset,
        "markdown": markdown(&name, &content_type, &asset),
        "name": name,
        "contentType": content_type,
        "size": bytes.len(),
    }))
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct FromUploadArgs {
    /// Issue identifier (ENG-123), id or link.
    issue: String,
    /// The fileId from prepare_file_upload or upload_file.
    file_id: String,
    /// Defaults to the file name.
    title: Option<String>,
}

fn human_size(bytes: i64) -> String {
    match bytes {
        b if b < 1024 => format!("{b} B"),
        b if b < 1024 * 1024 => format!("{:.1} KB", b as f64 / 1024.0),
        b => format!("{:.1} MB", b as f64 / (1024.0 * 1024.0)),
    }
}

async fn create_from_upload(ctx: Ctx, args: FromUploadArgs) -> ApiResult<Value> {
    let issue = resolve::issue(&ctx, &args.issue).await?;
    let not_found = || ApiError::not_found(format!("No uploaded file \"{}\"", args.file_id.trim()));
    let file = as_uuid(&args.file_id).ok_or_else(not_found)?;
    let (name, content_type, size, uploaded): (String, String, i64, bool) = sqlx::query_as(
        "SELECT name, content_type, size, data IS NOT NULL FROM files WHERE id = $1 AND workspace_id = $2",
    )
    .bind(file)
    .bind(ctx.workspace)
    .fetch_optional(ctx.db())
    .await?
    .ok_or_else(not_found)?;
    if !uploaded {
        return Err(ApiError::bad_request("The file hasn't been uploaded yet; PUT it to its uploadUrl first"));
    }
    let title = args.title.filter(|t| !t.trim().is_empty()).unwrap_or_else(|| name.clone());
    let subtitle = format!("{content_type}, {}", human_size(size));
    let metadata = json!({ "fileId": file, "name": name, "contentType": content_type, "size": size });
    let mut tx = ctx.db().begin().await?;
    let url = asset_url(ctx.public_url(), file);
    let id = save(&ctx, &mut tx, issue, &url, &title, Some(&subtitle), Some(metadata), None).await?;
    let value = attachment_value(&mut tx, id).await?;
    tx.commit().await?;
    Ok(value)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn sources_from_hosts() {
        let base = "https://ricrac.example.com";
        assert_eq!(source_type("https://github.com/acme/app/pull/1", base), "github");
        assert_eq!(source_type("https://www.figma.com/file/x", base), "figma");
        assert_eq!(source_type("https://acme.slack.com/archives/C1/p1", base), "slack");
        assert_eq!(source_type("https://gitlab.acme.dev/x/-/merge_requests/2", base), "gitlab");
        assert_eq!(source_type("https://ricrac.example.com/files/123", base), "upload");
        assert_eq!(source_type("https://user@notgithub.com:8080/x", base), "link");
    }

    #[test]
    fn base64_variants() {
        assert_eq!(decode_base64("aGk=").unwrap(), b"hi");
        assert_eq!(decode_base64("aGk").unwrap(), b"hi");
        assert_eq!(decode_base64("data:text/plain;base64,aGk=").unwrap(), b"hi");
        assert!(decode_base64("not base64!").is_err());
    }
}
