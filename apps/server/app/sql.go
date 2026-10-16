package app

import (
	"strconv"
	"strings"
)

// SQL builds a query with bound values: Bind writes the next $n.
type SQL struct {
	text strings.Builder
	Args []any
}

func NewSQL(text string, args ...any) *SQL {
	q := &SQL{Args: args}
	q.text.WriteString(text)
	return q
}

func (q *SQL) Push(text string) *SQL {
	q.text.WriteString(text)
	return q
}

func (q *SQL) Bind(value any) *SQL {
	q.Args = append(q.Args, value)
	q.text.WriteString("$" + strconv.Itoa(len(q.Args)))
	return q
}

func (q *SQL) String() string {
	return q.text.String()
}
