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

func (d *DB) ListMLFindings(ctx context.Context, orgID, modelID, projectID string) ([]mlstudio.Finding, error) {
	q := `
		SELECT id, org_id, model_id, version_id, title, summary, description, created_by, created_at, updated_at, deleted_at
		FROM ml_findings WHERE org_id=$1 AND deleted_at IS NULL`
	args := []any{orgID}
	if modelID != "" {
		args = append(args, modelID)
		q += fmt.Sprintf(" AND model_id=$%d", len(args))
	}
	if projectID != "" {
		args = append(args, projectID)
		q += fmt.Sprintf(" AND model_id IN (SELECT id FROM ml_models WHERE project_id=$%d)", len(args))
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

func (d *DB) CreateMLModel(ctx context.Context, m mlstudio.Model) error {
	tags := m.Tags
	if tags == nil {
		tags = []string{}
	}
	now := time.Now().UTC()
	const q = `
		INSERT INTO ml_models (id, org_id, project_id, name, description, domain, problem_type, tags, origin, mlflow_created_at, created_by, updated_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11,$10,$10)`
	_, err := d.db.ExecContext(ctx, q,
		m.ID, m.OrgID, m.ProjectID, m.Name, m.Description, m.Domain, m.ProblemType, pq.Array(tags), m.Origin, now, m.CreatedBy)
	if err != nil {
		return fmt.Errorf("postgres: CreateMLModel: %w", err)
	}
	return nil
}

func (d *DB) UpdateMLModelInfo(ctx context.Context, m mlstudio.Model) error {
	tags := m.Tags
	if tags == nil {
		tags = []string{}
	}
	const q = `
		UPDATE ml_models SET name=$1, description=$2, domain=$3, problem_type=$4, tags=$5, updated_at=$6
		WHERE org_id=$7 AND id=$8 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q,
		m.Name, m.Description, m.Domain, m.ProblemType, pq.Array(tags), time.Now().UTC(), m.OrgID, m.ID)
	if err != nil {
		return fmt.Errorf("postgres: UpdateMLModelInfo: %w", err)
	}
	return nil
}

func (d *DB) DeleteMLModel(ctx context.Context, orgID, id, deletedBy string) error {
	const q = `UPDATE ml_models SET deleted_at=$1, deleted_by=$2 WHERE org_id=$3 AND id=$4 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, orgID, id)
	if err != nil {
		return fmt.Errorf("postgres: DeleteMLModel: %w", err)
	}
	return nil
}

func (d *DB) CreateMLExperiment(ctx context.Context, e mlstudio.Experiment) error {
	tags := e.Tags
	if tags == nil {
		tags = []string{}
	}
	const q = `
		INSERT INTO ml_experiments (id, org_id, project_id, name, description, status, tags, started_at, source, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)`
	now := time.Now().UTC()
	_, err := d.db.ExecContext(ctx, q,
		e.ID, e.OrgID, e.ProjectID, e.Name, e.Description, e.Status, pq.Array(tags), e.StartedAt, e.Source, e.CreatedBy, now)
	if err != nil {
		return fmt.Errorf("postgres: CreateMLExperiment: %w", err)
	}
	return nil
}

func (d *DB) UpdateMLExperiment(ctx context.Context, e mlstudio.Experiment) error {
	tags := e.Tags
	if tags == nil {
		tags = []string{}
	}
	const q = `
		UPDATE ml_experiments SET project_id=$1, name=$2, description=$3, status=$4, tags=$5, started_at=$6, updated_at=$7
		WHERE org_id=$8 AND id=$9 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q,
		e.ProjectID, e.Name, e.Description, e.Status, pq.Array(tags), e.StartedAt, time.Now().UTC(), e.OrgID, e.ID)
	if err != nil {
		return fmt.Errorf("postgres: UpdateMLExperiment: %w", err)
	}
	return nil
}

func (d *DB) DeleteMLExperiment(ctx context.Context, orgID, id, deletedBy string) error {
	const q = `UPDATE ml_experiments SET deleted_at=$1, deleted_by=$2 WHERE org_id=$3 AND id=$4 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, orgID, id)
	if err != nil {
		return fmt.Errorf("postgres: DeleteMLExperiment: %w", err)
	}
	return nil
}

func (d *DB) CreateMLRun(ctx context.Context, run mlstudio.Run) error {
	params := run.Parameters
	if params == nil {
		params = map[string]any{}
	}
	metrics := run.Metrics
	if metrics == nil {
		metrics = map[string]any{}
	}
	paramsJSON, err := jsonBytes(params)
	if err != nil {
		return fmt.Errorf("postgres: CreateMLRun marshal parameters: %w", err)
	}
	metricsJSON, err := jsonBytes(metrics)
	if err != nil {
		return fmt.Errorf("postgres: CreateMLRun marshal metrics: %w", err)
	}
	const q = `
		INSERT INTO ml_runs (id, org_id, experiment_id, name, status, started_at, ended_at, notes, parameters, metrics, dataset_id, source, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$14)`
	now := time.Now().UTC()
	_, err = d.db.ExecContext(ctx, q,
		run.ID, run.OrgID, run.ExperimentID, run.Name, run.Status, run.StartedAt, run.EndedAt,
		run.Notes, paramsJSON, metricsJSON, run.DatasetID, run.Source, run.CreatedBy, now)
	if err != nil {
		return fmt.Errorf("postgres: CreateMLRun: %w", err)
	}
	return nil
}

func (d *DB) UpdateMLRun(ctx context.Context, run mlstudio.Run) error {
	params := run.Parameters
	if params == nil {
		params = map[string]any{}
	}
	metrics := run.Metrics
	if metrics == nil {
		metrics = map[string]any{}
	}
	paramsJSON, err := jsonBytes(params)
	if err != nil {
		return fmt.Errorf("postgres: UpdateMLRun marshal parameters: %w", err)
	}
	metricsJSON, err := jsonBytes(metrics)
	if err != nil {
		return fmt.Errorf("postgres: UpdateMLRun marshal metrics: %w", err)
	}
	const q = `
		UPDATE ml_runs SET
			name=$1, status=$2, started_at=$3, ended_at=$4, notes=$5,
			parameters=$6, metrics=$7, dataset_id=$8, updated_at=$9
		WHERE org_id=$10 AND id=$11 AND deleted_at IS NULL`
	_, err = d.db.ExecContext(ctx, q,
		run.Name, run.Status, run.StartedAt, run.EndedAt, run.Notes,
		paramsJSON, metricsJSON, run.DatasetID, time.Now().UTC(), run.OrgID, run.ID)
	if err != nil {
		return fmt.Errorf("postgres: UpdateMLRun: %w", err)
	}
	return nil
}

func (d *DB) DeleteMLRun(ctx context.Context, orgID, id, deletedBy string) error {
	const q = `UPDATE ml_runs SET deleted_at=$1, deleted_by=$2 WHERE org_id=$3 AND id=$4 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, orgID, id)
	if err != nil {
		return fmt.Errorf("postgres: DeleteMLRun: %w", err)
	}
	return nil
}

func (d *DB) CreateMLDataset(ctx context.Context, ds mlstudio.Dataset) error {
	schema := ds.Schema
	if schema == nil {
		schema = []mlstudio.SchemaField{}
	}
	tags := ds.Tags
	if tags == nil {
		tags = map[string]string{}
	}
	schemaJSON, err := jsonBytes(schema)
	if err != nil {
		return fmt.Errorf("postgres: CreateMLDataset marshal schema: %w", err)
	}
	tagsJSON, err := jsonBytes(tags)
	if err != nil {
		return fmt.Errorf("postgres: CreateMLDataset marshal tags: %w", err)
	}
	const q = `
		INSERT INTO ml_datasets (id, org_id, experiment_id, name, digest, source, source_type, context, row_count, schema, tags, origin, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$14)`
	now := time.Now().UTC()
	_, err = d.db.ExecContext(ctx, q,
		ds.ID, ds.OrgID, ds.ExperimentID, ds.Name, ds.Digest, ds.Source, ds.SourceType, ds.Context,
		ds.RowCount, schemaJSON, tagsJSON, ds.Origin, ds.CreatedBy, now)
	if err != nil {
		return fmt.Errorf("postgres: CreateMLDataset: %w", err)
	}
	return nil
}

func (d *DB) UpdateMLDataset(ctx context.Context, ds mlstudio.Dataset) error {
	schema := ds.Schema
	if schema == nil {
		schema = []mlstudio.SchemaField{}
	}
	tags := ds.Tags
	if tags == nil {
		tags = map[string]string{}
	}
	schemaJSON, err := jsonBytes(schema)
	if err != nil {
		return fmt.Errorf("postgres: UpdateMLDataset marshal schema: %w", err)
	}
	tagsJSON, err := jsonBytes(tags)
	if err != nil {
		return fmt.Errorf("postgres: UpdateMLDataset marshal tags: %w", err)
	}
	const q = `
		UPDATE ml_datasets SET
			name=$1, digest=$2, source=$3, source_type=$4, context=$5, row_count=$6, schema=$7, tags=$8, updated_at=$9
		WHERE org_id=$10 AND id=$11 AND deleted_at IS NULL`
	_, err = d.db.ExecContext(ctx, q,
		ds.Name, ds.Digest, ds.Source, ds.SourceType, ds.Context, ds.RowCount, schemaJSON, tagsJSON,
		time.Now().UTC(), ds.OrgID, ds.ID)
	if err != nil {
		return fmt.Errorf("postgres: UpdateMLDataset: %w", err)
	}
	return nil
}

func (d *DB) DeleteMLDataset(ctx context.Context, orgID, id, deletedBy string) error {
	const q = `UPDATE ml_datasets SET deleted_at=$1, deleted_by=$2 WHERE org_id=$3 AND id=$4 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, orgID, id)
	if err != nil {
		return fmt.Errorf("postgres: DeleteMLDataset: %w", err)
	}
	return nil
}

func (d *DB) ReplaceMLRunMetricPoints(ctx context.Context, orgID, runID string, points []mlstudio.MetricPoint) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres: ReplaceMLRunMetricPoints begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM ml_run_metric_points WHERE run_id IN (SELECT id FROM ml_runs WHERE org_id=$1 AND id=$2)`, orgID, runID); err != nil {
		return fmt.Errorf("postgres: ReplaceMLRunMetricPoints clear: %w", err)
	}
	for _, p := range points {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO ml_run_metric_points (run_id, key, step, value, ts) VALUES ($1,$2,$3,$4,$5)`,
			runID, p.Key, p.Step, p.Value, p.TS); err != nil {
			return fmt.Errorf("postgres: ReplaceMLRunMetricPoints insert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres: ReplaceMLRunMetricPoints commit: %w", err)
	}
	return nil
}
