// Package diagram defines the Diagram and DiagramVersion domain types
// and their store interface.
//
// Content storage model:
//   - Diagram content (ReactFlow JSON) is stored in object storage, not Postgres.
//   - The diagrams row holds content_key (storage object key) and content_hash
//     (SHA-256) for change detection.
//   - Each version snapshot is a separate object stored at a version-scoped key.
//   - Reads go through a Redis cache (key: "diagram:content:{id}") with 1-hour TTL.
package diagram

import (
	"context"
	"time"
)

// Diagram is the metadata record stored in Postgres.
// The actual ReactFlow JSON content lives in object storage.
type Diagram struct {
	ID          string  `json:"id"`
	OrgID       string  `json:"orgId"`
	FolderID    *string `json:"folderId,omitempty"`
	TeamID      *string `json:"teamId,omitempty"`
	Name        string  `json:"name"`
	ContentKey        string     `json:"contentKey"`
	ContentHash       string     `json:"contentHash"`
	ContentTokenCount int        `json:"contentTokenCount"`
	PreviewAssetID     *string    `json:"previewAssetId,omitempty"`
	PreviewContentHash *string    `json:"previewContentHash,omitempty"`
	Source             *string    `json:"source,omitempty"`
	CreatedBy          string     `json:"createdBy"`
	UpdatedBy          *string    `json:"updatedBy,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	DeletedAt          *time.Time `json:"deletedAt,omitempty"`
	DeletedBy          *string    `json:"deletedBy,omitempty"`
}

// Version is an immutable snapshot of a diagram's content at a point in time.
type Version struct {
	ID            string    `json:"id"`
	DiagramID     string    `json:"diagramId"`
	VersionNumber int       `json:"versionNumber"`
	Label         *string   `json:"label,omitempty"`
	ContentKey    string    `json:"contentKey"`
	ContentHash   string    `json:"contentHash"`
	IsAutoVersion bool      `json:"isAutoVersion"`
	Source        *string   `json:"source,omitempty"`
	CreatedBy     string    `json:"createdBy"`
	CreatedAt     time.Time `json:"createdAt"`
}

type Image struct {
	ID        string    `json:"diagramImageId"`
	DiagramID string    `json:"diagramId"`
	OrgID     string    `json:"orgId"`
	AssetID   string    `json:"assetId"`
	FileName  *string   `json:"fileName,omitempty"`
	Order     int       `json:"order"`
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
}

type Store interface {
	CreateDiagram(ctx context.Context, d Diagram) error
	GetDiagram(ctx context.Context, id string) (*Diagram, error)
	ListDiagrams(ctx context.Context, orgID string, folderID, teamID *string) ([]Diagram, error)
	UpdateDiagram(ctx context.Context, d Diagram) error
	SoftDeleteDiagram(ctx context.Context, id, deletedBy string) error

	CreateDiagramVersion(ctx context.Context, v Version) error
	GetDiagramVersion(ctx context.Context, id string) (*Version, error)
	ListDiagramVersions(ctx context.Context, diagramID string) ([]Version, error)
	// LatestVersionNumber returns the highest version_number for diagramID, or 0 if none.
	LatestVersionNumber(ctx context.Context, diagramID string) (int, error)

	CreateDiagramImage(ctx context.Context, img Image) error
	ListDiagramImages(ctx context.Context, diagramID string) ([]Image, error)
}
