// Package catalog defines domain types for the Service Catalog:
// Services, API Groups (with versioning), and API Endpoints.
package catalog

import (
	"context"
	"encoding/json"
	"time"
)

// ── Service ───────────────────────────────────────────────────────────────────

// Service is a software service registered in the catalog.
type Service struct {
	ID              string          `json:"id"`
	OrgID           string          `json:"orgId"`
	FolderID        *string         `json:"folderId,omitempty"`
	TeamID          *string         `json:"teamId,omitempty"`
	Name            string          `json:"name"`
	Slug            string          `json:"slug"`
	Description     string          `json:"description"`
	Status          string          `json:"status"`
	Tier            string          `json:"tier"`
	Category        string          `json:"category"`
	Language        string          `json:"language"`
	GitRepoURL      *string         `json:"gitRepoUrl,omitempty"`
	JiraProjectURL  *string         `json:"jiraProjectUrl,omitempty"`
	SlackChannelURL *string         `json:"slackChannelUrl,omitempty"`
	LastCommitSha   *string         `json:"lastCommitSha,omitempty"`
	Labels          []string        `json:"labels"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
	CreatedBy       string          `json:"createdBy"`
	UpdatedBy       *string         `json:"updatedBy,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
	DeletedAt       *time.Time      `json:"deletedAt,omitempty"`
	DeletedBy       *string         `json:"deletedBy,omitempty"`
}

// ── API Group ─────────────────────────────────────────────────────────────────

// APIGroup is a versioned collection of API endpoints for a service.
// The spec file content lives in object storage; only the key and hash are in Postgres.
type APIGroup struct {
	ID        string     `json:"id"`
	ServiceID string     `json:"serviceId"`
	OrgID     string     `json:"orgId"`
	Name      string     `json:"name"`
	Version   string     `json:"version"`
	Label     *string    `json:"label,omitempty"`
	Protocol  string     `json:"protocol"`
	SpecKey   *string    `json:"specKey,omitempty"`
	SpecHash  *string    `json:"specHash,omitempty"`
	CreatedBy string     `json:"createdBy"`
	UpdatedBy *string    `json:"updatedBy,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
	DeletedBy *string    `json:"deletedBy,omitempty"`
}

// APIGroupVersion is an immutable spec snapshot.
type APIGroupVersion struct {
	ID            string    `json:"id"`
	APIGroupID    string    `json:"apiGroupId"`
	VersionNumber int       `json:"versionNumber"`
	Label         *string   `json:"label,omitempty"`
	SpecKey       string    `json:"specKey"`
	SpecHash      string    `json:"specHash"`
	IsAutoVersion bool      `json:"isAutoVersion"`
	CreatedBy     string    `json:"createdBy"`
	CreatedAt     time.Time `json:"createdAt"`
}

// ── API Endpoint ──────────────────────────────────────────────────────────────

// APIEndpoint is a single operation within an API group.
type APIEndpoint struct {
	ID          string          `json:"id"`
	APIGroupID  string          `json:"apiGroupId"`
	ServiceID   string          `json:"serviceId"`
	OrgID       string          `json:"orgId"`
	OperationID string          `json:"operationId"`
	Method      string          `json:"method"`
	Path        string          `json:"path"`
	Summary     string          `json:"summary"`
	Description string          `json:"description"`
	Tags        []string        `json:"tags"`
	Parameters  json.RawMessage `json:"parameters"`
	RequestBody json.RawMessage `json:"requestBody"`
	Responses   json.RawMessage `json:"responses"`
	Order       float64         `json:"order"`
	CreatedBy   string          `json:"createdBy"`
	UpdatedBy   *string         `json:"updatedBy,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	DeletedAt   *time.Time      `json:"deletedAt,omitempty"`
	DeletedBy   *string         `json:"deletedBy,omitempty"`
}

// ── Store interface ───────────────────────────────────────────────────────────

// Store is the persistence interface for the service catalog.
type Store interface {
	// Services
	CreateService(ctx context.Context, s Service) error
	GetService(ctx context.Context, id string) (*Service, error)
	GetServiceBySlug(ctx context.Context, orgID, slug string) (*Service, error)
	ListServices(ctx context.Context, orgID string, folderID, teamID *string) ([]Service, error)
	UpdateService(ctx context.Context, s Service) error
	SoftDeleteService(ctx context.Context, id, deletedBy string) error

	// API groups
	CreateAPIGroup(ctx context.Context, g APIGroup) error
	GetAPIGroup(ctx context.Context, id string) (*APIGroup, error)
	ListAPIGroups(ctx context.Context, serviceID string) ([]APIGroup, error)
	UpdateAPIGroup(ctx context.Context, g APIGroup) error
	SoftDeleteAPIGroup(ctx context.Context, id, deletedBy string) error

	// API group versions
	CreateAPIGroupVersion(ctx context.Context, v APIGroupVersion) error
	GetAPIGroupVersion(ctx context.Context, id string) (*APIGroupVersion, error)
	ListAPIGroupVersions(ctx context.Context, apiGroupID string) ([]APIGroupVersion, error)
	LatestAPIGroupVersionNumber(ctx context.Context, apiGroupID string) (int, error)

	// API endpoints
	CreateAPIEndpoint(ctx context.Context, e APIEndpoint) error
	GetAPIEndpoint(ctx context.Context, id string) (*APIEndpoint, error)
	ListAPIEndpoints(ctx context.Context, apiGroupID string) ([]APIEndpoint, error)
	UpdateAPIEndpoint(ctx context.Context, e APIEndpoint) error
	SoftDeleteAPIEndpoint(ctx context.Context, id, deletedBy string) error
}
