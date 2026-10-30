package github

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// IssueFieldValue is the value an issue has for one of its organization's fields.
type IssueFieldValue struct {
	FieldID  int64  `json:"issue_field_id"`
	DataType string `json:"data_type"`
	Value    any    `json:"value"`
	Option   *struct {
		Name string `json:"name"`
	} `json:"single_select_option"`
}

// Text is the value as text: a select's option name, a number or date as written.
func (v IssueFieldValue) Text() string {
	if v.Option != nil {
		return v.Option.Name
	}
	switch value := v.Value.(type) {
	case nil:
		return ""
	case string:
		return value
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	return fmt.Sprint(v.Value)
}

// FieldSlot is a Ricrac issue field (as its history names it) and the organization fields that
// hold it on GitHub: the first of names, in any case, of the data type.
type FieldSlot struct {
	Key      string
	Names    []string
	DataType string
}

var FieldSlots = []FieldSlot{
	{"priority", []string{"Priority"}, "single_select"},
	{"estimate", []string{"Estimate"}, "number"},
	{"dueDate", []string{"Due date", "Target date"}, "date"},
	{"cycle", []string{"Cycle"}, "text"},
}

// FieldFor is the organization's field holding the slot, nil when it has none.
func (slot FieldSlot) FieldFor(fields []IssueField) *IssueField {
	for _, name := range slot.Names {
		for i, field := range fields {
			if field.DataType == slot.DataType && strings.EqualFold(strings.TrimSpace(field.Name), name) {
				return &fields[i]
			}
		}
	}
	return nil
}

var priorityNames = []string{"", "Urgent", "High", "Medium", "Low"}

// PriorityName is a Ricrac priority (1 urgent to 4 low) as GitHub's Priority option, "" for none.
func PriorityName(priority int) string {
	if priority < 1 || priority >= len(priorityNames) {
		return ""
	}
	return priorityNames[priority]
}

// PriorityOf is the Ricrac priority of a Priority option, false for one Ricrac has none like.
func PriorityOf(name string) (int, bool) {
	i := slices.IndexFunc(priorityNames, func(known string) bool { return known != "" && strings.EqualFold(known, strings.TrimSpace(name)) })
	return i, i > 0
}

// CycleText is how a cycle reads in the Cycle field: its name, else Cycle and its number.
func CycleText(number int, name *string) string {
	if name != nil && strings.TrimSpace(*name) != "" {
		return *name
	}
	return "Cycle " + strconv.Itoa(number)
}

// CycleReference is a Cycle field's text as Ricrac resolves a cycle: "Cycle 12" is number 12.
func CycleReference(text string) string {
	text = strings.TrimSpace(text)
	if rest, ok := strings.CutPrefix(strings.ToLower(text), "cycle "); ok {
		if _, err := strconv.Atoi(strings.TrimSpace(rest)); err == nil {
			return strings.TrimSpace(rest)
		}
	}
	return text
}

func IssueFieldValuesPath(fullName string, number int) string {
	return IssuePath(fullName, number) + "/issue-field-values"
}
