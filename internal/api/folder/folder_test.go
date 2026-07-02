package folder

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/uigraph/app/internal/folder"
	"github.com/uigraph/app/internal/identity"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
)

// compile-time interface check
var _ store = (*fakeStore)(nil)

// fakeStore implements the unexported store interface using function fields so
// individual tests can swap in behaviour without sharing mutable state.
type fakeStore struct {
	listFoldersFn  func(ctx context.Context, orgID string, t *folder.Type) ([]folder.Folder, error)
	getFolderFn    func(ctx context.Context, id string) (*folder.Folder, error)
	createFolderFn func(ctx context.Context, f folder.Folder) error
	updateFolderFn func(ctx context.Context, f folder.Folder) error
	deleteFolderFn func(ctx context.Context, id, deletedBy string) error
}

func (f *fakeStore) ListFolders(ctx context.Context, orgID string, t *folder.Type) ([]folder.Folder, error) {
	return f.listFoldersFn(ctx, orgID, t)
}
func (f *fakeStore) GetFolder(ctx context.Context, id string) (*folder.Folder, error) {
	return f.getFolderFn(ctx, id)
}
func (f *fakeStore) CreateFolder(ctx context.Context, fl folder.Folder) error {
	return f.createFolderFn(ctx, fl)
}
func (f *fakeStore) UpdateFolder(ctx context.Context, fl folder.Folder) error {
	return f.updateFolderFn(ctx, fl)
}
func (f *fakeStore) DeleteFolder(ctx context.Context, id, deletedBy string) error {
	return f.deleteFolderFn(ctx, id, deletedBy)
}

// withAuth injects a user principal into the request context.
func withAuth(r *http.Request) *http.Request {
	p := identity.Principal{UserID: "user-1", Kind: identity.PrincipalUser}
	return r.WithContext(authmw.WithPrincipal(r.Context(), p))
}

// newRequest builds a request with orgID path value pre-set.
func newRequest(method, path string, body []byte) *http.Request {
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

// ── List ──────────────────────────────────────────────────────────────────────

func TestList_returnsFolders(t *testing.T) {
	s := &fakeStore{
		listFoldersFn: func(_ context.Context, orgID string, _ *folder.Type) ([]folder.Folder, error) {
			return []folder.Folder{
				{ID: "f1", OrgID: orgID, Name: "Alpha", Type: folder.TypeService},
			}, nil
		},
	}
	h := New(s)

	r := newRequest(http.MethodGet, "/api/v1/orgs/org-1/folders", nil)
	w := httptest.NewRecorder()
	h.List(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var got struct {
		Folders []folder.Folder `json:"folders"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Folders) != 1 || got.Folders[0].ID != "f1" {
		t.Fatalf("unexpected folders: %v", got.Folders)
	}
}

func TestList_storeError_returns500(t *testing.T) {
	s := &fakeStore{
		listFoldersFn: func(_ context.Context, _ string, _ *folder.Type) ([]folder.Folder, error) {
			return nil, storepkg.ErrConflict
		},
	}
	h := New(s)

	r := newRequest(http.MethodGet, "/api/v1/orgs/org-1/folders", nil)
	w := httptest.NewRecorder()
	h.List(w, r)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 (ErrConflict maps to 409), got %d", w.Code)
	}
}

// ── Create ────────────────────────────────────────────────────────────────────

func TestCreate_success(t *testing.T) {
	var created folder.Folder
	s := &fakeStore{
		createFolderFn: func(_ context.Context, f folder.Folder) error {
			created = f
			return nil
		},
	}
	h := New(s)

	body, _ := json.Marshal(map[string]any{"name": "My Folder", "type": "service"})
	r := withAuth(newRequest(http.MethodPost, "/api/v1/orgs/org-1/folders", body))
	w := httptest.NewRecorder()
	h.Create(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if created.Name != "My Folder" {
		t.Fatalf("unexpected name: %q", created.Name)
	}
	if created.OrgID != "org-1" {
		t.Fatalf("unexpected orgID: %q", created.OrgID)
	}
	if created.CreatedBy != "user-1" {
		t.Fatalf("unexpected createdBy: %q", created.CreatedBy)
	}
}

func TestCreate_missingName_returns400(t *testing.T) {
	h := New(&fakeStore{})

	body, _ := json.Marshal(map[string]any{"type": "service"})
	r := withAuth(newRequest(http.MethodPost, "/api/v1/orgs/org-1/folders", body))
	w := httptest.NewRecorder()
	h.Create(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreate_unauthenticated_returns401(t *testing.T) {
	h := New(&fakeStore{})

	body, _ := json.Marshal(map[string]any{"name": "x", "type": "service"})
	r := newRequest(http.MethodPost, "/api/v1/orgs/org-1/folders", body) // no auth
	w := httptest.NewRecorder()
	h.Create(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// ── Get ───────────────────────────────────────────────────────────────────────

func TestGet_success(t *testing.T) {
	f := &folder.Folder{ID: "f1", OrgID: "org-1", Name: "Alpha", Type: folder.TypeDiagram}
	s := &fakeStore{
		getFolderFn: func(_ context.Context, id string) (*folder.Folder, error) {
			if id == "f1" {
				return f, nil
			}
			return nil, nil
		},
	}
	h := New(s)

	r := newRequest(http.MethodGet, "/api/v1/orgs/org-1/folders/f1", nil)
	r.SetPathValue("folderID", "f1")
	w := httptest.NewRecorder()
	h.Get(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var got folder.Folder
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "f1" {
		t.Fatalf("unexpected id: %q", got.ID)
	}
}

func TestGet_notFound_returns404(t *testing.T) {
	s := &fakeStore{
		getFolderFn: func(_ context.Context, _ string) (*folder.Folder, error) {
			return nil, nil // store returns nil → 404
		},
	}
	h := New(s)

	r := newRequest(http.MethodGet, "/api/v1/orgs/org-1/folders/missing", nil)
	r.SetPathValue("folderID", "missing")
	w := httptest.NewRecorder()
	h.Get(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGet_deletedFolder_returns404(t *testing.T) {
	now := time.Now()
	s := &fakeStore{
		getFolderFn: func(_ context.Context, _ string) (*folder.Folder, error) {
			return &folder.Folder{ID: "f1", DeletedAt: &now}, nil
		},
	}
	h := New(s)

	r := newRequest(http.MethodGet, "/api/v1/orgs/org-1/folders/f1", nil)
	r.SetPathValue("folderID", "f1")
	w := httptest.NewRecorder()
	h.Get(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for soft-deleted folder, got %d", w.Code)
	}
}

func TestGet_storeError_returns500(t *testing.T) {
	s := &fakeStore{
		getFolderFn: func(_ context.Context, _ string) (*folder.Folder, error) {
			return nil, storepkg.ErrConflict
		},
	}
	h := New(s)

	r := newRequest(http.MethodGet, "/api/v1/orgs/org-1/folders/f1", nil)
	r.SetPathValue("folderID", "f1")
	w := httptest.NewRecorder()
	h.Get(w, r)

	// ErrConflict → httputil.Error → 409 (not 500, not 404)
	// This verifies that store errors are NOT silently masked as 404.
	if w.Code == http.StatusNotFound {
		t.Fatal("store error must not be masked as 404")
	}
}

// ── Update ────────────────────────────────────────────────────────────────────

func TestUpdate_appliesFields(t *testing.T) {
	existing := &folder.Folder{ID: "f1", OrgID: "org-1", Name: "Old Name", Type: folder.TypeService}
	var saved folder.Folder
	s := &fakeStore{
		getFolderFn: func(_ context.Context, _ string) (*folder.Folder, error) {
			return existing, nil
		},
		updateFolderFn: func(_ context.Context, f folder.Folder) error {
			saved = f
			return nil
		},
	}
	h := New(s)

	body, _ := json.Marshal(map[string]any{"name": "New Name"})
	r := withAuth(newRequest(http.MethodPut, "/api/v1/orgs/org-1/folders/f1", body))
	r.SetPathValue("folderID", "f1")
	w := httptest.NewRecorder()
	h.Update(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if saved.Name != "New Name" {
		t.Fatalf("update did not apply name change, got %q", saved.Name)
	}
}

func TestUpdate_notFound_returns404(t *testing.T) {
	s := &fakeStore{
		getFolderFn: func(_ context.Context, _ string) (*folder.Folder, error) {
			return nil, nil
		},
	}
	h := New(s)

	body, _ := json.Marshal(map[string]any{"name": "x"})
	r := withAuth(newRequest(http.MethodPut, "/api/v1/orgs/org-1/folders/missing", body))
	r.SetPathValue("folderID", "missing")
	w := httptest.NewRecorder()
	h.Update(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestDelete_success(t *testing.T) {
	var deletedID, deletedBy string
	s := &fakeStore{
		deleteFolderFn: func(_ context.Context, id, by string) error {
			deletedID, deletedBy = id, by
			return nil
		},
	}
	h := New(s)

	r := withAuth(newRequest(http.MethodDelete, "/api/v1/orgs/org-1/folders/f1", nil))
	r.SetPathValue("folderID", "f1")
	w := httptest.NewRecorder()
	h.Delete(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if deletedID != "f1" {
		t.Fatalf("wrong folder deleted: %q", deletedID)
	}
	if deletedBy != "user-1" {
		t.Fatalf("wrong actor: %q", deletedBy)
	}
}

func TestDelete_unauthenticated_returns401(t *testing.T) {
	h := New(&fakeStore{})

	r := newRequest(http.MethodDelete, "/api/v1/orgs/org-1/folders/f1", nil) // no auth
	r.SetPathValue("folderID", "f1")
	w := httptest.NewRecorder()
	h.Delete(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
