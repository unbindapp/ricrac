use serde_json::json;
use sqlx::PgPool;

use super::*;

async fn enable_cycles(w: &World) {
    sqlx::query("UPDATE teams SET cycles_enabled = true").execute(&w.state.db).await.unwrap();
    ops::cycles::run_automations(&w.state).await.unwrap();
}

fn numbers(list: &Value) -> Vec<i64> {
    list["nodes"].as_array().unwrap().iter().map(|n| n["number"].as_i64().unwrap()).collect()
}

fn titles(list: &Value) -> Vec<String> {
    list["nodes"].as_array().unwrap().iter().map(|n| n["title"].as_str().unwrap().to_string()).collect()
}

#[sqlx::test]
async fn cycles_are_created_ahead_and_roll_unfinished_issues_over(db: PgPool) {
    let w = world(db).await;
    enable_cycles(&w).await;
    ops::cycles::run_automations(&w.state).await.unwrap();
    assert_eq!(numbers(&call(&w.ann, "list_cycles", json!({ "team": "ENG" })).await), vec![3, 2, 1]);
    assert_eq!(numbers(&call(&w.ann, "list_cycles", json!({ "team": "ENG", "type": "upcoming" })).await), vec![2, 3]);
    let current = call(&w.ann, "get_cycle", json!({ "id": "current", "team": "ENG" })).await;
    assert_eq!((current["number"].as_i64(), current["isActive"].as_bool()), (Some(1), Some(true)));
    assert_eq!(current["history"].as_array().unwrap().len(), 1, "one snapshot a day");

    call(&w.ann, "save_issue", json!({ "title": "Carry over", "cycle": "current", "estimate": null })).await;
    call(&w.ann, "save_issue", json!({ "title": "Done in time", "cycle": "current", "state": "Done" })).await;
    let current = call(&w.ann, "get_cycle", json!({ "id": "current", "team": "ENG" })).await;
    assert_eq!(current["scope"]["issues"], 2);
    assert_eq!(current["scope"]["completed"], 1);
    assert_eq!(current["progress"], 0.5);
    assert_eq!(current["issues"].as_array().unwrap().len(), 2);

    sqlx::query("UPDATE cycles SET ends_at = now() - interval '1 minute' WHERE number = 1")
        .execute(&w.state.db)
        .await
        .unwrap();
    ops::cycles::run_automations(&w.state).await.unwrap();
    let first = call(&w.ann, "get_cycle", json!({ "id": "1", "team": "ENG" })).await;
    assert!(first["completedAt"].is_string());
    assert_eq!(first["issues"][0]["identifier"], "ENG-2", "finished issues stay");
    assert_eq!(first["issues"].as_array().unwrap().len(), 1);
    let moved = call(&w.ann, "get_issue", json!({ "id": "ENG-1" })).await;
    assert_eq!(moved["cycle"]["number"], 2);
    let history = call(&w.ann, "get_issue_history", json!({ "id": "ENG-1" })).await;
    let rollover = history["history"].as_array().unwrap().iter().find(|h| h["action"] == "cycle_rollover").unwrap();
    assert_eq!(rollover["changes"]["cycle"]["from"]["number"], 1);
    assert_eq!(rollover["changes"]["cycle"]["to"]["number"], 2);
    assert!(rollover["actor"].is_null());
    assert_eq!(numbers(&call(&w.ann, "list_cycles", json!({ "team": "ENG", "type": "completed" })).await), vec![1]);
    assert_eq!(numbers(&call(&w.ann, "list_cycles", json!({ "team": "ENG" })).await), vec![3, 2, 1]);
}

#[sqlx::test]
async fn manual_cycles_dont_overlap(db: PgPool) {
    let w = world(db).await;
    let disabled =
        fails(&w.ann, "save_cycle", json!({ "team": "ENG", "startsAt": "2030-01-01", "endsAt": "2030-01-15" })).await;
    assert!(disabled.contains("cyclesEnabled"), "{disabled}");
    enable_cycles(&w).await;
    let overlap = fails(&w.ann, "save_cycle", json!({ "team": "ENG", "startsAt": "-PT1H", "endsAt": "P3D" })).await;
    assert!(overlap.contains("overlap cycle 1"), "{overlap}");
    let launch = call(
        &w.ann,
        "save_cycle",
        json!({ "team": "ENG", "startsAt": "2030-01-01", "endsAt": "2030-01-15", "name": "Launch" }),
    )
    .await;
    assert_eq!(launch["number"], 4);
    assert_eq!(launch["isFuture"], true);
    let backwards = fails(&w.ann, "save_cycle", json!({ "id": launch["id"], "endsAt": "2029-12-01" })).await;
    assert!(backwards.contains("after"), "{backwards}");
    let renamed = call(&w.ann, "save_cycle", json!({ "id": "Launch", "team": "ENG", "name": "Big launch" })).await;
    assert_eq!(renamed["name"], "Big launch");
    let current = fails(&w.ann, "archive_cycle", json!({ "id": "current", "team": "ENG" })).await;
    assert!(current.contains("current"), "{current}");
    call(&w.ann, "save_issue", json!({ "title": "Later", "cycle": "4" })).await;
    let archived = call(&w.ann, "archive_cycle", json!({ "id": launch["id"] })).await;
    assert!(archived["archivedAt"].is_string());
    assert!(call(&w.ann, "get_issue", json!({ "id": "ENG-1" })).await["cycle"].is_null());
    assert_eq!(numbers(&call(&w.ann, "list_cycles", json!({ "team": "ENG" })).await), vec![3, 2, 1]);
}

#[sqlx::test]
async fn documents_keep_versions_and_follow_visibility(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    let gus = w.member("gus@acme.test", "Gus Guest", "guest").await;
    call(&w.ann, "save_issue", json!({ "title": "Spec" })).await;
    let doc = call(
        &w.ann,
        "save_document",
        json!({ "title": "Design Notes!", "content": "Hello @bob", "team": "ENG", "icon": "📐" }),
    )
    .await;
    let slug = doc["slugId"].as_str().unwrap();
    assert_eq!(doc["url"], format!("http://localhost:3000/acme/document/design-notes-{slug}"));
    assert_eq!(doc["team"]["key"], "ENG");
    assert_eq!(doc["versionCount"], 0);
    let mentions: i64 = sqlx::query_scalar("SELECT count(*) FROM notifications WHERE type = 'documentMention'")
        .fetch_one(&w.state.db)
        .await
        .unwrap();
    assert_eq!(mentions, 1);
    let two = fails(&w.ann, "save_document", json!({ "title": "x", "team": "ENG", "issue": "ENG-1" })).await;
    assert!(two.contains("only one"), "{two}");
    let none = fails(&w.ann, "save_document", json!({ "title": "x" })).await;
    assert!(none.contains("belongs"), "{none}");
    call(&w.ann, "save_document", json!({ "title": "Issue doc", "issue": "ENG-1" })).await;

    let patched = call(
        &w.ann,
        "save_document",
        json!({ "id": slug, "patch": [{ "op": "replace", "old_string": "Hello", "new_string": "Hi" }] }),
    )
    .await;
    assert_eq!(patched["content"], "Hi @bob");
    assert_eq!(patched["versionCount"], 1);
    let again = call(&w.ann, "save_document", json!({ "id": doc["id"], "content": "Third" })).await;
    assert_eq!(again["versionCount"], 1, "one author's edits within 10 minutes make one version");
    let by_bob = call(&bob, "save_document", json!({ "id": doc["url"], "content": "Bob's" })).await;
    assert_eq!(by_bob["versionCount"], 2);
    assert_eq!(by_bob["updatedBy"]["email"], "bob@acme.test");
    let versions = call(&w.ann, "list_document_versions", json!({ "id": doc["id"] })).await;
    assert_eq!(versions[0]["author"]["email"], "ann@acme.test");
    let version = call(&w.ann, "get_document_version", json!({ "id": versions[0]["id"].to_string() })).await;
    assert_eq!(version["content"], "Third");

    let team_docs = call(&w.ann, "list_documents", json!({ "team": "ENG" })).await;
    assert_eq!(titles(&team_docs), vec!["Design Notes!"]);
    assert!(team_docs["nodes"][0].get("content").is_none());
    assert_eq!(titles(&call(&w.ann, "list_documents", json!({ "issue": "ENG-1" })).await), vec!["Issue doc"]);
    assert_eq!(titles(&call(&w.ann, "list_documents", json!({ "query": "design" })).await), vec!["Design Notes!"]);
    assert_eq!(titles(&call(&bob, "list_documents", json!({})).await).len(), 2);
    assert!(titles(&call(&gus, "list_documents", json!({})).await).is_empty());
    fails(&gus, "get_document", json!({ "id": doc["id"] })).await;
    fails(&gus, "get_document_version", json!({ "id": versions[0]["id"].to_string() })).await;

    call(&w.ann, "delete_document", json!({ "id": doc["id"] })).await;
    assert_eq!(titles(&call(&w.ann, "list_documents", json!({})).await), vec!["Issue doc"]);
    fails(&w.ann, "get_document", json!({ "id": doc["id"] })).await;
    let restored = call(&w.ann, "restore_document", json!({ "id": doc["id"] })).await;
    assert_eq!(restored["trashed"], false);
    let moved = call(&w.ann, "save_document", json!({ "id": doc["id"], "issue": "ENG-1" })).await;
    assert!(moved["team"].is_null());
    assert_eq!(moved["issue"]["identifier"], "ENG-1");
}

#[sqlx::test]
async fn custom_views_filter_issues(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    call(&w.ann, "save_issue", json!({ "title": "Urgent", "priority": 1 })).await;
    call(&w.ann, "save_issue", json!({ "title": "Later", "priority": 4 })).await;
    let view = call(
        &w.ann,
        "save_custom_view",
        json!({ "name": "Urgent work", "filter": { "priority": { "eq": 1 } }, "shared": true,
                "display": { "groupBy": "state" }, "team": "ENG" }),
    )
    .await;
    let slug = view["slugId"].as_str().unwrap();
    assert_eq!(view["url"], format!("http://localhost:3000/acme/view/urgent-work-{slug}"));
    assert_eq!(view["model"], "issue");
    assert_eq!(identifiers(&call(&bob, "list_issues", json!({ "customView": "Urgent work" })).await), vec!["ENG-1"]);
    assert_eq!(identifiers(&call(&w.ann, "list_issues", json!({ "customView": slug })).await), vec!["ENG-1"]);
    let bad =
        fails(&w.ann, "save_custom_view", json!({ "name": "Bad", "filter": { "colour": { "eq": "red" } } })).await;
    assert!(bad.contains("colour"), "{bad}");

    call(&bob, "save_custom_view", json!({ "name": "Mine", "filter": { "assignee": { "isMe": { "eq": true } } } }))
        .await;
    let names = |list: Value| -> Vec<String> {
        list["nodes"].as_array().unwrap().iter().map(|n| n["name"].as_str().unwrap().to_string()).collect()
    };
    assert_eq!(names(call(&bob, "list_custom_views", json!({})).await), vec!["Mine", "Urgent work"]);
    assert_eq!(names(call(&bob, "list_custom_views", json!({ "shared": false })).await), vec!["Mine"]);
    assert_eq!(names(call(&w.ann, "list_custom_views", json!({})).await), vec!["Urgent work"]);
    fails(&w.ann, "get_custom_view", json!({ "id": "Mine" })).await;
    let not_owner = fails(&bob, "save_custom_view", json!({ "id": view["id"], "name": "Mine now" })).await;
    assert!(not_owner.contains("owner"), "{not_owner}");
    let updated =
        call(&w.ann, "save_custom_view", json!({ "id": view["id"], "filter": { "priority": { "eq": 4 } } })).await;
    assert_eq!(updated["filter"]["priority"]["eq"], 4);
    assert_eq!(identifiers(&call(&w.ann, "list_issues", json!({ "customView": slug })).await), vec!["ENG-2"]);
    call(&w.ann, "delete_custom_view", json!({ "id": view["id"] })).await;
    fails(&w.ann, "get_custom_view", json!({ "id": view["id"] })).await;
}

#[sqlx::test]
async fn favorites_are_added_once_and_hide_what_is_gone(db: PgPool) {
    let w = world(db).await;
    call(&w.ann, "save_issue", json!({ "title": "Pinned" })).await;
    let first = call(&w.ann, "add_favorite", json!({ "type": "issue", "target": "ENG-1" })).await;
    assert_eq!(first["target"]["identifier"], "ENG-1");
    call(&w.ann, "add_favorite", json!({ "type": "team", "target": "ENG", "folder": "Teams" })).await;
    call(&w.ann, "add_favorite", json!({ "type": "user", "target": "me" })).await;
    let again = call(&w.ann, "add_favorite", json!({ "type": "issue", "target": "ENG-1", "sortOrder": 5000 })).await;
    assert_eq!(again["id"], first["id"]);
    let list = call(&w.ann, "list_favorites", json!({})).await;
    let types: Vec<&str> = list["nodes"].as_array().unwrap().iter().map(|n| n["type"].as_str().unwrap()).collect();
    assert_eq!(types, vec!["user", "issue", "team"]);
    assert_eq!(list["nodes"][2]["target"]["key"], "ENG");
    let wrong = fails(&w.ann, "add_favorite", json!({ "type": "board", "target": "x" })).await;
    assert!(wrong.contains("customView"), "{wrong}");

    call(&w.ann, "delete_issue", json!({ "id": "ENG-1" })).await;
    assert_eq!(call(&w.ann, "list_favorites", json!({})).await["nodes"].as_array().unwrap().len(), 2);
    call(&w.ann, "remove_favorite", json!({ "id": first["id"] })).await;
    call(&w.ann, "remove_favorite", json!({ "id": "ENG", "type": "team" })).await;
    let left = call(&w.ann, "list_favorites", json!({})).await;
    assert_eq!(left["nodes"].as_array().unwrap().len(), 1);
    fails(&w.ann, "remove_favorite", json!({ "id": first["id"] })).await;
}

#[sqlx::test]
async fn templates_fill_in_issues_and_documents(db: PgPool) {
    let w = world(db).await;
    let gus = w.member("gus@acme.test", "Gus Guest", "guest").await;
    let template = call(
        &w.ann,
        "save_template",
        json!({ "name": "Bug report", "type": "issue", "team": "ENG", "data": {
            "description": "## Steps", "priority": 2, "labels": ["Bug"],
            "subIssues": [{ "title": "Reproduce" }, { "title": "Fix", "priority": 1 }]
        } }),
    )
    .await;
    assert_eq!(template["team"]["key"], "ENG");
    let unknown =
        fails(&w.ann, "save_template", json!({ "name": "x", "type": "issue", "data": { "titel": "x" } })).await;
    assert!(unknown.contains("titel") && unknown.contains("subIssues"), "{unknown}");
    let bad_sub = fails(
        &w.ann,
        "save_template",
        json!({ "name": "x", "type": "issue", "data": { "subIssues": [{ "titel": "x" }] } }),
    )
    .await;
    assert!(bad_sub.contains("titel"), "{bad_sub}");
    let bad_milestone = fails(
        &w.ann,
        "save_template",
        json!({ "name": "x", "type": "project", "data": { "milestones": [{ "name": "M1", "due": "x" }] } }),
    )
    .await;
    assert!(bad_milestone.contains("targetDate"), "{bad_milestone}");
    fails(&gus, "save_template", json!({ "name": "x", "type": "document", "data": {} })).await;

    let listed = call(&w.ann, "list_templates", json!({ "type": "issue" })).await;
    assert_eq!(listed["nodes"].as_array().unwrap().len(), 1);
    assert!(listed["nodes"][0].get("data").is_none());
    assert!(call(&gus, "list_templates", json!({})).await["nodes"].as_array().unwrap().is_empty());
    let read = call(&w.ann, "get_template", json!({ "id": "bug report" })).await;
    assert_eq!(read["data"]["subIssues"].as_array().unwrap().len(), 2);

    let issue = call(&w.ann, "save_issue", json!({ "template": "Bug report", "title": "Bug: crash" })).await;
    assert_eq!((issue["priority"].as_i64(), issue["labels"][0]["name"].as_str()), (Some(2), Some("Bug")));
    assert_eq!(issue["description"], "## Steps");
    assert_eq!(issue["team"]["key"], "ENG");
    let parent = call(&w.ann, "get_issue", json!({ "id": issue["identifier"] })).await;
    let children: Vec<&str> =
        parent["children"].as_array().unwrap().iter().map(|c| c["title"].as_str().unwrap()).collect();
    assert_eq!(children, vec!["Reproduce", "Fix"]);

    call(
        &w.ann,
        "save_template",
        json!({ "name": "RFC", "type": "document", "data": { "title": "RFC", "content": "## Problem" } }),
    )
    .await;
    let doc = call(&w.ann, "save_document", json!({ "template": "RFC", "team": "ENG" })).await;
    assert_eq!((doc["title"].as_str(), doc["content"].as_str()), (Some("RFC"), Some("## Problem")));
    call(&w.ann, "delete_template", json!({ "id": template["id"] })).await;
    fails(&w.ann, "get_template", json!({ "id": "Bug report" })).await;
}

#[sqlx::test]
async fn recurring_templates_create_issues_on_schedule(db: PgPool) {
    let w = world(db).await;
    let template = call(
        &w.ann,
        "save_template",
        json!({ "name": "Weekly triage", "type": "issue", "team": "ENG", "data": { "priority": 3 },
                "recurrence": { "every": 1, "unit": "week", "startsAt": "-PT1M" } }),
    )
    .await;
    assert_eq!(template["recurrence"]["unit"], "week");
    crate::ops::templates::run_recurring(&w.state).await.unwrap();
    crate::ops::templates::run_recurring(&w.state).await.unwrap();
    let issues = call(&w.ann, "list_issues", json!({})).await;
    assert_eq!(identifiers(&issues), vec!["ENG-1"], "one issue per due run");
    assert_eq!(issues["nodes"][0]["title"], "Weekly triage");
    let next = call(&w.ann, "get_template", json!({ "id": "Weekly triage" })).await;
    assert!(next["nextRunAt"].as_str().unwrap() > chrono::Utc::now().to_rfc3339().as_str());
    let teamless = fails(
        &w.ann,
        "save_template",
        json!({ "name": "x", "type": "issue", "data": {}, "recurrence": { "every": 1, "unit": "day" } }),
    )
    .await;
    assert!(teamless.contains("team"), "{teamless}");
    let stopped = call(&w.ann, "save_template", json!({ "id": template["id"], "recurrence": null })).await;
    assert!(stopped["nextRunAt"].is_null());
}
