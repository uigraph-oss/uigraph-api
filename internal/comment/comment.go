// Package comment defines the Comment domain type and its store interface.
// Comments are org-scoped annotations attached to a resource (e.g. a diagram or
// map node) identified by ResourceID, optionally threaded via ParentCommentID.
package comment

import (
	"context"
	"time"
)

// Comment is an org-scoped annotation on a resource.
type Comment struct {
	ID              string     `json:"id"`
	OrgID           string     `json:"orgId"`
	ResourceID      string     `json:"resourceId"`
	ParentCommentID *string    `json:"parentCommentId,omitempty"`
	Text            string     `json:"text"`
	CreatedBy       string     `json:"createdBy"`
	UpdatedBy       *string    `json:"updatedBy,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	DeletedAt       *time.Time `json:"deletedAt,omitempty"`
	DeletedBy       *string    `json:"deletedBy,omitempty"`
}

// Store is the persistence interface for comments.
type Store interface {
	CreateComment(ctx context.Context, c Comment) error
	GetComment(ctx context.Context, id string) (*Comment, error)
	ListComments(ctx context.Context, orgID, resourceID string) ([]Comment, error)
	UpdateComment(ctx context.Context, c Comment) error
	SoftDeleteComment(ctx context.Context, id, deletedBy string) error
}
