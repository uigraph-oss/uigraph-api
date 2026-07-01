package component

import (
	"net/http"

	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/storage"
)

// @Summary  GetIcon
// @Tags     component-icons
// @Security BearerAuth
// @Param    slug  path  string  true  "slug"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /component-icons/{slug} [get]
func (h *Handler) GetIcon(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		httputil.BadRequest(w, "slug is required")
		return
	}
	httputil.StreamObject(w, r, h.storage.Download, storage.ComponentIconKey(slug))
}
