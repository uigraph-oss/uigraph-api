// Package timeline models per-service Release, Decision (ADR), and Incident
// events shown on a service's Timeline tab. Only manually-created events
// (Origin = OriginManual) are written today — OriginAuto is reserved for a
// future CLI repo-scan sync.
package timeline

import "time"

type EventType string

const (
	EventTypeRelease  EventType = "release"
	EventTypeDecision EventType = "decision"
	EventTypeIncident EventType = "incident"
)

type DecisionStatus string

const (
	DecisionStatusProposed   DecisionStatus = "proposed"
	DecisionStatusAccepted   DecisionStatus = "accepted"
	DecisionStatusSuperseded DecisionStatus = "superseded"
	DecisionStatusDeprecated DecisionStatus = "deprecated"
)

type Origin string

const (
	OriginAuto   Origin = "auto"
	OriginManual Origin = "manual"
)

// Touch is a free-text reference to a node or service an event affected.
// Not a foreign key — there's no generic node/service registry to link
// against yet, so this is display-only, same as the UI already treats it.
type Touch struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}

// Event is one Timeline entry for a service.
type Event struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"orgId"`
	ServiceID string    `json:"serviceId"`
	Type      EventType `json:"type"`
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	EventDate time.Time `json:"eventDate"`

	// Version is set for Type == EventTypeRelease.
	Version *string `json:"version,omitempty"`
	// ADRNumber and DecisionStatus are set for Type == EventTypeDecision.
	ADRNumber      *string         `json:"adrNumber,omitempty"`
	DecisionStatus *DecisionStatus `json:"decisionStatus,omitempty"`

	SourceLabel       *string `json:"sourceLabel,omitempty"`
	SourceURL         *string `json:"sourceUrl,omitempty"`
	IsAgentSummarized bool    `json:"isAgentSummarized"`
	Origin            Origin  `json:"origin"`
	Touches           []Touch `json:"touches"`

	// SourceRef is the CLI repo-scan's stable key for the file or tag this
	// event came from, and is nil for manually-created events.
	SourceRef *string `json:"sourceRef,omitempty"`

	AttachmentAssetID  *string `json:"attachmentAssetId,omitempty"`
	AttachmentFileName *string `json:"attachmentFileName,omitempty"`
	AttachmentFileType *string `json:"attachmentFileType,omitempty"`

	CreatedBy string    `json:"createdBy"`
	UpdatedBy *string   `json:"updatedBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Input carries the fields settable via the create/update REST endpoints.
type Input struct {
	Type              EventType
	Title             string
	Summary           string
	EventDate         time.Time
	Version           *string
	ADRNumber         *string
	DecisionStatus    *DecisionStatus
	SourceLabel       *string
	SourceURL         *string
	IsAgentSummarized bool
	Touches           []Touch

	AttachmentAssetID  *string
	AttachmentFileName *string
	AttachmentFileType *string

	// SourceRef is required by the sync endpoint and ignored by create/update.
	SourceRef string
}

// Validate reports whether in carries the required fields for its Type.
func (in Input) Validate() error {
	if in.Title == "" {
		return ErrTitleRequired
	}
	switch in.Type {
	case EventTypeRelease, EventTypeDecision, EventTypeIncident:
	default:
		return ErrUnknownEventType
	}
	if in.DecisionStatus != nil {
		switch *in.DecisionStatus {
		case DecisionStatusProposed, DecisionStatusAccepted, DecisionStatusSuperseded, DecisionStatusDeprecated:
		default:
			return ErrUnknownDecisionStatus
		}
	}
	return nil
}
