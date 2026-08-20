package githubapp

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

type APIClient interface {
	StartRun(ctx context.Context, installationID int64, value Import, repository Repository) error
	OpenPullRequest(ctx context.Context, installationID int64, value Import, repository Repository) (string, error)
	GetInstallationAccount(ctx context.Context, installationID int64) (Installation, error)
	GetRepository(ctx context.Context, installationID int64, owner, repo string) (Repository, error)
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

func (w *Worker) resolveRepository(ctx context.Context, installationID int64, value Import) (Repository, error) {
	account, err := w.client.GetInstallationAccount(ctx, installationID)
	if err != nil {
		return Repository{}, err
	}
	if account.AccountID != value.GitHubOwnerID {
		return Repository{}, fmt.Errorf("repository %q is not owned by the connected GitHub account", value.GitHubRepo)
	}
	return w.client.GetRepository(ctx, installationID, account.AccountLogin, value.GitHubRepo)
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
	if installation == nil {
		return fmt.Errorf("GitHub App is not connected")
	}
	repository, err := w.resolveRepository(ctx, installation.GitHubInstallationID, *value)
	if err != nil {
		return err
	}
	switch job.Kind {
	case JobStart:
		if err := w.client.StartRun(ctx, installation.GitHubInstallationID, *value, repository); err != nil {
			return err
		}
		return w.store.SetImportRunQueued(ctx, job.OrgID, job.ImportID)
	case JobOpenPR:
		url, err := w.client.OpenPullRequest(ctx, installation.GitHubInstallationID, *value, repository)
		if err != nil {
			return err
		}
		return w.store.SetImportPullRequest(ctx, job.OrgID, job.ImportID, url)
	default:
		return fmt.Errorf("unknown GitHub import job kind %q", job.Kind)
	}
}
