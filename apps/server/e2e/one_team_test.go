package e2e

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// unite runs what the one_team_per_workspace migration ran on every workspace.
func (w *World) unite() {
	w.t.Helper()
	w.exec("SELECT unite_workspace_teams($1)", w.Workspace)
}

func idOf(t *testing.T, list J, name string) string {
	t.Helper()
	for _, node := range list.At("nodes").List() {
		if node.At("name").Str() == name {
			return node.At("id").Str()
		}
	}
	t.Fatalf("no %q in %s", name, list.JSON())
	return ""
}

func TestMergesTheTeamsOfAWorkspace(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	des := w.createTeam("Design", "DES").String()
	gus := w.member("gus@else.test", "Gus", "guest")
	carl := w.member("carl@acme.test", "Carl", "member")
	w.exec("DELETE FROM team_members WHERE user_id = $1", carl.User)
	call(t, w.Ann, "save_team", M{"id": "DES", "addMembers": []any{"gus"}, "cyclesEnabled": true})
	call(t, w.Ann, "save_team", M{"id": "ENG", "cyclesEnabled": true})

	for _, title := range []string{"Login", "Signup", "Billing"} {
		call(t, w.Ann, "save_issue", M{"team": "ENG", "title": title})
	}
	call(t, w.Ann, "save_issue_status", M{"team": "DES", "name": "Design Review", "type": "started"})
	engPlatform := call(t, w.Ann, "save_issue_label", M{"name": "Platform", "team": "ENG"}).At("id").Str()
	desPlatform := call(t, w.Ann, "save_issue_label", M{"name": "platform", "team": "DES"}).At("id").Str()
	call(t, w.Ann, "save_issue_label", M{"name": "Visual", "team": "DES"})
	call(t, w.Ann, "save_cycle", M{"team": "ENG", "startsAt": "2030-01-01", "endsAt": "2030-01-15"})
	call(t, w.Ann, "save_cycle", M{"team": "DES", "startsAt": "2030-02-01", "endsAt": "2030-02-15"})
	call(t, w.Ann, "save_cycle", M{"team": "DES", "startsAt": "2030-03-01", "endsAt": "2030-03-15"})
	desTodo := idOf(t, call(t, w.Ann, "list_issue_statuses", M{"team": "DES"}), "Todo")
	engTodo := idOf(t, call(t, w.Ann, "list_issue_statuses", M{"team": "ENG"}), "Todo")
	call(t, w.Ann, "save_issue", M{"team": "DES", "title": "Logo", "state": "Todo", "labels": []any{"Platform"}, "cycle": "1"})
	call(t, w.Ann, "save_issue", M{"team": "DES", "title": "Colors", "state": "Design Review", "labels": []any{"Visual"}})
	call(t, w.Ann, "save_custom_view", M{"name": "Design todo", "shared": true, "team": "DES",
		"filter": M{"state": M{"id": M{"eq": desTodo}}, "team": M{"key": M{"eq": "DES"}}}})
	call(t, w.Ann, "add_favorite", M{"type": "team", "target": "DES"})
	call(t, w.Ann, "add_favorite", M{"type": "team", "target": "ENG"})
	call(t, w.Ann, "save_project", M{"name": "Rebrand", "addTeams": []any{"DES"}})
	hooks := newReceiver(w, http.StatusOK)
	hook := call(t, w.Ann, "save_webhook", M{"url": hooks.url, "resourceTypes": []any{"Issue"}, "team": "DES"}).At("id").Str()

	w.unite()

	teams := call(t, w.Ann, "list_teams", M{"includeArchived": true})
	eqList(t, teams.At("nodes").Strings("key"), []string{"ENG"}, "the team with the most issues stays")
	eq(t, call(t, w.Ann, "get_team", M{"id": "DES"}).At("key").Str(), "ENG")
	eq(t, call(t, w.Ann, "get_team", M{"id": des}).At("key").Str(), "ENG")
	eq(t, call(t, w.Ann, "get_team", M{"id": "design"}).At("key").Str(), "ENG")

	logo := call(t, w.Ann, "get_issue", M{"id": "DES-1"})
	eq(t, logo.At("identifier").Str(), "ENG-4")
	eqList(t, logo.At("previousIdentifiers").Strings(), []string{"DES-1"})
	eq(t, logo.At("state", "id").Str(), engTodo, "statuses of the same name fold together")
	eq(t, logo.At("labels", 0, "id").Str(), engPlatform, "labels of the same name fold together")
	eq(t, logo.At("cycle", "number").Num(), 2)
	colors := call(t, w.Ann, "get_issue", M{"id": "http://localhost:3000/acme/issue/DES-2"})
	eq(t, colors.At("identifier").Str(), "ENG-5")
	eq(t, colors.At("state", "name").Str(), "Design Review")

	eq(t, len(identifiers(call(t, w.Ann, "list_issues", M{"team": "DES"}))), 5)
	eq(t, len(identifiers(call(t, w.Ann, "list_issues", M{"team": des}))), 5)
	eq(t, len(identifiers(call(t, w.Ann, "list_issues", M{"filter": M{"team": M{"key": M{"in": []any{"des"}}}}}))), 5)
	eqList(t, identifiers(call(t, w.Ann, "list_issues", M{"filter": M{"labels": M{"some": M{"id": M{"eq": desPlatform}}}}})), []string{"ENG-4"})
	eqList(t, identifiers(call(t, w.Ann, "list_issues", M{"query": "DES-2"})), []string{"ENG-5"})
	eq(t, call(t, w.Ann, "search", M{"query": "des-1"}).At("nodes", 0, "identifier").Str(), "ENG-4")
	eq(t, call(t, w.Ann, "save_issue", M{"team": "DES", "title": "After"}).At("identifier").Str(), "ENG-6")
	eq(t, call(t, w.Ann, "save_issue", M{"team": des, "title": "After too", "state": desTodo}).At("state", "id").Str(), engTodo)

	statuses := call(t, w.Ann, "list_issue_statuses", M{}).At("nodes").Strings("name")
	eqList(t, statuses, []string{"Backlog", "Todo", "In Progress", "In Review", "Design Review", "Done", "Canceled", "Duplicate"})
	eq(t, call(t, w.Ann, "get_issue_status", M{"id": desTodo}).At("id").Str(), engTodo)
	eq(t, call(t, w.Ann, "get_issue_label", M{"id": desPlatform}).At("name").Str(), "Platform")
	labels := call(t, w.Ann, "list_issue_labels", M{"team": "DES"}).At("nodes").Strings("name")
	eq(t, len(slices.DeleteFunc(labels, func(name string) bool { return !strings.EqualFold(name, "platform") })), 1)
	eqList(t, numbers(call(t, w.Ann, "list_cycles", M{})), []int{3, 2, 1})

	view := call(t, w.Ann, "get_custom_view", M{"id": "Design todo"})
	eq(t, view.At("filter", "state", "id", "eq").Str(), engTodo)
	eq(t, view.At("team", "key").Str(), "ENG")
	eqList(t, identifiers(call(t, w.Ann, "list_issues", M{"customView": "Design todo"})), []string{"ENG-7", "ENG-4"})
	favorites := call(t, w.Ann, "list_favorites", M{})
	eqList(t, favorites.At("nodes").Strings("type"), []string{"team"}, "one favorite of the team")
	eqList(t, call(t, w.Ann, "get_project", M{"id": "Rebrand"}).At("teams").Strings("key"), []string{"ENG"})
	eqList(t, names(call(t, w.Ann, "list_projects", M{"filter": M{"team": M{"name": M{"eq": "design"}}}})), []string{"Rebrand"})

	members := call(t, w.Ann, "get_team", M{"id": "ENG"}).At("members").Strings("email")
	slices.Sort(members)
	eqList(t, members, []string{"ann@acme.test", "bob@acme.test", "carl@acme.test", "gus@else.test"})
	eq(t, call(t, gus, "get_issue", M{"id": "ENG-1"}).At("title").Str(), "Login", "the guest of DES is on the one team")
	eq(t, call(t, bob, "get_issue", M{"id": "DES-1"}).At("identifier").Str(), "ENG-4")

	key := apiKey(w, "read", "write")
	_, rest := send(w.Router, request("GET", "/v1/teams/DES/statuses", key, nil))
	eq(t, rest.At("nodes").Len(), 8, rest.JSON())
	_, tool := send(w.Router, rpc("/mcp", key, M{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": M{
		"name": "save_issue", "arguments": M{"team": "Design", "title": "Via MCP", "state": "Design Review"}}}))
	contains(t, tool.At("result", "content", 0, "text").Str(), "ENG-8")
	eq(t, call(t, w.Ann, "get_webhook", M{"id": hook}).At("team", "key").Str(), "ENG")
	deliverDue(t, w)
	eq(t, hooks.count(), 3, "the webhook of DES hears about the issues made since")

	before := call(t, w.Ann, "list_issues", M{"includeDescription": true}).JSON()
	w.unite()
	eq(t, call(t, w.Ann, "list_issues", M{"includeDescription": true}).JSON(), before, "uniting again changes nothing")
}

func TestOldIdentifiersLinkGitHubAfterTheMerge(t *testing.T) {
	g := withGitHub(t)
	g.connect()
	call(t, g.Ann, "save_issue", M{"team": "ENG", "title": "First"})
	call(t, g.Ann, "save_issue", M{"team": "ENG", "title": "Second"})
	g.createTeam("Unbind", "UNB")
	numbered(t, g.World, "UNB", 456)
	g.unite()
	eq(t, g.deliver(t, "push", "push.json", nil), http.StatusOK)
	branch := call(t, g.Ann, "get_issue", M{"id": "UNB-456"})
	eq(t, branch.At("identifier").Str(), "ENG-3")
	eq(t, branch.At("attachments", 0, "sourceType").Str(), "github_branch")
	eq(t, branch.At("state", "type").Str(), "started")
}

func TestTeamArgumentsDefaultToTheWorkspacesTeam(t *testing.T) {
	w := world(t)
	call(t, w.Ann, "save_team", M{"id": "ENG", "cyclesEnabled": true})

	eq(t, call(t, w.Ann, "list_issue_statuses", M{}).At("nodes").Len(), 7)
	eq(t, call(t, w.Ann, "get_issue_status", M{"id": "Todo"}).At("team", "key").Str(), "ENG")
	eq(t, call(t, w.Ann, "save_issue_status", M{"name": "QA", "type": "started"}).At("team", "key").Str(), "ENG")
	call(t, w.Ann, "delete_issue_status", M{"id": "QA"})
	cycle := call(t, w.Ann, "save_cycle", M{"startsAt": "-PT1H", "endsAt": "P7D", "name": "Now"})
	eq(t, cycle.At("team", "key").Str(), "ENG")
	eq(t, call(t, w.Ann, "get_cycle", M{"id": "current"}).At("name").Str(), "Now")
	eq(t, call(t, w.Ann, "get_cycle", M{"id": "1"}).At("name").Str(), "Now")
	eqList(t, numbers(call(t, w.Ann, "list_cycles", M{})), []int{1})
	issue := call(t, w.Ann, "save_issue", M{"title": "Plain", "cycle": "current"})
	eq(t, issue.At("identifier").Str(), "ENG-1")
	eq(t, call(t, w.Ann, "add_favorite", M{"type": "cycle", "target": "current"}).At("target", "team", "key").Str(), "ENG")
	imported := call(t, w.Ann, "import_issues", M{"issues": []any{M{"number": 7, "title": "Imported"}}})
	eqList(t, imported.At("identifiers").Strings(), []string{"ENG-7"})
	eqList(t, call(t, w.Ann, "save_project", M{"name": "Launch"}).At("teams").Strings("key"), []string{"ENG"})
	call(t, w.Ann, "save_issue_label", M{"name": "Platform", "team": "ENG"})
	eqList(t, identifiers(call(t, w.Ann, "list_issues", M{"label": "Platform"})), []string{})
}

func TestNewWorkspacesGetTheirTeam(t *testing.T) {
	w := world(t)
	create := func(body M) (int, J) {
		req := httptest.NewRequest("POST", "/api/workspaces", strings.NewReader(string(marshal(body))))
		req.Header.Set("Cookie", w.session(w.Ann.User))
		req.Header.Set("X-Ricrac", "1")
		req.Header.Set("Content-Type", "application/json")
		return send(w.Router, req)
	}
	status, created := create(M{"name": "Unbind Labs", "urlKey": "unbind"})
	eq(t, status, http.StatusOK, created.JSON())
	unbind := w.actor(w.Ann.User, uuid.MustParse(created.At("id").Str()))
	team := call(t, unbind, "list_teams", M{}).At("nodes", 0)
	eq(t, team.At("key").Str(), "UNB")
	eq(t, team.At("name").Str(), "Unbind Labs")
	eq(t, call(t, unbind, "save_issue", M{"title": "First"}).At("identifier").Str(), "UNB-1")

	status, named := create(M{"name": "Other", "urlKey": "other", "teamName": "Platform Team", "teamKey": "PLT"})
	eq(t, status, http.StatusOK, named.JSON())
	other := w.actor(w.Ann.User, uuid.MustParse(named.At("id").Str()))
	eq(t, call(t, other, "list_teams", M{}).At("nodes", 0, "key").Str(), "PLT")
}

func TestUnitingKeepsAnUnarchivedTeamAndMakesOneWhenThereIsNone(t *testing.T) {
	w := world(t)
	w.createTeam("Legacy", "OLD")
	call(t, w.Ann, "save_issue", M{"team": "OLD", "title": "Old one"})
	call(t, w.Ann, "save_issue", M{"team": "OLD", "title": "Old two"})
	call(t, w.Ann, "delete_team", M{"id": "OLD"})
	call(t, w.Ann, "save_team", M{"id": "ENG", "private": true})
	w.unite()
	team := call(t, w.Ann, "get_team", M{"id": "OLD"})
	eq(t, team.At("key").Str(), "ENG", "an archived team doesn't stay, whatever its issues")
	eq(t, team.At("private").Bool(), false)
	eq(t, call(t, w.Ann, "get_issue", M{"id": "OLD-2"}).At("identifier").Str(), "ENG-2")

	empty := scalar[uuid.UUID](w, "INSERT INTO workspaces (name, url_key) VALUES ('Hollow Co', 'hollow-co') RETURNING id")
	w.exec("INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')", empty, w.Ann.User)
	w.exec("SELECT unite_workspace_teams($1)", empty)
	hollow := w.actor(w.Ann.User, empty)
	made := call(t, hollow, "list_teams", M{}).At("nodes", 0)
	eq(t, made.At("key").Str(), "HC")
	eq(t, made.At("name").Str(), "Hollow Co")
	eq(t, made.At("isMember").Bool(), true)
	eq(t, call(t, hollow, "save_issue", M{"title": "First"}).At("state", "name").Str(), "Backlog")
}
