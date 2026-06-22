// Package mcpusage defines the domain types and store interface for recording
// raw token usage events emitted by the uigraph-mcp server and for computing
// live cost-savings summaries against the llm_models pricing table.
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
	ModelID             string    `json:"modelId"`
	TokensServed        int       `json:"tokensServed"`
	TokensRawEquivalent int       `json:"tokensRawEquivalent"`
	TokensSaved         int       `json:"tokensSaved"`
	ResponseSizeBytes   int       `json:"responseSizeBytes"`
	CreatedAt           time.Time `json:"createdAt"`
}

// SavingsSummary aggregates usage events for an org/model/period into a
// cost comparison between served (MCP-optimized) and raw-equivalent token usage.
type SavingsSummary struct {
	OrgID             string  `json:"orgId"`
	Period            string  `json:"period"`
	ModelID           string  `json:"modelId"`
	TotalCalls        int     `json:"totalCalls"`
	TotalTokensServed int     `json:"totalTokensServed"`
	TotalTokensSaved  int     `json:"totalTokensSaved"`
	CostServedUSD     float64 `json:"costServedUsd"`
	CostRawUSD        float64 `json:"costRawUsd"`
	CostSavedUSD      float64 `json:"costSavedUsd"`
	UniqueUsersCount  int     `json:"uniqueUsersCount"`
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
	GetSavingsSummary(ctx context.Context, orgID, modelID string, since time.Time) (*SavingsSummary, error)
}
