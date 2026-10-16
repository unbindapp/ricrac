use serde_json::json;
use sqlx::PgPool;

use super::*;

/// A new context, since one caches the teams it sees.
async fn fresh(c: &Ctx) -> Ctx {
    ctx(&c.state, c.user, c.workspace).await
}

fn names(list: &Value, field: &str) -> Vec<String> {
    list["nodes"].as_array().unwrap().iter().map(|n| n[field].as_str().unwrap().to_string()).collect()
}

#[sqlx::test]
async fn creates_teams_with_settings(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    let gus = w.member("gus@else.test", "Gus", "guest").await;

    let team =
        call(&bob, "save_team", json!({ "name": "Mobile Platform", "description": "Apps", "color": "#ABC" })).await;
    assert_eq!(team["key"], "MP");
    assert_eq!(team["description"], "Apps");
    assert_eq!(team["color"], "#abc");
    assert_eq!(team["members"][0]["email"], "bob@acme.test");
    assert_eq!(team["members"][0]["owner"], true);
    assert_eq!(team["defaultIssueState"]["name"], "Backlog");
    assert_eq!(team["autoCloseState"]["name"], "Canceled");
    let states: Vec<&str> = team["states"].as_array().unwrap().iter().map(|s| s["name"].as_str().unwrap()).collect();
    assert_eq!(states, vec!["Backlog", "Todo", "In Progress", "In Review", "Done", "Canceled", "Duplicate"]);
    assert_eq!(team["url"], "http://localhost:3000/acme/team/MP");

    assert!(fails(&gus, "save_team", json!({ "name": "Guests" })).await.contains("Guests can't"));
    let taken = fails(&bob, "save_team", json!({ "name": "Other", "key": "eng" })).await;
    assert!(taken.contains("ENG is taken by the team Engineering"), "{taken}");
    assert!(fails(&bob, "save_team", json!({ "name": "Bad", "key": "1AB" })).await.contains("team key"));
    assert!(fails(&bob, "save_team", json!({ "name": "engineering", "key": "EN2" })).await.contains("already"));
    let bob = fresh(&bob).await;

    let updated = call(
        &bob,
        "save_team",
        json!({
            "id": "MP", "estimateType": "fibonacci", "defaultEstimate": 3, "cyclesEnabled": true,
            "cycleDurationWeeks": 1, "cycleStartDay": 0, "autoArchiveMonths": null, "autoCloseMonths": 3,
            "defaultIssueState": "Todo", "timezone": "America/New_York", "parent": "ENG", "icon": "Phone"
        }),
    )
    .await;
    assert_eq!(updated["estimateType"], "fibonacci");
    assert_eq!(updated["allowedEstimates"], json!([1.0, 2.0, 3.0, 5.0, 8.0]));
    assert_eq!(updated["defaultEstimate"], 3);
    assert_eq!(updated["cyclesEnabled"], true);
    assert_eq!(updated["cycleDurationWeeks"], 1);
    assert!(updated["autoArchiveMonths"].is_null());
    assert_eq!(updated["autoCloseMonths"], 3);
    assert_eq!(updated["defaultIssueState"]["name"], "Todo");
    assert_eq!(updated["parent"]["key"], "ENG");
    assert_eq!(updated["description"], "Apps", "omitted fields stay");
    assert_eq!(call(&bob, "get_team", json!({ "id": "ENG" })).await["subTeams"][0]["key"], "MP");
    assert_eq!(call(&bob, "save_issue", json!({ "team": "MP", "title": "x" })).await["state"]["name"], "Todo");

    for (args, error) in [
        (json!({ "id": "MP", "cycleDurationWeeks": 9 }), "cycleDurationWeeks is 1 to 8"),
        (json!({ "id": "MP", "defaultEstimate": 4 }), "isn't on the fibonacci scale"),
        (json!({ "id": "MP", "autoCloseMonths": 2 }), "1, 3, 6, 9 or 12"),
        (json!({ "id": "MP", "estimateType": "huge" }), "estimateType"),
        (json!({ "id": "MP", "defaultIssueState": "Done" }), "backlog or unstarted"),
        (json!({ "id": "MP", "autoCloseState": "Done" }), "canceled"),
        (json!({ "id": "ENG", "parent": "MP" }), "own sub-teams"),
        (json!({ "id": "MP", "timezone": "Nowhere" }), "timezone"),
    ] {
        let message = fails(&bob, "save_team", args).await;
        assert!(message.contains(error), "{message}");
    }
    let listed = call(&bob, "list_teams", json!({ "mine": true })).await;
    assert_eq!(names(&listed, "key"), vec!["MP"]);
    assert_eq!(listed["nodes"][0]["isMember"], true);
    assert_eq!(listed["nodes"][0]["issueCount"], 1);
    assert_eq!(names(&call(&bob, "list_teams", json!({ "query": "eng" })).await, "key"), vec!["ENG"]);
}

#[sqlx::test]
async fn changing_the_key_keeps_old_identifiers_working(db: PgPool) {
    let w = world(db).await;
    call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "First" })).await;
    call(&w.ann, "save_issue", json!({ "team": "ENG", "title": "Second" })).await;
    let team = call(&w.ann, "save_team", json!({ "id": "ENG", "key": "core", "name": "Core" })).await;
    assert_eq!(team["key"], "CORE");
    assert_eq!(team["name"], "Core");
    let issue = call(&w.ann, "get_issue", json!({ "id": "ENG-2" })).await;
    assert_eq!(issue["identifier"], "CORE-2");
    assert_eq!(issue["previousIdentifiers"], json!(["ENG-2"]));
    call(&w.ann, "save_team", json!({ "id": "CORE", "key": "CO" })).await;
    let issue = call(&w.ann, "get_issue", json!({ "id": "ENG-1" })).await;
    assert_eq!(issue["identifier"], "CO-1");
    assert_eq!(issue["previousIdentifiers"], json!(["ENG-1", "CORE-1"]));
    assert_eq!(call(&w.ann, "save_issue", json!({ "title": "Third" })).await["identifier"], "CO-3");
}

#[sqlx::test]
async fn private_teams_are_hidden_from_others(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    let ada = w.member("ada@acme.test", "Ada Admin", "admin").await;
    call(&w.ann, "save_team", json!({ "name": "Secret", "key": "SEC", "private": true })).await;
    let ann = fresh(&w.ann).await;
    call(&ann, "save_issue", json!({ "team": "SEC", "title": "Hidden" })).await;

    assert_eq!(names(&call(&bob, "list_teams", json!({})).await, "key"), vec!["ENG"]);
    assert!(fails(&bob, "get_team", json!({ "id": "SEC" })).await.contains("No team"));
    assert!(fails(&bob, "get_issue", json!({ "id": "SEC-1" })).await.contains("No issue"));
    assert!(fails(&bob, "join_team", json!({ "id": "SEC" })).await.contains("No team"));
    assert!(identifiers(&call(&bob, "list_issues", json!({})).await).is_empty());
    assert_eq!(call(&bob, "get_workspace", json!({})).await["teams"].as_array().unwrap().len(), 1);

    let joined = call(&ada, "join_team", json!({ "id": "SEC" })).await;
    assert_eq!(joined["isMember"], true);
    call(&ann, "save_team", json!({ "id": "SEC", "addMembers": ["bob"] })).await;
    let bob = fresh(&bob).await;
    assert_eq!(call(&bob, "get_issue", json!({ "id": "SEC-1" })).await["title"], "Hidden");
    call(&bob, "save_team", json!({ "id": "SEC", "removeMembers": ["ada", "me"] })).await;
    let alone = fails(&ann, "leave_team", json!({ "id": "SEC" })).await;
    assert!(alone.contains("last member"), "{alone}");
    let emptied = fails(&ann, "save_team", json!({ "id": "SEC", "removeMembers": ["ann"] })).await;
    assert!(emptied.contains("at least one member"), "{emptied}");

    let left = call(&ann, "leave_team", json!({ "id": "ENG" })).await;
    assert_eq!(left["left"]["key"], "ENG");
    assert!(fails(&ann, "leave_team", json!({ "id": "ENG" })).await.contains("aren't a member"));
}

#[sqlx::test]
async fn archiving_a_team_keeps_its_issues_readable(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    call(&w.ann, "save_issue", json!({ "title": "Old work" })).await;
    let denied = fails(&bob, "delete_team", json!({ "id": "ENG" })).await;
    assert!(denied.contains("owners"), "{denied}");
    assert_eq!(call(&w.ann, "delete_team", json!({ "id": "ENG" })).await["key"], "ENG");
    let ann = fresh(&w.ann).await;
    assert!(names(&call(&ann, "list_teams", json!({})).await, "key").is_empty());
    assert_eq!(names(&call(&ann, "list_teams", json!({ "includeArchived": true })).await, "key"), vec!["ENG"]);
    assert_eq!(call(&ann, "get_issue", json!({ "id": "ENG-1" })).await["title"], "Old work");
    let reused = call(&ann, "save_team", json!({ "name": "Engineering", "key": "ENG" })).await;
    assert_eq!(reused["key"], "ENG", "the key and name are free again");
    let log = call(&ann, "get_audit_log", json!({ "type": "team.archived" })).await;
    assert_eq!(log["nodes"][0]["metadata"]["key"], "ENG");
}

#[sqlx::test]
async fn triage_routes_issues_to_the_responsible_person(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    w.member("gus@else.test", "Gus", "guest").await;
    let blind = fails(&w.ann, "save_team", json!({ "id": "ENG", "triageResponsible": ["gus@else.test"] })).await;
    assert!(blind.contains("can't see this team"), "{blind}");

    let team = call(
        &w.ann,
        "save_team",
        json!({ "id": "ENG", "triageEnabled": true, "triageAction": "assign", "triageResponsible": ["bob", "me"],
                "triageRotationWeeks": 1 }),
    )
    .await;
    assert_eq!(team["states"][0]["type"], "triage");
    assert_eq!(team["triageRotationStart"], json!(chrono::Utc::now().date_naive()));
    assert_eq!(team["triage"]["responsible"][0]["email"], "bob@acme.test", "Bob has the first shift");
    let issue = call(&w.ann, "save_issue", json!({ "title": "Report", "state": "triage" })).await;
    assert_eq!(issue["assignee"]["email"], "bob@acme.test");

    let duty = call(&bob, "get_triage_responsibility", json!({ "id": "ENG", "limit": 4 })).await;
    assert_eq!(duty["action"], "assign");
    assert_eq!(duty["rotation"]["weeks"], 1);
    let shifts: Vec<&str> =
        duty["upcomingShifts"].as_array().unwrap().iter().map(|s| s["user"]["email"].as_str().unwrap()).collect();
    assert_eq!(shifts, vec!["bob@acme.test", "ann@acme.test", "bob@acme.test", "ann@acme.test"]);
    assert_eq!(duty["upcomingShifts"][0]["endsAt"], duty["upcomingShifts"][1]["startsAt"]);
    let mine = call(&bob, "get_triage_responsibility", json!({ "id": "ENG", "user": "ann", "limit": 2 })).await;
    assert_eq!(mine["upcomingShifts"][0]["startsAt"], duty["upcomingShifts"][1]["startsAt"]);
    assert_eq!(mine["upcomingShifts"][1]["startsAt"], duty["upcomingShifts"][3]["startsAt"]);

    let off = call(&w.ann, "save_team", json!({ "id": "ENG", "triageEnabled": false })).await;
    assert_ne!(off["states"][0]["type"], "triage");
    assert_eq!(call(&w.ann, "get_issue", json!({ "id": "ENG-1" })).await["state"]["name"], "Backlog");
    let on = call(&w.ann, "save_team", json!({ "id": "ENG", "triageEnabled": true })).await;
    assert_eq!(on["states"][0]["name"], "Triage");
    let statuses = call(&w.ann, "list_issue_statuses", json!({ "team": "ENG", "includeArchived": true })).await;
    assert_eq!(names(&statuses, "type").iter().filter(|t| *t == "triage").count(), 1, "the old Triage comes back");
}

#[sqlx::test]
async fn statuses_are_created_changed_and_archived(db: PgPool) {
    let w = world(db).await;
    let status = call(
        &w.ann,
        "save_issue_status",
        json!({ "team": "ENG", "name": "QA", "type": "started", "description": "Testing" }),
    )
    .await;
    assert_eq!(status["color"], "#f2c94c", "the type's color");
    assert_eq!(status["team"]["key"], "ENG");
    let order = names(&call(&w.ann, "list_issue_statuses", json!({ "team": "ENG" })).await, "name");
    assert_eq!(order, vec!["Backlog", "Todo", "In Progress", "In Review", "QA", "Done", "Canceled", "Duplicate"]);
    assert!(
        fails(&w.ann, "save_issue_status", json!({ "team": "ENG", "name": "qa", "type": "started" }))
            .await
            .contains("already")
    );
    assert!(
        fails(&w.ann, "save_issue_status", json!({ "team": "ENG", "name": "X", "type": "doing" }))
            .await
            .contains("status type")
    );

    call(&w.ann, "save_issue", json!({ "title": "Check", "state": "QA" })).await;
    call(&w.ann, "save_issue", json!({ "title": "Check more", "state": "QA" })).await;
    let renamed =
        call(&w.ann, "save_issue_status", json!({ "id": "QA", "team": "ENG", "name": "Testing", "description": null }))
            .await;
    assert_eq!(renamed["name"], "Testing");
    assert!(renamed["description"].is_null());
    assert_eq!(renamed["issueCount"], 2);
    let retype = fails(&w.ann, "save_issue_status", json!({ "id": status["id"], "type": "completed" })).await;
    assert!(retype.contains("2 issues"), "{retype}");

    let needs_target = fails(&w.ann, "delete_issue_status", json!({ "id": status["id"] })).await;
    assert!(needs_target.contains("moveIssuesTo"), "{needs_target}");
    let deleted = call(&w.ann, "delete_issue_status", json!({ "id": status["id"], "moveIssuesTo": "Done" })).await;
    assert!(deleted["archivedAt"].is_string());
    assert_eq!(deleted["movedIssues"], 2);
    let moved = call(&w.ann, "get_issue", json!({ "id": "ENG-1", "includeHistory": true })).await;
    assert_eq!(moved["state"]["name"], "Done");
    assert!(moved["completedAt"].is_string(), "moved like any state change");
    assert_eq!(moved["history"].as_array().unwrap().last().unwrap()["changes"]["state"]["to"]["name"], "Done");
    assert_eq!(call(&w.ann, "get_issue_status", json!({ "id": status["id"] })).await["name"], "Testing");

    let default = fails(&w.ann, "delete_issue_status", json!({ "id": "Backlog", "team": "ENG" })).await;
    assert!(default.contains("default status"), "{default}");
    call(&w.ann, "delete_issue_status", json!({ "id": "Backlog", "team": "ENG", "moveIssuesTo": "Todo" })).await;
    let team = call(&w.ann, "get_team", json!({ "id": "ENG" })).await;
    assert_eq!(team["defaultIssueState"]["name"], "Todo", "the default moves to the replacement");
    let last =
        fails(&w.ann, "delete_issue_status", json!({ "id": "Done", "team": "ENG", "moveIssuesTo": "Todo" })).await;
    assert!(last.contains("completed status"), "{last}");
    call(&w.ann, "delete_issue_status", json!({ "id": "Duplicate", "team": "ENG" })).await;
    let canceled =
        fails(&w.ann, "delete_issue_status", json!({ "id": "Canceled", "team": "ENG", "moveIssuesTo": "Todo" })).await;
    assert!(canceled.contains("canceled"), "{canceled}");
}

#[sqlx::test]
async fn label_groups_retiring_and_deleting(db: PgPool) {
    let w = world(db).await;
    let group = call(&w.ann, "save_issue_label", json!({ "name": "Size", "isGroup": true })).await;
    assert_eq!(group["groupType"], "singleSelect");
    call(&w.ann, "save_issue_label", json!({ "name": "Small", "parent": "Size", "color": "#00ff00" })).await;
    call(&w.ann, "save_issue_label", json!({ "name": "Large", "parent": "Size" })).await;
    let team_label =
        call(&w.ann, "save_issue_label", json!({ "name": "Backend", "team": "ENG", "parent": "Size" })).await;
    assert_eq!(team_label["team"]["key"], "ENG");
    assert_eq!(team_label["parent"]["name"], "Size");
    assert!(fails(&w.ann, "save_issue_label", json!({ "name": "small", "parent": "Size" })).await.contains("already"));
    assert!(fails(&w.ann, "save_issue_label", json!({ "name": "X", "parent": "Bug" })).await.contains("isn't a group"));
    assert!(fails(&w.ann, "save_issue_label", json!({ "id": "Size", "parent": "Size" })).await.contains("own group"));
    assert!(
        fails(&w.ann, "save_issue_label", json!({ "id": "Bug", "groupType": "multiSelect" })).await.contains("groups")
    );

    call(&w.ann, "save_issue", json!({ "title": "Sized", "labels": ["Small", "Bug"] })).await;
    let issue = call(&w.ann, "save_issue", json!({ "id": "ENG-1", "addLabels": ["Size/Large"] })).await;
    let labels: Vec<&str> = issue["labels"].as_array().unwrap().iter().map(|l| l["name"].as_str().unwrap()).collect();
    assert_eq!(labels, vec!["Bug", "Large"], "Large replaces Small in a single-select group");
    assert!(fails(&w.ann, "save_issue", json!({ "id": "ENG-1", "addLabels": ["Size"] })).await.contains("No label"));
    assert!(fails(&w.ann, "save_issue_label", json!({ "id": "Bug", "isGroup": true })).await.contains("on issues"));

    let listed = call(&w.ann, "list_issue_labels", json!({ "team": "ENG" })).await;
    assert_eq!(names(&listed, "name"), vec!["Bug", "Feature", "Improvement", "Size", "Backend", "Large", "Small"]);
    assert_eq!(listed["nodes"][3]["issueCount"], 1);

    let retired = call(&w.ann, "retire_issue_label", json!({ "id": "Bug" })).await;
    assert!(retired["retiredAt"].is_string());
    let kept = call(&w.ann, "get_issue", json!({ "id": "ENG-1" })).await;
    assert_eq!(kept["labels"].as_array().unwrap().len(), 2, "retired labels stay on issues");
    assert!(fails(&w.ann, "save_issue", json!({ "title": "New", "labels": ["Bug"] })).await.contains("No label"));
    assert!(!names(&call(&w.ann, "list_issue_labels", json!({})).await, "name").contains(&"Bug".to_string()));
    call(&w.ann, "restore_issue_label", json!({ "id": "Bug" })).await;
    call(&w.ann, "save_issue", json!({ "title": "New", "labels": ["Bug"] })).await;

    let deleted = call(&w.ann, "delete_issue_label", json!({ "id": "Size" })).await;
    assert_eq!(deleted["removedFromIssues"], 1);
    let issue = call(&w.ann, "get_issue", json!({ "id": "ENG-1" })).await;
    assert_eq!(issue["labels"].as_array().unwrap().len(), 1, "the group's labels went with it");
    assert!(fails(&w.ann, "get_issue_label", json!({ "id": "Large" })).await.contains("No label"));
}
