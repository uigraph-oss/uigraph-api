// Package content holds handlers for folders, diagrams, maps, frames, and
// service catalog resources.
package content

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/uigraph/app/internal/folder"
	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/store"
)

// FolderHandler serves /api/v1/orgs/{orgID}/folders.
type FolderHandler struct {
	store store.Store
}

func NewFolderHandler(s store.Store) *FolderHandler {
	return &FolderHandler{store: s}
}

// List handles GET /api/v1/orgs/{orgID}/folders?type=
func (h *FolderHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	var t *folder.Type
	if raw := r.URL.Query().Get("type"); raw != "" {
		ft := folder.Type(raw)
		t = &ft
	}
	folders, err := h.store.ListFolders(r.Context(), orgID, t)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"folders": folders})
}

// Create handles POST /api/v1/orgs/{orgID}/folders
func (h *FolderHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var body struct {
		Name     string       `json:"name"`
		Type     folder.Type  `json:"type"`
		ParentID *string      `json:"parentId"`
		Order    float64      `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" || body.Type == "" {
		writeErr(w, http.StatusBadRequest, "name and type are required")
		return
	}

	now := time.Now().UTC()
	f := folder.Folder{
		ID:        uuid.NewString(),
		OrgID:     orgID,
		ParentID:  body.ParentID,
		Type:      body.Type,
		Name:      body.Name,
		Order:     body.Order,
		CreatedBy: p.UserID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.store.CreateFolder(r.Context(), f); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

// Get handles GET /api/v1/orgs/{orgID}/folders/{folderID}
func (h *FolderHandler) Get(w http.ResponseWriter, r *http.Request) {
	f, err := h.store.GetFolder(r.Context(), r.PathValue("folderID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if f == nil || f.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, f)
}

// Update handles PUT /api/v1/orgs/{orgID}/folders/{folderID}
func (h *FolderHandler) Update(w http.ResponseWriter, r *http.Request) {
	existing, err := h.store.GetFolder(r.Context(), r.PathValue("folderID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil || existing.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	var body struct {
		Name     *string  `json:"name"`
		ParentID *string  `json:"parentId"`
		Order    *float64 `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name != nil {
		existing.Name = *body.Name
	}
	if body.ParentID != nil {
		existing.ParentID = body.ParentID
	}
	if body.Order != nil {
		existing.Order = *body.Order
	}
	if err := h.store.UpdateFolder(r.Context(), *existing); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

// Delete handles DELETE /api/v1/orgs/{orgID}/folders/{folderID}
func (h *FolderHandler) Delete(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if err := h.store.DeleteFolder(r.Context(), r.PathValue("folderID"), p.UserID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
