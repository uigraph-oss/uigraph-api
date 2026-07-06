package org

import "time"

// Org is a tenant that groups users, resources, and settings.
type Org struct {
	ID             string
	Name           string
	LogoAssetID    *string
	Disabled       bool
	AutoJoin       bool
	OnboardingDone bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
