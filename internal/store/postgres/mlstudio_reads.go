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
		&m.License, pq.Array(&m.References), &m.IntendedUse,
		&m.Limitations, &m.Considerations, &m.Recommendations,
		&m.ProductionVersionID, &m.Origin, &m.CreatedAt, &m.UpdatedAt,
	)
	return m, err
}

const mlModelCols = `id, org_id, mlflow_id, project_id, name, description, domain, problem_type, tags, license, reference_links, intended_use, limitations, considerations, recommendations, production_version_id, origin, mlflow_created_at, updated_at`

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
		&v.DeploymentStatus, &v.RunID, &v.Source, &v.CreatedAt,
	)
	return v, err
}

const mlVersionCols = `id, org_id, mlflow_id, model_id, version, description, ` +
	`COALESCE((SELECT to_status FROM ml_version_deployments u WHERE u.version_id = ml_model_versions.id ORDER BY changed_at DESC, id DESC LIMIT 1), 'candidate') AS deployment_status, ` +
	`run_id, source, COALESCE(mlflow_created_at, created_at)`

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
		&e.ID, &e.OrgID, &e.MLflowID, &e.ProjectID, &e.Name, &e.Description, &e.Status, pq.Array(&e.Tags), &e.StartedAt,
		&e.Source, &e.CreatedBy, &e.CreatedAt, &e.UpdatedAt,
	)
	return e, err
}

const mlExperimentCols = `id, org_id, mlflow_id, project_id, name, description, status, tags, started_at, source, created_by, created_at, updated_at`

func scanMLProject(row interface{ Scan(...any) error }) (mlstudio.Project, error) {
	var p mlstudio.Project
	err := row.Scan(
		&p.ID, &p.OrgID, &p.Name, &p.Type, &p.Description, &p.SourceType, &p.SourceURL, &p.TeamID, &p.UpdatedAt,
	)
	return p, err
}

const mlProjectCols = `id, org_id, name, type, description, source_type, source_url, team_id, updated_at`

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
			&p.ID, &p.OrgID, &p.Name, &p.Type, &p.Description, &p.SourceType, &p.SourceURL, &p.TeamID, &p.UpdatedAt,
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
	err := row.Scan(
		&run.ID, &run.OrgID, &run.MLflowID, &run.ExperimentID, &run.Name, &run.Status,
		&run.StartedAt, &run.EndedAt, &run.Notes,
		&run.DatasetID, &run.UpdatedAt, &run.SyncedAt,
		&run.Source, &run.CreatedBy, &run.CreatedAt,
	)
	if err != nil {
		return run, err
	}
	return run, nil
}

const mlRunCols = `id, org_id, mlflow_id, experiment_id, name, status, started_at, ended_at, notes, dataset_id, updated_at, synced_at, source, created_by, created_at`

func (d *DB) attachMLRunValues(ctx context.Context, runs []mlstudio.Run) error {
	ids := make([]string, 0, len(runs))
	for _, run := range runs {
		ids = append(ids, run.ID)
	}
	params, err := loadMLParams(ctx, d.db, "ml_runs_params", "run_id", ids)
	if err != nil {
		return err
	}
	metrics, err := loadMLMetrics(ctx, d.db, "ml_runs_metrics", "run_id", ids)
	if err != nil {
		return err
	}
	for i := range runs {
		runs[i].Parameters = map[string]any{}
		if v, ok := params[runs[i].ID]; ok {
			runs[i].Parameters = v
		}
		runs[i].Metrics = map[string]any{}
		if v, ok := metrics[runs[i].ID]; ok {
			runs[i].Metrics = v
		}
	}
	return nil
}

func (d *DB) ListMLRuns(ctx context.Context, orgID string, q mlstudio.RunQuery) ([]mlstudio.Run, int, error) {
	where := ` FROM ml_runs WHERE org_id=$1 AND deleted_at IS NULL`
	args := []any{orgID}
	if q.ExperimentID != "" {
		args = append(args, q.ExperimentID)
		where += fmt.Sprintf(" AND experiment_id=$%d", len(args))
	}
	if q.ProjectID != "" {
		args = append(args, q.ProjectID)
		where += fmt.Sprintf(" AND experiment_id IN (SELECT id FROM ml_experiments WHERE project_id=$%d)", len(args))
	}
	if q.Search != "" {
		args = append(args, "%"+q.Search+"%")
		where += fmt.Sprintf(" AND name ILIKE $%d", len(args))
	}

	var total int
	if err := d.db.QueryRowContext(ctx, `SELECT COUNT(*)`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres: ListMLRuns count: %w", err)
	}

	sel := `SELECT ` + mlRunCols + where + ` ORDER BY started_at DESC NULLS LAST`
	if q.Limit > 0 {
		args = append(args, q.Limit)
		sel += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if q.Offset > 0 {
		args = append(args, q.Offset)
		sel += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := d.db.QueryContext(ctx, sel, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: ListMLRuns: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []mlstudio.Run
	for rows.Next() {
		run, err := scanMLRun(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("postgres: ListMLRuns scan: %w", err)
		}
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := d.attachMLRunValues(ctx, out); err != nil {
		return nil, 0, fmt.Errorf("postgres: ListMLRuns values: %w", err)
	}
	return out, total, nil
}

func (d *DB) GetMLRun(ctx context.Context, orgID, id string) (*mlstudio.Run, error) {
	run, err := scanMLRun(d.db.QueryRowContext(ctx, `SELECT `+mlRunCols+` FROM ml_runs WHERE org_id=$1 AND id=$2 AND deleted_at IS NULL`, orgID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetMLRun: %w", err)
	}
	runs := []mlstudio.Run{run}
	if err := d.attachMLRunValues(ctx, runs); err != nil {
		return nil, fmt.Errorf("postgres: GetMLRun values: %w", err)
	}
	return &runs[0], nil
}

func scanMLArtifact(row interface{ Scan(...any) error }) (mlstudio.Artifact, error) {
	var a mlstudio.Artifact
	err := row.Scan(
		&a.ID, &a.OrgID, &a.MLflowID, &a.RunID, &a.Name, &a.Type,
		&a.URI, &a.DownloadURI, &a.Size, &a.Format, &a.UpdatedAt, &a.SyncedAt,
	)
	return a, err
}

func (d *DB) ListMLArtifacts(ctx context.Context, orgID, runID string) ([]mlstudio.Artifact, error) {
	q := `SELECT id, org_id, mlflow_id, run_id, name, type, uri, download_uri, size, format, updated_at, synced_at
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
		&ds.Origin, &ds.CreatedBy, &ds.CreatedAt, &ds.UpdatedAt,
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

const mlDatasetCols = `id, org_id, experiment_id, mlflow_id, name, digest, source, source_type, context, row_count, schema, tags, origin, created_by, created_at, updated_at`

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

const mlEvaluationCols = `
	e.id, e.org_id, e.mlflow_id, e.version_id, e.experiment_id, m.name, v.version,
	e.dataset_id, e.name, e.type, e.description, e.summary, e.evaluated_at, e.evaluator,
	e.source, e.created_by`

const mlEvaluationFrom = `
	FROM ml_evaluations e
	JOIN ml_model_versions v ON v.id = e.version_id
	JOIN ml_models m ON m.id = v.model_id`

const mlEvaluationSelect = `SELECT ` + mlEvaluationCols + mlEvaluationFrom

func scanMLEvaluation(row interface{ Scan(...any) error }) (mlstudio.Evaluation, error) {
	var e mlstudio.Evaluation
	err := row.Scan(
		&e.ID, &e.OrgID, &e.MLflowID, &e.VersionID, &e.ExperimentID, &e.ModelName, &e.Version,
		&e.DatasetID, &e.Name, &e.Type, &e.Description, &e.Summary, &e.EvaluatedAt, &e.Evaluator,
		&e.Source, &e.CreatedBy,
	)
	if err != nil {
		return e, err
	}
	return e, nil
}

func (d *DB) attachMLEvaluationValues(ctx context.Context, evaluations []mlstudio.Evaluation) error {
	ids := make([]string, 0, len(evaluations))
	for _, e := range evaluations {
		ids = append(ids, e.ID)
	}
	params, err := loadMLParams(ctx, d.db, "ml_evaluations_params", "eval_id", ids)
	if err != nil {
		return err
	}
	metrics, err := loadMLMetrics(ctx, d.db, "ml_evaluations_metrics", "eval_id", ids)
	if err != nil {
		return err
	}
	for i := range evaluations {
		evaluations[i].Parameters = map[string]any{}
		if v, ok := params[evaluations[i].ID]; ok {
			evaluations[i].Parameters = v
		}
		evaluations[i].Metrics = map[string]any{}
		if v, ok := metrics[evaluations[i].ID]; ok {
			evaluations[i].Metrics = v
		}
	}
	return nil
}

func (d *DB) ListMLEvaluations(ctx context.Context, orgID string, q mlstudio.EvaluationQuery) ([]mlstudio.Evaluation, int, error) {
	where := mlEvaluationFrom + ` WHERE e.org_id=$1 AND e.deleted_at IS NULL`
	args := []any{orgID}
	if q.ExperimentID != "" {
		args = append(args, q.ExperimentID)
		where += fmt.Sprintf(" AND e.experiment_id=$%d", len(args))
	}
	if q.ProjectID != "" {
		args = append(args, q.ProjectID)
		where += fmt.Sprintf(" AND e.experiment_id IN (SELECT id FROM ml_experiments WHERE project_id=$%d)", len(args))
	}
	if q.Search != "" {
		args = append(args, "%"+q.Search+"%")
		where += fmt.Sprintf(" AND e.name ILIKE $%d", len(args))
	}

	var total int
	if err := d.db.QueryRowContext(ctx, `SELECT COUNT(*)`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres: ListMLEvaluations count: %w", err)
	}

	sel := `SELECT ` + mlEvaluationCols + where + ` ORDER BY e.evaluated_at DESC NULLS LAST`
	if q.Limit > 0 {
		args = append(args, q.Limit)
		sel += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if q.Offset > 0 {
		args = append(args, q.Offset)
		sel += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := d.db.QueryContext(ctx, sel, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: ListMLEvaluations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []mlstudio.Evaluation
	for rows.Next() {
		e, err := scanMLEvaluation(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("postgres: ListMLEvaluations scan: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := d.attachMLEvaluationValues(ctx, out); err != nil {
		return nil, 0, fmt.Errorf("postgres: ListMLEvaluations values: %w", err)
	}
	return out, total, nil
}

func (d *DB) ListMLVersionEvaluations(ctx context.Context, orgID, versionID string, q mlstudio.EvaluationQuery) ([]mlstudio.Evaluation, int, error) {
	return d.listMLEvaluations(ctx, "ListMLVersionEvaluations", "e.version_id", orgID, versionID, q)
}

func (d *DB) ListMLExperimentEvaluations(ctx context.Context, orgID, experimentID string, q mlstudio.EvaluationQuery) ([]mlstudio.Evaluation, int, error) {
	return d.listMLEvaluations(ctx, "ListMLExperimentEvaluations", "e.experiment_id", orgID, experimentID, q)
}

func (d *DB) listMLEvaluations(ctx context.Context, label, scopeCol, orgID, scopeID string, q mlstudio.EvaluationQuery) ([]mlstudio.Evaluation, int, error) {
	where := mlEvaluationFrom + ` WHERE e.org_id=$1 AND ` + scopeCol + `=$2 AND e.deleted_at IS NULL`
	args := []any{orgID, scopeID}
	if q.Search != "" {
		args = append(args, "%"+q.Search+"%")
		where += fmt.Sprintf(" AND e.name ILIKE $%d", len(args))
	}

	var total int
	if err := d.db.QueryRowContext(ctx, `SELECT COUNT(*)`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres: %s count: %w", label, err)
	}

	sel := `SELECT ` + mlEvaluationCols + where + ` ORDER BY e.evaluated_at DESC NULLS LAST`
	if q.Limit > 0 {
		args = append(args, q.Limit)
		sel += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if q.Offset > 0 {
		args = append(args, q.Offset)
		sel += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := d.db.QueryContext(ctx, sel, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: %s: %w", label, err)
	}
	defer func() { _ = rows.Close() }()
	var out []mlstudio.Evaluation
	for rows.Next() {
		e, err := scanMLEvaluation(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("postgres: %s scan: %w", label, err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := d.attachMLEvaluationValues(ctx, out); err != nil {
		return nil, 0, fmt.Errorf("postgres: %s values: %w", label, err)
	}
	return out, total, nil
}

func (d *DB) GetMLEvaluation(ctx context.Context, orgID, id string) (*mlstudio.Evaluation, error) {
	e, err := scanMLEvaluation(d.db.QueryRowContext(ctx, mlEvaluationSelect+`
		WHERE e.org_id=$1 AND e.id=$2 AND e.deleted_at IS NULL`, orgID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetMLEvaluation: %w", err)
	}
	evaluations := []mlstudio.Evaluation{e}
	if err := d.attachMLEvaluationValues(ctx, evaluations); err != nil {
		return nil, fmt.Errorf("postgres: GetMLEvaluation values: %w", err)
	}
	return &evaluations[0], nil
}
