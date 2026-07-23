package mlstudio

import (
	"net/http"

	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/mlstudio"
	storepkg "github.com/uigraph/app/internal/store"
)

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
		Name        string `json:"name"`
		Type        string `json:"type"`
		Description string `json:"description"`
		Team        string `json:"team"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" || body.Type == "" {
		httputil.BadRequest(w, "name and type are required")
		return
	}
	created, err := h.store.CreateMLProject(r.Context(), orgID, p.UserID, mlstudio.ProjectInput{
		Name:        body.Name,
		Type:        body.Type,
		Description: body.Description,
		Team:        body.Team,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, created)
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
	if err := h.store.UpsertMLRuns(r.Context(), orgID, p.UserID, in); err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"synced": len(in)})
}

func (h *Handler) SyncRunSeries(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	runMLflowID := r.PathValue("runId")
	var in []mlstudio.MetricPoint
	if err := httputil.Decode(r, &in); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if err := h.store.UpsertMLRunMetricPoints(r.Context(), orgID, runMLflowID, in); err != nil {
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
	if err := h.store.UpsertMLEvaluations(r.Context(), orgID, p.UserID, in); err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"synced": len(in)})
}
