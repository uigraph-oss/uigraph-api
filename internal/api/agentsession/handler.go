// Package agentsession exposes HTTP handlers for recording agent runs (a
// session plus its finished steps) and for querying them back for the Insights
// UI. Writes come from the agents themselves via a service-account token.
package agentsession

import (
	"context"
	"net/http"
	"time"

	agentpkg "github.com/uigraph/app/internal/agentsession"
	"github.com/uigraph/app/internal/modelpricing"
)

type store interface {
	CreateAgentSession(ctx context.Context, s agentpkg.Session) error
	GetAgentSession(ctx context.Context, id string) (*agentpkg.Session, error)
	ListAgentSessions(ctx context.Context, orgID string, f agentpkg.SessionFilter) ([]agentpkg.Session, int, error)
	FinishAgentSession(ctx context.Context, id, status string, report, errMsg *string, completedAt time.Time) error
	CreateAgentSessionStep(ctx context.Context, st agentpkg.Step) (int, error)
	ListAgentSessionSteps(ctx context.Context, sessionID string) ([]agentpkg.Step, error)
	GetAgentSessionSummary(ctx context.Context, orgID string, since time.Time, sessionType *string) (*agentpkg.Summary, error)
}

type Handler struct {
	store   store
	pricing *modelpricing.Provider
}

func New(s store, pricing *modelpricing.Provider) *Handler {
	return &Handler{store: s, pricing: pricing}
}

// Register wires the agent session routes onto mux. requireScope is the
// scope-gated registration helper shared by other domain handlers
// (signature: scope, method, pattern, handlerFunc).
func Register(mux *http.ServeMux, s store, pricing *modelpricing.Provider, requireScope func(scope, method, pattern string, h http.HandlerFunc)) {
	h := New(s, pricing)
	requireScope("agents:write", "POST", "/api/v1/orgs/{orgID}/agent-sessions", h.Create)
	requireScope("agents:write", "PUT", "/api/v1/orgs/{orgID}/agent-sessions/{sessionID}", h.Finish)
	requireScope("agents:write", "POST", "/api/v1/orgs/{orgID}/agent-sessions/{sessionID}/steps", h.CreateStep)
	requireScope("agents:read", "GET", "/api/v1/orgs/{orgID}/agent-sessions", h.List)
	requireScope("agents:read", "GET", "/api/v1/orgs/{orgID}/agent-sessions/summary", h.Summary)
	requireScope("agents:read", "GET", "/api/v1/orgs/{orgID}/agent-sessions/{sessionID}", h.Get)
}

// costUSD prices one step's tokens. Input and output are billed at different
// rates; cache tokens are not priced because the price list carries no cache
// rates, which is why the resulting figure is reported as an estimate.
func costUSD(inputTokens, outputTokens int, m modelpricing.Model) float64 {
	return float64(inputTokens)/1_000_000*m.InputCostPerMillion +
		float64(outputTokens)/1_000_000*m.OutputCostPerMillion
}
