package agentsession

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	agentpkg "github.com/uigraph/app/internal/agentsession"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/identity"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
)

// Create
// @Summary  Create agent session
// @Tags     agents
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    body  body  object  true  "request body"
// @Success  201  {object}  map[string]interface{}
// @Failure  400  {object}  httputil.errorBody
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/agent-sessions [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	var body struct {
		Type      string          `json:"type"`
		Title     *string         `json:"title"`
		ModelName *string         `json:"modelName"`
		Metadata  json.RawMessage `json:"metadata"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if !agentpkg.ValidType(body.Type) {
		httputil.BadRequest(w, "unknown agent session type")
		return
	}

	now := time.Now().UTC()
	s := agentpkg.Session{
		ID:        uuid.NewString(),
		OrgID:     r.PathValue("orgID"),
		Type:      body.Type,
		Status:    agentpkg.StatusRunning,
		Title:     body.Title,
		ModelName: body.ModelName,
		Metadata:  body.Metadata,
		StartedAt: now,
		UpdatedAt: now,
	}
	actorID := p.UserID
	if p.Kind == identity.PrincipalServiceAccount {
		s.ServiceAccountID = &actorID
	} else {
		s.UserID = &actorID
	}

	if err := h.store.CreateAgentSession(r.Context(), s); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, s)
}

// Finish
// @Summary  Finalize agent session
// @Tags     agents
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    sessionID  path  string  true  "sessionID"
// @Param    body  body  object  true  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  400  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  409  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/agent-sessions/{sessionID} [put]
func (h *Handler) Finish(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Status      string     `json:"status"`
		Report      *string    `json:"report"`
		Error       *string    `json:"error"`
		CompletedAt *time.Time `json:"completedAt"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if !agentpkg.ValidTerminalStatus(body.Status) {
		httputil.BadRequest(w, "status must be one of completed, failed, cancelled")
		return
	}

	sess, ok := h.loadSession(w, r)
	if !ok {
		return
	}

	completedAt := time.Now().UTC()
	if body.CompletedAt != nil {
		completedAt = body.CompletedAt.UTC()
	}
	if err := h.store.FinishAgentSession(r.Context(), sess.ID, body.Status, body.Report, body.Error, completedAt); err != nil {
		httputil.Error(w, r, err)
		return
	}

	updated, err := h.store.GetAgentSession(r.Context(), sess.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, updated)
}

// List
// @Summary  List agent sessions
// @Tags     agents
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    type  query  string  false  "agent type"
// @Param    status  query  string  false  "session status"
// @Param    period  query  string  false  "1d|7d|30d|1y"
// @Success  200  {object}  map[string]interface{}
// @Failure  400  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/agent-sessions [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	period, since := parsePeriod(q.Get("period"))
	f := agentpkg.SessionFilter{
		Since:  since,
		Limit:  httputil.ListLimit(q.Get("limit")),
		Offset: httputil.ListOffset(q.Get("offset")),
	}
	if t := q.Get("type"); t != "" {
		if !agentpkg.ValidType(t) {
			httputil.BadRequest(w, "unknown agent session type")
			return
		}
		f.Type = &t
	}
	if s := q.Get("status"); s != "" {
		if s != agentpkg.StatusRunning && !agentpkg.ValidTerminalStatus(s) {
			httputil.BadRequest(w, "unknown agent session status")
			return
		}
		f.Status = &s
	}

	sessions, total, err := h.store.ListAgentSessions(r.Context(), r.PathValue("orgID"), f)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if sessions == nil {
		sessions = []agentpkg.Session{}
	}
	httputil.JSON(w, http.StatusOK, map[string]any{
		"sessions": sessions,
		"total":    total,
		"period":   period,
		"limit":    f.Limit,
		"offset":   f.Offset,
	})
}

// Get
// @Summary  Get agent session with steps
// @Tags     agents
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    sessionID  path  string  true  "sessionID"
// @Success  200  {object}  map[string]interface{}
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/agent-sessions/{sessionID} [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.loadSession(w, r)
	if !ok {
		return
	}
	steps, err := h.store.ListAgentSessionSteps(r.Context(), sess.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if steps == nil {
		steps = []agentpkg.Step{}
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"session": sess, "steps": steps})
}

// loadSession fetches the path session and verifies it belongs to the path org.
// A false second return means the response has already been written.
func (h *Handler) loadSession(w http.ResponseWriter, r *http.Request) (*agentpkg.Session, bool) {
	sess, err := h.store.GetAgentSession(r.Context(), r.PathValue("sessionID"))
	if err != nil {
		httputil.Error(w, r, err)
		return nil, false
	}
	if sess == nil || sess.OrgID != r.PathValue("orgID") {
		httputil.Error(w, r, fmt.Errorf("agent session: %w", storepkg.ErrNotFound))
		return nil, false
	}
	return sess, true
}
