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

	"github.com/uigraph/app/internal/api"
	"github.com/uigraph/app/internal/authz"
	"github.com/uigraph/app/internal/bootstrap"
	"github.com/uigraph/app/internal/config"
	"github.com/uigraph/app/internal/identity"
	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/migrate"
	"github.com/uigraph/app/internal/oauth"
	"github.com/uigraph/app/internal/store/postgres"
	)

// ── shared state ──────────────────────────────────────────────────────────────

var (
	srv        *httptest.Server
	idp        *httptest.Server // fake OIDC provider for OAuth tests
	adminToken string
	orgID      string // default org created by bootstrap
)

const ssoUserEmail = "sso-user@example.com"

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
	defer db.Close()

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

	srv = httptest.NewServer(api.New(db, authmw.NewSessionVerifier(db, db), &config.Config{PublicURL: "http://localhost:8080", FrontendURL: ""}, testStorage, nil))
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

	org := mustDo(t_("TestMain"), "POST", "/api/v1/orgs", adminToken, M{"name": "Test Org", "slug": fmt.Sprintf("test-org-%d", time.Now().UnixNano())})
	orgID = str(org, "id")

	if err := db.UpsertOrgMember(ctx, u.ID, orgID, authz.RoleAdmin, "test"); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: add member: %v\n", err)
		os.Exit(1)
	}

	// Seed a DB-backed OAuth provider (instance slug "generic") globally.
	if err := db.UpsertOAuthProvider(ctx, identity.OAuthProviderConfig{
		ProviderName: oauth.Generic,
		Type:         oauth.Generic,
		DisplayName:  "Generic OAuth",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		AuthURL:      idp.URL + "/authorize",
		TokenURL:     idp.URL + "/token",
		UserinfoURL:  idp.URL + "/userinfo",
		Scopes:       "openid profile email",
		AllowSignUp:  true,
		EmailClaim:   "email",
		NameClaim:    "name",
		SubClaim:     "sub",
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

// ── OAuth ───────────────────────────────────────────────────────────────────--

func TestOAuth_ListProviders(t *testing.T) {
	body := mustDo(t, "GET", "/api/v1/auth/providers", "", nil)
	providers := list(body, "providers")
	found := false
	for _, p := range providers {
		if str(p, "name") == oauth.Generic {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected generic provider in %v", providers)
	}
}

func TestOAuth_CallbackProvisionsUserAndSession(t *testing.T) {
	// Don't follow the post-login redirect; we want the 302 + Set-Cookie.
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/auth/callback/generic?code=abc&state=xyz", nil)
	req.AddCookie(&http.Cookie{Name: "uigraph_oauth_state", Value: "xyz"})
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("callback request: %v", err)
	}
	defer resp.Body.Close()
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
	defer meResp.Body.Close()
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 from /me, got %d", meResp.StatusCode)
	}
	var me M
	_ = json.NewDecoder(meResp.Body).Decode(&me)
	if me["email"] != ssoUserEmail {
		t.Fatalf("want email %q, got %v", ssoUserEmail, me["email"])
	}
}

func TestOAuth_CallbackRejectsBadState(t *testing.T) {
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/auth/callback/generic?code=abc&state=xyz", nil)
	req.AddCookie(&http.Cookie{Name: "uigraph_oauth_state", Value: "different"})
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("callback request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 on state mismatch, got %d", resp.StatusCode)
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
	slug := fmt.Sprintf("test-org-%d", time.Now().UnixNano())

	// create
	created := mustDo(t, "POST", "/api/v1/orgs", adminToken, M{"name": "Test Org", "slug": slug})
	id := str(created, "id")
	if id == "" {
		t.Fatal("expected id in response")
	}

	// get
	got := mustDo(t, "GET", "/api/v1/orgs/"+id, adminToken, nil)
	if got["slug"] != slug {
		t.Fatalf("want slug %q, got %v", slug, got["slug"])
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

	// create service account
	sa := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/service-accounts", adminToken, M{
		"name": name, "role": "viewer",
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

	// revoke token
	r := do("DELETE", "/api/v1/orgs/"+orgID+"/service-accounts/"+saID+"/tokens/"+str(tok, "id"), adminToken, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204 on revoke, got %d", r.StatusCode)
	}
}

// ── SSO role mappings ─────────────────────────────────────────────────────────

func TestSSOMappings_CreateListDelete(t *testing.T) {
	val := fmt.Sprintf("admins-%d", time.Now().UnixNano())
	// create
	r := do("POST", "/api/v1/sso/role-mappings", adminToken, M{
		"claimKey": "groups", "claimValue": val, "role": "admin", "scope": "org",
	})
	if r.StatusCode != http.StatusCreated {
		t.Fatalf("want 201, got %d", r.StatusCode)
	}

	// list
	body := mustDo(t, "GET", "/api/v1/sso/role-mappings", adminToken, nil)
	mappings := list(body, "mappings")
	if len(mappings) == 0 {
		t.Fatal("expected at least one mapping")
	}

	// delete
	mappingID := str(mappings[len(mappings)-1], "id")
	r = do("DELETE", "/api/v1/sso/role-mappings/"+mappingID, adminToken, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", r.StatusCode)
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

func mustDo(t interface{ Helper(); Fatal(...any) }, method, path, token string, body any) M {
	t.Helper()
	resp := do(method, path, token, body)
	defer resp.Body.Close()
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
