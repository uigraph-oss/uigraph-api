package mlstudio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/uigraph/app/internal/identity"
	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/mlstudio"
)

type fakeStore struct {
	mlstudio.Store

	projects     []mlstudio.ProjectInput
	projectCount mlstudio.SyncCounts

	runPrune     []mlstudio.RunPruneInput
	runDeleted   int
	versionPrune mlstudio.VersionPruneInput
	versionCalls int
	verDeleted   int
}

func (f *fakeStore) UpsertMLProjects(_ context.Context, _, _ string, in []mlstudio.ProjectInput) (mlstudio.SyncCounts, error) {
	f.projects = in
	return f.projectCount, nil
}

func (f *fakeStore) PruneMLRuns(_ context.Context, _, _ string, in []mlstudio.RunPruneInput) (int, error) {
	f.runPrune = in
	return f.runDeleted, nil
}

func (f *fakeStore) PruneMLModelVersions(_ context.Context, _, _ string, in mlstudio.VersionPruneInput) (int, error) {
	f.versionPrune = in
	f.versionCalls++
	return f.verDeleted, nil
}

func call(t *testing.T, h http.HandlerFunc, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/v1/orgs/org1/ml/x", strings.NewReader(body))
	req.SetPathValue("orgID", "org1")
	req = req.WithContext(authmw.WithPrincipal(req.Context(), identity.Principal{
		Kind: identity.PrincipalUser, UserID: "user1", OrgID: "org1",
	}))
	rec := httptest.NewRecorder()
	h(rec, req)

	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestSyncProjects_ReportsCreatedAndUpdated(t *testing.T) {
	store := &fakeStore{projectCount: mlstudio.SyncCounts{Created: 1, Updated: 2}}
	h := New(store, nil)

	code, out := call(t, h.SyncProjects, `[{"name":"p1","type":"training"},{"name":"p2","type":"model"},{"name":"p3","type":"model"}]`)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%v)", code, out)
	}
	if out["synced"] != float64(3) || out["created"] != float64(1) || out["updated"] != float64(2) {
		t.Fatalf("response = %v, want synced 3 created 1 updated 2", out)
	}
	if len(store.projects) != 3 {
		t.Fatalf("store received %d projects, want 3", len(store.projects))
	}
}

func TestSyncProjects_RejectsUnknownType(t *testing.T) {
	h := New(&fakeStore{}, nil)

	code, _ := call(t, h.SyncProjects, `[{"name":"p1","type":"nonsense"}]`)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", code)
	}
}

func TestPruneRuns_ReportsDeleted(t *testing.T) {
	store := &fakeStore{runDeleted: 1}
	h := New(store, nil)

	code, out := call(t, h.PruneRuns, `[{"mlflowId":"run-abc"}]`)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%v)", code, out)
	}
	if out["deleted"] != float64(1) {
		t.Fatalf("response = %v, want deleted 1", out)
	}
	if len(store.runPrune) != 1 || store.runPrune[0].MLflowID != "run-abc" {
		t.Fatalf("store received %+v, want one run-abc", store.runPrune)
	}
}

func TestPruneRuns_RejectsMissingMLflowID(t *testing.T) {
	store := &fakeStore{}
	h := New(store, nil)

	code, _ := call(t, h.PruneRuns, `[{"mlflowId":""}]`)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", code)
	}
	if store.runPrune != nil {
		t.Fatalf("store was called with %+v, want no call", store.runPrune)
	}
}

func TestPruneVersions_ReportsDeleted(t *testing.T) {
	store := &fakeStore{verDeleted: 2}
	h := New(store, nil)

	code, out := call(t, h.PruneVersions, `{"modelMlflowId":"Saba","keep":["Saba/1","Saba/2"]}`)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%v)", code, out)
	}
	if out["deleted"] != float64(2) {
		t.Fatalf("response = %v, want deleted 2", out)
	}
	if store.versionPrune.ModelMLflowID != "Saba" || len(store.versionPrune.Keep) != 2 {
		t.Fatalf("store received %+v, want Saba with two kept versions", store.versionPrune)
	}
}

func TestPruneVersions_AcceptsEmptyKeep(t *testing.T) {
	store := &fakeStore{verDeleted: 5}
	h := New(store, nil)

	code, out := call(t, h.PruneVersions, `{"modelMlflowId":"Saba","keep":[]}`)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%v)", code, out)
	}
	if out["deleted"] != float64(5) {
		t.Fatalf("response = %v, want deleted 5", out)
	}
	if store.versionCalls != 1 {
		t.Fatalf("store called %d times, want 1 — an empty keep list must still reach the store", store.versionCalls)
	}
	if len(store.versionPrune.Keep) != 0 {
		t.Fatalf("keep = %+v, want empty", store.versionPrune.Keep)
	}
}

func TestPruneVersions_RejectsMissingModelID(t *testing.T) {
	store := &fakeStore{}
	h := New(store, nil)

	code, _ := call(t, h.PruneVersions, `{"keep":[]}`)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", code)
	}
	if store.versionCalls != 0 {
		t.Fatalf("store called %d times, want 0", store.versionCalls)
	}
}
