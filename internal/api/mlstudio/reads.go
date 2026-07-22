package mlstudio

import (
	"net/http"

	"github.com/uigraph/app/internal/httputil"
	storepkg "github.com/uigraph/app/internal/store"
)

func (h *Handler) ListModels(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	models, err := h.store.ListMLModels(r.Context(), orgID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"models": models})
}

func (h *Handler) GetModel(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	m, err := h.store.GetMLModel(r.Context(), orgID, r.PathValue("modelId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if m == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, m)
}

func (h *Handler) ListVersions(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	versions, err := h.store.ListMLModelVersions(r.Context(), orgID, r.PathValue("modelId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"versions": versions})
}

func (h *Handler) ListAllVersions(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	versions, err := h.store.ListMLModelVersions(r.Context(), orgID, r.URL.Query().Get("modelId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"versions": versions})
}

func (h *Handler) ListAllRuns(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	runs, err := h.store.ListMLRuns(r.Context(), orgID, r.URL.Query().Get("experimentId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"runs": runs})
}

func (h *Handler) ListAllArtifacts(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	artifacts, err := h.store.ListMLArtifacts(r.Context(), orgID, r.URL.Query().Get("runId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"artifacts": artifacts})
}

func (h *Handler) GetVersion(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	v, err := h.store.GetMLModelVersion(r.Context(), orgID, r.PathValue("versionId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if v == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, v)
}

func (h *Handler) ListVersionEvaluations(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	evals, err := h.store.ListMLVersionEvaluations(r.Context(), orgID, r.PathValue("versionId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"evaluations": evals})
}

func (h *Handler) ListExperiments(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	experiments, err := h.store.ListMLExperiments(r.Context(), orgID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"experiments": experiments})
}

func (h *Handler) GetExperiment(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	e, err := h.store.GetMLExperiment(r.Context(), orgID, r.PathValue("experimentId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if e == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, e)
}

func (h *Handler) ListRuns(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	runs, err := h.store.ListMLRuns(r.Context(), orgID, r.PathValue("experimentId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"runs": runs})
}

func (h *Handler) GetRun(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	run, err := h.store.GetMLRun(r.Context(), orgID, r.PathValue("runId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if run == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, run)
}

func (h *Handler) ListRunSeries(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	points, err := h.store.ListMLRunMetricPoints(r.Context(), orgID, r.PathValue("runId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"points": points})
}

func (h *Handler) ListRunArtifacts(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	artifacts, err := h.store.ListMLArtifacts(r.Context(), orgID, r.PathValue("runId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"artifacts": artifacts})
}

func (h *Handler) ListEvaluationDatasets(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	datasets, err := h.store.ListMLEvaluationDatasets(r.Context(), orgID, r.URL.Query().Get("experimentId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"evaluationDatasets": datasets})
}

func (h *Handler) GetEvaluationDataset(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	ds, err := h.store.GetMLEvaluationDataset(r.Context(), orgID, r.PathValue("datasetId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if ds == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, ds)
}

func (h *Handler) ListDatasets(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	datasets, err := h.store.ListMLDatasets(r.Context(), orgID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"datasets": datasets})
}

func (h *Handler) GetDataset(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	ds, err := h.store.GetMLDataset(r.Context(), orgID, r.PathValue("datasetId"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if ds == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, ds)
}
