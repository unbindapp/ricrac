// Package oapi registers the tracker's operations. Each is defined once and is reachable as
// POST /v1/ops/<name>, through its REST routes and as an MCP tool, with the same arguments,
// checks and answer.
package oapi

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"ricrac/internal/auth"
	"ricrac/internal/common/args"
	"ricrac/internal/common/errdefs"
)

type M = args.M

const workspaceArg = "The workspace to act in, by URL key (see list_workspaces) or id."

type Route struct {
	Method string
	Path   string
}

// Operation is what a handler group says about one of its operations. Routes are REST routes
// under /v1 written "METHOD /path"; their path parameters are passed in the arguments under the
// same name.
type Operation struct {
	Name        string
	Access      auth.Access
	Routes      []string
	Description string
}

// Op is a registered operation.
type Op struct {
	Name        string
	Description string
	Access      auth.Access
	Routes      []Route
	// JSON Schema of the arguments.
	Schema M
	// JSON Schema of the result.
	Result M
	// Runs for the caller, not in one workspace.
	Account bool

	inWorkspace func(ctx context.Context, actor *auth.Actor, raw M) (any, error)
	forAccount  func(ctx context.Context, caller *auth.Caller, raw M) (any, error)
}

// Picker puts a caller in the workspace a call acts in.
type Picker interface {
	Pick(ctx context.Context, caller *auth.Caller, named string, args map[string]any) (*auth.Actor, error)
}

// Registry is every operation, in the order they are listed.
type Registry struct {
	API    huma.API
	picker Picker
	ops    []*Op
	byName map[string]*Op
}

func NewRegistry(api huma.API, picker Picker) *Registry {
	return &Registry{API: api, picker: picker, byName: map[string]*Op{}}
}

func routes(specs []string) []Route {
	parsed := make([]Route, len(specs))
	for i, spec := range specs {
		method, path, _ := strings.Cut(spec, " ")
		parsed[i] = Route{method, path}
	}
	return parsed
}

func resultSchema[R any]() M {
	t := reflect.TypeFor[R]()
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() == reflect.Interface {
		return nil
	}
	return args.ResultOfType(t)
}

// Register adds an operation that runs in one workspace.
func Register[A, R any](reg *Registry, operation Operation, handler func(ctx context.Context, actor *auth.Actor, args *A) (R, error)) {
	reg.Add(&Op{
		Name: operation.Name, Description: operation.Description, Access: operation.Access, Routes: routes(operation.Routes),
		Schema: args.SchemaOf[A](), Result: resultSchema[R](),
		inWorkspace: func(ctx context.Context, actor *auth.Actor, raw M) (any, error) {
			parsed, err := args.Parse[A](raw)
			if err != nil {
				return nil, err
			}
			return handler(ctx, actor, &parsed)
		},
	})
}

// RegisterAccount adds an operation that isn't about one workspace, like listing the ones the
// caller reaches.
func RegisterAccount[A, R any](reg *Registry, operation Operation, handler func(ctx context.Context, caller *auth.Caller, args *A) (R, error)) {
	reg.Add(&Op{
		Name: operation.Name, Description: operation.Description, Access: operation.Access, Routes: routes(operation.Routes),
		Schema: args.SchemaOf[A](), Result: resultSchema[R](), Account: true,
		forAccount: func(ctx context.Context, caller *auth.Caller, raw M) (any, error) {
			parsed, err := args.Parse[A](raw)
			if err != nil {
				return nil, err
			}
			return handler(ctx, caller, &parsed)
		},
	})
}

// Add registers an operation built elsewhere.
func (reg *Registry) Add(op *Op) {
	if _, taken := reg.byName[op.Name]; taken {
		panic("operation registered twice: " + op.Name)
	}
	reg.ops = append(reg.ops, op)
	reg.byName[op.Name] = op
}

// NewOp is an operation run by functions of its own, for operations that don't come from a
// handler group.
func NewOp(op Op, inWorkspace func(ctx context.Context, actor *auth.Actor, raw M) (any, error), forAccount func(ctx context.Context, caller *auth.Caller, raw M) (any, error)) *Op {
	op.inWorkspace = inWorkspace
	op.forAccount = forAccount
	op.Account = forAccount != nil
	return &op
}

// Order sorts the operations as names lists them; the others go last.
func (reg *Registry) Order(names []string) {
	index := func(op *Op) int {
		if i := slices.Index(names, op.Name); i >= 0 {
			return i
		}
		return len(names)
	}
	slices.SortStableFunc(reg.ops, func(a, b *Op) int { return index(a) - index(b) })
}

// All is every operation, in a stable order.
func (reg *Registry) All() []*Op {
	return reg.ops
}

func (reg *Registry) Find(name string) *Op {
	return reg.byName[name]
}

// ReadOnly says whether the operation changes nothing.
func (o *Op) ReadOnly() bool {
	return o.Access == auth.Read
}

func (o *Op) RunsInWorkspace() bool {
	return !o.Account
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

// Call runs the operation as actor in their workspace.
func (o *Op) Call(ctx context.Context, actor *auth.Actor, raw M) (any, error) {
	if err := actor.Require(o.Access); err != nil {
		return nil, err
	}
	if o.forAccount != nil {
		return o.forAccount(ctx, actor.Caller(), raw)
	}
	return o.inWorkspace(ctx, actor, raw)
}

// CallAs runs the operation for a caller of the API or MCP, in the workspace named by the
// workspace argument, or else fallback (see access_service.Pick).
func (reg *Registry) CallAs(ctx context.Context, o *Op, caller *auth.Caller, raw M, fallback string) (any, error) {
	if raw == nil {
		raw = M{}
	}
	named, err := takeWorkspace(raw)
	if err != nil {
		return nil, err
	}
	if o.forAccount != nil {
		if err := caller.Require(o.Access); err != nil {
			return nil, err
		}
		return o.forAccount(ctx, caller, raw)
	}
	if named == "" {
		named = fallback
	}
	actor, err := reg.picker.Pick(ctx, caller, named, raw)
	if err != nil {
		return nil, err
	}
	return o.Call(ctx, actor, raw)
}

// takeWorkspace removes the workspace argument, which every operation takes besides its own.
func takeWorkspace(raw M) (string, error) {
	value, ok := raw["workspace"]
	delete(raw, "workspace")
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
	return "", errdefs.BadRequest("workspace is a URL key or id")
}

func cloneJSON(value M) M {
	var copied M
	_ = json.Unmarshal(errdefs.Marshal(value), &copied)
	return copied
}

// Call runs the named operation as actor, as templates and other services do.
func (reg *Registry) Call(ctx context.Context, actor *auth.Actor, name string, raw map[string]any) (any, error) {
	op := reg.Find(name)
	if op == nil {
		return nil, errdefs.BadRequest("No operation %q", name)
	}
	return op.Call(ctx, actor, raw)
}
