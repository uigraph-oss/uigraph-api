package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/uigraph/app/internal/asset"
	"github.com/uigraph/app/internal/authz"
	"github.com/uigraph/app/internal/crypto"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/identity"
	"github.com/uigraph/app/internal/oauth"
	"github.com/uigraph/app/internal/rolemap"
	"github.com/uigraph/app/internal/saml"
	"github.com/uigraph/app/internal/storage"
	"github.com/uigraph/app/internal/store"
)

const secretMask = "***"

type authProviderStore interface {
	identity.ProviderStore
}

type AuthProviderHandler struct {
	store     authProviderStore
	storage   storage.Client
	assets    *asset.Resolver
	cipher    *crypto.Cipher
	publicURL string
}

func NewAuthProviderHandler(s authProviderStore, st storage.Client, assets *asset.Resolver, cipher *crypto.Cipher, publicURL string) *AuthProviderHandler {
	return &AuthProviderHandler{
		store:     s,
		storage:   st,
		assets:    assets,
		cipher:    cipher,
		publicURL: strings.TrimRight(publicURL, "/"),
	}
}

type authProviderRequest struct {
	Slug           string     `json:"slug"`
	Kind           string     `json:"kind"`
	Type           string     `json:"type"`
	DisplayName    string     `json:"displayName"`
	Enabled        bool       `json:"enabled"`
	AllowSignUp    bool       `json:"allowSignUp"`
	AllowedDomains string     `json:"allowedDomains"`
	DefaultRole    authz.Role `json:"defaultRole"`

	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	AuthURL      string `json:"authUrl"`
	TokenURL     string `json:"tokenUrl"`
	UserinfoURL  string `json:"userinfoUrl"`
	APIURL       string `json:"apiUrl"`
	Scopes       string `json:"scopes"`
	EmailClaim   string `json:"emailClaim"`
	NameClaim    string `json:"nameClaim"`
	SubClaim     string `json:"subClaim"`
	GroupsClaim  string `json:"groupsClaim"`

	IDPMetadataURL  string `json:"idpMetadataUrl"`
	IDPMetadataXML  string `json:"idpMetadataXml"`
	IDPEntityID     string `json:"idpEntityId"`
	IDPSSOURL       string `json:"idpSsoUrl"`
	IDPCert         string `json:"idpCert"`
	SignRequests    bool   `json:"signRequests"`
	NameIDFormat    string `json:"nameIdFormat"`
	EmailAttribute  string `json:"emailAttribute"`
	NameAttribute   string `json:"nameAttribute"`
	GroupsAttribute string `json:"groupsAttribute"`
}

type roleMappingRequest struct {
	Priority       int        `json:"priority"`
	AttributeKey   string     `json:"attributeKey"`
	Operator       string     `json:"operator"`
	AttributeValue string     `json:"attributeValue"`
	Role           authz.Role `json:"role"`
}

type orgDomainRequest struct {
	Domain string `json:"domain"`
}

// @Summary  ListProviders
// @Tags     authproviders
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/auth/providers [get]
func (h *AuthProviderHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := h.store.ListAuthProviders(r.Context(), r.PathValue("orgID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	out := make([]identity.AuthProvider, 0, len(providers))
	for _, p := range providers {
		out = append(out, h.present(r, p))
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"providers": out})
}

// @Summary  GetProvider
// @Tags     authproviders
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    slug   path  string  true  "slug"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/auth/providers/{slug} [get]
func (h *AuthProviderHandler) GetProvider(w http.ResponseWriter, r *http.Request) {
	p, err := h.load(r)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, h.present(r, *p))
}

// @Summary  CreateProvider
// @Tags     authproviders
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    body  body  object  false  "request body"
// @Success  201  {object}  map[string]interface{}
// @Failure  400  {object}  httputil.errorBody
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/auth/providers [post]
func (h *AuthProviderHandler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	var req authProviderRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}

	p := identity.AuthProvider{ID: newID(), Slug: req.Slug, OrgID: r.PathValue("orgID")}
	if err := h.apply(r, &req, &p); err != nil {
		httputil.BadRequest(w, err.Error())
		return
	}
	if err := p.Validate(); err != nil {
		httputil.BadRequest(w, err.Error())
		return
	}
	if err := h.store.CreateAuthProvider(r.Context(), p); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, h.present(r, p))
}

// @Summary  UpdateProvider
// @Tags     authproviders
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    slug   path  string  true  "slug"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  400  {object}  httputil.errorBody
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/auth/providers/{slug} [put]
func (h *AuthProviderHandler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	p, err := h.load(r)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	var req authProviderRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	if req.Slug != p.Slug {
		httputil.BadRequest(w, "slug cannot be changed after the provider is created")
		return
	}
	if err := h.apply(r, &req, p); err != nil {
		httputil.BadRequest(w, err.Error())
		return
	}
	if err := p.Validate(); err != nil {
		httputil.BadRequest(w, err.Error())
		return
	}
	if err := h.store.UpdateAuthProvider(r.Context(), *p); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, h.present(r, *p))
}

// @Summary  DeleteProvider
// @Tags     authproviders
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    slug   path  string  true  "slug"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/auth/providers/{slug} [delete]
func (h *AuthProviderHandler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	p, err := h.load(r)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if err := h.store.DeleteAuthProvider(r.Context(), p.OrgID, p.ID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary  PrepareProviderIconUpload
// @Tags     authproviders
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    slug   path  string  true  "slug"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/auth/providers/{slug}/icon/prepare [post]
func (h *AuthProviderHandler) PrepareProviderIconUpload(w http.ResponseWriter, r *http.Request) {
	p, err := h.load(r)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	assetID := storage.AuthProviderIconAssetID(p.ID)
	url, err := h.storage.PresignPutURL(r.Context(), storage.AssetKey(assetID))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"assetId": assetID, "uploadUrl": url})
}

// @Summary  PutProviderIcon
// @Tags     authproviders
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    slug   path  string  true  "slug"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/auth/providers/{slug}/icon [put]
func (h *AuthProviderHandler) PutProviderIcon(w http.ResponseWriter, r *http.Request) {
	p, err := h.load(r)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	assetID := storage.AuthProviderIconAssetID(p.ID)
	if err := h.store.SetAuthProviderIcon(r.Context(), p.OrgID, p.ID, &assetID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"assetId": assetID})
}

// @Summary  DeleteProviderIcon
// @Tags     authproviders
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    slug   path  string  true  "slug"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/auth/providers/{slug}/icon [delete]
func (h *AuthProviderHandler) DeleteProviderIcon(w http.ResponseWriter, r *http.Request) {
	p, err := h.load(r)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	_ = h.storage.Delete(r.Context(), storage.AssetKey(storage.AuthProviderIconAssetID(p.ID)))
	if err := h.store.SetAuthProviderIcon(r.Context(), p.OrgID, p.ID, nil); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary  SAMLMetadata
// @Tags     authproviders
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    slug   path  string  true  "slug"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/auth/providers/{slug}/saml/metadata [get]
func (h *AuthProviderHandler) SAMLMetadata(w http.ResponseWriter, r *http.Request) {
	p, err := h.load(r)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if p.Kind != identity.KindSAML {
		httputil.BadRequest(w, "provider is not a SAML provider")
		return
	}

	if err := h.ensureSPKeypair(r, p); err != nil {
		httputil.Error(w, r, err)
		return
	}

	key, err := h.decrypt(p.SPKey)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	acs := acsURL(h.publicURL, p.OrgID, p.Slug)
	metadataURL := samlMetadataURL(h.publicURL, p.OrgID, p.Slug)

	doc, err := saml.SPMetadata(samlProviderFor(p, h.publicURL, key), acs, metadataURL)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"entityId":    metadataURL,
		"acsUrl":      acs,
		"metadataUrl": metadataURL,
		"metadata":    doc,
	})
}

// @Summary  ListRoleMappings
// @Tags     authproviders
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    slug   path  string  true  "slug"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/auth/providers/{slug}/role-mappings [get]
func (h *AuthProviderHandler) ListRoleMappings(w http.ResponseWriter, r *http.Request) {
	p, err := h.load(r)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	rules, err := h.store.ListRoleMappings(r.Context(), p.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"mappings": rules})
}

// @Summary  CreateRoleMapping
// @Tags     authproviders
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    slug   path  string  true  "slug"
// @Param    body  body  object  false  "request body"
// @Success  201  {object}  map[string]interface{}
// @Failure  400  {object}  httputil.errorBody
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/auth/providers/{slug}/role-mappings [post]
func (h *AuthProviderHandler) CreateRoleMapping(w http.ResponseWriter, r *http.Request) {
	p, err := h.load(r)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	var req roleMappingRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}

	rule := rolemap.Rule{
		ID:             newID(),
		ProviderID:     p.ID,
		Priority:       req.Priority,
		AttributeKey:   strings.TrimSpace(req.AttributeKey),
		Operator:       req.Operator,
		AttributeValue: req.AttributeValue,
		Role:           req.Role,
	}
	if err := rolemap.ValidateRule(rule); err != nil {
		httputil.BadRequest(w, err.Error())
		return
	}
	if err := h.store.CreateRoleMapping(r.Context(), rule); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, rule)
}

// @Summary  UpdateRoleMapping
// @Tags     authproviders
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    slug   path  string  true  "slug"
// @Param    mappingID  path  string  true  "mappingID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  400  {object}  httputil.errorBody
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/auth/providers/{slug}/role-mappings/{mappingID} [put]
func (h *AuthProviderHandler) UpdateRoleMapping(w http.ResponseWriter, r *http.Request) {
	p, err := h.load(r)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	var req roleMappingRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}

	rule := rolemap.Rule{
		ID:             r.PathValue("mappingID"),
		ProviderID:     p.ID,
		Priority:       req.Priority,
		AttributeKey:   strings.TrimSpace(req.AttributeKey),
		Operator:       req.Operator,
		AttributeValue: req.AttributeValue,
		Role:           req.Role,
	}
	if err := rolemap.ValidateRule(rule); err != nil {
		httputil.BadRequest(w, err.Error())
		return
	}
	if err := h.store.UpdateRoleMapping(r.Context(), rule); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, rule)
}

// @Summary  DeleteRoleMapping
// @Tags     authproviders
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    slug   path  string  true  "slug"
// @Param    mappingID  path  string  true  "mappingID"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/auth/providers/{slug}/role-mappings/{mappingID} [delete]
func (h *AuthProviderHandler) DeleteRoleMapping(w http.ResponseWriter, r *http.Request) {
	p, err := h.load(r)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if err := h.store.DeleteRoleMapping(r.Context(), p.ID, r.PathValue("mappingID")); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary  MappingOperators
// @Tags     authproviders
// @Security BearerAuth
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Router   /auth/mapping-operators [get]
func (h *AuthProviderHandler) MappingOperators(w http.ResponseWriter, r *http.Request) {
	type operator struct {
		Name       string `json:"name"`
		TakesValue bool   `json:"takesValue"`
	}
	out := make([]operator, 0, len(rolemap.ValidOperators))
	for _, op := range rolemap.ValidOperators {
		out = append(out, operator{Name: op, TakesValue: rolemap.TakesValue(op)})
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"operators": out})
}

// @Summary  ListDomains
// @Tags     authproviders
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/auth/domains [get]
func (h *AuthProviderHandler) ListDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := h.store.ListOrgDomains(r.Context(), r.PathValue("orgID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"domains": domains})
}

// @Summary  CreateDomain
// @Tags     authproviders
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    body  body  object  false  "request body"
// @Success  201  {object}  map[string]interface{}
// @Failure  400  {object}  httputil.errorBody
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  409  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/auth/domains [post]
func (h *AuthProviderHandler) CreateDomain(w http.ResponseWriter, r *http.Request) {
	var req orgDomainRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}

	domain := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(req.Domain, "@")))
	if domain == "" || strings.ContainsAny(domain, "@ /") || !strings.Contains(domain, ".") {
		httputil.BadRequest(w, "a valid email domain is required")
		return
	}

	d := identity.OrgDomain{ID: newID(), OrgID: r.PathValue("orgID"), Domain: domain}
	if err := h.store.CreateOrgDomain(r.Context(), d); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, d)
}

// @Summary  DeleteDomain
// @Tags     authproviders
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    domainID  path  string  true  "domainID"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/auth/domains/{domainID} [delete]
func (h *AuthProviderHandler) DeleteDomain(w http.ResponseWriter, r *http.Request) {
	err := h.store.DeleteOrgDomain(r.Context(), r.PathValue("orgID"), r.PathValue("domainID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthProviderHandler) load(r *http.Request) (*identity.AuthProvider, error) {
	p, err := h.store.GetAuthProviderBySlug(r.Context(), r.PathValue("orgID"), r.PathValue("slug"))
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, store.ErrNotFound
	}
	return p, nil
}

func (h *AuthProviderHandler) decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if h.cipher == nil {
		return "", fmt.Errorf("no secret key is configured, provider secrets cannot be read")
	}
	return h.cipher.Decrypt(ciphertext)
}

func (h *AuthProviderHandler) ensureSPKeypair(r *http.Request, p *identity.AuthProvider) error {
	if p.SPCert != "" && p.SPKey != "" {
		return nil
	}
	if h.cipher == nil {
		return fmt.Errorf("no secret key is configured, an SP keypair cannot be generated")
	}

	certPEM, keyPEM, err := saml.GenerateSPKeypair(samlMetadataURL(h.publicURL, p.OrgID, p.Slug))
	if err != nil {
		return err
	}
	enc, err := h.cipher.Encrypt(keyPEM)
	if err != nil {
		return err
	}

	p.SPCert = certPEM
	p.SPKey = enc
	return h.store.UpdateAuthProvider(r.Context(), *p)
}

func (h *AuthProviderHandler) present(r *http.Request, p identity.AuthProvider) identity.AuthProvider {
	if p.Kind == identity.KindSAML {
		p.SPEntityID = samlMetadataURL(h.publicURL, p.OrgID, p.Slug)
	}
	if p.ClientSecret != "" {
		p.ClientSecret = secretMask
	}
	if p.SPKey != "" {
		p.SPKey = secretMask
	}
	if p.IconAssetID != "" && h.assets != nil {
		if u, err := h.assets.Resolve(r.Context(), p.IconAssetID); err == nil {
			p.IconURL = u
		}
	}
	return p
}

func (h *AuthProviderHandler) apply(r *http.Request, req *authProviderRequest, p *identity.AuthProvider) error {
	p.Kind = req.Kind
	p.DisplayName = strings.TrimSpace(req.DisplayName)
	p.Enabled = req.Enabled
	p.AllowSignUp = req.AllowSignUp
	p.AllowedDomains = strings.TrimSpace(req.AllowedDomains)
	p.DefaultRole = req.DefaultRole

	switch req.Kind {
	case identity.KindOIDC:
		return h.applyOIDC(req, p)
	case identity.KindSAML:
		return h.applySAML(r, req, p)
	default:
		return fmt.Errorf("unknown provider kind %q", req.Kind)
	}
}

func (h *AuthProviderHandler) applyOIDC(req *authProviderRequest, p *identity.AuthProvider) error {
	p.Type = req.Type
	if p.Type == "" {
		p.Type = identity.TypeGeneric
	}
	p.ClientID = strings.TrimSpace(req.ClientID)
	p.AuthURL = strings.TrimSpace(req.AuthURL)
	p.TokenURL = strings.TrimSpace(req.TokenURL)
	p.UserinfoURL = strings.TrimSpace(req.UserinfoURL)
	p.APIURL = strings.TrimSpace(req.APIURL)
	p.Scopes = req.Scopes
	p.EmailClaim = req.EmailClaim
	p.NameClaim = req.NameClaim
	p.SubClaim = req.SubClaim
	p.GroupsClaim = req.GroupsClaim

	if p.AuthURL == "" || p.TokenURL == "" || p.UserinfoURL == "" {
		switch p.Type {
		case identity.TypeEntra:
			if p.APIURL == "" {
				return fmt.Errorf("an entra provider needs apiUrl (the directory/tenant ID)")
			}
			p.AuthURL, p.TokenURL, p.UserinfoURL = oauth.EntraEndpoints(p.APIURL)
		case identity.TypeOkta:
			if p.APIURL == "" {
				return fmt.Errorf("an okta provider needs apiUrl (the okta domain)")
			}
			p.AuthURL, p.TokenURL, p.UserinfoURL = oauth.OktaEndpoints(p.APIURL)
		}
	}

	secret := strings.TrimSpace(req.ClientSecret)
	if secret == "" || secret == secretMask {
		return nil
	}
	if h.cipher == nil {
		return fmt.Errorf("no secret key is configured, secrets cannot be stored")
	}
	enc, err := h.cipher.Encrypt(secret)
	if err != nil {
		return err
	}
	p.ClientSecret = enc
	return nil
}

func (h *AuthProviderHandler) applySAML(r *http.Request, req *authProviderRequest, p *identity.AuthProvider) error {
	p.Type = identity.TypeGeneric
	p.IDPMetadataURL = strings.TrimSpace(req.IDPMetadataURL)
	p.IDPMetadataXML = req.IDPMetadataXML
	p.IDPEntityID = strings.TrimSpace(req.IDPEntityID)
	p.IDPSSOURL = strings.TrimSpace(req.IDPSSOURL)
	p.IDPCert = strings.TrimSpace(req.IDPCert)
	p.SignRequests = req.SignRequests
	p.NameIDFormat = req.NameIDFormat
	p.EmailAttribute = req.EmailAttribute
	p.NameAttribute = req.NameAttribute
	p.GroupsAttribute = req.GroupsAttribute

	if p.IDPMetadataURL != "" || p.IDPMetadataXML != "" {
		md, err := saml.ParseMetadata(r.Context(), p.IDPMetadataURL, p.IDPMetadataXML)
		if err != nil {
			return err
		}
		if p.IDPEntityID == "" {
			p.IDPEntityID = md.EntityID
		}
		if p.IDPSSOURL == "" {
			p.IDPSSOURL = md.SSOURL
		}
		if p.IDPCert == "" {
			p.IDPCert = md.Cert
		}
	}

	if p.SPCert == "" || p.SPKey == "" {
		if h.cipher == nil {
			return fmt.Errorf("no secret key is configured, secrets cannot be stored")
		}
		certPEM, keyPEM, err := saml.GenerateSPKeypair(samlMetadataURL(h.publicURL, p.OrgID, p.Slug))
		if err != nil {
			return err
		}
		enc, err := h.cipher.Encrypt(keyPEM)
		if err != nil {
			return err
		}
		p.SPCert = certPEM
		p.SPKey = enc
	}
	return nil
}
