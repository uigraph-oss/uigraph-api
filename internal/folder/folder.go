// Package folder defines the Folder domain type and its store interface.
// Folders are org-scoped containers that organise services, diagrams, maps, and docs.
package folder

import (
	"context"
	"time"
)

// Type identifies what kind of items a folder holds.
type Type string

const (
	TypeService Type = "service"
	TypeDiagram Type = "diagram"
	TypeMap     Type = "map" // Maps (UI maps / user journeys)
	TypeDoc     Type = "doc"
)

// Folder is a named container within an org, optionally nested via ParentID.
type Folder struct {
	ID        string     `json:"id"`
	OrgID     string     `json:"orgId"`
	ParentID  *string    `json:"parentId,omitempty"`
	Type      Type       `json:"type"`
	Name      string     `json:"name"`
	Order     float64    `json:"order"`
	CreatedBy string     `json:"createdBy"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

// Store is the persistence interface for folders.
type Store interface {
	CreateFolder(ctx context.Context, f Folder) error
	GetFolder(ctx context.Context, id string) (*Folder, error)
	// ListFolders returns non-deleted folders for an org, optionally filtered by type.
	ListFolders(ctx context.Context, orgID string, t *Type) ([]Folder, error)
	UpdateFolder(ctx context.Context, f Folder) error
	DeleteFolder(ctx context.Context, id, deletedBy string) error
}
