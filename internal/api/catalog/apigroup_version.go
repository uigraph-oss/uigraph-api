package catalog

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	catalogpkg "github.com/uigraph/app/internal/catalog"
	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
)

// It publishes an explicit, immutable release. When a spec is supplied the
// working copy is replaced from it; otherwise the current working copy is
// snapshotted as-is. The label is user-supplied or auto-derived ("v{N}").
// @Summary  CreateAPIGroupVersion
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    apiGroupID  path  string  true  "apiGroupID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/versions [post]
func (h *Handler) CreateAPIGroupVersion(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	g, err := h.store.GetAPIGroup(r.Context(), r.PathValue("apiGroupID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if g == nil || g.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	var body struct {
		Label       *string `json:"label"`
		Spec        string  `json:"spec"`
		SpecAssetID *string `json:"specAssetId"`
	}
	_ = httputil.Decode(r, &body)

	specContent, err := h.resolveSpec(r.Context(), body.Spec, body.SpecAssetID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	var label *string
	if body.Label != nil && *body.Label != "" {
		label = body.Label
	}

	g.UpdatedBy = &p.UserID
	g.UpdatedByCommitHash = nil
	v, err := h.publishAPIGroupVersion(r.Context(), publishParams{
		group: g, serviceID: serviceID, spec: specContent,
		label: label, isAuto: false, actorID: p.UserID,
	})
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, v)
}

// It copies the version's spec and endpoint snapshot back onto the working copy
// and records the restored state as a new auto-version. Earlier versions are
// untouched.
// @Summary  RestoreAPIGroupVersion
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    apiGroupID  path  string  true  "apiGroupID"
// @Param    versionID  path  string  true  "versionID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/versions/{versionID}/restore [post]
func (h *Handler) RestoreAPIGroupVersion(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	g, err := h.store.GetAPIGroup(r.Context(), r.PathValue("apiGroupID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if g == nil || g.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	v, err := h.store.GetAPIGroupVersion(r.Context(), r.PathValue("versionID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if v == nil || v.APIGroupID != g.ID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	specContent, err := h.downloadSpec(r.Context(), v.SpecKey)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	// Restore the exact endpoint snapshot rather than re-parsing the spec, so
	// any per-endpoint edits captured in that version are preserved.
	snapshot, err := h.store.ListAPIEndpointsForVersion(r.Context(), g.ID, v.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	now := time.Now().UTC()
	working := make([]catalogpkg.APIEndpoint, 0, len(snapshot))
	for _, e := range snapshot {
		e.ID = uuid.NewString()
		e.APIGroupVersionID = nil
		e.CreatedBy = p.UserID
		e.UpdatedBy = nil
		e.CreatedAt = now
		e.UpdatedAt = now
		working = append(working, e)
	}

	g.UpdatedBy = &p.UserID
	g.UpdatedByCommitHash = nil
	if _, err := h.publishAPIGroupVersion(r.Context(), publishParams{
		group: g, serviceID: serviceID, spec: specContent,
		endpoints: working, label: nil, isAuto: true, actorID: p.UserID,
	}); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, g)
}
