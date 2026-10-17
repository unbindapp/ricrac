// Package scheduler runs the work that happens on its own: the automations every minute, webhook
// deliveries as soon as they are queued, and the GitHub sync.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"ricrac/internal/common/errdefs"
)

// Wakeups wakes the senders without waiting for them; it is the events service's Waker.
type Wakeups struct {
	deliveries chan struct{}
	github     chan struct{}
}

func NewWakeups() *Wakeups {
	return &Wakeups{deliveries: make(chan struct{}, 1), github: make(chan struct{}, 1)}
}

func wake(c chan struct{}) {
	select {
	case c <- struct{}{}:
	default:
	}
}

func (w *Wakeups) WakeDeliveries() { wake(w.deliveries) }

func (w *Wakeups) WakeGitHub() { wake(w.github) }

type Jobs struct {
	// Run in order every minute.
	EveryMinute []func(context.Context) error
	// Sends due webhook deliveries, saying how many.
	Deliveries func(context.Context) (int, error)
	// Sends changes to GitHub and imports issues, saying whether there's more to do right away;
	// nil when GitHub is off.
	GitHub func(context.Context) (bool, error)
}

// Start runs the jobs until ctx ends.
func Start(ctx context.Context, wakeups *Wakeups, jobs Jobs) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			for _, job := range jobs.EveryMinute {
				if err := job(ctx); err != nil && ctx.Err() == nil {
					slog.Warn("background job failed", "err", errdefs.As(err).Message)
					break
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	go loop(ctx, wakeups.deliveries, 10*time.Second, "webhook delivery failed", func(ctx context.Context) (bool, error) {
		sent, err := jobs.Deliveries(ctx)
		return sent > 0, err
	})
	if jobs.GitHub != nil {
		go loop(ctx, wakeups.github, 30*time.Second, "GitHub sync failed", jobs.GitHub)
	}
}

// loop runs work again right away while it is busy, else when woken or after wait.
func loop(ctx context.Context, woken chan struct{}, wait time.Duration, failure string, work func(context.Context) (bool, error)) {
	for ctx.Err() == nil {
		busy, err := work(ctx)
		if err != nil && ctx.Err() == nil {
			slog.Warn(failure, "err", errdefs.As(err).Message)
		}
		if busy && err == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-woken:
		case <-time.After(wait):
		}
		time.Sleep(200 * time.Millisecond)
	}
}
