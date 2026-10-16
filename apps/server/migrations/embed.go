// Package migrations holds the schema, applied in order on start.
package migrations

import "embed"

//go:embed *.sql
var Files embed.FS
