package catalog

import (
	"context"
	"io"
	"strings"
	"testing"

	catalogpkg "github.com/uigraph/app/internal/catalog"
)

// fakePublishStore embeds the catalog store interface (so it satisfies every
// method) but only implements PublishAPIGroupVersion. Any other call panics,
// which keeps the test honest about what publishAPIGroupVersion touches.
type fakePublishStore struct {
	store
	got *catalogpkg.PublishAPIGroupVersionInput
	ret catalogpkg.APIGroupVersion
	err error
}

func (f *fakePublishStore) PublishAPIGroupVersion(_ context.Context, in catalogpkg.PublishAPIGroupVersionInput) (catalogpkg.APIGroupVersion, error) {
	f.got = &in
	if f.err != nil {
		return catalogpkg.APIGroupVersion{}, f.err
	}
	return f.ret, nil
}

// fakeStorage is an in-memory objectStore.
type fakeStorage struct {
	objects map[string]string
}

func newFakeStorage() *fakeStorage { return &fakeStorage{objects: map[string]string{}} }

func (s *fakeStorage) Upload(_ context.Context, key, _ string, body io.Reader, _ int64) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.objects[key] = string(b)
	return nil
}

func (s *fakeStorage) Download(_ context.Context, key string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(s.objects[key])), nil
}

func (s *fakeStorage) Delete(_ context.Context, key string) error {
	delete(s.objects, key)
	return nil
}

const testOpenAPISpec = `{"openapi":"3.0.0","paths":{"/ping":{"get":{"operationId":"ping"}}}}`

func newGroup() *catalogpkg.APIGroup {
	return &catalogpkg.APIGroup{
		ID: "group-1", ServiceID: "svc-1", OrgID: "org-1",
		Name: "Orders", Version: "v1", Protocol: "REST",
	}
}

func TestPublishAPIGroupVersion_snapshotsIsolatedSpecAndEndpoints(t *testing.T) {
	label := "v2"
	fs := &fakePublishStore{ret: catalogpkg.APIGroupVersion{VersionNumber: 2, Label: &label}}
	st := newFakeStorage()
	h := &Handler{store: fs, storage: st}

	g := newGroup()
	v, err := h.publishAPIGroupVersion(context.Background(), publishParams{
		group: g, serviceID: "svc-1", spec: testOpenAPISpec,
		label: nil, isAuto: true, actorID: "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fs.got == nil {
		t.Fatal("store.PublishAPIGroupVersion was never called")
	}
	if !fs.got.ReplaceEndpoints {
		t.Fatal("expected ReplaceEndpoints=true when importing a new spec")
	}
	if len(fs.got.NewEndpoints) != 1 {
		t.Fatalf("expected 1 parsed endpoint, got %d", len(fs.got.NewEndpoints))
	}

	// The version spec must land at an isolated, version-scoped key — never the
	// working-copy key — so versions stay immutable.
	if !strings.Contains(fs.got.Version.SpecKey, "/versions/") {
		t.Fatalf("version spec key is not version-scoped: %q", fs.got.Version.SpecKey)
	}
	if fs.got.Version.SpecHash != specHash(testOpenAPISpec) {
		t.Fatal("version spec hash does not match content")
	}
	if _, ok := st.objects[fs.got.Version.SpecKey]; !ok {
		t.Fatal("version spec blob was not uploaded")
	}

	// The working copy is updated and points at the working-copy key (distinct).
	if g.SpecKey == nil || strings.Contains(*g.SpecKey, "/versions/") {
		t.Fatalf("working-copy spec key is wrong: %v", g.SpecKey)
	}
	if *g.SpecKey == fs.got.Version.SpecKey {
		t.Fatal("working-copy and version spec keys must differ")
	}

	// The resolved label is reflected on both the version and the group.
	if v.Label == nil || *v.Label != "v2" {
		t.Fatalf("expected version label v2, got %v", v.Label)
	}
	if g.Version != "v2" {
		t.Fatalf("expected group version to advance to v2, got %q", g.Version)
	}
}

func TestPublishAPIGroupVersion_surfacesStoreError(t *testing.T) {
	fs := &fakePublishStore{err: io.ErrUnexpectedEOF}
	h := &Handler{store: fs, storage: newFakeStorage()}

	g := newGroup()
	if _, err := h.publishAPIGroupVersion(context.Background(), publishParams{
		group: g, serviceID: "svc-1", spec: testOpenAPISpec, isAuto: true, actorID: "user-1",
	}); err == nil {
		t.Fatal("expected the store error to surface, got nil")
	}
	// A failed publish must not advance the working-copy version label.
	if g.Version != "v1" {
		t.Fatalf("group version must not change on failure, got %q", g.Version)
	}
}

func TestPublishAPIGroupVersion_userLabelTakesPrecedence(t *testing.T) {
	custom := "1.4.0"
	fs := &fakePublishStore{ret: catalogpkg.APIGroupVersion{VersionNumber: 3, Label: &custom}}
	h := &Handler{store: fs, storage: newFakeStorage()}

	g := newGroup()
	if _, err := h.publishAPIGroupVersion(context.Background(), publishParams{
		group: g, serviceID: "svc-1", spec: testOpenAPISpec,
		label: &custom, isAuto: false, actorID: "user-1",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fs.got.Version.Label == nil || *fs.got.Version.Label != "1.4.0" {
		t.Fatalf("expected user label to pass through, got %v", fs.got.Version.Label)
	}
}
