package database

import (
	"bytes"
	"context"
	"crypto/sha512"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"strconv"
	"strings"

	"github.com/pressly/goose/v3"
	gooseDatabase "github.com/pressly/goose/v3/database"

	"ricrac/ent/migrate"
)

// The server kept sqlx's _sqlx_migrations bookkeeping before goose; its migrations are goose's
// first versions, wrapped in these lines.
const (
	legacyHeader  = "-- +goose Up\n-- +goose StatementBegin\n"
	legacyFooter  = "-- +goose StatementEnd\n"
	lastLegacy    = 12
	migrationLock = 0x7269637261630001
)

// Migrate applies the migrations not applied yet. A database migrated by the old bookkeeping
// is adopted first: what it applied is recorded as applied for goose, after checking it ran the
// same SQL. _sqlx_migrations is left as it is, so the previous server still starts on it.
func Migrate(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrationLock); err != nil {
		return err
	}
	defer conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLock)
	if err := adoptLegacy(ctx, conn); err != nil {
		return fmt.Errorf("adopting the previous migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations(), goose.WithAllowOutofOrder(true))
	if err != nil {
		return err
	}
	results, err := provider.Up(ctx)
	for _, result := range results {
		slog.Info("migrated", "version", result.Source.Version, "file", result.Source.Path, "took", result.Duration)
	}
	return err
}

func migrations() fs.FS {
	files, err := fs.Sub(migrate.MigrationsFS, "migrations")
	if err != nil {
		panic(err)
	}
	return files
}

func tableExists(ctx context.Context, conn *sql.Conn, name string) (bool, error) {
	var exists bool
	err := conn.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", name).Scan(&exists)
	return exists, err
}

func adoptLegacy(ctx context.Context, conn *sql.Conn) error {
	adopted, err := tableExists(ctx, conn, "goose_db_version")
	if err != nil || adopted {
		return err
	}
	legacy, err := tableExists(ctx, conn, "_sqlx_migrations")
	if err != nil || !legacy {
		return err
	}
	checksums, err := legacyChecksums()
	if err != nil {
		return err
	}
	rows, err := conn.QueryContext(ctx, "SELECT version, checksum FROM _sqlx_migrations WHERE success ORDER BY version")
	if err != nil {
		return err
	}
	applied := map[int64][]byte{}
	for rows.Next() {
		var version int64
		var checksum []byte
		if err := rows.Scan(&version, &checksum); err != nil {
			rows.Close()
			return err
		}
		applied[version] = checksum
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for version, checksum := range applied {
		expected, known := checksums[version]
		if !known {
			return fmt.Errorf("migration %d was applied but isn't one this server knows", version)
		}
		if !bytes.Equal(expected, checksum) {
			return fmt.Errorf("migration %d was applied with different SQL than this server's", version)
		}
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	store, err := gooseDatabase.NewStore(goose.DialectPostgres, goose.DefaultTablename)
	if err != nil {
		return err
	}
	if err := store.CreateVersionTable(ctx, tx); err != nil {
		return err
	}
	for version := int64(0); version <= lastLegacy; version++ {
		if _, ok := applied[version]; !ok && version > 0 {
			continue
		}
		if err := store.Insert(ctx, tx, gooseDatabase.InsertRequest{Version: version}); err != nil {
			return err
		}
	}
	slog.Info("adopted the previous migrations for goose", "applied", len(applied))
	return tx.Commit()
}

// legacyChecksums is the SHA-384 the old bookkeeping stored for each of its migrations.
func legacyChecksums() (map[int64][]byte, error) {
	files, err := fs.Glob(migrations(), "*.sql")
	if err != nil {
		return nil, err
	}
	checksums := map[int64][]byte{}
	for _, name := range files {
		prefix, _, _ := strings.Cut(name, "_")
		version, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil || version > lastLegacy {
			continue
		}
		data, err := fs.ReadFile(migrations(), name)
		if err != nil {
			return nil, err
		}
		text := string(data)
		if !strings.HasPrefix(text, legacyHeader) || !strings.HasSuffix(text, legacyFooter) {
			return nil, fmt.Errorf("%s lost the lines that wrap the previous migration", name)
		}
		sum := sha512.Sum384([]byte(strings.TrimSuffix(strings.TrimPrefix(text, legacyHeader), legacyFooter)))
		checksums[version] = sum[:]
	}
	return checksums, nil
}
