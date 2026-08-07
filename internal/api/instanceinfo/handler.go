// Package instanceinfo exposes a tiny public, unauthenticated endpoint so
// frontends can ask "is this the managed deployment" without needing a
// session — e.g. to decide whether to show a "Billing" link at all. Self-
// hosted deployments never set these, so the fields are simply absent.
package instanceinfo

import (
	"net/http"

	"github.com/uigraph/app/internal/config"
	"github.com/uigraph/app/internal/httputil"
)

type Handler struct {
	billingURL string
}

func New(cfg *config.Config) *Handler {
	return &Handler{billingURL: cfg.EnterpriseBillingURL}
}

type response struct {
	EnterpriseEnabled bool   `json:"enterpriseEnabled"`
	BillingURL        string `json:"billingUrl,omitempty"`
}

// Get returns whether this is a managed/enterprise deployment and, if so,
// where its billing settings page lives.
// GET /api/v1/instance-info
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	httputil.JSON(w, http.StatusOK, response{
		EnterpriseEnabled: h.billingURL != "",
		BillingURL:        h.billingURL,
	})
}
