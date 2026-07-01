// Package actor provides an HTTP handler that resolves actor IDs to public identity info.
package actor

import (
	"net/http"
	"strings"

	actorpkg "github.com/uigraph/app/internal/actor"
	"github.com/uigraph/app/internal/asset"
	"github.com/uigraph/app/internal/cache"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/storage"
	"github.com/uigraph/app/internal/store"
)

const maxActorIDs = 200

// Handler wraps actor.Resolver for HTTP.
type Handler struct {
	resolver *actorpkg.Resolver
}

// New constructs a Handler. s must satisfy actor.Resolver's store requirements.
func New(s store.Store, c cache.Client, st storage.Client) *Handler {
	return &Handler{resolver: actorpkg.New(s, c, asset.New(st, c))}
}

// Register wires the actor route into mux.
func Register(
	mux *http.ServeMux,
	s store.Store,
	c cache.Client,
	st storage.Client,
	protected func(method, pattern string, h http.HandlerFunc),
) {
	h := New(s, c, st)
	protected("GET", "/api/v1/orgs/{orgID}/actors", h.Resolve)
}

// @Summary  Resolve
// @Tags     actors
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/actors [get]
func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("ids")
	if raw == "" {
		httputil.BadRequest(w, "ids query parameter is required")
		return
	}
	var ids []string
	for _, part := range strings.Split(raw, ",") {
		if id := strings.TrimSpace(part); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		httputil.BadRequest(w, "ids query parameter is required")
		return
	}
	if len(ids) > maxActorIDs {
		httputil.BadRequest(w, "too many ids")
		return
	}
	actors, err := h.resolver.ResolveMany(r.Context(), ids)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"actors": actors})
}
