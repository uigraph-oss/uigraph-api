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

// List handles GET /api/v1/orgs/{orgID}/folders?type=
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

// Create handles POST /api/v1/orgs/{orgID}/folders
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

// Get handles GET /api/v1/orgs/{orgID}/folders/{folderID}
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

// Update handles PUT /api/v1/orgs/{orgID}/folders/{folderID}
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
	httputil.JSON(w, http.StatusOK, existing)
}

// Delete handles DELETE /api/v1/orgs/{orgID}/folders/{folderID}
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
