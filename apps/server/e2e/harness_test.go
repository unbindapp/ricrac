// Package e2e holds the end-to-end tests: each starts on a fresh database (cloned from a migrated
// template on the compose Postgres) with a workspace, and talks to the server only through its
// HTTP router. backend_test.go is the one file that knows the server's packages.
package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxuuid "github.com/vgarvardt/pgx-google-uuid/v5"
)

type M = map[string]any

const publicURL = "http://localhost:3000"

func databaseURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://ricrac:ricrac@localhost:5436/ricrac"
}

func withDatabase(url, name string) string {
	config, err := pgx.ParseConfig(url)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", config.User, config.Password, config.Host, config.Port, name)
}

var (
	templateOnce sync.Once
	templateName string
	templateErr  error
)

// template is a database with the schema, migrated once per run, that tests clone.
func template() (string, error) {
	templateOnce.Do(func() {
		ctx := context.Background()
		admin, err := pgx.Connect(ctx, databaseURL())
		if err != nil {
			templateErr = err
			return
		}
		defer admin.Close(ctx)
		templateName = "ricrac_template_" + randomSuffix()
		if _, err := admin.Exec(ctx, "CREATE DATABASE "+templateName); err != nil {
			templateErr = err
			return
		}
		templateErr = migrateDatabase(ctx, withDatabase(databaseURL(), templateName))
	})
	return templateName, templateErr
}

func randomSuffix() string {
	bytes := make([]byte, 6)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func TestMain(m *testing.M) {
	code := m.Run()
	if templateName != "" {
		ctx := context.Background()
		if admin, err := pgx.Connect(ctx, databaseURL()); err == nil {
			_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+templateName+" WITH (FORCE)")
			admin.Close(ctx)
		}
	}
	os.Exit(code)
}

// freshDB is a new database with the schema, dropped when the test ends.
func freshDB(t *testing.T) string {
	t.Helper()
	tmpl, err := template()
	if err != nil {
		t.Fatalf("template database: %v", err)
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, databaseURL())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer admin.Close(ctx)
	name := "ricrac_test_" + randomSuffix()
	if _, err := admin.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", name, tmpl)); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		if admin, err := pgx.Connect(context.Background(), databaseURL()); err == nil {
			_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
			admin.Close(context.Background())
		}
	})
	return withDatabase(databaseURL(), name)
}

func openPool(t *testing.T, url string) *pgxpool.Pool {
	t.Helper()
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["timezone"] = "UTC"
	config.AfterConnect = func(_ context.Context, conn *pgx.Conn) error {
		pgxuuid.Register(conn.TypeMap())
		return nil
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// World is a workspace "acme" owned by Ann, with the team ENG.
type World struct {
	t         *testing.T
	DB        *pgxpool.Pool
	B         *backend
	Router    http.Handler
	Workspace uuid.UUID
	Ann       *Actor
}

// Actor is someone calling operations in a workspace, through the web app's session.
type Actor struct {
	w         *World
	User      uuid.UUID
	Workspace uuid.UUID
}

func world(t *testing.T) *World {
	t.Helper()
	url := freshDB(t)
	w := &World{t: t, DB: openPool(t, url)}
	w.B = newBackend(t, url)
	w.Router = http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) { w.B.Router().ServeHTTP(rw, r) })
	ann := w.addUser("ann@acme.test", "Ann Smith")
	w.Workspace = w.newWorkspace("Acme", "acme", "Engineering", "ENG", ann)
	w.Ann = &Actor{w: w, User: ann, Workspace: w.Workspace}
	return w
}

// newWorkspace is a workspace owned by owner with its defaults and one team.
func (w *World) newWorkspace(name, urlKey, teamName, teamKey string, owner uuid.UUID) uuid.UUID {
	w.t.Helper()
	ctx := context.Background()
	var workspace uuid.UUID
	err := w.DB.QueryRow(ctx, "INSERT INTO workspaces (name, url_key) VALUES ($1, $2) RETURNING id", name, urlKey).Scan(&workspace)
	if err != nil {
		w.t.Fatalf("workspace: %v", err)
	}
	w.exec("INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')", workspace, owner)
	if err := w.B.WorkspaceDefaults(ctx, workspace); err != nil {
		w.t.Fatalf("workspace defaults: %v", err)
	}
	if _, err := w.B.CreateTeam(ctx, workspace, owner, teamName, teamKey); err != nil {
		w.t.Fatalf("team: %v", err)
	}
	return workspace
}

func (w *World) addUser(email, name string) uuid.UUID {
	w.t.Helper()
	return scalar[uuid.UUID](w, "INSERT INTO users (email, name, display_name) VALUES ($1, $2, $3) RETURNING id", email, name, displayName(name, email))
}

// displayName is how users get their @handle: the first name, lowercased, or the email's local part.
func displayName(name, email string) string {
	var first string
	if words := strings.Fields(name); len(words) > 0 {
		first = strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return r
			}
			return -1
		}, words[0])
	}
	if first != "" {
		return strings.ToLower(first)
	}
	local, _, _ := strings.Cut(email, "@")
	if local == "" {
		return "user"
	}
	return strings.ToLower(local)
}

func (w *World) actor(user, workspace uuid.UUID) *Actor {
	return &Actor{w: w, User: user, Workspace: workspace}
}

// member adds someone to the workspace with role (member, admin or guest), and to its team
// unless they're a guest, as joining does.
func (w *World) member(email, name, role string) *Actor {
	w.t.Helper()
	user := w.addUser(email, name)
	w.exec("INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)", w.Workspace, user, role)
	if role != "guest" {
		w.exec("INSERT INTO team_members (team_id, user_id) SELECT id, $2 FROM teams WHERE workspace_id = $1 AND archived_at IS NULL", w.Workspace, user)
	}
	return w.actor(user, w.Workspace)
}

// secondWorkspace is another workspace owned by Ann, with one team.
func (w *World) secondWorkspace(name, urlKey, teamKey string) uuid.UUID {
	w.t.Helper()
	return w.newWorkspace(name, urlKey, name, teamKey, w.Ann.User)
}

// createTeam adds a team to the workspace with Ann as its first member.
func (w *World) createTeam(name, key string) uuid.UUID {
	w.t.Helper()
	team, err := w.B.CreateTeam(context.Background(), w.Workspace, w.Ann.User, name, key)
	if err != nil {
		w.t.Fatalf("create team: %v", err)
	}
	return team
}

func (w *World) exec(sql string, args ...any) {
	w.t.Helper()
	if _, err := w.DB.Exec(context.Background(), sql, args...); err != nil {
		w.t.Fatalf("exec %s: %v", sql, err)
	}
}

// strings reads one text column of every row.
func (w *World) strings(sql string, args ...any) []string {
	w.t.Helper()
	rows, err := w.DB.Query(context.Background(), sql, args...)
	if err != nil {
		w.t.Fatalf("query %s: %v", sql, err)
	}
	list, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		w.t.Fatalf("query %s: %v", sql, err)
	}
	if list == nil {
		list = []string{}
	}
	return list
}

// scalar reads one value.
func scalar[T any](w *World, sql string, args ...any) T {
	w.t.Helper()
	var value T
	if err := w.DB.QueryRow(context.Background(), sql, args...).Scan(&value); err != nil {
		w.t.Fatalf("query %s: %v", sql, err)
	}
	return value
}

// session is the Cookie header of a signed-in user.
func (w *World) session(user uuid.UUID) string {
	return w.B.SessionCookie(user)
}

// callError is an operation's error answer.
type callError struct {
	Status  int
	Message string
}

func (e *callError) Error() string { return fmt.Sprintf("%d: %s", e.Status, e.Message) }

// tryCall runs an operation as the web app does: POST /v1/ops/{name} with the session.
func tryCall(c *Actor, name string, args M) (J, error) {
	if args == nil {
		args = M{}
	}
	req := httptest.NewRequest("POST", "/v1/ops/"+name, bytes.NewReader(marshal(args)))
	req.Header.Set("Cookie", c.w.session(c.User))
	req.Header.Set("X-Ricrac", "1")
	req.Header.Set("X-Ricrac-Workspace", c.Workspace.String())
	req.Header.Set("Content-Type", "application/json")
	status, value := send(c.w.Router, req)
	if status != http.StatusOK {
		message := value.At("error").Str()
		if message == "" {
			message = value.JSON()
		}
		return J{}, &callError{status, message}
	}
	if schema := c.w.B.ResultSchema(name); schema != nil {
		if err := validate(schema, value.V, name); err != nil {
			panic(fmt.Sprintf("%s answered something its result schema doesn't allow: %v", name, err))
		}
	}
	return value, nil
}

// call calls an operation and returns its result, failing the test on an error.
func call(t *testing.T, c *Actor, name string, args M) J {
	t.Helper()
	value, err := tryCall(c, name, args)
	if err != nil {
		t.Fatalf("%s failed: %s", name, err.(*callError).Message)
	}
	return value
}

// fails calls an operation that must fail, returning the error message.
func fails(t *testing.T, c *Actor, name string, args M) string {
	t.Helper()
	value, err := tryCall(c, name, args)
	if err == nil {
		t.Fatalf("%s should have failed but returned %s", name, marshal(value.V))
	}
	return err.(*callError).Message
}

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

// marshal encodes JSON without escaping <, > and &.
func marshal(value any) []byte {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
	return bytes.TrimRight(buffer.Bytes(), "\n")
}

// randomSecret is a random URL-safe secret with a readable prefix, like the server makes.
func randomSecret(prefix string) string {
	bytes := make([]byte, 32)
	_, _ = rand.Read(bytes)
	return prefix + base64.RawURLEncoding.EncodeToString(bytes)
}

// secretHash is how the server stores secrets.
func secretHash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// J is a JSON value with accessors for tests.
type J struct{ V any }

// At follows object keys and list indexes.
func (j J) At(path ...any) J {
	value := j.V
	for _, step := range path {
		switch key := step.(type) {
		case string:
			object, _ := value.(map[string]any)
			value = object[key]
		case int:
			list, _ := value.([]any)
			if key < 0 || key >= len(list) {
				return J{}
			}
			value = list[key]
		}
	}
	return J{value}
}

func (j J) Str() string {
	s, _ := j.V.(string)
	return s
}

func (j J) Num() float64 {
	n, _ := j.V.(float64)
	return n
}

func (j J) Bool() bool {
	b, _ := j.V.(bool)
	return b
}

func (j J) IsNull() bool { return j.V == nil }

func (j J) IsString() bool {
	_, ok := j.V.(string)
	return ok
}

func (j J) List() []J {
	list, _ := j.V.([]any)
	items := make([]J, len(list))
	for i, v := range list {
		items[i] = J{v}
	}
	return items
}

func (j J) Len() int {
	switch v := j.V.(type) {
	case []any:
		return len(v)
	case map[string]any:
		return len(v)
	}
	return 0
}

func (j J) JSON() string {
	return string(marshal(j.V))
}

// Strings is the text of each item of a list, or of the key of each item.
func (j J) Strings(key ...string) []string {
	items := []string{}
	for _, item := range j.List() {
		if len(key) > 0 {
			item = item.At(key[0])
		}
		items = append(items, item.Str())
	}
	return items
}

func identifiers(list J) []string {
	return list.At("nodes").Strings("identifier")
}

func eq[T comparable](t *testing.T, got, want T, context ...any) {
	t.Helper()
	if got != want {
		t.Fatalf("got %v, want %v %s", got, want, fmt.Sprint(context...))
	}
}

func eqList[T comparable](t *testing.T, got, want []T, context ...any) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v %s", got, want, fmt.Sprint(context...))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v %s", got, want, fmt.Sprint(context...))
		}
	}
}

func contains(t *testing.T, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Fatalf("%q doesn't contain %q", text, part)
		}
	}
}

func check(t *testing.T, ok bool, context ...any) {
	t.Helper()
	if !ok {
		t.Fatal(context...)
	}
}
