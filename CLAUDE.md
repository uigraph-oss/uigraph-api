# uigraph-api — Claude Code Guidelines

Go module: `github.com/uigraph/app` · Go 1.25 · open-source project

---

## Project layout

```
internal/
  api/          HTTP handlers — one sub-package per domain
  catalog/      domain structs: Service, APIGroup, APIEndpoint, ServiceDB, …
  diagram/      domain structs: Diagram, Version, Image
  uimap/        domain structs: Map, Frame, FocalPoint, Canvas, Group, Link
  folder/       domain structs: Folder
  httputil/     shared HTTP helpers (single source of truth)
  store/        aggregate Store interface (wiring only)
  storage/      object-storage Client interface + MinIO impl
  cache/        cache Client interface + Redis impl
  middleware/   auth middleware + PrincipalFromCtx
  authz/        scopes, roles, authorizer
```

---

## Handler package pattern

Every domain under `internal/api/<domain>/` follows the same structure:

**`handler.go`** — interfaces, Handler struct, New(), Register()

```go
package diagram

import (
    "context"
    "io"
    "net/http"

    diagrampkg "github.com/uigraph/app/internal/diagram"
)

// store is the minimal persistence interface this package needs.
// postgres.DB satisfies it automatically — no changes needed there.
type store interface {
    GetDiagram(ctx context.Context, id string) (*diagrampkg.Diagram, error)
    CreateDiagram(ctx context.Context, d diagrampkg.Diagram) error
    // ... only what this package actually calls
}

// objectStore is the minimal storage interface this package needs.
type objectStore interface {
    Upload(ctx context.Context, key, contentType string, body io.Reader, size int64) error
    Download(ctx context.Context, key string) (io.ReadCloser, error)
}

type Handler struct {
    store   store
    storage objectStore
}

func New(s store, st objectStore) *Handler {
    return &Handler{store: s, storage: st}
}

// Register wires routes. requireScope converts a plain scope string to an
// authenticated, scope-checked handler registered on mux.
func Register(
    mux *http.ServeMux,
    s store,
    st objectStore,
    requireScope func(scope, method, pattern string, h http.HandlerFunc),
) {
    h := New(s, st)
    requireScope("diagrams:read", "GET", "/api/v1/orgs/{orgID}/diagrams", h.List)
    requireScope("diagrams:write", "POST", "/api/v1/orgs/{orgID}/diagrams", h.Create)
    // ...
}
```

Rules:
- **Narrow interfaces only.** Declare only the methods a package actually calls — never accept `store.Store`.
- **Register() lives in handler.go.** All route wiring for a domain is in one place.
- **scope strings are plain strings** matching `authz.Scope` constants exactly (`"diagrams:read"`, `"services:write"`, etc.). Check `internal/authz/scope.go` for the full list.
- **Package name collisions** — when the handler package name (`diagram`) collides with the domain package import path (`internal/diagram`), alias the domain import: `diagrampkg "github.com/uigraph/app/internal/diagram"`. Same pattern for `catalogpkg`, `storepkg`, `actorpkg`, `assetpkg`.

---

## HTTP response helpers

**Always use `httputil`** (`internal/httputil/respond.go`). Never use raw `http.Error`, `json.NewEncoder`, or any local `writeErr`/`writeJSON` helpers.

| Situation | Call |
|-----------|------|
| Success with body | `httputil.JSON(w, http.StatusOK, body)` |
| Created with body | `httputil.JSON(w, http.StatusCreated, body)` |
| No body (204) | `w.WriteHeader(http.StatusNoContent)` |
| Store/internal error | `httputil.Error(w, r, err)` |
| Not found (nil pointer) | `httputil.Error(w, r, storepkg.ErrNotFound)` |
| Bad request | `httputil.BadRequest(w, "message")` |
| Unauthenticated | `httputil.Unauthorized(w)` |
| Forbidden | `httputil.Forbidden(w)` |

`httputil.Error` auto-handles:
- `store.ErrNotFound` → 404
- `store.ErrConflict` → 409
- anything else → 500 + `slog.ErrorContext` log

---

## Error handling — the most important rule

**ALWAYS split `err != nil` from the nil-pointer check into two separate `if` blocks.**

```go
// CORRECT
svc, err := h.store.GetService(r.Context(), id)
if err != nil {
    httputil.Error(w, r, err)
    return
}
if svc == nil || svc.DeletedAt != nil {
    httputil.Error(w, r, storepkg.ErrNotFound)
    return
}

// WRONG — never collapse them
if err != nil || svc == nil || svc.DeletedAt != nil {
    httputil.Error(w, r, storepkg.ErrNotFound) // masks real store errors as 404
    return
}
```

Collapsing the two checks turns a real store failure (database down, network error) into a silent 404. Keep them separate.

---

## Auth in handlers

Get the authenticated principal from context:

```go
p, ok := authmw.PrincipalFromCtx(r.Context())
if !ok {
    httputil.Unauthorized(w)
    return
}
// use p.UserID, p.Kind, p.Scopes
```

Import: `authmw "github.com/uigraph/app/internal/middleware"`

Scope enforcement is done at the router level via `requireScope` — handlers themselves do not re-check scopes. Handlers only check `PrincipalFromCtx` when they need the actor's identity (e.g., to set `CreatedBy`).

---

## Comments

Default: **no comments.** Well-named identifiers are self-documenting.

Add a comment only when the **why** is non-obvious: a hidden constraint, a subtle invariant, a workaround for a specific bug, or behavior that would surprise a reader.

Never:
- Describe what the code does (`// set UpdatedAt to now`)
- Reference the current task or caller (`// used by SyncAPIGroup`)
- Write multi-line block comments on simple functions

Handler method comments are one line, format: `// List handles GET /api/v1/orgs/{orgID}/diagrams`

---

## File size

- Handler files: target under 300 lines. If a handler file grows past this, extract complex multi-step logic into a `_logic.go` file in the same package.
- Private helpers (hashing, slug generation, payload parsing) live in `service_logic.go` or similar — not inline in handler methods.

---

## Struct mutation pattern (Update handlers)

Fetch → check nil → decode body → apply non-nil fields → save → respond:

```go
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
        Name *string `json:"name"`
    }
    if err := httputil.Decode(r, &body); err != nil {
        httputil.BadRequest(w, "invalid request body")
        return
    }
    if body.Name != nil {
        existing.Name = *body.Name
    }
    if err := h.store.UpdateFolder(r.Context(), *existing); err != nil {
        httputil.Error(w, r, err)
        return
    }
    existing.UpdatedAt = time.Now().UTC() // set after save so response reflects actual time
    httputil.JSON(w, http.StatusOK, existing)
}
```

Always set `UpdatedAt` on the in-memory struct **after** a successful save, so the response body is accurate.

---

## New domain handler checklist

When adding a new domain:

1. Create `internal/api/<domain>/handler.go` with narrow `store` interface, `Handler` struct, `New()`, `Register()`
2. Create handler files (one per subdomain if large)
3. Scope strings in `Register()` must match constants in `internal/authz/scope.go`
4. Add `<domain>.Register(mux, s, ...)` call in `internal/api/router.go` using the `scopeFn` bridge
5. `go build ./...` must pass before committing

---

## Forbidden patterns

- `store.Store` in handler structs — use a narrow interface
- `writeErr` / `writeJSON` — use `httputil`
- Collapsed `if err != nil || x == nil` — always two blocks
- Comments that describe what, not why
- Business logic inside handler methods — extract to named helpers
- Skipping `go build ./...` before committing
