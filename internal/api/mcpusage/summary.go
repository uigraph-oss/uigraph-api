package mcpusage

import (
	"net/http"

	"github.com/uigraph/app/internal/httputil"
)

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
