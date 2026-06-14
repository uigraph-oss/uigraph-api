package content

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/uimap"
)

func (h *FrameHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.store.ListFrameGroups(r.Context(), r.PathValue("frameID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups})
}

func (h *FrameHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	frameID := r.PathValue("frameID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var body struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		LocationX   float64 `json:"locationX"`
		LocationY   float64 `json:"locationY"`
		Width       float64 `json:"width"`
		Height      float64 `json:"height"`
		Order       float64 `json:"order"`
		IsActive    *bool   `json:"isActive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}

	now := time.Now().UTC()
	g := uimap.FrameGroup{
		ID:          uuid.NewString(),
		FrameID:     frameID,
		OrgID:       orgID,
		Name:        body.Name,
		Description: body.Description,
		LocationX:   body.LocationX,
		LocationY:   body.LocationY,
		Width:       body.Width,
		Height:      body.Height,
		Order:       body.Order,
		IsActive:    body.IsActive == nil || *body.IsActive,
		CreatedBy:   p.UserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.store.CreateFrameGroup(r.Context(), g); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func (h *FrameHandler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	g, err := h.store.GetFrameGroup(r.Context(), r.PathValue("groupID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if g == nil || g.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	var body struct {
		Name        *string  `json:"name"`
		Description *string  `json:"description"`
		LocationX   *float64 `json:"locationX"`
		LocationY   *float64 `json:"locationY"`
		Width       *float64 `json:"width"`
		Height      *float64 `json:"height"`
		Order       *float64 `json:"order"`
		IsActive    *bool    `json:"isActive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name != nil {
		g.Name = *body.Name
	}
	if body.Description != nil {
		g.Description = *body.Description
	}
	if body.LocationX != nil {
		g.LocationX = *body.LocationX
	}
	if body.LocationY != nil {
		g.LocationY = *body.LocationY
	}
	if body.Width != nil {
		g.Width = *body.Width
	}
	if body.Height != nil {
		g.Height = *body.Height
	}
	if body.Order != nil {
		g.Order = *body.Order
	}
	if body.IsActive != nil {
		g.IsActive = *body.IsActive
	}
	g.UpdatedBy = &p.UserID

	if err := h.store.UpdateFrameGroup(r.Context(), *g); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (h *FrameHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if err := h.store.SoftDeleteFrameGroup(r.Context(), r.PathValue("groupID"), p.UserID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *FrameHandler) ListLinks(w http.ResponseWriter, r *http.Request) {
	links, err := h.store.ListFrameLinks(r.Context(), r.PathValue("frameID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"links": links})
}

func (h *FrameHandler) CreateLink(w http.ResponseWriter, r *http.Request) {
	frameID := r.PathValue("frameID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var body struct {
		Kind          string  `json:"kind"`
		TargetFrameID *string `json:"targetFrameId"`
		TargetMapID   *string `json:"targetMapId"`
		Label         string  `json:"label"`
		LocationX     float64 `json:"locationX"`
		LocationY     float64 `json:"locationY"`
		IsActive      *bool   `json:"isActive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Kind != "frame" && body.Kind != "map" {
		writeErr(w, http.StatusBadRequest, "kind must be 'frame' or 'map'")
		return
	}

	now := time.Now().UTC()
	l := uimap.FrameLink{
		ID:            uuid.NewString(),
		FrameID:       frameID,
		OrgID:         orgID,
		Kind:          body.Kind,
		TargetFrameID: body.TargetFrameID,
		TargetMapID:   body.TargetMapID,
		Label:         body.Label,
		LocationX:     body.LocationX,
		LocationY:     body.LocationY,
		IsActive:      body.IsActive == nil || *body.IsActive,
		CreatedBy:     p.UserID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := h.store.CreateFrameLink(r.Context(), l); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, l)
}

func (h *FrameHandler) UpdateLink(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	l, err := h.store.GetFrameLink(r.Context(), r.PathValue("linkID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if l == nil || l.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	var body struct {
		Kind          *string  `json:"kind"`
		TargetFrameID *string  `json:"targetFrameId"`
		TargetMapID   *string  `json:"targetMapId"`
		Label         *string  `json:"label"`
		LocationX     *float64 `json:"locationX"`
		LocationY     *float64 `json:"locationY"`
		IsActive      *bool    `json:"isActive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Kind != nil {
		l.Kind = *body.Kind
	}
	if body.TargetFrameID != nil {
		l.TargetFrameID = body.TargetFrameID
	}
	if body.TargetMapID != nil {
		l.TargetMapID = body.TargetMapID
	}
	if body.Label != nil {
		l.Label = *body.Label
	}
	if body.LocationX != nil {
		l.LocationX = *body.LocationX
	}
	if body.LocationY != nil {
		l.LocationY = *body.LocationY
	}
	if body.IsActive != nil {
		l.IsActive = *body.IsActive
	}
	l.UpdatedBy = &p.UserID

	if err := h.store.UpdateFrameLink(r.Context(), *l); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (h *FrameHandler) DeleteLink(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if err := h.store.SoftDeleteFrameLink(r.Context(), r.PathValue("linkID"), p.UserID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *FrameHandler) ListMeta(w http.ResponseWriter, r *http.Request) {
	metas, err := h.store.ListFocalPointMeta(r.Context(), r.PathValue("fpID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"meta": metas})
}

func (h *FrameHandler) CreateMeta(w http.ResponseWriter, r *http.Request) {
	fpID := r.PathValue("fpID")
	frameID := r.PathValue("frameID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var body struct {
		ComponentID          string          `json:"componentId"`
		ComponentLinkID      *string         `json:"componentLinkId"`
		ComponentImages      json.RawMessage `json:"componentImages"`
		ComponentFlowDiagram *string         `json:"componentFlowDiagram"`
		ComponentModalFields json.RawMessage `json:"componentModalFields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	now := time.Now().UTC()
	m := uimap.FocalPointMeta{
		ID:                   uuid.NewString(),
		FocalPointID:         fpID,
		OrgID:                orgID,
		FrameID:              frameID,
		ComponentID:          body.ComponentID,
		ComponentLinkID:      body.ComponentLinkID,
		ComponentImages:      body.ComponentImages,
		ComponentFlowDiagram: body.ComponentFlowDiagram,
		ComponentModalFields: body.ComponentModalFields,
		CreatedBy:            p.UserID,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := h.store.CreateFocalPointMeta(r.Context(), m); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (h *FrameHandler) UpdateMeta(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	m, err := h.store.GetFocalPointMeta(r.Context(), r.PathValue("metaID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if m == nil || m.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	var body struct {
		ComponentID          *string         `json:"componentId"`
		ComponentLinkID      *string         `json:"componentLinkId"`
		ComponentImages      json.RawMessage `json:"componentImages"`
		ComponentFlowDiagram *string         `json:"componentFlowDiagram"`
		ComponentModalFields json.RawMessage `json:"componentModalFields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.ComponentID != nil {
		m.ComponentID = *body.ComponentID
	}
	if body.ComponentLinkID != nil {
		m.ComponentLinkID = body.ComponentLinkID
	}
	if body.ComponentImages != nil {
		m.ComponentImages = body.ComponentImages
	}
	if body.ComponentFlowDiagram != nil {
		m.ComponentFlowDiagram = body.ComponentFlowDiagram
	}
	if body.ComponentModalFields != nil {
		m.ComponentModalFields = body.ComponentModalFields
	}
	m.UpdatedBy = &p.UserID

	if err := h.store.UpdateFocalPointMeta(r.Context(), *m); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (h *FrameHandler) DeleteMeta(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if err := h.store.SoftDeleteFocalPointMeta(r.Context(), r.PathValue("metaID"), p.UserID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
