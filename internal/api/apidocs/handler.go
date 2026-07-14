// Package apidocs provides HTTP handlers for org-level documentation CRUD.
package apidocs

import (
	"context"
	"io"
	"net/http"

	docspkg "github.com/uigraph/app/internal/docs"
)

// store is the minimal persistence interface this package needs.
type store interface {
	CreateDoc(ctx context.Context, d docspkg.Doc) error
	GetDoc(ctx context.Context, id string) (*docspkg.Doc, error)
	ListDocs(ctx context.Context, orgID string, p docspkg.ListParams) ([]docspkg.Doc, int, error)
	UpdateDoc(ctx context.Context, d docspkg.Doc) error
	SoftDeleteDoc(ctx context.Context, id, deletedBy string) error
}

// objectStore is the minimal storage interface this package needs.
type objectStore interface {
	Upload(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// Handler serves /api/v1/orgs/{orgID}/docs and nested resources.
type Handler struct {
	store   store
	storage objectStore
}

// New constructs a Handler.
func New(s store, st objectStore) *Handler {
	return &Handler{store: s, storage: st}
}

// Register wires doc routes into mux.
func Register(
	mux *http.ServeMux,
	s store,
	st objectStore,
	requireScope func(scope, method, pattern string, h http.HandlerFunc),
) {
	h := New(s, st)
	requireScope("docs:read", "GET", "/api/v1/orgs/{orgID}/docs", h.List)
	requireScope("docs:write", "POST", "/api/v1/orgs/{orgID}/docs", h.Create)
	requireScope("docs:read", "GET", "/api/v1/orgs/{orgID}/docs/{docID}", h.Get)
	requireScope("docs:read", "GET", "/api/v1/orgs/{orgID}/docs/{docID}/content", h.GetContent)
	requireScope("docs:write", "PUT", "/api/v1/orgs/{orgID}/docs/{docID}", h.Update)
	requireScope("docs:write", "DELETE", "/api/v1/orgs/{orgID}/docs/{docID}", h.Delete)
}
