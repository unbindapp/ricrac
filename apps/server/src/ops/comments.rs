//! Comments: threads on issues, projects, initiatives, documents, milestones and status
//! updates, with replies, resolving, and reactions (also on issues and status updates).

use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{Value, json};
use sqlx::PgConnection;
use uuid::Uuid;

use super::events::{self, Notice};
use super::resolve::{self, as_uuid};
use super::util::{Page, non_empty};
use super::{IdArgs, Op, op};
use crate::ctx::{Access, Ctx, Role};
use crate::error::{ApiError, ApiResult};

pub fn ops() -> Vec<Op> {
    vec![
        op(
            "list_comments",
            Access::Read,
            &[("GET", "/comments")],
            "List the comment threads on one issue, project, initiative, document, milestone or status update, oldest \
             first. Each thread is its first comment with its replies, reactions and whether it was resolved.",
            list,
        ),
        op(
            "save_comment",
            Access::Write,
            &[("POST", "/comments"), ("PATCH", "/comments/{id}")],
            "Comment on an issue, project, initiative, document, milestone or status update (pass exactly one of them), \
             or reply to a comment with parentId. Pass id and body to edit your own comment. The body is Markdown; \
             mention people with @displayName or @email to notify them. Commenting on an issue subscribes you to it and \
             notifies its subscribers.",
            save,
        ),
        op(
            "delete_comment",
            Access::Write,
            &[("DELETE", "/comments/{id}")],
            "Delete a comment (your own, or any as a workspace admin). Deleting a thread's first comment deletes its replies.",
            delete,
        ),
        op(
            "resolve_comment",
            Access::Write,
            &[("POST", "/comments/{id}/resolve")],
            "Mark a comment thread as resolved. Pass the thread's first comment.",
            resolve_thread,
        ),
        op(
            "unresolve_comment",
            Access::Write,
            &[("POST", "/comments/{id}/unresolve")],
            "Reopen a resolved comment thread.",
            unresolve_thread,
        ),
        op(
            "add_reaction",
            Access::Write,
            &[("POST", "/reactions")],
            "React with an emoji to an issue, a comment or a status update (exactly one). The emoji is a unicode emoji \
             (👍) or a shortcode (+1, :tada:). Reacting twice with the same emoji changes nothing.",
            add_reaction,
        ),
        op(
            "remove_reaction",
            Access::Write,
            &[("DELETE", "/reactions/{id}"), ("POST", "/reactions/remove")],
            "Take back one of your reactions: by its id, or by emoji and the issue, comment or status update.",
            remove_reaction,
        ),
    ]
}

/* ------------------------------------------------------------------ */
/*                           Representation                           */
/* ------------------------------------------------------------------ */

const TARGETS: [&str; 6] =
    ["issue_id", "project_id", "initiative_id", "document_id", "milestone_id", "status_update_id"];

fn target_column(column: &str) -> ApiResult<&'static str> {
    TARGETS
        .iter()
        .find(|c| **c == column)
        .copied()
        .ok_or_else(|| ApiError::bad_request(format!("Comments can't be on \"{column}\"")))
}

fn comment_json(c: &str) -> String {
    format!(
        "jsonb_build_object(
            'id', {c}.id,
            'body', {c}.body,
            'user', user_ref({c}.user_id),
            'quotedText', {c}.quoted_text,
            'reactions', (SELECT coalesce(jsonb_agg(jsonb_build_object('id', r.id, 'emoji', r.emoji, 'user', user_ref(r.user_id))
                ORDER BY r.created_at), '[]') FROM reactions r WHERE r.comment_id = {c}.id),
            'createdAt', {c}.created_at,
            'updatedAt', {c}.updated_at,
            'editedAt', {c}.edited_at,
            'resolvedAt', {c}.resolved_at,
            'resolvingUser', user_ref({c}.resolving_user_id)
        )"
    )
}

fn thread_json() -> String {
    format!(
        "({} || jsonb_build_object('replies', (SELECT coalesce(jsonb_agg({} ORDER BY rc.created_at, rc.id), '[]')
            FROM comments rc WHERE rc.parent_id = c.id)))",
        comment_json("c"),
        comment_json("rc")
    )
}

/// The comment threads on the thing whose id is in `column` (e.g. `issue_id`), oldest first,
/// each with its replies.
pub async fn threads(_ctx: &Ctx, db: &mut PgConnection, column: &str, id: Uuid) -> ApiResult<Value> {
    Ok(Value::Array(thread_list(db, column, id, None, 0).await?))
}

async fn thread_list(
    db: &mut PgConnection,
    column: &str,
    id: Uuid,
    limit: Option<i64>,
    offset: i64,
) -> ApiResult<Vec<Value>> {
    let column = target_column(column)?;
    let sql = format!(
        "SELECT {} FROM comments c WHERE c.{column} = $1 AND c.parent_id IS NULL
         ORDER BY c.created_at, c.id LIMIT $2 OFFSET $3",
        thread_json()
    );
    Ok(sqlx::query_scalar(&sql).bind(id).bind(limit).bind(offset).fetch_all(&mut *db).await?)
}

async fn comment_value(ctx: &Ctx, db: &mut PgConnection, id: Uuid) -> ApiResult<Value> {
    let sql = format!(
        "SELECT {} || jsonb_build_object(
            'parentId', c.parent_id,
            'issue', issue_ref(c.issue_id),
            'project', project_ref(c.project_id),
            'initiative', initiative_ref(c.initiative_id),
            'document', document_ref(c.document_id),
            'projectMilestone', milestone_ref(c.milestone_id),
            'statusUpdate', CASE WHEN c.status_update_id IS NOT NULL THEN jsonb_build_object('id', c.status_update_id) END,
            'url', CASE WHEN c.issue_id IS NOT NULL
                THEN $2 || '/' || w.url_key || '/issue/' || issue_identifier(c.issue_id) || '#comment-' || c.id END)
         FROM comments c JOIN workspaces w ON w.id = c.workspace_id WHERE c.id = $1",
        comment_json("c")
    );
    let value: Option<Value> =
        sqlx::query_scalar(&sql).bind(id).bind(ctx.public_url()).fetch_optional(&mut *db).await?;
    value.ok_or_else(|| ApiError::not_found("No such comment"))
}

/* ------------------------------------------------------------------ */
/*                        What comments are on                        */
/* ------------------------------------------------------------------ */

#[derive(Clone, Copy, PartialEq, Debug)]
struct Target {
    column: &'static str,
    id: Uuid,
}

/// The one thing named among the arguments, checked visible, or None when none is named.
async fn named_target(ctx: &Ctx, given: [(&'static str, &Option<String>); 6]) -> ApiResult<Option<Target>> {
    let named: Vec<(&'static str, &String)> = given.iter().filter_map(|(c, v)| v.as_ref().map(|v| (*c, v))).collect();
    let [(column, value)] = named.as_slice() else {
        if named.is_empty() {
            return Ok(None);
        }
        return Err(ApiError::bad_request(
            "Pass only one of issueId, projectId, initiativeId, documentId, milestoneId and statusUpdateId",
        ));
    };
    let id = match *column {
        "issue_id" => resolve::issue(ctx, value).await?,
        "project_id" => resolve::project(ctx, value).await?,
        "initiative_id" => resolve::initiative(ctx, value).await?,
        "document_id" => resolve::document(ctx, value).await?,
        "milestone_id" => resolve::milestone(ctx, None, value).await?,
        _ => {
            let id = resolve::by_id(ctx, "status_updates", "status update", value).await?;
            check_visible(ctx, Target { column: "status_update_id", id }).await?;
            id
        }
    };
    Ok(Some(Target { column, id }))
}

async fn check_visible(ctx: &Ctx, target: Target) -> ApiResult<()> {
    let id = target.id.to_string();
    let visible = match target.column {
        "issue_id" => resolve::can_see_issue(ctx, target.id).await?,
        "project_id" => resolve::project(ctx, &id).await.is_ok(),
        "initiative_id" => resolve::initiative(ctx, &id).await.is_ok(),
        "document_id" => resolve::can_see_document(ctx, target.id).await?,
        "milestone_id" => resolve::milestone(ctx, None, &id).await.is_ok(),
        _ => {
            let (project, initiative): (Option<Uuid>, Option<Uuid>) =
                sqlx::query_as("SELECT project_id, initiative_id FROM status_updates WHERE id = $1")
                    .bind(target.id)
                    .fetch_optional(ctx.db())
                    .await?
                    .unwrap_or_default();
            match (project, initiative) {
                (Some(p), _) => resolve::project(ctx, &p.to_string()).await.is_ok(),
                (None, Some(i)) => resolve::initiative(ctx, &i.to_string()).await.is_ok(),
                (None, None) => false,
            }
        }
    };
    if !visible {
        return Err(ApiError::not_found("Not found, or you can't see it"));
    }
    Ok(())
}

#[derive(sqlx::FromRow)]
struct CommentRow {
    id: Uuid,
    issue_id: Option<Uuid>,
    project_id: Option<Uuid>,
    initiative_id: Option<Uuid>,
    document_id: Option<Uuid>,
    milestone_id: Option<Uuid>,
    status_update_id: Option<Uuid>,
    parent_id: Option<Uuid>,
    user_id: Option<Uuid>,
    body: String,
}

impl CommentRow {
    fn target(&self) -> ApiResult<Target> {
        let ids = [
            self.issue_id,
            self.project_id,
            self.initiative_id,
            self.document_id,
            self.milestone_id,
            self.status_update_id,
        ];
        TARGETS
            .iter()
            .zip(ids)
            .find_map(|(column, id)| Some(Target { column, id: id? }))
            .ok_or_else(|| ApiError::not_found("That comment is on nothing"))
    }
}

/// A comment of the workspace by id, on something the caller can see.
async fn load_comment(ctx: &Ctx, value: &str) -> ApiResult<CommentRow> {
    let not_found = || ApiError::not_found(format!("No comment \"{}\"", value.trim()));
    let id = as_uuid(value).ok_or_else(not_found)?;
    let row: CommentRow = sqlx::query_as(
        "SELECT id, issue_id, project_id, initiative_id, document_id, milestone_id, status_update_id, parent_id, user_id, body
         FROM comments WHERE id = $1 AND workspace_id = $2",
    )
    .bind(id)
    .bind(ctx.workspace)
    .fetch_optional(ctx.db())
    .await?
    .ok_or_else(not_found)?;
    check_visible(ctx, row.target()?).await.map_err(|_| not_found())?;
    Ok(row)
}

/* ------------------------------------------------------------------ */
/*                                List                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ListCommentsArgs {
    /// Issue identifier (ENG-123), id or link.
    issue_id: Option<String>,
    /// Project name, slug or id.
    project_id: Option<String>,
    /// Initiative name, slug or id.
    initiative_id: Option<String>,
    /// Document title, slug or id.
    document_id: Option<String>,
    /// Project milestone id.
    milestone_id: Option<String>,
    /// Status update id.
    status_update_id: Option<String>,
    /// How many threads to return, 50 by default, at most 250.
    limit: Option<i64>,
    /// The endCursor of the previous page.
    cursor: Option<String>,
}

async fn list(ctx: Ctx, args: ListCommentsArgs) -> ApiResult<Value> {
    let target = named_target(
        &ctx,
        [
            ("issue_id", &args.issue_id),
            ("project_id", &args.project_id),
            ("initiative_id", &args.initiative_id),
            ("document_id", &args.document_id),
            ("milestone_id", &args.milestone_id),
            ("status_update_id", &args.status_update_id),
        ],
    )
    .await?
    .ok_or_else(|| {
        ApiError::bad_request("Pass one of issueId, projectId, initiativeId, documentId, milestoneId or statusUpdateId")
    })?;
    let page = Page { limit: args.limit, cursor: args.cursor };
    let mut db = ctx.db().acquire().await?;
    let nodes = thread_list(&mut db, target.column, target.id, Some(page.limit() + 1), page.offset()?).await?;
    page.wrap(nodes)
}

/* ------------------------------------------------------------------ */
/*                                Save                                */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SaveCommentArgs {
    /// Only to edit one of your comments: its id. Omit to create a comment.
    id: Option<String>,
    /// Markdown. Mention people with @displayName or @email; they are notified.
    body: Option<String>,
    /// Issue identifier (ENG-123), id or link to comment on.
    issue_id: Option<String>,
    /// Project name, slug or id to comment on.
    project_id: Option<String>,
    /// Initiative name, slug or id to comment on.
    initiative_id: Option<String>,
    /// Document title, slug or id to comment on.
    document_id: Option<String>,
    /// Project milestone id to comment on.
    milestone_id: Option<String>,
    /// Status update id to comment on.
    status_update_id: Option<String>,
    /// The comment to reply to; the reply joins its thread.
    parent_id: Option<String>,
    /// The passage of the document or description the comment is about.
    quoted_text: Option<String>,
}

async fn save(ctx: Ctx, args: SaveCommentArgs) -> ApiResult<Value> {
    let body = non_empty(args.body.as_deref().unwrap_or(""), "body")?;
    let target = named_target(
        &ctx,
        [
            ("issue_id", &args.issue_id),
            ("project_id", &args.project_id),
            ("initiative_id", &args.initiative_id),
            ("document_id", &args.document_id),
            ("milestone_id", &args.milestone_id),
            ("status_update_id", &args.status_update_id),
        ],
    )
    .await?;
    if let Some(id) = &args.id {
        if target.is_some() || args.parent_id.is_some() || args.quoted_text.is_some() {
            return Err(ApiError::bad_request("Only the body of a comment can be changed"));
        }
        return edit(&ctx, id, body).await;
    }
    let parent = match &args.parent_id {
        Some(parent) => Some(load_comment(&ctx, parent).await?),
        None => None,
    };
    let (target, root) = match (target, &parent) {
        (_, Some(parent)) => {
            let parent_target = parent.target()?;
            if target.is_some_and(|t| t != parent_target) {
                return Err(ApiError::bad_request("The parent comment is on something else"));
            }
            (parent_target, Some(parent.parent_id.unwrap_or(parent.id)))
        }
        (Some(target), None) => (target, None),
        (None, None) => {
            return Err(ApiError::bad_request(
                "Pass what to comment on (issueId, projectId, initiativeId, documentId, milestoneId or statusUpdateId) or parentId",
            ));
        }
    };
    let mut tx = ctx.db().begin().await?;
    let id: Uuid = sqlx::query_scalar(&format!(
        "INSERT INTO comments (workspace_id, {}, parent_id, user_id, body, quoted_text)
         VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
        target.column
    ))
    .bind(ctx.workspace)
    .bind(target.id)
    .bind(root)
    .bind(ctx.user)
    .bind(&body)
    .bind(args.quoted_text.as_deref().map(str::trim).filter(|q| !q.is_empty()))
    .fetch_one(&mut *tx)
    .await?;
    let about = about(&mut tx, target).await?;
    let mentioned = events::mentions(&ctx, &body).await?;
    let participants = match root {
        Some(root) => thread_people(&mut tx, root).await?,
        None => vec![],
    };
    let audience = match target.column {
        "issue_id" => {
            events::subscribe(&mut tx, target.id, &[ctx.user]).await?;
            events::subscribe(&mut tx, target.id, &mentioned).await?;
            sqlx::query("UPDATE issues SET updated_at = now() WHERE id = $1").bind(target.id).execute(&mut *tx).await?;
            events::issue_subscribers(&mut tx, target.id).await?
        }
        "status_update_id" => {
            sqlx::query_scalar("SELECT user_id FROM status_updates WHERE id = $1 AND user_id IS NOT NULL")
                .bind(target.id)
                .fetch_all(&mut *tx)
                .await?
        }
        _ => vec![],
    };
    let name = actor_name(&ctx).await?;
    let replied: Vec<Uuid> = participants.into_iter().filter(|u| !mentioned.contains(u)).collect();
    let others: Vec<Uuid> = audience.into_iter().filter(|u| !mentioned.contains(u) && !replied.contains(u)).collect();
    let notices = [
        (&mentioned, about.kinds.2, format!("{name} mentioned you in a comment on {}", about.label)),
        (&replied, about.kinds.1, format!("{name} replied to a thread on {}", about.label)),
        (&others, about.kinds.0, format!("{name} commented on {}", about.label)),
    ];
    for (users, kind, summary) in notices {
        let notice = Notice { kind, summary, comment: Some(id), ..about.notice.clone() };
        events::notify(&mut tx, &ctx, users, &notice).await?;
    }
    let value = comment_value(&ctx, &mut tx, id).await?;
    events::emit(&mut tx, &ctx, "Comment", "create", &value, None, about.team).await?;
    tx.commit().await?;
    Ok(value)
}

async fn edit(ctx: &Ctx, reference: &str, body: String) -> ApiResult<Value> {
    let row = load_comment(ctx, reference).await?;
    if row.user_id != Some(ctx.user) {
        return Err(ApiError::forbidden("Only its author can edit a comment"));
    }
    let mut tx = ctx.db().begin().await?;
    let before = comment_value(ctx, &mut tx, row.id).await?;
    sqlx::query("UPDATE comments SET body = $2, edited_at = now(), updated_at = now() WHERE id = $1")
        .bind(row.id)
        .bind(&body)
        .execute(&mut *tx)
        .await?;
    let previous = events::mentions(ctx, &row.body).await?;
    let mentioned: Vec<Uuid> =
        events::mentions(ctx, &body).await?.into_iter().filter(|u| !previous.contains(u)).collect();
    let target = row.target()?;
    let about = about(&mut tx, target).await?;
    if target.column == "issue_id" {
        events::subscribe(&mut tx, target.id, &mentioned).await?;
    }
    let notice = Notice {
        kind: about.kinds.2,
        summary: format!("{} mentioned you in a comment on {}", actor_name(ctx).await?, about.label),
        comment: Some(row.id),
        ..about.notice.clone()
    };
    events::notify(&mut tx, ctx, &mentioned, &notice).await?;
    let value = comment_value(ctx, &mut tx, row.id).await?;
    let updated_from = json!({ "body": before["body"], "editedAt": before["editedAt"] });
    events::emit(&mut tx, ctx, "Comment", "update", &value, Some(updated_from), about.team).await?;
    tx.commit().await?;
    Ok(value)
}

/// How notifications about comments on a target read and what they point at.
struct About {
    /// The kinds for a new comment, a reply in your thread, and a mention.
    kinds: (&'static str, &'static str, &'static str),
    label: String,
    notice: Notice,
    team: Option<Uuid>,
}

async fn about(tx: &mut PgConnection, target: Target) -> ApiResult<About> {
    let id = target.id;
    let named = |sql: &'static str| sqlx::query_scalar::<_, String>(sql).bind(id);
    Ok(match target.column {
        "issue_id" => About {
            kinds: ("issueNewComment", "issueCommentReply", "issueCommentMention"),
            label: named("SELECT issue_identifier($1)").fetch_one(&mut *tx).await?,
            notice: Notice { issue: Some(id), ..Default::default() },
            team: sqlx::query_scalar("SELECT team_id FROM issues WHERE id = $1").bind(id).fetch_one(&mut *tx).await?,
        },
        "project_id" => About {
            kinds: ("projectNewComment", "projectCommentReply", "projectCommentMention"),
            label: named("SELECT name FROM projects WHERE id = $1").fetch_one(&mut *tx).await?,
            notice: Notice { project: Some(id), ..Default::default() },
            team: None,
        },
        "milestone_id" => {
            let (project, label): (Uuid, String) = sqlx::query_as(
                "SELECT p.id, m.name || ' (' || p.name || ')' FROM project_milestones m
                 JOIN projects p ON p.id = m.project_id WHERE m.id = $1",
            )
            .bind(id)
            .fetch_one(&mut *tx)
            .await?;
            About {
                kinds: ("projectNewComment", "projectCommentReply", "projectCommentMention"),
                label,
                notice: Notice { project: Some(project), ..Default::default() },
                team: None,
            }
        }
        "initiative_id" => About {
            kinds: ("initiativeNewComment", "initiativeCommentReply", "initiativeCommentMention"),
            label: named("SELECT name FROM initiatives WHERE id = $1").fetch_one(&mut *tx).await?,
            notice: Notice { initiative: Some(id), ..Default::default() },
            team: None,
        },
        "document_id" => About {
            kinds: ("documentNewComment", "documentCommentReply", "documentCommentMention"),
            label: named("SELECT title FROM documents WHERE id = $1").fetch_one(&mut *tx).await?,
            notice: Notice { document: Some(id), ..Default::default() },
            team: None,
        },
        _ => {
            let (project, initiative, name): (Option<Uuid>, Option<Uuid>, String) = sqlx::query_as(
                "SELECT u.project_id, u.initiative_id, coalesce(p.name, i.name) FROM status_updates u
                 LEFT JOIN projects p ON p.id = u.project_id LEFT JOIN initiatives i ON i.id = u.initiative_id
                 WHERE u.id = $1",
            )
            .bind(id)
            .fetch_one(&mut *tx)
            .await?;
            let kinds = match project {
                Some(_) => ("projectUpdateNewComment", "projectUpdateCommentReply", "projectUpdateCommentMention"),
                None => {
                    ("initiativeUpdateNewComment", "initiativeUpdateCommentReply", "initiativeUpdateCommentMention")
                }
            };
            About {
                kinds,
                label: format!("the update on {name}"),
                notice: Notice { status_update: Some(id), project, initiative, ..Default::default() },
                team: None,
            }
        }
    })
}

/// Everyone who wrote in the thread.
async fn thread_people(tx: &mut PgConnection, root: Uuid) -> ApiResult<Vec<Uuid>> {
    Ok(sqlx::query_scalar(
        "SELECT DISTINCT user_id FROM comments WHERE (id = $1 OR parent_id = $1) AND user_id IS NOT NULL",
    )
    .bind(root)
    .fetch_all(&mut *tx)
    .await?)
}

async fn actor_name(ctx: &Ctx) -> ApiResult<String> {
    Ok(sqlx::query_scalar("SELECT name FROM users WHERE id = $1").bind(ctx.user).fetch_one(ctx.db()).await?)
}

/* ------------------------------------------------------------------ */
/*                      Delete, resolve, unresolve                     */
/* ------------------------------------------------------------------ */

async fn team_of(tx: &mut PgConnection, target: Target) -> ApiResult<Option<Uuid>> {
    if target.column != "issue_id" {
        return Ok(None);
    }
    Ok(sqlx::query_scalar("SELECT team_id FROM issues WHERE id = $1").bind(target.id).fetch_optional(&mut *tx).await?)
}

async fn delete(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    let row = load_comment(&ctx, &args.id).await?;
    if row.user_id != Some(ctx.user) && ctx.role < Role::Admin {
        return Err(ApiError::forbidden("Only its author or a workspace admin can delete a comment"));
    }
    let mut tx = ctx.db().begin().await?;
    let value = comment_value(&ctx, &mut tx, row.id).await?;
    let replies: i64 = sqlx::query_scalar("SELECT count(*) FROM comments WHERE parent_id = $1")
        .bind(row.id)
        .fetch_one(&mut *tx)
        .await?;
    sqlx::query("DELETE FROM comments WHERE id = $1").bind(row.id).execute(&mut *tx).await?;
    let team = team_of(&mut tx, row.target()?).await?;
    events::emit(&mut tx, &ctx, "Comment", "remove", &value, None, team).await?;
    tx.commit().await?;
    Ok(json!({ "deleted": row.id, "deletedReplies": replies }))
}

async fn set_resolved(ctx: Ctx, reference: &str, resolved: bool) -> ApiResult<Value> {
    let row = load_comment(&ctx, reference).await?;
    if row.parent_id.is_some() {
        return Err(ApiError::bad_request("Only a thread is resolved; pass the thread's first comment (the parentId)"));
    }
    let mut tx = ctx.db().begin().await?;
    sqlx::query(
        "UPDATE comments SET resolved_at = CASE WHEN $2 THEN coalesce(resolved_at, now()) END,
            resolving_user_id = CASE WHEN $2 THEN coalesce(resolving_user_id, $3) END, updated_at = now()
         WHERE id = $1",
    )
    .bind(row.id)
    .bind(resolved)
    .bind(ctx.user)
    .execute(&mut *tx)
    .await?;
    let value = comment_value(&ctx, &mut tx, row.id).await?;
    let team = team_of(&mut tx, row.target()?).await?;
    events::emit(&mut tx, &ctx, "Comment", "update", &value, None, team).await?;
    tx.commit().await?;
    Ok(value)
}

async fn resolve_thread(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    set_resolved(ctx, &args.id, true).await
}

async fn unresolve_thread(ctx: Ctx, args: IdArgs) -> ApiResult<Value> {
    set_resolved(ctx, &args.id, false).await
}

/* ------------------------------------------------------------------ */
/*                              Reactions                             */
/* ------------------------------------------------------------------ */

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ReactionArgs {
    /// A unicode emoji (👍) or a shortcode (+1, :tada:).
    emoji: String,
    /// Issue identifier (ENG-123), id or link.
    issue_id: Option<String>,
    /// Comment id.
    comment_id: Option<String>,
    /// Status update id.
    status_update_id: Option<String>,
}

#[derive(Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct RemoveReactionArgs {
    /// The reaction's id. Or pass emoji and what it is on.
    id: Option<String>,
    emoji: Option<String>,
    issue_id: Option<String>,
    comment_id: Option<String>,
    status_update_id: Option<String>,
}

/// Shortcodes are kept without colons, lowercase; unicode emoji as they are.
fn normalize_emoji(value: &str) -> ApiResult<String> {
    let trimmed = value.trim();
    let name = trimmed.strip_prefix(':').and_then(|v| v.strip_suffix(':')).unwrap_or(trimmed);
    let shortcode =
        name.len() <= 50 && name.bytes().all(|b| b.is_ascii_alphanumeric() || matches!(b, b'_' | b'+' | b'-'));
    let unicode = !name.is_ascii() && name.chars().count() <= 16 && !name.chars().any(char::is_whitespace);
    if name.is_empty() || !(shortcode || unicode) {
        return Err(ApiError::bad_request(format!(
            "\"{value}\" isn't an emoji; pass one like 👍 or a shortcode like +1 or :tada:"
        )));
    }
    Ok(if shortcode { name.to_lowercase() } else { name.to_string() })
}

/// The column and id a reaction is on, checked visible, and the issue's team for webhooks.
async fn reaction_target(
    ctx: &Ctx,
    issue: &Option<String>,
    comment: &Option<String>,
    status_update: &Option<String>,
) -> ApiResult<(&'static str, Uuid, Option<Uuid>)> {
    let target = match (issue, comment, status_update) {
        (Some(issue), None, None) => {
            let id = resolve::issue(ctx, issue).await?;
            ("issue_id", id, Some(Target { column: "issue_id", id }))
        }
        (None, Some(comment), None) => {
            let row = load_comment(ctx, comment).await?;
            ("comment_id", row.id, Some(row.target()?))
        }
        (None, None, Some(update)) => {
            let id = resolve::by_id(ctx, "status_updates", "status update", update).await?;
            check_visible(ctx, Target { column: "status_update_id", id }).await?;
            ("status_update_id", id, None)
        }
        _ => return Err(ApiError::bad_request("Pass exactly one of issueId, commentId and statusUpdateId")),
    };
    let team = match target.2 {
        Some(on) => team_of(&mut *ctx.db().acquire().await?, on).await?,
        None => None,
    };
    Ok((target.0, target.1, team))
}

const REACTION_JSON: &str = "jsonb_build_object('id', r.id, 'emoji', r.emoji, 'user', user_ref(r.user_id),
    'issue', issue_ref(r.issue_id),
    'comment', CASE WHEN r.comment_id IS NOT NULL THEN jsonb_build_object('id', r.comment_id) END,
    'statusUpdate', CASE WHEN r.status_update_id IS NOT NULL THEN jsonb_build_object('id', r.status_update_id) END,
    'createdAt', r.created_at)";

async fn add_reaction(ctx: Ctx, args: ReactionArgs) -> ApiResult<Value> {
    let emoji = normalize_emoji(&args.emoji)?;
    let (column, id, team) = reaction_target(&ctx, &args.issue_id, &args.comment_id, &args.status_update_id).await?;
    let mut tx = ctx.db().begin().await?;
    let inserted: Option<Uuid> = sqlx::query_scalar(&format!(
        "INSERT INTO reactions (workspace_id, user_id, emoji, {column}) VALUES ($1, $2, $3, $4)
         ON CONFLICT DO NOTHING RETURNING id"
    ))
    .bind(ctx.workspace)
    .bind(ctx.user)
    .bind(&emoji)
    .bind(id)
    .fetch_optional(&mut *tx)
    .await?;
    let value: Value = sqlx::query_scalar(&format!(
        "SELECT {REACTION_JSON} FROM reactions r WHERE r.user_id = $1 AND r.emoji = $2 AND r.{column} = $3"
    ))
    .bind(ctx.user)
    .bind(&emoji)
    .bind(id)
    .fetch_one(&mut *tx)
    .await?;
    if inserted.is_some() {
        events::emit(&mut tx, &ctx, "Reaction", "create", &value, None, team).await?;
    }
    tx.commit().await?;
    Ok(value)
}

async fn remove_reaction(ctx: Ctx, args: RemoveReactionArgs) -> ApiResult<Value> {
    let found: Option<(Uuid, Option<Uuid>)> = match (&args.id, &args.emoji) {
        (Some(id), None) => {
            let id = as_uuid(id).ok_or_else(|| ApiError::not_found(format!("No reaction \"{id}\"")))?;
            sqlx::query_as(
                "SELECT r.id, coalesce(i.team_id, ci.team_id) FROM reactions r
                 LEFT JOIN issues i ON i.id = r.issue_id
                 LEFT JOIN comments c ON c.id = r.comment_id LEFT JOIN issues ci ON ci.id = c.issue_id
                 WHERE r.id = $1 AND r.workspace_id = $2 AND r.user_id = $3",
            )
            .bind(id)
            .bind(ctx.workspace)
            .bind(ctx.user)
            .fetch_optional(ctx.db())
            .await?
        }
        (None, Some(emoji)) => {
            let emoji = normalize_emoji(emoji)?;
            let (column, id, team) =
                reaction_target(&ctx, &args.issue_id, &args.comment_id, &args.status_update_id).await?;
            let reaction: Option<Uuid> = sqlx::query_scalar(&format!(
                "SELECT id FROM reactions WHERE user_id = $1 AND emoji = $2 AND {column} = $3"
            ))
            .bind(ctx.user)
            .bind(&emoji)
            .bind(id)
            .fetch_optional(ctx.db())
            .await?;
            reaction.map(|r| (r, team))
        }
        _ => return Err(ApiError::bad_request("Pass the reaction's id, or emoji and what it is on")),
    };
    let Some((id, team)) = found else {
        return Err(ApiError::not_found("You have no such reaction"));
    };
    let mut tx = ctx.db().begin().await?;
    let value: Value = sqlx::query_scalar(&format!("SELECT {REACTION_JSON} FROM reactions r WHERE r.id = $1"))
        .bind(id)
        .fetch_one(&mut *tx)
        .await?;
    sqlx::query("DELETE FROM reactions WHERE id = $1").bind(id).execute(&mut *tx).await?;
    events::emit(&mut tx, &ctx, "Reaction", "remove", &value, None, team).await?;
    tx.commit().await?;
    Ok(json!({ "removed": id }))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn emoji_are_shortcodes_or_unicode() {
        assert_eq!(normalize_emoji(":tada:").unwrap(), "tada");
        assert_eq!(normalize_emoji("+1").unwrap(), "+1");
        assert_eq!(normalize_emoji("👍").unwrap(), "👍");
        assert!(normalize_emoji("not an emoji").is_err());
        assert!(normalize_emoji("::").is_err());
    }
}
