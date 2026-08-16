package githubapp

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/google/uuid"
)

type APIClient interface {
	StartRun(ctx context.Context, installationID int64, onboarding Onboarding, orgName string) error
	MissingAIConfiguration(ctx context.Context, installationID int64, repository Repository, account string) ([]string, error)
	OpenPullRequest(ctx context.Context, installationID int64, onboarding Onboarding) (string, error)
	PutOnboardingCredentials(ctx context.Context, installationID int64, installation Installation, repositories []Repository, plaintext string) error
}

type Worker struct {
	store  Store
	client APIClient
	owner  string
}

func NewWorker(store Store, client APIClient) *Worker {
	return &Worker{store: store, client: client, owner: uuid.NewString()}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if err := w.runOne(ctx); err != nil {
			slog.ErrorContext(ctx, "GitHub onboarding job failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) runOne(ctx context.Context) error {
	job, err := w.store.ClaimJob(ctx, w.owner, 2*time.Minute)
	if err != nil || job == nil {
		return err
	}
	err = w.process(ctx, *job)
	if err == nil {
		return w.store.CompleteJob(ctx, job.ID, w.owner)
	}
	delay := time.Duration(math.Min(math.Pow(2, float64(job.Attempts)), 300)) * time.Second
	if retryErr := w.store.RetryJob(ctx, *job, w.owner, time.Now().Add(delay), err.Error()); retryErr != nil {
		return fmt.Errorf("process: %v; persist retry: %w", err, retryErr)
	}
	return err
}

func (w *Worker) process(ctx context.Context, job Job) error {
	onboarding, err := w.store.GetOnboarding(ctx, job.OrgID, job.OnboardingID)
	if err != nil {
		return err
	}
	installation, err := w.store.GetInstallation(ctx, job.OrgID)
	if err != nil {
		return err
	}
	if installation == nil || installation.Status != "active" {
		return fmt.Errorf("GitHub App installation is not active")
	}
	switch job.Kind {
	case JobStart:
		if err := w.store.SetOnboardingStatus(ctx, job.OrgID, job.OnboardingID, StateCheckingAI, nil); err != nil {
			return err
		}
		batch, err := w.store.GetBatch(ctx, job.OrgID, onboarding.BatchID)
		if err != nil {
			return err
		}
		repositories := make([]Repository, 0, len(batch.Items))
		for _, item := range batch.Items {
			repositories = append(repositories, item.Repository)
		}
		missing, err := w.client.MissingAIConfiguration(ctx, installation.GitHubInstallationID, onboarding.Repository, installation.AccountLogin)
		if err != nil {
			return err
		}
		if len(missing) != 0 {
			return w.store.SetOnboardingStatus(ctx, job.OrgID, job.OnboardingID, StateWaitingAI, missing)
		}
		plaintext, err := w.store.CreateOnboardingToken(ctx, job.OrgID, time.Now().AddDate(10, 0, 0))
		if err != nil {
			return err
		}
		if err := w.client.PutOnboardingCredentials(ctx, installation.GitHubInstallationID, *installation, repositories, plaintext); err != nil {
			return err
		}
		orgName, err := w.store.GetOrgName(ctx, job.OrgID)
		if err != nil {
			return err
		}
		if err := w.client.StartRun(ctx, installation.GitHubInstallationID, *onboarding, orgName); err != nil {
			return err
		}
		return w.store.SetOnboardingStatus(ctx, job.OrgID, job.OnboardingID, StateRunQueued, nil)
	case JobOpenPR:
		url, err := w.client.OpenPullRequest(ctx, installation.GitHubInstallationID, *onboarding)
		if err != nil {
			slog.WarnContext(ctx, "GitHub onboarding pull request could not be opened", "onboarding", onboarding.ID, "err", err)
			return nil
		}
		return w.store.SetOnboardingPullRequest(ctx, job.OrgID, job.OnboardingID, url)
	default:
		return fmt.Errorf("unknown GitHub onboarding job kind %q", job.Kind)
	}
}
