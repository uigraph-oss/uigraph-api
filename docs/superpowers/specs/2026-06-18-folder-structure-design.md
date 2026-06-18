# uigraph-api: Production-Ready Folder Structure

**Date:** 2026-06-18
**Status:** Approved — pending implementation
**Goal:** Clean, testable, open-source-ready folder and file structure for the Go API server.

---

## Context

The codebase has a solid domain model layer but accumulated two structural problems as features grew:

1. `internal/api/content/` became a god package — 13+ files, mixed domains, with `services.go` at 1,200+ lines.
2. Every handler receives the aggregate `store.Store` (50+ methods), making unit tests impractical — each test must mock the entire store.

Additionally, the `httputil/` and `content/helpers.go` packages duplicate HTTP response helpers, and `internal/seed/` and `internal/bootstrap/` serve the same startup-seeding concern split across two packages.

This document defines the target structure. No logic changes are in scope — this is a reorganisation and file-split exercise.

---

## Design Principles

1. **One package, one clear purpose.** A contributor should be able to answer "what does this package do?" in one sentence.
2. **Accept narrow interfaces.** Handlers declare the minimal store interface they need — not the aggregate `store.Store`.
3. **Handlers are HTTP adapters only.** Parse → call logic/store → respond. Nothing else.
4. **Complex multi-step logic is named and extracted.** If an operation is too complex for one handler method, it lives in a `service.go` file in the same handler package — not inline.
5. **Errors are logged before swallowing.** Every `500` path calls `slog.ErrorContext` with the real error.
6. **No duplication.** One place for HTTP helpers, one place for startup seeding.

---

## Target Directory Tree

```
uigraph-api/
├── cmd/
│   └── api/
│       └── main.go                    # entry point — unchanged
│
├── assets/                            # embedded SVGs — unchanged
│   └── component-icons/
│
├── migrations/                        # SQL migrations — unchanged
│
├── docs/                              # project documentation
│   └── superpowers/specs/             # design specs
│
├── tests/                             # integration tests — unchanged
│
└── internal/
    │
    │   ── Domain layer ────────────────────────────────────────────────────
    │   Each package owns its structs and the store interface for that domain.
    │   No HTTP, no Postgres, no business logic beyond pure helper functions.
    │
    ├── catalog/
    │   ├── catalog.go                 # structs: Service, APIGroup, APIEndpoint,
    │   │                              #   APIGroupVersion, ServiceDoc, ServiceDB,
    │   │                              #   ServiceDBVersion, ServiceDiagram,
    │   │                              #   ServiceTestPack, ServiceTestCase,
    │   │                              #   ServiceTestRun, ServiceTestRunResult,
    │   │                              #   ServiceStats
    │   └── store.go                   # CatalogStore interface
    │                                  # also: toSlug(), specHash() pure helpers
    │
    ├── diagram/
    │   ├── diagram.go                 # structs: Diagram, Version, Image
    │   └── store.go                   # DiagramStore interface
    │
    ├── uimap/
    │   ├── uimap.go                   # structs: Map, Frame, FocalPoint, Canvas,
    │   │                              #   Group, Link, FrameMeta
    │   └── store.go                   # MapStore interface
    │
    ├── folder/
    │   ├── folder.go                  # structs: Folder
    │   └── store.go                   # FolderStore interface
    │
    ├── org/
    │   ├── org.go                     # structs: Org
    │   ├── user.go                    # structs: User
    │   ├── member.go                  # structs: Member
    │   ├── team.go                    # structs: Team
    │   ├── invitation.go              # structs: Invitation
    │   └── store.go                   # OrgStore, UserStore, MemberStore,
    │                                  #   TeamStore, InvitationStore interfaces
    │
    ├── identity/
    │   ├── principal.go               # structs: Principal, PrincipalKind
    │   ├── session.go                 # structs: Session
    │   ├── token.go                   # structs: Token
    │   ├── serviceaccount.go          # structs: ServiceAccount
    │   └── store.go                   # SessionStore, ProviderStore,
    │                                  #   ServiceAccountStore interfaces
    │
    ├── authz/
    │   ├── scope.go                   # Scope constants + Has()
    │   ├── role.go                    # Role definitions
    │   ├── resource.go                # Resource types
    │   ├── authorizer.go              # Authorizer: ScopesForUser, IsUserServerAdmin
    │   ├── sso.go                     # SSO role-mapping logic
    │   └── store.go                   # RBACStore interface
    │                                  # (rename from rbac_store.go)
    │
    ├── componentlib/                  # RENAME from componentcatalog/
    │   ├── catalog.go                 # ComponentCatalog type
    │   ├── convert.go                 # conversion helpers
    │   ├── loader.go                  # loads JSON files into catalog
    │   ├── flow_diagram_components.json
    │   └── focal_point_components.json
    │
    │   ── Infrastructure layer ────────────────────────────────────────────
    │   External systems: Postgres, object storage, Redis, OAuth.
    │   These packages know about wire protocols; domain packages do not.
    │
    ├── store/
    │   ├── store.go                   # aggregate Store interface
    │   │                              #   (composes all domain store interfaces)
    │   │                              #   used only in server.go for wiring
    │   └── postgres/
    │       ├── postgres.go            # DB connection + Open()
    │       ├── catalog.go             # implements CatalogStore (services,
    │       │                          #   API groups, endpoints, docs)
    │       ├── service_dbs.go         # implements DB schema store methods
    │       ├── service_diagrams.go    # implements service-diagram link methods
    │       ├── service_tests.go       # implements test pack/case/run methods
    │       ├── diagrams.go            # implements DiagramStore
    │       ├── maps.go                # implements MapStore
    │       ├── folders.go             # implements FolderStore
    │       ├── org_store.go           # implements OrgStore
    │       ├── user_store.go          # implements UserStore
    │       ├── member_store.go        # implements MemberStore
    │       ├── team_store.go          # implements TeamStore
    │       ├── invitation_store.go    # implements InvitationStore
    │       ├── session_store.go       # implements SessionStore
    │       ├── provider_store.go      # implements ProviderStore
    │       ├── serviceaccount_store.go # implements ServiceAccountStore
    │       ├── authz_store.go         # implements RBACStore
    │       ├── component_catalog.go   # implements componentlib store methods
    │       └── frame_extras.go        # implements frame extras (groups, links, meta)
    │
    ├── storage/
    │   ├── storage.go                 # Client interface + key helpers
    │   └── minio.go                   # MinIO/S3 implementation
    │
    ├── cache/
    │   └── cache.go                   # Client interface + Redis implementation
    │
    ├── oauth/
    │   └── oauth.go                   # OAuth client helpers
    │
    ├── migrate/
    │   └── migrate.go                 # migration runner
    │
    │   ── Application layer ──────────────────────────────────────────────
    │   HTTP handlers. Each sub-package handles one domain.
    │   Handlers accept narrow interfaces, not store.Store.
    │   Complex multi-step logic lives in service.go, not in handler methods.
    │
    ├── api/
    │   ├── router.go                  # calls register<Domain>() per domain
    │   │
    │   ├── health/
    │   │   └── handler.go             # Healthz, Livez — unchanged
    │   │
    │   ├── auth/                      # unchanged — already well structured
    │   │   ├── session.go             # login, logout, me, OAuth callbacks, SAML
    │   │   ├── user.go                # user management (server-admin)
    │   │   ├── org.go                 # org CRUD
    │   │   ├── member.go              # org membership
    │   │   ├── team.go                # teams + team membership
    │   │   ├── invite.go              # invitations
    │   │   ├── serviceaccount.go      # service accounts + tokens
    │   │   └── sso.go                 # SSO providers, LDAP, SAML, SCIM
    │   │
    │   ├── catalog/                   # SPLIT OUT from content/
    │   │   ├── handler.go             # ServiceHandler struct + narrow store interface
    │   │   ├── service.go             # Service CRUD handlers
    │   │   ├── service_logic.go       # SyncAPIGroup, CreateServiceWithVersion,
    │   │   │                          #   and other multi-step operations
    │   │   ├── apigroup.go            # API group CRUD + sync handlers
    │   │   ├── endpoint.go            # API endpoint CRUD handlers
    │   │   ├── doc.go                 # service doc upload/CRUD handlers
    │   │   ├── db.go                  # service DB schema CRUD + versions handlers
    │   │   └── testpack.go            # test pack, case, run, result handlers
    │   │
    │   ├── diagram/                   # SPLIT OUT from content/
    │   │   ├── handler.go             # DiagramHandler struct + narrow store interface
    │   │   ├── diagram.go             # diagram CRUD + sync handlers
    │   │   ├── version.go             # version list/create/restore handlers
    │   │   └── image.go               # image upload/list handlers
    │   │
    │   ├── map/                       # SPLIT OUT from content/
    │   │   ├── handler.go             # MapHandler struct + narrow store interface
    │   │   ├── map.go                 # map CRUD handlers
    │   │   ├── frame.go               # frame CRUD + sync handlers
    │   │   ├── focalpoint.go          # focal point CRUD + meta handlers
    │   │   ├── canvas.go              # canvas get/upsert handlers
    │   │   ├── group.go               # frame group CRUD handlers
    │   │   └── link.go                # frame link CRUD handlers
    │   │
    │   ├── folder/                    # SPLIT OUT from content/
    │   │   ├── handler.go             # FolderHandler struct + narrow store interface
    │   │   └── folder.go              # folder CRUD handlers
    │   │
    │   ├── component/                 # SPLIT OUT from content/
    │   │   ├── handler.go             # ComponentHandler + FlowComponentHandler
    │   │   ├── focal.go               # focal-point component palette handler
    │   │   ├── flow.go                # flow-diagram component palette handler
    │   │   └── icon.go                # component icon file serve handler
    │   │
    │   ├── actor/
    │   │   └── handler.go             # HTTP wrapper — calls internal/actor/ resolver
    │   │
    │   └── asset/
    │       └── handler.go             # HTTP wrapper — calls internal/asset/ resolver
    │
    │   ── Cross-cutting ────────────────────────────────────────────────
    │
    ├── httputil/
    │   └── respond.go                 # writeJSON, writeErr
    │                                  # CONSOLIDATES content/helpers.go + httputil/respond.go
    │
    ├── middleware/
    │   ├── middleware.go              # New() — builds the auth middleware chain
    │   ├── session_verifier.go        # BearerVerifier implementation
    │   └── context.go                 # PrincipalFromCtx, context key helpers
    │
    ├── config/
    │   └── config.go                  # app configuration — unchanged
    │
    ├── server/
    │   └── server.go                  # wiring: postgres → migrate → bootstrap →
    │                                  #   storage → cache → http.Server
    │
    └── bootstrap/
        ├── bootstrap.go               # seeds server-admin user on first boot
        └── components.go              # seeds component catalog into storage
                                       # ABSORBS internal/seed/ (same concern)
```

---

## Key Changes From Current Structure

| Current | Target | Reason |
|---|---|---|
| `internal/api/content/` (13 files, mixed domains) | `internal/api/catalog/`, `diagram/`, `map/`, `folder/`, `component/`, `actor/`, `asset/` | One package per domain; max ~300 lines per file |
| `store.Store` passed to every handler | Narrow interface declared in each handler package | Handlers mock only what they use — practical unit tests |
| Complex logic inline in handler methods | `service_logic.go` in handler package | Named, testable without HTTP |
| `content/helpers.go` + `httputil/respond.go` | `httputil/respond.go` only | No duplication |
| `internal/componentcatalog/` | `internal/componentlib/` | Avoids confusion with `internal/catalog/` |
| `internal/seed/` + `internal/bootstrap/` | `internal/bootstrap/` only | Same concern, one package |
| `internal/api/content/actors.go`, `content/assets.go` | `internal/api/actor/handler.go`, `internal/api/asset/handler.go` | HTTP code moves to api layer; `internal/actor/` and `internal/asset/` resolver packages stay in place |
| Duplicate JSON in `content/` and `componentcatalog/` | JSON lives in `componentlib/` only | Single source of truth |
| Errors swallowed silently | `slog.ErrorContext(r.Context(), "...", "err", err)` before every `writeErr(...500...)` | Operators and contributors can debug failures |
| `internal/authz/rbac_store.go` | `internal/authz/store.go` | Consistent naming with other domain packages |

---

## Narrow Interface Pattern (per handler package)

Each handler package declares the minimal interface it needs. Example for the folder handler:

```go
// internal/api/folder/handler.go

type folderStore interface {
    ListFolders(ctx context.Context, orgID string, teamID *string) ([]folder.Folder, error)
    GetFolder(ctx context.Context, id string) (*folder.Folder, error)
    CreateFolder(ctx context.Context, f folder.Folder) error
    UpdateFolder(ctx context.Context, f folder.Folder) error
    DeleteFolder(ctx context.Context, id, actorID string) error
}

type Handler struct {
    store folderStore
}
```

`postgres.DB` satisfies `folderStore` automatically — no change to the implementation needed. Tests pass a struct that implements only these 5 methods.

---

## Error Handling Pattern

Every internal error path must log the real error before returning a generic response:

```go
svc, err := h.store.GetService(r.Context(), r.PathValue("serviceID"))
if err != nil {
    slog.ErrorContext(r.Context(), "get service", "err", err)
    httputil.WriteErr(w, http.StatusInternalServerError, "internal error")
    return
}
```

`store.ErrNotFound` is checked explicitly to return 404, not 500:

```go
if errors.Is(err, store.ErrNotFound) {
    httputil.WriteErr(w, http.StatusNotFound, "not found")
    return
}
```

---

## Router Decomposition

`router.go` becomes a coordinator that calls one registration function per domain:

```go
func New(s store.Store, ...) http.Handler {
    mux := http.NewServeMux()
    // ...
    catalog.Register(mux, s, requireScope, st, c)
    diagram.Register(mux, s, requireScope, st, c)
    mapapi.Register(mux, s, requireScope, st)
    folder.Register(mux, s, requireScope)
    component.Register(mux, s, protected, st)
    // ...
    return mux
}
```

Each `Register()` function lives in its handler package and wires its own routes.

---

## What Is Out of Scope

- No logic changes — this is file/package reorganisation only.
- No new features.
- No database schema changes.
- The `store/postgres/` file split is cosmetic (the DB struct stays the same; files just move).
- The aggregate `store.Store` interface in `store/store.go` stays — it is only used in `server.go` for wiring.

---

## Success Criteria

- [ ] `internal/api/content/` is deleted.
- [ ] No handler file exceeds 300 lines.
- [ ] Every handler struct holds a narrow interface, not `store.Store`.
- [ ] `internal/componentcatalog/` is renamed to `internal/componentlib/`.
- [ ] `internal/seed/` is deleted (logic absorbed into `bootstrap/`).
- [ ] `httputil/respond.go` is the single source of HTTP response helpers.
- [ ] Every 500 error path logs the underlying error via `slog.ErrorContext`.
- [ ] All existing tests pass after the reorganisation.
- [ ] `go build ./...` passes with zero errors.
