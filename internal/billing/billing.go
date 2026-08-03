// Package billing models cloud-provider billing connections, the tag rules
// that link a service to real cloud resources, and the resource/cost data
// synced from each connection.
package billing

import "time"

type Provider string

const (
	ProviderAWS   Provider = "aws"
	ProviderAzure Provider = "azure"
	ProviderGCP   Provider = "gcp"
)

type ConnectionStatus string

const (
	ConnectionStatusPending   ConnectionStatus = "pending"
	ConnectionStatusConnected ConnectionStatus = "connected"
	ConnectionStatusError     ConnectionStatus = "error"
)

type ResourceStatus string

const (
	ResourceStatusRunning  ResourceStatus = "running"
	ResourceStatusStopped  ResourceStatus = "stopped"
	ResourceStatusDegraded ResourceStatus = "degraded"
)

// Connection is an org-scoped link to a cloud billing account. AuthPayload
// is only ever populated with the decrypted credential JSON in-process, long
// enough to call the provider — it is never serialized back to the API.
type Connection struct {
	ID             string
	OrgID          string
	Provider       Provider
	DisplayName    string
	AuthPayload    string
	Status         ConnectionStatus
	StatusMessage  string
	LastVerifiedAt *time.Time
	CreatedBy      string
	UpdatedBy      *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ConnectionInput is the AWS/GCP/Azure-specific credential payload supplied
// when creating or testing a connection, before it is encrypted for storage.
type ConnectionInput struct {
	Provider    Provider
	DisplayName string
	// AWS
	RoleARN    string
	ExternalID string
	// GCP
	ServiceAccountJSON string
	BillingDataset     string
	// Azure
	TenantID       string
	ClientID       string
	ClientSecret   string
	SubscriptionID string
}

// Validate reports whether in carries the required fields for its Provider.
func (in ConnectionInput) Validate() error {
	switch in.Provider {
	case ProviderAWS:
		if in.RoleARN == "" || in.ExternalID == "" {
			return ErrInvalidCredential
		}
	case ProviderGCP:
		if in.ServiceAccountJSON == "" || in.BillingDataset == "" {
			return ErrInvalidCredential
		}
	case ProviderAzure:
		if in.TenantID == "" || in.ClientID == "" || in.ClientSecret == "" || in.SubscriptionID == "" {
			return ErrInvalidCredential
		}
	default:
		return ErrUnknownProvider
	}
	return nil
}

// TagRule is one key=value match rule linking a service to cloud resources.
type TagRule struct {
	ID        string
	OrgID     string
	ServiceID string
	TagKey    string
	TagValue  string
	CreatedBy string
	CreatedAt time.Time
}

// Resource is a discovered cloud resource, synced from a Connection.
type Resource struct {
	ID                 string
	OrgID              string
	CloudConnectionID  string
	ExternalResourceID string
	Name               string
	ResourceType       string
	Provider           Provider
	Region             string
	Environment        string
	Status             ResourceStatus
	MonthlyCostUSD     float64
	Tags               map[string]string
	// MatchedTags is computed per-request against a service's TagRules —
	// the subset of Tags that matched one of the service's rules.
	MatchedTags  []string
	LastSyncedAt time.Time
}

// TrendPoint is one day of cost history for a set of resources.
type TrendPoint struct {
	Date     string
	TotalUSD float64
	AWSUSD   float64
	AzureUSD float64
	GCPUSD   float64
}

// Summary is the top-line KPI row for a service's cost dashboard.
type Summary struct {
	TotalMonthlyCostUSD float64
	MoMChangePct        float64
	ResourceCount       int
	ProviderCount       int
	TopCostDriverLabel  string
	TopCostDriverUSD    float64
}

// SyncRun records one sync attempt against a Connection.
type SyncRun struct {
	ID                string
	CloudConnectionID string
	StartedAt         time.Time
	FinishedAt        *time.Time
	Status            string
	ResourceCount     *int
	ErrorMessage      *string
}
