package org

import "time"

// Invitation is a pending org membership offer sent to an email address.
type Invitation struct {
	ID        string
	OrgID     string
	Email     string
	Role      string
	Code      string
	InvitedBy string
	Status    string // pending | accepted | revoked
	ExpiresAt time.Time
	CreatedAt time.Time
}
