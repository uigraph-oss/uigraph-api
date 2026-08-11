package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/uigraph/app/internal/enterprise"
	"github.com/uigraph/app/internal/org"
)

// fakeUserStore implements org.UserStore, only GetUserByEmail/CreateUser wired
// -- the only two methods MemberHandler.Add calls.
type fakeUserStore struct {
	byEmail map[string]*org.User
	created []org.User
}

func newFakeUserStore() *fakeUserStore { return &fakeUserStore{byEmail: map[string]*org.User{}} }

func (f *fakeUserStore) GetUserByEmail(ctx context.Context, email string) (*org.User, error) {
	return f.byEmail[email], nil
}
func (f *fakeUserStore) CreateUser(ctx context.Context, u org.User) error {
	f.created = append(f.created, u)
	f.byEmail[u.Email] = &u
	return nil
}
func (f *fakeUserStore) UpsertUser(ctx context.Context, u org.User) error { panic("not used by Add") }
func (f *fakeUserStore) GetUser(ctx context.Context, id string) (*org.User, error) {
	panic("not used by Add")
}
func (f *fakeUserStore) GetUserByLogin(ctx context.Context, login string) (*org.User, error) {
	panic("not used by Add")
}
func (f *fakeUserStore) ListUsers(ctx context.Context, orgID string) ([]org.User, error) {
	panic("not used by Add")
}
func (f *fakeUserStore) ListAllUsers(ctx context.Context) ([]org.User, error) {
	panic("not used by Add")
}
func (f *fakeUserStore) CountAllUsers(ctx context.Context) (int, error)    { panic("not used by Add") }
func (f *fakeUserStore) CountActiveUsers(ctx context.Context) (int, error) { panic("not used by Add") }
func (f *fakeUserStore) AnyUserExists(ctx context.Context) (bool, error)  { panic("not used by Add") }
func (f *fakeUserStore) UpdateUser(ctx context.Context, u org.User) error { panic("not used by Add") }
func (f *fakeUserStore) DisableUser(ctx context.Context, id string) error { panic("not used by Add") }
func (f *fakeUserStore) SetUserAvatar(ctx context.Context, userID string, assetID *string) error {
	panic("not used by Add")
}
func (f *fakeUserStore) TouchUser(ctx context.Context, id string) error { panic("not used by Add") }

// fakeMemberStore implements org.MemberStore, only AddMember/ListMembers
// wired -- what MemberHandler.Add and the seat gate actually call.
type fakeMemberStore struct {
	members []org.OrgMember
	added   []org.OrgMember
}

func (f *fakeMemberStore) AddMember(ctx context.Context, m org.OrgMember) error {
	f.added = append(f.added, m)
	f.members = append(f.members, m)
	return nil
}
func (f *fakeMemberStore) ListMembers(ctx context.Context, orgID string) ([]org.OrgMember, error) {
	return f.members, nil
}
func (f *fakeMemberStore) UpsertMember(ctx context.Context, m org.OrgMember) error {
	panic("not used by Add")
}
func (f *fakeMemberStore) GetMember(ctx context.Context, userID, orgID string) (*org.OrgMember, error) {
	panic("not used by Add")
}
func (f *fakeMemberStore) UpdateMemberRole(ctx context.Context, userID, orgID, role, source string) error {
	panic("not used by Add")
}
func (f *fakeMemberStore) RemoveMember(ctx context.Context, userID, orgID string) error {
	panic("not used by Add")
}
func (f *fakeMemberStore) ListOrgsForUser(ctx context.Context, userID string) ([]org.OrgMembershipView, error) {
	panic("not used by Add")
}

func addRequest(orgID string, body map[string]string) *http.Request {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orgs/"+orgID+"/members", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("orgID", orgID)
	return req
}

func TestMemberHandlerAdd_SelfHosted_SeatCheckSkipped(t *testing.T) {
	users := newFakeUserStore()
	members := &fakeMemberStore{members: []org.OrgMember{{UserID: "u1"}, {UserID: "u2"}, {UserID: "u3"}}}
	h := NewMemberHandler(members, users, nil, nil) // nil enterprise client == self-hosted

	rec := httptest.NewRecorder()
	h.Add(rec, addRequest("org1", map[string]string{
		"name": "New User", "email": "new@example.com", "password": "hunter22", "role": "editor",
	}))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s, want %d", rec.Code, rec.Body.String(), http.StatusCreated)
	}
	if len(members.added) != 1 {
		t.Fatalf("expected 1 member added, got %d", len(members.added))
	}
}

func TestMemberHandlerAdd_AtSeatLimit_Rejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"seatLimit":3,"planId":"team"}`))
	}))
	defer srv.Close()

	users := newFakeUserStore()
	members := &fakeMemberStore{members: []org.OrgMember{{UserID: "u1"}, {UserID: "u2"}, {UserID: "u3"}}}
	h := NewMemberHandler(members, users, nil, enterprise.New(srv.URL, "secret"))

	rec := httptest.NewRecorder()
	h.Add(rec, addRequest("org1", map[string]string{
		"name": "New User", "email": "new@example.com", "password": "hunter22", "role": "editor",
	}))

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s, want %d", rec.Code, rec.Body.String(), http.StatusConflict)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != "seat_limit_reached" {
		t.Fatalf("code = %q, want seat_limit_reached", body["code"])
	}
	if len(users.created) != 0 || len(members.added) != 0 {
		t.Fatalf("expected no side effects, got users.created=%d members.added=%d", len(users.created), len(members.added))
	}
}

func TestMemberHandlerAdd_UnderSeatLimit_Allowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"seatLimit":5,"planId":"team"}`))
	}))
	defer srv.Close()

	users := newFakeUserStore()
	members := &fakeMemberStore{members: []org.OrgMember{{UserID: "u1"}}}
	h := NewMemberHandler(members, users, nil, enterprise.New(srv.URL, "secret"))

	rec := httptest.NewRecorder()
	h.Add(rec, addRequest("org1", map[string]string{
		"name": "New User", "email": "new@example.com", "password": "hunter22", "role": "editor",
	}))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s, want %d", rec.Code, rec.Body.String(), http.StatusCreated)
	}
}

func TestMemberHandlerAdd_UnlimitedSeatLimit_Allowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"seatLimit":-1,"planId":"enterprise"}`))
	}))
	defer srv.Close()

	users := newFakeUserStore()
	members := &fakeMemberStore{members: []org.OrgMember{{UserID: "u1"}, {UserID: "u2"}, {UserID: "u3"}}}
	h := NewMemberHandler(members, users, nil, enterprise.New(srv.URL, "secret"))

	rec := httptest.NewRecorder()
	h.Add(rec, addRequest("org1", map[string]string{
		"name": "New User", "email": "new@example.com", "password": "hunter22", "role": "editor",
	}))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s, want %d", rec.Code, rec.Body.String(), http.StatusCreated)
	}
}

func TestMemberHandlerAdd_EnterpriseUnreachable_FailsOpen(t *testing.T) {
	users := newFakeUserStore()
	members := &fakeMemberStore{members: []org.OrgMember{{UserID: "u1"}, {UserID: "u2"}, {UserID: "u3"}}}
	// Nothing is listening on this port -- SeatLimit will return a connection error.
	h := NewMemberHandler(members, users, nil, enterprise.New("http://127.0.0.1:1", "secret"))

	rec := httptest.NewRecorder()
	h.Add(rec, addRequest("org1", map[string]string{
		"name": "New User", "email": "new@example.com", "password": "hunter22", "role": "editor",
	}))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s, want %d (fail open on enterprise-service error)", rec.Code, rec.Body.String(), http.StatusCreated)
	}
	if len(members.added) != 1 {
		t.Fatalf("expected the add to go through despite the enterprise-service error, got %d added", len(members.added))
	}
}
