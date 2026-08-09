package identity

import (
	"context"
	"time"
)

type AuthIdentity struct {
	UserID     string
	ProviderID string
	Sub        string
}

type AuthIdentityStore interface {
	UpsertAuthIdentity(ctx context.Context, a AuthIdentity) error
}

type LoginAttemptStore interface {
	RecordLoginAttempt(ctx context.Context, username, ip string) error
	CountLoginAttempts(ctx context.Context, username string, since time.Time) (int, error)
}
