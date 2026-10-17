package oapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"

	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
)

// Authenticate is the caller of a /v1 request and the workspace named in x-ricrac-workspace;
// it writes the error answer and says false when there's none.
type Authenticate func(w http.ResponseWriter, r *http.Request) (*auth.Caller, string, bool)

// Mount puts every operation on the API: POST /v1/ops/<name> and its REST routes.
func (reg *Registry) Mount(authenticate Authenticate) {
	for _, op := range reg.ops {
		reg.handle(op, http.MethodPost, "/v1/ops/"+op.Name, "call_"+op.Name, authenticate)
		for n, route := range op.Routes {
			id := op.Name
			if n > 0 {
				id = op.Name + "_" + strings.ToLower(route.Method)
			}
			reg.handle(op, route.Method, "/v1"+route.Path, id, authenticate)
		}
	}
}

func (reg *Registry) handle(op *Op, method, path, id string, authenticate Authenticate) {
	params := pathParams(path)
	operation := &huma.Operation{
		OperationID: id,
		Method:      method,
		Path:        path,
		Summary:     id,
		Description: op.Description,
		Responses:   map[string]*huma.Response{"200": {Description: "The result"}},
	}
	reg.API.OpenAPI().AddOperation(operation)
	reg.API.Adapter().Handle(operation, func(ctx huma.Context) {
		r, w := humachi.Unwrap(ctx)
		values := map[string]string{}
		for _, name := range params {
			values[name] = ctx.Param(name)
		}
		query := ""
		if path != "/v1/ops/"+op.Name {
			query = r.URL.RawQuery
		} else {
			values = nil
		}
		reg.serve(w, r, op, values, query, authenticate)
	})
}

func (reg *Registry) serve(w http.ResponseWriter, r *http.Request, op *Op, path map[string]string, query string, authenticate Authenticate) {
	caller, workspace, ok := authenticate(w, r)
	if !ok {
		return
	}
	body, err := ReadBody(r)
	if err != nil {
		errdefs.Write(w, err)
		return
	}
	raw, err := arguments(op, path, query, body)
	if err != nil {
		errdefs.Write(w, err)
		return
	}
	value, err := reg.CallAs(r.Context(), op, caller, raw, workspace)
	if err != nil {
		errdefs.Write(w, err)
		return
	}
	errdefs.WriteJSON(w, http.StatusOK, errdefs.Sorted(value))
}

// ReadBody reads a request body within its limit.
func ReadBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return nil, errdefs.Errorf(http.StatusRequestEntityTooLarge, "The body can have at most %d bytes", tooLarge.Limit)
	}
	if err != nil {
		return nil, errdefs.BadRequest("Couldn't read the body")
	}
	return body, nil
}

// arguments merges the JSON body, then the query string, then the path parameters into one
// object.
func arguments(op *Op, path map[string]string, query string, body []byte) (M, error) {
	args := M{}
	if len(bytes.TrimSpace(body)) > 0 {
		var value any
		if err := json.Unmarshal(body, &value); err != nil {
			return nil, errdefs.BadRequest("The body isn't valid JSON: %s", err)
		}
		object, ok := value.(M)
		if !ok {
			return nil, errdefs.BadRequest("The body must be a JSON object")
		}
		args = object
	}
	properties, _ := op.Schema["properties"].(M)
	lists := map[string][]string{}
	var order []string
	for pair := range strings.SplitSeq(query, "&") {
		if pair == "" {
			continue
		}
		key, value, _ := strings.Cut(pair, "=")
		key = strings.TrimSuffix(decodeQuery(key), "[]")
		if _, seen := lists[key]; !seen {
			order = append(order, key)
		}
		lists[key] = append(lists[key], decodeQuery(value))
	}
	for _, key := range order {
		schema, _ := properties[key].(M)
		args[key] = coerce(lists[key], schema)
	}
	for key, value := range path {
		args[key] = value
	}
	return args, nil
}

func decodeQuery(s string) string {
	s = strings.ReplaceAll(s, "+", " ")
	if decoded, err := url.PathUnescape(s); err == nil {
		return decoded
	}
	return s
}

// schemaTypes is the JSON types a schema allows, looking into anyOf and oneOf.
func schemaTypes(schema M) []string {
	var found []string
	switch kind := schema["type"].(type) {
	case string:
		found = append(found, kind)
	case []any:
		for _, k := range kind {
			if s, ok := k.(string); ok {
				found = append(found, s)
			}
		}
	}
	for _, key := range []string{"anyOf", "oneOf"} {
		options, _ := schema[key].([]any)
		for _, option := range options {
			if o, ok := option.(M); ok {
				found = append(found, schemaTypes(o)...)
			}
		}
	}
	return found
}

// coerce turns query values, which are text, into what the argument's schema expects.
func coerce(values []string, schema M) any {
	kinds := schemaTypes(schema)
	has := func(kind string) bool { return slices.Contains(kinds, kind) }
	if has("array") {
		items, _ := schema["items"].(M)
		if len(values) == 1 {
			var list []any
			if json.Unmarshal([]byte(values[0]), &list) == nil && list != nil {
				return list
			}
		}
		parts := []any{}
		for _, value := range values {
			for part := range strings.SplitSeq(value, ",") {
				if part = strings.TrimSpace(part); part != "" {
					parts = append(parts, coerce([]string{part}, items))
				}
			}
		}
		return parts
	}
	value := ""
	if len(values) > 0 {
		value = values[len(values)-1]
	}
	if value == "null" && has("null") {
		return nil
	}
	if has("boolean") && (value == "true" || value == "false") {
		return value == "true"
	}
	if (has("integer") || has("number")) && !has("string") {
		var number float64
		if json.Unmarshal([]byte(value), &number) == nil {
			return number
		}
	}
	if has("object") {
		var object M
		if json.Unmarshal([]byte(value), &object) == nil && object != nil {
			return object
		}
	}
	if len(kinds) == 0 {
		var parsed any
		if json.Unmarshal([]byte(value), &parsed) == nil {
			return parsed
		}
	}
	return value
}

// PathParams is the names of a route's path parameters.
func PathParams(path string) []string {
	return pathParams(path)
}

func pathParams(path string) []string {
	var names []string
	for segment := range strings.SplitSeq(path, "/") {
		if name, ok := strings.CutPrefix(segment, "{"); ok && strings.HasSuffix(name, "}") {
			names = append(names, strings.TrimSuffix(name, "}"))
		}
	}
	return names
}

// SchemaTypes is the JSON types a schema allows, looking into anyOf and oneOf.
func SchemaTypes(schema M) []string {
	return schemaTypes(schema)
}
