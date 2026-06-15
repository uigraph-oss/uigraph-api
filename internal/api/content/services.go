package content

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/uigraph/app/internal/catalog"
	diagramdom "github.com/uigraph/app/internal/diagram"
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

func (h *ServiceHandler) ListStats(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	var serviceID *string
	if v := r.URL.Query().Get("serviceId"); v != "" {
		serviceID = &v
	}

	stats, err := h.store.ListServiceStats(r.Context(), orgID, serviceID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stats": stats})
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

// ── Service docs ──────────────────────────────────────────────────────────────

func (h *ServiceHandler) ListDocs(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	docs, err := h.store.ListServiceDocs(r.Context(), serviceID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"docs": docs})
}

func (h *ServiceHandler) GetDoc(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	doc, err := h.store.GetServiceDoc(r.Context(), r.PathValue("docID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if doc == nil || doc.DeletedAt != nil || doc.ServiceID != serviceID {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (h *ServiceHandler) CreateDoc(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	serviceID := r.PathValue("serviceID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if h.storage == nil {
		writeErr(w, http.StatusInternalServerError, "storage is not configured")
		return
	}
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}

	fileName, fileType, description, fileBytes, err := h.readServiceDocPayload(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	docID := uuid.NewString()
	fileKey := storage.ServiceDocFileKey(orgID, serviceID, docID, fileName)
	if err := h.storage.Upload(r.Context(), fileKey, fileType, bytes.NewReader(fileBytes), int64(len(fileBytes))); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to store doc file")
		return
	}

	now := time.Now().UTC()
	doc := catalog.ServiceDoc{
		ID:          docID,
		ServiceID:   serviceID,
		OrgID:       orgID,
		FileKey:     fileKey,
		FileName:    fileName,
		FileType:    fileType,
		Description: description,
		ContentHash: sha256Bytes(fileBytes),
		CreatedBy:   p.UserID,
		UpdatedBy:   &p.UserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.store.CreateServiceDoc(r.Context(), doc); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, doc)
}

func (h *ServiceHandler) UpdateDoc(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	serviceID := r.PathValue("serviceID")
	docID := r.PathValue("docID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if h.storage == nil {
		writeErr(w, http.StatusInternalServerError, "storage is not configured")
		return
	}
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}

	doc, err := h.store.GetServiceDoc(r.Context(), docID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if doc == nil || doc.DeletedAt != nil || doc.ServiceID != serviceID {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	fileName, fileType, description, fileBytes, payloadErr := h.readOptionalServiceDocPayload(r, doc)
	if payloadErr != nil {
		writeErr(w, http.StatusBadRequest, payloadErr.Error())
		return
	}

	doc.Description = description
	doc.FileName = fileName
	doc.FileType = fileType
	doc.OrgID = orgID
	doc.UpdatedBy = &p.UserID

	if fileBytes != nil {
		newFileKey := storage.ServiceDocFileKey(orgID, serviceID, docID, fileName)
		if err := h.storage.Upload(r.Context(), newFileKey, fileType, bytes.NewReader(fileBytes), int64(len(fileBytes))); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to store doc file")
			return
		}
		if doc.FileKey != newFileKey {
			_ = h.storage.Delete(r.Context(), doc.FileKey)
		}
		doc.FileKey = newFileKey
		doc.ContentHash = sha256Bytes(fileBytes)
	}

	if err := h.store.UpdateServiceDoc(r.Context(), *doc); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (h *ServiceHandler) DeleteDoc(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	docID := r.PathValue("docID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	doc, err := h.store.GetServiceDoc(r.Context(), docID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if doc == nil || doc.DeletedAt != nil || doc.ServiceID != serviceID {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err := h.store.SoftDeleteServiceDoc(r.Context(), docID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Service diagrams ──────────────────────────────────────────────────────────

func (h *ServiceHandler) ListDiagrams(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	diagrams, err := h.store.ListServiceDiagrams(r.Context(), serviceID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"diagrams": diagrams})
}

func (h *ServiceHandler) CreateDiagram(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	serviceID := r.PathValue("serviceID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}

	var body struct {
		DiagramID *string `json:"diagramId"`
		Name      *string `json:"name"`
		Content   *string `json:"content"`
		FolderID  *string `json:"folderId"`
		TeamID    *string `json:"teamId"`
		Source    *string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var dg *diagramdom.Diagram
	if body.DiagramID != nil && strings.TrimSpace(*body.DiagramID) != "" {
		existing, err := h.store.GetDiagram(r.Context(), strings.TrimSpace(*body.DiagramID))
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		if existing == nil || existing.DeletedAt != nil || existing.OrgID != orgID {
			writeErr(w, http.StatusNotFound, "diagram not found")
			return
		}
		dg = existing
	} else {
		if h.storage == nil {
			writeErr(w, http.StatusInternalServerError, "storage is not configured")
			return
		}
		if body.Name == nil || strings.TrimSpace(*body.Name) == "" || body.Content == nil || strings.TrimSpace(*body.Content) == "" {
			writeErr(w, http.StatusBadRequest, "name and content are required when diagramId is not provided")
			return
		}

		id := uuid.NewString()
		content := *body.Content
		contentKey := storage.DiagramContentKey(orgID, id)
		if err := h.uploadSpec(r.Context(), contentKey, content); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to store content")
			return
		}

		now := time.Now().UTC()
		newDiagram := diagramdom.Diagram{
			ID:          id,
			OrgID:       orgID,
			FolderID:    body.FolderID,
			TeamID:      body.TeamID,
			Name:        strings.TrimSpace(*body.Name),
			ContentKey:  contentKey,
			ContentHash: specHash(content),
			Source:      body.Source,
			CreatedBy:   p.UserID,
			UpdatedBy:   &p.UserID,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := h.store.CreateDiagram(r.Context(), newDiagram); err != nil {
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}

		versionID := uuid.NewString()
		versionKey := storage.DiagramVersionKey(orgID, id, versionID)
		if err := h.uploadSpec(r.Context(), versionKey, content); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to store version content")
			return
		}
		_ = h.store.CreateDiagramVersion(r.Context(), diagramdom.Version{
			ID:            versionID,
			DiagramID:     id,
			VersionNumber: 1,
			ContentKey:    versionKey,
			ContentHash:   newDiagram.ContentHash,
			IsAutoVersion: body.Source != nil,
			Source:        body.Source,
			CreatedBy:     p.UserID,
			CreatedAt:     now,
		})

		dg = &newDiagram
	}

	now := time.Now().UTC()
	link := catalog.ServiceDiagram{
		ServiceID: serviceID,
		DiagramID: dg.ID,
		OrgID:     orgID,
		CreatedBy: p.UserID,
		UpdatedBy: &p.UserID,
		CreatedAt: now,
		UpdatedAt: now,
		Diagram:   dg,
	}
	if err := h.store.CreateServiceDiagram(r.Context(), link); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

func (h *ServiceHandler) DeleteDiagram(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	diagramID := r.PathValue("diagramID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	link, err := h.store.GetServiceDiagram(r.Context(), serviceID, diagramID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if link == nil || link.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err := h.store.SoftDeleteServiceDiagram(r.Context(), serviceID, diagramID, p.UserID); err != nil {
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

// SyncAPIGroup handles POST /api/v1/orgs/{orgID}/services/{serviceID}/api-groups/sync
// CLI upsert: creates or updates an API group, skipping the spec upload when hash is unchanged.
func (h *ServiceHandler) SyncAPIGroup(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
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

	newHash := specHash(body.Spec)

	// Update path.
	if body.APIGroupID != nil {
		g, err := h.store.GetAPIGroup(r.Context(), *body.APIGroupID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		if g == nil || g.DeletedAt != nil {
			writeErr(w, http.StatusNotFound, "api group not found")
			return
		}
		if body.Spec != "" && g.SpecHash != nil && newHash == *g.SpecHash {
			writeJSON(w, http.StatusOK, map[string]any{"apiGroupId": g.ID, "versionCreated": false})
			return
		}
		if body.Spec != "" && h.storage != nil {
			key := storage.APIGroupSpecKey(orgID, serviceID, g.ID)
			if err := h.uploadSpec(r.Context(), key, body.Spec); err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to store spec")
				return
			}
			g.SpecKey = &key
			g.SpecHash = &newHash
			latestVer, _ := h.store.LatestAPIGroupVersionNumber(r.Context(), g.ID)
			versionID := uuid.NewString()
			vKey := storage.APIGroupVersionSpecKey(orgID, serviceID, g.ID, versionID)
			if err := h.uploadSpec(r.Context(), vKey, body.Spec); err == nil {
				_ = h.store.CreateAPIGroupVersion(r.Context(), catalog.APIGroupVersion{
					ID: versionID, APIGroupID: g.ID, VersionNumber: latestVer + 1,
					SpecKey: vKey, SpecHash: newHash, IsAutoVersion: true,
					CreatedBy: p.UserID, CreatedAt: time.Now().UTC(),
				})
			}
		}
		g.Name = body.Name
		g.Version = body.Version
		g.Protocol = body.Protocol
		g.UpdatedBy = &p.UserID
		if err := h.store.UpdateAPIGroup(r.Context(), *g); err != nil {
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"apiGroupId": g.ID, "versionCreated": true})
		return
	}

	// Create path.
	id := uuid.NewString()
	now := time.Now().UTC()
	g := catalog.APIGroup{
		ID: id, ServiceID: serviceID, OrgID: orgID,
		Name: body.Name, Version: body.Version, Protocol: body.Protocol,
		CreatedBy: p.UserID, CreatedAt: now, UpdatedAt: now,
	}
	uploadedSpec := false
	if body.Spec != "" && h.storage != nil {
		key := storage.APIGroupSpecKey(orgID, serviceID, id)
		if err := h.uploadSpec(r.Context(), key, body.Spec); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to store spec")
			return
		}
		g.SpecKey = &key
		g.SpecHash = &newHash
		uploadedSpec = true
	}
	if err := h.store.CreateAPIGroup(r.Context(), g); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if uploadedSpec {
		versionID := uuid.NewString()
		vKey := storage.APIGroupVersionSpecKey(orgID, serviceID, id, versionID)
		if err := h.uploadSpec(r.Context(), vKey, body.Spec); err == nil {
			_ = h.store.CreateAPIGroupVersion(r.Context(), catalog.APIGroupVersion{
				ID: versionID, APIGroupID: id, VersionNumber: 1,
				SpecKey: vKey, SpecHash: newHash, IsAutoVersion: true,
				CreatedBy: p.UserID, CreatedAt: now,
			})
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"apiGroupId": id, "versionCreated": uploadedSpec})
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

func sha256Bytes(b []byte) string {
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum)
}

func (h *ServiceHandler) ensureServiceInOrg(w http.ResponseWriter, r *http.Request, serviceID string) bool {
	svc, err := h.store.GetService(r.Context(), serviceID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return false
	}
	if svc == nil || svc.DeletedAt != nil || svc.OrgID != r.PathValue("orgID") {
		writeErr(w, http.StatusNotFound, "not found")
		return false
	}
	return true
}

func (h *ServiceHandler) readServiceDocPayload(r *http.Request) (string, string, string, []byte, error) {
	fileName, fileType, description, fileBytes, err := h.readOptionalServiceDocPayload(r, nil)
	if err != nil {
		return "", "", "", nil, err
	}
	if len(fileBytes) == 0 {
		return "", "", "", nil, fmt.Errorf("file content is required")
	}
	return fileName, fileType, description, fileBytes, nil
}

func (h *ServiceHandler) readOptionalServiceDocPayload(r *http.Request, existing *catalog.ServiceDoc) (string, string, string, []byte, error) {
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		return readServiceDocFromMultipart(r, existing)
	}
	return readServiceDocFromJSON(r, existing)
}

func readServiceDocFromJSON(r *http.Request, existing *catalog.ServiceDoc) (string, string, string, []byte, error) {
	var body struct {
		FileName      *string `json:"fileName"`
		FileType      *string `json:"fileType"`
		Description   *string `json:"description"`
		ContentBase64 *string `json:"contentBase64"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return "", "", "", nil, fmt.Errorf("invalid request body")
	}

	fileName, fileType, description := "", "", ""
	if existing != nil {
		fileName, fileType, description = existing.FileName, existing.FileType, existing.Description
	}
	if body.FileName != nil {
		fileName = strings.TrimSpace(*body.FileName)
	}
	if body.FileType != nil {
		fileType = strings.TrimSpace(*body.FileType)
	}
	if body.Description != nil {
		description = strings.TrimSpace(*body.Description)
	}
	if fileType == "" {
		fileType = "application/octet-stream"
	}
	if fileName == "" {
		return "", "", "", nil, fmt.Errorf("fileName is required")
	}
	var out []byte
	if body.ContentBase64 != nil && strings.TrimSpace(*body.ContentBase64) != "" {
		raw, err := base64.StdEncoding.DecodeString(*body.ContentBase64)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("contentBase64 must be valid base64")
		}
		out = raw
	}
	return fileName, fileType, description, out, nil
}

func readServiceDocFromMultipart(r *http.Request, existing *catalog.ServiceDoc) (string, string, string, []byte, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return "", "", "", nil, fmt.Errorf("invalid multipart form")
	}
	fileName, fileType, description := "", "", ""
	if existing != nil {
		fileName, fileType, description = existing.FileName, existing.FileType, existing.Description
	}
	if v := strings.TrimSpace(r.FormValue("fileName")); v != "" {
		fileName = v
	}
	if v := strings.TrimSpace(r.FormValue("fileType")); v != "" {
		fileType = v
	}
	if v := r.FormValue("description"); v != "" {
		description = strings.TrimSpace(v)
	}
	if fileType == "" {
		fileType = "application/octet-stream"
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		if existing != nil {
			if fileName == "" {
				return "", "", "", nil, fmt.Errorf("fileName is required")
			}
			return fileName, fileType, description, nil, nil
		}
		return "", "", "", nil, fmt.Errorf("file is required")
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("failed to read file")
	}
	if fileName == "" {
		fileName = strings.TrimSpace(header.Filename)
	}
	if fileName == "" {
		return "", "", "", nil, fmt.Errorf("fileName is required")
	}
	if fileType == "application/octet-stream" {
		if headerType := strings.TrimSpace(header.Header.Get("Content-Type")); headerType != "" {
			fileType = headerType
		}
	}
	return fileName, fileType, description, content, nil
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
