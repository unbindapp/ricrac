package app

import (
	"context"
	"crypto/sha512"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxuuid "github.com/vgarvardt/pgx-google-uuid/v5"

	"ricrac/migrations"
)

// Querier is a pool, a connection or a transaction.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Connect opens a pool in UTC and brings the schema up to date.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	config.MaxConns = 20
	config.ConnConfig.RuntimeParams["timezone"] = "UTC"
	config.AfterConnect = func(_ context.Context, conn *pgx.Conn) error {
		pgxuuid.Register(conn.TypeMap())
		return nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

type migration struct {
	version     int64
	description string
	sql         string
	checksum    []byte
}

func readMigrations() ([]migration, error) {
	names, err := fs.Glob(migrations.Files, "*.sql")
	if err != nil {
		return nil, err
	}
	var found []migration
	for _, name := range names {
		prefix, rest, ok := strings.Cut(strings.TrimSuffix(name, ".sql"), "_")
		version, err := strconv.ParseInt(prefix, 10, 64)
		if !ok || err != nil {
			return nil, fmt.Errorf("migration %s isn't named <version>_<description>.sql", name)
		}
		sql, err := fs.ReadFile(migrations.Files, name)
		if err != nil {
			return nil, err
		}
		sum := sha512.Sum384(sql)
		found = append(found, migration{version, strings.ReplaceAll(rest, "_", " "), string(sql), sum[:]})
	}
	slices.SortFunc(found, func(a, b migration) int { return int(a.version - b.version) })
	return found, nil
}

// Migrate applies the migrations not applied yet, each in its own transaction. It keeps the
// bookkeeping of sqlx (the table, versions and SHA-384 checksums), which the server used before.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	all, err := readMigrations()
	if err != nil {
		return err
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	const lock = 0x7269637261630001
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lock); err != nil {
		return err
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", lock)
	_, err = conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS _sqlx_migrations (
		version BIGINT PRIMARY KEY,
		description TEXT NOT NULL,
		installed_on TIMESTAMPTZ NOT NULL DEFAULT now(),
		success BOOLEAN NOT NULL,
		checksum BYTEA NOT NULL,
		execution_time BIGINT NOT NULL
	)`)
	if err != nil {
		return err
	}
	applied := map[int64][]byte{}
	rows, err := conn.Query(ctx, "SELECT version, checksum FROM _sqlx_migrations WHERE success")
	if err != nil {
		return err
	}
	for rows.Next() {
		var version int64
		var checksum []byte
		if err := rows.Scan(&version, &checksum); err != nil {
			return err
		}
		applied[version] = checksum
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, m := range all {
		if checksum, ok := applied[m.version]; ok {
			if !slices.Equal(checksum, m.checksum) {
				return fmt.Errorf("migration %d was changed after it was applied", m.version)
			}
			continue
		}
		started := time.Now()
		err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, m.sql); err != nil {
				return err
			}
			_, err := tx.Exec(ctx,
				`INSERT INTO _sqlx_migrations (version, description, success, checksum, execution_time)
				 VALUES ($1, $2, true, $3, $4)`,
				m.version, m.description, m.checksum, time.Since(started).Nanoseconds())
			return err
		})
		if err != nil {
			return fmt.Errorf("migration %d (%s): %w", m.version, m.description, err)
		}
	}
	return nil
}

// One reads the single column of the one row a query returns; pgx.ErrNoRows when there's none.
func One[T any](ctx context.Context, q Querier, sql string, args ...any) (T, error) {
	var value T
	err := q.QueryRow(ctx, sql, args...).Scan(&value)
	return value, err
}

// Maybe reads the single column of the row a query may return.
func Maybe[T any](ctx context.Context, q Querier, sql string, args ...any) (T, bool, error) {
	value, err := One[T](ctx, q, sql, args...)
	if NoRows(err) {
		return value, false, nil
	}
	return value, err == nil, err
}

// All reads the single column of every row.
func All[T any](ctx context.Context, q Querier, sql string, args ...any) ([]T, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[T])
}

// Rows reads every row into a struct, column by column in order.
func Rows[T any](ctx context.Context, q Querier, sql string, args ...any) ([]T, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[T])
}

// Row reads the row a query may return into a struct, column by column in order.
func Row[T any](ctx context.Context, q Querier, sql string, args ...any) (T, bool, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		var zero T
		return zero, false, err
	}
	value, err := pgx.CollectOneRow(rows, pgx.RowToStructByPos[T])
	if NoRows(err) {
		return value, false, nil
	}
	return value, err == nil, err
}

// Exec runs a statement.
func Exec(ctx context.Context, q Querier, sql string, args ...any) error {
	_, err := q.Exec(ctx, sql, args...)
	return err
}
