package githubapp

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

type APIClient interface {
	StartRun(ctx context.Context, installationID int64, value Import, orgName string) error
	MissingAIConfiguration(ctx context.Context, installationID int64, repository Repository, account string) ([]string, error)
	OpenPullRequest(ctx context.Context, installationID int64, value Import) (string, error)
	PutImportCredentials(ctx context.Context, installationID int64, repository Repository, plaintext string) error
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
			slog.ErrorContext(ctx, "GitHub import job failed", "err", err)
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
	delay := min(time.Duration(1<<job.Attempts), 300) * time.Second
	if retryErr := w.store.RetryJob(ctx, *job, w.owner, time.Now().Add(delay), err.Error()); retryErr != nil {
		return fmt.Errorf("process: %v; persist retry: %w", err, retryErr)
	}
	return err
}

func (w *Worker) process(ctx context.Context, job Job) error {
	value, err := w.store.GetImport(ctx, job.OrgID, job.ImportID)
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
		if err := w.store.SetImportStatus(ctx, job.OrgID, job.ImportID, StateCheckingAI, nil); err != nil {
			return err
		}
		missing, err := w.client.MissingAIConfiguration(ctx, installation.GitHubInstallationID, value.Repository, installation.AccountLogin)
		if err != nil {
			return err
		}
		if len(missing) != 0 {
			return w.store.SetImportStatus(ctx, job.OrgID, job.ImportID, StateWaitingAI, missing)
		}
		plaintext, err := w.store.CreateImportToken(ctx, job.OrgID, time.Now().AddDate(10, 0, 0))
		if err != nil {
			return err
		}
		if err := w.client.PutImportCredentials(ctx, installation.GitHubInstallationID, value.Repository, plaintext); err != nil {
			return err
		}
		orgName, err := w.store.GetOrgName(ctx, job.OrgID)
		if err != nil {
			return err
		}
		if err := w.client.StartRun(ctx, installation.GitHubInstallationID, *value, orgName); err != nil {
			return err
		}
		return w.store.SetImportStatus(ctx, job.OrgID, job.ImportID, StateRunQueued, nil)
	case JobOpenPR:
		url, err := w.client.OpenPullRequest(ctx, installation.GitHubInstallationID, *value)
		if err != nil {
			slog.WarnContext(ctx, "GitHub import pull request could not be opened", "import", value.ID, "err", err)
			return nil
		}
		return w.store.SetImportPullRequest(ctx, job.OrgID, job.ImportID, url)
	default:
		return fmt.Errorf("unknown GitHub import job kind %q", job.Kind)
	}
}
