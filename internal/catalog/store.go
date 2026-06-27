// Package catalog defines domain types for the Service Catalog:
// Services, API Groups (with versioning), and API Endpoints.
package catalog

import "context"

type ListParams struct {
	FolderID *string
	TeamID   *string
	Search   *string
	SortBy   string
	SortDir  string
	Limit    int
	Offset   int
}

// Store is the persistence interface for the service catalog.
type Store interface {
	// Services
	CreateService(ctx context.Context, s Service) error
	GetService(ctx context.Context, id string) (*Service, error)
	GetServiceBySlug(ctx context.Context, orgID, slug string) (*Service, error)
	ListServices(ctx context.Context, orgID string, p ListParams) ([]Service, int, error)
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
	PublishAPIGroupVersion(ctx context.Context, in PublishAPIGroupVersionInput) (APIGroupVersion, error)

	// API endpoints
	CreateAPIEndpoint(ctx context.Context, e APIEndpoint) error
	GetAPIEndpoint(ctx context.Context, id string) (*APIEndpoint, error)
	ListAPIEndpoints(ctx context.Context, apiGroupID string) ([]APIEndpoint, error)
	ListAPIEndpointsForVersion(ctx context.Context, apiGroupID, versionID string) ([]APIEndpoint, error)
	UpdateAPIEndpoint(ctx context.Context, e APIEndpoint) error
	SoftDeleteAPIEndpoint(ctx context.Context, id, deletedBy string) error
	SoftDeleteCurrentAPIEndpoints(ctx context.Context, apiGroupID, deletedBy string) error
	CopyEndpointsForVersion(ctx context.Context, apiGroupID, versionID, actorID string) error

	// Service docs
	CreateServiceDoc(ctx context.Context, d ServiceDoc) error
	GetServiceDoc(ctx context.Context, serviceID, docID string) (*ServiceDoc, error)
	GetServiceDocByID(ctx context.Context, docID string) (*ServiceDoc, error)
	ListServiceDocs(ctx context.Context, serviceID string) ([]ServiceDoc, error)
	SoftDeleteServiceDoc(ctx context.Context, serviceID, docID, deletedBy string) error

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
