package e2e

import (
	"net/http/httptest"
	"testing"
)

func TestReadsAndUpdatesTheWorkspace(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	call(t, w.Ann, "save_issue", M{"title": "One"})
	workspace := call(t, bob, "get_workspace", M{})
	eq(t, workspace.At("urlKey").Str(), "acme")
	eq(t, workspace.At("url").Str(), "http://localhost:3000/acme")
	eq(t, workspace.At("role").Str(), "member")
	teams := []any{M{"id": workspace.At("teams", 0, "id").V, "key": "ENG", "name": "Engineering", "private": false}}
	eq(t, workspace.At("teams").JSON(), J{teams}.JSON())
	eq(t, workspace.At("memberCount").Num(), 2)
	eq(t, workspace.At("issueCount").Num(), 1)
	reachable := call(t, bob, "list_workspaces", M{})
	eq(t, reachable.At("nodes").Len(), 1)
	eq(t, reachable.At("nodes", 0, "urlKey").Str(), "acme")
	eq(t, reachable.At("nodes", 0, "role").Str(), "member")

	contains(t, fails(t, bob, "update_workspace", M{"name": "Bobco"}), "admins")
	w.exec("INSERT INTO workspaces (name, url_key) VALUES ('Other', 'other')")
	contains(t, fails(t, w.Ann, "update_workspace", M{"urlKey": "other"}), "taken")
	contains(t, fails(t, w.Ann, "update_workspace", M{"urlKey": "api"}), "reserved")
	updated := call(t, w.Ann, "update_workspace", M{"name": "Acme Inc", "urlKey": "Acme-Inc", "logoUrl": "https://x/l.png"})
	eq(t, updated.At("name").Str(), "Acme Inc")
	eq(t, updated.At("urlKey").Str(), "acme-inc")
	eq(t, updated.At("logoUrl").Str(), "https://x/l.png")
	cleared := call(t, w.Ann, "update_workspace", M{"logoUrl": nil})
	check(t, cleared.At("logoUrl").IsNull())
	eq(t, cleared.At("name").Str(), "Acme Inc", "omitted fields stay")

	log := call(t, w.Ann, "get_audit_log", M{"type": "workspace.updated"})
	eq(t, log.At("nodes").Len(), 2)
	eq(t, log.At("nodes", 1, "metadata", "urlKey").JSON(), `{"from":"acme","to":"acme-inc"}`)
	eq(t, log.At("nodes", 0, "actor", "email").Str(), "ann@acme.test")
	contains(t, fails(t, bob, "get_audit_log", M{}), "admins")
}

func TestSetsTheWorkspaceTheme(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	eq(t, call(t, bob, "get_workspace", M{}).At("theme").Str(), "default")

	contains(t, fails(t, bob, "update_workspace", M{"theme": "dank"}), "admins")
	contains(t, fails(t, w.Ann, "update_workspace", M{"theme": "neon"}), "theme")
	eq(t, call(t, w.Ann, "update_workspace", M{"theme": "dank"}).At("theme").Str(), "dank")
	eq(t, call(t, bob, "get_workspace", M{}).At("theme").Str(), "dank")

	req := httptest.NewRequest("GET", "/api/me", nil)
	req.Header.Set("Cookie", w.session(bob.User))
	_, me := send(w.Router, req)
	eq(t, me.At("workspaces", 0, "theme").Str(), "dank")

	log := call(t, w.Ann, "get_audit_log", M{"type": "workspace.updated"})
	eq(t, log.At("nodes").Len(), 1)
	eq(t, log.At("nodes", 0, "metadata", "theme").JSON(), `{"from":"default","to":"dank"}`)
}

func TestEachMemberMayOverrideTheWorkspaceTheme(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	call(t, w.Ann, "update_workspace", M{"theme": "dank"})
	ownTheme := func(c *Actor) J {
		req := httptest.NewRequest("GET", "/api/me", nil)
		req.Header.Set("Cookie", w.session(c.User))
		_, me := send(w.Router, req)
		return me.At("workspaces", 0, "ownTheme")
	}
	check(t, call(t, bob, "get_user", M{"id": "me"}).At("theme").IsNull())
	check(t, ownTheme(bob).IsNull(), "follows the workspace")

	contains(t, fails(t, bob, "update_user", M{"id": "me", "theme": "neon"}), "theme")
	contains(t, fails(t, bob, "update_user", M{"id": "ann", "theme": "default"}), "your own")
	eq(t, call(t, bob, "update_user", M{"id": "me", "theme": "default"}).At("theme").Str(), "default")
	eq(t, ownTheme(bob).Str(), "default")
	check(t, ownTheme(w.Ann).IsNull(), "others keep following")
	check(t, call(t, w.Ann, "get_user", M{"id": "bob"}).At("theme").IsNull(), "only shown for yourself")
	eq(t, call(t, bob, "get_workspace", M{}).At("theme").Str(), "dank")

	check(t, call(t, bob, "update_user", M{"id": "me", "theme": nil}).At("theme").IsNull())
	check(t, ownTheme(bob).IsNull())
}

func TestListsAndReadsMembers(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	w.member("cat@acme.test", "Cat Lee", "guest")
	call(t, w.Ann, "save_issue", M{"title": "Bob's", "assignee": "bob"})
	call(t, w.Ann, "save_issue", M{"title": "Done", "assignee": "bob", "state": "Done"})

	all := call(t, bob, "list_users", M{})
	eqList(t, all.At("nodes").Strings("name"), []string{"Ann Smith", "Bob Jones", "Cat Lee"})
	eq(t, all.At("nodes", 1, "isMe").Bool(), true)
	eq(t, all.At("nodes", 0, "role").Str(), "owner")
	eq(t, all.At("nodes", 0, "teams", 0, "key").Str(), "ENG")
	eq(t, call(t, bob, "list_users", M{"query": "lee"}).At("nodes", 0, "email").Str(), "cat@acme.test")
	eq(t, call(t, bob, "list_users", M{"team": "ENG"}).At("nodes").Len(), 2, "everyone but the guest is in ENG")
	eq(t, call(t, bob, "list_users", M{"limit": 2}).At("pageInfo", "hasNextPage").Bool(), true)

	me := call(t, bob, "get_user", M{"id": "me"})
	eq(t, me.At("email").Str(), "bob@acme.test")
	eq(t, me.At("openIssueCount").Num(), 1, "closed issues don't count")
	eq(t, call(t, bob, "get_user", M{"id": "cat@acme.test"}).At("role").Str(), "guest")
}

func TestRoleChangesFollowTheRules(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	ada := w.member("ada@acme.test", "Ada Admin", "admin")

	contains(t, fails(t, w.Ann, "update_user", M{"id": "me", "role": "admin"}), "needs an owner")
	contains(t, fails(t, bob, "update_user", M{"id": "ada", "role": "member"}), "admins")
	contains(t, fails(t, ada, "update_user", M{"id": "bob", "role": "owner"}), "Only owners")
	contains(t, fails(t, ada, "update_user", M{"id": "ann", "role": "admin"}), "Only owners")
	eq(t, call(t, ada, "update_user", M{"id": "bob", "role": "admin"}).At("role").Str(), "admin")
	contains(t, fails(t, ada, "update_user", M{"id": "bob", "role": "boss"}), "owner, admin")

	call(t, w.Ann, "update_user", M{"id": "bob", "role": "owner"})
	eq(t, call(t, w.Ann, "update_user", M{"id": "me", "role": "member"}).At("role").Str(), "member")
	contains(t, fails(t, bob, "update_user", M{"id": "me", "role": "admin"}), "needs an owner")

	log := call(t, bob, "get_audit_log", M{"type": "member.roleChanged"})
	eq(t, log.At("nodes", 0, "metadata", "to").Str(), "member")
	eq(t, log.At("nodes").Len(), 3)
}

func TestPeopleChangeTheirOwnProfile(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	me := call(t, bob, "update_user", M{"id": "me", "displayName": "bobby", "timezone": "Europe/Berlin"})
	eq(t, me.At("displayName").Str(), "bobby")
	eq(t, me.At("timezone").Str(), "Europe/Berlin")
	check(t, call(t, bob, "update_user", M{"id": "me", "timezone": nil}).At("timezone").IsNull())
	contains(t, fails(t, bob, "update_user", M{"id": "me", "timezone": "Mars/Base"}), "timezone")
	contains(t, fails(t, bob, "update_user", M{"id": "ann", "name": "Annie"}), "your own")
	contains(t, fails(t, w.Ann, "update_user", M{"id": "bob", "displayName": "b"}), "your own")
}

func TestRemovesMembersButNotTheLastOwner(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	ada := w.member("ada@acme.test", "Ada Admin", "admin")
	call(t, w.Ann, "save_team", M{"id": "ENG", "addMembers": []any{"bob"}})

	contains(t, fails(t, bob, "remove_user", M{"id": "ada"}), "admins")
	contains(t, fails(t, ada, "remove_user", M{"id": "ann"}), "Only owners")
	contains(t, fails(t, w.Ann, "remove_user", M{"id": "me"}), "last owner")
	removed := call(t, ada, "remove_user", M{"id": "bob@acme.test"})
	eq(t, removed.At("removed").Str(), bob.User.String())

	active := scalar[bool](w, "SELECT active FROM workspace_members WHERE workspace_id = $1 AND user_id = $2", w.Workspace, bob.User)
	check(t, !active, "removed members can't act in the workspace")
	eq(t, call(t, w.Ann, "list_users", M{}).At("nodes").Len(), 2)
	eq(t, call(t, w.Ann, "list_users", M{"includeInactive": true}).At("nodes").Len(), 3)
	read := call(t, w.Ann, "get_user", M{"id": bob.User.String()})
	eq(t, read.At("active").Bool(), false)
	eq(t, read.At("teams").JSON(), "[]", "removed from their teams")
	contains(t, fails(t, w.Ann, "get_user", M{"id": "bob"}), "No member")
	log := call(t, w.Ann, "get_audit_log", M{"type": "member.removed"})
	eq(t, log.At("nodes", 0, "metadata", "email").Str(), "bob@acme.test")
}

func TestInvitations(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")

	invited := call(t, bob, "invite_user", M{"email": " Dan@Else.test "})
	eq(t, invited.At("email").Str(), "dan@else.test")
	eq(t, invited.At("role").Str(), "member")
	eq(t, invited.At("inviter", "email").Str(), "bob@acme.test")
	check(t, invited.At("expiresAt").Str() > invited.At("createdAt").Str())

	contains(t, fails(t, bob, "invite_user", M{"email": "eve@else.test", "role": "admin"}), "admins")
	guest := call(t, bob, "invite_user", M{"email": "gus@else.test", "role": "guest"})
	eqList(t, guest.At("teams").Strings("key"), []string{"ENG"}, "guests see the workspace's team by default")
	guest = call(t, bob, "invite_user", M{"email": "gus@else.test", "role": "guest", "teams": []any{"ENG"}})
	eq(t, guest.At("teams", 0, "key").Str(), "ENG")
	contains(t, fails(t, bob, "invite_user", M{"email": "ann@acme.test"}), "already a member")
	contains(t, fails(t, bob, "invite_user", M{"email": "nope"}), "isn't an email")

	again := call(t, w.Ann, "invite_user", M{"email": "dan@else.test", "role": "admin"})
	eq(t, again.At("id").Str(), invited.At("id").Str(), "inviting again renews the invitation")
	eq(t, again.At("role").Str(), "admin")
	eq(t, again.At("inviter", "email").Str(), "ann@acme.test")

	eq(t, call(t, bob, "list_invitations", M{}).At("nodes").Len(), 2)
	contains(t, fails(t, bob, "delete_invitation", M{"id": again.At("id").Str()}), "someone else")
	deleted := call(t, w.Ann, "delete_invitation", M{"id": again.At("id").Str()})
	eq(t, deleted.At("deleted").Str(), again.At("id").Str())
	eq(t, call(t, bob, "list_invitations", M{}).At("nodes").Len(), 1)
}
