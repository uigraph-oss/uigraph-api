package identity

import (
	"context"
	"time"
)

// AuthIdentity links an IdP subject to a local user. ProviderID is the
// auth_providers row, so one person authenticating through two providers has two
// rows pointing at a single user.
type AuthIdentity struct {
	UserID     string
	ProviderID string
	Sub        string
}

// AuthIdentityStore records which IdP subject a user authenticated as.
type AuthIdentityStore interface {
	UpsertAuthIdentity(ctx context.Context, a AuthIdentity) error
}

// LoginAttemptStore backs brute-force and enumeration rate limiting on the
// unauthenticated login endpoints.
type LoginAttemptStore interface {
	RecordLoginAttempt(ctx context.Context, username, ip string) error
	CountLoginAttempts(ctx context.Context, username string, since time.Time) (int, error)
}
