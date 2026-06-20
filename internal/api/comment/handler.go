package comment

import (
	"context"
	"net/http"

	"github.com/uigraph/app/internal/comment"
)

type store interface {
	CreateComment(ctx context.Context, c comment.Comment) error
	GetComment(ctx context.Context, id string) (*comment.Comment, error)
	ListComments(ctx context.Context, orgID, resourceID string) ([]comment.Comment, error)
	UpdateComment(ctx context.Context, c comment.Comment) error
	SoftDeleteComment(ctx context.Context, id, deletedBy string) error
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
	requireScope("maps:read", "GET", "/api/v1/orgs/{orgID}/comments", h.List)
	requireScope("maps:write", "POST", "/api/v1/orgs/{orgID}/comments", h.Create)
	requireScope("maps:write", "PUT", "/api/v1/orgs/{orgID}/comments/{commentID}", h.Update)
	requireScope("maps:write", "DELETE", "/api/v1/orgs/{orgID}/comments/{commentID}", h.Delete)
}
