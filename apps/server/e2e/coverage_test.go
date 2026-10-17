package e2e

import (
	"context"
	"testing"
)

// Paths the other tests leave out, there for their recordings.

func TestAutomationsArchiveCloseAndEmptyTheTrash(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	w.exec("UPDATE teams SET auto_archive_months = 1, auto_close_months = 1")
	call(t, w.Ann, "save_issue", M{"title": "Shipped long ago", "state": "Done"})
	call(t, w.Ann, "save_issue", M{"title": "Forgotten idea"})
	call(t, w.Ann, "save_issue", M{"title": "Thrown away"})
	call(t, w.Ann, "save_issue", M{"title": "Due yesterday", "assignee": "bob", "dueDate": "2020-01-01"})
	call(t, w.Ann, "save_issue", M{"title": "Recent idea"})
	call(t, w.Ann, "delete_issue", M{"id": "ENG-3"})
	w.exec("UPDATE issues SET completed_at = now() - interval '2 months', updated_at = now() - interval '2 months' WHERE number = 1")
	w.exec("UPDATE issues SET updated_at = now() - interval '2 months' WHERE number = 2")
	w.exec("UPDATE issues SET trashed_at = now() - interval '31 days' WHERE number = 3")
	call(t, w.Ann, "save_issue", M{"id": "ENG-5", "snoozedUntil": "P1D"})
	w.exec("UPDATE issues SET snoozed_until = now() - interval '1 minute' WHERE number = 5")
	if err := w.B.Automations(context.Background()); err != nil {
		t.Fatal(err)
	}
	archived := call(t, w.Ann, "get_issue", M{"id": "ENG-1", "includeHistory": true})
	check(t, archived.At("archivedAt").IsString() && archived.At("autoArchivedAt").IsString(), archived.JSON())
	closed := call(t, w.Ann, "get_issue", M{"id": "ENG-2", "includeHistory": true})
	eq(t, closed.At("state", "name").Str(), "Canceled", closed.JSON())
	check(t, closed.At("autoClosedAt").IsString())
	contains(t, fails(t, w.Ann, "get_issue", M{"id": "ENG-3"}), "ENG-3")
	check(t, call(t, w.Ann, "get_issue", M{"id": "ENG-5"}).At("snoozedUntil").IsNull())
	inbox := call(t, bob, "get_notifications", M{})
	contains(t, inbox.JSON(), "ENG-4 is overdue")
	call(t, w.Ann, "list_issues", M{"includeArchived": true, "includeTrashed": true})
	if err := w.B.Automations(context.Background()); err != nil {
		t.Fatal(err)
	}
	eq(t, call(t, bob, "get_notifications", M{}).At("nodes").Len(), inbox.At("nodes").Len(), "due notices come once a day")
}

func TestReadsLabelsViewsAndTemplatesByName(t *testing.T) {
	w := world(t)
	call(t, w.Ann, "save_issue_label", M{"name": "Type", "isGroup": true, "groupType": "multiSelect"})
	call(t, w.Ann, "save_issue_label", M{"name": "Bug", "parent": "Type", "team": "ENG"})
	call(t, w.Ann, "get_issue_label", M{"id": "Type"})
	call(t, w.Ann, "get_issue_label", M{"id": "Type/Bug"})
	call(t, w.Ann, "get_issue_label", M{"id": "Bug", "team": "ENG"})
	fails(t, w.Ann, "get_issue_label", M{"id": "Bug"})
	call(t, w.Ann, "list_issue_labels", M{"team": "ENG", "query": "bu"})
	call(t, w.Ann, "save_custom_view", M{"name": "Urgent", "filter": M{"priority": M{"eq": 1}}, "shared": true})
	call(t, w.Ann, "get_custom_view", M{"id": "Urgent"})
	call(t, w.Ann, "save_issue", M{"title": "On fire", "priority": 1})
	call(t, w.Ann, "list_issues", M{"customView": "Urgent"})
}

func TestSavesIssuesWithEveryField(t *testing.T) {
	w := world(t)
	w.member("bob@acme.test", "Bob Jones", "member")
	call(t, w.Ann, "save_team", M{"id": "ENG", "estimateType": "fibonacci"})
	call(t, w.Ann, "save_project", M{"name": "Launch", "addTeams": []any{"ENG"}})
	call(t, w.Ann, "save_milestone", M{"project": "Launch", "name": "Beta"})
	call(t, w.Ann, "save_issue", M{"title": "Parent"})
	call(t, w.Ann, "save_issue", M{"title": "Other"})
	full := call(t, w.Ann, "save_issue", M{
		"team": "ENG", "title": "Everything", "description": "Hello @bob", "state": "Todo", "priority": 2, "estimate": 3,
		"assignee": "bob", "labels": []any{"Bug", "Feature"}, "project": "Launch", "milestone": "Beta", "parentId": "ENG-1",
		"dueDate": "2030-01-02", "blocks": []any{"ENG-2"}, "links": []any{M{"url": "https://figma.com/file/x", "title": "Design"}},
		"addSubscribers": []any{"bob"}, "slaBreachesAt": "2030-01-03T00:00:00Z", "slaType": "all", "sortOrder": 12.5,
		"subIssueSortOrder": 1,
	})
	eq(t, full.At("identifier").Str(), "ENG-3")
	call(t, w.Ann, "get_issue", M{"id": "ENG-3", "includeComments": true, "includeHistory": true})
	call(t, w.Ann, "save_issue", M{"id": "ENG-3", "estimate": nil, "assignee": nil, "addLabels": []any{"Improvement"},
		"removeLabels": []any{"Bug"}, "project": nil, "parentId": nil, "dueDate": nil, "removeBlocks": []any{"ENG-2"},
		"relatedTo": []any{"ENG-2"}, "removeSubscribers": []any{"bob"}, "slaBreachesAt": nil, "duplicateOf": "ENG-1"})
	call(t, w.Ann, "get_issue", M{"id": "ENG-3", "includeHistory": true})
	call(t, w.Ann, "update_issues", M{"ids": []any{"ENG-1", "ENG-2"}, "changes": M{"state": "In Progress", "priority": 1, "assignee": "me"}})
	fails(t, w.Ann, "update_issues", M{"ids": []any{"ENG-1", "ENG-404"}, "changes": M{"priority": 1}})
	fails(t, w.Ann, "save_issue", M{"id": "ENG-1", "estimate": 4})
	for _, args := range []M{
		{"state": "started"}, {"assignee": "me"}, {"label": "Improvement"}, {"project": "null"}, {"project": "Launch"},
		{"parentId": "ENG-1"}, {"priority": 1}, {"createdAt": "-P1D"}, {"updatedAt": "2020-01-01"}, {"dueBefore": "2030-12-31"},
		{"orderBy": "priority"}, {"orderBy": "createdAt"}, {"orderBy": "dueDate"}, {"orderBy": "identifier"}, {"orderBy": "manual"},
		{"includeDescription": true, "limit": 1}, {"subscriber": "me"}, {"creator": "me"},
	} {
		call(t, w.Ann, "list_issues", args)
	}
	call(t, w.Ann, "archive_issue", M{"id": "ENG-2"})
	call(t, w.Ann, "list_issues", M{"includeArchived": true})
	call(t, w.Ann, "unarchive_issue", M{"id": "ENG-2"})
	call(t, w.Ann, "get_issue_history", M{"id": "ENG-2"})
}
