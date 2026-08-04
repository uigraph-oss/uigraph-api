package timeline

import "context"

type Store interface {
	CreateEvent(ctx context.Context, orgID, serviceID, actorID string, in Input) (*Event, error)
	UpdateEvent(ctx context.Context, orgID, serviceID, eventID, actorID string, in Input) (*Event, error)
	DeleteEvent(ctx context.Context, orgID, serviceID, eventID string) error
	GetEvent(ctx context.Context, orgID, serviceID, eventID string) (*Event, error)
	ListEventsForService(ctx context.Context, orgID, serviceID string) ([]Event, error)
}
