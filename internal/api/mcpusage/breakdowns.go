package mcpusage

import (
	"net/http"

	"github.com/uigraph/app/internal/httputil"
	mcppkg "github.com/uigraph/app/internal/mcpusage"
)

func (h *Handler) Timeseries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	modelID := q.Get("model_id")
	_, since := parsePeriod(q.Get("period"))

	rows, err := h.store.GetSavingsTimeseries(r.Context(), r.PathValue("orgID"), modelID, since)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if rows == nil {
		rows = []mcppkg.DailySavings{}
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"timeseries": rows})
}

func (h *Handler) ByTool(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	modelID := q.Get("model_id")
	_, since := parsePeriod(q.Get("period"))

	rows, err := h.store.GetSavingsByTool(r.Context(), r.PathValue("orgID"), modelID, since)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if rows == nil {
		rows = []mcppkg.ToolSavings{}
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"byTool": rows})
}
