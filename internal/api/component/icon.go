package component

import (
	"net/http"

	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/storage"
)

// GetIcon handles GET /api/v1/component-icons/{slug}
func (h *Handler) GetIcon(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		httputil.BadRequest(w, "slug is required")
		return
	}
	httputil.StreamObject(w, r, h.storage.Download, storage.ComponentIconKey(slug))
}
