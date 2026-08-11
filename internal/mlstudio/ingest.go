package mlstudio

import "time"

type SyncCounts struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
}

type RunPruneInput struct {
	MLflowID string `json:"mlflowId"`
}

type VersionPruneInput struct {
	ModelMLflowID string   `json:"modelMlflowId"`
	Keep          []string `json:"keep"`
}

type ProjectInput struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	SourceType  string  `json:"sourceType"`
	SourceURL   string  `json:"sourceUrl"`
	TeamID      *string `json:"teamId,omitempty"`
	TeamName    string  `json:"team"`
}

type ModelInput struct {
	MLflowID                  string     `json:"mlflowId"`
	ProjectName               string     `json:"projectName"`
	Name                      string     `json:"name"`
	Description               string     `json:"description"`
	Tags                      []string   `json:"tags"`
	ProblemType               string     `json:"problemType"`
	Domain                    string     `json:"domain"`
	License                   string     `json:"license"`
	IntendedUse               string     `json:"intendedUse"`
	Limitations               string     `json:"limitations"`
	Recommendations           string     `json:"recommendations"`
	Considerations            string     `json:"considerations"`
	ProductionVersionMLflowID *string    `json:"productionVersionMlflowId"`
	CreatedAt                 *time.Time `json:"createdAt"`
	UpdatedAt                 *time.Time `json:"updatedAt"`
	UserEmail                 string     `json:"userEmail"`
	ActorID                   string     `json:"-"`
}

type ModelUpdateInput struct {
	Domain          string   `json:"domain"`
	ProblemType     string   `json:"problemType"`
	License         string   `json:"license"`
	References      []string `json:"references"`
	IntendedUse     string   `json:"intendedUse"`
	Limitations     string   `json:"limitations"`
	Considerations  string   `json:"considerations"`
	Recommendations string   `json:"recommendations"`
}

type ModelVersionInput struct {
	MLflowID      string     `json:"mlflowId"`
	ModelMLflowID string     `json:"modelMlflowId"`
	RunMLflowID   *string    `json:"runMlflowId"`
	Version       string     `json:"version"`
	Description   string     `json:"description"`
	CreatedAt     *time.Time `json:"createdAt"`
	UserEmail     string     `json:"userEmail"`
	ActorID       string     `json:"-"`
}

type ExperimentInput struct {
	MLflowID    string   `json:"mlflowId"`
	ProjectName string   `json:"projectName"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Tags        []string `json:"tags"`
	UserEmail   string   `json:"userEmail"`
	ActorID     string   `json:"-"`
}

type RunInput struct {
	MLflowID           string         `json:"mlflowId"`
	ExperimentMLflowID string         `json:"experimentMlflowId"`
	DatasetMLflowID    *string        `json:"datasetMlflowId"`
	Name               string         `json:"name"`
	Status             string         `json:"status"`
	StartedAt          *time.Time     `json:"startedAt"`
	EndedAt            *time.Time     `json:"endedAt"`
	Notes              string         `json:"notes"`
	Tags               []string       `json:"tags"`
	Parameters         map[string]any `json:"parameters"`
	Metrics            map[string]any `json:"metrics"`
	UserEmail          string         `json:"userEmail"`
	ActorID            string         `json:"-"`
}

type ArtifactInput struct {
	MLflowID    string `json:"mlflowId"`
	RunMLflowID string `json:"runMlflowId"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	URI         string `json:"uri"`
	DownloadURI string `json:"downloadUri"`
	Size        string `json:"size"`
	Format      string `json:"format"`
	UserEmail   string `json:"userEmail"`
	ActorID     string `json:"-"`
}

type DatasetInput struct {
	MLflowID           string        `json:"mlflowId"`
	ExperimentMLflowID string        `json:"experimentMlflowId"`
	Name               string        `json:"name"`
	Digest             string        `json:"digest"`
	Source             string        `json:"source"`
	SourceType         string        `json:"sourceType"`
	Context            string        `json:"context"`
	RowCount           int64         `json:"rowCount"`
	Schema             []SchemaField `json:"schema"`
	Tags               []string      `json:"tags"`
	UserEmail          string        `json:"userEmail"`
	ActorID            string        `json:"-"`
}

type EvaluationInput struct {
	MLflowID           string         `json:"mlflowId"`
	VersionMLflowID    string         `json:"versionMlflowId"`
	ExperimentMLflowID string         `json:"experimentMlflowId"`
	DatasetMLflowID    *string        `json:"datasetMlflowId"`
	Name               string         `json:"name"`
	Type               string         `json:"type"`
	Description        string         `json:"description"`
	Summary            string         `json:"summary"`
	StartedAt          *time.Time     `json:"startedAt"`
	EndedAt            *time.Time     `json:"endedAt"`
	Evaluator          string         `json:"evaluator"`
	Tags               []string       `json:"tags"`
	Parameters         map[string]any `json:"parameters"`
	Metrics            map[string]any `json:"metrics"`
	UserEmail          string         `json:"userEmail"`
	ActorID            string         `json:"-"`
}
