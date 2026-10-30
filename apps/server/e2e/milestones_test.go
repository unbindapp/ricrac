package e2e

import (
	"regexp"
	"testing"
	"time"
)

// Milestones of the workspace's own, those of projects, and both synced with GitHub milestones;
// issues created in Ricrac opening on GitHub.

func TestWorkspaceMilestones(t *testing.T) {
	w := world(t)
	created := call(t, w.Ann, "save_milestone", M{"name": "v1.0", "targetDate": "2026-12-01", "description": "First release"})
	check(t, created.At("project").IsNull())
	eq(t, created.At("targetDate").Str(), "2026-12-01")
	contains(t, fails(t, w.Ann, "save_milestone", M{"name": "V1.0"}), "already")
	project := call(t, w.Ann, "save_project", M{"name": "Launch", "setTeams": []any{"ENG"}})
	call(t, w.Ann, "save_milestone", M{"project": project.At("id").Str(), "name": "v1.0"})
	call(t, w.Ann, "save_milestone", M{"project": project.At("id").Str(), "name": "Beta"})

	listed := call(t, w.Ann, "list_milestones", nil)
	eq(t, listed.At("nodes").Len(), 1, "the workspace's own only")
	eq(t, call(t, w.Ann, "list_milestones", M{"project": "Launch"}).At("nodes").Len(), 2)

	call(t, w.Ann, "save_issue", M{"title": "Ship it", "project": "Launch", "milestone": "v1.0"})
	issue := call(t, w.Ann, "get_issue", M{"id": "ENG-1"})
	eq(t, issue.At("projectMilestone", "projectId").Str(), project.At("id").Str(), "the project's own comes first")
	call(t, w.Ann, "save_issue", M{"id": "ENG-1", "milestone": created.At("id").Str()})
	call(t, w.Ann, "save_issue", M{"id": "ENG-1", "project": nil})
	issue = call(t, w.Ann, "get_issue", M{"id": "ENG-1"})
	eq(t, issue.At("projectMilestone", "name").Str(), "v1.0", "leaving the project keeps the workspace's milestone")
	check(t, issue.At("projectMilestone", "projectId").IsNull())

	call(t, w.Ann, "save_issue", M{"id": "ENG-1", "project": "Launch", "milestone": "Beta"})
	call(t, w.Ann, "save_issue", M{"id": "ENG-1", "project": nil})
	check(t, call(t, w.Ann, "get_issue", M{"id": "ENG-1"}).At("projectMilestone").IsNull(), "a project's milestone stays with it")
	contains(t, lastHistory(t, w, "ENG-1").JSON(), `"projectMilestone"`)

	bob := w.member("bob@acme.test", "Bob", "member")
	call(t, w.Ann, "save_comment", M{"milestoneId": created.At("id").Str(), "body": "@Bob can you check the date?"})
	eq(t, call(t, bob, "get_notifications", nil).At("nodes").Len(), 1)

	closed := call(t, w.Ann, "save_milestone", M{"id": "v1.0", "closed": true})
	check(t, !closed.At("closedAt").IsNull())
	call(t, w.Ann, "save_milestone", M{"name": "v2.0"})
	eq(t, call(t, w.Ann, "list_milestones", nil).At("nodes", 0, "name").Str(), "v2.0", "open ones first")
	eq(t, call(t, w.Ann, "save_milestone", M{"id": "v1.0", "closed": false}).At("closedAt").V, nil)
}

func TestMilestonesSyncWithGitHub(t *testing.T) {
	g := withGitHub(t)
	g.fake.milestones = []M{
		milestone(801, 1, "v1.0", M{"description": "First", "due_on": "2026-12-01T08:00:00Z"}),
		milestone(802, 2, "v0.9", M{"state": "closed"}),
	}
	g.fake.milestoneIssues[1] = []M{{"id": 3500001347, "number": 1347}}
	g.syncRepo(t, "ENG")
	g.deliver(t, "issues", "issues_opened.json", nil)
	g.runJobs(t)

	listed := call(t, g.Ann, "list_milestones", nil).At("nodes")
	eq(t, listed.Len(), 2, listed.JSON())
	first := listed.At(0)
	eq(t, first.At("name").Str(), "v1.0")
	eq(t, first.At("description").Str(), "First")
	eq(t, first.At("targetDate").Str(), "2026-12-01")
	eq(t, first.At("github", 0, "number").Num(), 1.0)
	eq(t, first.At("github", 0, "repository").Str(), "unbindapp/unbind")
	check(t, !listed.At(1, "closedAt").IsNull(), "closed on GitHub")
	eq(t, call(t, g.Ann, "get_issue", M{"id": "ENG-1"}).At("projectMilestone", "name").Str(), "v1.0", "GitHub's issues join it")
	eq(t, scalar[int](g.World, "SELECT count(*) FROM github_outbox"), 0, "nothing goes back")

	g.deliver(t, "milestone", "issues_opened.json", func(p M) {
		delete(p, "issue")
		p["action"] = "edited"
		p["milestone"] = milestone(801, 1, "v1.0.0", M{"description": "First", "due_on": "2026-12-01T08:00:00Z",
			"updated_at": time.Now().UTC().Add(time.Minute)})
	})
	eq(t, call(t, g.Ann, "get_milestone", M{"id": first.At("id").Str()}).At("name").Str(), "v1.0.0")
	eq(t, scalar[int](g.World, "SELECT count(*) FROM github_outbox"), 0)

	call(t, g.Ann, "save_milestone", M{"id": "v1.0.0", "targetDate": "2027-01-15", "closed": true})
	g.runJobs(t)
	patched := g.fake.sent("PATCH", "/repos/unbindapp/unbind/milestones/1")
	eq(t, len(patched), 1)
	eq(t, patched[0].Body["due_on"], any("2027-01-15T12:00:00Z"))
	eq(t, patched[0].Body["state"], any("closed"))
	eq(t, patched[0].Body["title"], any("v1.0.0"))

	call(t, g.Ann, "save_milestone", M{"name": "v2.0"})
	call(t, g.Ann, "save_issue", M{"id": "ENG-1", "milestone": "v2.0"})
	g.runJobs(t)
	createdOnGitHub := g.fake.sent("POST", "/repos/unbindapp/unbind/milestones")
	eq(t, len(createdOnGitHub), 1)
	eq(t, createdOnGitHub[0].Body["title"], any("v2.0"))
	issuePatches := g.fake.sent("PATCH", "/repos/unbindapp/unbind/issues/1347")
	eq(t, issuePatches[len(issuePatches)-1].Body["milestone"], any(3.0))

	g.deliver(t, "issues", "issues_opened.json", func(p M) {
		p["action"] = "demilestoned"
		p["issue"].(M)["milestone"] = nil
	})
	check(t, call(t, g.Ann, "get_issue", M{"id": "ENG-1"}).At("projectMilestone").IsNull())
	g.deliver(t, "issues", "issues_opened.json", func(p M) {
		p["action"] = "milestoned"
		p["issue"].(M)["milestone"] = g.fake.milestoneNumbered(3)
	})
	eq(t, call(t, g.Ann, "get_issue", M{"id": "ENG-1"}).At("projectMilestone", "name").Str(), "v2.0")

	g.fake.milestones = g.fake.milestones[:1]
	g.exec("UPDATE github_repo_syncs SET milestones_synced_at = NULL")
	g.runJobs(t)
	names := call(t, g.Ann, "list_milestones", nil).At("nodes").Strings("name")
	eq(t, len(names), 1, "deleted on GitHub, deleted here")
	eq(t, names[0], "v1.0.0")

	call(t, g.Ann, "delete_milestone", M{"id": "v1.0.0"})
	g.runJobs(t)
	eq(t, len(g.fake.sent("DELETE", "/repos/unbindapp/unbind/milestones/1")), 1)
	check(t, call(t, g.Ann, "list_github_repo_syncs", nil).At("nodes", 0, "lastError").IsNull(),
		call(t, g.Ann, "list_github_repo_syncs", nil).JSON())
}

// opened is the issues the app opened on GitHub, oldest first.
func (f *fakeGitHub) opened() []fakeRequest {
	var found []fakeRequest
	for _, r := range f.sent("POST", "/issues") {
		if issuesPath.MatchString(r.Path) {
			found = append(found, r)
		}
	}
	return found
}

var issuesPath = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues$`)

func TestRicracIssuesOpenOnGitHubAsTheirAuthor(t *testing.T) {
	g := withGitHub(t)
	sync := g.syncRepo(t, "ENG")
	call(t, g.Ann, "save_issue", M{"title": "Not synced yet"})
	g.runJobs(t)
	eq(t, len(g.fake.opened()), 0, "syncs don't open Ricrac issues unless asked")

	call(t, g.Ann, "save_github_repo_sync", M{"id": sync.At("id").Str(), "createIssues": true, "linkBack": true})
	g.linkGitHub(t, g.Ann.User)
	g.exec("UPDATE users SET github_login = 'ann-gh' WHERE id = $1", g.Ann.User)
	call(t, g.Ann, "save_issue_label", M{"name": "ui"})
	call(t, g.Ann, "save_milestone", M{"name": "v1.0"})

	// GitHub's webhook for the new issue can come before Ricrac has linked it.
	echoed := make(chan int)
	g.fake.onIssue = func(number int) {
		go func() {
			echoed <- g.deliver(t, "issues", "issues_opened.json", func(p M) {
				opened := p["issue"].(M)
				opened["id"], opened["number"], opened["title"] = 4000000000+number, number, "Dark mode"
				opened["user"], opened["assignee"], opened["labels"] = M{"login": "Ann-GH"}, nil, []M{}
				p["sender"] = M{"login": "Ann-GH"}
			})
		}()
		time.Sleep(100 * time.Millisecond)
	}
	call(t, g.Ann, "save_issue", M{"title": "Dark mode", "description": "Follow the system setting.", "state": "Done",
		"assignee": "me", "labels": []string{"ui"}, "milestone": "v1.0"})
	g.runJobs(t)
	eq(t, <-echoed, 200)
	opened := g.fake.opened()
	eq(t, len(opened), 1)
	eq(t, opened[0].Token, "ghu_user", "opened as Ann")
	eq(t, string(marshal(opened[0].Body)), `{"body":"Follow the system setting.","title":"Dark mode"}`)
	issue := call(t, g.Ann, "get_issue", M{"id": "ENG-2"})
	eq(t, issue.At("github", "number").Num(), 2001.0)
	eq(t, call(t, g.Ann, "list_issues", nil).At("nodes").Len(), 2, "the webhook doesn't import it again")

	path := "/repos/unbindapp/unbind/issues/2001"
	patches := g.fake.sent("PATCH", path)
	eq(t, len(patches), 1)
	eq(t, patches[0].Body["state"], any("closed"))
	eq(t, patches[0].Body["state_reason"], any("completed"))
	eq(t, patches[0].Body["milestone"], any(1.0))
	eq(t, string(marshal(g.fake.sent("POST", path+"/labels")[0].Body)), `{"labels":["ui"]}`)
	eq(t, string(marshal(g.fake.sent("POST", path+"/assignees")[0].Body)), `{"assignees":["ann-gh"]}`)
	contains(t, g.fake.sent("POST", path+"/comments")[0].Body["body"].(string), "Tracked in Ricrac as [ENG-2]")

	call(t, g.Ann, "save_issue", M{"title": "Kept here", "githubRepository": "none"})
	contains(t, fails(t, g.Ann, "save_issue", M{"title": "Elsewhere", "githubRepository": "unbindapp/other"}), "unbindapp/unbind")
	bob := g.member("bob@acme.test", "Bob", "member")
	call(t, bob, "save_issue", M{"title": "From Bob", "githubRepository": "unbind"})
	g.runJobs(t)
	opened = g.fake.opened()
	eq(t, len(opened), 2)
	eq(t, opened[1].Token, "ghs_installation", "without a linked account the app opens it")
	eq(t, opened[1].Body["body"], any("**Bob on Ricrac**\n\n"))
	eq(t, scalar[int](g.World, "SELECT count(*) FROM github_outbox"), 0)
}

func TestFieldsSyncWithTheOrganizationsIssueFields(t *testing.T) {
	g := withGitHub(t)
	sync := g.syncRepo(t, "ENG")
	call(t, g.Ann, "save_team", M{"id": "ENG", "estimateType": "fibonacci"})
	g.fake.issueFields = []M{
		{"id": 11, "name": "Priority", "data_type": "single_select", "options": []M{{"name": "Urgent"}, {"name": "High"}, {"name": "Medium"}, {"name": "Low"}}},
		{"id": 12, "name": "Estimate", "data_type": "number"},
		{"id": 13, "name": "Target date", "data_type": "date"},
		{"id": 14, "name": "Effort", "data_type": "single_select"},
	}
	g.deliver(t, "issues", "issues_opened.json", nil)
	call(t, g.Ann, "save_issue", M{"id": "ENG-1", "priority": 2})
	g.runJobs(t)
	eq(t, len(g.fake.sent("POST", "/issue-field-values")), 0, "only syncs that ask")

	call(t, g.Ann, "save_github_repo_sync", M{"id": sync.At("id").Str(), "syncFields": true})
	call(t, g.Ann, "save_issue", M{"id": "ENG-1", "priority": 1, "estimate": 3, "dueDate": "2026-12-24"})
	g.runJobs(t)
	set := g.fake.sent("POST", "/repos/unbindapp/unbind/issues/1347/issue-field-values")
	eq(t, len(set), 1)
	eq(t, string(marshal(set[0].Body)),
		`{"issue_field_values":[{"field_id":11,"value":"Urgent"},{"field_id":12,"value":3},{"field_id":13,"value":"2026-12-24"}]}`)

	call(t, g.Ann, "save_issue", M{"id": "ENG-1", "estimate": nil})
	g.runJobs(t)
	eq(t, len(g.fake.sent("DELETE", "/issues/1347/issue-field-values/12")), 1)

	g.fake.fieldValues[1347] = []M{
		{"issue_field_id": 11, "data_type": "single_select", "value": "Low", "single_select_option": M{"id": 1, "name": "Low", "color": "green"}},
		{"issue_field_id": 12, "data_type": "number", "value": 5},
	}
	g.deliver(t, "issues", "issues_opened.json", func(p M) { p["action"] = "field_added" })
	issue := call(t, g.Ann, "get_issue", M{"id": "ENG-1"})
	eq(t, issue.At("priority").Num(), 4.0)
	eq(t, issue.At("estimate").Num(), 5.0)
	check(t, issue.At("dueDate").IsNull(), "a field GitHub cleared clears here")
	eq(t, scalar[int](g.World, "SELECT count(*) FROM github_outbox"), 0, "nothing goes back")
}

func TestPreviewingAnIssueSavesNothing(t *testing.T) {
	g := withGitHub(t)
	sync := g.syncRepo(t, "ENG")
	call(t, g.Ann, "save_github_repo_sync", M{"id": sync.At("id").Str(), "createIssues": true})
	preview := call(t, g.Ann, "save_issue", M{"title": "Dark mode", "description": "Follow the system.", "priority": 2, "preview": true})
	eq(t, preview.At("identifier").Str(), "ENG-1")
	eq(t, preview.At("description").Str(), "Follow the system.")
	eq(t, preview.At("preview", "github", "repository").Str(), "unbindapp/unbind")
	check(t, preview.At("preview", "github", "openedAs").IsNull(), "the app opens it while Ann has no linked account")
	g.linkGitHub(t, g.Ann.User)
	g.exec("UPDATE users SET github_login = 'ann-gh' WHERE id = $1", g.Ann.User)
	eq(t, call(t, g.Ann, "save_issue", M{"title": "Dark mode", "preview": true}).At("preview", "github", "openedAs").Str(), "ann-gh")
	check(t, call(t, g.Ann, "save_issue", M{"title": "Dark mode", "githubRepository": "none", "preview": true}).At("preview", "github").IsNull())

	eq(t, call(t, g.Ann, "list_issues", nil).At("nodes").Len(), 0)
	eq(t, scalar[int](g.World, "SELECT count(*) FROM github_outbox"), 0)
	g.runJobs(t)
	eq(t, len(g.fake.opened()), 0)

	created := call(t, g.Ann, "save_issue", M{"title": "Dark mode"})
	eq(t, created.At("identifier").Str(), "ENG-1", "previews take no numbers")
	edited := call(t, g.Ann, "save_issue", M{"id": "ENG-1", "title": "Dark theme", "preview": true})
	eq(t, edited.At("title").Str(), "Dark theme")
	eq(t, call(t, g.Ann, "get_issue", M{"id": "ENG-1"}).At("title").Str(), "Dark mode")
}
