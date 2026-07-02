package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uigraph/app/internal/uimap"
)

// ── Maps ──────────────────────────────────────────────────────────────────────

func (d *DB) CreateMap(ctx context.Context, m uimap.Map) error {
	const q = `
		INSERT INTO maps
			(id, org_id, folder_id, team_id, name, description, status,
			 created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`
	now := time.Now().UTC()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = now
	}
	if m.Status == "" {
		m.Status = "active"
	}
	_, err := d.db.ExecContext(ctx, q,
		m.ID, m.OrgID, m.FolderID, m.TeamID, m.Name, m.Description, m.Status,
		m.CreatedBy, m.CreatedAt, m.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateMap: %w", err)
	}
	return nil
}

func (d *DB) GetMap(ctx context.Context, id string) (*uimap.Map, error) {
	const q = `
		SELECT id, org_id, folder_id, team_id, name, description, status,
		       created_by, updated_by, created_at, updated_at, deleted_at, deleted_by
		FROM maps WHERE id = $1`
	m, err := scanMap(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetMap: %w", err)
	}
	return &m, nil
}

func (d *DB) ListMaps(ctx context.Context, orgID string, p uimap.ListParams) ([]uimap.Map, int, error) {
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
	if err := d.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM maps"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres: ListMaps count: %w", err)
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
		SELECT id, org_id, folder_id, team_id, name, description, status,
		       created_by, updated_by, created_at, updated_at, deleted_at, deleted_by
		FROM maps` + where + fmt.Sprintf(" ORDER BY %s %s", col, dir)
	if p.Limit > 0 {
		args = append(args, p.Limit)
		q += fmt.Sprintf(" LIMIT $%d", len(args))
		args = append(args, p.Offset)
		q += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: ListMaps: %w", err)
	}
	defer rows.Close()

	var out []uimap.Map
	for rows.Next() {
		m, err := scanMap(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("postgres: ListMaps scan: %w", err)
		}
		out = append(out, m)
	}
	return out, total, rows.Err()
}

func (d *DB) UpdateMap(ctx context.Context, m uimap.Map) error {
	const q = `
		UPDATE maps
		SET name=$1, description=$2, status=$3, folder_id=$4, team_id=$5,
		    updated_by=$6, updated_at=$7
		WHERE id=$8 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q,
		m.Name, m.Description, m.Status, m.FolderID, m.TeamID,
		m.UpdatedBy, time.Now().UTC(), m.ID,
	)
	if err != nil {
		return fmt.Errorf("postgres: UpdateMap: %w", err)
	}
	return nil
}

func (d *DB) SoftDeleteMap(ctx context.Context, id, deletedBy string) error {
	const q = `
		UPDATE maps SET deleted_at=$1, deleted_by=$2
		WHERE id=$3 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, id)
	if err != nil {
		return fmt.Errorf("postgres: SoftDeleteMap: %w", err)
	}
	return nil
}

func scanMap(row interface{ Scan(...any) error }) (uimap.Map, error) {
	var m uimap.Map
	return m, row.Scan(
		&m.ID, &m.OrgID, &m.FolderID, &m.TeamID,
		&m.Name, &m.Description, &m.Status,
		&m.CreatedBy, &m.UpdatedBy,
		&m.CreatedAt, &m.UpdatedAt, &m.DeletedAt, &m.DeletedBy,
	)
}

// ── Frames ────────────────────────────────────────────────────────────────────

func (d *DB) CreateFrame(ctx context.Context, f uimap.Frame) error {
	const q = `
		INSERT INTO frames
			(id, map_id, org_id, parent_frame_id, name, description, template_type,
			 screenshot_asset_id, screenshot_content_hash, status, ord, source,
			 created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`
	now := time.Now().UTC()
	if f.CreatedAt.IsZero() {
		f.CreatedAt = now
	}
	if f.UpdatedAt.IsZero() {
		f.UpdatedAt = now
	}
	if f.Status == "" {
		f.Status = "active"
	}
	_, err := d.db.ExecContext(ctx, q,
		f.ID, f.MapID, f.OrgID, f.ParentFrameID,
		f.Name, f.Description, f.TemplateType,
		f.ScreenshotAssetID, f.ScreenshotContentHash,
		f.Status, f.Order, f.Source,
		f.CreatedBy, f.CreatedAt, f.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateFrame: %w", err)
	}
	return nil
}

func (d *DB) GetFrame(ctx context.Context, id string) (*uimap.Frame, error) {
	const q = `
		SELECT id, map_id, org_id, parent_frame_id, name, description, template_type,
		       screenshot_asset_id, screenshot_content_hash, status, ord, source,
		       created_by, updated_by, created_at, updated_at, deleted_at, deleted_by,
		       (SELECT COUNT(*) FROM focal_points fp WHERE fp.frame_id = frames.id AND fp.deleted_at IS NULL)
		FROM frames WHERE id = $1`
	f, err := scanFrame(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetFrame: %w", err)
	}
	return &f, nil
}

func (d *DB) ListFrames(ctx context.Context, mapID string, p uimap.ListParams) ([]uimap.Frame, int, error) {
	where := " WHERE map_id = $1 AND deleted_at IS NULL"
	args := []any{mapID}
	if p.Search != nil && *p.Search != "" {
		args = append(args, "%"+*p.Search+"%")
		where += fmt.Sprintf(" AND name ILIKE $%d", len(args))
	}

	var total int
	if err := d.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM frames"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres: ListFrames count: %w", err)
	}

	sortCols := map[string]string{"name": "name", "created": "created_at", "updated": "updated_at"}
	order := " ORDER BY ord ASC, created_at ASC"
	if col, ok := sortCols[p.SortBy]; ok {
		dir := "DESC"
		if strings.EqualFold(p.SortDir, "asc") {
			dir = "ASC"
		}
		order = fmt.Sprintf(" ORDER BY %s %s", col, dir)
	}

	q := `
		SELECT id, map_id, org_id, parent_frame_id, name, description, template_type,
		       screenshot_asset_id, screenshot_content_hash, status, ord, source,
		       created_by, updated_by, created_at, updated_at, deleted_at, deleted_by,
		       (SELECT COUNT(*) FROM focal_points fp WHERE fp.frame_id = frames.id AND fp.deleted_at IS NULL)
		FROM frames` + where + order
	if p.Limit > 0 {
		args = append(args, p.Limit)
		q += fmt.Sprintf(" LIMIT $%d", len(args))
		args = append(args, p.Offset)
		q += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: ListFrames: %w", err)
	}
	defer rows.Close()

	var out []uimap.Frame
	for rows.Next() {
		f, err := scanFrame(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("postgres: ListFrames scan: %w", err)
		}
		out = append(out, f)
	}
	return out, total, rows.Err()
}

func (d *DB) UpdateFrame(ctx context.Context, f uimap.Frame) error {
	const q = `
		UPDATE frames
		SET name=$1, description=$2, template_type=$3,
		    screenshot_asset_id=$4, screenshot_content_hash=$5,
		    status=$6, ord=$7, source=$8,
		    updated_by=$9, updated_at=$10
		WHERE id=$11 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q,
		f.Name, f.Description, f.TemplateType,
		f.ScreenshotAssetID, f.ScreenshotContentHash,
		f.Status, f.Order, f.Source,
		f.UpdatedBy, time.Now().UTC(), f.ID,
	)
	if err != nil {
		return fmt.Errorf("postgres: UpdateFrame: %w", err)
	}
	return nil
}

func (d *DB) SoftDeleteFrame(ctx context.Context, id, deletedBy string) error {
	const q = `
		UPDATE frames SET deleted_at=$1, deleted_by=$2
		WHERE id=$3 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, id)
	if err != nil {
		return fmt.Errorf("postgres: SoftDeleteFrame: %w", err)
	}
	return nil
}

func scanFrame(row interface{ Scan(...any) error }) (uimap.Frame, error) {
	var f uimap.Frame
	return f, row.Scan(
		&f.ID, &f.MapID, &f.OrgID, &f.ParentFrameID,
		&f.Name, &f.Description, &f.TemplateType,
		&f.ScreenshotAssetID, &f.ScreenshotContentHash,
		&f.Status, &f.Order, &f.Source,
		&f.CreatedBy, &f.UpdatedBy,
		&f.CreatedAt, &f.UpdatedAt, &f.DeletedAt, &f.DeletedBy,
		&f.FocalPointCount,
	)
}

// ── Focal points ──────────────────────────────────────────────────────────────

func (d *DB) CreateFocalPoint(ctx context.Context, fp uimap.FocalPoint) error {
	const q = `
		INSERT INTO focal_points
			(id, frame_id, org_id, name, location_x, location_y,
			 visibility, is_active, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`
	now := time.Now().UTC()
	if fp.CreatedAt.IsZero() {
		fp.CreatedAt = now
	}
	if fp.UpdatedAt.IsZero() {
		fp.UpdatedAt = now
	}
	if fp.Visibility == "" {
		fp.Visibility = "public"
	}
	_, err := d.db.ExecContext(ctx, q,
		fp.ID, fp.FrameID, fp.OrgID, fp.Name,
		fp.LocationX, fp.LocationY, fp.Visibility, fp.IsActive,
		fp.CreatedBy, fp.CreatedAt, fp.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateFocalPoint: %w", err)
	}
	return nil
}

func (d *DB) GetFocalPoint(ctx context.Context, id string) (*uimap.FocalPoint, error) {
	const q = `
		SELECT id, frame_id, org_id, name, location_x, location_y,
		       visibility, is_active, created_by, updated_by,
		       created_at, updated_at, deleted_at, deleted_by
		FROM focal_points WHERE id = $1`
	fp, err := scanFocalPoint(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetFocalPoint: %w", err)
	}
	return &fp, nil
}

func (d *DB) ListFocalPoints(ctx context.Context, frameID string) ([]uimap.FocalPoint, error) {
	const q = `
		SELECT id, frame_id, org_id, name, location_x, location_y,
		       visibility, is_active, created_by, updated_by,
		       created_at, updated_at, deleted_at, deleted_by
		FROM focal_points
		WHERE frame_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC`

	rows, err := d.db.QueryContext(ctx, q, frameID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListFocalPoints: %w", err)
	}
	defer rows.Close()

	var out []uimap.FocalPoint
	for rows.Next() {
		fp, err := scanFocalPoint(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListFocalPoints scan: %w", err)
		}
		out = append(out, fp)
	}
	return out, rows.Err()
}

func (d *DB) UpdateFocalPoint(ctx context.Context, fp uimap.FocalPoint) error {
	const q = `
		UPDATE focal_points
		SET name=$1, location_x=$2, location_y=$3,
		    visibility=$4, is_active=$5,
		    updated_by=$6, updated_at=$7
		WHERE id=$8 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q,
		fp.Name, fp.LocationX, fp.LocationY,
		fp.Visibility, fp.IsActive,
		fp.UpdatedBy, time.Now().UTC(), fp.ID,
	)
	if err != nil {
		return fmt.Errorf("postgres: UpdateFocalPoint: %w", err)
	}
	return nil
}

func (d *DB) SoftDeleteFocalPoint(ctx context.Context, id, deletedBy string) error {
	const q = `
		UPDATE focal_points SET deleted_at=$1, deleted_by=$2
		WHERE id=$3 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, id)
	if err != nil {
		return fmt.Errorf("postgres: SoftDeleteFocalPoint: %w", err)
	}
	return nil
}

func scanFocalPoint(row interface{ Scan(...any) error }) (uimap.FocalPoint, error) {
	var fp uimap.FocalPoint
	return fp, row.Scan(
		&fp.ID, &fp.FrameID, &fp.OrgID,
		&fp.Name, &fp.LocationX, &fp.LocationY,
		&fp.Visibility, &fp.IsActive,
		&fp.CreatedBy, &fp.UpdatedBy,
		&fp.CreatedAt, &fp.UpdatedAt, &fp.DeletedAt, &fp.DeletedBy,
	)
}

// ── Canvas ────────────────────────────────────────────────────────────────────

func (d *DB) GetCanvas(ctx context.Context, mapID string) (*uimap.Canvas, error) {
	const q = `
		SELECT map_id, org_id, zoom, navigation_x, navigation_y,
		       frame_positions, updated_at
		FROM map_canvas WHERE map_id = $1`
	var c uimap.Canvas
	var posRaw []byte
	err := d.db.QueryRowContext(ctx, q, mapID).Scan(
		&c.MapID, &c.OrgID, &c.Zoom, &c.NavigationX, &c.NavigationY,
		&posRaw, &c.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetCanvas: %w", err)
	}
	if err := json.Unmarshal(posRaw, &c.FramePositions); err != nil {
		return nil, fmt.Errorf("postgres: GetCanvas decode positions: %w", err)
	}
	return &c, nil
}

func (d *DB) UpsertCanvas(ctx context.Context, c uimap.Canvas) error {
	const q = `
		INSERT INTO map_canvas
			(map_id, org_id, zoom, navigation_x, navigation_y, frame_positions, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (map_id) DO UPDATE SET
			zoom=$3, navigation_x=$4, navigation_y=$5,
			frame_positions=$6, updated_at=$7`
	posRaw, err := json.Marshal(c.FramePositions)
	if err != nil {
		return fmt.Errorf("postgres: UpsertCanvas encode positions: %w", err)
	}
	_, err = d.db.ExecContext(ctx, q,
		c.MapID, c.OrgID, c.Zoom, c.NavigationX, c.NavigationY,
		posRaw, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("postgres: UpsertCanvas: %w", err)
	}
	return nil
}
