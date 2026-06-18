package content

import (
	"net/http"
	"strings"

	"github.com/uigraph/app/internal/actor"
	"github.com/uigraph/app/internal/asset"
	"github.com/uigraph/app/internal/cache"
	"github.com/uigraph/app/internal/storage"
	"github.com/uigraph/app/internal/store"
)

// maxActorIDs caps how many ids one request may resolve.
const maxActorIDs = 200

type ActorHandler struct {
	resolver *actor.Resolver
}

func NewActorHandler(s store.Store, c cache.Client, st storage.Client) *ActorHandler {
	return &ActorHandler{resolver: actor.New(s, c, asset.New(st, c))}
}

// Resolve handles GET /api/v1/orgs/{orgID}/actors?ids=a,b,c and returns a map
// from each id to its public actor info (null for unresolved ids).
func (h *ActorHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("ids")
	if raw == "" {
		writeErr(w, http.StatusBadRequest, "ids query parameter is required")
		return
	}

	var ids []string
	for _, part := range strings.Split(raw, ",") {
		id := strings.TrimSpace(part)
		if id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		writeErr(w, http.StatusBadRequest, "ids query parameter is required")
		return
	}
	if len(ids) > maxActorIDs {
		writeErr(w, http.StatusBadRequest, "too many ids")
		return
	}

	actors, err := h.resolver.ResolveMany(r.Context(), ids)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"actors": actors})
}
