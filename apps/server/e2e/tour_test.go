package e2e

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Tours of the server's surface, there for their recordings: every operation through every door
// with good and bad arguments, the dashboard's API, MCP and the documents agents read.

type hit struct {
	method, path string
	body         any
	raw          string
	headers      map[string]string
}

func (w *World) do(h hit) (int, J) {
	var reader *strings.Reader
	switch {
	case h.raw != "":
		reader = strings.NewReader(h.raw)
	case h.body != nil:
		reader = strings.NewReader(string(marshal(h.body)))
	default:
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(h.method, h.path, reader)
	if h.body != nil || h.raw != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for name, value := range h.headers {
		req.Header.Set(name, value)
	}
	return send(w.Router, req)
}

func bearer(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func (w *World) dashboard(user uuid.UUID, workspace string) map[string]string {
	headers := map[string]string{"Cookie": w.session(user), "X-Ricrac": "1"}
	if workspace != "" {
		headers["X-Ricrac-Workspace"] = workspace
	}
	return headers
}

func TestToursTheDocumentsAgentsRead(t *testing.T) {
	w := world(t)
	for _, path := range []string{
		"/v1", "/v1/", "/v1/openapi.json", "/v1/ops", "/llms.txt", "/healthz", "/api/config",
		"/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp",
		"/.well-known/oauth-authorization-server", "/.well-known/oauth-authorization-server/mcp",
		"/v1/nope", "/acme/issue/ENG-1", "/favicon.ico", "/assets/missing.js",
	} {
		w.do(hit{method: "GET", path: path})
	}
	w.do(hit{method: "OPTIONS", path: "/v1/issues", headers: map[string]string{
		"Origin": "https://agent.example", "Access-Control-Request-Method": "POST", "Access-Control-Request-Headers": "authorization"}})
	w.do(hit{method: "OPTIONS", path: "/mcp", headers: map[string]string{"Origin": "https://agent.example", "Access-Control-Request-Method": "POST"}})
	w.do(hit{method: "GET", path: "/v1/issues", headers: map[string]string{"Origin": "https://agent.example"}})
	w.do(hit{method: "GET", path: "/api/me", headers: map[string]string{"Origin": "https://agent.example"}})
}

func TestToursMCP(t *testing.T) {
	w := world(t)
	call(t, w.Ann, "save_issue", M{"title": "Seed"})
	read, write, admin := apiKey(w, "read"), apiKey(w, "read", "write"), apiKey(w, "read", "write", "admin")
	for _, version := range []string{"2025-03-26", "2025-06-18", "2025-11-25", "2026-07-28"} {
		for _, key := range []string{read, write, admin} {
			for _, path := range []string{"/mcp", "/mcp?tools=all"} {
				req := rpc(path, key, M{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": M{
					"protocolVersion": version, "capabilities": M{}, "clientInfo": M{"name": "tour", "version": "1"}}})
				req.Header.Set("Mcp-Protocol-Version", version)
				send(w.Router, req)
				req = rpc(path, key, M{"jsonrpc": "2.0", "id": 2, "method": "tools/list"})
				req.Header.Set("Mcp-Protocol-Version", version)
				send(w.Router, req)
			}
		}
	}
	for _, method := range []string{"resources/list", "prompts/list", "resources/templates/list", "logging/setLevel", "completion/complete", "server/discover", "ping"} {
		send(w.Router, rpc("/mcp", admin, M{"jsonrpc": "2.0", "id": 3, "method": method, "params": M{}}))
	}
	for _, query := range []string{"", "webhook", "labels", "customer needs", "zzz", "release pipeline", "favorite"} {
		send(w.Router, rpc("/mcp", admin, M{"jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": M{
			"name": "find_operations", "arguments": M{"query": query}}}))
	}
	for _, args := range []M{
		{"name": "get_issue", "arguments": M{"id": "ENG-1"}},
		{"name": "nope", "arguments": M{}},
		{"name": "save_webhook", "arguments": M{"url": "nope"}},
		{"name": "list_issues"},
		{"arguments": M{}},
	} {
		send(w.Router, rpc("/mcp", admin, M{"jsonrpc": "2.0", "id": 5, "method": "tools/call", "params": M{"name": "call_operation", "arguments": args}}))
	}
	for _, params := range []M{
		{"name": "nope", "arguments": M{}},
		{"name": "get_issue", "arguments": M{"id": "ENG-1", "extra": true}},
		{"name": "get_issue", "arguments": M{"id": 7}},
		{"name": "get_issue"},
		{"name": "save_issue", "arguments": M{"title": "Through MCP", "labels": []any{"Bug"}}},
		{"name": "delete_issue", "arguments": M{"id": "ENG-2"}},
	} {
		send(w.Router, rpc("/mcp", write, M{"jsonrpc": "2.0", "id": 6, "method": "tools/call", "params": params}))
	}
	for _, raw := range []string{"", "{", "[]", `{"jsonrpc":"2.0"}`, `{"jsonrpc":"1.0","id":1,"method":"ping"}`, `{"id":1,"method":"ping"}`} {
		req := httptest.NewRequest("POST", "/mcp", strings.NewReader(raw))
		req.Host = "localhost:3000"
		req.Header.Set("Authorization", "Bearer "+read)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		send(w.Router, req)
	}
	for _, method := range []string{"GET", "DELETE", "PUT"} {
		req := httptest.NewRequest(method, "/mcp", nil)
		req.Host = "localhost:3000"
		req.Header.Set("Authorization", "Bearer "+read)
		send(w.Router, req)
	}
	noAccept := rpc("/mcp", read, M{"jsonrpc": "2.0", "id": 1, "method": "ping"})
	noAccept.Header.Del("Accept")
	send(w.Router, noAccept)
	stream := rpc("/mcp", read, M{"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
	stream.Header.Set("Accept", "text/event-stream")
	send(w.Router, stream)
	send(w.Router, rpc("/mcp", "rr_key_wrong", M{"jsonrpc": "2.0", "id": 1, "method": "ping"}))
	w.secondWorkspace("Beta", "beta", "OPS")
	all := keyReaching(w, nil, "read", "write")
	send(w.Router, rpc("/mcp", all, M{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}))
	send(w.Router, rpc("/mcp", all, M{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": M{"name": "list_issues", "arguments": M{}}}))
	send(w.Router, rpc("/mcp", all, M{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": M{"name": "list_issues", "arguments": M{"workspace": "beta"}}}))
}

// opsListed is every operation as GET /v1/ops lists it.
func opsListed(t *testing.T, w *World) []J {
	status, listed := w.do(hit{method: "GET", path: "/v1/ops"})
	eq(t, status, http.StatusOK)
	return listed.At("operations").List()
}

func TestToursEveryOperationWithBadArguments(t *testing.T) {
	w := world(t)
	call(t, w.Ann, "save_issue", M{"title": "Seed", "labels": []any{"Bug"}})
	key := apiKey(w, "read", "write", "admin")
	readOnly := apiKey(w, "read")
	for _, op := range opsListed(t, w) {
		name := op.At("name").Str()
		path := "/v1/ops/" + name
		w.do(hit{method: "POST", path: path, raw: `{"definitelyNotAnArgument": 1}`, headers: bearer(key)})
		w.do(hit{method: "POST", path: path, raw: `[1]`, headers: bearer(key)})
		properties := op.At("arguments", "properties")
		for _, property := range slices.Sorted(func(yield func(string) bool) {
			for name := range properties.V.(map[string]any) {
				if !yield(name) {
					return
				}
			}
		}) {
			wrong := M{property: M{"wrong": []any{true}}}
			if kinds := properties.At(property, "type"); kinds.Str() == "object" {
				wrong = M{property: 12.5}
			}
			w.do(hit{method: "POST", path: path, body: wrong, headers: bearer(key)})
		}
		if op.At("access").Str() != "read" {
			w.do(hit{method: "POST", path: path, body: M{}, headers: bearer(readOnly)})
		}
	}
}

func TestToursEveryReadThroughItsRoutes(t *testing.T) {
	w := world(t)
	bob := w.member("bob@acme.test", "Bob Jones", "member")
	call(t, w.Ann, "save_issue", M{"title": "Seed", "labels": []any{"Bug"}, "assignee": "bob", "description": "Hello @bob"})
	call(t, w.Ann, "save_comment", M{"issueId": "ENG-1", "body": "First"})
	call(t, w.Ann, "save_project", M{"name": "Launch", "addTeams": []any{"ENG"}})
	call(t, w.Ann, "save_document", M{"title": "Runbook", "content": "Steps", "project": "Launch"})
	key := apiKey(w, "read", "write", "admin")
	for _, op := range opsListed(t, w) {
		if op.At("access").Str() != "read" {
			continue
		}
		for _, route := range op.At("routes").Strings() {
			method, path, _ := strings.Cut(route, " ")
			if strings.Contains(path, "{") {
				path = strings.NewReplacer("{id}", "ENG-1", "{issue}", "ENG-1").Replace(path)
			}
			w.do(hit{method: method, path: path, headers: bearer(key)})
			w.do(hit{method: method, path: path + "?limit=1", headers: bearer(key)})
		}
		w.do(hit{method: "POST", path: "/v1/ops/" + op.At("name").Str(), body: M{}, headers: bearer(key)})
	}
	w.do(hit{method: "GET", path: "/v1/issues?filter=" + url.QueryEscape(`{"priority":{"gte":1}}`) + "&orderBy=updatedAt&limit=5", headers: bearer(key)})
	w.do(hit{method: "GET", path: "/v1/issues?labels=Bug,Feature&assignee=bob", headers: bearer(key)})
	w.do(hit{method: "GET", path: "/v1/issues?labels[]=Bug&labels[]=Feature", headers: bearer(key)})
	w.do(hit{method: "GET", path: "/v1/issues?includeDescription=1", headers: bearer(key)})
	w.do(hit{method: "GET", path: "/v1/issues?limit=abc", headers: bearer(key)})
	w.do(hit{method: "GET", path: "/v1/issues?cursor=nope", headers: bearer(key)})
	w.do(hit{method: "GET", path: "/v1/issues?query=seed+issue", headers: bearer(key)})
	w.do(hit{method: "GET", path: "/v1/search?query=seed", headers: bearer(key)})
	call(t, w.Ann, "search", M{"query": "runbook"})
	call(t, w.Ann, "search", M{"query": "first", "types": []any{"comment", "issue"}})
	call(t, w.Ann, "share_issue", M{"id": "ENG-1", "user": "bob"})
	call(t, w.Ann, "unshare_issue", M{"id": "ENG-1", "user": "bob"})
	_, _ = tryCall(bob, "unshare_issue", M{"id": "ENG-1", "user": "ann"})
	w.do(hit{method: "PATCH", path: "/v1/issues/ENG-1", body: M{"priority": 9}, headers: bearer(key)})
	w.do(hit{method: "PATCH", path: "/v1/issues/ENG-1", raw: "{nope", headers: bearer(key)})
	w.do(hit{method: "PATCH", path: "/v1/issues/ENG-1", raw: `"text"`, headers: bearer(key)})
	w.do(hit{method: "DELETE", path: "/v1/issues/ENG-404", headers: bearer(key)})
	w.do(hit{method: "POST", path: "/v1/ops/save_issue", raw: strings.Repeat(" ", 9<<20), headers: bearer(key)})
	w.do(hit{method: "POST", path: "/v1/ops/list_issues", body: M{}, headers: map[string]string{"Cookie": w.session(w.Ann.User)}})
	w.do(hit{method: "GET", path: "/v1/issues", headers: map[string]string{"Cookie": w.session(w.Ann.User)}})
	w.do(hit{method: "GET", path: "/v1/issues", headers: map[string]string{"Cookie": "rr_session=forged.value"}})
	w.do(hit{method: "GET", path: "/v1/issues", headers: map[string]string{"Authorization": "Basic abc"}})
	w.do(hit{method: "GET", path: "/v1/issues"})
}

func TestToursTheDashboardAPI(t *testing.T) {
	w := world(t)
	ann := w.dashboard(w.Ann.User, "")
	w.do(hit{method: "GET", path: "/api/me"})
	w.do(hit{method: "GET", path: "/api/me", headers: map[string]string{"Cookie": w.session(w.Ann.User)}})
	w.do(hit{method: "GET", path: "/api/me", headers: ann})
	w.do(hit{method: "PATCH", path: "/api/me", body: M{"name": "Ann Q. Smith"}, headers: map[string]string{"Cookie": w.session(w.Ann.User)}})
	w.do(hit{method: "PATCH", path: "/api/me", body: M{"name": "Ann Q. Smith", "displayName": "annie"}, headers: ann})
	w.do(hit{method: "PATCH", path: "/api/me", body: M{"name": "  "}, headers: ann})
	w.do(hit{method: "PATCH", path: "/api/me", body: M{"nope": 1}, headers: ann})

	w.do(hit{method: "POST", path: "/api/workspaces", body: M{"name": "Beta", "urlKey": "beta", "teamName": "Operations", "teamKey": "OPS"}, headers: ann})
	w.do(hit{method: "POST", path: "/api/workspaces", body: M{"name": "Beta again", "urlKey": "beta"}, headers: ann})
	w.do(hit{method: "POST", path: "/api/workspaces", body: M{"name": "Bad", "urlKey": "Not A Key!"}, headers: ann})
	w.do(hit{method: "POST", path: "/api/workspaces", body: M{"name": "", "urlKey": "empty"}, headers: ann})
	_, created := w.do(hit{method: "POST", path: "/api/workspaces", body: M{"name": "Gamma", "urlKey": "gamma"}, headers: ann})
	gamma := created.At("id").Str()
	w.do(hit{method: "GET", path: "/api/me", headers: ann})

	call(t, w.Ann, "invite_user", M{"email": "carol@acme.test", "role": "member"})
	call(t, w.Ann, "invite_user", M{"email": "carol@acme.test", "role": "admin"})
	carolID := w.addUser("carol@acme.test", "Carol Diaz")
	carol := w.dashboard(carolID, "")
	_, me := w.do(hit{method: "GET", path: "/api/me", headers: carol})
	invitation := me.At("invitations", 0, "id").Str()
	w.do(hit{method: "POST", path: "/api/invitations/" + uuid.NewString() + "/accept", headers: carol})
	w.do(hit{method: "POST", path: "/api/invitations/" + invitation + "/accept", headers: carol})
	w.do(hit{method: "POST", path: "/api/invitations/" + invitation + "/accept", headers: carol})
	call(t, w.Ann, "invite_user", M{"email": "dan@acme.test", "role": "guest", "teams": []any{"ENG"}})
	danID := w.addUser("dan@acme.test", "Dan")
	dan := w.dashboard(danID, "")
	_, me = w.do(hit{method: "GET", path: "/api/me", headers: dan})
	w.do(hit{method: "DELETE", path: "/api/invitations/" + me.At("invitations", 0, "id").Str(), headers: dan})
	w.do(hit{method: "GET", path: "/api/me", headers: dan})

	for _, body := range []M{
		{"label": "all", "scopes": []string{"read", "write"}},
		{"label": "one", "scopes": []string{"read"}, "workspaceIds": []string{w.Workspace.String()}},
		{"label": "none", "scopes": []string{"read"}, "workspaceIds": []string{}},
		{"label": "bad", "scopes": []string{"everything"}},
		{"label": "", "scopes": []string{"read"}},
		{"label": "elsewhere", "scopes": []string{"read"}, "workspaceIds": []string{uuid.NewString()}},
	} {
		w.do(hit{method: "POST", path: "/api/api-keys", body: body, headers: ann})
	}
	_, keys := w.do(hit{method: "GET", path: "/api/api-keys", headers: ann})
	w.do(hit{method: "DELETE", path: "/api/api-keys/" + keys.At(0, "id").Str(), headers: ann})
	w.do(hit{method: "DELETE", path: "/api/api-keys/" + uuid.NewString(), headers: ann})
	w.do(hit{method: "GET", path: "/api/api-keys", headers: ann})
	w.do(hit{method: "GET", path: "/api/api-keys", headers: map[string]string{"Cookie": w.session(w.Ann.User)}})

	w.do(hit{method: "GET", path: "/api/oauth-apps", headers: ann})
	acme := w.dashboard(w.Ann.User, "acme")
	w.do(hit{method: "GET", path: "/api/oauth-apps", headers: acme})
	for _, body := range []M{
		{"name": "Agent", "description": "Runs things", "homepageUrl": "https://agent.example", "redirectUris": []string{"https://agent.example/callback"}},
		{"name": "No redirect", "redirectUris": []string{}},
		{"name": "Fragment", "redirectUris": []string{"https://agent.example/cb#x"}},
		{"name": "Plain http", "redirectUris": []string{"http://agent.example/cb"}},
		{"name": "", "redirectUris": []string{"https://agent.example/cb"}},
	} {
		w.do(hit{method: "POST", path: "/api/oauth-apps", body: body, headers: acme})
	}
	_, apps := w.do(hit{method: "GET", path: "/api/oauth-apps", headers: acme})
	appID := apps.At(0, "id").Str()
	w.do(hit{method: "PATCH", path: "/api/oauth-apps/" + appID, body: M{"name": "Agent 2", "redirectUris": []string{"https://agent.example/callback", "http://localhost:9999/cb"}}, headers: acme})
	w.do(hit{method: "POST", path: "/api/oauth-apps/" + appID + "/secret", headers: acme})
	w.do(hit{method: "PATCH", path: "/api/oauth-apps/nope", body: M{"name": "x", "redirectUris": []string{"https://x.example"}}, headers: acme})

	verifier := randomSecret("")
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	query := url.Values{"client_id": {appID}, "redirect_uri": {"https://agent.example/callback"}, "response_type": {"code"},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"}, "state": {"s1"}, "scope": {"read write"}}
	w.do(hit{method: "GET", path: "/api/oauth/client?" + query.Encode(), headers: ann})
	w.do(hit{method: "GET", path: "/api/oauth/client?client_id=" + appID, headers: ann})
	w.do(hit{method: "GET", path: "/api/oauth/client?client_id=nope&redirect_uri=x&response_type=code", headers: ann})
	request := M{"client_id": appID, "redirect_uri": "https://agent.example/callback", "response_type": "code",
		"code_challenge": challenge, "code_challenge_method": "S256", "state": "s1", "scope": "read write"}
	w.do(hit{method: "POST", path: "/api/oauth/decide", body: M{"request": request, "workspaceIds": []string{w.Workspace.String()}, "allow": false}, headers: ann})
	_, decided := w.do(hit{method: "POST", path: "/api/oauth/decide", body: M{"request": request, "workspaceIds": []string{w.Workspace.String()}, "allow": true}, headers: ann})
	redirect, _ := url.Parse(decided.At("redirect").Str())
	w.do(hit{method: "POST", path: "/api/oauth/decide", body: M{"request": request, "workspaceIds": nil, "allow": true}, headers: ann})
	_, rotated := w.do(hit{method: "POST", path: "/api/oauth-apps/" + appID + "/secret", headers: acme})
	form := url.Values{"grant_type": {"authorization_code"}, "code": {redirect.Query().Get("code")}, "client_id": {appID},
		"client_secret": {rotated.At("clientSecret").Str()}, "redirect_uri": {"https://agent.example/callback"}, "code_verifier": {verifier}}
	send(w.Router, tokenRequest(form))
	w.do(hit{method: "GET", path: "/api/connected-apps", headers: ann})
	_, connected := w.do(hit{method: "GET", path: "/api/connected-apps", headers: ann})
	w.do(hit{method: "DELETE", path: "/api/connected-apps/" + connected.At(0, "id").Str(), headers: ann})
	w.do(hit{method: "DELETE", path: "/api/connected-apps/" + uuid.NewString(), headers: ann})
	w.do(hit{method: "DELETE", path: "/api/oauth-apps/" + appID, headers: acme})
	w.do(hit{method: "GET", path: "/api/oauth-apps", headers: acme})

	w.do(hit{method: "POST", path: "/api/workspaces/" + gamma + "/leave", headers: ann})
	w.do(hit{method: "DELETE", path: "/api/workspaces/" + gamma, body: M{"confirmUrlKey": "wrong"}, headers: ann})
	w.do(hit{method: "DELETE", path: "/api/workspaces/" + gamma, body: M{"confirmUrlKey": "gamma"}, headers: ann})
	w.do(hit{method: "DELETE", path: "/api/workspaces/" + w.Workspace.String(), body: M{"confirmUrlKey": "acme"}, headers: carol})
	w.do(hit{method: "POST", path: "/api/workspaces/" + w.Workspace.String() + "/leave", headers: carol})
	w.do(hit{method: "GET", path: "/api/me", headers: carol})

	w.do(hit{method: "GET", path: "/api/auth/sign-in?return_to=/acme", headers: map[string]string{"X-Ricrac": "1"}})
	w.do(hit{method: "GET", path: "/api/auth/callback/google?code=x&state=y", headers: map[string]string{"X-Ricrac": "1"}})
	w.do(hit{method: "POST", path: "/api/auth/sign-out", headers: ann})
	w.do(hit{method: "GET", path: "/api/github/connect", headers: ann})
	w.do(hit{method: "GET", path: "/api/nope", headers: ann})
	w.do(hit{method: "POST", path: "/github/webhook", raw: `{}`})
	w.do(hit{method: "GET", path: fmt.Sprintf("/files/%s", uuid.NewString())})
	w.do(hit{method: "PUT", path: "/uploads/nope", raw: "x"})
	w.do(hit{method: "GET", path: "/api/me", headers: map[string]string{"Cookie": w.session(uuid.New()), "X-Ricrac": "1"}})
}
