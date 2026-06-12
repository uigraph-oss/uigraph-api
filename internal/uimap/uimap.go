// Package uimap defines domain types for Maps, Frames, Focal Points, and Map Canvas.
// Maps are top-level UI journey containers; Frames are individual screens within a map.
package uimap

import (
	"context"
	"time"
)

// Map is a top-level UI journey container (renamed from "project" in enterprise).
type Map struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"orgId"`
	FolderID    *string    `json:"folderId,omitempty"`
	TeamID      *string    `json:"teamId,omitempty"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	CreatedBy   string     `json:"createdBy"`
	UpdatedBy   *string    `json:"updatedBy,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	DeletedAt   *time.Time `json:"deletedAt,omitempty"`
	DeletedBy   *string    `json:"deletedBy,omitempty"`
}

// Frame is a single screen/page within a Map (renamed from "page" in enterprise).
// Screenshot content is stored in object storage; only the key and hash live in Postgres.
type Frame struct {
	ID                    string     `json:"id"`
	MapID                 string     `json:"mapId"`
	OrgID                 string     `json:"orgId"`
	ParentFrameID         *string    `json:"parentFrameId,omitempty"`
	Name                  string     `json:"name"`
	Description           string     `json:"description"`
	TemplateType          string     `json:"templateType"`
	ScreenshotKey         *string    `json:"screenshotKey,omitempty"`
	ScreenshotContentHash *string    `json:"screenshotContentHash,omitempty"`
	Status                string     `json:"status"`
	Order                 float64    `json:"order"`
	Source                *string    `json:"source,omitempty"`
	CreatedBy             string     `json:"createdBy"`
	UpdatedBy             *string    `json:"updatedBy,omitempty"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
	DeletedAt             *time.Time `json:"deletedAt,omitempty"`
	DeletedBy             *string    `json:"deletedBy,omitempty"`
}

// FocalPoint is a named hotspot pinned to a (x,y) location on a Frame.
type FocalPoint struct {
	ID         string     `json:"id"`
	FrameID    string     `json:"frameId"`
	OrgID      string     `json:"orgId"`
	Name       string     `json:"name"`
	LocationX  float64    `json:"locationX"`
	LocationY  float64    `json:"locationY"`
	Visibility string     `json:"visibility"`
	IsActive   bool       `json:"isActive"`
	CreatedBy  string     `json:"createdBy"`
	UpdatedBy  *string    `json:"updatedBy,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	DeletedAt  *time.Time `json:"deletedAt,omitempty"`
	DeletedBy  *string    `json:"deletedBy,omitempty"`
}

// FramePosition stores the (x,y) position of a frame on the map canvas board.
type FramePosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Canvas holds the pan/zoom state and per-frame positions for the map board view.
// One row per map, upserted on every save.
type Canvas struct {
	MapID           string                    `json:"mapId"`
	OrgID           string                    `json:"orgId"`
	Zoom            float64                   `json:"zoom"`
	NavigationX     float64                   `json:"navigationX"`
	NavigationY     float64                   `json:"navigationY"`
	FramePositions  map[string]FramePosition  `json:"framePositions"`
	UpdatedAt       time.Time                 `json:"updatedAt"`
}

// Store is the persistence interface for all map-related entities.
type Store interface {
	// Maps
	CreateMap(ctx context.Context, m Map) error
	GetMap(ctx context.Context, id string) (*Map, error)
	ListMaps(ctx context.Context, orgID string, folderID, teamID *string) ([]Map, error)
	UpdateMap(ctx context.Context, m Map) error
	SoftDeleteMap(ctx context.Context, id, deletedBy string) error

	// Frames
	CreateFrame(ctx context.Context, f Frame) error
	GetFrame(ctx context.Context, id string) (*Frame, error)
	ListFrames(ctx context.Context, mapID string) ([]Frame, error)
	UpdateFrame(ctx context.Context, f Frame) error
	SoftDeleteFrame(ctx context.Context, id, deletedBy string) error

	// Focal points
	CreateFocalPoint(ctx context.Context, fp FocalPoint) error
	GetFocalPoint(ctx context.Context, id string) (*FocalPoint, error)
	ListFocalPoints(ctx context.Context, frameID string) ([]FocalPoint, error)
	UpdateFocalPoint(ctx context.Context, fp FocalPoint) error
	SoftDeleteFocalPoint(ctx context.Context, id, deletedBy string) error

	// Canvas
	GetCanvas(ctx context.Context, mapID string) (*Canvas, error)
	UpsertCanvas(ctx context.Context, c Canvas) error
}
