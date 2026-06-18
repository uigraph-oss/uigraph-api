// Package asset provides an HTTP handler that resolves asset IDs to presigned URLs.
package asset

import (
	"net/http"
	"strings"

	assetpkg "github.com/uigraph/app/internal/asset"
	"github.com/uigraph/app/internal/cache"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/storage"
)

const maxAssetIDs = 200

// Handler wraps asset.Resolver for HTTP.
type Handler struct {
	resolver *assetpkg.Resolver
}

// New constructs a Handler.
func New(st storage.Client, c cache.Client) *Handler {
	return &Handler{resolver: assetpkg.New(st, c)}
}

// Register wires the asset URL route into mux.
func Register(
	mux *http.ServeMux,
	st storage.Client,
	c cache.Client,
	protected func(method, pattern string, h http.HandlerFunc),
) {
	h := New(st, c)
	protected("GET", "/api/v1/orgs/{orgID}/assets/urls", h.Resolve)
}

// Resolve handles GET /api/v1/orgs/{orgID}/assets/urls?ids=a,b,c
func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("ids")
	if raw == "" {
		httputil.BadRequest(w, "ids query parameter is required")
		return
	}
	var ids []string
	for _, part := range strings.Split(raw, ",") {
		if id := strings.TrimSpace(part); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		httputil.BadRequest(w, "ids query parameter is required")
		return
	}
	if len(ids) > maxAssetIDs {
		httputil.BadRequest(w, "too many ids")
		return
	}
	urls, err := h.resolver.ResolveMany(r.Context(), ids)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"urls": urls})
}
