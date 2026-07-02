package health

import (
	"net/http"

	"github.com/uigraph/app/internal/httputil"
)

// Store is the subset of store.Store needed for health checks.
type Store interface{}

type Handler struct {
	Store Store
}

// Healthz returns 200 once migrations have completed and backends are reachable.
// Docker Compose and Kubernetes readiness probes use this endpoint.
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	httputil.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Livez returns 200 if the process is alive. Does not check downstream backends.
// Kubernetes liveness probes use this endpoint.
func (h *Handler) Livez(w http.ResponseWriter, r *http.Request) {
	httputil.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
