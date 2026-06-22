package mcpusage

import (
	"net/http"
	"time"

	"github.com/uigraph/app/internal/httputil"
)

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	modelID := q.Get("model_id")
	if modelID == "" {
		modelID = "claude-sonnet-4-6"
	}

	period := q.Get("period")
	var since time.Time
	switch period {
	case "1d":
		since = time.Now().UTC().Add(-24 * time.Hour)
	case "30d":
		since = time.Now().UTC().Add(-30 * 24 * time.Hour)
	case "1y":
		since = time.Now().UTC().Add(-365 * 24 * time.Hour)
	default:
		period = "7d"
		since = time.Now().UTC().Add(-7 * 24 * time.Hour)
	}

	summary, err := h.store.GetSavingsSummary(r.Context(), r.PathValue("orgID"), modelID, since)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	summary.Period = period
	httputil.JSON(w, http.StatusOK, summary)
}
