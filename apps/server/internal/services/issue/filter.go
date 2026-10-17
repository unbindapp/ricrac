package issue_service

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
	repository "ricrac/internal/repositories"
	issue_repo "ricrac/internal/repositories/issue"
)

// FilterHelp describes the filter language, for errors about filters.
const FilterHelp = `A filter object. Fields: id, number, title, description, ` +
	`priority, estimate, dueDate, createdAt, updatedAt, startedAt, completedAt, canceledAt, archivedAt, triagedAt, ` +
	`snoozedUntil, slaBreachesAt, state {id,name,type}, team {id,key,name}, assignee / creator {id,email,name,displayName,isMe}, ` +
	`labels {some|every|none: {id,name,parent}}, project {id,name,slugId}, projectMilestone {id,name}, cycle {id,number,name,isActive}, ` +
	`parent {id,identifier}, subscribers {some: user}, hasBlockedByRelations, hasBlockingRelations, hasRelatedRelations, ` +
	`hasDuplicateRelations, children {some: filter}, searchableContent {contains}, and, or. Comparators: eq, neq, in, nin, ` +
	`lt, lte, gt, gte, contains, notContains, startsWith, endsWith, null (true for empty). Dates take ISO 8601 or durations like ` +
	`-P2W. Text compares ignore case. Example: {"or": [{"priority": {"lte": 2, "neq": 0}}, {"labels": {"some": {"name": {"eq": "Bug"}}}}]}`

// CompileFilter turns an issue filter, e.g. {"state": {"type": {"in": ["started"]}},
// "assignee": {"isMe": {"eq": true}}}, into a condition; me is who isMe means. Nil for an empty
// filter.
func CompileFilter(filter any, me uuid.UUID) (*issue_repo.Condition, error) {
	object, err := asObject(filter, "filter")
	if err != nil || len(object) == 0 {
		return nil, err
	}
	q := &compiler{SQL: &issue_repo.SQL{}, me: me}
	if err := q.issue(issue_repo.IssueAlias, object); err != nil {
		return nil, err
	}
	condition := q.Condition()
	return &condition, nil
}

// CheckFilter compiles and runs the filter in tx, so that mistakes in it surface now rather
// than when it is used.
func (self *IssueService) CheckFilter(ctx context.Context, tx repository.TxInterface, actor *auth.Actor, filter any) error {
	condition, err := CompileFilter(filter, actor.User)
	if err != nil {
		return err
	}
	err = self.repo.Issue().TryFilter(ctx, tx, condition)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return errdefs.BadRequest("The filter doesn't work: %s. %s", pgErr.Message, FilterHelp)
	}
	return err
}

type compiler struct {
	*issue_repo.SQL
	me uuid.UUID
}

func asObject(value any, what string) (M, error) {
	object, ok := value.(M)
	if !ok {
		return nil, errdefs.BadRequest("%s must be an object", what)
	}
	return object, nil
}

func badFilter(field, message string) error {
	return errdefs.BadRequest("Invalid filter on %s: %s", field, message)
}

func sortedKeys(object M) []string {
	return slices.Sorted(maps.Keys(object))
}

func without(object M, key string) (M, any, bool) {
	rest := maps.Clone(object)
	value, found := rest[key]
	delete(rest, key)
	return rest, value, found
}

func nullClause(column, field string, value any) (string, error) {
	null, ok := value.(bool)
	if !ok {
		return "", badFilter(field, "null takes true or false")
	}
	if null {
		return column + " IS NULL", nil
	}
	return column + " IS NOT NULL", nil
}

var relationExists = map[string]string{
	"hasBlockedByRelations": "EXISTS (SELECT 1 FROM issue_relations r WHERE r.related_issue_id = %[1]s.id AND r.type = 'blocks')",
	"hasBlockingRelations":  "EXISTS (SELECT 1 FROM issue_relations r WHERE r.issue_id = %[1]s.id AND r.type = 'blocks')",
	"hasDuplicateRelations": "EXISTS (SELECT 1 FROM issue_relations r WHERE (r.issue_id = %[1]s.id OR r.related_issue_id = %[1]s.id) AND r.type = 'duplicate')",
}

// issue requires every field of the filter, for the issue at alias i; and / or nest.
func (q *compiler) issue(i string, filter M) error {
	q.Push("(TRUE")
	for _, field := range sortedKeys(filter) {
		value := filter[field]
		q.Push(" AND ")
		if err := q.field(i, field, value); err != nil {
			return err
		}
	}
	q.Push(")")
	return nil
}

func (q *compiler) field(i, field string, value any) error {
	switch field {
	case "and", "or":
		return q.combine(i, field, value)
	case "id":
		return q.ids(i+".id", field, value)
	case "number", "priority", "estimate":
		return q.numbers(i+"."+field, field, value)
	case "title", "description":
		return q.texts(i+"."+field, field, value)
	case "dueDate":
		return q.dates(i+".due_date", field, value, true)
	case "createdAt", "updatedAt", "startedAt", "completedAt", "canceledAt", "archivedAt", "triagedAt",
		"snoozedUntil", "slaBreachesAt", "autoClosedAt", "startedTriageAt":
		return q.dates(i+"."+snake(field), field, value, false)
	case "state":
		return q.related(field, value, fmt.Sprintf("EXISTS (SELECT 1 FROM workflow_states s WHERE s.id = %s.state_id AND ", i),
			[]known{{"id", kindID, "s.id"}, {"name", kindText, "s.name"}, {"type", kindText, "s.type"}})
	case "team":
		return q.related(field, value, fmt.Sprintf("EXISTS (SELECT 1 FROM teams t2 WHERE t2.id = %s.team_id AND ", i),
			[]known{{"id", kindID, "t2.id"}, {"key", kindText, "t2.key"}, {"name", kindText, "t2.name"}})
	case "assignee", "creator", "snoozedBy":
		column := map[string]string{"assignee": "assignee_id", "creator": "creator_id", "snoozedBy": "snoozed_by_id"}[field]
		object, err := asObject(value, field)
		if err != nil {
			return err
		}
		return q.user(i+"."+column, field, object)
	case "subscribers":
		return q.subscribers(i, field, value)
	case "labels":
		object, err := asObject(value, field)
		if err != nil {
			return err
		}
		return q.labels(i, field, object)
	case "project":
		return q.nullableRelated(i+".project_id", field, value,
			fmt.Sprintf("EXISTS (SELECT 1 FROM projects p2 WHERE p2.id = %s.project_id AND ", i),
			[]known{{"id", kindID, "p2.id"}, {"name", kindText, "p2.name"}, {"slugId", kindText, "p2.slug_id"}, {"priority", kindNumber, "p2.priority"}})
	case "projectMilestone", "milestone":
		return q.nullableRelated(i+".milestone_id", field, value,
			fmt.Sprintf("EXISTS (SELECT 1 FROM project_milestones m2 WHERE m2.id = %s.milestone_id AND ", i),
			[]known{{"id", kindID, "m2.id"}, {"name", kindText, "m2.name"}})
	case "cycle":
		return q.cycle(i, field, value)
	case "parent":
		return q.parent(i, field, value)
	case "children":
		return q.children(i, field, value)
	case "hasBlockedByRelations", "hasBlockingRelations", "hasRelatedRelations", "hasDuplicateRelations", "hasSuggestedRelatedIssues":
		want, ok := boolEq(value)
		if !ok {
			return badFilter(field, `takes {"eq": true|false}`)
		}
		exists := relationExists[field]
		if exists == "" {
			exists = "EXISTS (SELECT 1 FROM issue_relations r WHERE (r.issue_id = %[1]s.id OR r.related_issue_id = %[1]s.id) AND r.type = 'related')"
		}
		clause := fmt.Sprintf(exists, i)
		if !want {
			clause = "NOT " + clause
		}
		q.Push(clause)
		return nil
	case "searchableContent":
		object, err := asObject(value, field)
		if err != nil {
			return err
		}
		text, ok := object["contains"].(string)
		if !ok {
			return badFilter(field, `takes {"contains": "text"}`)
		}
		q.Append(issue_repo.Search(i, text))
		return nil
	case "release", "releases":
		object, err := asObject(value, field)
		if err != nil {
			return err
		}
		if some, ok := object["some"].(M); ok {
			object = some
		}
		q.Push(fmt.Sprintf("EXISTS (SELECT 1 FROM issue_releases ir JOIN releases r2 ON r2.id = ir.release_id WHERE ir.issue_id = %s.id AND ", i))
		if err := q.fields(field, object, []known{{"id", kindID, "r2.id"}, {"name", kindText, "r2.name"}, {"version", kindText, "r2.version"}}); err != nil {
			return err
		}
		q.Push(")")
		return nil
	}
	return errdefs.BadRequest("Unknown filter field %q. %s", field, FilterHelp)
}

func (q *compiler) combine(i, field string, value any) error {
	parts, ok := value.([]any)
	if !ok {
		return badFilter(field, "expected a list of filters")
	}
	joiner := " AND "
	if field == "or" {
		joiner = " OR "
	}
	q.Push("(")
	if len(parts) == 0 {
		q.Push("TRUE")
	}
	for n, part := range parts {
		if n > 0 {
			q.Push(joiner)
		}
		object, err := asObject(part, field)
		if err != nil {
			return err
		}
		if err := q.issue(i, object); err != nil {
			return err
		}
	}
	q.Push(")")
	return nil
}

func (q *compiler) subscribers(i, field string, value any) error {
	object, err := asObject(value, field)
	if err != nil {
		return err
	}
	inner, ok := object["some"]
	if !ok {
		return badFilter(field, `use {"some": {...user filter}}`)
	}
	innerObject, err := asObject(inner, field)
	if err != nil {
		return err
	}
	q.Push(fmt.Sprintf("EXISTS (SELECT 1 FROM issue_subscribers sub WHERE sub.issue_id = %s.id AND ", i))
	if err := q.user("sub.user_id", field, innerObject); err != nil {
		return err
	}
	q.Push(")")
	return nil
}

// related is EXISTS (<prefix> <fields of the related thing>).
func (q *compiler) related(field string, value any, prefix string, known []known) error {
	object, err := asObject(value, field)
	if err != nil {
		return err
	}
	q.Push(prefix)
	if err := q.fields(field, object, known); err != nil {
		return err
	}
	q.Push(")")
	return nil
}

// nullableRelated takes {"null": true} (unset), {"null": false} (set), or the related thing's
// fields.
func (q *compiler) nullableRelated(column, field string, value any, prefix string, known []known) error {
	object, err := asObject(value, field)
	if err != nil {
		return err
	}
	return q.nullableRef(column, field, object, func(rest M) error {
		q.Push(prefix)
		if err := q.fields(field, rest, known); err != nil {
			return err
		}
		q.Push(")")
		return nil
	})
}

func (q *compiler) nullableRef(column, field string, object M, rest func(M) error) error {
	remaining, null, hasNull := without(object, "null")
	q.Push("(TRUE")
	if hasNull {
		clause, err := nullClause(column, field, null)
		if err != nil {
			return err
		}
		q.Push(" AND " + clause)
	}
	if len(remaining) > 0 {
		q.Push(" AND ")
		if err := rest(remaining); err != nil {
			return err
		}
	}
	q.Push(")")
	return nil
}

func (q *compiler) cycle(i, field string, value any) error {
	object, err := asObject(value, field)
	if err != nil {
		return err
	}
	return q.nullableRef(i+".cycle_id", field, object, func(f M) error {
		q.Push(fmt.Sprintf("EXISTS (SELECT 1 FROM cycles c2 WHERE c2.id = %s.cycle_id AND ", i))
		rest, active, hasActive := without(f, "isActive")
		if hasActive {
			want, ok := boolEq(active)
			if !ok {
				return badFilter(field, `isActive takes {"eq": true|false}`)
			}
			if want {
				q.Push("(c2.starts_at <= now() AND c2.ends_at > now()) AND ")
			} else {
				q.Push("NOT (c2.starts_at <= now() AND c2.ends_at > now()) AND ")
			}
		}
		if err := q.fields(field, rest, []known{{"id", kindID, "c2.id"}, {"number", kindNumber, "c2.number"}, {"name", kindText, "c2.name"}}); err != nil {
			return err
		}
		q.Push(")")
		return nil
	})
}

func (q *compiler) parent(i, field string, value any) error {
	object, err := asObject(value, field)
	if err != nil {
		return err
	}
	return q.nullableRef(i+".parent_id", field, object, func(f M) error {
		rest, identifier, hasIdentifier := without(f, "identifier")
		if hasIdentifier {
			object, _ := identifier.(M)
			v, ok := object["eq"].(string)
			if !ok {
				return badFilter(field, `identifier takes {"eq": "ENG-1"}`)
			}
			q.Push(fmt.Sprintf("issue_identifier(%s.parent_id) = upper(", i)).Bind(v).Push(") AND ")
		}
		return q.fields(field, rest, []known{{"id", kindID, i + ".parent_id"}})
	})
}

func (q *compiler) children(i, field string, value any) error {
	object, err := asObject(value, field)
	if err != nil {
		return err
	}
	mode, inner, err := collection(field, object)
	if err != nil {
		return err
	}
	sub := fmt.Sprintf("EXISTS (SELECT 1 FROM issues ch WHERE ch.parent_id = %s.id AND ch.trashed_at IS NULL AND ", i)
	switch mode {
	case "some":
		q.Push(sub)
	case "none":
		q.Push("NOT " + sub)
	default:
		q.Push(fmt.Sprintf("NOT EXISTS (SELECT 1 FROM issues ch WHERE ch.parent_id = %s.id AND ch.trashed_at IS NULL AND NOT ", i))
	}
	if err := q.issue("ch", inner); err != nil {
		return err
	}
	q.Push(")")
	return nil
}

func snake(field string) string {
	var out strings.Builder
	for _, c := range field {
		if c >= 'A' && c <= 'Z' {
			out.WriteByte('_')
			out.WriteRune(c + ('a' - 'A'))
			continue
		}
		out.WriteRune(c)
	}
	return out.String()
}

func boolEq(value any) (bool, bool) {
	if b, ok := value.(bool); ok {
		return b, true
	}
	object, _ := value.(M)
	b, ok := object["eq"].(bool)
	return b, ok
}

func collection(field string, object M) (string, M, error) {
	for _, mode := range []string{"some", "every", "none"} {
		if inner, ok := object[mode]; ok {
			innerObject, err := asObject(inner, field)
			return mode, innerObject, err
		}
	}
	return "some", object, nil
}

func (q *compiler) labels(i, field string, object M) error {
	if null, ok := object["null"]; ok {
		isNull, ok := null.(bool)
		if !ok {
			return badFilter(field, "null takes true or false")
		}
		exists := fmt.Sprintf("EXISTS (SELECT 1 FROM issue_label_links ll WHERE ll.issue_id = %s.id)", i)
		if isNull {
			exists = "NOT " + exists
		}
		q.Push(exists)
		return nil
	}
	mode, inner, err := collection(field, object)
	if err != nil {
		return err
	}
	labelFields := func(inner M) error {
		rest, parent, hasParent := without(inner, "parent")
		if hasParent {
			parentObject, err := asObject(parent, field)
			if err != nil {
				return err
			}
			q.Push("EXISTS (SELECT 1 FROM issue_labels lp WHERE lp.id = lb.parent_id AND ")
			if err := q.fields(field, parentObject, []known{{"id", kindID, "lp.id"}, {"name", kindText, "lp.name"}}); err != nil {
				return err
			}
			q.Push(") AND ")
		}
		return q.fields(field, rest, []known{{"id", kindID, "lb.id"}, {"name", kindText, "lb.name"}})
	}
	join := fmt.Sprintf("SELECT 1 FROM issue_label_links ll JOIN issue_labels lb ON lb.id = ll.label_id WHERE ll.issue_id = %s.id AND ", i)
	switch mode {
	case "some":
		q.Push("EXISTS (" + join)
	case "none":
		q.Push("NOT EXISTS (" + join)
	default:
		q.Push(fmt.Sprintf("EXISTS (SELECT 1 FROM issue_label_links lx WHERE lx.issue_id = %s.id) AND NOT EXISTS (%s NOT ", i, join))
	}
	if mode == "every" {
		q.Push("(")
		if err := labelFields(inner); err != nil {
			return err
		}
		q.Push(")")
	} else if err := labelFields(inner); err != nil {
		return err
	}
	q.Push(")")
	return nil
}

func (q *compiler) user(column, field string, object M) error {
	rest := maps.Clone(object)
	q.Push("(TRUE")
	if null, ok := rest["null"]; ok {
		delete(rest, "null")
		clause, err := nullClause(column, field, null)
		if err != nil {
			return err
		}
		q.Push(" AND " + clause)
	}
	if isMe, ok := rest["isMe"]; ok {
		delete(rest, "isMe")
		want, ok := boolEq(isMe)
		if !ok {
			return badFilter(field, `isMe takes {"eq": true|false}`)
		}
		if want {
			q.Push(fmt.Sprintf(" AND %s IS NOT DISTINCT FROM ", column)).Bind(q.me)
		} else {
			q.Push(fmt.Sprintf(" AND %s IS DISTINCT FROM ", column)).Bind(q.me)
		}
	}
	if len(rest) > 0 {
		if id, ok := rest["id"].(M); ok {
			replaced := M{}
			for k, v := range id {
				replaced[k] = replaceMe(v, q.me)
			}
			rest["id"] = replaced
		}
		q.Push(fmt.Sprintf(" AND EXISTS (SELECT 1 FROM users uf WHERE uf.id = %s AND ", column))
		if err := q.fields(field, rest, []known{{"id", kindID, "uf.id"}, {"email", kindText, "uf.email"},
			{"name", kindText, "uf.name"}, {"displayName", kindText, "uf.display_name"}}); err != nil {
			return err
		}
		q.Push(")")
	}
	q.Push(")")
	return nil
}

// replaceMe makes "me" in an id comparison the caller.
func replaceMe(value any, user uuid.UUID) any {
	switch v := value.(type) {
	case string:
		if strings.EqualFold(v, "me") {
			return user.String()
		}
	case []any:
		items := make([]any, len(v))
		for i, item := range v {
			items[i] = replaceMe(item, user)
		}
		return items
	}
	return value
}

type kind int

const (
	kindID kind = iota
	kindText
	kindNumber
)

type known struct {
	name   string
	kind   kind
	column string
}

// fields requires each named field with its comparator; unknown fields are an error.
func (q *compiler) fields(parent string, object M, knowns []known) error {
	q.Push("(TRUE")
	for _, name := range sortedKeys(object) {
		index := slices.IndexFunc(knowns, func(k known) bool { return k.name == name })
		if index < 0 {
			names := make([]string, len(knowns))
			for i, k := range knowns {
				names[i] = k.name
			}
			return badFilter(parent, fmt.Sprintf("unknown field %q; use %s", name, strings.Join(names, ", ")))
		}
		q.Push(" AND ")
		field := parent + "." + name
		var err error
		switch k := knowns[index]; k.kind {
		case kindID:
			err = q.ids(k.column, field, object[name])
		case kindText:
			err = q.texts(k.column, field, object[name])
		case kindNumber:
			err = q.numbers(k.column, field, object[name])
		}
		if err != nil {
			return err
		}
	}
	q.Push(")")
	return nil
}

type comparison struct {
	op    string
	value any
}

func comparators(field string, value any) ([]comparison, error) {
	switch v := value.(type) {
	case M:
		var list []comparison
		for _, k := range sortedKeys(v) {
			list = append(list, comparison{k, v[k]})
		}
		return list, nil
	case []any:
		return []comparison{{"in", value}}, nil
	case nil:
		return nil, badFilter(field, `use {"null": true} for unset values`)
	}
	return []comparison{{"eq", value}}, nil
}

func listOf(field string, value any) ([]any, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, badFilter(field, "in / nin take a list")
	}
	return list, nil
}

func (q *compiler) ids(column, field string, value any) error {
	parse := func(v any) (uuid.UUID, error) {
		s, _ := v.(string)
		id, ok := utils.AsUUID(s)
		if !ok {
			return uuid.Nil, badFilter(field, "ids must be UUIDs")
		}
		return id, nil
	}
	list, err := comparators(field, value)
	if err != nil {
		return err
	}
	q.Push("(TRUE")
	for _, cmp := range list {
		q.Push(" AND ")
		switch cmp.op {
		case "eq", "neq":
			id, err := parse(cmp.value)
			if err != nil {
				return err
			}
			if cmp.op == "eq" {
				q.Push(column + " = ").Bind(id)
			} else {
				q.Push(column + " IS DISTINCT FROM ").Bind(id)
			}
		case "in", "nin":
			items, err := listOf(field, cmp.value)
			if err != nil {
				return err
			}
			ids := make([]uuid.UUID, len(items))
			for n, item := range items {
				if ids[n], err = parse(item); err != nil {
					return err
				}
			}
			if cmp.op == "nin" {
				q.Push("NOT ")
			}
			q.Push(column + " = ANY(").Bind(ids).Push(")")
		case "null":
			clause, err := nullClause(column, field, cmp.value)
			if err != nil {
				return err
			}
			q.Push(clause)
		default:
			return badFilter(field, "ids compare with eq, neq, in, nin or null, not "+cmp.op)
		}
	}
	q.Push(")")
	return nil
}

func (q *compiler) texts(column, field string, value any) error {
	str := func(v any) (string, error) {
		s, ok := v.(string)
		if !ok {
			return "", badFilter(field, "expected text")
		}
		return s, nil
	}
	list, err := comparators(field, value)
	if err != nil {
		return err
	}
	q.Push("(TRUE")
	for _, cmp := range list {
		q.Push(" AND ")
		like := func(negate bool, pattern func(string) string) error {
			s, err := str(cmp.value)
			if err != nil {
				return err
			}
			if negate {
				q.Push(column + " NOT ILIKE ")
			} else {
				q.Push(column + " ILIKE ")
			}
			q.Bind(pattern(utils.EscapeLike(s)))
			return nil
		}
		contains := func(s string) string { return "%" + s + "%" }
		prefix := func(s string) string { return s + "%" }
		suffix := func(s string) string { return "%" + s }
		var err error
		switch cmp.op {
		case "eq", "eqIgnoreCase", "neq", "neqIgnoreCase":
			s, err := str(cmp.value)
			if err != nil {
				return err
			}
			if strings.HasPrefix(cmp.op, "eq") {
				q.Push("lower(" + column + ") = lower(").Bind(s).Push(")")
			} else {
				q.Push("lower(" + column + ") IS DISTINCT FROM lower(").Bind(s).Push(")")
			}
		case "in", "nin":
			items, err := listOf(field, cmp.value)
			if err != nil {
				return err
			}
			values := make([]string, len(items))
			for n, item := range items {
				s, err := str(item)
				if err != nil {
					return err
				}
				values[n] = strings.ToLower(s)
			}
			if cmp.op == "nin" {
				q.Push("NOT ")
			}
			q.Push("coalesce(lower(" + column + "), '') = ANY(").Bind(values).Push(")")
		case "contains", "containsIgnoreCase":
			err = like(false, contains)
		case "notContains", "notContainsIgnoreCase":
			err = like(true, contains)
		case "startsWith", "startsWithIgnoreCase":
			err = like(false, prefix)
		case "notStartsWith":
			err = like(true, prefix)
		case "endsWith":
			err = like(false, suffix)
		case "notEndsWith":
			err = like(true, suffix)
		case "null":
			null, ok := cmp.value.(bool)
			if !ok {
				return badFilter(field, "null takes true or false")
			}
			if null {
				q.Push("coalesce(" + column + ", '') = ''")
			} else {
				q.Push("coalesce(" + column + ", '') <> ''")
			}
		default:
			return badFilter(field, "unknown comparator "+cmp.op)
		}
		if err != nil {
			return err
		}
	}
	q.Push(")")
	return nil
}

var sqlComparisons = map[string]string{"eq": "=", "neq": "IS DISTINCT FROM", "lt": "<", "lte": "<=", "gt": ">", "gte": ">="}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

func (q *compiler) numbers(column, field string, value any) error {
	num := func(v any) (float64, error) {
		n, ok := toFloat(v)
		if !ok {
			return 0, badFilter(field, "expected a number")
		}
		return n, nil
	}
	list, err := comparators(field, value)
	if err != nil {
		return err
	}
	q.Push("(TRUE")
	for _, cmp := range list {
		q.Push(" AND ")
		switch cmp.op {
		case "in", "nin":
			items, err := listOf(field, cmp.value)
			if err != nil {
				return err
			}
			values := make([]float64, len(items))
			for n, item := range items {
				if values[n], err = num(item); err != nil {
					return err
				}
			}
			if cmp.op == "nin" {
				q.Push("NOT ")
			}
			q.Push(column + "::float8 = ANY(").Bind(values).Push(")")
		case "null":
			clause, err := nullClause(column, field, cmp.value)
			if err != nil {
				return err
			}
			q.Push(clause)
		default:
			sqlOp, ok := sqlComparisons[cmp.op]
			if !ok {
				return badFilter(field, "unknown comparator "+cmp.op)
			}
			n, err := num(cmp.value)
			if err != nil {
				return err
			}
			q.Push(column + "::float8 " + sqlOp + " ").Bind(n)
		}
	}
	q.Push(")")
	return nil
}

func (q *compiler) dates(column, field string, value any, isDate bool) error {
	at := func(v any) (time.Time, error) {
		s, ok := v.(string)
		if !ok {
			return time.Time{}, badFilter(field, "expected an ISO date or duration")
		}
		return utils.ParseDatetime(s)
	}
	cast := ""
	if isDate {
		cast = "::timestamptz"
	}
	list, err := comparators(field, value)
	if err != nil {
		return err
	}
	q.Push("(TRUE")
	for _, cmp := range list {
		q.Push(" AND ")
		switch cmp.op {
		case "eq":
			t, err := at(cmp.value)
			if err != nil {
				return err
			}
			if isDate {
				q.Push(column + " = ").Bind(t.Format(time.DateOnly)).Push("::date")
			} else {
				q.Push("date_trunc('day', " + column + ") = date_trunc('day', ").Bind(t).Push("::timestamptz)")
			}
		case "null":
			clause, err := nullClause(column, field, cmp.value)
			if err != nil {
				return err
			}
			q.Push(clause)
		default:
			sqlOp, ok := sqlComparisons[cmp.op]
			if !ok {
				return badFilter(field, "dates compare with eq, neq, lt, lte, gt, gte or null, not "+cmp.op)
			}
			t, err := at(cmp.value)
			if err != nil {
				return err
			}
			q.Push(column + cast + " " + sqlOp + " ").Bind(t)
		}
	}
	q.Push(")")
	return nil
}
