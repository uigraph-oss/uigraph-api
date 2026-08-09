package api

import (
	"net/http"
	"strings"

	authmw "github.com/uigraph/app/internal/middleware"

	"github.com/uigraph/app/internal/api/actor"
	"github.com/uigraph/app/internal/api/admin"
	agentsessionapi "github.com/uigraph/app/internal/api/agentsession"
	docsapi "github.com/uigraph/app/internal/api/apidocs"
	assetapi "github.com/uigraph/app/internal/api/asset"
	"github.com/uigraph/app/internal/api/auth"
	billingapi "github.com/uigraph/app/internal/api/billing"
	catalogapi "github.com/uigraph/app/internal/api/catalog"
	chatapi "github.com/uigraph/app/internal/api/chat"
	commentapi "github.com/uigraph/app/internal/api/comment"
	"github.com/uigraph/app/internal/api/component"
	"github.com/uigraph/app/internal/api/diagram"
	figmaapi "github.com/uigraph/app/internal/api/figma"
	"github.com/uigraph/app/internal/api/folder"
	"github.com/uigraph/app/internal/api/health"
	"github.com/uigraph/app/internal/api/instanceinfo"
	mapspkg "github.com/uigraph/app/internal/api/maps"
	mcpusageapi "github.com/uigraph/app/internal/api/mcpusage"
	mlstudioapi "github.com/uigraph/app/internal/api/mlstudio"
	timelineapi "github.com/uigraph/app/internal/api/timeline"
	"github.com/uigraph/app/internal/asset"
	"github.com/uigraph/app/internal/authz"
	"github.com/uigraph/app/internal/billing"
	awsbilling "github.com/uigraph/app/internal/billing/providers/aws"
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
	mux.HandleFunc("GET /api/v1/instance-info", instanceinfo.New(cfg).Get)

	assetResolver := asset.New(st, c)

	// authCipher stays nil when no secret key is configured; the auth handlers
	// then refuse to read or write a provider secret rather than falling back to
	// plaintext.
	authCipher, err := crypto.NewCipher(cfg.SecretKey)
	if err != nil {
		authCipher = nil
	}

	sessionH := auth.NewSessionHandler(s, assetResolver, authCipher, cfg.PublicURL, cfg.FrontendURL, cfg.CookieDomain)
	mux.HandleFunc("POST /api/v1/auth/login", sessionH.Login)
	mux.HandleFunc("POST /api/v1/auth/discover-orgs", sessionH.DiscoverOrgs)
	mux.HandleFunc("GET /api/v1/auth/orgs/{orgID}/providers", sessionH.OrgProviders)
	mux.HandleFunc("GET /api/v1/auth/orgs/{orgID}/login/{slug}", sessionH.InitiateLogin)
	mux.HandleFunc("GET /api/v1/auth/orgs/{orgID}/callback/{slug}", sessionH.OIDCCallback)
	mux.HandleFunc("POST /api/v1/auth/orgs/{orgID}/saml/{slug}/acs", sessionH.SAMLACS)
	mux.HandleFunc("GET /api/v1/auth/orgs/{orgID}/saml/{slug}/metadata", sessionH.SAMLMetadata)

	if cfg.Enterprise {
		signupH := auth.NewSignupHandler(s, cfg.EnterpriseInternalToken)
		mux.HandleFunc("POST /internal/v1/accounts", signupH.ProvisionAccount)
	}

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
	protected("GET", "/api/v1/orgs", orgH.ListMine)
	protected("POST", "/api/v1/orgs", orgH.Create)
	protected("GET", "/api/v1/orgs/{orgID}", orgH.GetMine)
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

	ssoH := auth.NewSSOHandler(s)
	serverAdmin("GET", "/api/v1/sso/ldap", ssoH.GetLDAP)
	serverAdmin("PUT", "/api/v1/sso/ldap", ssoH.UpsertLDAP)
	serverAdmin("DELETE", "/api/v1/sso/ldap", ssoH.DeleteLDAP)
	serverAdmin("GET", "/api/v1/sso/scim", ssoH.GetSCIM)
	serverAdmin("PUT", "/api/v1/sso/scim", ssoH.UpsertSCIM)
	serverAdmin("POST", "/api/v1/sso/scim/rotate-token", ssoH.RotateSCIMToken)

	authProviderH := auth.NewAuthProviderHandler(s, st, assetResolver, authCipher, cfg.PublicURL)
	protected("GET", "/api/v1/auth/mapping-operators", authProviderH.MappingOperators)
	requireScope(authz.ScopeAuthProvidersRead, "GET", "/api/v1/orgs/{orgID}/auth/providers", authProviderH.ListProviders)
	requireScope(authz.ScopeAuthProvidersWrite, "POST", "/api/v1/orgs/{orgID}/auth/providers", authProviderH.CreateProvider)
	requireScope(authz.ScopeAuthProvidersRead, "GET", "/api/v1/orgs/{orgID}/auth/providers/{slug}", authProviderH.GetProvider)
	requireScope(authz.ScopeAuthProvidersWrite, "PUT", "/api/v1/orgs/{orgID}/auth/providers/{slug}", authProviderH.UpdateProvider)
	requireScope(authz.ScopeAuthProvidersWrite, "DELETE", "/api/v1/orgs/{orgID}/auth/providers/{slug}", authProviderH.DeleteProvider)
	requireScope(authz.ScopeAuthProvidersWrite, "POST", "/api/v1/orgs/{orgID}/auth/providers/{slug}/icon/prepare", authProviderH.PrepareProviderIconUpload)
	requireScope(authz.ScopeAuthProvidersWrite, "PUT", "/api/v1/orgs/{orgID}/auth/providers/{slug}/icon", authProviderH.PutProviderIcon)
	requireScope(authz.ScopeAuthProvidersWrite, "DELETE", "/api/v1/orgs/{orgID}/auth/providers/{slug}/icon", authProviderH.DeleteProviderIcon)
	requireScope(authz.ScopeAuthProvidersRead, "GET", "/api/v1/orgs/{orgID}/auth/providers/{slug}/saml/metadata", authProviderH.SAMLMetadata)
	requireScope(authz.ScopeAuthProvidersRead, "GET", "/api/v1/orgs/{orgID}/auth/providers/{slug}/role-mappings", authProviderH.ListRoleMappings)
	requireScope(authz.ScopeAuthProvidersWrite, "POST", "/api/v1/orgs/{orgID}/auth/providers/{slug}/role-mappings", authProviderH.CreateRoleMapping)
	requireScope(authz.ScopeAuthProvidersWrite, "PUT", "/api/v1/orgs/{orgID}/auth/providers/{slug}/role-mappings/{mappingID}", authProviderH.UpdateRoleMapping)
	requireScope(authz.ScopeAuthProvidersWrite, "DELETE", "/api/v1/orgs/{orgID}/auth/providers/{slug}/role-mappings/{mappingID}", authProviderH.DeleteRoleMapping)
	requireScope(authz.ScopeAuthProvidersRead, "GET", "/api/v1/orgs/{orgID}/auth/domains", authProviderH.ListDomains)
	requireScope(authz.ScopeAuthProvidersWrite, "POST", "/api/v1/orgs/{orgID}/auth/domains", authProviderH.CreateDomain)
	requireScope(authz.ScopeAuthProvidersWrite, "DELETE", "/api/v1/orgs/{orgID}/auth/domains/{domainID}", authProviderH.DeleteDomain)

	folder.Register(mux, s, scopeFn)

	actor.Register(mux, s, c, st, protected)

	assetapi.Register(mux, st, c, protected)

	diagram.Register(mux, s, st, c, q, scopeFn)

	docsapi.Register(mux, s, st, scopeFn)

	figmaRedirect := strings.TrimRight(cfg.PublicURL, "/") + "/api/v1/figma/callback"
	if cipher, err := crypto.NewCipher(cfg.SecretKey); err == nil {
		figmaClient := figma.New(cfg.FigmaClientID, cfg.FigmaClientSecret, figmaRedirect)
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

	agentsessionapi.Register(mux, s, pricing, scopeFn)

	mlstudioapi.Register(mux, s, st, scopeFn)

	if cipher, err := crypto.NewCipher(cfg.SecretKey); err == nil {
		adapters := map[billing.Provider]billing.ProviderAdapter{
			billing.ProviderAWS: awsbilling.New(),
		}
		billingapi.Register(mux, s, cipher, adapters, scopeFn)
	}

	timelineapi.Register(mux, s, scopeFn)

	return authmw.CORS(mux)
}
