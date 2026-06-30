package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uigraph/app/internal/diagram"
)

// ── Diagrams ──────────────────────────────────────────────────────────────────

func (d *DB) CreateDiagram(ctx context.Context, dg diagram.Diagram) error {
	const q = `
		INSERT INTO diagrams
			(id, org_id, folder_id, team_id, name, content_key, content_hash, content_token_count,
			 preview_asset_id, preview_content_hash, preview_status, source, created_by, updated_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`
	now := time.Now().UTC()
	if dg.CreatedAt.IsZero() {
		dg.CreatedAt = now
	}
	if dg.UpdatedAt.IsZero() {
		dg.UpdatedAt = now
	}
	if dg.PreviewStatus == "" {
		dg.PreviewStatus = diagram.PreviewStatusPending
	}
	_, err := d.db.ExecContext(ctx, q,
		dg.ID, dg.OrgID, dg.FolderID, dg.TeamID, dg.Name, dg.ContentKey, dg.ContentHash, dg.ContentTokenCount,
		dg.PreviewAssetID, dg.PreviewContentHash, dg.PreviewStatus, dg.Source, dg.CreatedBy, dg.UpdatedBy, dg.CreatedAt, dg.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateDiagram: %w", err)
	}
	return nil
}

func (d *DB) GetDiagram(ctx context.Context, id string) (*diagram.Diagram, error) {
	const q = `
		SELECT id, org_id, folder_id, team_id, name, content_key, content_hash, content_token_count,
		       preview_asset_id, preview_content_hash, preview_status, source, created_by, updated_by,
		       created_at, updated_at, deleted_at, deleted_by
		FROM diagrams WHERE id = $1`
	dg, err := scanDiagram(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetDiagram: %w", err)
	}
	return &dg, nil
}

func (d *DB) ListDiagrams(ctx context.Context, orgID string, p diagram.ListParams) ([]diagram.Diagram, int, error) {
	where := " WHERE org_id = $1 AND deleted_at IS NULL"
	args := []any{orgID}
	if p.FolderID != nil {
		args = append(args, *p.FolderID)
		where += fmt.Sprintf(" AND folder_id = $%d", len(args))
	}
	if p.TeamID != nil {
		args = append(args, *p.TeamID)
		where += fmt.Sprintf(" AND team_id = $%d", len(args))
	}
	if p.Search != nil && *p.Search != "" {
		args = append(args, "%"+*p.Search+"%")
		where += fmt.Sprintf(" AND name ILIKE $%d", len(args))
	}

	var total int
	if err := d.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM diagrams"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres: ListDiagrams count: %w", err)
	}

	sortCols := map[string]string{"name": "name", "created": "created_at", "updated": "updated_at"}
	col, ok := sortCols[p.SortBy]
	if !ok {
		col = "created_at"
	}
	dir := "DESC"
	if strings.EqualFold(p.SortDir, "asc") {
		dir = "ASC"
	}

	q := `
		SELECT id, org_id, folder_id, team_id, name, content_key, content_hash, content_token_count,
		       preview_asset_id, preview_content_hash, preview_status, source, created_by, updated_by,
		       created_at, updated_at, deleted_at, deleted_by
		FROM diagrams` + where + fmt.Sprintf(" ORDER BY %s %s", col, dir)
	if p.Limit > 0 {
		args = append(args, p.Limit)
		q += fmt.Sprintf(" LIMIT $%d", len(args))
		args = append(args, p.Offset)
		q += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: ListDiagrams: %w", err)
	}
	defer rows.Close()

	var out []diagram.Diagram
	for rows.Next() {
		dg, err := scanDiagram(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("postgres: ListDiagrams scan: %w", err)
		}
		out = append(out, dg)
	}
	return out, total, rows.Err()
}

func (d *DB) UpdateDiagram(ctx context.Context, dg diagram.Diagram) error {
	const q = `
		UPDATE diagrams
		SET name=$1, folder_id=$2, team_id=$3, content_key=$4, content_hash=$5, content_token_count=$6,
		    preview_asset_id=$7, preview_content_hash=$8, preview_status=$9, source=$10, updated_by=$11, updated_at=$12
		WHERE id=$13 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q,
		dg.Name, dg.FolderID, dg.TeamID, dg.ContentKey, dg.ContentHash, dg.ContentTokenCount,
		dg.PreviewAssetID, dg.PreviewContentHash, dg.PreviewStatus, dg.Source, dg.UpdatedBy, time.Now().UTC(), dg.ID,
	)
	if err != nil {
		return fmt.Errorf("postgres: UpdateDiagram: %w", err)
	}
	return nil
}

func (d *DB) SetDiagramPreviewStatus(ctx context.Context, id, status string) error {
	const q = `UPDATE diagrams SET preview_status=$1 WHERE id=$2 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, status, id)
	if err != nil {
		return fmt.Errorf("postgres: SetDiagramPreviewStatus: %w", err)
	}
	return nil
}

func (d *DB) SoftDeleteDiagram(ctx context.Context, id, deletedBy string) error {
	const q = `UPDATE diagrams SET deleted_at=$1, deleted_by=$2 WHERE id=$3 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, id)
	if err != nil {
		return fmt.Errorf("postgres: SoftDeleteDiagram: %w", err)
	}
	return nil
}

// ── Diagram versions ──────────────────────────────────────────────────────────

func (d *DB) CreateDiagramVersion(ctx context.Context, v diagram.Version) error {
	const q = `
		INSERT INTO diagram_versions
			(id, diagram_id, version_number, label, content_key, content_hash,
			 is_auto_version, source, created_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now().UTC()
	}
	_, err := d.db.ExecContext(ctx, q,
		v.ID, v.DiagramID, v.VersionNumber, v.Label, v.ContentKey, v.ContentHash,
		v.IsAutoVersion, v.Source, v.CreatedBy, v.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateDiagramVersion: %w", err)
	}
	return nil
}

func (d *DB) GetDiagramVersion(ctx context.Context, id string) (*diagram.Version, error) {
	const q = `
		SELECT id, diagram_id, version_number, label, content_key, content_hash,
		       is_auto_version, source, created_by, created_at
		FROM diagram_versions WHERE id = $1`
	v, err := scanVersion(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetDiagramVersion: %w", err)
	}
	return &v, nil
}

func (d *DB) ListDiagramVersions(ctx context.Context, diagramID string) ([]diagram.Version, error) {
	const q = `
		SELECT id, diagram_id, version_number, label, content_key, content_hash,
		       is_auto_version, source, created_by, created_at
		FROM diagram_versions WHERE diagram_id = $1 ORDER BY version_number DESC`

	rows, err := d.db.QueryContext(ctx, q, diagramID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListDiagramVersions: %w", err)
	}
	defer rows.Close()

	var out []diagram.Version
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListDiagramVersions scan: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (d *DB) LatestVersionNumber(ctx context.Context, diagramID string) (int, error) {
	const q = `SELECT COALESCE(MAX(version_number), 0) FROM diagram_versions WHERE diagram_id = $1`
	var n int
	if err := d.db.QueryRowContext(ctx, q, diagramID).Scan(&n); err != nil {
		return 0, fmt.Errorf("postgres: LatestVersionNumber: %w", err)
	}
	return n, nil
}

func (d *DB) CreateDiagramImage(ctx context.Context, img diagram.Image) error {
	const q = `
		INSERT INTO diagram_images
			(id, diagram_id, org_id, asset_id, file_name, "order", created_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	if img.CreatedAt.IsZero() {
		img.CreatedAt = time.Now().UTC()
	}
	_, err := d.db.ExecContext(ctx, q,
		img.ID, img.DiagramID, img.OrgID, img.AssetID, img.FileName, img.Order,
		img.CreatedBy, img.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateDiagramImage: %w", err)
	}
	return nil
}

func (d *DB) ListDiagramImages(ctx context.Context, diagramID string) ([]diagram.Image, error) {
	const q = `
		SELECT id, diagram_id, org_id, asset_id, file_name, "order", created_by, created_at
		FROM diagram_images WHERE diagram_id = $1 ORDER BY "order" ASC, created_at ASC`

	rows, err := d.db.QueryContext(ctx, q, diagramID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListDiagramImages: %w", err)
	}
	defer rows.Close()

	var out []diagram.Image
	for rows.Next() {
		var img diagram.Image
		if err := rows.Scan(
			&img.ID, &img.DiagramID, &img.OrgID, &img.AssetID, &img.FileName,
			&img.Order, &img.CreatedBy, &img.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres: ListDiagramImages scan: %w", err)
		}
		out = append(out, img)
	}
	return out, rows.Err()
}

// ── scanners ──────────────────────────────────────────────────────────────────

func scanDiagram(row interface{ Scan(...any) error }) (diagram.Diagram, error) {
	var dg diagram.Diagram
	return dg, row.Scan(
		&dg.ID, &dg.OrgID, &dg.FolderID, &dg.TeamID, &dg.Name,
		&dg.ContentKey, &dg.ContentHash, &dg.ContentTokenCount, &dg.PreviewAssetID, &dg.PreviewContentHash,
		&dg.PreviewStatus, &dg.Source, &dg.CreatedBy, &dg.UpdatedBy,
		&dg.CreatedAt, &dg.UpdatedAt, &dg.DeletedAt, &dg.DeletedBy,
	)
}

func scanVersion(row interface{ Scan(...any) error }) (diagram.Version, error) {
	var v diagram.Version
	return v, row.Scan(
		&v.ID, &v.DiagramID, &v.VersionNumber, &v.Label,
		&v.ContentKey, &v.ContentHash, &v.IsAutoVersion,
		&v.Source, &v.CreatedBy, &v.CreatedAt,
	)
}
