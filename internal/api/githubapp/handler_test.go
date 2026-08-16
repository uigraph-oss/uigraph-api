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

func (s *webhookStore) ApplyWorkflowRunEvent(context.Context, int64, int64, int64, string, string, string, string, string) error {
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
func (pollingClient) FindUserInstallation(context.Context, string) (*gh.Installation, error) {
	return nil, nil
}
func (pollingClient) VerifyUserInstallation(context.Context, string, int64) (*gh.Installation, error) {
	return nil, nil
}
func (pollingClient) ListInstallationRepositories(context.Context, int64) ([]domain.Repository, error) {
	return nil, nil
}
func (pollingClient) DeleteInstallation(context.Context, int64) error { return nil }
func (pollingClient) GetWorkflowRun(_ context.Context, _ int64, _ domain.Repository, branch string, _ int64) (domain.WorkflowRun, error) {
	return domain.WorkflowRun{ID: 42, Event: "push", Status: "completed", Conclusion: "success", HeadBranch: branch}, nil
}

func signedRequest(t *testing.T, body []byte, delivery string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/github-app/webhooks", bytes.NewReader(body))
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write(body)
	request.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	request.Header.Set("X-GitHub-Delivery", delivery)
	request.Header.Set("X-GitHub-Event", "workflow_run")
	return request
}

func TestWebhookSignatureDedupeAndTransition(t *testing.T) {
	store := &webhookStore{deliveries: map[string]bool{}}
	handler := New(store, nil, "", "", "secret")
	body := []byte(`{"installation":{"id":7},"repository":{"id":8},"workflow_run":{"id":42,"event":"push","status":"completed","conclusion":"success","head_branch":"uigraph/onboarding/id"}}`)
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

func TestReconcileBatchPollsTheOnboardingBranchRun(t *testing.T) {
	store := &webhookStore{}
	handler := New(store, pollingClient{}, "", "", "")
	batch := &domain.Batch{OrgID: "org", Items: []domain.Onboarding{
		{
			Status: domain.StateRunQueued, Branch: "uigraph/onboarding/id",
			Repository: domain.Repository{GitHubID: 8, DefaultBranch: "main"},
		},
		{
			Status: domain.StateCompleted, Branch: "uigraph/onboarding/done",
			Repository: domain.Repository{GitHubID: 9, DefaultBranch: "main"},
		},
	}}
	if err := handler.reconcileBatch(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	if store.transitions != 1 {
		t.Fatalf("transition count = %d", store.transitions)
	}
}
