package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/uigraph/app/internal/config"
	"github.com/uigraph/app/internal/server"
)

// @title                       uigraph API
// @version                     1.0
// @description                 REST API for uigraph.
// @BasePath                    /api/v1
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx, cfg); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}
