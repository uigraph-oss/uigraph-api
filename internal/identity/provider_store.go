package identity

import "context"

// ProviderStore is the persistence interface for external identity provider configs.
// Configs are global (one per instance), not per org.
type ProviderStore interface {
	// OAuth
	UpsertOAuthProvider(ctx context.Context, cfg OAuthProviderConfig) error
	GetOAuthProvider(ctx context.Context, provider string) (*OAuthProviderConfig, error)
	ListOAuthProviders(ctx context.Context) ([]OAuthProviderConfig, error)
	DeleteOAuthProvider(ctx context.Context, provider string) error
	SetOAuthProviderIcon(ctx context.Context, provider string, assetID *string) error

	// LDAP
	UpsertLDAPConfig(ctx context.Context, cfg LDAPConfig) error
	GetLDAPConfig(ctx context.Context) (*LDAPConfig, error)
	DeleteLDAPConfig(ctx context.Context) error

	// SAML
	UpsertSAMLConfig(ctx context.Context, cfg SAMLConfig) error
	GetSAMLConfig(ctx context.Context) (*SAMLConfig, error)

	// SCIM
	UpsertSCIMConfig(ctx context.Context, cfg SCIMConfig) error
	GetSCIMConfig(ctx context.Context) (*SCIMConfig, error)
	RotateSCIMToken(ctx context.Context, newHash string) error
}
