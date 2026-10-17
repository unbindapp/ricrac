package project_repo

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/internal/common/errdefs"
	"ricrac/internal/common/utils"
)

type M = map[string]any

// Condition is SQL on the projects table, each ? standing for the next argument.
type Condition struct {
	SQL  string
	Args []any
}

type conditionBuilder struct {
	sql  strings.Builder
	args []any
}

func (b *conditionBuilder) push(sql string) *conditionBuilder {
	b.sql.WriteString(sql)
	return b
}

func (b *conditionBuilder) bind(value any) *conditionBuilder {
	b.sql.WriteString("?")
	b.args = append(b.args, value)
	return b
}

// Filter is the condition of a project filter (the filter of a project view); me is the caller,
// for lead.isMe.
func Filter(filter any, me uuid.UUID) (Condition, error) {
	b := &conditionBuilder{}
	if err := b.filter(filter, me); err != nil {
		return Condition{}, err
	}
	return Condition{SQL: b.sql.String(), Args: b.args}, nil
}

type filterField struct{ name, column, kind string }

func sortedKeys(object M) []string {
	return slices.Sorted(maps.Keys(object))
}

func (b *conditionBuilder) filter(filter any, me uuid.UUID) error {
	object, ok := filter.(M)
	if !ok {
		return errdefs.BadRequest("A project filter must be an object")
	}
	b.push("(TRUE")
	for _, key := range sortedKeys(object) {
		value := object[key]
		b.push(" AND ")
		var err error
		switch key {
		case "and", "or":
			list, ok := value.([]any)
			if !ok {
				return errdefs.BadRequest("\"%s\" takes a list of filters", key)
			}
			joiner := " OR "
			if key == "and" {
				b.push("(TRUE")
				joiner = " AND "
			} else {
				b.push("(FALSE")
			}
			for _, item := range list {
				b.push(joiner)
				if err := b.filter(item, me); err != nil {
					return err
				}
			}
			b.push(")")
		case "priority":
			err = b.compare("projects.priority", "int", value)
		case "health":
			err = b.compare("projects.health", "text", value)
		case "name":
			err = b.compare("projects.name", "text", value)
		case "startDate":
			err = b.compare("projects.start_date", "date", value)
		case "targetDate":
			err = b.compare("projects.target_date", "date", value)
		case "status":
			b.push("EXISTS (SELECT 1 FROM project_statuses fs WHERE fs.id = projects.status_id")
			err = b.nested(key, value, []filterField{{"id", "fs.id", "uuid"}, {"name", "fs.name", "text"}, {"type", "fs.type", "text"}})
			b.push(")")
		case "team", "teams":
			b.push("EXISTS (SELECT 1 FROM project_teams ft JOIN teams t ON t.id = ft.team_id WHERE ft.project_id = projects.id")
			err = b.nested(key, value, []filterField{{"id", "t.id", "uuid"}, {"key", "t.key", "text"}, {"name", "t.name", "text"}})
			b.push(")")
		case "lead":
			err = b.lead(value, me)
		default:
			return errdefs.BadRequest("Project filters don't have \"%s\"; use status, lead, team, priority, health, name, startDate, "+
				"targetDate, and, or", key)
		}
		if err != nil {
			return err
		}
	}
	b.push(")")
	return nil
}

func (b *conditionBuilder) lead(value any, me uuid.UUID) error {
	fields, ok := value.(M)
	if !ok {
		return errdefs.BadRequest("lead takes {id, isMe, null}")
	}
	b.push("(TRUE")
	for _, field := range sortedKeys(fields) {
		condition := fields[field]
		b.push(" AND ")
		switch field {
		case "id":
			if err := b.compare("projects.lead_id", "uuid", condition); err != nil {
				return err
			}
		case "null":
			if boolOr(condition, true) {
				b.push("projects.lead_id IS NULL")
			} else {
				b.push("projects.lead_id IS NOT NULL")
			}
		case "isMe":
			isMe, ok := condition.(bool)
			if !ok {
				object, _ := condition.(M)
				isMe = boolOr(object["eq"], true)
			}
			if isMe {
				b.push("projects.lead_id = ").bind(me)
			} else {
				b.push("projects.lead_id IS DISTINCT FROM ").bind(me)
			}
		default:
			return errdefs.BadRequest("lead filters take id, isMe or null, not \"%s\"", field)
		}
	}
	b.push(")")
	return nil
}

func boolOr(value any, fallback bool) bool {
	if b, ok := value.(bool); ok {
		return b
	}
	return fallback
}

// nested pushes " AND <column> <comparators>" for each field of a nested filter like
// status: {type: {eq: "started"}}.
func (b *conditionBuilder) nested(key string, value any, fields []filterField) error {
	object, ok := value.(M)
	if !ok {
		return errdefs.BadRequest("%s takes an object", key)
	}
	for _, name := range sortedKeys(object) {
		index := slices.IndexFunc(fields, func(f filterField) bool { return f.name == name })
		if index < 0 {
			names := make([]string, len(fields))
			for i, f := range fields {
				names[i] = f.name
			}
			return errdefs.BadRequest("%s filters take %s, not \"%s\"", key, strings.Join(names, ", "), name)
		}
		b.push(" AND ")
		if err := b.compare(fields[index].column, fields[index].kind, object[name]); err != nil {
			return err
		}
	}
	return nil
}

func filterScalar(value any, kind string) (string, error) {
	var text string
	switch v := value.(type) {
	case string:
		text = v
	case float64:
		text = strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		text = strconv.FormatBool(v)
	default:
		return "", errdefs.BadRequest("%s isn't a value to compare with", errdefs.Marshal(value))
	}
	switch kind {
	case "date":
		at, err := utils.ParseDatetime(text)
		if err != nil {
			return "", err
		}
		return at.Format(time.DateOnly), nil
	case "text":
		return strings.ToLower(text), nil
	}
	return text, nil
}

// compare pushes "<column> <comparators>"; a plain value means eq. Text compares without case.
func (b *conditionBuilder) compare(column, kind string, condition any) error {
	if kind == "text" {
		column = "lower(" + column + ")"
	}
	comparators, ok := condition.(M)
	if !ok {
		text, err := filterScalar(condition, kind)
		if err != nil {
			return err
		}
		b.push(column + " = ").bind(text).push("::" + kind)
		return nil
	}
	signs := map[string]string{"eq": "=", "neq": "IS DISTINCT FROM", "lt": "<", "lte": "<=", "gt": ">", "gte": ">="}
	b.push("(TRUE")
	for _, comparator := range sortedKeys(comparators) {
		value := comparators[comparator]
		b.push(" AND ")
		switch comparator {
		case "in", "nin":
			items, ok := value.([]any)
			if !ok {
				return errdefs.BadRequest("%s takes a list", comparator)
			}
			list := make([]string, len(items))
			for i, item := range items {
				text, err := filterScalar(item, kind)
				if err != nil {
					return err
				}
				list[i] = text
			}
			if comparator == "in" {
				b.push(column + " = ANY(")
			} else {
				b.push("NOT (" + column + " = ANY(")
			}
			b.bind(list).push("::text[]::" + kind + "[])")
			if comparator == "nin" {
				b.push(")")
			}
		case "null":
			if boolOr(value, true) {
				b.push(column + " IS NULL")
			} else {
				b.push(column + " IS NOT NULL")
			}
		default:
			sign, ok := signs[comparator]
			if !ok {
				return errdefs.BadRequest("\"%s\" isn't a comparator; use eq, neq, in, nin, lt, lte, gt, gte or null", comparator)
			}
			text, err := filterScalar(value, kind)
			if err != nil {
				return err
			}
			b.push(column + " " + sign + " ").bind(text).push("::" + kind)
		}
	}
	b.push(")")
	return nil
}
