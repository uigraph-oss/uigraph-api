package identity

// PrincipalKind distinguishes human users from service accounts.
type PrincipalKind string

const (
	PrincipalUser           PrincipalKind = "user"
	PrincipalServiceAccount PrincipalKind = "service_account"
)

// Principal is placed in the request context by the HTTP middleware after
// successful authentication. Downstream handlers call the Authorizer using
// Principal.UserID and Principal.OrgID without re-querying the token layer.
type Principal struct {
	Kind PrincipalKind

	// UserID is the SSO/OIDC subject for users, or the service-account UUID.
	UserID string

	// OrgID is the organisation a service account is bound to.
	// Empty for users — user requests are scoped by the {orgID} path param.
	OrgID string

	// IsServerAdmin is true when the user row holds role = 'server_admin'.
	// Only valid for Kind == PrincipalUser.
	IsServerAdmin bool

	// ServiceAccountID is populated only when Kind == PrincipalServiceAccount.
	ServiceAccountID string

	// Scopes are the permissions granted to a service account, e.g.
	// "diagrams:create". Populated only when Kind == PrincipalServiceAccount.
	// User scopes are not stored here; they are resolved per-request from the
	// user's org role (which depends on the {orgID} route path param).
	Scopes []string

	// AuthProvider is how the session was created: 'password' or the OAuth
	// provider instance name. Empty for service accounts.
	AuthProvider string
}
