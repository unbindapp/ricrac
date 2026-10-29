package e2e

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
)

var visitorOps = []string{
	"get_workspace", "list_teams", "get_team", "list_issue_statuses", "list_issue_labels",
	"list_issues", "get_issue", "get_issue_history", "list_comments",
}

const signedOutMessage = "Authenticate with an OAuth access token or an API key: Authorization: Bearer <token>"

// visit runs an operation as someone signed out, naming the workspace as the web app does.
func visit(t *testing.T, w *World, workspace, name string, args M) (int, J) {
	t.Helper()
	req := httptest.NewRequest("POST", "/v1/ops/"+name, bytes.NewReader(marshal(args)))
	req.Header.Set("X-Ricrac", "1")
	req.Header.Set("X-Ricrac-Workspace", workspace)
	req.Header.Set("Content-Type", "application/json")
	status, value := send(w.Router, req)
	if schema := w.B.ResultSchema(name); status == http.StatusOK && schema != nil {
		if err := validate(schema, value.V, name); err != nil {
			t.Fatalf("%s answered visitors something its result schema doesn't allow: %v", name, err)
		}
	}
	return status, value
}

func visitOK(t *testing.T, w *World, name string, args M) J {
	t.Helper()
	status, value := visit(t, w, "acme", name, args)
	eq(t, status, http.StatusOK, name, value.JSON())
	check(t, !strings.Contains(value.JSON(), "@acme.test"), name, "shows an email", value.JSON())
	return value
}

func visitFails(t *testing.T, w *World, workspace, name string, args M) {
	t.Helper()
	status, value := visit(t, w, workspace, name, args)
	eq(t, status, http.StatusUnauthorized, name, value.JSON())
	eq(t, value.At("error").Str(), signedOutMessage, name)
}

func TestPublicWorkspacesAreReadableSignedOut(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	gus := w.member("gus@acme.test", "Gus Guest", "guest")
	call(t, w.Ann, "save_project", M{"name": "Launch", "addTeams": []any{"ENG"}})
	call(t, w.Ann, "save_issue", M{"title": "Crash on start", "assignee": "bob", "project": "Launch", "description": "It crashes"})
	call(t, w.Ann, "save_issue", M{"id": "ENG-1", "assignee": "ann", "labels": []any{"Bug"}})
	call(t, w.Ann, "share_issue", M{"id": "ENG-1", "user": "gus"})
	comment := call(t, bob, "save_comment", M{"issueId": "ENG-1", "body": "Seen it too"})
	call(t, w.Ann, "save_comment", M{"parentId": comment.At("id").V, "body": "Fixing"})
	call(t, w.Ann, "add_reaction", M{"commentId": comment.At("id").V, "emoji": "👍"})
	call(t, w.Ann, "save_issue", M{"title": "Thrown away"})
	call(t, w.Ann, "delete_issue", M{"id": "ENG-2"})

	eq(t, call(t, w.Ann, "get_workspace", M{}).At("public").Bool(), false, "workspaces start private")
	visitFails(t, w, "acme", "get_issue", M{"id": "ENG-1"})
	visitFails(t, w, w.Workspace.String(), "get_workspace", M{})
	contains(t, fails(t, bob, "update_workspace", M{"public": true}), "admins")
	eq(t, call(t, w.Ann, "update_workspace", M{"public": true}).At("public").Bool(), true)
	contains(t, call(t, w.Ann, "get_audit_log", M{}).JSON(), "public")

	workspace := visitOK(t, w, "get_workspace", M{})
	eq(t, workspace.At("name").Str(), "Acme")
	eq(t, workspace.At("teams", 0, "key").Str(), "ENG")
	eq(t, workspace.At("role").Str(), "guest")
	_, byID := visit(t, w, w.Workspace.String(), "get_workspace", M{})
	eq(t, byID.At("urlKey").Str(), "acme")

	issue := visitOK(t, w, "get_issue", M{"id": "ENG-1", "includeComments": true, "includeHistory": true})
	eq(t, issue.At("title").Str(), "Crash on start")
	eq(t, issue.At("description").Str(), "It crashes")
	eq(t, issue.At("assignee", "name").Str(), "Ann Smith")
	eq(t, issue.At("assignee", "email").Str(), "")
	eq(t, issue.At("labels", 0, "name").Str(), "Bug")
	check(t, issue.At("project").IsNull(), "projects are for members", issue.JSON())
	eq(t, issue.At("subscribers").Len(), 0)
	eq(t, issue.At("sharedWith").Len(), 0)
	eq(t, issue.At("comments", 0, "replies", 0, "body").Str(), "Fixing")
	eq(t, issue.At("comments", 0, "reactions", 0, "emoji").Str(), "👍")
	history := visitOK(t, w, "get_issue_history", M{"id": "ENG-1"})
	check(t, !strings.Contains(history.JSON(), "Launch"), "history leaves projects out", history.JSON())
	contains(t, history.JSON(), "Bob Jones")
	threads := visitOK(t, w, "list_comments", M{"issueId": "ENG-1"})
	eq(t, threads.At("nodes", 0, "user", "name").Str(), "Bob Jones")
	visitOK(t, w, "list_teams", M{})
	team := visitOK(t, w, "get_team", M{"id": "ENG"})
	eq(t, team.At("members").Len(), 0)
	visitOK(t, w, "list_issue_statuses", M{"team": "ENG"})
	visitOK(t, w, "list_issue_labels", M{})
	list := visitOK(t, w, "list_issues", M{"filter": M{"state": M{"type": M{"neq": "canceled"}}}})
	eq(t, list.At("nodes").Len(), 1)
	eq(t, list.At("nodes", 0, "identifier").Str(), "ENG-1")

	status, trashed := visit(t, w, "acme", "get_issue", M{"id": "ENG-2"})
	eq(t, status, http.StatusNotFound, trashed.JSON())
	for _, args := range []M{
		{"includeTrashed": true},
		{"assignee": "nobody"},
		{"project": "nothing"},
		{"filter": M{"assignee": M{"email": M{"startsWith": "a"}}}},
	} {
		status, refused := visit(t, w, "acme", "list_issues", args)
		eq(t, status, http.StatusForbidden, args, refused.JSON())
		check(t, !strings.Contains(refused.JSON(), "@acme.test"), refused.JSON())
	}
	project := call(t, w.Ann, "get_project", M{"id": "Launch"})
	status, value := visit(t, w, "acme", "list_comments", M{"projectId": project.At("id").V})
	eq(t, status, http.StatusUnauthorized, "visitors read comments of issues only", value.JSON())

	visitFails(t, w, "acme", "get_user", M{"id": "bob"})

	stranger := w.actor(w.addUser("eve@elsewhere.test", "Eve Stranger"), w.Workspace)
	eq(t, call(t, stranger, "get_issue", M{"id": "ENG-1"}).At("assignee", "email").Str(), "", "people signed in elsewhere read it as visitors")
	contains(t, fails(t, stranger, "save_comment", M{"issueId": "ENG-1", "body": "Hi"}), "workspace")
	contains(t, fails(t, stranger, "list_users", M{}), "workspace")
	eq(t, call(t, gus, "get_issue", M{"id": "ENG-1"}).At("title").Str(), "Crash on start", "members keep their own access")

	call(t, w.Ann, "update_workspace", M{"public": false})
	visitFails(t, w, "acme", "get_issue", M{"id": "ENG-1"})
	contains(t, fails(t, stranger, "get_issue", M{"id": "ENG-1"}), "workspace")
}

func TestVisitorsRunOnlyTheReadsOfPublicWorkspaces(t *testing.T) {
	w := world(t)
	call(t, w.Ann, "update_workspace", M{"public": true})
	call(t, w.Ann, "save_issue", M{"title": "Crash"})
	private := w.secondWorkspace("Private", "private", "PRI")
	status, listed := send(w.Router, httptest.NewRequest("GET", "/v1/ops", nil))
	eq(t, status, http.StatusOK)
	var open []string
	for _, op := range listed.At("operations").List() {
		name := op.At("name").Str()
		if slices.Contains(visitorOps, name) {
			eq(t, op.At("access").Str(), "read", name)
			visitFails(t, w, "private", name, M{})
			visitFails(t, w, private.String(), name, M{})
		}
		status, value := visit(t, w, "acme", name, M{})
		if status == http.StatusUnauthorized && value.At("error").Str() == signedOutMessage {
			continue
		}
		open = append(open, name)
	}
	slices.Sort(open)
	want := slices.Clone(visitorOps)
	slices.Sort(want)
	eqList(t, open, want, "only these operations answer visitors")

	req := httptest.NewRequest("POST", "/v1/ops/get_issue", strings.NewReader(`{"id":"ENG-1"}`))
	req.Header.Set("Authorization", "Bearer rr_key_nope")
	req.Header.Set("X-Ricrac-Workspace", "acme")
	status, value := send(w.Router, req)
	eq(t, status, http.StatusUnauthorized, "a bad token isn't let in as a visitor", value.JSON())
	contains(t, value.At("error").Str(), "invalid or expired")
	status, value = send(w.Router, httptest.NewRequest("GET", "/v1/issues/ENG-1", nil))
	eq(t, status, http.StatusUnauthorized, "visitors name the workspace", value.JSON())
}

func TestFilesOfPublicWorkspacesLoadSignedOut(t *testing.T) {
	w := world(t)
	call(t, w.Ann, "save_issue", M{"title": "Crash"})
	upload := func(workspace uuid.UUID) string {
		actor := w.actor(w.Ann.User, workspace)
		file := call(t, actor, "upload_file", M{"name": "shot.png", "contentType": "image/png", "base64": "iVBORw0K"})
		return strings.TrimPrefix(file.At("assetUrl").Str(), publicURL)
	}
	public := upload(w.Workspace)
	private := upload(w.secondWorkspace("Private", "private", "PRI"))
	call(t, w.Ann, "update_workspace", M{"public": true})
	get := func(path, cookie string) int {
		req := httptest.NewRequest("GET", path, nil)
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
		recorder := httptest.NewRecorder()
		w.Router.ServeHTTP(recorder, req)
		return recorder.Code
	}
	eq(t, get(public, ""), http.StatusOK)
	eq(t, get(private, ""), http.StatusUnauthorized)
	eq(t, get(fmt.Sprintf("/files/%s", "not-an-id"), ""), http.StatusUnauthorized)
	stranger := w.session(w.addUser("eve@elsewhere.test", "Eve Stranger"))
	eq(t, get(public, stranger), http.StatusOK)
	eq(t, get(private, stranger), http.StatusNotFound)
}
