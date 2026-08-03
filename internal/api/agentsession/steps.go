package agentsession

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	agentpkg "github.com/uigraph/app/internal/agentsession"
	"github.com/uigraph/app/internal/httputil"
)

// CreateStep
// @Summary  Append a finished agent session step
// @Tags     agents
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    sessionID  path  string  true  "sessionID"
// @Param    body  body  object  true  "request body"
// @Success  201  {object}  map[string]interface{}
// @Failure  400  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/agent-sessions/{sessionID}/steps [post]
func (h *Handler) CreateStep(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind               string          `json:"kind"`
		Name               *string         `json:"name"`
		ModelName          *string         `json:"modelName"`
		Input              json.RawMessage `json:"input"`
		Output             json.RawMessage `json:"output"`
		Text               *string         `json:"text"`
		FinishReason       *string         `json:"finishReason"`
		Error              *string         `json:"error"`
		InputTokens        *int            `json:"inputTokens"`
		OutputTokens       *int            `json:"outputTokens"`
		ReasoningTokens    *int            `json:"reasoningTokens"`
		CachedInputTokens  *int            `json:"cachedInputTokens"`
		CachedOutputTokens *int            `json:"cachedOutputTokens"`
		StartedAt          *time.Time      `json:"startedAt"`
		CompletedAt        *time.Time      `json:"completedAt"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if !agentpkg.ValidKind(body.Kind) {
		httputil.BadRequest(w, "kind must be one of llm, tool")
		return
	}
	if body.Kind == agentpkg.KindTool && body.Name == nil {
		httputil.BadRequest(w, "name is required on tool steps")
		return
	}
	if body.StartedAt == nil || body.CompletedAt == nil {
		httputil.BadRequest(w, "startedAt and completedAt are required")
		return
	}

	sess, ok := h.loadSession(w, r)
	if !ok {
		return
	}

	st := agentpkg.Step{
		ID:                 uuid.NewString(),
		SessionID:          sess.ID,
		Kind:               body.Kind,
		Name:               body.Name,
		ModelName:          body.ModelName,
		Input:              body.Input,
		Output:             body.Output,
		Text:               body.Text,
		FinishReason:       body.FinishReason,
		Error:              body.Error,
		InputTokens:        body.InputTokens,
		OutputTokens:       body.OutputTokens,
		ReasoningTokens:    body.ReasoningTokens,
		CachedInputTokens:  body.CachedInputTokens,
		CachedOutputTokens: body.CachedOutputTokens,
		StartedAt:          body.StartedAt.UTC(),
		CompletedAt:        body.CompletedAt.UTC(),
	}
	st.CostUSD = h.priceStep(st, sess.ModelName)

	seq, err := h.store.CreateAgentSessionStep(r.Context(), st)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	st.Seq = seq
	httputil.JSON(w, http.StatusCreated, st)
}

// priceStep prices a step at insert time, because the price list changes and a
// run's cost must reflect what it cost then. A nil result means "unknown", which
// is a different statement from zero: a tool step has no model, an LLM step on
// an unrecognized model burned tokens nobody can put a number on.
func (h *Handler) priceStep(st agentpkg.Step, sessionModel *string) *float64 {
	if st.Kind == agentpkg.KindTool {
		return nil
	}
	if st.InputTokens == nil && st.OutputTokens == nil {
		return nil
	}

	var name string
	if st.ModelName != nil {
		name = *st.ModelName
	} else if sessionModel != nil {
		name = *sessionModel
	} else {
		return nil
	}

	price, known := h.pricing.Lookup(name)
	if !known {
		return nil
	}

	var input, output int
	if st.InputTokens != nil {
		input = *st.InputTokens
	}
	if st.OutputTokens != nil {
		output = *st.OutputTokens
	}
	cost := costUSD(input, output, price)
	return &cost
}
