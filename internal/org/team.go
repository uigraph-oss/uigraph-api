package org

import "time"

// Team is a named group within an org, optionally synced from an IdP.
type Team struct {
	ID          string
	OrgID       string
	Name        string
	Email       string
	ExternalID  string
	MemberCount int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TeamMember is a user's membership in a team with a team-level permission.
type TeamMember struct {
	TeamID     string
	UserID     string
	OrgID      string
	Permission string // member | admin
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
