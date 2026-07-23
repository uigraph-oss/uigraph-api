package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lib/pq"

	"github.com/uigraph/app/internal/mlstudio"
)

func scanMLModel(row interface{ Scan(...any) error }) (mlstudio.Model, error) {
	var m mlstudio.Model
	err := row.Scan(
		&m.ID, &m.OrgID, &m.MLflowID, &m.ProjectID, &m.Name, &m.Description,
		&m.Domain, &m.ProblemType, pq.Array(&m.Tags),
		&m.Owners, &m.License, pq.Array(&m.References), &m.IntendedUse,
		&m.Limitations, &m.EthicalConsiderations, &m.Caveats,
		&m.Recommendations, &m.Considerations,
		&m.ProductionVersionID, &m.CreatedAt, &m.UpdatedAt,
	)
	return m, err
}

const mlModelCols = `id, org_id, mlflow_id, project_id, name, description, domain, problem_type, tags, owners, license, reference_links, intended_use, limitations, ethical_considerations, caveats, recommendations, considerations, production_version_id, mlflow_created_at, updated_at`

func (d *DB) ListMLModels(ctx context.Context, orgID, projectID string) ([]mlstudio.Model, error) {
	q := `SELECT ` + mlModelCols + ` FROM ml_models WHERE org_id=$1 AND deleted_at IS NULL`
	args := []any{orgID}
	if projectID != "" {
		args = append(args, projectID)
		q += fmt.Sprintf(" AND project_id=$%d", len(args))
	}
	q += " ORDER BY name ASC"
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListMLModels: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []mlstudio.Model
	for rows.Next() {
		m, err := scanMLModel(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListMLModels scan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (d *DB) GetMLModel(ctx context.Context, orgID, id string) (*mlstudio.Model, error) {
	m, err := scanMLModel(d.db.QueryRowContext(ctx, `SELECT `+mlModelCols+` FROM ml_models WHERE org_id=$1 AND id=$2 AND deleted_at IS NULL`, orgID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetMLModel: %w", err)
	}
	return &m, nil
}

func scanMLModelVersion(row interface{ Scan(...any) error }) (mlstudio.ModelVersion, error) {
	var v mlstudio.ModelVersion
	err := row.Scan(
		&v.ID, &v.OrgID, &v.MLflowID, &v.ModelID, &v.Version, &v.Description,
		&v.DeploymentStatus, &v.RunID, &v.CreatedAt,
	)
	return v, err
}

const mlVersionCols = `id, org_id, mlflow_id, model_id, version, description, ` +
	`COALESCE((SELECT to_status FROM ml_version_deployments u WHERE u.version_id = ml_model_versions.id ORDER BY changed_at DESC, id DESC LIMIT 1), 'candidate') AS deployment_status, ` +
	`run_id, mlflow_created_at`

func (d *DB) ListMLModelVersions(ctx context.Context, orgID, modelID, projectID string) ([]mlstudio.ModelVersion, error) {
	q := `SELECT ` + mlVersionCols + ` FROM ml_model_versions WHERE org_id=$1 AND deleted_at IS NULL`
	args := []any{orgID}
	if modelID != "" {
		args = append(args, modelID)
		q += fmt.Sprintf(" AND model_id=$%d", len(args))
	}
	if projectID != "" {
		args = append(args, projectID)
		q += fmt.Sprintf(" AND model_id IN (SELECT id FROM ml_models WHERE project_id=$%d)", len(args))
	}
	q += " ORDER BY version ASC"
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListMLModelVersions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []mlstudio.ModelVersion
	for rows.Next() {
		v, err := scanMLModelVersion(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListMLModelVersions scan: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (d *DB) GetMLModelVersion(ctx context.Context, orgID, id string) (*mlstudio.ModelVersion, error) {
	v, err := scanMLModelVersion(d.db.QueryRowContext(ctx, `SELECT `+mlVersionCols+` FROM ml_model_versions WHERE org_id=$1 AND id=$2 AND deleted_at IS NULL`, orgID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetMLModelVersion: %w", err)
	}
	return &v, nil
}

func scanMLExperiment(row interface{ Scan(...any) error }) (mlstudio.Experiment, error) {
	var e mlstudio.Experiment
	err := row.Scan(
		&e.ID, &e.OrgID, &e.MLflowID, &e.ProjectID, &e.Name, &e.Description, &e.Status, &e.StartedAt,
	)
	return e, err
}

const mlExperimentCols = `id, org_id, mlflow_id, project_id, name, description, status, started_at`

func scanMLProject(row interface{ Scan(...any) error }) (mlstudio.Project, error) {
	var p mlstudio.Project
	err := row.Scan(
		&p.ID, &p.OrgID, &p.Name, &p.Type, &p.Description, &p.SourceType, &p.SourceURL, &p.TeamID,
	)
	return p, err
}

const mlProjectCols = `id, org_id, name, type, description, source_type, source_url, team_id`

func (d *DB) ListMLProjects(ctx context.Context, orgID string) ([]mlstudio.Project, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT `+mlProjectCols+`,
			(SELECT COUNT(*) FROM ml_models m WHERE m.project_id = ml_projects.id AND m.deleted_at IS NULL) AS model_count,
			(SELECT COUNT(*) FROM ml_experiments e WHERE e.project_id = ml_projects.id AND e.deleted_at IS NULL) AS experiment_count,
			(SELECT COUNT(*) FROM ml_runs r JOIN ml_experiments e ON e.id = r.experiment_id WHERE e.project_id = ml_projects.id AND r.deleted_at IS NULL) AS run_count
		FROM ml_projects WHERE org_id=$1 AND deleted_at IS NULL ORDER BY name ASC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListMLProjects: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []mlstudio.Project
	for rows.Next() {
		var p mlstudio.Project
		var stats mlstudio.ProjectStats
		if err := rows.Scan(
			&p.ID, &p.OrgID, &p.Name, &p.Type, &p.Description, &p.SourceType, &p.SourceURL, &p.TeamID,
			&stats.ModelCount, &stats.ExperimentCount, &stats.RunCount,
		); err != nil {
			return nil, fmt.Errorf("postgres: ListMLProjects scan: %w", err)
		}
		p.Stats = &stats
		out = append(out, p)
	}
	return out, rows.Err()
}

func (d *DB) GetMLProject(ctx context.Context, orgID, id string) (*mlstudio.Project, error) {
	p, err := scanMLProject(d.db.QueryRowContext(ctx, `SELECT `+mlProjectCols+` FROM ml_projects WHERE org_id=$1 AND id=$2 AND deleted_at IS NULL`, orgID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetMLProject: %w", err)
	}
	return &p, nil
}

func (d *DB) ListMLExperiments(ctx context.Context, orgID, projectID string) ([]mlstudio.Experiment, error) {
	q := `SELECT ` + mlExperimentCols + ` FROM ml_experiments WHERE org_id=$1 AND deleted_at IS NULL`
	args := []any{orgID}
	if projectID != "" {
		args = append(args, projectID)
		q += fmt.Sprintf(" AND project_id=$%d", len(args))
	}
	q += " ORDER BY started_at DESC NULLS LAST, name ASC"
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListMLExperiments: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []mlstudio.Experiment
	for rows.Next() {
		e, err := scanMLExperiment(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListMLExperiments scan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *DB) GetMLExperiment(ctx context.Context, orgID, id string) (*mlstudio.Experiment, error) {
	e, err := scanMLExperiment(d.db.QueryRowContext(ctx, `SELECT `+mlExperimentCols+` FROM ml_experiments WHERE org_id=$1 AND id=$2 AND deleted_at IS NULL`, orgID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetMLExperiment: %w", err)
	}
	return &e, nil
}

func scanMLRun(row interface{ Scan(...any) error }) (mlstudio.Run, error) {
	var run mlstudio.Run
	var params, metrics []byte
	err := row.Scan(
		&run.ID, &run.OrgID, &run.MLflowID, &run.ExperimentID, &run.Name, &run.Status,
		&run.StartedAt, &run.EndedAt, &run.Duration, &run.Notes,
		&params, &metrics, &run.DatasetID, &run.UpdatedAt, &run.SyncedAt,
	)
	if err != nil {
		return run, err
	}
	if err := json.Unmarshal(params, &run.Parameters); err != nil {
		return run, err
	}
	if err := json.Unmarshal(metrics, &run.Metrics); err != nil {
		return run, err
	}
	return run, nil
}

const mlRunCols = `id, org_id, mlflow_id, experiment_id, name, status, started_at, ended_at, duration, notes, parameters, metrics, dataset_id, updated_at, synced_at`

func (d *DB) ListMLRuns(ctx context.Context, orgID, experimentID, projectID string) ([]mlstudio.Run, error) {
	q := `SELECT ` + mlRunCols + ` FROM ml_runs WHERE org_id=$1 AND deleted_at IS NULL`
	args := []any{orgID}
	if experimentID != "" {
		args = append(args, experimentID)
		q += fmt.Sprintf(" AND experiment_id=$%d", len(args))
	}
	if projectID != "" {
		args = append(args, projectID)
		q += fmt.Sprintf(" AND experiment_id IN (SELECT id FROM ml_experiments WHERE project_id=$%d)", len(args))
	}
	q += " ORDER BY started_at DESC NULLS LAST"
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListMLRuns: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []mlstudio.Run
	for rows.Next() {
		run, err := scanMLRun(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListMLRuns scan: %w", err)
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

func (d *DB) GetMLRun(ctx context.Context, orgID, id string) (*mlstudio.Run, error) {
	run, err := scanMLRun(d.db.QueryRowContext(ctx, `SELECT `+mlRunCols+` FROM ml_runs WHERE org_id=$1 AND id=$2 AND deleted_at IS NULL`, orgID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetMLRun: %w", err)
	}
	return &run, nil
}

func (d *DB) ListMLRunMetricPoints(ctx context.Context, orgID, runID string) ([]mlstudio.MetricPoint, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT p.key, p.step, p.value, p.ts
		FROM ml_run_metric_points p
		JOIN ml_runs r ON r.id = p.run_id
		WHERE r.org_id=$1 AND p.run_id=$2 AND r.deleted_at IS NULL
		ORDER BY p.key ASC, p.step ASC`, orgID, runID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListMLRunMetricPoints: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []mlstudio.MetricPoint
	for rows.Next() {
		var p mlstudio.MetricPoint
		if err := rows.Scan(&p.Key, &p.Step, &p.Value, &p.TS); err != nil {
			return nil, fmt.Errorf("postgres: ListMLRunMetricPoints scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanMLArtifact(row interface{ Scan(...any) error }) (mlstudio.Artifact, error) {
	var a mlstudio.Artifact
	err := row.Scan(
		&a.ID, &a.OrgID, &a.MLflowID, &a.RunID, &a.Name, &a.Type,
		&a.URI, &a.Size, &a.Format, &a.UpdatedAt, &a.SyncedAt,
	)
	return a, err
}

func (d *DB) ListMLArtifacts(ctx context.Context, orgID, runID string) ([]mlstudio.Artifact, error) {
	q := `SELECT id, org_id, mlflow_id, run_id, name, type, uri, size, format, updated_at, synced_at
		FROM ml_artifacts WHERE org_id=$1 AND deleted_at IS NULL`
	args := []any{orgID}
	if runID != "" {
		args = append(args, runID)
		q += fmt.Sprintf(" AND run_id=$%d", len(args))
	}
	q += " ORDER BY name ASC"
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListMLArtifacts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []mlstudio.Artifact
	for rows.Next() {
		a, err := scanMLArtifact(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListMLArtifacts scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func scanMLDataset(row interface{ Scan(...any) error }) (mlstudio.Dataset, error) {
	var ds mlstudio.Dataset
	var schema, tags []byte
	err := row.Scan(
		&ds.ID, &ds.OrgID, &ds.ExperimentID, &ds.MLflowID, &ds.Name, &ds.Digest,
		&ds.Source, &ds.SourceType, &ds.Context, &ds.RowCount, &schema, &tags,
	)
	if err != nil {
		return ds, err
	}
	if err := json.Unmarshal(schema, &ds.Schema); err != nil {
		return ds, err
	}
	if err := json.Unmarshal(tags, &ds.Tags); err != nil {
		return ds, err
	}
	return ds, nil
}

const mlDatasetCols = `id, org_id, experiment_id, mlflow_id, name, digest, source, source_type, context, row_count, schema, tags`

func (d *DB) ListMLDatasets(ctx context.Context, orgID, experimentID string) ([]mlstudio.Dataset, error) {
	q := `SELECT ` + mlDatasetCols + ` FROM ml_datasets WHERE org_id=$1 AND deleted_at IS NULL`
	args := []any{orgID}
	if experimentID != "" {
		args = append(args, experimentID)
		q += fmt.Sprintf(" AND experiment_id=$%d", len(args))
	}
	q += " ORDER BY name ASC"
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListMLDatasets: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []mlstudio.Dataset
	for rows.Next() {
		ds, err := scanMLDataset(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListMLDatasets scan: %w", err)
		}
		out = append(out, ds)
	}
	return out, rows.Err()
}

func (d *DB) GetMLDataset(ctx context.Context, orgID, id string) (*mlstudio.Dataset, error) {
	ds, err := scanMLDataset(d.db.QueryRowContext(ctx, `SELECT `+mlDatasetCols+` FROM ml_datasets WHERE org_id=$1 AND id=$2 AND deleted_at IS NULL`, orgID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetMLDataset: %w", err)
	}
	return &ds, nil
}

func (d *DB) ListMLVersionEvaluations(ctx context.Context, orgID, versionID string) ([]mlstudio.Evaluation, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, org_id, mlflow_id, version_id, dataset_id, name, type, description, summary, evaluated_at, evaluator
		FROM ml_evaluations WHERE org_id=$1 AND version_id=$2 AND deleted_at IS NULL ORDER BY evaluated_at DESC NULLS LAST`, orgID, versionID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListMLVersionEvaluations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []mlstudio.Evaluation
	for rows.Next() {
		var e mlstudio.Evaluation
		err := rows.Scan(
			&e.ID, &e.OrgID, &e.MLflowID, &e.VersionID, &e.DatasetID, &e.Name, &e.Type,
			&e.Description, &e.Summary, &e.EvaluatedAt, &e.Evaluator,
		)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListMLVersionEvaluations scan: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		metrics, err := d.listMLEvaluationMetrics(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Metrics = metrics
	}
	return out, nil
}

func (d *DB) listMLEvaluationMetrics(ctx context.Context, evaluationID string) ([]mlstudio.Metric, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, name, value, unit, direction, category, measured_at
		FROM ml_evaluation_metrics WHERE evaluation_id=$1 ORDER BY name ASC`, evaluationID)
	if err != nil {
		return nil, fmt.Errorf("postgres: listMLEvaluationMetrics: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []mlstudio.Metric
	for rows.Next() {
		var m mlstudio.Metric
		if err := rows.Scan(&m.ID, &m.Name, &m.Value, &m.Unit, &m.Direction, &m.Category, &m.MeasuredAt); err != nil {
			return nil, fmt.Errorf("postgres: listMLEvaluationMetrics scan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
