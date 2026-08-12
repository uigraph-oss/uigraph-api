package authz

import (
	"context"
	"errors"
	"testing"

	"github.com/uigraph/app/internal/org"
)

type stubRBAC struct {
	members map[string]OrgMember // keyed by userID+"/"+orgID
	calls   int
}

func (s *stubRBAC) GetOrgMember(_ context.Context, userID, orgID string) (OrgMember, error) {
	s.calls++
	m, ok := s.members[userID+"/"+orgID]
	if !ok {
		return OrgMember{}, ErrNotFound
	}
	return m, nil
}

func (s *stubRBAC) UpsertOrgMember(_ context.Context, _, _ string, _ Role, _ string) error {
	panic("not used")
}

type stubUsers struct {
	users map[string]*org.User
	err   error
}

func (s *stubUsers) GetUser(_ context.Context, id string) (*org.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.users[id], nil
}

func (s *stubUsers) GetUserByLogin(_ context.Context, _ string) (*org.User, error) {
	panic("not used")
}

func TestScopesForUser(t *testing.T) {
	rbac := &stubRBAC{members: map[string]OrgMember{
		"viewer/org-a": {UserID: "viewer", OrgID: "org-a", Role: RoleViewer},
	}}
	users := &stubUsers{users: map[string]*org.User{
		"viewer":   {ID: "viewer", Role: "user"},
		"admin":    {ID: "admin", Role: "server_admin"},
		"offadmin": {ID: "offadmin", Role: "server_admin", Disabled: true},
	}}
	a := New(rbac, users)

	tests := []struct {
		name   string
		userID string
		orgID  string
		want   []Scope
	}{
		{"server admin in an org they do not belong to", "admin", "org-z", ScopesForRole(RoleAdmin)},
		{"server admin in an org that does not exist", "admin", "nope", ScopesForRole(RoleAdmin)},
		{"member keeps their own role", "viewer", "org-a", ScopesForRole(RoleViewer)},
		{"non-member is denied", "viewer", "org-b", nil},
		{"unknown user is denied", "ghost", "org-a", nil},
		{"disabled server admin is denied", "offadmin", "org-a", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := a.ScopesForUser(context.Background(), tt.userID, tt.orgID)
			if err != nil {
				t.Fatalf("ScopesForUser: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d scopes %v, want %d %v", len(got), got, len(tt.want), tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("scope %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// A server admin must clear every scope the router can require, in any org.
func TestScopesForUserServerAdminSatisfiesEveryScope(t *testing.T) {
	users := &stubUsers{users: map[string]*org.User{"admin": {ID: "admin", Role: "server_admin"}}}
	a := New(&stubRBAC{members: map[string]OrgMember{}}, users)

	resolved, err := a.ScopesForUser(context.Background(), "admin", "any-org")
	if err != nil {
		t.Fatalf("ScopesForUser: %v", err)
	}
	granted := make([]string, len(resolved))
	for i, s := range resolved {
		granted[i] = string(s)
	}

	for _, want := range AllScopes {
		if !Has(granted, want) {
			t.Errorf("server admin lacks %q", want)
		}
	}
}

// The membership lookup must be skipped entirely for server admins -- they have
// no row to find, and falling through would deny them.
func TestScopesForUserServerAdminSkipsMembershipLookup(t *testing.T) {
	rbac := &stubRBAC{members: map[string]OrgMember{}}
	users := &stubUsers{users: map[string]*org.User{"admin": {ID: "admin", Role: "server_admin"}}}
	a := New(rbac, users)

	if _, err := a.ScopesForUser(context.Background(), "admin", "org-a"); err != nil {
		t.Fatalf("ScopesForUser: %v", err)
	}
	if rbac.calls != 0 {
		t.Errorf("GetOrgMember called %d times, want 0", rbac.calls)
	}
}

// A failing user lookup must deny, never fall through to a membership check
// that could silently succeed with the wrong answer.
func TestScopesForUserPropagatesUserLookupError(t *testing.T) {
	boom := errors.New("db down")
	a := New(&stubRBAC{members: map[string]OrgMember{}}, &stubUsers{err: boom})

	got, err := a.ScopesForUser(context.Background(), "admin", "org-a")
	if !errors.Is(err, boom) {
		t.Fatalf("got err %v, want %v", err, boom)
	}
	if got != nil {
		t.Errorf("got scopes %v on error, want nil", got)
	}
}

func TestIsUserServerAdmin(t *testing.T) {
	users := &stubUsers{users: map[string]*org.User{
		"admin":    {ID: "admin", Role: "server_admin"},
		"offadmin": {ID: "offadmin", Role: "server_admin", Disabled: true},
		"plain":    {ID: "plain", Role: "user"},
	}}
	a := New(&stubRBAC{members: map[string]OrgMember{}}, users)

	tests := []struct {
		userID string
		want   bool
	}{
		{"admin", true},
		{"offadmin", false},
		{"plain", false},
		{"ghost", false},
	}

	for _, tt := range tests {
		got, err := a.IsUserServerAdmin(context.Background(), tt.userID)
		if err != nil {
			t.Fatalf("IsUserServerAdmin(%s): %v", tt.userID, err)
		}
		if got != tt.want {
			t.Errorf("IsUserServerAdmin(%s) = %v, want %v", tt.userID, got, tt.want)
		}
	}
}
