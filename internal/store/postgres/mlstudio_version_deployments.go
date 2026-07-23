package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/uigraph/app/internal/mlstudio"
)

const mlVersionDeploymentCols = `id, org_id, version_id, from_status, to_status, changed_by, changed_at`

func scanMLVersionDeployment(row interface{ Scan(...any) error }) (mlstudio.VersionDeploymentUpdate, error) {
	var u mlstudio.VersionDeploymentUpdate
	err := row.Scan(&u.ID, &u.OrgID, &u.VersionID, &u.FromStatus, &u.ToStatus, &u.ChangedBy, &u.ChangedAt)
	return u, err
}

func (d *DB) CreateVersionDeploymentUpdate(ctx context.Context, u mlstudio.VersionDeploymentUpdate) error {
	const q = `
		INSERT INTO ml_version_deployments (id, org_id, version_id, from_status, to_status, changed_by, changed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`
	_, err := d.db.ExecContext(ctx, q, u.ID, u.OrgID, u.VersionID, u.FromStatus, u.ToStatus, u.ChangedBy, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("postgres: CreateVersionDeploymentUpdate: %w", err)
	}
	return nil
}

func (d *DB) ListVersionDeploymentUpdates(ctx context.Context, orgID, versionID, projectID string) ([]mlstudio.VersionDeploymentUpdate, error) {
	q := `SELECT ` + mlVersionDeploymentCols + ` FROM ml_version_deployments WHERE org_id=$1`
	args := []any{orgID}
	if versionID != "" {
		args = append(args, versionID)
		q += fmt.Sprintf(" AND version_id=$%d", len(args))
	}
	if projectID != "" {
		args = append(args, projectID)
		q += fmt.Sprintf(" AND version_id IN (SELECT v.id FROM ml_model_versions v JOIN ml_models m ON m.id=v.model_id WHERE m.project_id=$%d)", len(args))
	}
	q += " ORDER BY changed_at DESC, id DESC"
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListVersionDeploymentUpdates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []mlstudio.VersionDeploymentUpdate
	for rows.Next() {
		u, err := scanMLVersionDeployment(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListVersionDeploymentUpdates scan: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
