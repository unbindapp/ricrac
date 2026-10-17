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

	"ricrac/config"
	"ricrac/ent/user"
	"ricrac/internal/api/meta"
	"ricrac/internal/api/router"
	"ricrac/internal/api/server"
	"ricrac/internal/auth"
	"ricrac/internal/infrastructure/database"
	"ricrac/internal/legacy"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}
	if len(os.Args) == 3 && os.Args[1] == "session" {
		if err := printSession(cfg, os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if !cfg.GoogleConfigured() {
		slog.Warn("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET aren't set: nobody can sign in")
	}
	if cfg.Mail == nil {
		slog.Warn("SMTP_URL and EMAIL_FROM aren't set: no emails are sent")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client, db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connecting to Postgres failed", "err", err)
		os.Exit(1)
	}
	defer client.Close()
	if err := database.Migrate(ctx, db); err != nil {
		slog.Error("migrating failed", "err", err)
		os.Exit(1)
	}
	bridge, err := legacy.New(ctx, cfg)
	if err != nil {
		slog.Error("connecting to Postgres failed", "err", err)
		os.Exit(1)
	}
	defer bridge.Close()
	bridge.StartJobs(ctx)
	handler, _ := router.New(server.New(cfg, client), bridge)
	address := fmt.Sprintf("0.0.0.0:%d", cfg.Port)
	srv := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 30 * time.Second}
	failed := make(chan error, 1)
	go func() { failed <- srv.ListenAndServe() }()
	slog.Info("ricrac " + meta.Version + " listening on " + address)
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
// none, for signing in without Google during development: go run ./cmd/server session ann@example.com.
func printSession(cfg config.Config, email string) error {
	ctx := context.Background()
	client, db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := database.Migrate(ctx, db); err != nil {
		return err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	local, _, _ := strings.Cut(email, "@")
	id, err := client.User.Create().SetEmail(email).SetName(local).SetDisplayName(auth.DisplayName(local, email)).
		OnConflictColumns(user.FieldEmail).UpdateEmail().ID(ctx)
	if err != nil {
		return err
	}
	keys := auth.NewKeys(cfg)
	fmt.Printf("%s=%s\n", keys.SessionCookie, keys.SessionToken(id))
	return nil
}
