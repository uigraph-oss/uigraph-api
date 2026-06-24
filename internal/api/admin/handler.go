// Package admin provides server-admin-only instance overview handlers.
package admin

import (
	"context"
	"net/http"

	"github.com/uigraph/app/internal/config"
	"github.com/uigraph/app/internal/httputil"
)

// store is the minimal persistence interface this package needs.
// postgres.DB satisfies it automatically.
type store interface {
	CountAllUsers(ctx context.Context) (int, error)
	CountActiveUsers(ctx context.Context) (int, error)
	CountAllOrgs(ctx context.Context) (int, error)
}

// Handler serves /api/v1/server/* instance-admin endpoints.
type Handler struct {
	store store
	cfg   *config.Config
}

// New constructs a Handler.
func New(s store, cfg *config.Config) *Handler {
	return &Handler{store: s, cfg: cfg}
}

type overviewResponse struct {
	TotalUsers  int `json:"totalUsers"`
	ActiveUsers int `json:"activeUsers"`
	TotalOrgs   int `json:"totalOrgs"`
}

// Overview handles GET /api/v1/server/overview
func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	totalUsers, err := h.store.CountAllUsers(r.Context())
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	activeUsers, err := h.store.CountActiveUsers(r.Context())
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	totalOrgs, err := h.store.CountAllOrgs(r.Context())
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, overviewResponse{
		TotalUsers:  totalUsers,
		ActiveUsers: activeUsers,
		TotalOrgs:   totalOrgs,
	})
}

type configResponse struct {
	StorageBackend   string `json:"storageBackend"`
	StorageBucket    string `json:"storageBucket"`
	StorageEndpoint  string `json:"storageEndpoint"`
	StorageRegion    string `json:"storageRegion"`
	VectorBackend    string `json:"vectorBackend"`
	EmbeddingBackend string `json:"embeddingBackend"`
	EmbeddingModel   string `json:"embeddingModel"`
}

// Config handles GET /api/v1/server/config
func (h *Handler) Config(w http.ResponseWriter, r *http.Request) {
	httputil.JSON(w, http.StatusOK, configResponse{
		StorageBackend:   h.cfg.StorageBackend,
		StorageBucket:    h.cfg.StorageBucket,
		StorageEndpoint:  h.cfg.StorageEndpoint,
		StorageRegion:    h.cfg.StorageRegion,
		VectorBackend:    h.cfg.VectorBackend,
		EmbeddingBackend: h.cfg.EmbeddingBackend,
		EmbeddingModel:   h.cfg.EmbeddingModel,
	})
}
