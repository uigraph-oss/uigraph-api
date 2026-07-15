package chat

import (
	"context"
	"encoding/json"
	"time"
)

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

type ChatMessage struct {
	ID            string          `json:"id"`
	OrgID         string          `json:"orgId"`
	ChatSessionID string          `json:"chatSessionId"`
	Role          string          `json:"role"`
	Content       string          `json:"content"`
	Parts         json.RawMessage `json:"parts,omitempty"`
	CreatedAt     time.Time       `json:"createdAt"`
}

type Store interface {
	CreateChatSession(ctx context.Context, s ChatSession) error
	GetChatSession(ctx context.Context, id string) (*ChatSession, error)
	ListChatSessions(ctx context.Context, orgID, ownerUserID string) ([]ChatSession, error)
	UpdateChatSession(ctx context.Context, s ChatSession) error
	SoftDeleteChatSession(ctx context.Context, id, deletedBy string) error
	CreateChatMessage(ctx context.Context, m ChatMessage) error
	ListChatMessages(ctx context.Context, chatSessionID string) ([]ChatMessage, error)
}
