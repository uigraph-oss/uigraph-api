package timeline

import "context"

type Store interface {
	CreateEvent(ctx context.Context, orgID, serviceID, actorID string, in Input) (*Event, error)
	// UpsertEventBySourceRef is the CI/CLI-facing entry point: a single
	// INSERT ... ON CONFLICT (service_id, source_ref) DO UPDATE, race-free
	// under concurrent syncs. Always writes origin = 'auto'.
	UpsertEventBySourceRef(ctx context.Context, orgID, serviceID, actorID string, commitHash *string, in Input) (event *Event, created bool, err error)
	UpdateEvent(ctx context.Context, orgID, serviceID, eventID, actorID string, in Input) (*Event, error)
	DeleteEvent(ctx context.Context, orgID, serviceID, eventID string) error
	GetEvent(ctx context.Context, orgID, serviceID, eventID string) (*Event, error)
	ListEventsForService(ctx context.Context, orgID, serviceID string) ([]Event, error)
}
