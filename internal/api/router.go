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
	"github.com/uigraph/app/internal/identity"
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

	componentIconH := content.NewComponentIconHandler(st)
	mux.HandleFunc("GET /api/v1/component-icons/{slug}", componentIconH.Get)

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
	userH := auth.NewUserHandler(s, c)
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
	requireScope(authz.ScopeOrgUpdate, "PUT", "/api/v1/orgs/{orgID}", orgH.Update)
	requireScope(authz.ScopeOrgDelete, "DELETE", "/api/v1/orgs/{orgID}", orgH.Delete)

	// Scopes catalog — shared by role assignment and service-account assignment.
	saH := auth.NewServiceAccountHandler(s, c)
	requireScope(authz.ScopeMembersRead, "GET", "/api/v1/orgs/{orgID}/scopes", saH.ListScopes)

	// Members
	memberH := auth.NewMemberHandler(s)
	requireScope(authz.ScopeMembersRead, "GET", "/api/v1/orgs/{orgID}/members", memberH.List)
	requireScope(authz.ScopeMembersAdd, "POST", "/api/v1/orgs/{orgID}/members", memberH.Add)
	requireScope(authz.ScopeMembersUpdateRole, "PUT", "/api/v1/orgs/{orgID}/members/{userID}", memberH.UpdateRole)
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

	// Invitations
	inviteH := auth.NewInvitationHandler(s)
	requireScope(authz.ScopeInvitationsRead, "GET", "/api/v1/orgs/{orgID}/invitations", inviteH.List)
	requireScope(authz.ScopeInvitationsCreate, "POST", "/api/v1/orgs/{orgID}/invitations", inviteH.Create)
	requireScope(authz.ScopeInvitationsRevoke, "DELETE", "/api/v1/orgs/{orgID}/invitations/{inviteID}", inviteH.Revoke)
	requireScope(authz.ScopeInvitationsResend, "POST", "/api/v1/orgs/{orgID}/invitations/{inviteID}/resend", inviteH.Resend)

	// Service accounts
	requireScope(authz.ScopeServiceAccountsRead, "GET", "/api/v1/orgs/{orgID}/service-accounts", saH.List)
	requireScope(authz.ScopeServiceAccountsCreate, "POST", "/api/v1/orgs/{orgID}/service-accounts", saH.Create)
	requireScope(authz.ScopeServiceAccountsRead, "GET", "/api/v1/orgs/{orgID}/service-accounts/{saID}", saH.Get)
	requireScope(authz.ScopeServiceAccountsEdit, "PUT", "/api/v1/orgs/{orgID}/service-accounts/{saID}", saH.Update)
	requireScope(authz.ScopeServiceAccountsDelete, "DELETE", "/api/v1/orgs/{orgID}/service-accounts/{saID}", saH.Delete)
	requireScope(authz.ScopeServiceAccountsRead, "GET", "/api/v1/orgs/{orgID}/service-accounts/{saID}/tokens", saH.ListTokens)
	requireScope(authz.ScopeServiceAccountsCreateToken, "POST", "/api/v1/orgs/{orgID}/service-accounts/{saID}/tokens", saH.CreateToken)
	requireScope(authz.ScopeServiceAccountsRevokeToken, "DELETE", "/api/v1/orgs/{orgID}/service-accounts/{saID}/tokens/{tokenID}", saH.RevokeToken)

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
	requireScope(authz.ScopeFoldersRead, "GET", "/api/v1/orgs/{orgID}/folders", folderH.List)
	requireScope(authz.ScopeFoldersWrite, "POST", "/api/v1/orgs/{orgID}/folders", folderH.Create)
	requireScope(authz.ScopeFoldersRead, "GET", "/api/v1/orgs/{orgID}/folders/{folderID}", folderH.Get)
	requireScope(authz.ScopeFoldersWrite, "PUT", "/api/v1/orgs/{orgID}/folders/{folderID}", folderH.Update)
	requireScope(authz.ScopeFoldersWrite, "DELETE", "/api/v1/orgs/{orgID}/folders/{folderID}", folderH.Delete)

	// ── Actors ────────────────────────────────────────────────────────────
	// Resolves created_by / updated_by / deleted_by ids to public user or
	// service-account info. Available to any authenticated principal.
	actorH := content.NewActorHandler(s, c)
	protected("GET", "/api/v1/orgs/{orgID}/actors", actorH.Resolve)

	// ── Assets ────────────────────────────────────────────────────────────
	// Resolves an asset id (preview_asset_id / asset_id / screenshot_asset_id)
	// to a presigned GET URL. Available to any authenticated principal.
	assetH := content.NewAssetHandler(st, c)
	protected("GET", "/api/v1/orgs/{orgID}/assets/urls", assetH.Resolve)

	// ── Diagrams ──────────────────────────────────────────────────────────
	diagramH := content.NewDiagramHandler(s, st, c)
	requireScope(authz.ScopeDiagramsRead, "GET", "/api/v1/orgs/{orgID}/diagrams", diagramH.List)
	requireScope(authz.ScopeDiagramsWrite, "POST", "/api/v1/orgs/{orgID}/diagrams", diagramH.Create)
	requireScope(authz.ScopeDiagramsWrite, "POST", "/api/v1/orgs/{orgID}/diagrams/sync", diagramH.Sync)
	requireScope(authz.ScopeDiagramsRead, "GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}", diagramH.Get)
	requireScope(authz.ScopeDiagramsWrite, "PUT", "/api/v1/orgs/{orgID}/diagrams/{diagramID}", diagramH.Update)
	requireScope(authz.ScopeDiagramsWrite, "DELETE", "/api/v1/orgs/{orgID}/diagrams/{diagramID}", diagramH.Delete)
	requireScope(authz.ScopeDiagramsWrite, "POST", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/thumbnail", diagramH.UpdateThumbnail)
	requireScope(authz.ScopeDiagramsRead, "GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/images", diagramH.ListImages)
	requireScope(authz.ScopeDiagramsWrite, "POST", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/images", diagramH.CreateImage)
	requireScope(authz.ScopeDiagramsRead, "GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/content", diagramH.GetContent)
	requireScope(authz.ScopeDiagramsRead, "GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/versions", diagramH.ListVersions)
	requireScope(authz.ScopeDiagramsWrite, "POST", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/versions", diagramH.CreateVersion)
	requireScope(authz.ScopeDiagramsRead, "GET", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/versions/{versionID}/content", diagramH.GetVersionContent)
	requireScope(authz.ScopeDiagramsWrite, "POST", "/api/v1/orgs/{orgID}/diagrams/{diagramID}/versions/{versionID}/restore", diagramH.RestoreVersion)

	// ── Focal point component palette ─────────────────────────────────────
	componentH := content.NewComponentHandler(s)
	protected("GET", "/api/v1/orgs/{orgID}/components", componentH.List)

	// ── Flow diagram component palette ────────────────────────────────────
	flowCompH := content.NewFlowComponentHandler(s)
	protected("GET", "/api/v1/orgs/{orgID}/flow-diagram-components", flowCompH.List)

	// ── Services + API Groups + API Endpoints ─────────────────────────────
	svcH := content.NewServiceHandler(s, st)
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services", svcH.List)
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services/stats", svcH.ListStats)
	requireScope(authz.ScopeServicesWrite, "POST", "/api/v1/orgs/{orgID}/services", svcH.Create)
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services/{serviceID}", svcH.Get)
	requireScope(authz.ScopeServicesWrite, "PUT", "/api/v1/orgs/{orgID}/services/{serviceID}", svcH.Update)
	requireScope(authz.ScopeServicesWrite, "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}", svcH.Delete)
	// API groups — /sync before /{apiGroupID}
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups", svcH.ListAPIGroups)
	requireScope(authz.ScopeServicesWrite, "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups", svcH.CreateAPIGroup)
	requireScope(authz.ScopeServicesWrite, "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/sync", svcH.SyncAPIGroup)
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}", svcH.GetAPIGroup)
	requireScope(authz.ScopeServicesWrite, "PUT", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}", svcH.UpdateAPIGroup)
	requireScope(authz.ScopeServicesWrite, "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}", svcH.DeleteAPIGroup)
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/versions", svcH.ListAPIGroupVersions)
	// API endpoints
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints", svcH.ListAPIEndpoints)
	requireScope(authz.ScopeServicesWrite, "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints", svcH.CreateAPIEndpoint)
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints/{endpointID}", svcH.GetAPIEndpoint)
	requireScope(authz.ScopeServicesWrite, "PUT", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints/{endpointID}", svcH.UpdateAPIEndpoint)
	requireScope(authz.ScopeServicesWrite, "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/api-groups/{apiGroupID}/endpoints/{endpointID}", svcH.DeleteAPIEndpoint)
	// Service docs
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/docs", svcH.ListDocs)
	requireScope(authz.ScopeServicesWrite, "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/docs", svcH.CreateDoc)
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/docs/{docID}", svcH.GetDoc)
	requireScope(authz.ScopeServicesWrite, "PUT", "/api/v1/orgs/{orgID}/services/{serviceID}/docs/{docID}", svcH.UpdateDoc)
	requireScope(authz.ScopeServicesWrite, "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/docs/{docID}", svcH.DeleteDoc)
	// Service diagrams
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/diagrams", svcH.ListDiagrams)
	requireScope(authz.ScopeServicesWrite, "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/diagrams", svcH.CreateDiagram)
	requireScope(authz.ScopeServicesWrite, "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/diagrams/{diagramID}", svcH.DeleteDiagram)
	// Service DBs
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs", svcH.ListDBs)
	requireScope(authz.ScopeServicesWrite, "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs", svcH.CreateDB)
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}", svcH.GetDB)
	requireScope(authz.ScopeServicesWrite, "PUT", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}", svcH.UpdateDB)
	requireScope(authz.ScopeServicesWrite, "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}", svcH.DeleteDB)
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}/versions", svcH.ListDBVersions)
	requireScope(authz.ScopeServicesWrite, "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}/versions", svcH.CreateDBVersion)
	requireScope(authz.ScopeServicesWrite, "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/dbs/{dbID}/versions/{versionID}/restore", svcH.RestoreDBVersion)
	// Service tests
	requireScope(authz.ScopeServicesWrite, "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-pack", svcH.CreateTestPack)
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/test-packs", svcH.ListTestPacks)
	requireScope(authz.ScopeServicesWrite, "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-pack/{testPackID}", svcH.UpdateTestPack)
	requireScope(authz.ScopeServicesWrite, "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/test-pack/{testPackID}", svcH.DeleteTestPack)
	requireScope(authz.ScopeServicesWrite, "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-case", svcH.CreateTestCase)
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/test-cases", svcH.ListTestCases)
	requireScope(authz.ScopeServicesWrite, "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-case/{testCaseID}", svcH.UpdateTestCase)
	requireScope(authz.ScopeServicesWrite, "DELETE", "/api/v1/orgs/{orgID}/services/{serviceID}/test-case/{testCaseID}", svcH.DeleteTestCase)
	requireScope(authz.ScopeServicesWrite, "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-run", svcH.CreateTestRun)
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/test-runs", svcH.ListTestRuns)
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/test-runs-summary", svcH.ListTestRunsSummary)
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/test-run/{testRunID}", svcH.GetTestRun)
	requireScope(authz.ScopeServicesWrite, "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-run/{testRunID}", svcH.UpdateTestRun)
	requireScope(authz.ScopeServicesWrite, "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-run-result", svcH.CreateTestRunResult)
	requireScope(authz.ScopeServicesRead, "GET", "/api/v1/orgs/{orgID}/services/{serviceID}/test-run-results", svcH.ListTestRunResults)
	requireScope(authz.ScopeServicesWrite, "POST", "/api/v1/orgs/{orgID}/services/{serviceID}/test-run-result/{testRunResultID}", svcH.UpdateTestRunResult)

	// ── Maps ──────────────────────────────────────────────────────────────
	mapH := content.NewMapHandler(s)
	requireScope(authz.ScopeMapsRead, "GET", "/api/v1/orgs/{orgID}/maps", mapH.List)
	requireScope(authz.ScopeMapsWrite, "POST", "/api/v1/orgs/{orgID}/maps", mapH.Create)
	requireScope(authz.ScopeMapsRead, "GET", "/api/v1/orgs/{orgID}/maps/{mapID}", mapH.Get)
	requireScope(authz.ScopeMapsWrite, "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}", mapH.Update)
	requireScope(authz.ScopeMapsWrite, "DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}", mapH.Delete)

	// ── Frames (+ focal points + canvas) ──────────────────────────────────
	frameH := content.NewFrameHandler(s, st)
	requireScope(authz.ScopeMapsRead, "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames", frameH.List)
	requireScope(authz.ScopeMapsWrite, "POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames", frameH.Create)
	requireScope(authz.ScopeMapsWrite, "POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/sync", frameH.Sync)
	requireScope(authz.ScopeMapsRead, "GET", "/api/v1/orgs/{orgID}/frames/{frameID}", frameH.Get)
	requireScope(authz.ScopeMapsRead, "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}", frameH.Get)
	requireScope(authz.ScopeMapsWrite, "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}", frameH.Update)
	requireScope(authz.ScopeMapsWrite, "DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}", frameH.Delete)
	requireScope(authz.ScopeMapsRead, "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points", frameH.ListFocalPoints)
	requireScope(authz.ScopeMapsWrite, "POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points", frameH.CreateFocalPoint)
	requireScope(authz.ScopeMapsRead, "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}", frameH.GetFocalPoint)
	requireScope(authz.ScopeMapsWrite, "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}", frameH.UpdateFocalPoint)
	requireScope(authz.ScopeMapsWrite, "DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}", frameH.DeleteFocalPoint)
	requireScope(authz.ScopeMapsRead, "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/canvas", frameH.GetCanvas)
	requireScope(authz.ScopeMapsWrite, "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/canvas", frameH.UpsertCanvas)

	requireScope(authz.ScopeMapsRead, "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups", frameH.ListGroups)
	requireScope(authz.ScopeMapsWrite, "POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups", frameH.CreateGroup)
	requireScope(authz.ScopeMapsWrite, "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups/{groupID}", frameH.UpdateGroup)
	requireScope(authz.ScopeMapsWrite, "DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/groups/{groupID}", frameH.DeleteGroup)

	requireScope(authz.ScopeMapsRead, "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/links", frameH.ListLinks)
	requireScope(authz.ScopeMapsWrite, "POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/links", frameH.CreateLink)
	requireScope(authz.ScopeMapsWrite, "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/links/{linkID}", frameH.UpdateLink)
	requireScope(authz.ScopeMapsWrite, "DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/links/{linkID}", frameH.DeleteLink)

	requireScope(authz.ScopeMapsRead, "GET", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta", frameH.ListMeta)
	requireScope(authz.ScopeMapsWrite, "POST", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta", frameH.CreateMeta)
	requireScope(authz.ScopeMapsWrite, "PUT", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta/{metaID}", frameH.UpdateMeta)
	requireScope(authz.ScopeMapsWrite, "DELETE", "/api/v1/orgs/{orgID}/maps/{mapID}/frames/{frameID}/focal-points/{fpID}/meta/{metaID}", frameH.DeleteMeta)

	return mux
}
