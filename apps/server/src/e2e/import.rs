use base64::Engine;
use serde_json::json;
use sqlx::PgPool;

use super::*;

#[sqlx::test]
async fn imports_issues_with_their_numbers_dates_people_and_comments(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    let result = call(
        &w.ann,
        "import_issues",
        json!({ "team": "ENG", "issues": [
            {
                "number": 5,
                "title": "Login fails",
                "description": "Steps for @bob\n\n1. Open  \n2. Crash\n",
                "state": "Done",
                "priority": 2,
                "labels": ["Bug"],
                "creator": "bob@acme.test",
                "assignee": "bob@acme.test",
                "createdAt": "2025-02-09T08:00:01Z",
                "completedAt": "2025-02-10T09:00:00Z",
                "links": [{ "url": "https://github.com/acme/app/pull/4", "title": "Fix login", "subtitle": "master" }],
                "comments": [
                    { "body": "Seen on staging", "author": "bob@acme.test", "createdAt": "2025-02-09T10:00:00Z" },
                    { "body": "Fixed", "createdAt": "2025-02-10T09:00:00Z" }
                ]
            },
            { "number": 7, "title": "Login is slow", "parent": "ENG-5", "relatedTo": ["ENG-5"], "blockedBy": ["ENG-8"] },
            { "title": "Numbered next", "state": "Todo" }
        ]}),
    )
    .await;
    assert_eq!(result["identifiers"], json!(["ENG-5", "ENG-7", "ENG-8"]));
    assert_eq!(result["comments"], 2);
    assert_eq!(result["nextIdentifier"], "ENG-9");

    let issue = call(&w.ann, "get_issue", json!({ "id": "ENG-5", "includeComments": true })).await;
    assert_eq!(issue["description"], "Steps for @bob\n\n1. Open  \n2. Crash\n", "kept as is");
    assert_eq!(issue["state"]["name"], "Done");
    assert_eq!(issue["creator"]["name"], "Bob Jones");
    assert_eq!(issue["assignee"]["name"], "Bob Jones");
    assert!(issue["createdAt"].as_str().unwrap().starts_with("2025-02-09T08:00:01"));
    assert!(issue["completedAt"].as_str().unwrap().starts_with("2025-02-10T09:00:00"));
    assert!(issue["startedAt"].as_str().unwrap().starts_with("2025-02-09T08:00:01"), "set from createdAt");
    assert_eq!(issue["labels"][0]["name"], "Bug");
    assert_eq!(issue["attachments"][0]["subtitle"], "master");
    let comments = issue["comments"].as_array().unwrap();
    assert_eq!(comments[0]["user"]["name"], "Bob Jones");
    assert!(comments[0]["createdAt"].as_str().unwrap().starts_with("2025-02-09T10:00:00"));
    assert_eq!(comments[1]["user"]["name"], "Ann Smith");
    assert_eq!(issue["relations"]["related"][0]["identifier"], "ENG-7");
    assert_eq!(issue["children"][0]["identifier"], "ENG-7");

    let next = call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "After the import" })).await;
    assert_eq!(next["identifier"], "ENG-9");
    let slow = call(&w.ann, "get_issue", json!({ "id": "ENG-7" })).await;
    assert_eq!(slow["relations"]["blockedBy"][0]["identifier"], "ENG-8", "points at an issue later in the import");

    let gap =
        call(&w.ann, "import_issues", json!({ "team": "ENG", "issues": [{ "number": 6, "title": "Filled gap" }] }))
            .await;
    assert_eq!(gap["identifiers"], json!(["ENG-6"]));
    assert_eq!(gap["nextIdentifier"], "ENG-10", "filling a gap doesn't move the counter back");

    let inbox = call(&bob, "get_notifications", json!({})).await;
    assert!(inbox["nodes"].as_array().unwrap().is_empty(), "imports notify nobody: {inbox}");
}

#[sqlx::test]
async fn refuses_taken_numbers_and_imports_nothing_on_error(db: PgPool) {
    let w = world(db).await;
    call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "Existing" })).await;
    let taken =
        fails(&w.ann, "import_issues", json!({ "team": "ENG", "issues": [{ "number": 1, "title": "x" }] })).await;
    assert!(taken.contains("ENG-1"), "{taken}");
    let twice = fails(
        &w.ann,
        "import_issues",
        json!({ "team": "ENG", "issues": [{ "number": 3, "title": "a" }, { "number": 3, "title": "b" }] }),
    )
    .await;
    assert!(twice.contains("ENG-3 appears twice"), "{twice}");
    let bad_label = fails(
        &w.ann,
        "import_issues",
        json!({ "team": "ENG", "issues": [{ "number": 2, "title": "ok" }, { "number": 3, "title": "bad", "labels": ["Nope"] }] }),
    )
    .await;
    assert!(bad_label.starts_with("ENG-3:"), "says which issue failed: {bad_label}");
    let missing = fails(&w.ann, "get_issue", json!({ "id": "ENG-2" })).await;
    assert!(missing.contains("No issue"), "the whole import was rolled back: {missing}");

    let dry = call(
        &w.ann,
        "import_issues",
        json!({ "team": "ENG", "dryRun": true, "issues": [{ "number": 2, "title": "Dry" }] }),
    )
    .await;
    assert_eq!(dry["identifiers"], json!(["ENG-2"]));
    fails(&w.ann, "get_issue", json!({ "id": "ENG-2" })).await;

    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    let forbidden = fails(&bob, "import_issues", json!({ "team": "ENG", "issues": [{ "title": "x" }] })).await;
    assert!(forbidden.contains("admins"), "{forbidden}");
}

#[sqlx::test]
async fn imports_issues_from_an_uploaded_file(db: PgPool) {
    let w = world(db).await;
    let file = json!({ "issues": [{ "number": 42, "title": "From a file", "description": "Long text" }] }).to_string();
    let upload = call(
        &w.ann,
        "upload_file",
        json!({
            "name": "issues.json",
            "contentType": "application/json",
            "base64": base64::engine::general_purpose::STANDARD.encode(file),
        }),
    )
    .await;
    let result = call(&w.ann, "import_issues", json!({ "team": "ENG", "fileId": upload["fileId"] })).await;
    assert_eq!(result["identifiers"], json!(["ENG-42"]));
    let issue = call(&w.ann, "get_issue", json!({ "id": "ENG-42" })).await;
    assert_eq!(issue["description"], "Long text");
    let both = fails(&w.ann, "import_issues", json!({ "team": "ENG", "fileId": upload["fileId"], "issues": [] })).await;
    assert!(both.contains("either issues or fileId"), "{both}");
}
