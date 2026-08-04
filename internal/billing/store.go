package billing

import "context"

type Store interface {
	// CreateCloudConnection stores encryptedPayload as-is — callers must
	// encrypt the marshaled ConnectionInput with crypto.Cipher first.
	CreateCloudConnection(ctx context.Context, orgID, actorID string, provider Provider, displayName, encryptedPayload string) (*Connection, error)
	ListCloudConnections(ctx context.Context, orgID string) ([]Connection, error)
	GetCloudConnection(ctx context.Context, orgID, id string) (*Connection, error)
	// GetCloudConnectionAuth returns the connection plus its still-encrypted
	// auth payload, for the caller to decrypt — used by the sync worker /
	// test-connection flow only, never returned over the API.
	GetCloudConnectionAuth(ctx context.Context, orgID, id string) (*Connection, string, error)
	UpdateCloudConnectionStatus(ctx context.Context, id string, status ConnectionStatus, message string) error
	DeleteCloudConnection(ctx context.Context, orgID, id string) error
	ListActiveCloudConnections(ctx context.Context) ([]Connection, error)

	CreateTagRule(ctx context.Context, orgID, serviceID, actorID, tagKey, tagValue string) (*TagRule, error)
	ListTagRules(ctx context.Context, orgID, serviceID string) ([]TagRule, error)
	DeleteTagRule(ctx context.Context, orgID, serviceID, ruleID string) error

	// UpsertCostResources returns a map of ExternalResourceID -> internal
	// resource ID, so callers can attach daily usage points afterward.
	UpsertCostResources(ctx context.Context, orgID, connectionID string, resources []Resource) (map[string]string, error)
	UpsertCostUsageDaily(ctx context.Context, orgID, resourceID string, usageDate string, costUSD float64) error
	ListResourcesForService(ctx context.Context, orgID, serviceID string) ([]Resource, error)
	ListTrendForService(ctx context.Context, orgID, serviceID string, days int) ([]TrendPoint, error)
	// GetResource is org-scoped so a resource ID from one org can't be used
	// to read another org's cost data.
	GetResource(ctx context.Context, orgID, resourceID string) (*Resource, error)
	ListDailyCostsForResource(ctx context.Context, orgID, resourceID string, days int) ([]ResourceDailyCost, error)

	CreateSyncRun(ctx context.Context, connectionID string) (*SyncRun, error)
	FinishSyncRun(ctx context.Context, runID string, status string, resourceCount int, errMsg string) error
}
