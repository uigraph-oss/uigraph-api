package mlstudio

import (
	"errors"
	"net/http"

	"github.com/lib/pq"

	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/identity"
	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/mlstudio"
)

type Handler struct {
	store mlstudio.Store
}

func New(s mlstudio.Store) *Handler {
	return &Handler{store: s}
}

func Register(
	mux *http.ServeMux,
	s mlstudio.Store,
	requireScope func(scope, method, pattern string, h http.HandlerFunc),
) {
	h := New(s)
	const base = "/api/v1/orgs/{orgID}/ml"

	requireScope("mlstudio:write", "POST", base+"/models/sync", h.SyncModels)
	requireScope("mlstudio:write", "POST", base+"/versions/sync", h.SyncVersions)
	requireScope("mlstudio:write", "POST", base+"/experiments/sync", h.SyncExperiments)
	requireScope("mlstudio:write", "POST", base+"/runs/sync", h.SyncRuns)
	requireScope("mlstudio:write", "POST", base+"/runs/{runId}/series/sync", h.SyncRunSeries)
	requireScope("mlstudio:write", "POST", base+"/artifacts/sync", h.SyncArtifacts)
	requireScope("mlstudio:write", "POST", base+"/datasets/sync", h.SyncDatasets)
	requireScope("mlstudio:write", "POST", base+"/evaluation-datasets/sync", h.SyncEvaluationDatasets)
	requireScope("mlstudio:write", "POST", base+"/evaluations/sync", h.SyncEvaluations)

	requireScope("mlstudio:write", "PATCH", base+"/models/{modelId}", h.UpdateModel)

	requireScope("mlstudio:read", "GET", base+"/models", h.ListModels)
	requireScope("mlstudio:read", "GET", base+"/models/{modelId}", h.GetModel)
	requireScope("mlstudio:read", "GET", base+"/models/{modelId}/versions", h.ListVersions)
	requireScope("mlstudio:read", "GET", base+"/versions", h.ListAllVersions)
	requireScope("mlstudio:read", "GET", base+"/runs", h.ListAllRuns)
	requireScope("mlstudio:read", "GET", base+"/artifacts", h.ListAllArtifacts)
	requireScope("mlstudio:read", "GET", base+"/versions/{versionId}", h.GetVersion)
	requireScope("mlstudio:read", "GET", base+"/versions/{versionId}/evaluations", h.ListVersionEvaluations)
	requireScope("mlstudio:read", "GET", base+"/experiments", h.ListExperiments)
	requireScope("mlstudio:read", "GET", base+"/experiments/{experimentId}", h.GetExperiment)
	requireScope("mlstudio:read", "GET", base+"/experiments/{experimentId}/runs", h.ListRuns)
	requireScope("mlstudio:read", "GET", base+"/runs/{runId}", h.GetRun)
	requireScope("mlstudio:read", "GET", base+"/runs/{runId}/series", h.ListRunSeries)
	requireScope("mlstudio:read", "GET", base+"/runs/{runId}/artifacts", h.ListRunArtifacts)
	requireScope("mlstudio:read", "GET", base+"/datasets", h.ListDatasets)
	requireScope("mlstudio:read", "GET", base+"/datasets/{datasetId}", h.GetDataset)
	requireScope("mlstudio:read", "GET", base+"/evaluation-datasets", h.ListEvaluationDatasets)
	requireScope("mlstudio:read", "GET", base+"/evaluation-datasets/{datasetId}", h.GetEvaluationDataset)

	requireScope("mlstudio:write", "POST", base+"/deployments", h.CreateDeployment)
	requireScope("mlstudio:read", "GET", base+"/deployments", h.ListDeployments)
	requireScope("mlstudio:read", "GET", base+"/deployments/{deploymentId}", h.GetDeployment)
	requireScope("mlstudio:write", "PUT", base+"/deployments/{deploymentId}", h.UpdateDeployment)
	requireScope("mlstudio:write", "DELETE", base+"/deployments/{deploymentId}", h.DeleteDeployment)

	requireScope("mlstudio:write", "POST", base+"/findings", h.CreateFinding)
	requireScope("mlstudio:read", "GET", base+"/findings", h.ListFindings)
	requireScope("mlstudio:read", "GET", base+"/findings/{findingId}", h.GetFinding)
	requireScope("mlstudio:write", "PUT", base+"/findings/{findingId}", h.UpdateFinding)
	requireScope("mlstudio:write", "DELETE", base+"/findings/{findingId}", h.DeleteFinding)
}

func (h *Handler) authorizeOrg(w http.ResponseWriter, r *http.Request) (identity.Principal, string, bool) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return identity.Principal{}, "", false
	}
	orgID := r.PathValue("orgID")
	if p.Kind == identity.PrincipalServiceAccount && p.OrgID != orgID {
		httputil.Forbidden(w)
		return identity.Principal{}, "", false
	}
	return p, orgID, true
}

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, mlstudio.ErrParentNotFound) {
		httputil.BadRequest(w, err.Error())
		return
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && (pqErr.Code == "23514" || pqErr.Code == "23503") {
		httputil.BadRequest(w, pqErr.Message)
		return
	}
	httputil.Error(w, r, err)
}
