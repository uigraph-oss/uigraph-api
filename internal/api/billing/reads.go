package billing

import (
	"net/http"
	"strconv"

	"github.com/uigraph/app/internal/billing"
	"github.com/uigraph/app/internal/httputil"
)

const (
	defaultTrendDays = 90
	maxTrendDays     = 366
)

func parseTrendDays(raw string) int {
	if raw == "" {
		return defaultTrendDays
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultTrendDays
	}
	if n > maxTrendDays {
		return maxTrendDays
	}
	return n
}

func (h *Handler) ListConnections(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	conns, err := h.store.ListCloudConnections(r.Context(), orgID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"connections": conns})
}

func (h *Handler) GetConnection(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	conn, err := h.store.GetCloudConnection(r.Context(), orgID, r.PathValue("connectionID"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, conn)
}

func (h *Handler) ListTagRules(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	rules, err := h.store.ListTagRules(r.Context(), orgID, r.PathValue("serviceID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"tagRules": rules})
}

// resourcesAndRules loads a service's matched resources plus the rules that
// were used to match them — shared by GetSummary and ListResources so both
// endpoints see a consistent snapshot.
func (h *Handler) resourcesAndRules(r *http.Request, orgID, serviceID string) ([]billing.Resource, []billing.TagRule, error) {
	rules, err := h.store.ListTagRules(r.Context(), orgID, serviceID)
	if err != nil {
		return nil, nil, err
	}
	resources, err := h.store.ListResourcesForService(r.Context(), orgID, serviceID)
	if err != nil {
		return nil, nil, err
	}
	for i := range resources {
		resources[i].MatchedTags = billing.MatchTags(resources[i].Tags, rules)
	}
	return resources, rules, nil
}

func (h *Handler) GetSummary(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	serviceID := r.PathValue("serviceID")
	resources, _, err := h.resourcesAndRules(r, orgID, serviceID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	trend, err := h.store.ListTrendForService(r.Context(), orgID, serviceID, 60)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, billing.ComputeSummary(resources, trend))
}

func (h *Handler) ListResources(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	resources, _, err := h.resourcesAndRules(r, orgID, r.PathValue("serviceID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"resources": resources})
}

func (h *Handler) GetTrend(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	days := parseTrendDays(r.URL.Query().Get("days"))
	trend, err := h.store.ListTrendForService(r.Context(), orgID, r.PathValue("serviceID"), days)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"trend": trend})
}
