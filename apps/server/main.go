package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ricrac/app"
	"ricrac/server"
)

func main() {
	config, err := app.ConfigFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}
	if !config.GoogleConfigured() {
		slog.Warn("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET aren't set: nobody can sign in")
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
