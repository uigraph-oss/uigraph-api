package mcpusage

import (
	"net/http"

	"github.com/uigraph/app/internal/httputil"
)

// Summary
// @Summary  Summary
// @Tags     mcp
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/mcp/savings/summary [get]
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	modelID := q.Get("model_id")
	period, since := parsePeriod(q.Get("period"))

	summary, err := h.store.GetSavingsSummary(r.Context(), r.PathValue("orgID"), modelID, since)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	summary.Period = period
	httputil.JSON(w, http.StatusOK, summary)
}
