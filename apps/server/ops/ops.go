// Package ops holds every operation of the tracker, defined once. The REST API (/v1), the MCP
// tools and the OpenAPI document are all generated from this list.
package ops

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"

	"ricrac/app"
)

// M is a JSON object.
type M = map[string]any

type Route struct {
	Method string
	Path   string
}

type Op struct {
	// The MCP tool name, e.g. save_issue.
	Name        string
	Description string
	Access      app.Access
	// REST routes under /v1, e.g. PATCH /issues/{id}. Path parameters are passed in the
	// arguments under the same name.
	Routes []Route
	// JSON Schema of the arguments.
	Schema M
	// JSON Schema of the result (see returns).
	Result      M
	inWorkspace func(*app.Ctx, M) (any, error)
	forAccount  func(*app.Caller, M) (any, error)
}

const workspaceArg = "The workspace to act in, by URL key (see list_workspaces) or id."

func routes(specs []string) []Route {
	parsed := make([]Route, len(specs))
	for i, spec := range specs {
		method, path, _ := strings.Cut(spec, " ")
		parsed[i] = Route{method, path}
	}
	return parsed
}

// op defines an operation that runs in one workspace from a function taking the context and its
// arguments. Routes are written "METHOD /path".
func op[A any](name string, access app.Access, specs []string, description string, handler func(*app.Ctx, A) (any, error)) *Op {
	return &Op{
		Name: name, Description: description, Access: access, Routes: routes(specs), Schema: SchemaOf[A](),
		inWorkspace: func(c *app.Ctx, raw M) (any, error) {
			args, err := Parse[A](raw)
			if err != nil {
				return nil, err
			}
			return handler(c, args)
		},
	}
}

// accountOp defines an operation that isn't about one workspace, like listing the ones the
// caller reaches.
func accountOp[A any](name string, access app.Access, specs []string, description string, handler func(*app.Caller, A) (any, error)) *Op {
	return &Op{
		Name: name, Description: description, Access: access, Routes: routes(specs), Schema: SchemaOf[A](),
		forAccount: func(c *app.Caller, raw M) (any, error) {
			args, err := Parse[A](raw)
			if err != nil {
				return nil, err
			}
			return handler(c, args)
		},
	}
}

// returns declares the shape of the operation's result, which the web client is typed from and
// the end-to-end tests check every answer against.
func (o *Op) returns(schema M) *Op {
	o.Result = schema
	return o
}

func (o *Op) Call(c *app.Ctx, args M) (any, error) {
	if err := c.Require(o.Access); err != nil {
		return nil, err
	}
	if o.forAccount != nil {
		return o.forAccount(c.Caller(), args)
	}
	return o.inWorkspace(c, args)
}

// CallAs runs the operation for a caller of the API or MCP, in the workspace named by the
// workspace argument, or else fallback (see Caller.Ctx).
func (o *Op) CallAs(caller *app.Caller, args M, fallback string) (any, error) {
	if args == nil {
		args = M{}
	}
	named, err := takeWorkspace(args)
	if err != nil {
		return nil, err
	}
	if o.forAccount != nil {
		if err := caller.Require(o.Access); err != nil {
			return nil, err
		}
		return o.forAccount(caller, args)
	}
	if named == "" {
		named = fallback
	}
	c, err := caller.Ctx(named, args)
	if err != nil {
		return nil, err
	}
	return o.Call(c, args)
}

// ReadOnly says whether the operation changes nothing.
func (o *Op) ReadOnly() bool {
	return o.Access == app.Read
}

func (o *Op) RunsInWorkspace() bool {
	return o.inWorkspace != nil
}

// SchemaWithWorkspace is the schema with the workspace argument, required for tokens that reach
// several workspaces.
func (o *Op) SchemaWithWorkspace(required bool) M {
	schema := cloneJSON(o.Schema)
	if !o.RunsInWorkspace() {
		return schema
	}
	description := workspaceArg + " Needed when the token reaches several workspaces."
	if required {
		description = workspaceArg + " Required: this token reaches several workspaces."
	}
	schema["properties"].(M)["workspace"] = M{"type": "string", "description": description}
	if required {
		list, _ := schema["required"].([]any)
		schema["required"] = append([]any{"workspace"}, list...)
	}
	return schema
}

func cloneJSON(value M) M {
	var copied M
	_ = json.Unmarshal(app.Marshal(value), &copied)
	return copied
}

// takeWorkspace removes the workspace argument, which every operation takes besides its own.
func takeWorkspace(args M) (string, error) {
	value, ok := args["workspace"]
	delete(args, "workspace")
	if !ok || value == nil {
		return "", nil
	}
	switch v := value.(type) {
	case string:
		return v, nil
	case json.Number:
		return v.String(), nil
	case float64:
		return fmt.Sprint(v), nil
	}
	return "", app.BadRequest("workspace is a URL key or id")
}

// Parse decodes arguments into A, refusing unknown fields and asking for the required ones.
func Parse[A any](raw M) (A, error) {
	var args A
	if raw == nil {
		raw = M{}
	}
	for _, name := range requiredFields(reflect.TypeFor[A]()) {
		if _, ok := raw[name]; !ok {
			return args, app.BadRequest("Invalid arguments: missing field `%s`", name)
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(app.Marshal(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return args, app.BadRequest("Invalid arguments: %s", describeDecodeError(err))
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

var (
	registry     []*Op
	registryOnce sync.Once
)

// All is every operation, in a stable order.
func All() []*Op {
	registryOnce.Do(func() {
		for _, group := range [][]*Op{
			workspaceOps(),
			teamOps(),
			stateOps(),
			labelOps(),
			issueOps(),
			importOps(),
			commentOps(),
			attachmentOps(),
			projectOps(),
			milestoneOps(),
			statusUpdateOps(),
			initiativeOps(),
			cycleOps(),
			documentOps(),
			viewOps(),
			favoriteOps(),
			templateOps(),
			notificationOps(),
			searchOps(),
			customerOps(),
			releaseOps(),
			webhookOps(),
			githubOps(),
		} {
			registry = append(registry, group...)
		}
	})
	return registry
}

func Find(name string) *Op {
	index := slices.IndexFunc(All(), func(o *Op) bool { return o.Name == name })
	if index < 0 {
		return nil
	}
	return registry[index]
}

// NoArgs is arguments with no fields.
type NoArgs struct{}

// IDArgs names one thing by its id (or another reference the operation accepts).
type IDArgs struct {
	ID string `json:"id"`
}
