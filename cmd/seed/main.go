// Command seed populates the database with a small set of demo organisations,
// one admin user per org, and a baseline set of OAuth providers per org.
//
// The seed manifest is defined in internal/seed and currently creates the
// "acme", "globex", and "initech" orgs with three OAuth providers (auth0,
// github, google) replicated into each. The run is idempotent — re-running
// it against a populated database is a safe no-op for any org, user, or
// provider that already exists.
//
// Migrations are run before the seed so the script works against a fresh
// database.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/uigraph/app/internal/config"
	"github.com/uigraph/app/internal/migrate"
	"github.com/uigraph/app/internal/seed"
	"github.com/uigraph/app/internal/store/postgres"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := postgres.Open(cfg.PostgresURL)
	if err != nil {
		slog.Error("open postgres", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := migrate.Run(ctx, db.DB()); err != nil {
		slog.Error("migrations", "err", err)
		os.Exit(1)
	}

	if err := seed.Run(ctx, db); err != nil {
		slog.Error("seed", "err", err)
		os.Exit(1)
	}
}
