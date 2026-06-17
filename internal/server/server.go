// Package server wires together the HTTP server, migration runner, and stores.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	authmw "github.com/uigraph/app/internal/middleware"

	"github.com/uigraph/app/internal/api"
	"github.com/uigraph/app/internal/bootstrap"
	"github.com/uigraph/app/internal/cache"
	"github.com/uigraph/app/internal/config"
	"github.com/uigraph/app/internal/migrate"
	"github.com/uigraph/app/internal/storage"
	"github.com/uigraph/app/internal/store/postgres"
)

// Run starts the HTTP server and blocks until ctx is cancelled.
// Boot order:
//  1. Connect to Postgres
//  2. Run pending migrations (uigraph-app owns the schema)
//  3. Bootstrap default org + admin user (idempotent)
//  4. Connect to object storage (required — diagram content lives here)
//  5. Connect to Redis cache (optional — absent = no caching)
//  6. Bind HTTP listener and begin serving (healthz returns 200 after this)
func Run(ctx context.Context, cfg *config.Config) error {
	db, err := postgres.Open(cfg.PostgresURL)
	if err != nil {
		return fmt.Errorf("server: open postgres: %w", err)
	}
	defer db.Close()

	slog.InfoContext(ctx, "running migrations")
	if err := migrate.Run(ctx, db.DB()); err != nil {
		return fmt.Errorf("server: migrations: %w", err)
	}
	slog.InfoContext(ctx, "migrations complete")

	if err := bootstrap.Run(ctx, db, cfg); err != nil {
		return fmt.Errorf("server: bootstrap: %w", err)
	}

	storageClient, err := storage.New(storage.Config{
		Backend:   cfg.StorageBackend,
		Endpoint:  cfg.StorageEndpoint,
		Bucket:    cfg.StorageBucket,
		AccessKey: cfg.StorageAccessKey,
		SecretKey: cfg.StorageSecretKey,
		Region:    cfg.StorageRegion,
	})
	if err != nil {
		return fmt.Errorf("server: init storage: %w", err)
	}
	if err := storageClient.EnsureBucket(ctx); err != nil {
		return fmt.Errorf("server: ensure storage bucket: %w", err)
	}

	if err := bootstrap.SeedComponents(ctx, db, storageClient); err != nil {
		return fmt.Errorf("server: seed components: %w", err)
	}

	var cacheClient cache.Client
	if cfg.RedisURL != "" {
		c, err := cache.New(cfg.RedisURL)
		if err != nil {
			slog.WarnContext(ctx, "redis unavailable — caching disabled", "err", err)
		} else {
			cacheClient = c
			slog.InfoContext(ctx, "redis cache enabled")
		}
	}

	bearer := authmw.NewSessionVerifier(db, db)
	handler := api.New(db, bearer, cfg, storageClient, cacheClient)

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: handler,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.InfoContext(ctx, "listening", "addr", cfg.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		return srv.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}
