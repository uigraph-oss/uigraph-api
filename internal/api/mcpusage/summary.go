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

	summary, err := h.store.GetSavingsSummary(r.Context(), r.PathValue("orgID"), since)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	tools, err := h.store.GetSavingsByTool(r.Context(), r.PathValue("orgID"), since)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	m := h.pricing.PriceFor(modelID)
	summary.Period = period
	summary.ModelID = m.ModelID
	summary.CostServedUSD = costUSD(summary.TotalTokensServed, m.InputCostPerMillion)
	summary.CostRawUSD = costUSD(summary.TotalTokensRawEquivalent, m.InputCostPerMillion)
	summary.CostSavedUSD = costUSD(summary.TotalTokensSaved, m.InputCostPerMillion)
	estAgent := 0
	for _, t := range tools {
		estAgent += estAgentTimeMs(t.ToolName, t.TokensRawEquivalent)
	}
	summary.EstAgentTimeMs = estAgent
	summary.TimeSavedMs = timeSavedMs(estAgent, summary.TotalDurationMs)
	httputil.JSON(w, http.StatusOK, summary)
}
