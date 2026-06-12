package identity

import "context"

// ServiceAccountStore is the persistence interface for service accounts and tokens.
type ServiceAccountStore interface {
	CreateServiceAccount(ctx context.Context, sa ServiceAccount) error
	GetServiceAccount(ctx context.Context, id string) (*ServiceAccount, error)
	ListServiceAccounts(ctx context.Context, orgID string) ([]ServiceAccount, error)
	UpdateServiceAccount(ctx context.Context, sa ServiceAccount) error
	DeleteServiceAccount(ctx context.Context, id string) error

	CreateToken(ctx context.Context, t Token) error
	// GetTokenByPrefix returns the token whose prefix matches, or (nil, nil) if not found.
	GetTokenByPrefix(ctx context.Context, prefix string) (*Token, error)
	ListTokens(ctx context.Context, serviceAccountID string) ([]Token, error)
	RevokeToken(ctx context.Context, tokenID string) error
	// TouchToken updates last_used_at; called on every authenticated request.
	TouchToken(ctx context.Context, tokenID string) error
}
