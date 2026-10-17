package mcpserver

import (
	"encoding/json"

	"ricrac/internal/common/errdefs"
)

// resultText is an answer as tool text, with object keys in alphabetical order as the previous
// server wrote them.
func resultText(value any) string {
	var plain any
	if err := json.Unmarshal(errdefs.Marshal(value), &plain); err != nil {
		return string(errdefs.Marshal(value))
	}
	return string(errdefs.Marshal(plain))
}
