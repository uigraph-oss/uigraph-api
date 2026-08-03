package billing

import "errors"

var (
	ErrConnectionNotFound = errors.New("cloud connection not found")
	ErrTagRuleNotFound    = errors.New("tag rule not found")
	ErrUnknownProvider    = errors.New("unknown cloud provider")
	ErrInvalidCredential  = errors.New("invalid credential payload for provider")
)
