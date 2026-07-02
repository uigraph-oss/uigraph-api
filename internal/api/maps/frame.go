package maps

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/storage"
	storepkg "github.com/uigraph/app/internal/store"
	"github.com/uigraph/app/internal/uimap"
)

// @Summary  ListFrames
// @Tags     maps
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    mapID  path  string  true  "mapID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/maps/{mapID}/frames [get]
func (h *Handler) ListFrames(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	p := uimap.ListParams{
		SortBy:  q.Get("sortBy"),
		SortDir: q.Get("sortDir"),
	}
	if v := q.Get("search"); v != "" {
		p.Search = &v
	}
	if v := q.Get("limit"); v != "" {
		p.Limit = httputil.ListLimit(v)
		p.Offset = httputil.ListOffset(q.Get("offset"))
	}
	frames, total, err := h.store.ListFrames(r.Context(), r.PathValue("mapID"), p)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"frames": frames, "total": total})
}

// @Summary  CreateFrame
// @Tags     maps
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    mapID  path  string  true  "mapID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/maps/{mapID}/frames [post]
func (h *Handler) CreateFrame(w http.ResponseWriter, r *http.Request) {
	mapID := r.PathValue("mapID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	var body struct {
		Name          string  `json:"name"`
		Description   string  `json:"description"`
		TemplateType  string  `json:"templateType"`
		ParentFrameID *string `json:"parentFrameId"`
		Order         float64 `json:"order"`
		Screenshot    string  `json:"screenshot"`
		CommitHash    *string `json:"commitHash"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}

	id := uuid.NewString()
	now := time.Now().UTC()

	frame := uimap.Frame{
		ID:                  id,
		MapID:               mapID,
		OrgID:               orgID,
		ParentFrameID:       body.ParentFrameID,
		Name:                body.Name,
		Description:         body.Description,
		TemplateType:        body.TemplateType,
		Status:              "active",
		Order:               body.Order,
		CreatedBy:           p.UserID,
		CreatedByCommitHash: body.CommitHash,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if body.Screenshot != "" && h.storage != nil {
		assetID := storage.FrameScreenshotAssetID(id)
		if err := h.uploadScreenshot(r.Context(), storage.AssetKey(assetID), body.Screenshot); err != nil {
			httputil.Error(w, r, err)
			return
		}
		hash := screenshotHash(body.Screenshot)
		frame.ScreenshotAssetID = &assetID
		frame.ScreenshotContentHash = &hash
	}

	if err := h.store.CreateFrame(r.Context(), frame); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, frame)
}

// @Summary  GetFrame
// @Tags     frames
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    frameID  path  string  true  "frameID"
// @Param    mapID  path  string  true  "mapID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/frames/{frameID} [get]
// @Router   /orgs/{orgID}/maps/{mapID}/frames/{frameID} [get]
func (h *Handler) GetFrame(w http.ResponseWriter, r *http.Request) {
	f, err := h.store.GetFrame(r.Context(), r.PathValue("frameID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if f == nil || f.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, f)
}

// @Summary  UpdateFrame
// @Tags     maps
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    mapID  path  string  true  "mapID"
// @Param    frameID  path  string  true  "frameID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/maps/{mapID}/frames/{frameID} [put]
func (h *Handler) UpdateFrame(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	f, err := h.store.GetFrame(r.Context(), r.PathValue("frameID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if f == nil || f.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	var body struct {
		Name         *string  `json:"name"`
		Description  *string  `json:"description"`
		TemplateType *string  `json:"templateType"`
		Status       *string  `json:"status"`
		Order        *float64 `json:"order"`
		Screenshot   *string  `json:"screenshot"`
		CommitHash   *string  `json:"commitHash"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}

	if body.Name != nil {
		f.Name = *body.Name
	}
	if body.Description != nil {
		f.Description = *body.Description
	}
	if body.TemplateType != nil {
		f.TemplateType = *body.TemplateType
	}
	if body.Status != nil {
		f.Status = *body.Status
	}
	if body.Order != nil {
		f.Order = *body.Order
	}
	f.UpdatedBy = &p.UserID
	f.UpdatedByCommitHash = body.CommitHash

	if body.Screenshot != nil && h.storage != nil {
		newHash := screenshotHash(*body.Screenshot)
		if f.ScreenshotContentHash == nil || newHash != *f.ScreenshotContentHash {
			assetID := storage.FrameScreenshotAssetID(f.ID)
			if err := h.uploadScreenshot(r.Context(), storage.AssetKey(assetID), *body.Screenshot); err != nil {
				httputil.Error(w, r, err)
				return
			}
			f.ScreenshotAssetID = &assetID
			f.ScreenshotContentHash = &newHash
		}
	}

	if err := h.store.UpdateFrame(r.Context(), *f); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, f)
}

// @Summary  DeleteFrame
// @Tags     maps
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    mapID  path  string  true  "mapID"
// @Param    frameID  path  string  true  "frameID"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/maps/{mapID}/frames/{frameID} [delete]
func (h *Handler) DeleteFrame(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if err := h.store.SoftDeleteFrame(r.Context(), r.PathValue("frameID"), p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary  SyncFrames
// @Tags     maps
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    mapID  path  string  true  "mapID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/maps/{mapID}/frames/sync [post]
func (h *Handler) SyncFrames(w http.ResponseWriter, r *http.Request) {
	mapID := r.PathValue("mapID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	var body struct {
		FrameID      *string `json:"frameId"`
		Name         string  `json:"name"`
		TemplateType string  `json:"templateType"`
		Description  string  `json:"description"`
		Source       *string `json:"source"`
		Screenshot   string  `json:"screenshot"`
		CommitHash   *string `json:"commitHash"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}

	newHash := screenshotHash(body.Screenshot)

	// Update path — frameId provided.
	if body.FrameID != nil {
		f, err := h.store.GetFrame(r.Context(), *body.FrameID)
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
		if f == nil || f.DeletedAt != nil {
			httputil.Error(w, r, storepkg.ErrNotFound)
			return
		}

		// Skip upload if hash unchanged.
		if body.Screenshot != "" && f.ScreenshotContentHash != nil && newHash == *f.ScreenshotContentHash {
			httputil.JSON(w, http.StatusOK, map[string]any{
				"frameId":         f.ID,
				"screenshotSaved": false,
			})
			return
		}

		if body.Screenshot != "" && h.storage != nil {
			assetID := storage.FrameScreenshotAssetID(f.ID)
			if err := h.uploadScreenshot(r.Context(), storage.AssetKey(assetID), body.Screenshot); err != nil {
				httputil.Error(w, r, err)
				return
			}
			f.ScreenshotAssetID = &assetID
			f.ScreenshotContentHash = &newHash
		}
		f.Name = body.Name
		f.Description = body.Description
		f.TemplateType = body.TemplateType
		f.Source = body.Source
		f.UpdatedBy = &p.UserID
		f.UpdatedByCommitHash = body.CommitHash
		if err := h.store.UpdateFrame(r.Context(), *f); err != nil {
			httputil.Error(w, r, err)
			return
		}
		httputil.JSON(w, http.StatusOK, map[string]any{
			"frameId":         f.ID,
			"screenshotSaved": body.Screenshot != "",
		})
		return
	}

	// Create path.
	id := uuid.NewString()
	now := time.Now().UTC()
	frame := uimap.Frame{
		ID:                  id,
		MapID:               mapID,
		OrgID:               orgID,
		Name:                body.Name,
		Description:         body.Description,
		TemplateType:        body.TemplateType,
		Status:              "active",
		Source:              body.Source,
		CreatedBy:           p.UserID,
		CreatedByCommitHash: body.CommitHash,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if body.Screenshot != "" && h.storage != nil {
		assetID := storage.FrameScreenshotAssetID(id)
		if err := h.uploadScreenshot(r.Context(), storage.AssetKey(assetID), body.Screenshot); err != nil {
			httputil.Error(w, r, err)
			return
		}
		frame.ScreenshotAssetID = &assetID
		frame.ScreenshotContentHash = &newHash
	}

	if err := h.store.CreateFrame(r.Context(), frame); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, map[string]any{
		"frameId":         id,
		"screenshotSaved": body.Screenshot != "" && h.storage != nil,
	})
}

func (h *Handler) uploadScreenshot(ctx context.Context, key, content string) error {
	if contentType, raw, ok := decodeDataURL(content); ok {
		return h.storage.Upload(ctx, key, contentType, bytes.NewReader(raw), int64(len(raw)))
	}
	// Gateway/CLI flow: the image was already uploaded to a temp key via a
	// presigned PUT, and content is that key — not the image bytes. Copy the
	// real object into the canonical asset key, then drop the temp object.
	if strings.HasPrefix(content, "gateway-uploads/") {
		src, err := h.storage.Download(ctx, content)
		if err != nil {
			return err
		}
		defer src.Close()
		if err := h.storage.Upload(ctx, key, "image/png", src, -1); err != nil {
			return err
		}
		return h.storage.Delete(ctx, content)
	}
	r := strings.NewReader(content)
	return h.storage.Upload(ctx, key, "image/png", r, int64(r.Len()))
}

func decodeDataURL(s string) (contentType string, data []byte, ok bool) {
	if !strings.HasPrefix(s, "data:") {
		return "", nil, false
	}
	comma := strings.IndexByte(s, ',')
	if comma < 0 {
		return "", nil, false
	}
	meta := s[len("data:"):comma]
	if !strings.Contains(meta, ";base64") {
		return "", nil, false
	}
	contentType = strings.TrimSuffix(meta, ";base64")
	if contentType == "" {
		contentType = "image/png"
	}
	raw, err := base64.StdEncoding.DecodeString(s[comma+1:])
	if err != nil {
		return "", nil, false
	}
	return contentType, raw, true
}

func screenshotHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}
