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

// ListMaps handles GET /api/v1/orgs/{orgID}/maps
func (h *Handler) ListMaps(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	q := r.URL.Query()
	p := uimap.ListParams{
		SortBy:  q.Get("sortBy"),
		SortDir: q.Get("sortDir"),
	}
	if v := q.Get("limit"); v != "" {
		p.Limit = httputil.ListLimit(v)
		p.Offset = httputil.ListOffset(q.Get("offset"))
	}
	if v := q.Get("folderId"); v != "" {
		p.FolderID = &v
	}
	if v := q.Get("teamId"); v != "" {
		p.TeamID = &v
	}
	if v := q.Get("search"); v != "" {
		p.Search = &v
	}
	maps, total, err := h.store.ListMaps(r.Context(), orgID, p)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"maps": maps, "total": total})
}

// CreateMap handles POST /api/v1/orgs/{orgID}/maps
func (h *Handler) CreateMap(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	var body struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		FolderID    *string `json:"folderId"`
		TeamID      *string `json:"teamId"`
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
	m := uimap.Map{
		ID:          uuid.NewString(),
		OrgID:       orgID,
		FolderID:    body.FolderID,
		TeamID:      body.TeamID,
		Name:        body.Name,
		Description: body.Description,
		Status:      "active",
		CreatedBy:   p.UserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.store.CreateMap(r.Context(), m); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, m)
}

// GetMap handles GET /api/v1/orgs/{orgID}/maps/{mapID}
func (h *Handler) GetMap(w http.ResponseWriter, r *http.Request) {
	m, err := h.store.GetMap(r.Context(), r.PathValue("mapID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if m == nil || m.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, m)
}

// UpdateMap handles PUT /api/v1/orgs/{orgID}/maps/{mapID}
func (h *Handler) UpdateMap(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	m, err := h.store.GetMap(r.Context(), r.PathValue("mapID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if m == nil || m.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Status      *string `json:"status"`
		FolderID    *string `json:"folderId"`
		TeamID      *string `json:"teamId"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name != nil {
		m.Name = *body.Name
	}
	if body.Description != nil {
		m.Description = *body.Description
	}
	if body.Status != nil {
		m.Status = *body.Status
	}
	if body.FolderID != nil {
		m.FolderID = body.FolderID
	}
	if body.TeamID != nil {
		m.TeamID = body.TeamID
	}
	m.UpdatedBy = &p.UserID

	if err := h.store.UpdateMap(r.Context(), *m); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, m)
}

// DeleteMap handles DELETE /api/v1/orgs/{orgID}/maps/{mapID}
func (h *Handler) DeleteMap(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if err := h.store.SoftDeleteMap(r.Context(), r.PathValue("mapID"), p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
