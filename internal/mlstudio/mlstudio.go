package mlstudio

import "time"

type Model struct {
	ID                    string     `json:"id"`
	OrgID                 string     `json:"orgId"`
	MLflowID              string     `json:"mlflowId"`
	Name                  string     `json:"name"`
	Description           string     `json:"description"`
	Domain                string     `json:"domain"`
	ProblemType           string     `json:"problemType"`
	Tags                  []string   `json:"tags"`
	Owners                string     `json:"owners"`
	License               string     `json:"license"`
	References            []string   `json:"references"`
	IntendedUse           string     `json:"intendedUse"`
	Limitations           string     `json:"limitations"`
	EthicalConsiderations string     `json:"ethicalConsiderations"`
	Caveats               string     `json:"caveats"`
	ProductionVersionID   *string    `json:"productionVersionId,omitempty"`
	CreatedAt             *time.Time `json:"createdAt,omitempty"`
	UpdatedAt             *time.Time `json:"updatedAt,omitempty"`
}

type ModelVersion struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"orgId"`
	MLflowID    string     `json:"mlflowId"`
	ModelID     string     `json:"modelId"`
	Version     string     `json:"version"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Stage       string     `json:"stage"`
	RunID       *string    `json:"runId,omitempty"`
	CreatedAt   *time.Time `json:"createdAt,omitempty"`
}

type Experiment struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"orgId"`
	MLflowID    string     `json:"mlflowId"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
}

type Run struct {
	ID           string         `json:"id"`
	OrgID        string         `json:"orgId"`
	MLflowID     string         `json:"mlflowId"`
	ExperimentID string         `json:"experimentId"`
	Name         string         `json:"name"`
	Status       string         `json:"status"`
	StartedAt    *time.Time     `json:"startedAt,omitempty"`
	EndedAt      *time.Time     `json:"endedAt,omitempty"`
	Duration     string         `json:"duration"`
	Notes        string         `json:"notes"`
	Parameters   map[string]any `json:"parameters"`
	Metrics      map[string]any `json:"metrics"`
	DatasetID    *string        `json:"datasetId,omitempty"`
}

type MetricPoint struct {
	Key   string     `json:"key"`
	Step  int64      `json:"step"`
	Value float64    `json:"value"`
	TS    *time.Time `json:"ts,omitempty"`
}

type Artifact struct {
	ID       string `json:"id"`
	OrgID    string `json:"orgId"`
	MLflowID string `json:"mlflowId"`
	RunID    string `json:"runId"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	URI      string `json:"uri"`
	Size     string `json:"size"`
	Format   string `json:"format"`
}

type SchemaField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type Dataset struct {
	ID       string        `json:"id"`
	OrgID    string        `json:"orgId"`
	MLflowID string        `json:"mlflowId"`
	Name     string        `json:"name"`
	Source   string        `json:"source"`
	Type     string        `json:"type"`
	RowCount int64         `json:"rowCount"`
	Schema   []SchemaField `json:"schema"`
}

type EvaluationDataset struct {
	ID         string            `json:"id"`
	OrgID      string            `json:"orgId"`
	MLflowID   string            `json:"mlflowId"`
	Name       string            `json:"name"`
	Digest     string            `json:"digest"`
	Source     string            `json:"source"`
	SourceType string            `json:"sourceType"`
	RowCount   int64             `json:"rowCount"`
	Schema     []SchemaField     `json:"schema"`
	Tags       map[string]string `json:"tags"`
}

type Metric struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Value      float64    `json:"value"`
	Unit       string     `json:"unit"`
	Direction  string     `json:"direction"`
	Category   string     `json:"category"`
	MeasuredAt *time.Time `json:"measuredAt,omitempty"`
}

type Evaluation struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"orgId"`
	MLflowID    string     `json:"mlflowId"`
	VersionID   string     `json:"versionId"`
	DatasetID   *string    `json:"datasetId,omitempty"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	Description string     `json:"description"`
	Summary     string     `json:"summary"`
	EvaluatedAt *time.Time `json:"evaluatedAt,omitempty"`
	Evaluator   string     `json:"evaluator"`
	Metrics     []Metric   `json:"metrics"`
}
