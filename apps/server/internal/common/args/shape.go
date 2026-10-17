package args

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"ricrac/internal/common/errdefs"
)

// CheckShape refuses the unknown and missing fields of the objects in value, as t describes
// them, naming the fields it expected.
func CheckShape(value any, t reflect.Type) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Implements(nullableType) {
		return CheckShape(value, reflect.Zero(t).Interface().(nullable).innerType())
	}
	switch t.Kind() {
	case reflect.Slice:
		items, _ := value.([]any)
		for _, item := range items {
			if err := CheckShape(item, t.Elem()); err != nil {
				return err
			}
		}
	case reflect.Struct:
		object, ok := value.(M)
		if !ok {
			return nil
		}
		fields := map[string]reflect.Type{}
		var names []string
		structFields(t, func(name string, field reflect.StructField, _ bool) {
			fields[name] = field.Type
			names = append(names, "`"+name+"`")
		})
		for _, key := range slices.Sorted(maps.Keys(object)) {
			fieldType, known := fields[key]
			if !known {
				return fmt.Errorf("unknown field `%s`, expected one of %s", key, strings.Join(names, ", "))
			}
			if err := CheckShape(object[key], fieldType); err != nil {
				return err
			}
		}
		for _, name := range RequiredFields(t) {
			if _, ok := object[name]; !ok {
				return fmt.Errorf("missing field `%s`", name)
			}
		}
	}
	return nil
}

// CheckDecodes says why value doesn't decode into a t, as argument errors say it.
func CheckDecodes(value any, t reflect.Type) error {
	decoder := json.NewDecoder(bytes.NewReader(errdefs.Marshal(value)))
	if err := decoder.Decode(reflect.New(t).Interface()); err != nil {
		return fmt.Errorf("%s", describeDecodeError(err))
	}
	return nil
}
