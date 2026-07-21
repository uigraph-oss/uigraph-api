package mlstudio

import "context"

type Store interface {
	UpsertMLModels(ctx context.Context, orgID, actorID string, in []ModelInput) error
	UpdateMLModel(ctx context.Context, orgID, id, actorID string, in ModelUpdateInput) error
	UpsertMLModelVersions(ctx context.Context, orgID, actorID string, in []ModelVersionInput) error
	UpsertMLExperiments(ctx context.Context, orgID, actorID string, in []ExperimentInput) error
	UpsertMLRuns(ctx context.Context, orgID, actorID string, in []RunInput) error
	UpsertMLRunMetricPoints(ctx context.Context, orgID, runMLflowID string, in []MetricPoint) error
	UpsertMLArtifacts(ctx context.Context, orgID, actorID string, in []ArtifactInput) error
	UpsertMLDatasets(ctx context.Context, orgID, actorID string, in []DatasetInput) error
	UpsertMLEvaluations(ctx context.Context, orgID, actorID string, in []EvaluationInput) error

	ListMLModels(ctx context.Context, orgID string) ([]Model, error)
	GetMLModel(ctx context.Context, orgID, id string) (*Model, error)
	ListMLModelVersions(ctx context.Context, orgID, modelID string) ([]ModelVersion, error)
	GetMLModelVersion(ctx context.Context, orgID, id string) (*ModelVersion, error)
	ListMLExperiments(ctx context.Context, orgID string) ([]Experiment, error)
	GetMLExperiment(ctx context.Context, orgID, id string) (*Experiment, error)
	ListMLRuns(ctx context.Context, orgID, experimentID string) ([]Run, error)
	GetMLRun(ctx context.Context, orgID, id string) (*Run, error)
	ListMLRunMetricPoints(ctx context.Context, orgID, runID string) ([]MetricPoint, error)
	ListMLArtifacts(ctx context.Context, orgID, runID string) ([]Artifact, error)
	ListMLDatasets(ctx context.Context, orgID string) ([]Dataset, error)
	GetMLDataset(ctx context.Context, orgID, id string) (*Dataset, error)
	ListMLVersionEvaluations(ctx context.Context, orgID, versionID string) ([]Evaluation, error)

	CreateMLDeployment(ctx context.Context, d Deployment) error
	GetMLDeployment(ctx context.Context, orgID, id string) (*Deployment, error)
	ListMLDeployments(ctx context.Context, orgID, modelID, versionID string) ([]Deployment, error)
	UpdateMLDeployment(ctx context.Context, d Deployment) error
	DeleteMLDeployment(ctx context.Context, orgID, id, deletedBy string) error

	CreateMLFinding(ctx context.Context, f Finding) error
	GetMLFinding(ctx context.Context, orgID, id string) (*Finding, error)
	ListMLFindings(ctx context.Context, orgID, modelID string) ([]Finding, error)
	UpdateMLFinding(ctx context.Context, f Finding) error
	DeleteMLFinding(ctx context.Context, orgID, id, deletedBy string) error
}
