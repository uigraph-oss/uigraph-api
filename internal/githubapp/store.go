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
	DeleteInstallation(ctx context.Context, orgID string) error
	ListRepositories(ctx context.Context, orgID string) ([]Repository, error)
	CreateBatch(ctx context.Context, orgID, teamID, teamName, createdBy string, repositoryIDs []string) (*Batch, error)
	GetBatch(ctx context.Context, orgID, batchID string) (*Batch, error)
	GetLatestBatch(ctx context.Context, orgID string) (*Batch, error)
	GetOnboarding(ctx context.Context, orgID, onboardingID string) (*Onboarding, error)
	SetOnboardingStatus(ctx context.Context, orgID, onboardingID string, status State, missingAIConfiguration []string) error
	SetOnboardingPullRequest(ctx context.Context, orgID, onboardingID, url string) error
	RetryOnboarding(ctx context.Context, orgID, onboardingID string, recheck bool) error
	ClaimJob(ctx context.Context, owner string, lease time.Duration) (*Job, error)
	CompleteJob(ctx context.Context, jobID, owner string) error
	RetryJob(ctx context.Context, job Job, owner string, next time.Time, message string) error
	RecordWebhook(ctx context.Context, deliveryID, event, action string, installationID int64) (bool, error)
	ApplyWorkflowRunEvent(ctx context.Context, installationID, repositoryID, runID int64, event, status, conclusion, headBranch, htmlURL, path string) error
	CreateOnboardingToken(ctx context.Context, orgID string, expiresAt time.Time) (plaintext string, err error)
}
