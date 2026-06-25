package catalog

import (
	"net/http"

	"github.com/uigraph/app/internal/httputil"
	storepkg "github.com/uigraph/app/internal/store"
)

// GetAPIEndpointByID handles GET /api/v1/orgs/{orgID}/endpoints/{endpointID}
func (h *Handler) GetAPIEndpointByID(w http.ResponseWriter, r *http.Request) {
	e, err := h.store.GetAPIEndpoint(r.Context(), r.PathValue("endpointID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if e == nil || e.DeletedAt != nil || e.OrgID != r.PathValue("orgID") {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, e)
}

// GetTestPackByID handles GET /api/v1/orgs/{orgID}/test-packs/{testPackID}
func (h *Handler) GetTestPackByID(w http.ResponseWriter, r *http.Request) {
	p, err := h.store.GetTestPack(r.Context(), r.PathValue("testPackID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if p == nil || p.DeletedAt != nil || p.OrgID != r.PathValue("orgID") {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, p)
}

// GetServiceDocByID handles GET /api/v1/orgs/{orgID}/service-docs/{docID}
func (h *Handler) GetServiceDocByID(w http.ResponseWriter, r *http.Request) {
	sd, err := h.store.GetServiceDocByID(r.Context(), r.PathValue("docID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if sd == nil || sd.DeletedAt != nil || sd.OrgID != r.PathValue("orgID") {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, sd)
}
