package authz

// Role is one of the three fixed org roles. A role resolves to an explicit
// scope set via RoleScopes.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)
