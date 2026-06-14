package content

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/storage"
	"github.com/uigraph/app/internal/store"
	"github.com/uigraph/app/internal/uimap"
)

// FrameHandler serves frames + focal points + canvas under a map.
type FrameHandler struct {
	store   store.Store
	storage storage.Client // may be nil (no screenshot upload)
}

func NewFrameHandler(s store.Store, st storage.Client) *FrameHandler {
	return &FrameHandler{store: s, storage: st}
}

// ── Frames ────────────────────────────────────────────────────────────────────

// List handles GET /api/v1/orgs/{orgID}/maps/{mapID}/frames
func (h *FrameHandler) List(w http.ResponseWriter, r *http.Request) {
	frames, err := h.store.ListFrames(r.Context(), r.PathValue("mapID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	for i := range frames {
		h.resolveScreenshotURL(r.Context(), &frames[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{"frames": frames})
}

// Create handles POST /api/v1/orgs/{orgID}/maps/{mapID}/frames
func (h *FrameHandler) Create(w http.ResponseWriter, r *http.Request) {
	mapID := r.PathValue("mapID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var body struct {
		Name          string  `json:"name"`
		Description   string  `json:"description"`
		TemplateType  string  `json:"templateType"`
		ParentFrameID *string `json:"parentFrameId"`
		Order         float64 `json:"order"`
		Screenshot    string  `json:"screenshot"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}

	id := uuid.NewString()
	now := time.Now().UTC()

	frame := uimap.Frame{
		ID:            id,
		MapID:         mapID,
		OrgID:         orgID,
		ParentFrameID: body.ParentFrameID,
		Name:          body.Name,
		Description:   body.Description,
		TemplateType:  body.TemplateType,
		Status:        "active",
		Order:         body.Order,
		CreatedBy:     p.UserID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if body.Screenshot != "" && h.storage != nil {
		key := storage.FrameScreenshotKey(orgID, mapID, id)
		if err := h.uploadScreenshot(r.Context(), key, body.Screenshot); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to store screenshot")
			return
		}
		hash := screenshotHash(body.Screenshot)
		frame.ScreenshotKey = &key
		frame.ScreenshotContentHash = &hash
	}

	if err := h.store.CreateFrame(r.Context(), frame); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, frame)
}

// Get handles GET /api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}
func (h *FrameHandler) Get(w http.ResponseWriter, r *http.Request) {
	f, err := h.store.GetFrame(r.Context(), r.PathValue("frameID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if f == nil || f.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	h.resolveScreenshotURL(r.Context(), f)
	writeJSON(w, http.StatusOK, f)
}

// Update handles PUT /api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}
func (h *FrameHandler) Update(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	mapID := r.PathValue("mapID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	f, err := h.store.GetFrame(r.Context(), r.PathValue("frameID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if f == nil || f.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	var body struct {
		Name         *string  `json:"name"`
		Description  *string  `json:"description"`
		TemplateType *string  `json:"templateType"`
		Status       *string  `json:"status"`
		Order        *float64 `json:"order"`
		Screenshot   *string  `json:"screenshot"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
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

	if body.Screenshot != nil && h.storage != nil {
		newHash := screenshotHash(*body.Screenshot)
		if f.ScreenshotContentHash == nil || newHash != *f.ScreenshotContentHash {
			key := storage.FrameScreenshotKey(orgID, mapID, f.ID)
			if err := h.uploadScreenshot(r.Context(), key, *body.Screenshot); err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to store screenshot")
				return
			}
			f.ScreenshotKey = &key
			f.ScreenshotContentHash = &newHash
		}
	}

	if err := h.store.UpdateFrame(r.Context(), *f); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, f)
}

// Delete handles DELETE /api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}
func (h *FrameHandler) Delete(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if err := h.store.SoftDeleteFrame(r.Context(), r.PathValue("frameID"), p.UserID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Sync handles POST /api/v1/orgs/{orgID}/maps/{mapID}/frames/sync
// CLI upsert: creates or updates a frame, skipping the screenshot upload when
// the content hash is unchanged.
func (h *FrameHandler) Sync(w http.ResponseWriter, r *http.Request) {
	mapID := r.PathValue("mapID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var body struct {
		FrameID      *string `json:"frameId"`
		Name         string  `json:"name"`
		TemplateType string  `json:"templateType"`
		Description  string  `json:"description"`
		Source       *string `json:"source"`
		Screenshot   string  `json:"screenshot"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}

	newHash := screenshotHash(body.Screenshot)

	// Update path — frameId provided.
	if body.FrameID != nil {
		f, err := h.store.GetFrame(r.Context(), *body.FrameID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		if f == nil || f.DeletedAt != nil {
			writeErr(w, http.StatusNotFound, "frame not found")
			return
		}

		// Skip upload if hash unchanged.
		if body.Screenshot != "" && f.ScreenshotContentHash != nil && newHash == *f.ScreenshotContentHash {
			writeJSON(w, http.StatusOK, map[string]any{
				"frameId":         f.ID,
				"screenshotSaved": false,
			})
			return
		}

		if body.Screenshot != "" && h.storage != nil {
			key := storage.FrameScreenshotKey(orgID, mapID, f.ID)
			if err := h.uploadScreenshot(r.Context(), key, body.Screenshot); err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to store screenshot")
				return
			}
			f.ScreenshotKey = &key
			f.ScreenshotContentHash = &newHash
		}
		f.Name = body.Name
		f.Description = body.Description
		f.TemplateType = body.TemplateType
		f.Source = body.Source
		f.UpdatedBy = &p.UserID
		if err := h.store.UpdateFrame(r.Context(), *f); err != nil {
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"frameId":         f.ID,
			"screenshotSaved": body.Screenshot != "",
		})
		return
	}

	// Create path.
	id := uuid.NewString()
	now := time.Now().UTC()
	frame := uimap.Frame{
		ID:           id,
		MapID:        mapID,
		OrgID:        orgID,
		Name:         body.Name,
		Description:  body.Description,
		TemplateType: body.TemplateType,
		Status:       "active",
		Source:       body.Source,
		CreatedBy:    p.UserID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if body.Screenshot != "" && h.storage != nil {
		key := storage.FrameScreenshotKey(orgID, mapID, id)
		if err := h.uploadScreenshot(r.Context(), key, body.Screenshot); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to store screenshot")
			return
		}
		frame.ScreenshotKey = &key
		frame.ScreenshotContentHash = &newHash
	}

	if err := h.store.CreateFrame(r.Context(), frame); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"frameId":         id,
		"screenshotSaved": body.Screenshot != "" && h.storage != nil,
	})
}

// ── Focal points ──────────────────────────────────────────────────────────────

// ListFocalPoints handles GET /api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points
func (h *FrameHandler) ListFocalPoints(w http.ResponseWriter, r *http.Request) {
	fps, err := h.store.ListFocalPoints(r.Context(), r.PathValue("frameID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"focalPoints": fps})
}

// CreateFocalPoint handles POST /api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points
func (h *FrameHandler) CreateFocalPoint(w http.ResponseWriter, r *http.Request) {
	frameID := r.PathValue("frameID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var body struct {
		Name       string  `json:"name"`
		LocationX  float64 `json:"locationX"`
		LocationY  float64 `json:"locationY"`
		Visibility string  `json:"visibility"`
		IsActive   bool    `json:"isActive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if body.Visibility == "" {
		body.Visibility = "public"
	}

	now := time.Now().UTC()
	fp := uimap.FocalPoint{
		ID:         uuid.NewString(),
		FrameID:    frameID,
		OrgID:      orgID,
		Name:       body.Name,
		LocationX:  body.LocationX,
		LocationY:  body.LocationY,
		Visibility: body.Visibility,
		IsActive:   body.IsActive,
		CreatedBy:  p.UserID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := h.store.CreateFocalPoint(r.Context(), fp); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, fp)
}

// GetFocalPoint handles GET /api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}
func (h *FrameHandler) GetFocalPoint(w http.ResponseWriter, r *http.Request) {
	fp, err := h.store.GetFocalPoint(r.Context(), r.PathValue("fpID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if fp == nil || fp.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, fp)
}

// UpdateFocalPoint handles PUT /api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}
func (h *FrameHandler) UpdateFocalPoint(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	fp, err := h.store.GetFocalPoint(r.Context(), r.PathValue("fpID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if fp == nil || fp.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	var body struct {
		Name       *string  `json:"name"`
		LocationX  *float64 `json:"locationX"`
		LocationY  *float64 `json:"locationY"`
		Visibility *string  `json:"visibility"`
		IsActive   *bool    `json:"isActive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name != nil {
		fp.Name = *body.Name
	}
	if body.LocationX != nil {
		fp.LocationX = *body.LocationX
	}
	if body.LocationY != nil {
		fp.LocationY = *body.LocationY
	}
	if body.Visibility != nil {
		fp.Visibility = *body.Visibility
	}
	if body.IsActive != nil {
		fp.IsActive = *body.IsActive
	}
	fp.UpdatedBy = &p.UserID

	if err := h.store.UpdateFocalPoint(r.Context(), *fp); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, fp)
}

// DeleteFocalPoint handles DELETE /api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}
func (h *FrameHandler) DeleteFocalPoint(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if err := h.store.SoftDeleteFocalPoint(r.Context(), r.PathValue("fpID"), p.UserID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Canvas ────────────────────────────────────────────────────────────────────

// GetCanvas handles GET /api/v1/orgs/{orgID}/maps/{mapID}/canvas
func (h *FrameHandler) GetCanvas(w http.ResponseWriter, r *http.Request) {
	mapID := r.PathValue("mapID")
	c, err := h.store.GetCanvas(r.Context(), mapID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if c == nil {
		c = &uimap.Canvas{
			MapID:          mapID,
			Zoom:           1.0,
			FramePositions: map[string]uimap.FramePosition{},
		}
	}
	writeJSON(w, http.StatusOK, c)
}

// UpsertCanvas handles PUT /api/v1/orgs/{orgID}/maps/{mapID}/canvas
func (h *FrameHandler) UpsertCanvas(w http.ResponseWriter, r *http.Request) {
	mapID := r.PathValue("mapID")
	orgID := r.PathValue("orgID")

	var body struct {
		Zoom           *float64                       `json:"zoom"`
		NavigationX    *float64                       `json:"navigationX"`
		NavigationY    *float64                       `json:"navigationY"`
		FramePositions map[string]uimap.FramePosition `json:"framePositions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	c, err := h.store.GetCanvas(r.Context(), mapID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if c == nil {
		c = &uimap.Canvas{
			MapID:          mapID,
			OrgID:          orgID,
			Zoom:           1.0,
			FramePositions: map[string]uimap.FramePosition{},
		}
	}
	if body.Zoom != nil {
		c.Zoom = *body.Zoom
	}
	if body.NavigationX != nil {
		c.NavigationX = *body.NavigationX
	}
	if body.NavigationY != nil {
		c.NavigationY = *body.NavigationY
	}
	if body.FramePositions != nil {
		c.FramePositions = body.FramePositions
	}

	if err := h.store.UpsertCanvas(r.Context(), *c); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// ── internal helpers ──────────────────────────────────────────────────────────

// resolveScreenshotURL populates f.ScreenshotURL with a presigned GET URL when
// the frame has a stored screenshot.
func (h *FrameHandler) resolveScreenshotURL(ctx context.Context, f *uimap.Frame) {
	if f == nil || h.storage == nil || f.ScreenshotKey == nil || *f.ScreenshotKey == "" {
		return
	}
	u, err := h.storage.PresignURL(ctx, *f.ScreenshotKey)
	if err != nil {
		return
	}
	f.ScreenshotURL = &u
}

// uploadScreenshot stores a frame screenshot. The content may be a data URL
// (e.g. "data:image/png;base64,...") sent by the browser, in which case it is
// decoded to its raw bytes and stored with the declared content type. Any other
// value (e.g. raw bytes sent by the CLI) is stored verbatim as image/png.
func (h *FrameHandler) uploadScreenshot(ctx context.Context, key, content string) error {
	if contentType, raw, ok := decodeDataURL(content); ok {
		return h.storage.Upload(ctx, key, contentType, bytes.NewReader(raw), int64(len(raw)))
	}
	r := strings.NewReader(content)
	return h.storage.Upload(ctx, key, "image/png", r, int64(r.Len()))
}

// decodeDataURL parses a "data:<contentType>;base64,<payload>" string into its
// content type and decoded bytes. Returns ok=false for non data-URL input.
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
