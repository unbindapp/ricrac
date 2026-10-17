package e2e

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// TestCutover rehearses deploying this server over the one in production: the previous server
// (RICRAC_CUTOVER_FROM, a commit) migrates a database and fills it through its API, then this
// server starts on it. The schema, the data and every answer must stay the same, the previous
// server's credentials must keep working, and the previous server must still start on the
// database afterwards (a rollback). It builds both servers, so it only runs when asked:
//
//	RICRAC_CUTOVER_FROM=c8f2a37 go test ./e2e -run TestCutover -v
func TestCutover(t *testing.T) {
	from := os.Getenv("RICRAC_CUTOVER_FROM")
	if from == "" {
		t.Skip("set RICRAC_CUTOVER_FROM to the commit of the server in production")
	}
	dir := t.TempDir()
	previous := buildPrevious(t, from, dir)
	current := filepath.Join(dir, "current")
	run(t, "..", "go", "build", "-o", current, "./cmd/server")
	database := emptyDatabase(t)

	old := start(t, previous, database)
	seeded := seed(t, old)
	before := snapshot(t, old, seeded)
	old.stop(t)
	schemaBefore, dataBefore := describeDatabase(t, database)

	next := start(t, current, database)
	check(t, strings.Contains(next.output.String(), "adopted the previous migrations"), "the old bookkeeping is adopted: ", next.output.String())
	schemaAfter, dataAfter := describeDatabase(t, database)
	same(t, "schema", schemaBefore, schemaAfter)
	same(t, "data", dataBefore, dataAfter)
	after := snapshot(t, next, seeded)
	for i := range before {
		same(t, before[i].name, before[i].body, after[i].body)
	}
	refreshed := form(t, next, "/oauth/token", url.Values{"grant_type": {"refresh_token"}, "refresh_token": {seeded.refresh}, "client_id": {seeded.client}})
	check(t, refreshed["access_token"] != nil, "the previous server's refresh tokens still work: ", refreshed)
	created := next.op(t, seeded.key, "save_issue", M{"team": "ENG", "title": "Made after the cutover"})
	eq(t, created["identifier"], any("ENG-11"))
	next.stop(t)
	_, dataNext := describeDatabase(t, database)

	rollback := start(t, previous, database)
	read := rollback.op(t, seeded.key, "get_issue", M{"id": "ENG-11"})
	eq(t, read["title"], any("Made after the cutover"), "the previous server starts on the migrated database")
	rollback.stop(t)

	again := start(t, current, database)
	check(t, !strings.Contains(again.output.String(), "migrated version"), "nothing is migrated twice: ", again.output.String())
	again.stop(t)
	_, dataAgain := describeDatabase(t, database)
	same(t, "data after restarting", dataNext, dataAgain)

	fresh := emptyDatabase(t)
	start(t, current, fresh).stop(t)
	schemaFresh, _ := describeDatabase(t, fresh)
	same(t, "the schema of a new database", withoutLegacyBookkeeping(schemaAfter), schemaFresh)
}

func run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	command := exec.Command(name, args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, output)
	}
	return string(output)
}

// buildPrevious builds the server at commit from a worktree of its own.
func buildPrevious(t *testing.T, commit, dir string) string {
	t.Helper()
	root := strings.TrimSpace(run(t, ".", "git", "rev-parse", "--show-toplevel"))
	tree := filepath.Join(dir, "previous-tree")
	run(t, root, "git", "worktree", "add", "--detach", tree, commit)
	t.Cleanup(func() { _ = exec.Command("git", "-C", root, "worktree", "remove", "--force", tree).Run() })
	binary := filepath.Join(dir, "previous")
	run(t, filepath.Join(tree, "apps", "server"), "go", "build", "-o", binary, ".")
	return binary
}

func emptyDatabase(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, databaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	name := "ricrac_cutover_" + randomSuffix()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if admin, err := pgx.Connect(context.Background(), databaseURL()); err == nil {
			_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
			admin.Close(context.Background())
		}
	})
	return withDatabase(databaseURL(), name)
}

type process struct {
	command *exec.Cmd
	base    string
	output  *bytes.Buffer
	binary  string
	env     []string
}

const cutoverSecret = "cutover-secret-key-that-is-long-enough"

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// start runs a server binary on the database, every server under the same public URL so links
// and cookie names stay the same.
func start(t *testing.T, binary, database string) *process {
	t.Helper()
	port := freePort(t)
	env := append(os.Environ(), "DATABASE_URL="+database, "PUBLIC_URL=http://localhost:3000", "SECRET_KEY="+cutoverSecret,
		fmt.Sprintf("PORT=%d", port), "GOOGLE_CLIENT_ID=", "GOOGLE_CLIENT_SECRET=", "SMTP_URL=", "EMAIL_FROM=", "GITHUB_APP_ID=")
	p := &process{base: fmt.Sprintf("http://127.0.0.1:%d", port), output: &bytes.Buffer{}, binary: binary, env: env}
	p.command = exec.Command(binary)
	p.command.Env = env
	p.command.Stdout = p.output
	p.command.Stderr = p.output
	if err := p.command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.command.Process.Kill() })
	for range 200 {
		response, err := http.Get(p.base + "/healthz")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return p
			}
		}
		if p.command.ProcessState != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s didn't start:\n%s", binary, p.output)
	return nil
}

func (p *process) stop(t *testing.T) {
	t.Helper()
	_ = p.command.Process.Signal(syscall.SIGTERM)
	_ = p.command.Wait()
}

// session signs someone in on the server the way development does.
func (p *process) session(t *testing.T, email string) string {
	t.Helper()
	command := exec.Command(p.binary, "session", email)
	command.Env = p.env
	output, err := command.Output()
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	return strings.TrimSpace(string(output))
}

func (p *process) request(t *testing.T, method, path string, headers map[string]string, body any) (int, any) {
	t.Helper()
	status, data := p.raw(t, method, path, headers, body)
	var value any
	if json.Unmarshal(data, &value) != nil {
		value = string(data)
	}
	return status, value
}

// raw is the status and the body exactly as the server wrote it.
func (p *process) raw(t *testing.T, method, path string, headers map[string]string, body any) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(marshal(body))
	}
	req, err := http.NewRequest(method, p.base+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "localhost:3000"
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(response.Body)
	return response.StatusCode, data
}

func (p *process) op(t *testing.T, key, name string, args M) M {
	t.Helper()
	status, value := p.request(t, "POST", "/v1/ops/"+name, map[string]string{"Authorization": "Bearer " + key}, args)
	if status != http.StatusOK {
		t.Fatalf("%s: %d %v", name, status, value)
	}
	object, _ := value.(M)
	return object
}

func form(t *testing.T, p *process, path string, values url.Values) M {
	t.Helper()
	response, err := http.PostForm(p.base+path, values)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var value M
	_ = json.NewDecoder(response.Body).Decode(&value)
	return value
}

type seeded struct {
	cookie, key, readKey, access, refresh, client string
}

// seed fills the workspace "acme" through the previous server's API, as people and agents do.
func seed(t *testing.T, p *process) seeded {
	t.Helper()
	s := seeded{cookie: p.session(t, "ann@acme.test")}
	dashboard := map[string]string{"Cookie": s.cookie, "X-Ricrac": "1"}
	status, created := p.request(t, "POST", "/api/workspaces", dashboard, M{"name": "Acme", "urlKey": "acme", "teamName": "Engineering", "teamKey": "ENG"})
	eq(t, status, http.StatusOK, created)
	_, key := p.request(t, "POST", "/api/api-keys", dashboard, M{"label": "everything", "scopes": []string{"read", "write", "admin"}, "workspaceIds": nil})
	s.key = key.(M)["secret"].(string)
	_, readKey := p.request(t, "POST", "/api/api-keys", dashboard, M{"label": "reader", "scopes": []string{"read"}, "workspaceIds": []any{created.(M)["id"]}})
	s.readKey = readKey.(M)["secret"].(string)

	op := func(name string, args M) M { return p.op(t, s.key, name, args) }
	op("save_team", M{"name": "Design", "key": "DES"})
	op("invite_user", M{"email": "bob@acme.test", "role": "member"})
	bob := map[string]string{"Cookie": p.session(t, "bob@acme.test"), "X-Ricrac": "1"}
	_, me := p.request(t, "GET", "/api/me", bob, nil)
	invitation := me.(M)["invitations"].([]any)[0].(M)["id"].(string)
	p.request(t, "POST", "/api/invitations/"+invitation+"/accept", bob, nil)
	op("invite_user", M{"email": "carol@acme.test", "role": "guest", "teams": []any{"DES"}})
	op("save_issue_label", M{"name": "Type", "isGroup": true})
	op("save_issue_label", M{"name": "Regression", "parent": "Type", "color": "#eb5757"})
	op("save_issue_label", M{"name": "Platform", "team": "ENG"})
	project := op("save_project", M{"name": "Launch v2", "addTeams": []any{"ENG", "DES"}, "targetDate": "2030-03-31", "lead": "me"})
	op("save_milestone", M{"project": "Launch v2", "name": "Beta", "targetDate": "2030-02-01"})
	titles := []string{"Fix login redirect loop", "Add dark mode", "Crash on upload", "Improve onboarding copy", "Billing shows wrong currency",
		"Keyboard shortcuts", "Search is slow", "Export to CSV", "Mobile layout breaks", "Rate limit the API"}
	states := []string{"started", "unstarted", "started", "backlog", "completed", "backlog", "unstarted", "backlog", "started", "canceled"}
	for i, title := range titles {
		args := M{"team": "ENG", "title": title, "state": states[i], "priority": i % 5, "description": "## Context\n\n" + title + ".\n"}
		if i%2 == 0 {
			args["assignee"] = "bob"
			args["labels"] = []any{"Bug"}
		}
		if i < 4 {
			args["project"] = project["id"]
		}
		if i == 1 {
			args["dueDate"] = "2030-01-15"
			args["labels"] = []any{"Type/Regression", "Platform"}
		}
		op("save_issue", args)
	}
	op("save_issue", M{"id": "ENG-2", "parentId": "ENG-1", "blocks": []any{"ENG-3"}, "milestone": "Beta"})
	op("save_issue", M{"id": "ENG-4", "relatedTo": []any{"ENG-5"}})
	comment := op("save_comment", M{"issueId": "ENG-1", "body": "Happens on Safari only, I think. @bob"})
	op("save_comment", M{"issueId": "ENG-1", "parentId": comment["id"], "body": "Confirmed."})
	op("add_reaction", M{"commentId": comment["id"], "emoji": "👍"})
	op("create_attachment", M{"issue": "ENG-3", "url": "https://github.com/acme/app/pull/4", "title": "Fix upload crash"})
	op("save_document", M{"title": "Runbook", "content": "Steps to roll back.", "project": "Launch v2"})
	op("save_initiative", M{"name": "Q1 goals", "addProjects": []any{"Launch v2"}})
	op("save_status_update", M{"project": "Launch v2", "body": "On track for beta.", "health": "onTrack"})
	op("save_custom_view", M{"name": "My bugs", "filter": M{"labels": M{"some": M{"name": M{"eq": "Bug"}}}}, "shared": true})
	op("save_template", M{"name": "Bug report", "type": "issue", "team": "ENG", "data": M{"priority": 2}})
	op("save_customer", M{"name": "Globex", "domains": []any{"globex.com"}, "status": "Active"})
	op("save_customer_need", M{"customer": "Globex", "issue": "ENG-1", "body": "Blocking their rollout"})
	op("save_release_pipeline", M{"name": "Web", "type": "scheduled", "addTeams": []any{"ENG"}})
	op("save_release", M{"name": "Spring", "pipeline": "Web", "version": "1.0"})

	acme := map[string]string{"Cookie": s.cookie, "X-Ricrac": "1", "X-Ricrac-Workspace": "acme"}
	_, app := p.request(t, "POST", "/api/oauth-apps", acme, M{"name": "Agent", "redirectUris": []string{"https://agent.example/callback"}})
	s.client = app.(M)["app"].(M)["id"].(string)
	verifier := randomSecret("")
	sum := sha256.Sum256([]byte(verifier))
	request := M{"client_id": s.client, "redirect_uri": "https://agent.example/callback", "response_type": "code",
		"code_challenge": base64.RawURLEncoding.EncodeToString(sum[:]), "code_challenge_method": "S256", "state": "s"}
	_, decided := p.request(t, "POST", "/api/oauth/decide", dashboard, M{"request": request, "workspaceIds": nil, "allow": true})
	redirect, _ := url.Parse(decided.(M)["redirect"].(string))
	tokens := form(t, p, "/oauth/token", url.Values{"grant_type": {"authorization_code"}, "code": {redirect.Query().Get("code")},
		"client_id": {s.client}, "client_secret": {app.(M)["clientSecret"].(string)}, "redirect_uri": {"https://agent.example/callback"},
		"code_verifier": {verifier}})
	s.access, _ = tokens["access_token"].(string)
	s.refresh, _ = tokens["refresh_token"].(string)
	check(t, s.access != "" && s.refresh != "", "tokens: ", tokens)
	return s
}

type answer struct {
	name string
	body any
}

// snapshot reads everything a reader would through every kind of credential.
func snapshot(t *testing.T, p *process, s seeded) []answer {
	t.Helper()
	var answers []answer
	read := func(name, method, path string, headers map[string]string, body any) {
		status, data := p.raw(t, method, path, headers, body)
		answers = append(answers, answer{name, M{"status": status, "body": string(data)}})
	}
	bearer := func(key string) map[string]string { return map[string]string{"Authorization": "Bearer " + key} }
	_, listed := p.request(t, "GET", "/v1/ops", nil, nil)
	var names []string
	for _, op := range listed.(M)["operations"].([]any) {
		if op.(M)["access"] == "read" {
			names = append(names, op.(M)["name"].(string))
		}
	}
	slices.Sort(names)
	for _, name := range names {
		read(name, "POST", "/v1/ops/"+name, bearer(s.key), M{})
	}
	for _, id := range []string{"ENG-1", "ENG-2", "ENG-5"} {
		read("get_issue "+id, "POST", "/v1/ops/get_issue", bearer(s.key), M{"id": id, "includeComments": true, "includeHistory": true})
	}
	for _, get := range []struct {
		name string
		args M
	}{
		{"get_project", M{"id": "Launch v2"}}, {"get_document", M{"id": "Runbook"}}, {"get_initiative", M{"id": "Q1 goals"}},
		{"get_customer", M{"id": "Globex"}}, {"get_release", M{"id": "Spring"}}, {"get_team", M{"id": "ENG"}},
		{"get_user", M{"id": "bob"}}, {"get_template", M{"id": "Bug report"}},
		{"list_issues", M{"includeDescription": true, "includeArchived": true, "limit": 250}},
	} {
		read(get.name, "POST", "/v1/ops/"+get.name, bearer(s.key), get.args)
	}
	read("rest list", "GET", "/v1/issues?limit=3&orderBy=priority", bearer(s.readKey), nil)
	read("oauth token", "POST", "/v1/ops/list_issues", bearer(s.access), M{"workspace": "acme"})
	dashboard := map[string]string{"Cookie": s.cookie, "X-Ricrac": "1"}
	read("me", "GET", "/api/me", dashboard, nil)
	read("api keys", "GET", "/api/api-keys", dashboard, nil)
	read("connected apps", "GET", "/api/connected-apps", dashboard, nil)
	read("oauth apps", "GET", "/api/oauth-apps", map[string]string{"Cookie": s.cookie, "X-Ricrac": "1", "X-Ricrac-Workspace": "acme"}, nil)
	read("session ops", "POST", "/v1/ops/list_projects", map[string]string{"Cookie": s.cookie, "X-Ricrac": "1", "X-Ricrac-Workspace": "acme"}, M{})
	tools := M{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}
	read("mcp tools", "POST", "/mcp", map[string]string{"Authorization": "Bearer " + s.key, "Accept": "application/json, text/event-stream"}, tools)
	read("openapi", "GET", "/v1/openapi.json", nil, nil)
	return answers
}

// describeDatabase is the schema (without the migration bookkeeping) and a checksum of every table's
// rows (without the times credentials were last used).
func describeDatabase(t *testing.T, database string) (string, M) {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	rows, err := conn.Query(ctx, `
		SELECT 'column ' || table_name || '.' || column_name || ' ' || data_type || ' ' || is_nullable || ' ' ||
			coalesce(column_default, '') || ' ' || coalesce(generation_expression, '')
		FROM information_schema.columns WHERE table_schema = 'public' AND table_name NOT IN ('goose_db_version')
		UNION ALL
		SELECT 'constraint ' || conrelid::regclass || ' ' || conname || ' ' || pg_get_constraintdef(oid)
		FROM pg_constraint WHERE connamespace = 'public'::regnamespace AND conrelid::regclass::text <> 'goose_db_version'
		UNION ALL
		SELECT 'index ' || indexdef FROM pg_indexes WHERE schemaname = 'public' AND tablename <> 'goose_db_version'
		UNION ALL
		SELECT 'function ' || p.proname || ' ' || pg_get_functiondef(p.oid) FROM pg_proc p WHERE p.pronamespace = 'public'::regnamespace
		ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	tables, err := conn.Query(ctx, "SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename <> 'goose_db_version' ORDER BY 1")
	if err != nil {
		t.Fatal(err)
	}
	names, err := pgx.CollectRows(tables, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	data := M{}
	for _, table := range names {
		var sum string
		err := conn.QueryRow(ctx, fmt.Sprintf(
			`SELECT count(*) || ':' || coalesce(md5(string_agg(row, '|' ORDER BY row)), '')
			 FROM (SELECT (to_jsonb(t) - 'last_used_at' - 'execution_time')::text AS row FROM %s t) rows`, table)).Scan(&sum)
		if err != nil {
			t.Fatal(err)
		}
		data[table] = sum
	}
	return strings.Join(schema, "\n"), data
}

func same(t *testing.T, what string, want, got any) {
	t.Helper()
	if string(marshal(want)) != string(marshal(got)) {
		var differences []string
		diff("", toPlain(want), toPlain(got), &differences)
		t.Errorf("%s changed:\n  %s", what, strings.Join(differences[:min(len(differences), 20)], "\n  "))
	}
}

func toPlain(value any) any {
	var plain any
	_ = json.Unmarshal(marshal(value), &plain)
	return plain
}

func withoutLegacyBookkeeping(schema string) string {
	var lines []string
	for line := range strings.SplitSeq(schema, "\n") {
		if !strings.Contains(line, "_sqlx_migrations") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func diff(path string, w, h any, out *[]string) {
	wm, wok := w.(map[string]any)
	hm, hok := h.(map[string]any)
	if wok && hok {
		keys := slices.Sorted(maps.Keys(wm))
		for key := range hm {
			if _, ok := wm[key]; !ok {
				keys = append(keys, key)
			}
		}
		for _, key := range keys {
			if !reflect.DeepEqual(wm[key], hm[key]) {
				diff(path+"."+key, wm[key], hm[key], out)
			}
		}
		return
	}
	wl, wok := w.([]any)
	hl, hok := h.([]any)
	if wok && hok && len(wl) == len(hl) {
		for i := range wl {
			if !reflect.DeepEqual(wl[i], hl[i]) {
				diff(fmt.Sprintf("%s[%d]", path, i), wl[i], hl[i], out)
			}
		}
		return
	}
	ws, wok := w.(string)
	hs, hok := h.(string)
	if wok && hok && (len(ws) > 200 || len(hs) > 200) {
		at := 0
		for at < min(len(ws), len(hs)) && ws[at] == hs[at] {
			at++
		}
		from := max(0, at-80)
		*out = append(*out, fmt.Sprintf("%s: from byte %d, recorded %q, got %q", path, at, ws[from:min(len(ws), at+120)], hs[from:min(len(hs), at+120)]))
		return
	}
	*out = append(*out, fmt.Sprintf("%s: recorded %s, got %s", path, short(w), short(h)))
}

func short(v any) string {
	text := string(marshal(v))
	if len(text) > 300 {
		return text[:300] + "..."
	}
	return text
}
