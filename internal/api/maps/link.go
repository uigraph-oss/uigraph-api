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

// @Summary  ListLinks
// @Tags     maps
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    mapID  path  string  true  "mapID"
// @Param    frameID  path  string  true  "frameID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/maps/{mapID}/frames/{frameID}/links [get]
func (h *Handler) ListLinks(w http.ResponseWriter, r *http.Request) {
	links, err := h.store.ListFrameLinks(r.Context(), r.PathValue("frameID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"links": links})
}

// @Summary  CreateLink
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
// @Router   /orgs/{orgID}/maps/{mapID}/frames/{frameID}/links [post]
func (h *Handler) CreateLink(w http.ResponseWriter, r *http.Request) {
	frameID := r.PathValue("frameID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
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
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Kind != "frame" && body.Kind != "map" {
		httputil.BadRequest(w, "kind must be 'frame' or 'map'")
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
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, l)
}

// @Summary  UpdateLink
// @Tags     maps
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    mapID  path  string  true  "mapID"
// @Param    frameID  path  string  true  "frameID"
// @Param    linkID  path  string  true  "linkID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/maps/{mapID}/frames/{frameID}/links/{linkID} [put]
func (h *Handler) UpdateLink(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	l, err := h.store.GetFrameLink(r.Context(), r.PathValue("linkID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if l == nil || l.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
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
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
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
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, l)
}

// @Summary  DeleteLink
// @Tags     maps
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    mapID  path  string  true  "mapID"
// @Param    frameID  path  string  true  "frameID"
// @Param    linkID  path  string  true  "linkID"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/maps/{mapID}/frames/{frameID}/links/{linkID} [delete]
func (h *Handler) DeleteLink(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if err := h.store.SoftDeleteFrameLink(r.Context(), r.PathValue("linkID"), p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
