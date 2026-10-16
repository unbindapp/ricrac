use serde_json::json;
use sqlx::PgPool;

use super::*;

#[sqlx::test]
async fn creates_issues_with_identifiers_and_defaults(db: PgPool) {
    let w = world(db).await;
    let first = call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "First", "description": "Hello" })).await;
    assert_eq!(first["identifier"], "ENG-1");
    assert_eq!(first["state"]["name"], "Backlog");
    assert_eq!(first["creator"]["email"], "ann@acme.test");
    assert_eq!(first["url"], "http://localhost:3000/acme/issue/ENG-1");
    assert_eq!(first["branchName"], "eng-1-first");
    let second = call(
        &w.ann,
        "save_issue",
        json!({ "title": "Second", "state": "started", "priority": 1, "assignee": "me", "labels": ["Bug"] }),
    )
    .await;
    assert_eq!(second["identifier"], "ENG-2", "the only team is used when none is named");
    assert_eq!(second["state"]["name"], "In Progress");
    assert!(second["startedAt"].is_string());
    assert_eq!(second["priorityLabel"], "Urgent");
    assert_eq!(second["assignee"]["name"], "Ann Smith");
    assert_eq!(second["labels"][0]["name"], "Bug");
    let missing = fails(&w.ann, "save_issue", json!({ "team": "ENG" })).await;
    assert!(missing.contains("title"), "{missing}");
    let unknown = fails(&w.ann, "save_issue", json!({ "team": "ENG", "title": "x", "state": "Nope" })).await;
    assert!(unknown.contains("Backlog") && unknown.contains("Todo"), "lists the options: {unknown}");
    let typo = fails(&w.ann, "save_issue", json!({ "team": "ENG", "title": "x", "titel": "y" })).await;
    assert!(typo.contains("titel"), "{typo}");
}

#[sqlx::test]
async fn updates_states_with_timestamps_and_history(db: PgPool) {
    let w = world(db).await;
    call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "Fix login" })).await;
    let started = call(&w.ann, "save_issue", json!({ "id": "eng-1", "state": "In Progress" })).await;
    assert!(started["startedAt"].is_string());
    let done = call(&w.ann, "save_issue", json!({ "id": "ENG-1", "state": "Done", "title": "Fix the login" })).await;
    assert!(done["completedAt"].is_string());
    assert_eq!(done["title"], "Fix the login");
    let reopened = call(&w.ann, "save_issue", json!({ "id": "ENG-1", "state": "Todo" })).await;
    assert!(reopened["completedAt"].is_null() && reopened["startedAt"].is_null());
    let history = call(&w.ann, "get_issue_history", json!({ "id": "ENG-1" })).await;
    let actions: Vec<&str> =
        history["history"].as_array().unwrap().iter().map(|h| h["action"].as_str().unwrap()).collect();
    assert_eq!(actions[0], "created");
    assert!(history["history"].as_array().unwrap().iter().any(|h| h["changes"]["title"]["to"] == "Fix the login"));
}

#[sqlx::test]
async fn patches_descriptions_and_clears_fields(db: PgPool) {
    let w = world(db).await;
    call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "Doc", "description": "## Steps\n\n1. Open\n2. Crash\n", "dueDate": "2030-01-02", "estimate": null })).await;
    let patched = call(
        &w.ann,
        "save_issue",
        json!({ "id": "ENG-1", "patch": [
        { "op": "replace", "old_string": "Crash", "new_string": "It crashes" },
        { "op": "append", "text": "\nExpected: no crash\n" }
    ], "dueDate": null }),
    )
    .await;
    assert_eq!(patched["description"], "## Steps\n\n1. Open\n2. It crashes\n\nExpected: no crash\n");
    assert!(patched["dueDate"].is_null());
    let both = fails(
        &w.ann,
        "save_issue",
        json!({ "id": "ENG-1", "description": "x", "patch": [{ "op": "append", "text": "y" }] }),
    )
    .await;
    assert!(both.contains("either"), "{both}");
}

#[sqlx::test]
async fn lists_with_simple_arguments_and_filters(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    call(&w.ann, "save_issue", json!({ "title": "Urgent bug", "priority": 1, "labels": ["Bug"], "assignee": "bob" }))
        .await;
    call(
        &w.ann,
        "save_issue",
        json!({ "title": "Feature work", "priority": 3, "labels": ["Feature"], "assignee": "me", "state": "started" }),
    )
    .await;
    call(&w.ann, "save_issue", json!({ "title": "Unassigned idea", "description": "about payments" })).await;

    assert_eq!(identifiers(&call(&bob, "list_issues", json!({ "assignee": "me" })).await), vec!["ENG-1"]);
    assert_eq!(identifiers(&call(&w.ann, "list_issues", json!({ "assignee": "null" })).await), vec!["ENG-3"]);
    assert_eq!(identifiers(&call(&w.ann, "list_issues", json!({ "label": "bug" })).await), vec!["ENG-1"]);
    assert_eq!(identifiers(&call(&w.ann, "list_issues", json!({ "state": "started" })).await), vec!["ENG-2"]);
    assert_eq!(identifiers(&call(&w.ann, "list_issues", json!({ "query": "payments" })).await), vec!["ENG-3"]);
    assert_eq!(identifiers(&call(&w.ann, "list_issues", json!({ "query": "ENG-2" })).await), vec!["ENG-2"]);
    let filtered = call(
        &w.ann,
        "list_issues",
        json!({ "filter": { "or": [
        { "priority": { "lte": 2, "neq": 0 } },
        { "labels": { "some": { "name": { "eq": "feature" } } } }
    ] }, "orderBy": "priority" }),
    )
    .await;
    assert_eq!(identifiers(&filtered), vec!["ENG-1", "ENG-2"]);
    let none = call(&w.ann, "list_issues", json!({ "filter": { "labels": { "null": true } } })).await;
    assert_eq!(identifiers(&none), vec!["ENG-3"]);
    let bad = fails(&w.ann, "list_issues", json!({ "filter": { "colour": { "eq": "red" } } })).await;
    assert!(bad.contains("colour"), "{bad}");
    let page = call(&w.ann, "list_issues", json!({ "limit": 2, "orderBy": "identifier" })).await;
    assert_eq!(identifiers(&page), vec!["ENG-1", "ENG-2"]);
    let next = call(
        &w.ann,
        "list_issues",
        json!({ "limit": 2, "orderBy": "identifier", "cursor": page["pageInfo"]["endCursor"] }),
    )
    .await;
    assert_eq!(identifiers(&next), vec!["ENG-3"]);
    assert_eq!(next["pageInfo"]["hasNextPage"], false);
}

#[sqlx::test]
async fn relations_sub_issues_and_parent_automation(db: PgPool) {
    let w = world(db).await;
    sqlx::query("UPDATE teams SET auto_close_parent_issues = true").execute(&w.state.db).await.unwrap();
    call(&w.ann, "save_issue", json!({ "title": "Parent" })).await;
    call(&w.ann, "save_issue", json!({ "title": "Child A", "parentId": "ENG-1" })).await;
    call(&w.ann, "save_issue", json!({ "title": "Child B", "parentId": "ENG-1", "blocks": ["ENG-2"] })).await;
    let parent = call(&w.ann, "get_issue", json!({ "id": "ENG-1" })).await;
    assert_eq!(parent["children"].as_array().unwrap().len(), 2);
    let a = call(&w.ann, "get_issue", json!({ "id": "ENG-2" })).await;
    assert_eq!(a["relations"]["blockedBy"][0]["identifier"], "ENG-3");
    let cycle = fails(&w.ann, "save_issue", json!({ "id": "ENG-1", "parentId": "ENG-2" })).await;
    assert!(cycle.contains("own sub-issues"), "{cycle}");
    call(&w.ann, "save_issue", json!({ "id": "ENG-2", "state": "Done" })).await;
    let parent = call(&w.ann, "get_issue", json!({ "id": "ENG-1" })).await;
    assert_eq!(parent["state"]["name"], "Backlog", "one child is still open");
    call(&w.ann, "update_issues", json!({ "ids": ["ENG-3"], "changes": { "state": "completed" } })).await;
    let parent = call(&w.ann, "get_issue", json!({ "id": "ENG-1", "includeHistory": true })).await;
    assert_eq!(parent["state"]["type"], "completed");
    assert!(parent["history"].as_array().unwrap().iter().any(|h| h["action"] == "auto_completed_by_sub_issues"));
    let dup = call(&w.ann, "save_issue", json!({ "title": "Same", "duplicateOf": "ENG-1" })).await;
    assert_eq!(dup["state"]["type"], "canceled");
}

#[sqlx::test]
async fn moving_teams_keeps_old_identifiers_working(db: PgPool) {
    let w = world(db).await;
    let mut tx = w.state.db.begin().await.unwrap();
    seed::create_team(&mut tx, w.workspace, w.ann.user, "Design", "DES").await.unwrap();
    tx.commit().await.unwrap();
    call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "Logo", "state": "In Progress" })).await;
    let moved = call(&w.ann, "save_issue", json!({ "id": "ENG-1", "team": "Design" })).await;
    assert_eq!(moved["identifier"], "DES-1");
    assert_eq!(moved["state"]["name"], "In Progress", "a status of the same name");
    assert_eq!(moved["previousIdentifiers"], json!(["ENG-1"]));
    let found = call(&w.ann, "get_issue", json!({ "id": "ENG-1" })).await;
    assert_eq!(found["identifier"], "DES-1");
}

#[sqlx::test]
async fn trash_archive_and_private_teams(db: PgPool) {
    let w = world(db).await;
    let guest = w.member("gus@else.test", "Gus", "guest").await;
    call(&w.ann, "save_issue", json!({ "title": "Secret" })).await;
    assert!(fails(&guest, "get_issue", json!({ "id": "ENG-1" })).await.contains("No issue"));
    call(&w.ann, "share_issue", json!({ "id": "ENG-1", "user": "gus@else.test" })).await;
    assert_eq!(call(&guest, "get_issue", json!({ "id": "ENG-1" })).await["title"], "Secret");
    call(&w.ann, "delete_issue", json!({ "id": "ENG-1" })).await;
    assert!(identifiers(&call(&w.ann, "list_issues", json!({})).await).is_empty());
    assert_eq!(identifiers(&call(&w.ann, "list_issues", json!({ "includeTrashed": true })).await), vec!["ENG-1"]);
    call(&w.ann, "restore_issue", json!({ "id": "ENG-1" })).await;
    call(&w.ann, "archive_issue", json!({ "id": "ENG-1" })).await;
    assert!(identifiers(&call(&w.ann, "list_issues", json!({})).await).is_empty());
    call(&w.ann, "unarchive_issue", json!({ "id": "ENG-1" })).await;
    assert_eq!(identifiers(&call(&w.ann, "list_issues", json!({})).await), vec!["ENG-1"]);
}

#[sqlx::test]
async fn notifies_assignees_and_mentions(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    let cat = w.member("cat@acme.test", "Cat Lee", "member").await;
    call(&w.ann, "save_issue", json!({ "title": "Review", "assignee": "bob@acme.test", "description": "cc @cat" }))
        .await;
    let kinds = |user| {
        let db = w.state.db.clone();
        async move {
            sqlx::query_scalar::<_, String>("SELECT type FROM notifications WHERE user_id = $1 ORDER BY created_at")
                .bind(user)
                .fetch_all(&db)
                .await
                .unwrap()
        }
    };
    assert_eq!(kinds(bob.user).await, vec!["issueAssignedToYou"]);
    assert_eq!(kinds(cat.user).await, vec!["issueMention"]);
    assert!(kinds(w.ann.user).await.is_empty(), "nobody hears about their own change");
    call(&bob, "save_issue", json!({ "id": "ENG-1", "state": "Done" })).await;
    assert_eq!(kinds(w.ann.user).await, vec!["issueStatusChanged"]);
}
