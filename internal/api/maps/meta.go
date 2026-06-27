package maps

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
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

// ListMetaByLink handles GET /api/v1/orgs/{orgID}/focal-point-meta?linkId=...
func (h *Handler) ListMetaByLink(w http.ResponseWriter, r *http.Request) {
	linkID := r.URL.Query().Get("linkId")
	if linkID == "" {
		httputil.BadRequest(w, "linkId is required")
		return
	}
	metas, err := h.store.ListFocalPointMetaByLink(r.Context(), r.PathValue("orgID"), linkID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"meta": metas})
}

// ListComponentLinkUsages handles GET /api/v1/orgs/{orgID}/component-link-usages?linkId=...
// It returns the maps, screens, and focal points that reference the given link.
func (h *Handler) ListComponentLinkUsages(w http.ResponseWriter, r *http.Request) {
	linkID := r.URL.Query().Get("linkId")
	if linkID == "" {
		httputil.BadRequest(w, "linkId is required")
		return
	}
	usages, err := h.store.ListComponentLinkUsages(r.Context(), r.PathValue("orgID"), linkID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"usages": usages})
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
		ComponentID                string          `json:"componentId"`
		ComponentLinkDiagramID     *string         `json:"componentLinkDiagramId"`
		ComponentLinkAPIEndpointID *string         `json:"componentLinkApiEndpointId"`
		ComponentLinkTestPackID    *string         `json:"componentLinkTestPackId"`
		ComponentLinkServiceDocID  *string         `json:"componentLinkServiceDocId"`
		ComponentModalFields       json.RawMessage `json:"componentModalFields"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}

	now := time.Now().UTC()
	m := uimap.FocalPointMeta{
		ID:                         uuid.NewString(),
		FocalPointID:               fpID,
		OrgID:                      orgID,
		FrameID:                    frameID,
		ComponentID:                body.ComponentID,
		ComponentLinkDiagramID:     body.ComponentLinkDiagramID,
		ComponentLinkAPIEndpointID: body.ComponentLinkAPIEndpointID,
		ComponentLinkTestPackID:    body.ComponentLinkTestPackID,
		ComponentLinkServiceDocID:  body.ComponentLinkServiceDocID,
		ComponentModalFields:       body.ComponentModalFields,
		CreatedBy:                  p.UserID,
		CreatedAt:                  now,
		UpdatedAt:                  now,
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
		ComponentID                *string         `json:"componentId"`
		ComponentLinkDiagramID     *string         `json:"componentLinkDiagramId"`
		ComponentLinkAPIEndpointID *string         `json:"componentLinkApiEndpointId"`
		ComponentLinkTestPackID    *string         `json:"componentLinkTestPackId"`
		ComponentLinkServiceDocID  *string         `json:"componentLinkServiceDocId"`
		ComponentModalFields       json.RawMessage `json:"componentModalFields"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.ComponentID != nil {
		m.ComponentID = *body.ComponentID
	}
	if body.ComponentLinkDiagramID != nil {
		m.ComponentLinkDiagramID = body.ComponentLinkDiagramID
	}
	if body.ComponentLinkAPIEndpointID != nil {
		m.ComponentLinkAPIEndpointID = body.ComponentLinkAPIEndpointID
	}
	if body.ComponentLinkTestPackID != nil {
		m.ComponentLinkTestPackID = body.ComponentLinkTestPackID
	}
	if body.ComponentLinkServiceDocID != nil {
		m.ComponentLinkServiceDocID = body.ComponentLinkServiceDocID
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
