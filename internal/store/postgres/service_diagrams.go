package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uigraph/app/internal/catalog"
	"github.com/uigraph/app/internal/diagram"
)

func (d *DB) CreateServiceDiagram(ctx context.Context, sd catalog.ServiceDiagram) error {
	const q = `
		INSERT INTO service_diagrams
			(service_id, diagram_id, org_id, created_by, updated_by,
			 created_by_commit_hash, updated_by_commit_hash, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (service_id, diagram_id)
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
		sd.ServiceID, sd.DiagramID, sd.OrgID,
		sd.CreatedBy, sd.UpdatedBy,
		sd.CreatedByCommitHash, sd.UpdatedByCommitHash, sd.CreatedAt, sd.UpdatedAt,
	)
	return wrapErr("CreateServiceDiagram", err)
}

func (d *DB) GetServiceDiagram(ctx context.Context, serviceID, diagramID string) (*catalog.ServiceDiagram, error) {
	const q = `
		SELECT service_id, diagram_id, org_id, created_by, updated_by,
		       created_by_commit_hash, updated_by_commit_hash, created_at, updated_at, deleted_at
		FROM service_diagrams
		WHERE service_id = $1 AND diagram_id = $2`
	sd, err := scanServiceDiagram(d.db.QueryRowContext(ctx, q, serviceID, diagramID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetServiceDiagram: %w", err)
	}
	return &sd, nil
}

func (d *DB) ListServiceDiagrams(ctx context.Context, serviceID string) ([]catalog.ServiceDiagram, error) {
	const q = `
		SELECT
			sd.service_id, sd.diagram_id, sd.org_id, sd.created_by, sd.updated_by,
			sd.created_by_commit_hash, sd.updated_by_commit_hash, sd.created_at, sd.updated_at, sd.deleted_at,
			d.id, d.org_id, d.folder_id, d.team_id, d.name, d.content_key, d.content_hash,
			d.preview_asset_id, d.preview_content_hash, d.source, d.created_by, d.updated_by,
			d.created_at, d.updated_at, d.deleted_at, d.deleted_by
		FROM service_diagrams sd
		JOIN diagrams d ON d.id = sd.diagram_id
		WHERE sd.service_id = $1
		  AND sd.deleted_at IS NULL
		  AND d.deleted_at IS NULL
		ORDER BY sd.created_at DESC`
	rows, err := d.db.QueryContext(ctx, q, serviceID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListServiceDiagrams: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []catalog.ServiceDiagram
	for rows.Next() {
		var sd catalog.ServiceDiagram
		var dg diagram.Diagram
		if err := rows.Scan(
			&sd.ServiceID, &sd.DiagramID, &sd.OrgID, &sd.CreatedBy, &sd.UpdatedBy,
			&sd.CreatedByCommitHash, &sd.UpdatedByCommitHash, &sd.CreatedAt, &sd.UpdatedAt, &sd.DeletedAt,
			&dg.ID, &dg.OrgID, &dg.FolderID, &dg.TeamID, &dg.Name, &dg.ContentKey, &dg.ContentHash,
			&dg.PreviewAssetID, &dg.PreviewContentHash, &dg.Source, &dg.CreatedBy, &dg.UpdatedBy,
			&dg.CreatedAt, &dg.UpdatedAt, &dg.DeletedAt, &dg.DeletedBy,
		); err != nil {
			return nil, fmt.Errorf("postgres: ListServiceDiagrams scan: %w", err)
		}
		sd.Diagram = &dg
		out = append(out, sd)
	}
	return out, rows.Err()
}

func (d *DB) SoftDeleteServiceDiagram(ctx context.Context, serviceID, diagramID, deletedBy string) error {
	const q = `
		UPDATE service_diagrams
		SET deleted_at = $1, deleted_by = $2, updated_by = $2, updated_at = $1
		WHERE service_id = $3 AND diagram_id = $4 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, serviceID, diagramID)
	return wrapErr("SoftDeleteServiceDiagram", err)
}

func scanServiceDiagram(row interface{ Scan(...any) error }) (catalog.ServiceDiagram, error) {
	var sd catalog.ServiceDiagram
	return sd, row.Scan(
		&sd.ServiceID, &sd.DiagramID, &sd.OrgID, &sd.CreatedBy, &sd.UpdatedBy,
		&sd.CreatedByCommitHash, &sd.UpdatedByCommitHash, &sd.CreatedAt, &sd.UpdatedAt, &sd.DeletedAt,
	)
}
