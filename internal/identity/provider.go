package identity

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/uigraph/app/internal/authz"
)

// Provider kinds. Stored as text in auth_providers.kind and validated here
// rather than by a Postgres enum, per AGENTS.md.
const (
	KindOIDC = "oidc"
	KindSAML = "saml"
)

// OIDC provider presets. Entra and Okta derive their endpoints from a tenant or
// org URL; generic requires the endpoints to be supplied explicitly.
const (
	TypeGeneric = "generic"
	TypeEntra   = "entra"
	TypeOkta    = "okta"
)

// AuthProvider is one OIDC or SAML provider belonging to a single org. An org
// may configure many of either kind.
//
// Slug identifies the provider in every URL — login, callback, ACS, SP metadata
// and the admin API — and is unique within the org. ID never appears in a URL.
// The slug is fixed at creation because it is embedded in the redirect URI and
// Entity ID registered with the IdP, where a change would break sign-in.
//
// SPEntityID is derived from the org and slug rather than stored, so it cannot
// drift from the URL the metadata is actually served at.
//
// The two kinds share identity, presentation and role-resolution settings and
// differ only in their credential fields, so they share one table discriminated
// by Kind. Fields belonging to the other kind are zero.
//
// ClientSecret and SPKey hold ciphertext everywhere outside the handler that
// owns the cipher: the admin API encrypts before writing and masks on read, and
// the login path decrypts just before use. This mirrors internal/api/figma and
// internal/api/billing, which likewise keep the cipher in the handler rather
// than the store.
type AuthProvider struct {
	ID             string     `json:"id"`
	Slug           string     `json:"slug"`
	OrgID          string     `json:"orgId"`
	Kind           string     `json:"kind"`
	Type           string     `json:"type"`
	DisplayName    string     `json:"displayName"`
	IconAssetID    string     `json:"-"`
	IconURL        string     `json:"iconUrl"`
	Enabled        bool       `json:"enabled"`
	AllowSignUp    bool       `json:"allowSignUp"`
	AllowedDomains string     `json:"allowedDomains"`
	DefaultRole    authz.Role `json:"defaultRole"`

	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	AuthURL      string `json:"authUrl,omitempty"`
	TokenURL     string `json:"tokenUrl,omitempty"`
	UserinfoURL  string `json:"userinfoUrl,omitempty"`
	APIURL       string `json:"apiUrl,omitempty"`
	Scopes       string `json:"scopes,omitempty"`
	EmailClaim   string `json:"emailClaim,omitempty"`
	NameClaim    string `json:"nameClaim,omitempty"`
	SubClaim     string `json:"subClaim,omitempty"`
	GroupsClaim  string `json:"groupsClaim,omitempty"`

	IDPMetadataURL  string `json:"idpMetadataUrl,omitempty"`
	IDPMetadataXML  string `json:"idpMetadataXml,omitempty"`
	IDPEntityID     string `json:"idpEntityId,omitempty"`
	IDPSSOURL       string `json:"idpSsoUrl,omitempty"`
	IDPCert         string `json:"idpCert,omitempty"`
	SPEntityID      string `json:"spEntityId,omitempty"`
	SPCert          string `json:"spCert,omitempty"`
	SPKey           string `json:"spKey,omitempty"`
	SignRequests    bool   `json:"signRequests,omitempty"`
	NameIDFormat    string `json:"nameIdFormat,omitempty"`
	EmailAttribute  string `json:"emailAttribute,omitempty"`
	NameAttribute   string `json:"nameAttribute,omitempty"`
	GroupsAttribute string `json:"groupsAttribute,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// slugPattern is the accepted shape of a provider slug: lowercase
// alphanumerics separated by single hyphens.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateSlug checks a provider slug. The slug is typed by the admin and lands
// in URLs that get registered with an external IdP, so it is checked verbatim
// and never lowercased, trimmed or otherwise coerced into shape: a value that
// does not match is an error for the admin to fix, not something to guess at.
func ValidateSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("identity: slug is required")
	}
	if len(slug) < 2 || len(slug) > 63 {
		return fmt.Errorf("identity: slug must be between 2 and 63 characters")
	}
	if !slugPattern.MatchString(slug) {
		return fmt.Errorf("identity: slug must be lowercase letters, digits and single hyphens, e.g. acme-okta")
	}
	return nil
}

// Validate checks the fields required by the provider's kind. Kind-specific
// columns are nullable in the schema, so this is the only guard against a
// half-configured provider reaching the login path.
func (p AuthProvider) Validate() error {
	if strings.TrimSpace(p.OrgID) == "" {
		return fmt.Errorf("identity: orgId is required")
	}
	if err := ValidateSlug(p.Slug); err != nil {
		return err
	}
	if strings.TrimSpace(p.DisplayName) == "" {
		return fmt.Errorf("identity: displayName is required")
	}
	if p.DefaultRole != authz.RoleAdmin && p.DefaultRole != authz.RoleEditor && p.DefaultRole != authz.RoleViewer {
		return fmt.Errorf("identity: unknown default role %q", p.DefaultRole)
	}

	switch p.Kind {
	case KindOIDC:
		if p.Type != TypeGeneric && p.Type != TypeEntra && p.Type != TypeOkta {
			return fmt.Errorf("identity: unknown oidc type %q", p.Type)
		}
		if strings.TrimSpace(p.ClientID) == "" {
			return fmt.Errorf("identity: clientId is required for oidc providers")
		}
		if strings.TrimSpace(p.AuthURL) == "" || strings.TrimSpace(p.TokenURL) == "" {
			return fmt.Errorf("identity: authUrl and tokenUrl are required for oidc providers")
		}
		return nil

	case KindSAML:
		if strings.TrimSpace(p.IDPSSOURL) == "" {
			return fmt.Errorf("identity: idpSsoUrl is required for saml providers")
		}
		if strings.TrimSpace(p.IDPCert) == "" {
			return fmt.Errorf("identity: idpCert is required for saml providers")
		}
		return nil

	default:
		return fmt.Errorf("identity: unknown provider kind %q", p.Kind)
	}
}

// EmailAllowed reports whether an address passes the provider's allowed-domain
// filter. An empty AllowedDomains permits any domain.
func (p AuthProvider) EmailAllowed(email string) bool {
	if strings.TrimSpace(p.AllowedDomains) == "" {
		return true
	}
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return false
	}
	domain := strings.ToLower(email[at+1:])
	for _, d := range strings.Split(p.AllowedDomains, ",") {
		if strings.ToLower(strings.TrimSpace(d)) == domain {
			return true
		}
	}
	return false
}

// OrgDomain claims an email domain for an org. A domain may be claimed by
// several orgs, and an org may claim several domains. Domains are stored
// lowercased and without the '@'.
type OrgDomain struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"orgId"`
	Domain    string    `json:"domain"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// LoginState correlates a login redirect with the org and provider it started
// from. State doubles as the OIDC state parameter and the SAML RelayState.
// Rows are single-use: ConsumeLoginState deletes on read.
type LoginState struct {
	ID            string
	State         string
	OrgID         string
	ProviderID    string
	Nonce         string
	SAMLRequestID string
	RedirectTo    string
	CreatedAt     time.Time
	ExpiresAt     time.Time
}

// LDAPConfig holds the connection and attribute mapping for a global LDAP server.
type LDAPConfig struct {
	ID             string
	Host           string
	Port           int
	UseSSL         bool
	StartTLS       bool
	SkipTLSVerify  bool
	BindDN         string
	BindPassword   string // encrypted at rest
	SearchBaseDN   string
	SearchFilter   string
	EmailAttribute string
	NameAttribute  string
	UsernameAttr   string
	MemberOfAttr   string
	AllowSignUp    bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// SCIMConfig holds the global SCIM 2.0 provisioning configuration.
type SCIMConfig struct {
	ID              string
	Enabled         bool
	BearerTokenHash string
	SyncUsers       bool
	SyncGroups      bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
