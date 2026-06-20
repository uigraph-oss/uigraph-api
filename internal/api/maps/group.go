package maps

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/httputil"
	storepkg "github.com/uigraph/app/internal/store"
	"github.com/uigraph/app/internal/uimap"
)

// ListGroups handles GET /api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups
func (h *Handler) ListGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.store.ListFrameGroups(r.Context(), r.PathValue("frameID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"groups": groups})
}

// CreateGroup handles POST /api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups
func (h *Handler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	frameID := r.PathValue("frameID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
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
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" {
		httputil.BadRequest(w, "name is required")
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
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, g)
}

// UpdateGroup handles PUT /api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups/{groupID}
func (h *Handler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	g, err := h.store.GetFrameGroup(r.Context(), r.PathValue("groupID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if g == nil || g.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
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
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
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
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, g)
}

// DeleteGroup handles DELETE /api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups/{groupID}
func (h *Handler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if err := h.store.SoftDeleteFrameGroup(r.Context(), r.PathValue("groupID"), p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
