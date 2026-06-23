package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uigraph/app/internal/docs"
)

func (d *DB) CreateDoc(ctx context.Context, doc docs.Doc) error {
	const q = `
		INSERT INTO docs
			(id, org_id, folder_id, team_id, file_asset_id, file_name, file_type, description,
			 content_hash, doc_token_count, created_by, updated_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`
	now := time.Now().UTC()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	if doc.UpdatedAt.IsZero() {
		doc.UpdatedAt = now
	}
	_, err := d.db.ExecContext(ctx, q,
		doc.ID, doc.OrgID, doc.FolderID, doc.TeamID, doc.FileAssetID, doc.FileName, doc.FileType, doc.Description,
		doc.ContentHash, doc.DocTokenCount, doc.CreatedBy, doc.UpdatedBy, doc.CreatedAt, doc.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateDoc: %w", err)
	}
	return nil
}

func (d *DB) GetDoc(ctx context.Context, id string) (*docs.Doc, error) {
	const q = `
		SELECT id, org_id, folder_id, team_id, file_asset_id, file_name, file_type, description,
		       content_hash, doc_token_count, created_by, updated_by, created_at, updated_at, deleted_at, deleted_by
		FROM docs WHERE id = $1`
	doc, err := scanDoc(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetDoc: %w", err)
	}
	return &doc, nil
}

func (d *DB) ListDocs(ctx context.Context, orgID string, folderID, teamID *string) ([]docs.Doc, error) {
	q := `
		SELECT id, org_id, folder_id, team_id, file_asset_id, file_name, file_type, description,
		       content_hash, doc_token_count, created_by, updated_by, created_at, updated_at, deleted_at, deleted_by
		FROM docs WHERE org_id = $1 AND deleted_at IS NULL`
	args := []any{orgID}
	if folderID != nil {
		args = append(args, *folderID)
		q += fmt.Sprintf(" AND folder_id = $%d", len(args))
	}
	if teamID != nil {
		args = append(args, *teamID)
		q += fmt.Sprintf(" AND team_id = $%d", len(args))
	}
	q += " ORDER BY created_at DESC"

	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListDocs: %w", err)
	}
	defer rows.Close()

	var out []docs.Doc
	for rows.Next() {
		doc, err := scanDoc(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListDocs scan: %w", err)
		}
		out = append(out, doc)
	}
	return out, rows.Err()
}

func (d *DB) UpdateDoc(ctx context.Context, doc docs.Doc) error {
	const q = `
		UPDATE docs
		SET folder_id=$1, team_id=$2, file_asset_id=$3, file_name=$4, file_type=$5, description=$6,
		    content_hash=$7, doc_token_count=$8, updated_by=$9, updated_at=$10
		WHERE id=$11 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q,
		doc.FolderID, doc.TeamID, doc.FileAssetID, doc.FileName, doc.FileType, doc.Description,
		doc.ContentHash, doc.DocTokenCount, doc.UpdatedBy, time.Now().UTC(), doc.ID,
	)
	if err != nil {
		return fmt.Errorf("postgres: UpdateDoc: %w", err)
	}
	return nil
}

func (d *DB) SoftDeleteDoc(ctx context.Context, id, deletedBy string) error {
	const q = `UPDATE docs SET deleted_at=$1, deleted_by=$2 WHERE id=$3 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, id)
	if err != nil {
		return fmt.Errorf("postgres: SoftDeleteDoc: %w", err)
	}
	return nil
}

func scanDoc(row interface{ Scan(...any) error }) (docs.Doc, error) {
	var doc docs.Doc
	return doc, row.Scan(
		&doc.ID, &doc.OrgID, &doc.FolderID, &doc.TeamID, &doc.FileAssetID, &doc.FileName, &doc.FileType, &doc.Description,
		&doc.ContentHash, &doc.DocTokenCount, &doc.CreatedBy, &doc.UpdatedBy, &doc.CreatedAt, &doc.UpdatedAt, &doc.DeletedAt, &doc.DeletedBy,
	)
}
