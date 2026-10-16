use serde_json::json;
use sqlx::PgPool;

use super::*;

/// A new context, since one caches the teams it sees and its role.
async fn fresh(c: &Ctx) -> Ctx {
    ctx(&c.state, c.user, c.workspace).await
}

#[sqlx::test]
async fn reads_and_updates_the_workspace(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    call(&w.ann, "save_issue", json!({ "title": "One" })).await;
    let workspace = call(&bob, "get_workspace", json!({})).await;
    assert_eq!(workspace["urlKey"], "acme");
    assert_eq!(workspace["url"], "http://localhost:3000/acme");
    assert_eq!(workspace["role"], "member");
    assert_eq!(
        workspace["teams"],
        json!([{ "id": workspace["teams"][0]["id"], "key": "ENG", "name": "Engineering", "private": false }])
    );
    assert_eq!(workspace["memberCount"], 2);
    assert_eq!(workspace["issueCount"], 1);

    assert!(fails(&bob, "update_workspace", json!({ "name": "Bobco" })).await.contains("admins"));
    sqlx::query("INSERT INTO workspaces (name, url_key) VALUES ('Other', 'other')").execute(&w.state.db).await.unwrap();
    assert!(fails(&w.ann, "update_workspace", json!({ "urlKey": "other" })).await.contains("taken"));
    assert!(fails(&w.ann, "update_workspace", json!({ "urlKey": "api" })).await.contains("reserved"));
    let updated = call(
        &w.ann,
        "update_workspace",
        json!({ "name": "Acme Inc", "urlKey": "Acme-Inc", "logoUrl": "https://x/l.png" }),
    )
    .await;
    assert_eq!(updated["name"], "Acme Inc");
    assert_eq!(updated["urlKey"], "acme-inc");
    assert_eq!(updated["logoUrl"], "https://x/l.png");
    let cleared = call(&w.ann, "update_workspace", json!({ "logoUrl": null })).await;
    assert!(cleared["logoUrl"].is_null());
    assert_eq!(cleared["name"], "Acme Inc", "omitted fields stay");

    let log = call(&w.ann, "get_audit_log", json!({ "type": "workspace.updated" })).await;
    assert_eq!(log["nodes"].as_array().unwrap().len(), 2);
    assert_eq!(log["nodes"][1]["metadata"]["urlKey"], json!({ "from": "acme", "to": "acme-inc" }));
    assert_eq!(log["nodes"][0]["actor"]["email"], "ann@acme.test");
    assert!(fails(&bob, "get_audit_log", json!({})).await.contains("admins"));
}

#[sqlx::test]
async fn lists_and_reads_members(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    w.member("cat@acme.test", "Cat Lee", "guest").await;
    call(&w.ann, "save_issue", json!({ "title": "Bob's", "assignee": "bob" })).await;
    call(&w.ann, "save_issue", json!({ "title": "Done", "assignee": "bob", "state": "Done" })).await;

    let all = call(&bob, "list_users", json!({})).await;
    let names: Vec<&str> = all["nodes"].as_array().unwrap().iter().map(|u| u["name"].as_str().unwrap()).collect();
    assert_eq!(names, vec!["Ann Smith", "Bob Jones", "Cat Lee"]);
    assert_eq!(all["nodes"][1]["isMe"], true);
    assert_eq!(all["nodes"][0]["role"], "owner");
    assert_eq!(all["nodes"][0]["teams"][0]["key"], "ENG");
    let found = call(&bob, "list_users", json!({ "query": "lee" })).await;
    assert_eq!(found["nodes"][0]["email"], "cat@acme.test");
    let team = call(&bob, "list_users", json!({ "team": "ENG" })).await;
    assert_eq!(team["nodes"].as_array().unwrap().len(), 1, "only Ann is in ENG");
    let page = call(&bob, "list_users", json!({ "limit": 2 })).await;
    assert_eq!(page["pageInfo"]["hasNextPage"], true);

    let me = call(&bob, "get_user", json!({ "id": "me" })).await;
    assert_eq!(me["email"], "bob@acme.test");
    assert_eq!(me["openIssueCount"], 1, "closed issues don't count");
    assert_eq!(call(&bob, "get_user", json!({ "id": "cat@acme.test" })).await["role"], "guest");
}

#[sqlx::test]
async fn role_changes_follow_the_rules(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    let ada = w.member("ada@acme.test", "Ada Admin", "admin").await;

    let alone = fails(&w.ann, "update_user", json!({ "id": "me", "role": "admin" })).await;
    assert!(alone.contains("needs an owner"), "{alone}");
    assert!(fails(&bob, "update_user", json!({ "id": "ada", "role": "member" })).await.contains("admins"));
    assert!(fails(&ada, "update_user", json!({ "id": "bob", "role": "owner" })).await.contains("Only owners"));
    assert!(fails(&ada, "update_user", json!({ "id": "ann", "role": "admin" })).await.contains("Only owners"));
    assert_eq!(call(&ada, "update_user", json!({ "id": "bob", "role": "admin" })).await["role"], "admin");
    assert!(fails(&ada, "update_user", json!({ "id": "bob", "role": "boss" })).await.contains("owner, admin"));

    call(&w.ann, "update_user", json!({ "id": "bob", "role": "owner" })).await;
    assert_eq!(call(&w.ann, "update_user", json!({ "id": "me", "role": "member" })).await["role"], "member");
    let bob = fresh(&bob).await;
    let last = fails(&bob, "update_user", json!({ "id": "me", "role": "admin" })).await;
    assert!(last.contains("needs an owner"), "{last}");

    let log = call(&bob, "get_audit_log", json!({ "type": "member.roleChanged" })).await;
    assert_eq!(log["nodes"][0]["metadata"]["to"], "member");
    assert_eq!(log["nodes"].as_array().unwrap().len(), 3);
}

#[sqlx::test]
async fn people_change_their_own_profile(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    let me =
        call(&bob, "update_user", json!({ "id": "me", "displayName": "bobby", "timezone": "Europe/Berlin" })).await;
    assert_eq!(me["displayName"], "bobby");
    assert_eq!(me["timezone"], "Europe/Berlin");
    assert!(call(&bob, "update_user", json!({ "id": "me", "timezone": null })).await["timezone"].is_null());
    assert!(fails(&bob, "update_user", json!({ "id": "me", "timezone": "Mars/Base" })).await.contains("timezone"));
    assert!(fails(&bob, "update_user", json!({ "id": "ann", "name": "Annie" })).await.contains("your own"));
    assert!(fails(&w.ann, "update_user", json!({ "id": "bob", "displayName": "b" })).await.contains("your own"));
}

#[sqlx::test]
async fn removes_members_but_not_the_last_owner(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;
    let ada = w.member("ada@acme.test", "Ada Admin", "admin").await;
    call(&w.ann, "save_team", json!({ "id": "ENG", "addMembers": ["bob"] })).await;

    assert!(fails(&bob, "remove_user", json!({ "id": "ada" })).await.contains("admins"));
    assert!(fails(&ada, "remove_user", json!({ "id": "ann" })).await.contains("Only owners"));
    assert!(fails(&w.ann, "remove_user", json!({ "id": "me" })).await.contains("last owner"));
    let removed = call(&ada, "remove_user", json!({ "id": "bob@acme.test" })).await;
    assert_eq!(removed["removed"], json!(bob.user));

    assert!(Ctx::load(&w.state, bob.user, w.workspace, vec![Access::Read]).await.unwrap().is_none());
    let active = call(&w.ann, "list_users", json!({})).await;
    assert_eq!(active["nodes"].as_array().unwrap().len(), 2);
    let everyone = call(&w.ann, "list_users", json!({ "includeInactive": true })).await;
    assert_eq!(everyone["nodes"].as_array().unwrap().len(), 3);
    let gone = call(&w.ann, "get_user", json!({ "id": bob.user })).await;
    assert_eq!(gone["active"], false);
    assert_eq!(gone["teams"], json!([]), "removed from their teams");
    assert!(fails(&w.ann, "get_user", json!({ "id": "bob" })).await.contains("No member"));
    let log = call(&w.ann, "get_audit_log", json!({ "type": "member.removed" })).await;
    assert_eq!(log["nodes"][0]["metadata"]["email"], "bob@acme.test");
}

#[sqlx::test]
async fn invitations(db: PgPool) {
    let w = world(db).await;
    let bob = w.member("bob@acme.test", "Bob Jones", "member").await;

    let invited = call(&bob, "invite_user", json!({ "email": " Dan@Else.test " })).await;
    assert_eq!(invited["email"], "dan@else.test");
    assert_eq!(invited["role"], "member");
    assert_eq!(invited["inviter"]["email"], "bob@acme.test");
    assert!(invited["expiresAt"].as_str().unwrap() > invited["createdAt"].as_str().unwrap());

    assert!(fails(&bob, "invite_user", json!({ "email": "eve@else.test", "role": "admin" })).await.contains("admins"));
    let guest = fails(&bob, "invite_user", json!({ "email": "gus@else.test", "role": "guest" })).await;
    assert!(guest.contains("teams"), "{guest}");
    let guest = call(&bob, "invite_user", json!({ "email": "gus@else.test", "role": "guest", "teams": ["ENG"] })).await;
    assert_eq!(guest["teams"][0]["key"], "ENG");
    assert!(fails(&bob, "invite_user", json!({ "email": "ann@acme.test" })).await.contains("already a member"));
    assert!(fails(&bob, "invite_user", json!({ "email": "nope" })).await.contains("isn't an email"));

    let again = call(&w.ann, "invite_user", json!({ "email": "dan@else.test", "role": "admin" })).await;
    assert_eq!(again["id"], invited["id"], "inviting again renews the invitation");
    assert_eq!(again["role"], "admin");
    assert_eq!(again["inviter"]["email"], "ann@acme.test");

    let pending = call(&bob, "list_invitations", json!({})).await;
    assert_eq!(pending["nodes"].as_array().unwrap().len(), 2);
    let denied = fails(&bob, "delete_invitation", json!({ "id": again["id"] })).await;
    assert!(denied.contains("someone else"), "{denied}");
    let deleted = call(&w.ann, "delete_invitation", json!({ "id": again["id"] })).await;
    assert_eq!(deleted["deleted"], again["id"]);
    assert_eq!(call(&bob, "list_invitations", json!({})).await["nodes"].as_array().unwrap().len(), 1);
}
