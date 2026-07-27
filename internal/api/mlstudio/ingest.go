package mlstudio

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/mlstudio"
	storepkg "github.com/uigraph/app/internal/store"
)

func (h *Handler) resolveActors(ctx context.Context, orgID string, emails []string) (map[string]string, error) {
	wanted := map[string]bool{}
	for _, email := range emails {
		lower := strings.ToLower(strings.TrimSpace(email))
		if lower == "" {
			continue
		}
		wanted[lower] = true
	}
	if len(wanted) == 0 {
		return map[string]string{}, nil
	}
	list := make([]string, 0, len(wanted))
	for email := range wanted {
		list = append(list, email)
	}
	resolved, err := h.store.ResolveOrgUserIDsByEmail(ctx, orgID, list)
	if err != nil {
		return nil, err
	}
	var missing []string
	for _, email := range list {
		if _, ok := resolved[email]; !ok {
			missing = append(missing, email)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("%w: %s", mlstudio.ErrUnknownUser, strings.Join(missing, ", "))
	}
	return resolved, nil
}

func (h *Handler) SyncProjects(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	var in []mlstudio.ProjectInput
	if err := httputil.Decode(r, &in); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	for i := range in {
		if in[i].Type != "model" && in[i].Type != "training" {
			httputil.BadRequest(w, "type must be one of model, training")
			return
		}
	}
	if err := h.store.UpsertMLProjects(r.Context(), orgID, p.UserID, in); err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"synced": len(in)})
}

func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	var body struct {
		Name        string  `json:"name"`
		Type        string  `json:"type"`
		Description string  `json:"description"`
		TeamID      *string `json:"teamId"`
		TeamName    string  `json:"teamName"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" || body.Type == "" {
		httputil.BadRequest(w, "name and type are required")
		return
	}
	if body.Type != "model" && body.Type != "training" {
		httputil.BadRequest(w, "type must be one of model, training")
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
	created, err := h.store.CreateMLProject(r.Context(), orgID, p.UserID, mlstudio.ProjectInput{
		Name:        body.Name,
		Type:        body.Type,
		Description: body.Description,
		TeamID:      body.TeamID,
		TeamName:    body.TeamName,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, created)
}

func (h *Handler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	existing, err := h.store.GetMLProject(r.Context(), orgID, r.PathValue("projectId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	var body struct {
		Name        *string `json:"name"`
		Type        *string `json:"type"`
		Description *string `json:"description"`
		TeamID      *string `json:"teamId"`
		TeamName    string  `json:"teamName"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	in := mlstudio.ProjectInput{
		Name:        existing.Name,
		Type:        existing.Type,
		Description: existing.Description,
		TeamID:      existing.TeamID,
		TeamName:    body.TeamName,
	}
	if body.Name != nil {
		if *body.Name == "" {
			httputil.BadRequest(w, "name cannot be empty")
			return
		}
		in.Name = *body.Name
	}
	if body.Type != nil {
		if *body.Type != "model" && *body.Type != "training" {
			httputil.BadRequest(w, "type must be one of model, training")
			return
		}
		in.Type = *body.Type
	}
	if body.Description != nil {
		in.Description = *body.Description
	}
	if body.TeamID != nil {
		if *body.TeamID == "" {
			httputil.BadRequest(w, "team is required")
			return
		}
		in.TeamID = body.TeamID
	}
	if body.TeamID != nil && body.TeamName != "" {
		httputil.BadRequest(w, "provide either teamId or teamName, not both")
		return
	}
	updated, err := h.store.UpdateMLProject(r.Context(), orgID, existing.ID, p.UserID, in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, updated)
}

func (h *Handler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	existing, err := h.store.GetMLProject(r.Context(), orgID, r.PathValue("projectId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if err := h.store.DeleteMLProject(r.Context(), orgID, existing.ID, p.UserID); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) SyncModels(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	var in []mlstudio.ModelInput
	if err := httputil.Decode(r, &in); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	emails := make([]string, 0, len(in))
	for i := range in {
		emails = append(emails, in[i].UserEmail)
	}
	actors, err := h.resolveActors(r.Context(), orgID, emails)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	for i := range in {
		if in[i].UserEmail == "" {
			continue
		}
		in[i].ActorID = actors[strings.ToLower(strings.TrimSpace(in[i].UserEmail))]
	}
	for i := range in {
		if in[i].ProblemType == "" {
			in[i].ProblemType = "other"
		}
		if in[i].ProblemType != "classification" && in[i].ProblemType != "regression" && in[i].ProblemType != "ranking" && in[i].ProblemType != "generation" && in[i].ProblemType != "embedding" && in[i].ProblemType != "other" {
			httputil.BadRequest(w, "problemType must be one of classification, regression, ranking, generation, embedding, other")
			return
		}
	}
	if err := h.store.UpsertMLModels(r.Context(), orgID, p.UserID, in); err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"synced": len(in)})
}

func (h *Handler) UpdateModel(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	id := r.PathValue("modelId")
	var in mlstudio.ModelUpdateInput
	if err := httputil.Decode(r, &in); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if in.ProblemType == "" {
		in.ProblemType = "other"
	}
	if in.ProblemType != "classification" && in.ProblemType != "regression" && in.ProblemType != "ranking" && in.ProblemType != "generation" && in.ProblemType != "embedding" && in.ProblemType != "other" {
		httputil.BadRequest(w, "problemType must be one of classification, regression, ranking, generation, embedding, other")
		return
	}
	existing, err := h.store.GetMLModel(r.Context(), orgID, id)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if err := h.store.UpdateMLModel(r.Context(), orgID, id, p.UserID, in); err != nil {
		writeErr(w, r, err)
		return
	}
	updated, err := h.store.GetMLModel(r.Context(), orgID, id)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, updated)
}

func (h *Handler) SyncVersions(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	var in []mlstudio.ModelVersionInput
	if err := httputil.Decode(r, &in); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	emails := make([]string, 0, len(in))
	for i := range in {
		emails = append(emails, in[i].UserEmail)
	}
	actors, err := h.resolveActors(r.Context(), orgID, emails)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	for i := range in {
		if in[i].UserEmail == "" {
			continue
		}
		in[i].ActorID = actors[strings.ToLower(strings.TrimSpace(in[i].UserEmail))]
	}
	if err := h.store.UpsertMLModelVersions(r.Context(), orgID, p.UserID, in); err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"synced": len(in)})
}

func (h *Handler) SyncExperiments(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	var in []mlstudio.ExperimentInput
	if err := httputil.Decode(r, &in); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	emails := make([]string, 0, len(in))
	for i := range in {
		emails = append(emails, in[i].UserEmail)
	}
	actors, err := h.resolveActors(r.Context(), orgID, emails)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	for i := range in {
		if in[i].UserEmail == "" {
			continue
		}
		in[i].ActorID = actors[strings.ToLower(strings.TrimSpace(in[i].UserEmail))]
	}
	for i := range in {
		if in[i].Status == "" {
			in[i].Status = "active"
		}
		if in[i].Status != "active" && in[i].Status != "concluded" && in[i].Status != "archived" {
			httputil.BadRequest(w, "status must be one of active, concluded, archived")
			return
		}
	}
	if err := h.store.UpsertMLExperiments(r.Context(), orgID, p.UserID, in); err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"synced": len(in)})
}

func (h *Handler) SyncRuns(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	var in []mlstudio.RunInput
	if err := httputil.Decode(r, &in); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	now := time.Now().UTC()
	for i := range in {
		if in[i].StartedAt == nil {
			in[i].StartedAt = &now
		}
	}
	emails := make([]string, 0, len(in))
	for i := range in {
		emails = append(emails, in[i].UserEmail)
	}
	actors, err := h.resolveActors(r.Context(), orgID, emails)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	for i := range in {
		if in[i].UserEmail == "" {
			continue
		}
		in[i].ActorID = actors[strings.ToLower(strings.TrimSpace(in[i].UserEmail))]
	}
	for i := range in {
		if in[i].Status == "" {
			in[i].Status = "running"
		}
		if in[i].Status != "running" && in[i].Status != "completed" && in[i].Status != "failed" && in[i].Status != "cancelled" {
			httputil.BadRequest(w, "status must be one of running, completed, failed, cancelled")
			return
		}
	}
	if err := h.store.UpsertMLRuns(r.Context(), orgID, p.UserID, in); err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"synced": len(in)})
}

func (h *Handler) SyncArtifacts(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	var in []mlstudio.ArtifactInput
	if err := httputil.Decode(r, &in); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	emails := make([]string, 0, len(in))
	for i := range in {
		emails = append(emails, in[i].UserEmail)
	}
	actors, err := h.resolveActors(r.Context(), orgID, emails)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	for i := range in {
		if in[i].UserEmail == "" {
			continue
		}
		in[i].ActorID = actors[strings.ToLower(strings.TrimSpace(in[i].UserEmail))]
	}
	if err := h.store.UpsertMLArtifacts(r.Context(), orgID, p.UserID, in); err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"synced": len(in)})
}

func (h *Handler) SyncDatasets(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	var in []mlstudio.DatasetInput
	if err := httputil.Decode(r, &in); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	emails := make([]string, 0, len(in))
	for i := range in {
		emails = append(emails, in[i].UserEmail)
	}
	actors, err := h.resolveActors(r.Context(), orgID, emails)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	for i := range in {
		if in[i].UserEmail == "" {
			continue
		}
		in[i].ActorID = actors[strings.ToLower(strings.TrimSpace(in[i].UserEmail))]
	}
	for i := range in {
		if in[i].Context == "" {
			in[i].Context = "training"
		}
		if in[i].Context != "training" && in[i].Context != "evaluation" {
			httputil.BadRequest(w, "context must be one of training, evaluation")
			return
		}
	}
	if err := h.store.UpsertMLDatasets(r.Context(), orgID, p.UserID, in); err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"synced": len(in)})
}

func (h *Handler) SyncEvaluations(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	var in []mlstudio.EvaluationInput
	if err := httputil.Decode(r, &in); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	now := time.Now().UTC()
	for i := range in {
		if in[i].StartedAt == nil {
			in[i].StartedAt = &now
		}
	}
	emails := make([]string, 0, len(in))
	for i := range in {
		emails = append(emails, in[i].UserEmail)
	}
	actors, err := h.resolveActors(r.Context(), orgID, emails)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	for i := range in {
		if in[i].UserEmail == "" {
			continue
		}
		in[i].ActorID = actors[strings.ToLower(strings.TrimSpace(in[i].UserEmail))]
	}
	for i := range in {
		if in[i].Type != "Offline Benchmark" && in[i].Type != "Online A/B Test" && in[i].Type != "Human Review" && in[i].Type != "Production Monitoring" {
			httputil.BadRequest(w, "type must be one of Offline Benchmark, Online A/B Test, Human Review, Production Monitoring")
			return
		}
	}
	if err := h.store.UpsertMLEvaluations(r.Context(), orgID, p.UserID, in); err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"synced": len(in)})
}
