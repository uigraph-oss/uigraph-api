package identity

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/uigraph/app/internal/authz"
)

const (
	KindOIDC = "oidc"
	KindSAML = "saml"
)

const (
	TypeGeneric = "generic"
	TypeEntra   = "entra"
	TypeOkta    = "okta"
)

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

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
var domainLabelPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
var domainLetterPattern = regexp.MustCompile(`[a-z]`)

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

type OrgDomain struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"orgId"`
	Domain    string    `json:"domain"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func NormalizeOrgDomain(value string) (string, error) {
	domain := strings.TrimSpace(value)
	domain = strings.TrimPrefix(domain, "@")
	domain = strings.ToLower(strings.TrimSpace(domain))
	if len(domain) < 3 || len(domain) > 253 {
		return "", fmt.Errorf("identity: invalid organization domain")
	}

	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("identity: invalid organization domain")
	}

	for _, label := range labels {
		if len(label) < 1 || len(label) > 63 || !domainLabelPattern.MatchString(label) {
			return "", fmt.Errorf("identity: invalid organization domain")
		}
	}

	if !domainLetterPattern.MatchString(labels[len(labels)-1]) {
		return "", fmt.Errorf("identity: invalid organization domain")
	}

	return domain, nil
}

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
