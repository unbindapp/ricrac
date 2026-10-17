// Package errdefs is the errors callers can act on: the message says what was wrong. Database
// failures are logged and reported without detail.
package errdefs

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
)

type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }

func (e *Error) GetStatus() int { return e.Status }

func (e *Error) MarshalJSON() ([]byte, error) {
	return Marshal(map[string]string{"error": e.Message}), nil
}

func (e *Error) ContentType(string) string { return "application/json" }

func Errorf(status int, format string, args ...any) *Error {
	return &Error{Status: status, Message: fmt.Sprintf(format, args...)}
}

func BadRequest(format string, args ...any) *Error {
	return Errorf(http.StatusBadRequest, format, args...)
}

func NotFound(format string, args ...any) *Error {
	return Errorf(http.StatusNotFound, format, args...)
}

func Forbidden(format string, args ...any) *Error {
	return Errorf(http.StatusForbidden, format, args...)
}

func Conflict(format string, args ...any) *Error {
	return Errorf(http.StatusConflict, format, args...)
}

func Unauthorized() *Error {
	return Errorf(http.StatusUnauthorized, "Not signed in")
}

// As turns any error into one for the caller: constraint violations become 409s and 400s,
// anything else unexpected a logged 500.
func As(err error) *Error {
	if err == nil {
		return nil
	}
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			constraint := pgErr.ConstraintName
			if constraint == "" {
				constraint = "unique"
			}
			return Conflict("Already exists (%s)", constraint)
		case "23503":
			return BadRequest("Refers to something that doesn't exist or is still in use")
		}
	}
	slog.Error("unexpected error", "err", err)
	return Errorf(http.StatusInternalServerError, "Something went wrong")
}

// NoRows says whether a query found nothing.
func NoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func Write(w http.ResponseWriter, err error) {
	apiErr := As(err)
	WriteJSON(w, apiErr.Status, map[string]any{"error": apiErr.Message})
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
}

// Marshal encodes JSON without escaping <, > and &.
func Marshal(value any) []byte {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
	return bytes.TrimRight(buffer.Bytes(), "\n")
}
