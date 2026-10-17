package issue_repo

import (
	"strings"

	entsql "entgo.io/ent/dialect/sql"

	"ricrac/internal/common/utils"
	repository "ricrac/internal/repositories"
)

// IssueAlias stands for the issues table in a Condition until it is applied to a query.
const IssueAlias = "{i}"

// Condition is SQL about an issue, with ? for each of Args and IssueAlias for the issues table.
type Condition struct {
	SQL  string
	Args []any
}

// On is the condition on the issues table known in the query as alias.
func (c Condition) On(alias string) *entsql.Predicate {
	return repository.Raw(strings.ReplaceAll(c.SQL, IssueAlias, alias), c.Args...)
}

// SQL builds a Condition.
type SQL struct {
	text strings.Builder
	args []any
}

func (q *SQL) Push(text string) *SQL {
	q.text.WriteString(text)
	return q
}

// Bind adds an argument.
func (q *SQL) Bind(value any) *SQL {
	q.text.WriteString("?")
	q.args = append(q.args, value)
	return q
}

// Append adds a condition built elsewhere.
func (q *SQL) Append(c Condition) *SQL {
	q.text.WriteString(c.SQL)
	q.args = append(q.args, c.Args...)
	return q
}

func (q *SQL) Condition() Condition {
	return Condition{SQL: q.text.String(), Args: q.args}
}

// Search matches the issue at alias i against query, as words in its title, description and
// comments.
func Search(i, query string) Condition {
	q := &SQL{}
	q.Push("(" + i + ".search @@ websearch_to_tsquery('english', ").Bind(query)
	q.Push(") OR " + i + ".title ILIKE ").Bind("%" + utils.EscapeLike(query) + "%")
	q.Push(" OR EXISTS (SELECT 1 FROM comments sc WHERE sc.issue_id = " + i + ".id AND sc.search @@ websearch_to_tsquery('english', ").Bind(query)
	q.Push(")))")
	return q.Condition()
}
