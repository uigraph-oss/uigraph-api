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

type githubClient interface {
	AuthorizationURL(state, callback string) string
	InstallationURL(state string) string
	ExchangeCode(ctx context.Context, code string) (string, error)
	AuthenticatedUserID(ctx context.Context, token string) (int64, error)
	FindUserInstallation(ctx context.Context, token string) (*gh.Installation, error)
	VerifyUserInstallation(ctx context.Context, token string, installationID int64) (*gh.Installation, error)
	ListInstallationRepositories(ctx context.Context, installationID int64) ([]domain.Repository, error)
	DeleteInstallation(ctx context.Context, installationID int64) error
}

type githubPollingClient interface {
	GetPullRequest(ctx context.Context, installationID int64, repository domain.Repository, number int) (domain.PullRequest, error)
	GetWorkflowRun(ctx context.Context, installationID int64, repository domain.Repository, workflow string, runID int64, createdAfter time.Time) (domain.WorkflowRun, error)
}

type Handler struct {
	store         domain.Store
	client        githubClient
	callbackURL   string
	frontendURL   string
	webhookSecret []byte
}

func New(store domain.Store, client githubClient, callbackURL, frontendURL, webhookSecret string) *Handler {
	return &Handler{store: store, client: client, callbackURL: callbackURL, frontendURL: frontendURL, webhookSecret: []byte(webhookSecret)}
}

func Register(mux *http.ServeMux, h *Handler, requireScope func(scope, method, pattern string, handler http.HandlerFunc)) {
	requireScope("integrations:read", "GET", "/api/v1/orgs/{orgID}/github-app", h.GetInstallation)
	requireScope("integrations:write", "POST", "/api/v1/orgs/{orgID}/github-app/install", h.Install)
	requireScope("integrations:write", "DELETE", "/api/v1/orgs/{orgID}/github-app", h.DeleteInstallation)
	requireScope("integrations:read", "GET", "/api/v1/orgs/{orgID}/github-app/repositories", h.ListRepositories)
	requireScope("integrations:write", "POST", "/api/v1/orgs/{orgID}/repository-onboarding", h.CreateBatch)
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
	destination := strings.TrimRight(h.frontendURL, "/") + "/settings/integrations/github?installed=true"
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
	if err != nil || installation == nil {
		httpError(w, r, errOrNotFound(err))
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
	if err := h.reconcileBatch(r.Context(), batch); err != nil {
		slog.WarnContext(r.Context(), "GitHub onboarding poll failed", "err", err)
	} else {
		batch, err = h.store.GetBatch(r.Context(), r.PathValue("orgID"), r.PathValue("batchID"))
		if err != nil {
			httpError(w, r, err)
			return
		}
	}
	httputil.JSON(w, http.StatusOK, batch)
}

func (h *Handler) reconcileBatch(ctx context.Context, batch *domain.Batch) error {
	client, ok := h.client.(githubPollingClient)
	if !ok {
		return nil
	}
	installation, err := h.store.GetInstallation(ctx, batch.OrgID)
	if err != nil || installation == nil {
		return err
	}
	for _, onboarding := range batch.Items {
		switch onboarding.Status {
		case domain.StateWaitingSetupMerge:
			pull, err := client.GetPullRequest(ctx, installation.GitHubInstallationID, onboarding.Repository, onboarding.SetupPRNumber)
			if err != nil {
				return err
			}
			if pull.Closed {
				if err := h.store.ApplyPullRequestEvent(ctx, installation.GitHubInstallationID, onboarding.Repository.GitHubID, pull.Number, "closed", pull.Merged, onboarding.SetupBranch, onboarding.Repository.DefaultBranch); err != nil {
					return err
				}
			}
		case domain.StateWaitingArtifactsMerge:
			pull, err := client.GetPullRequest(ctx, installation.GitHubInstallationID, onboarding.Repository, onboarding.ArtifactsPRNumber)
			if err != nil {
				return err
			}
			if pull.Closed {
				if err := h.store.ApplyPullRequestEvent(ctx, installation.GitHubInstallationID, onboarding.Repository.GitHubID, pull.Number, "closed", pull.Merged, onboarding.ArtifactsBranch, onboarding.Repository.DefaultBranch); err != nil {
					return err
				}
			}
		case domain.StateGenerationQueued, domain.StateGenerationRunning:
			if err := h.reconcileWorkflow(ctx, client, *installation, onboarding, "uigraph-generate.yml", onboarding.GenerationRunID); err != nil {
				return err
			}
		case domain.StateSyncQueued, domain.StateSyncRunning:
			if err := h.reconcileWorkflow(ctx, client, *installation, onboarding, "uigraph-sync.yml", onboarding.SyncRunID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *Handler) reconcileWorkflow(ctx context.Context, client githubPollingClient, installation domain.Installation, onboarding domain.Onboarding, workflow string, runID int64) error {
	run, err := client.GetWorkflowRun(ctx, installation.GitHubInstallationID, onboarding.Repository, workflow, runID, onboarding.UpdatedAt)
	if err != nil || run.ID == 0 {
		return err
	}
	name := run.Name
	if name == "" {
		name = run.Path
	}
	return h.store.ApplyWorkflowRunEvent(ctx, installation.GitHubInstallationID, onboarding.Repository.GitHubID, run.ID, name, run.Event, run.Status, run.Conclusion, run.HeadBranch, run.HTMLURL)
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
	switch event {
	case "pull_request":
		var payload struct {
			Action       string `json:"action"`
			Installation struct {
				ID int64 `json:"id"`
			} `json:"installation"`
			Repository struct {
				ID int64 `json:"id"`
			} `json:"repository"`
			PullRequest struct {
				Number int  `json:"number"`
				Merged bool `json:"merged"`
				Head   struct {
					Ref string `json:"ref"`
				} `json:"head"`
				Base struct {
					Ref string `json:"ref"`
				} `json:"base"`
			} `json:"pull_request"`
		}
		if err := json.Unmarshal(body, &payload); err == nil {
			err = h.store.ApplyPullRequestEvent(r.Context(), payload.Installation.ID, payload.Repository.ID, payload.PullRequest.Number, payload.Action, payload.PullRequest.Merged, payload.PullRequest.Head.Ref, payload.PullRequest.Base.Ref)
		}
	case "workflow_run":
		var payload struct {
			Installation struct {
				ID int64 `json:"id"`
			} `json:"installation"`
			Repository struct {
				ID int64 `json:"id"`
			} `json:"repository"`
			WorkflowRun struct {
				ID         int64  `json:"id"`
				Name       string `json:"name"`
				Path       string `json:"path"`
				Event      string `json:"event"`
				Status     string `json:"status"`
				Conclusion string `json:"conclusion"`
				HeadBranch string `json:"head_branch"`
				HTMLURL    string `json:"html_url"`
			} `json:"workflow_run"`
		}
		if err := json.Unmarshal(body, &payload); err == nil {
			workflow := payload.WorkflowRun.Name
			if workflow == "" {
				workflow = payload.WorkflowRun.Path
			}
			err = h.store.ApplyWorkflowRunEvent(r.Context(), payload.Installation.ID, payload.Repository.ID, payload.WorkflowRun.ID, workflow, payload.WorkflowRun.Event, payload.WorkflowRun.Status, payload.WorkflowRun.Conclusion, payload.WorkflowRun.HeadBranch, payload.WorkflowRun.HTMLURL)
		}
	case "ping", "installation", "installation_repositories", "repository":
		err = nil
	default:
		err = nil
	}
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

func errOrNotFound(err error) error {
	if err != nil {
		return err
	}
	return store.ErrNotFound
}
