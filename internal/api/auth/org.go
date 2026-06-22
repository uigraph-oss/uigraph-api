package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/uigraph/app/internal/asset"
	"github.com/uigraph/app/internal/authz"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/identity"
	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/org"
	"github.com/uigraph/app/internal/store"
)

type saProvisioner interface {
	CreateServiceAccount(ctx context.Context, sa identity.ServiceAccount) error
}

type OrgHandler struct {
	store   org.OrgStore
	members org.MemberStore
	sa      saProvisioner
	assets  *asset.Resolver // presigns the logo URL; may be nil
}

func NewOrgHandler(s org.OrgStore, m org.MemberStore, sa saProvisioner, assets *asset.Resolver) *OrgHandler {
	return &OrgHandler{store: s, members: m, sa: sa, assets: assets}
}

// logoURL presigns the logo asset id, returning "" when there is no logo or no
// resolver configured.
func (h *OrgHandler) logoURL(r *http.Request, assetID *string) string {
	if assetID == nil || *assetID == "" || h.assets == nil {
		return ""
	}
	u, err := h.assets.Resolve(r.Context(), *assetID)
	if err != nil {
		return ""
	}
	return u
}

// ── Request / Response types ─────────────────────────────────────────────────

type orgResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	LogoURL   string    `json:"logoUrl,omitempty"`
	Disabled  bool      `json:"disabled"`
	AutoJoin  bool      `json:"autoJoin"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (h *OrgHandler) orgToResponse(r *http.Request, o org.Org) orgResponse {
	return orgResponse{
		ID: o.ID, Name: o.Name, LogoURL: h.logoURL(r, o.LogoAssetID),
		Disabled: o.Disabled, AutoJoin: o.AutoJoin, CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt,
	}
}

type createOrgRequest struct {
	Name     string `json:"name"`
	AutoJoin bool   `json:"autoJoin"`
}

type updateOrgRequest struct {
	Name     string `json:"name"`
	Disabled bool   `json:"disabled"`
	AutoJoin bool   `json:"autoJoin"`
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// List returns all orgs visible to the caller.
// GET /api/v1/orgs
func (h *OrgHandler) List(w http.ResponseWriter, r *http.Request) {
	orgs, err := h.store.ListOrgs(r.Context())
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	out := make([]orgResponse, len(orgs))
	for i, o := range orgs {
		out[i] = h.orgToResponse(r, o)
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"orgs": out})
}

// Create provisions a new org. Any authenticated user may create one; the
// creating user becomes the org's first admin.
// POST /api/v1/orgs
func (h *OrgHandler) Create(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	var req createOrgRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	if req.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}
	o := org.Org{
		ID:       newUUID(),
		Name:     req.Name,
		AutoJoin: req.AutoJoin,
	}
	if err := h.store.CreateOrg(r.Context(), o); err != nil {
		httputil.Error(w, r, err)
		return
	}
	if p.Kind == identity.PrincipalUser {
		err := h.members.AddMember(r.Context(), org.OrgMember{
			UserID: p.UserID, OrgID: o.ID, Role: "admin", Source: "manual",
		})
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
	}
	sa := identity.NewSystemServiceAccount(o.ID, allScopeStrings())
	if err := h.sa.CreateServiceAccount(r.Context(), sa); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, h.orgToResponse(r, o))
}

func allScopeStrings() []string {
	scopes := make([]string, len(authz.AllScopes))
	for i, s := range authz.AllScopes {
		scopes[i] = string(s)
	}
	return scopes
}

// Get returns a single org by ID.
// GET /api/v1/orgs/{orgID}
func (h *OrgHandler) Get(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	o, err := h.store.GetOrg(r.Context(), orgID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if o == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, h.orgToResponse(r, *o))
}

// Update changes an org's name or disabled state.
// PUT /api/v1/orgs/{orgID}
func (h *OrgHandler) Update(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	var req updateOrgRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	if req.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}
	o, err := h.store.GetOrg(r.Context(), orgID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if o == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}
	o.Name = req.Name
	o.Disabled = req.Disabled
	o.AutoJoin = req.AutoJoin
	if err := h.store.UpdateOrg(r.Context(), *o); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, h.orgToResponse(r, *o))
}

// Delete removes an org and all its data.
// DELETE /api/v1/orgs/{orgID}
func (h *OrgHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	if err := h.store.DeleteOrg(r.Context(), orgID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func newUUID() string { return uuid.NewString() }
