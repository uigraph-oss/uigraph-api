package identity

import (
	"context"
	"time"
)

const figmaProvider = "figma"

const FigmaProvider = figmaProvider

type FigmaAuth struct {
	UserID       string
	FigmaUserID  string
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

type FigmaAuthStore interface {
	GetFigmaAuth(ctx context.Context, userID string) (*FigmaAuth, error)
	UpsertFigmaAuth(ctx context.Context, a FigmaAuth) error
	DeleteFigmaAuth(ctx context.Context, userID string) error
}
