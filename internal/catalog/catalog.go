// Package catalog defines domain types for the Service Catalog:
// Services, API Groups (with versioning), and API Endpoints.
package catalog

import (
	"encoding/json"
	"time"

	"github.com/uigraph/app/internal/diagram"
	"github.com/uigraph/app/internal/docs"
)

// ── Service ───────────────────────────────────────────────────────────────────

// DocLink is a pasted reference to a doc that lives outside UIGraph (a
// Confluence page, a Notion doc, a Google Doc, a one-pager) — just a label
// and a URL, no file upload involved. Distinct from Doc/ServiceDoc, which
// model uploaded file assets.
type DocLink struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	URL   string `json:"url"`
}

// Service is a software service registered in the catalog.
type Service struct {
	ID                  string          `json:"id"`
	OrgID               string          `json:"orgId"`
	FolderID            *string         `json:"folderId,omitempty"`
	TeamID              *string         `json:"teamId,omitempty"`
	TeamName            string          `json:"-"`
	Name                string          `json:"name"`
	Description         string          `json:"description"`
	Status              string          `json:"status"`
	Tier                string          `json:"tier"`
	Category            string          `json:"category"`
	Language            string          `json:"language"`
	GitRepoURL          *string         `json:"gitRepoUrl,omitempty"`
	JiraProjectURL      *string         `json:"jiraProjectUrl,omitempty"`
	SlackChannelURL     *string         `json:"slackChannelUrl,omitempty"`
	LastCommitSha       *string         `json:"lastCommitSha,omitempty"`
	Labels              []string        `json:"labels"`
	Metadata            json.RawMessage `json:"metadata,omitempty"`
	DocLinks            []DocLink       `json:"docLinks,omitempty"`
	CreatedBy           string          `json:"createdBy"`
	UpdatedBy           *string         `json:"updatedBy,omitempty"`
	CreatedByCommitHash *string         `json:"createdByCommitHash,omitempty"`
	UpdatedByCommitHash *string         `json:"updatedByCommitHash,omitempty"`
	CreatedAt           time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
	DeletedAt           *time.Time      `json:"deletedAt,omitempty"`
	DeletedBy           *string         `json:"deletedBy,omitempty"`
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

type ServiceDependency struct {
	ID                  string     `json:"id"`
	ServiceID           string     `json:"serviceId"`
	DependencyID        *string    `json:"dependencyId,omitempty"`
	DependencyName      string     `json:"dependencyName"`
	Direction           string     `json:"direction"`
	OrgID               string     `json:"orgId"`
	Name                string     `json:"name"`
	Type                string     `json:"type"`
	Criticality         string     `json:"criticality"`
	Description         string     `json:"description"`
	APIGroupName        *string    `json:"apiGroupName,omitempty"`
	APIEndpointNames    []string   `json:"apiEndpointNames"`
	DatabaseName        *string    `json:"databaseName,omitempty"`
	CreatedBy           string     `json:"createdBy"`
	UpdatedBy           *string    `json:"updatedBy,omitempty"`
	CreatedByCommitHash *string    `json:"createdByCommitHash,omitempty"`
	UpdatedByCommitHash *string    `json:"updatedByCommitHash,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
	DeletedAt           *time.Time `json:"deletedAt,omitempty"`
	DeletedBy           *string    `json:"deletedBy,omitempty"`
}

type ServiceDependencyEdge struct {
	ServiceDependency
	Service    *Service `json:"service,omitempty"`
	Dependency *Service `json:"dependency,omitempty"`
}

// ── API Group ─────────────────────────────────────────────────────────────────

// APIGroup is a versioned collection of API endpoints for a service.
// The spec file content lives in object storage; only the key and hash are in Postgres.
type APIGroup struct {
	ID                  string     `json:"id"`
	ServiceID           string     `json:"serviceId"`
	OrgID               string     `json:"orgId"`
	Name                string     `json:"name"`
	Version             string     `json:"version"`
	Label               *string    `json:"label,omitempty"`
	Protocol            string     `json:"protocol"`
	SpecKey             *string    `json:"specKey,omitempty"`
	SpecHash            *string    `json:"specHash,omitempty"`
	CreatedBy           string     `json:"createdBy"`
	UpdatedBy           *string    `json:"updatedBy,omitempty"`
	CreatedByCommitHash *string    `json:"createdByCommitHash,omitempty"`
	UpdatedByCommitHash *string    `json:"updatedByCommitHash,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
	DeletedAt           *time.Time `json:"deletedAt,omitempty"`
	DeletedBy           *string    `json:"deletedBy,omitempty"`
}

// APIGroupVersion is an immutable spec snapshot.
type APIGroupVersion struct {
	ID                  string    `json:"id"`
	APIGroupID          string    `json:"apiGroupId"`
	VersionNumber       int       `json:"versionNumber"`
	Label               *string   `json:"label,omitempty"`
	SpecKey             string    `json:"specKey"`
	SpecHash            string    `json:"specHash"`
	IsAutoVersion       bool      `json:"isAutoVersion"`
	CreatedBy           string    `json:"createdBy"`
	CreatedByCommitHash *string   `json:"createdByCommitHash,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
}

// PublishAPIGroupVersionInput drives an atomic API-group version snapshot.
//
// The entire operation runs in a single transaction: optionally replacing the
// working-copy endpoints, assigning the next version number, inserting the
// immutable version row, copying the working-copy endpoints into that version,
// and updating the api_groups working-copy row. Either all of it commits or none
// of it does, so the working copy is never mutated without a matching snapshot.
type PublishAPIGroupVersionInput struct {
	// Group is the working-copy row to persist (spec key/hash, name, version
	// label, protocol, updatedBy). Its Version field is overwritten with the
	// resolved version label.
	Group APIGroup
	// ReplaceEndpoints replaces the working-copy endpoints with NewEndpoints
	// before snapshotting (used when a new spec is imported). When false the
	// current working copy is snapshotted as-is (explicit publish / restore).
	ReplaceEndpoints bool
	NewEndpoints     []APIEndpoint
	// Version is the snapshot row to insert. VersionNumber <= 0 auto-assigns the
	// next number (MAX+1). When Label is nil, a "v{N}" label is derived and also
	// applied to Group.Version.
	Version APIGroupVersion
	ActorID string
}

// ── API Endpoint ──────────────────────────────────────────────────────────────

// EndpointSLA is a set of pass/fail thresholds defined once on an API
// endpoint (e.g. "P95 latency <= 300ms") — any load test run that targets
// this endpoint is graded against it, instead of every test pack redeclaring
// its own thresholds. Reuses LoadTestThreshold's generic {id, metric,
// comparator, value} shape.
type EndpointSLA struct {
	Thresholds []LoadTestThreshold `json:"thresholds,omitempty"`
}

// APIEndpoint is a single operation within an API group.
// APIGroupVersionID is nil for the current working copy; set to the version's ID when snapshotted.
type APIEndpoint struct {
	ID                  string          `json:"id"`
	APIGroupID          string          `json:"apiGroupId"`
	APIGroupVersionID   *string         `json:"apiGroupVersionId,omitempty"`
	ServiceID           string          `json:"serviceId"`
	OrgID               string          `json:"orgId"`
	OperationID         string          `json:"operationId"`
	Method              string          `json:"method"`
	Path                string          `json:"path"`
	Summary             string          `json:"summary"`
	Description         string          `json:"description"`
	Tags                []string        `json:"tags"`
	TokenCount          int             `json:"tokenCount"`
	Parameters          json.RawMessage `json:"parameters"`
	RequestBody         json.RawMessage `json:"requestBody"`
	Responses           json.RawMessage `json:"responses"`
	ExampleRequests     json.RawMessage `json:"exampleRequests"`
	ExampleResponses    json.RawMessage `json:"exampleResponses"`
	Order               float64         `json:"order"`
	SLA                 *EndpointSLA    `json:"sla,omitempty"`
	CreatedBy           string          `json:"createdBy"`
	UpdatedBy           *string         `json:"updatedBy,omitempty"`
	CreatedByCommitHash *string         `json:"createdByCommitHash,omitempty"`
	UpdatedByCommitHash *string         `json:"updatedByCommitHash,omitempty"`
	CreatedAt           time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
	DeletedAt           *time.Time      `json:"deletedAt,omitempty"`
	DeletedBy           *string         `json:"deletedBy,omitempty"`
}

// ── Service Doc ───────────────────────────────────────────────────────────────

// ServiceDoc is a doc linked to a service through a junction row.
type ServiceDoc struct {
	ServiceID           string     `json:"serviceId"`
	DocID               string     `json:"docId"`
	OrgID               string     `json:"orgId"`
	CreatedBy           string     `json:"createdBy"`
	UpdatedBy           *string    `json:"updatedBy,omitempty"`
	CreatedByCommitHash *string    `json:"createdByCommitHash,omitempty"`
	UpdatedByCommitHash *string    `json:"updatedByCommitHash,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
	DeletedAt           *time.Time `json:"deletedAt,omitempty"`
	Doc                 *docs.Doc  `json:"doc,omitempty"`
}

// ServiceDiagram is a diagram linked to a service through a junction row.
type ServiceDiagram struct {
	ServiceID           string           `json:"serviceId"`
	DiagramID           string           `json:"diagramId"`
	OrgID               string           `json:"orgId"`
	CreatedBy           string           `json:"createdBy"`
	UpdatedBy           *string          `json:"updatedBy,omitempty"`
	CreatedByCommitHash *string          `json:"createdByCommitHash,omitempty"`
	UpdatedByCommitHash *string          `json:"updatedByCommitHash,omitempty"`
	CreatedAt           time.Time        `json:"createdAt"`
	UpdatedAt           time.Time        `json:"updatedAt"`
	DeletedAt           *time.Time       `json:"deletedAt,omitempty"`
	Diagram             *diagram.Diagram `json:"diagram,omitempty"`
}

// ── Service DB ────────────────────────────────────────────────────────────────

// ServiceDB is the current database schema attached to a service.
type ServiceDB struct {
	ID                  string          `json:"id"`
	ServiceID           string          `json:"serviceId"`
	OrgID               string          `json:"orgId"`
	DBName              string          `json:"dbName"`
	DBType              string          `json:"dbType"`
	Dialect             string          `json:"dialect"`
	SchemaJSON          json.RawMessage `json:"schemaJson"`
	Source              *string         `json:"source,omitempty"`
	SourceTS            *time.Time      `json:"sourceTs,omitempty"`
	SchemaTokenCount    int             `json:"schemaTokenCount"`
	CreatedBy           string          `json:"createdBy"`
	UpdatedBy           *string         `json:"updatedBy,omitempty"`
	CreatedByCommitHash *string         `json:"createdByCommitHash,omitempty"`
	UpdatedByCommitHash *string         `json:"updatedByCommitHash,omitempty"`
	CreatedAt           time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
	DeletedAt           *time.Time      `json:"deletedAt,omitempty"`
	DeletedBy           *string         `json:"deletedBy,omitempty"`
}

// ServiceDBVersion is an immutable snapshot of a service DB schema.
type ServiceDBVersion struct {
	ID                  string          `json:"id"`
	ServiceDBID         string          `json:"serviceDbId"`
	VersionNumber       int             `json:"versionNumber"`
	Label               *string         `json:"label,omitempty"`
	SchemaJSON          json.RawMessage `json:"schemaJson"`
	Source              *string         `json:"source,omitempty"`
	SourceTS            *time.Time      `json:"sourceTs,omitempty"`
	IsAutoVersion       bool            `json:"isAutoVersion"`
	CreatedBy           string          `json:"createdBy"`
	CreatedByCommitHash *string         `json:"createdByCommitHash,omitempty"`
	CreatedAt           time.Time       `json:"createdAt"`
}

// ── Saved Queries ─────────────────────────────────────────────────────────────

type SavedQueryScope string

const (
	SavedQueryScopePersonal SavedQueryScope = "personal"
	SavedQueryScopeTeam     SavedQueryScope = "team"
)

// SavedQueryFolder groups SavedQueries within a service DB, scoped either to a
// single user (personal) or the whole org/team (team).
type SavedQueryFolder struct {
	ID          string          `json:"id"`
	OrgID       string          `json:"orgId"`
	ServiceDBID string          `json:"serviceDbId"`
	Scope       SavedQueryScope `json:"scope"`
	OwnerUserID *string         `json:"ownerUserId,omitempty"`
	TeamID      *string         `json:"teamId,omitempty"`
	Name        string          `json:"name"`
	CreatedBy   string          `json:"createdBy"`
	UpdatedBy   *string         `json:"updatedBy,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	DeletedAt   *time.Time      `json:"deletedAt,omitempty"`
	DeletedBy   *string         `json:"deletedBy,omitempty"`
}

// SavedQuery is a saved SQL/NoSQL query snippet attached to a service DB.
// CLI/CI-synced rows always have Scope=team, TeamID=nil, Source="ci", and a
// non-nil SourceRef used to upsert on repeated syncs without duplicating rows.
type SavedQuery struct {
	ID                  string          `json:"id"`
	OrgID               string          `json:"orgId"`
	ServiceDBID         string          `json:"serviceDbId"`
	FolderID            *string         `json:"folderId,omitempty"`
	Scope               SavedQueryScope `json:"scope"`
	OwnerUserID         *string         `json:"ownerUserId,omitempty"`
	TeamID              *string         `json:"teamId,omitempty"`
	Title               string          `json:"title"`
	Description         string          `json:"description"`
	QueryText           string          `json:"queryText"`
	Tags                []string        `json:"tags"`
	Source              *string         `json:"source,omitempty"`
	SourceRef           *string         `json:"sourceRef,omitempty"`
	CreatedBy           string          `json:"createdBy"`
	UpdatedBy           *string         `json:"updatedBy,omitempty"`
	CreatedByCommitHash *string         `json:"createdByCommitHash,omitempty"`
	UpdatedByCommitHash *string         `json:"updatedByCommitHash,omitempty"`
	CreatedAt           time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
	DeletedAt           *time.Time      `json:"deletedAt,omitempty"`
	DeletedBy           *string         `json:"deletedBy,omitempty"`
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

// LoadTestThreshold is a pass/fail gate evaluated against a load test
// run's metrics, e.g. {metric: "p95LatencyMs", comparator: "<", value: 300}.
type LoadTestThreshold struct {
	ID         string  `json:"id"`
	Metric     string  `json:"metric"`
	Comparator string  `json:"comparator"`
	Value      float64 `json:"value"`
}

// LoadPackConfig holds what a "load" TestPack targets and how its runs
// should be graded. Stored on TestPack as a nullable JSONB payload,
// mirroring the TestCase sub-type payload pattern below.
type LoadPackConfig struct {
	TargetEndpoints []string            `json:"targetEndpoints,omitempty"`
	Thresholds      []LoadTestThreshold `json:"thresholds,omitempty"`
}

type LoadTestTimeSeriesPoint struct {
	TSec         int     `json:"tSec"`
	RPS          float64 `json:"rps"`
	ErrorRate    float64 `json:"errorRate"`
	P95LatencyMs float64 `json:"p95LatencyMs"`
	ActiveVUs    int     `json:"activeVUs"`
}

type LoadTestEndpointBreakdown struct {
	Endpoint       string  `json:"endpoint"`
	Method         string  `json:"method"`
	RequestCount   int     `json:"requestCount"`
	RequestsPerSec float64 `json:"requestsPerSec"`
	ErrorRate      float64 `json:"errorRate"`
	AvgLatencyMs   float64 `json:"avgLatencyMs"`
	P50LatencyMs   float64 `json:"p50LatencyMs"`
	P90LatencyMs   float64 `json:"p90LatencyMs"`
	P95LatencyMs   float64 `json:"p95LatencyMs"`
	P99LatencyMs   float64 `json:"p99LatencyMs"`
	// APIEndpointID, when set, links this breakdown row back to the real
	// APIEndpoint so its SLA can be looked up and evaluated against.
	APIEndpointID *string `json:"apiEndpointId,omitempty"`
}

// CustomMetricPoint is one sample of a user-defined metric over time,
// e.g. a queue-depth or GC-pause curve pulled from an external observability
// tool.
type CustomMetricPoint struct {
	T float64 `json:"t"`
	V float64 `json:"v"`
}

// CustomMetric is an arbitrary, user-named metric attached to a load test
// run beyond the fixed throughput/latency fields above — e.g. "DB CPU %" or
// "Queue Depth" pulled from Grafana/Splunk/SignalFx. Either Value (a single
// summary number) or TimeSeries (for charting) or both may be set.
type CustomMetric struct {
	Name       string              `json:"name"`
	Value      *float64            `json:"value,omitempty"`
	Unit       *string             `json:"unit,omitempty"`
	TimeSeries []CustomMetricPoint `json:"timeSeries,omitempty"`
}

// LoadTestMetrics is the result of one load test run, imported from an
// external tool (e.g. k6) or entered manually. Stored on TestRun as a
// nullable JSONB payload.
type LoadTestMetrics struct {
	DurationSec    float64                     `json:"durationSec"`
	TotalRequests  int                         `json:"totalRequests"`
	RequestsPerSec float64                     `json:"requestsPerSec"`
	ErrorRate      float64                     `json:"errorRate"`
	MinLatencyMs   float64                     `json:"minLatencyMs"`
	AvgLatencyMs   float64                     `json:"avgLatencyMs"`
	MaxLatencyMs   float64                     `json:"maxLatencyMs"`
	P50LatencyMs   float64                     `json:"p50LatencyMs"`
	P90LatencyMs   float64                     `json:"p90LatencyMs"`
	P95LatencyMs   float64                     `json:"p95LatencyMs"`
	P99LatencyMs   float64                     `json:"p99LatencyMs"`
	VirtualUsers   int                         `json:"virtualUsers"`
	TimeSeries     []LoadTestTimeSeriesPoint   `json:"timeSeries,omitempty"`
	PerEndpoint    []LoadTestEndpointBreakdown `json:"perEndpoint,omitempty"`
	CustomMetrics  []CustomMetric              `json:"customMetrics,omitempty"`
	Notes          *string                     `json:"notes,omitempty"`
	// ScreenshotURLs stores asset IDs (not literal URLs), matching the
	// existing (if misleadingly named) TestRunResult.ScreenshotURLs
	// convention — resolve to signed URLs via the asset-url query.
	ScreenshotURLs []string `json:"screenshotUrls,omitempty"`
}

type TestPack struct {
	ID                  string          `json:"testPackId"`
	ServiceID           string          `json:"serviceId"`
	OrgID               string          `json:"orgId"`
	Name                string          `json:"name"`
	Type                string          `json:"type"`
	LoadConfig          *LoadPackConfig `json:"loadConfig,omitempty"`
	BaselineRunID       *string         `json:"baselineRunId,omitempty"`
	CreatedBy           string          `json:"createdBy"`
	UpdatedBy           *string         `json:"updatedBy,omitempty"`
	CreatedByCommitHash *string         `json:"createdByCommitHash,omitempty"`
	UpdatedByCommitHash *string         `json:"updatedByCommitHash,omitempty"`
	DeletedBy           *string         `json:"deletedBy,omitempty"`
	CreatedAt           time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
	DeletedAt           *time.Time      `json:"deletedAt,omitempty"`
}

type TestCase struct {
	ID                    string   `json:"testCaseId"`
	TestPackID            string   `json:"testPackId"`
	ServiceID             string   `json:"serviceId"`
	OrgID                 string   `json:"orgId"`
	Title                 string   `json:"title"`
	Order                 float64  `json:"order"`
	Type                  string   `json:"type"`
	Description           *string  `json:"description,omitempty"`
	Priority              *string  `json:"priority,omitempty"`
	Labels                []string `json:"labels,omitempty"`
	LinkedTicket          *string  `json:"linkedTicket,omitempty"`
	EstimatedDurationMins *int     `json:"estimatedDurationMins,omitempty"`
	TestOwner             *string  `json:"testOwner,omitempty"`
	LinkedMapNodeID       *string  `json:"linkedMapNodeId,omitempty"`
	IsCritical            bool     `json:"isCritical"`
	EvidenceRequired      bool     `json:"evidenceRequired"`
	// ScreenshotURLs are reference screenshots attached to the test case's
	// own definition (e.g. what the expected UI state looks like) — stores
	// asset IDs, distinct from TestRunResult.ScreenshotURLs, which captures
	// evidence from executing the test case.
	ScreenshotURLs      []string          `json:"screenshotUrls,omitempty"`
	Manual              *ManualTestCase   `json:"manual,omitempty"`
	API                 *APITestCase      `json:"api,omitempty"`
	GraphQL             *GraphQLTestCase  `json:"graphql,omitempty"`
	Database            *DatabaseTestCase `json:"database,omitempty"`
	GRPC                *GRPCTestCase     `json:"grpc,omitempty"`
	Status              string            `json:"status"`
	Version             int               `json:"version"`
	BaselineRunResultID *string           `json:"baselineRunResultId,omitempty"`
	Dependencies        []string          `json:"dependencies,omitempty"`
	CreatedBy           string            `json:"createdBy"`
	UpdatedBy           *string           `json:"updatedBy,omitempty"`
	CreatedByCommitHash *string           `json:"createdByCommitHash,omitempty"`
	UpdatedByCommitHash *string           `json:"updatedByCommitHash,omitempty"`
	DeletedBy           *string           `json:"deletedBy,omitempty"`
	CreatedAt           time.Time         `json:"createdAt"`
	UpdatedAt           time.Time         `json:"updatedAt"`
	DeletedAt           *time.Time        `json:"deletedAt,omitempty"`
}

type TestRun struct {
	ID            string           `json:"testRunId"`
	TestPackID    string           `json:"testPackId"`
	ServiceID     string           `json:"serviceId"`
	OrgID         string           `json:"orgId"`
	Environment   string           `json:"environment"`
	ReleaseLabel  *string          `json:"releaseLabel,omitempty"`
	StartedAt     *time.Time       `json:"startedAt,omitempty"`
	CompletedAt   *time.Time       `json:"completedAt,omitempty"`
	Status        string           `json:"status"`
	StartedBy     *string          `json:"startedBy,omitempty"`
	ExecutedBy    string           `json:"executedBy"`
	ExecutedAt    time.Time        `json:"executedAt"`
	OverallStatus string           `json:"overallStatus"`
	LoadMetrics   *LoadTestMetrics `json:"loadMetrics,omitempty"`
	CreatedBy     string           `json:"createdBy"`
	UpdatedBy     *string          `json:"updatedBy,omitempty"`
	CreatedAt     time.Time        `json:"createdAt"`
	UpdatedAt     time.Time        `json:"updatedAt"`
	DeletedAt     *time.Time       `json:"deletedAt,omitempty"`
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
