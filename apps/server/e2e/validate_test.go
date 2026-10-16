package e2e

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// validate checks a JSON value against the subset of JSON Schema the operations' results use.
func validate(schema M, value any, path string) error {
	if len(schema) == 0 {
		return nil
	}
	if options, ok := schema["anyOf"].([]any); ok {
		return oneMatches(options, value, path)
	}
	if options, ok := schema["oneOf"].([]any); ok {
		return oneMatches(options, value, path)
	}
	if allowed, ok := schema["enum"].([]any); ok && !slices.ContainsFunc(allowed, func(a any) bool { return a == value }) {
		return fmt.Errorf("%s: %v isn't one of %v", path, value, allowed)
	}
	types := typesOf(schema["type"])
	if len(types) > 0 && !slices.Contains(types, kindOf(value)) && !(kindOf(value) == "integer" && slices.Contains(types, "number")) {
		return fmt.Errorf("%s: expected %s, got %s (%v)", path, strings.Join(types, " or "), kindOf(value), value)
	}
	switch v := value.(type) {
	case map[string]any:
		properties, _ := schema["properties"].(M)
		for _, name := range stringsOf(schema["required"]) {
			if _, ok := v[name]; !ok {
				return fmt.Errorf("%s: missing %q", path, name)
			}
		}
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			property, known := properties[key].(M)
			if !known {
				if schema["additionalProperties"] == false {
					return fmt.Errorf("%s: unexpected field %q", path, key)
				}
				if extra, ok := schema["additionalProperties"].(M); ok {
					property = extra
				}
			}
			if err := validate(property, v[key], path+"."+key); err != nil {
				return err
			}
		}
	case []any:
		items, _ := schema["items"].(M)
		for i, item := range v {
			if err := validate(items, item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func oneMatches(options []any, value any, path string) error {
	var errs []string
	for _, option := range options {
		schema, _ := option.(M)
		err := validate(schema, value, path)
		if err == nil {
			return nil
		}
		errs = append(errs, err.Error())
	}
	return fmt.Errorf("%s: matches none of: %s", path, strings.Join(errs, "; "))
}

func typesOf(value any) []string {
	switch t := value.(type) {
	case string:
		return []string{t}
	case []any:
		return stringsOf(t)
	}
	return nil
}

func stringsOf(value any) []string {
	list, _ := value.([]any)
	var out []string
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func kindOf(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case float64:
		if v == float64(int64(v)) {
			return "integer"
		}
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return fmt.Sprintf("%T", value)
}
