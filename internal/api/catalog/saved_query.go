package catalog

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	catalogpkg "github.com/uigraph/app/internal/catalog"
	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
)

// ensureDBInService verifies the db exists, isn't deleted, and belongs to serviceID.
// Assumes ensureServiceInOrg has already validated serviceID belongs to orgID.
func (h *Handler) ensureDBInService(w http.ResponseWriter, r *http.Request, serviceID, dbID string) bool {
	db, err := h.store.GetServiceDB(r.Context(), dbID)
	if err != nil {
		httputil.Error(w, r, err)
		return false
	}
	if db == nil || db.DeletedAt != nil || db.ServiceID != serviceID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return false
	}
	return true
}

func parseSavedQueryScope(w http.ResponseWriter, r *http.Request) (catalogpkg.SavedQueryScope, bool) {
	switch catalogpkg.SavedQueryScope(r.URL.Query().Get("scope")) {
	case catalogpkg.SavedQueryScopePersonal:
		return catalogpkg.SavedQueryScopePersonal, true
	case catalogpkg.SavedQueryScopeTeam:
		return catalogpkg.SavedQueryScopeTeam, true
	default:
		httputil.BadRequest(w, "scope must be 'personal' or 'team'")
		return "", false
	}
}

func (h *Handler) ListSavedQueryFolders(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	dbID := r.PathValue("dbID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	if ok := h.ensureDBInService(w, r, serviceID, dbID); !ok {
		return
	}
	scope, ok := parseSavedQueryScope(w, r)
	if !ok {
		return
	}
	var ownerUserID *string
	if scope == catalogpkg.SavedQueryScopePersonal {
		ownerUserID = &p.UserID
	}
	folders, err := h.store.ListSavedQueryFolders(r.Context(), dbID, scope, ownerUserID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"folders": folders})
}

func (h *Handler) CreateSavedQueryFolder(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	orgID := r.PathValue("orgID")
	dbID := r.PathValue("dbID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	if ok := h.ensureDBInService(w, r, serviceID, dbID); !ok {
		return
	}

	var body struct {
		Name  string                     `json:"name"`
		Scope catalogpkg.SavedQueryScope `json:"scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}
	if body.Scope != catalogpkg.SavedQueryScopePersonal && body.Scope != catalogpkg.SavedQueryScopeTeam {
		httputil.BadRequest(w, "scope must be 'personal' or 'team'")
		return
	}

	now := time.Now().UTC()
	folder := catalogpkg.SavedQueryFolder{
		ID:          uuid.NewString(),
		OrgID:       orgID,
		ServiceDBID: dbID,
		Scope:       body.Scope,
		Name:        body.Name,
		CreatedBy:   p.UserID,
		UpdatedBy:   &p.UserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if body.Scope == catalogpkg.SavedQueryScopePersonal {
		folder.OwnerUserID = &p.UserID
	}
	if err := h.store.CreateSavedQueryFolder(r.Context(), folder); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, folder)
}

func (h *Handler) DeleteSavedQueryFolder(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	dbID := r.PathValue("dbID")
	folderID := r.PathValue("folderID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	if ok := h.ensureDBInService(w, r, serviceID, dbID); !ok {
		return
	}
	folder, err := h.store.GetSavedQueryFolder(r.Context(), folderID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if folder == nil || folder.DeletedAt != nil || folder.ServiceDBID != dbID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if folder.Scope == catalogpkg.SavedQueryScopePersonal && (folder.OwnerUserID == nil || *folder.OwnerUserID != p.UserID) {
		httputil.Forbidden(w)
		return
	}
	if err := h.store.SoftDeleteSavedQueryFolder(r.Context(), folderID, p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListSavedQueries(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	dbID := r.PathValue("dbID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	if ok := h.ensureDBInService(w, r, serviceID, dbID); !ok {
		return
	}
	scope, ok := parseSavedQueryScope(w, r)
	if !ok {
		return
	}
	var ownerUserID *string
	if scope == catalogpkg.SavedQueryScopePersonal {
		ownerUserID = &p.UserID
	}
	queries, err := h.store.ListSavedQueries(r.Context(), dbID, scope, ownerUserID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"queries": queries})
}

func (h *Handler) CreateSavedQuery(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	orgID := r.PathValue("orgID")
	dbID := r.PathValue("dbID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	if ok := h.ensureDBInService(w, r, serviceID, dbID); !ok {
		return
	}

	var body struct {
		Title       string                     `json:"title"`
		Description string                     `json:"description"`
		QueryText   string                     `json:"queryText"`
		Tags        []string                   `json:"tags"`
		FolderID    *string                    `json:"folderId"`
		Scope       catalogpkg.SavedQueryScope `json:"scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Title == "" {
		httputil.BadRequest(w, "title is required")
		return
	}
	if body.Scope != catalogpkg.SavedQueryScopePersonal && body.Scope != catalogpkg.SavedQueryScopeTeam {
		httputil.BadRequest(w, "scope must be 'personal' or 'team'")
		return
	}

	source := "ui"
	now := time.Now().UTC()
	q := catalogpkg.SavedQuery{
		ID:          uuid.NewString(),
		OrgID:       orgID,
		ServiceDBID: dbID,
		FolderID:    body.FolderID,
		Scope:       body.Scope,
		Title:       body.Title,
		Description: body.Description,
		QueryText:   body.QueryText,
		Tags:        body.Tags,
		Source:      &source,
		CreatedBy:   p.UserID,
		UpdatedBy:   &p.UserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if body.Scope == catalogpkg.SavedQueryScopePersonal {
		q.OwnerUserID = &p.UserID
	}
	if err := h.store.CreateSavedQuery(r.Context(), q); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, q)
}

func (h *Handler) UpdateSavedQuery(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	dbID := r.PathValue("dbID")
	queryID := r.PathValue("queryID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	if ok := h.ensureDBInService(w, r, serviceID, dbID); !ok {
		return
	}
	q, err := h.store.GetSavedQuery(r.Context(), queryID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if q == nil || q.DeletedAt != nil || q.ServiceDBID != dbID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if q.Scope == catalogpkg.SavedQueryScopePersonal && (q.OwnerUserID == nil || *q.OwnerUserID != p.UserID) {
		httputil.Forbidden(w)
		return
	}

	var body struct {
		Title       *string  `json:"title"`
		Description *string  `json:"description"`
		QueryText   *string  `json:"queryText"`
		Tags        []string `json:"tags"`
		FolderID    *string  `json:"folderId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Title != nil {
		q.Title = *body.Title
	}
	if body.Description != nil {
		q.Description = *body.Description
	}
	if body.QueryText != nil {
		q.QueryText = *body.QueryText
	}
	if body.Tags != nil {
		q.Tags = body.Tags
	}
	q.FolderID = body.FolderID
	q.UpdatedBy = &p.UserID

	if err := h.store.UpdateSavedQuery(r.Context(), *q); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, q)
}

func (h *Handler) DeleteSavedQuery(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	dbID := r.PathValue("dbID")
	queryID := r.PathValue("queryID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	if ok := h.ensureDBInService(w, r, serviceID, dbID); !ok {
		return
	}
	q, err := h.store.GetSavedQuery(r.Context(), queryID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if q == nil || q.DeletedAt != nil || q.ServiceDBID != dbID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if q.Scope == catalogpkg.SavedQueryScopePersonal && (q.OwnerUserID == nil || *q.OwnerUserID != p.UserID) {
		httputil.Forbidden(w)
		return
	}
	if err := h.store.SoftDeleteSavedQuery(r.Context(), queryID, p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SyncSavedQuery is the CI/CLI-facing upsert endpoint, called only from
// uigraph-gateway with a service-account principal. It always writes a
// team-scoped, org-shared, source="ci" row and relies on a real Postgres
// unique constraint on (service_db_id, source_ref) to stay duplicate-free
// even under concurrent syncs.
func (h *Handler) SyncSavedQuery(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	orgID := r.PathValue("orgID")
	dbID := r.PathValue("dbID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	if ok := h.ensureDBInService(w, r, serviceID, dbID); !ok {
		return
	}

	var body struct {
		SourceRef   string   `json:"sourceRef"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		QueryText   string   `json:"queryText"`
		Tags        []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.SourceRef == "" {
		httputil.BadRequest(w, "sourceRef is required")
		return
	}
	if body.Title == "" {
		httputil.BadRequest(w, "title is required")
		return
	}

	source := "ci"
	q := catalogpkg.SavedQuery{
		ID:          uuid.NewString(),
		OrgID:       orgID,
		ServiceDBID: dbID,
		Scope:       catalogpkg.SavedQueryScopeTeam,
		Title:       body.Title,
		Description: body.Description,
		QueryText:   body.QueryText,
		Tags:        body.Tags,
		Source:      &source,
		SourceRef:   &body.SourceRef,
		CreatedBy:   p.UserID,
		UpdatedBy:   &p.UserID,
	}
	result, created, err := h.store.UpsertSavedQueryBySourceRef(r.Context(), q)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"id": result.ID, "created": created})
}
