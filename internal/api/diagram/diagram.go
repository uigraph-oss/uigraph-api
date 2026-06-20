package diagram

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	diagrampkg "github.com/uigraph/app/internal/diagram"
	"github.com/uigraph/app/internal/cache"
	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
	"github.com/uigraph/app/internal/storage"
)

const diagramCacheTTL = time.Hour

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	q := r.URL.Query()
	var folderID, teamID *string
	if v := q.Get("folderId"); v != "" {
		folderID = &v
	}
	if v := q.Get("teamId"); v != "" {
		teamID = &v
	}
	diagrams, err := h.store.ListDiagrams(r.Context(), orgID, folderID, teamID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"diagrams": diagrams})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	var body struct {
		Name     string  `json:"name"`
		FolderID *string `json:"folderId"`
		TeamID   *string `json:"teamId"`
		Source   *string `json:"source"`
		Content  string  `json:"content"` // ReactFlow JSON
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" || body.Content == "" {
		httputil.BadRequest(w, "name and content are required")
		return
	}

	id := uuid.NewString()
	hash := sha256Hex(body.Content)
	contentKey := storage.DiagramContentKey(orgID, id)

	if err := h.uploadContent(r.Context(), contentKey, body.Content); err != nil {
		httputil.Error(w, r, err)
		return
	}

	now := time.Now().UTC()
	dg := diagrampkg.Diagram{
		ID:          id,
		OrgID:       orgID,
		FolderID:    body.FolderID,
		TeamID:      body.TeamID,
		Name:        body.Name,
		ContentKey:  contentKey,
		ContentHash: hash,
		Source:      body.Source,
		CreatedBy:   p.UserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.store.CreateDiagram(r.Context(), dg); err != nil {
		httputil.Error(w, r, err)
		return
	}

	// Create version 1 automatically.
	versionID := uuid.NewString()
	vKey := storage.DiagramVersionKey(orgID, id, versionID)
	// Version content is the same object — copy the key rather than re-uploading.
	// Store a second object so version blobs are immutable even if current changes.
	if err := h.uploadContent(r.Context(), vKey, body.Content); err != nil {
		httputil.Error(w, r, err)
		return
	}
	_ = h.store.CreateDiagramVersion(r.Context(), diagrampkg.Version{
		ID:            versionID,
		DiagramID:     id,
		VersionNumber: 1,
		ContentKey:    vKey,
		ContentHash:   hash,
		IsAutoVersion: body.Source != nil,
		Source:        body.Source,
		CreatedBy:     p.UserID,
		CreatedAt:     now,
	})

	h.cacheSet(r.Context(), id, body.Content)
	httputil.JSON(w, http.StatusCreated, dg)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	dg, err := h.store.GetDiagram(r.Context(), r.PathValue("diagramID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if dg == nil || dg.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, dg)
}

func (h *Handler) UpdateThumbnail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("diagramID")
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

	file, header, err := r.FormFile("file")
	if err != nil {
		httputil.BadRequest(w, "missing file")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	raw, err := io.ReadAll(file)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	assetID := storage.DiagramThumbnailAssetID(id)
	if err := h.storage.Upload(r.Context(), storage.AssetKey(assetID), contentType, bytes.NewReader(raw), int64(len(raw))); err != nil {
		httputil.Error(w, r, err)
		return
	}
	hash := sha256Hex(string(raw))

	dg.PreviewAssetID = &assetID
	dg.PreviewContentHash = &hash
	dg.UpdatedBy = &p.UserID
	if err := h.store.UpdateDiagram(r.Context(), *dg); err != nil {
		httputil.Error(w, r, err)
		return
	}

	if h.cache != nil {
		_ = h.cache.Del(r.Context(), cache.AssetURLKey(assetID))
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"assetId":            assetID,
		"previewContentHash": hash,
	})
}

func (h *Handler) GetContent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("diagramID")

	if content, ok := h.cacheGet(r.Context(), id); ok {
		httputil.JSON(w, http.StatusOK, map[string]any{"diagramId": id, "content": content})
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

	content, err := h.downloadContent(r.Context(), dg.ContentKey)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	h.cacheSet(r.Context(), id, content)

	httputil.JSON(w, http.StatusOK, map[string]any{"diagramId": id, "content": content})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("diagramID")
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
		Name     *string `json:"name"`
		FolderID *string `json:"folderId"`
		TeamID   *string `json:"teamId"`
		Source   *string `json:"source"`
		Content  *string `json:"content"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}

	if body.Name != nil {
		dg.Name = *body.Name
	}
	if body.FolderID != nil {
		dg.FolderID = body.FolderID
	}
	if body.TeamID != nil {
		dg.TeamID = body.TeamID
	}
	if body.Source != nil {
		dg.Source = body.Source
	}
	dg.UpdatedBy = &p.UserID

	if body.Content != nil {
		newHash := sha256Hex(*body.Content)
		if newHash != dg.ContentHash {
			if err := h.uploadContent(r.Context(), dg.ContentKey, *body.Content); err != nil {
				httputil.Error(w, r, err)
				return
			}
			dg.ContentHash = newHash
			h.cacheDel(r.Context(), id)
		}
	}

	if err := h.store.UpdateDiagram(r.Context(), *dg); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, dg)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("diagramID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if err := h.store.SoftDeleteDiagram(r.Context(), id, p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	h.cacheDel(r.Context(), id)
	w.WriteHeader(http.StatusNoContent)
}

// Sync handles POST /api/v1/orgs/{orgID}/diagrams/sync
// Upserts a diagram by name (or diagramId). Creates an auto-version only when
// content has actually changed (hash comparison — no storage read required).
func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	var body struct {
		DiagramID *string `json:"diagramId"`
		Name      string  `json:"name"`
		FolderID  *string `json:"folderId"`
		TeamID    *string `json:"teamId"`
		Source    *string `json:"source"`
		Content   string  `json:"content"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" || body.Content == "" {
		httputil.BadRequest(w, "name and content are required")
		return
	}

	newHash := sha256Hex(body.Content)

	if body.DiagramID != nil {
		dg, err := h.store.GetDiagram(r.Context(), *body.DiagramID)
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
		if dg == nil || dg.DeletedAt != nil {
			httputil.Error(w, r, storepkg.ErrNotFound)
			return
		}

		if newHash == dg.ContentHash {
			httputil.JSON(w, http.StatusOK, map[string]any{
				"diagramId":      dg.ID,
				"versionCreated": false,
			})
			return
		}

		if err := h.uploadContent(r.Context(), dg.ContentKey, body.Content); err != nil {
			httputil.Error(w, r, err)
			return
		}
		dg.ContentHash = newHash
		dg.Source = body.Source
		dg.UpdatedBy = &p.UserID
		if err := h.store.UpdateDiagram(r.Context(), *dg); err != nil {
			httputil.Error(w, r, err)
			return
		}
		h.cacheDel(r.Context(), dg.ID)

		latestVer, _ := h.store.LatestVersionNumber(r.Context(), dg.ID)
		versionID := uuid.NewString()
		vKey := storage.DiagramVersionKey(orgID, dg.ID, versionID)
		if err := h.uploadContent(r.Context(), vKey, body.Content); err == nil {
			_ = h.store.CreateDiagramVersion(r.Context(), diagrampkg.Version{
				ID:            versionID,
				DiagramID:     dg.ID,
				VersionNumber: latestVer + 1,
				ContentKey:    vKey,
				ContentHash:   newHash,
				IsAutoVersion: true,
				Source:        body.Source,
				CreatedBy:     p.UserID,
				CreatedAt:     time.Now().UTC(),
			})
		}

		httputil.JSON(w, http.StatusOK, map[string]any{
			"diagramId":      dg.ID,
			"versionCreated": true,
		})
		return
	}

	// Create path — no diagramId.
	id := uuid.NewString()
	contentKey := storage.DiagramContentKey(orgID, id)
	if err := h.uploadContent(r.Context(), contentKey, body.Content); err != nil {
		httputil.Error(w, r, err)
		return
	}

	now := time.Now().UTC()
	dg := diagrampkg.Diagram{
		ID:          id,
		OrgID:       orgID,
		FolderID:    body.FolderID,
		TeamID:      body.TeamID,
		Name:        body.Name,
		ContentKey:  contentKey,
		ContentHash: newHash,
		Source:      body.Source,
		CreatedBy:   p.UserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.store.CreateDiagram(r.Context(), dg); err != nil {
		httputil.Error(w, r, err)
		return
	}

	versionID := uuid.NewString()
	vKey := storage.DiagramVersionKey(orgID, id, versionID)
	if err := h.uploadContent(r.Context(), vKey, body.Content); err == nil {
		_ = h.store.CreateDiagramVersion(r.Context(), diagrampkg.Version{
			ID:            versionID,
			DiagramID:     id,
			VersionNumber: 1,
			ContentKey:    vKey,
			ContentHash:   newHash,
			IsAutoVersion: true,
			Source:        body.Source,
			CreatedBy:     p.UserID,
			CreatedAt:     now,
		})
	}

	httputil.JSON(w, http.StatusCreated, map[string]any{
		"diagramId":      id,
		"versionCreated": true,
	})
}

// ── internal helpers ──────────────────────────────────────────────────────────

func (h *Handler) uploadContent(ctx context.Context, key, content string) error {
	r := strings.NewReader(content)
	return h.storage.Upload(ctx, key, "application/json", r, int64(r.Len()))
}

func (h *Handler) downloadContent(ctx context.Context, key string) (string, error) {
	rc, err := h.storage.Download(ctx, key)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, rc); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (h *Handler) getContent(ctx context.Context, id, key string) (string, error) {
	if content, ok := h.cacheGet(ctx, id); ok {
		return content, nil
	}
	content, err := h.downloadContent(ctx, key)
	if err != nil {
		return "", err
	}
	h.cacheSet(ctx, id, content)
	return content, nil
}

func (h *Handler) cacheGet(ctx context.Context, id string) (string, bool) {
	if h.cache == nil {
		return "", false
	}
	v, err := h.cache.Get(ctx, cache.DiagramContentKey(id))
	if errors.Is(err, cache.ErrNotFound) || err != nil {
		return "", false
	}
	return v, true
}

func (h *Handler) cacheSet(ctx context.Context, id, content string) {
	if h.cache == nil {
		return
	}
	_ = h.cache.Set(ctx, cache.DiagramContentKey(id), content, diagramCacheTTL)
}

func (h *Handler) cacheDel(ctx context.Context, id string) {
	if h.cache == nil {
		return
	}
	_ = h.cache.Del(ctx, cache.DiagramContentKey(id))
}

func (h *Handler) PrepareThumbnailUpload(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	diagramID := r.PathValue("diagramID")
	_, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	dg, err := h.store.GetDiagram(r.Context(), diagramID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if dg == nil || dg.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if dg.OrgID != orgID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	assetID := storage.DiagramThumbnailAssetID(diagramID)
	uploadURL, err := h.storage.PresignPutURL(r.Context(), storage.AssetKey(assetID))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"uploadUrl": uploadURL,
		"assetId":   assetID,
	})
}

func (h *Handler) ConfirmThumbnailUpload(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	diagramID := r.PathValue("diagramID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	dg, err := h.store.GetDiagram(r.Context(), diagramID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if dg == nil || dg.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if dg.OrgID != orgID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	var body struct {
		ContentHash string `json:"contentHash"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.ContentHash == "" {
		httputil.BadRequest(w, "contentHash is required")
		return
	}

	assetID := storage.DiagramThumbnailAssetID(diagramID)
	dg.PreviewAssetID = &assetID
	dg.PreviewContentHash = &body.ContentHash
	dg.UpdatedBy = &p.UserID
	if err := h.store.UpdateDiagram(r.Context(), *dg); err != nil {
		httputil.Error(w, r, err)
		return
	}

	if h.cache != nil {
		_ = h.cache.Del(r.Context(), cache.AssetURLKey(assetID))
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}
