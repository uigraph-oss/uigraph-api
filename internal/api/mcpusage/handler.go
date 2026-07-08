// Package mcpusage exposes HTTP handlers for recording MCP tool usage events
// and querying live cost-savings summaries.
package mcpusage

import (
	"context"
	"net/http"
	"time"

	mcppkg "github.com/uigraph/app/internal/mcpusage"
	"github.com/uigraph/app/internal/modelpricing"
)

type store interface {
	CreateUsageEvent(ctx context.Context, e mcppkg.UsageEvent) error
	ListUsageEvents(ctx context.Context, orgID string, f mcppkg.Filter) ([]mcppkg.UsageEvent, error)
	GetSavingsSummary(ctx context.Context, orgID string, since time.Time) (*mcppkg.SavingsSummary, error)
	GetSavingsTimeseries(ctx context.Context, orgID string, since time.Time) ([]mcppkg.DailySavings, error)
	GetSavingsByTool(ctx context.Context, orgID string, since time.Time) ([]mcppkg.ToolSavings, error)
	GetSavingsByClient(ctx context.Context, orgID string, since time.Time) ([]mcppkg.ClientSavings, error)
	GetSavingsByUser(ctx context.Context, orgID string, since time.Time) ([]mcppkg.UserSavings, error)
}

type Handler struct {
	store   store
	pricing *modelpricing.Provider
}

func New(s store, pricing *modelpricing.Provider) *Handler {
	return &Handler{store: s, pricing: pricing}
}

// Register wires the MCP usage routes onto mux. requireScope is the
// scope-gated registration helper shared by other domain handlers
// (signature: scope, method, pattern, handlerFunc).
func costUSD(tokens int, inputCostPerMillion float64) float64 {
	return float64(tokens) / 1_000_000 * inputCostPerMillion
}

func Register(mux *http.ServeMux, s store, pricing *modelpricing.Provider, requireScope func(scope, method, pattern string, h http.HandlerFunc)) {
	h := New(s, pricing)
	requireScope("services:read", "POST", "/api/v1/orgs/{orgID}/mcp/usage", h.Record)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/mcp/usage", h.List)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/mcp/savings/summary", h.Summary)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/mcp/savings/timeseries", h.Timeseries)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/mcp/savings/by-tool", h.ByTool)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/mcp/savings/by-client", h.ByClient)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/mcp/savings/by-model", h.ByModel)
	requireScope("services:read", "GET", "/api/v1/orgs/{orgID}/mcp/savings/by-user", h.ByUser)
}
