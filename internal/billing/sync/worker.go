// Package sync runs the scheduled cloud-billing sync: on an interval, pull
// resource inventory + cost data for every connected cloud account and
// upsert it into cost_resources / cost_usage_daily. Billing data from every
// provider is inherently 24-48h lagged, so a ticker-driven goroutine in the
// existing API binary is enough — no separate worker service or cron
// system, the same shape as internal/screenshot's queue worker.
package sync

import (
	"context"
	"log/slog"
	"time"

	"github.com/uigraph/app/internal/billing"
	"github.com/uigraph/app/internal/crypto"
)

const defaultInterval = 6 * time.Hour

type Worker struct {
	store    billing.Store
	cipher   *crypto.Cipher
	adapters map[billing.Provider]billing.ProviderAdapter
	interval time.Duration
}

func New(store billing.Store, cipher *crypto.Cipher, adapters map[billing.Provider]billing.ProviderAdapter) *Worker {
	return &Worker{store: store, cipher: cipher, adapters: adapters, interval: defaultInterval}
}

// Run blocks until ctx is cancelled, syncing all active connections once
// immediately and then again every interval.
func (w *Worker) Run(ctx context.Context) {
	w.syncAll(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.syncAll(ctx)
		}
	}
}

func (w *Worker) syncAll(ctx context.Context) {
	conns, err := w.store.ListActiveCloudConnections(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "billing sync: list connections", "err", err)
		return
	}
	for _, conn := range conns {
		if err := w.syncOne(ctx, conn.OrgID, conn.ID); err != nil {
			slog.ErrorContext(ctx, "billing sync: connection failed", "connectionId", conn.ID, "provider", conn.Provider, "err", err)
		}
	}
}

func (w *Worker) syncOne(ctx context.Context, orgID, connectionID string) error {
	conn, encrypted, err := w.store.GetCloudConnectionAuth(ctx, orgID, connectionID)
	if err != nil {
		return err
	}
	adapter, ok := w.adapters[conn.Provider]
	if !ok {
		return nil // no adapter registered for this provider yet — skip quietly
	}
	_, err = billing.RunSync(ctx, w.store, w.cipher, adapter, orgID, conn, encrypted)
	return err
}
