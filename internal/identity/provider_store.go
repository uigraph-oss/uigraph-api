package identity

import (
	"context"

	"github.com/uigraph/app/internal/org"
	"github.com/uigraph/app/internal/rolemap"
)

// ProviderStore is the persistence interface for external identity provider
// configuration. Auth providers, their role mappings and org email domains are
// per-org; LDAP and SCIM remain global and unimplemented.
type ProviderStore interface {
	// Auth providers.
	//
	// Every method takes orgID so a caller cannot reach another org's provider
	// by guessing its ID. Lookups are by slug because that is what the URL
	// carries; the UUID stays internal and is what the delete, icon and role
	// mapping methods key on once a handler has resolved the slug.
	CreateAuthProvider(ctx context.Context, p AuthProvider) error
	UpdateAuthProvider(ctx context.Context, p AuthProvider) error
	GetAuthProviderBySlug(ctx context.Context, orgID, slug string) (*AuthProvider, error)
	ListAuthProviders(ctx context.Context, orgID string) ([]AuthProvider, error)
	ListEnabledAuthProviders(ctx context.Context, orgID string) ([]AuthProvider, error)
	DeleteAuthProvider(ctx context.Context, orgID, providerID string) error
	SetAuthProviderIcon(ctx context.Context, orgID, providerID string, assetID *string) error

	// Role mappings, owned by a provider and evaluated by internal/rolemap.
	CreateRoleMapping(ctx context.Context, m rolemap.Rule) error
	UpdateRoleMapping(ctx context.Context, m rolemap.Rule) error
	ListRoleMappings(ctx context.Context, providerID string) ([]rolemap.Rule, error)
	DeleteRoleMapping(ctx context.Context, providerID, mappingID string) error

	// Org email domains. ListOrgsByDomain drives email-first org discovery and
	// may return several orgs for one domain.
	CreateOrgDomain(ctx context.Context, d OrgDomain) error
	ListOrgDomains(ctx context.Context, orgID string) ([]OrgDomain, error)
	DeleteOrgDomain(ctx context.Context, orgID, domainID string) error
	ListOrgsByDomain(ctx context.Context, domain string) ([]org.Org, error)

	// Login handshake state. Rows are single-use: ConsumeLoginState deletes the
	// row it returns, so a replayed state or RelayState finds nothing.
	CreateLoginState(ctx context.Context, s LoginState) error
	ConsumeLoginState(ctx context.Context, state string) (*LoginState, error)
	PurgeExpiredLoginState(ctx context.Context) error

	// LDAP
	UpsertLDAPConfig(ctx context.Context, cfg LDAPConfig) error
	GetLDAPConfig(ctx context.Context) (*LDAPConfig, error)
	DeleteLDAPConfig(ctx context.Context) error

	// SCIM
	UpsertSCIMConfig(ctx context.Context, cfg SCIMConfig) error
	GetSCIMConfig(ctx context.Context) (*SCIMConfig, error)
	RotateSCIMToken(ctx context.Context, newHash string) error
}
