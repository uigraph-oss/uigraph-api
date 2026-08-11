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
}

func (f *fakeStore) UpsertMLProjects(_ context.Context, _, _ string, in []mlstudio.ProjectInput) (mlstudio.SyncCounts, error) {
	f.projects = in
	return f.projectCount, nil
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
	store := &fakeStore{projectCount: mlstudio.SyncCounts{Created: 1, Updated: 1}}
	h := New(store, nil)

	code, out := call(t, h.SyncProjects, `[{"name":"p1","type":"training"},{"name":"p2","type":"model"},{"name":"p3","type":"model"}]`)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%v)", code, out)
	}
	if out["synced"] != float64(3) || out["created"] != float64(1) || out["updated"] != float64(1) {
		t.Fatalf("response = %v, want synced 3 created 1 updated 1", out)
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
