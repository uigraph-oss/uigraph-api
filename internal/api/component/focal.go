package component

import (
	"net/http"

	"github.com/uigraph/app/internal/componentlib"
	"github.com/uigraph/app/internal/httputil"
)

// @Summary  ListFocal
// @Tags     components
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/components [get]
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
