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
	enterprise bool
	billingURL string
	githubApp  bool
}

func New(cfg *config.Config) *Handler {
	return &Handler{enterprise: cfg.Enterprise, billingURL: cfg.EnterpriseBillingURL, githubApp: cfg.GitHubAppConfigured}
}

type response struct {
	EnterpriseEnabled bool   `json:"enterpriseEnabled"`
	BillingURL        string `json:"billingUrl,omitempty"`
	GitHubEnabled     bool   `json:"githubEnabled"`
}

// Get returns whether this is a managed/enterprise deployment, where its
// billing settings page lives, and which optional integrations this
// deployment was configured with.
// GET /api/v1/instance-info
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	httputil.JSON(w, http.StatusOK, response{
		EnterpriseEnabled: h.enterprise,
		BillingURL:        h.billingURL,
		GitHubEnabled:     h.githubApp,
	})
}
