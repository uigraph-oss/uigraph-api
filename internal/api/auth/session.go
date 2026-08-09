// Package auth contains HTTP handlers for authentication and authorization APIs.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/uigraph/app/internal/asset"
	"github.com/uigraph/app/internal/crypto"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/identity"
	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/oauth"
	"github.com/uigraph/app/internal/org"
	"github.com/uigraph/app/internal/rolemap"
	"github.com/uigraph/app/internal/saml"
	"github.com/uigraph/app/internal/store"
)

// sessionCookie is the name of the HttpOnly cookie carrying the session token.
const sessionCookie = "uigraph_session"

// loginStateTTL bounds how long a started login may take to come back. It is
// also the window a state row stays replayable in before ConsumeLoginState
// rejects it as expired.
const loginStateTTL = 10 * time.Minute

// discoverWindow and discoverLimit rate-limit org discovery. The endpoint is
// unauthenticated and reveals which orgs claim an email domain, so an address
// may only be probed a bounded number of times per window.
const (
	discoverWindow = 15 * time.Minute
	discoverLimit  = 20
)

type sessionStore interface {
	identity.SessionStore
	identity.ProviderStore
	identity.AuthIdentityStore
	identity.LoginAttemptStore
	identity.ServiceAccountStore
	org.OrgStore
	org.UserStore
	org.MemberStore
}

type SessionHandler struct {
	store        sessionStore
	assets       *asset.Resolver // presigns the avatar URL on /auth/me; may be nil
	cipher       *crypto.Cipher  // decrypts provider client secrets and SP keys; may be nil
	publicURL    string          // externally reachable base URL, e.g. https://uigraph.example.com
	frontendURL  string          // SPA base URL; empty falls back to publicURL
	cookieDomain string          // e.g. ".example.com" to share the session cookie across subdomains; empty = host-only
}

func NewSessionHandler(s sessionStore, assets *asset.Resolver, cipher *crypto.Cipher, publicURL, frontendURL, cookieDomain string) *SessionHandler {
	return &SessionHandler{
		store:        s,
		assets:       assets,
		cipher:       cipher,
		publicURL:    strings.TrimRight(publicURL, "/"),
		frontendURL:  strings.TrimRight(frontendURL, "/"),
		cookieDomain: cookieDomain,
	}
}

// avatarURL presigns the avatar asset id, returning "" when there is no avatar
// or no resolver configured.
func (h *SessionHandler) avatarURL(r *http.Request, assetID *string) string {
	if assetID == nil || *assetID == "" || h.assets == nil {
		return ""
	}
	u, err := h.assets.Resolve(r.Context(), *assetID)
	if err != nil {
		return ""
	}
	return u
}

// frontendBase returns the SPA base URL to redirect to after auth, falling back
// to publicURL when no dedicated frontend URL is configured (same-origin prod).
func (h *SessionHandler) frontendBase() string {
	if h.frontendURL != "" {
		return h.frontendURL
	}
	return h.publicURL
}

// ── Request / Response types ─────────────────────────────────────────────────

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token              string `json:"token"`
	MustChangePassword bool   `json:"mustChangePassword,omitempty"`
}

type meResponse struct {
	UserID       string `json:"userId"`
	OrgID        string `json:"orgId,omitempty"` // service-account binding org; empty for users
	Email        string `json:"email"`
	Name         string `json:"name"`
	Login        string `json:"login"`
	Kind         string `json:"kind"`         // user | service_account
	Role         string `json:"role"`         // global user role (e.g. user | server_admin)
	AuthProvider string `json:"authProvider"` // 'password' or OAuth provider instance name
	AvatarURL    string `json:"avatarUrl,omitempty"`
}

type myOrg struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	LogoURL        string `json:"logoUrl,omitempty"`
	Role           string `json:"role"`
	OnboardingDone bool   `json:"onboardingDone"`
}

type discoverOrgsRequest struct {
	Email string `json:"email"`
}

// discoveredOrg is deliberately minimal: this endpoint is unauthenticated, so it
// exposes only what the org picker has to render.
type discoveredOrg struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	LogoURL string `json:"logoUrl,omitempty"`
}

type providersResponse struct {
	Providers []providerInfo `json:"providers"`
}

type providerInfo struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Kind        string `json:"kind"`
	DisplayName string `json:"displayName"`
	IconURL     string `json:"iconUrl"`
	LoginURL    string `json:"loginUrl"`
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// Login authenticates with email + password and returns a session token.
// POST /api/v1/auth/login
// @Summary  Login
// @Tags     auth
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /auth/login [post]
func (h *SessionHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	if req.Email == "" || req.Password == "" {
		httputil.BadRequest(w, "email and password are required")
		return
	}

	u, err := h.store.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if u == nil || u.PasswordHash == "" {
		httputil.Unauthorized(w)
		return
	}
	if u.Disabled {
		httputil.Forbidden(w)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		httputil.Unauthorized(w)
		return
	}

	plaintext, hash, err := generateSessionToken()
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	sess := identity.Session{
		ID:           newID(),
		UserID:       u.ID,
		TokenHash:    hash,
		UserAgent:    r.UserAgent(),
		ClientIP:     r.RemoteAddr,
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    time.Now().UTC().Add(30 * 24 * time.Hour),
		LastActiveAt: time.Now().UTC(),
		RotatedAt:    time.Now().UTC(),
	}
	if err := h.store.CreateSession(r.Context(), sess); err != nil {
		httputil.Error(w, r, err)
		return
	}

	h.setSessionCookie(w, plaintext, sess.ExpiresAt)

	httputil.JSON(w, http.StatusOK, loginResponse{
		Token:              plaintext,
		MustChangePassword: u.MustChangePassword,
	})
}

// DiscoverOrgs returns the organizations an email address may sign in to: those
// claiming its domain, plus any the address already has a membership in.
//
// It is the first step of login and is unauthenticated, so it always answers 200
// with a possibly empty list — never a 404 that would confirm an address exists.
// POST /api/v1/auth/discover-orgs
// @Summary  DiscoverOrgs
// @Tags     auth
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  400  {object}  httputil.errorBody
// @Failure  429  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /auth/discover-orgs [post]
func (h *SessionHandler) DiscoverOrgs(w http.ResponseWriter, r *http.Request) {
	var req discoverOrgsRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		httputil.BadRequest(w, "a valid email address is required")
		return
	}

	n, err := h.store.CountLoginAttempts(r.Context(), email, time.Now().UTC().Add(-discoverWindow))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if n >= discoverLimit {
		httputil.TooManyRequests(w, "too many attempts, try again later")
		return
	}
	if err := h.store.RecordLoginAttempt(r.Context(), email, r.RemoteAddr); err != nil {
		httputil.Error(w, r, err)
		return
	}

	byDomain, err := h.store.ListOrgsByDomain(r.Context(), email[at+1:])
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	seen := map[string]bool{}
	orgs := []discoveredOrg{}
	for _, o := range byDomain {
		seen[o.ID] = true
		orgs = append(orgs, discoveredOrg{ID: o.ID, Name: o.Name, LogoURL: h.avatarURL(r, o.LogoAssetID)})
	}

	// A user invited into an org that does not claim their domain still has to be
	// able to reach it, so memberships are unioned in.
	u, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if u != nil {
		memberships, err := h.store.ListOrgsForUser(r.Context(), u.ID)
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
		for _, m := range memberships {
			if seen[m.Org.ID] {
				continue
			}
			seen[m.Org.ID] = true
			orgs = append(orgs, discoveredOrg{ID: m.Org.ID, Name: m.Org.Name, LogoURL: h.avatarURL(r, m.Org.LogoAssetID)})
		}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"orgs": orgs})
}

// OrgProviders lists the enabled auth providers of one org, which is what the
// login page renders once an org has been selected. Secrets are never returned.
// GET /api/v1/auth/orgs/{orgID}/providers
// @Summary  OrgProviders
// @Tags     auth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  500  {object}  httputil.errorBody
// @Router   /auth/orgs/{orgID}/providers [get]
func (h *SessionHandler) OrgProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := h.store.ListEnabledAuthProviders(r.Context(), r.PathValue("orgID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	out := providersResponse{Providers: []providerInfo{}}
	for _, p := range providers {
		out.Providers = append(out.Providers, providerInfo{
			ID:          p.ID,
			Slug:        p.Slug,
			Kind:        p.Kind,
			DisplayName: p.DisplayName,
			IconURL:     h.avatarURL(r, &p.IconAssetID),
			LoginURL:    "/api/v1/auth/orgs/" + p.OrgID + "/login/" + p.Slug,
		})
	}
	httputil.JSON(w, http.StatusOK, out)
}

// InitiateLogin records the login handshake and redirects the browser to the
// provider, as an OIDC authorization request or a SAML AuthnRequest.
// GET /api/v1/auth/orgs/{orgID}/login/{slug}
// @Summary  InitiateLogin
// @Tags     auth
// @Param    orgID  path  string  true  "orgID"
// @Param    slug   path  string  true  "slug"
// @Success  302
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /auth/orgs/{orgID}/login/{slug} [get]
func (h *SessionHandler) InitiateLogin(w http.ResponseWriter, r *http.Request) {
	prov, err := h.loadProvider(r)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	state, _, err := generateSessionToken()
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	ls := identity.LoginState{
		ID:         newID(),
		State:      state,
		OrgID:      prov.OrgID,
		ProviderID: prov.ID,
		RedirectTo: safeRedirect(r.URL.Query().Get("redirect_to")),
		CreatedAt:  time.Now().UTC(),
		ExpiresAt:  time.Now().UTC().Add(loginStateTTL),
	}

	var redirect string
	switch prov.Kind {
	case identity.KindOIDC:
		nonce, _, err := generateSessionToken()
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
		ls.Nonce = nonce
		p, err := h.oauthProvider(prov)
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
		redirect = p.AuthCodeURL(oidcRedirectURI(h.publicURL, prov.OrgID, prov.Slug), state, nonce)

	case identity.KindSAML:
		p, err := h.samlProvider(prov)
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
		url, requestID, err := saml.AuthnRequest(p, acsURL(h.publicURL, prov.OrgID, prov.Slug), state)
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
		ls.SAMLRequestID = requestID
		redirect = url

	default:
		httputil.Error(w, r, fmt.Errorf("auth: provider %s has unknown kind %q", prov.Slug, prov.Kind))
		return
	}

	if err := h.store.CreateLoginState(r.Context(), ls); err != nil {
		httputil.Error(w, r, err)
		return
	}
	http.Redirect(w, r, redirect, http.StatusFound)
}

// OIDCCallback handles the provider redirect: it consumes the login state,
// exchanges the code, merges the id_token and userinfo claims, and hands off to
// the shared sign-in step.
// GET /api/v1/auth/orgs/{orgID}/callback/{slug}?code=...&state=...
// @Summary  OIDCCallback
// @Tags     auth
// @Param    orgID  path  string  true  "orgID"
// @Param    slug   path  string  true  "slug"
// @Success  302
// @Failure  400  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /auth/orgs/{orgID}/callback/{slug} [get]
func (h *SessionHandler) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	prov, ls, ok := h.consumeState(w, r, r.URL.Query().Get("state"), identity.KindOIDC)
	if !ok {
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		httputil.BadRequest(w, "missing authorization code")
		return
	}

	p, err := h.oauthProvider(prov)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	tok, err := oauth.Exchange(r.Context(), p, code, oidcRedirectURI(h.publicURL, prov.OrgID, prov.Slug))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	claims, err := oauth.MergedClaims(r.Context(), p, tok, ls.Nonce)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	h.completeLogin(w, r, prov, ls, completedAuth{
		Sub:        claimString(claims, claimOr(prov.SubClaim, "sub")),
		Email:      claimString(claims, claimOr(prov.EmailClaim, "email")),
		Name:       claimString(claims, claimOr(prov.NameClaim, "name")),
		Attributes: claims,
	})
}

// SAMLACS is the assertion consumer service: the IdP POSTs the signed response
// here cross-site, correlated to the pending login by RelayState.
// POST /api/v1/auth/orgs/{orgID}/saml/{slug}/acs
// @Summary  SAMLACS
// @Tags     auth
// @Param    orgID  path  string  true  "orgID"
// @Param    slug   path  string  true  "slug"
// @Param    body  body  object  false  "request body"
// @Success  302
// @Failure  400  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /auth/orgs/{orgID}/saml/{slug}/acs [post]
func (h *SessionHandler) SAMLACS(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		httputil.BadRequest(w, "invalid ACS form")
		return
	}
	prov, ls, ok := h.consumeState(w, r, r.PostForm.Get("RelayState"), identity.KindSAML)
	if !ok {
		return
	}

	p, err := h.samlProvider(prov)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	id, err := saml.ValidateResponse(p, acsURL(h.publicURL, prov.OrgID, prov.Slug), r, ls.SAMLRequestID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	// The NameID is the subject and, for the emailAddress format every IdP
	// console defaults to, also the fallback email when no attribute carries one.
	email := attrString(id.Attributes, claimOr(prov.EmailAttribute, "email"))
	if email == "" {
		email = id.NameID
	}

	h.completeLogin(w, r, prov, ls, completedAuth{
		Sub:        id.NameID,
		Email:      email,
		Name:       attrString(id.Attributes, claimOr(prov.NameAttribute, "displayName")),
		Attributes: id.Attributes,
	})
}

// SAMLMetadata serves the SP metadata document an admin uploads to their IdP.
// GET /api/v1/auth/orgs/{orgID}/saml/{slug}/metadata
// @Summary  SAMLMetadata
// @Tags     auth
// @Param    orgID  path  string  true  "orgID"
// @Param    slug   path  string  true  "slug"
// @Success  200  {string}  string
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /auth/orgs/{orgID}/saml/{slug}/metadata [get]
func (h *SessionHandler) SAMLMetadata(w http.ResponseWriter, r *http.Request) {
	prov, err := h.loadProvider(r)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if prov.Kind != identity.KindSAML {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}

	p, err := h.samlProvider(prov)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	out, err := saml.SPMetadata(p, acsURL(h.publicURL, prov.OrgID, prov.Slug), samlMetadataURL(h.publicURL, prov.OrgID, prov.Slug))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/samlmetadata+xml")
	_, _ = w.Write([]byte(out))
}

// completedAuth is what an OIDC or SAML callback proved about the person at the
// browser. Attributes holds the raw claims or assertion attributes that the
// provider's role mapping rules are evaluated against.
type completedAuth struct {
	Sub        string
	Email      string
	Name       string
	Attributes map[string]any
}

// completeLogin turns a validated authentication into a session. Nothing here
// runs until the IdP response has been verified, which is what keeps the
// promise that no user is created or attached before authentication succeeds.
func (h *SessionHandler) completeLogin(w http.ResponseWriter, r *http.Request, prov *identity.AuthProvider, ls *identity.LoginState, auth completedAuth) {
	slog.InfoContext(r.Context(), "sso identity",
		"provider", prov.Slug,
		"kind", prov.Kind,
		"sub", auth.Sub,
		"email", auth.Email,
		"name", auth.Name,
		"attributeCount", len(auth.Attributes),
	)
	for k, v := range auth.Attributes {
		slog.InfoContext(r.Context(), "sso attribute",
			"provider", prov.Slug,
			"key", k,
			"value", v,
			"type", fmt.Sprintf("%T", v),
		)
	}

	if auth.Email == "" {
		httputil.BadRequest(w, "the provider returned no email address")
		return
	}
	if !prov.EmailAllowed(auth.Email) {
		httputil.Forbidden(w)
		return
	}

	u, err := h.store.GetUserByEmail(r.Context(), auth.Email)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if u == nil && !prov.AllowSignUp {
		httputil.Forbidden(w)
		return
	}
	if u == nil {
		name := auth.Name
		if name == "" {
			name = auth.Email
		}
		u = &org.User{
			ID:    newID(),
			Email: auth.Email,
			Name:  name,
			Login: auth.Email,
		}
		if err := h.store.CreateUser(r.Context(), *u); err != nil {
			httputil.Error(w, r, err)
			return
		}
	}
	if u.Disabled {
		httputil.Forbidden(w)
		return
	}

	rules, err := h.store.ListRoleMappings(r.Context(), prov.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	role, err := rolemap.Resolve(rules, auth.Attributes, prov.DefaultRole)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	slog.InfoContext(r.Context(), "sso role resolved",
		"provider", prov.Slug,
		"email", auth.Email,
		"ruleCount", len(rules),
		"defaultRole", prov.DefaultRole,
		"role", role,
	)
	for _, rule := range rules {
		matched, matchErr := rolemap.Match(rule, auth.Attributes)
		slog.InfoContext(r.Context(), "sso rule",
			"provider", prov.Slug,
			"priority", rule.Priority,
			"attributeKey", rule.AttributeKey,
			"operator", rule.Operator,
			"attributeValue", rule.AttributeValue,
			"grants", rule.Role,
			"matched", matched,
			"matchErr", matchErr,
		)
	}

	// The role is re-resolved on every sign-in, so a change at the IdP takes
	// effect on the next login rather than needing an admin edit here.
	err = h.store.UpsertMember(r.Context(), org.OrgMember{
		UserID: u.ID,
		OrgID:  prov.OrgID,
		Role:   string(role),
		Source: "sso",
	})
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	if auth.Sub != "" {
		err := h.store.UpsertAuthIdentity(r.Context(), identity.AuthIdentity{
			UserID:     u.ID,
			ProviderID: prov.ID,
			Sub:        auth.Sub,
		})
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
	}

	plaintext, hash, err := generateSessionToken()
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	sess := identity.Session{
		ID:           newID(),
		UserID:       u.ID,
		TokenHash:    hash,
		UserAgent:    r.UserAgent(),
		ClientIP:     r.RemoteAddr,
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    time.Now().UTC().Add(30 * 24 * time.Hour),
		LastActiveAt: time.Now().UTC(),
		RotatedAt:    time.Now().UTC(),
		AuthProvider: prov.DisplayName,
	}
	if err := h.store.CreateSession(r.Context(), sess); err != nil {
		httputil.Error(w, r, err)
		return
	}

	h.setSessionCookie(w, plaintext, sess.ExpiresAt)
	http.Redirect(w, r, h.frontendBase()+ls.RedirectTo, http.StatusFound)
}

// Logout deletes the current session.
// POST /api/v1/auth/logout
// @Summary  Logout
// @Tags     auth
// @Security BearerAuth
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /auth/logout [post]
func (h *SessionHandler) Logout(w http.ResponseWriter, r *http.Request) {
	token := extractBearerToken(r)
	if token == "" {
		if c, err := r.Cookie(sessionCookie); err == nil {
			token = c.Value
		}
	}
	h.clearCookie(w, sessionCookie, "/")
	if token == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	hash := identity.Hash(token)
	sess, err := h.store.GetSessionByToken(r.Context(), hash)
	if err != nil || sess == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	_ = h.store.DeleteSession(r.Context(), sess.ID)
	w.WriteHeader(http.StatusNoContent)
}

// Me returns the authenticated principal's profile and org context.
// GET /api/v1/auth/me
// @Summary  Me
// @Tags     auth
// @Security BearerAuth
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /auth/me [get]
func (h *SessionHandler) Me(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	if p.Kind == identity.PrincipalServiceAccount {
		sa, err := h.store.GetServiceAccount(r.Context(), p.ServiceAccountID)
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
		if sa == nil {
			httputil.Error(w, r, store.ErrNotFound)
			return
		}
		httputil.JSON(w, http.StatusOK, meResponse{
			UserID:    sa.ID,
			OrgID:     sa.OrgID,
			Name:      sa.Name,
			Kind:      "service_account",
			AvatarURL: h.avatarURL(r, sa.AvatarAssetID),
		})
		return
	}

	u, err := h.store.GetUser(r.Context(), p.UserID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if u == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}

	httputil.JSON(w, http.StatusOK, meResponse{
		UserID:       p.UserID,
		Email:        u.Email,
		Name:         u.Name,
		Login:        u.Login,
		Kind:         "user",
		Role:         u.Role,
		AuthProvider: p.AuthProvider,
		AvatarURL:    h.avatarURL(r, u.AvatarAssetID),
	})
}

// MyOrgs returns the orgs the authenticated user is a member of, with the
// caller's role in each and which one the session is currently scoped to.
// GET /api/v1/auth/orgs
// @Summary  MyOrgs
// @Tags     auth
// @Security BearerAuth
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /auth/orgs [get]
func (h *SessionHandler) MyOrgs(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	if p.Kind == identity.PrincipalServiceAccount {
		sa, err := h.store.GetServiceAccount(r.Context(), p.ServiceAccountID)
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
		if sa == nil {
			httputil.Error(w, r, store.ErrNotFound)
			return
		}
		o, err := h.store.GetOrg(r.Context(), sa.OrgID)
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
		if o == nil {
			httputil.Error(w, r, store.ErrNotFound)
			return
		}
		httputil.JSON(w, http.StatusOK, map[string]any{"orgs": []myOrg{{
			ID:      o.ID,
			Name:    o.Name,
			LogoURL: h.avatarURL(r, o.LogoAssetID),
		}}})
		return
	}

	if p.Kind != identity.PrincipalUser {
		httputil.Unauthorized(w)
		return
	}
	memberships, err := h.store.ListOrgsForUser(r.Context(), p.UserID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	orgs := make([]myOrg, 0, len(memberships))
	for _, m := range memberships {
		orgs = append(orgs, myOrg{
			ID:             m.Org.ID,
			Name:           m.Org.Name,
			LogoURL:        h.avatarURL(r, m.Org.LogoAssetID),
			Role:           m.Role,
			OnboardingDone: m.Org.OnboardingDone,
		})
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"orgs": orgs})
}

type sessionTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// SessionToken
// @Summary  SessionToken
// @Tags     auth
// @Security BearerAuth
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /auth/session-token [post]
func (h *SessionHandler) SessionToken(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok || p.Kind != identity.PrincipalUser {
		httputil.Unauthorized(w)
		return
	}

	plaintext, hash, err := generateSessionToken()
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	sess := identity.Session{
		ID:           newID(),
		UserID:       p.UserID,
		TokenHash:    hash,
		UserAgent:    r.UserAgent(),
		ClientIP:     r.RemoteAddr,
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    time.Now().UTC().Add(30 * 24 * time.Hour),
		LastActiveAt: time.Now().UTC(),
		RotatedAt:    time.Now().UTC(),
		AuthProvider: p.AuthProvider,
	}
	if err := h.store.CreateSession(r.Context(), sess); err != nil {
		httputil.Error(w, r, err)
		return
	}

	httputil.JSON(w, http.StatusOK, sessionTokenResponse{
		Token:     plaintext,
		ExpiresAt: sess.ExpiresAt,
	})
}

// ── helpers ───────────────────────────────────────────────────────────────────

func generateSessionToken() (plaintext, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate session token: %w", err)
	}
	plaintext = hex.EncodeToString(b)
	hash = identity.Hash(plaintext)
	return plaintext, hash, nil
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return ""
}

// The three URLs an IdP is configured with. They are built here, from the org
// and slug, and never stored: the admin API shows the admin the same strings the
// login path will later assert against, so the two cannot drift apart.

// oidcRedirectURI is the callback URL registered with the provider's IdP.
func oidcRedirectURI(publicURL, orgID, slug string) string {
	return publicURL + "/api/v1/auth/orgs/" + orgID + "/callback/" + slug
}

// acsURL is the assertion consumer service URL the IdP POSTs its response to.
func acsURL(publicURL, orgID, slug string) string {
	return publicURL + "/api/v1/auth/orgs/" + orgID + "/saml/" + slug + "/acs"
}

// samlMetadataURL is where the SP metadata document for a provider is served,
// and doubles as the SP Entity ID.
func samlMetadataURL(publicURL, orgID, slug string) string {
	return publicURL + "/api/v1/auth/orgs/" + orgID + "/saml/" + slug + "/metadata"
}

// loadProvider fetches the enabled provider addressed by the org and slug in the
// URL. A disabled provider is reported as missing so turning one off immediately
// stops it being usable, even by someone holding its login URL.
func (h *SessionHandler) loadProvider(r *http.Request) (*identity.AuthProvider, error) {
	prov, err := h.store.GetAuthProviderBySlug(r.Context(), r.PathValue("orgID"), r.PathValue("slug"))
	if err != nil {
		return nil, err
	}
	if prov == nil || !prov.Enabled {
		return nil, store.ErrNotFound
	}
	return prov, nil
}

// consumeState redeems the single-use login state behind an OIDC state parameter
// or a SAML RelayState and loads the provider it was issued for.
//
// The state row is the whole CSRF defence: it is created server-side at
// initiation and deleted on first use, so a replayed callback finds nothing. The
// provider on the row must also match the one in the URL, otherwise a state
// issued for a cheap provider could be redeemed against a privileged one.
func (h *SessionHandler) consumeState(w http.ResponseWriter, r *http.Request, state, kind string) (*identity.AuthProvider, *identity.LoginState, bool) {
	if state == "" {
		httputil.BadRequest(w, "missing login state")
		return nil, nil, false
	}
	ls, err := h.store.ConsumeLoginState(r.Context(), state)
	if err != nil {
		httputil.Error(w, r, err)
		return nil, nil, false
	}
	if ls == nil {
		httputil.BadRequest(w, "login state is unknown or expired")
		return nil, nil, false
	}
	prov, err := h.loadProvider(r)
	if err != nil {
		httputil.Error(w, r, err)
		return nil, nil, false
	}
	if ls.ProviderID != prov.ID {
		httputil.BadRequest(w, "login state does not belong to this provider")
		return nil, nil, false
	}
	if prov.Kind != kind {
		httputil.BadRequest(w, "login state does not belong to this provider")
		return nil, nil, false
	}
	return prov, ls, true
}

// oauthProvider builds the runtime OIDC provider, decrypting the client secret.
func (h *SessionHandler) oauthProvider(p *identity.AuthProvider) (oauth.Provider, error) {
	secret, err := h.decrypt(p.ClientSecret)
	if err != nil {
		return oauth.Provider{}, err
	}
	return oauth.Provider{
		Name:         p.DisplayName,
		ClientID:     p.ClientID,
		ClientSecret: secret,
		AuthURL:      p.AuthURL,
		TokenURL:     p.TokenURL,
		UserinfoURL:  p.UserinfoURL,
		Scopes:       p.Scopes,
	}, nil
}

// samlProvider builds the runtime SAML provider, decrypting the SP private key.
// The SP Entity ID is derived from the org and slug rather than read off the
// row, so it can never disagree with the URL the metadata is served at.
func (h *SessionHandler) samlProvider(p *identity.AuthProvider) (saml.Provider, error) {
	key, err := h.decrypt(p.SPKey)
	if err != nil {
		return saml.Provider{}, err
	}
	return saml.Provider{
		IDPEntityID:  p.IDPEntityID,
		IDPSSOURL:    p.IDPSSOURL,
		IDPCert:      p.IDPCert,
		SPEntityID:   samlMetadataURL(h.publicURL, p.OrgID, p.Slug),
		SPCert:       p.SPCert,
		SPKey:        key,
		SignRequests: p.SignRequests,
		NameIDFormat: p.NameIDFormat,
	}, nil
}

// decrypt unseals a stored provider secret. An empty value stays empty: SAML
// providers that do not sign their requests have no SP key at all.
func (h *SessionHandler) decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if h.cipher == nil {
		return "", fmt.Errorf("auth: no secret key configured, provider secrets cannot be read")
	}
	return h.cipher.Decrypt(ciphertext)
}

// safeRedirect keeps a post-login redirect on our own origin. Anything that is
// not a plain absolute path — including the "//host" form a browser reads as a
// protocol-relative URL — is discarded in favour of the app root.
func safeRedirect(raw string) string {
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return raw
	}
	return "/"
}

// claimOr returns configured if non-empty, otherwise fallback.
func claimOr(configured, fallback string) string {
	if configured != "" {
		return configured
	}
	return fallback
}

// secureCookies returns true when the public URL is served over HTTPS.
func (h *SessionHandler) secureCookies() bool {
	return strings.HasPrefix(h.publicURL, "https://")
}

func (h *SessionHandler) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Domain:   h.cookieDomain,
		Expires:  expires,
		HttpOnly: true,
		Secure:   h.secureCookies(),
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *SessionHandler) clearCookie(w http.ResponseWriter, name, path string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     path,
		Domain:   h.cookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookies(),
		SameSite: http.SameSiteLaxMode,
	})
}

// claimString extracts a string-valued claim from a userinfo response.
func claimString(claims map[string]any, key string) string {
	if v, ok := claims[key].(string); ok {
		return v
	}
	return ""
}

// attrString extracts a SAML attribute, taking the first value when the IdP sent
// several under one name.
func attrString(attrs map[string]any, key string) string {
	switch v := attrs[key].(type) {
	case string:
		return v
	case []string:
		if len(v) > 0 {
			return v[0]
		}
		return ""
	default:
		return ""
	}
}
