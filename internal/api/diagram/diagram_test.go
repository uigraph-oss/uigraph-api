package diagram

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	diagrampkg "github.com/uigraph/app/internal/diagram"
	"github.com/uigraph/app/internal/identity"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
)

// compile-time interface checks
var _ store = (*fakeDiagramStore)(nil)
var _ objectStore = (*fakeObjectStore)(nil)

// fakeDiagramStore implements the unexported store interface.
type fakeDiagramStore struct {
	listDiagramsFn      func(ctx context.Context, orgID string, p diagrampkg.ListParams) ([]diagrampkg.Diagram, int, error)
	getDiagramFn        func(ctx context.Context, id string) (*diagrampkg.Diagram, error)
	createDiagramFn     func(ctx context.Context, d diagrampkg.Diagram) error
	updateDiagramFn     func(ctx context.Context, d diagrampkg.Diagram) error
	softDeleteDiagramFn func(ctx context.Context, id, deletedBy string) error
	listVersionsFn      func(ctx context.Context, diagramID string) ([]diagrampkg.Version, error)
	getVersionFn        func(ctx context.Context, id string) (*diagrampkg.Version, error)
	createVersionFn     func(ctx context.Context, v diagrampkg.Version) error
	latestVersionFn     func(ctx context.Context, diagramID string) (int, error)
	listImagesFn        func(ctx context.Context, diagramID string) ([]diagrampkg.Image, error)
	createImageFn       func(ctx context.Context, img diagrampkg.Image) error
}

func (f *fakeDiagramStore) ListDiagrams(ctx context.Context, orgID string, p diagrampkg.ListParams) ([]diagrampkg.Diagram, int, error) {
	return f.listDiagramsFn(ctx, orgID, p)
}
func (f *fakeDiagramStore) GetDiagram(ctx context.Context, id string) (*diagrampkg.Diagram, error) {
	return f.getDiagramFn(ctx, id)
}
func (f *fakeDiagramStore) CreateDiagram(ctx context.Context, d diagrampkg.Diagram) error {
	return f.createDiagramFn(ctx, d)
}
func (f *fakeDiagramStore) UpdateDiagram(ctx context.Context, d diagrampkg.Diagram) error {
	return f.updateDiagramFn(ctx, d)
}
func (f *fakeDiagramStore) SetDiagramPreviewStatus(ctx context.Context, id, status string) error {
	return nil
}
func (f *fakeDiagramStore) SoftDeleteDiagram(ctx context.Context, id, deletedBy string) error {
	return f.softDeleteDiagramFn(ctx, id, deletedBy)
}
func (f *fakeDiagramStore) ListDiagramVersions(ctx context.Context, diagramID string) ([]diagrampkg.Version, error) {
	return f.listVersionsFn(ctx, diagramID)
}
func (f *fakeDiagramStore) GetDiagramVersion(ctx context.Context, id string) (*diagrampkg.Version, error) {
	return f.getVersionFn(ctx, id)
}
func (f *fakeDiagramStore) CreateDiagramVersion(ctx context.Context, v diagrampkg.Version) error {
	return f.createVersionFn(ctx, v)
}
func (f *fakeDiagramStore) LatestVersionNumber(ctx context.Context, diagramID string) (int, error) {
	return f.latestVersionFn(ctx, diagramID)
}
func (f *fakeDiagramStore) ListDiagramImages(ctx context.Context, diagramID string) ([]diagrampkg.Image, error) {
	return f.listImagesFn(ctx, diagramID)
}
func (f *fakeDiagramStore) CreateDiagramImage(ctx context.Context, img diagrampkg.Image) error {
	return f.createImageFn(ctx, img)
}

// fakeObjectStore implements the unexported objectStore interface.
type fakeObjectStore struct {
	uploadFn        func(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	downloadFn      func(ctx context.Context, key string) (io.ReadCloser, error)
	presignPutURLFn func(ctx context.Context, key string) (string, error)
}

func (f *fakeObjectStore) Upload(ctx context.Context, key, contentType string, body io.Reader, size int64) error {
	if f.uploadFn != nil {
		return f.uploadFn(ctx, key, contentType, body, size)
	}
	return nil
}
func (f *fakeObjectStore) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	if f.downloadFn != nil {
		return f.downloadFn(ctx, key)
	}
	return nil, nil
}
func (f *fakeObjectStore) PresignPutURL(ctx context.Context, key string) (string, error) {
	if f.presignPutURLFn != nil {
		return f.presignPutURLFn(ctx, key)
	}
	return "https://presigned.example.com/" + key, nil
}

// withAuth injects a user principal into the request context.
func withAuth(r *http.Request) *http.Request {
	p := identity.Principal{UserID: "user-1", Kind: identity.PrincipalUser}
	return r.WithContext(authmw.WithPrincipal(r.Context(), p))
}

func newReq(method, path string, body []byte) *http.Request {
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

func TestList_returnsDiagrams(t *testing.T) {
	s := &fakeDiagramStore{
		listDiagramsFn: func(_ context.Context, orgID string, _ diagrampkg.ListParams) ([]diagrampkg.Diagram, int, error) {
			return []diagrampkg.Diagram{{ID: "d1", OrgID: orgID, Name: "Flow"}}, 1, nil
		},
	}
	h := New(s, nil, nil, nil)

	r := newReq(http.MethodGet, "/api/v1/orgs/org-1/diagrams", nil)
	w := httptest.NewRecorder()
	h.List(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var got struct {
		Diagrams []diagrampkg.Diagram `json:"diagrams"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Diagrams) != 1 || got.Diagrams[0].ID != "d1" {
		t.Fatalf("unexpected diagrams: %v", got.Diagrams)
	}
}

func TestList_propagatesFolderIDFilter(t *testing.T) {
	var capturedFolderID *string
	s := &fakeDiagramStore{
		listDiagramsFn: func(_ context.Context, _ string, p diagrampkg.ListParams) ([]diagrampkg.Diagram, int, error) {
			capturedFolderID = p.FolderID
			return nil, 0, nil
		},
	}
	h := New(s, nil, nil, nil)

	r := newReq(http.MethodGet, "/api/v1/orgs/org-1/diagrams?folderId=folder-42", nil)
	w := httptest.NewRecorder()
	h.List(w, r)

	if capturedFolderID == nil || *capturedFolderID != "folder-42" {
		t.Fatalf("folderId query param not propagated, got: %v", capturedFolderID)
	}
}

func TestList_storeError_returns500(t *testing.T) {
	s := &fakeDiagramStore{
		listDiagramsFn: func(_ context.Context, _ string, _ diagrampkg.ListParams) ([]diagrampkg.Diagram, int, error) {
			return nil, 0, storepkg.ErrConflict
		},
	}
	h := New(s, nil, nil, nil)

	r := newReq(http.MethodGet, "/api/v1/orgs/org-1/diagrams", nil)
	w := httptest.NewRecorder()
	h.List(w, r)

	if w.Code == http.StatusOK {
		t.Fatal("should not return 200 on store error")
	}
}

// ── Get ───────────────────────────────────────────────────────────────────────

func TestGet_success(t *testing.T) {
	dg := &diagrampkg.Diagram{ID: "d1", OrgID: "org-1", Name: "Arch"}
	s := &fakeDiagramStore{
		getDiagramFn: func(_ context.Context, id string) (*diagrampkg.Diagram, error) {
			if id == "d1" {
				return dg, nil
			}
			return nil, nil
		},
	}
	h := New(s, nil, nil, nil)

	r := newReq(http.MethodGet, "/api/v1/orgs/org-1/diagrams/d1", nil)
	r.SetPathValue("diagramID", "d1")
	w := httptest.NewRecorder()
	h.Get(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var got diagrampkg.Diagram
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "d1" {
		t.Fatalf("unexpected id: %q", got.ID)
	}
}

func TestGet_notFound_returns404(t *testing.T) {
	s := &fakeDiagramStore{
		getDiagramFn: func(_ context.Context, _ string) (*diagrampkg.Diagram, error) {
			return nil, nil
		},
	}
	h := New(s, nil, nil, nil)

	r := newReq(http.MethodGet, "/api/v1/orgs/org-1/diagrams/missing", nil)
	r.SetPathValue("diagramID", "missing")
	w := httptest.NewRecorder()
	h.Get(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGet_softDeleted_returns404(t *testing.T) {
	now := time.Now()
	s := &fakeDiagramStore{
		getDiagramFn: func(_ context.Context, _ string) (*diagrampkg.Diagram, error) {
			return &diagrampkg.Diagram{ID: "d1", DeletedAt: &now}, nil
		},
	}
	h := New(s, nil, nil, nil)

	r := newReq(http.MethodGet, "/api/v1/orgs/org-1/diagrams/d1", nil)
	r.SetPathValue("diagramID", "d1")
	w := httptest.NewRecorder()
	h.Get(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for deleted diagram, got %d", w.Code)
	}
}

func TestGet_storeErrorNotMaskedAs404(t *testing.T) {
	s := &fakeDiagramStore{
		getDiagramFn: func(_ context.Context, _ string) (*diagrampkg.Diagram, error) {
			return nil, storepkg.ErrConflict
		},
	}
	h := New(s, nil, nil, nil)

	r := newReq(http.MethodGet, "/api/v1/orgs/org-1/diagrams/d1", nil)
	r.SetPathValue("diagramID", "d1")
	w := httptest.NewRecorder()
	h.Get(w, r)

	if w.Code == http.StatusNotFound {
		t.Fatal("store error must not be masked as 404 — split checks rule violated")
	}
}

// ── Create ────────────────────────────────────────────────────────────────────

func TestCreate_missingNameOrContent_returns400(t *testing.T) {
	cases := []struct {
		name string
		body map[string]any
	}{
		{"missing name", map[string]any{"content": "{}"}},
		{"missing content", map[string]any{"name": "Arch"}},
		{"both empty", map[string]any{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := New(&fakeDiagramStore{}, nil, nil, nil)

			body, _ := json.Marshal(tc.body)
			r := withAuth(newReq(http.MethodPost, "/api/v1/orgs/org-1/diagrams", body))
			w := httptest.NewRecorder()
			h.Create(w, r)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", w.Code)
			}
		})
	}
}

func TestCreate_unauthenticated_returns401(t *testing.T) {
	h := New(&fakeDiagramStore{}, nil, nil, nil)

	body, _ := json.Marshal(map[string]any{"name": "x", "content": "{}"})
	r := newReq(http.MethodPost, "/api/v1/orgs/org-1/diagrams", body)
	w := httptest.NewRecorder()
	h.Create(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestCreate_success(t *testing.T) {
	var createdDiagram diagrampkg.Diagram
	st := &fakeObjectStore{
		uploadFn: func(_ context.Context, _, _ string, _ io.Reader, _ int64) error {
			return nil
		},
	}
	s := &fakeDiagramStore{
		createDiagramFn: func(_ context.Context, d diagrampkg.Diagram) error {
			createdDiagram = d
			return nil
		},
		createVersionFn: func(_ context.Context, _ diagrampkg.Version) error {
			return nil
		},
	}
	h := New(s, st, nil, nil)

	body, _ := json.Marshal(map[string]any{"name": "Architecture", "content": `{"nodes":[]}`})
	r := withAuth(newReq(http.MethodPost, "/api/v1/orgs/org-1/diagrams", body))
	w := httptest.NewRecorder()
	h.Create(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if createdDiagram.Name != "Architecture" {
		t.Fatalf("unexpected name: %q", createdDiagram.Name)
	}
	if createdDiagram.OrgID != "org-1" {
		t.Fatalf("unexpected orgID: %q", createdDiagram.OrgID)
	}
	if createdDiagram.CreatedBy != "user-1" {
		t.Fatalf("unexpected createdBy: %q", createdDiagram.CreatedBy)
	}
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestDelete_success(t *testing.T) {
	var deletedID, deletedBy string
	s := &fakeDiagramStore{
		softDeleteDiagramFn: func(_ context.Context, id, by string) error {
			deletedID, deletedBy = id, by
			return nil
		},
	}
	h := New(s, nil, nil, nil)

	r := withAuth(newReq(http.MethodDelete, "/api/v1/orgs/org-1/diagrams/d1", nil))
	r.SetPathValue("diagramID", "d1")
	w := httptest.NewRecorder()
	h.Delete(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if deletedID != "d1" {
		t.Fatalf("wrong diagram deleted: %q", deletedID)
	}
	if deletedBy != "user-1" {
		t.Fatalf("wrong actor: %q", deletedBy)
	}
}

func TestDelete_unauthenticated_returns401(t *testing.T) {
	h := New(&fakeDiagramStore{}, nil, nil, nil)

	r := newReq(http.MethodDelete, "/api/v1/orgs/org-1/diagrams/d1", nil)
	r.SetPathValue("diagramID", "d1")
	w := httptest.NewRecorder()
	h.Delete(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// ── PrepareThumbnailUpload ────────────────────────────────────────────────────

func TestPrepareThumbnailUpload_success(t *testing.T) {
	dg := &diagrampkg.Diagram{ID: "d1", OrgID: "org-1", Name: "Flow"}
	s := &fakeDiagramStore{
		getDiagramFn: func(_ context.Context, id string) (*diagrampkg.Diagram, error) {
			if id == "d1" {
				return dg, nil
			}
			return nil, nil
		},
	}
	st := &fakeObjectStore{
		presignPutURLFn: func(_ context.Context, key string) (string, error) {
			return "https://storage.example.com/put/" + key, nil
		},
	}
	h := New(s, st, nil, nil)

	r := withAuth(newReq(http.MethodPost, "/api/v1/orgs/org-1/diagrams/d1/thumbnail/prepare", nil))
	r.SetPathValue("diagramID", "d1")
	w := httptest.NewRecorder()
	h.PrepareThumbnailUpload(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got map[string]string
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["uploadUrl"] == "" {
		t.Fatal("expected uploadUrl in response")
	}
	if got["assetId"] == "" {
		t.Fatal("expected assetId in response")
	}
}

func TestPrepareThumbnailUpload_unauthenticated_returns401(t *testing.T) {
	h := New(&fakeDiagramStore{}, &fakeObjectStore{}, nil, nil)

	r := newReq(http.MethodPost, "/api/v1/orgs/org-1/diagrams/d1/thumbnail/prepare", nil)
	r.SetPathValue("diagramID", "d1")
	w := httptest.NewRecorder()
	h.PrepareThumbnailUpload(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestPrepareThumbnailUpload_notFound_returns404(t *testing.T) {
	s := &fakeDiagramStore{
		getDiagramFn: func(_ context.Context, _ string) (*diagrampkg.Diagram, error) {
			return nil, nil
		},
	}
	h := New(s, &fakeObjectStore{}, nil, nil)

	r := withAuth(newReq(http.MethodPost, "/api/v1/orgs/org-1/diagrams/missing/thumbnail/prepare", nil))
	r.SetPathValue("diagramID", "missing")
	w := httptest.NewRecorder()
	h.PrepareThumbnailUpload(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// ── ConfirmThumbnailUpload ────────────────────────────────────────────────────

func TestConfirmThumbnailUpload_success(t *testing.T) {
	dg := &diagrampkg.Diagram{ID: "d1", OrgID: "org-1", Name: "Flow"}
	var updatedDiagram diagrampkg.Diagram
	s := &fakeDiagramStore{
		getDiagramFn: func(_ context.Context, id string) (*diagrampkg.Diagram, error) {
			if id == "d1" {
				return dg, nil
			}
			return nil, nil
		},
		updateDiagramFn: func(_ context.Context, d diagrampkg.Diagram) error {
			updatedDiagram = d
			return nil
		},
	}
	h := New(s, &fakeObjectStore{}, nil, nil)

	body, _ := json.Marshal(map[string]any{"contentHash": "abc123"})
	r := withAuth(newReq(http.MethodPost, "/api/v1/orgs/org-1/diagrams/d1/thumbnail/confirm", body))
	r.SetPathValue("diagramID", "d1")
	w := httptest.NewRecorder()
	h.ConfirmThumbnailUpload(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if updatedDiagram.PreviewContentHash == nil || *updatedDiagram.PreviewContentHash != "abc123" {
		t.Fatalf("expected PreviewContentHash=abc123, got %v", updatedDiagram.PreviewContentHash)
	}
	if updatedDiagram.PreviewAssetID == nil {
		t.Fatal("expected PreviewAssetID to be set")
	}
}

func TestConfirmThumbnailUpload_missingHash_returns400(t *testing.T) {
	dg := &diagrampkg.Diagram{ID: "d1", OrgID: "org-1"}
	s := &fakeDiagramStore{
		getDiagramFn: func(_ context.Context, _ string) (*diagrampkg.Diagram, error) {
			return dg, nil
		},
	}
	h := New(s, &fakeObjectStore{}, nil, nil)

	body, _ := json.Marshal(map[string]any{"contentHash": ""})
	r := withAuth(newReq(http.MethodPost, "/api/v1/orgs/org-1/diagrams/d1/thumbnail/confirm", body))
	r.SetPathValue("diagramID", "d1")
	w := httptest.NewRecorder()
	h.ConfirmThumbnailUpload(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestConfirmThumbnailUpload_unauthenticated_returns401(t *testing.T) {
	h := New(&fakeDiagramStore{}, &fakeObjectStore{}, nil, nil)

	body, _ := json.Marshal(map[string]any{"contentHash": "abc"})
	r := newReq(http.MethodPost, "/api/v1/orgs/org-1/diagrams/d1/thumbnail/confirm", body)
	r.SetPathValue("diagramID", "d1")
	w := httptest.NewRecorder()
	h.ConfirmThumbnailUpload(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
