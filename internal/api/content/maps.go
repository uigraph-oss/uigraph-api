package content

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/store"
	"github.com/uigraph/app/internal/uimap"
)

// MapHandler serves /api/v1/orgs/{orgID}/maps.
type MapHandler struct {
	store store.Store
}

func NewMapHandler(s store.Store) *MapHandler {
	return &MapHandler{store: s}
}

// List handles GET /api/v1/orgs/{orgID}/maps
func (h *MapHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	q := r.URL.Query()
	var folderID, teamID *string
	if v := q.Get("folderId"); v != "" {
		folderID = &v
	}
	if v := q.Get("teamId"); v != "" {
		teamID = &v
	}
	maps, err := h.store.ListMaps(r.Context(), orgID, folderID, teamID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"maps": maps})
}

// Create handles POST /api/v1/orgs/{orgID}/maps
func (h *MapHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var body struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		FolderID    *string `json:"folderId"`
		TeamID      *string `json:"teamId"`
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
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

// Get handles GET /api/v1/orgs/{orgID}/maps/{mapID}
func (h *MapHandler) Get(w http.ResponseWriter, r *http.Request) {
	m, err := h.store.GetMap(r.Context(), r.PathValue("mapID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if m == nil || m.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// Update handles PUT /api/v1/orgs/{orgID}/maps/{mapID}
func (h *MapHandler) Update(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	m, err := h.store.GetMap(r.Context(), r.PathValue("mapID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if m == nil || m.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Status      *string `json:"status"`
		FolderID    *string `json:"folderId"`
		TeamID      *string `json:"teamId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
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
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// Delete handles DELETE /api/v1/orgs/{orgID}/maps/{mapID}
func (h *MapHandler) Delete(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if err := h.store.SoftDeleteMap(r.Context(), r.PathValue("mapID"), p.UserID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
