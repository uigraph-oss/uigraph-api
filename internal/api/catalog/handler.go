// Package catalog provides HTTP handlers for service catalog resources:
// Services, Docs, Diagrams, API Groups, API Endpoints, DBs, and Tests.
package catalog

import (
	"context"
	"io"
	"log/slog"
	"net/http"

	"github.com/uigraph/app/internal/cache"
	catalogpkg "github.com/uigraph/app/internal/catalog"
	diagrampkg "github.com/uigraph/app/internal/diagram"
	docspkg "github.com/uigraph/app/internal/docs"
	"github.com/uigraph/app/internal/queue"
)

// store is the minimal persistence interface this package needs.
type store interface {
	// Services
	ListServices(ctx context.Context, orgID string, p catalogpkg.ListParams) ([]catalogpkg.Service, int, error)
	ListServiceStats(ctx context.Context, orgID string, serviceID *string) ([]catalogpkg.ServiceStats, error)
	CreateService(ctx context.Context, s catalogpkg.Service) error
	GetService(ctx context.Context, id string) (*catalogpkg.Service, error)
	UpdateService(ctx context.Context, s catalogpkg.Service) error
	SoftDeleteService(ctx context.Context, id, actorID string) error
	SyncServiceDependencies(ctx context.Context, orgID, serviceID, actorID string, commitHash *string, dependencies []catalogpkg.ServiceDependency) error
	ListServiceDependencies(ctx context.Context, orgID, serviceID, direction, criticality string) ([]catalogpkg.ServiceDependencyEdge, error)
	DependencyGraph(ctx context.Context, orgID, serviceID string) ([]catalogpkg.ServiceDependencyEdge, error)
	Impact(ctx context.Context, orgID, serviceID, direction string, maxDepth int) ([]catalogpkg.ServiceDependencyEdge, error)

	// Service docs
	ListServiceDocs(ctx context.Context, serviceID string) ([]catalogpkg.ServiceDoc, error)
	GetServiceDoc(ctx context.Context, serviceID, docID string) (*catalogpkg.ServiceDoc, error)
	GetServiceDocByID(ctx context.Context, docID string) (*catalogpkg.ServiceDoc, error)
	CreateServiceDoc(ctx context.Context, d catalogpkg.ServiceDoc) error
	SoftDeleteServiceDoc(ctx context.Context, serviceID, docID, actorID string) error

	// Docs (used by CreateDoc handler)
	GetDoc(ctx context.Context, id string) (*docspkg.Doc, error)
	CreateDoc(ctx context.Context, d docspkg.Doc) error

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
	GetAPIGroupVersion(ctx context.Context, id string) (*catalogpkg.APIGroupVersion, error)
	CreateAPIGroupVersion(ctx context.Context, v catalogpkg.APIGroupVersion) error
	LatestAPIGroupVersionNumber(ctx context.Context, apiGroupID string) (int, error)
	PublishAPIGroupVersion(ctx context.Context, in catalogpkg.PublishAPIGroupVersionInput) (catalogpkg.APIGroupVersion, error)

	// API Endpoints
	ListAPIEndpoints(ctx context.Context, apiGroupID string) ([]catalogpkg.APIEndpoint, error)
	ListAPIEndpointsForVersion(ctx context.Context, apiGroupID, versionID string) ([]catalogpkg.APIEndpoint, error)
	GetAPIEndpoint(ctx context.Context, id string) (*catalogpkg.APIEndpoint, error)
	CreateAPIEndpoint(ctx context.Context, e catalogpkg.APIEndpoint) error
	UpdateAPIEndpoint(ctx context.Context, e catalogpkg.APIEndpoint) error
	SoftDeleteAPIEndpoint(ctx context.Context, id, actorID string) error
	SoftDeleteCurrentAPIEndpoints(ctx context.Context, apiGroupID, actorID string) error
	CopyEndpointsForVersion(ctx context.Context, apiGroupID, versionID, actorID string) error

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

	// Saved query folders
	ListSavedQueryFolders(ctx context.Context, serviceDBID string, scope catalogpkg.SavedQueryScope, ownerUserID *string) ([]catalogpkg.SavedQueryFolder, error)
	GetSavedQueryFolder(ctx context.Context, id string) (*catalogpkg.SavedQueryFolder, error)
	CreateSavedQueryFolder(ctx context.Context, f catalogpkg.SavedQueryFolder) error
	SoftDeleteSavedQueryFolder(ctx context.Context, id, actorID string) error

	// Saved queries
	ListSavedQueries(ctx context.Context, serviceDBID string, scope catalogpkg.SavedQueryScope, ownerUserID *string) ([]catalogpkg.SavedQuery, error)
	GetSavedQuery(ctx context.Context, id string) (*catalogpkg.SavedQuery, error)
	CreateSavedQuery(ctx context.Context, q catalogpkg.SavedQuery) error
	UpdateSavedQuery(ctx context.Context, q catalogpkg.SavedQuery) error
	SoftDeleteSavedQuery(ctx context.Context, id, actorID string) error
	UpsertSavedQueryBySourceRef(ctx context.Context, q catalogpkg.SavedQuery) (catalogpkg.SavedQuery, bool, error)

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
	queue   *queue.Queue // may be nil
	cache   cache.Client // may be nil
}

// New constructs a Handler.
func New(s store, st objectStore, q *queue.Queue, c cache.Client) *Handler {
	return &Handler{store: s, storage: st, queue: q, cache: c}
}

func (h *Handler) enqueueScreenshot(ctx context.Context, orgID, diagramID string) {
	if h.queue == nil {
		return
	}
	if err := h.queue.EnqueueScreenshot(ctx, queue.ScreenshotJob{OrgID: orgID, DiagramID: diagramID}); err != nil {
		slog.WarnContext(ctx, "enqueue screenshot job failed", "diagramId", diagramID, "err", err)
	}
}

// Register wires catalog routes into mux.
// requireScope signature: func(scope, method, pattern string, h http.HandlerFunc)
func Register(
	mux *http.ServeMux,
	s store,
	st objectStore,
	q *queue.Queue,
	c cache.Client,
	requireScope func(scope, method, pattern string, h http.HandlerFunc),
) {
	h := New(s, st, q, c)
	// By-id lookups (resolve a leaf id to its full record incl. parent ids)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/endpoints/{endpointID}", h.GetAPIEndpointByID)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/test-packs/{testPackID}", h.GetTestPackByID)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/service-docs/{docID}", h.GetServiceDocByID)
	// Services
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services", h.List)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/stats", h.ListStats)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services", h.Create)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}", h.Get)
	requireScope("services:write", "PUT", "/api/v1/orgs/{orgID}/services/{serviceID}", h.Update)
	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}", h.Delete)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/dependency-graph", h.GetDependencyGraph)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/dependencies/sync", h.SyncDependencies)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/dependencies", h.ListDependencies)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/dependency-graph", h.GetServiceDependencyGraph)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/impact", h.GetImpact)
	// API groups — /sync before /{apiGroupID}
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups", h.ListAPIGroups)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups", h.CreateAPIGroup)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/sync", h.SyncAPIGroup)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}", h.GetAPIGroup)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/spec", h.GetAPIGroupSpec)
	requireScope("services:write", "PUT", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}", h.UpdateAPIGroup)
	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}", h.DeleteAPIGroup)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/versions", h.ListAPIGroupVersions)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/versions", h.CreateAPIGroupVersion)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/versions/{versionID}/restore", h.RestoreAPIGroupVersion)
	// API endpoints
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints", h.ListAPIEndpoints)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints", h.CreateAPIEndpoint)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints/{endpointID}", h.GetAPIEndpoint)
	requireScope("services:write", "PUT", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints/{endpointID}", h.UpdateAPIEndpoint)
	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints/{endpointID}", h.DeleteAPIEndpoint)
	// Service docs
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/docs", h.ListDocs)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/docs", h.CreateDoc)
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
	// Saved queries
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}/query-folders", h.ListSavedQueryFolders)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}/query-folders", h.CreateSavedQueryFolder)
	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}/query-folders/{folderID}", h.DeleteSavedQueryFolder)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}/queries", h.ListSavedQueries)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}/queries", h.CreateSavedQuery)
	requireScope("services:write", "PUT", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}/queries/{queryID}", h.UpdateSavedQuery)
	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}/queries/{queryID}", h.DeleteSavedQuery)
	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}/queries/sync", h.SyncSavedQuery)
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
