// Package chat defines the ChatSession and ChatMessage domain types and their
// store interface. Chat sessions are per-user AI assistant conversations scoped
// to an org; each session owns an ordered list of messages.
package chat

import (
	"context"
	"time"
)

// ChatSession is a single AI assistant conversation owned by one user.
type ChatSession struct {
	ID           string     `json:"id"`
	OrgID        string     `json:"orgId"`
	OwnerUserID  string     `json:"ownerUserId"`
	Title        string     `json:"title"`
	IsPinned     bool       `json:"isPinned"`
	MessageCount int        `json:"messageCount"`
	CreatedBy    string     `json:"createdBy"`
	UpdatedBy    *string    `json:"updatedBy,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	DeletedAt    *time.Time `json:"deletedAt,omitempty"`
	DeletedBy    *string    `json:"deletedBy,omitempty"`
}

// ChatMessage is a single turn in a session. Messages are immutable once written.
type ChatMessage struct {
	ID            string    `json:"id"`
	OrgID         string    `json:"orgId"`
	ChatSessionID string    `json:"chatSessionId"`
	Role          string    `json:"role"`
	Content       string    `json:"content"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Store is the persistence interface for chat sessions and messages.
type Store interface {
	CreateChatSession(ctx context.Context, s ChatSession) error
	GetChatSession(ctx context.Context, id string) (*ChatSession, error)
	ListChatSessions(ctx context.Context, orgID, ownerUserID string) ([]ChatSession, error)
	UpdateChatSession(ctx context.Context, s ChatSession) error
	SoftDeleteChatSession(ctx context.Context, id, deletedBy string) error
	CreateChatMessage(ctx context.Context, m ChatMessage) error
	ListChatMessages(ctx context.Context, chatSessionID string) ([]ChatMessage, error)
}
