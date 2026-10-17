package args

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"ricrac/internal/common/errdefs"
)

// Parse decodes arguments into A, refusing unknown fields and asking for the required ones.
func Parse[A any](raw M) (A, error) {
	var args A
	if raw == nil {
		raw = M{}
	}
	for _, name := range RequiredFields(reflect.TypeFor[A]()) {
		if _, ok := raw[name]; !ok {
			return args, errdefs.BadRequest("Invalid arguments: missing field `%s`", name)
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(errdefs.Marshal(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return args, errdefs.BadRequest("Invalid arguments: %s", describeDecodeError(err))
	}
	return args, nil
}

func describeDecodeError(err error) string {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return fmt.Sprintf("`%s` should be %s, not %s", typeErr.Field, jsonKind(typeErr.Type), typeErr.Value)
	}
	message := strings.TrimPrefix(err.Error(), "json: ")
	if field, ok := strings.CutPrefix(message, "unknown field "); ok {
		return fmt.Sprintf("unknown field `%s`", strings.Trim(field, `"`))
	}
	return message
}

func jsonKind(t reflect.Type) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String:
		return "a string"
	case reflect.Bool:
		return "true or false"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint32, reflect.Uint64:
		return "an integer"
	case reflect.Float32, reflect.Float64:
		return "a number"
	case reflect.Slice:
		return "a list"
	case reflect.Map, reflect.Struct:
		return "an object"
	}
	return t.String()
}
