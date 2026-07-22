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

func resolveMLID(ctx context.Context, tx *sql.Tx, table, orgID, mlflowID string) (string, bool, error) {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id FROM `+table+` WHERE org_id=$1 AND mlflow_id=$2 AND deleted_at IS NULL`, orgID, mlflowID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

func jsonBytes(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (d *DB) UpsertMLModels(ctx context.Context, orgID, actorID string, in []mlstudio.ModelInput) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres: UpsertMLModels begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, m := range in {
		var pv any
		if m.ProductionVersionMLflowID != nil && *m.ProductionVersionMLflowID != "" {
			id, ok, err := resolveMLID(ctx, tx, "ml_model_versions", orgID, *m.ProductionVersionMLflowID)
			if err != nil {
				return fmt.Errorf("postgres: UpsertMLModels resolve version: %w", err)
			}
			if ok {
				pv = id
			}
		}
		tags := m.Tags
		if tags == nil {
			tags = []string{}
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO ml_models (org_id, mlflow_id, name, description, tags, production_version_id, mlflow_created_at, mlflow_updated_at, synced_at, created_by, updated_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW(),$9,$9)
			ON CONFLICT (org_id, mlflow_id) DO UPDATE SET
				name=EXCLUDED.name, description=EXCLUDED.description, tags=EXCLUDED.tags,
				production_version_id=COALESCE(EXCLUDED.production_version_id, ml_models.production_version_id),
				mlflow_created_at=EXCLUDED.mlflow_created_at, mlflow_updated_at=EXCLUDED.mlflow_updated_at,
				synced_at=NOW(), updated_by=EXCLUDED.updated_by, updated_at=NOW()`,
			orgID, m.MLflowID, m.Name, m.Description, pq.Array(tags), pv, m.CreatedAt, m.UpdatedAt, actorID)
		if err != nil {
			return fmt.Errorf("postgres: UpsertMLModels upsert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres: UpsertMLModels commit: %w", err)
	}
	return nil
}

func (d *DB) UpdateMLModel(ctx context.Context, orgID, id, actorID string, in mlstudio.ModelUpdateInput) error {
	refs := in.References
	if refs == nil {
		refs = []string{}
	}
	_, err := d.db.ExecContext(ctx, `
		UPDATE ml_models SET domain=$1, problem_type=$2, owners=$3, license=$4,
			reference_links=$5, intended_use=$6, limitations=$7,
			ethical_considerations=$8, caveats=$9, updated_by=$10, updated_at=NOW()
		WHERE org_id=$11 AND id=$12 AND deleted_at IS NULL`,
		in.Domain, in.ProblemType, in.Owners, in.License, pq.Array(refs),
		in.IntendedUse, in.Limitations, in.EthicalConsiderations, in.Caveats,
		actorID, orgID, id)
	if err != nil {
		return fmt.Errorf("postgres: UpdateMLModel: %w", err)
	}
	return nil
}

func (d *DB) UpsertMLModelVersions(ctx context.Context, orgID, actorID string, in []mlstudio.ModelVersionInput) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres: UpsertMLModelVersions begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, v := range in {
		modelID, ok, err := resolveMLID(ctx, tx, "ml_models", orgID, v.ModelMLflowID)
		if err != nil {
			return fmt.Errorf("postgres: UpsertMLModelVersions resolve model: %w", err)
		}
		if !ok {
			return fmt.Errorf("%w: model %q", mlstudio.ErrParentNotFound, v.ModelMLflowID)
		}
		var runID any
		if v.RunMLflowID != nil && *v.RunMLflowID != "" {
			id, found, err := resolveMLID(ctx, tx, "ml_runs", orgID, *v.RunMLflowID)
			if err != nil {
				return fmt.Errorf("postgres: UpsertMLModelVersions resolve run: %w", err)
			}
			if found {
				runID = id
			}
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ml_model_versions (org_id, mlflow_id, model_id, version, description, run_id, mlflow_created_at, synced_at, created_by, updated_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),$8,$8)
			ON CONFLICT (org_id, mlflow_id) DO UPDATE SET
				model_id=EXCLUDED.model_id, version=EXCLUDED.version, description=EXCLUDED.description,
				run_id=COALESCE(EXCLUDED.run_id, ml_model_versions.run_id),
				mlflow_created_at=EXCLUDED.mlflow_created_at,
				synced_at=NOW(), updated_by=EXCLUDED.updated_by, updated_at=NOW()`,
			orgID, v.MLflowID, modelID, v.Version, v.Description, runID, v.CreatedAt, actorID)
		if err != nil {
			return fmt.Errorf("postgres: UpsertMLModelVersions upsert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres: UpsertMLModelVersions commit: %w", err)
	}
	return nil
}

func (d *DB) UpsertMLExperiments(ctx context.Context, orgID, actorID string, in []mlstudio.ExperimentInput) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres: UpsertMLExperiments begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, e := range in {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ml_experiments (org_id, mlflow_id, name, description, status, started_at, synced_at, created_by, updated_by)
			VALUES ($1,$2,$3,$4,$5,$6,NOW(),$7,$7)
			ON CONFLICT (org_id, mlflow_id) DO UPDATE SET
				name=EXCLUDED.name, description=EXCLUDED.description, status=EXCLUDED.status, started_at=EXCLUDED.started_at,
				synced_at=NOW(), updated_by=EXCLUDED.updated_by, updated_at=NOW()`,
			orgID, e.MLflowID, e.Name, e.Description, e.Status, e.StartedAt, actorID)
		if err != nil {
			return fmt.Errorf("postgres: UpsertMLExperiments upsert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres: UpsertMLExperiments commit: %w", err)
	}
	return nil
}

func (d *DB) UpsertMLRuns(ctx context.Context, orgID, actorID string, in []mlstudio.RunInput) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres: UpsertMLRuns begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, run := range in {
		experimentID, ok, err := resolveMLID(ctx, tx, "ml_experiments", orgID, run.ExperimentMLflowID)
		if err != nil {
			return fmt.Errorf("postgres: UpsertMLRuns resolve experiment: %w", err)
		}
		if !ok {
			return fmt.Errorf("%w: experiment %q", mlstudio.ErrParentNotFound, run.ExperimentMLflowID)
		}
		var datasetID any
		if run.DatasetMLflowID != nil && *run.DatasetMLflowID != "" {
			var id string
			err := tx.QueryRowContext(ctx, `SELECT id FROM ml_datasets WHERE org_id=$1 AND experiment_id=$2 AND mlflow_id=$3 AND deleted_at IS NULL`, orgID, experimentID, *run.DatasetMLflowID).Scan(&id)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("postgres: UpsertMLRuns resolve dataset: %w", err)
			}
			if err == nil {
				datasetID = id
			}
		}
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
			return fmt.Errorf("postgres: UpsertMLRuns marshal parameters: %w", err)
		}
		metricsJSON, err := jsonBytes(metrics)
		if err != nil {
			return fmt.Errorf("postgres: UpsertMLRuns marshal metrics: %w", err)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ml_runs (org_id, mlflow_id, experiment_id, name, status, started_at, ended_at, duration, notes, parameters, metrics, dataset_id, synced_at, created_by, updated_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW(),$13,$13)
			ON CONFLICT (org_id, mlflow_id) DO UPDATE SET
				experiment_id=EXCLUDED.experiment_id, name=EXCLUDED.name, status=EXCLUDED.status,
				started_at=EXCLUDED.started_at, ended_at=EXCLUDED.ended_at, duration=EXCLUDED.duration,
				notes=EXCLUDED.notes, parameters=EXCLUDED.parameters, metrics=EXCLUDED.metrics,
				dataset_id=COALESCE(EXCLUDED.dataset_id, ml_runs.dataset_id),
				synced_at=NOW(), updated_by=EXCLUDED.updated_by, updated_at=NOW()`,
			orgID, run.MLflowID, experimentID, run.Name, run.Status, run.StartedAt, run.EndedAt, run.Duration, run.Notes, paramsJSON, metricsJSON, datasetID, actorID)
		if err != nil {
			return fmt.Errorf("postgres: UpsertMLRuns upsert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres: UpsertMLRuns commit: %w", err)
	}
	return nil
}

func (d *DB) UpsertMLRunMetricPoints(ctx context.Context, orgID, runMLflowID string, in []mlstudio.MetricPoint) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres: UpsertMLRunMetricPoints begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	runID, ok, err := resolveMLID(ctx, tx, "ml_runs", orgID, runMLflowID)
	if err != nil {
		return fmt.Errorf("postgres: UpsertMLRunMetricPoints resolve run: %w", err)
	}
	if !ok {
		return fmt.Errorf("%w: run %q", mlstudio.ErrParentNotFound, runMLflowID)
	}
	for _, p := range in {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO ml_run_metric_points (run_id, key, step, value, ts)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (run_id, key, step) DO UPDATE SET value=EXCLUDED.value, ts=EXCLUDED.ts`,
			runID, p.Key, p.Step, p.Value, p.TS)
		if err != nil {
			return fmt.Errorf("postgres: UpsertMLRunMetricPoints upsert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres: UpsertMLRunMetricPoints commit: %w", err)
	}
	return nil
}

func (d *DB) UpsertMLArtifacts(ctx context.Context, orgID, actorID string, in []mlstudio.ArtifactInput) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres: UpsertMLArtifacts begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, a := range in {
		runID, ok, err := resolveMLID(ctx, tx, "ml_runs", orgID, a.RunMLflowID)
		if err != nil {
			return fmt.Errorf("postgres: UpsertMLArtifacts resolve run: %w", err)
		}
		if !ok {
			return fmt.Errorf("%w: run %q", mlstudio.ErrParentNotFound, a.RunMLflowID)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ml_artifacts (org_id, mlflow_id, run_id, name, type, uri, size, format, synced_at, created_by, updated_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW(),$9,$9)
			ON CONFLICT (org_id, mlflow_id) DO UPDATE SET
				run_id=EXCLUDED.run_id, name=EXCLUDED.name, type=EXCLUDED.type,
				uri=EXCLUDED.uri, size=EXCLUDED.size, format=EXCLUDED.format,
				synced_at=NOW(), updated_by=EXCLUDED.updated_by, updated_at=NOW()`,
			orgID, a.MLflowID, runID, a.Name, a.Type, a.URI, a.Size, a.Format, actorID)
		if err != nil {
			return fmt.Errorf("postgres: UpsertMLArtifacts upsert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres: UpsertMLArtifacts commit: %w", err)
	}
	return nil
}

func (d *DB) UpsertMLDatasets(ctx context.Context, orgID, actorID string, in []mlstudio.DatasetInput) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres: UpsertMLDatasets begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, ds := range in {
		experimentID, ok, err := resolveMLID(ctx, tx, "ml_experiments", orgID, ds.ExperimentMLflowID)
		if err != nil {
			return fmt.Errorf("postgres: UpsertMLDatasets resolve experiment: %w", err)
		}
		if !ok {
			return fmt.Errorf("%w: experiment %q", mlstudio.ErrParentNotFound, ds.ExperimentMLflowID)
		}
		schema := ds.Schema
		if schema == nil {
			schema = []mlstudio.SchemaField{}
		}
		schemaJSON, err := jsonBytes(schema)
		if err != nil {
			return fmt.Errorf("postgres: UpsertMLDatasets marshal schema: %w", err)
		}
		tags := ds.Tags
		if tags == nil {
			tags = map[string]string{}
		}
		tagsJSON, err := jsonBytes(tags)
		if err != nil {
			return fmt.Errorf("postgres: UpsertMLDatasets marshal tags: %w", err)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ml_datasets (org_id, experiment_id, mlflow_id, name, digest, source, source_type, context, row_count, schema, tags, synced_at, created_by, updated_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW(),$12,$12)
			ON CONFLICT (org_id, experiment_id, mlflow_id) DO UPDATE SET
				name=EXCLUDED.name, digest=EXCLUDED.digest, source=EXCLUDED.source, source_type=EXCLUDED.source_type,
				context=EXCLUDED.context, row_count=EXCLUDED.row_count, schema=EXCLUDED.schema, tags=EXCLUDED.tags,
				synced_at=NOW(), updated_by=EXCLUDED.updated_by, updated_at=NOW()`,
			orgID, experimentID, ds.MLflowID, ds.Name, ds.Digest, ds.Source, ds.SourceType, ds.Context, ds.RowCount, schemaJSON, tagsJSON, actorID)
		if err != nil {
			return fmt.Errorf("postgres: UpsertMLDatasets upsert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres: UpsertMLDatasets commit: %w", err)
	}
	return nil
}

func (d *DB) UpsertMLEvaluations(ctx context.Context, orgID, actorID string, in []mlstudio.EvaluationInput) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres: UpsertMLEvaluations begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, e := range in {
		versionID, ok, err := resolveMLID(ctx, tx, "ml_model_versions", orgID, e.VersionMLflowID)
		if err != nil {
			return fmt.Errorf("postgres: UpsertMLEvaluations resolve version: %w", err)
		}
		if !ok {
			return fmt.Errorf("%w: version %q", mlstudio.ErrParentNotFound, e.VersionMLflowID)
		}
		datasetID, err := resolveOptional(ctx, tx, "ml_datasets", orgID, e.DatasetMLflowID)
		if err != nil {
			return fmt.Errorf("postgres: UpsertMLEvaluations resolve dataset: %w", err)
		}
		var evalID string
		err = tx.QueryRowContext(ctx, `
			INSERT INTO ml_evaluations (org_id, mlflow_id, version_id, dataset_id, name, type, description, summary, evaluated_at, evaluator, synced_at, created_by, updated_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),$11,$11)
			ON CONFLICT (org_id, mlflow_id) DO UPDATE SET
				version_id=EXCLUDED.version_id, dataset_id=COALESCE(EXCLUDED.dataset_id, ml_evaluations.dataset_id),
				name=EXCLUDED.name, type=EXCLUDED.type, description=EXCLUDED.description, summary=EXCLUDED.summary,
				evaluated_at=EXCLUDED.evaluated_at, evaluator=EXCLUDED.evaluator,
				synced_at=NOW(), updated_by=EXCLUDED.updated_by, updated_at=NOW()
			RETURNING id`,
			orgID, e.MLflowID, versionID, datasetID, e.Name, e.Type, e.Description, e.Summary, e.EvaluatedAt, e.Evaluator, actorID).Scan(&evalID)
		if err != nil {
			return fmt.Errorf("postgres: UpsertMLEvaluations upsert: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM ml_evaluation_metrics WHERE evaluation_id=$1`, evalID); err != nil {
			return fmt.Errorf("postgres: UpsertMLEvaluations clear metrics: %w", err)
		}
		for _, m := range e.Metrics {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO ml_evaluation_metrics (evaluation_id, name, value, unit, direction, category, measured_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7)`,
				evalID, m.Name, m.Value, m.Unit, m.Direction, m.Category, m.MeasuredAt)
			if err != nil {
				return fmt.Errorf("postgres: UpsertMLEvaluations metric: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres: UpsertMLEvaluations commit: %w", err)
	}
	return nil
}

func resolveOptional(ctx context.Context, tx *sql.Tx, table, orgID string, mlflowID *string) (any, error) {
	if mlflowID == nil || *mlflowID == "" {
		return nil, nil
	}
	id, ok, err := resolveMLID(ctx, tx, table, orgID, *mlflowID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return id, nil
}
