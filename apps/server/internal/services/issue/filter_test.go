package issue_service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ricrac/internal/common/errdefs"
)

func TestFiltersCompileToSQLWithBoundValues(t *testing.T) {
	me := uuid.New()
	state := uuid.New()
	cases := []struct {
		name   string
		filter M
		sql    string
		args   []any
	}{
		{
			name:   "fields are required in sorted order",
			filter: M{"title": M{"contains": "50%"}, "priority": M{"lte": 2.0, "neq": 0.0}},
			sql:    "(TRUE AND (TRUE AND {i}.priority::float8 <= ? AND {i}.priority::float8 IS DISTINCT FROM ?) AND (TRUE AND {i}.title ILIKE ?))",
			args:   []any{2.0, 0.0, `%50\%%`},
		},
		{
			name:   "or joins its parts",
			filter: M{"or": []any{M{"state": M{"id": state.String()}}, M{"assignee": M{"isMe": true}}}},
			sql: "(TRUE AND ((TRUE AND EXISTS (SELECT 1 FROM workflow_states s WHERE s.id = {i}.state_id AND (TRUE AND (TRUE AND s.id = ?)))) OR " +
				"(TRUE AND (TRUE AND {i}.assignee_id IS NOT DISTINCT FROM ?))))",
			args: []any{state, me},
		},
		{
			name:   "me in user ids is the caller",
			filter: M{"creator": M{"id": M{"in": []any{"me"}}}},
			sql:    "(TRUE AND (TRUE AND EXISTS (SELECT 1 FROM users uf WHERE uf.id = {i}.creator_id AND (TRUE AND (TRUE AND uf.id = ANY(?))))))",
			args:   []any{[]uuid.UUID{me}},
		},
		{
			name:   "children filter sub-issues",
			filter: M{"children": M{"every": M{"number": 3.0}}},
			sql: "(TRUE AND NOT EXISTS (SELECT 1 FROM issues ch WHERE ch.parent_id = {i}.id AND ch.trashed_at IS NULL AND NOT " +
				"(TRUE AND (TRUE AND ch.number::float8 = ?))))",
			args: []any{3.0},
		},
		{
			name:   "null project",
			filter: M{"project": M{"null": true}},
			sql:    "(TRUE AND (TRUE AND {i}.project_id IS NULL))",
		},
		{
			name:   "labels without any",
			filter: M{"labels": M{"null": true}},
			sql:    "(TRUE AND NOT EXISTS (SELECT 1 FROM issue_label_links ll WHERE ll.issue_id = {i}.id))",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			condition, err := CompileFilter(c.filter, me)
			require.NoError(t, err)
			assert.Equal(t, c.sql, condition.SQL)
			assert.Equal(t, c.args, condition.Args)
		})
	}
}

func TestEmptyFiltersAddNothing(t *testing.T) {
	condition, err := CompileFilter(M{}, uuid.New())
	require.NoError(t, err)
	assert.Nil(t, condition)
}

func TestFilterMistakesSayWhatIsWrong(t *testing.T) {
	cases := []struct {
		filter  any
		message string
	}{
		{"open", "filter must be an object"},
		{M{"or": M{}}, "Invalid filter on or: expected a list of filters"},
		{M{"id": "ENG-1"}, "Invalid filter on id: ids must be UUIDs"},
		{M{"state": M{"color": "red"}}, `Invalid filter on state: unknown field "color"; use id, name, type`},
		{M{"title": M{"near": "x"}}, "Invalid filter on title: unknown comparator near"},
		{M{"dueDate": M{"gte": "soon"}}, `"soon" isn't a time; use ISO 8601 (2026-01-31 or 2026-01-31T12:00:00Z) or a duration like -P1D`},
		{M{"priority": nil}, `Invalid filter on priority: use {"null": true} for unset values`},
		{M{"subscribers": M{"id": "x"}}, `Invalid filter on subscribers: use {"some": {...user filter}}`},
	}
	for _, c := range cases {
		_, err := CompileFilter(c.filter, uuid.New())
		require.Error(t, err)
		assert.Equal(t, c.message, errdefs.As(err).Message)
	}
	_, err := CompileFilter(M{"colour": "red"}, uuid.New())
	assert.Equal(t, `Unknown filter field "colour". `+FilterHelp, errdefs.As(err).Message)
}
