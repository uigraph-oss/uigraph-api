// Package uimap defines domain types for Maps, Frames, Focal Points, and Map Canvas.
// Maps are top-level UI journey containers; Frames are individual screens within a map.
package uimap

import (
	"context"
	"encoding/json"
	"time"
)

// Map is a top-level UI journey container (renamed from "project" in enterprise).
type Map struct {
	ID                  string     `json:"id"`
	OrgID               string     `json:"orgId"`
	FolderID            *string    `json:"folderId,omitempty"`
	TeamID              *string    `json:"teamId,omitempty"`
	Name                string     `json:"name"`
	Description         string     `json:"description"`
	Status              string     `json:"status"`
	CreatedBy           string     `json:"createdBy"`
	UpdatedBy           *string    `json:"updatedBy,omitempty"`
	CreatedByCommitHash *string    `json:"createdByCommitHash,omitempty"`
	UpdatedByCommitHash *string    `json:"updatedByCommitHash,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
	DeletedAt           *time.Time `json:"deletedAt,omitempty"`
	DeletedBy           *string    `json:"deletedBy,omitempty"`
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
	ScreenshotAssetID     *string    `json:"screenshotAssetId,omitempty"`
	ScreenshotContentHash *string    `json:"screenshotContentHash,omitempty"`
	Status                string     `json:"status"`
	Order                 float64    `json:"order"`
	Source                *string    `json:"source,omitempty"`
	CreatedBy             string     `json:"createdBy"`
	UpdatedBy             *string    `json:"updatedBy,omitempty"`
	CreatedByCommitHash   *string    `json:"createdByCommitHash,omitempty"`
	UpdatedByCommitHash   *string    `json:"updatedByCommitHash,omitempty"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
	DeletedAt             *time.Time `json:"deletedAt,omitempty"`
	DeletedBy             *string    `json:"deletedBy,omitempty"`
	FocalPointCount       int        `json:"focalPointCount"`
}

// FocalPoint is a named hotspot pinned to a (x,y) location on a Frame.
type FocalPoint struct {
	ID                  string     `json:"id"`
	FrameID             string     `json:"frameId"`
	OrgID               string     `json:"orgId"`
	Name                string     `json:"name"`
	LocationX           float64    `json:"locationX"`
	LocationY           float64    `json:"locationY"`
	Visibility          string     `json:"visibility"`
	IsActive            bool       `json:"isActive"`
	CreatedBy           string     `json:"createdBy"`
	UpdatedBy           *string    `json:"updatedBy,omitempty"`
	CreatedByCommitHash *string    `json:"createdByCommitHash,omitempty"`
	UpdatedByCommitHash *string    `json:"updatedByCommitHash,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
	DeletedAt           *time.Time `json:"deletedAt,omitempty"`
	DeletedBy           *string    `json:"deletedBy,omitempty"`
}

type FrameGroup struct {
	ID          string     `json:"id"`
	FrameID     string     `json:"frameId"`
	OrgID       string     `json:"orgId"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	LocationX   float64    `json:"locationX"`
	LocationY   float64    `json:"locationY"`
	Width       float64    `json:"width"`
	Height      float64    `json:"height"`
	Order       float64    `json:"order"`
	IsActive    bool       `json:"isActive"`
	CreatedBy   string     `json:"createdBy"`
	UpdatedBy   *string    `json:"updatedBy,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	DeletedAt   *time.Time `json:"deletedAt,omitempty"`
	DeletedBy   *string    `json:"deletedBy,omitempty"`
}

type FrameLink struct {
	ID            string     `json:"id"`
	FrameID       string     `json:"frameId"`
	OrgID         string     `json:"orgId"`
	Kind          string     `json:"kind"`
	TargetFrameID *string    `json:"targetFrameId,omitempty"`
	TargetMapID   *string    `json:"targetMapId,omitempty"`
	Label         string     `json:"label"`
	LocationX     float64    `json:"locationX"`
	LocationY     float64    `json:"locationY"`
	IsActive      bool       `json:"isActive"`
	CreatedBy     string     `json:"createdBy"`
	UpdatedBy     *string    `json:"updatedBy,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	DeletedAt     *time.Time `json:"deletedAt,omitempty"`
	DeletedBy     *string    `json:"deletedBy,omitempty"`
}

type FocalPointMeta struct {
	ID                         string          `json:"id"`
	FocalPointID               string          `json:"focalPointId"`
	OrgID                      string          `json:"orgId"`
	FrameID                    string          `json:"frameId"`
	ComponentID                string          `json:"componentId"`
	ComponentLinkDiagramID     *string         `json:"componentLinkDiagramId,omitempty"`
	ComponentLinkAPIEndpointID *string         `json:"componentLinkApiEndpointId,omitempty"`
	ComponentLinkTestPackID    *string         `json:"componentLinkTestPackId,omitempty"`
	ComponentLinkServiceDocID  *string         `json:"componentLinkServiceDocId,omitempty"`
	ComponentModalFields       json.RawMessage `json:"componentModalFields"`
	CreatedBy                  string          `json:"createdBy"`
	UpdatedBy                  *string         `json:"updatedBy,omitempty"`
	CreatedByCommitHash        *string         `json:"createdByCommitHash,omitempty"`
	UpdatedByCommitHash        *string         `json:"updatedByCommitHash,omitempty"`
	CreatedAt                  time.Time       `json:"createdAt"`
	UpdatedAt                  time.Time       `json:"updatedAt"`
	DeletedAt                  *time.Time      `json:"deletedAt,omitempty"`
	DeletedBy                  *string         `json:"deletedBy,omitempty"`
}

// ComponentLinkUsage is an aggregated, read-only view of where a component link
// (diagram, API endpoint, test pack, or service doc) is referenced. It joins a
// focal point meta record up to its focal point, frame (screen), and map so a
// single query answers "where is this used?".
type ComponentLinkUsage struct {
	MetaID            string  `json:"metaId"`
	OrgID             string  `json:"orgId"`
	ComponentID       string  `json:"componentId"`
	MapID             string  `json:"mapId"`
	MapName           string  `json:"mapName"`
	FrameID           string  `json:"frameId"`
	FrameName         string  `json:"frameName"`
	ScreenshotAssetID *string `json:"screenshotAssetId,omitempty"`
	FocalPointID      string  `json:"focalPointId"`
	FocalPointName    string  `json:"focalPointName"`
	LocationX         float64 `json:"locationX"`
	LocationY         float64 `json:"locationY"`
}

type FramePosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Canvas struct {
	MapID          string                   `json:"mapId"`
	OrgID          string                   `json:"orgId"`
	Zoom           float64                  `json:"zoom"`
	NavigationX    float64                  `json:"navigationX"`
	NavigationY    float64                  `json:"navigationY"`
	FramePositions map[string]FramePosition `json:"framePositions"`
	UpdatedAt      time.Time                `json:"updatedAt"`
}

type ListParams struct {
	FolderID *string
	TeamID   *string
	Search   *string
	SortBy   string
	SortDir  string
	Limit    int
	Offset   int
}

type Store interface {
	CreateMap(ctx context.Context, m Map) error
	GetMap(ctx context.Context, id string) (*Map, error)
	ListMaps(ctx context.Context, orgID string, p ListParams) ([]Map, int, error)
	UpdateMap(ctx context.Context, m Map) error
	SoftDeleteMap(ctx context.Context, id, deletedBy string) error

	CreateFrame(ctx context.Context, f Frame) error
	GetFrame(ctx context.Context, id string) (*Frame, error)
	ListFrames(ctx context.Context, mapID string, p ListParams) ([]Frame, int, error)
	UpdateFrame(ctx context.Context, f Frame) error
	SoftDeleteFrame(ctx context.Context, id, deletedBy string) error

	CreateFocalPoint(ctx context.Context, fp FocalPoint) error
	GetFocalPoint(ctx context.Context, id string) (*FocalPoint, error)
	ListFocalPoints(ctx context.Context, frameID string) ([]FocalPoint, error)
	UpdateFocalPoint(ctx context.Context, fp FocalPoint) error
	SoftDeleteFocalPoint(ctx context.Context, id, deletedBy string) error

	CreateFrameGroup(ctx context.Context, g FrameGroup) error
	GetFrameGroup(ctx context.Context, id string) (*FrameGroup, error)
	ListFrameGroups(ctx context.Context, frameID string) ([]FrameGroup, error)
	UpdateFrameGroup(ctx context.Context, g FrameGroup) error
	SoftDeleteFrameGroup(ctx context.Context, id, deletedBy string) error

	CreateFrameLink(ctx context.Context, l FrameLink) error
	GetFrameLink(ctx context.Context, id string) (*FrameLink, error)
	ListFrameLinks(ctx context.Context, frameID string) ([]FrameLink, error)
	UpdateFrameLink(ctx context.Context, l FrameLink) error
	SoftDeleteFrameLink(ctx context.Context, id, deletedBy string) error

	CreateFocalPointMeta(ctx context.Context, m FocalPointMeta) error
	GetFocalPointMeta(ctx context.Context, id string) (*FocalPointMeta, error)
	ListFocalPointMeta(ctx context.Context, focalPointID string) ([]FocalPointMeta, error)
	ListFocalPointMetaByLink(ctx context.Context, orgID, linkID string) ([]FocalPointMeta, error)
	ListComponentLinkUsages(ctx context.Context, orgID, linkID string) ([]ComponentLinkUsage, error)
	UpdateFocalPointMeta(ctx context.Context, m FocalPointMeta) error
	SoftDeleteFocalPointMeta(ctx context.Context, id, deletedBy string) error

	GetCanvas(ctx context.Context, mapID string) (*Canvas, error)
	UpsertCanvas(ctx context.Context, c Canvas) error
}
