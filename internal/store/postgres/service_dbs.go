package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/uigraph/app/internal/catalog"
	"github.com/uigraph/app/internal/store"
)

func (d *DB) CreateServiceDB(ctx context.Context, sd catalog.ServiceDB) error {
	const q = `
		INSERT INTO service_dbs
			(id, service_id, org_id, db_name, db_type, dialect, schema_json,
			 source, source_ts, schema_token_count, created_by, updated_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`
	now := time.Now().UTC()
	if sd.CreatedAt.IsZero() {
		sd.CreatedAt = now
	}
	if sd.UpdatedAt.IsZero() {
		sd.UpdatedAt = now
	}
	schema := sd.SchemaJSON
	if schema == nil {
		schema = json.RawMessage("{}")
	}
	_, err := d.db.ExecContext(ctx, q,
		sd.ID, sd.ServiceID, sd.OrgID, sd.DBName, sd.DBType, sd.Dialect, schema,
		sd.Source, sd.SourceTS, sd.SchemaTokenCount, sd.CreatedBy, sd.UpdatedBy, sd.CreatedAt, sd.UpdatedAt,
	)
	if uniqueViolation(err, "idx_service_dbs_service_name") {
		return fmt.Errorf("%w: %s", store.ErrDataSourceNameExists, sd.DBName)
	}
	return wrapErr("CreateServiceDB", err)
}

func (d *DB) GetServiceDB(ctx context.Context, id string) (*catalog.ServiceDB, error) {
	const q = `
		SELECT id, service_id, org_id, db_name, db_type, dialect, schema_json,
		       source, source_ts, schema_token_count, created_by, updated_by, created_at, updated_at, deleted_at, deleted_by
		FROM service_dbs
		WHERE id = $1`
	sd, err := scanServiceDB(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetServiceDB: %w", err)
	}
	return &sd, nil
}

func (d *DB) ListServiceDBs(ctx context.Context, serviceID string) ([]catalog.ServiceDB, error) {
	const q = `
		SELECT id, service_id, org_id, db_name, db_type, dialect, schema_json,
		       source, source_ts, schema_token_count, created_by, updated_by, created_at, updated_at, deleted_at, deleted_by
		FROM service_dbs
		WHERE service_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`
	rows, err := d.db.QueryContext(ctx, q, serviceID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListServiceDBs: %w", err)
	}
	defer rows.Close()

	var out []catalog.ServiceDB
	for rows.Next() {
		sd, scanErr := scanServiceDB(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("postgres: ListServiceDBs scan: %w", scanErr)
		}
		out = append(out, sd)
	}
	return out, rows.Err()
}

func (d *DB) UpdateServiceDB(ctx context.Context, sd catalog.ServiceDB) error {
	const q = `
		UPDATE service_dbs
		SET db_name=$1, db_type=$2, dialect=$3, schema_json=$4,
		    source=$5, source_ts=$6, schema_token_count=$7,
		    updated_by=$8, updated_at=$9
		WHERE id=$10 AND deleted_at IS NULL`
	schema := sd.SchemaJSON
	if schema == nil {
		schema = json.RawMessage("{}")
	}
	_, err := d.db.ExecContext(ctx, q,
		sd.DBName, sd.DBType, sd.Dialect, schema,
		sd.Source, sd.SourceTS, sd.SchemaTokenCount,
		sd.UpdatedBy, time.Now().UTC(), sd.ID,
	)
	if uniqueViolation(err, "idx_service_dbs_service_name") {
		return fmt.Errorf("%w: %s", store.ErrDataSourceNameExists, sd.DBName)
	}
	return wrapErr("UpdateServiceDB", err)
}

func (d *DB) SoftDeleteServiceDB(ctx context.Context, id, deletedBy string) error {
	const q = `UPDATE service_dbs SET deleted_at=$1, deleted_by=$2 WHERE id=$3 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, id)
	return wrapErr("SoftDeleteServiceDB", err)
}

func scanServiceDB(row interface{ Scan(...any) error }) (catalog.ServiceDB, error) {
	var sd catalog.ServiceDB
	var schema []byte
	err := row.Scan(
		&sd.ID, &sd.ServiceID, &sd.OrgID, &sd.DBName, &sd.DBType, &sd.Dialect, &schema,
		&sd.Source, &sd.SourceTS, &sd.SchemaTokenCount, &sd.CreatedBy, &sd.UpdatedBy, &sd.CreatedAt, &sd.UpdatedAt, &sd.DeletedAt, &sd.DeletedBy,
	)
	if err != nil {
		return sd, err
	}
	sd.SchemaJSON = schema
	return sd, nil
}

func (d *DB) CreateServiceDBVersion(ctx context.Context, v catalog.ServiceDBVersion) error {
	const q = `
		INSERT INTO service_db_versions
			(id, service_db_id, version_number, label, schema_json, source, source_ts, is_auto_version, created_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now().UTC()
	}
	schema := v.SchemaJSON
	if schema == nil {
		schema = json.RawMessage("{}")
	}
	_, err := d.db.ExecContext(ctx, q,
		v.ID, v.ServiceDBID, v.VersionNumber, v.Label, schema, v.Source, v.SourceTS, v.IsAutoVersion, v.CreatedBy, v.CreatedAt,
	)
	return wrapErr("CreateServiceDBVersion", err)
}

func (d *DB) GetServiceDBVersion(ctx context.Context, id string) (*catalog.ServiceDBVersion, error) {
	const q = `
		SELECT id, service_db_id, version_number, label, schema_json, source, source_ts, is_auto_version, created_by, created_at
		FROM service_db_versions
		WHERE id = $1`
	var v catalog.ServiceDBVersion
	var schema []byte
	err := d.db.QueryRowContext(ctx, q, id).Scan(
		&v.ID, &v.ServiceDBID, &v.VersionNumber, &v.Label, &schema, &v.Source, &v.SourceTS, &v.IsAutoVersion, &v.CreatedBy, &v.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetServiceDBVersion: %w", err)
	}
	v.SchemaJSON = schema
	return &v, nil
}

func (d *DB) ListServiceDBVersions(ctx context.Context, serviceDBID string) ([]catalog.ServiceDBVersion, error) {
	const q = `
		SELECT id, service_db_id, version_number, label, schema_json, source, source_ts, is_auto_version, created_by, created_at
		FROM service_db_versions
		WHERE service_db_id = $1
		ORDER BY version_number DESC`
	rows, err := d.db.QueryContext(ctx, q, serviceDBID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListServiceDBVersions: %w", err)
	}
	defer rows.Close()

	var out []catalog.ServiceDBVersion
	for rows.Next() {
		var v catalog.ServiceDBVersion
		var schema []byte
		if err := rows.Scan(
			&v.ID, &v.ServiceDBID, &v.VersionNumber, &v.Label, &schema, &v.Source, &v.SourceTS, &v.IsAutoVersion, &v.CreatedBy, &v.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres: ListServiceDBVersions scan: %w", err)
		}
		v.SchemaJSON = schema
		out = append(out, v)
	}
	return out, rows.Err()
}

func (d *DB) LatestServiceDBVersionNumber(ctx context.Context, serviceDBID string) (int, error) {
	const q = `SELECT COALESCE(MAX(version_number), 0) FROM service_db_versions WHERE service_db_id = $1`
	var n int
	return n, d.db.QueryRowContext(ctx, q, serviceDBID).Scan(&n)
}
