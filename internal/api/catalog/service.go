package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	catalogpkg "github.com/uigraph/app/internal/catalog"
	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
)

// List
// @Summary  List
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	q := r.URL.Query()
	p := catalogpkg.ListParams{
		SortBy:  q.Get("sortBy"),
		SortDir: q.Get("sortDir"),
	}
	if v := q.Get("limit"); v != "" {
		p.Limit = httputil.ListLimit(v)
		p.Offset = httputil.ListOffset(q.Get("offset"))
	}
	if v := q.Get("folderId"); v != "" {
		p.FolderID = &v
	}
	if v := q.Get("teamId"); v != "" {
		p.TeamID = &v
	}
	if v := q.Get("search"); v != "" {
		p.Search = &v
	}
	svcs, total, err := h.store.ListServices(r.Context(), orgID, p)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"services": svcs, "total": total})
}

// ListStats
// @Summary  ListStats
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/stats [get]
func (h *Handler) ListStats(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	var serviceID *string
	if v := r.URL.Query().Get("serviceId"); v != "" {
		serviceID = &v
	}

	stats, err := h.store.ListServiceStats(r.Context(), orgID, serviceID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"stats": stats})
}

// Create
// @Summary  Create
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	var body struct {
		Name            string          `json:"name"`
		Description     string          `json:"description"`
		Status          string          `json:"status"`
		Tier            string          `json:"tier"`
		Category        string          `json:"category"`
		Language        string          `json:"language"`
		FolderID        *string         `json:"folderId"`
		TeamID          *string         `json:"teamId"`
		TeamName        string          `json:"teamName"`
		GitRepoURL      *string         `json:"gitRepoUrl"`
		JiraProjectURL  *string         `json:"jiraProjectUrl"`
		SlackChannelURL *string         `json:"slackChannelUrl"`
		Labels          []string        `json:"labels"`
		Metadata        json.RawMessage `json:"metadata"`
		CommitHash      *string         `json:"commitHash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}
	hasTeamID := body.TeamID != nil && *body.TeamID != ""
	hasTeamName := body.TeamName != ""
	if hasTeamID && hasTeamName {
		httputil.BadRequest(w, "provide either teamId or teamName, not both")
		return
	}
	if !hasTeamID && !hasTeamName {
		httputil.BadRequest(w, "team is required")
		return
	}
	if body.Status == "" {
		body.Status = "active"
	}
	if body.Tier == "" {
		body.Tier = "tier3"
	}

	now := time.Now().UTC()
	svc := catalogpkg.Service{
		ID:                  uuid.NewString(),
		OrgID:               orgID,
		FolderID:            body.FolderID,
		TeamID:              body.TeamID,
		TeamName:            body.TeamName,
		Name:                body.Name,
		Description:         body.Description,
		Status:              body.Status,
		Tier:                body.Tier,
		Category:            body.Category,
		Language:            body.Language,
		GitRepoURL:          body.GitRepoURL,
		JiraProjectURL:      body.JiraProjectURL,
		SlackChannelURL:     body.SlackChannelURL,
		Labels:              body.Labels,
		Metadata:            body.Metadata,
		CreatedBy:           p.UserID,
		CreatedByCommitHash: body.CommitHash,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := h.store.CreateService(r.Context(), svc); err != nil {
		if errors.Is(err, storepkg.ErrTeamNotFound) {
			httputil.BadRequest(w, fmt.Sprintf("team %q does not exist", body.TeamName))
			return
		}
		if errors.Is(err, storepkg.ErrServiceNameExists) {
			httputil.Conflict(w, fmt.Sprintf("a service named %q already exists in this organization", body.Name))
			return
		}
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, svc)
}

// Get
// @Summary  Get
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID} [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	svc, err := h.store.GetService(r.Context(), r.PathValue("serviceID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if svc == nil || svc.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, svc)
}

// Update
// @Summary  Update
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID} [put]
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	svc, err := h.store.GetService(r.Context(), r.PathValue("serviceID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if svc == nil || svc.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	var body struct {
		Name            *string         `json:"name"`
		Description     *string         `json:"description"`
		Status          *string         `json:"status"`
		Tier            *string         `json:"tier"`
		Category        *string         `json:"category"`
		Language        *string         `json:"language"`
		FolderID        *string         `json:"folderId"`
		TeamID          *string         `json:"teamId"`
		TeamName        *string         `json:"teamName"`
		GitRepoURL      *string         `json:"gitRepoUrl"`
		JiraProjectURL  *string         `json:"jiraProjectUrl"`
		SlackChannelURL *string         `json:"slackChannelUrl"`
		LastCommitSha   *string         `json:"lastCommitSha"`
		Labels          []string        `json:"labels"`
		Metadata        json.RawMessage `json:"metadata"`
		CommitHash      *string         `json:"commitHash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name != nil {
		svc.Name = *body.Name
	}
	if body.Description != nil {
		svc.Description = *body.Description
	}
	if body.Status != nil {
		svc.Status = *body.Status
	}
	if body.Tier != nil {
		svc.Tier = *body.Tier
	}
	if body.Category != nil {
		svc.Category = *body.Category
	}
	if body.Language != nil {
		svc.Language = *body.Language
	}
	if body.FolderID != nil {
		svc.FolderID = body.FolderID
	}
	hasTeamID := body.TeamID != nil && *body.TeamID != ""
	hasTeamName := body.TeamName != nil && *body.TeamName != ""
	if hasTeamID && hasTeamName {
		httputil.BadRequest(w, "provide either teamId or teamName, not both")
		return
	}
	if hasTeamID {
		svc.TeamID = body.TeamID
	}
	if hasTeamName {
		svc.TeamID = nil
		svc.TeamName = *body.TeamName
	}
	if body.GitRepoURL != nil {
		svc.GitRepoURL = body.GitRepoURL
	}
	if body.JiraProjectURL != nil {
		svc.JiraProjectURL = body.JiraProjectURL
	}
	if body.SlackChannelURL != nil {
		svc.SlackChannelURL = body.SlackChannelURL
	}
	if body.LastCommitSha != nil {
		svc.LastCommitSha = body.LastCommitSha
	}
	if body.Labels != nil {
		svc.Labels = body.Labels
	}
	if body.Metadata != nil {
		svc.Metadata = body.Metadata
	}
	svc.UpdatedBy = &p.UserID
	svc.UpdatedByCommitHash = body.CommitHash

	if err := h.store.UpdateService(r.Context(), *svc); err != nil {
		if errors.Is(err, storepkg.ErrTeamNotFound) {
			httputil.BadRequest(w, fmt.Sprintf("team %q does not exist", svc.TeamName))
			return
		}
		if errors.Is(err, storepkg.ErrServiceNameExists) {
			httputil.Conflict(w, fmt.Sprintf("a service named %q already exists in this organization", svc.Name))
			return
		}
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, svc)
}

// Delete
// @Summary  Delete
// @Tags     services
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if err := h.store.SoftDeleteService(r.Context(), r.PathValue("serviceID"), p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
