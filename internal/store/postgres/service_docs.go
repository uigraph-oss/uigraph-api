package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uigraph/app/internal/catalog"
	"github.com/uigraph/app/internal/docs"
)

func (d *DB) CreateServiceDoc(ctx context.Context, sd catalog.ServiceDoc) error {
	const q = `
		INSERT INTO service_docs
			(service_id, doc_id, org_id, created_by, updated_by,
			 created_by_commit_hash, updated_by_commit_hash, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (service_id, doc_id)
		DO UPDATE SET
			org_id = EXCLUDED.org_id,
			updated_by = EXCLUDED.updated_by,
			updated_by_commit_hash = EXCLUDED.updated_by_commit_hash,
			updated_at = EXCLUDED.updated_at,
			deleted_at = NULL,
			deleted_by = NULL`
	now := time.Now().UTC()
	if sd.CreatedAt.IsZero() {
		sd.CreatedAt = now
	}
	if sd.UpdatedAt.IsZero() {
		sd.UpdatedAt = now
	}
	_, err := d.db.ExecContext(ctx, q,
		sd.ServiceID, sd.DocID, sd.OrgID,
		sd.CreatedBy, sd.UpdatedBy,
		sd.CreatedByCommitHash, sd.UpdatedByCommitHash, sd.CreatedAt, sd.UpdatedAt,
	)
	return wrapErr("CreateServiceDoc", err)
}

func (d *DB) GetServiceDoc(ctx context.Context, serviceID, docID string) (*catalog.ServiceDoc, error) {
	const q = `
		SELECT service_id, doc_id, org_id, created_by, updated_by,
		       created_by_commit_hash, updated_by_commit_hash, created_at, updated_at, deleted_at
		FROM service_docs
		WHERE service_id = $1 AND doc_id = $2`
	sd, err := scanServiceDoc(d.db.QueryRowContext(ctx, q, serviceID, docID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetServiceDoc: %w", err)
	}
	return &sd, nil
}

func (d *DB) GetServiceDocByID(ctx context.Context, docID string) (*catalog.ServiceDoc, error) {
	const q = `
		SELECT service_id, doc_id, org_id, created_by, updated_by,
		       created_by_commit_hash, updated_by_commit_hash, created_at, updated_at, deleted_at
		FROM service_docs
		WHERE doc_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC
		LIMIT 1`
	sd, err := scanServiceDoc(d.db.QueryRowContext(ctx, q, docID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetServiceDocByID: %w", err)
	}
	return &sd, nil
}

func (d *DB) ListServiceDocs(ctx context.Context, serviceID string) ([]catalog.ServiceDoc, error) {
	const q = `
		SELECT
			sd.service_id, sd.doc_id, sd.org_id, sd.created_by, sd.updated_by,
			sd.created_by_commit_hash, sd.updated_by_commit_hash, sd.created_at, sd.updated_at, sd.deleted_at,
			d.id, d.org_id, d.folder_id, d.team_id, d.file_asset_id, d.file_name, d.file_type, d.description,
			d.content_hash, d.doc_token_count, d.created_by, d.updated_by,
			d.created_at, d.updated_at, d.deleted_at, d.deleted_by
		FROM service_docs sd
		JOIN docs d ON d.id = sd.doc_id
		WHERE sd.service_id = $1
		  AND sd.deleted_at IS NULL
		  AND d.deleted_at IS NULL
		ORDER BY sd.created_at DESC`
	rows, err := d.db.QueryContext(ctx, q, serviceID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListServiceDocs: %w", err)
	}
	defer rows.Close()

	var out []catalog.ServiceDoc
	for rows.Next() {
		var sd catalog.ServiceDoc
		var doc docs.Doc
		if err := rows.Scan(
			&sd.ServiceID, &sd.DocID, &sd.OrgID, &sd.CreatedBy, &sd.UpdatedBy,
			&sd.CreatedByCommitHash, &sd.UpdatedByCommitHash, &sd.CreatedAt, &sd.UpdatedAt, &sd.DeletedAt,
			&doc.ID, &doc.OrgID, &doc.FolderID, &doc.TeamID, &doc.FileAssetID, &doc.FileName, &doc.FileType, &doc.Description,
			&doc.ContentHash, &doc.DocTokenCount, &doc.CreatedBy, &doc.UpdatedBy,
			&doc.CreatedAt, &doc.UpdatedAt, &doc.DeletedAt, &doc.DeletedBy,
		); err != nil {
			return nil, fmt.Errorf("postgres: ListServiceDocs scan: %w", err)
		}
		sd.Doc = &doc
		out = append(out, sd)
	}
	return out, rows.Err()
}

func (d *DB) SoftDeleteServiceDoc(ctx context.Context, serviceID, docID, deletedBy string) error {
	const q = `
		UPDATE service_docs
		SET deleted_at = $1, deleted_by = $2, updated_by = $2, updated_at = $1
		WHERE service_id = $3 AND doc_id = $4 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, serviceID, docID)
	return wrapErr("SoftDeleteServiceDoc", err)
}

func scanServiceDoc(row interface{ Scan(...any) error }) (catalog.ServiceDoc, error) {
	var sd catalog.ServiceDoc
	return sd, row.Scan(
		&sd.ServiceID, &sd.DocID, &sd.OrgID, &sd.CreatedBy, &sd.UpdatedBy,
		&sd.CreatedByCommitHash, &sd.UpdatedByCommitHash, &sd.CreatedAt, &sd.UpdatedAt, &sd.DeletedAt,
	)
}
