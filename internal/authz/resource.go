package authz

// ResourceType identifies a UIGraph resource kind.
type ResourceType string

const (
	ResourceDiagram ResourceType = "diagram"
	ResourceService ResourceType = "service"
	ResourceProject ResourceType = "project"
)
