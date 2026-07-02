package identity

import "errors"

var ErrNotFound = errors.New("identity: not found")
var ErrConflict = errors.New("identity: conflict")
