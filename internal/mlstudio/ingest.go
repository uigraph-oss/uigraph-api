package mlstudio

import "time"

type ModelInput struct {
	MLflowID                  string     `json:"mlflowId"`
	Name                      string     `json:"name"`
	Description               string     `json:"description"`
	Tags                      []string   `json:"tags"`
	ProductionVersionMLflowID *string    `json:"productionVersionMlflowId"`
	CreatedAt                 *time.Time `json:"createdAt"`
	UpdatedAt                 *time.Time `json:"updatedAt"`
}

type ModelUpdateInput struct {
	Domain                string   `json:"domain"`
	ProblemType           string   `json:"problemType"`
	Owners                string   `json:"owners"`
	License               string   `json:"license"`
	References            []string `json:"references"`
	IntendedUse           string   `json:"intendedUse"`
	Limitations           string   `json:"limitations"`
	EthicalConsiderations string   `json:"ethicalConsiderations"`
	Caveats               string   `json:"caveats"`
}

type ModelVersionInput struct {
	MLflowID      string     `json:"mlflowId"`
	ModelMLflowID string     `json:"modelMlflowId"`
	RunMLflowID   *string    `json:"runMlflowId"`
	Version       string     `json:"version"`
	Description   string     `json:"description"`
	CreatedAt     *time.Time `json:"createdAt"`
}

type ExperimentInput struct {
	MLflowID    string     `json:"mlflowId"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	StartedAt   *time.Time `json:"startedAt"`
}

type RunInput struct {
	MLflowID           string         `json:"mlflowId"`
	ExperimentMLflowID string         `json:"experimentMlflowId"`
	DatasetMLflowID    *string        `json:"datasetMlflowId"`
	Name               string         `json:"name"`
	Status             string         `json:"status"`
	StartedAt          *time.Time     `json:"startedAt"`
	EndedAt            *time.Time     `json:"endedAt"`
	Duration           string         `json:"duration"`
	Notes              string         `json:"notes"`
	Parameters         map[string]any `json:"parameters"`
	Metrics            map[string]any `json:"metrics"`
}

type ArtifactInput struct {
	MLflowID    string `json:"mlflowId"`
	RunMLflowID string `json:"runMlflowId"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	URI         string `json:"uri"`
	Size        string `json:"size"`
	Format      string `json:"format"`
}

type DatasetInput struct {
	MLflowID           string            `json:"mlflowId"`
	ExperimentMLflowID string            `json:"experimentMlflowId"`
	Name               string            `json:"name"`
	Digest             string            `json:"digest"`
	Source             string            `json:"source"`
	SourceType         string            `json:"sourceType"`
	Context            string            `json:"context"`
	RowCount           int64             `json:"rowCount"`
	Schema             []SchemaField     `json:"schema"`
	Tags               map[string]string `json:"tags"`
}

type EvaluationInput struct {
	MLflowID        string        `json:"mlflowId"`
	VersionMLflowID string        `json:"versionMlflowId"`
	DatasetMLflowID *string       `json:"datasetMlflowId"`
	Name            string        `json:"name"`
	Type            string        `json:"type"`
	Description     string        `json:"description"`
	Summary         string        `json:"summary"`
	EvaluatedAt     *time.Time    `json:"evaluatedAt"`
	Evaluator       string        `json:"evaluator"`
	Metrics         []MetricInput `json:"metrics"`
}

type MetricInput struct {
	Name       string     `json:"name"`
	Value      float64    `json:"value"`
	Unit       string     `json:"unit"`
	Direction  string     `json:"direction"`
	Category   string     `json:"category"`
	MeasuredAt *time.Time `json:"measuredAt"`
}
