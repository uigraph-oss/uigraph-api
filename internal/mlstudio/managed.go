package mlstudio

import "time"

type Deployment struct {
	ID           string     `json:"id"`
	OrgID        string     `json:"orgId"`
	ModelID      string     `json:"modelId"`
	VersionID    string     `json:"versionId"`
	Name         string     `json:"name"`
	Environment  string     `json:"environment"`
	Status       string     `json:"status"`
	Endpoint     string     `json:"endpoint"`
	Region       string     `json:"region"`
	DeployedAt   *time.Time `json:"deployedAt,omitempty"`
	RolledBackAt *time.Time `json:"rolledBackAt,omitempty"`
	CreatedBy    string     `json:"createdBy"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	DeletedAt    *time.Time `json:"deletedAt,omitempty"`
}

type Finding struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"orgId"`
	ModelID     string     `json:"modelId"`
	VersionID   *string    `json:"versionId,omitempty"`
	Title       string     `json:"title"`
	Summary     string     `json:"summary"`
	Description string     `json:"description"`
	RunIDs      []string   `json:"runIds"`
	CreatedBy   string     `json:"createdBy"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	DeletedAt   *time.Time `json:"deletedAt,omitempty"`
}
