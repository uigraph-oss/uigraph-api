package mlstudio

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/mlstudio"
	"github.com/uigraph/app/internal/storage"
	storepkg "github.com/uigraph/app/internal/store"
)

func (h *Handler) CreateDeployment(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	var body struct {
		ModelID      string     `json:"modelId"`
		VersionID    string     `json:"versionId"`
		Name         string     `json:"name"`
		Environment  string     `json:"environment"`
		Status       string     `json:"status"`
		Endpoint     string     `json:"endpoint"`
		Region       string     `json:"region"`
		DeployedAt   *time.Time `json:"deployedAt"`
		RolledBackAt *time.Time `json:"rolledBackAt"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.ModelID == "" || body.VersionID == "" || body.Name == "" {
		httputil.BadRequest(w, "modelId, versionId and name are required")
		return
	}
	if !h.ensureModelInOrg(w, r, orgID, body.ModelID) {
		return
	}
	if !h.ensureVersionInOrg(w, r, orgID, body.VersionID) {
		return
	}
	dep := mlstudio.Deployment{
		ID:           uuid.NewString(),
		OrgID:        orgID,
		ModelID:      body.ModelID,
		VersionID:    body.VersionID,
		Name:         body.Name,
		Environment:  body.Environment,
		Status:       body.Status,
		Endpoint:     body.Endpoint,
		Region:       body.Region,
		DeployedAt:   body.DeployedAt,
		RolledBackAt: body.RolledBackAt,
		CreatedBy:    p.UserID,
	}
	if dep.Status == "" {
		dep.Status = "live"
	}
	if dep.Status != "live" && dep.Status != "rolling-out" && dep.Status != "rolled-back" && dep.Status != "stopped" {
		httputil.BadRequest(w, "status must be one of live, rolling-out, rolled-back, stopped")
		return
	}
	if err := h.store.CreateMLDeployment(r.Context(), dep); err != nil {
		writeErr(w, r, err)
		return
	}
	created, err := h.store.GetMLDeployment(r.Context(), orgID, dep.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, created)
}

func (h *Handler) ListDeployments(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	modelID := r.URL.Query().Get("modelId")
	versionID := r.URL.Query().Get("versionId")
	deployments, err := h.store.ListMLDeployments(r.Context(), orgID, modelID, versionID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"deployments": deployments})
}

func (h *Handler) GetDeployment(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	dep, err := h.store.GetMLDeployment(r.Context(), orgID, r.PathValue("deploymentId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if dep == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, dep)
}

func (h *Handler) UpdateDeployment(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	existing, err := h.store.GetMLDeployment(r.Context(), orgID, r.PathValue("deploymentId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	var body struct {
		Name         *string    `json:"name"`
		Environment  *string    `json:"environment"`
		Status       *string    `json:"status"`
		Endpoint     *string    `json:"endpoint"`
		Region       *string    `json:"region"`
		DeployedAt   *time.Time `json:"deployedAt"`
		RolledBackAt *time.Time `json:"rolledBackAt"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name != nil {
		existing.Name = *body.Name
	}
	if body.Environment != nil {
		existing.Environment = *body.Environment
	}
	if body.Status != nil {
		if *body.Status != "live" && *body.Status != "rolling-out" && *body.Status != "rolled-back" && *body.Status != "stopped" {
			httputil.BadRequest(w, "status must be one of live, rolling-out, rolled-back, stopped")
			return
		}
		existing.Status = *body.Status
	}
	if body.Endpoint != nil {
		existing.Endpoint = *body.Endpoint
	}
	if body.Region != nil {
		existing.Region = *body.Region
	}
	if body.DeployedAt != nil {
		existing.DeployedAt = body.DeployedAt
	}
	if body.RolledBackAt != nil {
		existing.RolledBackAt = body.RolledBackAt
	}
	if err := h.store.UpdateMLDeployment(r.Context(), *existing); err != nil {
		writeErr(w, r, err)
		return
	}
	updated, err := h.store.GetMLDeployment(r.Context(), orgID, existing.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, updated)
}

func (h *Handler) DeleteDeployment(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteMLDeployment(r.Context(), orgID, r.PathValue("deploymentId"), p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateFinding(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	var body struct {
		ModelID       string   `json:"modelId"`
		VersionID     *string  `json:"versionId"`
		Title         string   `json:"title"`
		Summary       string   `json:"summary"`
		Description   string   `json:"description"`
		RunIDs        []string `json:"runIds"`
		EvaluationIDs []string `json:"evaluationIds"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.ModelID == "" || body.Title == "" {
		httputil.BadRequest(w, "modelId and title are required")
		return
	}
	if !h.ensureModelInOrg(w, r, orgID, body.ModelID) {
		return
	}
	if body.VersionID != nil && *body.VersionID != "" {
		if !h.ensureVersionInOrg(w, r, orgID, *body.VersionID) {
			return
		}
	}
	if !h.ensureRunsInOrg(w, r, orgID, body.RunIDs) {
		return
	}
	if !h.ensureEvaluationsInOrg(w, r, orgID, body.EvaluationIDs) {
		return
	}
	f := mlstudio.Finding{
		ID:            uuid.NewString(),
		OrgID:         orgID,
		ModelID:       body.ModelID,
		VersionID:     body.VersionID,
		Title:         body.Title,
		Summary:       body.Summary,
		Description:   body.Description,
		RunIDs:        body.RunIDs,
		EvaluationIDs: body.EvaluationIDs,
		CreatedBy:     p.UserID,
	}
	if err := h.store.CreateMLFinding(r.Context(), f); err != nil {
		writeErr(w, r, err)
		return
	}
	created, err := h.store.GetMLFinding(r.Context(), orgID, f.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, created)
}

func (h *Handler) ListFindings(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	modelID := r.URL.Query().Get("modelId")
	findings, err := h.store.ListMLFindings(r.Context(), orgID, modelID, r.URL.Query().Get("projectId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"findings": findings})
}

func (h *Handler) GetFinding(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	f, err := h.store.GetMLFinding(r.Context(), orgID, r.PathValue("findingId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if f == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, f)
}

func (h *Handler) UpdateFinding(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	existing, err := h.store.GetMLFinding(r.Context(), orgID, r.PathValue("findingId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	var body struct {
		VersionID     *string   `json:"versionId"`
		Title         *string   `json:"title"`
		Summary       *string   `json:"summary"`
		Description   *string   `json:"description"`
		RunIDs        *[]string `json:"runIds"`
		EvaluationIDs *[]string `json:"evaluationIds"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.VersionID != nil {
		if *body.VersionID != "" && !h.ensureVersionInOrg(w, r, orgID, *body.VersionID) {
			return
		}
		existing.VersionID = body.VersionID
	}
	if body.Title != nil {
		existing.Title = *body.Title
	}
	if body.Summary != nil {
		existing.Summary = *body.Summary
	}
	if body.Description != nil {
		existing.Description = *body.Description
	}
	if body.RunIDs != nil {
		if !h.ensureRunsInOrg(w, r, orgID, *body.RunIDs) {
			return
		}
		existing.RunIDs = *body.RunIDs
	}
	if body.EvaluationIDs != nil {
		if !h.ensureEvaluationsInOrg(w, r, orgID, *body.EvaluationIDs) {
			return
		}
		existing.EvaluationIDs = *body.EvaluationIDs
	}
	if err := h.store.UpdateMLFinding(r.Context(), *existing); err != nil {
		writeErr(w, r, err)
		return
	}
	updated, err := h.store.GetMLFinding(r.Context(), orgID, existing.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, updated)
}

func (h *Handler) DeleteFinding(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteMLFinding(r.Context(), orgID, r.PathValue("findingId"), p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateModel(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	var body struct {
		ProjectID   string   `json:"projectId"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Domain      string   `json:"domain"`
		ProblemType string   `json:"problemType"`
		Tags        []string `json:"tags"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.ProjectID == "" || body.Name == "" {
		httputil.BadRequest(w, "projectId and name are required")
		return
	}
	if !h.ensureProjectInOrg(w, r, orgID, body.ProjectID) {
		return
	}
	problemType := body.ProblemType
	if problemType == "" {
		problemType = "other"
	}
	if problemType != "classification" && problemType != "regression" && problemType != "ranking" && problemType != "generation" && problemType != "embedding" && problemType != "other" {
		httputil.BadRequest(w, "problemType must be one of classification, regression, ranking, generation, embedding, other")
		return
	}
	projectID := body.ProjectID
	m := mlstudio.Model{
		ID:          uuid.NewString(),
		OrgID:       orgID,
		ProjectID:   &projectID,
		Name:        body.Name,
		Description: body.Description,
		Domain:      body.Domain,
		ProblemType: problemType,
		Tags:        body.Tags,
		Origin:      "manual",
		CreatedBy:   p.UserID,
	}
	if err := h.store.CreateMLModel(r.Context(), m); err != nil {
		writeErr(w, r, err)
		return
	}
	v := mlstudio.ModelVersion{
		ID:        uuid.NewString(),
		OrgID:     orgID,
		ModelID:   m.ID,
		Version:   "1",
		Source:    "manual",
		CreatedBy: p.UserID,
	}
	if err := h.store.CreateMLModelVersion(r.Context(), v); err != nil {
		writeErr(w, r, err)
		return
	}
	created, err := h.store.GetMLModel(r.Context(), orgID, m.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, created)
}

func (h *Handler) UpdateModelInfo(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	existing, err := h.store.GetMLModel(r.Context(), orgID, r.PathValue("modelId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if existing.Origin != "manual" {
		httputil.BadRequest(w, "only manually registered models can be edited")
		return
	}
	var body struct {
		Name        *string   `json:"name"`
		Description *string   `json:"description"`
		Domain      *string   `json:"domain"`
		ProblemType *string   `json:"problemType"`
		Tags        *[]string `json:"tags"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name != nil {
		existing.Name = *body.Name
	}
	if body.Description != nil {
		existing.Description = *body.Description
	}
	if body.Domain != nil {
		existing.Domain = *body.Domain
	}
	if body.ProblemType != nil {
		if *body.ProblemType != "classification" && *body.ProblemType != "regression" && *body.ProblemType != "ranking" && *body.ProblemType != "generation" && *body.ProblemType != "embedding" && *body.ProblemType != "other" {
			httputil.BadRequest(w, "problemType must be one of classification, regression, ranking, generation, embedding, other")
			return
		}
		existing.ProblemType = *body.ProblemType
	}
	if body.Tags != nil {
		existing.Tags = *body.Tags
	}
	if err := h.store.UpdateMLModelInfo(r.Context(), *existing); err != nil {
		writeErr(w, r, err)
		return
	}
	updated, err := h.store.GetMLModel(r.Context(), orgID, existing.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, updated)
}

func (h *Handler) DeleteModel(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	existing, err := h.store.GetMLModel(r.Context(), orgID, r.PathValue("modelId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if existing.Origin != "manual" {
		httputil.BadRequest(w, "only manually registered models can be deleted")
		return
	}
	if err := h.store.DeleteMLModel(r.Context(), orgID, existing.ID, p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateExperiment(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	var body struct {
		ProjectID   string   `json:"projectId"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Status      string   `json:"status"`
		Tags        []string `json:"tags"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.ProjectID == "" || body.Name == "" {
		httputil.BadRequest(w, "projectId and name are required")
		return
	}
	if !h.ensureProjectInOrg(w, r, orgID, body.ProjectID) {
		return
	}
	status := body.Status
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "concluded" && status != "archived" {
		httputil.BadRequest(w, "status must be one of active, concluded, archived")
		return
	}
	projectID := body.ProjectID
	e := mlstudio.Experiment{
		ID:          uuid.NewString(),
		OrgID:       orgID,
		ProjectID:   &projectID,
		Name:        body.Name,
		Description: body.Description,
		Status:      status,
		Tags:        body.Tags,
		Source:      "manual",
		CreatedBy:   p.UserID,
	}
	if err := h.store.CreateMLExperiment(r.Context(), e); err != nil {
		writeErr(w, r, err)
		return
	}
	created, err := h.store.GetMLExperiment(r.Context(), orgID, e.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, created)
}

func (h *Handler) UpdateExperiment(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	existing, err := h.store.GetMLExperiment(r.Context(), orgID, r.PathValue("experimentId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	var body struct {
		ProjectID   *string   `json:"projectId"`
		Name        *string   `json:"name"`
		Description *string   `json:"description"`
		Status      *string   `json:"status"`
		Tags        *[]string `json:"tags"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	// Tags are ML Studio metadata rather than MLflow state, so they stay
	// editable on synced experiments; every other field stays manual-only.
	syncedFields := body.ProjectID != nil || body.Name != nil || body.Description != nil ||
		body.Status != nil
	if existing.Source != "manual" && syncedFields {
		httputil.BadRequest(w, "only tags can be edited on synced experiments")
		return
	}
	if body.ProjectID != nil {
		if *body.ProjectID != "" && !h.ensureProjectInOrg(w, r, orgID, *body.ProjectID) {
			return
		}
		existing.ProjectID = body.ProjectID
	}
	if body.Name != nil {
		existing.Name = *body.Name
	}
	if body.Description != nil {
		existing.Description = *body.Description
	}
	if body.Status != nil {
		if *body.Status != "active" && *body.Status != "concluded" && *body.Status != "archived" {
			httputil.BadRequest(w, "status must be one of active, concluded, archived")
			return
		}
		existing.Status = *body.Status
	}
	if body.Tags != nil {
		existing.Tags = *body.Tags
	}
	existing.UpdatedBy = &p.UserID
	if err := h.store.UpdateMLExperiment(r.Context(), *existing); err != nil {
		writeErr(w, r, err)
		return
	}
	updated, err := h.store.GetMLExperiment(r.Context(), orgID, existing.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, updated)
}

func (h *Handler) DeleteExperiment(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	existing, err := h.store.GetMLExperiment(r.Context(), orgID, r.PathValue("experimentId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if existing.Source != "manual" {
		httputil.BadRequest(w, "only manually created experiments can be deleted")
		return
	}
	if err := h.store.DeleteMLExperiment(r.Context(), orgID, existing.ID, p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateRun(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	experimentID := r.PathValue("experimentId")
	if !h.ensureExperimentInOrg(w, r, orgID, experimentID) {
		return
	}
	var body struct {
		Name       string         `json:"name"`
		Status     string         `json:"status"`
		StartedAt  *time.Time     `json:"startedAt"`
		EndedAt    *time.Time     `json:"endedAt"`
		Notes      string         `json:"notes"`
		Tags       []string       `json:"tags"`
		Parameters map[string]any `json:"parameters"`
		Metrics    map[string]any `json:"metrics"`
		DatasetID  *string        `json:"datasetId"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}
	if body.StartedAt == nil {
		httputil.BadRequest(w, "startedAt is required")
		return
	}
	if body.EndedAt != nil && body.EndedAt.Before(*body.StartedAt) {
		httputil.BadRequest(w, "endedAt must not be before startedAt")
		return
	}
	status := body.Status
	if status == "" {
		status = "running"
	}
	if status != "running" && status != "completed" && status != "failed" && status != "cancelled" {
		httputil.BadRequest(w, "status must be one of running, completed, failed, cancelled")
		return
	}
	run := mlstudio.Run{
		ID:           uuid.NewString(),
		OrgID:        orgID,
		ExperimentID: experimentID,
		Name:         body.Name,
		Status:       status,
		StartedAt:    *body.StartedAt,
		EndedAt:      body.EndedAt,
		Notes:        body.Notes,
		Tags:         body.Tags,
		Parameters:   body.Parameters,
		Metrics:      body.Metrics,
		DatasetID:    body.DatasetID,
		Source:       "manual",
		CreatedBy:    p.UserID,
	}
	if err := h.store.CreateMLRun(r.Context(), run); err != nil {
		writeErr(w, r, err)
		return
	}
	created, err := h.store.GetMLRun(r.Context(), orgID, run.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, created)
}

func (h *Handler) UpdateRun(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	existing, err := h.store.GetMLRun(r.Context(), orgID, r.PathValue("runId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if existing.Source != "manual" {
		httputil.BadRequest(w, "only manually created runs can be edited")
		return
	}
	var body struct {
		Name       *string        `json:"name"`
		Status     *string        `json:"status"`
		StartedAt  *time.Time     `json:"startedAt"`
		EndedAt    *time.Time     `json:"endedAt"`
		Notes      *string        `json:"notes"`
		Tags       *[]string      `json:"tags"`
		Parameters map[string]any `json:"parameters"`
		Metrics    map[string]any `json:"metrics"`
		DatasetID  *string        `json:"datasetId"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name != nil {
		existing.Name = *body.Name
	}
	if body.Status != nil {
		if *body.Status != "running" && *body.Status != "completed" && *body.Status != "failed" && *body.Status != "cancelled" {
			httputil.BadRequest(w, "status must be one of running, completed, failed, cancelled")
			return
		}
		existing.Status = *body.Status
	}
	if body.StartedAt == nil {
		httputil.BadRequest(w, "startedAt is required")
		return
	}
	if body.EndedAt != nil && body.EndedAt.Before(*body.StartedAt) {
		httputil.BadRequest(w, "endedAt must not be before startedAt")
		return
	}
	existing.StartedAt = *body.StartedAt
	existing.EndedAt = body.EndedAt
	if body.Notes != nil {
		existing.Notes = *body.Notes
	}
	if body.Tags != nil {
		existing.Tags = *body.Tags
	}
	if body.Parameters != nil {
		existing.Parameters = body.Parameters
	}
	if body.Metrics != nil {
		existing.Metrics = body.Metrics
	}
	if body.DatasetID != nil {
		existing.DatasetID = body.DatasetID
	}
	if err := h.store.UpdateMLRun(r.Context(), *existing); err != nil {
		writeErr(w, r, err)
		return
	}
	updated, err := h.store.GetMLRun(r.Context(), orgID, existing.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, updated)
}

func (h *Handler) DeleteRun(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	existing, err := h.store.GetMLRun(r.Context(), orgID, r.PathValue("runId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if existing.Source != "manual" {
		httputil.BadRequest(w, "only manually created runs can be deleted")
		return
	}
	if err := h.store.DeleteMLRun(r.Context(), orgID, existing.ID, p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateDataset(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	experimentID := r.PathValue("experimentId")
	if !h.ensureExperimentInOrg(w, r, orgID, experimentID) {
		return
	}
	var body struct {
		Name       string   `json:"name"`
		Digest     string   `json:"digest"`
		Source     string   `json:"source"`
		SourceType string   `json:"sourceType"`
		Context    string   `json:"context"`
		RowCount   int64    `json:"rowCount"`
		Tags       []string `json:"tags"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}
	context := body.Context
	if context == "" {
		context = "training"
	}
	if context != "training" && context != "evaluation" {
		httputil.BadRequest(w, "context must be one of training, evaluation")
		return
	}
	ds := mlstudio.Dataset{
		ID:           uuid.NewString(),
		OrgID:        orgID,
		ExperimentID: experimentID,
		Name:         body.Name,
		Digest:       body.Digest,
		Source:       body.Source,
		SourceType:   body.SourceType,
		Context:      context,
		RowCount:     body.RowCount,
		Tags:         body.Tags,
		Origin:       "manual",
		CreatedBy:    p.UserID,
	}
	if err := h.store.CreateMLDataset(r.Context(), ds); err != nil {
		writeErr(w, r, err)
		return
	}
	created, err := h.store.GetMLDataset(r.Context(), orgID, ds.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, created)
}

func (h *Handler) UpdateDataset(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	existing, err := h.store.GetMLDataset(r.Context(), orgID, r.PathValue("datasetId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if existing.Origin != "manual" {
		httputil.BadRequest(w, "only manually logged datasets can be edited")
		return
	}
	var body struct {
		Name       *string   `json:"name"`
		Digest     *string   `json:"digest"`
		Source     *string   `json:"source"`
		SourceType *string   `json:"sourceType"`
		Context    *string   `json:"context"`
		RowCount   *int64    `json:"rowCount"`
		Tags       *[]string `json:"tags"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name != nil {
		existing.Name = *body.Name
	}
	if body.Digest != nil {
		existing.Digest = *body.Digest
	}
	if body.Source != nil {
		existing.Source = *body.Source
	}
	if body.SourceType != nil {
		existing.SourceType = *body.SourceType
	}
	if body.Context != nil {
		if *body.Context != "training" && *body.Context != "evaluation" {
			httputil.BadRequest(w, "context must be one of training, evaluation")
			return
		}
		existing.Context = *body.Context
	}
	if body.RowCount != nil {
		existing.RowCount = *body.RowCount
	}
	if body.Tags != nil {
		existing.Tags = *body.Tags
	}
	if err := h.store.UpdateMLDataset(r.Context(), *existing); err != nil {
		writeErr(w, r, err)
		return
	}
	updated, err := h.store.GetMLDataset(r.Context(), orgID, existing.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, updated)
}

func (h *Handler) DeleteDataset(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	existing, err := h.store.GetMLDataset(r.Context(), orgID, r.PathValue("datasetId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if existing.Origin != "manual" {
		httputil.BadRequest(w, "only manually logged datasets can be deleted")
		return
	}
	if err := h.store.DeleteMLDataset(r.Context(), orgID, existing.ID, p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// maxArtifactUploadBytes caps manually uploaded artifacts at 5MB. This is the
// authoritative check; the browser enforces the same limit for feedback only.
const maxArtifactUploadBytes = 5 << 20

func (h *Handler) CreateArtifact(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	runID := r.PathValue("runId")
	if !h.ensureRunsInOrg(w, r, orgID, []string{runID}) {
		return
	}
	var body struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		URI         string `json:"uri"`
		DownloadURI string `json:"downloadUri"`
		Size        string `json:"size"`
		Format      string `json:"format"`
		MimeType    string `json:"mimeType"`
		SizeBytes   *int64 `json:"sizeBytes"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}
	if body.Type == "" {
		httputil.BadRequest(w, "type is required")
		return
	}
	a := mlstudio.Artifact{
		ID:          uuid.NewString(),
		OrgID:       orgID,
		RunID:       runID,
		Name:        body.Name,
		Type:        body.Type,
		URI:         body.URI,
		DownloadURI: body.DownloadURI,
		Size:        body.Size,
		Format:      body.Format,
		Source:      "manual",
		MimeType:    body.MimeType,
		SizeBytes:   body.SizeBytes,
		CreatedBy:   p.UserID,
	}
	if err := h.store.CreateMLArtifact(r.Context(), a); err != nil {
		writeErr(w, r, err)
		return
	}
	created, err := h.store.GetMLArtifact(r.Context(), orgID, a.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, created)
}

func (h *Handler) UploadArtifact(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	runID := r.PathValue("runId")
	if !h.ensureRunsInOrg(w, r, orgID, []string{runID}) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxArtifactUploadBytes)
	file, header, err := r.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httputil.JSON(w, http.StatusRequestEntityTooLarge, map[string]any{
				"error": map[string]any{
					"code":    "file_too_large",
					"message": "file must be 5MB or smaller",
				},
			})
			return
		}
		httputil.BadRequest(w, "missing file")
		return
	}
	defer func() { _ = file.Close() }()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	artifactID := uuid.NewString()
	key := storage.MLArtifactKey(orgID, runID, artifactID, header.Filename)
	if err := h.storage.Upload(r.Context(), key, contentType, file, header.Size); err != nil {
		httputil.Error(w, r, err)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		name = header.Filename
	}
	artifactType := r.FormValue("type")
	if artifactType == "" {
		httputil.BadRequest(w, "type is required")
		return
	}
	size := header.Size
	a := mlstudio.Artifact{
		ID:         artifactID,
		OrgID:      orgID,
		RunID:      runID,
		Name:       name,
		Type:       artifactType,
		Size:       r.FormValue("size"),
		Format:     r.FormValue("format"),
		Source:     "manual",
		StorageKey: key,
		MimeType:   contentType,
		SizeBytes:  &size,
		CreatedBy:  p.UserID,
	}
	if err := h.store.CreateMLArtifact(r.Context(), a); err != nil {
		_ = h.storage.Delete(r.Context(), key)
		writeErr(w, r, err)
		return
	}
	created, err := h.store.GetMLArtifact(r.Context(), orgID, a.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	h.presignArtifactDownload(r, created)
	httputil.JSON(w, http.StatusCreated, created)
}

func (h *Handler) UpdateArtifact(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	existing, err := h.store.GetMLArtifact(r.Context(), orgID, r.PathValue("artifactId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if existing.Source != "manual" {
		httputil.BadRequest(w, "only manually added artifacts can be edited")
		return
	}
	var body struct {
		Name        *string `json:"name"`
		Type        *string `json:"type"`
		URI         *string `json:"uri"`
		DownloadURI *string `json:"downloadUri"`
		Size        *string `json:"size"`
		Format      *string `json:"format"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name != nil {
		existing.Name = *body.Name
	}
	if body.Type != nil {
		existing.Type = *body.Type
	}
	if body.URI != nil {
		existing.URI = *body.URI
	}
	if body.DownloadURI != nil {
		existing.DownloadURI = *body.DownloadURI
	}
	if body.Size != nil {
		existing.Size = *body.Size
	}
	if body.Format != nil {
		existing.Format = *body.Format
	}
	if err := h.store.UpdateMLArtifact(r.Context(), *existing); err != nil {
		writeErr(w, r, err)
		return
	}
	updated, err := h.store.GetMLArtifact(r.Context(), orgID, existing.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	h.presignArtifactDownload(r, updated)
	httputil.JSON(w, http.StatusOK, updated)
}

func (h *Handler) DeleteArtifact(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	existing, err := h.store.GetMLArtifact(r.Context(), orgID, r.PathValue("artifactId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if existing.Source != "manual" {
		httputil.BadRequest(w, "only manually added artifacts can be deleted")
		return
	}
	if err := h.store.DeleteMLArtifact(r.Context(), orgID, existing.ID, p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing.StorageKey != "" {
		_ = h.storage.Delete(r.Context(), existing.StorageKey)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateEvaluation(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	experimentID := r.PathValue("experimentId")
	if !h.ensureExperimentInOrg(w, r, orgID, experimentID) {
		return
	}
	var body struct {
		VersionID   string         `json:"versionId"`
		DatasetID   *string        `json:"datasetId"`
		Name        string         `json:"name"`
		Type        string         `json:"type"`
		Description string         `json:"description"`
		Summary     string         `json:"summary"`
		StartedAt   *time.Time     `json:"startedAt"`
		EndedAt     *time.Time     `json:"endedAt"`
		Evaluator   string         `json:"evaluator"`
		Tags        []string       `json:"tags"`
		Parameters  map[string]any `json:"parameters"`
		Metrics     map[string]any `json:"metrics"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}
	if body.VersionID == "" {
		httputil.BadRequest(w, "versionId is required")
		return
	}
	if body.Type != "Offline Benchmark" && body.Type != "Online A/B Test" && body.Type != "Human Review" && body.Type != "Production Monitoring" {
		httputil.BadRequest(w, "type must be one of Offline Benchmark, Online A/B Test, Human Review, Production Monitoring")
		return
	}
	if body.StartedAt == nil {
		httputil.BadRequest(w, "startedAt is required")
		return
	}
	if body.EndedAt != nil && body.EndedAt.Before(*body.StartedAt) {
		httputil.BadRequest(w, "endedAt must not be before startedAt")
		return
	}
	version, err := h.store.GetMLModelVersion(r.Context(), orgID, body.VersionID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if version == nil {
		httputil.BadRequest(w, "version not found in org")
		return
	}
	eval := mlstudio.Evaluation{
		ID:           uuid.NewString(),
		OrgID:        orgID,
		ExperimentID: experimentID,
		VersionID:    body.VersionID,
		DatasetID:    body.DatasetID,
		Name:         body.Name,
		Type:         body.Type,
		Description:  body.Description,
		Summary:      body.Summary,
		StartedAt:    *body.StartedAt,
		EndedAt:      body.EndedAt,
		Evaluator:    body.Evaluator,
		Tags:         body.Tags,
		Parameters:   body.Parameters,
		Metrics:      body.Metrics,
		Source:       "manual",
		CreatedBy:    &p.UserID,
	}
	if err := h.store.CreateMLEvaluation(r.Context(), eval); err != nil {
		writeErr(w, r, err)
		return
	}
	created, err := h.store.GetMLEvaluation(r.Context(), orgID, eval.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, created)
}

func (h *Handler) UpdateEvaluation(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	existing, err := h.store.GetMLEvaluation(r.Context(), orgID, r.PathValue("evaluationId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if existing.Source != "manual" {
		httputil.BadRequest(w, "only manually logged evaluations can be edited")
		return
	}
	var body struct {
		DatasetID   *string        `json:"datasetId"`
		Name        *string        `json:"name"`
		Type        *string        `json:"type"`
		Description *string        `json:"description"`
		Summary     *string        `json:"summary"`
		StartedAt   *time.Time     `json:"startedAt"`
		EndedAt     *time.Time     `json:"endedAt"`
		Evaluator   *string        `json:"evaluator"`
		Tags        *[]string      `json:"tags"`
		Parameters  map[string]any `json:"parameters"`
		Metrics     map[string]any `json:"metrics"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name != nil {
		existing.Name = *body.Name
	}
	if body.Type != nil {
		if *body.Type != "Offline Benchmark" && *body.Type != "Online A/B Test" && *body.Type != "Human Review" && *body.Type != "Production Monitoring" {
			httputil.BadRequest(w, "type must be one of Offline Benchmark, Online A/B Test, Human Review, Production Monitoring")
			return
		}
		existing.Type = *body.Type
	}
	if body.Description != nil {
		existing.Description = *body.Description
	}
	if body.Summary != nil {
		existing.Summary = *body.Summary
	}
	if body.StartedAt == nil {
		httputil.BadRequest(w, "startedAt is required")
		return
	}
	if body.EndedAt != nil && body.EndedAt.Before(*body.StartedAt) {
		httputil.BadRequest(w, "endedAt must not be before startedAt")
		return
	}
	existing.StartedAt = *body.StartedAt
	existing.EndedAt = body.EndedAt
	if body.Evaluator != nil {
		existing.Evaluator = *body.Evaluator
	}
	if body.Tags != nil {
		existing.Tags = *body.Tags
	}
	if body.DatasetID != nil {
		existing.DatasetID = body.DatasetID
	}
	if body.Parameters != nil {
		existing.Parameters = body.Parameters
	}
	if body.Metrics != nil {
		existing.Metrics = body.Metrics
	}
	if err := h.store.UpdateMLEvaluation(r.Context(), *existing); err != nil {
		writeErr(w, r, err)
		return
	}
	updated, err := h.store.GetMLEvaluation(r.Context(), orgID, existing.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, updated)
}

func (h *Handler) DeleteEvaluation(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	existing, err := h.store.GetMLEvaluation(r.Context(), orgID, r.PathValue("evaluationId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if existing.Source != "manual" {
		httputil.BadRequest(w, "only manually logged evaluations can be deleted")
		return
	}
	if err := h.store.DeleteMLEvaluation(r.Context(), orgID, existing.ID, p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

var allowedVersionTransitions = map[string][]string{
	"candidate":  {"staging"},
	"staging":    {"production", "candidate"},
	"production": {"staging", "retired"},
	"retired":    {"staging"},
}

func (h *Handler) CreateVersionDeploymentUpdate(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	versionID := r.PathValue("versionId")
	version, err := h.store.GetMLModelVersion(r.Context(), orgID, versionID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if version == nil {
		httputil.BadRequest(w, "version not found in org")
		return
	}
	var body struct {
		ToStatus string `json:"toStatus"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	from := version.DeploymentStatus
	allowed := false
	for _, to := range allowedVersionTransitions[from] {
		if to == body.ToStatus {
			allowed = true
		}
	}
	if !allowed {
		httputil.BadRequest(w, fmt.Sprintf("invalid transition from %q to %q", from, body.ToStatus))
		return
	}
	fromStatus := from
	u := mlstudio.VersionDeploymentUpdate{
		ID:         uuid.NewString(),
		OrgID:      orgID,
		VersionID:  versionID,
		FromStatus: &fromStatus,
		ToStatus:   body.ToStatus,
		ChangedBy:  p.UserID,
	}
	if err := h.store.CreateVersionDeploymentUpdate(r.Context(), u); err != nil {
		writeErr(w, r, err)
		return
	}
	updates, err := h.store.ListVersionDeploymentUpdates(r.Context(), orgID, versionID, "")
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, updates[0])
}

func (h *Handler) ListVersionDeploymentUpdates(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	updates, err := h.store.ListVersionDeploymentUpdates(r.Context(), orgID, r.PathValue("versionId"), r.URL.Query().Get("projectId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"updates": updates})
}

func (h *Handler) SetVersionRun(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	versionID := r.PathValue("versionId")
	if !h.ensureVersionInOrg(w, r, orgID, versionID) {
		return
	}
	var body struct {
		RunID *string `json:"runId"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	runID := body.RunID
	if runID != nil && *runID == "" {
		runID = nil
	}
	if runID != nil && !h.ensureRunsInOrg(w, r, orgID, []string{*runID}) {
		return
	}
	if err := h.store.SetMLModelVersionRun(r.Context(), orgID, versionID, runID, p.UserID); err != nil {
		writeErr(w, r, err)
		return
	}
	updated, err := h.store.GetMLModelVersion(r.Context(), orgID, versionID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, updated)
}

func (h *Handler) LinkVersionEvaluations(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	versionID := r.PathValue("versionId")
	if !h.ensureVersionInOrg(w, r, orgID, versionID) {
		return
	}
	var body struct {
		EvaluationIDs []string `json:"evaluationIds"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if len(body.EvaluationIDs) == 0 {
		httputil.BadRequest(w, "evaluationIds is required")
		return
	}
	for _, id := range body.EvaluationIDs {
		e, err := h.store.GetMLEvaluation(r.Context(), orgID, id)
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
		if e == nil {
			httputil.BadRequest(w, "evaluation not found in org: "+id)
			return
		}
	}
	if err := h.store.SetMLEvaluationsVersion(r.Context(), orgID, versionID, body.EvaluationIDs, p.UserID); err != nil {
		writeErr(w, r, err)
		return
	}
	evals, _, err := h.store.ListMLVersionEvaluations(r.Context(), orgID, versionID, mlstudio.EvaluationQuery{})
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"evaluations": evals})
}

func (h *Handler) ensureModelInOrg(w http.ResponseWriter, r *http.Request, orgID, modelID string) bool {
	m, err := h.store.GetMLModel(r.Context(), orgID, modelID)
	if err != nil {
		httputil.Error(w, r, err)
		return false
	}
	if m == nil {
		httputil.BadRequest(w, "model not found in org")
		return false
	}
	return true
}

func (h *Handler) ensureVersionInOrg(w http.ResponseWriter, r *http.Request, orgID, versionID string) bool {
	v, err := h.store.GetMLModelVersion(r.Context(), orgID, versionID)
	if err != nil {
		httputil.Error(w, r, err)
		return false
	}
	if v == nil {
		httputil.BadRequest(w, "version not found in org")
		return false
	}
	return true
}

func (h *Handler) ensureProjectInOrg(w http.ResponseWriter, r *http.Request, orgID, projectID string) bool {
	proj, err := h.store.GetMLProject(r.Context(), orgID, projectID)
	if err != nil {
		httputil.Error(w, r, err)
		return false
	}
	if proj == nil {
		httputil.BadRequest(w, "project not found in org")
		return false
	}
	return true
}

func (h *Handler) ensureExperimentInOrg(w http.ResponseWriter, r *http.Request, orgID, experimentID string) bool {
	e, err := h.store.GetMLExperiment(r.Context(), orgID, experimentID)
	if err != nil {
		httputil.Error(w, r, err)
		return false
	}
	if e == nil {
		httputil.BadRequest(w, "experiment not found in org")
		return false
	}
	return true
}

func (h *Handler) ensureRunsInOrg(w http.ResponseWriter, r *http.Request, orgID string, runIDs []string) bool {
	for _, runID := range runIDs {
		run, err := h.store.GetMLRun(r.Context(), orgID, runID)
		if err != nil {
			httputil.Error(w, r, err)
			return false
		}
		if run == nil {
			httputil.BadRequest(w, "run not found in org: "+runID)
			return false
		}
	}
	return true
}

func (h *Handler) ensureEvaluationsInOrg(w http.ResponseWriter, r *http.Request, orgID string, evaluationIDs []string) bool {
	for _, evaluationID := range evaluationIDs {
		evaluation, err := h.store.GetMLEvaluation(r.Context(), orgID, evaluationID)
		if err != nil {
			httputil.Error(w, r, err)
			return false
		}
		if evaluation == nil {
			httputil.BadRequest(w, "evaluation not found in org: "+evaluationID)
			return false
		}
	}
	return true
}
