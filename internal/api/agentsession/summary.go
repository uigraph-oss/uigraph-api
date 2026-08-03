package agentsession

import (
	"net/http"

	agentpkg "github.com/uigraph/app/internal/agentsession"
	"github.com/uigraph/app/internal/httputil"
)

// Summary
// @Summary  Agent session summary
// @Tags     agents
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    period  query  string  false  "1d|7d|30d|1y"
// @Param    type  query  string  false  "agent type"
// @Success  200  {object}  map[string]interface{}
// @Failure  400  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/agent-sessions/summary [get]
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	period, since := parsePeriod(q.Get("period"))

	var sessionType *string
	if t := q.Get("type"); t != "" {
		if !agentpkg.ValidType(t) {
			httputil.BadRequest(w, "unknown agent session type")
			return
		}
		sessionType = &t
	}

	sum, err := h.store.GetAgentSessionSummary(r.Context(), r.PathValue("orgID"), since, sessionType)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"period": period, "summary": sum})
}
