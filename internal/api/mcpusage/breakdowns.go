package mcpusage

import (
	"net/http"
	"sort"

	"github.com/uigraph/app/internal/httputil"
	mcppkg "github.com/uigraph/app/internal/mcpusage"
)

// Timeseries
// @Summary  Timeseries
// @Tags     mcp
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/mcp/savings/timeseries [get]
func (h *Handler) Timeseries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	modelID := q.Get("model_id")
	_, since := parsePeriod(q.Get("period"))

	rows, err := h.store.GetSavingsTimeseries(r.Context(), r.PathValue("orgID"), since)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if rows == nil {
		rows = []mcppkg.DailySavings{}
	}
	m := h.pricing.PriceFor(modelID)
	for i := range rows {
		rows[i].CostServedUSD = costUSD(rows[i].TotalTokensServed, m.InputCostPerMillion)
		rows[i].CostRawUSD = costUSD(rows[i].TotalTokensRawEquivalent, m.InputCostPerMillion)
		rows[i].CostSavedUSD = costUSD(rows[i].TotalTokensSaved, m.InputCostPerMillion)
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"timeseries": rows})
}

// ByTool
// @Summary  ByTool
// @Tags     mcp
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/mcp/savings/by-tool [get]
func (h *Handler) ByTool(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	modelID := q.Get("model_id")
	_, since := parsePeriod(q.Get("period"))

	rows, err := h.store.GetSavingsByTool(r.Context(), r.PathValue("orgID"), since)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if rows == nil {
		rows = []mcppkg.ToolSavings{}
	}
	m := h.pricing.PriceFor(modelID)
	for i := range rows {
		rows[i].CostSavedUSD = costUSD(rows[i].TokensSaved, m.InputCostPerMillion)
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"byTool": rows})
}

// ByClient
// @Summary  ByClient
// @Tags     mcp
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/mcp/savings/by-client [get]
func (h *Handler) ByClient(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	modelID := q.Get("model_id")
	_, since := parsePeriod(q.Get("period"))

	rows, err := h.store.GetSavingsByClient(r.Context(), r.PathValue("orgID"), since)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if rows == nil {
		rows = []mcppkg.ClientSavings{}
	}
	m := h.pricing.PriceFor(modelID)
	for i := range rows {
		rows[i].CostSavedUSD = costUSD(rows[i].TokensSaved, m.InputCostPerMillion)
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"byClient": rows})
}

// ByModel
// @Summary  ByModel
// @Tags     mcp
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/mcp/savings/by-model [get]
func (h *Handler) ByModel(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	_, since := parsePeriod(q.Get("period"))

	summary, err := h.store.GetSavingsSummary(r.Context(), r.PathValue("orgID"), since)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	rows := []mcppkg.ModelSavings{}
	for _, m := range h.pricing.Models() {
		rows = append(rows, mcppkg.ModelSavings{
			ModelID:      m.ModelID,
			DisplayName:  m.DisplayName,
			Provider:     m.Provider,
			TotalCalls:   summary.TotalCalls,
			TokensSaved:  summary.TotalTokensSaved,
			CostRawUSD:   costUSD(summary.TotalTokensRawEquivalent, m.InputCostPerMillion),
			CostSavedUSD: costUSD(summary.TotalTokensSaved, m.InputCostPerMillion),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CostSavedUSD > rows[j].CostSavedUSD })

	httputil.JSON(w, http.StatusOK, map[string]any{"byModel": rows})
}

// ByUser
// @Summary  ByUser
// @Tags     mcp
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/mcp/savings/by-user [get]
func (h *Handler) ByUser(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	modelID := q.Get("model_id")
	_, since := parsePeriod(q.Get("period"))

	rows, err := h.store.GetSavingsByUser(r.Context(), r.PathValue("orgID"), since)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if rows == nil {
		rows = []mcppkg.UserSavings{}
	}
	m := h.pricing.PriceFor(modelID)
	for i := range rows {
		rows[i].CostSavedUSD = costUSD(rows[i].TokensSaved, m.InputCostPerMillion)
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"byUser": rows})
}
