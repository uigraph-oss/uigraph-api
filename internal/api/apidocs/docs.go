package docs

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	docspkg "github.com/uigraph/app/internal/docs"
	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
	"github.com/uigraph/app/internal/storage"
)

// List handles GET /api/v1/orgs/{orgID}/docs
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	q := r.URL.Query()
	p := docspkg.ListParams{
		SortBy:  q.Get("sortBy"),
		SortDir: q.Get("sortDir"),
	}
	if v := q.Get("limit"); v != "" {
		p.Limit = httputil.ListLimit(v)
		p.Offset = httputil.ListOffset(q.Get("offset"))
	}
	if v := strings.TrimSpace(q.Get("folderId")); v != "" {
		p.FolderID = &v
	}
	if v := strings.TrimSpace(q.Get("teamId")); v != "" {
		p.TeamID = &v
	}
	if v := strings.TrimSpace(q.Get("search")); v != "" {
		p.Search = &v
	}
	list, total, err := h.store.ListDocs(r.Context(), orgID, p)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"docs": list, "total": total})
}

// Get handles GET /api/v1/orgs/{orgID}/docs/{docID}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	doc, err := h.store.GetDoc(r.Context(), r.PathValue("docID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if doc == nil || doc.DeletedAt != nil || doc.OrgID != orgID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, doc)
}

// Create handles POST /api/v1/orgs/{orgID}/docs
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if h.storage == nil {
		httputil.BadRequest(w, "storage is not configured")
		return
	}

	body, err := decodeDocPayload(r, nil)
	if err != nil {
		httputil.BadRequest(w, err.Error())
		return
	}
	if len(body.fileBytes) == 0 {
		httputil.BadRequest(w, "file content is required")
		return
	}

	fileAssetID := storage.NewFileAssetID()
	if err := h.storage.Upload(r.Context(), storage.AssetKey(fileAssetID), body.fileType, bytes.NewReader(body.fileBytes), int64(len(body.fileBytes))); err != nil {
		httputil.Error(w, r, err)
		return
	}

	now := time.Now().UTC()
	doc := docspkg.Doc{
		ID:          uuid.NewString(),
		OrgID:       orgID,
		FolderID:    body.folderID,
		TeamID:      body.teamID,
		FileAssetID: fileAssetID,
		FileName:    body.fileName,
		FileType:    body.fileType,
		Description: body.description,
		ContentHash: sha256Bytes(body.fileBytes),
		CreatedBy:   p.UserID,
		UpdatedBy:   &p.UserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.store.CreateDoc(r.Context(), doc); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, doc)
}

// Update handles PUT /api/v1/orgs/{orgID}/docs/{docID}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if h.storage == nil {
		httputil.BadRequest(w, "storage is not configured")
		return
	}

	doc, err := h.store.GetDoc(r.Context(), r.PathValue("docID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if doc == nil || doc.DeletedAt != nil || doc.OrgID != orgID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	body, err := decodeDocPayload(r, doc)
	if err != nil {
		httputil.BadRequest(w, err.Error())
		return
	}

	doc.FileName = body.fileName
	doc.FileType = body.fileType
	doc.Description = body.description
	doc.FolderID = body.folderID
	doc.TeamID = body.teamID
	doc.UpdatedBy = &p.UserID

	if len(body.fileBytes) > 0 {
		newAssetID := storage.NewFileAssetID()
		if err := h.storage.Upload(r.Context(), storage.AssetKey(newAssetID), body.fileType, bytes.NewReader(body.fileBytes), int64(len(body.fileBytes))); err != nil {
			httputil.Error(w, r, err)
			return
		}
		if doc.FileAssetID != "" {
			_ = h.storage.Delete(r.Context(), storage.AssetKey(doc.FileAssetID))
		}
		doc.FileAssetID = newAssetID
		doc.ContentHash = sha256Bytes(body.fileBytes)
	}

	if err := h.store.UpdateDoc(r.Context(), *doc); err != nil {
		httputil.Error(w, r, err)
		return
	}
	doc.UpdatedAt = time.Now().UTC()
	httputil.JSON(w, http.StatusOK, doc)
}

// Delete handles DELETE /api/v1/orgs/{orgID}/docs/{docID}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	doc, err := h.store.GetDoc(r.Context(), r.PathValue("docID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if doc == nil || doc.DeletedAt != nil || doc.OrgID != orgID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if err := h.store.SoftDeleteDoc(r.Context(), doc.ID, p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type docPayload struct {
	fileName    string
	fileType    string
	description string
	folderID    *string
	teamID      *string
	fileBytes   []byte
}

func decodeDocPayload(r *http.Request, existing *docspkg.Doc) (docPayload, error) {
	var body struct {
		FileName      *string `json:"fileName"`
		FileType      *string `json:"fileType"`
		Description   *string `json:"description"`
		FolderID      *string `json:"folderId"`
		TeamID        *string `json:"teamId"`
		ContentBase64 *string `json:"contentBase64"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return docPayload{}, fmt.Errorf("invalid request body")
	}

	out := docPayload{}
	if existing != nil {
		out.fileName = existing.FileName
		out.fileType = existing.FileType
		out.description = existing.Description
		out.folderID = existing.FolderID
		out.teamID = existing.TeamID
	}
	if body.FileName != nil {
		out.fileName = strings.TrimSpace(*body.FileName)
	}
	if body.FileType != nil {
		out.fileType = strings.TrimSpace(*body.FileType)
	}
	if body.Description != nil {
		out.description = strings.TrimSpace(*body.Description)
	}
	if body.FolderID != nil {
		out.folderID = trimToPtr(*body.FolderID)
	}
	if body.TeamID != nil {
		out.teamID = trimToPtr(*body.TeamID)
	}
	if out.fileType == "" {
		out.fileType = "application/octet-stream"
	}
	if out.fileName == "" {
		return docPayload{}, fmt.Errorf("fileName is required")
	}
	if body.ContentBase64 != nil && strings.TrimSpace(*body.ContentBase64) != "" {
		raw, err := base64.StdEncoding.DecodeString(*body.ContentBase64)
		if err != nil {
			return docPayload{}, fmt.Errorf("contentBase64 must be valid base64")
		}
		out.fileBytes = raw
	}
	return out, nil
}

func trimToPtr(s string) *string {
	v := strings.TrimSpace(s)
	if v == "" {
		return nil
	}
	return &v
}

func sha256Bytes(b []byte) string {
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum)
}
