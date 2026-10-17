package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func enableCycles(w *World) {
	w.t.Helper()
	w.exec("UPDATE teams SET cycles_enabled = true")
	runCycleAutomations(w)
}

func runCycleAutomations(w *World) {
	w.t.Helper()
	if err := w.B.RunCycleAutomations(context.Background()); err != nil {
		w.t.Fatalf("cycle automations: %v", err)
	}
}

func numbers(list J) []int {
	found := []int{}
	for _, node := range list.At("nodes").List() {
		found = append(found, int(node.At("number").Num()))
	}
	return found
}

func titles(list J) []string {
	return list.At("nodes").Strings("title")
}

func TestCyclesAreCreatedAheadAndRollUnfinishedIssuesOver(t *testing.T) {
	w := world(t)
	enableCycles(w)
	runCycleAutomations(w)
	eqList(t, numbers(call(t, w.Ann, "list_cycles", M{"team": "ENG"})), []int{3, 2, 1})
	eqList(t, numbers(call(t, w.Ann, "list_cycles", M{"team": "ENG", "type": "upcoming"})), []int{2, 3})
	current := call(t, w.Ann, "get_cycle", M{"id": "current", "team": "ENG"})
	eq(t, current.At("number").Num(), 1)
	eq(t, current.At("isActive").Bool(), true)
	eq(t, current.At("history").Len(), 1, "one snapshot a day")

	call(t, w.Ann, "save_issue", M{"title": "Carry over", "cycle": "current", "estimate": nil})
	call(t, w.Ann, "save_issue", M{"title": "Done in time", "cycle": "current", "state": "Done"})
	current = call(t, w.Ann, "get_cycle", M{"id": "current", "team": "ENG"})
	eq(t, current.At("scope", "issues").Num(), 2)
	eq(t, current.At("scope", "completed").Num(), 1)
	eq(t, current.At("progress").Num(), 0.5)
	eq(t, current.At("issues").Len(), 2)

	w.exec("UPDATE cycles SET ends_at = now() - interval '1 minute' WHERE number = 1")
	runCycleAutomations(w)
	first := call(t, w.Ann, "get_cycle", M{"id": "1", "team": "ENG"})
	check(t, first.At("completedAt").IsString())
	eq(t, first.At("issues", 0, "identifier").Str(), "ENG-2", "finished issues stay")
	eq(t, first.At("issues").Len(), 1)
	moved := call(t, w.Ann, "get_issue", M{"id": "ENG-1"})
	eq(t, moved.At("cycle", "number").Num(), 2)
	history := call(t, w.Ann, "get_issue_history", M{"id": "ENG-1"})
	var rollover *J
	for _, h := range history.At("history").List() {
		if h.At("action").Str() == "cycle_rollover" {
			rollover = &h
		}
	}
	check(t, rollover != nil, "the rollover is in the history")
	eq(t, rollover.At("changes", "cycle", "from", "number").Num(), 1)
	eq(t, rollover.At("changes", "cycle", "to", "number").Num(), 2)
	check(t, rollover.At("actor").IsNull())
	eqList(t, numbers(call(t, w.Ann, "list_cycles", M{"team": "ENG", "type": "completed"})), []int{1})
	eqList(t, numbers(call(t, w.Ann, "list_cycles", M{"team": "ENG"})), []int{3, 2, 1})
}

func TestManualCyclesDontOverlap(t *testing.T) {
	w := world(t)
	disabled := fails(t, w.Ann, "save_cycle", M{"team": "ENG", "startsAt": "2030-01-01", "endsAt": "2030-01-15"})
	contains(t, disabled, "cyclesEnabled")
	enableCycles(w)
	overlap := fails(t, w.Ann, "save_cycle", M{"team": "ENG", "startsAt": "-PT1H", "endsAt": "P3D"})
	contains(t, overlap, "overlap cycle 1")
	launch := call(t, w.Ann, "save_cycle", M{"team": "ENG", "startsAt": "2030-01-01", "endsAt": "2030-01-15", "name": "Launch"})
	eq(t, launch.At("number").Num(), 4)
	eq(t, launch.At("isFuture").Bool(), true)
	backwards := fails(t, w.Ann, "save_cycle", M{"id": launch.At("id").V, "endsAt": "2029-12-01"})
	contains(t, backwards, "after")
	renamed := call(t, w.Ann, "save_cycle", M{"id": "Launch", "team": "ENG", "name": "Big launch"})
	eq(t, renamed.At("name").Str(), "Big launch")
	current := fails(t, w.Ann, "archive_cycle", M{"id": "current", "team": "ENG"})
	contains(t, current, "current")
	call(t, w.Ann, "save_issue", M{"title": "Later", "cycle": "4"})
	archived := call(t, w.Ann, "archive_cycle", M{"id": launch.At("id").V})
	check(t, archived.At("archivedAt").IsString())
	check(t, call(t, w.Ann, "get_issue", M{"id": "ENG-1"}).At("cycle").IsNull())
	eqList(t, numbers(call(t, w.Ann, "list_cycles", M{"team": "ENG"})), []int{3, 2, 1})
}

func TestDocumentsKeepVersionsAndFollowVisibility(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	gus := w.member("gus@acme.test", "Gus Guest", "guest")
	call(t, w.Ann, "save_issue", M{"title": "Spec"})
	doc := call(t, w.Ann, "save_document", M{"title": "Design Notes!", "content": "Hello @bob", "team": "ENG", "icon": "📐"})
	slug := doc.At("slugId").Str()
	eq(t, doc.At("url").Str(), "http://localhost:3000/acme/document/design-notes-"+slug)
	eq(t, doc.At("team", "key").Str(), "ENG")
	eq(t, doc.At("versionCount").Num(), 0)
	eq(t, scalar[int64](w, "SELECT count(*) FROM notifications WHERE type = 'documentMention'"), 1)
	two := fails(t, w.Ann, "save_document", M{"title": "x", "team": "ENG", "issue": "ENG-1"})
	contains(t, two, "only one")
	none := fails(t, w.Ann, "save_document", M{"title": "x"})
	contains(t, none, "belongs")
	call(t, w.Ann, "save_document", M{"title": "Issue doc", "issue": "ENG-1"})

	patched := call(t, w.Ann, "save_document", M{"id": slug, "patch": []any{M{"op": "replace", "old_string": "Hello", "new_string": "Hi"}}})
	eq(t, patched.At("content").Str(), "Hi @bob")
	eq(t, patched.At("versionCount").Num(), 1)
	again := call(t, w.Ann, "save_document", M{"id": doc.At("id").V, "content": "Third"})
	eq(t, again.At("versionCount").Num(), 1, "one author's edits within 10 minutes make one version")
	byBob := call(t, bob, "save_document", M{"id": doc.At("url").V, "content": "Bob's"})
	eq(t, byBob.At("versionCount").Num(), 2)
	eq(t, byBob.At("updatedBy", "email").Str(), "bob@acme.test")
	versions := call(t, w.Ann, "list_document_versions", M{"id": doc.At("id").V})
	eq(t, versions.At(0, "author", "email").Str(), "ann@acme.test")
	versionID := fmt.Sprint(int64(versions.At(0, "id").Num()))
	version := call(t, w.Ann, "get_document_version", M{"id": versionID})
	eq(t, version.At("content").Str(), "Third")

	back := call(t, w.Ann, "save_document", M{"id": slug, "restoreVersion": versions.At(0, "id").Num()})
	eq(t, back.At("content").Str(), "Third")
	eq(t, back.At("versionCount").Num(), 3)
	first := call(t, w.Ann, "save_document", M{"id": slug, "restoreVersion": versions.At(1, "id").Num()})
	eq(t, first.At("content").Str(), "Hello @bob")
	eq(t, first.At("versionCount").Num(), 4, "a restore keeps the text it replaces, even right after an edit")
	contains(t, fails(t, w.Ann, "save_document", M{"id": slug, "restoreVersion": 1, "content": "x"}), "don't pass")
	fails(t, w.Ann, "save_document", M{"id": slug, "restoreVersion": 999999})
	fails(t, w.Ann, "save_document", M{"title": "New", "team": "ENG", "restoreVersion": 1})

	teamDocs := call(t, w.Ann, "list_documents", M{"team": "ENG"})
	eqList(t, titles(teamDocs), []string{"Design Notes!"})
	_, hasContent := teamDocs.At("nodes", 0).V.(map[string]any)["content"]
	check(t, !hasContent, "contents are left out")
	eqList(t, titles(call(t, w.Ann, "list_documents", M{"issue": "ENG-1"})), []string{"Issue doc"})
	eqList(t, titles(call(t, w.Ann, "list_documents", M{"query": "design"})), []string{"Design Notes!"})
	eq(t, len(titles(call(t, bob, "list_documents", M{}))), 2)
	eq(t, len(titles(call(t, gus, "list_documents", M{}))), 0)
	fails(t, gus, "get_document", M{"id": doc.At("id").V})
	fails(t, gus, "get_document_version", M{"id": versionID})

	call(t, w.Ann, "delete_document", M{"id": doc.At("id").V})
	eqList(t, titles(call(t, w.Ann, "list_documents", M{})), []string{"Issue doc"})
	fails(t, w.Ann, "get_document", M{"id": doc.At("id").V})
	restored := call(t, w.Ann, "restore_document", M{"id": doc.At("id").V})
	eq(t, restored.At("trashed").Bool(), false)
	moved := call(t, w.Ann, "save_document", M{"id": doc.At("id").V, "issue": "ENG-1"})
	check(t, moved.At("team").IsNull())
	eq(t, moved.At("issue", "identifier").Str(), "ENG-1")
}

func TestCustomViewsFilterIssues(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	call(t, w.Ann, "save_issue", M{"title": "Urgent", "priority": 1})
	call(t, w.Ann, "save_issue", M{"title": "Later", "priority": 4})
	view := call(t, w.Ann, "save_custom_view", M{"name": "Urgent work", "filter": M{"priority": M{"eq": 1}}, "shared": true,
		"display": M{"groupBy": "state"}, "team": "ENG"})
	slug := view.At("slugId").Str()
	eq(t, view.At("url").Str(), "http://localhost:3000/acme/view/urgent-work-"+slug)
	eq(t, view.At("model").Str(), "issue")
	eqList(t, identifiers(call(t, bob, "list_issues", M{"customView": "Urgent work"})), []string{"ENG-1"})
	eqList(t, identifiers(call(t, w.Ann, "list_issues", M{"customView": slug})), []string{"ENG-1"})
	bad := fails(t, w.Ann, "save_custom_view", M{"name": "Bad", "filter": M{"colour": M{"eq": "red"}}})
	contains(t, bad, "colour")

	call(t, bob, "save_custom_view", M{"name": "Mine", "filter": M{"assignee": M{"isMe": M{"eq": true}}}})
	eqList(t, names(call(t, bob, "list_custom_views", M{})), []string{"Mine", "Urgent work"})
	eqList(t, names(call(t, bob, "list_custom_views", M{"shared": false})), []string{"Mine"})
	eqList(t, names(call(t, w.Ann, "list_custom_views", M{})), []string{"Urgent work"})
	fails(t, w.Ann, "get_custom_view", M{"id": "Mine"})
	notOwner := fails(t, bob, "save_custom_view", M{"id": view.At("id").V, "name": "Mine now"})
	contains(t, notOwner, "owner")
	updated := call(t, w.Ann, "save_custom_view", M{"id": view.At("id").V, "filter": M{"priority": M{"eq": 4}}})
	eq(t, updated.At("filter", "priority", "eq").Num(), 4)
	eqList(t, identifiers(call(t, w.Ann, "list_issues", M{"customView": slug})), []string{"ENG-2"})
	call(t, w.Ann, "delete_custom_view", M{"id": view.At("id").V})
	fails(t, w.Ann, "get_custom_view", M{"id": view.At("id").V})
}

func TestFavoritesAreAddedOnceAndHideWhatIsGone(t *testing.T) {
	w := world(t)
	call(t, w.Ann, "save_issue", M{"title": "Pinned"})
	first := call(t, w.Ann, "add_favorite", M{"type": "issue", "target": "ENG-1"})
	eq(t, first.At("target", "identifier").Str(), "ENG-1")
	call(t, w.Ann, "add_favorite", M{"type": "team", "target": "ENG", "folder": "Teams"})
	call(t, w.Ann, "add_favorite", M{"type": "user", "target": "me"})
	again := call(t, w.Ann, "add_favorite", M{"type": "issue", "target": "ENG-1", "sortOrder": 5000})
	eq(t, again.At("id").Str(), first.At("id").Str())
	list := call(t, w.Ann, "list_favorites", M{})
	eqList(t, list.At("nodes").Strings("type"), []string{"user", "issue", "team"})
	eq(t, list.At("nodes", 2, "target", "key").Str(), "ENG")
	wrong := fails(t, w.Ann, "add_favorite", M{"type": "board", "target": "x"})
	contains(t, wrong, "customView")

	call(t, w.Ann, "delete_issue", M{"id": "ENG-1"})
	eq(t, call(t, w.Ann, "list_favorites", M{}).At("nodes").Len(), 2)
	call(t, w.Ann, "remove_favorite", M{"id": first.At("id").V})
	call(t, w.Ann, "remove_favorite", M{"id": "ENG", "type": "team"})
	eq(t, call(t, w.Ann, "list_favorites", M{}).At("nodes").Len(), 1)
	fails(t, w.Ann, "remove_favorite", M{"id": first.At("id").V})
}

func TestFavoritesShowWhatTheyPointAt(t *testing.T) {
	w := world(t)
	enableCycles(w)
	call(t, w.Ann, "save_project", M{"name": "Apollo", "setTeams": []any{"ENG"}})
	call(t, w.Ann, "save_initiative", M{"name": "Grow"})
	call(t, w.Ann, "save_document", M{"title": "Runbook", "team": "ENG"})
	call(t, w.Ann, "save_custom_view", M{"name": "Urgent", "filter": M{"priority": M{"eq": 1}}})
	for _, f := range []M{
		{"type": "project", "target": "Apollo"},
		{"type": "initiative", "target": "Grow"},
		{"type": "cycle", "target": "current", "team": "ENG"},
		{"type": "customView", "target": "Urgent"},
		{"type": "document", "target": "Runbook"},
		{"type": "label", "target": "Bug"},
	} {
		call(t, w.Ann, "add_favorite", f)
	}
	list := call(t, w.Ann, "list_favorites", M{})
	eqList(t, list.At("nodes").Strings("type"), []string{"project", "initiative", "cycle", "customView", "document", "label"})
	eq(t, list.At("nodes", 2, "target", "team", "key").Str(), "ENG")
	eq(t, list.At("nodes", 4, "target", "title").Str(), "Runbook")
}

func TestTemplatesFillInIssuesAndDocuments(t *testing.T) {
	w := world(t)
	gus := w.member("gus@acme.test", "Gus Guest", "guest")
	template := call(t, w.Ann, "save_template", M{"name": "Bug report", "type": "issue", "team": "ENG", "data": M{
		"description": "## Steps", "priority": 2, "labels": []any{"Bug"},
		"subIssues": []any{M{"title": "Reproduce"}, M{"title": "Fix", "priority": 1}},
	}})
	eq(t, template.At("team", "key").Str(), "ENG")
	unknown := fails(t, w.Ann, "save_template", M{"name": "x", "type": "issue", "data": M{"titel": "x"}})
	contains(t, unknown, "titel", "subIssues")
	badSub := fails(t, w.Ann, "save_template", M{"name": "x", "type": "issue", "data": M{"subIssues": []any{M{"titel": "x"}}}})
	contains(t, badSub, "titel")
	badMilestone := fails(t, w.Ann, "save_template", M{"name": "x", "type": "project", "data": M{"milestones": []any{M{"name": "M1", "due": "x"}}}})
	contains(t, badMilestone, "targetDate")
	fails(t, gus, "save_template", M{"name": "x", "type": "document", "data": M{}})

	listed := call(t, w.Ann, "list_templates", M{"type": "issue"})
	eq(t, listed.At("nodes").Len(), 1)
	_, hasData := listed.At("nodes", 0).V.(map[string]any)["data"]
	check(t, !hasData, "data is left out of lists")
	eq(t, call(t, gus, "list_templates", M{}).At("nodes").Len(), 0)
	read := call(t, w.Ann, "get_template", M{"id": "bug report"})
	eq(t, read.At("data", "subIssues").Len(), 2)

	issue := call(t, w.Ann, "save_issue", M{"template": "Bug report", "title": "Bug: crash"})
	eq(t, issue.At("priority").Num(), 2)
	eq(t, issue.At("labels", 0, "name").Str(), "Bug")
	eq(t, issue.At("description").Str(), "## Steps")
	eq(t, issue.At("team", "key").Str(), "ENG")
	parent := call(t, w.Ann, "get_issue", M{"id": issue.At("identifier").V})
	eqList(t, parent.At("children").Strings("title"), []string{"Reproduce", "Fix"})

	call(t, w.Ann, "save_template", M{"name": "RFC", "type": "document", "data": M{"title": "RFC", "content": "## Problem"}})
	doc := call(t, w.Ann, "save_document", M{"template": "RFC", "team": "ENG"})
	eq(t, doc.At("title").Str(), "RFC")
	eq(t, doc.At("content").Str(), "## Problem")
	call(t, w.Ann, "delete_template", M{"id": template.At("id").V})
	fails(t, w.Ann, "get_template", M{"id": "Bug report"})
}

func TestRecurringTemplatesCreateIssuesOnSchedule(t *testing.T) {
	w := world(t)
	template := call(t, w.Ann, "save_template", M{"name": "Weekly triage", "type": "issue", "team": "ENG", "data": M{"priority": 3},
		"recurrence": M{"every": 1, "unit": "week", "startsAt": "-PT1M"}})
	eq(t, template.At("recurrence", "unit").Str(), "week")
	for range 2 {
		if err := w.B.RunRecurringTemplates(context.Background()); err != nil {
			t.Fatalf("recurring templates: %v", err)
		}
	}
	issues := call(t, w.Ann, "list_issues", M{})
	eqList(t, identifiers(issues), []string{"ENG-1"}, "one issue per due run")
	eq(t, issues.At("nodes", 0, "title").Str(), "Weekly triage")
	next := call(t, w.Ann, "get_template", M{"id": "Weekly triage"})
	nextRun, err := time.Parse(time.RFC3339Nano, next.At("nextRunAt").Str())
	check(t, err == nil && nextRun.After(time.Now()), "the next run is ahead: ", next.At("nextRunAt").Str())
	teamless := fails(t, w.Ann, "save_template", M{"name": "x", "type": "issue", "data": M{}, "recurrence": M{"every": 1, "unit": "day"}})
	contains(t, teamless, "team")
	stopped := call(t, w.Ann, "save_template", M{"id": template.At("id").V, "recurrence": nil})
	check(t, stopped.At("nextRunAt").IsNull())
}
