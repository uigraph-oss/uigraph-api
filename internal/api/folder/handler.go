// Package folder provides HTTP handlers for folder CRUD.
package folder

import (
	"context"
	"net/http"

	"github.com/uigraph/app/internal/folder"
)

// store is the minimal persistence interface this package needs.
// postgres.DB satisfies it automatically.
type store interface {
	CreateFolder(ctx context.Context, f folder.Folder) error
	GetFolder(ctx context.Context, id string) (*folder.Folder, error)
	ListFolders(ctx context.Context, orgID string, t *folder.Type) ([]folder.Folder, error)
	UpdateFolder(ctx context.Context, f folder.Folder) error
	DeleteFolder(ctx context.Context, id, deletedBy string) error
}

// Handler serves /api/v1/orgs/{orgID}/folders.
type Handler struct {
	store store
}

// New constructs a Handler.
func New(s store) *Handler {
	return &Handler{store: s}
}

// Register wires folder routes into mux.
// requireScope signature: func(scope, method, pattern string, h http.HandlerFunc)
func Register(
	mux *http.ServeMux,
	s store,
	requireScope func(scope, method, pattern string, h http.HandlerFunc),
) {
	h := New(s)
	requireScope("folders:read", "GET", "/api/v1/orgs/{orgID}/folders", h.List)
	requireScope("folders:write", "POST", "/api/v1/orgs/{orgID}/folders", h.Create)
	requireScope("folders:read", "GET", "/api/v1/orgs/{orgID}/folders/{folderID}", h.Get)
	requireScope("folders:write", "PUT", "/api/v1/orgs/{orgID}/folders/{folderID}", h.Update)
	requireScope("folders:write", "DELETE", "/api/v1/orgs/{orgID}/folders/{folderID}", h.Delete)
}
