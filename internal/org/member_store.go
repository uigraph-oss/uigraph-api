package org

import "context"

// MemberStore is the persistence interface for org membership.
type MemberStore interface {
	AddMember(ctx context.Context, m OrgMember) error
	GetMember(ctx context.Context, userID, orgID string) (*OrgMember, error)
	ListMembers(ctx context.Context, orgID string) ([]OrgMember, error)
	UpdateMemberRole(ctx context.Context, userID, orgID, role, source string) error
	RemoveMember(ctx context.Context, userID, orgID string) error
	ListOrgsForUser(ctx context.Context, userID string) ([]OrgMembershipView, error)
}
