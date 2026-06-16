package identity

import "time"

// ServiceAccount represents a non-human principal scoped to an org.
type ServiceAccount struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"orgId"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	// Scopes are the named permissions granted to this account's tokens,
	// e.g. "diagrams:write". See authz.AllScopes for the full catalog.
	Scopes   []string `json:"scopes"`
	Disabled bool     `json:"disabled"`
	CreatedBy   string    `json:"createdBy,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Token is a single API token belonging to a ServiceAccount.
// One account may have many tokens; each is independently revocable.
type Token struct {
	ID               string     `json:"id"`
	ServiceAccountID string     `json:"serviceAccountId"`
	Name             string     `json:"name"`
	// Prefix is the first 12 characters of the plaintext token (e.g. "uig_a3f9b1c2").
	// Stored for display and indexed lookup; does not expose the secret.
	Prefix           string     `json:"prefix"`
	// Hash is the lower-case hex SHA-256 digest of the plaintext. Never stored as plaintext.
	Hash             string     `json:"-"`
	ExpiresAt        *time.Time `json:"expiresAt,omitempty"`
	LastUsedAt       *time.Time `json:"lastUsedAt,omitempty"`
	Revoked          bool       `json:"revoked"`
	CreatedBy        string     `json:"createdBy,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
}
