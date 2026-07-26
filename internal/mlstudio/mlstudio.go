package mlstudio

import "time"

type Project struct {
	ID          string        `json:"id"`
	OrgID       string        `json:"orgId"`
	Name        string        `json:"name"`
	Type        string        `json:"type"`
	Description string        `json:"description"`
	SourceType  string        `json:"sourceType"`
	SourceURL   string        `json:"sourceUrl"`
	TeamID      *string       `json:"teamId,omitempty"`
	TeamName    string        `json:"-"`
	UpdatedAt   *time.Time    `json:"updatedAt,omitempty"`
	Stats       *ProjectStats `json:"stats,omitempty"`
}

type ProjectStats struct {
	ModelCount      int `json:"modelCount"`
	ExperimentCount int `json:"experimentCount"`
	RunCount        int `json:"runCount"`
}

type Model struct {
	ID                  string     `json:"id"`
	OrgID               string     `json:"orgId"`
	MLflowID            *string    `json:"mlflowId,omitempty"`
	ProjectID           *string    `json:"projectId,omitempty"`
	Name                string     `json:"name"`
	Description         string     `json:"description"`
	Domain              string     `json:"domain"`
	ProblemType         string     `json:"problemType"`
	Tags                []string   `json:"tags"`
	License             string     `json:"license"`
	References          []string   `json:"references"`
	IntendedUse         string     `json:"intendedUse"`
	Limitations         string     `json:"limitations"`
	Considerations      string     `json:"considerations"`
	Recommendations     string     `json:"recommendations"`
	ProductionVersionID *string    `json:"productionVersionId,omitempty"`
	Origin              string     `json:"origin"`
	CreatedBy           string     `json:"-"`
	CreatedAt           *time.Time `json:"createdAt,omitempty"`
	UpdatedAt           *time.Time `json:"updatedAt,omitempty"`
}

type ModelVersion struct {
	ID               string     `json:"id"`
	OrgID            string     `json:"orgId"`
	MLflowID         string     `json:"mlflowId"`
	ModelID          string     `json:"modelId"`
	Version          string     `json:"version"`
	Description      string     `json:"description"`
	DeploymentStatus string     `json:"deploymentStatus"`
	RunID            *string    `json:"runId,omitempty"`
	CreatedAt        *time.Time `json:"createdAt,omitempty"`
}

type VersionDeploymentUpdate struct {
	ID         string     `json:"id"`
	OrgID      string     `json:"orgId"`
	VersionID  string     `json:"versionId"`
	FromStatus *string    `json:"fromStatus,omitempty"`
	ToStatus   string     `json:"toStatus"`
	ChangedBy  string     `json:"changedBy"`
	ChangedAt  *time.Time `json:"changedAt,omitempty"`
}

type Experiment struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"orgId"`
	MLflowID    *string    `json:"mlflowId,omitempty"`
	ProjectID   *string    `json:"projectId,omitempty"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Tags        []string   `json:"tags"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	Source      string     `json:"source"`
	CreatedBy   string     `json:"createdBy,omitempty"`
	CreatedAt   *time.Time `json:"createdAt,omitempty"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty"`
	DeletedAt   *time.Time `json:"deletedAt,omitempty"`
}

type Run struct {
	ID           string         `json:"id"`
	OrgID        string         `json:"orgId"`
	MLflowID     *string        `json:"mlflowId,omitempty"`
	ExperimentID string         `json:"experimentId"`
	Name         string         `json:"name"`
	Status       string         `json:"status"`
	StartedAt    *time.Time     `json:"startedAt,omitempty"`
	EndedAt      *time.Time     `json:"endedAt,omitempty"`
	Notes        string         `json:"notes"`
	Parameters   map[string]any `json:"parameters"`
	Metrics      map[string]any `json:"metrics"`
	DatasetID    *string        `json:"datasetId,omitempty"`
	Source       string         `json:"source"`
	CreatedBy    string         `json:"createdBy,omitempty"`
	CreatedAt    *time.Time     `json:"createdAt,omitempty"`
	UpdatedAt    *time.Time     `json:"updatedAt,omitempty"`
	SyncedAt     *time.Time     `json:"syncedAt,omitempty"`
	DeletedAt    *time.Time     `json:"deletedAt,omitempty"`
}

type RunQuery struct {
	ExperimentID string
	ProjectID    string
	Search       string
	Limit        int
	Offset       int
}

type Artifact struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"orgId"`
	MLflowID    string     `json:"mlflowId"`
	RunID       string     `json:"runId"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	URI         string     `json:"uri"`
	DownloadURI string     `json:"downloadUri"`
	Size        string     `json:"size"`
	Format      string     `json:"format"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty"`
	SyncedAt    *time.Time `json:"syncedAt,omitempty"`
}

type SchemaField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type Dataset struct {
	ID           string            `json:"id"`
	OrgID        string            `json:"orgId"`
	ExperimentID string            `json:"experimentId"`
	MLflowID     *string           `json:"mlflowId,omitempty"`
	Name         string            `json:"name"`
	Digest       string            `json:"digest"`
	Source       string            `json:"source"`
	SourceType   string            `json:"sourceType"`
	Context      string            `json:"context"`
	RowCount     int64             `json:"rowCount"`
	Schema       []SchemaField     `json:"schema"`
	Tags         map[string]string `json:"tags"`
	Origin       string            `json:"origin"`
	CreatedBy    string            `json:"createdBy,omitempty"`
	CreatedAt    *time.Time        `json:"createdAt,omitempty"`
	UpdatedAt    *time.Time        `json:"updatedAt,omitempty"`
	DeletedAt    *time.Time        `json:"deletedAt,omitempty"`
}

type Evaluation struct {
	ID           string         `json:"id"`
	OrgID        string         `json:"orgId"`
	MLflowID     string         `json:"mlflowId"`
	VersionID    string         `json:"versionId"`
	ExperimentID string         `json:"experimentId"`
	ModelName    string         `json:"modelName"`
	Version      string         `json:"version"`
	DatasetID    *string        `json:"datasetId,omitempty"`
	Name         string         `json:"name"`
	Type         string         `json:"type"`
	Description  string         `json:"description"`
	Summary      string         `json:"summary"`
	EvaluatedAt  *time.Time     `json:"evaluatedAt,omitempty"`
	Evaluator    string         `json:"evaluator"`
	Parameters   map[string]any `json:"parameters"`
	Metrics      map[string]any `json:"metrics"`
}
