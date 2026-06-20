package org

import "context"

// UserStore is the persistence interface for user accounts.
type UserStore interface {
	CreateUser(ctx context.Context, u User) error
	UpsertUser(ctx context.Context, u User) error
	GetUser(ctx context.Context, id string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByLogin(ctx context.Context, login string) (*User, error)
	ListUsers(ctx context.Context, orgID string) ([]User, error)
	ListAllUsers(ctx context.Context) ([]User, error)
	CountAllUsers(ctx context.Context) (int, error)
	CountActiveUsers(ctx context.Context) (int, error)
	AnyUserExists(ctx context.Context) (bool, error)
	UpdateUser(ctx context.Context, u User) error
	DisableUser(ctx context.Context, id string) error
	// SetUserAvatar sets or clears (assetID nil) a user's avatar asset id.
	SetUserAvatar(ctx context.Context, userID string, assetID *string) error
	// TouchUser updates last_seen_at to now.
	TouchUser(ctx context.Context, id string) error
}
