package content

import (
	"net/http"
	"strings"

	"github.com/uigraph/app/internal/asset"
	"github.com/uigraph/app/internal/cache"
	"github.com/uigraph/app/internal/storage"
)

// maxAssetIDs caps how many ids one request may resolve.
const maxAssetIDs = 200

type AssetHandler struct {
	resolver *asset.Resolver
}

func NewAssetHandler(st storage.Client, c cache.Client) *AssetHandler {
	return &AssetHandler{resolver: asset.New(st, c)}
}

// Resolve handles GET /api/v1/orgs/{orgID}/assets/urls?ids=a,b,c and returns a
// map from each asset id to its presigned GET URL.
func (h *AssetHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("ids")
	if raw == "" {
		writeErr(w, http.StatusBadRequest, "ids query parameter is required")
		return
	}

	var ids []string
	for _, part := range strings.Split(raw, ",") {
		id := strings.TrimSpace(part)
		if id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		writeErr(w, http.StatusBadRequest, "ids query parameter is required")
		return
	}
	if len(ids) > maxAssetIDs {
		writeErr(w, http.StatusBadRequest, "too many ids")
		return
	}

	urls, err := h.resolver.ResolveMany(r.Context(), ids)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"urls": urls})
}
