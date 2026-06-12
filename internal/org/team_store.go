package org

import "context"

// TeamStore is the persistence interface for teams and their memberships.
type TeamStore interface {
	CreateTeam(ctx context.Context, t Team) error
	GetTeam(ctx context.Context, id string) (*Team, error)
	ListTeams(ctx context.Context, orgID string) ([]Team, error)
	UpdateTeam(ctx context.Context, t Team) error
	DeleteTeam(ctx context.Context, id string) error

	AddTeamMember(ctx context.Context, m TeamMember) error
	ListTeamMembers(ctx context.Context, teamID string) ([]TeamMember, error)
	RemoveTeamMember(ctx context.Context, teamID, userID string) error
}
