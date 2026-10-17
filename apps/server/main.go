package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"ricrac/app"
	"ricrac/server"
)

func main() {
	config, err := app.ConfigFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}
	if len(os.Args) == 3 && os.Args[1] == "session" {
		if err := printSession(config, os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if !config.GoogleConfigured() {
		slog.Warn("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET aren't set: nobody can sign in")
	}
	if config.Mail == nil {
		slog.Warn("SMTP_URL and EMAIL_FROM aren't set: no emails are sent")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := app.Connect(ctx, config.DatabaseURL)
	if err != nil {
		slog.Error("connecting to Postgres and migrating failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	state := app.NewState(db, config)
	server.StartJobs(ctx, state)
	address := fmt.Sprintf("0.0.0.0:%d", config.Port)
	srv := &http.Server{Addr: address, Handler: server.Router(state), ReadHeaderTimeout: 30 * time.Second}
	failed := make(chan error, 1)
	go func() { failed <- srv.ListenAndServe() }()
	slog.Info("ricrac " + server.Version + " listening on " + address)
	select {
	case err := <-failed:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}
}

// printSession prints a session cookie for the user with this email, creating them when there's
// none, for signing in without Google during development: go run . session ann@example.com.
func printSession(config app.Config, email string) error {
	ctx := context.Background()
	db, err := app.Connect(ctx, config.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	email = strings.ToLower(strings.TrimSpace(email))
	local, _, _ := strings.Cut(email, "@")
	user, err := app.One[uuid.UUID](ctx, db,
		`INSERT INTO users (email, name, display_name) VALUES ($1, $2, $3)
		 ON CONFLICT (email) DO UPDATE SET email = excluded.email RETURNING id`, email, local, app.DisplayName(local, email))
	if err != nil {
		return err
	}
	keys := app.NewKeys(config)
	fmt.Printf("%s=%s\n", keys.SessionCookie, keys.SessionToken(user))
	return nil
}
