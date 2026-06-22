package auth

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/uigraph/app/internal/authz"
	"github.com/uigraph/app/internal/cache"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/identity"
	"github.com/uigraph/app/internal/store"
)

type serviceAccountStore interface {
	identity.ServiceAccountStore
}

type ServiceAccountHandler struct {
	store serviceAccountStore
	cache cache.Client // may be nil
}

func NewServiceAccountHandler(s serviceAccountStore, c cache.Client) *ServiceAccountHandler {
	return &ServiceAccountHandler{store: s, cache: c}
}

// ── Request / Response types ─────────────────────────────────────────────────

type createServiceAccountRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Scopes      []string `json:"scopes"` // e.g. ["diagrams:read", "diagrams:write"]
}

type updateServiceAccountRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Scopes      []string `json:"scopes"`
	Disabled    bool     `json:"disabled"`
}

// knownScopes drops any scope that is not a known grantable scope, so stale
// scopes from older clients are silently ignored rather than rejected.
func knownScopes(scopes []string) []string {
	kept := make([]string, 0, len(scopes))
	for _, s := range scopes {
		if authz.ValidScope(s) {
			kept = append(kept, s)
		}
	}
	return kept
}

type createTokenRequest struct {
	Name      string  `json:"name"`
	ExpiresAt *string `json:"expiresAt,omitempty"` // RFC3339 or null = no expiry
}

// createTokenResponse is the only response that includes the plaintext — shown once.
type createTokenResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Token string `json:"token"` // plaintext, shown exactly once
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// List returns all active service accounts in an org.
// GET /api/v1/orgs/{orgID}/service-accounts
func (h *ServiceAccountHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	accounts, err := h.store.ListServiceAccounts(r.Context(), orgID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"serviceAccounts": accounts})
}

// Create provisions a new service account with an org-level role.
// POST /api/v1/orgs/{orgID}/service-accounts
func (h *ServiceAccountHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	var req createServiceAccountRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	if req.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}
	sa := identity.ServiceAccount{
		ID:          uuid.NewString(),
		OrgID:       orgID,
		Name:        req.Name,
		Description: req.Description,
		Scopes:      knownScopes(req.Scopes),
	}
	if err := h.store.CreateServiceAccount(r.Context(), sa); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, sa)
}

// Get returns a single service account.
// GET /api/v1/orgs/{orgID}/service-accounts/{saID}
func (h *ServiceAccountHandler) Get(w http.ResponseWriter, r *http.Request) {
	saID := r.PathValue("saID")
	sa, err := h.store.GetServiceAccount(r.Context(), saID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if sa == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, sa)
}

// Update changes a service account's name, description, or disabled state.
// PUT /api/v1/orgs/{orgID}/service-accounts/{saID}
func (h *ServiceAccountHandler) Update(w http.ResponseWriter, r *http.Request) {
	saID := r.PathValue("saID")
	existing, err := h.store.GetServiceAccount(r.Context(), saID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}
	if existing.IsInternal {
		httputil.Forbidden(w)
		return
	}
	var req updateServiceAccountRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	if req.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}
	sa := identity.ServiceAccount{
		ID:          saID,
		Name:        req.Name,
		Description: req.Description,
		Scopes:      knownScopes(req.Scopes),
		Disabled:    req.Disabled,
	}
	if err := h.store.UpdateServiceAccount(r.Context(), sa); err != nil {
		httputil.Error(w, r, err)
		return
	}
	if h.cache != nil {
		_ = h.cache.Del(r.Context(), cache.ActorKey(saID))
	}
	w.WriteHeader(http.StatusNoContent)
}

// Delete soft-deletes a service account and revokes all its tokens.
// DELETE /api/v1/orgs/{orgID}/service-accounts/{saID}
func (h *ServiceAccountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	saID := r.PathValue("saID")
	existing, err := h.store.GetServiceAccount(r.Context(), saID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}
	if existing.IsInternal {
		httputil.Forbidden(w)
		return
	}
	if err := h.store.DeleteServiceAccount(r.Context(), saID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	if h.cache != nil {
		_ = h.cache.Del(r.Context(), cache.ActorKey(saID))
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListTokens returns all tokens for a service account (no hashes or plaintexts).
// GET /api/v1/orgs/{orgID}/service-accounts/{saID}/tokens
func (h *ServiceAccountHandler) ListTokens(w http.ResponseWriter, r *http.Request) {
	saID := r.PathValue("saID")
	tokens, err := h.store.ListTokens(r.Context(), saID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"tokens": tokens})
}

// CreateToken generates a new API token. The plaintext is returned once and never stored.
// POST /api/v1/orgs/{orgID}/service-accounts/{saID}/tokens
func (h *ServiceAccountHandler) CreateToken(w http.ResponseWriter, r *http.Request) {
	saID := r.PathValue("saID")
	var req createTokenRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	if req.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}

	id, plaintext, hash, err := identity.Generate()
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	tok := identity.Token{
		ID:               id,
		ServiceAccountID: saID,
		Name:             req.Name,
		Prefix:           identity.Prefix(plaintext),
		Hash:             hash,
	}
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		exp, perr := time.Parse(time.RFC3339, *req.ExpiresAt)
		if perr != nil {
			httputil.BadRequest(w, "expiresAt must be an RFC3339 timestamp")
			return
		}
		tok.ExpiresAt = &exp
	}

	if err := h.store.CreateToken(r.Context(), tok); err != nil {
		httputil.Error(w, r, err)
		return
	}

	httputil.JSON(w, http.StatusCreated, createTokenResponse{
		ID:    id,
		Name:  req.Name,
		Token: plaintext,
	})
}

// RevokeToken marks a token as revoked.
// DELETE /api/v1/orgs/{orgID}/service-accounts/{saID}/tokens/{tokenID}
func (h *ServiceAccountHandler) RevokeToken(w http.ResponseWriter, r *http.Request) {
	tokenID := r.PathValue("tokenID")
	if err := h.store.RevokeToken(r.Context(), tokenID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListScopes returns the catalog of grantable scopes, shared by role and
// service-account assignment.
// GET /api/v1/orgs/{orgID}/scopes
func (h *ServiceAccountHandler) ListScopes(w http.ResponseWriter, r *http.Request) {
	httputil.JSON(w, http.StatusOK, map[string]any{"scopes": authz.AllScopes})
}
