package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	timeLayout = "2006-01-02T15:04:05.999999-07:00"
	dateLayout = "2006-01-02"
)

// Time is a timestamp as Postgres writes it in JSON: UTC, microseconds without trailing zeros,
// and +00:00.
func Time(t time.Time) string {
	return t.UTC().Truncate(time.Microsecond).Format(timeLayout)
}

func TimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	value := Time(*t)
	return &value
}

// Date is a calendar day, YYYY-MM-DD.
func Date(t time.Time) string {
	return t.Format(dateLayout)
}

func DatePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	value := Date(*t)
	return &value
}

// ID is an id as JSON shows it.
func ID(id uuid.UUID) string {
	return id.String()
}

func IDPtr(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	value := id.String()
	return &value
}
