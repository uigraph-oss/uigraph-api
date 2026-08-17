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
	GetRepository(ctx context.Context, installationID int64, owner, repo string) (domain.Repository, error)
	GetInstallationAccount(ctx context.Context, installationID int64) (domain.Installation, error)
	DeleteInstallation(ctx context.Context, installationID int64) error
	GetWorkflowRun(ctx context.Context, installationID int64, repository domain.Repository, branch string, runID int64) (domain.WorkflowRun, error)
	ListWorkflowRunJobs(ctx context.Context, installationID int64, repository domain.Repository, runID int64) ([]domain.Step, error)
}

type Handler struct {
	store         domain.Store
	client        Client
	callbackURL   string
	webhookSecret []byte
}

func New(store domain.Store, client Client, callbackURL, webhookSecret string) *Handler {
	return &Handler{store: store, client: client, callbackURL: callbackURL, webhookSecret: []byte(webhookSecret)}
}

func Register(mux *http.ServeMux, h *Handler, requireScope func(scope, method, pattern string, handler http.HandlerFunc)) {
	requireScope("integrations:read", "GET", "/api/v1/orgs/{orgID}/github-app", h.GetInstallation)
	requireScope("integrations:write", "POST", "/api/v1/orgs/{orgID}/github-app/install", h.Install)
	requireScope("integrations:write", "DELETE", "/api/v1/orgs/{orgID}/github-app", h.DeleteInstallation)
	requireScope("integrations:read", "GET", "/api/v1/orgs/{orgID}/github-app/repositories", h.ListRepositories)
	requireScope("integrations:write", "POST", "/api/v1/orgs/{orgID}/repository-imports", h.CreateImport)
	requireScope("integrations:read", "GET", "/api/v1/orgs/{orgID}/repository-imports/latest", h.GetLatestImport)
	requireScope("integrations:read", "GET", "/api/v1/orgs/{orgID}/repository-imports/{importID}", h.GetImport)
	requireScope("integrations:write", "POST", "/api/v1/orgs/{orgID}/repository-imports/{importID}/recheck", h.Recheck)
	requireScope("integrations:write", "POST", "/api/v1/orgs/{orgID}/repository-imports/{importID}/retry", h.Retry)
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
	if installation == nil {
		httputil.JSON(w, http.StatusOK, map[string]any{"enabled": true, "installation": nil})
		return
	}
	account, err := h.client.GetInstallationAccount(r.Context(), installation.GitHubInstallationID)
	if notFound(err) {
		httputil.JSON(w, http.StatusOK, map[string]any{"enabled": true, "installation": nil})
		return
	}
	if err != nil {
		httpError(w, r, err)
		return
	}
	installation.AccountID, installation.AccountLogin = account.AccountID, account.AccountLogin
	installation.AccountType, installation.Suspended = account.AccountType, account.Suspended
	httputil.JSON(w, http.StatusOK, map[string]any{"enabled": true, "installation": installation})
}

func notFound(err error) bool {
	var apiError *gh.ErrorResponse
	return errors.As(err, &apiError) && apiError.Response != nil && apiError.Response.StatusCode == http.StatusNotFound
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
// @Success 200
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
	if _, err := h.client.VerifyUserInstallation(r.Context(), token, installationID); err != nil {
		httpError(w, r, err)
		return
	}
	orgID, err := h.store.ConsumeInstallState(r.Context(), stateHash, githubUserID)
	if err != nil {
		httpError(w, r, err)
		return
	}
	if err := h.store.UpsertInstallation(r.Context(), orgID, installationID); err != nil {
		httpError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<!doctype html><meta charset="utf-8"><script>window.close()</script>`))
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
	repositories, err := h.client.ListInstallationRepositories(r.Context(), installation.GitHubInstallationID)
	if err != nil {
		httpError(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"repositories": repositories})
}

// @Summary Start a repository import
// @Tags integrations
// @Security BearerAuth
// @Param orgID path string true "Organization ID"
// @Param body body object true "teamId, owner and repo"
// @Success 201 {object} domain.Import
// @Router /orgs/{orgID}/repository-imports [post]
func (h *Handler) CreateImport(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		http.Error(w, `{"error":"GitHub App is not configured","code":503}`, http.StatusServiceUnavailable)
		return
	}
	var request struct {
		TeamID string `json:"teamId"`
		Owner  string `json:"owner"`
		Repo   string `json:"repo"`
	}
	if err := httputil.Decode(r, &request); err != nil || request.TeamID == "" || request.Owner == "" || request.Repo == "" {
		httputil.BadRequest(w, "teamId, owner and repo are required")
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
	repository, err := h.client.GetRepository(r.Context(), installation.GitHubInstallationID, request.Owner, request.Repo)
	if notFound(err) {
		httputil.BadRequest(w, "the GitHub App cannot access this repository")
		return
	}
	if err != nil {
		httpError(w, r, err)
		return
	}
	value, err := h.store.CreateImport(r.Context(), r.PathValue("orgID"), request.TeamID, repository.OwnerID, repository.Name)
	if err != nil {
		httpError(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, value)
}

// @Summary Get a repository import
// @Tags integrations
// @Security BearerAuth
// @Param orgID path string true "Organization ID"
// @Param importID path string true "Import ID"
// @Success 200 {object} domain.Import
// @Router /orgs/{orgID}/repository-imports/{importID} [get]
func (h *Handler) GetImport(w http.ResponseWriter, r *http.Request) {
	value, err := h.store.GetImport(r.Context(), r.PathValue("orgID"), r.PathValue("importID"))
	if err != nil {
		httpError(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, h.reconciled(r.Context(), value))
}

// @Summary Get the latest repository import
// @Tags integrations
// @Security BearerAuth
// @Param orgID path string true "Organization ID"
// @Success 200 {object} map[string]interface{}
// @Router /orgs/{orgID}/repository-imports/latest [get]
func (h *Handler) GetLatestImport(w http.ResponseWriter, r *http.Request) {
	value, err := h.store.GetLatestImport(r.Context(), r.PathValue("orgID"))
	if err != nil {
		httpError(w, r, err)
		return
	}
	if value == nil {
		httputil.JSON(w, http.StatusOK, map[string]any{"import": nil})
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"import": h.reconciled(r.Context(), value)})
}

func (h *Handler) reconciled(ctx context.Context, value *domain.Import) *domain.Import {
	applied, err := h.reconcileImport(ctx, value)
	if err != nil {
		slog.WarnContext(ctx, "GitHub import poll failed", "err", err)
	}
	if !applied {
		return value
	}
	refreshed, err := h.store.GetImport(ctx, value.OrgID, value.ID)
	if err != nil {
		slog.WarnContext(ctx, "GitHub import reload failed", "err", err)
		return value
	}
	return refreshed
}

// reconcileImport is the polling safety net for missed or delayed webhooks. It
// only ever touches an import that is still running, so once an import is
// terminal UiGraph stops asking GitHub about that repository entirely.
func (h *Handler) reconcileImport(ctx context.Context, value *domain.Import) (bool, error) {
	if h.client == nil || (value.Status != domain.StateRunQueued && value.Status != domain.StateRunRunning) {
		return false, nil
	}
	installation, err := h.store.GetInstallation(ctx, value.OrgID)
	if err != nil || installation == nil {
		return false, err
	}
	account, err := h.client.GetInstallationAccount(ctx, installation.GitHubInstallationID)
	if err != nil {
		return false, err
	}
	repository, err := h.client.GetRepository(ctx, installation.GitHubInstallationID, account.AccountLogin, value.GitHubRepo)
	if err != nil {
		return false, err
	}
	run, err := h.client.GetWorkflowRun(ctx, installation.GitHubInstallationID, repository, value.Branch, value.RunID)
	if err != nil {
		return false, err
	}
	if run.ID == 0 {
		return false, nil
	}
	steps, err := h.client.ListWorkflowRunJobs(ctx, installation.GitHubInstallationID, repository, run.ID)
	if err != nil {
		return false, err
	}
	if len(steps) != 0 {
		if err := h.store.ApplyWorkflowJobEvent(ctx, run.HeadBranch, run.ID, steps); err != nil {
			return false, err
		}
	}
	return true, h.store.ApplyWorkflowRunEvent(ctx, run, repository.URL)
}

// @Summary Recheck repository AI configuration
// @Tags integrations
// @Security BearerAuth
// @Param orgID path string true "Organization ID"
// @Param importID path string true "Import ID"
// @Success 202
// @Router /orgs/{orgID}/repository-imports/{importID}/recheck [post]
func (h *Handler) Recheck(w http.ResponseWriter, r *http.Request) {
	h.retry(w, r, true)
}

// @Summary Retry a failed repository import
// @Tags integrations
// @Security BearerAuth
// @Param orgID path string true "Organization ID"
// @Param importID path string true "Import ID"
// @Success 202
// @Router /orgs/{orgID}/repository-imports/{importID}/retry [post]
func (h *Handler) Retry(w http.ResponseWriter, r *http.Request) {
	h.retry(w, r, false)
}

func (h *Handler) retry(w http.ResponseWriter, r *http.Request, recheck bool) {
	if err := h.store.RetryImport(r.Context(), r.PathValue("orgID"), r.PathValue("importID"), recheck); err != nil {
		httpError(w, r, err)
		return
	}
	value, err := h.store.GetImport(r.Context(), r.PathValue("orgID"), r.PathValue("importID"))
	if err != nil {
		httpError(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusAccepted, value)
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
	var payload struct {
		Action       string `json:"action"`
		Installation struct {
			ID int64 `json:"id"`
		} `json:"installation"`
		Repository struct {
			HTMLURL string `json:"html_url"`
		} `json:"repository"`
		WorkflowRun domain.WorkflowRun `json:"workflow_run"`
		WorkflowJob *gh.WorkflowJob    `json:"workflow_job"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		httputil.BadRequest(w, "invalid webhook JSON")
		return
	}
	created, err := h.store.RecordWebhook(r.Context(), delivery, event, payload.Action, payload.Installation.ID)
	if err != nil {
		httpError(w, r, err)
		return
	}
	if !created {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	switch event {
	case "workflow_run":
		err = h.store.ApplyWorkflowRunEvent(r.Context(), payload.WorkflowRun, payload.Repository.HTMLURL)
		if err != nil {
			httpError(w, r, err)
			return
		}
	case "workflow_job":
		if payload.WorkflowJob == nil {
			httputil.BadRequest(w, "invalid workflow_job payload")
			return
		}
		job := payload.WorkflowJob
		steps := domain.JobSteps(job.GetID(), job.GetName(), job.Steps)
		err = h.store.ApplyWorkflowJobEvent(r.Context(), job.GetHeadBranch(), job.GetRunID(), steps)
		if err != nil {
			httpError(w, r, err)
			return
		}
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
