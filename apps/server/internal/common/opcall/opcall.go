// Package opcall lets services run an operation by name, as templates do with the arguments
// they store.
package opcall

import (
	"context"

	"ricrac/internal/auth"
)

type Caller interface {
	Call(ctx context.Context, actor *auth.Actor, name string, raw map[string]any) (any, error)
}

// Late is a Caller set once the operations are registered, after the services exist.
type Late struct {
	caller Caller
}

func (l *Late) Set(caller Caller) {
	l.caller = caller
}

func (l *Late) Call(ctx context.Context, actor *auth.Actor, name string, raw map[string]any) (any, error) {
	return l.caller.Call(ctx, actor, name, raw)
}
