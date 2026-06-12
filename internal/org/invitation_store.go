package org

import "context"

// InvitationStore is the persistence interface for org invitations.
type InvitationStore interface {
	CreateInvitation(ctx context.Context, inv Invitation) error
	GetInvitationByCode(ctx context.Context, code string) (*Invitation, error)
	GetInvitation(ctx context.Context, id string) (*Invitation, error)
	ListInvitations(ctx context.Context, orgID string) ([]Invitation, error)
	AcceptInvitation(ctx context.Context, id string) error
	RevokeInvitation(ctx context.Context, id string) error
}
