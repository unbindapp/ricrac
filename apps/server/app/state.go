package app

import (
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ricrac/mail"
)

type State struct {
	DB      *pgxpool.Pool
	Config  Config
	Keys    Keys
	HTTP    *http.Client
	Limiter *RateLimiter
	// Wakes the webhook sender when deliveries are queued.
	Deliveries chan struct{}
	// Sends emails; nil when email is off.
	Mailer mail.Sender
}

func NewState(db *pgxpool.Pool, config Config) *State {
	var mailer mail.Sender
	if config.Mail != nil {
		mailer = mail.SMTP{Config: *config.Mail}
	}
	return &State{
		DB:         db,
		Config:     config,
		Keys:       NewKeys(config),
		HTTP:       &http.Client{Timeout: 15 * time.Second},
		Limiter:    NewRateLimiter(20, 15*time.Minute),
		Deliveries: make(chan struct{}, 1),
		Mailer:     mailer,
	}
}

// QueueDeliveries wakes the webhook sender without waiting for it.
func (s *State) QueueDeliveries() {
	select {
	case s.Deliveries <- struct{}{}:
	default:
	}
}

// RateLimiter counts attempts per key inside a window.
type RateLimiter struct {
	maxAttempts int
	window      time.Duration
	mu          sync.Mutex
	entries     map[string]attempts
}

type attempts struct {
	count int
	since time.Time
}

func NewRateLimiter(maxAttempts int, window time.Duration) *RateLimiter {
	return &RateLimiter{maxAttempts: maxAttempts, window: window, entries: map[string]attempts{}}
}

// Hit records an attempt and says whether it went over the limit.
func (l *RateLimiter) Hit(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.entries[key]
	if !ok || time.Since(entry.since) >= l.window {
		entry = attempts{since: time.Now()}
	}
	entry.count++
	l.entries[key] = entry
	return entry.count > l.maxAttempts
}

func (l *RateLimiter) Prune() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, entry := range l.entries {
		if time.Since(entry.since) >= l.window {
			delete(l.entries, key)
		}
	}
}
