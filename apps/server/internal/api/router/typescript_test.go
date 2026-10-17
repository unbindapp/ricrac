package router

import (
	"os"
	"testing"

	"ricrac/app"
	"ricrac/config"
	"ricrac/internal/api/server"
	"ricrac/internal/legacy"
)

func bridge() *legacy.Bridge {
	return &legacy.Bridge{State: &app.State{Config: app.Config{PublicURL: "http://localhost:3000"}}}
}

func registry(t *testing.T) *server.Server {
	t.Helper()
	return server.New(config.Config{PublicURL: "http://localhost:3000"}, nil, nil)
}

// TestWebTypes writes the web client's types when the operations change, as the build expects
// them to be current.
func TestWebTypes(t *testing.T) {
	_, reg := New(registry(t), bridge())
	types, err := reg.TypeScript()
	if err != nil {
		t.Fatal(err)
	}
	const path = "../../../../web/src/api/ops.gen.ts"
	if current, err := os.ReadFile(path); err == nil && string(current) == types {
		return
	}
	if err := os.WriteFile(path, []byte(types), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEveryOperationDeclaresItsResult(t *testing.T) {
	_, reg := New(registry(t), bridge())
	for _, o := range reg.All() {
		if o.Result == nil {
			t.Errorf("%s doesn't declare its result", o.Name)
		}
	}
}
