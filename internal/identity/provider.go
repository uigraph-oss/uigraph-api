package identity

import "time"

// OAuthProviderConfig holds the configuration for an OAuth2 provider instance.
// Configured once at the instance level, not per org.
type OAuthProviderConfig struct {
	ID             string    `json:"id"`
	ProviderName   string    `json:"providerName"` // unique instance slug used in login URLs
	Type           string    `json:"type"`         // generic | entra | okta
	DisplayName    string    `json:"displayName"`  // label shown on the login page
	IconURL        string    `json:"iconUrl"`      // logo shown on the login button
	ClientID       string    `json:"clientId"`
	ClientSecret   string    `json:"clientSecret"` // encrypted at rest
	AuthURL        string    `json:"authUrl"`
	TokenURL       string    `json:"tokenUrl"`
	UserinfoURL    string    `json:"userinfoUrl"`
	APIURL         string    `json:"apiUrl"`
	Scopes         string    `json:"scopes"`
	AllowedDomains string    `json:"allowedDomains"`
	AllowSignUp    bool      `json:"allowSignUp"`
	EmailClaim     string    `json:"emailClaim"`
	NameClaim      string    `json:"nameClaim"`
	SubClaim       string    `json:"subClaim"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
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

// SAMLConfig holds the global SP and IdP configuration for SAML 2.0 SSO.
type SAMLConfig struct {
	ID              string
	IDPMetadataURL  string
	IDPMetadataXML  string
	IDPEntityID     string
	IDPSSOUrl       string
	IDPCert         string
	SPEntityID      string
	SPCert          string
	SPKey           string // encrypted at rest
	SignRequests    bool
	NameIDFormat    string
	EmailAttribute  string
	NameAttribute   string
	LoginAttribute  string
	GroupsAttribute string
	AllowSignUp     bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
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
