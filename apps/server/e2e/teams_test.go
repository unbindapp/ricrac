package e2e

import (
	"slices"
	"testing"
	"time"
)

func TestCreatesTeamsWithSettings(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	gus := w.member("gus@else.test", "Gus", "guest")

	team := call(t, bob, "save_team", M{"name": "Mobile Platform", "description": "Apps", "color": "#ABC"})
	eq(t, team.At("key").Str(), "MP")
	eq(t, team.At("description").Str(), "Apps")
	eq(t, team.At("color").Str(), "#abc")
	eq(t, team.At("members", 0, "email").Str(), "bob@acme.test")
	eq(t, team.At("members", 0, "owner").Bool(), true)
	eq(t, team.At("defaultIssueState", "name").Str(), "Backlog")
	eq(t, team.At("autoCloseState", "name").Str(), "Canceled")
	eqList(t, team.At("states").Strings("name"), []string{"Backlog", "Todo", "In Progress", "In Review", "Done", "Canceled", "Duplicate"})
	eq(t, team.At("url").Str(), "http://localhost:3000/acme/team/MP")

	contains(t, fails(t, gus, "save_team", M{"name": "Guests"}), "Guests can't")
	contains(t, fails(t, bob, "save_team", M{"name": "Other", "key": "eng"}), "ENG is taken by the team Engineering")
	contains(t, fails(t, bob, "save_team", M{"name": "Bad", "key": "1AB"}), "team key")
	contains(t, fails(t, bob, "save_team", M{"name": "engineering", "key": "EN2"}), "already")

	updated := call(t, bob, "save_team", M{
		"id": "MP", "estimateType": "fibonacci", "defaultEstimate": 3, "cyclesEnabled": true,
		"cycleDurationWeeks": 1, "cycleStartDay": 0, "autoArchiveMonths": nil, "autoCloseMonths": 3,
		"defaultIssueState": "Todo", "timezone": "America/New_York", "parent": "ENG", "icon": "Phone",
	})
	eq(t, updated.At("estimateType").Str(), "fibonacci")
	eq(t, updated.At("allowedEstimates").JSON(), "[1,2,3,5,8]")
	eq(t, updated.At("defaultEstimate").Num(), 3)
	eq(t, updated.At("cyclesEnabled").Bool(), true)
	eq(t, updated.At("cycleDurationWeeks").Num(), 1)
	check(t, updated.At("autoArchiveMonths").IsNull())
	eq(t, updated.At("autoCloseMonths").Num(), 3)
	eq(t, updated.At("defaultIssueState", "name").Str(), "Todo")
	eq(t, updated.At("parent", "key").Str(), "ENG")
	eq(t, updated.At("description").Str(), "Apps", "omitted fields stay")
	eq(t, call(t, bob, "get_team", M{"id": "ENG"}).At("subTeams", 0, "key").Str(), "MP")
	eq(t, call(t, bob, "save_issue", M{"team": "MP", "title": "x"}).At("state", "name").Str(), "Todo")

	for _, c := range []struct {
		args  M
		error string
	}{
		{M{"id": "MP", "cycleDurationWeeks": 9}, "cycleDurationWeeks is 1 to 8"},
		{M{"id": "MP", "defaultEstimate": 4}, "isn't on the fibonacci scale"},
		{M{"id": "MP", "autoCloseMonths": 2}, "1, 3, 6, 9 or 12"},
		{M{"id": "MP", "estimateType": "huge"}, "estimateType"},
		{M{"id": "MP", "defaultIssueState": "Done"}, "backlog or unstarted"},
		{M{"id": "MP", "autoCloseState": "Done"}, "canceled"},
		{M{"id": "ENG", "parent": "MP"}, "own sub-teams"},
		{M{"id": "MP", "timezone": "Nowhere"}, "timezone"},
	} {
		contains(t, fails(t, bob, "save_team", c.args), c.error)
	}
	listed := call(t, bob, "list_teams", M{"mine": true})
	eqList(t, listed.At("nodes").Strings("key"), []string{"MP"})
	eq(t, listed.At("nodes", 0, "isMember").Bool(), true)
	eq(t, listed.At("nodes", 0, "issueCount").Num(), 1)
	eqList(t, call(t, bob, "list_teams", M{"query": "eng"}).At("nodes").Strings("key"), []string{"ENG"})
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
}

func TestPrivateTeamsAreHiddenFromOthers(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	ada := w.member("ada@acme.test", "Ada Admin", "admin")
	call(t, w.Ann, "save_team", M{"name": "Secret", "key": "SEC", "private": true})
	call(t, w.Ann, "save_issue", M{"team": "SEC", "title": "Hidden"})

	eqList(t, call(t, bob, "list_teams", M{}).At("nodes").Strings("key"), []string{"ENG"})
	contains(t, fails(t, bob, "get_team", M{"id": "SEC"}), "No team")
	contains(t, fails(t, bob, "get_issue", M{"id": "SEC-1"}), "No issue")
	contains(t, fails(t, bob, "join_team", M{"id": "SEC"}), "No team")
	eq(t, len(identifiers(call(t, bob, "list_issues", M{}))), 0)
	eq(t, call(t, bob, "get_workspace", M{}).At("teams").Len(), 1)

	joined := call(t, ada, "join_team", M{"id": "SEC"})
	eq(t, joined.At("isMember").Bool(), true)
	call(t, w.Ann, "save_team", M{"id": "SEC", "addMembers": []any{"bob"}})
	eq(t, call(t, bob, "get_issue", M{"id": "SEC-1"}).At("title").Str(), "Hidden")
	call(t, bob, "save_team", M{"id": "SEC", "removeMembers": []any{"ada", "me"}})
	contains(t, fails(t, w.Ann, "leave_team", M{"id": "SEC"}), "last member")
	contains(t, fails(t, w.Ann, "save_team", M{"id": "SEC", "removeMembers": []any{"ann"}}), "at least one member")

	left := call(t, w.Ann, "leave_team", M{"id": "ENG"})
	eq(t, left.At("left", "key").Str(), "ENG")
	contains(t, fails(t, w.Ann, "leave_team", M{"id": "ENG"}), "aren't a member")
}

func TestArchivingATeamKeepsItsIssuesReadable(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	call(t, w.Ann, "save_issue", M{"title": "Old work"})
	contains(t, fails(t, bob, "delete_team", M{"id": "ENG"}), "owners")
	eq(t, call(t, w.Ann, "delete_team", M{"id": "ENG"}).At("key").Str(), "ENG")
	eq(t, call(t, w.Ann, "list_teams", M{}).At("nodes").Len(), 0)
	eqList(t, call(t, w.Ann, "list_teams", M{"includeArchived": true}).At("nodes").Strings("key"), []string{"ENG"})
	eq(t, call(t, w.Ann, "get_issue", M{"id": "ENG-1"}).At("title").Str(), "Old work")
	reused := call(t, w.Ann, "save_team", M{"name": "Engineering", "key": "ENG"})
	eq(t, reused.At("key").Str(), "ENG", "the key and name are free again")
	log := call(t, w.Ann, "get_audit_log", M{"type": "team.archived"})
	eq(t, log.At("nodes", 0, "metadata", "key").Str(), "ENG")
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
