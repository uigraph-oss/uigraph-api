package component

import (
	"net/http"

	"github.com/uigraph/app/internal/componentlib"
	"github.com/uigraph/app/internal/httputil"
)

// @Summary  ListFlow
// @Tags     flow-diagram-components
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/flow-diagram-components [get]
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
