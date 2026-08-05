// Package agentsession defines the domain types and store interface for agent
// runs: one session per run plus its ordered, already-finished steps. Everything
// a run accumulates (tokens, duration, step count) lives on the steps and is
// aggregated on read; only per-step cost is priced on write, because the price
// list changes over time.
package agentsession

import (
	"context"
	"encoding/json"
	"time"
)

const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

const (
	KindLLM  = "llm"
	KindTool = "tool"
)

const (
	TypeArtifacts = "artifacts"
	TypeImpact    = "impact"
)

// Types is the catalog of agent types that may open a session. A new agent type
// adds a value here; anything else is rejected at the API boundary.
var Types = []string{TypeArtifacts, TypeImpact}

func ValidType(t string) bool {
	for _, known := range Types {
		if known == t {
			return true
		}
	}
	return false
}

// ValidTerminalStatus reports whether status is one a run may be finalized with.
// StatusRunning is deliberately excluded — it is only ever set on create.
func ValidTerminalStatus(status string) bool {
	return status == StatusCompleted || status == StatusFailed || status == StatusCancelled
}

func ValidKind(kind string) bool {
	return kind == KindLLM || kind == KindTool
}

// Session is one agent run.
type Session struct {
	ID               string          `json:"id"`
	OrgID            string          `json:"orgId"`
	Type             string          `json:"type"`
	Status           string          `json:"status"`
	UserID           *string         `json:"userId,omitempty"`
	ServiceAccountID *string         `json:"serviceAccountId,omitempty"`
	Title            *string         `json:"title,omitempty"`
	ModelName        *string         `json:"modelName,omitempty"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
	Report           *string         `json:"report,omitempty"`
	Error            *string         `json:"error,omitempty"`
	StartedAt        time.Time       `json:"startedAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
	CompletedAt      *time.Time      `json:"completedAt,omitempty"`
	Totals           SessionTotals   `json:"totals"`
}

// SessionTotals are aggregated from the session's steps at read time. CostUSD is
// nil when no step could be priced, which is a different statement from 0.
type SessionTotals struct {
	StepCount          int      `json:"stepCount"`
	InputTokens        int      `json:"inputTokens"`
	OutputTokens       int      `json:"outputTokens"`
	ReasoningTokens    int      `json:"reasoningTokens"`
	CachedInputTokens  int      `json:"cachedInputTokens"`
	CachedOutputTokens int      `json:"cachedOutputTokens"`
	CostUSD            *float64 `json:"costUsd,omitempty"`
	UnpricedSteps      int      `json:"unpricedSteps"`
	StepDurationMs     int      `json:"stepDurationMs"`
}

// Step is one finished LLM turn or tool call. A step row is only ever written
// once the step is done, so it has no status: failure is a non-nil Error.
type Step struct {
	ID                 string          `json:"id"`
	SessionID          string          `json:"sessionId"`
	Seq                int             `json:"seq"`
	Kind               string          `json:"kind"`
	Name               *string         `json:"name,omitempty"`
	ModelName          *string         `json:"modelName,omitempty"`
	Input              json.RawMessage `json:"input,omitempty"`
	Output             json.RawMessage `json:"output,omitempty"`
	Text               *string         `json:"text,omitempty"`
	FinishReason       *string         `json:"finishReason,omitempty"`
	Error              *string         `json:"error,omitempty"`
	InputTokens        *int            `json:"inputTokens,omitempty"`
	OutputTokens       *int            `json:"outputTokens,omitempty"`
	ReasoningTokens    *int            `json:"reasoningTokens,omitempty"`
	CachedInputTokens  *int            `json:"cachedInputTokens,omitempty"`
	CachedOutputTokens *int            `json:"cachedOutputTokens,omitempty"`
	CostUSD            *float64        `json:"costUsd,omitempty"`
	StartedAt          time.Time       `json:"startedAt"`
	CompletedAt        time.Time       `json:"completedAt"`
}

// SessionFilter narrows ListAgentSessions results.
type SessionFilter struct {
	Type   *string
	Status *string
	Since  time.Time
	Limit  int
	Offset int
}

// Summary aggregates sessions and their steps over a period.
type Summary struct {
	TotalSessions     int           `json:"totalSessions"`
	CompletedSessions int           `json:"completedSessions"`
	FailedSessions    int           `json:"failedSessions"`
	RunningSessions   int           `json:"runningSessions"`
	TotalDurationMs   int           `json:"totalDurationMs"`
	Totals            SessionTotals `json:"totals"`
	ByType            []TypeSummary `json:"byType"`
}

// TypeSummary is one agent type's slice of a Summary.
type TypeSummary struct {
	Type              string        `json:"type"`
	TotalSessions     int           `json:"totalSessions"`
	CompletedSessions int           `json:"completedSessions"`
	FailedSessions    int           `json:"failedSessions"`
	RunningSessions   int           `json:"runningSessions"`
	TotalDurationMs   int           `json:"totalDurationMs"`
	Totals            SessionTotals `json:"totals"`
}

// Store persists and queries agent sessions and their steps.
type Store interface {
	CreateAgentSession(ctx context.Context, s Session) error
	GetAgentSession(ctx context.Context, id string) (*Session, error)
	ListAgentSessions(ctx context.Context, orgID string, f SessionFilter) ([]Session, int, error)
	FinishAgentSession(ctx context.Context, id, status string, report, errMsg *string, completedAt time.Time) error
	CreateAgentSessionStep(ctx context.Context, st Step) (int, error)
	ListAgentSessionSteps(ctx context.Context, sessionID string) ([]Step, error)
	GetAgentSessionSummary(ctx context.Context, orgID string, since time.Time, sessionType *string) (*Summary, error)
}
