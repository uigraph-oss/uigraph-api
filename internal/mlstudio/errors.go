package mlstudio

import "errors"

var ErrParentNotFound = errors.New("mlstudio: parent entity not found")

var ErrInvalidValue = errors.New("mlstudio: invalid value")

var ErrUnknownUser = errors.New("mlstudio: unknown uigraph.user email")
