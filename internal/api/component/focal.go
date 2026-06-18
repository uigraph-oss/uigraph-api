package component

import (
	"net/http"

	"github.com/uigraph/app/internal/componentlib"
	"github.com/uigraph/app/internal/httputil"
)

// ListFocal handles GET /api/v1/orgs/{orgID}/components
func (h *Handler) ListFocal(w http.ResponseWriter, r *http.Request) {
	comps, err := h.store.ListComponentsByKind(r.Context(), componentlib.KindFocalPoint)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	out := make([]componentlib.FocalPointComponent, len(comps))
	for i, c := range comps {
		out[i] = componentlib.ToFocalPointComponent(c, iconURL(c))
	}
	httputil.JSON(w, http.StatusOK, map[string]any{
		"components":       out,
		"customComponents": []componentlib.FocalPointComponent{},
	})
}

func iconURL(c componentlib.Component) string {
	return "/api/v1/component-icons/" + componentlib.IconSlug(c)
}
