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

	custom, err := h.store.ListCustomComponents(r.Context(), r.PathValue("orgID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	customOut := make([]componentlib.FocalPointComponent, len(custom))
	for i, c := range custom {
		customOut[i] = componentlib.ToFocalPointComponent(c, "")
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"components":       out,
		"customComponents": customOut,
	})
}

func iconURL(c componentlib.Component) string {
	return "/api/v1/component-icons/" + componentlib.IconSlug(c)
}
