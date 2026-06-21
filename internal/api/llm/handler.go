package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/uigraph/app/internal/httputil"
	llmpkg "github.com/uigraph/app/internal/llm"
	storepkg "github.com/uigraph/app/internal/store"
)

type store interface {
	ListLLMModels(ctx context.Context) ([]llmpkg.LLMModel, error)
	GetLLMModel(ctx context.Context, id string) (*llmpkg.LLMModel, error)
	CreateLLMModel(ctx context.Context, m llmpkg.LLMModel) error
	UpdateLLMModel(ctx context.Context, m llmpkg.LLMModel) error
	DeactivateLLMModel(ctx context.Context, id string) error
}

type Handler struct{ store store }

func New(s store) *Handler { return &Handler{store: s} }

func Register(mux *http.ServeMux, s store, authenticated func(method, pattern string, h http.HandlerFunc), serverAdmin func(method, pattern string, h http.HandlerFunc)) {
	h := New(s)
	authenticated("GET", "/api/v1/llm/models", h.List)
	serverAdmin("POST", "/api/v1/llm/models", h.Create)
	serverAdmin("PUT", "/api/v1/llm/models/{modelID}", h.Update)
	serverAdmin("DELETE", "/api/v1/llm/models/{modelID}", h.Deactivate)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	models, err := h.store.ListLLMModels(r.Context())
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if models == nil {
		models = []llmpkg.LLMModel{}
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"models": models})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ModelID              string  `json:"modelId"`
		Provider             string  `json:"provider"`
		DisplayName          string  `json:"displayName"`
		InputCostPerMillion  float64 `json:"inputCostPerMillion"`
		OutputCostPerMillion float64 `json:"outputCostPerMillion"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.ModelID == "" || body.Provider == "" || body.DisplayName == "" {
		httputil.BadRequest(w, "modelId, provider, and displayName are required")
		return
	}
	now := time.Now().UTC()
	m := llmpkg.LLMModel{
		ID:                   uuid.NewString(),
		ModelID:              body.ModelID,
		Provider:             body.Provider,
		DisplayName:          body.DisplayName,
		InputCostPerMillion:  body.InputCostPerMillion,
		OutputCostPerMillion: body.OutputCostPerMillion,
		IsActive:             true,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := h.store.CreateLLMModel(r.Context(), m); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, m)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	existing, err := h.store.GetLLMModel(r.Context(), r.PathValue("modelID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	var body struct {
		DisplayName          *string  `json:"displayName"`
		InputCostPerMillion  *float64 `json:"inputCostPerMillion"`
		OutputCostPerMillion *float64 `json:"outputCostPerMillion"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.DisplayName != nil {
		existing.DisplayName = *body.DisplayName
	}
	if body.InputCostPerMillion != nil {
		existing.InputCostPerMillion = *body.InputCostPerMillion
	}
	if body.OutputCostPerMillion != nil {
		existing.OutputCostPerMillion = *body.OutputCostPerMillion
	}
	if err := h.store.UpdateLLMModel(r.Context(), *existing); err != nil {
		httputil.Error(w, r, err)
		return
	}
	existing.UpdatedAt = time.Now().UTC()
	httputil.JSON(w, http.StatusOK, existing)
}

func (h *Handler) Deactivate(w http.ResponseWriter, r *http.Request) {
	existing, err := h.store.GetLLMModel(r.Context(), r.PathValue("modelID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	if err := h.store.DeactivateLLMModel(r.Context(), r.PathValue("modelID")); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
