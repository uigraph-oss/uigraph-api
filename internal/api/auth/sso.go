package auth

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/uigraph/app/internal/asset"
	"github.com/uigraph/app/internal/authz"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/identity"
	"github.com/uigraph/app/internal/oauth"
	"github.com/uigraph/app/internal/storage"
	"github.com/uigraph/app/internal/store"
)

const maxIconBytes = 5 << 20

type ssoStore interface {
	identity.ProviderStore
	authz.RBACStore
}

type SSOHandler struct {
	store   ssoStore
	storage storage.Client
	assets  *asset.Resolver
}

func NewSSOHandler(s ssoStore, st storage.Client, assets *asset.Resolver) *SSOHandler {
	return &SSOHandler{store: s, storage: st, assets: assets}
}

func (h *SSOHandler) iconURL(r *http.Request, assetID string) string {
	if assetID == "" || h.assets == nil {
		return ""
	}
	u, err := h.assets.Resolve(r.Context(), assetID)
	if err != nil {
		return ""
	}
	return u
}

// ── Request / Response types ─────────────────────────────────────────────────

type upsertOAuthRequest struct {
	Type           string `json:"type"` // generic | entra | okta
	DisplayName    string `json:"displayName"`
	ClientID       string `json:"clientId"`
	ClientSecret   string `json:"clientSecret"`
	AuthURL        string `json:"authUrl"`
	TokenURL       string `json:"tokenUrl"`
	UserinfoURL    string `json:"userinfoUrl"`
	APIURL         string `json:"apiUrl,omitempty"` // entra: tenant ID; okta: domain
	Scopes         string `json:"scopes"`
	AllowedDomains string `json:"allowedDomains,omitempty"`
	AllowSignUp    bool   `json:"allowSignUp"`
	EmailClaim     string `json:"emailClaim"`
	NameClaim      string `json:"nameClaim"`
	SubClaim       string `json:"subClaim"`
}

type createMappingRequest struct {
	ClaimKey     string `json:"claimKey"`
	ClaimValue   string `json:"claimValue"`
	Role         string `json:"role"`
	Scope        string `json:"scope"`        // org | resource
	ResourceType string `json:"resourceType"` // required when scope=resource
	ResourceID   string `json:"resourceId"`   // empty = all of that type
}

type upsertLDAPRequest struct {
	Host          string `json:"host"`
	Port          int    `json:"port"`
	UseSSL        bool   `json:"useSsl"`
	StartTLS      bool   `json:"startTls"`
	SkipTLSVerify bool   `json:"skipTlsVerify"`
	BindDN        string `json:"bindDn,omitempty"`
	BindPassword  string `json:"bindPassword,omitempty"`
	SearchBaseDN  string `json:"searchBaseDn"`
	SearchFilter  string `json:"searchFilter"`
	EmailAttribute string `json:"emailAttribute"`
	NameAttribute  string `json:"nameAttribute"`
	UsernameAttr   string `json:"usernameAttribute"`
	MemberOfAttr   string `json:"memberOfAttribute"`
	AllowSignUp    bool   `json:"allowSignUp"`
}

type upsertSAMLRequest struct {
	IDPMetadataURL  string `json:"idpMetadataUrl,omitempty"`
	IDPMetadataXML  string `json:"idpMetadataXml,omitempty"`
	SPEntityID      string `json:"spEntityId"`
	SignRequests     bool   `json:"signRequests"`
	NameIDFormat    string `json:"nameIdFormat"`
	EmailAttribute  string `json:"emailAttribute"`
	NameAttribute   string `json:"nameAttribute"`
	LoginAttribute  string `json:"loginAttribute"`
	GroupsAttribute string `json:"groupsAttribute,omitempty"`
	AllowSignUp     bool   `json:"allowSignUp"`
}

type upsertSCIMRequest struct {
	Enabled    bool `json:"enabled"`
	SyncUsers  bool `json:"syncUsers"`
	SyncGroups bool `json:"syncGroups"`
}

// ── OAuth handlers ────────────────────────────────────────────────────────────

// ListOAuthProviders returns all configured OAuth providers (global).
// GET /api/v1/sso/oauth
// @Summary  ListOAuthProviders
// @Tags     sso
// @Security BearerAuth
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /sso/oauth [get]
func (h *SSOHandler) ListOAuthProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := h.store.ListOAuthProviders(r.Context())
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	// Never expose stored secrets; mask to indicate one is set.
	for i := range providers {
		if providers[i].ClientSecret != "" {
			providers[i].ClientSecret = "***"
		}
		providers[i].IconURL = h.iconURL(r, providers[i].IconURL)
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"providers": providers})
}

// PutOAuthProviderIcon
// @Summary  PutOAuthProviderIcon
// @Tags     sso
// @Security BearerAuth
// @Param    provider  path  string  true  "provider"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /sso/oauth/{provider}/icon [put]
func (h *SSOHandler) PutOAuthProviderIcon(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")

	existing, err := h.store.GetOAuthProvider(r.Context(), provider)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httputil.BadRequest(w, "missing file")
		return
	}
	defer file.Close()

	if header.Size > maxIconBytes {
		httputil.BadRequest(w, "icon too large")
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	assetID := storage.OAuthProviderIconAssetID(provider)
	if err := h.storage.Upload(r.Context(), storage.AssetKey(assetID), contentType, file, header.Size); err != nil {
		httputil.Error(w, r, err)
		return
	}
	if err := h.store.SetOAuthProviderIcon(r.Context(), provider, &assetID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"assetId": assetID})
}

// DeleteOAuthProviderIcon
// @Summary  DeleteOAuthProviderIcon
// @Tags     sso
// @Security BearerAuth
// @Param    provider  path  string  true  "provider"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /sso/oauth/{provider}/icon [delete]
func (h *SSOHandler) DeleteOAuthProviderIcon(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	assetID := storage.OAuthProviderIconAssetID(provider)
	_ = h.storage.Delete(r.Context(), storage.AssetKey(assetID))
	if err := h.store.SetOAuthProviderIcon(r.Context(), provider, nil); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UpsertOAuthProvider creates or updates an OAuth provider configuration.
// PUT /api/v1/sso/oauth/{provider}
// @Summary  UpsertOAuthProvider
// @Tags     sso
// @Security BearerAuth
// @Param    provider  path  string  true  "provider"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /sso/oauth/{provider} [put]
func (h *SSOHandler) UpsertOAuthProvider(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	var req upsertOAuthRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}

	// Trim surrounding whitespace from credentials and endpoints; pasted secrets
	// frequently carry a trailing space/newline that the IdP then rejects.
	req.ClientID = strings.TrimSpace(req.ClientID)
	req.ClientSecret = strings.TrimSpace(req.ClientSecret)
	req.AuthURL = strings.TrimSpace(req.AuthURL)
	req.TokenURL = strings.TrimSpace(req.TokenURL)
	req.UserinfoURL = strings.TrimSpace(req.UserinfoURL)
	req.APIURL = strings.TrimSpace(req.APIURL)

	providerType := req.Type
	if providerType == "" {
		providerType = oauth.Generic
	}

	authURL, tokenURL, userinfoURL := req.AuthURL, req.TokenURL, req.UserinfoURL
	// For Entra/Okta, derive endpoints from the tenant ID / domain (apiUrl) when
	// explicit URLs were not supplied, so the login path can read them directly.
	switch providerType {
	case oauth.Entra:
		if authURL == "" || tokenURL == "" || userinfoURL == "" {
			if req.APIURL == "" {
				httputil.BadRequest(w, "entra provider requires apiUrl (tenant ID)")
				return
			}
			authURL, tokenURL, userinfoURL = oauth.EntraEndpoints(req.APIURL)
		}
	case oauth.Okta:
		if authURL == "" || tokenURL == "" || userinfoURL == "" {
			if req.APIURL == "" {
				httputil.BadRequest(w, "okta provider requires apiUrl (domain)")
				return
			}
			authURL, tokenURL, userinfoURL = oauth.OktaEndpoints(req.APIURL)
		}
	}

	// Preserve the stored secret when the client omits it (or sends the mask),
	// so editing other fields doesn't wipe the secret.
	clientSecret := req.ClientSecret
	if clientSecret == "" || clientSecret == "***" {
		existing, err := h.store.GetOAuthProvider(r.Context(), provider)
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
		if existing != nil {
			clientSecret = existing.ClientSecret
		}
	}

	// TODO: encrypt client_secret before persistence
	cfg := identity.OAuthProviderConfig{
		ProviderName: provider,
		Type:         providerType, DisplayName: req.DisplayName,
		ClientID: req.ClientID, ClientSecret: clientSecret,
		AuthURL: authURL, TokenURL: tokenURL, UserinfoURL: userinfoURL,
		APIURL: req.APIURL, Scopes: req.Scopes, AllowedDomains: req.AllowedDomains,
		AllowSignUp: req.AllowSignUp, EmailClaim: req.EmailClaim,
		NameClaim: req.NameClaim, SubClaim: req.SubClaim,
	}
	if err := h.store.UpsertOAuthProvider(r.Context(), cfg); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteOAuthProvider removes an OAuth provider configuration.
// DELETE /api/v1/sso/oauth/{provider}
// @Summary  DeleteOAuthProvider
// @Tags     sso
// @Security BearerAuth
// @Param    provider  path  string  true  "provider"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /sso/oauth/{provider} [delete]
func (h *SSOHandler) DeleteOAuthProvider(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	if err := h.store.DeleteOAuthProvider(r.Context(), provider); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── SSO role mapping handlers ─────────────────────────────────────────────────

// ListMappings returns all SSO claim → role mappings (global).
// GET /api/v1/sso/role-mappings
// @Summary  ListMappings
// @Tags     sso
// @Security BearerAuth
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /sso/role-mappings [get]
func (h *SSOHandler) ListMappings(w http.ResponseWriter, r *http.Request) {
	mappings, err := h.store.ListSSOMappings(r.Context(), "")
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"mappings": mappings})
}

// CreateMapping adds a claim → role rule evaluated on every SSO login.
// POST /api/v1/sso/role-mappings
// @Summary  CreateMapping
// @Tags     sso
// @Security BearerAuth
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /sso/role-mappings [post]
func (h *SSOHandler) CreateMapping(w http.ResponseWriter, r *http.Request) {
	var req createMappingRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	if req.ClaimKey == "" || req.ClaimValue == "" || req.Role == "" || req.Scope == "" {
		httputil.BadRequest(w, "claimKey, claimValue, role, and scope are required")
		return
	}
	m := authz.SSOMapping{
		ID:           uuid.NewString(),
		ClaimKey:     req.ClaimKey,
		ClaimValue:   req.ClaimValue,
		Role:         authz.Role(req.Role),
		Scope:        req.Scope,
		ResourceType: authz.ResourceType(req.ResourceType),
		ResourceID:   req.ResourceID,
	}
	if err := h.store.CreateSSOMapping(r.Context(), m); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// DeleteMapping removes a claim → role mapping.
// DELETE /api/v1/sso/role-mappings/{mappingID}
// @Summary  DeleteMapping
// @Tags     sso
// @Security BearerAuth
// @Param    mappingID  path  string  true  "mappingID"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /sso/role-mappings/{mappingID} [delete]
func (h *SSOHandler) DeleteMapping(w http.ResponseWriter, r *http.Request) {
	mappingID := r.PathValue("mappingID")
	if err := h.store.DeleteSSOMapping(r.Context(), mappingID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

// ── SAML handlers ─────────────────────────────────────────────────────────────

// GetSAML returns the global SAML SP configuration (SP private key masked).
// GET /api/v1/sso/saml
// @Summary  GetSAML
// @Tags     sso
// @Security BearerAuth
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /sso/saml [get]
func (h *SSOHandler) GetSAML(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetSAMLConfig(r.Context())
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if cfg == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}
	cfg.SPKey = "***"
	httputil.JSON(w, http.StatusOK, cfg)
}

// UpsertSAML creates or replaces the global SAML configuration.
// PUT /api/v1/sso/saml
// @Summary  UpsertSAML
// @Tags     sso
// @Security BearerAuth
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /sso/saml [put]
func (h *SSOHandler) UpsertSAML(w http.ResponseWriter, r *http.Request) {
	var req upsertSAMLRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	// TODO: fetch + parse IdP metadata if IDPMetadataURL is set
	cfg := identity.SAMLConfig{
		IDPMetadataURL: req.IDPMetadataURL, IDPMetadataXML: req.IDPMetadataXML,
		SPEntityID: req.SPEntityID, SignRequests: req.SignRequests,
		NameIDFormat: req.NameIDFormat, EmailAttribute: req.EmailAttribute,
		NameAttribute: req.NameAttribute, LoginAttribute: req.LoginAttribute,
		GroupsAttribute: req.GroupsAttribute, AllowSignUp: req.AllowSignUp,
	}
	if err := h.store.UpsertSAMLConfig(r.Context(), cfg); err != nil {
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
