// Package mcpusage defines the domain types and store interface for recording
// raw token usage events emitted by the uigraph-mcp server. The store returns
// token aggregates only; cost estimation is applied in the API layer using
// models.dev pricing.
package mcpusage

import (
	"context"
	"time"
)

// UsageEvent is a single recorded MCP tool call's token usage.
type UsageEvent struct {
	ID                  string    `json:"id"`
	OrgID               string    `json:"orgId"`
	UserID              *string   `json:"userId,omitempty"`
	ServiceAccountID    *string   `json:"serviceAccountId,omitempty"`
	ToolName            string    `json:"toolName"`
	ResourceIDs         []string  `json:"resourceIds"`
	TokensServed        int       `json:"tokensServed"`
	TokensRawEquivalent int       `json:"tokensRawEquivalent"`
	TokensSaved         int       `json:"tokensSaved"`
	ResponseSizeBytes   int       `json:"responseSizeBytes"`
	ClientName          *string   `json:"clientName,omitempty"`
	ClientVersion       *string   `json:"clientVersion,omitempty"`
	DurationMs          int       `json:"durationMs"`
	CreatedAt           time.Time `json:"createdAt"`
}

// SavingsSummary aggregates usage events for an org/model/period into a
// cost comparison between served (MCP-optimized) and raw-equivalent token usage.
type SavingsSummary struct {
	OrgID                    string  `json:"orgId"`
	Period                   string  `json:"period"`
	ModelID                  string  `json:"modelId"`
	TotalCalls               int     `json:"totalCalls"`
	TotalTokensServed        int     `json:"totalTokensServed"`
	TotalTokensRawEquivalent int     `json:"totalTokensRawEquivalent"`
	TotalTokensSaved         int     `json:"totalTokensSaved"`
	CostServedUSD            float64 `json:"costServedUsd"`
	CostRawUSD               float64 `json:"costRawUsd"`
	CostSavedUSD             float64 `json:"costSavedUsd"`
	UniqueUsersCount         int     `json:"uniqueUsersCount"`
	TotalDurationMs          int     `json:"totalDurationMs"`
	EstAgentTimeMs           int     `json:"estAgentTimeMs"`
	TimeSavedMs              int     `json:"timeSavedMs"`
}

// DailySavings is one day's aggregated usage/cost-savings totals.
type DailySavings struct {
	Date                     time.Time `json:"date"`
	TotalCalls               int       `json:"totalCalls"`
	TotalTokensServed        int       `json:"totalTokensServed"`
	TotalTokensRawEquivalent int       `json:"totalTokensRawEquivalent"`
	TotalTokensSaved         int       `json:"totalTokensSaved"`
	CostServedUSD            float64   `json:"costServedUsd"`
	CostRawUSD               float64   `json:"costRawUsd"`
	CostSavedUSD             float64   `json:"costSavedUsd"`
	TotalDurationMs          int       `json:"totalDurationMs"`
	EstAgentTimeMs           int       `json:"estAgentTimeMs"`
	TimeSavedMs              int       `json:"timeSavedMs"`
}

// ToolSavings is one MCP tool's aggregated usage/cost-savings totals.
type ToolSavings struct {
	ToolName            string  `json:"toolName"`
	TotalCalls          int     `json:"totalCalls"`
	TokensSaved         int     `json:"tokensSaved"`
	TokensRawEquivalent int     `json:"tokensRawEquivalent"`
	CostSavedUSD        float64 `json:"costSavedUsd"`
	TotalDurationMs     int     `json:"totalDurationMs"`
	EstAgentTimeMs      int     `json:"estAgentTimeMs"`
	TimeSavedMs         int     `json:"timeSavedMs"`
}

// ClientSavings is one coding tool's (MCP client's) aggregated usage/cost-savings totals.
type ClientSavings struct {
	ClientName      string  `json:"clientName"`
	TotalCalls      int     `json:"totalCalls"`
	TokensSaved     int     `json:"tokensSaved"`
	CostSavedUSD    float64 `json:"costSavedUsd"`
	TotalDurationMs int     `json:"totalDurationMs"`
}

// ModelSavings is one LLM model's aggregated usage/cost-savings totals.
type ModelSavings struct {
	ModelID      string  `json:"modelId"`
	DisplayName  string  `json:"displayName"`
	Provider     string  `json:"provider"`
	TotalCalls   int     `json:"totalCalls"`
	TokensSaved  int     `json:"tokensSaved"`
	CostRawUSD   float64 `json:"costRawUsd"`
	CostSavedUSD float64 `json:"costSavedUsd"`
}

// UserSavings is one user's (or service account's) aggregated usage/cost-savings
// totals. Exactly one of UserID/ServiceAccountID is non-nil per row.
type UserSavings struct {
	UserID           *string `json:"userId,omitempty"`
	ServiceAccountID *string `json:"serviceAccountId,omitempty"`
	TotalCalls       int     `json:"totalCalls"`
	TokensSaved      int     `json:"tokensSaved"`
	CostSavedUSD     float64 `json:"costSavedUsd"`
	TotalDurationMs  int     `json:"totalDurationMs"`
}

// Filter narrows ListUsageEvents results.
type Filter struct {
	Tool   *string
	FromTS *time.Time
	ToTS   *time.Time
}

// Store persists and queries MCP usage events.
type Store interface {
	CreateUsageEvent(ctx context.Context, e UsageEvent) error
	ListUsageEvents(ctx context.Context, orgID string, f Filter) ([]UsageEvent, error)
	GetSavingsSummary(ctx context.Context, orgID string, since time.Time) (*SavingsSummary, error)
	GetSavingsTimeseries(ctx context.Context, orgID string, since time.Time) ([]DailySavings, error)
	GetSavingsByTool(ctx context.Context, orgID string, since time.Time) ([]ToolSavings, error)
	GetSavingsByClient(ctx context.Context, orgID string, since time.Time) ([]ClientSavings, error)
	GetSavingsByUser(ctx context.Context, orgID string, since time.Time) ([]UserSavings, error)
}
