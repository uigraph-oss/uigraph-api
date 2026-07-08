// Package auth contains HTTP handlers for authentication and authorization APIs.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/uigraph/app/internal/asset"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/identity"
	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/oauth"
	"github.com/uigraph/app/internal/org"
	"github.com/uigraph/app/internal/store"
)

// sessionCookie is the name of the HttpOnly cookie carrying the session token.
const sessionCookie = "uigraph_session"

// stateCookie is the short-lived HttpOnly cookie carrying the OAuth CSRF state.
const stateCookie = "uigraph_oauth_state"

type sessionStore interface {
	identity.SessionStore
	identity.ProviderStore
	identity.ServiceAccountStore
	org.OrgStore
	org.UserStore
	org.MemberStore
}

type SessionHandler struct {
	store       sessionStore
	assets      *asset.Resolver // presigns the avatar URL on /auth/me; may be nil
	publicURL   string          // externally reachable base URL, e.g. https://uigraph.example.com
	frontendURL string          // SPA base URL; empty falls back to publicURL
}

func NewSessionHandler(s sessionStore, assets *asset.Resolver, publicURL, frontendURL string) *SessionHandler {
	return &SessionHandler{
		store:       s,
		assets:      assets,
		publicURL:   strings.TrimRight(publicURL, "/"),
		frontendURL: strings.TrimRight(frontendURL, "/"),
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

type providersResponse struct {
	Providers []providerInfo `json:"providers"`
}

type providerInfo struct {
	Name        string `json:"name"`
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

// ListProviders returns all globally configured OAuth providers.
// GET /api/v1/auth/providers
// @Summary  ListProviders
// @Tags     auth
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /auth/providers [get]
func (h *SessionHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	configs, err := h.store.ListOAuthProviders(r.Context())
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	out := providersResponse{Providers: []providerInfo{}}
	for _, c := range configs {
		label := c.DisplayName
		if label == "" {
			label = c.ProviderName
		}
		out.Providers = append(out.Providers, providerInfo{
			Name:        c.ProviderName,
			DisplayName: label,
			IconURL:     h.avatarURL(r, &c.IconURL),
			LoginURL:    "/api/v1/auth/login/" + c.ProviderName,
		})
	}
	httputil.JSON(w, http.StatusOK, out)
}

// InitiateOAuth redirects the browser to the IdP authorization endpoint.
// GET /api/v1/auth/login/{provider}
// @Summary  InitiateOAuth
// @Tags     auth
// @Param    provider  path  string  true  "provider"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /auth/login/{provider} [get]
func (h *SessionHandler) InitiateOAuth(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetOAuthProvider(r.Context(), r.PathValue("provider"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if cfg == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}
	p := providerFromConfig(cfg)

	state, _, err := generateSessionToken()
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookie,
		Value:    state,
		Path:     "/api/v1/auth",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		Secure:   h.secureCookies(),
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, p.AuthCodeURL(h.redirectURI(cfg.ProviderName), state), http.StatusFound)
}

// OAuthCallback handles the IdP redirect, exchanges the code for an access
// token, reads the userinfo claims, provisions a global user, creates a
// session, sets the session cookie, and redirects to the SPA.
// GET /api/v1/auth/callback/{provider}?code=...&state=...
// @Summary  OAuthCallback
// @Tags     auth
// @Param    provider  path  string  true  "provider"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /auth/callback/{provider} [get]
func (h *SessionHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetOAuthProvider(r.Context(), r.PathValue("provider"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if cfg == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}
	p := providerFromConfig(cfg)

	// Verify CSRF state against the cookie set at initiation, then clear it.
	c, err := r.Cookie(stateCookie)
	if err != nil || c.Value == "" || c.Value != r.URL.Query().Get("state") {
		httputil.BadRequest(w, "invalid OAuth state")
		return
	}
	h.clearCookie(w, stateCookie, "/api/v1/auth")

	code := r.URL.Query().Get("code")
	if code == "" {
		httputil.BadRequest(w, "missing authorization code")
		return
	}

	accessToken, err := oauth.Exchange(r.Context(), p, code, h.redirectURI(cfg.ProviderName))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	claims, err := oauth.FetchUserInfo(r.Context(), p, accessToken)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	email := claimString(claims, claimOr(cfg.EmailClaim, "email"))
	if email == "" {
		httputil.BadRequest(w, "userinfo response has no email claim")
		return
	}
	if !p.EmailAllowed(email) {
		httputil.Forbidden(w)
		return
	}

	u, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if u == nil {
		name := claimString(claims, claimOr(cfg.NameClaim, "name"))
		if name == "" {
			name = email
		}
		u = &org.User{
			ID:    newID(),
			Email: email,
			Name:  name,
			Login: email,
		}
		if err := h.store.CreateUser(r.Context(), *u); err != nil {
			httputil.Error(w, r, err)
			return
		}
		autoJoinOrgs, err := h.store.ListAutoJoinOrgs(r.Context())
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
		for _, o := range autoJoinOrgs {
			err := h.store.AddMember(r.Context(), org.OrgMember{
				UserID: u.ID, OrgID: o.ID, Role: "viewer", Source: "sso",
			})
			if err != nil {
				httputil.Error(w, r, err)
				return
			}
		}
	}
	if u.Disabled {
		httputil.Forbidden(w)
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
		AuthProvider: cfg.ProviderName,
	}
	if err := h.store.CreateSession(r.Context(), sess); err != nil {
		httputil.Error(w, r, err)
		return
	}

	h.setSessionCookie(w, plaintext, sess.ExpiresAt)
	http.Redirect(w, r, h.frontendBase()+"/", http.StatusFound)
}

// SAMLCallback handles the IdP POST to the ACS endpoint.
// POST /api/v1/auth/saml/acs
// @Summary  SAMLCallback
// @Tags     auth
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /auth/saml/acs [post]
func (h *SessionHandler) SAMLCallback(w http.ResponseWriter, r *http.Request) {
	httputil.NotImplemented(w)
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

// redirectURI is the IdP callback URL registered for the given provider.
func (h *SessionHandler) redirectURI(provider string) string {
	return h.publicURL + "/api/v1/auth/callback/" + provider
}

// providerFromConfig builds the runtime OAuth provider from a stored config row.
func providerFromConfig(c *identity.OAuthProviderConfig) oauth.Provider {
	return oauth.Provider{
		Name:           c.ProviderName,
		ClientID:       c.ClientID,
		ClientSecret:   c.ClientSecret,
		AuthURL:        c.AuthURL,
		TokenURL:       c.TokenURL,
		UserinfoURL:    c.UserinfoURL,
		Scopes:         c.Scopes,
		AllowedDomains: c.AllowedDomains,
	}
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
