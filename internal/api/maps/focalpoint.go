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

// @Summary  ListFocalPoints
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
// @Router   /orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points [get]
func (h *Handler) ListFocalPoints(w http.ResponseWriter, r *http.Request) {
	fps, err := h.store.ListFocalPoints(r.Context(), r.PathValue("frameID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"focalPoints": fps})
}

// @Summary  CreateFocalPoint
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
// @Router   /orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points [post]
func (h *Handler) CreateFocalPoint(w http.ResponseWriter, r *http.Request) {
	frameID := r.PathValue("frameID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	var body struct {
		Name       string  `json:"name"`
		LocationX  float64 `json:"locationX"`
		LocationY  float64 `json:"locationY"`
		Visibility string  `json:"visibility"`
		IsActive   bool    `json:"isActive"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" {
		httputil.BadRequest(w, "name is required")
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
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, fp)
}

// @Summary  GetFocalPoint
// @Tags     maps
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    mapID  path  string  true  "mapID"
// @Param    frameID  path  string  true  "frameID"
// @Param    fpID  path  string  true  "fpID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID} [get]
func (h *Handler) GetFocalPoint(w http.ResponseWriter, r *http.Request) {
	fp, err := h.store.GetFocalPoint(r.Context(), r.PathValue("fpID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if fp == nil || fp.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, fp)
}

// @Summary  UpdateFocalPoint
// @Tags     maps
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    mapID  path  string  true  "mapID"
// @Param    frameID  path  string  true  "frameID"
// @Param    fpID  path  string  true  "fpID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID} [put]
func (h *Handler) UpdateFocalPoint(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	fp, err := h.store.GetFocalPoint(r.Context(), r.PathValue("fpID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if fp == nil || fp.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	var body struct {
		Name       *string  `json:"name"`
		LocationX  *float64 `json:"locationX"`
		LocationY  *float64 `json:"locationY"`
		Visibility *string  `json:"visibility"`
		IsActive   *bool    `json:"isActive"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
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
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, fp)
}

// @Summary  DeleteFocalPoint
// @Tags     maps
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    mapID  path  string  true  "mapID"
// @Param    frameID  path  string  true  "frameID"
// @Param    fpID  path  string  true  "fpID"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID} [delete]
func (h *Handler) DeleteFocalPoint(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if err := h.store.SoftDeleteFocalPoint(r.Context(), r.PathValue("fpID"), p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
