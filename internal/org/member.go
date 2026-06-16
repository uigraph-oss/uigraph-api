package org

import "time"

// OrgMember is a user's membership record within an org.
type OrgMember struct {
	UserID    string
	OrgID     string
	Role      string // admin | editor | viewer
	Source    string // manual | sso
	CreatedAt time.Time
	UpdatedAt time.Time
}

// OrgMembershipView pairs an org with the caller's membership in it. Used to
// populate the org switcher.
type OrgMembershipView struct {
	Org      Org
	Role     string    // admin | editor | viewer
	JoinedAt time.Time // when the membership was created
}
