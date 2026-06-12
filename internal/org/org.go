package org

import "time"

// Org is a tenant that groups users, resources, and settings.
type Org struct {
	ID        string
	Name      string
	Slug      string
	Disabled  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
