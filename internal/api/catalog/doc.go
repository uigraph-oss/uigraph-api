package catalog

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	catalogpkg "github.com/uigraph/app/internal/catalog"
	diagrampkg "github.com/uigraph/app/internal/diagram"
	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
	"github.com/uigraph/app/internal/storage"
)

// ── Service docs ──────────────────────────────────────────────────────────────

func (h *Handler) ListDocs(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	docs, err := h.store.ListServiceDocs(r.Context(), serviceID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"docs": docs})
}

func (h *Handler) GetDoc(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	doc, err := h.store.GetServiceDoc(r.Context(), r.PathValue("docID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if doc == nil || doc.DeletedAt != nil || doc.ServiceID != serviceID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, doc)
}

func (h *Handler) CreateDoc(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	serviceID := r.PathValue("serviceID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if h.storage == nil {
		httputil.BadRequest(w, "storage is not configured")
		return
	}
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}

	fileName, fileType, description, fileBytes, err := h.readServiceDocPayload(r)
	if err != nil {
		httputil.BadRequest(w, err.Error())
		return
	}

	docID := uuid.NewString()
	fileAssetID := storage.NewFileAssetID()
	if err := h.storage.Upload(r.Context(), storage.AssetKey(fileAssetID), fileType, bytes.NewReader(fileBytes), int64(len(fileBytes))); err != nil {
		httputil.Error(w, r, err)
		return
	}

	now := time.Now().UTC()
	doc := catalogpkg.ServiceDoc{
		ID:          docID,
		ServiceID:   serviceID,
		OrgID:       orgID,
		FileAssetID: fileAssetID,
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
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, doc)
}

func (h *Handler) UpdateDoc(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	serviceID := r.PathValue("serviceID")
	docID := r.PathValue("docID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if h.storage == nil {
		httputil.BadRequest(w, "storage is not configured")
		return
	}
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}

	doc, err := h.store.GetServiceDoc(r.Context(), docID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if doc == nil || doc.DeletedAt != nil || doc.ServiceID != serviceID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	fileName, fileType, description, fileBytes, payloadErr := h.readOptionalServiceDocPayload(r, doc)
	if payloadErr != nil {
		httputil.BadRequest(w, payloadErr.Error())
		return
	}

	doc.Description = description
	doc.FileName = fileName
	doc.FileType = fileType
	doc.OrgID = orgID
	doc.UpdatedBy = &p.UserID

	if fileBytes != nil {
		newAssetID := storage.NewFileAssetID()
		if err := h.storage.Upload(r.Context(), storage.AssetKey(newAssetID), fileType, bytes.NewReader(fileBytes), int64(len(fileBytes))); err != nil {
			httputil.Error(w, r, err)
			return
		}
		if doc.FileAssetID != "" {
			_ = h.storage.Delete(r.Context(), storage.AssetKey(doc.FileAssetID))
		}
		doc.FileAssetID = newAssetID
		doc.ContentHash = sha256Bytes(fileBytes)
	}

	if err := h.store.UpdateServiceDoc(r.Context(), *doc); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, doc)
}

func (h *Handler) DeleteDoc(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	docID := r.PathValue("docID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	doc, err := h.store.GetServiceDoc(r.Context(), docID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if doc == nil || doc.DeletedAt != nil || doc.ServiceID != serviceID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if err := h.store.SoftDeleteServiceDoc(r.Context(), docID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Service diagrams ──────────────────────────────────────────────────────────

func (h *Handler) ListDiagrams(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	diagrams, err := h.store.ListServiceDiagrams(r.Context(), serviceID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"diagrams": diagrams})
}

func (h *Handler) CreateDiagram(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	serviceID := r.PathValue("serviceID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
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
		httputil.BadRequest(w, "invalid request body")
		return
	}

	var dg *diagrampkg.Diagram
	if body.DiagramID != nil && strings.TrimSpace(*body.DiagramID) != "" {
		existing, err := h.store.GetDiagram(r.Context(), strings.TrimSpace(*body.DiagramID))
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
		if existing == nil || existing.DeletedAt != nil || existing.OrgID != orgID {
			httputil.Error(w, r, storepkg.ErrNotFound)
			return
		}
		dg = existing
	} else {
		if h.storage == nil {
			httputil.BadRequest(w, "storage is not configured")
			return
		}
		if body.Name == nil || strings.TrimSpace(*body.Name) == "" || body.Content == nil || strings.TrimSpace(*body.Content) == "" {
			httputil.BadRequest(w, "name and content are required when diagramId is not provided")
			return
		}

		id := uuid.NewString()
		content := *body.Content
		contentKey := storage.DiagramContentKey(orgID, id)
		if err := h.uploadSpec(r.Context(), contentKey, content); err != nil {
			httputil.Error(w, r, err)
			return
		}

		now := time.Now().UTC()
		newDiagram := diagrampkg.Diagram{
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
			httputil.Error(w, r, err)
			return
		}

		versionID := uuid.NewString()
		versionKey := storage.DiagramVersionKey(orgID, id, versionID)
		if err := h.uploadSpec(r.Context(), versionKey, content); err != nil {
			httputil.Error(w, r, err)
			return
		}
		_ = h.store.CreateDiagramVersion(r.Context(), diagrampkg.Version{
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
	link := catalogpkg.ServiceDiagram{
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
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, link)
}

func (h *Handler) DeleteDiagram(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	diagramID := r.PathValue("diagramID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	link, err := h.store.GetServiceDiagram(r.Context(), serviceID, diagramID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if link == nil || link.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if err := h.store.SoftDeleteServiceDiagram(r.Context(), serviceID, diagramID, p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
