// Package e2e holds the end-to-end tests: each starts on a fresh database (cloned from a migrated
// template on the compose Postgres) with a workspace, and calls operations as the API does.
package e2e

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ricrac/app"
	"ricrac/ops"
)

type M = map[string]any

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
		pool, err := app.Connect(ctx, withDatabase(databaseURL(), templateName))
		if err != nil {
			templateErr = err
			return
		}
		pool.Close()
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
func freshDB(t *testing.T) *pgxpool.Pool {
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
	pool, err := app.Connect(ctx, withDatabase(databaseURL(), name))
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		if admin, err := pgx.Connect(context.Background(), databaseURL()); err == nil {
			_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
			admin.Close(context.Background())
		}
	})
	return pool
}

func testConfig() app.Config {
	return app.Config{
		PublicURL:     "http://localhost:3000",
		SecretKey:     "test-secret-key-that-is-long-enough-123",
		MaxUploadSize: 1024 * 1024,
	}
}

// World is a workspace "acme" owned by Ann, with the team ENG.
type World struct {
	t         *testing.T
	State     *app.State
	Workspace uuid.UUID
	Ann       *app.Ctx
}

func world(t *testing.T) *World {
	t.Helper()
	state := app.NewState(freshDB(t), testConfig())
	w := &World{t: t, State: state}
	ann := w.addUser("ann@acme.test", "Ann Smith")
	ctx := context.Background()
	err := pgx.BeginFunc(ctx, state.DB, func(tx pgx.Tx) error {
		workspace, err := app.One[uuid.UUID](ctx, tx, "INSERT INTO workspaces (name, url_key) VALUES ('Acme', 'acme') RETURNING id")
		if err != nil {
			return err
		}
		w.Workspace = workspace
		if err := app.Exec(ctx, tx, "INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')", workspace, ann); err != nil {
			return err
		}
		if err := ops.WorkspaceDefaults(ctx, tx, workspace); err != nil {
			return err
		}
		_, err = ops.CreateTeam(ctx, tx, workspace, ann, "Engineering", "ENG")
		return err
	})
	if err != nil {
		t.Fatalf("world: %v", err)
	}
	w.Ann = w.ctx(ann, w.Workspace)
	return w
}

func (w *World) addUser(email, name string) uuid.UUID {
	w.t.Helper()
	id, err := app.One[uuid.UUID](context.Background(), w.State.DB,
		"INSERT INTO users (email, name, display_name) VALUES ($1, $2, $3) RETURNING id", email, name, app.DisplayName(name, email))
	if err != nil {
		w.t.Fatalf("add user: %v", err)
	}
	return id
}

func (w *World) ctx(user, workspace uuid.UUID) *app.Ctx {
	w.t.Helper()
	c, err := app.LoadCtx(context.Background(), w.State, user, workspace, app.EveryAccess)
	if err != nil || c == nil {
		w.t.Fatalf("load ctx: %v", err)
	}
	return c
}

// member adds someone to the workspace with role (member, admin or guest).
func (w *World) member(email, name, role string) *app.Ctx {
	w.t.Helper()
	user := w.addUser(email, name)
	w.exec("INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)", w.Workspace, user, role)
	return w.ctx(user, w.Workspace)
}

// secondWorkspace is another workspace owned by Ann, with one team.
func (w *World) secondWorkspace(name, urlKey, teamKey string) uuid.UUID {
	w.t.Helper()
	ctx := context.Background()
	var workspace uuid.UUID
	err := pgx.BeginFunc(ctx, w.State.DB, func(tx pgx.Tx) error {
		var err error
		if workspace, err = app.One[uuid.UUID](ctx, tx, "INSERT INTO workspaces (name, url_key) VALUES ($1, $2) RETURNING id", name, urlKey); err != nil {
			return err
		}
		if err := app.Exec(ctx, tx, "INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')", workspace, w.Ann.User); err != nil {
			return err
		}
		if err := ops.WorkspaceDefaults(ctx, tx, workspace); err != nil {
			return err
		}
		_, err = ops.CreateTeam(ctx, tx, workspace, w.Ann.User, name, teamKey)
		return err
	})
	if err != nil {
		w.t.Fatalf("second workspace: %v", err)
	}
	return workspace
}

// createTeam adds a team to the workspace with Ann as its first member.
func (w *World) createTeam(name, key string) uuid.UUID {
	w.t.Helper()
	ctx := context.Background()
	var team uuid.UUID
	err := pgx.BeginFunc(ctx, w.State.DB, func(tx pgx.Tx) error {
		var err error
		team, err = ops.CreateTeam(ctx, tx, w.Workspace, w.Ann.User, name, key)
		return err
	})
	if err != nil {
		w.t.Fatalf("create team: %v", err)
	}
	return team
}

func (w *World) exec(sql string, args ...any) {
	w.t.Helper()
	if _, err := w.State.DB.Exec(context.Background(), sql, args...); err != nil {
		w.t.Fatalf("exec %s: %v", sql, err)
	}
}

// strings reads one text column of every row.
func (w *World) strings(sql string, args ...any) []string {
	w.t.Helper()
	list, err := app.All[string](context.Background(), w.State.DB, sql, args...)
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
	value, err := app.One[T](context.Background(), w.State.DB, sql, args...)
	if err != nil {
		w.t.Fatalf("query %s: %v", sql, err)
	}
	return value
}

func tryCall(c *app.Ctx, name string, args M) (J, error) {
	op := ops.Find(name)
	if op == nil {
		panic("no operation " + name)
	}
	fresh, err := app.LoadCtx(context.Background(), c.State, c.User, c.Workspace, c.Scopes)
	if err != nil {
		return J{}, err
	}
	if fresh == nil {
		return J{}, app.Forbidden("Not a member of this workspace")
	}
	if args == nil {
		args = M{}
	}
	// The arguments go through JSON, as they do in the API.
	var decoded M
	if err := json.Unmarshal(app.Marshal(args), &decoded); err != nil {
		panic(err)
	}
	result, err := op.Call(fresh, decoded)
	if err != nil {
		return J{}, err
	}
	var value any
	if err := json.Unmarshal(app.Marshal(result), &value); err != nil {
		panic(err)
	}
	return J{value}, nil
}

// call calls an operation and returns its result, failing the test on an error.
func call(t *testing.T, c *app.Ctx, name string, args M) J {
	t.Helper()
	value, err := tryCall(c, name, args)
	if err != nil {
		t.Fatalf("%s failed: %s", name, app.AsError(err).Message)
	}
	return value
}

// fails calls an operation that must fail, returning the error message.
func fails(t *testing.T, c *app.Ctx, name string, args M) string {
	t.Helper()
	value, err := tryCall(c, name, args)
	if err == nil {
		t.Fatalf("%s should have failed but returned %s", name, app.Marshal(value.V))
	}
	return app.AsError(err).Message
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
	return string(app.Marshal(j.V))
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
