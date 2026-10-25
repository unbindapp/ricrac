// Package database opens Postgres for ent and brings its schema up to date with goose.
package database

import (
	"context"
	"database/sql"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	pgxuuid "github.com/vgarvardt/pgx-google-uuid/v5"

	"ricrac/ent"
	_ "ricrac/ent/runtime"
)

const (
	maxConnections = 20
	maxIdleTime    = 5 * time.Minute
)

// Open connects in UTC through pgx's database/sql driver.
func Open(ctx context.Context, url string) (*ent.Client, *sql.DB, error) {
	config, err := pgx.ParseConfig(url)
	if err != nil {
		return nil, nil, err
	}
	config.RuntimeParams["timezone"] = "UTC"
	db := stdlib.OpenDB(*config, stdlib.OptionAfterConnect(func(_ context.Context, conn *pgx.Conn) error {
		pgxuuid.Register(conn.TypeMap())
		return nil
	}))
	db.SetMaxOpenConns(maxConnections)
	// database/sql keeps 2 idle by default, so bursts reconnect (DNS, TCP, SCRAM) inside requests.
	db.SetMaxIdleConns(maxConnections)
	db.SetConnMaxIdleTime(maxIdleTime)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	driver := entsql.OpenDB("postgres", db)
	return ent.NewClient(ent.Driver(driver)), db, nil
}
