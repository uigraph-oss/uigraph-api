package billing

import (
	"context"
	"encoding/json"

	"github.com/uigraph/app/internal/crypto"
)

// RunSync decrypts conn's credential payload, runs one sync against
// adapter, and persists the result — the sequence shared by the manual
// "sync now" endpoint and the scheduled background worker.
func RunSync(ctx context.Context, store Store, cipher *crypto.Cipher, adapter ProviderAdapter, orgID string, conn *Connection, encryptedPayload string) (int, error) {
	decrypted, err := cipher.Decrypt(encryptedPayload)
	if err != nil {
		return 0, err
	}
	var in ConnectionInput
	if err := json.Unmarshal([]byte(decrypted), &in); err != nil {
		return 0, err
	}

	run, err := store.CreateSyncRun(ctx, conn.ID)
	if err != nil {
		return 0, err
	}

	result, err := adapter.Sync(ctx, in)
	if err != nil {
		_ = store.FinishSyncRun(ctx, run.ID, "error", 0, err.Error())
		_ = store.UpdateCloudConnectionStatus(ctx, conn.ID, ConnectionStatusError, err.Error())
		return 0, err
	}

	ids, err := store.UpsertCostResources(ctx, orgID, conn.ID, result.Resources)
	if err != nil {
		_ = store.FinishSyncRun(ctx, run.ID, "error", 0, err.Error())
		return 0, err
	}
	for _, u := range result.Usage {
		resourceID, ok := ids[u.ExternalResourceID]
		if !ok {
			continue
		}
		if err := store.UpsertCostUsageDaily(ctx, orgID, resourceID, u.Date, u.CostUSD); err != nil {
			return 0, err
		}
	}

	_ = store.FinishSyncRun(ctx, run.ID, "success", len(result.Resources), "")
	if err := store.UpdateCloudConnectionStatus(ctx, conn.ID, ConnectionStatusConnected, ""); err != nil {
		return len(result.Resources), err
	}
	return len(result.Resources), nil
}
