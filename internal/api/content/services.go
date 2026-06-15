package content

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/uigraph/app/internal/catalog"
	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/storage"
	"github.com/uigraph/app/internal/store"
)

// ServiceHandler serves /api/v1/orgs/{orgID}/services and all nested resources.
type ServiceHandler struct {
	store   store.Store
	storage storage.Client // may be nil
}

func NewServiceHandler(s store.Store, st storage.Client) *ServiceHandler {
	return &ServiceHandler{store: s, storage: st}
}

// ── Services ──────────────────────────────────────────────────────────────────

func (h *ServiceHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	q := r.URL.Query()
	var folderID, teamID *string
	if v := q.Get("folderId"); v != "" {
		folderID = &v
	}
	if v := q.Get("teamId"); v != "" {
		teamID = &v
	}
	svcs, err := h.store.ListServices(r.Context(), orgID, folderID, teamID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": svcs})
}

func (h *ServiceHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var body struct {
		Name            string          `json:"name"`
		Slug            string          `json:"slug"`
		Description     string          `json:"description"`
		Status          string          `json:"status"`
		Tier            string          `json:"tier"`
		Category        string          `json:"category"`
		Language        string          `json:"language"`
		FolderID        *string         `json:"folderId"`
		TeamID          *string         `json:"teamId"`
		GitRepoURL      *string         `json:"gitRepoUrl"`
		JiraProjectURL  *string         `json:"jiraProjectUrl"`
		SlackChannelURL *string         `json:"slackChannelUrl"`
		Labels          []string        `json:"labels"`
		Metadata        json.RawMessage `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if body.Slug == "" {
		body.Slug = toSlug(body.Name)
	}
	if body.Status == "" {
		body.Status = "active"
	}
	if body.Tier == "" {
		body.Tier = "tier3"
	}

	now := time.Now().UTC()
	svc := catalog.Service{
		ID:              uuid.NewString(),
		OrgID:           orgID,
		FolderID:        body.FolderID,
		TeamID:          body.TeamID,
		Name:            body.Name,
		Slug:            body.Slug,
		Description:     body.Description,
		Status:          body.Status,
		Tier:            body.Tier,
		Category:        body.Category,
		Language:        body.Language,
		GitRepoURL:      body.GitRepoURL,
		JiraProjectURL:  body.JiraProjectURL,
		SlackChannelURL: body.SlackChannelURL,
		Labels:          body.Labels,
		Metadata:        body.Metadata,
		CreatedBy:       p.UserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := h.store.CreateService(r.Context(), svc); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, svc)
}

func (h *ServiceHandler) Get(w http.ResponseWriter, r *http.Request) {
	svc, err := h.store.GetService(r.Context(), r.PathValue("serviceID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if svc == nil || svc.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, svc)
}

func (h *ServiceHandler) Update(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	svc, err := h.store.GetService(r.Context(), r.PathValue("serviceID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if svc == nil || svc.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	var body struct {
		Name            *string         `json:"name"`
		Slug            *string         `json:"slug"`
		Description     *string         `json:"description"`
		Status          *string         `json:"status"`
		Tier            *string         `json:"tier"`
		Category        *string         `json:"category"`
		Language        *string         `json:"language"`
		FolderID        *string         `json:"folderId"`
		TeamID          *string         `json:"teamId"`
		GitRepoURL      *string         `json:"gitRepoUrl"`
		JiraProjectURL  *string         `json:"jiraProjectUrl"`
		SlackChannelURL *string         `json:"slackChannelUrl"`
		LastCommitSha   *string         `json:"lastCommitSha"`
		Labels          []string        `json:"labels"`
		Metadata        json.RawMessage `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name != nil {
		svc.Name = *body.Name
	}
	if body.Slug != nil {
		svc.Slug = *body.Slug
	}
	if body.Description != nil {
		svc.Description = *body.Description
	}
	if body.Status != nil {
		svc.Status = *body.Status
	}
	if body.Tier != nil {
		svc.Tier = *body.Tier
	}
	if body.Category != nil {
		svc.Category = *body.Category
	}
	if body.Language != nil {
		svc.Language = *body.Language
	}
	if body.FolderID != nil {
		svc.FolderID = body.FolderID
	}
	if body.TeamID != nil {
		svc.TeamID = body.TeamID
	}
	if body.GitRepoURL != nil {
		svc.GitRepoURL = body.GitRepoURL
	}
	if body.JiraProjectURL != nil {
		svc.JiraProjectURL = body.JiraProjectURL
	}
	if body.SlackChannelURL != nil {
		svc.SlackChannelURL = body.SlackChannelURL
	}
	if body.LastCommitSha != nil {
		svc.LastCommitSha = body.LastCommitSha
	}
	if body.Labels != nil {
		svc.Labels = body.Labels
	}
	if body.Metadata != nil {
		svc.Metadata = body.Metadata
	}
	svc.UpdatedBy = &p.UserID

	if err := h.store.UpdateService(r.Context(), *svc); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, svc)
}

func (h *ServiceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if err := h.store.SoftDeleteService(r.Context(), r.PathValue("serviceID"), p.UserID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── API Groups ────────────────────────────────────────────────────────────────

func (h *ServiceHandler) ListAPIGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.store.ListAPIGroups(r.Context(), r.PathValue("serviceID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"apiGroups": groups})
}

func (h *ServiceHandler) CreateAPIGroup(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var body struct {
		Name     string  `json:"name"`
		Version  string  `json:"version"`
		Label    *string `json:"label"`
		Protocol string  `json:"protocol"`
		Spec     string  `json:"spec"` // spec content (OpenAPI YAML/JSON, GraphQL SDL, proto)
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
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
	g := catalog.APIGroup{
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
			writeErr(w, http.StatusInternalServerError, "failed to store spec")
			return
		}
		hash := specHash(body.Spec)
		g.SpecKey = &key
		g.SpecHash = &hash
		specContent = body.Spec
	}

	if err := h.store.CreateAPIGroup(r.Context(), g); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Auto-version 1 — created after the parent row is committed.
	if specContent != "" {
		hash := specHash(specContent)
		versionID := uuid.NewString()
		vKey := storage.APIGroupVersionSpecKey(orgID, serviceID, id, versionID)
		if err := h.uploadSpec(r.Context(), vKey, specContent); err == nil {
			_ = h.store.CreateAPIGroupVersion(r.Context(), catalog.APIGroupVersion{
				ID: versionID, APIGroupID: id, VersionNumber: 1,
				SpecKey: vKey, SpecHash: hash, IsAutoVersion: true,
				CreatedBy: p.UserID, CreatedAt: now,
			})
		}
	}

	writeJSON(w, http.StatusCreated, g)
}

func (h *ServiceHandler) GetAPIGroup(w http.ResponseWriter, r *http.Request) {
	g, err := h.store.GetAPIGroup(r.Context(), r.PathValue("apiGroupID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if g == nil || g.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (h *ServiceHandler) UpdateAPIGroup(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	serviceID := r.PathValue("serviceID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	g, err := h.store.GetAPIGroup(r.Context(), r.PathValue("apiGroupID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if g == nil || g.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
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
		writeErr(w, http.StatusBadRequest, "invalid request body")
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
				writeErr(w, http.StatusInternalServerError, "failed to store spec")
				return
			}
			g.SpecKey = &key
			g.SpecHash = &newHash

			latestVer, _ := h.store.LatestAPIGroupVersionNumber(r.Context(), g.ID)
			versionID := uuid.NewString()
			vKey := storage.APIGroupVersionSpecKey(orgID, serviceID, g.ID, versionID)
			if err := h.uploadSpec(r.Context(), vKey, *body.Spec); err == nil {
				_ = h.store.CreateAPIGroupVersion(r.Context(), catalog.APIGroupVersion{
					ID: versionID, APIGroupID: g.ID, VersionNumber: latestVer + 1,
					SpecKey: vKey, SpecHash: newHash, IsAutoVersion: true,
					CreatedBy: p.UserID, CreatedAt: time.Now().UTC(),
				})
			}
		}
	}

	if err := h.store.UpdateAPIGroup(r.Context(), *g); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (h *ServiceHandler) DeleteAPIGroup(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if err := h.store.SoftDeleteAPIGroup(r.Context(), r.PathValue("apiGroupID"), p.UserID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListAPIGroupVersions handles GET /api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/versions
func (h *ServiceHandler) ListAPIGroupVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := h.store.ListAPIGroupVersions(r.Context(), r.PathValue("apiGroupID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": versions})
}

// ── API Endpoints ─────────────────────────────────────────────────────────────

func (h *ServiceHandler) ListAPIEndpoints(w http.ResponseWriter, r *http.Request) {
	endpoints, err := h.store.ListAPIEndpoints(r.Context(), r.PathValue("apiGroupID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"endpoints": endpoints})
}

func (h *ServiceHandler) CreateAPIEndpoint(w http.ResponseWriter, r *http.Request) {
	apiGroupID := r.PathValue("apiGroupID")
	serviceID := r.PathValue("serviceID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var body struct {
		OperationID string          `json:"operationId"`
		Method      string          `json:"method"`
		Path        string          `json:"path"`
		Summary     string          `json:"summary"`
		Description string          `json:"description"`
		Tags        []string        `json:"tags"`
		Parameters  json.RawMessage `json:"parameters"`
		RequestBody json.RawMessage `json:"requestBody"`
		Responses   json.RawMessage `json:"responses"`
		Order       float64         `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Method == "" || body.Path == "" {
		writeErr(w, http.StatusBadRequest, "method and path are required")
		return
	}

	now := time.Now().UTC()
	e := catalog.APIEndpoint{
		ID:          uuid.NewString(),
		APIGroupID:  apiGroupID,
		ServiceID:   serviceID,
		OrgID:       orgID,
		OperationID: body.OperationID,
		Method:      body.Method,
		Path:        body.Path,
		Summary:     body.Summary,
		Description: body.Description,
		Tags:        body.Tags,
		Parameters:  body.Parameters,
		RequestBody: body.RequestBody,
		Responses:   body.Responses,
		Order:       body.Order,
		CreatedBy:   p.UserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.store.CreateAPIEndpoint(r.Context(), e); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, e)
}

func (h *ServiceHandler) GetAPIEndpoint(w http.ResponseWriter, r *http.Request) {
	e, err := h.store.GetAPIEndpoint(r.Context(), r.PathValue("endpointID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if e == nil || e.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (h *ServiceHandler) UpdateAPIEndpoint(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	e, err := h.store.GetAPIEndpoint(r.Context(), r.PathValue("endpointID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if e == nil || e.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	var body struct {
		OperationID *string         `json:"operationId"`
		Method      *string         `json:"method"`
		Path        *string         `json:"path"`
		Summary     *string         `json:"summary"`
		Description *string         `json:"description"`
		Tags        []string        `json:"tags"`
		Parameters  json.RawMessage `json:"parameters"`
		RequestBody json.RawMessage `json:"requestBody"`
		Responses   json.RawMessage `json:"responses"`
		Order       *float64        `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.OperationID != nil {
		e.OperationID = *body.OperationID
	}
	if body.Method != nil {
		e.Method = *body.Method
	}
	if body.Path != nil {
		e.Path = *body.Path
	}
	if body.Summary != nil {
		e.Summary = *body.Summary
	}
	if body.Description != nil {
		e.Description = *body.Description
	}
	if body.Tags != nil {
		e.Tags = body.Tags
	}
	if body.Parameters != nil {
		e.Parameters = body.Parameters
	}
	if body.RequestBody != nil {
		e.RequestBody = body.RequestBody
	}
	if body.Responses != nil {
		e.Responses = body.Responses
	}
	if body.Order != nil {
		e.Order = *body.Order
	}
	e.UpdatedBy = &p.UserID

	if err := h.store.UpdateAPIEndpoint(r.Context(), *e); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (h *ServiceHandler) DeleteAPIEndpoint(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if err := h.store.SoftDeleteAPIEndpoint(r.Context(), r.PathValue("endpointID"), p.UserID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── internal helpers ──────────────────────────────────────────────────────────

func (h *ServiceHandler) uploadSpec(ctx context.Context, key, content string) error {
	r := strings.NewReader(content)
	return h.storage.Upload(ctx, key, "application/octet-stream", r, int64(r.Len()))
}

func specHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}

// toSlug converts a name to a URL-safe slug (lowercase, hyphens).
func toSlug(name string) string {
	s := strings.ToLower(name)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.' {
			return r
		}
		return '-'
	}, s)
	// Collapse consecutive hyphens.
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}
