package repository

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"ricrac/ent"
)

//go:generate go run -mod=mod github.com/vburenin/ifacemaker -f "*.go" -i BaseRepositoryInterface -p repository -s BaseRepository -o repository_iface.go
type BaseRepository struct {
	DB *ent.Client
}

func NewBaseRepository(db *ent.Client) *BaseRepository {
	return &BaseRepository{DB: db}
}

// WithTx runs fn in a transaction, committed when it returns nil.
func (r *BaseRepository) WithTx(ctx context.Context, fn func(tx TxInterface) error) error {
	tx, err := r.DB.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if v := recover(); v != nil {
			slog.Error("panic in a transaction", "panic", v, "stack", string(debug.Stack()))
			_ = tx.Rollback()
			panic(v)
		}
	}()
	if err := fn(tx); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			err = fmt.Errorf("%w: rolling back transaction: %v", err, rerr)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

// Client is the transaction's client, or the pool's without one.
func (r *BaseRepository) Client(tx TxInterface) *ent.Client {
	if tx != nil {
		return tx.Client()
	}
	return r.DB
}

// Now is Postgres' now(): the transaction's start, so what it writes shares one time.
func (r *BaseRepository) Now(ctx context.Context, tx TxInterface) (time.Time, error) {
	rows, err := r.Client(tx).QueryContext(ctx, "SELECT now()")
	if err != nil {
		return time.Time{}, err
	}
	defer rows.Close()
	var now time.Time
	if rows.Next() {
		err = rows.Scan(&now)
	}
	if err == nil {
		err = rows.Err()
	}
	return now.UTC(), err
}

type TxInterface interface {
	Commit() error
	Rollback() error
	Client() *ent.Client
}
