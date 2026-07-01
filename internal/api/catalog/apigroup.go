package catalog

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	catalogpkg "github.com/uigraph/app/internal/catalog"
	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
	"github.com/uigraph/app/internal/storage"
)

// ── API Groups ────────────────────────────────────────────────────────────────

// ListAPIGroups
// @Summary  ListAPIGroups
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/api-groups [get]
func (h *Handler) ListAPIGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.store.ListAPIGroups(r.Context(), r.PathValue("serviceID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"apiGroups": groups})
}

// CreateAPIGroup
// @Summary  CreateAPIGroup
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/api-groups [post]
func (h *Handler) CreateAPIGroup(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	var body struct {
		Name        string  `json:"name"`
		Version     string  `json:"version"`
		Label       *string `json:"label"`
		Protocol    string  `json:"protocol"`
		Spec        string  `json:"spec"`
		SpecAssetID *string `json:"specAssetId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}
	if body.Protocol == "" {
		body.Protocol = "REST"
	}
	var label *string
	if body.Version != "" {
		v := body.Version
		label = &v
	} else {
		body.Version = "v1"
	}

	specContent, err := h.resolveSpec(r.Context(), body.Spec, body.SpecAssetID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	id := uuid.NewString()
	now := time.Now().UTC()
	g := catalogpkg.APIGroup{
		ID:        id,
		ServiceID: serviceID,
		OrgID:     orgID,
		Name:      body.Name,
		Version:   body.Version,
		Label:     body.Label,
		Protocol:  body.Protocol,
		CreatedBy: p.UserID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.store.CreateAPIGroup(r.Context(), g); err != nil {
		httputil.Error(w, r, err)
		return
	}

	// Auto-version 1 — created atomically after the parent row is committed.
	if specContent != "" && h.storage != nil {
		g.UpdatedBy = &p.UserID
		if _, err := h.publishAPIGroupVersion(r.Context(), publishParams{
			group: &g, serviceID: serviceID, spec: specContent,
			label: label, isAuto: true, actorID: p.UserID,
		}); err != nil {
			httputil.Error(w, r, err)
			return
		}
	}

	httputil.JSON(w, http.StatusCreated, g)
}

// GetAPIGroup
// @Summary  GetAPIGroup
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    apiGroupID  path  string  true  "apiGroupID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID} [get]
func (h *Handler) GetAPIGroup(w http.ResponseWriter, r *http.Request) {
	g, err := h.store.GetAPIGroup(r.Context(), r.PathValue("apiGroupID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if g == nil || g.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, g)
}

// @Summary  GetAPIGroupSpec
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    apiGroupID  path  string  true  "apiGroupID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/spec [get]
func (h *Handler) GetAPIGroupSpec(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	serviceID := r.PathValue("serviceID")

	g, err := h.store.GetAPIGroup(r.Context(), r.PathValue("apiGroupID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if g == nil || g.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	var key string
	if versionID := r.URL.Query().Get("versionId"); versionID != "" {
		key = storage.APIGroupVersionSpecKey(orgID, serviceID, g.ID, versionID)
	}
	if key == "" && g.SpecKey != nil {
		key = *g.SpecKey
	}
	if key == "" {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	rc, err := h.storage.Download(r.Context(), key)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	defer rc.Close()

	content, err := io.ReadAll(rc)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"apiGroupId": g.ID,
		"fileName":   g.Name,
		"content":    string(content),
	})
}

// UpdateAPIGroup
// @Summary  UpdateAPIGroup
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
// @Router   /orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID} [put]
func (h *Handler) UpdateAPIGroup(w http.ResponseWriter, r *http.Request) {
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
		Name        *string `json:"name"`
		Version     *string `json:"version"`
		Label       *string `json:"label"`
		Protocol    *string `json:"protocol"`
		Spec        *string `json:"spec"`
		SpecAssetID *string `json:"specAssetId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name != nil {
		g.Name = *body.Name
	}
	if body.Version != nil {
		g.Version = *body.Version
	}
	if body.Label != nil {
		g.Label = body.Label
	}
	if body.Protocol != nil {
		g.Protocol = *body.Protocol
	}
	g.UpdatedBy = &p.UserID

	if (body.Spec != nil || body.SpecAssetID != nil) && h.storage != nil {
		rawSpec := ""
		if body.Spec != nil {
			rawSpec = *body.Spec
		}
		specContent, err := h.resolveSpec(r.Context(), rawSpec, body.SpecAssetID)
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
		if specContent != "" {
			newHash := specHash(specContent)
			if g.SpecHash == nil || newHash != *g.SpecHash {
				// New spec → atomically import endpoints and snapshot a version.
				// publishAPIGroupVersion also persists the working-copy row.
				var label *string
				if body.Version != nil && *body.Version != "" {
					label = body.Version
				}
				if _, err := h.publishAPIGroupVersion(r.Context(), publishParams{
					group: g, serviceID: serviceID, spec: specContent,
					label: label, isAuto: true, actorID: p.UserID,
				}); err != nil {
					httputil.Error(w, r, err)
					return
				}
				httputil.JSON(w, http.StatusOK, g)
				return
			}
		}
	}

	// No spec change: persist metadata edits only.
	if err := h.store.UpdateAPIGroup(r.Context(), *g); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, g)
}

// DeleteAPIGroup
// @Summary  DeleteAPIGroup
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    apiGroupID  path  string  true  "apiGroupID"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID} [delete]
func (h *Handler) DeleteAPIGroup(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if err := h.store.SoftDeleteAPIGroup(r.Context(), r.PathValue("apiGroupID"), p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CLI upsert: creates or updates an API group, skipping the spec upload when hash is unchanged.
// @Summary  SyncAPIGroup
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/api-groups/sync [post]
func (h *Handler) SyncAPIGroup(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	var body struct {
		APIGroupID  *string `json:"apiGroupId"`
		Name        string  `json:"name"`
		Version     string  `json:"version"`
		Protocol    string  `json:"protocol"`
		Spec        string  `json:"spec"`
		SpecAssetID *string `json:"specAssetId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}
	if body.Protocol == "" {
		body.Protocol = "REST"
	}
	var label *string
	if body.Version != "" {
		v := body.Version
		label = &v
	}

	specContent, err := h.resolveSpec(r.Context(), body.Spec, body.SpecAssetID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	newHash := specHash(specContent)

	// Update path.
	if body.APIGroupID != nil {
		g, err := h.store.GetAPIGroup(r.Context(), *body.APIGroupID)
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
		if g == nil || g.DeletedAt != nil {
			httputil.Error(w, r, storepkg.ErrNotFound)
			return
		}
		// Unchanged spec → no-op, the existing version stands.
		if specContent != "" && g.SpecHash != nil && newHash == *g.SpecHash {
			httputil.JSON(w, http.StatusOK, map[string]any{"apiGroupId": g.ID, "versionCreated": false})
			return
		}
		g.Name = body.Name
		g.Protocol = body.Protocol
		g.UpdatedBy = &p.UserID
		if label != nil {
			g.Version = *label
		}

		versionCreated := false
		if specContent != "" && h.storage != nil {
			if _, err := h.publishAPIGroupVersion(r.Context(), publishParams{
				group: g, serviceID: serviceID, spec: specContent,
				label: label, isAuto: true, actorID: p.UserID,
			}); err != nil {
				httputil.Error(w, r, err)
				return
			}
			versionCreated = true
		} else if err := h.store.UpdateAPIGroup(r.Context(), *g); err != nil {
			httputil.Error(w, r, err)
			return
		}
		httputil.JSON(w, http.StatusOK, map[string]any{"apiGroupId": g.ID, "versionCreated": versionCreated})
		return
	}

	// Create path.
	id := uuid.NewString()
	now := time.Now().UTC()
	g := catalogpkg.APIGroup{
		ID: id, ServiceID: serviceID, OrgID: orgID,
		Name: body.Name, Version: "v1", Protocol: body.Protocol,
		CreatedBy: p.UserID, UpdatedBy: &p.UserID, CreatedAt: now, UpdatedAt: now,
	}
	if label != nil {
		g.Version = *label
	}
	if err := h.store.CreateAPIGroup(r.Context(), g); err != nil {
		httputil.Error(w, r, err)
		return
	}
	versionCreated := false
	if specContent != "" && h.storage != nil {
		if _, err := h.publishAPIGroupVersion(r.Context(), publishParams{
			group: &g, serviceID: serviceID, spec: specContent,
			label: label, isAuto: true, actorID: p.UserID,
		}); err != nil {
			httputil.Error(w, r, err)
			return
		}
		versionCreated = true
	}
	httputil.JSON(w, http.StatusCreated, map[string]any{"apiGroupId": id, "versionCreated": versionCreated})
}

// @Summary  ListAPIGroupVersions
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    apiGroupID  path  string  true  "apiGroupID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/versions [get]
func (h *Handler) ListAPIGroupVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := h.store.ListAPIGroupVersions(r.Context(), r.PathValue("apiGroupID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"versions": versions})
}
