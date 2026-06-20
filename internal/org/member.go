package org

import "time"

// OrgMember is a user's membership record within an org. Email/Name and the
// first-team association (TeamID) are populated by ListMembers via a join on
// users + team_members; they are empty/nil on GetMember.
type OrgMember struct {
	UserID    string
	OrgID     string
	Role      string // admin | editor | viewer
	Source    string // manual | sso
	Email     string
	Name      string
	TeamID    *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// OrgMembershipView pairs an org with the caller's role in it. Used to populate
// the org switcher.
type OrgMembershipView struct {
	Org  Org
	Role string
}
