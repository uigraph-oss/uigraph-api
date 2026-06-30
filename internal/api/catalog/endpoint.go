package catalog

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	catalogpkg "github.com/uigraph/app/internal/catalog"
	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
)

// ── API Endpoints ─────────────────────────────────────────────────────────────

// ListAPIEndpoints returns the working-copy endpoints, or a specific version's
// snapshot when ?versionId= is supplied.
func (h *Handler) ListAPIEndpoints(w http.ResponseWriter, r *http.Request) {
	apiGroupID := r.PathValue("apiGroupID")
	var (
		endpoints []catalogpkg.APIEndpoint
		err       error
	)
	if versionID := r.URL.Query().Get("versionId"); versionID != "" {
		endpoints, err = h.store.ListAPIEndpointsForVersion(r.Context(), apiGroupID, versionID)
	} else {
		endpoints, err = h.store.ListAPIEndpoints(r.Context(), apiGroupID)
	}
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"endpoints": endpoints})
}

func (h *Handler) CreateAPIEndpoint(w http.ResponseWriter, r *http.Request) {
	apiGroupID := r.PathValue("apiGroupID")
	serviceID := r.PathValue("serviceID")
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	var body struct {
		OperationID string          `json:"operationId"`
		Method      string          `json:"method"`
		Path        string          `json:"path"`
		Summary     string          `json:"summary"`
		Description string          `json:"description"`
		Tags        []string        `json:"tags"`
		Parameters        json.RawMessage `json:"parameters"`
		RequestBody       json.RawMessage `json:"requestBody"`
		Responses         json.RawMessage `json:"responses"`
		ExampleRequests   json.RawMessage `json:"exampleRequests"`
		ExampleResponses  json.RawMessage `json:"exampleResponses"`
		Order             float64         `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Method == "" || body.Path == "" {
		httputil.BadRequest(w, "method and path are required")
		return
	}

	now := time.Now().UTC()
	e := catalogpkg.APIEndpoint{
		ID:          uuid.NewString(),
		APIGroupID:  apiGroupID,
		ServiceID:   serviceID,
		OrgID:       orgID,
		OperationID: body.OperationID,
		Method:      body.Method,
		Path:        body.Path,
		Summary:     body.Summary,
		Description: body.Description,
		Tags:        body.Tags,
		Parameters:       normalizeStoredJSON(body.Parameters),
		RequestBody:      normalizeStoredJSON(body.RequestBody),
		Responses:        normalizeStoredJSON(body.Responses),
		ExampleRequests:  normalizeStoredJSON(body.ExampleRequests),
		ExampleResponses: normalizeStoredJSON(body.ExampleResponses),
		Order:            body.Order,
		CreatedBy:   p.UserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.store.CreateAPIEndpoint(r.Context(), e); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, e)
}

func (h *Handler) GetAPIEndpoint(w http.ResponseWriter, r *http.Request) {
	e, err := h.store.GetAPIEndpoint(r.Context(), r.PathValue("endpointID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if e == nil || e.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, e)
}

func (h *Handler) UpdateAPIEndpoint(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	e, err := h.store.GetAPIEndpoint(r.Context(), r.PathValue("endpointID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if e == nil || e.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	var body struct {
		OperationID *string         `json:"operationId"`
		Method      *string         `json:"method"`
		Path        *string         `json:"path"`
		Summary     *string         `json:"summary"`
		Description *string         `json:"description"`
		Tags        []string        `json:"tags"`
		Parameters       json.RawMessage `json:"parameters"`
		RequestBody      json.RawMessage `json:"requestBody"`
		Responses        json.RawMessage `json:"responses"`
		ExampleRequests  json.RawMessage `json:"exampleRequests"`
		ExampleResponses json.RawMessage `json:"exampleResponses"`
		Order            *float64        `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.OperationID != nil {
		e.OperationID = *body.OperationID
	}
	if body.Method != nil {
		e.Method = *body.Method
	}
	if body.Path != nil {
		e.Path = *body.Path
	}
	if body.Summary != nil {
		e.Summary = *body.Summary
	}
	if body.Description != nil {
		e.Description = *body.Description
	}
	if body.Tags != nil {
		e.Tags = body.Tags
	}
	if body.Parameters != nil {
		e.Parameters = normalizeStoredJSON(body.Parameters)
	}
	if body.RequestBody != nil {
		e.RequestBody = normalizeStoredJSON(body.RequestBody)
	}
	if body.Responses != nil {
		e.Responses = normalizeStoredJSON(body.Responses)
	}
	if body.ExampleRequests != nil {
		e.ExampleRequests = normalizeStoredJSON(body.ExampleRequests)
	}
	if body.ExampleResponses != nil {
		e.ExampleResponses = normalizeStoredJSON(body.ExampleResponses)
	}
	if body.Order != nil {
		e.Order = *body.Order
	}
	e.UpdatedBy = &p.UserID

	if err := h.store.UpdateAPIEndpoint(r.Context(), *e); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, e)
}

func (h *Handler) DeleteAPIEndpoint(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if err := h.store.SoftDeleteAPIEndpoint(r.Context(), r.PathValue("endpointID"), p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
