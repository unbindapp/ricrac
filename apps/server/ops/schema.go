package ops

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
)

// Nullable is an argument that may be left out (stays as it is), null (clears it) or a value.
type Nullable[T any] struct {
	Set   bool
	Valid bool
	Value T
}

func (n *Nullable[T]) UnmarshalJSON(data []byte) error {
	n.Set = true
	if string(data) == "null" {
		n.Valid = false
		return nil
	}
	n.Valid = true
	return json.Unmarshal(data, &n.Value)
}

func (n Nullable[T]) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.Value)
}

// Ptr is the value as a pointer, nil when null or left out.
func (n Nullable[T]) Ptr() *T {
	if !n.Valid {
		return nil
	}
	return &n.Value
}

func (Nullable[T]) innerType() reflect.Type { return reflect.TypeFor[T]() }

type nullable interface{ innerType() reflect.Type }

// schemaer is a type that describes its own JSON Schema.
type schemaer interface{ JSONSchema() M }

var (
	nullableType = reflect.TypeFor[nullable]()
	schemaerType = reflect.TypeFor[schemaer]()
	rawType      = reflect.TypeFor[json.RawMessage]()
)

var schemaCache sync.Map

// SchemaOf is the JSON Schema of an argument type: inlined, so clients that don't follow $refs
// still see every field. Fields take their description from the `doc` tag and allowed values
// from `enum` (comma-separated). A field is required unless it is a pointer, Nullable, list,
// map, interface or tagged omitempty.
func SchemaOf[A any]() M {
	t := reflect.TypeFor[A]()
	if cached, ok := schemaCache.Load(t); ok {
		return cloneJSON(cached.(M))
	}
	schema := schemaFor(t)
	if _, ok := schema["properties"]; !ok {
		schema["properties"] = M{}
	}
	schemaCache.Store(t, schema)
	return cloneJSON(schema)
}

func schemaFor(t reflect.Type) M {
	if t.Implements(schemaerType) {
		return reflect.Zero(t).Interface().(schemaer).JSONSchema()
	}
	if t.Implements(nullableType) {
		inner := schemaFor(reflect.Zero(t).Interface().(nullable).innerType())
		return withNull(inner)
	}
	if t == rawType {
		return M{}
	}
	switch t.Kind() {
	case reflect.Pointer:
		return withNull(schemaFor(t.Elem()))
	case reflect.String:
		return M{"type": "string"}
	case reflect.Bool:
		return M{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint32, reflect.Uint64:
		return M{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return M{"type": "number"}
	case reflect.Slice, reflect.Array:
		return M{"type": "array", "items": schemaFor(t.Elem())}
	case reflect.Map:
		if t.Elem().Kind() == reflect.Interface {
			return M{"type": "object"}
		}
		return M{"type": "object", "additionalProperties": schemaFor(t.Elem())}
	case reflect.Interface:
		return M{}
	case reflect.Struct:
		properties := M{}
		var required []any
		structFields(t, func(name string, field reflect.StructField, isRequired bool) {
			property := schemaFor(field.Type)
			if doc := field.Tag.Get("doc"); doc != "" {
				property["description"] = doc
			}
			if values := field.Tag.Get("enum"); values != "" {
				var list []any
				for _, v := range strings.Split(values, ",") {
					list = append(list, v)
				}
				if _, isNullable := property["type"].([]any); isNullable {
					list = append(list, nil)
				}
				property["enum"] = list
			}
			properties[name] = property
			if isRequired {
				required = append(required, name)
			}
		})
		schema := M{"type": "object", "properties": properties, "additionalProperties": false}
		if len(required) > 0 {
			schema["required"] = required
		}
		return schema
	}
	return M{}
}

func withNull(schema M) M {
	switch kind := schema["type"].(type) {
	case string:
		schema["type"] = []any{kind, "null"}
	case nil:
		if len(schema) > 0 {
			return M{"anyOf": []any{schema, M{"type": "null"}}}
		}
	}
	return schema
}

// structFields visits the JSON fields of a struct, flattening embedded structs as encoding/json
// does.
func structFields(t reflect.Type, visit func(name string, field reflect.StructField, required bool)) {
	for i := range t.NumField() {
		field := t.Field(i)
		tag := field.Tag.Get("json")
		name, options, _ := strings.Cut(tag, ",")
		if field.Anonymous && name == "" && field.Type.Kind() == reflect.Struct {
			structFields(field.Type, visit)
			continue
		}
		if tag == "-" || !field.IsExported() {
			continue
		}
		if name == "" {
			name = field.Name
		}
		visit(name, field, isRequired(field.Type, options))
	}
}

func isRequired(t reflect.Type, options string) bool {
	if strings.Contains(options, "omitempty") || strings.Contains(options, "omitzero") {
		return false
	}
	if t.Implements(nullableType) {
		return false
	}
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Interface:
		return false
	}
	return t != rawType
}

func requiredFields(t reflect.Type) []string {
	if t.Kind() != reflect.Struct {
		return nil
	}
	var names []string
	structFields(t, func(name string, _ reflect.StructField, required bool) {
		if required {
			names = append(names, name)
		}
	})
	return names
}
