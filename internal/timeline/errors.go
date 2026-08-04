package timeline

import "errors"

var (
	ErrEventNotFound         = errors.New("timeline event not found")
	ErrTitleRequired         = errors.New("title is required")
	ErrUnknownEventType      = errors.New("unknown timeline event type")
	ErrUnknownDecisionStatus = errors.New("unknown decision status")
)
