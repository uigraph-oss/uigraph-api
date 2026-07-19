package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	catalogpkg "github.com/uigraph/app/internal/catalog"
	diagrampkg "github.com/uigraph/app/internal/diagram"
	docspkg "github.com/uigraph/app/internal/docs"
	"github.com/uigraph/app/internal/identity"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
)

var _ store = (*fakeDependencyStore)(nil)

type fakeDependencyStore struct {
	getSvcFn              func(ctx context.Context, id string) (*catalogpkg.Service, error)
	syncDepsFn            func(ctx context.Context, orgID, serviceID, actorID string, commitHash *string, dependencies []catalogpkg.ServiceDependency) error
	listDepsFn            func(ctx context.Context, orgID, serviceID, direction, criticality string) ([]catalogpkg.ServiceDependencyEdge, error)
	depGraphFn            func(ctx context.Context, orgID, serviceID string) ([]catalogpkg.ServiceDependencyEdge, error)
	impactFn              func(ctx context.Context, orgID, serviceID, direction string, maxDepth int) ([]catalogpkg.ServiceDependencyEdge, error)
}

func (f *fakeDependencyStore) GetService(ctx context.Context, id string) (*catalogpkg.Service, error) {
	return f.getSvcFn(ctx, id)
}

func (f *fakeDependencyStore) SyncServiceDependencies(ctx context.Context, orgID, serviceID, actorID string, commitHash *string, dependencies []catalogpkg.ServiceDependency) error {
	return f.syncDepsFn(ctx, orgID, serviceID, actorID, commitHash, dependencies)
}

func (f *fakeDependencyStore) ListServiceDependencies(ctx context.Context, orgID, serviceID, direction, criticality string) ([]catalogpkg.ServiceDependencyEdge, error) {
	return f.listDepsFn(ctx, orgID, serviceID, direction, criticality)
}

func (f *fakeDependencyStore) DependencyGraph(ctx context.Context, orgID, serviceID string) ([]catalogpkg.ServiceDependencyEdge, error) {
	return f.depGraphFn(ctx, orgID, serviceID)
}

func (f *fakeDependencyStore) Impact(ctx context.Context, orgID, serviceID, direction string, maxDepth int) ([]catalogpkg.ServiceDependencyEdge, error) {
	return f.impactFn(ctx, orgID, serviceID, direction, maxDepth)
}

var errNotMocked = storepkg.ErrNotFound

func (*fakeDependencyStore) ListServices(_ context.Context, _ string, _ catalogpkg.ListParams) ([]catalogpkg.Service, int, error) {
	return nil, 0, errNotMocked
}
func (*fakeDependencyStore) ListServiceStats(_ context.Context, _ string, _ *string) ([]catalogpkg.ServiceStats, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) CreateService(_ context.Context, _ catalogpkg.Service) error { return errNotMocked }
func (*fakeDependencyStore) UpdateService(_ context.Context, _ catalogpkg.Service) error { return errNotMocked }
func (*fakeDependencyStore) SoftDeleteService(_ context.Context, _, _ string) error     { return errNotMocked }
func (*fakeDependencyStore) ListServiceDocs(_ context.Context, _ string) ([]catalogpkg.ServiceDoc, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) GetServiceDoc(_ context.Context, _, _ string) (*catalogpkg.ServiceDoc, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) GetServiceDocByID(_ context.Context, _ string) (*catalogpkg.ServiceDoc, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) CreateServiceDoc(_ context.Context, _ catalogpkg.ServiceDoc) error { return errNotMocked }
func (*fakeDependencyStore) SoftDeleteServiceDoc(_ context.Context, _, _, _ string) error     { return errNotMocked }
func (*fakeDependencyStore) GetDoc(_ context.Context, _ string) (*docspkg.Doc, error)       { return nil, errNotMocked }
func (*fakeDependencyStore) CreateDoc(_ context.Context, _ docspkg.Doc) error                { return errNotMocked }
func (*fakeDependencyStore) ListServiceDiagrams(_ context.Context, _ string) ([]catalogpkg.ServiceDiagram, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) GetServiceDiagram(_ context.Context, _, _ string) (*catalogpkg.ServiceDiagram, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) CreateServiceDiagram(_ context.Context, _ catalogpkg.ServiceDiagram) error {
	return errNotMocked
}
func (*fakeDependencyStore) SoftDeleteServiceDiagram(_ context.Context, _, _, _ string) error { return errNotMocked }
func (*fakeDependencyStore) GetDiagram(_ context.Context, _ string) (*diagrampkg.Diagram, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) CreateDiagram(_ context.Context, _ diagrampkg.Diagram) error            { return errNotMocked }
func (*fakeDependencyStore) CreateDiagramVersion(_ context.Context, _ diagrampkg.Version) error      { return errNotMocked }
func (*fakeDependencyStore) ListAPIGroups(_ context.Context, _ string) ([]catalogpkg.APIGroup, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) GetAPIGroup(_ context.Context, _ string) (*catalogpkg.APIGroup, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) CreateAPIGroup(_ context.Context, _ catalogpkg.APIGroup) error  { return errNotMocked }
func (*fakeDependencyStore) UpdateAPIGroup(_ context.Context, _ catalogpkg.APIGroup) error  { return errNotMocked }
func (*fakeDependencyStore) SoftDeleteAPIGroup(_ context.Context, _, _ string) error         { return errNotMocked }
func (*fakeDependencyStore) ListAPIGroupVersions(_ context.Context, _ string) ([]catalogpkg.APIGroupVersion, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) GetAPIGroupVersion(_ context.Context, _ string) (*catalogpkg.APIGroupVersion, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) CreateAPIGroupVersion(_ context.Context, _ catalogpkg.APIGroupVersion) error {
	return errNotMocked
}
func (*fakeDependencyStore) LatestAPIGroupVersionNumber(_ context.Context, _ string) (int, error) {
	return 0, errNotMocked
}
func (*fakeDependencyStore) PublishAPIGroupVersion(_ context.Context, _ catalogpkg.PublishAPIGroupVersionInput) (catalogpkg.APIGroupVersion, error) {
	return catalogpkg.APIGroupVersion{}, errNotMocked
}
func (*fakeDependencyStore) ListAPIEndpoints(_ context.Context, _ string) ([]catalogpkg.APIEndpoint, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) ListAPIEndpointsForVersion(_ context.Context, _, _ string) ([]catalogpkg.APIEndpoint, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) GetAPIEndpoint(_ context.Context, _ string) (*catalogpkg.APIEndpoint, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) CreateAPIEndpoint(_ context.Context, _ catalogpkg.APIEndpoint) error {
	return errNotMocked
}
func (*fakeDependencyStore) UpdateAPIEndpoint(_ context.Context, _ catalogpkg.APIEndpoint) error {
	return errNotMocked
}
func (*fakeDependencyStore) SoftDeleteAPIEndpoint(_ context.Context, _, _ string) error   { return errNotMocked }
func (*fakeDependencyStore) SoftDeleteCurrentAPIEndpoints(_ context.Context, _, _ string) error {
	return errNotMocked
}
func (*fakeDependencyStore) CopyEndpointsForVersion(_ context.Context, _, _, _ string) error {
	return errNotMocked
}
func (*fakeDependencyStore) ListServiceDBs(_ context.Context, _ string) ([]catalogpkg.ServiceDB, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) GetServiceDB(_ context.Context, _ string) (*catalogpkg.ServiceDB, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) CreateServiceDB(_ context.Context, _ catalogpkg.ServiceDB) error {
	return errNotMocked
}
func (*fakeDependencyStore) UpdateServiceDB(_ context.Context, _ catalogpkg.ServiceDB) error {
	return errNotMocked
}
func (*fakeDependencyStore) SoftDeleteServiceDB(_ context.Context, _, _ string) error { return errNotMocked }
func (*fakeDependencyStore) ListServiceDBVersions(_ context.Context, _ string) ([]catalogpkg.ServiceDBVersion, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) GetServiceDBVersion(_ context.Context, _ string) (*catalogpkg.ServiceDBVersion, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) CreateServiceDBVersion(_ context.Context, _ catalogpkg.ServiceDBVersion) error {
	return errNotMocked
}
func (*fakeDependencyStore) LatestServiceDBVersionNumber(_ context.Context, _ string) (int, error) {
	return 0, errNotMocked
}
func (*fakeDependencyStore) ListSavedQueryFolders(_ context.Context, _ string, _ catalogpkg.SavedQueryScope, _ *string) ([]catalogpkg.SavedQueryFolder, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) GetSavedQueryFolder(_ context.Context, _ string) (*catalogpkg.SavedQueryFolder, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) CreateSavedQueryFolder(_ context.Context, _ catalogpkg.SavedQueryFolder) error {
	return errNotMocked
}
func (*fakeDependencyStore) SoftDeleteSavedQueryFolder(_ context.Context, _, _ string) error {
	return errNotMocked
}
func (*fakeDependencyStore) ListSavedQueries(_ context.Context, _ string, _ catalogpkg.SavedQueryScope, _ *string) ([]catalogpkg.SavedQuery, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) GetSavedQuery(_ context.Context, _ string) (*catalogpkg.SavedQuery, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) CreateSavedQuery(_ context.Context, _ catalogpkg.SavedQuery) error {
	return errNotMocked
}
func (*fakeDependencyStore) UpdateSavedQuery(_ context.Context, _ catalogpkg.SavedQuery) error {
	return errNotMocked
}
func (*fakeDependencyStore) SoftDeleteSavedQuery(_ context.Context, _, _ string) error   { return errNotMocked }
func (*fakeDependencyStore) UpsertSavedQueryBySourceRef(_ context.Context, _ catalogpkg.SavedQuery) (catalogpkg.SavedQuery, bool, error) {
	return catalogpkg.SavedQuery{}, false, errNotMocked
}
func (*fakeDependencyStore) ListTestPacks(_ context.Context, _ string) ([]catalogpkg.TestPack, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) GetTestPack(_ context.Context, _ string) (*catalogpkg.TestPack, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) CreateTestPack(_ context.Context, _ catalogpkg.TestPack) error { return errNotMocked }
func (*fakeDependencyStore) UpdateTestPack(_ context.Context, _ catalogpkg.TestPack) error { return errNotMocked }
func (*fakeDependencyStore) SoftDeleteTestPack(_ context.Context, _, _ string) error        { return errNotMocked }
func (*fakeDependencyStore) ListTestCases(_ context.Context, _ string, _ *string) ([]catalogpkg.TestCase, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) GetTestCase(_ context.Context, _ string) (*catalogpkg.TestCase, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) CreateTestCase(_ context.Context, _ catalogpkg.TestCase) error { return errNotMocked }
func (*fakeDependencyStore) UpdateTestCase(_ context.Context, _ catalogpkg.TestCase) error { return errNotMocked }
func (*fakeDependencyStore) SoftDeleteTestCase(_ context.Context, _, _ string) error        { return errNotMocked }
func (*fakeDependencyStore) CreateTestRun(_ context.Context, _ catalogpkg.TestRun) error    { return errNotMocked }
func (*fakeDependencyStore) GetTestRun(_ context.Context, _ string) (*catalogpkg.TestRun, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) ListTestRuns(_ context.Context, _ string, _ *string) ([]catalogpkg.TestRun, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) ListTestRunsSummary(_ context.Context, _ string, _ catalogpkg.TestRunSummaryFilter) ([]catalogpkg.TestRunSummary, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) UpdateTestRun(_ context.Context, _ catalogpkg.TestRun) error { return errNotMocked }
func (*fakeDependencyStore) CreateTestRunResult(_ context.Context, _ catalogpkg.TestRunResult) error {
	return errNotMocked
}
func (*fakeDependencyStore) GetTestRunResult(_ context.Context, _ string) (*catalogpkg.TestRunResult, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) ListTestRunResults(_ context.Context, _, _ string) ([]catalogpkg.TestRunResult, error) {
	return nil, errNotMocked
}
func (*fakeDependencyStore) UpdateTestRunResult(_ context.Context, _ catalogpkg.TestRunResult) error {
	return errNotMocked
}

func withDepAuth(r *http.Request) *http.Request {
	p := identity.Principal{UserID: "user-1", Kind: identity.PrincipalUser}
	return r.WithContext(authmw.WithPrincipal(r.Context(), p))
}

func depRequest(method, path string, body []byte) *http.Request {
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.SetPathValue("orgID", "org-1")
	return r
}

// ── SyncDependencies ───────────────────────────────────────────────────────

func TestSyncDependencies_success(t *testing.T) {
	s := &fakeDependencyStore{
		getSvcFn: func(_ context.Context, id string) (*catalogpkg.Service, error) {
			return &catalogpkg.Service{ID: id, OrgID: "org-1"}, nil
		},
		syncDepsFn: func(_ context.Context, orgID, serviceID, actorID string, commitHash *string, deps []catalogpkg.ServiceDependency) error {
			return nil
		},
	}
	h := New(s, nil, nil, nil)

	body := `{"dependencies":[{"name":"payments","service":"Stripe","type":"http","criticality":"hard","apiGroupName":"v1"}]}`
	r := withDepAuth(depRequest(http.MethodPost, "/api/v1/orgs/org-1/services/svc-1/dependencies/sync", []byte(body)))
	r.SetPathValue("serviceID", "svc-1")
	w := httptest.NewRecorder()
	h.SyncDependencies(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Dependencies []catalogpkg.ServiceDependency `json:"dependencies"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Dependencies) != 1 || resp.Dependencies[0].Name != "payments" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestSyncDependencies_missingName_returns400(t *testing.T) {
	s := &fakeDependencyStore{
		getSvcFn: func(_ context.Context, id string) (*catalogpkg.Service, error) {
			return &catalogpkg.Service{ID: id, OrgID: "org-1"}, nil
		},
	}
	h := New(s, nil, nil, nil)

	body := `{"dependencies":[{"service":"Stripe","type":"http","criticality":"hard"}]}`
	r := withDepAuth(depRequest(http.MethodPost, "/api/v1/orgs/org-1/services/svc-1/dependencies/sync", []byte(body)))
	r.SetPathValue("serviceID", "svc-1")
	w := httptest.NewRecorder()
	h.SyncDependencies(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSyncDependencies_duplicateName_returns400(t *testing.T) {
	s := &fakeDependencyStore{
		getSvcFn: func(_ context.Context, id string) (*catalogpkg.Service, error) {
			return &catalogpkg.Service{ID: id, OrgID: "org-1"}, nil
		},
	}
	h := New(s, nil, nil, nil)

	body := `{"dependencies":[{"name":"dup","service":"S1","type":"http","criticality":"hard","apiGroupName":"v1"},{"name":"dup","service":"S2","type":"database","criticality":"soft"}]}`
	r := withDepAuth(depRequest(http.MethodPost, "/api/v1/orgs/org-1/services/svc-1/dependencies/sync", []byte(body)))
	r.SetPathValue("serviceID", "svc-1")
	w := httptest.NewRecorder()
	h.SyncDependencies(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSyncDependencies_invalidType_returns400(t *testing.T) {
	s := &fakeDependencyStore{
		getSvcFn: func(_ context.Context, id string) (*catalogpkg.Service, error) {
			return &catalogpkg.Service{ID: id, OrgID: "org-1"}, nil
		},
	}
	h := New(s, nil, nil, nil)

	body := `{"dependencies":[{"name":"x","service":"Y","type":"rest","criticality":"hard"}]}`
	r := withDepAuth(depRequest(http.MethodPost, "/api/v1/orgs/org-1/services/svc-1/dependencies/sync", []byte(body)))
	r.SetPathValue("serviceID", "svc-1")
	w := httptest.NewRecorder()
	h.SyncDependencies(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ── ListDependencies ───────────────────────────────────────────────────────

func TestListDependencies_returnsEdges(t *testing.T) {
	s := &fakeDependencyStore{
		getSvcFn: func(_ context.Context, id string) (*catalogpkg.Service, error) {
			return &catalogpkg.Service{ID: id, OrgID: "org-1"}, nil
		},
		listDepsFn: func(_ context.Context, orgID, serviceID, direction, criticality string) ([]catalogpkg.ServiceDependencyEdge, error) {
		return []catalogpkg.ServiceDependencyEdge{
			{ServiceDependency: catalogpkg.ServiceDependency{Name: "payments", Type: "http"}, Direction: "upstream"},
		}, nil
	},
}
	h := New(s, nil, nil, nil)

	r := withDepAuth(depRequest(http.MethodGet, "/api/v1/orgs/org-1/services/svc-1/dependencies", nil))
	r.SetPathValue("serviceID", "svc-1")
	w := httptest.NewRecorder()
	h.ListDependencies(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Edges []catalogpkg.ServiceDependencyEdge `json:"edges"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Edges) != 1 || resp.Edges[0].Name != "payments" {
		t.Fatalf("unexpected edges: %+v", resp.Edges)
	}
}

func TestListDependencies_invalidDirection_returns400(t *testing.T) {
	s := &fakeDependencyStore{
		getSvcFn: func(_ context.Context, id string) (*catalogpkg.Service, error) {
			return &catalogpkg.Service{ID: id, OrgID: "org-1"}, nil
		},
	}
	h := New(s, nil, nil, nil)

	r := withDepAuth(depRequest(http.MethodGet, "/api/v1/orgs/org-1/services/svc-1/dependencies?direction=invalid", nil))
	r.SetPathValue("serviceID", "svc-1")
	w := httptest.NewRecorder()
	h.ListDependencies(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListDependencies_invalidCriticality_returns400(t *testing.T) {
	s := &fakeDependencyStore{
		getSvcFn: func(_ context.Context, id string) (*catalogpkg.Service, error) {
			return &catalogpkg.Service{ID: id, OrgID: "org-1"}, nil
		},
	}
	h := New(s, nil, nil, nil)

	r := withDepAuth(depRequest(http.MethodGet, "/api/v1/orgs/org-1/services/svc-1/dependencies?criticality=maybe", nil))
	r.SetPathValue("serviceID", "svc-1")
	w := httptest.NewRecorder()
	h.ListDependencies(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ── GetServiceDependencyGraph ──────────────────────────────────────────────

func TestGetServiceDependencyGraph_returnsGraph(t *testing.T) {
	s := &fakeDependencyStore{
		getSvcFn: func(_ context.Context, id string) (*catalogpkg.Service, error) {
			return &catalogpkg.Service{ID: id, OrgID: "org-1"}, nil
		},
		depGraphFn: func(_ context.Context, orgID, serviceID string) ([]catalogpkg.ServiceDependencyEdge, error) {
			return []catalogpkg.ServiceDependencyEdge{
				{ServiceDependency: catalogpkg.ServiceDependency{Name: "payments", Type: "http"}},
			}, nil
		},
	}
	h := New(s, nil, nil, nil)

	r := withDepAuth(depRequest(http.MethodGet, "/api/v1/orgs/org-1/services/svc-1/dependency-graph", nil))
	r.SetPathValue("serviceID", "svc-1")
	w := httptest.NewRecorder()
	h.GetServiceDependencyGraph(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Edges []catalogpkg.ServiceDependencyEdge `json:"edges"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Edges) != 1 || resp.Edges[0].Name != "payments" {
		t.Fatalf("unexpected edges: %+v", resp)
	}
}

// ── GetDependencyGraph ─────────────────────────────────────────────────────

func TestGetDependencyGraph_orgWide_returnsGraph(t *testing.T) {
	s := &fakeDependencyStore{
		getSvcFn: func(_ context.Context, id string) (*catalogpkg.Service, error) {
			return &catalogpkg.Service{ID: id, OrgID: "org-1"}, nil
		},
		depGraphFn: func(_ context.Context, orgID, serviceID string) ([]catalogpkg.ServiceDependencyEdge, error) {
			return []catalogpkg.ServiceDependencyEdge{
				{ServiceDependency: catalogpkg.ServiceDependency{Name: "dep-a", Type: "http"}},
				{ServiceDependency: catalogpkg.ServiceDependency{Name: "dep-b", Type: "http"}},
			}, nil
		},
	}
	h := New(s, nil, nil, nil)

	r := withDepAuth(depRequest(http.MethodGet, "/api/v1/orgs/org-1/dependency-graph", nil))
	w := httptest.NewRecorder()
	h.GetDependencyGraph(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Edges []catalogpkg.ServiceDependencyEdge `json:"edges"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(resp.Edges))
	}
}

// ── GetImpact ──────────────────────────────────────────────────────────────

func TestGetImpact_returnsGraph(t *testing.T) {
	s := &fakeDependencyStore{
		getSvcFn: func(_ context.Context, id string) (*catalogpkg.Service, error) {
			return &catalogpkg.Service{ID: id, OrgID: "org-1"}, nil
		},
		impactFn: func(_ context.Context, orgID, serviceID, direction string, maxDepth int) ([]catalogpkg.ServiceDependencyEdge, error) {
			return []catalogpkg.ServiceDependencyEdge{
				{ServiceDependency: catalogpkg.ServiceDependency{Name: "dep", Type: "http"}, Direction: "downstream"},
			}, nil
		},
	}
	h := New(s, nil, nil, nil)

	r := withDepAuth(depRequest(http.MethodGet, "/api/v1/orgs/org-1/services/svc-1/impact?direction=downstream&maxDepth=5", nil))
	r.SetPathValue("serviceID", "svc-1")
	w := httptest.NewRecorder()
	h.GetImpact(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Edges []catalogpkg.ServiceDependencyEdge `json:"edges"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Edges) != 1 || resp.Edges[0].Name != "dep" {
		t.Fatalf("unexpected edges: %+v", resp)
	}
}

func TestGetImpact_missingDirection_returns400(t *testing.T) {
	s := &fakeDependencyStore{
		getSvcFn: func(_ context.Context, id string) (*catalogpkg.Service, error) {
			return &catalogpkg.Service{ID: id, OrgID: "org-1"}, nil
		},
	}
	h := New(s, nil, nil, nil)

	r := withDepAuth(depRequest(http.MethodGet, "/api/v1/orgs/org-1/services/svc-1/impact", nil))
	r.SetPathValue("serviceID", "svc-1")
	w := httptest.NewRecorder()
	h.GetImpact(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetImpact_invalidMaxDepth_returns400(t *testing.T) {
	s := &fakeDependencyStore{
		getSvcFn: func(_ context.Context, id string) (*catalogpkg.Service, error) {
			return &catalogpkg.Service{ID: id, OrgID: "org-1"}, nil
		},
	}
	h := New(s, nil, nil, nil)

	r := withDepAuth(depRequest(http.MethodGet, "/api/v1/orgs/org-1/services/svc-1/impact?direction=downstream&maxDepth=0", nil))
	r.SetPathValue("serviceID", "svc-1")
	w := httptest.NewRecorder()
	h.GetImpact(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetImpact_maxDepthTooHigh_returns400(t *testing.T) {
	s := &fakeDependencyStore{
		getSvcFn: func(_ context.Context, id string) (*catalogpkg.Service, error) {
			return &catalogpkg.Service{ID: id, OrgID: "org-1"}, nil
		},
	}
	h := New(s, nil, nil, nil)

	r := withDepAuth(depRequest(http.MethodGet, "/api/v1/orgs/org-1/services/svc-1/impact?direction=upstream&maxDepth=101", nil))
	r.SetPathValue("serviceID", "svc-1")
	w := httptest.NewRecorder()
	h.GetImpact(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ── Unused store methods (compilation safety) ──────────────────────────────

type nopReadCloser struct{}

func (nopReadCloser) Read([]byte) (int, error) { return 0, io.EOF }
func (nopReadCloser) Close() error              { return nil }

type fakeObjectStore struct{}
func (*fakeObjectStore) Upload(_ context.Context, _, _ string, _ io.Reader, _ int64) error { return nil }
func (*fakeObjectStore) Download(_ context.Context, _ string) (io.ReadCloser, error)        { return nopReadCloser{}, nil }
func (*fakeObjectStore) Delete(_ context.Context, _ string) error                           { return nil }
