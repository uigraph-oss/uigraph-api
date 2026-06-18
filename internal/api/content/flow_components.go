package content

import (
	"net/http"

	"github.com/uigraph/app/internal/componentlib"
	"github.com/uigraph/app/internal/storage"
	"github.com/uigraph/app/internal/store"
)

// FlowComponentHandler serves the flow diagram component palette.
type FlowComponentHandler struct {
	store store.Store
}

func NewFlowComponentHandler(s store.Store) *FlowComponentHandler {
	return &FlowComponentHandler{store: s}
}

// List handles GET /api/v1/orgs/{orgID}/flow-diagram-components
func (h *FlowComponentHandler) List(w http.ResponseWriter, r *http.Request) {
	comps, err := h.store.ListComponentsByKind(r.Context(), componentlib.KindFlowDiagram)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list flow diagram components")
		return
	}

	out := make([]componentlib.FlowDiagramComponent, len(comps))
	for i, c := range comps {
		out[i] = componentlib.ToFlowDiagramComponent(c, componentIconURL(r, c))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"components":       out,
		"customComponents": []componentlib.FlowDiagramComponent{},
	})
}

// ComponentIconHandler serves native component icon SVGs from object storage.
type ComponentIconHandler struct {
	storage storage.Client
}

func NewComponentIconHandler(st storage.Client) *ComponentIconHandler {
	return &ComponentIconHandler{storage: st}
}

// Get handles GET /api/v1/component-icons/{slug}
func (h *ComponentIconHandler) Get(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		writeErr(w, http.StatusBadRequest, "slug is required")
		return
	}
	streamObject(w, r, h.storage, storage.ComponentIconKey(slug))
}
