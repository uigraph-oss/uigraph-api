// Package billing models cloud-provider billing connections, the tag rules
// that link a service to real cloud resources, and the resource/cost data
// synced from each connection.
package billing

import (
	"encoding/json"
	"sort"
	"time"
)

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
	ConnectionStatusSyncing   ConnectionStatus = "syncing"
	ConnectionStatusError     ConnectionStatus = "error"
)

type ResourceStatus string

const (
	ResourceStatusRunning  ResourceStatus = "running"
	ResourceStatusStopped  ResourceStatus = "stopped"
	ResourceStatusDegraded ResourceStatus = "degraded"
)

// Connection is an org-scoped link to a cloud billing account. Its
// encrypted credential payload is never held on this struct — callers
// fetch it separately via Store.GetCloudConnectionAuth, only when about to
// decrypt and use it.
type Connection struct {
	ID             string           `json:"id"`
	OrgID          string           `json:"orgId"`
	Provider       Provider         `json:"provider"`
	DisplayName    string           `json:"displayName"`
	Status         ConnectionStatus `json:"status"`
	StatusMessage  string           `json:"statusMessage"`
	LastVerifiedAt *time.Time       `json:"lastVerifiedAt,omitempty"`
	CreatedBy      string           `json:"createdBy"`
	UpdatedBy      *string          `json:"updatedBy,omitempty"`
	CreatedAt      time.Time        `json:"createdAt"`
	UpdatedAt      time.Time        `json:"updatedAt"`
}

// ConnectionInput is the AWS/GCP/Azure-specific credential payload supplied
// when creating or testing a connection, before it is encrypted for storage.
type ConnectionInput struct {
	Provider    Provider `json:"provider"`
	DisplayName string   `json:"displayName"`
	// AWS
	RoleARN    string `json:"roleArn,omitempty"`
	ExternalID string `json:"externalId,omitempty"`
	// GCP
	ServiceAccountJSON string `json:"serviceAccountJson,omitempty"`
	BillingDataset     string `json:"billingDataset,omitempty"`
	// Azure
	TenantID       string `json:"tenantId,omitempty"`
	ClientID       string `json:"clientId,omitempty"`
	ClientSecret   string `json:"clientSecret,omitempty"`
	SubscriptionID string `json:"subscriptionId,omitempty"`
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
	ID        string    `json:"id"`
	OrgID     string    `json:"orgId"`
	ServiceID string    `json:"serviceId"`
	TagKey    string    `json:"tagKey"`
	TagValue  string    `json:"tagValue"`
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
}

// Resource is a discovered cloud resource, synced from a Connection. Tags
// is a real key/value map internally (for matching against TagRules), but
// serializes as a flat "key:value" string list — the shape the UI already
// expects for display.
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

func (r Resource) MarshalJSON() ([]byte, error) {
	tags := make([]string, 0, len(r.Tags))
	for k, v := range r.Tags {
		tags = append(tags, k+":"+v)
	}
	sort.Strings(tags)

	return json.Marshal(struct {
		ID                 string         `json:"id"`
		ExternalResourceID string         `json:"externalResourceId"`
		Name               string         `json:"name"`
		ResourceType       string         `json:"resourceType"`
		Provider           Provider       `json:"provider"`
		Region             string         `json:"region"`
		Environment        string         `json:"environment"`
		Status             ResourceStatus `json:"status"`
		MonthlyCostUSD     float64        `json:"monthlyCostUsd"`
		Tags               []string       `json:"tags"`
		MatchedTags        []string       `json:"matchedTags"`
		LastSyncedAt       time.Time      `json:"lastSyncedAt"`
	}{
		ID:                 r.ID,
		ExternalResourceID: r.ExternalResourceID,
		Name:               r.Name,
		ResourceType:       r.ResourceType,
		Provider:           r.Provider,
		Region:             r.Region,
		Environment:        r.Environment,
		Status:             r.Status,
		MonthlyCostUSD:     r.MonthlyCostUSD,
		Tags:               tags,
		MatchedTags:        nonNil(r.MatchedTags),
		LastSyncedAt:       r.LastSyncedAt,
	})
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// TrendPoint is one day of cost history for a set of resources.
type TrendPoint struct {
	Date     string  `json:"date"`
	TotalUSD float64 `json:"totalUsd"`
	AWSUSD   float64 `json:"awsUsd"`
	AzureUSD float64 `json:"azureUsd"`
	GCPUSD   float64 `json:"gcpUsd"`
}

// ResourceDailyCost is one day of cost history for a single resource.
type ResourceDailyCost struct {
	Date    string  `json:"date"`
	CostUSD float64 `json:"costUsd"`
}

// Summary is the top-line KPI row for a service's cost dashboard.
type Summary struct {
	TotalMonthlyCostUSD float64 `json:"totalMonthlyCostUsd"`
	MoMChangePct        float64 `json:"momChangePct"`
	ResourceCount       int     `json:"resourceCount"`
	ProviderCount       int     `json:"providerCount"`
	TopCostDriverLabel  string  `json:"topCostDriverLabel"`
	TopCostDriverUSD    float64 `json:"topCostDriverUsd"`
}

// SyncRun records one sync attempt against a Connection.
type SyncRun struct {
	ID                string     `json:"id"`
	CloudConnectionID string     `json:"cloudConnectionId"`
	StartedAt         time.Time  `json:"startedAt"`
	FinishedAt        *time.Time `json:"finishedAt,omitempty"`
	Status            string     `json:"status"`
	ResourceCount     *int       `json:"resourceCount,omitempty"`
	ErrorMessage      *string    `json:"errorMessage,omitempty"`
}
