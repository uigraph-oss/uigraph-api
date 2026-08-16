package githubapp

import (
	"context"
	"errors"
	"testing"
	"time"
)

type retryStore struct {
	Store
	job       *Job
	retried   bool
	completed bool
	enqueued  string
	status    State
}

func (s *retryStore) ClaimJob(context.Context, string, time.Duration) (*Job, error) {
	job := s.job
	s.job = nil
	return job, nil
}

func (s *retryStore) GetOnboarding(context.Context, string, string) (*Onboarding, error) {
	return &Onboarding{ID: "onboarding", OrgID: "org", Status: StateSelected, Repository: Repository{FullName: "acme/repo", DefaultBranch: "main"}}, nil
}

func (s *retryStore) GetInstallation(context.Context, string) (*Installation, error) {
	return &Installation{GitHubInstallationID: 7, Status: "active"}, nil
}

func (s *retryStore) UpdateOnboarding(_ context.Context, onboarding Onboarding) error {
	s.status = onboarding.Status
	return nil
}

func (s *retryStore) EnqueueJob(_ context.Context, _, _, kind string) error {
	s.enqueued = kind
	return nil
}

func (s *retryStore) GetOrgName(context.Context, string) (string, error) { return "Acme", nil }

func (s *retryStore) RetryJob(context.Context, Job, string, time.Time, string) error {
	s.retried = true
	return nil
}

func (s *retryStore) CompleteJob(context.Context, string, string) error {
	s.completed = true
	return nil
}

type failingClient struct{}

func (failingClient) CreateSetupPullRequest(context.Context, int64, Onboarding, string) (PullRequest, error) {
	return PullRequest{}, errors.New("temporary GitHub failure")
}

func (failingClient) MissingAIConfiguration(context.Context, int64, Repository, string) ([]string, error) {
	return nil, nil
}

func (failingClient) DispatchWorkflow(context.Context, int64, Repository, string) error { return nil }

func (failingClient) FindArtifactsPullRequest(context.Context, int64, Onboarding) (PullRequest, error) {
	return PullRequest{}, nil
}

func (failingClient) PutOnboardingSecret(context.Context, int64, Installation, []Repository, string) error {
	return nil
}

func TestWorkerRetriesFailedJobWithoutCompletingIt(t *testing.T) {
	store := &retryStore{job: &Job{ID: "job", OrgID: "org", OnboardingID: "onboarding", Kind: JobSetupPR, Attempts: 1, MaxAttempts: 8}}
	worker := NewWorker(store, failingClient{})
	if err := worker.runOne(context.Background()); err == nil {
		t.Fatal("expected worker error")
	}
	if !store.retried || store.completed {
		t.Fatalf("retried=%v completed=%v", store.retried, store.completed)
	}
}

type mergedArtifactsClient struct{ failingClient }

func (mergedArtifactsClient) FindArtifactsPullRequest(context.Context, int64, Onboarding) (PullRequest, error) {
	return PullRequest{Number: 9, URL: "https://github.test/pr/9", Merged: true}, nil
}

func TestWorkerReplaysAlreadyMergedArtifactsPullRequest(t *testing.T) {
	store := &retryStore{job: &Job{ID: "job", OrgID: "org", OnboardingID: "onboarding", Kind: JobFindArtifacts, Attempts: 1, MaxAttempts: 8}}
	worker := NewWorker(store, mergedArtifactsClient{})
	if err := worker.runOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.status != StateSyncQueued || store.enqueued != JobSync || !store.completed {
		t.Fatalf("status=%s enqueued=%s completed=%v", store.status, store.enqueued, store.completed)
	}
}
