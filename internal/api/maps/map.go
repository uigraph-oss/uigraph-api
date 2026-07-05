package maps

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
	"github.com/uigraph/app/internal/uimap"
)

// @Summary  ListMaps
// @Tags     maps
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/maps [get]
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

// @Summary  CreateMap
// @Tags     maps
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/maps [post]
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
		CommitHash  *string `json:"commitHash"`
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
		ID:                  uuid.NewString(),
		OrgID:               orgID,
		FolderID:            body.FolderID,
		TeamID:              body.TeamID,
		Name:                body.Name,
		Description:         body.Description,
		Status:              "active",
		CreatedBy:           p.UserID,
		CreatedByCommitHash: body.CommitHash,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := h.store.CreateMap(r.Context(), m); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, m)
}

// @Summary  GetMap
// @Tags     maps
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    mapID  path  string  true  "mapID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/maps/{mapID} [get]
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

// @Summary  UpdateMap
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
// @Router   /orgs/{orgID}/maps/{mapID} [put]
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
		CommitHash  *string `json:"commitHash"`
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
	m.UpdatedByCommitHash = body.CommitHash

	if err := h.store.UpdateMap(r.Context(), *m); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, m)
}

// @Summary  DeleteMap
// @Tags     maps
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    mapID  path  string  true  "mapID"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/maps/{mapID} [delete]
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
