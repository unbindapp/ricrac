package e2e

import (
	"testing"
)

func TestCreatesIssuesWithIdentifiersAndDefaults(t *testing.T) {
	w := world(t)
	first := call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "First", "description": "Hello"})
	eq(t, first.At("identifier").Str(), "ENG-1")
	eq(t, first.At("state", "name").Str(), "Backlog")
	eq(t, first.At("creator", "email").Str(), "ann@acme.test")
	eq(t, first.At("url").Str(), "http://localhost:3000/acme/issue/ENG-1")
	eq(t, first.At("branchName").Str(), "eng-1-first")
	second := call(t, w.Ann, "save_issue", M{"title": "Second", "state": "started", "priority": 1, "assignee": "me", "labels": []any{"Bug"}})
	eq(t, second.At("identifier").Str(), "ENG-2", "the only team is used when none is named")
	eq(t, second.At("state", "name").Str(), "In Progress")
	check(t, second.At("startedAt").IsString())
	eq(t, second.At("priorityLabel").Str(), "Urgent")
	eq(t, second.At("assignee", "name").Str(), "Ann Smith")
	eq(t, second.At("labels", 0, "name").Str(), "Bug")
	missing := fails(t, w.Ann, "save_issue", M{"team": "ENG"})
	contains(t, missing, "title")
	unknown := fails(t, w.Ann, "save_issue", M{"team": "ENG", "title": "x", "state": "Nope"})
	contains(t, unknown, "Backlog", "Todo")
	typo := fails(t, w.Ann, "save_issue", M{"team": "ENG", "title": "x", "titel": "y"})
	contains(t, typo, "titel")
}

func TestUpdatesStatesWithTimestampsAndHistory(t *testing.T) {
	w := world(t)
	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Fix login"})
	started := call(t, w.Ann, "save_issue", M{"id": "eng-1", "state": "In Progress"})
	check(t, started.At("startedAt").IsString())
	done := call(t, w.Ann, "save_issue", M{"id": "ENG-1", "state": "Done", "title": "Fix the login"})
	check(t, done.At("completedAt").IsString())
	eq(t, done.At("title").Str(), "Fix the login")
	reopened := call(t, w.Ann, "save_issue", M{"id": "ENG-1", "state": "Todo"})
	check(t, reopened.At("completedAt").IsNull() && reopened.At("startedAt").IsNull())
	history := call(t, w.Ann, "get_issue_history", M{"id": "ENG-1"})
	actions := history.At("history").Strings("action")
	eq(t, actions[0], "created")
	renamed := false
	for _, h := range history.At("history").List() {
		renamed = renamed || h.At("changes", "title", "to").Str() == "Fix the login"
	}
	check(t, renamed, "the title change is in the history")
}

func TestPatchesDescriptionsAndClearsFields(t *testing.T) {
	w := world(t)
	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Doc", "description": "## Steps\n\n1. Open\n2. Crash\n", "dueDate": "2030-01-02", "estimate": nil})
	patched := call(t, w.Ann, "save_issue", M{"id": "ENG-1", "patch": []any{
		M{"op": "replace", "old_string": "Crash", "new_string": "It crashes"},
		M{"op": "append", "text": "\nExpected: no crash\n"},
	}, "dueDate": nil})
	eq(t, patched.At("description").Str(), "## Steps\n\n1. Open\n2. It crashes\n\nExpected: no crash\n")
	check(t, patched.At("dueDate").IsNull())
	both := fails(t, w.Ann, "save_issue", M{"id": "ENG-1", "description": "x", "patch": []any{M{"op": "append", "text": "y"}}})
	contains(t, both, "either")
}

func TestListsWithSimpleArgumentsAndFilters(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	call(t, w.Ann, "save_issue", M{"title": "Urgent bug", "priority": 1, "labels": []any{"Bug"}, "assignee": "bob"})
	call(t, w.Ann, "save_issue", M{"title": "Feature work", "priority": 3, "labels": []any{"Feature"}, "assignee": "me", "state": "started"})
	call(t, w.Ann, "save_issue", M{"title": "Unassigned idea", "description": "about payments"})

	eqList(t, identifiers(call(t, bob, "list_issues", M{"assignee": "me"})), []string{"ENG-1"})
	eqList(t, identifiers(call(t, w.Ann, "list_issues", M{"assignee": "null"})), []string{"ENG-3"})
	eqList(t, identifiers(call(t, w.Ann, "list_issues", M{"label": "bug"})), []string{"ENG-1"})
	eqList(t, identifiers(call(t, w.Ann, "list_issues", M{"state": "started"})), []string{"ENG-2"})
	eqList(t, identifiers(call(t, w.Ann, "list_issues", M{"query": "payments"})), []string{"ENG-3"})
	eqList(t, identifiers(call(t, w.Ann, "list_issues", M{"query": "ENG-2"})), []string{"ENG-2"})
	filtered := call(t, w.Ann, "list_issues", M{"filter": M{"or": []any{
		M{"priority": M{"lte": 2, "neq": 0}},
		M{"labels": M{"some": M{"name": M{"eq": "feature"}}}},
	}}, "orderBy": "priority"})
	eqList(t, identifiers(filtered), []string{"ENG-1", "ENG-2"})
	none := call(t, w.Ann, "list_issues", M{"filter": M{"labels": M{"null": true}}})
	eqList(t, identifiers(none), []string{"ENG-3"})
	bad := fails(t, w.Ann, "list_issues", M{"filter": M{"colour": M{"eq": "red"}}})
	contains(t, bad, "colour")
	page := call(t, w.Ann, "list_issues", M{"limit": 2, "orderBy": "identifier"})
	eqList(t, identifiers(page), []string{"ENG-1", "ENG-2"})
	next := call(t, w.Ann, "list_issues", M{"limit": 2, "orderBy": "identifier", "cursor": page.At("pageInfo", "endCursor").V})
	eqList(t, identifiers(next), []string{"ENG-3"})
	eq(t, next.At("pageInfo", "hasNextPage").Bool(), false)
}

func TestRelationsSubIssuesAndParentAutomation(t *testing.T) {
	w := world(t)
	w.exec("UPDATE teams SET auto_close_parent_issues = true")
	call(t, w.Ann, "save_issue", M{"title": "Parent"})
	call(t, w.Ann, "save_issue", M{"title": "Child A", "parentId": "ENG-1"})
	call(t, w.Ann, "save_issue", M{"title": "Child B", "parentId": "ENG-1", "blocks": []any{"ENG-2"}})
	parent := call(t, w.Ann, "get_issue", M{"id": "ENG-1"})
	eq(t, parent.At("children").Len(), 2)
	a := call(t, w.Ann, "get_issue", M{"id": "ENG-2"})
	eq(t, a.At("relations", "blockedBy", 0, "identifier").Str(), "ENG-3")
	loop := fails(t, w.Ann, "save_issue", M{"id": "ENG-1", "parentId": "ENG-2"})
	contains(t, loop, "own sub-issues")
	call(t, w.Ann, "save_issue", M{"id": "ENG-2", "state": "Done"})
	parent = call(t, w.Ann, "get_issue", M{"id": "ENG-1"})
	eq(t, parent.At("state", "name").Str(), "Backlog", "one child is still open")
	call(t, w.Ann, "update_issues", M{"ids": []any{"ENG-3"}, "changes": M{"state": "completed"}})
	parent = call(t, w.Ann, "get_issue", M{"id": "ENG-1", "includeHistory": true})
	eq(t, parent.At("state", "type").Str(), "completed")
	automated := false
	for _, h := range parent.At("history").List() {
		automated = automated || h.At("action").Str() == "auto_completed_by_sub_issues"
	}
	check(t, automated, "the parent was completed by its sub-issues")
	dup := call(t, w.Ann, "save_issue", M{"title": "Same", "duplicateOf": "ENG-1"})
	eq(t, dup.At("state", "type").Str(), "canceled")
}

func TestMovingTeamsKeepsOldIdentifiersWorking(t *testing.T) {
	w := world(t)
	w.createTeam("Design", "DES")
	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Logo", "state": "In Progress"})
	moved := call(t, w.Ann, "save_issue", M{"id": "ENG-1", "team": "Design"})
	eq(t, moved.At("identifier").Str(), "DES-1")
	eq(t, moved.At("state", "name").Str(), "In Progress", "a status of the same name")
	eqList(t, moved.At("previousIdentifiers").Strings(), []string{"ENG-1"})
	found := call(t, w.Ann, "get_issue", M{"id": "ENG-1"})
	eq(t, found.At("identifier").Str(), "DES-1")
}

func TestTrashArchiveAndPrivateTeams(t *testing.T) {
	w := world(t)
	guest := w.member("gus@else.test", "Gus", "guest")
	call(t, w.Ann, "save_issue", M{"title": "Secret"})
	contains(t, fails(t, guest, "get_issue", M{"id": "ENG-1"}), "No issue")
	call(t, w.Ann, "share_issue", M{"id": "ENG-1", "user": "gus@else.test"})
	eq(t, call(t, guest, "get_issue", M{"id": "ENG-1"}).At("title").Str(), "Secret")
	call(t, w.Ann, "delete_issue", M{"id": "ENG-1"})
	eq(t, len(identifiers(call(t, w.Ann, "list_issues", M{}))), 0)
	eqList(t, identifiers(call(t, w.Ann, "list_issues", M{"includeTrashed": true})), []string{"ENG-1"})
	call(t, w.Ann, "restore_issue", M{"id": "ENG-1"})
	call(t, w.Ann, "archive_issue", M{"id": "ENG-1"})
	eq(t, len(identifiers(call(t, w.Ann, "list_issues", M{}))), 0)
	call(t, w.Ann, "unarchive_issue", M{"id": "ENG-1"})
	eqList(t, identifiers(call(t, w.Ann, "list_issues", M{})), []string{"ENG-1"})
}

func TestNotifiesAssigneesAndMentions(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	cat := w.member("cat@acme.test", "Cat Lee", "member")
	call(t, w.Ann, "save_issue", M{"title": "Review", "assignee": "bob@acme.test", "description": "cc @cat"})
	kinds := func(user any) []string {
		return w.strings("SELECT type FROM notifications WHERE user_id = $1 ORDER BY created_at", user)
	}
	eqList(t, kinds(bob.User), []string{"issueAssignedToYou"})
	eqList(t, kinds(cat.User), []string{"issueMention"})
	eq(t, len(kinds(w.Ann.User)), 0, "nobody hears about their own change")
	call(t, bob, "save_issue", M{"id": "ENG-1", "state": "Done"})
	eqList(t, kinds(w.Ann.User), []string{"issueStatusChanged"})
}
