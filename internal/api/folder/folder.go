package folder

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/uigraph/app/internal/folder"
	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
)

// @Summary  List folders
// @Tags     folders
// @Security BearerAuth
// @Param    orgID  path      string  true   "Org ID"
// @Param    type   query     string  false  "Filter by folder type"
// @Success  200    {object}  map[string]interface{}  "envelope: {folders: []folder.Folder}"
// @Failure  401    {object}  httputil.errorBody
// @Failure  403    {object}  httputil.errorBody
// @Failure  500    {object}  httputil.errorBody
// @Router   /orgs/{orgID}/folders [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	var t *folder.Type
	if raw := r.URL.Query().Get("type"); raw != "" {
		ft := folder.Type(raw)
		t = &ft
	}
	folders, err := h.store.ListFolders(r.Context(), orgID, t)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"folders": folders})
}

// @Summary  Create a folder
// @Tags     folders
// @Security BearerAuth
// @Param    orgID  path      string  true  "Org ID"
// @Param    body   body      object  true  "Folder fields: name, type, parentId, teamId, order"
// @Success  201    {object}  folder.Folder
// @Failure  400    {object}  httputil.errorBody
// @Failure  401    {object}  httputil.errorBody
// @Failure  403    {object}  httputil.errorBody
// @Failure  500    {object}  httputil.errorBody
// @Router   /orgs/{orgID}/folders [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	var body struct {
		Name     string      `json:"name"`
		Type     folder.Type `json:"type"`
		ParentID *string     `json:"parentId"`
		TeamID   *string     `json:"teamId"`
		Order    float64     `json:"order"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" || body.Type == "" {
		httputil.BadRequest(w, "name and type are required")
		return
	}

	now := time.Now().UTC()
	f := folder.Folder{
		ID:        uuid.NewString(),
		OrgID:     orgID,
		ParentID:  body.ParentID,
		TeamID:    body.TeamID,
		Type:      body.Type,
		Name:      body.Name,
		Order:     body.Order,
		CreatedBy: p.UserID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.store.CreateFolder(r.Context(), f); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, f)
}

// @Summary  Get a folder
// @Tags     folders
// @Security BearerAuth
// @Param    orgID     path      string  true  "Org ID"
// @Param    folderID  path      string  true  "Folder ID"
// @Success  200       {object}  folder.Folder
// @Failure  401       {object}  httputil.errorBody
// @Failure  403       {object}  httputil.errorBody
// @Failure  404       {object}  httputil.errorBody
// @Router   /orgs/{orgID}/folders/{folderID} [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	f, err := h.store.GetFolder(r.Context(), r.PathValue("folderID"))
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

// @Summary  Update a folder
// @Tags     folders
// @Security BearerAuth
// @Param    orgID     path      string  true  "Org ID"
// @Param    folderID  path      string  true  "Folder ID"
// @Param    body      body      object  true  "Updatable fields: name, parentId, teamId, order"
// @Success  200       {object}  folder.Folder
// @Failure  400       {object}  httputil.errorBody
// @Failure  401       {object}  httputil.errorBody
// @Failure  403       {object}  httputil.errorBody
// @Failure  404       {object}  httputil.errorBody
// @Router   /orgs/{orgID}/folders/{folderID} [put]
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	existing, err := h.store.GetFolder(r.Context(), r.PathValue("folderID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil || existing.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	var body struct {
		Name     *string  `json:"name"`
		ParentID *string  `json:"parentId"`
		TeamID   *string  `json:"teamId"`
		Order    *float64 `json:"order"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name != nil {
		existing.Name = *body.Name
	}
	if body.ParentID != nil {
		existing.ParentID = body.ParentID
	}
	if body.TeamID != nil {
		existing.TeamID = body.TeamID
	}
	if body.Order != nil {
		existing.Order = *body.Order
	}
	if err := h.store.UpdateFolder(r.Context(), *existing); err != nil {
		httputil.Error(w, r, err)
		return
	}
	existing.UpdatedAt = time.Now().UTC()
	httputil.JSON(w, http.StatusOK, existing)
}

// @Summary  Delete a folder
// @Tags     folders
// @Security BearerAuth
// @Param    orgID     path  string  true  "Org ID"
// @Param    folderID  path  string  true  "Folder ID"
// @Success  204       "No Content"
// @Failure  401       {object}  httputil.errorBody
// @Failure  403       {object}  httputil.errorBody
// @Router   /orgs/{orgID}/folders/{folderID} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if err := h.store.DeleteFolder(r.Context(), r.PathValue("folderID"), p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
