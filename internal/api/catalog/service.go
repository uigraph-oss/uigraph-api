package catalog

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	authmw "github.com/uigraph/app/internal/middleware"
	catalogpkg "github.com/uigraph/app/internal/catalog"
	"github.com/uigraph/app/internal/httputil"
	storepkg "github.com/uigraph/app/internal/store"
)

// ── Services ──────────────────────────────────────────────────────────────────

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	q := r.URL.Query()
	var folderID, teamID *string
	if v := q.Get("folderId"); v != "" {
		folderID = &v
	}
	if v := q.Get("teamId"); v != "" {
		teamID = &v
	}
	svcs, err := h.store.ListServices(r.Context(), orgID, folderID, teamID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"services": svcs})
}

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

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	var body struct {
		Name            string          `json:"name"`
		Slug            string          `json:"slug"`
		Description     string          `json:"description"`
		Status          string          `json:"status"`
		Tier            string          `json:"tier"`
		Category        string          `json:"category"`
		Language        string          `json:"language"`
		FolderID        *string         `json:"folderId"`
		TeamID          *string         `json:"teamId"`
		GitRepoURL      *string         `json:"gitRepoUrl"`
		JiraProjectURL  *string         `json:"jiraProjectUrl"`
		SlackChannelURL *string         `json:"slackChannelUrl"`
		Labels          []string        `json:"labels"`
		Metadata        json.RawMessage `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}
	if body.Slug == "" {
		body.Slug = toSlug(body.Name)
	}
	if body.Status == "" {
		body.Status = "active"
	}
	if body.Tier == "" {
		body.Tier = "tier3"
	}

	now := time.Now().UTC()
	svc := catalogpkg.Service{
		ID:              uuid.NewString(),
		OrgID:           orgID,
		FolderID:        body.FolderID,
		TeamID:          body.TeamID,
		Name:            body.Name,
		Slug:            body.Slug,
		Description:     body.Description,
		Status:          body.Status,
		Tier:            body.Tier,
		Category:        body.Category,
		Language:        body.Language,
		GitRepoURL:      body.GitRepoURL,
		JiraProjectURL:  body.JiraProjectURL,
		SlackChannelURL: body.SlackChannelURL,
		Labels:          body.Labels,
		Metadata:        body.Metadata,
		CreatedBy:       p.UserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := h.store.CreateService(r.Context(), svc); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, svc)
}

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
		Slug            *string         `json:"slug"`
		Description     *string         `json:"description"`
		Status          *string         `json:"status"`
		Tier            *string         `json:"tier"`
		Category        *string         `json:"category"`
		Language        *string         `json:"language"`
		FolderID        *string         `json:"folderId"`
		TeamID          *string         `json:"teamId"`
		GitRepoURL      *string         `json:"gitRepoUrl"`
		JiraProjectURL  *string         `json:"jiraProjectUrl"`
		SlackChannelURL *string         `json:"slackChannelUrl"`
		LastCommitSha   *string         `json:"lastCommitSha"`
		Labels          []string        `json:"labels"`
		Metadata        json.RawMessage `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name != nil {
		svc.Name = *body.Name
	}
	if body.Slug != nil {
		svc.Slug = *body.Slug
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
	if body.TeamID != nil {
		svc.TeamID = body.TeamID
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

	if err := h.store.UpdateService(r.Context(), *svc); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, svc)
}

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
