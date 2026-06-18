package component

import (
	"net/http"

	"github.com/uigraph/app/internal/componentlib"
	"github.com/uigraph/app/internal/httputil"
)

// ListFlow handles GET /api/v1/orgs/{orgID}/flow-diagram-components
func (h *Handler) ListFlow(w http.ResponseWriter, r *http.Request) {
	comps, err := h.store.ListComponentsByKind(r.Context(), componentlib.KindFlowDiagram)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	out := make([]componentlib.FlowDiagramComponent, len(comps))
	for i, c := range comps {
		out[i] = componentlib.ToFlowDiagramComponent(c, iconURL(c))
	}
	httputil.JSON(w, http.StatusOK, map[string]any{
		"components":       out,
		"customComponents": []componentlib.FlowDiagramComponent{},
	})
}
