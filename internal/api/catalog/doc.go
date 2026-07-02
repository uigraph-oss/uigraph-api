package catalog

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	catalogpkg "github.com/uigraph/app/internal/catalog"
	diagrampkg "github.com/uigraph/app/internal/diagram"
	docspkg "github.com/uigraph/app/internal/docs"
	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/storage"
	storepkg "github.com/uigraph/app/internal/store"
)

// ── Service docs ──────────────────────────────────────────────────────────────

// ListDocs
// @Summary  ListDocs
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/docs [get]
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

// CreateDoc
// @Summary  CreateDoc
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
// @Router   /orgs/{orgID}/services/{serviceID}/docs [post]
func (h *Handler) CreateDoc(w http.ResponseWriter, r *http.Request) {
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
		DocID         *string `json:"docId"`
		FileName      *string `json:"fileName"`
		FileType      *string `json:"fileType"`
		Description   *string `json:"description"`
		ContentBase64 *string `json:"contentBase64"`
		FolderID      *string `json:"folderId"`
		TeamID        *string `json:"teamId"`
		CommitHash    *string `json:"commitHash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}

	var doc *docspkg.Doc
	if body.DocID != nil && strings.TrimSpace(*body.DocID) != "" {
		existing, err := h.store.GetDoc(r.Context(), strings.TrimSpace(*body.DocID))
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
		if existing == nil || existing.DeletedAt != nil || existing.OrgID != orgID {
			httputil.Error(w, r, storepkg.ErrNotFound)
			return
		}
		doc = existing
	} else {
		if h.storage == nil {
			httputil.BadRequest(w, "storage is not configured")
			return
		}
		if body.FileName == nil || strings.TrimSpace(*body.FileName) == "" || body.ContentBase64 == nil || strings.TrimSpace(*body.ContentBase64) == "" {
			httputil.BadRequest(w, "fileName and contentBase64 are required when docId is not provided")
			return
		}
		fileBytes, err := base64.StdEncoding.DecodeString(*body.ContentBase64)
		if err != nil {
			httputil.BadRequest(w, "contentBase64 must be valid base64")
			return
		}

		fileType := "application/octet-stream"
		if body.FileType != nil && strings.TrimSpace(*body.FileType) != "" {
			fileType = strings.TrimSpace(*body.FileType)
		}
		description := ""
		if body.Description != nil {
			description = strings.TrimSpace(*body.Description)
		}

		fileAssetID := storage.NewFileAssetID()
		if err := h.storage.Upload(r.Context(), storage.AssetKey(fileAssetID), fileType, bytes.NewReader(fileBytes), int64(len(fileBytes))); err != nil {
			httputil.Error(w, r, err)
			return
		}

		now := time.Now().UTC()
		newDoc := docspkg.Doc{
			ID:          uuid.NewString(),
			OrgID:       orgID,
			FolderID:    nonEmptyPtr(body.FolderID),
			TeamID:      nonEmptyPtr(body.TeamID),
			FileAssetID: fileAssetID,
			FileName:    strings.TrimSpace(*body.FileName),
			FileType:    fileType,
			Description: description,
			ContentHash: sha256Bytes(fileBytes),
			CreatedBy:   p.UserID,
			UpdatedBy:   &p.UserID,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := h.store.CreateDoc(r.Context(), newDoc); err != nil {
			httputil.Error(w, r, err)
			return
		}
		doc = &newDoc
	}

	now := time.Now().UTC()
	link := catalogpkg.ServiceDoc{
		ServiceID:           serviceID,
		DocID:               doc.ID,
		OrgID:               orgID,
		CreatedBy:           p.UserID,
		UpdatedBy:           &p.UserID,
		CreatedByCommitHash: body.CommitHash,
		UpdatedByCommitHash: body.CommitHash,
		CreatedAt:           now,
		UpdatedAt:           now,
		Doc:                 doc,
	}
	if err := h.store.CreateServiceDoc(r.Context(), link); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, link)
}

// DeleteDoc
// @Summary  DeleteDoc
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    docID  path  string  true  "docID"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/docs/{docID} [delete]
func (h *Handler) DeleteDoc(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	docID := r.PathValue("docID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	link, err := h.store.GetServiceDoc(r.Context(), serviceID, docID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if link == nil || link.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if err := h.store.SoftDeleteServiceDoc(r.Context(), serviceID, docID, p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func nonEmptyPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}

// ── Service diagrams ──────────────────────────────────────────────────────────

// ListDiagrams
// @Summary  ListDiagrams
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/diagrams [get]
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

// CreateDiagram
// @Summary  CreateDiagram
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
// @Router   /orgs/{orgID}/services/{serviceID}/diagrams [post]
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
		DiagramID  *string `json:"diagramId"`
		Name       *string `json:"name"`
		Content    *string `json:"content"`
		FolderID   *string `json:"folderId"`
		TeamID     *string `json:"teamId"`
		Source     *string `json:"source"`
		CommitHash *string `json:"commitHash"`
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

		h.enqueueScreenshot(r.Context(), orgID, id)
		dg = &newDiagram
	}

	now := time.Now().UTC()
	link := catalogpkg.ServiceDiagram{
		ServiceID:           serviceID,
		DiagramID:           dg.ID,
		OrgID:               orgID,
		CreatedBy:           p.UserID,
		UpdatedBy:           &p.UserID,
		CreatedByCommitHash: body.CommitHash,
		UpdatedByCommitHash: body.CommitHash,
		CreatedAt:           now,
		UpdatedAt:           now,
		Diagram:             dg,
	}
	if err := h.store.CreateServiceDiagram(r.Context(), link); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, link)
}

// DeleteDiagram
// @Summary  DeleteDiagram
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    diagramID  path  string  true  "diagramID"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/diagrams/{diagramID} [delete]
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
