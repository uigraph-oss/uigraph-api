// Package diagram provides HTTP handlers for diagram CRUD, versions, and images.
package diagram

import (
	"context"
	"io"
	"net/http"

	"github.com/uigraph/app/internal/cache"
	diagrampkg "github.com/uigraph/app/internal/diagram"
	"github.com/uigraph/app/internal/queue"
)

// store is the minimal persistence interface this package needs.
type store interface {
	CreateDiagram(ctx context.Context, d diagrampkg.Diagram) error
	GetDiagram(ctx context.Context, id string) (*diagrampkg.Diagram, error)
	ListDiagrams(ctx context.Context, orgID string, p diagrampkg.ListParams) ([]diagrampkg.Diagram, int, error)
	UpdateDiagram(ctx context.Context, d diagrampkg.Diagram) error
	SetDiagramPreviewStatus(ctx context.Context, id, status string) error
	SoftDeleteDiagram(ctx context.Context, id, deletedBy string) error

	CreateDiagramVersion(ctx context.Context, v diagrampkg.Version) error
	GetDiagramVersion(ctx context.Context, id string) (*diagrampkg.Version, error)
	ListDiagramVersions(ctx context.Context, diagramID string) ([]diagrampkg.Version, error)
	LatestVersionNumber(ctx context.Context, diagramID string) (int, error)

	CreateDiagramImage(ctx context.Context, img diagrampkg.Image) error
	ListDiagramImages(ctx context.Context, diagramID string) ([]diagrampkg.Image, error)
}

// objectStore is the minimal storage interface this package needs.
type objectStore interface {
	Upload(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	PresignPutURL(ctx context.Context, key string) (string, error)
}

// Handler serves diagram endpoints.
type Handler struct {
	store   store
	storage objectStore
	cache   cache.Client // may be nil
	queue   *queue.Queue // may be nil
}

// New constructs a Handler.
func New(s store, st objectStore, c cache.Client, q *queue.Queue) *Handler {
	return &Handler{store: s, storage: st, cache: c, queue: q}
}

// Register wires diagram routes into mux.
func Register(
	mux *http.ServeMux,
	s store,
	st objectStore,
	c cache.Client,
	q *queue.Queue,
	requireScope func(scope, method, pattern string, h http.HandlerFunc),
) {
	h := New(s, st, c, q)
	requireScope("diagrams:read", "GET", "/api/v1/orgs/{orgID}/diagrams", h.List)
	requireScope("diagrams:write", "POST", "/api/v1/orgs/{orgID}/diagrams", h.Create)
	requireScope("diagrams:write", "POST", "/api/v1/orgs/{orgID}/diagrams/sync", h.Sync)
	requireScope("diagrams:read", "GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}", h.Get)
	requireScope("diagrams:write", "PUT", "/api/v1/orgs/{orgID}/diagrams/{diagramID}", h.Update)
	requireScope("diagrams:write", "DELETE", "/api/v1/orgs/{orgID}/diagrams/{diagramID}", h.Delete)
	requireScope("diagrams:write", "POST", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/thumbnail", h.UpdateThumbnail)
	requireScope("diagrams:write", "POST", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/thumbnail/prepare", h.PrepareThumbnailUpload)
	requireScope("diagrams:write", "POST", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/thumbnail/confirm", h.ConfirmThumbnailUpload)
	requireScope("diagrams:read", "GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/content", h.GetContent)
	requireScope("diagrams:read", "GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/images", h.ListImages)
	requireScope("diagrams:write", "POST", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/images", h.CreateImage)
	requireScope("diagrams:read", "GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/versions", h.ListVersions)
	requireScope("diagrams:write", "POST", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/versions", h.CreateVersion)
	requireScope("diagrams:read", "GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/versions/{versionID}/content", h.GetVersionContent)
	requireScope("diagrams:write", "POST", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/versions/{versionID}/restore", h.RestoreVersion)
}
