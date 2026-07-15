package chat

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	chatpkg "github.com/uigraph/app/internal/chat"
	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
)

// ListSessions
// @Summary  ListChatSessions
// @Tags     chat
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/chat-sessions [get]
func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	sessions, err := h.store.ListChatSessions(r.Context(), orgID, p.UserID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

// CreateSession
// @Summary  CreateChatSession
// @Tags     chat
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    body  body  object  false  "request body"
// @Success  201  {object}  chat.ChatSession
// @Failure  400  {object}  httputil.errorBody
// @Failure  401  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/chat-sessions [post]
func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Title == "" {
		body.Title = "New chat"
	}

	now := time.Now().UTC()
	s := chatpkg.ChatSession{
		ID:          uuid.NewString(),
		OrgID:       orgID,
		OwnerUserID: p.UserID,
		Title:       body.Title,
		IsPinned:    false,
		CreatedBy:   p.UserID,
		UpdatedBy:   &p.UserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.store.CreateChatSession(r.Context(), s); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, s)
}

// GetSession returns a session together with its ordered messages.
// @Summary  GetChatSession
// @Tags     chat
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    sessionID  path  string  true  "sessionID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/chat-sessions/{sessionID} [get]
func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	s, ok := h.ownedSession(w, r, sessionID)
	if !ok {
		return
	}
	messages, err := h.store.ListChatMessages(r.Context(), sessionID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"session": s, "messages": messages})
}

// UpdateSession updates a session's title and/or pinned state.
// @Summary  UpdateChatSession
// @Tags     chat
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    sessionID  path  string  true  "sessionID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  chat.ChatSession
// @Failure  400  {object}  httputil.errorBody
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/chat-sessions/{sessionID} [put]
func (h *Handler) UpdateSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	s, ok := h.ownedSession(w, r, sessionID)
	if !ok {
		return
	}
	p, _ := authmw.PrincipalFromCtx(r.Context())

	var body struct {
		Title    *string `json:"title"`
		IsPinned *bool   `json:"isPinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Title != nil {
		s.Title = *body.Title
	}
	if body.IsPinned != nil {
		s.IsPinned = *body.IsPinned
	}
	s.UpdatedBy = &p.UserID

	if err := h.store.UpdateChatSession(r.Context(), *s); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, s)
}

// DeleteSession
// @Summary  DeleteChatSession
// @Tags     chat
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    sessionID  path  string  true  "sessionID"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/chat-sessions/{sessionID} [delete]
func (h *Handler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	_, ok := h.ownedSession(w, r, sessionID)
	if !ok {
		return
	}
	p, _ := authmw.PrincipalFromCtx(r.Context())
	if err := h.store.SoftDeleteChatSession(r.Context(), sessionID, p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListMessages returns a session's ordered messages.
// @Summary  ListChatMessages
// @Tags     chat
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    sessionID  path  string  true  "sessionID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/chat-sessions/{sessionID}/messages [get]
func (h *Handler) ListMessages(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	if _, ok := h.ownedSession(w, r, sessionID); !ok {
		return
	}
	messages, err := h.store.ListChatMessages(r.Context(), sessionID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"messages": messages})
}

// CreateMessage appends a message to a session. Called by the frontend for user
// messages and by uigraph-gateway for assistant messages.
// @Summary  CreateChatMessage
// @Tags     chat
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    sessionID  path  string  true  "sessionID"
// @Param    body  body  object  false  "request body"
// @Success  201  {object}  chat.ChatMessage
// @Failure  400  {object}  httputil.errorBody
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/chat-sessions/{sessionID}/messages [post]
func (h *Handler) CreateMessage(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	s, ok := h.ownedSession(w, r, sessionID)
	if !ok {
		return
	}

	var body struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Role != "user" && body.Role != "assistant" && body.Role != "system" {
		httputil.BadRequest(w, "role must be 'user', 'assistant', or 'system'")
		return
	}
	if body.Content == "" {
		httputil.BadRequest(w, "content is required")
		return
	}

	m := chatpkg.ChatMessage{
		ID:            uuid.NewString(),
		OrgID:         s.OrgID,
		ChatSessionID: sessionID,
		Role:          body.Role,
		Content:       body.Content,
		CreatedAt:     time.Now().UTC(),
	}
	if err := h.store.CreateChatMessage(r.Context(), m); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, m)
}
