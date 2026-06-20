// Package component provides HTTP handlers for component palettes and icons.
package component

import (
	"context"
	"io"
	"net/http"

	"github.com/uigraph/app/internal/componentlib"
	"github.com/uigraph/app/internal/storage"
)

// store is the minimal persistence interface this package needs.
type store interface {
	ListComponentsByKind(ctx context.Context, kind string) ([]componentlib.Component, error)
	ListCustomComponents(ctx context.Context, orgID string) ([]componentlib.Component, error)
	SaveCustomComponent(ctx context.Context, c componentlib.Component) error
	GetComponent(ctx context.Context, id string) (*componentlib.Component, error)
	DeleteComponent(ctx context.Context, id string) error
}

// objectStore is the minimal storage interface this package needs.
type objectStore interface {
	Download(ctx context.Context, key string) (io.ReadCloser, error)
}

// Handler serves component palette and icon endpoints.
type Handler struct {
	store   store
	storage objectStore
}

// New constructs a Handler.
func New(s store, st objectStore) *Handler {
	return &Handler{store: s, storage: st}
}

// Register wires component routes into mux.
// protected wraps a handler requiring authentication (no scope check).
func Register(
	mux *http.ServeMux,
	s store,
	st storage.Client,
	protected func(method, pattern string, h http.HandlerFunc),
	requireScope func(scope, method, pattern string, h http.HandlerFunc),
) {
	h := New(s, st)
	// Unauthenticated — icon assets are public.
	mux.HandleFunc("GET /api/v1/component-icons/{slug}", h.GetIcon)
	// Authenticated.
	protected("GET", "/api/v1/orgs/{orgID}/components", h.ListFocal)
	protected("GET", "/api/v1/orgs/{orgID}/flow-diagram-components", h.ListFlow)
	requireScope("maps:write", "POST", "/api/v1/orgs/{orgID}/components", h.Create)
	requireScope("maps:write", "PUT", "/api/v1/orgs/{orgID}/components/{componentID}", h.Update)
	requireScope("maps:write", "DELETE", "/api/v1/orgs/{orgID}/components/{componentID}", h.Delete)
}
