package api

import (
	"net/http"

	authmw "github.com/uigraph/app/internal/middleware"

	"github.com/uigraph/app/internal/api/auth"
	"github.com/uigraph/app/internal/api/content"
	"github.com/uigraph/app/internal/api/health"
	"github.com/uigraph/app/internal/authz"
	"github.com/uigraph/app/internal/cache"
	"github.com/uigraph/app/internal/config"
	"github.com/uigraph/app/internal/storage"
	"github.com/uigraph/app/internal/store"
)

// New builds and returns the root HTTP handler for uigraph-app.
// Routes:
//
//	/healthz                                  — liveness + readiness probes (unauthenticated)
//	/livez                                    — liveness probe
//	/api/v1/auth/*                            — session, OAuth callbacks, invitation acceptance
//	/api/v1/users/*                           — user management  (requires auth)
//	/api/v1/orgs/*                            — org + nested resources (requires auth)
//	/api/v1/orgs/{orgID}/folders/*            — folder hierarchy
//	/api/v1/orgs/{orgID}/diagrams/*           — diagrams + versions + sync
//	/api/v1/orgs/{orgID}/maps/*               — maps + frames + focal points + canvas
func New(s store.Store, bearer authmw.BearerVerifier, cfg *config.Config, st storage.Client, c cache.Client) http.Handler {
	mux := http.NewServeMux()
	mw := authmw.New(bearer, s)
	authorizer := authz.New(s, s)

	// ── Unauthenticated ───────────────────────────────────────────────────
	healthHandler := &health.Handler{}
	mux.HandleFunc("GET /healthz", healthHandler.Healthz)
	mux.HandleFunc("GET /livez", healthHandler.Livez)

	sessionH := auth.NewSessionHandler(s, cfg.PublicURL, cfg.FrontendURL)
	mux.HandleFunc("POST /api/v1/auth/login", sessionH.Login)
	mux.HandleFunc("GET /api/v1/auth/providers", sessionH.ListProviders)
	mux.HandleFunc("GET /api/v1/auth/login/{provider}", sessionH.InitiateOAuth)
	mux.HandleFunc("GET /api/v1/auth/callback/{provider}", sessionH.OAuthCallback)
	mux.HandleFunc("POST /api/v1/auth/saml/acs", sessionH.SAMLCallback)
	mux.HandleFunc("POST /api/v1/auth/invitations/{code}/accept", sessionH.AcceptInvitation)

	// ── Authenticated ─────────────────────────────────────────────────────
	protected := func(method, pattern string, h http.HandlerFunc) {
		mux.Handle(method+" "+pattern, mw.Handler(http.HandlerFunc(h)))
	}

	// orgAdmin requires the authenticated principal to hold the admin role in the
	// {orgID} path segment, or to be a server admin. Applied to org-scoped admin endpoints.
	orgAdmin := func(method, pattern string, h http.HandlerFunc) {
		guarded := func(w http.ResponseWriter, r *http.Request) {
			p, ok := authmw.PrincipalFromCtx(r.Context())
			if !ok {
				http.Error(w, `{"error":"unauthenticated","code":401}`, http.StatusUnauthorized)
				return
			}
			// server admins bypass org-level checks
			if isAdmin, _ := authorizer.IsUserServerAdmin(r.Context(), p.UserID); isAdmin {
				h(w, r)
				return
			}
			ok, err := authorizer.HasOrgRole(r.Context(), p.UserID, r.PathValue("orgID"), authz.RoleAdmin)
			if err != nil || !ok {
				http.Error(w, `{"error":"forbidden","code":403}`, http.StatusForbidden)
				return
			}
			h(w, r)
		}
		mux.Handle(method+" "+pattern, mw.Handler(http.HandlerFunc(guarded)))
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

	// Users (global — server-admin only)
	userH := auth.NewUserHandler(s)
	serverAdmin("GET", "/api/v1/users", userH.List)
	serverAdmin("POST", "/api/v1/users", userH.Create)
	serverAdmin("GET", "/api/v1/users/{userID}", userH.Get)
	serverAdmin("PUT", "/api/v1/users/{userID}", userH.Update)
	serverAdmin("DELETE", "/api/v1/users/{userID}", userH.Disable)

	// Orgs
	orgH := auth.NewOrgHandler(s, s)
	protected("GET", "/api/v1/orgs", orgH.List)
	protected("POST", "/api/v1/orgs", orgH.Create)
	protected("GET", "/api/v1/orgs/{orgID}", orgH.Get)
	orgAdmin("PUT", "/api/v1/orgs/{orgID}", orgH.Update)
	orgAdmin("DELETE", "/api/v1/orgs/{orgID}", orgH.Delete)

	// Members
	memberH := auth.NewMemberHandler(s)
	protected("GET", "/api/v1/orgs/{orgID}/members", memberH.List)
	orgAdmin("POST", "/api/v1/orgs/{orgID}/members", memberH.Add)
	orgAdmin("PUT", "/api/v1/orgs/{orgID}/members/{userID}", memberH.UpdateRole)
	orgAdmin("DELETE", "/api/v1/orgs/{orgID}/members/{userID}", memberH.Remove)

	// Teams
	teamH := auth.NewTeamHandler(s)
	protected("GET", "/api/v1/orgs/{orgID}/teams", teamH.List)
	orgAdmin("POST", "/api/v1/orgs/{orgID}/teams", teamH.Create)
	protected("GET", "/api/v1/orgs/{orgID}/teams/{teamID}", teamH.Get)
	orgAdmin("PUT", "/api/v1/orgs/{orgID}/teams/{teamID}", teamH.Update)
	orgAdmin("DELETE", "/api/v1/orgs/{orgID}/teams/{teamID}", teamH.Delete)
	protected("GET", "/api/v1/orgs/{orgID}/teams/{teamID}/members", teamH.ListMembers)
	orgAdmin("POST", "/api/v1/orgs/{orgID}/teams/{teamID}/members", teamH.AddMember)
	orgAdmin("DELETE", "/api/v1/orgs/{orgID}/teams/{teamID}/members/{userID}", teamH.RemoveMember)

	// Invitations
	inviteH := auth.NewInvitationHandler(s)
	protected("GET", "/api/v1/orgs/{orgID}/invitations", inviteH.List)
	orgAdmin("POST", "/api/v1/orgs/{orgID}/invitations", inviteH.Create)
	orgAdmin("DELETE", "/api/v1/orgs/{orgID}/invitations/{inviteID}", inviteH.Revoke)
	orgAdmin("POST", "/api/v1/orgs/{orgID}/invitations/{inviteID}/resend", inviteH.Resend)

	// Service accounts
	saH := auth.NewServiceAccountHandler(s)
	protected("GET", "/api/v1/orgs/{orgID}/service-accounts", saH.List)
	orgAdmin("POST", "/api/v1/orgs/{orgID}/service-accounts", saH.Create)
	protected("GET", "/api/v1/orgs/{orgID}/service-accounts/{saID}", saH.Get)
	orgAdmin("PUT", "/api/v1/orgs/{orgID}/service-accounts/{saID}", saH.Update)
	orgAdmin("DELETE", "/api/v1/orgs/{orgID}/service-accounts/{saID}", saH.Delete)
	protected("GET", "/api/v1/orgs/{orgID}/service-accounts/{saID}/tokens", saH.ListTokens)
	orgAdmin("POST", "/api/v1/orgs/{orgID}/service-accounts/{saID}/tokens", saH.CreateToken)
	orgAdmin("DELETE", "/api/v1/orgs/{orgID}/service-accounts/{saID}/tokens/{tokenID}", saH.RevokeToken)

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
	folderH := content.NewFolderHandler(s)
	protected("GET", "/api/v1/orgs/{orgID}/folders", folderH.List)
	protected("POST", "/api/v1/orgs/{orgID}/folders", folderH.Create)
	protected("GET", "/api/v1/orgs/{orgID}/folders/{folderID}", folderH.Get)
	protected("PUT", "/api/v1/orgs/{orgID}/folders/{folderID}", folderH.Update)
	protected("DELETE", "/api/v1/orgs/{orgID}/folders/{folderID}", folderH.Delete)

	// ── Diagrams ──────────────────────────────────────────────────────────
	// NOTE: /sync must be registered before /{diagramID} so the literal path wins.
	diagramH := content.NewDiagramHandler(s, st, c)
	protected("GET", "/api/v1/orgs/{orgID}/diagrams", diagramH.List)
	protected("POST", "/api/v1/orgs/{orgID}/diagrams", diagramH.Create)
	protected("POST", "/api/v1/orgs/{orgID}/diagrams/sync", diagramH.Sync)
	protected("GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}", diagramH.Get)
	protected("PUT", "/api/v1/orgs/{orgID}/diagrams/{diagramID}", diagramH.Update)
	protected("DELETE", "/api/v1/orgs/{orgID}/diagrams/{diagramID}", diagramH.Delete)
	protected("POST", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/thumbnail", diagramH.UpdateThumbnail)
	protected("GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/images", diagramH.ListImages)
	protected("POST", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/images", diagramH.CreateImage)
	protected("GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/content", diagramH.GetContent)
	protected("GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/versions", diagramH.ListVersions)
	protected("POST", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/versions", diagramH.CreateVersion)
	protected("GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/versions/{versionID}/content", diagramH.GetVersionContent)
	protected("POST", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/versions/{versionID}/restore", diagramH.RestoreVersion)

	// ── Flow diagram component palette ────────────────────────────────────
	flowCompH := content.NewFlowComponentHandler()
	protected("GET", "/api/v1/orgs/{orgID}/flow-diagram-components", flowCompH.List)

	// ── Services + API Groups + API Endpoints ─────────────────────────────
	svcH := content.NewServiceHandler(s, st)
	protected("GET", "/api/v1/orgs/{orgID}/services", svcH.List)
	protected("POST", "/api/v1/orgs/{orgID}/services", svcH.Create)
	protected("GET", "/api/v1/orgs/{orgID}/services/{serviceID}", svcH.Get)
	protected("PUT", "/api/v1/orgs/{orgID}/services/{serviceID}", svcH.Update)
	protected("DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}", svcH.Delete)
	// API groups — /sync before /{apiGroupID}
	protected("GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups", svcH.ListAPIGroups)
	protected("POST", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups", svcH.CreateAPIGroup)
	protected("POST", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/sync", svcH.SyncAPIGroup)
	protected("GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}", svcH.GetAPIGroup)
	protected("PUT", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}", svcH.UpdateAPIGroup)
	protected("DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}", svcH.DeleteAPIGroup)
	protected("GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/versions", svcH.ListAPIGroupVersions)
	// API endpoints
	protected("GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints", svcH.ListAPIEndpoints)
	protected("POST", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints", svcH.CreateAPIEndpoint)
	protected("GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints/{endpointID}", svcH.GetAPIEndpoint)
	protected("PUT", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints/{endpointID}", svcH.UpdateAPIEndpoint)
	protected("DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints/{endpointID}", svcH.DeleteAPIEndpoint)

	// ── Maps ──────────────────────────────────────────────────────────────
	mapH := content.NewMapHandler(s)
	protected("GET", "/api/v1/orgs/{orgID}/maps", mapH.List)
	protected("POST", "/api/v1/orgs/{orgID}/maps", mapH.Create)
	protected("GET", "/api/v1/orgs/{orgID}/maps/{mapID}", mapH.Get)
	protected("PUT", "/api/v1/orgs/{orgID}/maps/{mapID}", mapH.Update)
	protected("DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}", mapH.Delete)

	// ── Frames (+ focal points + canvas) ──────────────────────────────────
	// NOTE: /sync must come before /{frameID} so the literal path wins.
	frameH := content.NewFrameHandler(s, st)
	protected("GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames", frameH.List)
	protected("POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames", frameH.Create)
	protected("POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/sync", frameH.Sync)
	protected("GET", "/api/v1/orgs/{orgID}/frames/{frameID}", frameH.Get) // by id, without map scope (UI deep-link)
	protected("GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}", frameH.Get)
	protected("PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}", frameH.Update)
	protected("DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}", frameH.Delete)
	protected("GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points", frameH.ListFocalPoints)
	protected("POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points", frameH.CreateFocalPoint)
	protected("GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}", frameH.GetFocalPoint)
	protected("PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}", frameH.UpdateFocalPoint)
	protected("DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}", frameH.DeleteFocalPoint)
	protected("GET", "/api/v1/orgs/{orgID}/maps/{mapID}/canvas", frameH.GetCanvas)
	protected("PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/canvas", frameH.UpsertCanvas)

	// Frame groups (rectangular regions on a frame).
	protected("GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups", frameH.ListGroups)
	protected("POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups", frameH.CreateGroup)
	protected("PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups/{groupID}", frameH.UpdateGroup)
	protected("DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups/{groupID}", frameH.DeleteGroup)

	// Frame links (frame→frame and frame→map navigation hotspots).
	protected("GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/links", frameH.ListLinks)
	protected("POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/links", frameH.CreateLink)
	protected("PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/links/{linkID}", frameH.UpdateLink)
	protected("DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/links/{linkID}", frameH.DeleteLink)

	// Focal point meta (component metadata attached to a focal point).
	protected("GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta", frameH.ListMeta)
	protected("POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta", frameH.CreateMeta)
	protected("PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta/{metaID}", frameH.UpdateMeta)
	protected("DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta/{metaID}", frameH.DeleteMeta)

	return mux
}
