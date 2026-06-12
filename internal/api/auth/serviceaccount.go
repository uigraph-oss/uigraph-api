package auth

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/identity"
	"github.com/uigraph/app/internal/store"
)

type serviceAccountStore interface {
	identity.ServiceAccountStore
}

type ServiceAccountHandler struct {
	store serviceAccountStore
}

func NewServiceAccountHandler(s serviceAccountStore) *ServiceAccountHandler {
	return &ServiceAccountHandler{store: s}
}

// ── Request / Response types ─────────────────────────────────────────────────

type createServiceAccountRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Role        string `json:"role"` // admin | editor | viewer
}

type updateServiceAccountRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Disabled    bool   `json:"disabled"`
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
	if req.Name == "" || req.Role == "" {
		httputil.BadRequest(w, "name and role are required")
		return
	}
	sa := identity.ServiceAccount{
		ID:          uuid.NewString(),
		OrgID:       orgID,
		Name:        req.Name,
		Description: req.Description,
		Role:        req.Role,
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
		Disabled:    req.Disabled,
	}
	if err := h.store.UpdateServiceAccount(r.Context(), sa); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Delete soft-deletes a service account and revokes all its tokens.
// DELETE /api/v1/orgs/{orgID}/service-accounts/{saID}
func (h *ServiceAccountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	saID := r.PathValue("saID")
	if err := h.store.DeleteServiceAccount(r.Context(), saID); err != nil {
		httputil.Error(w, r, err)
		return
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
	// TODO: parse req.ExpiresAt into tok.ExpiresAt

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
