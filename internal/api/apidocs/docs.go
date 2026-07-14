package apidocs

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

	docspkg "github.com/uigraph/app/internal/docs"
	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/storage"
	storepkg "github.com/uigraph/app/internal/store"
)

// @Summary  List
// @Tags     docs
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/docs [get]
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

// @Summary  Get
// @Tags     docs
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    docID  path  string  true  "docID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/docs/{docID} [get]
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

// @Summary  GetContent
// @Tags     docs
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    docID  path  string  true  "docID"
// @Success  200  {string}  string  "raw file bytes"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/docs/{docID}/content [get]
func (h *Handler) GetContent(w http.ResponseWriter, r *http.Request) {
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
	if doc.FileAssetID == "" {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	rc, err := h.storage.Download(r.Context(), storage.AssetKey(doc.FileAssetID))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	defer rc.Close()
	contentType := doc.FileType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
}

// @Summary  Create
// @Tags     docs
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/docs [post]
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

	fileAssetID, contentHash, err := h.resolveDocContent(r.Context(), body)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if fileAssetID == "" {
		httputil.BadRequest(w, "file content is required")
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
		ContentHash: contentHash,
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

// @Summary  Update
// @Tags     docs
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    docID  path  string  true  "docID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/docs/{docID} [put]
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

	if body.fileAssetID != "" || len(body.fileBytes) > 0 {
		newAssetID, newHash, err := h.resolveDocContent(r.Context(), body)
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
		if doc.FileAssetID != "" && doc.FileAssetID != newAssetID {
			_ = h.storage.Delete(r.Context(), storage.AssetKey(doc.FileAssetID))
		}
		doc.FileAssetID = newAssetID
		doc.ContentHash = newHash
	}

	if err := h.store.UpdateDoc(r.Context(), *doc); err != nil {
		httputil.Error(w, r, err)
		return
	}
	doc.UpdatedAt = time.Now().UTC()
	httputil.JSON(w, http.StatusOK, doc)
}

// @Summary  Delete
// @Tags     docs
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    docID  path  string  true  "docID"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/docs/{docID} [delete]
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
	fileAssetID string
}

func (h *Handler) resolveDocContent(ctx context.Context, body docPayload) (string, string, error) {
	if body.fileAssetID != "" {
		hash, err := storage.HashAsset(ctx, h.storage, body.fileAssetID)
		if err != nil {
			return "", "", err
		}
		return body.fileAssetID, hash, nil
	}
	if len(body.fileBytes) > 0 {
		newAssetID := storage.NewFileAssetID()
		if err := h.storage.Upload(ctx, storage.AssetKey(newAssetID), body.fileType, bytes.NewReader(body.fileBytes), int64(len(body.fileBytes))); err != nil {
			return "", "", err
		}
		return newAssetID, sha256Bytes(body.fileBytes), nil
	}
	return "", "", nil
}

func decodeDocPayload(r *http.Request, existing *docspkg.Doc) (docPayload, error) {
	var body struct {
		FileName      *string `json:"fileName"`
		FileType      *string `json:"fileType"`
		Description   *string `json:"description"`
		FolderID      *string `json:"folderId"`
		TeamID        *string `json:"teamId"`
		ContentBase64 *string `json:"contentBase64"`
		FileAssetID   *string `json:"fileAssetId"`
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
	if body.FileAssetID != nil {
		out.fileAssetID = strings.TrimSpace(*body.FileAssetID)
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
