package githubapp

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gh "github.com/google/go-github/v74/github"
	domain "github.com/uigraph/app/internal/githubapp"
)

type webhookStore struct {
	domain.Store
	deliveries  map[string]bool
	transitions int
}

func (s *webhookStore) RecordWebhook(_ context.Context, deliveryID, event, action string, installationID int64) (bool, error) {
	if s.deliveries[deliveryID] {
		return false, nil
	}
	s.deliveries[deliveryID] = true
	return true, nil
}

func (s *webhookStore) ApplyPullRequestEvent(context.Context, int64, int64, int, string, bool, string, string) error {
	s.transitions++
	return nil
}

func (s *webhookStore) GetInstallation(context.Context, string) (*domain.Installation, error) {
	return &domain.Installation{GitHubInstallationID: 7}, nil
}

type pollingClient struct{}

func (pollingClient) AuthorizationURL(string, string) string { return "" }
func (pollingClient) InstallationURL(string) string          { return "" }
func (pollingClient) ExchangeCode(context.Context, string) (string, error) {
	return "", nil
}
func (pollingClient) AuthenticatedUserID(context.Context, string) (int64, error) {
	return 0, nil
}
func (pollingClient) VerifyUserInstallation(context.Context, string, int64) (*gh.Installation, error) {
	return nil, nil
}
func (pollingClient) ListInstallationRepositories(context.Context, int64) ([]domain.Repository, error) {
	return nil, nil
}
func (pollingClient) DeleteInstallation(context.Context, int64) error { return nil }
func (pollingClient) GetPullRequest(context.Context, int64, domain.Repository, int) (domain.PullRequest, error) {
	return domain.PullRequest{Number: 2, Merged: true, Closed: true}, nil
}
func (pollingClient) GetWorkflowRun(context.Context, int64, domain.Repository, string, int64, time.Time) (domain.WorkflowRun, error) {
	return domain.WorkflowRun{}, nil
}

func signedRequest(t *testing.T, body []byte, delivery string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/github-app/webhooks", bytes.NewReader(body))
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write(body)
	request.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	request.Header.Set("X-GitHub-Delivery", delivery)
	request.Header.Set("X-GitHub-Event", "pull_request")
	return request
}

func TestWebhookSignatureDedupeAndTransition(t *testing.T) {
	store := &webhookStore{deliveries: map[string]bool{}}
	handler := New(store, nil, "", "", "secret")
	body := []byte(`{"action":"closed","installation":{"id":7},"repository":{"id":8},"pull_request":{"number":2,"merged":true,"head":{"ref":"uigraph/setup/id"},"base":{"ref":"main"}}}`)
	for range 2 {
		response := httptest.NewRecorder()
		handler.Webhook(response, signedRequest(t, body, "delivery-1"))
		if response.Code != http.StatusAccepted {
			t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
		}
	}
	if store.transitions != 1 {
		t.Fatalf("transition count = %d", store.transitions)
	}
}

func TestWebhookRejectsInvalidSignature(t *testing.T) {
	handler := New(&webhookStore{deliveries: map[string]bool{}}, nil, "", "", "secret")
	request := httptest.NewRequest(http.MethodPost, "/api/v1/github-app/webhooks", bytes.NewReader([]byte(`{}`)))
	request.Header.Set("X-Hub-Signature-256", "sha256=00")
	response := httptest.NewRecorder()
	handler.Webhook(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestRegisterOmitsWebhookWithoutSecret(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, New(&webhookStore{}, nil, "", "", ""), func(string, string, string, http.HandlerFunc) {})
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/github-app/webhooks", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestReconcileBatchPollsMergedPullRequest(t *testing.T) {
	store := &webhookStore{}
	handler := New(store, pollingClient{}, "", "", "")
	batch := &domain.Batch{OrgID: "org", Items: []domain.Onboarding{{
		Status: domain.StateWaitingSetupMerge, SetupPRNumber: 2, SetupBranch: "uigraph/setup/id",
		Repository: domain.Repository{GitHubID: 8, DefaultBranch: "main"},
	}}}
	if err := handler.reconcileBatch(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	if store.transitions != 1 {
		t.Fatalf("transition count = %d", store.transitions)
	}
}
