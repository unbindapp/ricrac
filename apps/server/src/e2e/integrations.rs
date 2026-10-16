use std::sync::{Arc, Mutex};

use axum::body::Bytes;
use axum::http::{HeaderMap, StatusCode};
use hmac::{Hmac, Mac};
use serde_json::json;
use sha2::Sha256;
use sqlx::PgPool;

use super::*;
use crate::ops::webhooks::deliver_due;

type Requests = Arc<Mutex<Vec<(HeaderMap, Vec<u8>)>>>;

/// A local HTTP server answering every POST to /hook with `status`, recording what it got.
async fn receiver(status: StatusCode) -> (String, Requests) {
    let requests: Requests = Arc::default();
    let seen = requests.clone();
    let app = axum::Router::new().route(
        "/hook",
        axum::routing::post(move |headers: HeaderMap, body: Bytes| {
            let seen = seen.clone();
            async move {
                seen.lock().unwrap().push((headers, body.to_vec()));
                status
            }
        }),
    );
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let url = format!("http://127.0.0.1:{}/hook", listener.local_addr().unwrap().port());
    tokio::spawn(async move { axum::serve(listener, app).await.unwrap() });
    (url, requests)
}

fn verifies(secret: &str, body: &[u8], signature: &str) -> bool {
    let mut mac = Hmac::<Sha256>::new_from_slice(secret.as_bytes()).unwrap();
    mac.update(body);
    mac.verify_slice(&hex::decode(signature).unwrap()).is_ok()
}

#[sqlx::test]
async fn delivers_signed_webhooks(db: PgPool) {
    let w = world(db).await;
    let (url, requests) = receiver(StatusCode::OK).await;
    let hook = call(&w.ann, "save_webhook", json!({ "url": url, "label": "CI", "resourceTypes": ["issue"] })).await;
    let secret = hook["secret"].as_str().unwrap().to_string();
    assert!(secret.starts_with("rr_whs_"));
    assert_eq!(hook["resourceTypes"], json!(["Issue"]));
    let id = hook["id"].as_str().unwrap().to_string();
    assert!(call(&w.ann, "get_webhook", json!({ "id": id })).await.get("secret").is_none());

    call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "Ship it" })).await;
    assert_eq!(deliver_due(&w.state).await.unwrap(), 1);
    let (headers, body) = requests.lock().unwrap().pop().unwrap();
    let payload: Value = serde_json::from_slice(&body).unwrap();
    assert_eq!(payload["action"], "create");
    assert_eq!(payload["type"], "Issue");
    assert_eq!(payload["data"]["identifier"], "ENG-1");
    assert!(payload["webhookTimestamp"].is_i64());
    assert_eq!(headers["ricrac-event"], "Issue");
    assert_eq!(headers["user-agent"], "Ricrac-Webhook/1.0");
    assert!(verifies(&secret, &body, headers["ricrac-signature"].to_str().unwrap()));
    assert!(!verifies("rr_whs_wrong", &body, headers["ricrac-signature"].to_str().unwrap()));

    let read = call(&w.ann, "get_webhook", json!({ "id": id })).await;
    let delivery = &read["deliveries"][0];
    assert_eq!(delivery["action"], "create");
    assert_eq!(delivery["statusCode"], 200);
    assert_eq!(delivery["attempts"], 1);
    assert!(delivery["deliveredAt"].is_string());
    assert_eq!(headers["ricrac-delivery"], delivery["id"].as_str().unwrap());
    assert_eq!(deliver_due(&w.state).await.unwrap(), 0, "nothing is left to send");

    let rotated = call(&w.ann, "rotate_webhook_secret", json!({ "id": id })).await;
    let new_secret = rotated["secret"].as_str().unwrap();
    assert_ne!(new_secret, secret);
    let ping = call(&w.ann, "test_webhook", json!({ "id": id })).await;
    assert_eq!(ping["delivered"], true);
    assert_eq!(ping["statusCode"], 200);
    let (headers, body) = requests.lock().unwrap().pop().unwrap();
    let payload: Value = serde_json::from_slice(&body).unwrap();
    assert_eq!((payload["action"].as_str(), payload["type"].as_str()), (Some("test"), Some("Ping")));
    assert!(verifies(new_secret, &body, headers["ricrac-signature"].to_str().unwrap()));

    let comments_only = call(&w.ann, "save_webhook", json!({ "id": id, "resourceTypes": ["Comment"] })).await;
    assert_eq!(comments_only["resourceTypes"], json!(["Comment"]));
    call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "Unheard" })).await;
    assert_eq!(deliver_due(&w.state).await.unwrap(), 0, "the webhook no longer listens to issues");
}

#[sqlx::test]
async fn retries_failed_deliveries(db: PgPool) {
    let w = world(db).await;
    let (url, requests) = receiver(StatusCode::INTERNAL_SERVER_ERROR).await;
    let hook = call(&w.ann, "save_webhook", json!({ "url": url, "resourceTypes": ["Issue"], "team": "ENG" })).await;
    assert_eq!(hook["team"]["key"], "ENG");
    let id = hook["id"].as_str().unwrap().to_string();
    call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "Flaky" })).await;
    assert_eq!(deliver_due(&w.state).await.unwrap(), 1);
    assert_eq!(requests.lock().unwrap().len(), 1);
    let deliveries = call(&w.ann, "list_webhook_deliveries", json!({ "id": id })).await;
    let delivery = &deliveries["nodes"][0];
    assert_eq!(delivery["attempts"], 1);
    assert_eq!(delivery["statusCode"], 500);
    assert!(delivery["error"].as_str().unwrap().contains("500"));
    assert!(delivery["nextAttemptAt"].is_string() && delivery["failedAt"].is_null());
    assert_eq!(deliver_due(&w.state).await.unwrap(), 0, "the retry waits a minute");

    let ping = call(&w.ann, "test_webhook", json!({ "id": id })).await;
    assert_eq!((ping["delivered"].as_bool(), ping["statusCode"].as_i64()), (Some(false), Some(500)));

    sqlx::query("UPDATE webhook_deliveries SET next_attempt_at = now()").execute(&w.state.db).await.unwrap();
    call(&w.ann, "save_webhook", json!({ "id": id, "enabled": false })).await;
    assert_eq!(deliver_due(&w.state).await.unwrap(), 0);
    let deliveries = call(&w.ann, "list_webhook_deliveries", json!({ "id": id })).await;
    assert!(deliveries["nodes"].as_array().unwrap().iter().all(|d| d["failedAt"].is_string()));
    assert_eq!(requests.lock().unwrap().len(), 2, "only the first attempt and the ping were sent");
}

#[sqlx::test]
async fn only_admins_manage_webhooks(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    assert!(fails(&bob, "list_webhooks", json!({})).await.contains("admin"));
    let http = fails(&w.ann, "save_webhook", json!({ "url": "http://hooks.example.com/x" })).await;
    assert!(http.contains("https"), "{http}");
    let kind =
        fails(&w.ann, "save_webhook", json!({ "url": "https://hooks.example.com", "resourceTypes": ["Bug"] })).await;
    assert!(kind.contains("CustomerNeed"), "lists the types: {kind}");
    let hook = call(&w.ann, "save_webhook", json!({ "url": "https://hooks.example.com" })).await;
    assert_eq!(hook["resourceTypes"], json!(["Issue", "Comment"]));
    assert_eq!(call(&w.ann, "list_webhooks", json!({})).await["nodes"].as_array().unwrap().len(), 1);
    call(&w.ann, "delete_webhook", json!({ "id": hook["id"] })).await;
    assert!(call(&w.ann, "list_webhooks", json!({})).await["nodes"].as_array().unwrap().is_empty());
}

#[sqlx::test]
async fn customers_with_needs_statuses_and_tiers(db: PgPool) {
    let w = world(db).await;
    let statuses = call(&w.ann, "list_customer_statuses", json!({})).await;
    assert_eq!(statuses["nodes"][0]["name"], "Active");
    call(&w.ann, "save_customer_tier", json!({ "name": "Enterprise", "color": "#5E6AD2" })).await;
    let globex = call(
        &w.ann,
        "save_customer",
        json!({ "name": "Globex", "domains": ["https://Globex.com/"], "status": "active", "tier": "enterprise",
            "owner": "me", "revenue": 50000, "size": 200, "externalIds": ["crm-42"] }),
    )
    .await;
    assert_eq!(globex["domains"], json!(["globex.com"]));
    assert_eq!(globex["status"]["name"], "Active");
    assert_eq!(globex["tier"]["color"], "#5e6ad2");
    assert_eq!(globex["owner"]["email"], "ann@acme.test");
    let taken = fails(&w.ann, "save_customer", json!({ "name": "Initech", "domains": ["globex.com"] })).await;
    assert!(taken.contains("Globex"), "{taken}");

    call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "SSO" })).await;
    let need = call(
        &w.ann,
        "save_customer_need",
        json!({ "customer": "globex.com", "issue": "ENG-1", "body": "We need SSO", "priority": 1 }),
    )
    .await;
    assert_eq!(need["customer"]["name"], "Globex");
    assert_eq!(need["issue"]["identifier"], "ENG-1");
    let neither = fails(&w.ann, "save_customer_need", json!({ "customer": "Globex", "body": "?" })).await;
    assert!(neither.contains("issue or the project"), "{neither}");
    assert!(
        fails(&w.ann, "save_customer_need", json!({ "customer": "Globex", "issue": "ENG-9" })).await.contains("ENG-9")
    );

    let read = call(&w.ann, "get_customer", json!({ "id": "crm-42" })).await;
    assert_eq!(read["needCount"], 1);
    assert_eq!(read["needs"][0]["body"], "We need SSO");
    assert_eq!(read["needs"][0]["issue"]["state"]["name"], "Backlog");
    let by_issue = call(&w.ann, "list_customer_needs", json!({ "issue": "ENG-1" })).await;
    assert_eq!(by_issue["nodes"][0]["priority"], 1);
    let found = call(&w.ann, "list_customers", json!({ "query": "globex.c", "tier": "Enterprise" })).await;
    assert_eq!(found["nodes"].as_array().unwrap().len(), 1);
    assert!(
        call(&w.ann, "list_customers", json!({ "status": "Churned" })).await["nodes"].as_array().unwrap().is_empty()
    );

    let cleared = call(&w.ann, "delete_customer_tier", json!({ "id": "Enterprise" })).await;
    assert_eq!(cleared["customersCleared"], 1);
    let updated = call(&w.ann, "save_customer", json!({ "id": "Globex", "status": null, "revenue": null })).await;
    assert!(updated["tier"].is_null() && updated["status"].is_null() && updated["revenue"].is_null());
    assert_eq!(updated["size"], 200);

    let archived = call(&w.ann, "delete_customer", json!({ "id": "Globex" })).await;
    assert!(archived["archivedAt"].is_string());
    assert!(call(&w.ann, "list_customers", json!({})).await["nodes"].as_array().unwrap().is_empty());
    assert_eq!(call(&w.ann, "list_customers", json!({ "includeArchived": true })).await["nodes"][0]["name"], "Globex");
    assert_eq!(
        call(&w.ann, "list_customer_needs", json!({ "issue": "ENG-1" })).await["nodes"].as_array().unwrap().len(),
        1
    );
    call(&w.ann, "delete_customer_need", json!({ "id": need["id"] })).await;
    assert!(call(&w.ann, "list_customer_needs", json!({})).await["nodes"].as_array().unwrap().is_empty());
}

fn names(list: &Value) -> Vec<String> {
    list["nodes"].as_array().unwrap().iter().map(|n| n["name"].as_str().unwrap().to_string()).collect()
}

#[sqlx::test]
async fn releases_move_through_stages_with_issues_and_notes(db: PgPool) {
    let w = world(db).await;
    let pipeline = call(
        &w.ann,
        "save_release_pipeline",
        json!({ "name": "Web", "type": "scheduled", "isProduction": true, "addTeams": ["ENG"] }),
    )
    .await;
    let stages: Vec<&str> =
        pipeline["stages"].as_array().unwrap().iter().map(|s| s["name"].as_str().unwrap()).collect();
    assert_eq!(stages, ["Planned", "In Progress", "Released", "Canceled"]);
    assert_eq!(pipeline["teams"][0]["key"], "ENG");
    let listed = call(&w.ann, "list_release_pipelines", json!({ "team": "ENG", "includeStages": true })).await;
    assert_eq!(listed["nodes"][0]["stages"].as_array().unwrap().len(), 4);

    let spring = call(
        &w.ann,
        "save_release",
        json!({ "name": "Spring", "pipeline": "Web", "version": "1.0", "createdAt": "2026-01-01T00:00:00Z" }),
    )
    .await;
    assert_eq!(spring["stage"]["name"], "Planned");
    assert!(spring["startedAt"].is_null());
    let started = call(&w.ann, "save_release", json!({ "id": "Spring", "stage": "started" })).await;
    assert_eq!(started["stage"]["name"], "In Progress");
    assert!(started["startedAt"].is_string() && started["completedAt"].is_null());
    let released = call(&w.ann, "save_release", json!({ "id": spring["slugId"], "stage": "Released" })).await;
    assert!(released["completedAt"].is_string());
    assert_eq!(released["startedAt"], started["startedAt"]);
    let dated = call(
        &w.ann,
        "save_release",
        json!({ "id": "Spring", "completedAt": "2026-01-05T10:00:00Z", "targetDate": "2026-01-04" }),
    )
    .await;
    assert!(dated["completedAt"].as_str().unwrap().starts_with("2026-01-05T10:00:00"));
    assert_eq!(dated["targetDate"], "2026-01-04");

    call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "Login", "addReleases": ["Spring"] })).await;
    call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "Not shipped" })).await;
    assert_eq!(identifiers(&call(&w.ann, "list_issues", json!({ "release": "Spring" })).await), ["ENG-1"]);
    let read = call(&w.ann, "get_release", json!({ "id": "Spring" })).await;
    assert_eq!(read["issueCount"], 1);
    assert_eq!(read["issues"][0]["identifier"], "ENG-1");
    assert_eq!(read["issues"][0]["state"]["name"], "Backlog");

    for (name, version, month) in [("Summer", "1.1", 2), ("Autumn", "1.2", 3), ("Winter", "1.3", 4)] {
        let created_at = format!("2026-0{month}-01T00:00:00Z");
        call(
            &w.ann,
            "save_release",
            json!({ "name": name, "pipeline": "Web", "version": version, "createdAt": created_at }),
        )
        .await;
    }
    let note = call(
        &w.ann,
        "save_release_note",
        json!({ "pipeline": "Web", "rangeFromRelease": "Autumn", "rangeToRelease": "Summer", "content": "Faster pages" }),
    )
    .await;
    assert_eq!(note["title"], "Autumn (1.2)");
    let covered: Vec<&str> = note["releases"].as_array().unwrap().iter().map(|r| r["name"].as_str().unwrap()).collect();
    assert_eq!(covered, ["Summer", "Autumn"]);
    let patched = call(
        &w.ann,
        "save_release_note",
        json!({ "id": note["slugId"], "patch": [{ "op": "append", "text": "\nAnd fixes" }] }),
    )
    .await;
    assert_eq!(patched["content"], "Faster pages\nAnd fixes");
    assert_eq!(
        call(&w.ann, "get_release", json!({ "id": "Autumn" })).await["releaseNotes"][0]["title"],
        "Autumn (1.2)"
    );
    assert_eq!(
        call(&w.ann, "list_release_notes", json!({ "pipeline": "Web" })).await["nodes"][0]["title"],
        "Autumn (1.2)"
    );

    assert_eq!(
        names(&call(&w.ann, "list_releases", json!({ "pipeline": "Web", "stage": "Released" })).await),
        ["Spring"]
    );
    assert_eq!(
        names(&call(&w.ann, "list_releases", json!({ "stage": "planned" })).await),
        ["Winter", "Autumn", "Summer"]
    );
    assert!(fails(&w.ann, "list_releases", json!({ "stage": "Released" })).await.contains("pipeline"));

    call(&w.ann, "save_release_stage", json!({ "pipeline": "Web", "name": "QA", "type": "started" })).await;
    let planned = pipeline["stages"][0]["id"].as_str().unwrap();
    let busy = fails(&w.ann, "delete_release_stage", json!({ "id": planned })).await;
    assert!(busy.contains("moveReleasesTo"), "{busy}");
    let deleted = call(&w.ann, "delete_release_stage", json!({ "id": planned, "moveReleasesTo": "QA" })).await;
    assert_eq!(deleted["releasesMoved"], 3);
    assert_eq!(call(&w.ann, "get_release", json!({ "id": "Winter" })).await["stage"]["name"], "QA");

    call(&w.ann, "delete_release", json!({ "id": "Spring" })).await;
    assert_eq!(
        names(&call(&w.ann, "list_releases", json!({ "pipeline": "Web" })).await),
        ["Winter", "Autumn", "Summer"]
    );
}
