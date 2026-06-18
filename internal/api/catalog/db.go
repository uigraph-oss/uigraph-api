package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	catalogpkg "github.com/uigraph/app/internal/catalog"
	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
)

func (h *Handler) ListDBs(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	dbs, err := h.store.ListServiceDBs(r.Context(), serviceID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"dbs": dbs})
}

func (h *Handler) GetDB(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	db, err := h.store.GetServiceDB(r.Context(), r.PathValue("dbID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if db == nil || db.DeletedAt != nil || db.ServiceID != serviceID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, db)
}

func (h *Handler) CreateDB(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	serviceID := r.PathValue("serviceID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}

	var body struct {
		DBName     string          `json:"dbName"`
		DBType     string          `json:"dbType"`
		Dialect    string          `json:"dialect"`
		SchemaJSON json.RawMessage `json:"schemaJson"`
		Source     *string         `json:"source"`
		SourceTS   *time.Time      `json:"sourceTs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.DBName == "" {
		httputil.BadRequest(w, "dbName is required")
		return
	}
	if len(body.SchemaJSON) == 0 {
		body.SchemaJSON = json.RawMessage("{}")
	}
	now := time.Now().UTC()
	db := catalogpkg.ServiceDB{
		ID:         uuid.NewString(),
		ServiceID:  serviceID,
		OrgID:      orgID,
		DBName:     body.DBName,
		DBType:     body.DBType,
		Dialect:    body.Dialect,
		SchemaJSON: body.SchemaJSON,
		Source:     body.Source,
		SourceTS:   body.SourceTS,
		CreatedBy:  p.UserID,
		UpdatedBy:  &p.UserID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := h.store.CreateServiceDB(r.Context(), db); err != nil {
		httputil.Error(w, r, err)
		return
	}
	_, _ = h.createServiceDBVersionSnapshot(r, db, nil, true, p.UserID)
	httputil.JSON(w, http.StatusCreated, db)
}

func (h *Handler) UpdateDB(w http.ResponseWriter, r *http.Request) {
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
	db, err := h.store.GetServiceDB(r.Context(), dbID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if db == nil || db.DeletedAt != nil || db.ServiceID != serviceID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	prevSchema := append(json.RawMessage(nil), db.SchemaJSON...)
	var body struct {
		DBName     *string         `json:"dbName"`
		DBType     *string         `json:"dbType"`
		Dialect    *string         `json:"dialect"`
		SchemaJSON json.RawMessage `json:"schemaJson"`
		Source     *string         `json:"source"`
		SourceTS   *time.Time      `json:"sourceTs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.DBName != nil {
		db.DBName = *body.DBName
	}
	if body.DBType != nil {
		db.DBType = *body.DBType
	}
	if body.Dialect != nil {
		db.Dialect = *body.Dialect
	}
	if body.SchemaJSON != nil {
		db.SchemaJSON = body.SchemaJSON
	}
	db.Source = body.Source
	db.SourceTS = body.SourceTS
	db.UpdatedBy = &p.UserID

	if err := h.store.UpdateServiceDB(r.Context(), *db); err != nil {
		httputil.Error(w, r, err)
		return
	}
	if !bytes.Equal(prevSchema, db.SchemaJSON) {
		_, _ = h.createServiceDBVersionSnapshot(r, *db, nil, true, p.UserID)
	}
	httputil.JSON(w, http.StatusOK, db)
}

func (h *Handler) DeleteDB(w http.ResponseWriter, r *http.Request) {
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
	db, err := h.store.GetServiceDB(r.Context(), dbID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if db == nil || db.DeletedAt != nil || db.ServiceID != serviceID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if err := h.store.SoftDeleteServiceDB(r.Context(), dbID, p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListDBVersions(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	dbID := r.PathValue("dbID")
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	db, err := h.store.GetServiceDB(r.Context(), dbID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if db == nil || db.DeletedAt != nil || db.ServiceID != serviceID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	versions, err := h.store.ListServiceDBVersions(r.Context(), dbID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"versions": versions})
}

func (h *Handler) CreateDBVersion(w http.ResponseWriter, r *http.Request) {
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
	db, err := h.store.GetServiceDB(r.Context(), dbID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if db == nil || db.DeletedAt != nil || db.ServiceID != serviceID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	var body struct {
		Label         *string         `json:"label"`
		IsAutoVersion *bool           `json:"isAutoVersion"`
		DBName        *string         `json:"dbName"`
		DBType        *string         `json:"dbType"`
		Dialect       *string         `json:"dialect"`
		SchemaJSON    json.RawMessage `json:"schemaJson"`
		Source        *string         `json:"source"`
		SourceTS      *time.Time      `json:"sourceTs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.DBName != nil {
		db.DBName = *body.DBName
	}
	if body.DBType != nil {
		db.DBType = *body.DBType
	}
	if body.Dialect != nil {
		db.Dialect = *body.Dialect
	}
	if body.SchemaJSON != nil {
		db.SchemaJSON = body.SchemaJSON
	}
	if body.Source != nil {
		db.Source = body.Source
	}
	if body.SourceTS != nil {
		db.SourceTS = body.SourceTS
	}
	db.UpdatedBy = &p.UserID
	if err := h.store.UpdateServiceDB(r.Context(), *db); err != nil {
		httputil.Error(w, r, err)
		return
	}
	isAuto := false
	if body.IsAutoVersion != nil {
		isAuto = *body.IsAutoVersion
	}
	version, err := h.createServiceDBVersionSnapshot(r, *db, body.Label, isAuto, p.UserID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, version)
}

func (h *Handler) RestoreDBVersion(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	dbID := r.PathValue("dbID")
	versionID := r.PathValue("versionID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if ok := h.ensureServiceInOrg(w, r, serviceID); !ok {
		return
	}
	db, err := h.store.GetServiceDB(r.Context(), dbID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if db == nil || db.DeletedAt != nil || db.ServiceID != serviceID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	version, err := h.store.GetServiceDBVersion(r.Context(), versionID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if version == nil || version.ServiceDBID != dbID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	restoreLabel := fmt.Sprintf("Restored from v%d", version.VersionNumber)
	_, _ = h.createServiceDBVersionSnapshot(r, *db, &restoreLabel, true, p.UserID)

	db.SchemaJSON = version.SchemaJSON
	db.Source = version.Source
	db.SourceTS = version.SourceTS
	db.UpdatedBy = &p.UserID
	if err := h.store.UpdateServiceDB(r.Context(), *db); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, db)
}

func (h *Handler) createServiceDBVersionSnapshot(r *http.Request, db catalogpkg.ServiceDB, label *string, isAuto bool, userID string) (*catalogpkg.ServiceDBVersion, error) {
	latest, err := h.store.LatestServiceDBVersionNumber(r.Context(), db.ID)
	if err != nil {
		return nil, err
	}
	version := catalogpkg.ServiceDBVersion{
		ID:            uuid.NewString(),
		ServiceDBID:   db.ID,
		VersionNumber: latest + 1,
		Label:         label,
		SchemaJSON:    db.SchemaJSON,
		Source:        db.Source,
		SourceTS:      db.SourceTS,
		IsAutoVersion: isAuto,
		CreatedBy:     userID,
		CreatedAt:     time.Now().UTC(),
	}
	if err := h.store.CreateServiceDBVersion(r.Context(), version); err != nil {
		return nil, err
	}
	return &version, nil
}
