package api

import (
	"net/http"

	authmw "github.com/uigraph/app/internal/middleware"

	"github.com/uigraph/app/internal/api/admin"
	"github.com/uigraph/app/internal/api/auth"
	"github.com/uigraph/app/internal/api/actor"
	assetapi "github.com/uigraph/app/internal/api/asset"
	catalogapi "github.com/uigraph/app/internal/api/catalog"
	commentapi "github.com/uigraph/app/internal/api/comment"
	"github.com/uigraph/app/internal/api/component"
	"github.com/uigraph/app/internal/api/diagram"
	"github.com/uigraph/app/internal/api/folder"
	"github.com/uigraph/app/internal/api/health"
	mapspkg "github.com/uigraph/app/internal/api/maps"
	"github.com/uigraph/app/internal/asset"
	"github.com/uigraph/app/internal/authz"
	"github.com/uigraph/app/internal/cache"
	"github.com/uigraph/app/internal/config"
	"github.com/uigraph/app/internal/identity"
	"github.com/uigraph/app/internal/storage"
	"github.com/uigraph/app/internal/store"
)

// New builds and returns the root HTTP handler for uigraph-app.
// Routes:
//
//	/healthz                                  — liveness + readiness probes (unauthenticated)
//	/livez                                    — liveness probe
//	/api/v1/auth/*                            — session, OAuth callbacks
//	/api/v1/users/*                           — user management  (requires auth)
//	/api/v1/orgs/*                            — org + nested resources (requires auth)
//	/api/v1/orgs/{orgID}/folders/*            — folder hierarchy
//	/api/v1/orgs/{orgID}/diagrams/*           — diagrams + versions
//	/api/v1/orgs/{orgID}/maps/*               — maps + frames + focal points + canvas
func New(s store.Store, bearer authmw.BearerVerifier, cfg *config.Config, st storage.Client, c cache.Client) http.Handler {
	mux := http.NewServeMux()
	mw := authmw.New(bearer, s)
	authorizer := authz.New(s, s)

	// ── Unauthenticated ───────────────────────────────────────────────────
	healthHandler := &health.Handler{}
	mux.HandleFunc("GET /healthz", healthHandler.Healthz)
	mux.HandleFunc("GET /livez", healthHandler.Livez)

	assetResolver := asset.New(st, c)
	sessionH := auth.NewSessionHandler(s, assetResolver, cfg.PublicURL, cfg.FrontendURL)
	mux.HandleFunc("POST /api/v1/auth/login", sessionH.Login)
	mux.HandleFunc("GET /api/v1/auth/providers", sessionH.ListProviders)
	mux.HandleFunc("GET /api/v1/auth/login/{provider}", sessionH.InitiateOAuth)
	mux.HandleFunc("GET /api/v1/auth/callback/{provider}", sessionH.OAuthCallback)
	mux.HandleFunc("POST /api/v1/auth/saml/acs", sessionH.SAMLCallback)

	// ── Authenticated ─────────────────────────────────────────────────────
	protected := func(method, pattern string, h http.HandlerFunc) {
		mux.Handle(method+" "+pattern, mw.Handler(http.HandlerFunc(h)))
	}

	// requireScope authenticates the request and authorizes it against scope.
	// Service accounts are checked against their granted scopes; users are checked
	// against the scopes resolved from their org role in the {orgID} path segment.
	// The global server_admin axis is intentionally not consulted here.
	requireScope := func(scope authz.Scope, method, pattern string, h http.HandlerFunc) {
		guarded := func(w http.ResponseWriter, r *http.Request) {
			p, ok := authmw.PrincipalFromCtx(r.Context())
			if !ok {
				http.Error(w, `{"error":"unauthenticated","code":401}`, http.StatusUnauthorized)
				return
			}
			var scopes []string
			if p.Kind == identity.PrincipalServiceAccount {
				scopes = p.Scopes
			} else {
				resolved, err := authorizer.ScopesForUser(r.Context(), p.UserID, r.PathValue("orgID"))
				if err != nil {
					http.Error(w, `{"error":"forbidden","code":403}`, http.StatusForbidden)
					return
				}
				scopes = make([]string, len(resolved))
				for i, s := range resolved {
					scopes[i] = string(s)
				}
			}
			if !authz.Has(scopes, scope) {
				http.Error(w, `{"error":"forbidden","code":403}`, http.StatusForbidden)
				return
			}
			h(w, r)
		}
		mux.Handle(method+" "+pattern, mw.Handler(http.HandlerFunc(guarded)))
	}

	// scopeFn bridges the typed authz.Scope requireScope to the plain-string
	// signature expected by the per-domain Register() functions.
	scopeFn := func(scope, method, pattern string, h http.HandlerFunc) {
		requireScope(authz.Scope(scope), method, pattern, h)
	}

	// serverAdmin requires the authenticated principal to hold the global
	// server_admin role. Applied to global user management and SSO configuration.
	serverAdmin := func(method, pattern string, h http.HandlerFunc) {
		guarded := func(w http.ResponseWriter, r *http.Request) {
			p, ok := authmw.PrincipalFromCtx(r.Context())
			if !ok {
				http.Error(w, `{"error":"unauthenticated","code":401}`, http.StatusUnauthorized)
				return
			}
			ok, err := authorizer.IsUserServerAdmin(r.Context(), p.UserID)
			if err != nil || !ok {
				http.Error(w, `{"error":"forbidden","code":403}`, http.StatusForbidden)
				return
			}
			h(w, r)
		}
		mux.Handle(method+" "+pattern, mw.Handler(http.HandlerFunc(guarded)))
	}

	// Session
	protected("POST", "/api/v1/auth/logout", sessionH.Logout)
	protected("GET", "/api/v1/auth/me", sessionH.Me)
	protected("GET", "/api/v1/auth/orgs", sessionH.MyOrgs)

	// Avatars — a user sets their own; a service account's is set by an admin (below).
	avatarH := auth.NewAvatarHandler(s, st, c)
	protected("PUT", "/api/v1/users/me/avatar", avatarH.PutUserAvatar)
	protected("DELETE", "/api/v1/users/me/avatar", avatarH.DeleteUserAvatar)

	// Users (global — server-admin only)
	userH := auth.NewUserHandler(s, c)
	serverAdmin("GET", "/api/v1/users", userH.List)
	serverAdmin("POST", "/api/v1/users", userH.Create)
	serverAdmin("GET", "/api/v1/users/{userID}", userH.Get)
	serverAdmin("PUT", "/api/v1/users/{userID}", userH.Update)
	serverAdmin("DELETE", "/api/v1/users/{userID}", userH.Disable)

	// Orgs
	orgH := auth.NewOrgHandler(s, s, assetResolver)
	protected("GET", "/api/v1/orgs", orgH.List)
	protected("POST", "/api/v1/orgs", orgH.Create)
	protected("GET", "/api/v1/orgs/{orgID}", orgH.Get)
	requireScope(authz.ScopeOrgUpdate, "PUT", "/api/v1/orgs/{orgID}", orgH.Update)
	requireScope(authz.ScopeOrgUpdate, "PUT", "/api/v1/orgs/{orgID}/logo", avatarH.PutOrgLogo)
	requireScope(authz.ScopeOrgUpdate, "DELETE", "/api/v1/orgs/{orgID}/logo", avatarH.DeleteOrgLogo)
	requireScope(authz.ScopeOrgDelete, "DELETE", "/api/v1/orgs/{orgID}", orgH.Delete)

	// Scopes catalog — shared by role assignment and service-account assignment.
	saH := auth.NewServiceAccountHandler(s, c)
	requireScope(authz.ScopeMembersRead, "GET", "/api/v1/orgs/{orgID}/scopes", saH.ListScopes)

	// Members
	memberH := auth.NewMemberHandler(s, s, s)
	requireScope(authz.ScopeMembersRead, "GET", "/api/v1/orgs/{orgID}/members", memberH.List)
	requireScope(authz.ScopeMembersAdd, "POST", "/api/v1/orgs/{orgID}/members", memberH.Add)
	requireScope(authz.ScopeMembersUpdateRole, "PUT", "/api/v1/orgs/{orgID}/members/{userID}", memberH.UpdateMember)
	requireScope(authz.ScopeMembersRemove, "DELETE", "/api/v1/orgs/{orgID}/members/{userID}", memberH.Remove)

	// Teams
	teamH := auth.NewTeamHandler(s)
	requireScope(authz.ScopeTeamsRead, "GET", "/api/v1/orgs/{orgID}/teams", teamH.List)
	requireScope(authz.ScopeTeamsCreate, "POST", "/api/v1/orgs/{orgID}/teams", teamH.Create)
	requireScope(authz.ScopeTeamsRead, "GET", "/api/v1/orgs/{orgID}/teams/{teamID}", teamH.Get)
	requireScope(authz.ScopeTeamsEdit, "PUT", "/api/v1/orgs/{orgID}/teams/{teamID}", teamH.Update)
	requireScope(authz.ScopeTeamsDelete, "DELETE", "/api/v1/orgs/{orgID}/teams/{teamID}", teamH.Delete)
	requireScope(authz.ScopeTeamsRead, "GET", "/api/v1/orgs/{orgID}/teams/{teamID}/members", teamH.ListMembers)
	requireScope(authz.ScopeTeamsAddMember, "POST", "/api/v1/orgs/{orgID}/teams/{teamID}/members", teamH.AddMember)
	requireScope(authz.ScopeTeamsRemoveMember, "DELETE", "/api/v1/orgs/{orgID}/teams/{teamID}/members/{userID}", teamH.RemoveMember)

	// Service accounts
	requireScope(authz.ScopeServiceAccountsRead, "GET", "/api/v1/orgs/{orgID}/service-accounts", saH.List)
	requireScope(authz.ScopeServiceAccountsCreate, "POST", "/api/v1/orgs/{orgID}/service-accounts", saH.Create)
	requireScope(authz.ScopeServiceAccountsRead, "GET", "/api/v1/orgs/{orgID}/service-accounts/{saID}", saH.Get)
	requireScope(authz.ScopeServiceAccountsEdit, "PUT", "/api/v1/orgs/{orgID}/service-accounts/{saID}", saH.Update)
	requireScope(authz.ScopeServiceAccountsEdit, "PUT", "/api/v1/orgs/{orgID}/service-accounts/{saID}/avatar", avatarH.PutServiceAccountAvatar)
	requireScope(authz.ScopeServiceAccountsEdit, "DELETE", "/api/v1/orgs/{orgID}/service-accounts/{saID}/avatar", avatarH.DeleteServiceAccountAvatar)
	requireScope(authz.ScopeServiceAccountsDelete, "DELETE", "/api/v1/orgs/{orgID}/service-accounts/{saID}", saH.Delete)
	requireScope(authz.ScopeServiceAccountsRead, "GET", "/api/v1/orgs/{orgID}/service-accounts/{saID}/tokens", saH.ListTokens)
	requireScope(authz.ScopeServiceAccountsCreateToken, "POST", "/api/v1/orgs/{orgID}/service-accounts/{saID}/tokens", saH.CreateToken)
	requireScope(authz.ScopeServiceAccountsRevokeToken, "DELETE", "/api/v1/orgs/{orgID}/service-accounts/{saID}/tokens/{tokenID}", saH.RevokeToken)

	// Server overview (global — server-admin only)
	adminH := admin.New(s)
	serverAdmin("GET", "/api/v1/server/overview", adminH.Overview)

	// Server org management (global — server-admin only)
	serverAdmin("GET", "/api/v1/server/orgs", orgH.List)
	serverAdmin("POST", "/api/v1/server/orgs", orgH.Create)
	serverAdmin("GET", "/api/v1/server/orgs/{orgID}", orgH.Get)
	serverAdmin("PUT", "/api/v1/server/orgs/{orgID}", orgH.Update)
	serverAdmin("DELETE", "/api/v1/server/orgs/{orgID}", orgH.Delete)
	serverAdmin("PUT", "/api/v1/server/orgs/{orgID}/logo", avatarH.PutOrgLogo)
	serverAdmin("DELETE", "/api/v1/server/orgs/{orgID}/logo", avatarH.DeleteOrgLogo)

	// SSO (global — server-admin only)
	ssoH := auth.NewSSOHandler(s)
	serverAdmin("GET", "/api/v1/sso/oauth", ssoH.ListOAuthProviders)
	serverAdmin("PUT", "/api/v1/sso/oauth/{provider}", ssoH.UpsertOAuthProvider)
	serverAdmin("DELETE", "/api/v1/sso/oauth/{provider}", ssoH.DeleteOAuthProvider)
	serverAdmin("GET", "/api/v1/sso/role-mappings", ssoH.ListMappings)
	serverAdmin("POST", "/api/v1/sso/role-mappings", ssoH.CreateMapping)
	serverAdmin("DELETE", "/api/v1/sso/role-mappings/{mappingID}", ssoH.DeleteMapping)
	serverAdmin("GET", "/api/v1/sso/ldap", ssoH.GetLDAP)
	serverAdmin("PUT", "/api/v1/sso/ldap", ssoH.UpsertLDAP)
	serverAdmin("DELETE", "/api/v1/sso/ldap", ssoH.DeleteLDAP)
	serverAdmin("GET", "/api/v1/sso/saml", ssoH.GetSAML)
	serverAdmin("PUT", "/api/v1/sso/saml", ssoH.UpsertSAML)
	serverAdmin("GET", "/api/v1/sso/scim", ssoH.GetSCIM)
	serverAdmin("PUT", "/api/v1/sso/scim", ssoH.UpsertSCIM)
	serverAdmin("POST", "/api/v1/sso/scim/rotate-token", ssoH.RotateSCIMToken)

	// ── Folders ───────────────────────────────────────────────────────────
	folder.Register(mux, s, scopeFn)

	// ── Actors ────────────────────────────────────────────────────────────
	// Resolves created_by / updated_by / deleted_by ids to public user or
	// service-account info. Available to any authenticated principal.
	actor.Register(mux, s, c, st, protected)

	// ── Assets ────────────────────────────────────────────────────────────
	// Resolves an asset id (preview_asset_id / asset_id / screenshot_asset_id)
	// to a presigned GET URL. Available to any authenticated principal.
	assetapi.Register(mux, st, c, protected)

	// ── Diagrams ──────────────────────────────────────────────────────────
	diagram.Register(mux, s, st, c, scopeFn)

	// ── Component palettes + icons ────────────────────────────────────────
	component.Register(mux, s, st, protected, scopeFn)

	// ── Services + API Groups + API Endpoints ─────────────────────────────
	catalogapi.Register(mux, s, st, scopeFn)

	// ── Maps + Frames + Focal Points + Canvas ─────────────────────────────
	mapspkg.Register(mux, s, st, scopeFn)

	// ── Comments ──────────────────────────────────────────────────────────
	commentapi.Register(mux, s, scopeFn)

	return mux
}
