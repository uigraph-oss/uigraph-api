// Package catalog provides HTTP handlers for service catalog resources:
// Services, Docs, Diagrams, API Groups, API Endpoints, DBs, and Tests.
package catalog

import (
	"context"
	"io"
	"net/http"

	catalogpkg "github.com/uigraph/app/internal/catalog"
	diagrampkg "github.com/uigraph/app/internal/diagram"
)

// store is the minimal persistence interface this package needs.
type store interface {
	// Services
	ListServices(ctx context.Context, orgID string, folderID, teamID *string) ([]catalogpkg.Service, error)
	ListServiceStats(ctx context.Context, orgID string, serviceID *string) ([]catalogpkg.ServiceStats, error)
	CreateService(ctx context.Context, s catalogpkg.Service) error
	GetService(ctx context.Context, id string) (*catalogpkg.Service, error)
	UpdateService(ctx context.Context, s catalogpkg.Service) error
	SoftDeleteService(ctx context.Context, id, actorID string) error

	// Service docs
	ListServiceDocs(ctx context.Context, serviceID string) ([]catalogpkg.ServiceDoc, error)
	GetServiceDoc(ctx context.Context, id string) (*catalogpkg.ServiceDoc, error)
	CreateServiceDoc(ctx context.Context, d catalogpkg.ServiceDoc) error
	UpdateServiceDoc(ctx context.Context, d catalogpkg.ServiceDoc) error
	SoftDeleteServiceDoc(ctx context.Context, id string) error

	// Service diagrams
	ListServiceDiagrams(ctx context.Context, serviceID string) ([]catalogpkg.ServiceDiagram, error)
	GetServiceDiagram(ctx context.Context, serviceID, diagramID string) (*catalogpkg.ServiceDiagram, error)
	CreateServiceDiagram(ctx context.Context, link catalogpkg.ServiceDiagram) error
	SoftDeleteServiceDiagram(ctx context.Context, serviceID, diagramID, actorID string) error

	// Diagrams (used by CreateDiagram handler)
	GetDiagram(ctx context.Context, id string) (*diagrampkg.Diagram, error)
	CreateDiagram(ctx context.Context, d diagrampkg.Diagram) error
	CreateDiagramVersion(ctx context.Context, v diagrampkg.Version) error

	// API Groups
	ListAPIGroups(ctx context.Context, serviceID string) ([]catalogpkg.APIGroup, error)
	GetAPIGroup(ctx context.Context, id string) (*catalogpkg.APIGroup, error)
	CreateAPIGroup(ctx context.Context, g catalogpkg.APIGroup) error
	UpdateAPIGroup(ctx context.Context, g catalogpkg.APIGroup) error
	SoftDeleteAPIGroup(ctx context.Context, id, actorID string) error
	ListAPIGroupVersions(ctx context.Context, apiGroupID string) ([]catalogpkg.APIGroupVersion, error)
	CreateAPIGroupVersion(ctx context.Context, v catalogpkg.APIGroupVersion) error
	LatestAPIGroupVersionNumber(ctx context.Context, apiGroupID string) (int, error)

	// API Endpoints
	ListAPIEndpoints(ctx context.Context, apiGroupID string) ([]catalogpkg.APIEndpoint, error)
	GetAPIEndpoint(ctx context.Context, id string) (*catalogpkg.APIEndpoint, error)
	CreateAPIEndpoint(ctx context.Context, e catalogpkg.APIEndpoint) error
	UpdateAPIEndpoint(ctx context.Context, e catalogpkg.APIEndpoint) error
	SoftDeleteAPIEndpoint(ctx context.Context, id, actorID string) error

	// Service DBs
	ListServiceDBs(ctx context.Context, serviceID string) ([]catalogpkg.ServiceDB, error)
	GetServiceDB(ctx context.Context, id string) (*catalogpkg.ServiceDB, error)
	CreateServiceDB(ctx context.Context, db catalogpkg.ServiceDB) error
	UpdateServiceDB(ctx context.Context, db catalogpkg.ServiceDB) error
	SoftDeleteServiceDB(ctx context.Context, id, actorID string) error
	ListServiceDBVersions(ctx context.Context, dbID string) ([]catalogpkg.ServiceDBVersion, error)
	GetServiceDBVersion(ctx context.Context, id string) (*catalogpkg.ServiceDBVersion, error)
	CreateServiceDBVersion(ctx context.Context, v catalogpkg.ServiceDBVersion) error
	LatestServiceDBVersionNumber(ctx context.Context, dbID string) (int, error)

	// Test packs
	ListTestPacks(ctx context.Context, serviceID string) ([]catalogpkg.TestPack, error)
	GetTestPack(ctx context.Context, id string) (*catalogpkg.TestPack, error)
	CreateTestPack(ctx context.Context, p catalogpkg.TestPack) error
	UpdateTestPack(ctx context.Context, p catalogpkg.TestPack) error
	SoftDeleteTestPack(ctx context.Context, id, actorID string) error

	// Test cases
	ListTestCases(ctx context.Context, serviceID string, testPackID *string) ([]catalogpkg.TestCase, error)
	GetTestCase(ctx context.Context, id string) (*catalogpkg.TestCase, error)
	CreateTestCase(ctx context.Context, tc catalogpkg.TestCase) error
	UpdateTestCase(ctx context.Context, tc catalogpkg.TestCase) error
	SoftDeleteTestCase(ctx context.Context, id, actorID string) error

	// Test runs
	CreateTestRun(ctx context.Context, tr catalogpkg.TestRun) error
	GetTestRun(ctx context.Context, id string) (*catalogpkg.TestRun, error)
	ListTestRuns(ctx context.Context, serviceID string, testPackID *string) ([]catalogpkg.TestRun, error)
	ListTestRunsSummary(ctx context.Context, serviceID string, filter catalogpkg.TestRunSummaryFilter) ([]catalogpkg.TestRunSummary, error)
	UpdateTestRun(ctx context.Context, tr catalogpkg.TestRun) error

	// Test run results
	CreateTestRunResult(ctx context.Context, rr catalogpkg.TestRunResult) error
	GetTestRunResult(ctx context.Context, id string) (*catalogpkg.TestRunResult, error)
	ListTestRunResults(ctx context.Context, serviceID, testRunID string) ([]catalogpkg.TestRunResult, error)
	UpdateTestRunResult(ctx context.Context, rr catalogpkg.TestRunResult) error
}

// objectStore is the minimal storage interface this package needs.
type objectStore interface {
	Upload(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// Handler serves /api/v1/orgs/{orgID}/services and all nested resources.
type Handler struct {
	store   store
	storage objectStore
}

// New constructs a Handler.
func New(s store, st objectStore) *Handler {
	return &Handler{store: s, storage: st}
}

// Register wires catalog routes into mux.
// requireScope signature: func(scope, method, pattern string, h http.HandlerFunc)
func Register(
	mux *http.ServeMux,
	s store,
	st objectStore,
	requireScope func(scope, method, pattern string, h http.HandlerFunc),
) {
	h := New(s, st)
	// Services
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services", h.List)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/stats", h.ListStats)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services", h.Create)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}", h.Get)
	requireScope("services:write", "PUT", "/api/v1/orgs/{orgID}/services/{serviceID}", h.Update)
	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}", h.Delete)
	// API groups — /sync before /{apiGroupID}
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups", h.ListAPIGroups)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups", h.CreateAPIGroup)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/sync", h.SyncAPIGroup)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}", h.GetAPIGroup)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/spec", h.GetAPIGroupSpec)
	requireScope("services:write", "PUT", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}", h.UpdateAPIGroup)
	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}", h.DeleteAPIGroup)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/versions", h.ListAPIGroupVersions)
	// API endpoints
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints", h.ListAPIEndpoints)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints", h.CreateAPIEndpoint)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints/{endpointID}", h.GetAPIEndpoint)
	requireScope("services:write", "PUT", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints/{endpointID}", h.UpdateAPIEndpoint)
	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints/{endpointID}", h.DeleteAPIEndpoint)
	// Service docs
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/docs", h.ListDocs)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/docs", h.CreateDoc)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/docs/{docID}", h.GetDoc)
	requireScope("services:write", "PUT", "/api/v1/orgs/{orgID}/services/{serviceID}/docs/{docID}", h.UpdateDoc)
	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/docs/{docID}", h.DeleteDoc)
	// Service diagrams
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/diagrams", h.ListDiagrams)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/diagrams", h.CreateDiagram)
	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/diagrams/{diagramID}", h.DeleteDiagram)
	// Service DBs
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs", h.ListDBs)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs", h.CreateDB)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}", h.GetDB)
	requireScope("services:write", "PUT", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}", h.UpdateDB)
	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}", h.DeleteDB)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}/versions", h.ListDBVersions)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}/versions", h.CreateDBVersion)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}/versions/{versionID}/restore", h.RestoreDBVersion)
	// Test packs/cases/runs
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-pack", h.CreateTestPack)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/test-packs", h.ListTestPacks)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-pack/{testPackID}", h.UpdateTestPack)
	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/test-pack/{testPackID}", h.DeleteTestPack)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-case", h.CreateTestCase)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/test-cases", h.ListTestCases)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-case/{testCaseID}", h.UpdateTestCase)
	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/test-case/{testCaseID}", h.DeleteTestCase)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-run", h.CreateTestRun)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/test-runs", h.ListTestRuns)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/test-runs-summary", h.ListTestRunsSummary)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/test-run/{testRunID}", h.GetTestRun)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-run/{testRunID}", h.UpdateTestRun)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-run-result", h.CreateTestRunResult)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/test-run-results", h.ListTestRunResults)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-run-result/{testRunResultID}", h.UpdateTestRunResult)
}
