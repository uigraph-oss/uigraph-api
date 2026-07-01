package diagram

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	diagrampkg "github.com/uigraph/app/internal/diagram"
	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
	"github.com/uigraph/app/internal/storage"
)

// ListVersions
// @Summary  ListVersions
// @Tags     diagrams
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    diagramID  path  string  true  "diagramID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/diagrams/{diagramID}/versions [get]
func (h *Handler) ListVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := h.store.ListDiagramVersions(r.Context(), r.PathValue("diagramID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"versions": versions})
}

// CreateVersion
// @Summary  CreateVersion
// @Tags     diagrams
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    diagramID  path  string  true  "diagramID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/diagrams/{diagramID}/versions [post]
func (h *Handler) CreateVersion(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("diagramID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	dg, err := h.store.GetDiagram(r.Context(), id)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if dg == nil || dg.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	var body struct {
		Label *string `json:"label"`
	}
	_ = httputil.Decode(r, &body)

	content, err := h.getContent(r.Context(), id, dg.ContentKey)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	latestVer, _ := h.store.LatestVersionNumber(r.Context(), id)
	versionID := uuid.NewString()
	vKey := storage.DiagramVersionKey(orgID, id, versionID)
	if err := h.uploadContent(r.Context(), vKey, content); err != nil {
		httputil.Error(w, r, err)
		return
	}

	v := diagrampkg.Version{
		ID:            versionID,
		DiagramID:     id,
		VersionNumber: latestVer + 1,
		Label:         body.Label,
		ContentKey:    vKey,
		ContentHash:   dg.ContentHash,
		IsAutoVersion: false,
		Source:        dg.Source,
		CreatedBy:     p.UserID,
		CreatedAt:     time.Now().UTC(),
	}
	if err := h.store.CreateDiagramVersion(r.Context(), v); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, v)
}

// GetVersionContent
// @Summary  GetVersionContent
// @Tags     diagrams
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    diagramID  path  string  true  "diagramID"
// @Param    versionID  path  string  true  "versionID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/diagrams/{diagramID}/versions/{versionID}/content [get]
func (h *Handler) GetVersionContent(w http.ResponseWriter, r *http.Request) {
	v, err := h.store.GetDiagramVersion(r.Context(), r.PathValue("versionID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if v == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	content, err := h.downloadContent(r.Context(), v.ContentKey)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"versionId": v.ID, "content": content})
}

// RestoreVersion
// @Summary  RestoreVersion
// @Tags     diagrams
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    diagramID  path  string  true  "diagramID"
// @Param    versionID  path  string  true  "versionID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/diagrams/{diagramID}/versions/{versionID}/restore [post]
func (h *Handler) RestoreVersion(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("diagramID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	dg, err := h.store.GetDiagram(r.Context(), id)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if dg == nil || dg.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	v, err := h.store.GetDiagramVersion(r.Context(), r.PathValue("versionID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if v == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	content, err := h.downloadContent(r.Context(), v.ContentKey)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	// Write restored content as the new current object.
	if err := h.uploadContent(r.Context(), dg.ContentKey, content); err != nil {
		httputil.Error(w, r, err)
		return
	}

	dg.ContentHash = v.ContentHash
	src := fmt.Sprintf("restore:v%d", v.VersionNumber)
	dg.Source = &src
	dg.UpdatedBy = &p.UserID
	if err := h.store.UpdateDiagram(r.Context(), *dg); err != nil {
		httputil.Error(w, r, err)
		return
	}

	// Record the restore as a new auto-version.
	latestVer, _ := h.store.LatestVersionNumber(r.Context(), id)
	versionID := uuid.NewString()
	vKey := storage.DiagramVersionKey(orgID, id, versionID)
	if err := h.uploadContent(r.Context(), vKey, content); err == nil {
		_ = h.store.CreateDiagramVersion(r.Context(), diagrampkg.Version{
			ID:            versionID,
			DiagramID:     id,
			VersionNumber: latestVer + 1,
			ContentKey:    vKey,
			ContentHash:   v.ContentHash,
			IsAutoVersion: true,
			Source:        &src,
			CreatedBy:     p.UserID,
			CreatedAt:     time.Now().UTC(),
		})
	}

	h.cacheDel(r.Context(), id)
	httputil.JSON(w, http.StatusOK, dg)
}
