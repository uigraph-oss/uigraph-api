package identity

import "context"

// SessionStore is the persistence interface for user sessions.
type SessionStore interface {
	CreateSession(ctx context.Context, s Session) error
	GetSessionByToken(ctx context.Context, hash string) (*Session, error)
	RotateSession(ctx context.Context, id, newHash, prevHash string) error
	MarkTokenSeen(ctx context.Context, id string) error
	DeleteSession(ctx context.Context, id string) error
	DeleteUserSessions(ctx context.Context, userID string) error
	ListUserSessions(ctx context.Context, userID string) ([]Session, error)
}
