package schema

import "entgo.io/ent/dialect"

var (
	integer   = map[string]string{dialect.Postgres: "integer"}
	date      = map[string]string{dialect.Postgres: "date"}
	textArray = map[string]string{dialect.Postgres: "text[]"}
	uuidArray = map[string]string{dialect.Postgres: "uuid[]"}
)
