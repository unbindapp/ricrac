package e2e

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"ricrac/app"
	"ricrac/server"
)

// The REST API, OAuth and MCP through the real router.

func send(router http.Handler, request *http.Request) (int, J) {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	body, _ := io.ReadAll(recorder.Result().Body)
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		value = string(body)
	}
	return recorder.Code, J{value}
}

func apiKey(w *World, scopes ...string) string {
	return keyReaching(w, []uuid.UUID{w.Workspace}, scopes...)
}

// keyReaching is a key of Ann's for these workspaces, or all of hers when nil.
func keyReaching(w *World, workspaces []uuid.UUID, scopes ...string) string {
	key := app.RandomSecret(server.APIKeyPrefix)
	w.exec("INSERT INTO api_keys (workspace_ids, user_id, label, key_hash, key_hint, scopes) VALUES ($1, $2, 'test', $3, 'x', $4)",
		workspaces, w.Ann.User, app.SecretHash(key), scopes)
	return key
}

func request(method, uri, token string, body any) *http.Request {
	var reader io.Reader
	if body != nil {
		reader = strings.NewReader(string(app.Marshal(body)))
	}
	req := httptest.NewRequest(method, uri, reader)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func TestRESTRoutesAndScopes(t *testing.T) {
	w := world(t)
	router := server.Router(w.State)
	key := apiKey(w, "read", "write")
	status, created := send(router, request("POST", "/v1/issues", key, M{"team": "ENG", "title": "From REST"}))
	eq(t, status, http.StatusOK, created.JSON())
	eq(t, created.At("identifier").Str(), "ENG-1")
	_, updated := send(router, request("PATCH", "/v1/issues/ENG-1", key, M{"priority": 2}))
	eq(t, updated.At("priority").Num(), 2.0)
	_, listed := send(router, request("GET", "/v1/issues?priority=2&includeDescription=true&labels=", key, nil))
	check(t, listed.At("error").IsString(), "unknown query arguments are refused: ", listed.JSON())
	_, listed = send(router, request("GET", "/v1/issues?priority=2&includeDescription=true", key, nil))
	eq(t, listed.At("nodes", 0, "identifier").Str(), "ENG-1", listed.JSON())
	filter := url.QueryEscape(`{"title":{"contains":"rest"}}`)
	_, filtered := send(router, request("GET", "/v1/issues?filter="+filter, key, nil))
	eq(t, filtered.At("nodes").Len(), 1, filtered.JSON())
	_, byName := send(router, request("POST", "/v1/ops/get_issue", key, M{"id": "ENG-1"}))
	eq(t, byName.At("title").Str(), "From REST")
	readOnly := apiKey(w, "read")
	status, _ = send(router, request("POST", "/v1/issues", readOnly, M{"title": "x"}))
	eq(t, status, http.StatusForbidden)
	status, _ = send(router, request("GET", "/v1/issues", "rr_key_wrong", nil))
	eq(t, status, http.StatusUnauthorized)
	status, document := send(router, httptest.NewRequest("GET", "/v1/openapi.json", nil))
	eq(t, status, http.StatusOK)
	check(t, document.At("paths", "/v1/issues/{id}", "patch").Len() > 0)
}

func tokenRequest(form url.Values) *http.Request {
	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestOAuthFlowGivesATokenForTheChosenWorkspace(t *testing.T) {
	w := world(t)
	router := server.Router(w.State)
	register := httptest.NewRequest("POST", "/oauth/register",
		strings.NewReader(`{"client_name": "Claude", "redirect_uris": ["http://localhost/callback"]}`))
	register.Header.Set("Content-Type", "application/json")
	status, client := send(router, register)
	eq(t, status, http.StatusCreated, client.JSON())
	clientID := client.At("client_id").Str()
	verifier := app.RandomSecret("")
	sum := sha256.Sum256([]byte(verifier))
	challenge, method, state, resource := base64.RawURLEncoding.EncodeToString(sum[:]), "S256", "xyz", "http://localhost:3000/mcp"
	req := &server.OAuthRequest{
		ClientID:            clientID,
		RedirectURI:         "http://localhost:4567/callback",
		ResponseType:        "code",
		CodeChallenge:       &challenge,
		CodeChallengeMethod: &method,
		State:               &state,
		Resource:            &resource,
	}
	stranger := w.secondWorkspace("Beta", "beta", "OPS")
	w.exec("UPDATE workspace_members SET active = false WHERE workspace_id = $1", stranger)
	ctx := context.Background()
	for _, workspaces := range [][]uuid.UUID{{}, {w.Workspace, stranger}} {
		_, err := server.Decide(ctx, w.State, w.Ann.User, workspaces, req, true)
		check(t, err != nil, "granting ", workspaces, " should fail")
	}
	redirect, err := server.Decide(ctx, w.State, w.Ann.User, []uuid.UUID{w.Workspace}, req, true)
	check(t, err == nil, err)
	check(t, strings.HasPrefix(redirect, "http://localhost:4567/callback?code=") && strings.Contains(redirect, "state=xyz"), redirect)
	parsed, _ := url.Parse(redirect)
	code := parsed.Query().Get("code")
	status, tokens := send(router, tokenRequest(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {clientID},
		"redirect_uri":  {"http://localhost:4567/callback"},
		"code_verifier": {verifier},
	}))
	eq(t, status, http.StatusOK, tokens.JSON())
	access := tokens.At("access_token").Str()
	_, workspace := send(router, request("POST", "/v1/ops/list_issues", access, M{}))
	_, isList := workspace.At("nodes").V.([]any)
	check(t, isList, workspace.JSON())
	status, _ = send(router, tokenRequest(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {clientID},
		"code_verifier": {verifier},
	}))
	eq(t, status, http.StatusBadRequest, "codes are single use")
	refresh := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tokens.At("refresh_token").Str()}, "client_id": {clientID}}
	status, refreshed := send(router, tokenRequest(refresh))
	eq(t, status, http.StatusOK)
	status, _ = send(router, tokenRequest(refresh))
	eq(t, status, http.StatusBadRequest, "refresh tokens rotate")
	revoke := httptest.NewRequest("POST", "/oauth/revoke", strings.NewReader(url.Values{
		"token": {refreshed.At("access_token").Str()}, "client_id": {clientID},
	}.Encode()))
	revoke.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	status, _ = send(router, revoke)
	eq(t, status, http.StatusOK)
	status, _ = send(router, request("GET", "/v1/issues", refreshed.At("access_token").Str(), nil))
	eq(t, status, http.StatusUnauthorized)
}

func rpc(uri, key string, body any) *http.Request {
	req := httptest.NewRequest("POST", uri, strings.NewReader(string(app.Marshal(body))))
	req.Host = "localhost:3000"
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", "2025-06-18")
	return req
}

func toolNames(tools J) []string {
	return tools.At("result", "tools").Strings("name")
}

func TestMCPListsAndCallsTools(t *testing.T) {
	w := world(t)
	router := server.Router(w.State)
	key := apiKey(w, "read", "write", "admin")
	status, _ := send(router, httptest.NewRequest("POST", "/mcp", nil))
	eq(t, status, http.StatusUnauthorized)
	status, init := send(router, rpc("/mcp", key, M{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": M{
		"protocolVersion": "2025-06-18", "capabilities": M{}, "clientInfo": M{"name": "test", "version": "1"}}}))
	eq(t, status, http.StatusOK, init.JSON())
	eq(t, init.At("result", "protocolVersion").Str(), "2025-06-18")
	contains(t, init.At("result", "instructions").Str(), "issue tracker")
	_, tools := send(router, rpc("/mcp", key, M{"jsonrpc": "2.0", "id": 2, "method": "tools/list"}))
	names := toolNames(tools)
	check(t, slices.Contains(names, "save_issue") && slices.Contains(names, "find_operations"), names)
	check(t, !slices.Contains(names, "save_webhook"), "rarer operations are found through find_operations")
	_, result := send(router, rpc("/mcp", key, M{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": M{
		"name": "save_issue", "arguments": M{"team": "ENG", "title": "From MCP"}}}))
	contains(t, result.At("result", "content", 0, "text").Str(), "ENG-1")
	_, failed := send(router, rpc("/mcp", key, M{"jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": M{
		"name": "save_issue", "arguments": M{"team": "NOPE", "title": "x"}}}))
	eq(t, failed.At("result", "isError").V, any(true), failed.JSON())

	_, everything := send(router, rpc("/mcp?tools=all", key, M{"jsonrpc": "2.0", "id": 7, "method": "tools/list"}))
	check(t, slices.Contains(toolNames(everything), "save_webhook"))
	_, found := send(router, rpc("/mcp", key, M{"jsonrpc": "2.0", "id": 5, "method": "tools/call", "params": M{
		"name": "find_operations", "arguments": M{"query": "webhook"}}}))
	contains(t, found.At("result", "content", 0, "text").Str(), "save_webhook", "resourceTypes")
	_, called := send(router, rpc("/mcp", key, M{"jsonrpc": "2.0", "id": 6, "method": "tools/call", "params": M{
		"name": "call_operation", "arguments": M{"name": "list_webhooks", "arguments": M{}}}}))
	eq(t, called.At("result", "isError").V, any(false), called.JSON())
}

func TestMCPAnswersBatchesNotificationsAndChecksTheHost(t *testing.T) {
	w := world(t)
	router := server.Router(w.State)
	key := apiKey(w, "read")
	status, batch := send(router, rpc("/mcp", key, []any{
		M{"jsonrpc": "2.0", "method": "notifications/initialized"},
		M{"jsonrpc": "2.0", "id": 1, "method": "ping"},
		M{"jsonrpc": "2.0", "id": 2, "method": "nope"},
	}))
	eq(t, status, http.StatusOK, batch.JSON())
	eq(t, batch.Len(), 2, batch.JSON())
	eq(t, batch.At(1, "error", "code").Num(), -32601.0)
	status, _ = send(router, rpc("/mcp", key, M{"jsonrpc": "2.0", "method": "notifications/initialized"}))
	eq(t, status, http.StatusAccepted)
	_, init := send(router, rpc("/mcp", key, M{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": M{"protocolVersion": "1999-01-01"}}))
	eq(t, init.At("result", "protocolVersion").Str(), "2026-07-28")
	rebound := rpc("/mcp", key, M{"jsonrpc": "2.0", "id": 1, "method": "ping"})
	rebound.Host = "evil.example"
	status, _ = send(router, rebound)
	eq(t, status, http.StatusForbidden)
	_, tools := send(router, rpc("/mcp", key, M{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}))
	names := toolNames(tools)
	check(t, !slices.Contains(names, "save_issue") && slices.Contains(names, "get_issue"), "read keys see read tools: ", names)
	_, found := send(router, rpc("/mcp", key, M{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": M{
		"name": "find_operations", "arguments": M{"query": "issue histories"}}}))
	text := found.At("result", "content", 0, "text").Str()
	contains(t, text, "get_issue_history", `"properties"`)
	check(t, !strings.Contains(text, "save_issue"), text)
	_, typo := send(router, rpc("/mcp", key, M{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": M{
		"name": "find_operations", "arguments": M{"querry": "issue"}}}))
	eq(t, typo.At("result", "isError").V, any(true), typo.JSON())
	_, called := send(router, rpc("/mcp", key, M{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": M{
		"name": "call_operation", "arguments": M{"name": "list_issues", "arguments": M{}}}}))
	eq(t, called.At("result", "isError").V, any(false), called.JSON())
}

func TestMCPSpeaksTheStatelessRevision(t *testing.T) {
	w := world(t)
	router := server.Router(w.State)
	key := apiKey(w, "read", "write")
	modern := func(body any) J {
		req := rpc("/mcp", key, body)
		req.Header.Set("Mcp-Protocol-Version", "2026-07-28")
		_, response := send(router, req)
		return response
	}
	discover := modern(M{"jsonrpc": "2.0", "id": 1, "method": "server/discover"})
	eq(t, discover.At("result", "resultType").Str(), "complete", discover.JSON())
	check(t, slices.Contains(discover.At("result", "supportedVersions").Strings(), "2026-07-28"), discover.JSON())
	eq(t, discover.At("result", "_meta", "io.modelcontextprotocol/serverInfo", "name").Str(), "ricrac")
	init := modern(M{"jsonrpc": "2.0", "id": 2, "method": "initialize", "params": M{"protocolVersion": "2026-07-28"}})
	eq(t, init.At("result", "protocolVersion").Str(), "2026-07-28")
	eq(t, init.At("result", "resultType").Str(), "complete")
	tools := modern(M{"jsonrpc": "2.0", "id": 3, "method": "tools/list"})
	eq(t, tools.At("result", "resultType").Str(), "complete")
	eq(t, tools.At("result", "cacheScope").Str(), "private")
	called := modern(M{"jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": M{"name": "list_workspaces", "arguments": M{}}})
	eq(t, called.At("result", "resultType").Str(), "complete", called.JSON())
	failed := modern(M{"jsonrpc": "2.0", "id": 5, "method": "tools/call", "params": M{"name": "get_issue", "arguments": M{"id": "ENG-99"}}})
	eq(t, failed.At("result", "resultType").Str(), "complete", failed.JSON())
	eq(t, failed.At("result", "isError").V, any(true))
	_, inMeta := send(router, rpc("/mcp", key, M{"jsonrpc": "2.0", "id": 6, "method": "tools/call", "params": M{
		"name": "list_workspaces", "arguments": M{}, "_meta": M{"io.modelcontextprotocol/protocolVersion": "2026-07-28"}}}))
	eq(t, inMeta.At("result", "resultType").Str(), "complete", "the version can come in _meta")
	_, legacy := send(router, rpc("/mcp", key, M{"jsonrpc": "2.0", "id": 7, "method": "tools/call", "params": M{"name": "list_workspaces", "arguments": M{}}}))
	check(t, legacy.At("result", "resultType").IsNull(), "older revisions keep their shape: ", legacy.JSON())
}

func TestTokensForSeveralWorkspacesNameOnePerCall(t *testing.T) {
	w := world(t)
	beta := w.secondWorkspace("Beta", "beta", "OPS")
	router := server.Router(w.State)
	all := keyReaching(w, nil, "read", "write")

	status, failure := send(router, request("POST", "/v1/issues", all, M{"team": "ENG", "title": "In Acme"}))
	eq(t, status, http.StatusBadRequest)
	contains(t, failure.At("error").Str(), `"acme" (Acme)`, `"beta" (Beta)`)
	_, created := send(router, request("POST", "/v1/issues", all, M{"workspace": "acme", "team": "ENG", "title": "In Acme"}))
	eq(t, created.At("identifier").Str(), "ENG-1", created.JSON())
	inBeta := request("POST", "/v1/issues", all, M{"team": "OPS", "title": "In Beta"})
	inBeta.Header.Set("X-Ricrac-Workspace", "beta")
	_, created = send(router, inBeta)
	eq(t, created.At("identifier").Str(), "OPS-1", created.JSON())
	_, read := send(router, request("GET", "/v1/issues/ENG-1?workspace=acme", all, nil))
	eq(t, read.At("title").Str(), "In Acme")
	_, read = send(router, request("POST", "/v1/ops/get_issue", all, M{"id": "http://localhost:3000/beta/issue/OPS-1"}))
	eq(t, read.At("title").Str(), "In Beta", "links name their workspace")
	status, _ = send(router, request("POST", "/v1/ops/get_issue", all, M{"workspace": "nope", "id": "ENG-1"}))
	eq(t, status, http.StatusBadRequest)

	onlyBeta := keyReaching(w, []uuid.UUID{beta}, "read")
	_, listed := send(router, request("GET", "/v1/issues", onlyBeta, nil))
	eqList(t, identifiers(listed), []string{"OPS-1"})
	status, _ = send(router, request("GET", "/v1/issues?workspace=acme", onlyBeta, nil))
	eq(t, status, http.StatusBadRequest, "the key doesn't reach acme")

	_, workspaces := send(router, request("GET", "/v1/workspaces", all, nil))
	eqList(t, workspaces.At("nodes").Strings("urlKey"), []string{"acme", "beta"})
}

func TestMCPToolsTakeTheWorkspaceWhenTheTokenReachesSeveral(t *testing.T) {
	w := world(t)
	router := server.Router(w.State)
	all := keyReaching(w, nil, "read", "write")
	listTools := func() []J {
		_, tools := send(router, rpc("/mcp", all, M{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}))
		return tools.At("result", "tools").List()
	}
	saveIssueSchema := func(tools []J) J {
		for _, tool := range tools {
			if tool.At("name").Str() == "save_issue" {
				return tool.At("inputSchema")
			}
		}
		t.Fatal("no save_issue tool")
		return J{}
	}
	schema := saveIssueSchema(listTools())
	check(t, schema.At("properties", "workspace").IsNull(), "one workspace needs no argument")
	w.secondWorkspace("Beta", "beta", "OPS")
	tools := listTools()
	schema = saveIssueSchema(tools)
	check(t, schema.At("properties", "workspace").Len() > 0)
	check(t, slices.Contains(schema.At("required").Strings(), "workspace"))

	check(t, slices.ContainsFunc(tools, func(tool J) bool { return tool.At("name").Str() == "list_workspaces" }))
}

func TestLeavingAWorkspaceTakesItOutOfKeys(t *testing.T) {
	w := world(t)
	beta := w.secondWorkspace("Beta", "beta", "OPS")
	bob := w.member("bob@acme.test", "Bob", "member").User
	w.exec("INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'member')", beta, bob)
	router := server.Router(w.State)
	cookie := fmt.Sprintf("%s=%s", w.State.Keys.SessionCookie, w.State.Keys.SessionToken(bob))
	dashboard := func(method, uri string, body any) *http.Request {
		req := httptest.NewRequest(method, uri, strings.NewReader(string(app.Marshal(body))))
		req.Header.Set("Cookie", cookie)
		req.Header.Set("X-Ricrac", "1")
		req.Header.Set("Content-Type", "application/json")
		return req
	}
	var keys []string
	for _, workspaces := range []any{[]uuid.UUID{beta}, []uuid.UUID{w.Workspace, beta}, nil} {
		status, created := send(router, dashboard("POST", "/api/api-keys", M{"label": "agent", "scopes": []string{"read"}, "workspaceIds": workspaces}))
		eq(t, status, http.StatusOK, created.JSON())
		keys = append(keys, created.At("key", "id").Str())
	}
	status, _ := send(router, dashboard("POST", "/api/api-keys", M{"label": "admin", "scopes": []string{"admin"}, "workspaceIds": []uuid.UUID{beta}}))
	eq(t, status, http.StatusBadRequest, "Bob isn't an admin in beta")

	status, _ = send(router, dashboard("POST", fmt.Sprintf("/api/workspaces/%s/leave", beta), M{}))
	eq(t, status, http.StatusOK)
	_, listed := send(router, dashboard("GET", "/api/api-keys", nil))
	reach := map[string]string{}
	for _, key := range listed.List() {
		reach[key.At("id").Str()] = fmt.Sprint(key.At("allWorkspaces").Bool(), key.At("workspaces").JSON())
	}
	_, kept := reach[keys[0]]
	check(t, !kept, "the key for beta alone is gone")
	eq(t, reach[keys[1]], `false["Acme"]`)
	eq(t, reach[keys[2]], "true[]")
}

func TestMeShowsTheLogoOfEveryWorkspace(t *testing.T) {
	w := world(t)
	w.secondWorkspace("Beta", "beta", "OPS")
	call(t, w.Ann, "update_workspace", M{"logoUrl": "https://x/acme.png"})
	req := httptest.NewRequest("GET", "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: w.State.Keys.SessionCookie, Value: w.State.Keys.SessionToken(w.Ann.User)})
	_, me := send(server.Router(w.State), req)
	eq(t, me.At("workspaces", 0, "name").Str(), "Acme")
	eq(t, me.At("workspaces", 0, "logoUrl").Str(), "https://x/acme.png")
	eq(t, me.At("workspaces", 1, "name").Str(), "Beta")
	check(t, me.At("workspaces", 1, "logoUrl").IsNull())
}
