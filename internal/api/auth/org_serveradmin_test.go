package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/uigraph/app/internal/identity"
	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/org"
)

// orgStoreStub implements org.OrgStore; only the reads the two handlers touch
// are wired.
type orgStoreStub struct {
	orgs []org.Org
}

func (s *orgStoreStub) ListOrgs(context.Context) ([]org.Org, error) { return s.orgs, nil }
func (s *orgStoreStub) GetOrg(_ context.Context, id string) (*org.Org, error) {
	for i := range s.orgs {
		if s.orgs[i].ID == id {
			return &s.orgs[i], nil
		}
	}
	return nil, nil
}
func (s *orgStoreStub) CreateOrg(context.Context, org.Org) error { panic("not used") }
func (s *orgStoreStub) ListAutoJoinOrgs(context.Context) ([]org.Org, error) {
	panic("not used")
}
func (s *orgStoreStub) CountAllOrgs(context.Context) (int, error)  { panic("not used") }
func (s *orgStoreStub) UpdateOrg(context.Context, org.Org) error   { panic("not used") }
func (s *orgStoreStub) DeleteOrg(context.Context, string) error    { panic("not used") }
func (s *orgStoreStub) AnyOrgExists(context.Context) (bool, error) { panic("not used") }
func (s *orgStoreStub) SetOrgLogo(context.Context, string, *string) error {
	panic("not used")
}
func (s *orgStoreStub) SetOnboardingDone(context.Context, string) error { panic("not used") }

// memberStoreStub implements org.MemberStore; membership reads only.
type memberStoreStub struct {
	byUser  map[string][]org.OrgMembershipView
	members map[string]*org.OrgMember // keyed by userID+"/"+orgID
	// getMemberCalls counts membership lookups so tests can assert the
	// server-admin path skips them entirely.
	getMemberCalls int
}

func (s *memberStoreStub) ListOrgsForUser(_ context.Context, userID string) ([]org.OrgMembershipView, error) {
	return s.byUser[userID], nil
}
func (s *memberStoreStub) GetMember(_ context.Context, userID, orgID string) (*org.OrgMember, error) {
	s.getMemberCalls++
	return s.members[userID+"/"+orgID], nil
}
func (s *memberStoreStub) AddMember(context.Context, org.OrgMember) error    { panic("not used") }
func (s *memberStoreStub) UpsertMember(context.Context, org.OrgMember) error { panic("not used") }
func (s *memberStoreStub) ListMembers(context.Context, string) ([]org.OrgMember, error) {
	panic("not used")
}
func (s *memberStoreStub) UpdateMemberRole(context.Context, string, string, string, string) error {
	panic("not used")
}
func (s *memberStoreStub) RemoveMember(context.Context, string, string) error { panic("not used") }

func withPrincipal(req *http.Request, p identity.Principal) *http.Request {
	return req.WithContext(authmw.WithPrincipal(req.Context(), p))
}

func newOrgFixture() (*orgStoreStub, *memberStoreStub, *OrgHandler) {
	orgs := &orgStoreStub{orgs: []org.Org{
		{ID: "org-a", Name: "Alpha"},
		{ID: "org-b", Name: "Beta"},
		{ID: "org-c", Name: "Gamma"},
	}}
	members := &memberStoreStub{
		byUser: map[string][]org.OrgMembershipView{
			"plain": {{Org: org.Org{ID: "org-a", Name: "Alpha"}, Role: "viewer"}},
		},
		members: map[string]*org.OrgMember{
			"plain/org-a": {UserID: "plain", OrgID: "org-a", Role: "viewer"},
		},
	}
	return orgs, members, NewOrgHandler(orgs, members, nil, nil)
}

func decodeOrgIDs(t *testing.T, rec *httptest.ResponseRecorder) []string {
	t.Helper()
	var body struct {
		Orgs []struct {
			ID string `json:"id"`
		} `json:"orgs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (body %s)", err, rec.Body.String())
	}
	ids := make([]string, len(body.Orgs))
	for i, o := range body.Orgs {
		ids[i] = o.ID
	}
	return ids
}

func TestListMineServerAdminSeesEveryOrg(t *testing.T) {
	_, members, h := newOrgFixture()

	rec := httptest.NewRecorder()
	req := withPrincipal(
		httptest.NewRequest(http.MethodGet, "/api/v1/orgs", nil),
		identity.Principal{Kind: identity.PrincipalUser, UserID: "admin", IsServerAdmin: true},
	)
	h.ListMine(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	got := decodeOrgIDs(t, rec)
	want := []string{"org-a", "org-b", "org-c"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if members.getMemberCalls != 0 {
		t.Errorf("membership lookups = %d, want 0", members.getMemberCalls)
	}
}

func TestListMineNonAdminStillScopedToMemberships(t *testing.T) {
	_, _, h := newOrgFixture()

	rec := httptest.NewRecorder()
	req := withPrincipal(
		httptest.NewRequest(http.MethodGet, "/api/v1/orgs", nil),
		identity.Principal{Kind: identity.PrincipalUser, UserID: "plain"},
	)
	h.ListMine(rec, req)

	got := decodeOrgIDs(t, rec)
	if len(got) != 1 || got[0] != "org-a" {
		t.Fatalf("got %v, want [org-a] -- non-admin must not see other orgs", got)
	}
}

// A service account must stay pinned to its own org even though the
// server-admin branch sits directly above it in the handler.
func TestListMineServiceAccountUnaffected(t *testing.T) {
	_, _, h := newOrgFixture()

	rec := httptest.NewRecorder()
	req := withPrincipal(
		httptest.NewRequest(http.MethodGet, "/api/v1/orgs", nil),
		identity.Principal{
			Kind:             identity.PrincipalServiceAccount,
			UserID:           "sa-1",
			ServiceAccountID: "sa-1",
			OrgID:            "org-b",
			IsServerAdmin:    true, // must be ignored for service accounts
		},
	)
	h.ListMine(rec, req)

	got := decodeOrgIDs(t, rec)
	if len(got) != 1 || got[0] != "org-b" {
		t.Fatalf("got %v, want [org-b]", got)
	}
}

func TestGetMineServerAdminEntersAnyOrg(t *testing.T) {
	_, members, h := newOrgFixture()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orgs/org-c", nil)
	req.SetPathValue("orgID", "org-c")
	req = withPrincipal(req, identity.Principal{
		Kind: identity.PrincipalUser, UserID: "admin", IsServerAdmin: true,
	})
	h.GetMine(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s, want 200", rec.Code, rec.Body.String())
	}
	if members.getMemberCalls != 0 {
		t.Errorf("membership lookups = %d, want 0", members.getMemberCalls)
	}
}

func TestGetMineNonMemberStillRejected(t *testing.T) {
	_, _, h := newOrgFixture()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orgs/org-c", nil)
	req.SetPathValue("orgID", "org-c")
	req = withPrincipal(req, identity.Principal{Kind: identity.PrincipalUser, UserID: "plain"})
	h.GetMine(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s, want 404", rec.Code, rec.Body.String())
	}
}

// A server admin asking for an org that does not exist must 404, not 200 with
// a zero-valued org.
func TestGetMineServerAdminUnknownOrg(t *testing.T) {
	_, _, h := newOrgFixture()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orgs/nope", nil)
	req.SetPathValue("orgID", "nope")
	req = withPrincipal(req, identity.Principal{
		Kind: identity.PrincipalUser, UserID: "admin", IsServerAdmin: true,
	})
	h.GetMine(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s, want 404", rec.Code, rec.Body.String())
	}
}

// A service account must not reach another org through GetMine.
func TestGetMineServiceAccountCrossOrgRejected(t *testing.T) {
	_, _, h := newOrgFixture()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orgs/org-a", nil)
	req.SetPathValue("orgID", "org-a")
	req = withPrincipal(req, identity.Principal{
		Kind:             identity.PrincipalServiceAccount,
		UserID:           "sa-1",
		ServiceAccountID: "sa-1",
		OrgID:            "org-b",
		IsServerAdmin:    true,
	})
	h.GetMine(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s, want 403", rec.Code, rec.Body.String())
	}
}

// sessionStoreStub embeds the sessionStore interface so every method the test
// does not exercise panics on call rather than needing a stub body.
type sessionStoreStub struct {
	sessionStore
	orgs    []org.Org
	byUser  map[string][]org.OrgMembershipView
	listAll int
}

func (s *sessionStoreStub) ListOrgs(context.Context) ([]org.Org, error) {
	s.listAll++
	return s.orgs, nil
}

func (s *sessionStoreStub) ListOrgsForUser(_ context.Context, userID string) ([]org.OrgMembershipView, error) {
	return s.byUser[userID], nil
}

func newSessionFixture() (*sessionStoreStub, *SessionHandler) {
	st := &sessionStoreStub{
		orgs: []org.Org{
			{ID: "org-a", Name: "Alpha", OnboardingDone: true},
			{ID: "org-b", Name: "Beta"},
		},
		byUser: map[string][]org.OrgMembershipView{
			"plain": {{Org: org.Org{ID: "org-a", Name: "Alpha"}, Role: "viewer"}},
		},
	}
	return st, NewSessionHandler(st, nil, nil, "", "", "", nil)
}

func decodeMyOrgs(t *testing.T, rec *httptest.ResponseRecorder) []myOrg {
	t.Helper()
	var body struct {
		Orgs []myOrg `json:"orgs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (body %s)", err, rec.Body.String())
	}
	return body.Orgs
}

func TestMyOrgsServerAdminSeesEveryOrgAsAdmin(t *testing.T) {
	_, h := newSessionFixture()

	rec := httptest.NewRecorder()
	req := withPrincipal(
		httptest.NewRequest(http.MethodGet, "/api/v1/auth/orgs", nil),
		identity.Principal{Kind: identity.PrincipalUser, UserID: "admin", IsServerAdmin: true},
	)
	h.MyOrgs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	got := decodeMyOrgs(t, rec)
	if len(got) != 2 {
		t.Fatalf("got %d orgs, want 2: %+v", len(got), got)
	}
	for _, o := range got {
		// The UI derives its whole permission model from this role, so it must
		// match the scopes the authorizer actually grants.
		if o.Role != "admin" {
			t.Errorf("org %s role = %q, want admin", o.ID, o.Role)
		}
	}
	if !got[0].OnboardingDone || got[1].OnboardingDone {
		t.Errorf("onboardingDone not carried through: %+v", got)
	}
}

func TestMyOrgsNonAdminStillScopedToMemberships(t *testing.T) {
	_, h := newSessionFixture()

	rec := httptest.NewRecorder()
	req := withPrincipal(
		httptest.NewRequest(http.MethodGet, "/api/v1/auth/orgs", nil),
		identity.Principal{Kind: identity.PrincipalUser, UserID: "plain"},
	)
	h.MyOrgs(rec, req)

	got := decodeMyOrgs(t, rec)
	if len(got) != 1 || got[0].ID != "org-a" || got[0].Role != "viewer" {
		t.Fatalf("got %+v, want one org-a/viewer entry", got)
	}
}
