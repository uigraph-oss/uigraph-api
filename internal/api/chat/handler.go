package chat

import (
	"context"
	"net/http"

	chatpkg "github.com/uigraph/app/internal/chat"
	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
)

type store interface {
	CreateChatSession(ctx context.Context, s chatpkg.ChatSession) error
	GetChatSession(ctx context.Context, id string) (*chatpkg.ChatSession, error)
	ListChatSessions(ctx context.Context, orgID, ownerUserID string) ([]chatpkg.ChatSession, error)
	UpdateChatSession(ctx context.Context, s chatpkg.ChatSession) error
	SoftDeleteChatSession(ctx context.Context, id, deletedBy string) error
	CreateChatMessage(ctx context.Context, m chatpkg.ChatMessage) error
	ListChatMessages(ctx context.Context, chatSessionID string) ([]chatpkg.ChatMessage, error)
}

type Handler struct {
	store store
}

func New(s store) *Handler {
	return &Handler{store: s}
}

func Register(
	mux *http.ServeMux,
	s store,
	requireScope func(scope, method, pattern string, h http.HandlerFunc),
) {
	h := New(s)
	requireScope("chat:read", "GET", "/api/v1/orgs/{orgID}/chat-sessions", h.ListSessions)
	requireScope("chat:write", "POST", "/api/v1/orgs/{orgID}/chat-sessions", h.CreateSession)
	requireScope("chat:read", "GET", "/api/v1/orgs/{orgID}/chat-sessions/{sessionID}", h.GetSession)
	requireScope("chat:write", "PUT", "/api/v1/orgs/{orgID}/chat-sessions/{sessionID}", h.UpdateSession)
	requireScope("chat:write", "DELETE", "/api/v1/orgs/{orgID}/chat-sessions/{sessionID}", h.DeleteSession)
	requireScope("chat:read", "GET", "/api/v1/orgs/{orgID}/chat-sessions/{sessionID}/messages", h.ListMessages)
	requireScope("chat:write", "POST", "/api/v1/orgs/{orgID}/chat-sessions/{sessionID}/messages", h.CreateMessage)
}

func (h *Handler) ownedSession(w http.ResponseWriter, r *http.Request, sessionID string) (*chatpkg.ChatSession, bool) {
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return nil, false
	}
	s, err := h.store.GetChatSession(r.Context(), sessionID)
	if err != nil {
		httputil.Error(w, r, err)
		return nil, false
	}
	if s == nil || s.DeletedAt != nil || s.OrgID != orgID {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return nil, false
	}
	if s.OwnerUserID != p.UserID {
		httputil.Forbidden(w)
		return nil, false
	}
	return s, true
}
