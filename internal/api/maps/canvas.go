package maps

import (
	"net/http"

	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/uimap"
)

// GetCanvas handles GET /api/v1/orgs/{orgID}/maps/{mapID}/canvas
func (h *Handler) GetCanvas(w http.ResponseWriter, r *http.Request) {
	mapID := r.PathValue("mapID")
	c, err := h.store.GetCanvas(r.Context(), mapID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if c == nil {
		c = &uimap.Canvas{
			MapID:          mapID,
			Zoom:           1.0,
			FramePositions: map[string]uimap.FramePosition{},
		}
	}
	httputil.JSON(w, http.StatusOK, c)
}

// UpsertCanvas handles PUT /api/v1/orgs/{orgID}/maps/{mapID}/canvas
func (h *Handler) UpsertCanvas(w http.ResponseWriter, r *http.Request) {
	mapID := r.PathValue("mapID")
	orgID := r.PathValue("orgID")

	var body struct {
		Zoom           *float64                       `json:"zoom"`
		NavigationX    *float64                       `json:"navigationX"`
		NavigationY    *float64                       `json:"navigationY"`
		FramePositions map[string]uimap.FramePosition `json:"framePositions"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}

	c, err := h.store.GetCanvas(r.Context(), mapID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if c == nil {
		c = &uimap.Canvas{
			MapID:          mapID,
			OrgID:          orgID,
			Zoom:           1.0,
			FramePositions: map[string]uimap.FramePosition{},
		}
	}
	if body.Zoom != nil {
		c.Zoom = *body.Zoom
	}
	if body.NavigationX != nil {
		c.NavigationX = *body.NavigationX
	}
	if body.NavigationY != nil {
		c.NavigationY = *body.NavigationY
	}
	if body.FramePositions != nil {
		c.FramePositions = body.FramePositions
	}

	if err := h.store.UpsertCanvas(r.Context(), *c); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, c)
}
