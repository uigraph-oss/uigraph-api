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

func (h *Handler) ListAPIGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.store.ListAPIGroups(r.Context(), r.PathValue("serviceID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"apiGroups": groups})
}

func (h *Handler) CreateAPIGroup(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	var body struct {
		Name     string  `json:"name"`
		Version  string  `json:"version"`
		Label    *string `json:"label"`
		Protocol string  `json:"protocol"`
		Spec     string  `json:"spec"`
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
	if body.Version == "" {
		body.Version = "v1"
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

	var specContent string
	if body.Spec != "" && h.storage != nil {
		key := storage.APIGroupSpecKey(orgID, serviceID, id)
		if err := h.uploadSpec(r.Context(), key, body.Spec); err != nil {
			httputil.Error(w, r, err)
			return
		}
		hash := specHash(body.Spec)
		g.SpecKey = &key
		g.SpecHash = &hash
		specContent = body.Spec
	}

	if err := h.store.CreateAPIGroup(r.Context(), g); err != nil {
		httputil.Error(w, r, err)
		return
	}

	// Auto-version 1 — created after the parent row is committed.
	if specContent != "" {
		hash := specHash(specContent)
		versionID := uuid.NewString()
		vKey := storage.APIGroupVersionSpecKey(orgID, serviceID, id, versionID)
		if err := h.uploadSpec(r.Context(), vKey, specContent); err == nil {
			_ = h.store.CreateAPIGroupVersion(r.Context(), catalogpkg.APIGroupVersion{
				ID: versionID, APIGroupID: id, VersionNumber: 1,
				SpecKey: vKey, SpecHash: hash, IsAutoVersion: true,
				CreatedBy: p.UserID, CreatedAt: now,
			})
		}
	}

	httputil.JSON(w, http.StatusCreated, g)
}

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

// GetAPIGroupSpec handles GET /api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/spec
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

func (h *Handler) UpdateAPIGroup(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
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
		Name     *string `json:"name"`
		Version  *string `json:"version"`
		Label    *string `json:"label"`
		Protocol *string `json:"protocol"`
		Spec     *string `json:"spec"`
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

	if body.Spec != nil && h.storage != nil {
		newHash := specHash(*body.Spec)
		if g.SpecHash == nil || newHash != *g.SpecHash {
			key := storage.APIGroupSpecKey(orgID, serviceID, g.ID)
			if err := h.uploadSpec(r.Context(), key, *body.Spec); err != nil {
				httputil.Error(w, r, err)
				return
			}
			g.SpecKey = &key
			g.SpecHash = &newHash

			latestVer, _ := h.store.LatestAPIGroupVersionNumber(r.Context(), g.ID)
			versionID := uuid.NewString()
			vKey := storage.APIGroupVersionSpecKey(orgID, serviceID, g.ID, versionID)
			if err := h.uploadSpec(r.Context(), vKey, *body.Spec); err == nil {
				_ = h.store.CreateAPIGroupVersion(r.Context(), catalogpkg.APIGroupVersion{
					ID: versionID, APIGroupID: g.ID, VersionNumber: latestVer + 1,
					SpecKey: vKey, SpecHash: newHash, IsAutoVersion: true,
					CreatedBy: p.UserID, CreatedAt: time.Now().UTC(),
				})
			}
		}
	}

	if err := h.store.UpdateAPIGroup(r.Context(), *g); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, g)
}

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

// SyncAPIGroup handles POST /api/v1/orgs/{orgID}/services/{serviceID}/api-groups/sync
// CLI upsert: creates or updates an API group, skipping the spec upload when hash is unchanged.
func (h *Handler) SyncAPIGroup(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	var body struct {
		APIGroupID *string `json:"apiGroupId"`
		Name       string  `json:"name"`
		Version    string  `json:"version"`
		Protocol   string  `json:"protocol"`
		Spec       string  `json:"spec"`
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
	if body.Version == "" {
		body.Version = "v1"
	}

	newHash := specHash(body.Spec)

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
		if body.Spec != "" && g.SpecHash != nil && newHash == *g.SpecHash {
			httputil.JSON(w, http.StatusOK, map[string]any{"apiGroupId": g.ID, "versionCreated": false})
			return
		}
		versionCreated := false
		if body.Spec != "" && h.storage != nil {
			key := storage.APIGroupSpecKey(orgID, serviceID, g.ID)
			if err := h.uploadSpec(r.Context(), key, body.Spec); err != nil {
				httputil.Error(w, r, err)
				return
			}
			g.SpecKey = &key
			g.SpecHash = &newHash
			latestVer, _ := h.store.LatestAPIGroupVersionNumber(r.Context(), g.ID)
			versionID := uuid.NewString()
			vKey := storage.APIGroupVersionSpecKey(orgID, serviceID, g.ID, versionID)
			if err := h.uploadSpec(r.Context(), vKey, body.Spec); err == nil {
				if err := h.store.CreateAPIGroupVersion(r.Context(), catalogpkg.APIGroupVersion{
					ID: versionID, APIGroupID: g.ID, VersionNumber: latestVer + 1,
					SpecKey: vKey, SpecHash: newHash, IsAutoVersion: true,
					CreatedBy: p.UserID, CreatedAt: time.Now().UTC(),
				}); err == nil {
					versionCreated = true
				}
			}
		}
		g.Name = body.Name
		g.Version = body.Version
		g.Protocol = body.Protocol
		g.UpdatedBy = &p.UserID
		if err := h.store.UpdateAPIGroup(r.Context(), *g); err != nil {
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
		Name: body.Name, Version: body.Version, Protocol: body.Protocol,
		CreatedBy: p.UserID, CreatedAt: now, UpdatedAt: now,
	}
	uploadedSpec := false
	if body.Spec != "" && h.storage != nil {
		key := storage.APIGroupSpecKey(orgID, serviceID, id)
		if err := h.uploadSpec(r.Context(), key, body.Spec); err != nil {
			httputil.Error(w, r, err)
			return
		}
		g.SpecKey = &key
		g.SpecHash = &newHash
		uploadedSpec = true
	}
	if err := h.store.CreateAPIGroup(r.Context(), g); err != nil {
		httputil.Error(w, r, err)
		return
	}
	if uploadedSpec {
		versionID := uuid.NewString()
		vKey := storage.APIGroupVersionSpecKey(orgID, serviceID, id, versionID)
		if err := h.uploadSpec(r.Context(), vKey, body.Spec); err == nil {
			_ = h.store.CreateAPIGroupVersion(r.Context(), catalogpkg.APIGroupVersion{
				ID: versionID, APIGroupID: id, VersionNumber: 1,
				SpecKey: vKey, SpecHash: newHash, IsAutoVersion: true,
				CreatedBy: p.UserID, CreatedAt: now,
			})
		}
	}
	httputil.JSON(w, http.StatusCreated, map[string]any{"apiGroupId": id, "versionCreated": uploadedSpec})
}

// ListAPIGroupVersions handles GET /api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/versions
func (h *Handler) ListAPIGroupVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := h.store.ListAPIGroupVersions(r.Context(), r.PathValue("apiGroupID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"versions": versions})
}
