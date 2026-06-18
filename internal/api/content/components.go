package content

import (
	"net/http"

	"github.com/uigraph/app/internal/componentlib"
	"github.com/uigraph/app/internal/store"
)

// ComponentHandler serves the focal-point component palette.
type ComponentHandler struct {
	store store.Store
}

func NewComponentHandler(s store.Store) *ComponentHandler {
	return &ComponentHandler{store: s}
}

// List handles GET /api/v1/orgs/{orgID}/components
func (h *ComponentHandler) List(w http.ResponseWriter, r *http.Request) {
	comps, err := h.store.ListComponentsByKind(r.Context(), componentlib.KindFocalPoint)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list components")
		return
	}

	out := make([]componentlib.FocalPointComponent, len(comps))
	for i, c := range comps {
		out[i] = componentlib.ToFocalPointComponent(c, componentIconURL(r, c))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"components":       out,
		"customComponents": []componentlib.FocalPointComponent{},
	})
}

func componentIconURL(r *http.Request, c componentlib.Component) string {
	slug := componentlib.IconSlug(c)
	return "/api/v1/component-icons/" + slug
}
