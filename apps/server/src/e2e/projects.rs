use serde_json::json;
use sqlx::PgPool;

use super::*;

fn names(list: &Value) -> Vec<String> {
    list["nodes"].as_array().unwrap().iter().map(|n| n["name"].as_str().unwrap().to_string()).collect()
}

async fn kinds(w: &World, user: Uuid) -> Vec<String> {
    sqlx::query_scalar("SELECT type FROM notifications WHERE user_id = $1 ORDER BY created_at")
        .bind(user)
        .fetch_all(&w.state.db)
        .await
        .unwrap()
}

#[sqlx::test]
async fn creates_projects_with_milestones_and_issue_progress(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    call(&w.ann, "save_project_label", json!({ "name": "Q3", "color": "#ff0000" })).await;
    let project = call(
        &w.ann,
        "save_project",
        json!({ "name": "Launch v2!", "setTeams": ["ENG"], "lead": "bob", "labels": ["Q3"], "summary": "Ship it",
            "startDate": "2030-01-01", "targetDate": "2030-03-31", "targetDateResolution": "quarter", "priority": 2 }),
    )
    .await;
    let slug = project["slugId"].as_str().unwrap();
    assert_eq!(project["url"], format!("http://localhost:3000/acme/project/launch-v2-{slug}"));
    assert_eq!(project["status"]["name"], "Backlog");
    assert_eq!(project["lead"]["email"], "bob@acme.test");
    assert_eq!(project["teams"][0]["key"], "ENG");
    assert_eq!(project["labels"][0]["name"], "Q3");
    assert_eq!(project["targetDateResolution"], "quarter");
    assert_eq!(project["priorityLabel"], "High");
    assert_eq!(project["progress"], 0);
    assert_eq!(kinds(&w, bob.user).await, vec!["projectAssigned"]);
    let found = call(&w.ann, "get_project", json!({ "id": project["url"] })).await;
    assert_eq!(found["id"], project["id"], "found by link");

    call(&w.ann, "save_milestone", json!({ "project": "Launch v2!", "name": "Beta", "targetDate": "2030-02-01" }))
        .await;
    call(&w.ann, "save_milestone", json!({ "project": "Launch v2!", "name": "GA", "targetDate": "2030-03-01" })).await;
    let dup = fails(&w.ann, "save_milestone", json!({ "project": "Launch v2!", "name": "beta" })).await;
    assert!(dup.contains("already"), "{dup}");
    call(&w.ann, "save_issue", json!({ "title": "A", "project": "Launch v2!", "milestone": "Beta", "state": "Done" }))
        .await;
    call(&w.ann, "save_issue", json!({ "title": "B", "project": "Launch v2!", "milestone": "Beta" })).await;
    call(&w.ann, "save_issue", json!({ "title": "C", "project": "Launch v2!", "state": "Canceled" })).await;
    let found = call(&w.ann, "get_project", json!({ "id": "launch v2!" })).await;
    assert_eq!(found["issueCount"], 3);
    assert_eq!(found["completedIssueCount"], 1);
    assert_eq!(found["progress"], 0.5, "canceled issues don't count");
    assert_eq!(found["issueCountByStateType"]["completed"], 1);
    assert_eq!(found["milestones"][0]["name"], "Beta");
    assert_eq!(found["milestones"][0]["progress"], 0.5);
    assert_eq!(found["milestones"][1]["issueCount"], 0);
    let milestones = call(&w.ann, "list_milestones", json!({ "project": slug })).await;
    assert_eq!(names(&milestones), vec!["Beta", "GA"]);
    let beta = call(&w.ann, "get_milestone", json!({ "id": "Beta", "project": slug })).await;
    assert_eq!(beta["issues"].as_array().unwrap().len(), 2);
    call(&w.ann, "delete_milestone", json!({ "id": beta["id"] })).await;
    let issue = call(&w.ann, "get_issue", json!({ "id": "ENG-1" })).await;
    assert!(issue["projectMilestone"].is_null() && issue["project"]["name"] == "Launch v2!");

    let no_team = fails(&w.ann, "save_project", json!({ "name": "Nowhere" })).await;
    assert!(no_team.contains("team"), "{no_team}");
    let long = fails(&w.ann, "save_project", json!({ "id": slug, "summary": "x".repeat(256) })).await;
    assert!(long.contains("255"), "{long}");
    let last = fails(&w.ann, "save_project", json!({ "id": slug, "removeTeams": ["ENG"] })).await;
    assert!(last.contains("at least one team"), "{last}");
}

#[sqlx::test]
async fn status_changes_relations_links_and_lists(db: PgPool) {
    let w = world(db).await;
    call(&w.ann, "save_project", json!({ "name": "Alpha", "addTeams": ["ENG"] })).await;
    call(&w.ann, "save_project", json!({ "name": "Beta", "addTeams": ["ENG"], "lead": "me" })).await;
    let started = call(&w.ann, "save_project", json!({ "id": "Alpha", "state": "started" })).await;
    assert_eq!(started["status"]["name"], "In Progress");
    assert!(started["startedAt"].is_string() && started["completedAt"].is_null());
    let done = call(&w.ann, "save_project", json!({ "id": "Alpha", "state": "Completed" })).await;
    assert!(done["startedAt"].is_string() && done["completedAt"].is_string());
    let reopened = call(&w.ann, "save_project", json!({ "id": "Alpha", "state": "planned" })).await;
    assert!(reopened["startedAt"].is_null() && reopened["completedAt"].is_null());

    let alpha = call(
        &w.ann,
        "save_project",
        json!({ "id": "Alpha", "blocks": ["Beta"], "links": [{ "url": "https://x.dev/spec", "title": "Spec" }] }),
    )
    .await;
    assert_eq!(alpha["relations"]["blocks"][0]["name"], "Beta");
    assert_eq!(alpha["links"][0]["label"], "Spec");
    let beta = call(&w.ann, "get_project", json!({ "id": "Beta" })).await;
    assert_eq!(beta["relations"]["blockedBy"][0]["name"], "Alpha");
    let own = fails(&w.ann, "save_project", json!({ "id": "Alpha", "relatedTo": ["Alpha"] })).await;
    assert!(own.contains("itself"), "{own}");
    let alpha = call(
        &w.ann,
        "save_project",
        json!({ "id": "Alpha", "links": [{ "url": "https://x.dev/spec", "title": "The spec" }], "removeBlocks": ["Beta"] }),
    )
    .await;
    assert_eq!(alpha["links"].as_array().unwrap().len(), 1);
    assert_eq!(alpha["links"][0]["label"], "The spec");
    assert!(alpha["relations"]["blocks"].as_array().unwrap().is_empty());
    let alpha = call(&w.ann, "save_project", json!({ "id": "Alpha", "removeLinks": ["https://x.dev/spec"] })).await;
    assert!(alpha["links"].as_array().unwrap().is_empty());

    assert_eq!(names(&call(&w.ann, "list_projects", json!({ "lead": "me" })).await), vec!["Beta"]);
    assert_eq!(names(&call(&w.ann, "list_projects", json!({ "status": "planned" })).await), vec!["Alpha"]);
    assert_eq!(names(&call(&w.ann, "list_projects", json!({ "query": "alp" })).await), vec!["Alpha"]);
    let listed = call(&w.ann, "list_projects", json!({ "orderBy": "manual" })).await;
    assert_eq!(names(&listed), vec!["Alpha", "Beta"]);
    assert!(listed["nodes"][0].get("description").is_none(), "descriptions are left out of lists");
    sqlx::query(
        "INSERT INTO custom_views (workspace_id, owner_id, slug_id, name, model, filter)
         VALUES ($1, $2, 'aaaabbbbcccc', 'Mine', 'project', '{\"lead\": {\"isMe\": {\"eq\": true}}, \"status\": {\"type\": {\"in\": [\"backlog\"]}}}')",
    )
    .bind(w.workspace)
    .bind(w.ann.user)
    .execute(&w.state.db)
    .await
    .unwrap();
    assert_eq!(names(&call(&w.ann, "list_projects", json!({ "customView": "Mine" })).await), vec!["Beta"]);
    let filtered = call(
        &w.ann,
        "list_projects",
        json!({ "filter": { "or": [{ "name": "alpha" }, { "priority": { "gt": 2 } }] } }),
    )
    .await;
    assert_eq!(names(&filtered), vec!["Alpha"]);
    let bad = fails(&w.ann, "list_projects", json!({ "filter": { "colour": "red" } })).await;
    assert!(bad.contains("colour"), "{bad}");
}

#[sqlx::test]
async fn status_updates_set_health_and_notify(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    let cat = w.member("cat@acme.test", "Cat Lee", "member").await;
    call(&w.ann, "save_project", json!({ "name": "Apollo", "setTeams": ["ENG"], "addMembers": ["bob"] })).await;
    let first = call(
        &w.ann,
        "save_status_update",
        json!({ "project": "Apollo", "body": "Slipping, cc @cat", "health": "atRisk" }),
    )
    .await;
    assert_eq!(first["type"], "project");
    assert_eq!(first["user"]["email"], "ann@acme.test");
    let project = call(&w.ann, "get_project", json!({ "id": "Apollo" })).await;
    assert_eq!(project["health"], "atRisk");
    assert_eq!(project["latestUpdate"]["id"], first["id"]);
    assert_eq!(kinds(&w, bob.user).await, vec!["projectUpdateCreated"]);
    assert_eq!(kinds(&w, cat.user).await, vec!["projectUpdateMention"]);

    let second = call(&w.ann, "save_status_update", json!({ "project": "Apollo", "body": "Still slipping" })).await;
    assert_eq!(second["health"], "atRisk", "the previous update's health");
    let third = call(
        &w.ann,
        "save_status_update",
        json!({ "project": "Apollo", "body": "Back on track", "health": "onTrack" }),
    )
    .await;
    let listed = call(&bob, "get_status_updates", json!({ "project": "Apollo" })).await;
    let ids: Vec<&Value> = listed["nodes"].as_array().unwrap().iter().map(|n| &n["id"]).collect();
    assert_eq!(ids, vec![&third["id"], &second["id"], &first["id"]], "newest first");
    let not_author = fails(&bob, "save_status_update", json!({ "id": third["id"], "body": "Mine now" })).await;
    assert!(not_author.contains("author"), "{not_author}");
    let edited = call(&w.ann, "save_status_update", json!({ "id": third["id"], "health": "offTrack" })).await;
    assert!(edited["editedAt"].is_string());
    assert_eq!(call(&w.ann, "get_project", json!({ "id": "Apollo" })).await["health"], "offTrack");
    fails(&bob, "delete_status_update", json!({ "id": third["id"] })).await;
    call(&w.ann, "delete_status_update", json!({ "id": third["id"] })).await;
    let project = call(&w.ann, "list_projects", json!({ "health": "atRisk" })).await;
    assert_eq!(names(&project), vec!["Apollo"], "health goes back to the newest remaining update's");
}

#[sqlx::test]
async fn initiatives_group_projects_with_progress(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    call(&w.ann, "save_project", json!({ "name": "One", "setTeams": ["ENG"] })).await;
    call(&w.ann, "save_project", json!({ "name": "Two", "setTeams": ["ENG"] })).await;
    call(&w.ann, "save_issue", json!({ "title": "Done", "project": "One", "state": "Done" })).await;
    call(&w.ann, "save_issue", json!({ "title": "Open", "project": "Two" })).await;
    let goal = call(
        &w.ann,
        "save_initiative",
        json!({ "name": "Grow 2030", "owner": "bob", "setProjects": ["One", "Two"], "targetDate": "2030-12-31",
            "links": [{ "url": "https://x.dev/okr", "title": "OKRs" }] }),
    )
    .await;
    let slug = goal["slugId"].as_str().unwrap();
    assert_eq!(goal["url"], format!("http://localhost:3000/acme/initiative/grow-2030-{slug}"));
    assert_eq!(goal["projectCount"], 2);
    assert_eq!(goal["progress"], 0.5, "the average of 1.0 and 0.0");
    assert_eq!(goal["status"], "planned");
    let active = call(&w.ann, "save_initiative", json!({ "id": slug, "status": "active" })).await;
    assert!(active["startedAt"].is_string());
    call(&w.ann, "save_initiative", json!({ "name": "Grow EU", "parent": "Grow 2030" })).await;
    let looped = fails(&w.ann, "save_initiative", json!({ "id": "Grow 2030", "parent": "Grow EU" })).await;
    assert!(looped.contains("itself"), "{looped}");
    call(&w.ann, "save_status_update", json!({ "initiative": "Grow 2030", "body": "Going well" })).await;
    assert_eq!(kinds(&w, bob.user).await, vec!["initiativeUpdateCreated"]);

    let got = call(&w.ann, "get_initiative", json!({ "id": "grow 2030" })).await;
    assert_eq!(got["projects"].as_array().unwrap().len(), 2);
    assert_eq!(got["projects"][0]["progress"], 1);
    assert_eq!(got["subInitiatives"][0]["name"], "Grow EU");
    assert_eq!(got["links"][0]["label"], "OKRs");
    assert_eq!(got["health"], "onTrack");
    assert_eq!(got["latestUpdate"]["body"], "Going well");
    let project = call(&w.ann, "get_project", json!({ "id": "One" })).await;
    assert_eq!(project["initiatives"][0]["name"], "Grow 2030");
    assert_eq!(names(&call(&w.ann, "list_initiatives", json!({ "status": "active" })).await), vec!["Grow 2030"]);
    assert_eq!(names(&call(&w.ann, "list_initiatives", json!({ "parent": "Grow 2030" })).await), vec!["Grow EU"]);
    assert_eq!(names(&call(&w.ann, "list_projects", json!({ "initiative": slug })).await).len(), 2);
    call(&w.ann, "delete_initiative", json!({ "id": "Grow EU" })).await;
    assert_eq!(names(&call(&w.ann, "list_initiatives", json!({})).await), vec!["Grow 2030"]);
    call(&w.ann, "restore_initiative", json!({ "id": "Grow EU" })).await;
    assert_eq!(names(&call(&w.ann, "list_initiatives", json!({})).await).len(), 2);
}

#[sqlx::test]
async fn project_templates_create_milestones_and_issues(db: PgPool) {
    let w = world(db).await;
    sqlx::query("INSERT INTO templates (workspace_id, type, name, data) VALUES ($1, 'project', 'Launch', $2)")
        .bind(w.workspace)
        .bind(json!({
            "summary": "A launch",
            "description": "## Plan",
            "state": "planned",
            "teams": ["ENG"],
            "milestones": [{ "name": "Alpha" }, { "name": "Release", "targetDate": "2030-06-01" }],
            "issues": [
                { "title": "Write the plan", "milestone": "alpha", "priority": 2 },
                { "title": "Announce", "milestone": "Release" },
                { "title": "Loose end" }
            ]
        }))
        .execute(&w.state.db)
        .await
        .unwrap();
    let project = call(&w.ann, "save_project", json!({ "name": "Mobile launch", "template": "Launch" })).await;
    assert_eq!(project["summary"], "A launch");
    assert_eq!(project["description"], "## Plan");
    assert_eq!(project["status"]["type"], "planned");
    assert_eq!(project["teams"][0]["key"], "ENG");
    assert_eq!(project["milestones"][0]["name"], "Alpha");
    assert_eq!(project["milestones"][0]["issueCount"], 1);
    assert_eq!(project["milestones"][1]["targetDate"], "2030-06-01");
    assert_eq!(project["issueCount"], 3);
    let plan = call(&w.ann, "get_issue", json!({ "id": "ENG-1" })).await;
    assert_eq!(plan["projectMilestone"]["name"], "Alpha");
    assert_eq!(plan["priority"], 2);
    let wins =
        call(&w.ann, "save_project", json!({ "name": "Web launch", "template": "Launch", "state": "started" })).await;
    assert_eq!(wins["status"]["type"], "started", "arguments win over the template");
}

#[sqlx::test]
async fn visibility_trash_and_archive(db: PgPool) {
    let w = world(db).await;
    let guest = w.member("gus@else.test", "Gus", "guest").await;
    call(&w.ann, "save_project", json!({ "name": "Secret", "setTeams": ["ENG"] })).await;
    assert!(names(&call(&guest, "list_projects", json!({})).await).is_empty());
    assert!(fails(&guest, "get_project", json!({ "id": "Secret" })).await.contains("No project"));
    let eng: Uuid = sqlx::query_scalar("SELECT id FROM teams WHERE key = 'ENG'").fetch_one(&w.state.db).await.unwrap();
    sqlx::query("INSERT INTO team_members (team_id, user_id) VALUES ($1, $2)")
        .bind(eng)
        .bind(guest.user)
        .execute(&w.state.db)
        .await
        .unwrap();
    let guest = ctx(&w.state, guest.user, w.workspace).await;
    assert_eq!(call(&guest, "get_project", json!({ "id": "Secret" })).await["name"], "Secret");

    let trashed = call(&w.ann, "delete_project", json!({ "id": "Secret" })).await;
    assert_eq!(trashed["trashed"], true);
    assert!(names(&call(&w.ann, "list_projects", json!({})).await).is_empty());
    assert_eq!(names(&call(&w.ann, "list_projects", json!({ "includeTrashed": true })).await), vec!["Secret"]);
    fails(&w.ann, "get_project", json!({ "id": "Secret" })).await;
    call(&w.ann, "restore_project", json!({ "id": "Secret" })).await;
    call(&w.ann, "archive_project", json!({ "id": "Secret" })).await;
    assert!(names(&call(&w.ann, "list_projects", json!({})).await).is_empty());
    assert_eq!(names(&call(&w.ann, "list_projects", json!({ "includeArchived": true })).await), vec!["Secret"]);
    call(&w.ann, "unarchive_project", json!({ "id": "Secret" })).await;
    assert_eq!(names(&call(&w.ann, "list_projects", json!({})).await), vec!["Secret"]);
}

#[sqlx::test]
async fn project_statuses_and_labels(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    call(&w.ann, "save_project", json!({ "name": "P", "setTeams": ["ENG"], "state": "Paused" })).await;
    let statuses = call(&w.ann, "list_project_statuses", json!({})).await;
    let paused = statuses["nodes"].as_array().unwrap().iter().find(|s| s["name"] == "Paused").unwrap();
    assert_eq!(paused["projectCount"], 1);
    assert!(fails(&bob, "save_project_status", json!({ "name": "Review", "type": "started" })).await.contains("admin"));
    let review = call(&w.ann, "save_project_status", json!({ "name": "Review", "type": "started" })).await;
    assert_eq!(review["position"], 6);
    let in_use = fails(&w.ann, "delete_project_status", json!({ "id": "Paused" })).await;
    assert!(in_use.contains("moveProjectsTo"), "{in_use}");
    call(&w.ann, "delete_project_status", json!({ "id": "Paused", "moveProjectsTo": "Review" })).await;
    let moved = call(&w.ann, "get_project", json!({ "id": "P" })).await;
    assert_eq!(moved["status"]["name"], "Review");
    assert!(moved["startedAt"].is_string());

    call(&bob, "save_project_label", json!({ "name": "Area", "isGroup": true })).await;
    call(&bob, "save_project_label", json!({ "name": "Mobile", "parent": "Area" })).await;
    call(&bob, "save_project", json!({ "id": "P", "addLabels": ["Area/Mobile"] })).await;
    let labels = call(&bob, "list_project_labels", json!({})).await;
    assert_eq!(names(&labels), vec!["Area", "Mobile"]);
    assert_eq!(labels["nodes"][1]["projectCount"], 1);
    let group = fails(&bob, "save_project", json!({ "id": "P", "addLabels": ["Area"] })).await;
    assert!(group.contains("No project label"), "{group}");
    call(&bob, "retire_project_label", json!({ "id": "Area" })).await;
    assert!(names(&call(&bob, "list_project_labels", json!({})).await).is_empty());
    assert_eq!(names(&call(&bob, "list_project_labels", json!({ "includeRetired": true })).await).len(), 2);
    call(&bob, "delete_project_label", json!({ "id": "Mobile" })).await;
    assert!(call(&bob, "get_project", json!({ "id": "P" })).await["labels"].as_array().unwrap().is_empty());
}
