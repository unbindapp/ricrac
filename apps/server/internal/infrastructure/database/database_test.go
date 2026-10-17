package database

import (
	"context"
	"crypto/rand"
	"crypto/sha512"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"ricrac/ent"
	"ricrac/ent/schema/pgarray"
	"ricrac/ent/user"
)

func adminURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://ricrac:ricrac@localhost:5436/ricrac"
}

// scratch is an empty database, dropped when the test ends.
func scratch(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, adminURL())
	require.NoError(t, err)
	defer admin.Close(ctx)
	suffix := make([]byte, 6)
	_, _ = rand.Read(suffix)
	name := "ricrac_migrate_" + hex.EncodeToString(suffix)
	_, err = admin.Exec(ctx, "CREATE DATABASE "+name)
	require.NoError(t, err)
	t.Cleanup(func() {
		if admin, err := pgx.Connect(context.Background(), adminURL()); err == nil {
			_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
			admin.Close(context.Background())
		}
	})
	config, err := pgx.ParseConfig(adminURL())
	require.NoError(t, err)
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", config.User, config.Password, config.Host, config.Port, name)
}

func open(t *testing.T, url string) (*ent.Client, *sql.DB) {
	t.Helper()
	client, db, err := Open(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client, db
}

type legacyMigration struct {
	version int64
	sql     string
}

func legacyMigrations(t *testing.T) []legacyMigration {
	t.Helper()
	names, err := fs.Glob(migrations(), "*.sql")
	require.NoError(t, err)
	var found []legacyMigration
	for _, name := range names {
		prefix, _, _ := strings.Cut(name, "_")
		version, err := strconv.ParseInt(prefix, 10, 64)
		require.NoError(t, err)
		if version > lastLegacy {
			continue
		}
		data, err := fs.ReadFile(migrations(), name)
		require.NoError(t, err)
		text := strings.TrimSuffix(strings.TrimPrefix(string(data), legacyHeader), legacyFooter)
		found = append(found, legacyMigration{version, text})
	}
	return found
}

// migrateLegacy migrates as the previous server did: each file in a transaction, recorded in
// _sqlx_migrations with its SHA-384, up to and including version upTo.
func migrateLegacy(t *testing.T, db *sql.DB, upTo int64) {
	t.Helper()
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS _sqlx_migrations (
		version BIGINT PRIMARY KEY,
		description TEXT NOT NULL,
		installed_on TIMESTAMPTZ NOT NULL DEFAULT now(),
		success BOOLEAN NOT NULL,
		checksum BYTEA NOT NULL,
		execution_time BIGINT NOT NULL
	)`)
	require.NoError(t, err)
	for _, m := range legacyMigrations(t) {
		if m.version > upTo {
			break
		}
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, m.sql)
		require.NoError(t, err, "legacy migration %d", m.version)
		sum := sha512.Sum384([]byte(m.sql))
		_, err = tx.ExecContext(ctx, `INSERT INTO _sqlx_migrations (version, description, success, checksum, execution_time)
			VALUES ($1, 'legacy', true, $2, 1)`, m.version, sum[:])
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
	}
}

// schemaOf describes the tables, columns, constraints, indexes and functions, leaving out the
// migration bookkeeping.
func schemaOf(t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), `
		SELECT 'column ' || table_name || '.' || column_name || ' ' || data_type || ' ' || is_nullable || ' ' ||
			coalesce(column_default, '') || ' ' || coalesce(generation_expression, '')
		FROM information_schema.columns WHERE table_schema = 'public'
		UNION ALL
		SELECT 'constraint ' || conrelid::regclass || ' ' || conname || ' ' || pg_get_constraintdef(oid)
		FROM pg_constraint WHERE connamespace = 'public'::regnamespace
		UNION ALL
		SELECT 'index ' || indexdef FROM pg_indexes WHERE schemaname = 'public'
		UNION ALL
		SELECT 'function ' || p.proname || ' ' || pg_get_functiondef(p.oid)
		FROM pg_proc p WHERE p.pronamespace = 'public'::regnamespace`)
	require.NoError(t, err)
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		if strings.Contains(line, "_sqlx_migrations") || strings.Contains(line, "goose_db_version") {
			continue
		}
		lines = append(lines, line)
	}
	require.NoError(t, rows.Err())
	slices.Sort(lines)
	return strings.Join(lines, "\n")
}

func gooseVersions(t *testing.T, db *sql.DB) []int64 {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), "SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY version_id")
	require.NoError(t, err)
	defer rows.Close()
	var versions []int64
	for rows.Next() {
		var v int64
		require.NoError(t, rows.Scan(&v))
		versions = append(versions, v)
	}
	return versions
}

func allVersions(t *testing.T) []int64 {
	t.Helper()
	names, err := fs.Glob(migrations(), "*.sql")
	require.NoError(t, err)
	versions := []int64{0}
	for _, name := range names {
		prefix, _, _ := strings.Cut(name, "_")
		version, err := strconv.ParseInt(prefix, 10, 64)
		require.NoError(t, err)
		versions = append(versions, version)
	}
	return versions
}

func legacyRows(t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), "SELECT version, encode(checksum, 'hex'), installed_on FROM _sqlx_migrations ORDER BY version")
	require.NoError(t, err)
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var version int64
		var checksum string
		var installed time.Time
		require.NoError(t, rows.Scan(&version, &checksum, &installed))
		lines = append(lines, fmt.Sprint(version, checksum, installed.UnixMicro()))
	}
	return strings.Join(lines, "\n")
}

func TestMigratesAnEmptyDatabase(t *testing.T) {
	_, db := open(t, scratch(t))
	require.NoError(t, Migrate(context.Background(), db))
	require.Equal(t, allVersions(t), gooseVersions(t, db))
	var legacy bool
	require.NoError(t, db.QueryRow("SELECT to_regclass('_sqlx_migrations') IS NOT NULL").Scan(&legacy))
	require.False(t, legacy, "new databases don't get the old bookkeeping")
	require.NoError(t, Migrate(context.Background(), db), "migrating again changes nothing")
	require.Equal(t, allVersions(t), gooseVersions(t, db))
}

func TestAdoptsADatabaseOfThePreviousServer(t *testing.T) {
	ctx := context.Background()
	_, fresh := open(t, scratch(t))
	require.NoError(t, Migrate(ctx, fresh))

	client, db := open(t, scratch(t))
	migrateLegacy(t, db, lastLegacy)
	var userID uuid.UUID
	require.NoError(t, db.QueryRow(`INSERT INTO users (email, name, display_name) VALUES ('ann@acme.test', 'Ann', 'ann') RETURNING id`).Scan(&userID))
	before := legacyRows(t, db)

	require.NoError(t, Migrate(ctx, db))
	require.Equal(t, allVersions(t), gooseVersions(t, db))
	require.Equal(t, schemaOf(t, fresh), schemaOf(t, db), "an adopted database has the schema of a new one")
	require.Equal(t, before, legacyRows(t, db), "the old bookkeeping is kept for the previous server")
	ann, err := client.User.Query().Where(user.Email("ann@acme.test")).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, userID, ann.ID)

	require.NoError(t, Migrate(ctx, db))
	require.Equal(t, allVersions(t), gooseVersions(t, db))
}

func TestFinishesADatabaseThePreviousServerLeftBehind(t *testing.T) {
	ctx := context.Background()
	_, fresh := open(t, scratch(t))
	require.NoError(t, Migrate(ctx, fresh))

	_, db := open(t, scratch(t))
	migrateLegacy(t, db, 9)
	require.NoError(t, Migrate(ctx, db))
	require.Equal(t, allVersions(t), gooseVersions(t, db))
	require.Equal(t, schemaOf(t, fresh), schemaOf(t, db))
}

func TestRefusesALegacyDatabaseItDoesntKnow(t *testing.T) {
	ctx := context.Background()
	_, changed := open(t, scratch(t))
	migrateLegacy(t, changed, lastLegacy)
	_, err := changed.Exec("UPDATE _sqlx_migrations SET checksum = '\\x00' WHERE version = 4")
	require.NoError(t, err)
	require.ErrorContains(t, Migrate(ctx, changed), "migration 4 was applied with different SQL")

	_, unknown := open(t, scratch(t))
	migrateLegacy(t, unknown, lastLegacy)
	_, err = unknown.Exec("INSERT INTO _sqlx_migrations (version, description, success, checksum, execution_time) VALUES (13, 'x', true, '\\x00', 1)")
	require.NoError(t, err)
	require.ErrorContains(t, Migrate(ctx, unknown), "migration 13 was applied but isn't one this server knows")
	var adopted bool
	require.NoError(t, unknown.QueryRow("SELECT to_regclass('goose_db_version') IS NOT NULL").Scan(&adopted))
	require.False(t, adopted, "a refused database is left as it was")
}

func TestMigratesOnceWhenServersStartTogether(t *testing.T) {
	url := scratch(t)
	_, db := open(t, url)
	migrateLegacy(t, db, lastLegacy)
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Go(func() {
			_, other := open(t, url)
			errs[i] = Migrate(context.Background(), other)
		})
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, allVersions(t), gooseVersions(t, db))
}

func TestEntReadsAndWritesTheColumnTypes(t *testing.T) {
	ctx := context.Background()
	client, db := open(t, scratch(t))
	require.NoError(t, Migrate(ctx, db))
	ann, err := client.User.Create().SetEmail("ann@acme.test").SetName("Ann").SetDisplayName("ann").Save(ctx)
	require.NoError(t, err)
	workspace, err := client.Workspace.Create().SetName("Acme").SetURLKey("acme").Save(ctx)
	require.NoError(t, err)
	key, err := client.APIKey.Create().SetUserID(ann.ID).SetLabel("k").SetKeyHash("h").SetKeyHint("x").
		SetScopes(pgarray.Text{"read", "write"}).SetWorkspaceIds(pgarray.UUID{workspace.ID}).Save(ctx)
	require.NoError(t, err)
	read, err := client.APIKey.Get(ctx, key.ID)
	require.NoError(t, err)
	require.Equal(t, pgarray.Text{"read", "write"}, read.Scopes)
	require.Equal(t, pgarray.UUID{workspace.ID}, *read.WorkspaceIds)
	require.False(t, read.CreatedAt.IsZero(), "database defaults fill what isn't set")
	everywhere, err := client.APIKey.Create().SetUserID(ann.ID).SetLabel("all").SetKeyHash("h2").SetKeyHint("x").
		SetScopes(pgarray.Text{"read"}).Save(ctx)
	require.NoError(t, err)
	read, err = client.APIKey.Get(ctx, everywhere.ID)
	require.NoError(t, err)
	require.Nil(t, read.WorkspaceIds, "a NULL list stays NULL")
	ws, err := client.Workspace.Get(ctx, workspace.ID)
	require.NoError(t, err)
	require.JSONEq(t, "{}", string(ws.DefaultTeamSettings))
}
