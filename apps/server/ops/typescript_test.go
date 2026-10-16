package ops

import (
	"os"
	"testing"
)

// TestWebTypes writes the web client's types when the operations change, as the build expects
// them to be current.
func TestWebTypes(t *testing.T) {
	types, err := TypeScript()
	if err != nil {
		t.Fatal(err)
	}
	const path = "../../web/src/api/ops.gen.ts"
	if current, err := os.ReadFile(path); err == nil && string(current) == types {
		return
	}
	if err := os.WriteFile(path, []byte(types), 0o644); err != nil {
		t.Fatal(err)
	}
}
