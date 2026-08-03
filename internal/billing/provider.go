package billing

import "context"

// UsagePoint is one day of cost for one resource, as reported by a provider.
type UsagePoint struct {
	ExternalResourceID string
	Date               string // YYYY-MM-DD
	CostUSD            float64
}

// SyncResult is what a Provider returns for one connection sync pass.
type SyncResult struct {
	Resources []Resource
	Usage     []UsagePoint
}

// ProviderAdapter adapts one cloud vendor's billing + resource-tagging APIs.
// Implementations live under internal/billing/providers/<name>.
type ProviderAdapter interface {
	// TestConnection verifies the credentials in `in` can authenticate and
	// read billing data, without persisting anything.
	TestConnection(ctx context.Context, in ConnectionInput) error
	// Sync pulls the current resource inventory and recent daily cost
	// history for the account described by `in`.
	Sync(ctx context.Context, in ConnectionInput) (SyncResult, error)
}
