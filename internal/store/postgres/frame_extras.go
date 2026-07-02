package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uigraph/app/internal/uimap"
)

func (d *DB) CreateFrameGroup(ctx context.Context, g uimap.FrameGroup) error {
	const q = `
		INSERT INTO frame_groups
			(id, frame_id, org_id, name, description, location_x, location_y,
			 width, height, ord, is_active, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`
	now := time.Now().UTC()
	if g.CreatedAt.IsZero() {
		g.CreatedAt = now
	}
	if g.UpdatedAt.IsZero() {
		g.UpdatedAt = now
	}
	_, err := d.db.ExecContext(ctx, q,
		g.ID, g.FrameID, g.OrgID, g.Name, g.Description,
		g.LocationX, g.LocationY, g.Width, g.Height, g.Order, g.IsActive,
		g.CreatedBy, g.CreatedAt, g.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateFrameGroup: %w", err)
	}
	return nil
}

func (d *DB) GetFrameGroup(ctx context.Context, id string) (*uimap.FrameGroup, error) {
	const q = `
		SELECT id, frame_id, org_id, name, description, location_x, location_y,
		       width, height, ord, is_active, created_by, updated_by,
		       created_at, updated_at, deleted_at, deleted_by
		FROM frame_groups WHERE id = $1`
	g, err := scanFrameGroup(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetFrameGroup: %w", err)
	}
	return &g, nil
}

func (d *DB) ListFrameGroups(ctx context.Context, frameID string) ([]uimap.FrameGroup, error) {
	const q = `
		SELECT id, frame_id, org_id, name, description, location_x, location_y,
		       width, height, ord, is_active, created_by, updated_by,
		       created_at, updated_at, deleted_at, deleted_by
		FROM frame_groups
		WHERE frame_id = $1 AND deleted_at IS NULL
		ORDER BY ord ASC, created_at ASC`
	rows, err := d.db.QueryContext(ctx, q, frameID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListFrameGroups: %w", err)
	}
	defer rows.Close()

	var out []uimap.FrameGroup
	for rows.Next() {
		g, err := scanFrameGroup(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListFrameGroups scan: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (d *DB) UpdateFrameGroup(ctx context.Context, g uimap.FrameGroup) error {
	const q = `
		UPDATE frame_groups
		SET name=$1, description=$2, location_x=$3, location_y=$4,
		    width=$5, height=$6, ord=$7, is_active=$8,
		    updated_by=$9, updated_at=$10
		WHERE id=$11 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q,
		g.Name, g.Description, g.LocationX, g.LocationY,
		g.Width, g.Height, g.Order, g.IsActive,
		g.UpdatedBy, time.Now().UTC(), g.ID,
	)
	if err != nil {
		return fmt.Errorf("postgres: UpdateFrameGroup: %w", err)
	}
	return nil
}

func (d *DB) SoftDeleteFrameGroup(ctx context.Context, id, deletedBy string) error {
	const q = `
		UPDATE frame_groups SET deleted_at=$1, deleted_by=$2
		WHERE id=$3 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, id)
	if err != nil {
		return fmt.Errorf("postgres: SoftDeleteFrameGroup: %w", err)
	}
	return nil
}

func scanFrameGroup(row interface{ Scan(...any) error }) (uimap.FrameGroup, error) {
	var g uimap.FrameGroup
	return g, row.Scan(
		&g.ID, &g.FrameID, &g.OrgID, &g.Name, &g.Description,
		&g.LocationX, &g.LocationY, &g.Width, &g.Height, &g.Order, &g.IsActive,
		&g.CreatedBy, &g.UpdatedBy,
		&g.CreatedAt, &g.UpdatedAt, &g.DeletedAt, &g.DeletedBy,
	)
}

func (d *DB) CreateFrameLink(ctx context.Context, l uimap.FrameLink) error {
	const q = `
		INSERT INTO frame_links
			(id, frame_id, org_id, kind, target_frame_id, target_map_id,
			 label, location_x, location_y, is_active, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`
	now := time.Now().UTC()
	if l.CreatedAt.IsZero() {
		l.CreatedAt = now
	}
	if l.UpdatedAt.IsZero() {
		l.UpdatedAt = now
	}
	_, err := d.db.ExecContext(ctx, q,
		l.ID, l.FrameID, l.OrgID, l.Kind, l.TargetFrameID, l.TargetMapID,
		l.Label, l.LocationX, l.LocationY, l.IsActive,
		l.CreatedBy, l.CreatedAt, l.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateFrameLink: %w", err)
	}
	return nil
}

func (d *DB) GetFrameLink(ctx context.Context, id string) (*uimap.FrameLink, error) {
	const q = `
		SELECT id, frame_id, org_id, kind, target_frame_id, target_map_id,
		       label, location_x, location_y, is_active, created_by, updated_by,
		       created_at, updated_at, deleted_at, deleted_by
		FROM frame_links WHERE id = $1`
	l, err := scanFrameLink(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetFrameLink: %w", err)
	}
	return &l, nil
}

func (d *DB) ListFrameLinks(ctx context.Context, frameID string) ([]uimap.FrameLink, error) {
	const q = `
		SELECT id, frame_id, org_id, kind, target_frame_id, target_map_id,
		       label, location_x, location_y, is_active, created_by, updated_by,
		       created_at, updated_at, deleted_at, deleted_by
		FROM frame_links
		WHERE frame_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC`
	rows, err := d.db.QueryContext(ctx, q, frameID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListFrameLinks: %w", err)
	}
	defer rows.Close()

	var out []uimap.FrameLink
	for rows.Next() {
		l, err := scanFrameLink(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListFrameLinks scan: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (d *DB) UpdateFrameLink(ctx context.Context, l uimap.FrameLink) error {
	const q = `
		UPDATE frame_links
		SET kind=$1, target_frame_id=$2, target_map_id=$3, label=$4,
		    location_x=$5, location_y=$6, is_active=$7,
		    updated_by=$8, updated_at=$9
		WHERE id=$10 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q,
		l.Kind, l.TargetFrameID, l.TargetMapID, l.Label,
		l.LocationX, l.LocationY, l.IsActive,
		l.UpdatedBy, time.Now().UTC(), l.ID,
	)
	if err != nil {
		return fmt.Errorf("postgres: UpdateFrameLink: %w", err)
	}
	return nil
}

func (d *DB) SoftDeleteFrameLink(ctx context.Context, id, deletedBy string) error {
	const q = `
		UPDATE frame_links SET deleted_at=$1, deleted_by=$2
		WHERE id=$3 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, id)
	if err != nil {
		return fmt.Errorf("postgres: SoftDeleteFrameLink: %w", err)
	}
	return nil
}

func scanFrameLink(row interface{ Scan(...any) error }) (uimap.FrameLink, error) {
	var l uimap.FrameLink
	return l, row.Scan(
		&l.ID, &l.FrameID, &l.OrgID, &l.Kind, &l.TargetFrameID, &l.TargetMapID,
		&l.Label, &l.LocationX, &l.LocationY, &l.IsActive,
		&l.CreatedBy, &l.UpdatedBy,
		&l.CreatedAt, &l.UpdatedAt, &l.DeletedAt, &l.DeletedBy,
	)
}

func (d *DB) CreateFocalPointMeta(ctx context.Context, m uimap.FocalPointMeta) error {
	const q = `
		INSERT INTO focal_point_meta
			(id, focal_point_id, org_id, frame_id, component_id,
			 component_link_diagram_id, component_link_api_endpoint_id,
			 component_link_test_pack_id, component_link_service_doc_id,
			 component_modal_fields, created_by, created_by_commit_hash, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`
	now := time.Now().UTC()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = now
	}
	fields := defaultJSON(m.ComponentModalFields, "[]")
	_, err := d.db.ExecContext(ctx, q,
		m.ID, m.FocalPointID, m.OrgID, m.FrameID, m.ComponentID,
		m.ComponentLinkDiagramID, m.ComponentLinkAPIEndpointID,
		m.ComponentLinkTestPackID, m.ComponentLinkServiceDocID,
		fields, m.CreatedBy, m.CreatedByCommitHash, m.CreatedAt, m.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateFocalPointMeta: %w", err)
	}
	return nil
}

func (d *DB) GetFocalPointMeta(ctx context.Context, id string) (*uimap.FocalPointMeta, error) {
	const q = `
		SELECT id, focal_point_id, org_id, frame_id, component_id,
		       component_link_diagram_id, component_link_api_endpoint_id,
		       component_link_test_pack_id, component_link_service_doc_id,
		       component_modal_fields,
		       created_by, updated_by, created_by_commit_hash, updated_by_commit_hash,
		       created_at, updated_at, deleted_at, deleted_by
		FROM focal_point_meta WHERE id = $1`
	m, err := scanFocalPointMeta(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetFocalPointMeta: %w", err)
	}
	return &m, nil
}

func (d *DB) ListFocalPointMeta(ctx context.Context, focalPointID string) ([]uimap.FocalPointMeta, error) {
	const q = `
		SELECT id, focal_point_id, org_id, frame_id, component_id,
		       component_link_diagram_id, component_link_api_endpoint_id,
		       component_link_test_pack_id, component_link_service_doc_id,
		       component_modal_fields,
		       created_by, updated_by, created_by_commit_hash, updated_by_commit_hash,
		       created_at, updated_at, deleted_at, deleted_by
		FROM focal_point_meta
		WHERE focal_point_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC`
	rows, err := d.db.QueryContext(ctx, q, focalPointID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListFocalPointMeta: %w", err)
	}
	defer rows.Close()

	var out []uimap.FocalPointMeta
	for rows.Next() {
		m, err := scanFocalPointMeta(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListFocalPointMeta scan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (d *DB) ListFocalPointMetaByLink(ctx context.Context, orgID, linkID string) ([]uimap.FocalPointMeta, error) {
	const q = `
		SELECT id, focal_point_id, org_id, frame_id, component_id,
		       component_link_diagram_id, component_link_api_endpoint_id,
		       component_link_test_pack_id, component_link_service_doc_id,
		       component_modal_fields,
		       created_by, updated_by, created_by_commit_hash, updated_by_commit_hash,
		       created_at, updated_at, deleted_at, deleted_by
		FROM focal_point_meta
		WHERE org_id = $1 AND deleted_at IS NULL
		  AND $2::uuid IN (component_link_diagram_id, component_link_api_endpoint_id,
		                   component_link_test_pack_id, component_link_service_doc_id)
		ORDER BY created_at ASC`
	rows, err := d.db.QueryContext(ctx, q, orgID, linkID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListFocalPointMetaByLink: %w", err)
	}
	defer rows.Close()

	var out []uimap.FocalPointMeta
	for rows.Next() {
		m, err := scanFocalPointMeta(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListFocalPointMetaByLink scan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListComponentLinkUsages returns the maps, frames (screens), and focal points
// that reference linkID through their focal point meta. It walks meta → focal
// point → frame → map in a single join so callers don't have to fan out.
func (d *DB) ListComponentLinkUsages(ctx context.Context, orgID, linkID string) ([]uimap.ComponentLinkUsage, error) {
	const q = `
		SELECT m.id, m.org_id, m.component_id,
		       mp.id, mp.name,
		       fr.id, fr.name, fr.screenshot_asset_id,
		       fp.id, fp.name, fp.location_x, fp.location_y
		FROM focal_point_meta m
		JOIN focal_points fp ON fp.id = m.focal_point_id AND fp.deleted_at IS NULL
		JOIN frames fr ON fr.id = m.frame_id AND fr.deleted_at IS NULL
		JOIN maps mp ON mp.id = fr.map_id AND mp.deleted_at IS NULL
		WHERE m.org_id = $1 AND m.deleted_at IS NULL
		  AND $2::uuid IN (m.component_link_diagram_id, m.component_link_api_endpoint_id,
		                   m.component_link_test_pack_id, m.component_link_service_doc_id)
		ORDER BY mp.name ASC, fr.ord ASC, fp.name ASC`
	rows, err := d.db.QueryContext(ctx, q, orgID, linkID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListComponentLinkUsages: %w", err)
	}
	defer rows.Close()

	var out []uimap.ComponentLinkUsage
	for rows.Next() {
		var u uimap.ComponentLinkUsage
		if err := rows.Scan(
			&u.MetaID, &u.OrgID, &u.ComponentID,
			&u.MapID, &u.MapName,
			&u.FrameID, &u.FrameName, &u.ScreenshotAssetID,
			&u.FocalPointID, &u.FocalPointName, &u.LocationX, &u.LocationY,
		); err != nil {
			return nil, fmt.Errorf("postgres: ListComponentLinkUsages scan: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (d *DB) UpdateFocalPointMeta(ctx context.Context, m uimap.FocalPointMeta) error {
	const q = `
		UPDATE focal_point_meta
		SET component_id=$1,
		    component_link_diagram_id=$2, component_link_api_endpoint_id=$3,
		    component_link_test_pack_id=$4, component_link_service_doc_id=$5,
		    component_modal_fields=$6, updated_by=$7, updated_by_commit_hash=$8, updated_at=$9
		WHERE id=$10 AND deleted_at IS NULL`
	fields := defaultJSON(m.ComponentModalFields, "[]")
	_, err := d.db.ExecContext(ctx, q,
		m.ComponentID,
		m.ComponentLinkDiagramID, m.ComponentLinkAPIEndpointID,
		m.ComponentLinkTestPackID, m.ComponentLinkServiceDocID,
		fields, m.UpdatedBy, m.UpdatedByCommitHash, time.Now().UTC(), m.ID,
	)
	if err != nil {
		return fmt.Errorf("postgres: UpdateFocalPointMeta: %w", err)
	}
	return nil
}

func (d *DB) SoftDeleteFocalPointMeta(ctx context.Context, id, deletedBy string) error {
	const q = `
		UPDATE focal_point_meta SET deleted_at=$1, deleted_by=$2
		WHERE id=$3 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, id)
	if err != nil {
		return fmt.Errorf("postgres: SoftDeleteFocalPointMeta: %w", err)
	}
	return nil
}

func scanFocalPointMeta(row interface{ Scan(...any) error }) (uimap.FocalPointMeta, error) {
	var m uimap.FocalPointMeta
	return m, row.Scan(
		&m.ID, &m.FocalPointID, &m.OrgID, &m.FrameID, &m.ComponentID,
		&m.ComponentLinkDiagramID, &m.ComponentLinkAPIEndpointID,
		&m.ComponentLinkTestPackID, &m.ComponentLinkServiceDocID,
		&m.ComponentModalFields,
		&m.CreatedBy, &m.UpdatedBy,
		&m.CreatedByCommitHash, &m.UpdatedByCommitHash,
		&m.CreatedAt, &m.UpdatedAt, &m.DeletedAt, &m.DeletedBy,
	)
}

func defaultJSON(raw []byte, fallback string) []byte {
	if len(raw) == 0 {
		return []byte(fallback)
	}
	return raw
}
