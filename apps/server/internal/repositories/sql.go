package repository

import (
	"strings"

	entsql "entgo.io/ent/dialect/sql"
)

// Raw is a predicate written in SQL, each ? standing for the next argument. Postgres' jsonb ?
// operators can't be written here; use jsonb_exists and its kin instead.
func Raw(sql string, args ...any) *entsql.Predicate {
	return entsql.P(func(b *entsql.Builder) {
		parts := strings.Split(sql, "?")
		for i, part := range parts {
			b.WriteString(part)
			if i < len(parts)-1 {
				b.Arg(args[i])
			}
		}
	})
}

// RawExpr is an SQL expression for orders and selections, each ? standing for the next argument.
func RawExpr(sql string, args ...any) entsql.Querier {
	return entsql.ExprFunc(func(b *entsql.Builder) {
		parts := strings.Split(sql, "?")
		for i, part := range parts {
			b.WriteString(part)
			if i < len(parts)-1 {
				b.Arg(args[i])
			}
		}
	})
}
