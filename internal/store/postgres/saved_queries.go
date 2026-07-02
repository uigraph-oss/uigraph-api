package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/uigraph/app/internal/catalog"
)

func (d *DB) CreateSavedQueryFolder(ctx context.Context, f catalog.SavedQueryFolder) error {
	const q = `
		INSERT INTO saved_query_folders
			(id, org_id, service_db_id, scope, owner_user_id, team_id, name, created_by, updated_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`
	now := time.Now().UTC()
	if f.CreatedAt.IsZero() {
		f.CreatedAt = now
	}
	if f.UpdatedAt.IsZero() {
		f.UpdatedAt = now
	}
	_, err := d.db.ExecContext(ctx, q,
		f.ID, f.OrgID, f.ServiceDBID, f.Scope, f.OwnerUserID, f.TeamID, f.Name, f.CreatedBy, f.UpdatedBy, f.CreatedAt, f.UpdatedAt,
	)
	return wrapErr("CreateSavedQueryFolder", err)
}

func (d *DB) GetSavedQueryFolder(ctx context.Context, id string) (*catalog.SavedQueryFolder, error) {
	const q = `
		SELECT id, org_id, service_db_id, scope, owner_user_id, team_id, name,
		       created_by, updated_by, created_at, updated_at, deleted_at, deleted_by
		FROM saved_query_folders
		WHERE id = $1`
	f, err := scanSavedQueryFolder(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetSavedQueryFolder: %w", err)
	}
	return &f, nil
}

func (d *DB) ListSavedQueryFolders(ctx context.Context, serviceDBID string, scope catalog.SavedQueryScope, ownerUserID *string) ([]catalog.SavedQueryFolder, error) {
	const q = `
		SELECT id, org_id, service_db_id, scope, owner_user_id, team_id, name,
		       created_by, updated_by, created_at, updated_at, deleted_at, deleted_by
		FROM saved_query_folders
		WHERE service_db_id = $1 AND scope = $2 AND deleted_at IS NULL
		  AND ($3::uuid IS NULL OR owner_user_id = $3)
		ORDER BY name ASC`
	rows, err := d.db.QueryContext(ctx, q, serviceDBID, scope, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListSavedQueryFolders: %w", err)
	}
	defer rows.Close()

	var out []catalog.SavedQueryFolder
	for rows.Next() {
		f, scanErr := scanSavedQueryFolder(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("postgres: ListSavedQueryFolders scan: %w", scanErr)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// SoftDeleteSavedQueryFolder soft-deletes the folder and clears folder_id on
// any member queries first, since soft-deletes don't trigger the FK's
// ON DELETE SET NULL.
func (d *DB) SoftDeleteSavedQueryFolder(ctx context.Context, id, deletedBy string) error {
	now := time.Now().UTC()
	if _, err := d.db.ExecContext(ctx,
		`UPDATE saved_queries SET folder_id = NULL, updated_at = $1 WHERE folder_id = $2 AND deleted_at IS NULL`,
		now, id,
	); err != nil {
		return wrapErr("SoftDeleteSavedQueryFolder(clear children)", err)
	}
	_, err := d.db.ExecContext(ctx,
		`UPDATE saved_query_folders SET deleted_at = $1, deleted_by = $2 WHERE id = $3 AND deleted_at IS NULL`,
		now, deletedBy, id,
	)
	return wrapErr("SoftDeleteSavedQueryFolder", err)
}

func scanSavedQueryFolder(row interface{ Scan(...any) error }) (catalog.SavedQueryFolder, error) {
	var f catalog.SavedQueryFolder
	err := row.Scan(
		&f.ID, &f.OrgID, &f.ServiceDBID, &f.Scope, &f.OwnerUserID, &f.TeamID, &f.Name,
		&f.CreatedBy, &f.UpdatedBy, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt, &f.DeletedBy,
	)
	return f, err
}

func (d *DB) CreateSavedQuery(ctx context.Context, sq catalog.SavedQuery) error {
	const q = `
		INSERT INTO saved_queries
			(id, org_id, service_db_id, folder_id, scope, owner_user_id, team_id,
			 title, description, query_text, tags, source, source_ref,
			 created_by, updated_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`
	now := time.Now().UTC()
	if sq.CreatedAt.IsZero() {
		sq.CreatedAt = now
	}
	if sq.UpdatedAt.IsZero() {
		sq.UpdatedAt = now
	}
	_, err := d.db.ExecContext(ctx, q,
		sq.ID, sq.OrgID, sq.ServiceDBID, sq.FolderID, sq.Scope, sq.OwnerUserID, sq.TeamID,
		sq.Title, sq.Description, sq.QueryText, pq.Array(nonNilTags(sq.Tags)), sq.Source, sq.SourceRef,
		sq.CreatedBy, sq.UpdatedBy, sq.CreatedAt, sq.UpdatedAt,
	)
	return wrapErr("CreateSavedQuery", err)
}

// nonNilTags avoids sending SQL NULL for a nil slice, which would violate the
// tags NOT NULL constraint (the column's DEFAULT '{}' only applies when the
// column is omitted from the INSERT, not when explicitly bound to NULL).
func nonNilTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}

func (d *DB) GetSavedQuery(ctx context.Context, id string) (*catalog.SavedQuery, error) {
	const q = `
		SELECT id, org_id, service_db_id, folder_id, scope, owner_user_id, team_id,
		       title, description, query_text, tags, source, source_ref,
		       created_by, updated_by, created_at, updated_at, deleted_at, deleted_by
		FROM saved_queries
		WHERE id = $1`
	sq, err := scanSavedQuery(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetSavedQuery: %w", err)
	}
	return &sq, nil
}

func (d *DB) ListSavedQueries(ctx context.Context, serviceDBID string, scope catalog.SavedQueryScope, ownerUserID *string) ([]catalog.SavedQuery, error) {
	const q = `
		SELECT id, org_id, service_db_id, folder_id, scope, owner_user_id, team_id,
		       title, description, query_text, tags, source, source_ref,
		       created_by, updated_by, created_at, updated_at, deleted_at, deleted_by
		FROM saved_queries
		WHERE service_db_id = $1 AND scope = $2 AND deleted_at IS NULL
		  AND ($3::uuid IS NULL OR owner_user_id = $3)
		ORDER BY updated_at DESC`
	rows, err := d.db.QueryContext(ctx, q, serviceDBID, scope, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListSavedQueries: %w", err)
	}
	defer rows.Close()

	var out []catalog.SavedQuery
	for rows.Next() {
		sq, scanErr := scanSavedQuery(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("postgres: ListSavedQueries scan: %w", scanErr)
		}
		out = append(out, sq)
	}
	return out, rows.Err()
}

func (d *DB) UpdateSavedQuery(ctx context.Context, sq catalog.SavedQuery) error {
	const q = `
		UPDATE saved_queries
		SET folder_id=$1, title=$2, description=$3, query_text=$4, tags=$5, updated_by=$6, updated_at=$7
		WHERE id=$8 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q,
		sq.FolderID, sq.Title, sq.Description, sq.QueryText, pq.Array(nonNilTags(sq.Tags)), sq.UpdatedBy, time.Now().UTC(), sq.ID,
	)
	return wrapErr("UpdateSavedQuery", err)
}

func (d *DB) SoftDeleteSavedQuery(ctx context.Context, id, deletedBy string) error {
	const q = `UPDATE saved_queries SET deleted_at=$1, deleted_by=$2 WHERE id=$3 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, id)
	return wrapErr("SoftDeleteSavedQuery", err)
}

// UpsertSavedQueryBySourceRef is the CI/CLI-facing entry point. It performs a
// single INSERT ... ON CONFLICT DO UPDATE against the partial unique index on
// (service_db_id, source_ref), so concurrent syncs of the same query can never
// create duplicate rows. `xmax = 0` is the standard Postgres trick to learn
// whether the RETURNING row came from the INSERT branch or the UPDATE branch
// without a second round-trip.
func (d *DB) UpsertSavedQueryBySourceRef(ctx context.Context, sq catalog.SavedQuery) (catalog.SavedQuery, bool, error) {
	const q = `
		INSERT INTO saved_queries
			(id, org_id, service_db_id, folder_id, scope, owner_user_id, team_id,
			 title, description, query_text, tags, source, source_ref,
			 created_by, updated_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT (service_db_id, source_ref) WHERE source_ref IS NOT NULL AND deleted_at IS NULL
		DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			query_text = EXCLUDED.query_text,
			tags = EXCLUDED.tags,
			folder_id = EXCLUDED.folder_id,
			team_id = EXCLUDED.team_id,
			updated_by = EXCLUDED.updated_by,
			updated_at = EXCLUDED.updated_at
		RETURNING id, org_id, service_db_id, folder_id, scope, owner_user_id, team_id,
		          title, description, query_text, tags, source, source_ref,
		          created_by, updated_by, created_at, updated_at, (xmax = 0) AS was_inserted`
	now := time.Now().UTC()
	if sq.CreatedAt.IsZero() {
		sq.CreatedAt = now
	}
	sq.UpdatedAt = now

	var out catalog.SavedQuery
	var tags pq.StringArray
	var wasInserted bool
	err := d.db.QueryRowContext(ctx, q,
		sq.ID, sq.OrgID, sq.ServiceDBID, sq.FolderID, sq.Scope, sq.OwnerUserID, sq.TeamID,
		sq.Title, sq.Description, sq.QueryText, pq.Array(nonNilTags(sq.Tags)), sq.Source, sq.SourceRef,
		sq.CreatedBy, sq.UpdatedBy, sq.CreatedAt, sq.UpdatedAt,
	).Scan(
		&out.ID, &out.OrgID, &out.ServiceDBID, &out.FolderID, &out.Scope, &out.OwnerUserID, &out.TeamID,
		&out.Title, &out.Description, &out.QueryText, &tags, &out.Source, &out.SourceRef,
		&out.CreatedBy, &out.UpdatedBy, &out.CreatedAt, &out.UpdatedAt, &wasInserted,
	)
	if err != nil {
		return catalog.SavedQuery{}, false, fmt.Errorf("postgres: UpsertSavedQueryBySourceRef: %w", err)
	}
	out.Tags = []string(tags)
	return out, wasInserted, nil
}

func scanSavedQuery(row interface{ Scan(...any) error }) (catalog.SavedQuery, error) {
	var sq catalog.SavedQuery
	var tags pq.StringArray
	err := row.Scan(
		&sq.ID, &sq.OrgID, &sq.ServiceDBID, &sq.FolderID, &sq.Scope, &sq.OwnerUserID, &sq.TeamID,
		&sq.Title, &sq.Description, &sq.QueryText, &tags, &sq.Source, &sq.SourceRef,
		&sq.CreatedBy, &sq.UpdatedBy, &sq.CreatedAt, &sq.UpdatedAt, &sq.DeletedAt, &sq.DeletedBy,
	)
	if err != nil {
		return sq, err
	}
	sq.Tags = []string(tags)
	return sq, nil
}
