package llm

import (
	"context"
	"time"
)

type LLMModel struct {
	ID                   string    `json:"id"`
	ModelID              string    `json:"modelId"`
	Provider             string    `json:"provider"`
	DisplayName          string    `json:"displayName"`
	InputCostPerMillion  float64   `json:"inputCostPerMillion"`
	OutputCostPerMillion float64   `json:"outputCostPerMillion"`
	IsActive             bool      `json:"isActive"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

type Store interface {
	ListLLMModels(ctx context.Context) ([]LLMModel, error)
	GetLLMModel(ctx context.Context, id string) (*LLMModel, error)
	CreateLLMModel(ctx context.Context, m LLMModel) error
	UpdateLLMModel(ctx context.Context, m LLMModel) error
	DeactivateLLMModel(ctx context.Context, id string) error
}
