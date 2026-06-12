package authz

// Role is one of the three fixed org/resource roles.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

// roleRank maps roles to a numeric weight used by AtLeast.
// Higher value = more permissive.
var roleRank = map[Role]int{
	RoleViewer: 1,
	RoleEditor: 2,
	RoleAdmin:  3,
}

// AtLeast returns true when r is at least as permissive as minimum.
// Unknown roles always return false.
func (r Role) AtLeast(minimum Role) bool {
	return roleRank[r] >= roleRank[minimum]
}
