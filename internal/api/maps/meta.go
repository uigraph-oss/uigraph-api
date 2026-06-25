package maps

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/httputil"
	storepkg "github.com/uigraph/app/internal/store"
	"github.com/uigraph/app/internal/uimap"
)

// ListMeta handles GET /api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta
func (h *Handler) ListMeta(w http.ResponseWriter, r *http.Request) {
	metas, err := h.store.ListFocalPointMeta(r.Context(), r.PathValue("fpID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"meta": metas})
}

// ListMetaByLink handles GET /api/v1/orgs/{orgID}/focal-point-meta?linkKey=...&linkValue=...
func (h *Handler) ListMetaByLink(w http.ResponseWriter, r *http.Request) {
	linkKey := r.URL.Query().Get("linkKey")
	linkValue := r.URL.Query().Get("linkValue")
	if linkKey == "" || linkValue == "" {
		httputil.BadRequest(w, "linkKey and linkValue are required")
		return
	}
	metas, err := h.store.ListFocalPointMetaByLink(r.Context(), r.PathValue("orgID"), linkKey, linkValue)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"meta": metas})
}

// CreateMeta handles POST /api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta
func (h *Handler) CreateMeta(w http.ResponseWriter, r *http.Request) {
	fpID := r.PathValue("fpID")
	frameID := r.PathValue("frameID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	var body struct {
		ComponentID          string          `json:"componentId"`
		ComponentLink        json.RawMessage `json:"componentLink"`
		ComponentModalFields json.RawMessage `json:"componentModalFields"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}

	now := time.Now().UTC()
	m := uimap.FocalPointMeta{
		ID:                   uuid.NewString(),
		FocalPointID:         fpID,
		OrgID:                orgID,
		FrameID:              frameID,
		ComponentID:          body.ComponentID,
		ComponentLink:        body.ComponentLink,
		ComponentModalFields: body.ComponentModalFields,
		CreatedBy:            p.UserID,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := h.store.CreateFocalPointMeta(r.Context(), m); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, m)
}

// UpdateMeta handles PUT /api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta/{metaID}
func (h *Handler) UpdateMeta(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	m, err := h.store.GetFocalPointMeta(r.Context(), r.PathValue("metaID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if m == nil || m.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	var body struct {
		ComponentID          *string         `json:"componentId"`
		ComponentLink        json.RawMessage `json:"componentLink"`
		ComponentModalFields json.RawMessage `json:"componentModalFields"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.ComponentID != nil {
		m.ComponentID = *body.ComponentID
	}
	if body.ComponentLink != nil {
		m.ComponentLink = body.ComponentLink
	}
	if body.ComponentModalFields != nil {
		m.ComponentModalFields = body.ComponentModalFields
	}
	m.UpdatedBy = &p.UserID

	if err := h.store.UpdateFocalPointMeta(r.Context(), *m); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, m)
}

// DeleteMeta handles DELETE /api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta/{metaID}
func (h *Handler) DeleteMeta(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if err := h.store.SoftDeleteFocalPointMeta(r.Context(), r.PathValue("metaID"), p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
