package authz

import "errors"

// ErrNotFound is returned when no membership / permission row exists.
var ErrNotFound = errors.New("authz: not found")

// ErrForbidden is returned when an authenticated principal lacks the required role.
var ErrForbidden = errors.New("authz: forbidden")

// ErrLastAdmin is returned when an operation would leave an org with no admin.
var ErrLastAdmin = errors.New("authz: cannot remove the last admin user")
