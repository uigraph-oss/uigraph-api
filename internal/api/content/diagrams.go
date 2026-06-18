package content

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/uigraph/app/internal/cache"
	"github.com/uigraph/app/internal/diagram"
	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/storage"
	"github.com/uigraph/app/internal/store"
)

const diagramCacheTTL = time.Hour

type DiagramHandler struct {
	store   store.Store
	storage storage.Client
	cache   cache.Client // may be nil
}

func NewDiagramHandler(s store.Store, st storage.Client, c cache.Client) *DiagramHandler {
	return &DiagramHandler{store: s, storage: st, cache: c}
}

func (h *DiagramHandler) List(w http.ResponseWriter, r *http.Request) {
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
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"diagrams": diagrams})
}

func (h *DiagramHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var body struct {
		Name     string  `json:"name"`
		FolderID *string `json:"folderId"`
		TeamID   *string `json:"teamId"`
		Source   *string `json:"source"`
		Content  string  `json:"content"` // ReactFlow JSON
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" || body.Content == "" {
		writeErr(w, http.StatusBadRequest, "name and content are required")
		return
	}

	id := uuid.NewString()
	hash := sha256Hex(body.Content)
	contentKey := storage.DiagramContentKey(orgID, id)

	if err := h.uploadContent(r.Context(), contentKey, body.Content); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to store content")
		return
	}

	now := time.Now().UTC()
	dg := diagram.Diagram{
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
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Create version 1 automatically.
	versionID := uuid.NewString()
	vKey := storage.DiagramVersionKey(orgID, id, versionID)
	// Version content is the same object — copy the key rather than re-uploading.
	// Store a second object so version blobs are immutable even if current changes.
	if err := h.uploadContent(r.Context(), vKey, body.Content); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to store version content")
		return
	}
	_ = h.store.CreateDiagramVersion(r.Context(), diagram.Version{
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
	writeJSON(w, http.StatusCreated, dg)
}

func (h *DiagramHandler) Get(w http.ResponseWriter, r *http.Request) {
	dg, err := h.store.GetDiagram(r.Context(), r.PathValue("diagramID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if dg == nil || dg.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, dg)
}

func (h *DiagramHandler) UpdateThumbnail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("diagramID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	dg, err := h.store.GetDiagram(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if dg == nil || dg.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "missing file")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	raw, err := io.ReadAll(file)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to read thumbnail")
		return
	}

	assetID := storage.DiagramThumbnailAssetID(id)
	if err := h.storage.Upload(r.Context(), storage.AssetKey(assetID), contentType, bytes.NewReader(raw), int64(len(raw))); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to store thumbnail")
		return
	}
	hash := sha256Hex(string(raw))

	dg.PreviewAssetID = &assetID
	dg.PreviewContentHash = &hash
	dg.UpdatedBy = &p.UserID
	if err := h.store.UpdateDiagram(r.Context(), *dg); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	if h.cache != nil {
		_ = h.cache.Del(r.Context(), cache.AssetURLKey(assetID))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"assetId":            assetID,
		"previewContentHash": hash,
	})
}

func (h *DiagramHandler) GetContent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("diagramID")

	if content, ok := h.cacheGet(r.Context(), id); ok {
		writeJSON(w, http.StatusOK, map[string]any{"diagramId": id, "content": content})
		return
	}

	dg, err := h.store.GetDiagram(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if dg == nil || dg.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	content, err := h.downloadContent(r.Context(), dg.ContentKey)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch content")
		return
	}

	h.cacheSet(r.Context(), id, content)

	writeJSON(w, http.StatusOK, map[string]any{"diagramId": id, "content": content})
}

func (h *DiagramHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("diagramID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	dg, err := h.store.GetDiagram(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if dg == nil || dg.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	var body struct {
		Name     *string `json:"name"`
		FolderID *string `json:"folderId"`
		TeamID   *string `json:"teamId"`
		Source   *string `json:"source"`
		Content  *string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
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
				writeErr(w, http.StatusInternalServerError, "failed to store content")
				return
			}
			dg.ContentHash = newHash
			h.cacheDel(r.Context(), id)
		}
	}

	if err := h.store.UpdateDiagram(r.Context(), *dg); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, dg)
}

func (h *DiagramHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("diagramID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if err := h.store.SoftDeleteDiagram(r.Context(), id, p.UserID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	h.cacheDel(r.Context(), id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *DiagramHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := h.store.ListDiagramVersions(r.Context(), r.PathValue("diagramID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": versions})
}

func (h *DiagramHandler) CreateVersion(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("diagramID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	dg, err := h.store.GetDiagram(r.Context(), id)
	if err != nil || dg == nil || dg.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	var body struct {
		Label *string `json:"label"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	content, err := h.getContent(r.Context(), id, dg.ContentKey)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to read current content")
		return
	}

	latestVer, _ := h.store.LatestVersionNumber(r.Context(), id)
	versionID := uuid.NewString()
	vKey := storage.DiagramVersionKey(orgID, id, versionID)
	if err := h.uploadContent(r.Context(), vKey, content); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to store version")
		return
	}

	v := diagram.Version{
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
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (h *DiagramHandler) GetVersionContent(w http.ResponseWriter, r *http.Request) {
	v, err := h.store.GetDiagramVersion(r.Context(), r.PathValue("versionID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if v == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	content, err := h.downloadContent(r.Context(), v.ContentKey)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch version content")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versionId": v.ID, "content": content})
}

func (h *DiagramHandler) RestoreVersion(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("diagramID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	dg, err := h.store.GetDiagram(r.Context(), id)
	if err != nil || dg == nil || dg.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "diagram not found")
		return
	}
	v, err := h.store.GetDiagramVersion(r.Context(), r.PathValue("versionID"))
	if err != nil || v == nil {
		writeErr(w, http.StatusNotFound, "version not found")
		return
	}

	content, err := h.downloadContent(r.Context(), v.ContentKey)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to read version content")
		return
	}

	// Write restored content as the new current object.
	if err := h.uploadContent(r.Context(), dg.ContentKey, content); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to restore content")
		return
	}

	dg.ContentHash = v.ContentHash
	src := fmt.Sprintf("restore:v%d", v.VersionNumber)
	dg.Source = &src
	dg.UpdatedBy = &p.UserID
	if err := h.store.UpdateDiagram(r.Context(), *dg); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Record the restore as a new auto-version.
	latestVer, _ := h.store.LatestVersionNumber(r.Context(), id)
	versionID := uuid.NewString()
	vKey := storage.DiagramVersionKey(orgID, id, versionID)
	if err := h.uploadContent(r.Context(), vKey, content); err == nil {
		_ = h.store.CreateDiagramVersion(r.Context(), diagram.Version{
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
	writeJSON(w, http.StatusOK, dg)
}

func (h *DiagramHandler) ListImages(w http.ResponseWriter, r *http.Request) {
	images, err := h.store.ListDiagramImages(r.Context(), r.PathValue("diagramID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"images": images})
}

func (h *DiagramHandler) CreateImage(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	id := r.PathValue("diagramID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	dg, err := h.store.GetDiagram(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if dg == nil || dg.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "missing file")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	order := 0
	if v := r.FormValue("order"); v != "" {
		if n, convErr := strconv.Atoi(v); convErr == nil {
			order = n
		}
	}

	var fileName *string
	if v := r.FormValue("fileName"); v != "" {
		fileName = &v
	} else if header.Filename != "" {
		name := header.Filename
		fileName = &name
	}

	assetID := storage.NewFileAssetID()
	if err := h.storage.Upload(r.Context(), storage.AssetKey(assetID), contentType, file, header.Size); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to store image")
		return
	}

	img := diagram.Image{
		ID:        uuid.NewString(),
		DiagramID: id,
		OrgID:     orgID,
		AssetID:   assetID,
		FileName:  fileName,
		Order:     order,
		CreatedBy: p.UserID,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.store.CreateDiagramImage(r.Context(), img); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"diagramImageId": img.ID,
		"assetId":        assetID,
	})
}

// ── Sync (CLI / CI-CD) ────────────────────────────────────────────────────────

// Sync handles POST /api/v1/orgs/{orgID}/diagrams/sync
// Upserts a diagram by name (or diagramId). Creates an auto-version only when
// content has actually changed (hash comparison — no storage read required).
func (h *DiagramHandler) Sync(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" || body.Content == "" {
		writeErr(w, http.StatusBadRequest, "name and content are required")
		return
	}

	newHash := sha256Hex(body.Content)

	if body.DiagramID != nil {
		dg, err := h.store.GetDiagram(r.Context(), *body.DiagramID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		if dg == nil || dg.DeletedAt != nil {
			writeErr(w, http.StatusNotFound, "diagram not found")
			return
		}

		if newHash == dg.ContentHash {
			writeJSON(w, http.StatusOK, map[string]any{
				"diagramId":      dg.ID,
				"versionCreated": false,
			})
			return
		}

		if err := h.uploadContent(r.Context(), dg.ContentKey, body.Content); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to store content")
			return
		}
		dg.ContentHash = newHash
		dg.Source = body.Source
		dg.UpdatedBy = &p.UserID
		if err := h.store.UpdateDiagram(r.Context(), *dg); err != nil {
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		h.cacheDel(r.Context(), dg.ID)

		latestVer, _ := h.store.LatestVersionNumber(r.Context(), dg.ID)
		versionID := uuid.NewString()
		vKey := storage.DiagramVersionKey(orgID, dg.ID, versionID)
		if err := h.uploadContent(r.Context(), vKey, body.Content); err == nil {
			_ = h.store.CreateDiagramVersion(r.Context(), diagram.Version{
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

		writeJSON(w, http.StatusOK, map[string]any{
			"diagramId":      dg.ID,
			"versionCreated": true,
		})
		return
	}

	// Create path — no diagramId.
	id := uuid.NewString()
	contentKey := storage.DiagramContentKey(orgID, id)
	if err := h.uploadContent(r.Context(), contentKey, body.Content); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to store content")
		return
	}

	now := time.Now().UTC()
	dg := diagram.Diagram{
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
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	versionID := uuid.NewString()
	vKey := storage.DiagramVersionKey(orgID, id, versionID)
	if err := h.uploadContent(r.Context(), vKey, body.Content); err == nil {
		_ = h.store.CreateDiagramVersion(r.Context(), diagram.Version{
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

	writeJSON(w, http.StatusCreated, map[string]any{
		"diagramId":      id,
		"versionCreated": true,
	})
}

// ── internal helpers ──────────────────────────────────────────────────────────

func (h *DiagramHandler) uploadContent(ctx context.Context, key, content string) error {
	r := strings.NewReader(content)
	return h.storage.Upload(ctx, key, "application/json", r, int64(r.Len()))
}

func (h *DiagramHandler) downloadContent(ctx context.Context, key string) (string, error) {
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

func (h *DiagramHandler) getContent(ctx context.Context, id, key string) (string, error) {
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

func (h *DiagramHandler) cacheGet(ctx context.Context, id string) (string, bool) {
	if h.cache == nil {
		return "", false
	}
	v, err := h.cache.Get(ctx, cache.DiagramContentKey(id))
	if errors.Is(err, cache.ErrNotFound) || err != nil {
		return "", false
	}
	return v, true
}

func (h *DiagramHandler) cacheSet(ctx context.Context, id, content string) {
	if h.cache == nil {
		return
	}
	_ = h.cache.Set(ctx, cache.DiagramContentKey(id), content, diagramCacheTTL)
}

func (h *DiagramHandler) cacheDel(ctx context.Context, id string) {
	if h.cache == nil {
		return
	}
	_ = h.cache.Del(ctx, cache.DiagramContentKey(id))
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}
