package org

import "time"

// User is a human account that can be a member of one or more orgs.
type User struct {
	ID                 string
	Email              string
	Name               string
	Login              string
	PasswordHash       string
	MustChangePassword bool
	Disabled           bool
	Role               string // "user" | "server_admin"
	LastSeenAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
