// Package docs defines the Doc domain type and its store interface.
// Doc bytes live in object storage; Postgres stores metadata + asset key.
package docs

import (
	"context"
	"time"
)

// Doc is an org-level documentation file. The bytes are stored in object
// storage; Postgres stores metadata + the object asset key.
type Doc struct {
	ID            string     `json:"id"`
	OrgID         string     `json:"orgId"`
	FolderID      *string    `json:"folderId,omitempty"`
	TeamID        *string    `json:"teamId,omitempty"`
	FileAssetID   string     `json:"fileAssetId"`
	FileName      string     `json:"fileName"`
	FileType      string     `json:"fileType"`
	Description   string     `json:"description"`
	ContentHash   string     `json:"contentHash"`
	DocTokenCount int        `json:"docTokenCount"`
	CreatedBy     string     `json:"createdBy"`
	UpdatedBy     *string    `json:"updatedBy,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	DeletedAt     *time.Time `json:"deletedAt,omitempty"`
	DeletedBy     *string    `json:"deletedBy,omitempty"`
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
	CreateDoc(ctx context.Context, d Doc) error
	GetDoc(ctx context.Context, id string) (*Doc, error)
	ListDocs(ctx context.Context, orgID string, p ListParams) ([]Doc, int, error)
	UpdateDoc(ctx context.Context, d Doc) error
	SoftDeleteDoc(ctx context.Context, id, deletedBy string) error
}
