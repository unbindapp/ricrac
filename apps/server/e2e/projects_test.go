package e2e

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func notificationKinds(w *World, user uuid.UUID) []string {
	return w.strings("SELECT type FROM notifications WHERE user_id = $1 ORDER BY created_at", user)
}

func TestCreatesProjectsWithMilestonesAndIssueProgress(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	call(t, w.Ann, "save_project_label", M{"name": "Q3", "color": "#ff0000"})
	project := call(t, w.Ann, "save_project", M{"name": "Launch v2!", "setTeams": []any{"ENG"}, "lead": "bob", "labels": []any{"Q3"},
		"summary": "Ship it", "startDate": "2030-01-01", "targetDate": "2030-03-31", "targetDateResolution": "quarter", "priority": 2})
	slug := project.At("slugId").Str()
	eq(t, project.At("url").Str(), "http://localhost:3000/acme/project/launch-v2-"+slug)
	eq(t, project.At("status", "name").Str(), "Backlog")
	eq(t, project.At("lead", "email").Str(), "bob@acme.test")
	eq(t, project.At("teams", 0, "key").Str(), "ENG")
	eq(t, project.At("labels", 0, "name").Str(), "Q3")
	eq(t, project.At("targetDateResolution").Str(), "quarter")
	eq(t, project.At("priorityLabel").Str(), "High")
	eq[any](t, project.At("progress").V, 0.0)
	eqList(t, notificationKinds(w, bob.User), []string{"projectAssigned"})
	found := call(t, w.Ann, "get_project", M{"id": project.At("url").Str()})
	eq(t, found.At("id").Str(), project.At("id").Str(), "found by link")

	call(t, w.Ann, "save_milestone", M{"project": "Launch v2!", "name": "Beta", "targetDate": "2030-02-01"})
	call(t, w.Ann, "save_milestone", M{"project": "Launch v2!", "name": "GA", "targetDate": "2030-03-01"})
	dup := fails(t, w.Ann, "save_milestone", M{"project": "Launch v2!", "name": "beta"})
	contains(t, dup, "already")
	call(t, w.Ann, "save_issue", M{"title": "A", "project": "Launch v2!", "milestone": "Beta", "state": "Done"})
	call(t, w.Ann, "save_issue", M{"title": "B", "project": "Launch v2!", "milestone": "Beta"})
	call(t, w.Ann, "save_issue", M{"title": "C", "project": "Launch v2!", "state": "Canceled"})
	found = call(t, w.Ann, "get_project", M{"id": "launch v2!"})
	eq(t, found.At("issueCount").Num(), 3)
	eq(t, found.At("completedIssueCount").Num(), 1)
	eq(t, found.At("progress").Num(), 0.5, "canceled issues don't count")
	eq(t, found.At("issueCountByStateType", "completed").Num(), 1)
	eq(t, found.At("milestones", 0, "name").Str(), "Beta")
	eq(t, found.At("milestones", 0, "progress").Num(), 0.5)
	eq[any](t, found.At("milestones", 1, "issueCount").V, 0.0)
	milestones := call(t, w.Ann, "list_milestones", M{"project": slug})
	eqList(t, names(milestones), []string{"Beta", "GA"})
	beta := call(t, w.Ann, "get_milestone", M{"id": "Beta", "project": slug})
	eq(t, beta.At("issues").Len(), 2)
	call(t, w.Ann, "delete_milestone", M{"id": beta.At("id").Str()})
	issue := call(t, w.Ann, "get_issue", M{"id": "ENG-1"})
	check(t, issue.At("projectMilestone").IsNull() && issue.At("project", "name").Str() == "Launch v2!")

	noTeam := fails(t, w.Ann, "save_project", M{"name": "Nowhere"})
	contains(t, noTeam, "team")
	long := fails(t, w.Ann, "save_project", M{"id": slug, "summary": strings.Repeat("x", 256)})
	contains(t, long, "255")
	last := fails(t, w.Ann, "save_project", M{"id": slug, "removeTeams": []any{"ENG"}})
	contains(t, last, "at least one team")
}

func TestStatusChangesRelationsLinksAndLists(t *testing.T) {
	w := world(t)
	call(t, w.Ann, "save_project", M{"name": "Alpha", "addTeams": []any{"ENG"}})
	call(t, w.Ann, "save_project", M{"name": "Beta", "addTeams": []any{"ENG"}, "lead": "me"})
	started := call(t, w.Ann, "save_project", M{"id": "Alpha", "state": "started"})
	eq(t, started.At("status", "name").Str(), "In Progress")
	check(t, started.At("startedAt").IsString() && started.At("completedAt").IsNull())
	done := call(t, w.Ann, "save_project", M{"id": "Alpha", "state": "Completed"})
	check(t, done.At("startedAt").IsString() && done.At("completedAt").IsString())
	reopened := call(t, w.Ann, "save_project", M{"id": "Alpha", "state": "planned"})
	check(t, reopened.At("startedAt").IsNull() && reopened.At("completedAt").IsNull())

	alpha := call(t, w.Ann, "save_project", M{"id": "Alpha", "blocks": []any{"Beta"},
		"links": []any{M{"url": "https://x.dev/spec", "title": "Spec"}}})
	eq(t, alpha.At("relations", "blocks", 0, "name").Str(), "Beta")
	eq(t, alpha.At("links", 0, "label").Str(), "Spec")
	beta := call(t, w.Ann, "get_project", M{"id": "Beta"})
	eq(t, beta.At("relations", "blockedBy", 0, "name").Str(), "Alpha")
	own := fails(t, w.Ann, "save_project", M{"id": "Alpha", "relatedTo": []any{"Alpha"}})
	contains(t, own, "itself")
	alpha = call(t, w.Ann, "save_project", M{"id": "Alpha", "links": []any{M{"url": "https://x.dev/spec", "title": "The spec"}},
		"removeBlocks": []any{"Beta"}})
	eq(t, alpha.At("links").Len(), 1)
	eq(t, alpha.At("links", 0, "label").Str(), "The spec")
	eq(t, alpha.At("relations", "blocks").Len(), 0)
	alpha = call(t, w.Ann, "save_project", M{"id": "Alpha", "removeLinks": []any{"https://x.dev/spec"}})
	eq(t, alpha.At("links").Len(), 0)

	eqList(t, names(call(t, w.Ann, "list_projects", M{"lead": "me"})), []string{"Beta"})
	eqList(t, names(call(t, w.Ann, "list_projects", M{"status": "planned"})), []string{"Alpha"})
	eqList(t, names(call(t, w.Ann, "list_projects", M{"query": "alp"})), []string{"Alpha"})
	listed := call(t, w.Ann, "list_projects", M{"orderBy": "manual"})
	eqList(t, names(listed), []string{"Alpha", "Beta"})
	_, hasDescription := listed.At("nodes", 0).V.(map[string]any)["description"]
	check(t, !hasDescription, "descriptions are left out of lists")
	w.exec(`INSERT INTO custom_views (workspace_id, owner_id, slug_id, name, model, filter)
		VALUES ($1, $2, 'aaaabbbbcccc', 'Mine', 'project', '{"lead": {"isMe": {"eq": true}}, "status": {"type": {"in": ["backlog"]}}}')`,
		w.Workspace, w.Ann.User)
	eqList(t, names(call(t, w.Ann, "list_projects", M{"customView": "Mine"})), []string{"Beta"})
	filtered := call(t, w.Ann, "list_projects", M{"filter": M{"or": []any{M{"name": "alpha"}, M{"priority": M{"gt": 2}}}}})
	eqList(t, names(filtered), []string{"Alpha"})
	bad := fails(t, w.Ann, "list_projects", M{"filter": M{"colour": "red"}})
	contains(t, bad, "colour")
}

func TestStatusUpdatesSetHealthAndNotify(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	cat := w.member("cat@acme.test", "Cat Lee", "member")
	call(t, w.Ann, "save_project", M{"name": "Apollo", "setTeams": []any{"ENG"}, "addMembers": []any{"bob"}})
	first := call(t, w.Ann, "save_status_update", M{"project": "Apollo", "body": "Slipping, cc @cat", "health": "atRisk"})
	eq(t, first.At("type").Str(), "project")
	eq(t, first.At("user", "email").Str(), "ann@acme.test")
	project := call(t, w.Ann, "get_project", M{"id": "Apollo"})
	eq(t, project.At("health").Str(), "atRisk")
	eq(t, project.At("latestUpdate", "id").Str(), first.At("id").Str())
	eqList(t, notificationKinds(w, bob.User), []string{"projectUpdateCreated"})
	eqList(t, notificationKinds(w, cat.User), []string{"projectUpdateMention"})

	second := call(t, w.Ann, "save_status_update", M{"project": "Apollo", "body": "Still slipping"})
	eq(t, second.At("health").Str(), "atRisk", "the previous update's health")
	third := call(t, w.Ann, "save_status_update", M{"project": "Apollo", "body": "Back on track", "health": "onTrack"})
	listed := call(t, bob, "get_status_updates", M{"project": "Apollo"})
	eqList(t, listed.At("nodes").Strings("id"), []string{third.At("id").Str(), second.At("id").Str(), first.At("id").Str()}, "newest first")
	notAuthor := fails(t, bob, "save_status_update", M{"id": third.At("id").Str(), "body": "Mine now"})
	contains(t, notAuthor, "author")
	edited := call(t, w.Ann, "save_status_update", M{"id": third.At("id").Str(), "health": "offTrack"})
	check(t, edited.At("editedAt").IsString())
	eq(t, call(t, w.Ann, "get_project", M{"id": "Apollo"}).At("health").Str(), "offTrack")
	fails(t, bob, "delete_status_update", M{"id": third.At("id").Str()})
	call(t, w.Ann, "delete_status_update", M{"id": third.At("id").Str()})
	eqList(t, names(call(t, w.Ann, "list_projects", M{"health": "atRisk"})), []string{"Apollo"},
		"health goes back to the newest remaining update's")
}

func TestInitiativesGroupProjectsWithProgress(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	call(t, w.Ann, "save_project", M{"name": "One", "setTeams": []any{"ENG"}})
	call(t, w.Ann, "save_project", M{"name": "Two", "setTeams": []any{"ENG"}})
	call(t, w.Ann, "save_issue", M{"title": "Done", "project": "One", "state": "Done"})
	call(t, w.Ann, "save_issue", M{"title": "Open", "project": "Two"})
	goal := call(t, w.Ann, "save_initiative", M{"name": "Grow 2030", "owner": "bob", "setProjects": []any{"One", "Two"},
		"targetDate": "2030-12-31", "links": []any{M{"url": "https://x.dev/okr", "title": "OKRs"}}})
	slug := goal.At("slugId").Str()
	eq(t, goal.At("url").Str(), "http://localhost:3000/acme/initiative/grow-2030-"+slug)
	eq(t, goal.At("projectCount").Num(), 2)
	eq(t, goal.At("progress").Num(), 0.5, "the average of 1.0 and 0.0")
	eq(t, goal.At("status").Str(), "planned")
	active := call(t, w.Ann, "save_initiative", M{"id": slug, "status": "active"})
	check(t, active.At("startedAt").IsString())
	call(t, w.Ann, "save_initiative", M{"name": "Grow EU", "parent": "Grow 2030"})
	looped := fails(t, w.Ann, "save_initiative", M{"id": "Grow 2030", "parent": "Grow EU"})
	contains(t, looped, "itself")
	call(t, w.Ann, "save_status_update", M{"initiative": "Grow 2030", "body": "Going well"})
	eqList(t, notificationKinds(w, bob.User), []string{"initiativeUpdateCreated"})

	got := call(t, w.Ann, "get_initiative", M{"id": "grow 2030"})
	eq(t, got.At("projects").Len(), 2)
	eq(t, got.At("projects", 0, "progress").Num(), 1)
	eq(t, got.At("subInitiatives", 0, "name").Str(), "Grow EU")
	eq(t, got.At("links", 0, "label").Str(), "OKRs")
	eq(t, got.At("health").Str(), "onTrack")
	eq(t, got.At("latestUpdate", "body").Str(), "Going well")
	project := call(t, w.Ann, "get_project", M{"id": "One"})
	eq(t, project.At("initiatives", 0, "name").Str(), "Grow 2030")
	eqList(t, names(call(t, w.Ann, "list_initiatives", M{"status": "active"})), []string{"Grow 2030"})
	eqList(t, names(call(t, w.Ann, "list_initiatives", M{"parent": "Grow 2030"})), []string{"Grow EU"})
	eq(t, len(names(call(t, w.Ann, "list_projects", M{"initiative": slug}))), 2)
	call(t, w.Ann, "delete_initiative", M{"id": "Grow EU"})
	eqList(t, names(call(t, w.Ann, "list_initiatives", M{})), []string{"Grow 2030"})
	call(t, w.Ann, "restore_initiative", M{"id": "Grow EU"})
	eq(t, len(names(call(t, w.Ann, "list_initiatives", M{}))), 2)
	archived := call(t, w.Ann, "archive_initiative", M{"id": "Grow EU"})
	check(t, archived.At("archivedAt").IsString())
	eqList(t, names(call(t, w.Ann, "list_initiatives", M{})), []string{"Grow 2030"})
	eq(t, len(names(call(t, w.Ann, "list_initiatives", M{"includeArchived": true}))), 2)
	unarchived := call(t, w.Ann, "unarchive_initiative", M{"id": "Grow EU"})
	check(t, unarchived.At("archivedAt").IsNull())
}

func TestProjectTemplatesCreateMilestonesAndIssues(t *testing.T) {
	w := world(t)
	w.exec("INSERT INTO templates (workspace_id, type, name, data) VALUES ($1, 'project', 'Launch', $2)", w.Workspace, M{
		"summary":     "A launch",
		"description": "## Plan",
		"state":       "planned",
		"teams":       []any{"ENG"},
		"milestones":  []any{M{"name": "Alpha"}, M{"name": "Release", "targetDate": "2030-06-01"}},
		"issues": []any{
			M{"title": "Write the plan", "milestone": "alpha", "priority": 2},
			M{"title": "Announce", "milestone": "Release"},
			M{"title": "Loose end"},
		},
	})
	project := call(t, w.Ann, "save_project", M{"name": "Mobile launch", "template": "Launch"})
	eq(t, project.At("summary").Str(), "A launch")
	eq(t, project.At("description").Str(), "## Plan")
	eq(t, project.At("status", "type").Str(), "planned")
	eq(t, project.At("teams", 0, "key").Str(), "ENG")
	eq(t, project.At("milestones", 0, "name").Str(), "Alpha")
	eq(t, project.At("milestones", 0, "issueCount").Num(), 1)
	eq(t, project.At("milestones", 1, "targetDate").Str(), "2030-06-01")
	eq(t, project.At("issueCount").Num(), 3)
	plan := call(t, w.Ann, "get_issue", M{"id": "ENG-1"})
	eq(t, plan.At("projectMilestone", "name").Str(), "Alpha")
	eq(t, plan.At("priority").Num(), 2)
	wins := call(t, w.Ann, "save_project", M{"name": "Web launch", "template": "Launch", "state": "started"})
	eq(t, wins.At("status", "type").Str(), "started", "arguments win over the template")
}

func TestProjectVisibilityTrashAndArchive(t *testing.T) {
	w := world(t)
	guest := w.member("gus@else.test", "Gus", "guest")
	call(t, w.Ann, "save_project", M{"name": "Secret", "setTeams": []any{"ENG"}})
	eq(t, len(names(call(t, guest, "list_projects", M{}))), 0)
	contains(t, fails(t, guest, "get_project", M{"id": "Secret"}), "No project")
	w.exec("INSERT INTO team_members (team_id, user_id) SELECT id, $1 FROM teams WHERE key = 'ENG'", guest.User)
	eq(t, call(t, guest, "get_project", M{"id": "Secret"}).At("name").Str(), "Secret")

	trashed := call(t, w.Ann, "delete_project", M{"id": "Secret"})
	check(t, trashed.At("trashed").Bool())
	eq(t, len(names(call(t, w.Ann, "list_projects", M{}))), 0)
	eqList(t, names(call(t, w.Ann, "list_projects", M{"includeTrashed": true})), []string{"Secret"})
	fails(t, w.Ann, "get_project", M{"id": "Secret"})
	call(t, w.Ann, "restore_project", M{"id": "Secret"})
	call(t, w.Ann, "archive_project", M{"id": "Secret"})
	eq(t, len(names(call(t, w.Ann, "list_projects", M{}))), 0)
	eqList(t, names(call(t, w.Ann, "list_projects", M{"includeArchived": true})), []string{"Secret"})
	call(t, w.Ann, "unarchive_project", M{"id": "Secret"})
	eqList(t, names(call(t, w.Ann, "list_projects", M{})), []string{"Secret"})
}

func TestProjectStatusesAndLabels(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	call(t, w.Ann, "save_project", M{"name": "P", "setTeams": []any{"ENG"}, "state": "Paused"})
	statuses := call(t, w.Ann, "list_project_statuses", M{})
	var paused J
	for _, s := range statuses.At("nodes").List() {
		if s.At("name").Str() == "Paused" {
			paused = s
		}
	}
	eq(t, paused.At("projectCount").Num(), 1)
	contains(t, fails(t, bob, "save_project_status", M{"name": "Review", "type": "started"}), "admin")
	review := call(t, w.Ann, "save_project_status", M{"name": "Review", "type": "started"})
	eq(t, review.At("position").Num(), 6)
	inUse := fails(t, w.Ann, "delete_project_status", M{"id": "Paused"})
	contains(t, inUse, "moveProjectsTo")
	call(t, w.Ann, "delete_project_status", M{"id": "Paused", "moveProjectsTo": "Review"})
	moved := call(t, w.Ann, "get_project", M{"id": "P"})
	eq(t, moved.At("status", "name").Str(), "Review")
	check(t, moved.At("startedAt").IsString())

	call(t, bob, "save_project_label", M{"name": "Area", "isGroup": true})
	call(t, bob, "save_project_label", M{"name": "Mobile", "parent": "Area"})
	call(t, bob, "save_project", M{"id": "P", "addLabels": []any{"Area/Mobile"}})
	labels := call(t, bob, "list_project_labels", M{})
	eqList(t, names(labels), []string{"Area", "Mobile"})
	eq(t, labels.At("nodes", 1, "projectCount").Num(), 1)
	group := fails(t, bob, "save_project", M{"id": "P", "addLabels": []any{"Area"}})
	contains(t, group, "No project label")
	call(t, bob, "retire_project_label", M{"id": "Area"})
	eq(t, len(names(call(t, bob, "list_project_labels", M{}))), 0)
	eq(t, len(names(call(t, bob, "list_project_labels", M{"includeRetired": true}))), 2)
	restored := call(t, bob, "restore_project_label", M{"id": "Area"})
	check(t, restored.At("retiredAt").IsNull())
	eq(t, len(names(call(t, bob, "list_project_labels", M{}))), 2, "the group's labels come back with it")
	call(t, bob, "delete_project_label", M{"id": "Mobile"})
	eq(t, call(t, bob, "get_project", M{"id": "P"}).At("labels").Len(), 0)
}

func TestProgressOnlyCountsIssuesTheReaderSees(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	secret := w.createTeam("Secret", "SEC")
	w.exec("UPDATE teams SET private = true WHERE id = $1", secret)
	call(t, w.Ann, "save_project", M{"name": "Launch", "setTeams": []any{"ENG", "SEC"}})
	call(t, w.Ann, "save_milestone", M{"project": "Launch", "name": "Beta"})
	call(t, w.Ann, "save_issue", M{"team": "ENG", "title": "Public", "project": "Launch", "milestone": "Beta", "state": "Done"})
	call(t, w.Ann, "save_issue", M{"team": "SEC", "title": "Hidden", "project": "Launch", "milestone": "Beta"})
	call(t, w.Ann, "save_initiative", M{"name": "Grow", "addProjects": []any{"Launch"}})

	eq(t, call(t, w.Ann, "get_project", M{"id": "Launch"}).At("issueCount").Num(), 2)
	seen := call(t, bob, "get_project", M{"id": "Launch"})
	eq(t, seen.At("issueCount").Num(), 1, "Bob isn't in the private team")
	eq(t, seen.At("progress").Num(), 1)
	eq(t, seen.At("issueCountByStateType").Len(), 1)
	eq(t, call(t, bob, "get_milestone", M{"id": "Beta", "project": "Launch"}).At("issueCount").Num(), 1)
	eq(t, call(t, bob, "get_initiative", M{"id": "Grow"}).At("progress").Num(), 1)
	eq(t, call(t, w.Ann, "get_initiative", M{"id": "Grow"}).At("progress").Num(), 0.5)
}

func TestDateResolutionsNeedADate(t *testing.T) {
	w := world(t)
	call(t, w.Ann, "save_project", M{"name": "Launch", "setTeams": []any{"ENG"}, "targetDate": "2030-03-31"})
	cleared := call(t, w.Ann, "save_project", M{"id": "Launch", "targetDate": nil, "targetDateResolution": "quarter"})
	check(t, cleared.At("targetDate").IsNull() && cleared.At("targetDateResolution").IsNull(), cleared.JSON())
	call(t, w.Ann, "save_initiative", M{"name": "Grow", "targetDate": "2030-03-31"})
	initiative := call(t, w.Ann, "save_initiative", M{"id": "Grow", "targetDate": nil, "targetDateResolution": "year"})
	check(t, initiative.At("targetDateResolution").IsNull(), initiative.JSON())
}

func TestLinksWithoutTheSlugIDFindThingsByName(t *testing.T) {
	w := world(t)
	project := call(t, w.Ann, "save_project", M{"name": "Launch v2!", "setTeams": []any{"ENG"}})
	id := project.At("id").Str()
	eq(t, call(t, w.Ann, "get_project", M{"id": "launch-v2"}).At("id").Str(), id)
	eq(t, call(t, w.Ann, "get_project", M{"id": "http://localhost:3000/acme/project/launch-v2"}).At("id").Str(), id)
	initiative := call(t, w.Ann, "save_initiative", M{"name": "Grow 2030"})
	eq(t, call(t, w.Ann, "get_initiative", M{"id": "grow-2030"}).At("id").Str(), initiative.At("id").Str())
	document := call(t, w.Ann, "save_document", M{"title": "Q3: Plan", "project": id})
	eq(t, call(t, w.Ann, "get_document", M{"id": "q3-plan"}).At("id").Str(), document.At("id").Str())
	contains(t, fails(t, w.Ann, "get_project", M{"id": "launch"}), "Launch v2!")
}
