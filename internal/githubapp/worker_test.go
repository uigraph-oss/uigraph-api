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
	status    State
	missing   []string
	pr        int
}

func (s *retryStore) ClaimJob(context.Context, string, time.Duration) (*Job, error) {
	job := s.job
	s.job = nil
	return job, nil
}

func (s *retryStore) GetOnboarding(context.Context, string, string) (*Onboarding, error) {
	return &Onboarding{
		ID: "onboarding", OrgID: "org", BatchID: "batch", Status: StateSelected, Branch: Branch("onboarding"),
		Repository: Repository{FullName: "acme/repo", DefaultBranch: "main"},
	}, nil
}

func (s *retryStore) GetInstallation(context.Context, string) (*Installation, error) {
	return &Installation{GitHubInstallationID: 7, Status: "active"}, nil
}

func (s *retryStore) GetBatch(context.Context, string, string) (*Batch, error) {
	return &Batch{ID: "batch", Items: []Onboarding{{Repository: Repository{GitHubID: 11}}}}, nil
}

func (s *retryStore) CreateOnboardingToken(context.Context, string, time.Time) (string, error) {
	return "plaintext", nil
}

func (s *retryStore) UpdateOnboarding(_ context.Context, onboarding Onboarding) error {
	s.status = onboarding.Status
	s.missing = onboarding.MissingAIConfiguration
	s.pr = onboarding.PRNumber
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

func (failingClient) StartRun(context.Context, int64, Onboarding, string) error {
	return errors.New("temporary GitHub failure")
}

func (failingClient) MissingAIConfiguration(context.Context, int64, Repository, string) ([]string, error) {
	return nil, nil
}

func (failingClient) OpenPullRequest(context.Context, int64, Onboarding) (PullRequest, error) {
	return PullRequest{}, nil
}

func (failingClient) PutOnboardingSecret(context.Context, int64, Installation, []Repository, string) error {
	return nil
}

func TestWorkerRetriesFailedJobWithoutCompletingIt(t *testing.T) {
	store := &retryStore{job: &Job{ID: "job", OrgID: "org", OnboardingID: "onboarding", Kind: JobStart, Attempts: 1, MaxAttempts: 8}}
	worker := NewWorker(store, failingClient{})
	if err := worker.runOne(context.Background()); err == nil {
		t.Fatal("expected worker error")
	}
	if !store.retried || store.completed {
		t.Fatalf("retried=%v completed=%v", store.retried, store.completed)
	}
}

type missingAIClient struct{ failingClient }

func (missingAIClient) MissingAIConfiguration(context.Context, int64, Repository, string) ([]string, error) {
	return []string{"AI_PROVIDER_API_KEY"}, nil
}

func TestWorkerParksOnboardingWhenAISettingsAreMissing(t *testing.T) {
	store := &retryStore{job: &Job{ID: "job", OrgID: "org", OnboardingID: "onboarding", Kind: JobStart, Attempts: 1, MaxAttempts: 8}}
	worker := NewWorker(store, missingAIClient{})
	if err := worker.runOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.status != StateWaitingAI || len(store.missing) != 1 || !store.completed {
		t.Fatalf("status=%s missing=%v completed=%v", store.status, store.missing, store.completed)
	}
}

type recordingClient struct {
	failingClient
	order []string
}

func (c *recordingClient) PutOnboardingSecret(context.Context, int64, Installation, []Repository, string) error {
	c.order = append(c.order, "secret")
	return nil
}

func (c *recordingClient) StartRun(context.Context, int64, Onboarding, string) error {
	c.order = append(c.order, "start")
	return nil
}

func TestWorkerInstallsTheTokenBeforeStartingTheRun(t *testing.T) {
	store := &retryStore{job: &Job{ID: "job", OrgID: "org", OnboardingID: "onboarding", Kind: JobStart, Attempts: 1, MaxAttempts: 8}}
	client := &recordingClient{}
	worker := NewWorker(store, client)
	if err := worker.runOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(client.order) != 2 || client.order[0] != "secret" || client.order[1] != "start" {
		t.Fatalf("call order = %v", client.order)
	}
	if store.status != StateRunQueued || !store.completed {
		t.Fatalf("status=%s completed=%v", store.status, store.completed)
	}
}

type failingPullRequestClient struct{ failingClient }

func (failingPullRequestClient) OpenPullRequest(context.Context, int64, Onboarding) (PullRequest, error) {
	return PullRequest{}, errors.New("pull requests are disabled")
}

func TestWorkerKeepsOnboardingCompleteWhenThePullRequestCannotBeOpened(t *testing.T) {
	store := &retryStore{job: &Job{ID: "job", OrgID: "org", OnboardingID: "onboarding", Kind: JobOpenPR, Attempts: 1, MaxAttempts: 8}}
	worker := NewWorker(store, failingPullRequestClient{})
	if err := worker.runOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.retried || !store.completed {
		t.Fatalf("retried=%v completed=%v", store.retried, store.completed)
	}
}
