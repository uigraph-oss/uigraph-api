package githubapp

import (
	"context"
	"time"
)

type Store interface {
	CreateInstallState(ctx context.Context, orgID, userID, stateHash string, expiresAt time.Time) error
	AuthorizeInstallState(ctx context.Context, stateHash string, githubUserID int64) (string, error)
	ConsumeInstallState(ctx context.Context, stateHash string, githubUserID int64) (orgID string, err error)
	UpsertInstallation(ctx context.Context, orgID string, installationID int64) error
	GetInstallation(ctx context.Context, orgID string) (*Installation, error)
	DeleteInstallation(ctx context.Context, orgID string) error
	CreateImport(ctx context.Context, orgID, teamID string, ownerID int64, repo string) (*Import, error)
	GetImport(ctx context.Context, orgID, importID string) (*Import, error)
	GetLatestImport(ctx context.Context, orgID string) (*Import, error)
	SetImportRunQueued(ctx context.Context, orgID, importID string) error
	SetImportPullRequest(ctx context.Context, orgID, importID, url string) error
	RetryImport(ctx context.Context, orgID, importID string) error
	ResumeImportRun(ctx context.Context, orgID, importID string) error
	ClaimJob(ctx context.Context, owner string, lease time.Duration) (*Job, error)
	CompleteJob(ctx context.Context, jobID, owner string) error
	RetryJob(ctx context.Context, job Job, owner string, next time.Time, message string) error
	RecordWebhook(ctx context.Context, deliveryID, event, action string, installationID int64) (bool, error)
	ApplyWorkflowRunEvent(ctx context.Context, run WorkflowRun, repositoryURL string) error
	ApplyWorkflowJobEvent(ctx context.Context, branch string, runID int64, runAttempt int, steps []Step) error
	CreateImportToken(ctx context.Context, orgID, repository string, expiresAt time.Time) (plaintext string, err error)
}
