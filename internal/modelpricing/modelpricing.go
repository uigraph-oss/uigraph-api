package modelpricing

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const apiURL = "https://models.dev/api.json"

const refreshInterval = 24 * time.Hour

type Model struct {
	ModelID              string  `json:"modelId"`
	Provider             string  `json:"provider"`
	DisplayName          string  `json:"displayName"`
	InputCostPerMillion  float64 `json:"inputCostPerMillion"`
	OutputCostPerMillion float64 `json:"outputCostPerMillion"`
}

var catalog = []Model{
	{ModelID: "gpt-5.5", Provider: "openai", DisplayName: "GPT-5.5", InputCostPerMillion: 5, OutputCostPerMillion: 30},
	{ModelID: "gpt-5", Provider: "openai", DisplayName: "GPT-5", InputCostPerMillion: 1.25, OutputCostPerMillion: 10},
	{ModelID: "claude-opus-4-8", Provider: "anthropic", DisplayName: "Claude Opus 4.8", InputCostPerMillion: 5, OutputCostPerMillion: 25},
	{ModelID: "claude-sonnet-4-6", Provider: "anthropic", DisplayName: "Claude Sonnet 4.6", InputCostPerMillion: 3, OutputCostPerMillion: 15},
	{ModelID: "claude-sonnet-5", Provider: "anthropic", DisplayName: "Claude Sonnet 5", InputCostPerMillion: 2, OutputCostPerMillion: 10},
	{ModelID: "claude-haiku-4-5", Provider: "anthropic", DisplayName: "Claude Haiku 4.5", InputCostPerMillion: 1, OutputCostPerMillion: 5},
	{ModelID: "gemini-2.5-pro", Provider: "google", DisplayName: "Gemini 2.5 Pro", InputCostPerMillion: 1.25, OutputCostPerMillion: 10},
	{ModelID: "gemini-2.5-flash", Provider: "google", DisplayName: "Gemini 2.5 Flash", InputCostPerMillion: 0.3, OutputCostPerMillion: 2.5},
	{ModelID: "glm-5.2", Provider: "zhipuai", DisplayName: "GLM-5.2", InputCostPerMillion: 1.4, OutputCostPerMillion: 4.4},
	{ModelID: "grok-4.3", Provider: "xai", DisplayName: "Grok 4.3", InputCostPerMillion: 1.25, OutputCostPerMillion: 2.5},
}

type Provider struct {
	mu     sync.RWMutex
	models []Model
	client *http.Client
}

func New() *Provider {
	p := &Provider{client: &http.Client{Timeout: 30 * time.Second}}
	p.models = append([]Model(nil), catalog...)
	_ = p.refresh(context.Background())
	go p.loop()
	return p
}

func (p *Provider) loop() {
	t := time.NewTicker(refreshInterval)
	defer t.Stop()
	for range t.C {
		_ = p.refresh(context.Background())
	}
}

func (p *Provider) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var raw map[string]struct {
		Models map[string]struct {
			Name string `json:"name"`
			Cost struct {
				Input  float64 `json:"input"`
				Output float64 `json:"output"`
			} `json:"cost"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return err
	}

	updated := make([]Model, len(catalog))
	for i, m := range catalog {
		updated[i] = m
		prov, ok := raw[m.Provider]
		if !ok {
			continue
		}
		md, ok := prov.Models[m.ModelID]
		if !ok {
			continue
		}
		updated[i].InputCostPerMillion = md.Cost.Input
		updated[i].OutputCostPerMillion = md.Cost.Output
		if md.Name != "" {
			updated[i].DisplayName = md.Name
		}
	}

	p.mu.Lock()
	p.models = updated
	p.mu.Unlock()
	return nil
}

func (p *Provider) Models() []Model {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]Model, len(p.models))
	copy(out, p.models)
	return out
}

func (p *Provider) PriceFor(modelID string) Model {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, m := range p.models {
		if m.ModelID == modelID {
			return m
		}
	}
	return p.models[0]
}
