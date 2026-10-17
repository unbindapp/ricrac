package project_repo

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"ricrac/internal/common/errdefs"
)

func TestFiltersCompileToConditionsOnProjects(t *testing.T) {
	me := uuid.New()
	condition, err := Filter(M{
		"status":   M{"type": M{"in": []any{"planned", "started"}}},
		"priority": float64(2),
		"lead":     M{"isMe": true},
	}, me)
	assert.NoError(t, err)
	assert.Equal(t, "(TRUE AND (TRUE AND projects.lead_id = ?) AND projects.priority = ?::int"+
		" AND EXISTS (SELECT 1 FROM project_statuses fs WHERE fs.id = projects.status_id AND (TRUE AND lower(fs.type) = ANY(?::text[]::text[]))))",
		condition.SQL)
	assert.Equal(t, []any{me, "2", []string{"planned", "started"}}, condition.Args)
}

func TestOrFiltersJoinTheirParts(t *testing.T) {
	condition, err := Filter(M{"or": []any{M{"health": "atRisk"}, M{"name": M{"null": true}}}}, uuid.New())
	assert.NoError(t, err)
	assert.Equal(t, "(TRUE AND (FALSE OR (TRUE AND lower(projects.health) = ?::text) OR (TRUE AND (TRUE AND lower(projects.name) IS NULL))))", condition.SQL)
	assert.Equal(t, []any{"atrisk"}, condition.Args)
}

func TestFiltersRefuseUnknownFieldsAndComparators(t *testing.T) {
	_, err := Filter(M{"owner": "me"}, uuid.New())
	assert.Equal(t, `Project filters don't have "owner"; use status, lead, team, priority, health, name, startDate, targetDate, and, or`, errdefs.As(err).Message)
	_, err = Filter(M{"priority": M{"like": float64(1)}}, uuid.New())
	assert.Equal(t, `"like" isn't a comparator; use eq, neq, in, nin, lt, lte, gt, gte or null`, errdefs.As(err).Message)
	_, err = Filter(M{"team": M{"slug": "eng"}}, uuid.New())
	assert.Equal(t, `team filters take id, key, name, not "slug"`, errdefs.As(err).Message)
	_, err = Filter("planned", uuid.New())
	assert.Equal(t, "A project filter must be an object", errdefs.As(err).Message)
}
