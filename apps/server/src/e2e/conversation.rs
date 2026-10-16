//! Comments, reactions, notifications, attachments, uploads and search.

use axum::body::Body;
use axum::http::{Request, StatusCode};
use serde_json::{Value, json};
use sqlx::PgPool;
use tower::ServiceExt;

use super::*;
use crate::keys::{random_secret, secret_hash};

fn summaries(inbox: &Value) -> Vec<String> {
    inbox["nodes"].as_array().unwrap().iter().map(|n| n["summary"].as_str().unwrap().to_string()).collect()
}

fn kinds(inbox: &Value) -> Vec<String> {
    inbox["nodes"].as_array().unwrap().iter().map(|n| n["type"].as_str().unwrap().to_string()).collect()
}

#[sqlx::test]
async fn comment_threads_with_replies_and_resolving(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "Crash" })).await;
    let root = call(&w.ann, "save_comment", json!({ "issueId": "ENG-1", "body": "First" })).await;
    assert_eq!(root["issue"]["identifier"], "ENG-1");
    assert_eq!(root["url"], format!("http://localhost:3000/acme/issue/ENG-1#comment-{}", root["id"].as_str().unwrap()));
    let reply = call(&bob, "save_comment", json!({ "parentId": root["id"], "body": "A reply" })).await;
    assert_eq!(reply["parentId"], root["id"]);
    let nested = call(&w.ann, "save_comment", json!({ "parentId": reply["id"], "body": "Reply to the reply" })).await;
    assert_eq!(nested["parentId"], root["id"], "replies to replies join the thread");
    assert_eq!(nested["issue"]["identifier"], "ENG-1");
    call(&w.ann, "save_comment", json!({ "issueId": "ENG-1", "body": "Second", "quotedText": "Crash" })).await;

    let threads = call(&w.ann, "list_comments", json!({ "issueId": "ENG-1" })).await;
    let nodes = threads["nodes"].as_array().unwrap();
    assert_eq!(nodes.len(), 2);
    assert_eq!(nodes[0]["body"], "First");
    let replies: Vec<&str> =
        nodes[0]["replies"].as_array().unwrap().iter().map(|r| r["body"].as_str().unwrap()).collect();
    assert_eq!(replies, ["A reply", "Reply to the reply"]);
    assert_eq!(nodes[0]["replies"][0]["user"]["name"], "Bob Jones");
    assert_eq!(nodes[1]["quotedText"], "Crash");
    let first_page = call(&w.ann, "list_comments", json!({ "issueId": "ENG-1", "limit": 1 })).await;
    assert_eq!(first_page["pageInfo"]["hasNextPage"], true);
    let cursor = first_page["pageInfo"]["endCursor"].clone();
    let second_page = call(&w.ann, "list_comments", json!({ "issueId": "ENG-1", "limit": 1, "cursor": cursor })).await;
    assert_eq!(second_page["nodes"][0]["body"], "Second");

    let resolved = call(&bob, "resolve_comment", json!({ "id": root["id"] })).await;
    assert!(resolved["resolvedAt"].is_string());
    assert_eq!(resolved["resolvingUser"]["name"], "Bob Jones");
    let not_root = fails(&bob, "resolve_comment", json!({ "id": reply["id"] })).await;
    assert!(not_root.contains("first comment"), "{not_root}");
    let reopened = call(&w.ann, "unresolve_comment", json!({ "id": root["id"] })).await;
    assert!(reopened["resolvedAt"].is_null() && reopened["resolvingUser"].is_null());

    let not_author = fails(&bob, "save_comment", json!({ "id": root["id"], "body": "Hijacked" })).await;
    assert!(not_author.contains("author"), "{not_author}");
    let edited = call(&w.ann, "save_comment", json!({ "id": root["id"], "body": "First, edited" })).await;
    assert_eq!(edited["body"], "First, edited");
    assert!(edited["editedAt"].is_string());

    let issue = call(&w.ann, "get_issue", json!({ "id": "ENG-1", "includeComments": true })).await;
    assert_eq!(issue["commentCount"], 4);
    assert_eq!(issue["comments"][0]["body"], "First, edited");
    assert_eq!(issue["comments"][0]["replies"].as_array().unwrap().len(), 2);

    let two = fails(&w.ann, "save_comment", json!({ "issueId": "ENG-1", "projectId": "x", "body": "x" })).await;
    assert!(two.contains("only one"), "{two}");
    let none = fails(&w.ann, "save_comment", json!({ "body": "x" })).await;
    assert!(none.contains("parentId"), "{none}");
    let empty = fails(&w.ann, "save_comment", json!({ "issueId": "ENG-1", "body": "  " })).await;
    assert!(empty.contains("body"), "{empty}");

    let forbidden = fails(&bob, "delete_comment", json!({ "id": root["id"] })).await;
    assert!(forbidden.contains("author"), "{forbidden}");
    let deleted = call(&w.ann, "delete_comment", json!({ "id": root["id"] })).await;
    assert_eq!(deleted["deletedReplies"], 2);
    let issue = call(&w.ann, "get_issue", json!({ "id": "ENG-1" })).await;
    assert_eq!(issue["commentCount"], 1, "the thread's replies went with it");
}

#[sqlx::test]
async fn comments_notify_subscribers_participants_and_mentioned(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    let carl = w.member("carl@acme.test", "Carl Clark", "member").await;
    let dave = w.member("dave@acme.test", "Dave Brown", "member").await;
    call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "Crash" })).await;
    let root = call(&bob, "save_comment", json!({ "issueId": "ENG-1", "body": "Looks bad" })).await;
    call(&carl, "save_comment", json!({ "parentId": root["id"], "body": "Agreed, @dave can you check?" })).await;

    let inbox = call(&w.ann, "get_notifications", json!({})).await;
    assert_eq!(summaries(&inbox), ["Carl Clark commented on ENG-1", "Bob Jones commented on ENG-1"]);
    assert_eq!(kinds(&inbox), ["issueNewComment", "issueNewComment"]);
    assert_eq!(inbox["unreadCount"], 2);
    assert_eq!(inbox["nodes"][0]["actor"]["name"], "Carl Clark");
    assert_eq!(inbox["nodes"][0]["issue"]["identifier"], "ENG-1");
    assert_eq!(inbox["nodes"][0]["comment"]["body"], "Agreed, @dave can you check?");
    let bob_inbox = call(&bob, "get_notifications", json!({})).await;
    assert_eq!(kinds(&bob_inbox), ["issueCommentReply"], "bob wrote the thread; his own comment didn't notify him");
    let dave_inbox = call(&dave, "get_notifications", json!({})).await;
    assert_eq!(kinds(&dave_inbox), ["issueCommentMention"]);
    assert_eq!(summaries(&dave_inbox), ["Carl Clark mentioned you in a comment on ENG-1"]);
    let carl_inbox = call(&carl, "get_notifications", json!({})).await;
    assert_eq!(carl_inbox["nodes"].as_array().unwrap().len(), 0);
    let issue = call(&w.ann, "get_issue", json!({ "id": "ENG-1" })).await;
    let subscribers: Vec<&str> =
        issue["subscribers"].as_array().unwrap().iter().map(|s| s["name"].as_str().unwrap()).collect();
    for name in ["Ann Smith", "Bob Jones", "Carl Clark", "Dave Brown"] {
        assert!(subscribers.contains(&name), "{name} is subscribed: {subscribers:?}");
    }

    let newest = inbox["nodes"][0]["id"].clone();
    let oldest = inbox["nodes"][1]["id"].clone();
    let read = call(&w.ann, "mark_notification", json!({ "id": newest, "action": "read" })).await;
    assert!(read["nodes"][0]["readAt"].is_string());
    let inbox = call(&w.ann, "get_notifications", json!({ "unreadOnly": true })).await;
    assert_eq!(inbox["unreadCount"], 1);
    assert_eq!(inbox["nodes"].as_array().unwrap().len(), 1);
    call(&w.ann, "mark_notification", json!({ "ids": [newest], "action": "unread" })).await;
    call(&w.ann, "mark_notification", json!({ "id": oldest, "action": "archive" })).await;
    let inbox = call(&w.ann, "get_notifications", json!({})).await;
    assert_eq!(inbox["nodes"].as_array().unwrap().len(), 1);
    assert_eq!(inbox["unreadCount"], 1);
    let all = call(&w.ann, "get_notifications", json!({ "includeArchived": true })).await;
    assert_eq!(all["nodes"].as_array().unwrap().len(), 2);
    call(&w.ann, "mark_notification", json!({ "id": oldest, "action": "unarchive" })).await;

    call(&w.ann, "mark_notification", json!({ "id": newest, "action": "read" })).await;
    let snoozed = call(&w.ann, "mark_notification", json!({ "id": newest, "action": "snooze", "until": "P1D" })).await;
    assert!(snoozed["nodes"][0]["snoozedUntil"].is_string());
    let inbox = call(&w.ann, "get_notifications", json!({})).await;
    assert_eq!(inbox["nodes"].as_array().unwrap().len(), 1, "snoozed notifications are hidden");
    let with_snoozed = call(&w.ann, "get_notifications", json!({ "includeSnoozed": true })).await;
    assert_eq!(with_snoozed["nodes"].as_array().unwrap().len(), 2);
    sqlx::query("UPDATE notifications SET snoozed_until = now() - interval '1 minute' WHERE id = $1::uuid")
        .bind(newest.as_str().unwrap())
        .execute(&w.state.db)
        .await
        .unwrap();
    crate::jobs::automations(&w.state).await.unwrap();
    let inbox = call(&w.ann, "get_notifications", json!({})).await;
    assert_eq!(inbox["nodes"][0]["id"], newest);
    assert!(inbox["nodes"][0]["readAt"].is_null() && inbox["nodes"][0]["snoozedUntil"].is_null(), "woke up unread");
    assert_eq!(inbox["unreadCount"], 2);

    let missing = fails(&w.ann, "mark_notification", json!({ "id": newest, "action": "snooze" })).await;
    assert!(missing.contains("until"), "{missing}");
    let bad = fails(&w.ann, "mark_notification", json!({ "id": newest, "action": "explode" })).await;
    assert!(bad.contains("unsnooze"), "{bad}");
    let not_yours = fails(&bob, "mark_notification", json!({ "id": newest, "action": "read" })).await;
    assert!(not_yours.contains("No such notification"), "{not_yours}");

    let filtered = call(&dave, "get_notifications", json!({ "type": "issueNewComment" })).await;
    assert_eq!(filtered["nodes"].as_array().unwrap().len(), 0);
    let marked = call(&w.ann, "mark_all_notifications_read", json!({ "issue": "ENG-1" })).await;
    assert_eq!(marked["marked"], 2);
    let inbox = call(&w.ann, "get_notifications", json!({})).await;
    assert_eq!(inbox["unreadCount"], 0);
}

#[sqlx::test]
async fn reactions_are_idempotent(db: PgPool) {
    let w = world(db).await;
    call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "Crash" })).await;
    let first = call(&w.ann, "add_reaction", json!({ "issueId": "ENG-1", "emoji": ":+1:" })).await;
    assert_eq!(first["emoji"], "+1");
    let again = call(&w.ann, "add_reaction", json!({ "issueId": "ENG-1", "emoji": "+1" })).await;
    assert_eq!(again["id"], first["id"]);
    call(&w.ann, "add_reaction", json!({ "issueId": "ENG-1", "emoji": "🎉" })).await;
    let issue = call(&w.ann, "get_issue", json!({ "id": "ENG-1" })).await;
    assert_eq!(issue["reactions"].as_array().unwrap().len(), 2);

    let comment = call(&w.ann, "save_comment", json!({ "issueId": "ENG-1", "body": "Hi" })).await;
    let on_comment = call(&w.ann, "add_reaction", json!({ "commentId": comment["id"], "emoji": "eyes" })).await;
    let threads = call(&w.ann, "list_comments", json!({ "issueId": "ENG-1" })).await;
    assert_eq!(threads["nodes"][0]["reactions"][0]["emoji"], "eyes");
    call(&w.ann, "remove_reaction", json!({ "commentId": comment["id"], "emoji": ":eyes:" })).await;
    let gone = fails(&w.ann, "remove_reaction", json!({ "id": on_comment["id"] })).await;
    assert!(gone.contains("no such reaction"), "{gone}");
    call(&w.ann, "remove_reaction", json!({ "id": first["id"] })).await;
    let issue = call(&w.ann, "get_issue", json!({ "id": "ENG-1" })).await;
    assert_eq!(issue["reactions"][0]["emoji"], "🎉");

    let both =
        fails(&w.ann, "add_reaction", json!({ "issueId": "ENG-1", "commentId": comment["id"], "emoji": "+1" })).await;
    assert!(both.contains("exactly one"), "{both}");
    let not_emoji = fails(&w.ann, "add_reaction", json!({ "issueId": "ENG-1", "emoji": "thumbs up" })).await;
    assert!(not_emoji.contains("emoji"), "{not_emoji}");
}

#[sqlx::test]
async fn attachments_are_unique_per_url(db: PgPool) {
    let w = world(db).await;
    let pr = "https://github.com/acme/app/pull/7";
    call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "Crash", "links": [{ "url": pr, "title": "PR" }] }))
        .await;
    call(&w.ann, "save_issue", json!({ "id": "ENG-1", "links": [{ "url": pr, "title": "Fix crash PR" }] })).await;
    let issue = call(&w.ann, "get_issue", json!({ "id": "ENG-1" })).await;
    assert_eq!(issue["attachments"].as_array().unwrap().len(), 1);
    assert_eq!(issue["attachments"][0]["title"], "Fix crash PR");
    assert_eq!(issue["attachments"][0]["sourceType"], "github");

    let same = call(
        &w.ann,
        "create_attachment",
        json!({ "issue": "ENG-1", "url": pr, "title": "Fix crash", "subtitle": "Open", "metadata": { "state": "open" } }),
    )
    .await;
    assert_eq!(same["id"], issue["attachments"][0]["id"]);
    assert_eq!(same["subtitle"], "Open");
    assert_eq!(same["metadata"]["state"], "open");
    assert_eq!(same["issue"]["identifier"], "ENG-1");
    assert_eq!(same["creator"]["name"], "Ann Smith");
    let design = call(
        &w.ann,
        "create_attachment",
        json!({ "issue": "ENG-1", "url": "https://www.figma.com/file/abc", "title": "Design" }),
    )
    .await;
    assert_eq!(design["sourceType"], "figma");
    let listed = call(&w.ann, "list_attachments", json!({ "issue": "ENG-1" })).await;
    assert_eq!(listed["nodes"].as_array().unwrap().len(), 2);
    let got = call(&w.ann, "get_attachment", json!({ "id": design["id"] })).await;
    assert_eq!(got["title"], "Design");
    call(&w.ann, "delete_attachment", json!({ "id": design["id"] })).await;
    let listed = call(&w.ann, "list_attachments", json!({ "issue": "ENG-1" })).await;
    assert_eq!(listed["nodes"].as_array().unwrap().len(), 1);

    let not_link =
        fails(&w.ann, "create_attachment", json!({ "issue": "ENG-1", "url": "ftp://x", "title": "x" })).await;
    assert!(not_link.contains("http"), "{not_link}");
    let bad_metadata = fails(
        &w.ann,
        "create_attachment",
        json!({ "issue": "ENG-1", "url": "https://x.dev", "title": "x", "metadata": [1] }),
    )
    .await;
    assert!(bad_metadata.contains("metadata"), "{bad_metadata}");
}

async fn send(router: &axum::Router, request: Request<Body>) -> (StatusCode, axum::http::HeaderMap, Vec<u8>) {
    let response = router.clone().oneshot(request).await.unwrap();
    let status = response.status();
    let headers = response.headers().clone();
    let bytes = axum::body::to_bytes(response.into_body(), usize::MAX).await.unwrap();
    (status, headers, bytes.to_vec())
}

async fn api_key(w: &World) -> String {
    let key = random_secret(crate::oauth::API_KEY_PREFIX);
    sqlx::query("INSERT INTO api_keys (workspace_ids, user_id, label, key_hash, key_hint, scopes) VALUES (ARRAY[$1]::uuid[], $2, 'test', $3, 'x', $4)")
        .bind(w.workspace)
        .bind(w.ann.user)
        .bind(secret_hash(&key))
        .bind(vec!["read".to_string(), "write".to_string()])
        .execute(&w.state.db)
        .await
        .unwrap();
    key
}

fn get_file(uri: &str, header: Option<(&str, String)>) -> Request<Body> {
    let mut builder = Request::get(uri);
    if let Some((name, value)) = header {
        builder = builder.header(name, value);
    }
    builder.body(Body::empty()).unwrap()
}

#[sqlx::test]
async fn files_upload_once_and_download_for_members(db: PgPool) {
    let w = world(db).await;
    let router = crate::app::router(w.state.clone());
    call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "Crash" })).await;
    let bytes = b"\x89PNG fake image".to_vec();
    let prepared = call(
        &w.ann,
        "prepare_file_upload",
        json!({ "name": "shot.png", "contentType": "image/png", "size": bytes.len() }),
    )
    .await;
    let upload_url = prepared["uploadUrl"].as_str().unwrap();
    let asset_url = prepared["assetUrl"].as_str().unwrap();
    assert!(prepared["curl"].as_str().unwrap().contains(upload_url));
    assert_eq!(prepared["markdown"], format!("![shot.png]({asset_url})"));
    let upload_path = upload_url.strip_prefix("http://localhost:3000").unwrap();
    let file_path = asset_url.strip_prefix("http://localhost:3000").unwrap();

    let key = api_key(&w).await;
    let bearer = || Some(("authorization", format!("Bearer {key}")));
    let (status, _, _) = send(&router, get_file(file_path, bearer())).await;
    assert_eq!(status, StatusCode::NOT_FOUND, "nothing to download before the upload");

    let put = || Request::put(upload_path).header("content-type", "image/png").body(Body::from(bytes.clone())).unwrap();
    let (status, _, body) = send(&router, put()).await;
    assert_eq!(status, StatusCode::OK, "{}", String::from_utf8_lossy(&body));
    let stored: Value = serde_json::from_slice(&body).unwrap();
    assert_eq!(stored["fileId"], prepared["fileId"]);
    assert_eq!(stored["size"], bytes.len());
    let (status, _, _) = send(&router, put()).await;
    assert_eq!(status, StatusCode::NOT_FOUND, "upload URLs are single use");

    let (status, _, _) = send(&router, get_file(file_path, None)).await;
    assert_eq!(status, StatusCode::UNAUTHORIZED);
    let (status, _, _) = send(&router, get_file(file_path, Some(("authorization", "Bearer rr_key_nope".into())))).await;
    assert_eq!(status, StatusCode::UNAUTHORIZED);
    let (status, headers, body) = send(&router, get_file(file_path, bearer())).await;
    assert_eq!(status, StatusCode::OK);
    assert_eq!(body, bytes);
    assert_eq!(headers["content-type"], "image/png");
    assert_eq!(headers["x-content-type-options"], "nosniff");
    assert_eq!(headers["content-security-policy"], "sandbox");
    assert!(headers["content-disposition"].to_str().unwrap().starts_with("inline; filename=\"shot.png\""));

    let cookie = |user| format!("{}={}", w.state.keys.session_cookie, w.state.keys.session_token(user));
    let (status, _, body) = send(&router, get_file(file_path, Some(("cookie", cookie(w.ann.user))))).await;
    assert_eq!((status, body), (StatusCode::OK, bytes.clone()));
    let stranger = add_user(&w.state, "eve@elsewhere.test", "Eve Stranger").await;
    let (status, _, _) = send(&router, get_file(file_path, Some(("cookie", cookie(stranger))))).await;
    assert_eq!(status, StatusCode::NOT_FOUND);

    let attachment =
        call(&w.ann, "create_attachment_from_upload", json!({ "issue": "ENG-1", "fileId": prepared["fileId"] })).await;
    assert_eq!(attachment["url"], asset_url);
    assert_eq!(attachment["sourceType"], "upload");
    assert_eq!(attachment["title"], "shot.png");

    let inline = call(
        &w.ann,
        "upload_file",
        json!({ "name": "notes.html", "contentType": "text/html", "base64": "PGgxPmhpPC9oMT4=" }),
    )
    .await;
    let inline_path = inline["assetUrl"].as_str().unwrap().strip_prefix("http://localhost:3000").unwrap().to_string();
    let (status, headers, body) = send(&router, get_file(&inline_path, bearer())).await;
    assert_eq!((status, body), (StatusCode::OK, b"<h1>hi</h1>".to_vec()));
    assert!(headers["content-disposition"].to_str().unwrap().starts_with("attachment;"), "html is downloaded");

    let too_big = fails(
        &w.ann,
        "prepare_file_upload",
        json!({ "name": "big.bin", "contentType": "application/octet-stream", "size": 2 << 20 }),
    )
    .await;
    assert!(too_big.contains("at most"), "{too_big}");
    let bad_type = fails(&w.ann, "prepare_file_upload", json!({ "name": "x", "contentType": "png", "size": 1 })).await;
    assert!(bad_type.contains("content type"), "{bad_type}");
    let pending =
        call(&w.ann, "prepare_file_upload", json!({ "name": "later.txt", "contentType": "text/plain", "size": 1 }))
            .await;
    let not_yet =
        fails(&w.ann, "create_attachment_from_upload", json!({ "issue": "ENG-1", "fileId": pending["fileId"] })).await;
    assert!(not_yet.contains("uploaded"), "{not_yet}");
}

#[sqlx::test]
async fn search_finds_what_the_caller_can_see(db: PgPool) {
    let w = world(db).await;
    let gus = w.member("gus@acme.test", "Gus Guest", "guest").await;
    let mut tx = w.state.db.begin().await.unwrap();
    let design = crate::seed::create_team(&mut tx, w.workspace, w.ann.user, "Design", "DES").await.unwrap();
    tx.commit().await.unwrap();
    sqlx::query("INSERT INTO team_members (team_id, user_id) VALUES ($1, $2)")
        .bind(design)
        .bind(gus.user)
        .execute(&w.state.db)
        .await
        .unwrap();
    call(
        &w.ann,
        "save_issue",
        json!({ "team": "ENG", "title": "Login page crashes", "description": "The login button throws an error." }),
    )
    .await;
    call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "Signup flow", "description": "Mentions login once." }))
        .await;
    call(&w.ann, "save_issue", json!({ "team": "DES", "title": "Login mockups" })).await;
    call(&w.ann, "save_comment", json!({ "issueId": "ENG-2", "body": "There is a deadlock in the scheduler" })).await;
    let eng: uuid::Uuid =
        sqlx::query_scalar("SELECT id FROM teams WHERE key = 'ENG'").fetch_one(&w.state.db).await.unwrap();
    let project: uuid::Uuid = sqlx::query_scalar(
        "INSERT INTO projects (workspace_id, slug_id, name, status_id)
         VALUES ($1, 'abcdef123456', 'Login Revamp!', (SELECT id FROM project_statuses WHERE workspace_id = $1 LIMIT 1))
         RETURNING id",
    )
    .bind(w.workspace)
    .fetch_one(&w.state.db)
    .await
    .unwrap();
    sqlx::query("INSERT INTO project_teams (project_id, team_id) VALUES ($1, $2)")
        .bind(project)
        .bind(eng)
        .execute(&w.state.db)
        .await
        .unwrap();
    sqlx::query(
        "INSERT INTO documents (workspace_id, slug_id, title, content, team_id)
         VALUES ($1, '0123456789ab', 'Runbook', 'How to debug login crashes in production.', $2)",
    )
    .bind(w.workspace)
    .bind(eng)
    .execute(&w.state.db)
    .await
    .unwrap();

    let found = call(&w.ann, "search", json!({ "query": "login" })).await;
    let nodes = found["nodes"].as_array().unwrap();
    let titles: Vec<&str> = nodes.iter().map(|n| n["title"].as_str().unwrap()).collect();
    for title in ["Login page crashes", "Signup flow", "Login mockups", "Login Revamp!", "Runbook"] {
        assert!(titles.contains(&title), "{title} in {titles:?}");
    }
    let position = |title: &str| titles.iter().position(|t| *t == title).unwrap();
    assert!(position("Login page crashes") < position("Signup flow"), "title matches first: {titles:?}");
    assert!(position("Login Revamp!") < position("Runbook"), "title matches first: {titles:?}");
    let project_node = nodes.iter().find(|n| n["type"] == "project").unwrap();
    assert_eq!(project_node["url"], "http://localhost:3000/acme/project/login-revamp-abcdef123456");
    assert_eq!(project_node["status"]["name"], "Backlog");
    let document = nodes.iter().find(|n| n["type"] == "document").unwrap();
    assert_eq!(document["url"], "http://localhost:3000/acme/document/runbook-0123456789ab");
    assert!(document["snippet"].as_str().unwrap().contains("**login**"), "{document}");
    let issue = nodes.iter().find(|n| n["identifier"] == "ENG-1").unwrap();
    assert_eq!(issue["url"], "http://localhost:3000/acme/issue/ENG-1");
    assert_eq!(issue["state"]["name"], "Backlog");

    let by_identifier = call(&w.ann, "search", json!({ "query": "eng-2" })).await;
    assert_eq!(by_identifier["nodes"][0]["identifier"], "ENG-2");
    let only_issues = call(&w.ann, "search", json!({ "query": "crash", "types": ["issue"] })).await;
    assert_eq!(only_issues["nodes"].as_array().unwrap().len(), 1, "stemmed words match: {only_issues}");
    let in_team = call(&w.ann, "search", json!({ "query": "login", "team": "DES" })).await;
    assert_eq!(in_team["nodes"].as_array().unwrap().len(), 1);
    assert_eq!(in_team["nodes"][0]["identifier"], "DES-1");
    let comments = call(&w.ann, "search", json!({ "query": "deadlock", "types": ["comments"] })).await;
    assert_eq!(comments["nodes"][0]["type"], "comment");
    assert_eq!(comments["nodes"][0]["identifier"], "ENG-2");
    let default_types = call(&w.ann, "search", json!({ "query": "deadlock" })).await;
    assert_eq!(default_types["nodes"].as_array().unwrap().len(), 0, "comments only when asked for");

    let guest =
        call(&gus, "search", json!({ "query": "login", "types": ["issue", "project", "document", "comment"] })).await;
    let guest_titles: Vec<&str> =
        guest["nodes"].as_array().unwrap().iter().map(|n| n["title"].as_str().unwrap()).collect();
    assert_eq!(guest_titles, ["Login mockups"]);
    let guest_comments = call(&gus, "search", json!({ "query": "deadlock", "types": ["comment"] })).await;
    assert_eq!(guest_comments["nodes"].as_array().unwrap().len(), 0);

    let bad = fails(&w.ann, "search", json!({ "query": "x", "types": ["widgets"] })).await;
    assert!(bad.contains("initiative"), "{bad}");
}

#[sqlx::test]
async fn comments_on_projects_and_updates_notify_the_mentioned(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    let project: uuid::Uuid = sqlx::query_scalar(
        "INSERT INTO projects (workspace_id, slug_id, name, status_id)
         VALUES ($1, 'abcdef123456', 'Launch', (SELECT id FROM project_statuses WHERE workspace_id = $1 LIMIT 1))
         RETURNING id",
    )
    .bind(w.workspace)
    .fetch_one(&w.state.db)
    .await
    .unwrap();
    sqlx::query("INSERT INTO project_teams (project_id, team_id) SELECT $1, id FROM teams WHERE key = 'ENG'")
        .bind(project)
        .execute(&w.state.db)
        .await
        .unwrap();
    let update: uuid::Uuid = sqlx::query_scalar(
        "INSERT INTO status_updates (workspace_id, project_id, user_id, body, health) VALUES ($1, $2, $3, 'On track', 'onTrack') RETURNING id",
    )
    .bind(w.workspace)
    .bind(project)
    .bind(bob.user)
    .fetch_one(&w.state.db)
    .await
    .unwrap();

    let comment = call(&w.ann, "save_comment", json!({ "projectId": "Launch", "body": "@bob what's left?" })).await;
    assert_eq!(comment["project"]["name"], "Launch");
    call(&w.ann, "save_comment", json!({ "statusUpdateId": update, "body": "Nice progress" })).await;
    let inbox = call(&bob, "get_notifications", json!({})).await;
    assert_eq!(kinds(&inbox), ["projectUpdateNewComment", "projectCommentMention"]);
    assert_eq!(summaries(&inbox)[1], "Ann Smith mentioned you in a comment on Launch");
    assert_eq!(inbox["nodes"][1]["project"]["name"], "Launch");
    assert_eq!(inbox["nodes"][0]["statusUpdate"]["id"], json!(update));
    let threads = call(&bob, "list_comments", json!({ "projectId": "Launch" })).await;
    assert_eq!(threads["nodes"][0]["body"], "@bob what's left?");
    let reaction = call(&bob, "add_reaction", json!({ "statusUpdateId": update, "emoji": "rocket" })).await;
    assert_eq!(reaction["statusUpdate"]["id"], json!(update));
}
