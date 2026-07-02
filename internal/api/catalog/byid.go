package catalog

import (
	"net/http"

	"github.com/uigraph/app/internal/httputil"
	storepkg "github.com/uigraph/app/internal/store"
)

// @Summary  GetAPIEndpointByID
// @Tags     endpoints
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    endpointID  path  string  true  "endpointID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/endpoints/{endpointID} [get]
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

// @Summary  GetTestPackByID
// @Tags     test-packs
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    testPackID  path  string  true  "testPackID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/test-packs/{testPackID} [get]
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

// @Summary  GetServiceDocByID
// @Tags     service-docs
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    docID  path  string  true  "docID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/service-docs/{docID} [get]
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
