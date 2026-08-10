package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/uigraph/app/internal/enterprise"
	"github.com/uigraph/app/internal/identity"
	"github.com/uigraph/app/internal/org"
	"github.com/uigraph/app/internal/rolemap"
)

// fakeSessionStore embeds the (unexported) sessionStore interface as a nil
// value and overrides only the handful of methods completeLogin's seat-gate
// path actually calls -- any other method falling through to the embedded
// nil interface panics on use, which is the point: it surfaces immediately
// if a test exercises a code path this fake wasn't built to support, without
// needing ~40 lines of unrelated stub methods for interfaces completeLogin
// never touches in this flow.
type fakeSessionStore struct {
	sessionStore

	usersByEmail   map[string]*org.User
	existingMember *org.OrgMember // returned by GetMember; nil = brand-new membership
	members        []org.OrgMember

	createdUser    *org.User
	upsertedMember *org.OrgMember
	sessionCreated bool
}

func (f *fakeSessionStore) GetUserByEmail(ctx context.Context, email string) (*org.User, error) {
	return f.usersByEmail[email], nil
}
func (f *fakeSessionStore) CreateUser(ctx context.Context, u org.User) error {
	f.createdUser = &u
	f.usersByEmail[u.Email] = &u
	return nil
}
func (f *fakeSessionStore) ListRoleMappings(ctx context.Context, providerID string) ([]rolemap.Rule, error) {
	return nil, nil
}
func (f *fakeSessionStore) GetMember(ctx context.Context, userID, orgID string) (*org.OrgMember, error) {
	return f.existingMember, nil
}
func (f *fakeSessionStore) ListMembers(ctx context.Context, orgID string) ([]org.OrgMember, error) {
	return f.members, nil
}
func (f *fakeSessionStore) UpsertMember(ctx context.Context, m org.OrgMember) error {
	f.upsertedMember = &m
	return nil
}
func (f *fakeSessionStore) UpsertAuthIdentity(ctx context.Context, a identity.AuthIdentity) error {
	return nil
}
func (f *fakeSessionStore) CreateSession(ctx context.Context, s identity.Session) error {
	f.sessionCreated = true
	return nil
}

func testProvider() *identity.AuthProvider {
	return &identity.AuthProvider{
		ID:          "prov1",
		OrgID:       "org1",
		AllowSignUp: true,
		DefaultRole: "viewer",
	}
}

func TestCompleteLogin_NewMember_AtSeatLimit_Rejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"seatLimit":3,"planId":"team"}`))
	}))
	defer srv.Close()

	store := &fakeSessionStore{
		usersByEmail: map[string]*org.User{},
		members:      []org.OrgMember{{UserID: "u1"}, {UserID: "u2"}, {UserID: "u3"}},
	}
	h := NewSessionHandler(store, nil, nil, "https://app.example.com", "", "", enterprise.New(srv.URL, "secret"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/orgs/org1/callback/okta", nil)
	h.completeLogin(rec, req, testProvider(), &identity.LoginState{}, completedAuth{Email: "new@example.com", Name: "New User"})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s, want %d", rec.Code, rec.Body.String(), http.StatusForbidden)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != "seat_limit_reached" {
		t.Fatalf("code = %q, want seat_limit_reached", body["code"])
	}
	if store.upsertedMember != nil || store.sessionCreated {
		t.Fatal("expected no membership/session to be created when rejected")
	}
}

func TestCompleteLogin_ExistingMember_OverSeatCap_StillLogsIn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"seatLimit":3,"planId":"team"}`))
	}))
	defer srv.Close()

	existing := &org.OrgMember{UserID: "u1", OrgID: "org1", Role: "viewer"}
	store := &fakeSessionStore{
		usersByEmail:   map[string]*org.User{"existing@example.com": {ID: "u1", Email: "existing@example.com"}},
		existingMember: existing, // already a member -- org being at/over cap must not lock them out
		members:        []org.OrgMember{{UserID: "u1"}, {UserID: "u2"}, {UserID: "u3"}, {UserID: "u4"}},
	}
	h := NewSessionHandler(store, nil, nil, "https://app.example.com", "", "", enterprise.New(srv.URL, "secret"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/orgs/org1/callback/okta", nil)
	h.completeLogin(rec, req, testProvider(), &identity.LoginState{}, completedAuth{Email: "existing@example.com", Name: "Existing User"})

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, body = %s, want %d (existing member should still be able to log in)", rec.Code, rec.Body.String(), http.StatusFound)
	}
	if store.upsertedMember == nil || !store.sessionCreated {
		t.Fatal("expected the existing member's login to complete (membership refresh + session)")
	}
}

func TestCompleteLogin_NewMember_UnderSeatLimit_Allowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"seatLimit":5,"planId":"team"}`))
	}))
	defer srv.Close()

	store := &fakeSessionStore{
		usersByEmail: map[string]*org.User{},
		members:      []org.OrgMember{{UserID: "u1"}},
	}
	h := NewSessionHandler(store, nil, nil, "https://app.example.com", "", "", enterprise.New(srv.URL, "secret"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/orgs/org1/callback/okta", nil)
	h.completeLogin(rec, req, testProvider(), &identity.LoginState{}, completedAuth{Email: "new@example.com", Name: "New User"})

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, body = %s, want %d", rec.Code, rec.Body.String(), http.StatusFound)
	}
	if store.upsertedMember == nil || !store.sessionCreated {
		t.Fatal("expected the new member to be added and logged in")
	}
}

func TestCompleteLogin_SelfHosted_SeatCheckSkipped(t *testing.T) {
	store := &fakeSessionStore{
		usersByEmail: map[string]*org.User{},
		members:      []org.OrgMember{{UserID: "u1"}, {UserID: "u2"}, {UserID: "u3"}},
	}
	h := NewSessionHandler(store, nil, nil, "https://app.example.com", "", "", nil) // nil enterprise client == self-hosted

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/orgs/org1/callback/okta", nil)
	h.completeLogin(rec, req, testProvider(), &identity.LoginState{}, completedAuth{Email: "new@example.com", Name: "New User"})

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, body = %s, want %d (self-hosted must be unaffected)", rec.Code, rec.Body.String(), http.StatusFound)
	}
}
