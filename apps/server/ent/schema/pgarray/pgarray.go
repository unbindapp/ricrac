// Package pgarray maps Postgres text[] and uuid[] columns for ent.
package pgarray

import (
	"database/sql/driver"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var types = pgtype.NewMap()

// Text is a text[] column; NULL scans as an empty list.
type Text []string

func (a *Text) Scan(src any) error {
	var list []string
	if err := types.SQLScanner(&list).Scan(src); err != nil {
		return err
	}
	*a = list
	return nil
}

func (a Text) Value() (driver.Value, error) {
	if a == nil {
		return []string{}, nil
	}
	return []string(a), nil
}

// UUID is a uuid[] column; NULL scans as an empty list.
type UUID []uuid.UUID

func (a *UUID) Scan(src any) error {
	var texts []string
	if err := types.SQLScanner(&texts).Scan(src); err != nil {
		return err
	}
	if texts == nil {
		*a = nil
		return nil
	}
	list := make([]uuid.UUID, len(texts))
	for i, text := range texts {
		id, err := uuid.Parse(text)
		if err != nil {
			return err
		}
		list[i] = id
	}
	*a = list
	return nil
}

func (a UUID) Value() (driver.Value, error) {
	if a == nil {
		return []uuid.UUID{}, nil
	}
	return []uuid.UUID(a), nil
}
