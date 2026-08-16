package githubapp

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v74/github"
	domain "github.com/uigraph/app/internal/githubapp"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/identity"
	"github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/store"
)

const maxWebhookBody = 1 << 20

type Client interface {
	AuthorizationURL(state, callback string) string
	InstallationURL(state string) string
	ExchangeCode(ctx context.Context, code string) (string, error)
	AuthenticatedUserID(ctx context.Context, token string) (int64, error)
	FindUserInstallation(ctx context.Context, token string) (*gh.Installation, error)
	VerifyUserInstallation(ctx context.Context, token string, installationID int64) (*gh.Installation, error)
	ListInstallationRepositories(ctx context.Context, installationID int64) ([]domain.Repository, error)
	DeleteInstallation(ctx context.Context, installationID int64) error
	GetWorkflowRun(ctx context.Context, installationID int64, repository domain.Repository, branch string, runID int64) (domain.WorkflowRun, error)
}

type Handler struct {
	store         domain.Store
	client        Client
	callbackURL   string
	frontendURL   string
	webhookSecret []byte
}

func New(store domain.Store, client Client, callbackURL, frontendURL, webhookSecret string) *Handler {
	return &Handler{store: store, client: client, callbackURL: callbackURL, frontendURL: frontendURL, webhookSecret: []byte(webhookSecret)}
}

func Register(mux *http.ServeMux, h *Handler, requireScope func(scope, method, pattern string, handler http.HandlerFunc)) {
	requireScope("integrations:read", "GET", "/api/v1/orgs/{orgID}/github-app", h.GetInstallation)
	requireScope("integrations:write", "POST", "/api/v1/orgs/{orgID}/github-app/install", h.Install)
	requireScope("integrations:write", "DELETE", "/api/v1/orgs/{orgID}/github-app", h.DeleteInstallation)
	requireScope("integrations:read", "GET", "/api/v1/orgs/{orgID}/github-app/repositories", h.ListRepositories)
	requireScope("integrations:write", "POST", "/api/v1/orgs/{orgID}/repository-onboarding", h.CreateBatch)
	requireScope("integrations:read", "GET", "/api/v1/orgs/{orgID}/repository-onboarding", h.GetLatestBatch)
	requireScope("integrations:read", "GET", "/api/v1/orgs/{orgID}/repository-onboarding/{batchID}", h.GetBatch)
	requireScope("integrations:write", "POST", "/api/v1/orgs/{orgID}/repository-onboarding/{batchID}/repositories/{onboardingID}/recheck", h.Recheck)
	requireScope("integrations:write", "POST", "/api/v1/orgs/{orgID}/repository-onboarding/{batchID}/repositories/{onboardingID}/retry", h.Retry)
	mux.HandleFunc("GET /api/v1/github-app/callback", h.Callback)
	if len(h.webhookSecret) != 0 {
		mux.HandleFunc("POST /api/v1/github-app/webhooks", h.Webhook)
	}
}

// @Summary Get GitHub App installation
// @Tags integrations
// @Security BearerAuth
// @Param orgID path string true "Organization ID"
// @Success 200 {object} map[string]interface{}
// @Router /orgs/{orgID}/github-app [get]
func (h *Handler) GetInstallation(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		httputil.JSON(w, http.StatusOK, map[string]any{"enabled": false, "installation": nil})
		return
	}
	installation, err := h.store.GetInstallation(r.Context(), r.PathValue("orgID"))
	if err != nil {
		httpError(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"enabled": true, "installation": installation})
}

// @Summary Start GitHub App installation
// @Tags integrations
// @Security BearerAuth
// @Param orgID path string true "Organization ID"
// @Success 201 {object} map[string]string
// @Router /orgs/{orgID}/github-app/install [post]
func (h *Handler) Install(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		http.Error(w, `{"error":"GitHub App is not configured","code":503}`, http.StatusServiceUnavailable)
		return
	}
	principal, ok := middleware.PrincipalFromCtx(r.Context())
	if !ok || principal.Kind != identity.PrincipalUser || principal.UserID == "" {
		httputil.Forbidden(w)
		return
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		httpError(w, r, err)
		return
	}
	state := hex.EncodeToString(raw)
	digest := sha256.Sum256([]byte(state))
	if err := h.store.CreateInstallState(r.Context(), r.PathValue("orgID"), principal.UserID, hex.EncodeToString(digest[:]), time.Now().Add(10*time.Minute)); err != nil {
		httpError(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, map[string]string{"url": h.client.AuthorizationURL(state, h.callbackURL)})
}

// @Summary Complete GitHub App authorization and installation
// @Tags integrations
// @Param state query string true "One-use installation state"
// @Param code query string false "GitHub OAuth code"
// @Param installation_id query integer false "GitHub installation ID"
// @Success 302
// @Router /github-app/callback [get]
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		http.NotFound(w, r)
		return
	}
	state := r.URL.Query().Get("state")
	if state == "" {
		httputil.BadRequest(w, "state is required")
		return
	}
	digest := sha256.Sum256([]byte(state))
	stateHash := hex.EncodeToString(digest[:])
	code := r.URL.Query().Get("code")
	installationID, _ := strconv.ParseInt(r.URL.Query().Get("installation_id"), 10, 64)
	if installationID != 0 && code == "" {
		callback := h.callbackURL + "?installation_id=" + strconv.FormatInt(installationID, 10)
		http.Redirect(w, r, h.client.AuthorizationURL(state, callback), http.StatusFound)
		return
	}
	if code == "" {
		httputil.BadRequest(w, "code is required")
		return
	}
	token, err := h.client.ExchangeCode(r.Context(), code)
	if err != nil {
		httpError(w, r, err)
		return
	}
	githubUserID, err := h.client.AuthenticatedUserID(r.Context(), token)
	if err != nil {
		httpError(w, r, err)
		return
	}
	if installationID == 0 {
		if _, err := h.store.AuthorizeInstallState(r.Context(), stateHash, githubUserID); err != nil {
			httpError(w, r, err)
			return
		}
		existing, err := h.client.FindUserInstallation(r.Context(), token)
		if err != nil {
			httpError(w, r, err)
			return
		}
		if existing == nil {
			http.Redirect(w, r, h.client.InstallationURL(state), http.StatusFound)
			return
		}
		installationID = existing.GetID()
	}
	installation, err := h.client.VerifyUserInstallation(r.Context(), token, installationID)
	if err != nil {
		httpError(w, r, err)
		return
	}
	orgID, err := h.store.ConsumeInstallState(r.Context(), stateHash, githubUserID)
	if err != nil {
		httpError(w, r, err)
		return
	}
	repositories, err := h.client.ListInstallationRepositories(r.Context(), installationID)
	if err != nil {
		httpError(w, r, err)
		return
	}
	account := installation.GetAccount()
	value := domain.Installation{
		OrgID: orgID, GitHubInstallationID: installationID, AccountID: account.GetID(), AccountLogin: account.GetLogin(),
		AccountType: account.GetType(), TargetType: installation.GetTargetType(), Status: "active",
	}
	if err := h.store.UpsertInstallation(r.Context(), value, repositories); err != nil {
		httpError(w, r, err)
		return
	}
	destination := strings.TrimRight(h.frontendURL, "/") + "/integrations/github/installed"
	http.Redirect(w, r, destination, http.StatusFound)
}

// @Summary Delete GitHub App installation
// @Tags integrations
// @Security BearerAuth
// @Param orgID path string true "Organization ID"
// @Success 204
// @Router /orgs/{orgID}/github-app [delete]
func (h *Handler) DeleteInstallation(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		http.Error(w, `{"error":"GitHub App is not configured","code":503}`, http.StatusServiceUnavailable)
		return
	}
	installation, err := h.store.GetInstallation(r.Context(), r.PathValue("orgID"))
	if err != nil {
		httpError(w, r, err)
		return
	}
	if installation == nil {
		httpError(w, r, store.ErrNotFound)
		return
	}
	if err := h.client.DeleteInstallation(r.Context(), installation.GitHubInstallationID); err != nil {
		httpError(w, r, err)
		return
	}
	if err := h.store.DeleteInstallation(r.Context(), r.PathValue("orgID")); err != nil {
		httpError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary List repositories visible to the GitHub App
// @Tags integrations
// @Security BearerAuth
// @Param orgID path string true "Organization ID"
// @Success 200 {object} map[string]interface{}
// @Router /orgs/{orgID}/github-app/repositories [get]
func (h *Handler) ListRepositories(w http.ResponseWriter, r *http.Request) {
	repositories, err := h.store.ListRepositories(r.Context(), r.PathValue("orgID"))
	if err != nil {
		httpError(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"repositories": repositories})
}

// @Summary Create a repository onboarding batch
// @Tags integrations
// @Security BearerAuth
// @Param orgID path string true "Organization ID"
// @Param body body object true "teamId and repositoryIds"
// @Success 201 {object} domain.Batch
// @Router /orgs/{orgID}/repository-onboarding [post]
func (h *Handler) CreateBatch(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TeamID        string   `json:"teamId"`
		RepositoryIDs []string `json:"repositoryIds"`
	}
	if err := httputil.Decode(r, &request); err != nil || request.TeamID == "" || len(request.RepositoryIDs) == 0 {
		httputil.BadRequest(w, "teamId and repositoryIds are required")
		return
	}
	principal, ok := middleware.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Forbidden(w)
		return
	}
	batch, err := h.store.CreateBatch(r.Context(), r.PathValue("orgID"), request.TeamID, "", principal.UserID, request.RepositoryIDs)
	if err != nil {
		httpError(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, batch)
}

// @Summary Get a repository onboarding batch
// @Tags integrations
// @Security BearerAuth
// @Param orgID path string true "Organization ID"
// @Param batchID path string true "Batch ID"
// @Success 200 {object} domain.Batch
// @Router /orgs/{orgID}/repository-onboarding/{batchID} [get]
func (h *Handler) GetBatch(w http.ResponseWriter, r *http.Request) {
	batch, err := h.store.GetBatch(r.Context(), r.PathValue("orgID"), r.PathValue("batchID"))
	if err != nil {
		httpError(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, h.reconciled(r.Context(), batch))
}

// @Summary Get the latest repository onboarding batch
// @Tags integrations
// @Security BearerAuth
// @Param orgID path string true "Organization ID"
// @Success 200 {object} domain.Batch
// @Router /orgs/{orgID}/repository-onboarding [get]
func (h *Handler) GetLatestBatch(w http.ResponseWriter, r *http.Request) {
	batch, err := h.store.GetLatestBatch(r.Context(), r.PathValue("orgID"))
	if err != nil {
		httpError(w, r, err)
		return
	}
	if batch == nil {
		httputil.JSON(w, http.StatusOK, map[string]any{"batch": nil})
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"batch": h.reconciled(r.Context(), batch)})
}

func (h *Handler) reconciled(ctx context.Context, batch *domain.Batch) *domain.Batch {
	applied, err := h.reconcileBatch(ctx, batch)
	if err != nil {
		slog.WarnContext(ctx, "GitHub onboarding poll failed", "err", err)
	}
	if !applied {
		return batch
	}
	refreshed, err := h.store.GetBatch(ctx, batch.OrgID, batch.ID)
	if err != nil {
		slog.WarnContext(ctx, "GitHub onboarding reload failed", "err", err)
		return batch
	}
	return refreshed
}

func (h *Handler) reconcileBatch(ctx context.Context, batch *domain.Batch) (bool, error) {
	if h.client == nil {
		return false, nil
	}
	installation, err := h.store.GetInstallation(ctx, batch.OrgID)
	if err != nil {
		return false, err
	}
	if installation == nil {
		return false, nil
	}
	applied := false
	for _, onboarding := range batch.Items {
		if onboarding.Status != domain.StateRunQueued && onboarding.Status != domain.StateRunRunning {
			continue
		}
		run, err := h.client.GetWorkflowRun(ctx, installation.GitHubInstallationID, onboarding.Repository, onboarding.Branch, onboarding.RunID)
		if err != nil {
			return applied, err
		}
		if run.ID == 0 {
			continue
		}
		if err := h.store.ApplyWorkflowRunEvent(ctx, installation.GitHubInstallationID, onboarding.Repository.GitHubID, run.ID, run.Event, run.Status, run.Conclusion, run.HeadBranch, run.HTMLURL, run.Path); err != nil {
			return applied, err
		}
		applied = true
	}
	return applied, nil
}

// @Summary Recheck repository AI configuration
// @Tags integrations
// @Security BearerAuth
// @Param orgID path string true "Organization ID"
// @Param batchID path string true "Batch ID"
// @Param onboardingID path string true "Onboarding ID"
// @Success 202
// @Router /orgs/{orgID}/repository-onboarding/{batchID}/repositories/{onboardingID}/recheck [post]
func (h *Handler) Recheck(w http.ResponseWriter, r *http.Request) {
	h.retry(w, r, true)
}

// @Summary Retry failed repository onboarding
// @Tags integrations
// @Security BearerAuth
// @Param orgID path string true "Organization ID"
// @Param batchID path string true "Batch ID"
// @Param onboardingID path string true "Onboarding ID"
// @Success 202
// @Router /orgs/{orgID}/repository-onboarding/{batchID}/repositories/{onboardingID}/retry [post]
func (h *Handler) Retry(w http.ResponseWriter, r *http.Request) {
	h.retry(w, r, false)
}

func (h *Handler) retry(w http.ResponseWriter, r *http.Request, recheck bool) {
	batch, err := h.store.GetBatch(r.Context(), r.PathValue("orgID"), r.PathValue("batchID"))
	if err != nil {
		httpError(w, r, err)
		return
	}
	found := false
	for _, item := range batch.Items {
		if item.ID == r.PathValue("onboardingID") {
			found = true
			break
		}
	}
	if !found {
		httpError(w, r, store.ErrNotFound)
		return
	}
	if err := h.store.RetryOnboarding(r.Context(), r.PathValue("orgID"), r.PathValue("onboardingID"), recheck); err != nil {
		httpError(w, r, err)
		return
	}
	onboarding, err := h.store.GetOnboarding(r.Context(), r.PathValue("orgID"), r.PathValue("onboardingID"))
	if err != nil {
		httpError(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusAccepted, onboarding)
}

// @Summary Receive signed GitHub App webhooks
// @Tags integrations
// @Success 202
// @Router /github-app/webhooks [post]
func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		httputil.BadRequest(w, "webhook body is too large")
		return
	}
	signature := r.Header.Get("X-Hub-Signature-256")
	if !validSignature(body, signature, h.webhookSecret) {
		http.Error(w, `{"error":"invalid signature","code":401}`, http.StatusUnauthorized)
		return
	}
	delivery := r.Header.Get("X-GitHub-Delivery")
	event := r.Header.Get("X-GitHub-Event")
	if delivery == "" || event == "" {
		httputil.BadRequest(w, "GitHub delivery and event headers are required")
		return
	}
	var envelope struct {
		Action       string `json:"action"`
		Installation struct {
			ID int64 `json:"id"`
		} `json:"installation"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		httputil.BadRequest(w, "invalid webhook JSON")
		return
	}
	created, err := h.store.RecordWebhook(r.Context(), delivery, event, envelope.Action, envelope.Installation.ID)
	if err != nil {
		httpError(w, r, err)
		return
	}
	if !created {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if event != "workflow_run" {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	var payload struct {
		Installation struct {
			ID int64 `json:"id"`
		} `json:"installation"`
		Repository struct {
			ID int64 `json:"id"`
		} `json:"repository"`
		WorkflowRun struct {
			ID         int64  `json:"id"`
			Event      string `json:"event"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			HeadBranch string `json:"head_branch"`
			HTMLURL    string `json:"html_url"`
			Path       string `json:"path"`
		} `json:"workflow_run"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		httputil.BadRequest(w, "invalid workflow_run payload")
		return
	}
	err = h.store.ApplyWorkflowRunEvent(r.Context(), payload.Installation.ID, payload.Repository.ID, payload.WorkflowRun.ID, payload.WorkflowRun.Event, payload.WorkflowRun.Status, payload.WorkflowRun.Conclusion, payload.WorkflowRun.HeadBranch, payload.WorkflowRun.HTMLURL, payload.WorkflowRun.Path)
	if err != nil {
		httpError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func validSignature(body []byte, signature string, secret []byte) bool {
	if !strings.HasPrefix(signature, "sha256=") || len(secret) == 0 {
		return false
	}
	received, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(body)
	return hmac.Equal(received, mac.Sum(nil))
}

func httpError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}
	if errors.Is(err, store.ErrConflict) || errors.Is(err, store.ErrTeamNotFound) {
		httputil.BadRequest(w, err.Error())
		return
	}
	httputil.Error(w, r, fmt.Errorf("github app: %w", err))
}
