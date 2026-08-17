package githubapp

import (
	"context"
	"time"
)

type Store interface {
	GetOrgName(ctx context.Context, orgID string) (string, error)
	CreateInstallState(ctx context.Context, orgID, userID, stateHash string, expiresAt time.Time) error
	AuthorizeInstallState(ctx context.Context, stateHash string, githubUserID int64) (string, error)
	ConsumeInstallState(ctx context.Context, stateHash string, githubUserID int64) (orgID string, err error)
	UpsertInstallation(ctx context.Context, installation Installation, repositories []Repository) error
	GetInstallation(ctx context.Context, orgID string) (*Installation, error)
	DisconnectInstallation(ctx context.Context, orgID string) error
	ListRepositories(ctx context.Context, orgID string) ([]Repository, error)
	CreateImport(ctx context.Context, orgID, teamID, repositoryID string) (*Import, error)
	GetImport(ctx context.Context, orgID, importID string) (*Import, error)
	GetLatestImport(ctx context.Context, orgID string) (*Import, error)
	ListImports(ctx context.Context, orgID string) ([]Import, error)
	SetImportStatus(ctx context.Context, orgID, importID string, status State, missingAIConfiguration []string) error
	SetImportPullRequest(ctx context.Context, orgID, importID, url string) error
	RetryImport(ctx context.Context, orgID, importID string, recheck bool) error
	ClaimJob(ctx context.Context, owner string, lease time.Duration) (*Job, error)
	CompleteJob(ctx context.Context, jobID, owner string) error
	RetryJob(ctx context.Context, job Job, owner string, next time.Time, message string) error
	RecordWebhook(ctx context.Context, deliveryID, event, action string, installationID int64) (bool, error)
	ApplyWorkflowRunEvent(ctx context.Context, installationID, repositoryID int64, run WorkflowRun) error
	ApplyWorkflowJobEvent(ctx context.Context, installationID, repositoryID, runID int64, headBranch string, steps []Step) error
	CreateImportToken(ctx context.Context, orgID string, expiresAt time.Time) (plaintext string, err error)
}
