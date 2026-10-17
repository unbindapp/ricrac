// Package legacy serves, from the previous server's packages, what isn't ported yet.
package legacy

import (
	"context"
	"net/http"

	"ricrac/app"
	"ricrac/config"
	"ricrac/internal/api/oapi"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/ops"
	"ricrac/server"
)

type Bridge struct {
	State *app.State
}

func New(ctx context.Context, cfg config.Config) (*Bridge, error) {
	pool, err := app.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	return &Bridge{State: app.NewState(pool, app.Config(cfg))}, nil
}

func (b *Bridge) Close() {
	b.State.DB.Close()
}

// Mux serves the routes not ported yet.
func (b *Bridge) Mux() http.Handler {
	return server.Mux(b.State)
}

func (b *Bridge) StartJobs(ctx context.Context) {
	server.StartJobs(ctx, b.State)
}

// Order is the operations' order.
func (b *Bridge) Order() []string {
	var names []string
	for _, op := range ops.All() {
		names = append(names, op.Name)
	}
	return names
}

// AddOps registers the operations not ported yet.
func (b *Bridge) AddOps(reg *oapi.Registry) {
	for _, op := range ops.All() {
		if reg.Find(op.Name) != nil {
			continue
		}
		var routes []oapi.Route
		for _, route := range op.Routes {
			routes = append(routes, oapi.Route{Method: route.Method, Path: route.Path})
		}
		meta := oapi.Op{Name: op.Name, Description: op.Description, Access: auth.Access(op.Access), Routes: routes, Schema: op.Schema, Result: op.Result}
		if op.RunsInWorkspace() {
			reg.Add(oapi.NewOp(meta, func(ctx context.Context, actor *auth.Actor, raw oapi.M) (any, error) {
				return converted(op.Call(b.ctx(ctx, actor), raw))
			}, nil))
			continue
		}
		reg.Add(oapi.NewOp(meta, nil, func(ctx context.Context, caller *auth.Caller, raw oapi.M) (any, error) {
			return converted(op.CallAs(b.caller(ctx, caller), raw, ""))
		}))
	}
}

func (b *Bridge) ctx(ctx context.Context, actor *auth.Actor) *app.Ctx {
	c := &app.Ctx{Context: ctx, State: b.State, User: actor.User, Workspace: actor.Workspace, Role: app.Role(actor.Role), Scopes: scopes(actor.Scopes)}
	if actor.Origin != nil {
		c.Origin = &app.Origin{Label: actor.Origin.Label, URL: actor.Origin.URL, FromGitHubIssue: actor.Origin.FromGitHubIssue}
	}
	return c
}

func (b *Bridge) caller(ctx context.Context, caller *auth.Caller) *app.Caller {
	return &app.Caller{Context: ctx, State: b.State, User: caller.User, Scopes: scopes(caller.Scopes), Workspaces: caller.Workspaces}
}

func scopes(list []auth.Access) []app.Access {
	converted := make([]app.Access, len(list))
	for i, access := range list {
		converted[i] = app.Access(access)
	}
	return converted
}

func converted(value any, err error) (any, error) {
	if err == nil {
		return value, nil
	}
	legacy := app.AsError(err)
	return nil, &errdefs.Error{Status: legacy.Status, Message: legacy.Message}
}

func (b *Bridge) WakeDeliveries() {
	b.State.QueueDeliveries()
}

func (b *Bridge) WakeGitHub() {
	b.State.QueueGitHub()
}
