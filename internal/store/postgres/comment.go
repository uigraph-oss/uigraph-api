package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uigraph/app/internal/comment"
)

func (d *DB) CreateComment(ctx context.Context, c comment.Comment) error {
	const q = `
		INSERT INTO comments
			(id, org_id, resource_id, parent_comment_id, text,
			 created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	now := time.Now().UTC()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = now
	}
	_, err := d.db.ExecContext(ctx, q,
		c.ID, c.OrgID, c.ResourceID, c.ParentCommentID, c.Text,
		c.CreatedBy, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateComment: %w", err)
	}
	return nil
}

func (d *DB) GetComment(ctx context.Context, id string) (*comment.Comment, error) {
	const q = `
		SELECT id, org_id, resource_id, parent_comment_id, text,
		       created_by, updated_by, created_at, updated_at, deleted_at, deleted_by
		FROM comments WHERE id = $1`
	c, err := scanComment(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetComment: %w", err)
	}
	return &c, nil
}

func (d *DB) ListComments(ctx context.Context, orgID, resourceID string) ([]comment.Comment, error) {
	const q = `
		SELECT id, org_id, resource_id, parent_comment_id, text,
		       created_by, updated_by, created_at, updated_at, deleted_at, deleted_by
		FROM comments
		WHERE org_id = $1 AND resource_id = $2 AND deleted_at IS NULL
		ORDER BY created_at ASC`
	rows, err := d.db.QueryContext(ctx, q, orgID, resourceID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListComments: %w", err)
	}
	defer rows.Close()

	var out []comment.Comment
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListComments scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *DB) UpdateComment(ctx context.Context, c comment.Comment) error {
	const q = `
		UPDATE comments SET text=$1, updated_by=$2, updated_at=$3
		WHERE id=$4 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, c.Text, c.UpdatedBy, time.Now().UTC(), c.ID)
	if err != nil {
		return fmt.Errorf("postgres: UpdateComment: %w", err)
	}
	return nil
}

func (d *DB) SoftDeleteComment(ctx context.Context, id, deletedBy string) error {
	const q = `
		UPDATE comments SET deleted_at=$1, deleted_by=$2
		WHERE id=$3 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, id)
	if err != nil {
		return fmt.Errorf("postgres: SoftDeleteComment: %w", err)
	}
	return nil
}

func scanComment(row interface{ Scan(...any) error }) (comment.Comment, error) {
	var c comment.Comment
	return c, row.Scan(
		&c.ID, &c.OrgID, &c.ResourceID, &c.ParentCommentID, &c.Text,
		&c.CreatedBy, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt, &c.DeletedBy,
	)
}
