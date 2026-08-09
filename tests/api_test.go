// Package tests contains end-to-end HTTP tests against a real Postgres instance.
// Tests share a single httptest.Server created in TestMain.
//
// Required env (defaults work with docker-compose.dev.yml):
//
//	TEST_POSTGRES_URL  postgres://uigraph:devpassword@localhost:5432/uigraph?sslmode=disable
package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/uigraph/app/internal/api"
	"github.com/uigraph/app/internal/authz"
	"github.com/uigraph/app/internal/bootstrap"
	"github.com/uigraph/app/internal/config"
	"github.com/uigraph/app/internal/crypto"
	"github.com/uigraph/app/internal/identity"
	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/migrate"
	"github.com/uigraph/app/internal/store/postgres"
)

// ── shared state ──────────────────────────────────────────────────────────────

var (
	srv            *httptest.Server
	idp            *httptest.Server
	adminToken     string
	orgID          string
	oidcProviderID string
	testDB         *postgres.DB
)

const oidcProviderSlug = "seeded-oidc"

const (
	ssoUserEmail  = "sso-user@example.com"
	ssoUserDomain = "example.com"
	testSecretKey = "test-secret-key"
)

func TestMain(m *testing.M) {
	pgURL := os.Getenv("TEST_POSTGRES_URL")
	if pgURL == "" {
		pgURL = "postgres://uigraph:devpassword@localhost:5432/uigraph?sslmode=disable"
	}

	db, err := postgres.Open(pgURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SKIP: postgres unavailable (%v)\n", err)
		os.Exit(0)
	}
	defer func() { _ = db.Close() }()
	testDB = db

	ctx := context.Background()
	if err := migrate.Run(ctx, db.DB()); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: migrate: %v\n", err)
		os.Exit(1)
	}

	cfg := &config.Config{AdminEmail: "admin@localhost", AdminPassword: "admin"}
	if err := bootstrap.Run(ctx, db, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: bootstrap: %v\n", err)
		os.Exit(1)
	}

	// Fake OIDC provider: returns a static access token and userinfo claims.
	idp = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			_, _ = w.Write([]byte(`{"access_token":"fake-access-token","token_type":"Bearer"}`))
		case "/userinfo":
			_, _ = fmt.Fprintf(w, `{"email":%q,"name":"SSO User","sub":"sub-123"}`, ssoUserEmail)
		default:
			http.NotFound(w, r)
		}
	}))
	defer idp.Close()

	srv = httptest.NewServer(api.New(db, authmw.NewSessionVerifier(db, db), &config.Config{PublicURL: "http://localhost:8080", FrontendURL: "", SecretKey: testSecretKey}, testStorage, nil, nil))
	defer srv.Close()

	// obtain admin token once for all tests
	resp := mustDo(t_("TestMain"), "POST", "/api/v1/auth/login", "", M{
		"email": "admin@localhost", "password": "admin",
	})
	adminToken = str(resp, "token")

	// Ensure admin is server_admin and create a test org + membership.
	u, _ := db.GetUserByEmail(ctx, cfg.AdminEmail)
	if u != nil {
		u.Role = "server_admin"
		_ = db.UpdateUser(ctx, *u)
	}

	org := mustDo(t_("TestMain"), "POST", "/api/v1/orgs", adminToken, M{"name": "Test Org"})
	orgID = str(org, "id")

	if err := db.UpsertOrgMember(ctx, u.ID, orgID, authz.RoleAdmin, "test"); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: add member: %v\n", err)
		os.Exit(1)
	}

	if err := db.CreateOrgDomain(ctx, identity.OrgDomain{ID: uuid.NewString(), OrgID: orgID, Domain: ssoUserDomain}); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: seed domain: %v\n", err)
		os.Exit(1)
	}

	cipher, err := crypto.NewCipher(testSecretKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: cipher: %v\n", err)
		os.Exit(1)
	}
	clientSecret, err := cipher.Encrypt("test-secret")
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: encrypt: %v\n", err)
		os.Exit(1)
	}

	oidcProviderID = uuid.NewString()
	if err := db.CreateAuthProvider(ctx, identity.AuthProvider{
		ID:           oidcProviderID,
		Slug:         oidcProviderSlug,
		OrgID:        orgID,
		Kind:         identity.KindOIDC,
		Type:         identity.TypeGeneric,
		DisplayName:  "Generic OIDC",
		Enabled:      true,
		AllowSignUp:  true,
		DefaultRole:  authz.RoleViewer,
		ClientID:     "test-client",
		ClientSecret: clientSecret,
		AuthURL:      idp.URL + "/authorize",
		TokenURL:     idp.URL + "/token",
		UserinfoURL:  idp.URL + "/userinfo",
		Scopes:       "openid profile email",
		EmailClaim:   "email",
		NameClaim:    "name",
		SubClaim:     "sub",
		GroupsClaim:  "groups",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: seed provider: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// ── auth ──────────────────────────────────────────────────────────────────────

func TestLogin_Success(t *testing.T) {
	body := mustDo(t, "POST", "/api/v1/auth/login", "", M{
		"email": "admin@localhost", "password": "admin",
	})
	if body["token"] == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	r := do("POST", "/api/v1/auth/login", "", M{"email": "admin@localhost", "password": "wrong"})
	if r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", r.StatusCode)
	}
}

func TestLogin_UnknownUser(t *testing.T) {
	r := do("POST", "/api/v1/auth/login", "", M{"email": "nobody@example.com", "password": "x"})
	if r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", r.StatusCode)
	}
}

func TestMe_Authenticated(t *testing.T) {
	body := mustDo(t, "GET", "/api/v1/auth/me", adminToken, nil)
	if body["email"] != "admin@localhost" {
		t.Fatalf("want admin@localhost, got %v", body["email"])
	}
	if body["kind"] != "user" {
		t.Fatalf("want kind=user, got %v", body["kind"])
	}
}

func TestMe_Unauthenticated(t *testing.T) {
	r := do("GET", "/api/v1/auth/me", "", nil)
	if r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", r.StatusCode)
	}
}

func TestLogout(t *testing.T) {
	// login → logout → me should fail
	body := mustDo(t, "POST", "/api/v1/auth/login", "", M{
		"email": "admin@localhost", "password": "admin",
	})
	tok := str(body, "token")

	r := do("POST", "/api/v1/auth/logout", tok, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204 from logout, got %d", r.StatusCode)
	}

	r = do("GET", "/api/v1/auth/me", tok, nil)
	if r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401 after logout, got %d", r.StatusCode)
	}
}

func TestDiscoverOrgs_ByDomain(t *testing.T) {
	body := mustDo(t, "POST", "/api/v1/auth/discover-orgs", "", M{"email": "someone@" + ssoUserDomain})
	orgs := list(body, "orgs")
	found := false
	for _, o := range orgs {
		if str(o, "id") == orgID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected org %s in %v", orgID, orgs)
	}
}

func TestDiscoverOrgs_RejectsInvalidEmail(t *testing.T) {
	r := do("POST", "/api/v1/auth/discover-orgs", "", M{"email": "not-an-email"})
	if r.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", r.StatusCode)
	}
}

func TestOrgProviders(t *testing.T) {
	body := mustDo(t, "GET", "/api/v1/auth/orgs/"+orgID+"/providers", "", nil)
	providers := list(body, "providers")
	found := false
	for _, p := range providers {
		if str(p, "id") == oidcProviderID && str(p, "kind") == "oidc" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected provider %s in %v", oidcProviderID, providers)
	}
}

func newLoginState(t *testing.T) string {
	t.Helper()
	state := uuid.NewString()
	err := testDB.CreateLoginState(context.Background(), identity.LoginState{
		ID:         uuid.NewString(),
		State:      state,
		OrgID:      orgID,
		ProviderID: oidcProviderID,
		CreatedAt:  time.Now().UTC(),
		ExpiresAt:  time.Now().UTC().Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create login state: %v", err)
	}
	return state
}

func TestOIDC_CallbackProvisionsUserAndSession(t *testing.T) {
	// Don't follow the post-login redirect; we want the 302 + Set-Cookie.
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	url := srv.URL + "/api/v1/auth/orgs/" + orgID + "/callback/" + oidcProviderSlug + "?code=abc&state=" + newLoginState(t)
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("callback request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("want 302 from callback, got %d", resp.StatusCode)
	}

	var sessionTok string
	for _, c := range resp.Cookies() {
		if c.Name == "uigraph_session" {
			sessionTok = c.Value
		}
	}
	if sessionTok == "" {
		t.Fatal("expected uigraph_session cookie to be set")
	}

	// The session cookie should authenticate /me as the provisioned SSO user.
	meReq, _ := http.NewRequest("GET", srv.URL+"/api/v1/auth/me", nil)
	meReq.AddCookie(&http.Cookie{Name: "uigraph_session", Value: sessionTok})
	meResp, err := http.DefaultClient.Do(meReq)
	if err != nil {
		t.Fatalf("me request: %v", err)
	}
	defer func() { _ = meResp.Body.Close() }()
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 from /me, got %d", meResp.StatusCode)
	}
	var me M
	_ = json.NewDecoder(meResp.Body).Decode(&me)
	if me["email"] != ssoUserEmail {
		t.Fatalf("want email %q, got %v", ssoUserEmail, me["email"])
	}

	u, err := testDB.GetUserByEmail(context.Background(), ssoUserEmail)
	if err != nil || u == nil {
		t.Fatalf("get sso user: %v", err)
	}
	m, err := testDB.GetMember(context.Background(), u.ID, orgID)
	if err != nil || m == nil {
		t.Fatalf("get member: %v", err)
	}
	if m.Role != string(authz.RoleViewer) {
		t.Fatalf("want viewer membership, got %q", m.Role)
	}
}

func TestOIDC_CallbackRejectsUnknownState(t *testing.T) {
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	url := srv.URL + "/api/v1/auth/orgs/" + orgID + "/callback/" + oidcProviderSlug + "?code=abc&state=" + uuid.NewString()
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("callback request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 on unknown state, got %d", resp.StatusCode)
	}
}

func TestOIDC_CallbackRejectsReplayedState(t *testing.T) {
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	url := srv.URL + "/api/v1/auth/orgs/" + orgID + "/callback/" + oidcProviderSlug + "?code=abc&state=" + newLoginState(t)

	first, err := client.Get(url)
	if err != nil {
		t.Fatalf("first callback: %v", err)
	}
	_ = first.Body.Close()
	if first.StatusCode != http.StatusFound {
		t.Fatalf("want 302 on first use, got %d", first.StatusCode)
	}

	second, err := client.Get(url)
	if err != nil {
		t.Fatalf("second callback: %v", err)
	}
	defer func() { _ = second.Body.Close() }()
	if second.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 on replay, got %d", second.StatusCode)
	}
}

func TestOIDC_CallbackRejectsStateFromAnotherProvider(t *testing.T) {
	slug := fmt.Sprintf("other-oidc-%d", time.Now().UnixNano())
	mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/auth/providers", adminToken, M{
		"slug": slug,
		"kind": "oidc", "type": "generic", "displayName": "Other OIDC " + slug,
		"enabled": true, "allowSignUp": true, "defaultRole": "admin",
		"clientId": "abc", "clientSecret": "shh",
		"authUrl": idp.URL + "/authorize", "tokenUrl": idp.URL + "/token",
		"userinfoUrl": idp.URL + "/userinfo",
	})
	defer func() { _ = do("DELETE", "/api/v1/orgs/"+orgID+"/auth/providers/"+slug, adminToken, nil) }()

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	url := srv.URL + "/api/v1/auth/orgs/" + orgID + "/callback/" + slug + "?code=abc&state=" + newLoginState(t)
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("callback request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 for a state from another provider, got %d", resp.StatusCode)
	}
}

// ── health ────────────────────────────────────────────────────────────────────

func TestHealthz(t *testing.T) {
	r := do("GET", "/healthz", "", nil)
	if r.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", r.StatusCode)
	}
}

// ── orgs ──────────────────────────────────────────────────────────────────────

func TestOrgs_List(t *testing.T) {
	body := mustDo(t, "GET", "/api/v1/orgs", adminToken, nil)
	orgs := list(body, "orgs")
	if len(orgs) == 0 {
		t.Fatal("expected at least one org")
	}
}

func TestOrgs_CRUD(t *testing.T) {
	// create
	created := mustDo(t, "POST", "/api/v1/orgs", adminToken, M{"name": "Test Org"})
	id := str(created, "id")
	if id == "" {
		t.Fatal("expected id in response")
	}

	// get
	got := mustDo(t, "GET", "/api/v1/orgs/"+id, adminToken, nil)
	if got["name"] != "Test Org" {
		t.Fatalf("want name %q, got %v", "Test Org", got["name"])
	}

	// update
	updated := mustDo(t, "PUT", "/api/v1/orgs/"+id, adminToken, M{"name": "Updated Org"})
	if updated["name"] != "Updated Org" {
		t.Fatalf("want updated name, got %v", updated["name"])
	}

	// delete
	r := do("DELETE", "/api/v1/orgs/"+id, adminToken, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", r.StatusCode)
	}

	// confirm gone
	r = do("GET", "/api/v1/orgs/"+id, adminToken, nil)
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 after delete, got %d", r.StatusCode)
	}
}

// ── members ───────────────────────────────────────────────────────────────────

func TestMembers_List(t *testing.T) {
	body := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/members", adminToken, nil)
	members := list(body, "members")
	if len(members) == 0 {
		t.Fatal("expected at least one member (admin)")
	}
}

// ── teams ─────────────────────────────────────────────────────────────────────

func TestTeams_CRUD(t *testing.T) {
	name := fmt.Sprintf("team-%d", time.Now().UnixNano())

	// create
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/teams", adminToken, M{"name": name})
	teamID := str(created, "id")
	if teamID == "" {
		t.Fatal("expected id in response")
	}

	// list — should contain new team
	body := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/teams", adminToken, nil)
	found := false
	for _, tm := range list(body, "teams") {
		if str(tm, "id") == teamID {
			found = true
		}
	}
	if !found {
		t.Fatal("new team not found in list")
	}

	// get
	got := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/teams/"+teamID, adminToken, nil)
	if got["name"] != name {
		t.Fatalf("want name %q, got %v", name, got["name"])
	}

	// update
	updated := mustDo(t, "PUT", "/api/v1/orgs/"+orgID+"/teams/"+teamID, adminToken, M{"name": name + "-updated"})
	if updated["name"] != name+"-updated" {
		t.Fatalf("want updated name, got %v", updated["name"])
	}

	// delete
	r := do("DELETE", "/api/v1/orgs/"+orgID+"/teams/"+teamID, adminToken, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", r.StatusCode)
	}
}

// ── service accounts ──────────────────────────────────────────────────────────

func TestServiceAccounts_CreateAndToken(t *testing.T) {
	name := fmt.Sprintf("sa-%d", time.Now().UnixNano())

	// create service account with a single read-only scope
	sa := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/service-accounts", adminToken, M{
		"name": name, "scopes": []string{"diagrams:read"},
	})
	saID := str(sa, "id")
	if saID == "" {
		t.Fatal("expected id in response")
	}

	// create token
	tok := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/service-accounts/"+saID+"/tokens", adminToken, M{
		"name": "ci-token",
	})
	plaintext := str(tok, "token")
	if plaintext == "" {
		t.Fatal("expected plaintext token in response")
	}

	// authenticate with that token via X-API-Key
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/auth/me", nil)
	req.Header.Set("X-API-Key", plaintext)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("service account token auth: want 200, got %d", resp.StatusCode)
	}

	// granted scope (diagrams:read) allows listing diagrams
	listReq, _ := http.NewRequest("GET", srv.URL+"/api/v1/orgs/"+orgID+"/diagrams", nil)
	listReq.Header.Set("X-API-Key", plaintext)
	listResp, _ := http.DefaultClient.Do(listReq)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("scoped diagrams:read list: want 200, got %d", listResp.StatusCode)
	}

	// missing scope (diagrams:write) is denied with 403
	createReq, _ := http.NewRequest("POST", srv.URL+"/api/v1/orgs/"+orgID+"/diagrams", bytes.NewReader([]byte(`{"name":"x","content":"{}"}`)))
	createReq.Header.Set("X-API-Key", plaintext)
	createReq.Header.Set("Content-Type", "application/json")
	createResp, _ := http.DefaultClient.Do(createReq)
	if createResp.StatusCode != http.StatusForbidden {
		t.Fatalf("unscoped diagrams:write: want 403, got %d", createResp.StatusCode)
	}

	// revoke token
	r := do("DELETE", "/api/v1/orgs/"+orgID+"/service-accounts/"+saID+"/tokens/"+str(tok, "id"), adminToken, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204 on revoke, got %d", r.StatusCode)
	}
}

func TestAuthProviders_ListMasksSecrets(t *testing.T) {
	body := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/auth/providers", adminToken, nil)
	providers := list(body, "providers")
	for _, p := range providers {
		if str(p, "id") != oidcProviderID {
			continue
		}
		if str(p, "clientSecret") != "***" {
			t.Fatalf("want masked clientSecret, got %q", str(p, "clientSecret"))
		}
		return
	}
	t.Fatalf("expected provider %s in %v", oidcProviderID, providers)
}

func TestAuthProviders_CreateUpdateDelete(t *testing.T) {
	name := fmt.Sprintf("Test OIDC %d", time.Now().UnixNano())
	slug := fmt.Sprintf("test-oidc-%d", time.Now().UnixNano())
	body := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/auth/providers", adminToken, M{
		"slug": slug,
		"kind": "oidc", "type": "generic", "displayName": name,
		"enabled": true, "allowSignUp": true, "defaultRole": "viewer",
		"clientId": "abc", "clientSecret": "shh",
		"authUrl": "https://idp.test/authorize", "tokenUrl": "https://idp.test/token",
	})
	if str(body, "slug") != slug {
		t.Fatalf("want slug %q, got %v", slug, body)
	}

	mustDo(t, "PUT", "/api/v1/orgs/"+orgID+"/auth/providers/"+slug, adminToken, M{
		"slug": slug,
		"kind": "oidc", "type": "generic", "displayName": name,
		"enabled": false, "allowSignUp": true, "defaultRole": "editor",
		"clientId": "abc", "clientSecret": "***",
		"authUrl": "https://idp.test/authorize", "tokenUrl": "https://idp.test/token",
	})
	stored, err := testDB.GetAuthProviderBySlug(context.Background(), orgID, slug)
	if err != nil || stored == nil {
		t.Fatalf("get provider: %v", err)
	}
	if stored.ClientSecret == "" || stored.ClientSecret == "***" {
		t.Fatalf("want the stored secret preserved, got %q", stored.ClientSecret)
	}
	if stored.DefaultRole != authz.RoleEditor {
		t.Fatalf("want editor default role, got %q", stored.DefaultRole)
	}

	r := do("DELETE", "/api/v1/orgs/"+orgID+"/auth/providers/"+slug, adminToken, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", r.StatusCode)
	}
}

func TestAuthProviders_RejectsIncompleteProvider(t *testing.T) {
	r := do("POST", "/api/v1/orgs/"+orgID+"/auth/providers", adminToken, M{
		"slug": "missing-endpoints",
		"kind": "oidc", "type": "generic", "displayName": "Missing Endpoints",
		"defaultRole": "viewer", "clientId": "abc",
	})
	if r.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", r.StatusCode)
	}
}

func TestAuthProviders_RejectsMalformedSlug(t *testing.T) {
	for _, slug := range []string{"", "Acme Okta", "ACME", "-okta", "okta-", "acme--okta", "acme_okta"} {
		r := do("POST", "/api/v1/orgs/"+orgID+"/auth/providers", adminToken, M{
			"slug": slug,
			"kind": "oidc", "type": "generic", "displayName": "Slug " + slug,
			"enabled": true, "defaultRole": "viewer", "clientId": "abc",
			"authUrl": "https://idp.test/authorize", "tokenUrl": "https://idp.test/token",
		})
		if r.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400 for slug %q, got %d", slug, r.StatusCode)
		}
	}
}

func TestAuthProviders_RejectsSlugChange(t *testing.T) {
	slug := fmt.Sprintf("immutable-%d", time.Now().UnixNano())
	mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/auth/providers", adminToken, M{
		"slug": slug,
		"kind": "oidc", "type": "generic", "displayName": "Immutable " + slug,
		"enabled": true, "defaultRole": "viewer", "clientId": "abc",
		"authUrl": "https://idp.test/authorize", "tokenUrl": "https://idp.test/token",
	})
	defer func() { _ = do("DELETE", "/api/v1/orgs/"+orgID+"/auth/providers/"+slug, adminToken, nil) }()

	r := do("PUT", "/api/v1/orgs/"+orgID+"/auth/providers/"+slug, adminToken, M{
		"slug": slug + "-renamed",
		"kind": "oidc", "type": "generic", "displayName": "Immutable " + slug,
		"enabled": true, "defaultRole": "viewer", "clientId": "abc",
		"authUrl": "https://idp.test/authorize", "tokenUrl": "https://idp.test/token",
	})
	if r.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 on slug change, got %d", r.StatusCode)
	}
}

func TestAuthProviders_RejectsDuplicateSlug(t *testing.T) {
	slug := fmt.Sprintf("duplicate-%d", time.Now().UnixNano())
	body := M{
		"slug": slug,
		"kind": "oidc", "type": "generic", "displayName": "Duplicate " + slug,
		"enabled": true, "defaultRole": "viewer", "clientId": "abc",
		"authUrl": "https://idp.test/authorize", "tokenUrl": "https://idp.test/token",
	}
	mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/auth/providers", adminToken, body)
	defer func() { _ = do("DELETE", "/api/v1/orgs/"+orgID+"/auth/providers/"+slug, adminToken, nil) }()

	body["displayName"] = "Duplicate " + slug + " again"
	r := do("POST", "/api/v1/orgs/"+orgID+"/auth/providers", adminToken, body)
	if r.StatusCode != http.StatusConflict {
		t.Fatalf("want 409 on duplicate slug, got %d", r.StatusCode)
	}
}

func TestRoleMappings_CreateListDelete(t *testing.T) {
	base := "/api/v1/orgs/" + orgID + "/auth/providers/" + oidcProviderSlug + "/role-mappings"
	val := fmt.Sprintf("admins-%d", time.Now().UnixNano())

	created := mustDo(t, "POST", base, adminToken, M{
		"priority": 1, "attributeKey": "groups", "operator": "contains",
		"attributeValue": val, "role": "admin",
	})
	mappingID := str(created, "id")
	if mappingID == "" {
		t.Fatalf("expected an id, got %v", created)
	}

	body := mustDo(t, "GET", base, adminToken, nil)
	if len(list(body, "mappings")) == 0 {
		t.Fatal("expected at least one mapping")
	}

	r := do("DELETE", base+"/"+mappingID, adminToken, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", r.StatusCode)
	}
}

func TestRoleMappings_RejectsUnknownOperator(t *testing.T) {
	base := "/api/v1/orgs/" + orgID + "/auth/providers/" + oidcProviderSlug + "/role-mappings"
	r := do("POST", base, adminToken, M{
		"priority": 1, "attributeKey": "groups", "operator": "sortOf",
		"attributeValue": "x", "role": "admin",
	})
	if r.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", r.StatusCode)
	}
}

func TestOrgDomains_CreateListDelete(t *testing.T) {
	base := "/api/v1/orgs/" + orgID + "/auth/domains"
	domain := fmt.Sprintf("d%d.example.net", time.Now().UnixNano())

	created := mustDo(t, "POST", base, adminToken, M{"domain": domain})
	domainID := str(created, "id")
	if domainID == "" {
		t.Fatalf("expected an id, got %v", created)
	}

	body := mustDo(t, "GET", base, adminToken, nil)
	if len(list(body, "domains")) == 0 {
		t.Fatal("expected at least one domain")
	}

	r := do("DELETE", base+"/"+domainID, adminToken, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", r.StatusCode)
	}
}

func TestMappingOperators(t *testing.T) {
	body := mustDo(t, "GET", "/api/v1/auth/mapping-operators", adminToken, nil)
	if len(list(body, "operators")) == 0 {
		t.Fatal("expected a non-empty operator catalog")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

type M = map[string]any

func do(method, path, token string, body any) *http.Response {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, srv.URL+path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(fmt.Sprintf("http %s %s: %v", method, path, err))
	}
	return resp
}

func mustDo(t interface {
	Helper()
	Fatal(...any)
}, method, path, token string, body any) M {
	t.Helper()
	resp := do(method, path, token, body)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		var e M
		_ = json.NewDecoder(resp.Body).Decode(&e)
		t.Fatal(method, path, "→", resp.StatusCode, e)
	}
	var out M
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out
}

func str(m M, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func obj(m M, key string) M {
	v, _ := m[key].(map[string]any)
	return v
}

func list(m M, key string) []M {
	raw, _ := m[key].([]any)
	out := make([]M, len(raw))
	for i, v := range raw {
		out[i], _ = v.(map[string]any)
	}
	return out
}

// t_ returns a minimal testing.T-like value for TestMain (where *testing.T is unavailable).
func t_(name string) *fatalLogger { return &fatalLogger{name: name} }

type fatalLogger struct{ name string }

func (f *fatalLogger) Helper() {}
func (f *fatalLogger) Fatal(args ...any) {
	fmt.Fprintf(os.Stderr, "FATAL [%s]: %v\n", f.name, fmt.Sprint(args...))
	os.Exit(1)
}
