package mlstudio

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/mlstudio"
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
		ModelID     string   `json:"modelId"`
		VersionID   *string  `json:"versionId"`
		Title       string   `json:"title"`
		Summary     string   `json:"summary"`
		Description string   `json:"description"`
		RunIDs      []string `json:"runIds"`
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
	f := mlstudio.Finding{
		ID:          uuid.NewString(),
		OrgID:       orgID,
		ModelID:     body.ModelID,
		VersionID:   body.VersionID,
		Title:       body.Title,
		Summary:     body.Summary,
		Description: body.Description,
		RunIDs:      body.RunIDs,
		CreatedBy:   p.UserID,
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
	findings, err := h.store.ListMLFindings(r.Context(), orgID, modelID)
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
		VersionID   *string   `json:"versionId"`
		Title       *string   `json:"title"`
		Summary     *string   `json:"summary"`
		Description *string   `json:"description"`
		RunIDs      *[]string `json:"runIds"`
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
	updates, err := h.store.ListVersionDeploymentUpdates(r.Context(), orgID, versionID)
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
	updates, err := h.store.ListVersionDeploymentUpdates(r.Context(), orgID, r.PathValue("versionId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"updates": updates})
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
