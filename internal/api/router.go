package api

import (
	"net/http"

	authmw "github.com/uigraph/app/internal/middleware"

	"github.com/uigraph/app/internal/api/actor"
	"github.com/uigraph/app/internal/api/admin"
	docsapi "github.com/uigraph/app/internal/api/apidocs"
	assetapi "github.com/uigraph/app/internal/api/asset"
	"github.com/uigraph/app/internal/api/auth"
	catalogapi "github.com/uigraph/app/internal/api/catalog"
	chatapi "github.com/uigraph/app/internal/api/chat"
	commentapi "github.com/uigraph/app/internal/api/comment"
	"github.com/uigraph/app/internal/api/component"
	"github.com/uigraph/app/internal/api/diagram"
	figmaapi "github.com/uigraph/app/internal/api/figma"
	"github.com/uigraph/app/internal/api/folder"
	"github.com/uigraph/app/internal/api/health"
	mapspkg "github.com/uigraph/app/internal/api/maps"
	mcpusageapi "github.com/uigraph/app/internal/api/mcpusage"
	"github.com/uigraph/app/internal/asset"
	"github.com/uigraph/app/internal/authz"
	"github.com/uigraph/app/internal/cache"
	"github.com/uigraph/app/internal/config"
	"github.com/uigraph/app/internal/crypto"
	"github.com/uigraph/app/internal/figma"
	"github.com/uigraph/app/internal/identity"
	"github.com/uigraph/app/internal/modelpricing"
	"github.com/uigraph/app/internal/queue"
	"github.com/uigraph/app/internal/storage"
	"github.com/uigraph/app/internal/store"
)

func New(s store.Store, bearer authmw.BearerVerifier, cfg *config.Config, st storage.Client, c cache.Client, q *queue.Queue) http.Handler {
	mux := http.NewServeMux()
	mw := authmw.New(bearer, s)
	authorizer := authz.New(s, s)

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

	protected := func(method, pattern string, h http.HandlerFunc) {
		mux.Handle(method+" "+pattern, mw.Handler(http.HandlerFunc(h)))
	}

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

	scopeFn := func(scope, method, pattern string, h http.HandlerFunc) {
		requireScope(authz.Scope(scope), method, pattern, h)
	}

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

	protected("POST", "/api/v1/auth/logout", sessionH.Logout)
	protected("POST", "/api/v1/auth/session-token", sessionH.SessionToken)
	protected("GET", "/api/v1/auth/me", sessionH.Me)
	protected("GET", "/api/v1/auth/orgs", sessionH.MyOrgs)

	avatarH := auth.NewAvatarHandler(s, st, c)
	protected("POST", "/api/v1/users/me/avatar/prepare", avatarH.PrepareUserAvatarUpload)
	protected("PUT", "/api/v1/users/me/avatar", avatarH.PutUserAvatar)
	protected("DELETE", "/api/v1/users/me/avatar", avatarH.DeleteUserAvatar)

	userH := auth.NewUserHandler(s, c, assetResolver)
	serverAdmin("GET", "/api/v1/users", userH.List)
	serverAdmin("POST", "/api/v1/users", userH.Create)
	serverAdmin("GET", "/api/v1/users/{userID}", userH.Get)
	serverAdmin("PUT", "/api/v1/users/{userID}", userH.Update)
	serverAdmin("DELETE", "/api/v1/users/{userID}", userH.Disable)

	orgH := auth.NewOrgHandler(s, s, s, assetResolver)
	protected("GET", "/api/v1/orgs", orgH.List)
	protected("POST", "/api/v1/orgs", orgH.Create)
	protected("GET", "/api/v1/orgs/{orgID}", orgH.Get)
	requireScope(authz.ScopeOrgUpdate, "PUT", "/api/v1/orgs/{orgID}", orgH.Update)
	requireScope(authz.ScopeOrgUpdate, "POST", "/api/v1/orgs/{orgID}/onboarding-complete", orgH.CompleteOnboarding)
	requireScope(authz.ScopeOrgUpdate, "PUT", "/api/v1/orgs/{orgID}/logo", avatarH.PutOrgLogo)
	requireScope(authz.ScopeOrgUpdate, "DELETE", "/api/v1/orgs/{orgID}/logo", avatarH.DeleteOrgLogo)
	requireScope(authz.ScopeOrgDelete, "DELETE", "/api/v1/orgs/{orgID}", orgH.Delete)

	saH := auth.NewServiceAccountHandler(s, c)
	requireScope(authz.ScopeMembersRead, "GET", "/api/v1/orgs/{orgID}/scopes", saH.ListScopes)

	memberH := auth.NewMemberHandler(s, s, s)
	requireScope(authz.ScopeMembersRead, "GET", "/api/v1/orgs/{orgID}/members", memberH.List)
	requireScope(authz.ScopeMembersAdd, "POST", "/api/v1/orgs/{orgID}/members", memberH.Add)
	requireScope(authz.ScopeMembersUpdateRole, "PUT", "/api/v1/orgs/{orgID}/members/{userID}", memberH.UpdateMember)
	requireScope(authz.ScopeMembersRemove, "DELETE", "/api/v1/orgs/{orgID}/members/{userID}", memberH.Remove)

	teamH := auth.NewTeamHandler(s)
	requireScope(authz.ScopeTeamsRead, "GET", "/api/v1/orgs/{orgID}/teams", teamH.List)
	requireScope(authz.ScopeTeamsCreate, "POST", "/api/v1/orgs/{orgID}/teams", teamH.Create)
	requireScope(authz.ScopeTeamsRead, "GET", "/api/v1/orgs/{orgID}/teams/{teamID}", teamH.Get)
	requireScope(authz.ScopeTeamsEdit, "PUT", "/api/v1/orgs/{orgID}/teams/{teamID}", teamH.Update)
	requireScope(authz.ScopeTeamsDelete, "DELETE", "/api/v1/orgs/{orgID}/teams/{teamID}", teamH.Delete)
	requireScope(authz.ScopeTeamsRead, "GET", "/api/v1/orgs/{orgID}/teams/{teamID}/members", teamH.ListMembers)
	requireScope(authz.ScopeTeamsAddMember, "POST", "/api/v1/orgs/{orgID}/teams/{teamID}/members", teamH.AddMember)
	requireScope(authz.ScopeTeamsRemoveMember, "DELETE", "/api/v1/orgs/{orgID}/teams/{teamID}/members/{userID}", teamH.RemoveMember)

	requireScope(authz.ScopeServiceAccountsRead, "GET", "/api/v1/orgs/{orgID}/service-accounts", saH.List)
	requireScope(authz.ScopeServiceAccountsCreate, "POST", "/api/v1/orgs/{orgID}/service-accounts", saH.Create)
	requireScope(authz.ScopeServiceAccountsRead, "GET", "/api/v1/orgs/{orgID}/service-accounts/{saID}", saH.Get)
	requireScope(authz.ScopeServiceAccountsEdit, "PUT", "/api/v1/orgs/{orgID}/service-accounts/{saID}", saH.Update)
	requireScope(authz.ScopeServiceAccountsEdit, "POST", "/api/v1/orgs/{orgID}/service-accounts/{saID}/avatar/prepare", avatarH.PrepareServiceAccountAvatarUpload)
	requireScope(authz.ScopeServiceAccountsEdit, "PUT", "/api/v1/orgs/{orgID}/service-accounts/{saID}/avatar", avatarH.PutServiceAccountAvatar)
	requireScope(authz.ScopeServiceAccountsEdit, "DELETE", "/api/v1/orgs/{orgID}/service-accounts/{saID}/avatar", avatarH.DeleteServiceAccountAvatar)
	requireScope(authz.ScopeServiceAccountsDelete, "DELETE", "/api/v1/orgs/{orgID}/service-accounts/{saID}", saH.Delete)
	requireScope(authz.ScopeServiceAccountsRead, "GET", "/api/v1/orgs/{orgID}/service-accounts/{saID}/tokens", saH.ListTokens)
	requireScope(authz.ScopeServiceAccountsCreateToken, "POST", "/api/v1/orgs/{orgID}/service-accounts/{saID}/tokens", saH.CreateToken)
	requireScope(authz.ScopeServiceAccountsRevokeToken, "DELETE", "/api/v1/orgs/{orgID}/service-accounts/{saID}/tokens/{tokenID}", saH.RevokeToken)

	adminH := admin.New(s, cfg)
	serverAdmin("GET", "/api/v1/server/overview", adminH.Overview)
	serverAdmin("GET", "/api/v1/server/config", adminH.Config)

	serverAdmin("GET", "/api/v1/server/orgs", orgH.List)
	serverAdmin("POST", "/api/v1/server/orgs", orgH.Create)
	serverAdmin("GET", "/api/v1/server/orgs/{orgID}", orgH.Get)
	serverAdmin("PUT", "/api/v1/server/orgs/{orgID}", orgH.Update)
	serverAdmin("DELETE", "/api/v1/server/orgs/{orgID}", orgH.Delete)
	serverAdmin("POST", "/api/v1/server/orgs/{orgID}/logo/prepare", avatarH.PrepareOrgLogoUpload)
	serverAdmin("PUT", "/api/v1/server/orgs/{orgID}/logo", avatarH.PutOrgLogo)
	serverAdmin("DELETE", "/api/v1/server/orgs/{orgID}/logo", avatarH.DeleteOrgLogo)

	ssoH := auth.NewSSOHandler(s, st, assetResolver)
	serverAdmin("GET", "/api/v1/sso/oauth", ssoH.ListOAuthProviders)
	serverAdmin("PUT", "/api/v1/sso/oauth/{provider}", ssoH.UpsertOAuthProvider)
	serverAdmin("DELETE", "/api/v1/sso/oauth/{provider}", ssoH.DeleteOAuthProvider)
	serverAdmin("POST", "/api/v1/sso/oauth/{provider}/icon/prepare", ssoH.PrepareOAuthProviderIconUpload)
	serverAdmin("PUT", "/api/v1/sso/oauth/{provider}/icon", ssoH.PutOAuthProviderIcon)
	serverAdmin("DELETE", "/api/v1/sso/oauth/{provider}/icon", ssoH.DeleteOAuthProviderIcon)
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

	folder.Register(mux, s, scopeFn)

	actor.Register(mux, s, c, st, protected)

	assetapi.Register(mux, st, c, protected)

	diagram.Register(mux, s, st, c, q, scopeFn)

	docsapi.Register(mux, s, st, scopeFn)

	if cipher, err := crypto.NewCipher(cfg.SecretKey); err == nil {
		figmaClient := figma.New(cfg.FigmaClientID, cfg.FigmaClientSecret, cfg.FigmaRedirectURI)
		figmaH := figmaapi.New(s, figmaClient, cipher, cfg.PublicURL, cfg.FrontendURL)
		figmaapi.Register(mux, figmaH, protected)
	}

	component.Register(mux, s, st, protected, scopeFn)

	catalogapi.Register(mux, s, st, q, c, scopeFn)

	mapspkg.Register(mux, s, st, scopeFn)

	chatapi.Register(mux, s, scopeFn)

	commentapi.Register(mux, s, scopeFn)

	pricing := modelpricing.New()
	mcpusageapi.Register(mux, s, pricing, scopeFn)

	return authmw.CORS(mux)
}
