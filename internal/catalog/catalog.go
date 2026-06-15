// Package catalog defines domain types for the Service Catalog:
// Services, API Groups (with versioning), and API Endpoints.
package catalog

import (
	"context"
	"encoding/json"
	"time"

	"github.com/uigraph/app/internal/diagram"
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

// ServiceStats is a lightweight aggregate for service catalog dashboard counts.
type ServiceStats struct {
	ServiceID     string `json:"serviceId"`
	EndpointCount int    `json:"endpointCount"`
	DiagramCount  int    `json:"diagramCount"`
	DocCount      int    `json:"docCount"`
	DBTableCount  int    `json:"dbTableCount"`
	TestCaseCount int    `json:"testCaseCount"`
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

// ── Service Doc ───────────────────────────────────────────────────────────────

// ServiceDoc is a documentation file attached to a service.
// The bytes are stored in object storage; Postgres stores metadata + object key.
type ServiceDoc struct {
	ID          string     `json:"id"`
	ServiceID   string     `json:"serviceId"`
	OrgID       string     `json:"orgId"`
	FileKey     string     `json:"fileKey"`
	FileName    string     `json:"fileName"`
	FileType    string     `json:"fileType"`
	Description string     `json:"description"`
	ContentHash string     `json:"contentHash"`
	CreatedBy   string     `json:"createdBy"`
	UpdatedBy   *string    `json:"updatedBy,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	DeletedAt   *time.Time `json:"deletedAt,omitempty"`
}

// ServiceDiagram is a diagram linked to a service through a junction row.
type ServiceDiagram struct {
	ServiceID string           `json:"serviceId"`
	DiagramID string           `json:"diagramId"`
	OrgID     string           `json:"orgId"`
	CreatedBy string           `json:"createdBy"`
	UpdatedBy *string          `json:"updatedBy,omitempty"`
	CreatedAt time.Time        `json:"createdAt"`
	UpdatedAt time.Time        `json:"updatedAt"`
	DeletedAt *time.Time       `json:"deletedAt,omitempty"`
	Diagram   *diagram.Diagram `json:"diagram,omitempty"`
}

// ── Service DB ────────────────────────────────────────────────────────────────

// ServiceDB is the current database schema attached to a service.
type ServiceDB struct {
	ID         string          `json:"id"`
	ServiceID  string          `json:"serviceId"`
	OrgID      string          `json:"orgId"`
	DBName     string          `json:"dbName"`
	DBType     string          `json:"dbType"`
	Dialect    string          `json:"dialect"`
	SchemaJSON json.RawMessage `json:"schemaJson"`
	Source     *string         `json:"source,omitempty"`
	SourceTS   *time.Time      `json:"sourceTs,omitempty"`
	CreatedBy  string          `json:"createdBy"`
	UpdatedBy  *string         `json:"updatedBy,omitempty"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
	DeletedAt  *time.Time      `json:"deletedAt,omitempty"`
	DeletedBy  *string         `json:"deletedBy,omitempty"`
}

// ServiceDBVersion is an immutable snapshot of a service DB schema.
type ServiceDBVersion struct {
	ID            string          `json:"id"`
	ServiceDBID   string          `json:"serviceDbId"`
	VersionNumber int             `json:"versionNumber"`
	Label         *string         `json:"label,omitempty"`
	SchemaJSON    json.RawMessage `json:"schemaJson"`
	Source        *string         `json:"source,omitempty"`
	SourceTS      *time.Time      `json:"sourceTs,omitempty"`
	IsAutoVersion bool            `json:"isAutoVersion"`
	CreatedBy     string          `json:"createdBy"`
	CreatedAt     time.Time       `json:"createdAt"`
}

// ── Service Tests ─────────────────────────────────────────────────────────────

type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Assertion struct {
	Field string `json:"field"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type AuthConfig struct {
	Type          string  `json:"type"`
	BearerToken   *string `json:"bearerToken,omitempty"`
	APIKeyHeader  *string `json:"apiKeyHeader,omitempty"`
	APIKeyValue   *string `json:"apiKeyValue,omitempty"`
	BasicUsername *string `json:"basicUsername,omitempty"`
	BasicPassword *string `json:"basicPassword,omitempty"`
}

type TestCaseStep struct {
	Order          int    `json:"order"`
	Action         string `json:"action"`
	ExpectedResult string `json:"expectedResult"`
}

type ManualTestCase struct {
	Preconditions   *string        `json:"preconditions,omitempty"`
	TestData        *string        `json:"testData,omitempty"`
	Steps           []TestCaseStep `json:"steps,omitempty"`
	ExpectedOutcome *string        `json:"expectedOutcome,omitempty"`
	Postconditions  *string        `json:"postconditions,omitempty"`
}

type APITestCase struct {
	HTTPMethod         string      `json:"httpMethod"`
	APISpecID          *string     `json:"apiSpecId,omitempty"`
	OperationID        *string     `json:"operationId,omitempty"`
	Auth               *AuthConfig `json:"auth,omitempty"`
	RequestHeaders     []KeyValue  `json:"requestHeaders,omitempty"`
	QueryParams        []KeyValue  `json:"queryParams,omitempty"`
	RequestBody        *string     `json:"requestBody,omitempty"`
	ExpectedStatusCode *int        `json:"expectedStatusCode,omitempty"`
	MaxResponseTimeMs  *int        `json:"maxResponseTimeMs,omitempty"`
	ResponseBody       *string     `json:"responseBody,omitempty"`
	Assertions         []Assertion `json:"assertions,omitempty"`
}

type GraphQLTestCase struct {
	OperationType string      `json:"operationType"`
	OperationName *string     `json:"operationName,omitempty"`
	Query         string      `json:"query"`
	Variables     *string     `json:"variables,omitempty"`
	ResponseBody  *string     `json:"responseBody,omitempty"`
	Assertions    []Assertion `json:"assertions,omitempty"`
	ExpectError   bool        `json:"expectError"`
}

type DatabaseTestCase struct {
	Dialect       string      `json:"dialect"`
	SchemaID      *string     `json:"schemaId,omitempty"`
	Query         string      `json:"query"`
	Assertions    []Assertion `json:"assertions,omitempty"`
	SetupQuery    *string     `json:"setupQuery,omitempty"`
	TeardownQuery *string     `json:"teardownQuery,omitempty"`
}

type GRPCTestCase struct {
	ServiceName    string      `json:"serviceName"`
	MethodName     string      `json:"methodName"`
	CallMode       string      `json:"callMode"`
	ProtoFileID    *string     `json:"protoFileId,omitempty"`
	ServerAddress  *string     `json:"serverAddress,omitempty"`
	RequestMessage *string     `json:"requestMessage,omitempty"`
	Metadata       []KeyValue  `json:"metadata,omitempty"`
	ExpectedStatus string      `json:"expectedStatus"`
	DeadlineMs     *int        `json:"deadlineMs,omitempty"`
	ResponseBody   *string     `json:"responseBody,omitempty"`
	Assertions     []Assertion `json:"assertions,omitempty"`
	UseTLS         bool        `json:"useTLS"`
	ExpectError    bool        `json:"expectError"`
}

type TestPack struct {
	ID        string     `json:"testPackId"`
	ServiceID string     `json:"serviceId"`
	OrgID     string     `json:"orgId"`
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	CreatedBy string     `json:"createdBy"`
	UpdatedBy *string    `json:"updatedBy,omitempty"`
	DeletedBy *string    `json:"deletedBy,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type TestCase struct {
	ID                    string            `json:"testCaseId"`
	TestPackID            string            `json:"testPackId"`
	ServiceID             string            `json:"serviceId"`
	OrgID                 string            `json:"orgId"`
	Title                 string            `json:"title"`
	Order                 float64           `json:"order"`
	Type                  string            `json:"type"`
	Description           *string           `json:"description,omitempty"`
	Priority              *string           `json:"priority,omitempty"`
	Labels                []string          `json:"labels,omitempty"`
	LinkedTicket          *string           `json:"linkedTicket,omitempty"`
	EstimatedDurationMins *int              `json:"estimatedDurationMins,omitempty"`
	TestOwner             *string           `json:"testOwner,omitempty"`
	LinkedMapNodeID       *string           `json:"linkedMapNodeId,omitempty"`
	IsCritical            bool              `json:"isCritical"`
	EvidenceRequired      bool              `json:"evidenceRequired"`
	Manual                *ManualTestCase   `json:"manual,omitempty"`
	API                   *APITestCase      `json:"api,omitempty"`
	GraphQL               *GraphQLTestCase  `json:"graphql,omitempty"`
	Database              *DatabaseTestCase `json:"database,omitempty"`
	GRPC                  *GRPCTestCase     `json:"grpc,omitempty"`
	Status                string            `json:"status"`
	Version               int               `json:"version"`
	BaselineRunResultID   *string           `json:"baselineRunResultId,omitempty"`
	Dependencies          []string          `json:"dependencies,omitempty"`
	CreatedBy             string            `json:"createdBy"`
	UpdatedBy             *string           `json:"updatedBy,omitempty"`
	DeletedBy             *string           `json:"deletedBy,omitempty"`
	CreatedAt             time.Time         `json:"createdAt"`
	UpdatedAt             time.Time         `json:"updatedAt"`
	DeletedAt             *time.Time        `json:"deletedAt,omitempty"`
}

type TestRun struct {
	ID            string     `json:"testRunId"`
	TestPackID    string     `json:"testPackId"`
	ServiceID     string     `json:"serviceId"`
	OrgID         string     `json:"orgId"`
	Environment   string     `json:"environment"`
	ReleaseLabel  *string    `json:"releaseLabel,omitempty"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
	Status        string     `json:"status"`
	StartedBy     *string    `json:"startedBy,omitempty"`
	ExecutedBy    string     `json:"executedBy"`
	ExecutedAt    time.Time  `json:"executedAt"`
	OverallStatus string     `json:"overallStatus"`
	CreatedBy     string     `json:"createdBy"`
	UpdatedBy     *string    `json:"updatedBy,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	DeletedAt     *time.Time `json:"deletedAt,omitempty"`
}

type TestRunSummary struct {
	TestRunID     string     `json:"testRunId"`
	TestPackID    string     `json:"testPackId"`
	ServiceID     string     `json:"serviceId"`
	Environment   string     `json:"environment"`
	ReleaseLabel  *string    `json:"releaseLabel,omitempty"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
	Status        string     `json:"status"`
	StartedBy     *string    `json:"startedBy,omitempty"`
	ExecutedBy    string     `json:"executedBy"`
	ExecutedAt    time.Time  `json:"executedAt"`
	OverallStatus string     `json:"overallStatus"`
	PassedCount   int        `json:"passedCount"`
	FailedCount   int        `json:"failedCount"`
	SkippedCount  int        `json:"skippedCount"`
	BlockedCount  int        `json:"blockedCount"`
}

type TestRunSummaryFilter struct {
	TestPackID  *string
	Environment *string
	Status      *string
	ExecutedBy  *string
	FromDate    *time.Time
	ToDate      *time.Time
}

type TestRunResult struct {
	ID             string     `json:"testRunResultId"`
	TestRunID      string     `json:"testRunId"`
	TestCaseID     string     `json:"testCaseId"`
	ServiceID      string     `json:"serviceId"`
	OrgID          string     `json:"orgId"`
	Status         string     `json:"status"`
	BlockedReason  *string    `json:"blockedReason,omitempty"`
	ResponseStatus *int       `json:"responseStatus,omitempty"`
	ResponseBody   *string    `json:"responseBody,omitempty"`
	ResponseTimeMs *int64     `json:"responseTimeMs,omitempty"`
	Notes          *string    `json:"notes,omitempty"`
	ScreenshotURLs []string   `json:"screenshotUrls,omitempty"`
	ExecutedAt     time.Time  `json:"executedAt"`
	ExecutedBy     string     `json:"executedBy"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	DeletedAt      *time.Time `json:"deletedAt,omitempty"`
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
	ListServiceStats(ctx context.Context, orgID string, serviceID *string) ([]ServiceStats, error)

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

	// Service docs
	CreateServiceDoc(ctx context.Context, d ServiceDoc) error
	GetServiceDoc(ctx context.Context, id string) (*ServiceDoc, error)
	ListServiceDocs(ctx context.Context, serviceID string) ([]ServiceDoc, error)
	UpdateServiceDoc(ctx context.Context, d ServiceDoc) error
	SoftDeleteServiceDoc(ctx context.Context, id string) error

	// Service test packs
	CreateTestPack(ctx context.Context, p TestPack) error
	GetTestPack(ctx context.Context, id string) (*TestPack, error)
	ListTestPacks(ctx context.Context, serviceID string) ([]TestPack, error)
	UpdateTestPack(ctx context.Context, p TestPack) error
	SoftDeleteTestPack(ctx context.Context, id, deletedBy string) error

	// Service test cases
	CreateTestCase(ctx context.Context, tc TestCase) error
	GetTestCase(ctx context.Context, id string) (*TestCase, error)
	ListTestCases(ctx context.Context, serviceID string, testPackID *string) ([]TestCase, error)
	UpdateTestCase(ctx context.Context, tc TestCase) error
	SoftDeleteTestCase(ctx context.Context, id, deletedBy string) error

	// Service test runs
	CreateTestRun(ctx context.Context, tr TestRun) error
	GetTestRun(ctx context.Context, id string) (*TestRun, error)
	ListTestRuns(ctx context.Context, serviceID string, testPackID *string) ([]TestRun, error)
	ListTestRunsSummary(ctx context.Context, serviceID string, filter TestRunSummaryFilter) ([]TestRunSummary, error)
	UpdateTestRun(ctx context.Context, tr TestRun) error

	// Service test run results
	CreateTestRunResult(ctx context.Context, rr TestRunResult) error
	GetTestRunResult(ctx context.Context, id string) (*TestRunResult, error)
	ListTestRunResults(ctx context.Context, serviceID, testRunID string) ([]TestRunResult, error)
	UpdateTestRunResult(ctx context.Context, rr TestRunResult) error

	// Service diagrams
	CreateServiceDiagram(ctx context.Context, d ServiceDiagram) error
	GetServiceDiagram(ctx context.Context, serviceID, diagramID string) (*ServiceDiagram, error)
	ListServiceDiagrams(ctx context.Context, serviceID string) ([]ServiceDiagram, error)
	SoftDeleteServiceDiagram(ctx context.Context, serviceID, diagramID, deletedBy string) error

	// Service DBs
	CreateServiceDB(ctx context.Context, d ServiceDB) error
	GetServiceDB(ctx context.Context, id string) (*ServiceDB, error)
	ListServiceDBs(ctx context.Context, serviceID string) ([]ServiceDB, error)
	UpdateServiceDB(ctx context.Context, d ServiceDB) error
	SoftDeleteServiceDB(ctx context.Context, id, deletedBy string) error

	// Service DB versions
	CreateServiceDBVersion(ctx context.Context, v ServiceDBVersion) error
	GetServiceDBVersion(ctx context.Context, id string) (*ServiceDBVersion, error)
	ListServiceDBVersions(ctx context.Context, serviceDBID string) ([]ServiceDBVersion, error)
	LatestServiceDBVersionNumber(ctx context.Context, serviceDBID string) (int, error)
}
