package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"ricrac/app"
	"ricrac/github"
	"ricrac/server"
)

// GitHub through a fake GitHub API: connecting the app, webhooks, linking, the two-way issue sync
// and the import of existing issues.

const (
	webhookSecret = "whsec_test"
	installation  = 77
	repoID        = 700000001
)

var (
	keyOnce sync.Once
	appKey  *rsa.PrivateKey
)

func testAppKey(t *testing.T) *rsa.PrivateKey {
	keyOnce.Do(func() {
		var err error
		if appKey, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
			t.Fatal(err)
		}
	})
	return appKey
}

type fakeRequest struct {
	Method, Path, Query string
	Body                M
}

// fakeGitHub answers the API calls Ricrac makes and records them.
type fakeGitHub struct {
	server        *httptest.Server
	mu            sync.Mutex
	requests      []fakeRequest
	login         string
	installations []M
	repositories  []M
	issuePages    [][]M
	comments      map[int][]M
	// Refuse what the app does with its installation token.
	failing bool
}

var issuePath = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/(\d+)$`)

func newFakeGitHub(t *testing.T) *fakeGitHub {
	f := &fakeGitHub{
		login:         "Ann-GH",
		installations: []M{{"id": installation, "account": M{"login": "acme-org", "type": "Organization"}}},
		repositories: []M{{"id": repoID, "full_name": "unbindapp/unbind", "html_url": "https://github.com/unbindapp/unbind",
			"private": true, "has_issues": true, "default_branch": "master"}},
		comments: map[int][]M{},
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeGitHub) serve(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body M
	if r.Header.Get("Content-Type") == "application/json" {
		_ = json.Unmarshal(raw, &body)
	}
	if r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
		form, _ := url.ParseQuery(string(raw))
		body = M{"code": form.Get("code"), "client_id": form.Get("client_id")}
	}
	f.mu.Lock()
	f.requests = append(f.requests, fakeRequest{r.Method, r.URL.Path, r.URL.RawQuery, body})
	f.mu.Unlock()
	path := r.URL.Path
	answer := func(value any) { app.WriteJSON(w, http.StatusOK, value) }
	switch {
	case f.failing && strings.HasPrefix(path, "/repos/"):
		app.WriteJSON(w, http.StatusForbidden, M{"message": "Resource not accessible by integration"})
	case strings.HasSuffix(path, "/access_tokens"):
		answer(M{"token": "ghs_installation", "expires_at": time.Now().Add(time.Hour)})
	case path == "/login/oauth/access_token":
		if body["code"] != "good-code" {
			answer(M{"error": "bad_verification_code"})
			return
		}
		answer(M{"access_token": "gho_user"})
	case path == "/user":
		answer(M{"login": f.login})
	case path == "/user/installations":
		answer(M{"installations": f.installations})
	case path == "/installation/repositories":
		answer(M{"repositories": f.repositories})
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/comments"):
		number, _ := strconv.Atoi(strings.Split(path, "/")[5])
		answer(f.comments[number])
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/issues"):
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < len(f.issuePages) {
			w.Header().Set("Link", fmt.Sprintf(`<%s%s?page=%d>; rel="next"`, f.server.URL, path, page+1))
		}
		answer(f.issuePages[page-1])
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/comments"):
		answer(M{"id": 9001, "body": body["body"]})
	case r.Method == http.MethodPatch && issuePath.MatchString(path):
		answer(M{"id": 1, "number": 1, "updated_at": time.Now().UTC()})
	case r.Method == http.MethodDelete:
		w.WriteHeader(http.StatusNoContent)
	default:
		answer(M{})
	}
}

// sent is the requests made with method to paths containing part, oldest first.
func (f *fakeGitHub) sent(method, part string) []fakeRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	var found []fakeRequest
	for _, r := range f.requests {
		if r.Method == method && strings.Contains(r.Path, part) {
			found = append(found, r)
		}
	}
	return found
}

type gitHubWorld struct {
	*World
	fake   *fakeGitHub
	router http.Handler
}

func withGitHub(t *testing.T) *gitHubWorld {
	w := world(t)
	fake := newFakeGitHub(t)
	config := github.Config{
		AppID: 1, Slug: "ricrac-test", PrivateKey: testAppKey(t), WebhookSecret: webhookSecret,
		ClientID: "Iv1.test", ClientSecret: "client-secret", APIURL: fake.server.URL, WebURL: fake.server.URL,
	}
	w.State.Config.GitHub = &config
	w.State.GitHub = github.NewApp(config, fake.server.Client())
	return &gitHubWorld{w, fake, server.Router(w.State)}
}

// connect stores the installation as the setup callback does.
func (g *gitHubWorld) connect() {
	g.exec(`INSERT INTO github_installations (id, workspace_id, account_login, account_type, installer_id)
		VALUES ($1, $2, 'acme-org', 'Organization', $3)`, installation, g.Workspace, g.Ann.User)
}

// syncRepo connects GitHub and syncs unbindapp/unbind with the team.
func (g *gitHubWorld) syncRepo(t *testing.T, team string) J {
	g.connect()
	return call(t, g.Ann, "save_github_repo_sync", M{"repository": "unbindapp/unbind", "team": team, "backfill": false})
}

// deliver sends a webhook from testdata/github as GitHub does, for the installation.
func (g *gitHubWorld) deliver(t *testing.T, event, file string, edit func(M)) int {
	t.Helper()
	data, err := os.ReadFile("testdata/github/" + file)
	if err != nil {
		t.Fatal(err)
	}
	var payload M
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	payload["installation"] = M{"id": installation, "account": M{"login": "acme-org"}}
	if edit != nil {
		edit(payload)
	}
	body := app.Marshal(payload)
	req := httptest.NewRequest("POST", "/github/webhook", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", event)
	req.Header.Set("X-Hub-Signature-256", github.Sign(webhookSecret, body))
	status, answer := send(g.router, req)
	if status != http.StatusOK {
		t.Logf("%s answered %d: %s", file, status, answer.JSON())
	}
	return status
}

func (g *gitHubWorld) runJobs(t *testing.T) {
	t.Helper()
	for range 10 {
		busy, err := server.GitHubJobs(context.Background(), g.State)
		if err != nil {
			t.Fatalf("GitHub jobs: %v", err)
		}
		if !busy {
			return
		}
	}
	t.Fatal("the GitHub jobs never finished")
}

// browse is a GET through the router as a signed-in user, answering the redirect's location.
func (g *gitHubWorld) browse(t *testing.T, user uuid.UUID, uri string) string {
	t.Helper()
	req := httptest.NewRequest("GET", uri, nil)
	if user != uuid.Nil {
		req.AddCookie(&http.Cookie{Name: g.State.Keys.SessionCookie, Value: g.State.Keys.SessionToken(user)})
	}
	recorder := httptest.NewRecorder()
	g.router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("GET %s answered %d: %s", uri, recorder.Code, recorder.Body.String())
	}
	return recorder.Header().Get("Location")
}

func queryOf(t *testing.T, link string) url.Values {
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.Query()
}

func TestConnectingGitHubChecksTheInstallerCanReachTheInstallation(t *testing.T) {
	g := withGitHub(t)
	before := call(t, g.Ann, "get_github_integration", nil)
	check(t, before.At("configured").Bool() && before.At("installations").Len() == 0, before.JSON())
	bob := g.member("bob@acme.test", "Bob", "member")
	contains(t, fails(t, bob, "connect_github", nil), "admins")

	state := queryOf(t, call(t, g.Ann, "connect_github", nil).At("installUrl").Str()).Get("state")
	setup := func(user uuid.UUID, installationID, state string) string {
		return g.browse(t, user, "/api/github/setup?"+url.Values{
			"installation_id": {installationID}, "setup_action": {"install"}, "state": {state}, "code": {"good-code"},
		}.Encode())
	}
	eq(t, setup(g.Ann.User, "99", state), "/acme/settings/integrations?github_error=not_your_installation")
	eq(t, setup(bob.User, "77", state), "/acme/settings/integrations?github_error=signed_out")
	eq(t, setup(g.Ann.User, "77", state+"x"), "/?github_error=expired")
	old := g.State.Keys.Sign(fmt.Sprintf("github-install:%d:%s:%s", time.Now().Add(-time.Minute).Unix(), g.Workspace, g.Ann.User))
	eq(t, setup(g.Ann.User, "77", old), "/acme/settings/integrations?github_error=expired", "an old link still finds its workspace")
	eq(t, g.browse(t, g.Ann.User, "/api/github/callback?setup_action=request&state="+url.QueryEscape(state)),
		"/acme/settings/integrations?github_error=requested")
	eq(t, setup(g.Ann.User, "77", state), "/acme/settings/integrations")
	token := g.fake.sent("POST", "/login/oauth/access_token")
	eq(t, token[len(token)-1].Body["code"], any("good-code"))

	integration := call(t, g.Ann, "get_github_integration", nil)
	eq(t, integration.At("installations", 0, "account").Str(), "acme-org", integration.JSON())
	eq(t, integration.At("installations", 0, "installer", "email").Str(), "ann@acme.test")
	eq(t, integration.At("installations", 0, "manageUrl").Str(), g.fake.server.URL+"/organizations/acme-org/settings/installations/77")
	eq(t, integration.At("githubLogin").Str(), "ann-gh")

	repositories := call(t, g.Ann, "list_github_repositories", nil)
	eq(t, repositories.At("nodes", 0, "fullName").Str(), "unbindapp/unbind", repositories.JSON())
	check(t, repositories.At("nodes", 0, "repoSync").IsNull())
	contains(t, fails(t, g.Ann, "save_github_repo_sync", M{"repository": "other/repo", "team": "ENG"}), "can't see")
	synced := call(t, g.Ann, "save_github_repo_sync", M{"repository": "unbindapp/unbind", "team": "ENG", "backfill": false})
	eq(t, synced.At("repository", "id").Num(), float64(repoID))
	check(t, synced.At("backfill").IsNull(), synced.JSON())
	contains(t, fails(t, g.Ann, "save_github_repo_sync", M{"repository": "unbindapp/unbind", "team": "ENG"}), "already syncs")
	eq(t, call(t, g.Ann, "list_github_repositories", nil).At("nodes", 0, "repoSync").Str(), synced.At("id").Str())
	eq(t, call(t, g.Ann, "list_github_repo_syncs", nil).At("nodes").Len(), 1)

	eq(t, call(t, g.Ann, "disconnect_github", nil).At("disconnected").Str(), "acme-org")
	after := call(t, g.Ann, "get_github_integration", nil)
	check(t, after.At("installations").Len() == 0 && after.At("repoSyncs").Len() == 0, after.JSON())
	eq(t, scalar[int](g.World, "SELECT count(*) FROM audit_entries WHERE type LIKE 'github.%'"), 2)
}

func TestPeopleLinkTheirGitHubAccount(t *testing.T) {
	g := withGitHub(t)
	authorize := g.browse(t, g.Ann.User, "/api/github/connect?return_to=/acme/settings/account")
	check(t, strings.HasPrefix(authorize, g.fake.server.URL+"/login/oauth/authorize?"), authorize)
	query := queryOf(t, authorize)
	eq(t, query.Get("redirect_uri"), "http://localhost:3000/api/github/callback")
	bob := g.member("bob@acme.test", "Bob", "member")
	eq(t, g.browse(t, bob.User, "/api/github/callback?code=good-code&state="+url.QueryEscape(query.Get("state"))),
		"/acme/settings/account?github_error=expired")
	eq(t, g.browse(t, g.Ann.User, "/api/github/callback?code=bad&state="+url.QueryEscape(query.Get("state"))),
		"/acme/settings/account?github_error=failed")
	eq(t, g.browse(t, g.Ann.User, "/api/github/callback?code=good-code&state="+url.QueryEscape(query.Get("state"))), "/acme/settings/account")
	req := httptest.NewRequest("GET", "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: g.State.Keys.SessionCookie, Value: g.State.Keys.SessionToken(g.Ann.User)})
	_, me := send(g.router, req)
	eq(t, me.At("githubLogin").Str(), "ann-gh", me.JSON())
	check(t, call(t, g.Ann, "disconnect_github_account", nil).At("githubLogin").IsNull())
	check(t, call(t, g.Ann, "get_github_integration", nil).At("githubLogin").IsNull())
}

func TestGitHubWebhooksNeedTheirSignature(t *testing.T) {
	g := withGitHub(t)
	g.connect()
	body := []byte(`{"zen": "Keep it logically awesome.", "installation": {"id": 77}}`)
	for _, signature := range []string{"", github.Sign("wrong", body), "sha256=zz"} {
		req := httptest.NewRequest("POST", "/github/webhook", bytes.NewReader(body))
		req.Header.Set("X-GitHub-Event", "ping")
		req.Header.Set("X-Hub-Signature-256", signature)
		status, _ := send(g.router, req)
		eq(t, status, http.StatusUnauthorized)
	}
	check(t, call(t, g.Ann, "get_github_integration", nil).At("installations", 0, "lastEvent").IsNull())
	eq(t, g.deliver(t, "ping", "ping.json", nil), http.StatusOK)
	eq(t, call(t, g.Ann, "get_github_integration", nil).At("installations", 0, "lastEvent").Str(), "ping")
}

func TestGitHubInstallationEvents(t *testing.T) {
	g := withGitHub(t)
	g.syncRepo(t, "ENG")
	g.deliver(t, "installation", "ping.json", func(p M) { p["action"] = "suspend" })
	check(t, call(t, g.Ann, "get_github_integration", nil).At("installations", 0, "suspendedAt").IsString())
	g.deliver(t, "installation_repositories", "ping.json", func(p M) {
		p["action"] = "removed"
		p["repositories_removed"] = []M{{"id": repoID, "full_name": "unbindapp/unbind"}}
	})
	eq(t, call(t, g.Ann, "list_github_repo_syncs", nil).At("nodes").Len(), 0)
	g.deliver(t, "installation", "ping.json", func(p M) { p["action"] = "deleted" })
	eq(t, call(t, g.Ann, "get_github_integration", nil).At("installations").Len(), 0)
}

// numbered is a new issue of the team with this number, like UNB-455.
func numbered(t *testing.T, w *World, team string, number int) string {
	created := call(t, w.Ann, "save_issue", M{"team": team, "title": fmt.Sprintf("Issue %d", number)})
	w.exec("UPDATE issues SET number = $2 WHERE id = $1", created.At("id").Str(), number)
	return fmt.Sprintf("%s-%d", team, number)
}

func lastHistory(t *testing.T, w *World, issue string) J {
	history := call(t, w.Ann, "get_issue_history", M{"id": issue}).At("history")
	return history.At(history.Len() - 1)
}

func TestPullRequestsBranchesAndCommitsLinkAndMoveIssues(t *testing.T) {
	g := withGitHub(t)
	g.connect()
	g.createTeam("Unbind", "UNB")
	for _, n := range []int{455, 456, 12, 13, 14} {
		numbered(t, g.World, "UNB", n)
	}
	call(t, g.Ann, "save_team", M{"id": "UNB", "githubPrOpenedState": "In Review"})
	eq(t, call(t, g.Ann, "get_team", M{"id": "UNB"}).At("githubPrOpenedState", "name").Str(), "In Review")

	eq(t, g.deliver(t, "push", "push.json", nil), http.StatusOK)
	branch := call(t, g.Ann, "get_issue", M{"id": "UNB-456"})
	eq(t, branch.At("state", "name").Str(), "In Review", branch.JSON())
	eq(t, branch.At("attachments", 0, "sourceType").Str(), "github_branch")
	eq(t, branch.At("attachments", 0, "url").Str(), "https://github.com/unbindapp/unbind/tree/bbedward/unb-456-retry-deploys")
	check(t, branch.At("attachments", 0, "creator").IsNull())
	entry := lastHistory(t, g.World, "UNB-456")
	check(t, entry.At("actor").IsNull(), entry.JSON())
	eq(t, entry.At("changes", "github", "label").Str(), "Branch bbedward/unb-456-retry-deploys pushed")
	eq(t, entry.At("changes", "state", "to", "name").Str(), "In Review")

	g.deliver(t, "pull_request", "pull_request_opened.json", nil)
	pr := call(t, g.Ann, "get_issue", M{"id": "UNB-455"})
	eq(t, pr.At("state", "name").Str(), "In Review")
	eq(t, pr.At("attachments", 0, "sourceType").Str(), "github")
	eq(t, pr.At("attachments", 0, "title").Str(), "#42 Limit websocket message size")
	eq(t, pr.At("attachments", 0, "metadata", "state").Str(), "open")
	eq(t, pr.At("attachments", 0, "metadata", "number").Num(), 42.0)
	// A push to the pull request's branch doesn't add the branch too.
	g.deliver(t, "push", "push.json", func(p M) { p["ref"] = "refs/heads/bbedward/unb-455-websocket-limit" })
	eq(t, call(t, g.Ann, "get_issue", M{"id": "UNB-455"}).At("attachments").Len(), 1)

	g.deliver(t, "pull_request", "pull_request_closed_merged.json", nil)
	merged := call(t, g.Ann, "get_issue", M{"id": "UNB-455"})
	eq(t, merged.At("state", "name").Str(), "Done", merged.JSON())
	eq(t, merged.At("attachments", 0, "metadata", "state").Str(), "merged")
	eq(t, lastHistory(t, g.World, "UNB-455").At("changes", "github", "label").Str(), "Pull request #42 merged")

	// A commit that fixes issues only closes them on the default branch.
	g.deliver(t, "push", "push_commits_feature.json", nil)
	feature := call(t, g.Ann, "get_issue", M{"id": "UNB-12"})
	eq(t, feature.At("state", "name").Str(), "In Review", feature.JSON())
	eq(t, feature.At("attachments", 0, "sourceType").Str(), "github_commit")
	eq(t, feature.At("attachments", 0, "title").Str(), "3e1f0c9 Cap websocket frames at 1 MiB")
	g.deliver(t, "push", "push_commits_master.json", nil)
	for _, closed := range []string{"UNB-12", "UNB-13"} {
		eq(t, call(t, g.Ann, "get_issue", M{"id": closed}).At("state", "name").Str(), "Done", closed)
	}
	eq(t, lastHistory(t, g.World, "UNB-13").At("changes", "github", "label").Str(), "Commit 3e1f0c9 on master closed the issue")
	eq(t, call(t, g.Ann, "get_issue", M{"id": "UNB-14"}).At("state", "name").Str(), "In Review")
	eq(t, scalar[int](g.World, "SELECT count(*) FROM github_outbox"), 0)
}

func TestGitHubIssuesAndCommentsSyncIntoRicrac(t *testing.T) {
	g := withGitHub(t)
	g.syncRepo(t, "ENG")
	eve := g.member("eve@acme.test", "Eve", "member")
	g.exec("UPDATE users SET github_login = 'eve-gh' WHERE id = $1", eve.User)

	for range 2 {
		eq(t, g.deliver(t, "issues", "issues_opened.json", nil), http.StatusOK)
	}
	issues := call(t, g.Ann, "list_issues", M{"includeDescription": true})
	eq(t, issues.At("nodes").Len(), 1, "deliveries are idempotent")
	issue := issues.At("nodes", 0)
	eq(t, issue.At("title").Str(), "Deploy logs stop streaming after 10 minutes")
	eq(t, issue.At("description").Str(), "The log stream closes with **1006** after about ten minutes.")
	eq(t, issue.At("labels", 0, "name").Str(), "Bug", "GitHub labels use Ricrac's of the same name")
	eq(t, issue.At("assignee", "email").Str(), "eve@acme.test")
	check(t, issue.At("creator").IsNull(), issue.JSON())
	created := lastHistory(t, g.World, "ENG-1")
	eq(t, created.At("changes", "github", "label").Str(), "Synced from GitHub #1347 by @octocat")
	eq(t, created.At("changes", "github", "url").Str(), "https://github.com/unbindapp/unbind/issues/1347")

	g.deliver(t, "issues", "issues_labeled.json", nil)
	labeled := call(t, g.Ann, "get_issue", M{"id": "ENG-1"}).At("labels")
	eqList(t, labeled.Strings("name"), []string{"Bug", "websocket"})
	eq(t, labeled.At(1, "color").Str(), "#0e8a16")
	g.deliver(t, "issues", "issues_edited_stale.json", nil)
	eq(t, call(t, g.Ann, "get_issue", M{"id": "ENG-1"}).At("title").Str(), "Deploy logs stop streaming after 10 minutes")
	g.deliver(t, "issues", "issues_edited_stale.json", func(p M) { p["issue"].(M)["updated_at"] = "2026-10-16T10:30:00Z" })
	eq(t, call(t, g.Ann, "get_issue", M{"id": "ENG-1"}).At("title").Str(), "Deploy logs stop streaming after 10 minutes on Firefox")
	g.deliver(t, "issues", "issues_closed_not_planned.json", nil)
	eq(t, call(t, g.Ann, "get_issue", M{"id": "ENG-1"}).At("state", "type").Str(), "canceled")
	g.deliver(t, "issues", "issues_reopened.json", nil)
	eq(t, call(t, g.Ann, "get_issue", M{"id": "ENG-1"}).At("state", "type").Str(), "backlog")
	g.deliver(t, "issues", "issues_reopened.json", func(p M) {
		p["action"] = "unassigned"
		p["assignee"] = M{"login": "Eve-GH"}
	})
	check(t, call(t, g.Ann, "get_issue", M{"id": "ENG-1"}).At("assignee").IsNull())

	for range 2 {
		g.deliver(t, "issue_comment", "issue_comment_created.json", nil)
	}
	comments := call(t, g.Ann, "list_comments", M{"issueId": "ENG-1"}).At("nodes")
	eq(t, comments.Len(), 1, comments.JSON())
	eq(t, comments.At(0, "body").Str(), "**@octocat on GitHub**\n\nSame here on the staging cluster.")
	check(t, comments.At(0, "user").IsNull())
	g.deliver(t, "issue_comment", "issue_comment_edited.json", nil)
	eq(t, call(t, g.Ann, "list_comments", M{"issueId": "ENG-1"}).At("nodes", 0, "body").Str(),
		"**@octocat on GitHub**\n\nSame here on the staging and production clusters.")
	g.deliver(t, "issue_comment", "issue_comment_created.json", func(p M) {
		comment := p["comment"].(M)
		comment["id"], comment["user"], comment["body"] = 2400000002, M{"login": "Eve-GH"}, "Looking into it."
		comment["created_at"] = "2026-10-16T10:20:00Z"
	})
	byEve := call(t, g.Ann, "list_comments", M{"issueId": "ENG-1"}).At("nodes", 1)
	eq(t, byEve.At("body").Str(), "Looking into it.")
	eq(t, byEve.At("user", "email").Str(), "eve@acme.test")
	g.deliver(t, "issue_comment", "issue_comment_deleted.json", nil)
	eq(t, call(t, g.Ann, "list_comments", M{"issueId": "ENG-1"}).At("nodes").Len(), 1)
	eq(t, scalar[int](g.World, "SELECT count(*) FROM github_outbox"), 0)
	check(t, call(t, g.Ann, "list_github_repo_syncs", nil).At("nodes", 0, "lastError").IsNull())

	g.deliver(t, "issues", "issues_deleted.json", nil)
	check(t, call(t, g.Ann, "get_issue", M{"id": "ENG-1"}).At("trashed").Bool())
	eq(t, scalar[int](g.World, "SELECT count(*) FROM github_issue_links"), 0)
}

func TestRicracChangesGoToGitHubButNotBack(t *testing.T) {
	g := withGitHub(t)
	g.syncRepo(t, "ENG")
	g.exec("UPDATE users SET github_login = 'ann-gh' WHERE id = $1", g.Ann.User)
	g.deliver(t, "issues", "issues_opened.json", func(p M) { p["issue"].(M)["assignee"] = nil })

	call(t, g.Ann, "save_issue", M{"id": "ENG-1", "title": "Logs stop after ten minutes", "priority": 2})
	call(t, g.Ann, "save_issue", M{"id": "ENG-1", "state": "Done", "assignee": "me", "addLabels": []string{"bug"}})
	call(t, g.Ann, "save_issue_label", M{"name": "ui"})
	call(t, g.Ann, "save_issue", M{"id": "ENG-1", "addLabels": []string{"ui"}, "removeLabels": []string{"bug"}})
	g.runJobs(t)
	patches := g.fake.sent("PATCH", "/repos/unbindapp/unbind/issues/1347")
	eq(t, len(patches), 2)
	eq(t, string(app.Marshal(patches[0].Body)), `{"title":"Logs stop after ten minutes"}`)
	eq(t, string(app.Marshal(patches[1].Body)), `{"state":"closed","state_reason":"completed"}`)
	eq(t, string(app.Marshal(g.fake.sent("POST", "/issues/1347/assignees")[0].Body)), `{"assignees":["ann-gh"]}`)
	eq(t, string(app.Marshal(g.fake.sent("POST", "/issues/1347/labels")[0].Body)), `{"labels":["ui"]}`)
	eq(t, len(g.fake.sent("DELETE", "/issues/1347/labels/Bug")), 1)
	eq(t, scalar[int](g.World, "SELECT count(*) FROM github_outbox"), 0)

	comment := call(t, g.Ann, "save_comment", M{"issueId": "ENG-1", "body": "Fixed by raising the idle timeout."})
	g.runJobs(t)
	posted := g.fake.sent("POST", "/issues/1347/comments")
	eq(t, len(posted), 1)
	eq(t, posted[0].Body["body"], any("**Ann Smith on Ricrac**\n\nFixed by raising the idle timeout."))
	call(t, g.Ann, "save_comment", M{"id": comment.At("id").Str(), "body": "Fixed by raising the timeout."})
	g.runJobs(t)
	edited := g.fake.sent("PATCH", "/issues/comments/9001")
	eq(t, edited[0].Body["body"], any("**Ann Smith on Ricrac**\n\nFixed by raising the timeout."))

	// GitHub tells Ricrac about what the app did; none of it comes back.
	g.deliver(t, "issue_comment", "issue_comment_created.json", func(p M) {
		p["comment"].(M)["id"], p["comment"].(M)["body"] = 9001, posted[0].Body["body"]
		p["sender"] = M{"login": "ricrac-test[bot]", "type": "Bot"}
	})
	g.deliver(t, "issue_comment", "issue_comment_created.json", func(p M) {
		p["comment"].(M)["id"], p["comment"].(M)["body"] = 9002, posted[0].Body["body"]
	})
	g.deliver(t, "issues", "issues_edited_stale.json", func(p M) {
		p["issue"].(M)["updated_at"] = time.Now().UTC().Add(time.Hour)
		p["sender"] = M{"login": "ricrac-test[bot]", "type": "Bot"}
	})
	eq(t, call(t, g.Ann, "list_comments", M{"issueId": "ENG-1"}).At("nodes").Len(), 1)
	eq(t, call(t, g.Ann, "get_issue", M{"id": "ENG-1"}).At("title").Str(), "Logs stop after ten minutes")

	// Changes from GitHub aren't sent back.
	g.deliver(t, "issues", "issues_closed_not_planned.json", func(p M) { p["issue"].(M)["updated_at"] = time.Now().UTC().Add(time.Hour) })
	eq(t, call(t, g.Ann, "get_issue", M{"id": "ENG-1"}).At("state", "type").Str(), "canceled")
	eq(t, scalar[int](g.World, "SELECT count(*) FROM github_outbox"), 0)

	call(t, g.Ann, "delete_comment", M{"id": comment.At("id").Str()})
	g.runJobs(t)
	eq(t, len(g.fake.sent("DELETE", "/issues/comments/9001")), 1)
	check(t, call(t, g.Ann, "list_github_repo_syncs", nil).At("nodes", 0, "lastError").IsNull())

	call(t, g.Ann, "delete_github_repo_sync", M{"id": "unbindapp/unbind"})
	call(t, g.Ann, "save_issue", M{"id": "ENG-1", "title": "No longer synced"})
	eq(t, scalar[int](g.World, "SELECT count(*) FROM github_outbox"), 0)
}

func TestFailedPushesAreRetriedAndReported(t *testing.T) {
	g := withGitHub(t)
	g.syncRepo(t, "ENG")
	g.deliver(t, "issues", "issues_opened.json", nil)
	g.fake.failing = true
	call(t, g.Ann, "save_issue", M{"id": "ENG-1", "title": "Renamed"})
	g.runJobs(t)
	eq(t, call(t, g.Ann, "list_github_repo_syncs", nil).At("nodes", 0, "lastError").Str(), "GitHub answered 403: Resource not accessible by integration")
	eq(t, scalar[int](g.World, "SELECT attempts FROM github_outbox"), 1)
}

func TestBackfillImportsARepositorysIssues(t *testing.T) {
	g := withGitHub(t)
	g.connect()
	issue := func(id, number int, title string, extra M) M {
		value := M{"id": id, "node_id": fmt.Sprintf("I_%d", id), "number": number, "title": title, "body": "", "state": "open",
			"html_url": fmt.Sprintf("https://github.com/unbindapp/unbind/issues/%d", number), "user": M{"login": "octocat"},
			"labels": []M{}, "comments": 0, "created_at": "2026-10-01T10:00:00Z", "updated_at": "2026-10-01T10:00:00Z"}
		for key, v := range extra {
			value[key] = v
		}
		return value
	}
	g.fake.issuePages = [][]M{
		{issue(1, 1, "First", nil), issue(2, 2, "A pull request", M{"pull_request": M{"url": "x"}})},
		{issue(3, 3, "Closed one", M{"state": "closed", "state_reason": "completed", "comments": 1})},
	}
	g.fake.comments[3] = []M{{"id": 501, "body": "Done in v2.", "user": M{"login": "octocat"}, "html_url": "https://github.com/x",
		"created_at": "2026-10-02T10:00:00Z", "updated_at": "2026-10-02T10:00:00Z"}}
	started := call(t, g.Ann, "save_github_repo_sync", M{"repository": "unbindapp/unbind", "team": "ENG", "includeClosed": true})
	eq(t, started.At("backfill", "status").Str(), "pending", started.JSON())
	g.runJobs(t)
	listed := g.fake.sent("GET", "/repos/unbindapp/unbind/issues")
	contains(t, listed[0].Query, "state=all", "page=1")
	done := call(t, g.Ann, "list_github_repo_syncs", nil).At("nodes", 0)
	eq(t, done.At("backfill", "status").Str(), "completed", done.JSON())
	eq(t, done.At("backfill", "issuesCreated").Num(), 2.0)
	eq(t, done.At("backfill", "commentsCreated").Num(), 1.0)
	eq(t, done.At("backfill", "skipped").Num(), 1.0)
	eq(t, done.At("linkedIssueCount").Num(), 2.0)
	closed := call(t, g.Ann, "get_issue", M{"id": "ENG-2", "includeComments": true})
	eq(t, closed.At("title").Str(), "Closed one")
	eq(t, closed.At("state", "type").Str(), "completed")
	eq(t, closed.At("comments", 0, "body").Str(), "**@octocat on GitHub**\n\nDone in v2.")
	contains(t, closed.At("comments", 0, "createdAt").Str(), "2026-10-02")

	g.fake.issuePages = [][]M{{issue(1, 1, "First, renamed", M{"updated_at": "2026-10-03T10:00:00Z"})}}
	again := call(t, g.Ann, "save_github_repo_sync", M{"id": started.At("id").Str(), "backfill": true})
	eq(t, again.At("backfill", "issuesCreated").Num(), 0.0)
	g.runJobs(t)
	eq(t, call(t, g.Ann, "list_github_repo_syncs", nil).At("nodes", 0, "backfill", "issuesUpdated").Num(), 1.0)
	eq(t, call(t, g.Ann, "get_issue", M{"id": "ENG-1"}).At("title").Str(), "First, renamed")
	eq(t, scalar[int](g.World, "SELECT count(*) FROM github_outbox"), 0)
}
