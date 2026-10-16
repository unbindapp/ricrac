package ops

import (
	"context"

	"ricrac/app"
)

// Not ported yet.
func webhookOps() []*Op { return nil }

// DeliverDueWebhooks is not ported yet. It sends the queued deliveries that are due and says how
// many it sent.
func DeliverDueWebhooks(ctx context.Context, state *app.State) (int, error) { return 0, nil }
