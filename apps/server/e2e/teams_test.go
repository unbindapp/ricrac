package e2e

import (
	"slices"
	"testing"
	"time"
)

func TestTheWorkspacesTeamTakesSettings(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	gus := w.member("gus@else.test", "Gus", "guest")

	contains(t, fails(t, gus, "save_team", M{"id": "ENG", "description": "Apps"}), "Guests can't")
	contains(t, fails(t, bob, "save_team", M{"id": "ENG", "key": "1AB"}), "team key")
	team := call(t, bob, "save_team", M{"id": "ENG", "description": "Apps", "color": "#ABC"})
	eq(t, team.At("description").Str(), "Apps")
	eq(t, team.At("color").Str(), "#abc")
	eq(t, team.At("defaultIssueState", "name").Str(), "Backlog")
	eq(t, team.At("autoCloseState", "name").Str(), "Canceled")
	eqList(t, team.At("states").Strings("name"), []string{"Backlog", "Todo", "In Progress", "In Review", "Done", "Canceled", "Duplicate"})
	eq(t, team.At("url").Str(), "http://localhost:3000/acme/team/ENG")

	updated := call(t, bob, "save_team", M{
		"id": "ENG", "estimateType": "fibonacci", "defaultEstimate": 3, "cyclesEnabled": true,
		"cycleDurationWeeks": 1, "cycleStartDay": 0, "autoArchiveMonths": nil, "autoCloseMonths": 3,
		"defaultIssueState": "Todo", "timezone": "America/New_York", "icon": "Phone",
	})
	eq(t, updated.At("estimateType").Str(), "fibonacci")
	eq(t, updated.At("allowedEstimates").JSON(), "[1,2,3,5,8]")
	eq(t, updated.At("defaultEstimate").Num(), 3)
	eq(t, updated.At("cyclesEnabled").Bool(), true)
	eq(t, updated.At("cycleDurationWeeks").Num(), 1)
	check(t, updated.At("autoArchiveMonths").IsNull())
	eq(t, updated.At("autoCloseMonths").Num(), 3)
	eq(t, updated.At("defaultIssueState", "name").Str(), "Todo")
	eq(t, updated.At("description").Str(), "Apps", "omitted fields stay")
	eq(t, call(t, bob, "save_issue", M{"title": "x"}).At("state", "name").Str(), "Todo")

	for _, c := range []struct {
		args  M
		error string
	}{
		{M{"id": "ENG", "cycleDurationWeeks": 9}, "cycleDurationWeeks is 1 to 8"},
		{M{"id": "ENG", "defaultEstimate": 4}, "isn't on the fibonacci scale"},
		{M{"id": "ENG", "autoCloseMonths": 2}, "1, 3, 6, 9 or 12"},
		{M{"id": "ENG", "estimateType": "huge"}, "estimateType"},
		{M{"id": "ENG", "defaultIssueState": "Done"}, "backlog or unstarted"},
		{M{"id": "ENG", "autoCloseState": "Done"}, "canceled"},
		{M{"id": "ENG", "parent": "ENG"}, "own sub-teams"},
		{M{"id": "ENG", "timezone": "Nowhere"}, "timezone"},
	} {
		contains(t, fails(t, bob, "save_team", c.args), c.error)
	}
	listed := call(t, bob, "list_teams", M{"mine": true})
	eqList(t, listed.At("nodes").Strings("key"), []string{"ENG"})
	eq(t, listed.At("nodes", 0, "isMember").Bool(), true)
	eq(t, listed.At("nodes", 0, "issueCount").Num(), 1)
}

func TestAWorkspaceHasOneTeam(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	gus := w.member("gus@else.test", "Gus", "guest")

	created := fails(t, w.Ann, "save_team", M{"name": "Design", "key": "DES"})
	contains(t, created, "ENG (Engineering)", `id "ENG"`)
	contains(t, fails(t, w.Ann, "delete_team", M{"id": "ENG"}), "can't be deleted")
	contains(t, fails(t, bob, "leave_team", M{"id": "ENG"}), "leave the workspace instead")
	eqList(t, call(t, w.Ann, "list_teams", M{"includeArchived": true}).At("nodes").Strings("key"), []string{"ENG"})

	eqList(t, call(t, w.Ann, "get_team", M{"id": "ENG"}).At("members").Strings("email"), []string{"ann@acme.test", "bob@acme.test"})
	joined := call(t, bob, "join_team", M{"id": "ENG"})
	eq(t, joined.At("key").Str(), "ENG")
	eq(t, joined.At("isMember").Bool(), true)
	contains(t, fails(t, gus, "join_team", M{"id": "ENG"}), "Guests can't")
	contains(t, fails(t, w.Ann, "save_team", M{"id": "ENG", "removeMembers": []any{"bob"}}), "remove Bob Jones from the workspace")

	call(t, w.Ann, "save_issue", M{"title": "Not for guests yet"})
	contains(t, fails(t, gus, "get_issue", M{"id": "ENG-1"}), "No issue")
	call(t, w.Ann, "save_team", M{"id": "ENG", "addMembers": []any{"gus"}})
	eq(t, call(t, gus, "get_issue", M{"id": "ENG-1"}).At("title").Str(), "Not for guests yet")
	call(t, w.Ann, "save_team", M{"id": "ENG", "removeMembers": []any{"gus"}})
	contains(t, fails(t, gus, "get_issue", M{"id": "ENG-1"}), "No issue")

	call(t, w.Ann, "update_user", M{"id": "gus", "role": "member"})
	eqList(t, call(t, w.Ann, "get_team", M{"id": "ENG"}).At("members").Strings("email"),
		[]string{"ann@acme.test", "bob@acme.test", "gus@else.test"}, "members join the workspace's team")
}

func TestChangingTheKeyKeepsOldIdentifiersWorking(t *testing.T) {
	w := world(t)
	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "First"})
	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Second"})
	team := call(t, w.Ann, "save_team", M{"id": "ENG", "key": "core", "name": "Core"})
	eq(t, team.At("key").Str(), "CORE")
	eq(t, team.At("name").Str(), "Core")
	issue := call(t, w.Ann, "get_issue", M{"id": "ENG-2"})
	eq(t, issue.At("identifier").Str(), "CORE-2")
	eqList(t, issue.At("previousIdentifiers").Strings(), []string{"ENG-2"})
	call(t, w.Ann, "save_team", M{"id": "CORE", "key": "CO"})
	issue = call(t, w.Ann, "get_issue", M{"id": "ENG-1"})
	eq(t, issue.At("identifier").Str(), "CO-1")
	eqList(t, issue.At("previousIdentifiers").Strings(), []string{"ENG-1", "CORE-1"})
	eq(t, call(t, w.Ann, "save_issue", M{"title": "Third"}).At("identifier").Str(), "CO-3")
	eq(t, call(t, w.Ann, "get_team", M{"id": "ENG"}).At("key").Str(), "CO", "earlier keys still name the team")
	eq(t, call(t, w.Ann, "get_team", M{"id": "engineering"}).At("key").Str(), "CO", "and earlier names")
	eq(t, len(identifiers(call(t, w.Ann, "list_issues", M{"team": "core"}))), 3)
	eq(t, call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Fourth"}).At("identifier").Str(), "CO-4")
	back := call(t, w.Ann, "save_team", M{"id": "CO", "key": "ENG"})
	eq(t, back.At("key").Str(), "ENG")
	eq(t, call(t, w.Ann, "get_team", M{"id": "CORE"}).At("key").Str(), "ENG")
}

func TestTriageRoutesIssuesToTheResponsiblePerson(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	w.member("gus@else.test", "Gus", "guest")
	contains(t, fails(t, w.Ann, "save_team", M{"id": "ENG", "triageResponsible": []any{"gus@else.test"}}), "can't see this team")

	team := call(t, w.Ann, "save_team", M{
		"id": "ENG", "triageEnabled": true, "triageAction": "assign", "triageResponsible": []any{"bob", "me"},
		"triageRotationWeeks": 1,
	})
	eq(t, team.At("states", 0, "type").Str(), "triage")
	eq(t, team.At("triageRotationStart").Str(), time.Now().UTC().Format(time.DateOnly))
	eq(t, team.At("triage", "responsible", 0, "email").Str(), "bob@acme.test", "Bob has the first shift")
	issue := call(t, w.Ann, "save_issue", M{"title": "Report", "state": "triage"})
	eq(t, issue.At("assignee", "email").Str(), "bob@acme.test")

	duty := call(t, bob, "get_triage_responsibility", M{"id": "ENG", "limit": 4})
	eq(t, duty.At("action").Str(), "assign")
	eq(t, duty.At("rotation", "weeks").Num(), 1)
	var shifts []string
	for _, s := range duty.At("upcomingShifts").List() {
		shifts = append(shifts, s.At("user", "email").Str())
	}
	eqList(t, shifts, []string{"bob@acme.test", "ann@acme.test", "bob@acme.test", "ann@acme.test"})
	eq(t, duty.At("upcomingShifts", 0, "endsAt").Str(), duty.At("upcomingShifts", 1, "startsAt").Str())
	mine := call(t, bob, "get_triage_responsibility", M{"id": "ENG", "user": "ann", "limit": 2})
	eq(t, mine.At("upcomingShifts", 0, "startsAt").Str(), duty.At("upcomingShifts", 1, "startsAt").Str())
	eq(t, mine.At("upcomingShifts", 1, "startsAt").Str(), duty.At("upcomingShifts", 3, "startsAt").Str())

	off := call(t, w.Ann, "save_team", M{"id": "ENG", "triageEnabled": false})
	check(t, off.At("states", 0, "type").Str() != "triage")
	eq(t, call(t, w.Ann, "get_issue", M{"id": "ENG-1"}).At("state", "name").Str(), "Backlog")
	on := call(t, w.Ann, "save_team", M{"id": "ENG", "triageEnabled": true})
	eq(t, on.At("states", 0, "name").Str(), "Triage")
	statuses := call(t, w.Ann, "list_issue_statuses", M{"team": "ENG", "includeArchived": true})
	triage := slices.DeleteFunc(statuses.At("nodes").Strings("type"), func(kind string) bool { return kind != "triage" })
	eq(t, len(triage), 1, "the old Triage comes back")
}

func TestStatusesAreCreatedChangedAndArchived(t *testing.T) {
	w := world(t)
	status := call(t, w.Ann, "save_issue_status", M{"team": "ENG", "name": "QA", "type": "started", "description": "Testing"})
	eq(t, status.At("color").Str(), "#f2c94c", "the type's color")
	eq(t, status.At("team", "key").Str(), "ENG")
	eqList(t, call(t, w.Ann, "list_issue_statuses", M{"team": "ENG"}).At("nodes").Strings("name"),
		[]string{"Backlog", "Todo", "In Progress", "In Review", "QA", "Done", "Canceled", "Duplicate"})
	contains(t, fails(t, w.Ann, "save_issue_status", M{"team": "ENG", "name": "qa", "type": "started"}), "already")
	contains(t, fails(t, w.Ann, "save_issue_status", M{"team": "ENG", "name": "X", "type": "doing"}), "status type")

	call(t, w.Ann, "save_issue", M{"title": "Check", "state": "QA"})
	call(t, w.Ann, "save_issue", M{"title": "Check more", "state": "QA"})
	renamed := call(t, w.Ann, "save_issue_status", M{"id": "QA", "team": "ENG", "name": "Testing", "description": nil})
	eq(t, renamed.At("name").Str(), "Testing")
	check(t, renamed.At("description").IsNull())
	eq(t, renamed.At("issueCount").Num(), 2)
	id := status.At("id").Str()
	contains(t, fails(t, w.Ann, "save_issue_status", M{"id": id, "type": "completed"}), "2 issues")

	contains(t, fails(t, w.Ann, "delete_issue_status", M{"id": id}), "moveIssuesTo")
	deleted := call(t, w.Ann, "delete_issue_status", M{"id": id, "moveIssuesTo": "Done"})
	check(t, deleted.At("archivedAt").IsString())
	eq(t, deleted.At("movedIssues").Num(), 2)
	moved := call(t, w.Ann, "get_issue", M{"id": "ENG-1", "includeHistory": true})
	eq(t, moved.At("state", "name").Str(), "Done")
	check(t, moved.At("completedAt").IsString(), "moved like any state change")
	history := moved.At("history").List()
	eq(t, history[len(history)-1].At("changes", "state", "to", "name").Str(), "Done")
	eq(t, call(t, w.Ann, "get_issue_status", M{"id": id}).At("name").Str(), "Testing")

	contains(t, fails(t, w.Ann, "delete_issue_status", M{"id": "Backlog", "team": "ENG"}), "default status")
	call(t, w.Ann, "delete_issue_status", M{"id": "Backlog", "team": "ENG", "moveIssuesTo": "Todo"})
	team := call(t, w.Ann, "get_team", M{"id": "ENG"})
	eq(t, team.At("defaultIssueState", "name").Str(), "Todo", "the default moves to the replacement")
	contains(t, fails(t, w.Ann, "delete_issue_status", M{"id": "Done", "team": "ENG", "moveIssuesTo": "Todo"}), "completed status")
	call(t, w.Ann, "delete_issue_status", M{"id": "Duplicate", "team": "ENG"})
	contains(t, fails(t, w.Ann, "delete_issue_status", M{"id": "Canceled", "team": "ENG", "moveIssuesTo": "Todo"}), "canceled")
}

func TestLabelGroupsRetiringAndDeleting(t *testing.T) {
	w := world(t)
	group := call(t, w.Ann, "save_issue_label", M{"name": "Size", "isGroup": true})
	eq(t, group.At("groupType").Str(), "singleSelect")
	call(t, w.Ann, "save_issue_label", M{"name": "Small", "parent": "Size", "color": "#00ff00"})
	call(t, w.Ann, "save_issue_label", M{"name": "Large", "parent": "Size"})
	teamLabel := call(t, w.Ann, "save_issue_label", M{"name": "Backend", "team": "ENG", "parent": "Size"})
	eq(t, teamLabel.At("team", "key").Str(), "ENG")
	eq(t, teamLabel.At("parent", "name").Str(), "Size")
	contains(t, fails(t, w.Ann, "save_issue_label", M{"name": "small", "parent": "Size"}), "already")
	contains(t, fails(t, w.Ann, "save_issue_label", M{"name": "X", "parent": "Bug"}), "isn't a group")
	contains(t, fails(t, w.Ann, "save_issue_label", M{"id": "Size", "parent": "Size"}), "own group")
	contains(t, fails(t, w.Ann, "save_issue_label", M{"id": "Bug", "groupType": "multiSelect"}), "groups")

	call(t, w.Ann, "save_issue", M{"title": "Sized", "labels": []any{"Small", "Bug"}})
	issue := call(t, w.Ann, "save_issue", M{"id": "ENG-1", "addLabels": []any{"Size/Large"}})
	eqList(t, issue.At("labels").Strings("name"), []string{"Bug", "Large"}, "Large replaces Small in a single-select group")
	contains(t, fails(t, w.Ann, "save_issue", M{"id": "ENG-1", "addLabels": []any{"Size"}}), "No label")
	contains(t, fails(t, w.Ann, "save_issue_label", M{"id": "Bug", "isGroup": true}), "on issues")

	listed := call(t, w.Ann, "list_issue_labels", M{"team": "ENG"})
	eqList(t, listed.At("nodes").Strings("name"), []string{"Bug", "Feature", "Improvement", "Size", "Backend", "Large", "Small"})
	eq(t, listed.At("nodes", 3, "issueCount").Num(), 1)

	retired := call(t, w.Ann, "retire_issue_label", M{"id": "Bug"})
	check(t, retired.At("retiredAt").IsString())
	eq(t, call(t, w.Ann, "get_issue", M{"id": "ENG-1"}).At("labels").Len(), 2, "retired labels stay on issues")
	contains(t, fails(t, w.Ann, "save_issue", M{"title": "New", "labels": []any{"Bug"}}), "No label")
	check(t, !slices.Contains(call(t, w.Ann, "list_issue_labels", M{}).At("nodes").Strings("name"), "Bug"))
	call(t, w.Ann, "restore_issue_label", M{"id": "Bug"})
	call(t, w.Ann, "save_issue", M{"title": "New", "labels": []any{"Bug"}})

	deleted := call(t, w.Ann, "delete_issue_label", M{"id": "Size"})
	eq(t, deleted.At("removedFromIssues").Num(), 1)
	eq(t, call(t, w.Ann, "get_issue", M{"id": "ENG-1"}).At("labels").Len(), 1, "the group's labels went with it")
	contains(t, fails(t, w.Ann, "get_issue_label", M{"id": "Large"}), "No label")
}
