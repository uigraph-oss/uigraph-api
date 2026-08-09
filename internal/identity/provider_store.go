package identity

import (
	"context"

	"github.com/uigraph/app/internal/org"
	"github.com/uigraph/app/internal/rolemap"
)

type ProviderStore interface {
	CreateAuthProvider(ctx context.Context, p AuthProvider) error
	UpdateAuthProvider(ctx context.Context, p AuthProvider) error
	GetAuthProviderBySlug(ctx context.Context, orgID, slug string) (*AuthProvider, error)
	ListAuthProviders(ctx context.Context, orgID string) ([]AuthProvider, error)
	ListEnabledAuthProviders(ctx context.Context, orgID string) ([]AuthProvider, error)
	DeleteAuthProvider(ctx context.Context, orgID, providerID string) error
	SetAuthProviderIcon(ctx context.Context, orgID, providerID string, assetID *string) error

	CreateRoleMapping(ctx context.Context, m rolemap.Rule) error
	UpdateRoleMapping(ctx context.Context, m rolemap.Rule) error
	ListRoleMappings(ctx context.Context, providerID string) ([]rolemap.Rule, error)
	DeleteRoleMapping(ctx context.Context, providerID, mappingID string) error

	CreateOrgDomain(ctx context.Context, d OrgDomain) error
	ListOrgDomains(ctx context.Context, orgID string) ([]OrgDomain, error)
	DeleteOrgDomain(ctx context.Context, orgID, domainID string) error
	ListOrgsByDomain(ctx context.Context, domain string) ([]org.Org, error)

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
