package auth

import (
	"net/http"

	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/identity"
	"github.com/uigraph/app/internal/store"
)

// SSOHandler covers the directory-integration features that are still global
// and still unimplemented: LDAP and SCIM. OIDC and SAML moved to the org-scoped
// AuthProviderHandler.
type ssoStore interface {
	identity.ProviderStore
}

type SSOHandler struct {
	store ssoStore
}

func NewSSOHandler(s ssoStore) *SSOHandler {
	return &SSOHandler{store: s}
}

type upsertLDAPRequest struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	UseSSL         bool   `json:"useSsl"`
	StartTLS       bool   `json:"startTls"`
	SkipTLSVerify  bool   `json:"skipTlsVerify"`
	BindDN         string `json:"bindDn,omitempty"`
	BindPassword   string `json:"bindPassword,omitempty"`
	SearchBaseDN   string `json:"searchBaseDn"`
	SearchFilter   string `json:"searchFilter"`
	EmailAttribute string `json:"emailAttribute"`
	NameAttribute  string `json:"nameAttribute"`
	UsernameAttr   string `json:"usernameAttribute"`
	MemberOfAttr   string `json:"memberOfAttribute"`
	AllowSignUp    bool   `json:"allowSignUp"`
}

type upsertSCIMRequest struct {
	Enabled    bool `json:"enabled"`
	SyncUsers  bool `json:"syncUsers"`
	SyncGroups bool `json:"syncGroups"`
}

// ── LDAP handlers ─────────────────────────────────────────────────────────────

// GetLDAP returns the global LDAP configuration (password masked).
// GET /api/v1/sso/ldap
// @Summary  GetLDAP
// @Tags     sso
// @Security BearerAuth
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /sso/ldap [get]
func (h *SSOHandler) GetLDAP(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetLDAPConfig(r.Context())
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if cfg == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}
	cfg.BindPassword = "***"
	httputil.JSON(w, http.StatusOK, cfg)
}

// UpsertLDAP creates or replaces the global LDAP configuration.
// PUT /api/v1/sso/ldap
// @Summary  UpsertLDAP
// @Tags     sso
// @Security BearerAuth
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /sso/ldap [put]
func (h *SSOHandler) UpsertLDAP(w http.ResponseWriter, r *http.Request) {
	var req upsertLDAPRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	// TODO: encrypt bind_password before persistence
	cfg := identity.LDAPConfig{
		Host: req.Host, Port: req.Port,
		UseSSL: req.UseSSL, StartTLS: req.StartTLS, SkipTLSVerify: req.SkipTLSVerify,
		BindDN: req.BindDN, BindPassword: req.BindPassword,
		SearchBaseDN: req.SearchBaseDN, SearchFilter: req.SearchFilter,
		EmailAttribute: req.EmailAttribute, NameAttribute: req.NameAttribute,
		UsernameAttr: req.UsernameAttr, MemberOfAttr: req.MemberOfAttr,
		AllowSignUp: req.AllowSignUp,
	}
	if err := h.store.UpsertLDAPConfig(r.Context(), cfg); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteLDAP removes the global LDAP configuration.
// DELETE /api/v1/sso/ldap
// @Summary  DeleteLDAP
// @Tags     sso
// @Security BearerAuth
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /sso/ldap [delete]
func (h *SSOHandler) DeleteLDAP(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteLDAPConfig(r.Context()); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── SCIM handlers ─────────────────────────────────────────────────────────────

// GetSCIM returns the global SCIM provisioning config (token hash masked).
// GET /api/v1/sso/scim
// @Summary  GetSCIM
// @Tags     sso
// @Security BearerAuth
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /sso/scim [get]
func (h *SSOHandler) GetSCIM(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetSCIMConfig(r.Context())
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if cfg == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}
	cfg.BearerTokenHash = "***"
	httputil.JSON(w, http.StatusOK, cfg)
}

// UpsertSCIM enables or reconfigures global SCIM provisioning.
// PUT /api/v1/sso/scim
// @Summary  UpsertSCIM
// @Tags     sso
// @Security BearerAuth
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /sso/scim [put]
func (h *SSOHandler) UpsertSCIM(w http.ResponseWriter, r *http.Request) {
	var req upsertSCIMRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	httputil.NotImplemented(w)
}

// RotateSCIMToken generates a new SCIM bearer token, stores its hash, and
// returns the plaintext once.
// POST /api/v1/sso/scim/rotate-token
// @Summary  RotateSCIMToken
// @Tags     sso
// @Security BearerAuth
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /sso/scim/rotate-token [post]
func (h *SSOHandler) RotateSCIMToken(w http.ResponseWriter, r *http.Request) {
	// TODO: generate token, hash it, call store.RotateSCIMToken, return plaintext
	httputil.NotImplemented(w)
}
