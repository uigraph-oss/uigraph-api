# Folder Structure Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reorganise `uigraph-api` into a clean, open-source-ready package layout — domain-aligned handler packages, narrow store interfaces, no god packages, no duplicated helpers.

**Architecture:** Split `internal/api/content/` (13-file god package) into one handler package per domain (`folder/`, `component/`, `actor/`, `asset/`, `diagram/`, `maps/`, `catalog/`, `auth/avatar`). Each new handler package declares the minimal store interface it needs instead of accepting the 50-method aggregate `store.Store`. Shared HTTP helpers live exclusively in `httputil/`. Router decomposes into per-domain `Register()` calls.

**Tech Stack:** Go 1.25, `net/http` stdlib mux, Postgres (`lib/pq`), MinIO (`minio-go`), Redis (`go-redis`), `log/slog`.

**Verify after every task:** `go build ./...` must pass before committing.

---

## Phase 1 — Foundations

*No handler logic changes. Everything compiles and tests pass at the end of each task.*

---

### Task 1: Add `StreamObject` to `httputil` and remove `content/helpers.go` duplication

`content/helpers.go` has private `writeJSON`, `writeErr`, and `streamObject` helpers. `httputil/respond.go` already has superior replacements (`JSON`, `Error`, `BadRequest`, etc.) plus error-aware logging. The only missing piece is `StreamObject`. Add it to `httputil`, then content's private helpers become dead weight (they are deleted in Phase 3 when `content/` is removed).

**Files:**
- Modify: `internal/httputil/respond.go`

- [ ] **Step 1: Add `StreamObject` to `internal/httputil/respond.go`**

  Append at the bottom of the file (after `apiError`):

  ```go
  // StreamObject proxies an object from storage to the HTTP response.
  // It sniffs the first 512 bytes to detect Content-Type.
  func StreamObject(w http.ResponseWriter, r *http.Request, download func(context.Context, string) (io.ReadCloser, error), key string) {
  	rc, err := download(r.Context(), key)
  	if err != nil {
  		Error(w, r, store.ErrNotFound)
  		return
  	}
  	defer rc.Close()

  	head := make([]byte, 512)
  	n, err := io.ReadFull(rc, head)
  	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
  		slog.ErrorContext(r.Context(), "stream object: read head", "err", err, "key", key)
  		JSON(w, http.StatusInternalServerError, apiError("internal_error", "failed to read object"))
  		return
  	}
  	head = head[:n]

  	w.Header().Set("Content-Type", http.DetectContentType(head))
  	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
  	w.WriteHeader(http.StatusOK)
  	_, _ = w.Write(head)
  	_, _ = io.Copy(w, rc)
  }
  ```

  Add required imports to the import block:

  ```go
  import (
  	"context"
  	"encoding/json"
  	"errors"
  	"io"
  	"log/slog"
  	"net/http"

  	"github.com/uigraph/app/internal/store"
  )
  ```

- [ ] **Step 2: Verify build**

  ```bash
  go build ./internal/httputil/...
  ```

  Expected: no output (success).

- [ ] **Step 3: Commit**

  ```bash
  git add internal/httputil/respond.go
  git commit -m "feat(httputil): add StreamObject helper for object-storage proxying"
  ```

---

### Task 2: Delete dead `internal/seed/` package

`cmd/seed/main.go` was deleted (visible in git status as ` D cmd/seed/main.go`). `internal/seed/` is now unreachable dead code. Remove it.

**Files:**
- Delete: `internal/seed/seed.go`
- Delete: `internal/seed/providers.go`

- [ ] **Step 1: Delete the seed package**

  ```bash
  rm -rf internal/seed/
  ```

- [ ] **Step 2: Verify nothing imports it**

  ```bash
  grep -r '"github.com/uigraph/app/internal/seed"' .
  ```

  Expected: no output.

- [ ] **Step 3: Verify build**

  ```bash
  go build ./...
  ```

  Expected: no output (success).

- [ ] **Step 4: Commit**

  ```bash
  git add -A
  git commit -m "chore: delete orphaned internal/seed package (cmd/seed was removed)"
  ```

---

### Task 3: Rename `internal/componentcatalog/` → `internal/componentlib/`

The name `componentcatalog` collides visually with `catalog` (the service catalog domain). Rename to `componentlib`.

**Files:**
- Rename directory: `internal/componentcatalog/` → `internal/componentlib/`
- Update package declaration in all 5 files
- Update imports in: `internal/store/store.go`, `internal/store/postgres/component_catalog.go`, `internal/bootstrap/components.go`, `internal/api/content/components.go`, `internal/api/content/flow_components.go`, `internal/api/router.go`

- [ ] **Step 1: Rename the directory and update package declarations**

  ```bash
  mv internal/componentcatalog internal/componentlib
  ```

  In each of the 5 files inside `internal/componentlib/`, change the first line from:

  ```go
  package componentcatalog
  ```

  to:

  ```go
  package componentlib
  ```

  Files to update:
  - `internal/componentlib/catalog.go`
  - `internal/componentlib/convert.go`
  - `internal/componentlib/loader.go`

- [ ] **Step 2: Update all import paths**

  ```bash
  find . -name "*.go" | xargs grep -l 'uigraph/app/internal/componentcatalog' | sort
  ```

  For each file returned, replace:
  ```
  "github.com/uigraph/app/internal/componentcatalog"
  ```
  with:
  ```
  "github.com/uigraph/app/internal/componentlib"
  ```

  And replace every usage of `componentcatalog.` with `componentlib.` in those files.

- [ ] **Step 3: Update `internal/store/store.go`**

  Change:
  ```go
  import (
      ...
      "github.com/uigraph/app/internal/componentcatalog"
      ...
  )

  type Store interface {
      ...
      componentcatalog.Store
  }
  ```

  To:
  ```go
  import (
      ...
      "github.com/uigraph/app/internal/componentlib"
      ...
  )

  type Store interface {
      ...
      componentlib.Store
  }
  ```

- [ ] **Step 4: Verify build**

  ```bash
  go build ./...
  ```

  Expected: no output.

- [ ] **Step 5: Run existing tests**

  ```bash
  go test ./internal/componentlib/... -v
  ```

  Expected: all pass (or no test files — that's fine).

- [ ] **Step 6: Commit**

  ```bash
  git add -A
  git commit -m "refactor: rename componentcatalog → componentlib to avoid naming collision"
  ```

---

### Task 4: Rename `internal/authz/rbac_store.go` → `internal/authz/store.go`

Aligns with the naming convention used by other domain packages (`folder/folder.go` has its Store interface, etc.). Pure file rename — no code changes.

**Files:**
- Rename: `internal/authz/rbac_store.go` → `internal/authz/store.go`

- [ ] **Step 1: Rename the file**

  ```bash
  mv internal/authz/rbac_store.go internal/authz/store.go
  ```

- [ ] **Step 2: Verify build**

  ```bash
  go build ./internal/authz/...
  ```

  Expected: no output.

- [ ] **Step 3: Commit**

  ```bash
  git add -A
  git commit -m "refactor(authz): rename rbac_store.go → store.go for consistency"
  ```

---

### Task 5: Split `internal/catalog/catalog.go` into types + store interface

`catalog.go` is 468 lines. The `Store` interface starts at line 386. Move it to `internal/catalog/store.go` so types and the persistence contract live in separate files. No logic changes.

**Files:**
- Modify: `internal/catalog/catalog.go` (remove Store interface + catalog Store imports)
- Create: `internal/catalog/store.go`

- [ ] **Step 1: Read lines 386–468 of `catalog.go`**

  ```bash
  sed -n '386,468p' internal/catalog/catalog.go
  ```

  This is the `Store` interface block. Copy it.

- [ ] **Step 2: Create `internal/catalog/store.go`**

  Create the file with the Store interface moved out of `catalog.go`:

  ```go
  package catalog

  import "context"

  // Store is the persistence interface for the service catalog.
  // The postgres implementation lives in store/postgres.
  type Store interface {
      // Services
      CreateService(ctx context.Context, s Service) error
      GetService(ctx context.Context, id string) (*Service, error)
      ListServices(ctx context.Context, orgID string, folderID, teamID *string) ([]Service, error)
      UpdateService(ctx context.Context, s Service) error
      SoftDeleteService(ctx context.Context, id, deletedBy string) error
      ListServiceStats(ctx context.Context, orgID string, serviceID *string) ([]ServiceStats, error)

      // API Groups
      CreateAPIGroup(ctx context.Context, g APIGroup) error
      GetAPIGroup(ctx context.Context, id string) (*APIGroup, error)
      ListAPIGroups(ctx context.Context, serviceID string) ([]APIGroup, error)
      UpdateAPIGroup(ctx context.Context, g APIGroup) error
      SoftDeleteAPIGroup(ctx context.Context, id, deletedBy string) error
      CreateAPIGroupVersion(ctx context.Context, v APIGroupVersion) error
      ListAPIGroupVersions(ctx context.Context, apiGroupID string) ([]APIGroupVersion, error)
      LatestAPIGroupVersionNumber(ctx context.Context, apiGroupID string) (int, error)

      // API Endpoints
      CreateAPIEndpoint(ctx context.Context, e APIEndpoint) error
      GetAPIEndpoint(ctx context.Context, id string) (*APIEndpoint, error)
      ListAPIEndpoints(ctx context.Context, apiGroupID string) ([]APIEndpoint, error)
      UpdateAPIEndpoint(ctx context.Context, e APIEndpoint) error
      SoftDeleteAPIEndpoint(ctx context.Context, id, deletedBy string) error

      // Service Docs
      CreateServiceDoc(ctx context.Context, d ServiceDoc) error
      GetServiceDoc(ctx context.Context, id string) (*ServiceDoc, error)
      ListServiceDocs(ctx context.Context, serviceID string) ([]ServiceDoc, error)
      UpdateServiceDoc(ctx context.Context, d ServiceDoc) error
      SoftDeleteServiceDoc(ctx context.Context, id string) error

      // Service Diagrams
      CreateServiceDiagram(ctx context.Context, sd ServiceDiagram) error
      GetServiceDiagram(ctx context.Context, serviceID, diagramID string) (*ServiceDiagram, error)
      ListServiceDiagrams(ctx context.Context, serviceID string) ([]ServiceDiagram, error)
      SoftDeleteServiceDiagram(ctx context.Context, serviceID, diagramID, deletedBy string) error

      // Service DBs
      CreateServiceDB(ctx context.Context, db ServiceDB) error
      GetServiceDB(ctx context.Context, id string) (*ServiceDB, error)
      ListServiceDBs(ctx context.Context, serviceID string) ([]ServiceDB, error)
      UpdateServiceDB(ctx context.Context, db ServiceDB) error
      SoftDeleteServiceDB(ctx context.Context, id, deletedBy string) error
      CreateServiceDBVersion(ctx context.Context, v ServiceDBVersion) error
      ListServiceDBVersions(ctx context.Context, dbID string) ([]ServiceDBVersion, error)
      LatestServiceDBVersionNumber(ctx context.Context, dbID string) (int, error)
      RestoreServiceDBVersion(ctx context.Context, dbID, versionID, restoredBy string) error

      // Test Packs
      CreateTestPack(ctx context.Context, tp TestPack) error
      GetTestPack(ctx context.Context, id string) (*TestPack, error)
      ListTestPacks(ctx context.Context, serviceID string) ([]TestPack, error)
      UpdateTestPack(ctx context.Context, tp TestPack) error
      SoftDeleteTestPack(ctx context.Context, id, deletedBy string) error

      // Test Cases
      CreateTestCase(ctx context.Context, tc TestCase) error
      GetTestCase(ctx context.Context, id string) (*TestCase, error)
      ListTestCases(ctx context.Context, serviceID string) ([]TestCase, error)
      UpdateTestCase(ctx context.Context, tc TestCase) error
      SoftDeleteTestCase(ctx context.Context, id, deletedBy string) error

      // Test Runs
      CreateTestRun(ctx context.Context, tr TestRun) error
      GetTestRun(ctx context.Context, id string) (*TestRun, error)
      ListTestRuns(ctx context.Context, serviceID string) ([]TestRun, error)
      ListTestRunsSummary(ctx context.Context, serviceID string) ([]TestRunSummary, error)
      UpdateTestRun(ctx context.Context, tr TestRun) error

      // Test Run Results
      CreateTestRunResult(ctx context.Context, r TestRunResult) error
      GetTestRunResult(ctx context.Context, id string) (*TestRunResult, error)
      ListTestRunResults(ctx context.Context, serviceID string) ([]TestRunResult, error)
      UpdateTestRunResult(ctx context.Context, r TestRunResult) error
  }
  ```

  > **Note:** verify the exact method names match what's in `catalog.go` lines 386–468 before saving — copy exactly, don't rely on this list alone.

- [ ] **Step 3: Remove the Store interface from `catalog.go`**

  Delete lines 386–468 (the Store interface and its `import "context"` if that becomes unused).

- [ ] **Step 4: Verify build**

  ```bash
  go build ./...
  ```

  Expected: no output.

- [ ] **Step 5: Commit**

  ```bash
  git add internal/catalog/
  git commit -m "refactor(catalog): split Store interface into catalog/store.go"
  ```

---

## Phase 2 — New Handler Packages

*Create each new handler package from scratch. The old `content/` package is untouched until Phase 3. `go build ./...` passes after every task.*

---

### Task 6: Create `internal/api/folder/`

Simplest handler package — no storage dependency, pure Postgres CRUD.

**Files:**
- Create: `internal/api/folder/handler.go`
- Create: `internal/api/folder/folder.go`

- [ ] **Step 1: Create `internal/api/folder/handler.go`**

  ```go
  // Package folder provides HTTP handlers for folder CRUD.
  package folder

  import (
  	"context"
  	"net/http"

  	"github.com/uigraph/app/internal/folder"
  )

  // store is the minimal persistence interface this package needs.
  // postgres.DB satisfies it automatically.
  type store interface {
  	CreateFolder(ctx context.Context, f folder.Folder) error
  	GetFolder(ctx context.Context, id string) (*folder.Folder, error)
  	ListFolders(ctx context.Context, orgID string, t *folder.Type) ([]folder.Folder, error)
  	UpdateFolder(ctx context.Context, f folder.Folder) error
  	DeleteFolder(ctx context.Context, id, deletedBy string) error
  }

  // Handler serves /api/v1/orgs/{orgID}/folders.
  type Handler struct {
  	store store
  }

  // New constructs a Handler.
  func New(s store) *Handler {
  	return &Handler{store: s}
  }

  // Register wires folder routes into mux under the given middleware wrappers.
  // requireScope has signature: func(scope, method, pattern string, h http.HandlerFunc)
  func Register(
  	mux *http.ServeMux,
  	s store,
  	requireScope func(scope, method, pattern string, h http.HandlerFunc),
  ) {
  	h := New(s)
  	requireScope("folders:read", "GET", "/api/v1/orgs/{orgID}/folders", h.List)
  	requireScope("folders:write", "POST", "/api/v1/orgs/{orgID}/folders", h.Create)
  	requireScope("folders:read", "GET", "/api/v1/orgs/{orgID}/folders/{folderID}", h.Get)
  	requireScope("folders:write", "PUT", "/api/v1/orgs/{orgID}/folders/{folderID}", h.Update)
  	requireScope("folders:write", "DELETE", "/api/v1/orgs/{orgID}/folders/{folderID}", h.Delete)
  }
  ```

- [ ] **Step 2: Create `internal/api/folder/folder.go`**

  Copy handler method bodies verbatim from `internal/api/content/folders.go`, replacing:
  - `package content` → `package folder`
  - `"github.com/uigraph/app/internal/store"` import removed (uses local `store` interface)
  - `writeJSON(` → `httputil.JSON(`
  - `writeErr(w, http.StatusBadRequest, msg)` → `httputil.BadRequest(w, msg)`
  - `writeErr(w, http.StatusUnauthorized, ...)` → `httputil.Unauthorized(w)`
  - `writeErr(w, http.StatusInternalServerError, ...)` → `httputil.Error(w, r, err)`
  - `writeErr(w, http.StatusNotFound, ...)` → `httputil.Error(w, r, store.ErrNotFound)`

  Full file:

  ```go
  package folder

  import (
  	"encoding/json"
  	"net/http"
  	"time"

  	"github.com/google/uuid"

  	"github.com/uigraph/app/internal/folder"
  	"github.com/uigraph/app/internal/httputil"
  	authmw "github.com/uigraph/app/internal/middleware"
  	"github.com/uigraph/app/internal/store"
  )

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

  func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
  	f, err := h.store.GetFolder(r.Context(), r.PathValue("folderID"))
  	if err != nil {
  		httputil.Error(w, r, err)
  		return
  	}
  	if f == nil || f.DeletedAt != nil {
  		httputil.Error(w, r, store.ErrNotFound)
  		return
  	}
  	httputil.JSON(w, http.StatusOK, f)
  }

  func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
  	existing, err := h.store.GetFolder(r.Context(), r.PathValue("folderID"))
  	if err != nil {
  		httputil.Error(w, r, err)
  		return
  	}
  	if existing == nil || existing.DeletedAt != nil {
  		httputil.Error(w, r, store.ErrNotFound)
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
  ```

- [ ] **Step 3: Verify build**

  ```bash
  go build ./internal/api/folder/...
  ```

  Expected: no output.

- [ ] **Step 4: Commit**

  ```bash
  git add internal/api/folder/
  git commit -m "feat(api/folder): new domain-aligned folder handler package with narrow store interface"
  ```

---

### Task 7: Create `internal/api/component/`

Covers the focal-point palette, flow-diagram palette, and component icon file serving. Three small handlers previously scattered across `content/components.go`, `content/flow_components.go`.

**Files:**
- Create: `internal/api/component/handler.go`
- Create: `internal/api/component/focal.go`
- Create: `internal/api/component/flow.go`
- Create: `internal/api/component/icon.go`

- [ ] **Step 1: Create `internal/api/component/handler.go`**

  ```go
  // Package component provides HTTP handlers for component palettes and icons.
  package component

  import (
  	"context"
  	"net/http"

  	"github.com/uigraph/app/internal/componentlib"
  	"github.com/uigraph/app/internal/storage"
  )

  // store is the minimal persistence interface this package needs.
  type store interface {
  	ListComponentsByKind(ctx context.Context, kind componentlib.Kind) ([]componentlib.Component, error)
  }

  // Handler serves component palette and icon endpoints.
  type Handler struct {
  	store   store
  	storage storage.Client
  }

  // New constructs a Handler.
  func New(s store, st storage.Client) *Handler {
  	return &Handler{store: s, storage: st}
  }

  // Register wires component routes into mux.
  // protected wraps a handler requiring authentication (no scope check).
  func Register(
  	mux *http.ServeMux,
  	s store,
  	st storage.Client,
  	protected func(method, pattern string, h http.HandlerFunc),
  ) {
  	h := New(s, st)
  	// Unauthenticated — icon assets are public.
  	mux.HandleFunc("GET /api/v1/component-icons/{slug}", h.GetIcon)
  	// Authenticated.
  	protected("GET", "/api/v1/orgs/{orgID}/components", h.ListFocal)
  	protected("GET", "/api/v1/orgs/{orgID}/flow-diagram-components", h.ListFlow)
  }
  ```

- [ ] **Step 2: Create `internal/api/component/focal.go`**

  ```go
  package component

  import (
  	"net/http"

  	"github.com/uigraph/app/internal/componentlib"
  	"github.com/uigraph/app/internal/httputil"
  )

  // ListFocal handles GET /api/v1/orgs/{orgID}/components
  func (h *Handler) ListFocal(w http.ResponseWriter, r *http.Request) {
  	comps, err := h.store.ListComponentsByKind(r.Context(), componentlib.KindFocalPoint)
  	if err != nil {
  		httputil.Error(w, r, err)
  		return
  	}
  	out := make([]componentlib.FocalPointComponent, len(comps))
  	for i, c := range comps {
  		out[i] = componentlib.ToFocalPointComponent(c, iconURL(r, c))
  	}
  	httputil.JSON(w, http.StatusOK, map[string]any{
  		"components":       out,
  		"customComponents": []componentlib.FocalPointComponent{},
  	})
  }

  func iconURL(r *http.Request, c componentlib.Component) string {
  	return "/api/v1/component-icons/" + componentlib.IconSlug(c)
  }
  ```

- [ ] **Step 3: Create `internal/api/component/flow.go`**

  ```go
  package component

  import (
  	"net/http"

  	"github.com/uigraph/app/internal/componentlib"
  	"github.com/uigraph/app/internal/httputil"
  )

  // ListFlow handles GET /api/v1/orgs/{orgID}/flow-diagram-components
  func (h *Handler) ListFlow(w http.ResponseWriter, r *http.Request) {
  	comps, err := h.store.ListComponentsByKind(r.Context(), componentlib.KindFlowDiagram)
  	if err != nil {
  		httputil.Error(w, r, err)
  		return
  	}
  	out := make([]componentlib.FlowDiagramComponent, len(comps))
  	for i, c := range comps {
  		out[i] = componentlib.ToFlowDiagramComponent(c, iconURL(r, c))
  	}
  	httputil.JSON(w, http.StatusOK, map[string]any{
  		"components":       out,
  		"customComponents": []componentlib.FlowDiagramComponent{},
  	})
  }
  ```

- [ ] **Step 4: Create `internal/api/component/icon.go`**

  ```go
  package component

  import (
  	"net/http"

  	"github.com/uigraph/app/internal/httputil"
  	"github.com/uigraph/app/internal/storage"
  )

  // GetIcon handles GET /api/v1/component-icons/{slug}
  func (h *Handler) GetIcon(w http.ResponseWriter, r *http.Request) {
  	slug := r.PathValue("slug")
  	if slug == "" {
  		httputil.BadRequest(w, "slug is required")
  		return
  	}
  	httputil.StreamObject(w, r, h.storage.Download, storage.ComponentIconKey(slug))
  }
  ```

- [ ] **Step 5: Verify build**

  ```bash
  go build ./internal/api/component/...
  ```

  Expected: no output.

- [ ] **Step 6: Commit**

  ```bash
  git add internal/api/component/
  git commit -m "feat(api/component): new component palette and icon handler package"
  ```

---

### Task 8: Create `internal/api/actor/` and `internal/api/asset/`

These are thin HTTP wrappers over the existing resolver packages (`internal/actor/`, `internal/asset/`). Previously inline in `content/actors.go` and `content/assets.go`.

**Files:**
- Create: `internal/api/actor/handler.go`
- Create: `internal/api/asset/handler.go`

- [ ] **Step 1: Create `internal/api/actor/handler.go`**

  ```go
  // Package actor provides an HTTP handler that resolves actor IDs to public identity info.
  package actor

  import (
  	"net/http"
  	"strings"

  	"github.com/uigraph/app/internal/actor"
  	"github.com/uigraph/app/internal/asset"
  	"github.com/uigraph/app/internal/cache"
  	"github.com/uigraph/app/internal/httputil"
  	"github.com/uigraph/app/internal/storage"
  	"github.com/uigraph/app/internal/store"
  )

  const maxActorIDs = 200

  // Handler wraps actor.Resolver for HTTP.
  type Handler struct {
  	resolver *actor.Resolver
  }

  // New constructs a Handler. s must satisfy actor.Resolver's store requirements.
  func New(s store.Store, c cache.Client, st storage.Client) *Handler {
  	return &Handler{resolver: actor.New(s, c, asset.New(st, c))}
  }

  // Register wires the actor route into mux.
  func Register(
  	mux *http.ServeMux,
  	s store.Store,
  	c cache.Client,
  	st storage.Client,
  	protected func(method, pattern string, h http.HandlerFunc),
  ) {
  	h := New(s, c, st)
  	protected("GET", "/api/v1/orgs/{orgID}/actors", h.Resolve)
  }

  // Resolve handles GET /api/v1/orgs/{orgID}/actors?ids=a,b,c
  func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) {
  	raw := r.URL.Query().Get("ids")
  	if raw == "" {
  		httputil.BadRequest(w, "ids query parameter is required")
  		return
  	}
  	var ids []string
  	for _, part := range strings.Split(raw, ",") {
  		if id := strings.TrimSpace(part); id != "" {
  			ids = append(ids, id)
  		}
  	}
  	if len(ids) == 0 {
  		httputil.BadRequest(w, "ids query parameter is required")
  		return
  	}
  	if len(ids) > maxActorIDs {
  		httputil.BadRequest(w, "too many ids")
  		return
  	}
  	actors, err := h.resolver.ResolveMany(r.Context(), ids)
  	if err != nil {
  		httputil.Error(w, r, err)
  		return
  	}
  	httputil.JSON(w, http.StatusOK, map[string]any{"actors": actors})
  }
  ```

- [ ] **Step 2: Create `internal/api/asset/handler.go`**

  ```go
  // Package asset provides an HTTP handler that resolves asset IDs to presigned URLs.
  package asset

  import (
  	"net/http"
  	"strings"

  	"github.com/uigraph/app/internal/asset"
  	"github.com/uigraph/app/internal/cache"
  	"github.com/uigraph/app/internal/httputil"
  	"github.com/uigraph/app/internal/storage"
  )

  const maxAssetIDs = 200

  // Handler wraps asset.Resolver for HTTP.
  type Handler struct {
  	resolver *asset.Resolver
  }

  // New constructs a Handler.
  func New(st storage.Client, c cache.Client) *Handler {
  	return &Handler{resolver: asset.New(st, c)}
  }

  // Register wires the asset URL route into mux.
  func Register(
  	mux *http.ServeMux,
  	st storage.Client,
  	c cache.Client,
  	protected func(method, pattern string, h http.HandlerFunc),
  ) {
  	h := New(st, c)
  	protected("GET", "/api/v1/orgs/{orgID}/assets/urls", h.Resolve)
  }

  // Resolve handles GET /api/v1/orgs/{orgID}/assets/urls?ids=a,b,c
  func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) {
  	raw := r.URL.Query().Get("ids")
  	if raw == "" {
  		httputil.BadRequest(w, "ids query parameter is required")
  		return
  	}
  	var ids []string
  	for _, part := range strings.Split(raw, ",") {
  		if id := strings.TrimSpace(part); id != "" {
  			ids = append(ids, id)
  		}
  	}
  	if len(ids) == 0 {
  		httputil.BadRequest(w, "ids query parameter is required")
  		return
  	}
  	if len(ids) > maxAssetIDs {
  		httputil.BadRequest(w, "too many ids")
  		return
  	}
  	urls, err := h.resolver.ResolveMany(r.Context(), ids)
  	if err != nil {
  		httputil.Error(w, r, err)
  		return
  	}
  	httputil.JSON(w, http.StatusOK, map[string]any{"urls": urls})
  }
  ```

- [ ] **Step 3: Verify build**

  ```bash
  go build ./internal/api/actor/... ./internal/api/asset/...
  ```

  Expected: no output.

- [ ] **Step 4: Commit**

  ```bash
  git add internal/api/actor/ internal/api/asset/
  git commit -m "feat(api/actor,asset): extract actor and asset resolution handlers from content/"
  ```

---

### Task 8b: Move `content/avatars.go` into `internal/api/auth/`

`content/avatars.go` (162 lines) serves user-avatar and service-account-avatar upload/delete routes. These belong alongside the existing `auth/` handlers since they operate on user and SA identity records.

**Files:**
- Create: `internal/api/auth/avatar.go`

- [ ] **Step 1: Read the source file**

  ```bash
  cat internal/api/content/avatars.go
  ```

- [ ] **Step 2: Create `internal/api/auth/avatar.go`**

  Copy all handler methods verbatim from `content/avatars.go`. Change:
  - `package content` → `package auth`
  - Remove `"github.com/uigraph/app/internal/store"` import (the AvatarHandler struct already accepts `store.Store` in `auth/` since that package predates the narrow-interface refactor — match the existing pattern in `auth/`)
  - `writeJSON(` → `httputil.JSON(`
  - `writeErr(w, http.StatusBadRequest, msg)` → `httputil.BadRequest(w, msg)`
  - `writeErr(w, http.StatusUnauthorized, ...)` → `httputil.Unauthorized(w)`
  - `writeErr(w, http.StatusInternalServerError, ...)` → `httputil.Error(w, r, err)`
  - `writeErr(w, http.StatusNotFound, ...)` → `httputil.Error(w, r, store.ErrNotFound)`

  The `NewAvatarHandler` constructor and `AvatarHandler` struct stay in `avatar.go`. The avatar routes are registered inside the existing auth section of `router.go` (already there — just change the import from `content.NewAvatarHandler` to `auth.NewAvatarHandler`).

- [ ] **Step 3: Verify build**

  ```bash
  go build ./internal/api/auth/...
  ```

  Expected: no output.

- [ ] **Step 4: Commit**

  ```bash
  git add internal/api/auth/avatar.go
  git commit -m "feat(api/auth): move avatar handler from content/ into auth package"
  ```

---

### Task 9: Create `internal/api/diagram/`

Source: `internal/api/content/diagrams.go` (full file not shown here — read it before starting).

**Files:**
- Create: `internal/api/diagram/handler.go`
- Create: `internal/api/diagram/diagram.go`
- Create: `internal/api/diagram/version.go`
- Create: `internal/api/diagram/image.go`

- [ ] **Step 1: Read the source file**

  ```bash
  wc -l internal/api/content/diagrams.go
  cat internal/api/content/diagrams.go
  ```

- [ ] **Step 2: Create `internal/api/diagram/handler.go`**

  ```go
  // Package diagram provides HTTP handlers for diagram CRUD, versions, and images.
  package diagram

  import (
  	"context"
  	"net/http"

  	"github.com/uigraph/app/internal/cache"
  	"github.com/uigraph/app/internal/diagram"
  	"github.com/uigraph/app/internal/storage"
  )

  // store is the minimal persistence interface this package needs.
  type store interface {
  	CreateDiagram(ctx context.Context, d diagram.Diagram) error
  	GetDiagram(ctx context.Context, id string) (*diagram.Diagram, error)
  	ListDiagrams(ctx context.Context, orgID string, folderID, teamID *string) ([]diagram.Diagram, error)
  	UpdateDiagram(ctx context.Context, d diagram.Diagram) error
  	SoftDeleteDiagram(ctx context.Context, id, deletedBy string) error

  	CreateDiagramVersion(ctx context.Context, v diagram.Version) error
  	GetDiagramVersion(ctx context.Context, id string) (*diagram.Version, error)
  	ListDiagramVersions(ctx context.Context, diagramID string) ([]diagram.Version, error)
  	LatestVersionNumber(ctx context.Context, diagramID string) (int, error)

  	CreateDiagramImage(ctx context.Context, img diagram.Image) error
  	ListDiagramImages(ctx context.Context, diagramID string) ([]diagram.Image, error)
  }

  // Handler serves diagram endpoints.
  type Handler struct {
  	store   store
  	storage storage.Client
  	cache   cache.Client // may be nil
  }

  // New constructs a Handler.
  func New(s store, st storage.Client, c cache.Client) *Handler {
  	return &Handler{store: s, storage: st, cache: c}
  }

  // Register wires diagram routes into mux.
  func Register(
  	mux *http.ServeMux,
  	s store,
  	st storage.Client,
  	c cache.Client,
  	requireScope func(scope, method, pattern string, h http.HandlerFunc),
  ) {
  	h := New(s, st, c)
  	requireScope("diagrams:read", "GET", "/api/v1/orgs/{orgID}/diagrams", h.List)
  	requireScope("diagrams:write", "POST", "/api/v1/orgs/{orgID}/diagrams", h.Create)
  	requireScope("diagrams:write", "POST", "/api/v1/orgs/{orgID}/diagrams/sync", h.Sync)
  	requireScope("diagrams:read", "GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}", h.Get)
  	requireScope("diagrams:write", "PUT", "/api/v1/orgs/{orgID}/diagrams/{diagramID}", h.Update)
  	requireScope("diagrams:write", "DELETE", "/api/v1/orgs/{orgID}/diagrams/{diagramID}", h.Delete)
  	requireScope("diagrams:write", "POST", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/thumbnail", h.UpdateThumbnail)
  	requireScope("diagrams:read", "GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/content", h.GetContent)
  	requireScope("diagrams:read", "GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/images", h.ListImages)
  	requireScope("diagrams:write", "POST", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/images", h.CreateImage)
  	requireScope("diagrams:read", "GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/versions", h.ListVersions)
  	requireScope("diagrams:write", "POST", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/versions", h.CreateVersion)
  	requireScope("diagrams:read", "GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/versions/{versionID}/content", h.GetVersionContent)
  	requireScope("diagrams:write", "POST", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/versions/{versionID}/restore", h.RestoreVersion)
  }
  ```

- [ ] **Step 3: Create `internal/api/diagram/diagram.go`**

  Copy `List`, `Create`, `Sync`, `Get`, `Update`, `Delete`, `UpdateThumbnail`, `GetContent` handler methods from `content/diagrams.go` verbatim. Update:
  - `package content` → `package diagram`
  - Remove `"github.com/uigraph/app/internal/store"` import (use local `store` interface)
  - `writeJSON(` → `httputil.JSON(`
  - `writeErr(w, http.StatusBadRequest, msg)` → `httputil.BadRequest(w, msg)`
  - `writeErr(w, http.StatusUnauthorized, ...)` → `httputil.Unauthorized(w)`
  - `writeErr(w, http.StatusNotFound, ...)` → `httputil.Error(w, r, store.ErrNotFound)` (import `store` for the sentinel only)
  - `writeErr(w, http.StatusInternalServerError, ...)` → `httputil.Error(w, r, err)`
  - `streamObject(w, r, h.storage, key)` → `httputil.StreamObject(w, r, h.storage.Download, key)`

- [ ] **Step 4: Create `internal/api/diagram/version.go`**

  Copy `ListVersions`, `CreateVersion`, `GetVersionContent`, `RestoreVersion` from `content/diagrams.go`. Apply the same import substitutions.

- [ ] **Step 5: Create `internal/api/diagram/image.go`**

  Copy `ListImages`, `CreateImage` from `content/diagrams.go`. Apply the same import substitutions.

- [ ] **Step 6: Verify build**

  ```bash
  go build ./internal/api/diagram/...
  ```

  Expected: no output.

- [ ] **Step 7: Commit**

  ```bash
  git add internal/api/diagram/
  git commit -m "feat(api/diagram): new diagram handler package with narrow store interface"
  ```

---

### Task 10: Create `internal/api/maps/`

Source files: `content/maps.go`, `content/frames.go`, `content/frame_extras.go`.

**Files:**
- Create: `internal/api/maps/handler.go`
- Create: `internal/api/maps/map.go`
- Create: `internal/api/maps/frame.go`
- Create: `internal/api/maps/focalpoint.go`
- Create: `internal/api/maps/canvas.go`
- Create: `internal/api/maps/group.go`
- Create: `internal/api/maps/link.go`

- [ ] **Step 1: Read the source files**

  ```bash
  cat internal/api/content/maps.go
  cat internal/api/content/frames.go
  cat internal/api/content/frame_extras.go
  ```

- [ ] **Step 2: Create `internal/api/maps/handler.go`**

  ```go
  // Package map_ provides HTTP handlers for maps, frames, focal points, and canvas.
  // Directory: internal/api/maps/
  package maps

  import (
  	"context"
  	"net/http"

  	"github.com/uigraph/app/internal/storage"
  	"github.com/uigraph/app/internal/uimap"
  )

  // store is the minimal persistence interface this package needs.
  type store interface {
  	CreateMap(ctx context.Context, m uimap.Map) error
  	GetMap(ctx context.Context, id string) (*uimap.Map, error)
  	ListMaps(ctx context.Context, orgID string, folderID, teamID *string) ([]uimap.Map, error)
  	UpdateMap(ctx context.Context, m uimap.Map) error
  	SoftDeleteMap(ctx context.Context, id, deletedBy string) error

  	CreateFrame(ctx context.Context, f uimap.Frame) error
  	GetFrame(ctx context.Context, id string) (*uimap.Frame, error)
  	ListFrames(ctx context.Context, mapID string) ([]uimap.Frame, error)
  	UpdateFrame(ctx context.Context, f uimap.Frame) error
  	SoftDeleteFrame(ctx context.Context, id, deletedBy string) error

  	CreateFocalPoint(ctx context.Context, fp uimap.FocalPoint) error
  	GetFocalPoint(ctx context.Context, id string) (*uimap.FocalPoint, error)
  	ListFocalPoints(ctx context.Context, frameID string) ([]uimap.FocalPoint, error)
  	UpdateFocalPoint(ctx context.Context, fp uimap.FocalPoint) error
  	SoftDeleteFocalPoint(ctx context.Context, id, deletedBy string) error

  	CreateFrameGroup(ctx context.Context, g uimap.FrameGroup) error
  	GetFrameGroup(ctx context.Context, id string) (*uimap.FrameGroup, error)
  	ListFrameGroups(ctx context.Context, frameID string) ([]uimap.FrameGroup, error)
  	UpdateFrameGroup(ctx context.Context, g uimap.FrameGroup) error
  	SoftDeleteFrameGroup(ctx context.Context, id, deletedBy string) error

  	CreateFrameLink(ctx context.Context, l uimap.FrameLink) error
  	GetFrameLink(ctx context.Context, id string) (*uimap.FrameLink, error)
  	ListFrameLinks(ctx context.Context, frameID string) ([]uimap.FrameLink, error)
  	UpdateFrameLink(ctx context.Context, l uimap.FrameLink) error
  	SoftDeleteFrameLink(ctx context.Context, id, deletedBy string) error

  	CreateFocalPointMeta(ctx context.Context, m uimap.FocalPointMeta) error
  	GetFocalPointMeta(ctx context.Context, id string) (*uimap.FocalPointMeta, error)
  	ListFocalPointMeta(ctx context.Context, focalPointID string) ([]uimap.FocalPointMeta, error)
  	UpdateFocalPointMeta(ctx context.Context, m uimap.FocalPointMeta) error
  	SoftDeleteFocalPointMeta(ctx context.Context, id, deletedBy string) error

  	GetCanvas(ctx context.Context, mapID string) (*uimap.Canvas, error)
  	UpsertCanvas(ctx context.Context, c uimap.Canvas) error
  }

  // Handler serves map, frame, focal point, canvas, group, and link endpoints.
  type Handler struct {
  	store   store
  	storage storage.Client
  }

  // New constructs a Handler.
  func New(s store, st storage.Client) *Handler {
  	return &Handler{store: s, storage: st}
  }

  // Register wires all map-domain routes into mux.
  func Register(
  	mux *http.ServeMux,
  	s store,
  	st storage.Client,
  	requireScope func(scope, method, pattern string, h http.HandlerFunc),
  ) {
  	h := New(s, st)

  	// Maps
  	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps", h.ListMaps)
  	requireScope("maps:write", "POST", "/api/v1/orgs/{orgID}/maps", h.CreateMap)
  	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps/{mapID}", h.GetMap)
  	requireScope("maps:write", "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}", h.UpdateMap)
  	requireScope("maps:write", "DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}", h.DeleteMap)

  	// Frames
  	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames", h.ListFrames)
  	requireScope("maps:write", "POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames", h.CreateFrame)
  	requireScope("maps:write", "POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/sync", h.SyncFrames)
  	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/frames/{frameID}", h.GetFrame)
  	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}", h.GetFrame)
  	requireScope("maps:write", "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}", h.UpdateFrame)
  	requireScope("maps:write", "DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}", h.DeleteFrame)

  	// Focal Points
  	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points", h.ListFocalPoints)
  	requireScope("maps:write", "POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points", h.CreateFocalPoint)
  	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}", h.GetFocalPoint)
  	requireScope("maps:write", "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}", h.UpdateFocalPoint)
  	requireScope("maps:write", "DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}", h.DeleteFocalPoint)

  	// Focal Point Meta
  	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta", h.ListMeta)
  	requireScope("maps:write", "POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta", h.CreateMeta)
  	requireScope("maps:write", "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta/{metaID}", h.UpdateMeta)
  	requireScope("maps:write", "DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta/{metaID}", h.DeleteMeta)

  	// Canvas
  	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/canvas", h.GetCanvas)
  	requireScope("maps:write", "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/canvas", h.UpsertCanvas)

  	// Groups
  	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups", h.ListGroups)
  	requireScope("maps:write", "POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups", h.CreateGroup)
  	requireScope("maps:write", "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups/{groupID}", h.UpdateGroup)
  	requireScope("maps:write", "DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups/{groupID}", h.DeleteGroup)

  	// Links
  	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/links", h.ListLinks)
  	requireScope("maps:write", "POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/links", h.CreateLink)
  	requireScope("maps:write", "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/links/{linkID}", h.UpdateLink)
  	requireScope("maps:write", "DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/links/{linkID}", h.DeleteLink)
  }
  ```

  > **Note on package name:** Directory is `internal/api/maps/`, package declaration is `package maps`. Import in `router.go` as `mapsapi "github.com/uigraph/app/internal/api/maps"`.

- [ ] **Step 3: Create `internal/api/maps/map.go`**

  Copy `List`, `Create`, `Get`, `Update`, `Delete` map handler methods from `content/maps.go`. Rename receiver from `h *MapHandler` to `h *Handler`. Apply httputil substitutions.

- [ ] **Step 4: Create `internal/api/maps/frame.go`**

  Copy all frame handler methods from `content/frames.go` (List, Create, Sync, Get, Update, Delete). Apply httputil substitutions. Keep `streamObject` calls using `httputil.StreamObject`.

- [ ] **Step 5: Create `internal/api/maps/focalpoint.go`**

  Copy focal point methods from `content/frames.go`. Apply httputil substitutions.

- [ ] **Step 6: Create `internal/api/maps/canvas.go`**

  Copy `GetCanvas`, `UpsertCanvas` from `content/frames.go`. Apply httputil substitutions.

- [ ] **Step 7: Create `internal/api/maps/group.go`**

  Copy group methods from `content/frame_extras.go`. Apply httputil substitutions.

- [ ] **Step 8: Create `internal/api/maps/link.go`**

  Copy link methods from `content/frame_extras.go`. Apply httputil substitutions.

- [ ] **Step 9: Create `internal/api/maps/meta.go`**

  Copy focal point meta methods from `content/frame_extras.go`. Apply httputil substitutions.

- [ ] **Step 10: Verify build**

  ```bash
  go build ./internal/api/maps/...
  ```

  Expected: no output.

- [ ] **Step 11: Commit**

  ```bash
  git add internal/api/maps/
  git commit -m "feat(api/map): new map/frame/focal-point handler package with narrow store interface"
  ```

---

### Task 11: Create `internal/api/catalog/`

Largest task. Source: `content/services.go` (1,217 lines), `content/service_dbs.go` (345 lines), `content/service_tests.go` (598 lines). Complex multi-step logic (`SyncAPIGroup`, `CreateServiceDiagram`) is extracted into `service_logic.go`.

**Files:**
- Create: `internal/api/catalog/handler.go`
- Create: `internal/api/catalog/service.go`
- Create: `internal/api/catalog/service_logic.go`
- Create: `internal/api/catalog/apigroup.go`
- Create: `internal/api/catalog/endpoint.go`
- Create: `internal/api/catalog/doc.go`
- Create: `internal/api/catalog/db.go`
- Create: `internal/api/catalog/testpack.go`

- [ ] **Step 1: Read all three source files**

  ```bash
  cat internal/api/content/services.go
  cat internal/api/content/service_dbs.go
  cat internal/api/content/service_tests.go
  ```

- [ ] **Step 2: Create `internal/api/catalog/handler.go`**

  ```go
  // Package catalog provides HTTP handlers for the service catalog.
  package catalog

  import (
  	"context"
  	"net/http"

  	"github.com/uigraph/app/internal/catalog"
  	"github.com/uigraph/app/internal/diagram"
  	"github.com/uigraph/app/internal/storage"
  )

  // store is the minimal persistence interface this package needs.
  type store interface {
  	// Services
  	CreateService(ctx context.Context, s catalog.Service) error
  	GetService(ctx context.Context, id string) (*catalog.Service, error)
  	ListServices(ctx context.Context, orgID string, folderID, teamID *string) ([]catalog.Service, error)
  	UpdateService(ctx context.Context, s catalog.Service) error
  	SoftDeleteService(ctx context.Context, id, deletedBy string) error
  	ListServiceStats(ctx context.Context, orgID string, serviceID *string) ([]catalog.ServiceStats, error)

  	// API Groups
  	CreateAPIGroup(ctx context.Context, g catalog.APIGroup) error
  	GetAPIGroup(ctx context.Context, id string) (*catalog.APIGroup, error)
  	ListAPIGroups(ctx context.Context, serviceID string) ([]catalog.APIGroup, error)
  	UpdateAPIGroup(ctx context.Context, g catalog.APIGroup) error
  	SoftDeleteAPIGroup(ctx context.Context, id, deletedBy string) error
  	CreateAPIGroupVersion(ctx context.Context, v catalog.APIGroupVersion) error
  	ListAPIGroupVersions(ctx context.Context, apiGroupID string) ([]catalog.APIGroupVersion, error)
  	LatestAPIGroupVersionNumber(ctx context.Context, apiGroupID string) (int, error)

  	// API Endpoints
  	CreateAPIEndpoint(ctx context.Context, e catalog.APIEndpoint) error
  	GetAPIEndpoint(ctx context.Context, id string) (*catalog.APIEndpoint, error)
  	ListAPIEndpoints(ctx context.Context, apiGroupID string) ([]catalog.APIEndpoint, error)
  	UpdateAPIEndpoint(ctx context.Context, e catalog.APIEndpoint) error
  	SoftDeleteAPIEndpoint(ctx context.Context, id, deletedBy string) error

  	// Service Docs
  	CreateServiceDoc(ctx context.Context, d catalog.ServiceDoc) error
  	GetServiceDoc(ctx context.Context, id string) (*catalog.ServiceDoc, error)
  	ListServiceDocs(ctx context.Context, serviceID string) ([]catalog.ServiceDoc, error)
  	UpdateServiceDoc(ctx context.Context, d catalog.ServiceDoc) error
  	SoftDeleteServiceDoc(ctx context.Context, id string) error

  	// Service Diagrams
  	CreateServiceDiagram(ctx context.Context, sd catalog.ServiceDiagram) error
  	GetServiceDiagram(ctx context.Context, serviceID, diagramID string) (*catalog.ServiceDiagram, error)
  	ListServiceDiagrams(ctx context.Context, serviceID string) ([]catalog.ServiceDiagram, error)
  	SoftDeleteServiceDiagram(ctx context.Context, serviceID, diagramID, deletedBy string) error

  	// Diagram (needed for service-diagram link creation)
  	CreateDiagram(ctx context.Context, d diagram.Diagram) error
  	GetDiagram(ctx context.Context, id string) (*diagram.Diagram, error)
  	CreateDiagramVersion(ctx context.Context, v diagram.Version) error
  	LatestVersionNumber(ctx context.Context, diagramID string) (int, error)

  	// Service DBs
  	CreateServiceDB(ctx context.Context, db catalog.ServiceDB) error
  	GetServiceDB(ctx context.Context, id string) (*catalog.ServiceDB, error)
  	ListServiceDBs(ctx context.Context, serviceID string) ([]catalog.ServiceDB, error)
  	UpdateServiceDB(ctx context.Context, db catalog.ServiceDB) error
  	SoftDeleteServiceDB(ctx context.Context, id, deletedBy string) error
  	CreateServiceDBVersion(ctx context.Context, v catalog.ServiceDBVersion) error
  	ListServiceDBVersions(ctx context.Context, dbID string) ([]catalog.ServiceDBVersion, error)
  	LatestServiceDBVersionNumber(ctx context.Context, dbID string) (int, error)
  	RestoreServiceDBVersion(ctx context.Context, dbID, versionID, restoredBy string) error

  	// Test Packs / Cases / Runs
  	CreateTestPack(ctx context.Context, tp catalog.TestPack) error
  	GetTestPack(ctx context.Context, id string) (*catalog.TestPack, error)
  	ListTestPacks(ctx context.Context, serviceID string) ([]catalog.TestPack, error)
  	UpdateTestPack(ctx context.Context, tp catalog.TestPack) error
  	SoftDeleteTestPack(ctx context.Context, id, deletedBy string) error
  	CreateTestCase(ctx context.Context, tc catalog.TestCase) error
  	GetTestCase(ctx context.Context, id string) (*catalog.TestCase, error)
  	ListTestCases(ctx context.Context, serviceID string) ([]catalog.TestCase, error)
  	UpdateTestCase(ctx context.Context, tc catalog.TestCase) error
  	SoftDeleteTestCase(ctx context.Context, id, deletedBy string) error
  	CreateTestRun(ctx context.Context, tr catalog.TestRun) error
  	GetTestRun(ctx context.Context, id string) (*catalog.TestRun, error)
  	ListTestRuns(ctx context.Context, serviceID string) ([]catalog.TestRun, error)
  	ListTestRunsSummary(ctx context.Context, serviceID string) ([]catalog.TestRunSummary, error)
  	UpdateTestRun(ctx context.Context, tr catalog.TestRun) error
  	CreateTestRunResult(ctx context.Context, r catalog.TestRunResult) error
  	GetTestRunResult(ctx context.Context, id string) (*catalog.TestRunResult, error)
  	ListTestRunResults(ctx context.Context, serviceID string) ([]catalog.TestRunResult, error)
  	UpdateTestRunResult(ctx context.Context, r catalog.TestRunResult) error
  }

  // Handler serves service catalog endpoints.
  type Handler struct {
  	store   store
  	storage storage.Client // may be nil
  }

  // New constructs a Handler.
  func New(s store, st storage.Client) *Handler {
  	return &Handler{store: s, storage: st}
  }

  // Register wires all catalog routes into mux.
  func Register(
  	mux *http.ServeMux,
  	s store,
  	st storage.Client,
  	requireScope func(scope, method, pattern string, h http.HandlerFunc),
  ) {
  	h := New(s, st)

  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services", h.List)
  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/stats", h.ListStats)
  	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services", h.Create)
  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}", h.Get)
  	requireScope("services:write", "PUT", "/api/v1/orgs/{orgID}/services/{serviceID}", h.Update)
  	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}", h.Delete)

  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups", h.ListAPIGroups)
  	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups", h.CreateAPIGroup)
  	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/sync", h.SyncAPIGroup)
  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}", h.GetAPIGroup)
  	requireScope("services:write", "PUT", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}", h.UpdateAPIGroup)
  	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}", h.DeleteAPIGroup)
  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/versions", h.ListAPIGroupVersions)

  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints", h.ListAPIEndpoints)
  	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints", h.CreateAPIEndpoint)
  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints/{endpointID}", h.GetAPIEndpoint)
  	requireScope("services:write", "PUT", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints/{endpointID}", h.UpdateAPIEndpoint)
  	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints/{endpointID}", h.DeleteAPIEndpoint)

  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/docs", h.ListDocs)
  	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/docs", h.CreateDoc)
  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/docs/{docID}", h.GetDoc)
  	requireScope("services:write", "PUT", "/api/v1/orgs/{orgID}/services/{serviceID}/docs/{docID}", h.UpdateDoc)
  	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/docs/{docID}", h.DeleteDoc)

  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/diagrams", h.ListDiagrams)
  	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/diagrams", h.CreateDiagram)
  	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/diagrams/{diagramID}", h.DeleteDiagram)

  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs", h.ListDBs)
  	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs", h.CreateDB)
  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}", h.GetDB)
  	requireScope("services:write", "PUT", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}", h.UpdateDB)
  	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}", h.DeleteDB)
  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}/versions", h.ListDBVersions)
  	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}/versions", h.CreateDBVersion)
  	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}/versions/{versionID}/restore", h.RestoreDBVersion)

  	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-pack", h.CreateTestPack)
  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/test-packs", h.ListTestPacks)
  	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-pack/{testPackID}", h.UpdateTestPack)
  	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/test-pack/{testPackID}", h.DeleteTestPack)
  	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-case", h.CreateTestCase)
  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/test-cases", h.ListTestCases)
  	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-case/{testCaseID}", h.UpdateTestCase)
  	requireScope("services:write", "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/test-case/{testCaseID}", h.DeleteTestCase)
  	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-run", h.CreateTestRun)
  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/test-runs", h.ListTestRuns)
  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/test-runs-summary", h.ListTestRunsSummary)
  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/test-run/{testRunID}", h.GetTestRun)
  	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-run/{testRunID}", h.UpdateTestRun)
  	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-run-result", h.CreateTestRunResult)
  	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/test-run-results", h.ListTestRunResults)
  	requireScope("services:write", "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-run-result/{testRunResultID}", h.UpdateTestRunResult)
  }
  ```

- [ ] **Step 3: Create `internal/api/catalog/service_logic.go`**

  Extract the three functions that are too complex for a handler method body. Copy from `content/services.go`:

  ```go
  package catalog

  // toSlug, specHash, sha256Bytes, ensureServiceInOrg,
  // readServiceDocPayload, readOptionalServiceDocPayload,
  // readServiceDocFromJSON, readServiceDocFromMultipart,
  // uploadSpec — copy verbatim from content/services.go.
  // Change package declaration only.
  ```

  Exact functions to include (copy verbatim, update package only):
  - `toSlug(name string) string`
  - `specHash(s string) string`
  - `sha256Bytes(b []byte) string`
  - `(h *Handler) ensureServiceInOrg(...) bool`
  - `(h *Handler) uploadSpec(ctx, key, content) error`
  - `(h *Handler) readServiceDocPayload(r) (...)`
  - `(h *Handler) readOptionalServiceDocPayload(r, existing) (...)`
  - `readServiceDocFromJSON(r, existing) (...)`
  - `readServiceDocFromMultipart(r, existing) (...)`

- [ ] **Step 4: Create `internal/api/catalog/service.go`**

  Copy service CRUD handler methods from `content/services.go`: `List`, `ListStats`, `Create`, `Get`, `Update`, `Delete`. Apply httputil substitutions.

- [ ] **Step 5: Create `internal/api/catalog/apigroup.go`**

  Copy `ListAPIGroups`, `CreateAPIGroup`, `GetAPIGroup`, `UpdateAPIGroup`, `DeleteAPIGroup`, `SyncAPIGroup`, `ListAPIGroupVersions` from `content/services.go`. Apply httputil substitutions.

- [ ] **Step 6: Create `internal/api/catalog/endpoint.go`**

  Copy `ListAPIEndpoints`, `CreateAPIEndpoint`, `GetAPIEndpoint`, `UpdateAPIEndpoint`, `DeleteAPIEndpoint` from `content/services.go`. Apply httputil substitutions.

- [ ] **Step 7: Create `internal/api/catalog/doc.go`**

  Copy `ListDocs`, `CreateDoc`, `GetDoc`, `UpdateDoc`, `DeleteDoc` plus the doc diagram methods from `content/services.go`. Apply httputil substitutions.

- [ ] **Step 8: Create `internal/api/catalog/db.go`**

  Copy all methods from `content/service_dbs.go`. Apply httputil substitutions (package `content` → `catalog`).

- [ ] **Step 9: Create `internal/api/catalog/testpack.go`**

  Copy all methods from `content/service_tests.go`. Apply httputil substitutions.

- [ ] **Step 10: Verify build**

  ```bash
  go build ./internal/api/catalog/...
  ```

  Expected: no output.

- [ ] **Step 11: Commit**

  ```bash
  git add internal/api/catalog/
  git commit -m "feat(api/catalog): new service catalog handler package with narrow store interface"
  ```

---

## Phase 3 — Router Decomposition and Cleanup

---

### Task 12: Decompose `router.go` and switch to new handler packages

Update `router.go` to call each new package's `Register()` function. Keep the old `content/` import until the switch is complete and verified.

**Files:**
- Modify: `internal/api/router.go`

- [ ] **Step 1: Read current router.go**

  ```bash
  cat internal/api/router.go
  ```

- [ ] **Step 2: Rewrite `internal/api/router.go`**

  The new `router.go` wires scope constants and calls `Register()` per domain. Key structural changes:
  - The `requireScope` closure stays (it's wiring, not business logic)
  - `serverAdmin` closure stays
  - `protected` closure stays
  - Remove all inline route registrations for content that moved
  - Add one `Register()` call per new package

  ```go
  package api

  import (
  	"net/http"

  	actorapi  "github.com/uigraph/app/internal/api/actor"
  	assetapi  "github.com/uigraph/app/internal/api/asset"
  	"github.com/uigraph/app/internal/api/auth"
  	catalogapi "github.com/uigraph/app/internal/api/catalog"
  	componentapi "github.com/uigraph/app/internal/api/component"
  	diagramapi "github.com/uigraph/app/internal/api/diagram"
  	folderapi  "github.com/uigraph/app/internal/api/folder"
  	"github.com/uigraph/app/internal/api/health"
  	mapsapi    "github.com/uigraph/app/internal/api/maps"
  	authmw     "github.com/uigraph/app/internal/middleware"

  	"github.com/uigraph/app/internal/asset"
  	"github.com/uigraph/app/internal/authz"
  	"github.com/uigraph/app/internal/cache"
  	"github.com/uigraph/app/internal/config"
  	"github.com/uigraph/app/internal/identity"
  	"github.com/uigraph/app/internal/storage"
  	"github.com/uigraph/app/internal/store"
  )

  func New(s store.Store, bearer authmw.BearerVerifier, cfg *config.Config, st storage.Client, c cache.Client) http.Handler {
  	mux := http.NewServeMux()
  	mw := authmw.New(bearer, s)
  	authorizer := authz.New(s, s)

  	// ── Middleware helpers ────────────────────────────────────────────────
  	protected := func(method, pattern string, h http.HandlerFunc) {
  		mux.Handle(method+" "+pattern, mw.Handler(http.HandlerFunc(h)))
  	}

  	requireScope := func(scope authz.Scope, method, pattern string, h http.HandlerFunc) {
  		guarded := func(w http.ResponseWriter, r *http.Request) {
  			p, ok := authmw.PrincipalFromCtx(r.Context())
  			if !ok {
  				http.Error(w, `{"error":"unauthenticated","code":401}`, http.StatusUnauthorized)
  				return
  			}
  			var scopes []string
  			if p.Kind == identity.PrincipalServiceAccount {
  				scopes = p.Scopes
  			} else {
  				resolved, err := authorizer.ScopesForUser(r.Context(), p.UserID, r.PathValue("orgID"))
  				if err != nil {
  					http.Error(w, `{"error":"forbidden","code":403}`, http.StatusForbidden)
  					return
  				}
  				scopes = make([]string, len(resolved))
  				for i, sc := range resolved {
  					scopes[i] = string(sc)
  				}
  			}
  			if !authz.Has(scopes, scope) {
  				http.Error(w, `{"error":"forbidden","code":403}`, http.StatusForbidden)
  				return
  			}
  			h(w, r)
  		}
  		mux.Handle(method+" "+pattern, mw.Handler(http.HandlerFunc(guarded)))
  	}

  	serverAdmin := func(method, pattern string, h http.HandlerFunc) {
  		guarded := func(w http.ResponseWriter, r *http.Request) {
  			p, ok := authmw.PrincipalFromCtx(r.Context())
  			if !ok {
  				http.Error(w, `{"error":"unauthenticated","code":401}`, http.StatusUnauthorized)
  				return
  			}
  			ok, err := authorizer.IsUserServerAdmin(r.Context(), p.UserID)
  			if err != nil || !ok {
  				http.Error(w, `{"error":"forbidden","code":403}`, http.StatusForbidden)
  				return
  			}
  			h(w, r)
  		}
  		mux.Handle(method+" "+pattern, mw.Handler(http.HandlerFunc(guarded)))
  	}

  	// Each domain Register() adapts its scope strings to authz.Scope constants.
  	// Bridge: wrap requireScope so domain packages pass string scope names.
  	scopeFn := func(scopeStr, method, pattern string, h http.HandlerFunc) {
  		requireScope(authz.Scope(scopeStr), method, pattern, h)
  	}

  	// ── Health ────────────────────────────────────────────────────────────
  	healthHandler := &health.Handler{}
  	mux.HandleFunc("GET /healthz", healthHandler.Healthz)
  	mux.HandleFunc("GET /livez", healthHandler.Livez)

  	// ── Auth ──────────────────────────────────────────────────────────────
  	// Copy all auth route registrations verbatim from the current router.go.
  	// These are lines 45–193 of the existing file (session, users, orgs, members,
  	// teams, invitations, service accounts, SSO). Do not change any auth routes.
  	assetResolver := asset.New(st, c)
  	sessionH := auth.NewSessionHandler(s, assetResolver, cfg.PublicURL, cfg.FrontendURL)
  	mux.HandleFunc("POST /api/v1/auth/login", sessionH.Login)
  	// ... paste remaining auth registrations from current router.go lines 47–193 ...

  	// ── Domain packages ───────────────────────────────────────────────────
  	folderapi.Register(mux, s, scopeFn)
  	componentapi.Register(mux, s, st, protected)
  	actorapi.Register(mux, s, c, st, protected)
  	assetapi.Register(mux, st, c, protected)
  	diagramapi.Register(mux, s, st, c, scopeFn)
  	mapsapi.Register(mux, s, st, scopeFn)
  	catalogapi.Register(mux, s, st, scopeFn)

  	return mux
  }
  ```

  > **Important:** the `scopeFn` bridge works because every scope string passed from domain `Register()` calls matches an `authz.Scope` constant exactly (e.g. `"folders:read"` = `authz.ScopeFoldersRead`). Verify this by cross-referencing `internal/authz/scope.go` before running. If scope string names differ, update them in the `Register()` calls to match the actual constant values.

- [ ] **Step 3: Verify build**

  ```bash
  go build ./...
  ```

  Expected: no output.

- [ ] **Step 4: Run integration tests**

  ```bash
  go test ./tests/... -v -count=1
  ```

  Expected: all pass (same results as before this task).

- [ ] **Step 5: Commit**

  ```bash
  git add internal/api/router.go
  git commit -m "refactor(api): decompose router.go into per-domain Register() calls"
  ```

---

### Task 13: Delete `internal/api/content/`

Now that all routes are served by the new domain packages, remove the old god package.

**Files:**
- Delete: `internal/api/content/` (all files)

- [ ] **Step 1: Verify nothing imports content/ any more**

  ```bash
  grep -r '"github.com/uigraph/app/internal/api/content"' .
  ```

  Expected: no output. If any file still imports it, fix the import first.

- [ ] **Step 2: Delete the directory**

  ```bash
  rm -rf internal/api/content/
  ```

- [ ] **Step 3: Verify build**

  ```bash
  go build ./...
  ```

  Expected: no output.

- [ ] **Step 4: Run integration tests**

  ```bash
  go test ./tests/... -v -count=1
  ```

  Expected: all pass.

- [ ] **Step 5: Commit**

  ```bash
  git add -A
  git commit -m "refactor: delete internal/api/content/ god package — replaced by domain-aligned handler packages"
  ```

---

### Task 14: Verify scope string alignment

The `scopeFn` bridge in `router.go` passes string scope names (e.g. `"folders:read"`) to `authz.Has()` which expects `authz.Scope` constants. Verify every scope string passed from `Register()` functions matches the actual constant values.

**Files:**
- Read: `internal/authz/scope.go`
- Possibly modify: each `Register()` function if scope strings don't match

- [ ] **Step 1: Read scope constants**

  ```bash
  cat internal/authz/scope.go
  ```

- [ ] **Step 2: Cross-reference**

  For each scope string used in a `Register()` function (e.g. `"folders:read"`, `"diagrams:write"`), verify it matches the corresponding `authz.Scope` constant value. Make a list of any mismatches.

- [ ] **Step 3: Fix any mismatches**

  If `authz.ScopeFoldersRead = "folders:read"` then no change needed. If the constant value is different (e.g. `"org:folders:read"`), update the string in the `Register()` call to match.

- [ ] **Step 4: Verify build and tests**

  ```bash
  go build ./... && go test ./tests/... -v -count=1
  ```

  Expected: all pass.

- [ ] **Step 5: Commit if any fixes were needed**

  ```bash
  git add -A
  git commit -m "fix(api): align scope strings in Register() calls with authz.Scope constants"
  ```

---

## Success Criteria

All of these must be true before this plan is considered complete:

- [ ] `internal/api/content/` does not exist
- [ ] `internal/seed/` does not exist
- [ ] `internal/componentcatalog/` does not exist (renamed to `componentlib/`)
- [ ] No handler file exceeds 400 lines (`find internal/api -name '*.go' | xargs wc -l | sort -rn | head -20`)
- [ ] Every handler struct holds a narrow `store` interface, not `store.Store`
- [ ] `httputil.StreamObject` is the only place object streaming is implemented
- [ ] `go build ./...` passes with zero errors
- [ ] `go test ./tests/... -count=1` passes (same results as before the refactor)
- [ ] `go vet ./...` passes with zero warnings
