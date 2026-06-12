package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uigraph/app/internal/folder"
)

func (d *DB) CreateFolder(ctx context.Context, f folder.Folder) error {
	const q = `
		INSERT INTO folders (id, org_id, parent_id, type, name, ord, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	now := time.Now().UTC()
	if f.CreatedAt.IsZero() {
		f.CreatedAt = now
	}
	if f.UpdatedAt.IsZero() {
		f.UpdatedAt = now
	}
	_, err := d.db.ExecContext(ctx, q,
		f.ID, f.OrgID, f.ParentID, string(f.Type), f.Name, f.Order,
		f.CreatedBy, f.CreatedAt, f.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateFolder: %w", err)
	}
	return nil
}

func (d *DB) GetFolder(ctx context.Context, id string) (*folder.Folder, error) {
	const q = `
		SELECT id, org_id, parent_id, type, name, ord, created_by, created_at, updated_at, deleted_at
		FROM folders WHERE id = $1`
	f, err := scanFolder(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetFolder: %w", err)
	}
	return &f, nil
}

func (d *DB) ListFolders(ctx context.Context, orgID string, t *folder.Type) ([]folder.Folder, error) {
	q := `
		SELECT id, org_id, parent_id, type, name, ord, created_by, created_at, updated_at, deleted_at
		FROM folders WHERE org_id = $1 AND deleted_at IS NULL`
	args := []any{orgID}
	if t != nil {
		q += " AND type = $2"
		args = append(args, string(*t))
	}
	q += " ORDER BY ord ASC, created_at ASC"

	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListFolders: %w", err)
	}
	defer rows.Close()

	var out []folder.Folder
	for rows.Next() {
		f, err := scanFolder(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListFolders scan: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (d *DB) UpdateFolder(ctx context.Context, f folder.Folder) error {
	const q = `
		UPDATE folders SET name=$1, ord=$2, parent_id=$3, updated_at=$4
		WHERE id=$5 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, f.Name, f.Order, f.ParentID, time.Now().UTC(), f.ID)
	if err != nil {
		return fmt.Errorf("postgres: UpdateFolder: %w", err)
	}
	return nil
}

func (d *DB) DeleteFolder(ctx context.Context, id, _ string) error {
	const q = `UPDATE folders SET deleted_at=$1 WHERE id=$2 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("postgres: DeleteFolder: %w", err)
	}
	return nil
}

func scanFolder(row interface{ Scan(...any) error }) (folder.Folder, error) {
	var f folder.Folder
	var t string
	err := row.Scan(
		&f.ID, &f.OrgID, &f.ParentID, &t, &f.Name, &f.Order,
		&f.CreatedBy, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt,
	)
	f.Type = folder.Type(t)
	return f, err
}
