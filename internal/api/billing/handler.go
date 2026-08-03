// Package billing exposes cloud-connection and service-cost REST endpoints
// backed by internal/billing.
package billing

import (
	"errors"
	"net/http"

	"github.com/uigraph/app/internal/billing"
	"github.com/uigraph/app/internal/crypto"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/identity"
	authmw "github.com/uigraph/app/internal/middleware"
)

type Handler struct {
	store    billing.Store
	cipher   *crypto.Cipher
	adapters map[billing.Provider]billing.ProviderAdapter
}

// New builds a Handler. adapters may omit providers that aren't wired up
// yet — connections for those providers can still be created, but
// TestConnection/manual sync will fail with a clear error until an adapter
// is registered.
func New(s billing.Store, cipher *crypto.Cipher, adapters map[billing.Provider]billing.ProviderAdapter) *Handler {
	return &Handler{store: s, cipher: cipher, adapters: adapters}
}

func Register(
	mux *http.ServeMux,
	s billing.Store,
	cipher *crypto.Cipher,
	adapters map[billing.Provider]billing.ProviderAdapter,
	requireScope func(scope, method, pattern string, h http.HandlerFunc),
) {
	h := New(s, cipher, adapters)
	const connBase = "/api/v1/orgs/{orgID}/billing/connections"
	const costBase = "/api/v1/orgs/{orgID}/services/{serviceID}/costs"

	requireScope("billing:write", "POST", connBase, h.CreateConnection)
	requireScope("billing:read", "GET", connBase, h.ListConnections)
	requireScope("billing:read", "GET", connBase+"/{connectionID}", h.GetConnection)
	requireScope("billing:write", "DELETE", connBase+"/{connectionID}", h.DeleteConnection)
	requireScope("billing:write", "POST", connBase+"/{connectionID}/test", h.TestConnection)
	requireScope("billing:write", "POST", connBase+"/{connectionID}/sync", h.TriggerSync)

	requireScope("billing:read", "GET", costBase+"/summary", h.GetSummary)
	requireScope("billing:read", "GET", costBase+"/resources", h.ListResources)
	requireScope("billing:read", "GET", costBase+"/trend", h.GetTrend)
	requireScope("billing:read", "GET", costBase+"/tag-rules", h.ListTagRules)
	requireScope("billing:write", "POST", costBase+"/tag-rules", h.CreateTagRule)
	requireScope("billing:write", "DELETE", costBase+"/tag-rules/{ruleID}", h.DeleteTagRule)
}

func (h *Handler) authorizeOrg(w http.ResponseWriter, r *http.Request) (identity.Principal, string, bool) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return identity.Principal{}, "", false
	}
	orgID := r.PathValue("orgID")
	if p.Kind == identity.PrincipalServiceAccount && p.OrgID != orgID {
		httputil.Forbidden(w)
		return identity.Principal{}, "", false
	}
	return p, orgID, true
}

func (h *Handler) adapterFor(provider billing.Provider) (billing.ProviderAdapter, error) {
	a, ok := h.adapters[provider]
	if !ok {
		return nil, errors.New("no provider adapter registered for " + string(provider))
	}
	return a, nil
}

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, billing.ErrConnectionNotFound), errors.Is(err, billing.ErrTagRuleNotFound):
		httputil.JSON(w, http.StatusNotFound, map[string]string{"code": "not_found", "message": err.Error()})
	case errors.Is(err, billing.ErrUnknownProvider), errors.Is(err, billing.ErrInvalidCredential):
		httputil.BadRequest(w, err.Error())
	default:
		httputil.Error(w, r, err)
	}
}
