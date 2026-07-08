package mcpusage

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/xid"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/identity"
	mcppkg "github.com/uigraph/app/internal/mcpusage"
	authmw "github.com/uigraph/app/internal/middleware"
)

// Record
// @Summary  Record
// @Tags     mcp
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/mcp/usage [post]
func (h *Handler) Record(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	var body struct {
		ToolName            string   `json:"toolName"`
		ResourceIDs         []string `json:"resourceIds"`
		TokensServed        int      `json:"tokensServed"`
		TokensRawEquivalent int      `json:"tokensRawEquivalent"`
		TokensSaved         int      `json:"tokensSaved"`
		ResponseSizeBytes   int      `json:"responseSizeBytes"`
		ClientName          string   `json:"clientName"`
		ClientVersion       string   `json:"clientVersion"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.ToolName == "" {
		httputil.BadRequest(w, "toolName is required")
		return
	}

	e := mcppkg.UsageEvent{
		ID:                  xid.New().String(),
		OrgID:               r.PathValue("orgID"),
		ToolName:            body.ToolName,
		ResourceIDs:         body.ResourceIDs,
		TokensServed:        body.TokensServed,
		TokensRawEquivalent: body.TokensRawEquivalent,
		TokensSaved:         body.TokensSaved,
		ResponseSizeBytes:   body.ResponseSizeBytes,
		CreatedAt:           time.Now().UTC(),
	}
	if body.ClientName != "" {
		e.ClientName = &body.ClientName
	}
	if body.ClientVersion != "" {
		e.ClientVersion = &body.ClientVersion
	}
	userID := p.UserID
	if p.Kind == identity.PrincipalServiceAccount {
		e.ServiceAccountID = &userID
	} else {
		e.UserID = &userID
	}

	if err := h.store.CreateUsageEvent(r.Context(), e); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, e)
}

// List
// @Summary  List
// @Tags     mcp
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/mcp/usage [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := mcppkg.Filter{}
	if t := q.Get("tool"); t != "" {
		f.Tool = &t
	}
	if from := q.Get("from"); from != "" {
		if ts, err := time.Parse(time.RFC3339, from); err == nil {
			f.FromTS = &ts
		}
	}
	if to := q.Get("to"); to != "" {
		if ts, err := time.Parse(time.RFC3339, to); err == nil {
			f.ToTS = &ts
		}
	}
	events, err := h.store.ListUsageEvents(r.Context(), r.PathValue("orgID"), f)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if events == nil {
		events = []mcppkg.UsageEvent{}
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"events": events})
}
