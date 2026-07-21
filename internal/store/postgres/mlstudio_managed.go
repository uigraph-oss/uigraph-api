package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/uigraph/app/internal/mlstudio"
)

func (d *DB) CreateMLDeployment(ctx context.Context, dep mlstudio.Deployment) error {
	const q = `
		INSERT INTO ml_deployments (id, org_id, model_id, version_id, name, environment, status, endpoint, region, deployed_at, rolled_back_at, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13)`
	now := time.Now().UTC()
	_, err := d.db.ExecContext(ctx, q,
		dep.ID, dep.OrgID, dep.ModelID, dep.VersionID, dep.Name, dep.Environment, dep.Status,
		dep.Endpoint, dep.Region, dep.DeployedAt, dep.RolledBackAt, dep.CreatedBy, now)
	if err != nil {
		return fmt.Errorf("postgres: CreateMLDeployment: %w", err)
	}
	return nil
}

func scanMLDeployment(row interface{ Scan(...any) error }) (mlstudio.Deployment, error) {
	var dep mlstudio.Deployment
	err := row.Scan(
		&dep.ID, &dep.OrgID, &dep.ModelID, &dep.VersionID, &dep.Name, &dep.Environment, &dep.Status,
		&dep.Endpoint, &dep.Region, &dep.DeployedAt, &dep.RolledBackAt, &dep.CreatedBy,
		&dep.CreatedAt, &dep.UpdatedAt, &dep.DeletedAt,
	)
	return dep, err
}

const mlDeploymentCols = `id, org_id, model_id, version_id, name, environment, status, endpoint, region, deployed_at, rolled_back_at, created_by, created_at, updated_at, deleted_at`

func (d *DB) GetMLDeployment(ctx context.Context, orgID, id string) (*mlstudio.Deployment, error) {
	dep, err := scanMLDeployment(d.db.QueryRowContext(ctx, `SELECT `+mlDeploymentCols+` FROM ml_deployments WHERE org_id=$1 AND id=$2 AND deleted_at IS NULL`, orgID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetMLDeployment: %w", err)
	}
	return &dep, nil
}

func (d *DB) ListMLDeployments(ctx context.Context, orgID, modelID, versionID string) ([]mlstudio.Deployment, error) {
	q := `SELECT ` + mlDeploymentCols + ` FROM ml_deployments WHERE org_id=$1 AND deleted_at IS NULL`
	args := []any{orgID}
	if modelID != "" {
		args = append(args, modelID)
		q += fmt.Sprintf(" AND model_id=$%d", len(args))
	}
	if versionID != "" {
		args = append(args, versionID)
		q += fmt.Sprintf(" AND version_id=$%d", len(args))
	}
	q += " ORDER BY created_at DESC"
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListMLDeployments: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []mlstudio.Deployment
	for rows.Next() {
		dep, err := scanMLDeployment(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListMLDeployments scan: %w", err)
		}
		out = append(out, dep)
	}
	return out, rows.Err()
}

func (d *DB) UpdateMLDeployment(ctx context.Context, dep mlstudio.Deployment) error {
	const q = `
		UPDATE ml_deployments SET
			name=$1, environment=$2, status=$3, endpoint=$4, region=$5, deployed_at=$6, rolled_back_at=$7, updated_at=$8
		WHERE org_id=$9 AND id=$10 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q,
		dep.Name, dep.Environment, dep.Status, dep.Endpoint, dep.Region, dep.DeployedAt, dep.RolledBackAt,
		time.Now().UTC(), dep.OrgID, dep.ID)
	if err != nil {
		return fmt.Errorf("postgres: UpdateMLDeployment: %w", err)
	}
	return nil
}

func (d *DB) DeleteMLDeployment(ctx context.Context, orgID, id, deletedBy string) error {
	const q = `UPDATE ml_deployments SET deleted_at=$1, deleted_by=$2 WHERE org_id=$3 AND id=$4 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, orgID, id)
	if err != nil {
		return fmt.Errorf("postgres: DeleteMLDeployment: %w", err)
	}
	return nil
}

func (d *DB) CreateMLFinding(ctx context.Context, f mlstudio.Finding) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres: CreateMLFinding begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO ml_findings (id, org_id, model_id, version_id, title, summary, description, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)`,
		f.ID, f.OrgID, f.ModelID, f.VersionID, f.Title, f.Summary, f.Description, f.CreatedBy, now)
	if err != nil {
		return fmt.Errorf("postgres: CreateMLFinding: %w", err)
	}
	if err := replaceMLFindingRuns(ctx, tx, f.ID, f.RunIDs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres: CreateMLFinding commit: %w", err)
	}
	return nil
}

func replaceMLFindingRuns(ctx context.Context, tx *sql.Tx, findingID string, runIDs []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM ml_finding_runs WHERE finding_id=$1`, findingID); err != nil {
		return fmt.Errorf("postgres: replaceMLFindingRuns clear: %w", err)
	}
	for _, runID := range runIDs {
		_, err := tx.ExecContext(ctx, `INSERT INTO ml_finding_runs (finding_id, run_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, findingID, runID)
		if err != nil {
			return fmt.Errorf("postgres: replaceMLFindingRuns insert: %w", err)
		}
	}
	return nil
}

func (d *DB) GetMLFinding(ctx context.Context, orgID, id string) (*mlstudio.Finding, error) {
	f, err := scanMLFinding(d.db.QueryRowContext(ctx, `
		SELECT id, org_id, model_id, version_id, title, summary, description, created_by, created_at, updated_at, deleted_at
		FROM ml_findings WHERE org_id=$1 AND id=$2 AND deleted_at IS NULL`, orgID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetMLFinding: %w", err)
	}
	runIDs, err := d.listMLFindingRunIDs(ctx, f.ID)
	if err != nil {
		return nil, err
	}
	f.RunIDs = runIDs
	return &f, nil
}

func scanMLFinding(row interface{ Scan(...any) error }) (mlstudio.Finding, error) {
	var f mlstudio.Finding
	err := row.Scan(
		&f.ID, &f.OrgID, &f.ModelID, &f.VersionID, &f.Title, &f.Summary, &f.Description,
		&f.CreatedBy, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt,
	)
	return f, err
}

func (d *DB) listMLFindingRunIDs(ctx context.Context, findingID string) ([]string, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT run_id FROM ml_finding_runs WHERE finding_id=$1 ORDER BY run_id ASC`, findingID)
	if err != nil {
		return nil, fmt.Errorf("postgres: listMLFindingRunIDs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []string{}
	for rows.Next() {
		var runID string
		if err := rows.Scan(&runID); err != nil {
			return nil, fmt.Errorf("postgres: listMLFindingRunIDs scan: %w", err)
		}
		out = append(out, runID)
	}
	return out, rows.Err()
}

func (d *DB) ListMLFindings(ctx context.Context, orgID, modelID string) ([]mlstudio.Finding, error) {
	q := `
		SELECT id, org_id, model_id, version_id, title, summary, description, created_by, created_at, updated_at, deleted_at
		FROM ml_findings WHERE org_id=$1 AND deleted_at IS NULL`
	args := []any{orgID}
	if modelID != "" {
		args = append(args, modelID)
		q += fmt.Sprintf(" AND model_id=$%d", len(args))
	}
	q += " ORDER BY created_at DESC"
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListMLFindings: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []mlstudio.Finding
	var ids []string
	for rows.Next() {
		f, err := scanMLFinding(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListMLFindings scan: %w", err)
		}
		f.RunIDs = []string{}
		out = append(out, f)
		ids = append(ids, f.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return out, nil
	}
	linkRows, err := d.db.QueryContext(ctx, `SELECT finding_id, run_id FROM ml_finding_runs WHERE finding_id = ANY($1) ORDER BY run_id ASC`, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("postgres: ListMLFindings runs: %w", err)
	}
	defer func() { _ = linkRows.Close() }()
	byFinding := map[string][]string{}
	for linkRows.Next() {
		var findingID, runID string
		if err := linkRows.Scan(&findingID, &runID); err != nil {
			return nil, fmt.Errorf("postgres: ListMLFindings runs scan: %w", err)
		}
		byFinding[findingID] = append(byFinding[findingID], runID)
	}
	if err := linkRows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if runIDs, ok := byFinding[out[i].ID]; ok {
			out[i].RunIDs = runIDs
		}
	}
	return out, nil
}

func (d *DB) UpdateMLFinding(ctx context.Context, f mlstudio.Finding) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres: UpdateMLFinding begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
		UPDATE ml_findings SET version_id=$1, title=$2, summary=$3, description=$4, updated_at=$5
		WHERE org_id=$6 AND id=$7 AND deleted_at IS NULL`,
		f.VersionID, f.Title, f.Summary, f.Description, time.Now().UTC(), f.OrgID, f.ID)
	if err != nil {
		return fmt.Errorf("postgres: UpdateMLFinding: %w", err)
	}
	if err := replaceMLFindingRuns(ctx, tx, f.ID, f.RunIDs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres: UpdateMLFinding commit: %w", err)
	}
	return nil
}

func (d *DB) DeleteMLFinding(ctx context.Context, orgID, id, deletedBy string) error {
	const q = `UPDATE ml_findings SET deleted_at=$1, deleted_by=$2 WHERE org_id=$3 AND id=$4 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, orgID, id)
	if err != nil {
		return fmt.Errorf("postgres: DeleteMLFinding: %w", err)
	}
	return nil
}
