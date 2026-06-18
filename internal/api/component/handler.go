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
) {
	h := New(s, st)
	// Unauthenticated — icon assets are public.
	mux.HandleFunc("GET /api/v1/component-icons/{slug}", h.GetIcon)
	// Authenticated.
	protected("GET", "/api/v1/orgs/{orgID}/components", h.ListFocal)
	protected("GET", "/api/v1/orgs/{orgID}/flow-diagram-components", h.ListFlow)
}
