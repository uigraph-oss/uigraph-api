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

type mlQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func replaceMLParams(ctx context.Context, tx *sql.Tx, table, parentCol, parentID string, params map[string]any) error {
	keys := make([]string, 0, len(params))
	for key, value := range params {
		encoded, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("param %q: %w", key, err)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO `+table+` (`+parentCol+`, key, value) VALUES ($1,$2,$3)
			ON CONFLICT (`+parentCol+`, key) DO UPDATE SET value=EXCLUDED.value`, parentID, key, string(encoded))
		if err != nil {
			return err
		}
		keys = append(keys, key)
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE `+parentCol+`=$1 AND key <> ALL($2)`, parentID, pq.Array(keys))
	return err
}

func replaceMLMetrics(ctx context.Context, tx *sql.Tx, table, parentCol, parentID string, metrics map[string]any) error {
	keys := make([]string, 0, len(metrics))
	for key, value := range metrics {
		var number float64
		switch v := value.(type) {
		case float64:
			number = v
		case json.Number:
			parsed, err := v.Float64()
			if err != nil {
				return fmt.Errorf("metric %q: %w", key, err)
			}
			number = parsed
		default:
			return fmt.Errorf("%w: metric %q must be a number", mlstudio.ErrInvalidValue, key)
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO `+table+` (`+parentCol+`, key, value) VALUES ($1,$2,$3)
			ON CONFLICT (`+parentCol+`, key) DO UPDATE SET value=EXCLUDED.value`, parentID, key, number)
		if err != nil {
			return err
		}
		keys = append(keys, key)
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE `+parentCol+`=$1 AND key <> ALL($2)`, parentID, pq.Array(keys))
	return err
}

func loadMLParams(ctx context.Context, q mlQuerier, table, parentCol string, parentIDs []string) (map[string]map[string]any, error) {
	out := map[string]map[string]any{}
	if len(parentIDs) == 0 {
		return out, nil
	}
	rows, err := q.QueryContext(ctx, `SELECT `+parentCol+`, key, value FROM `+table+` WHERE `+parentCol+` = ANY($1) ORDER BY key ASC`, pq.Array(parentIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var parentID, key, value string
		if err := rows.Scan(&parentID, &key, &value); err != nil {
			return nil, err
		}
		var decoded any
		if err := json.Unmarshal([]byte(value), &decoded); err != nil {
			return nil, fmt.Errorf("param %q: %w", key, err)
		}
		if out[parentID] == nil {
			out[parentID] = map[string]any{}
		}
		out[parentID][key] = decoded
	}
	return out, rows.Err()
}

func loadMLMetrics(ctx context.Context, q mlQuerier, table, parentCol string, parentIDs []string) (map[string]map[string]any, error) {
	out := map[string]map[string]any{}
	if len(parentIDs) == 0 {
		return out, nil
	}
	rows, err := q.QueryContext(ctx, `SELECT `+parentCol+`, key, value FROM `+table+` WHERE `+parentCol+` = ANY($1) ORDER BY key ASC`, pq.Array(parentIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var parentID, key string
		var value float64
		if err := rows.Scan(&parentID, &key, &value); err != nil {
			return nil, err
		}
		if out[parentID] == nil {
			out[parentID] = map[string]any{}
		}
		out[parentID][key] = value
	}
	return out, rows.Err()
}

func resolveProjectID(ctx context.Context, tx *sql.Tx, orgID, projectName string) (any, error) {
	if projectName == "" {
		return nil, nil
	}
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id FROM ml_projects WHERE org_id=$1 AND name=$2 AND deleted_at IS NULL`, orgID, projectName).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: project %q", mlstudio.ErrParentNotFound, projectName)
	}
	if err != nil {
		return nil, err
	}
	return id, nil
}

func (d *DB) ResolveOrgUserIDsByEmail(ctx context.Context, orgID string, emails []string) (map[string]string, error) {
	out := map[string]string{}
	if len(emails) == 0 {
		return out, nil
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT LOWER(u.email), u.id
		FROM   users u
		JOIN   org_members om ON om.user_id = u.id
		WHERE  om.org_id = $1 AND LOWER(u.email) = ANY($2)`, orgID, pq.Array(emails))
	if err != nil {
		return nil, fmt.Errorf("postgres: ResolveOrgUserIDsByEmail: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var email, id string
		if err := rows.Scan(&email, &id); err != nil {
			return nil, fmt.Errorf("postgres: ResolveOrgUserIDsByEmail scan: %w", err)
		}
		out[email] = id
	}
	return out, rows.Err()
}

func (d *DB) UpsertMLProjects(ctx context.Context, orgID, actorID string, in []mlstudio.ProjectInput) (mlstudio.SyncCounts, error) {
	var counts mlstudio.SyncCounts
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return counts, fmt.Errorf("postgres: UpsertMLProjects begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, p := range in {
		if (p.TeamID == nil || *p.TeamID == "") && p.TeamName != "" {
			id, err := d.resolveTeamID(ctx, orgID, p.TeamName)
			if err != nil {
				return counts, err
			}
			p.TeamID = &id
		}
		var existing string
		err := tx.QueryRowContext(ctx, `SELECT id FROM ml_projects WHERE org_id=$1 AND name=$2 AND deleted_at IS NULL`, orgID, p.Name).Scan(&existing)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return counts, fmt.Errorf("postgres: UpsertMLProjects existing: %w", err)
		}
		if err == nil {
			counts.Updated++
		}
		if errors.Is(err, sql.ErrNoRows) {
			counts.Created++
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ml_projects (org_id, name, type, description, source_type, source_url, team_id, synced_at, created_by, updated_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),$8,$8)
			ON CONFLICT (org_id, name) DO UPDATE SET
				type=EXCLUDED.type, description=EXCLUDED.description, source_type=EXCLUDED.source_type,
				source_url=EXCLUDED.source_url, team_id=EXCLUDED.team_id,
				deleted_at=NULL, deleted_by=NULL,
				synced_at=NOW(), updated_by=EXCLUDED.updated_by, updated_at=NOW()`,
			orgID, p.Name, p.Type, p.Description, p.SourceType, p.SourceURL, p.TeamID, actorID)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLProjects upsert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return counts, fmt.Errorf("postgres: UpsertMLProjects commit: %w", err)
	}
	return counts, nil
}

func (d *DB) CreateMLProject(ctx context.Context, orgID, actorID string, p mlstudio.ProjectInput) (*mlstudio.Project, error) {
	if (p.TeamID == nil || *p.TeamID == "") && p.TeamName != "" {
		id, err := d.resolveTeamID(ctx, orgID, p.TeamName)
		if err != nil {
			return nil, err
		}
		p.TeamID = &id
	}
	var id string
	err := d.db.QueryRowContext(ctx, `
		INSERT INTO ml_projects (org_id, name, type, description, source_type, source_url, team_id, synced_at, created_by, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),$8,$8)
		ON CONFLICT (org_id, name) DO UPDATE SET
			type=EXCLUDED.type, description=EXCLUDED.description, source_type=EXCLUDED.source_type,
			source_url=EXCLUDED.source_url, team_id=EXCLUDED.team_id,
			synced_at=NOW(), updated_by=EXCLUDED.updated_by, updated_at=NOW()
		RETURNING id`,
		orgID, p.Name, p.Type, p.Description, p.SourceType, p.SourceURL, p.TeamID, actorID).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("postgres: CreateMLProject: %w", err)
	}
	return d.GetMLProject(ctx, orgID, id)
}

func (d *DB) UpdateMLProject(ctx context.Context, orgID, id, actorID string, p mlstudio.ProjectInput) (*mlstudio.Project, error) {
	if (p.TeamID == nil || *p.TeamID == "") && p.TeamName != "" {
		teamID, err := d.resolveTeamID(ctx, orgID, p.TeamName)
		if err != nil {
			return nil, err
		}
		p.TeamID = &teamID
	}
	_, err := d.db.ExecContext(ctx, `
		UPDATE ml_projects SET name=$1, type=$2, description=$3, team_id=$4, updated_by=$5, updated_at=NOW()
		WHERE org_id=$6 AND id=$7 AND deleted_at IS NULL`,
		p.Name, p.Type, p.Description, p.TeamID, actorID, orgID, id)
	if err != nil {
		return nil, fmt.Errorf("postgres: UpdateMLProject: %w", err)
	}
	return d.GetMLProject(ctx, orgID, id)
}

func (d *DB) DeleteMLProject(ctx context.Context, orgID, id, actorID string) error {
	_, err := d.db.ExecContext(ctx, `
		UPDATE ml_projects SET deleted_at=NOW(), updated_by=$1, updated_at=NOW()
		WHERE org_id=$2 AND id=$3 AND deleted_at IS NULL`, actorID, orgID, id)
	if err != nil {
		return fmt.Errorf("postgres: DeleteMLProject: %w", err)
	}
	return nil
}

func (d *DB) UpsertMLModels(ctx context.Context, orgID, actorID string, in []mlstudio.ModelInput) (mlstudio.SyncCounts, error) {
	var counts mlstudio.SyncCounts
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return counts, fmt.Errorf("postgres: UpsertMLModels begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, m := range in {
		var pv any
		if m.ProductionVersionMLflowID != nil && *m.ProductionVersionMLflowID != "" {
			id, ok, err := resolveMLID(ctx, tx, "ml_model_versions", orgID, *m.ProductionVersionMLflowID)
			if err != nil {
				return counts, fmt.Errorf("postgres: UpsertMLModels resolve version: %w", err)
			}
			if ok {
				pv = id
			}
		}
		tags := m.Tags
		if tags == nil {
			tags = []string{}
		}
		projectID, err := resolveProjectID(ctx, tx, orgID, m.ProjectName)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLModels resolve project: %w", err)
		}
		problemType := m.ProblemType
		if problemType == "" {
			problemType = "other"
		}
		actor := actorID
		if m.ActorID != "" {
			actor = m.ActorID
		}
		_, existed, err := resolveMLID(ctx, tx, "ml_models", orgID, m.MLflowID)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLModels existing: %w", err)
		}
		if existed {
			counts.Updated++
		}
		if !existed {
			counts.Created++
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ml_models (org_id, mlflow_id, project_id, name, description, tags, problem_type, domain, license, intended_use, limitations, recommendations, considerations, production_version_id, mlflow_created_at, mlflow_updated_at, origin, synced_at, created_by, updated_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,'mlflow',NOW(),$17,$17)
			ON CONFLICT (org_id, mlflow_id) DO UPDATE SET
				project_id=COALESCE(EXCLUDED.project_id, ml_models.project_id),
				name=EXCLUDED.name, description=EXCLUDED.description, tags=EXCLUDED.tags,
				problem_type=EXCLUDED.problem_type, domain=EXCLUDED.domain, license=EXCLUDED.license,
				intended_use=EXCLUDED.intended_use, limitations=EXCLUDED.limitations,
				recommendations=EXCLUDED.recommendations, considerations=EXCLUDED.considerations,
				production_version_id=COALESCE(EXCLUDED.production_version_id, ml_models.production_version_id),
				mlflow_created_at=EXCLUDED.mlflow_created_at, mlflow_updated_at=EXCLUDED.mlflow_updated_at,
				deleted_at=NULL, deleted_by=NULL,
				synced_at=NOW(), updated_by=EXCLUDED.updated_by, updated_at=NOW()`,
			orgID, m.MLflowID, projectID, m.Name, m.Description, pq.Array(tags), problemType, m.Domain, m.License, m.IntendedUse, m.Limitations, m.Recommendations, m.Considerations, pv, m.CreatedAt, m.UpdatedAt, actor)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLModels upsert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return counts, fmt.Errorf("postgres: UpsertMLModels commit: %w", err)
	}
	return counts, nil
}

func (d *DB) UpdateMLModel(ctx context.Context, orgID, id, actorID string, in mlstudio.ModelUpdateInput) error {
	refs := in.References
	if refs == nil {
		refs = []string{}
	}
	_, err := d.db.ExecContext(ctx, `
		UPDATE ml_models SET domain=$1, problem_type=$2, license=$3,
			reference_links=$4, intended_use=$5, limitations=$6,
			considerations=$7, recommendations=$8, updated_by=$9, updated_at=NOW()
		WHERE org_id=$10 AND id=$11 AND deleted_at IS NULL`,
		in.Domain, in.ProblemType, in.License, pq.Array(refs),
		in.IntendedUse, in.Limitations, in.Considerations, in.Recommendations,
		actorID, orgID, id)
	if err != nil {
		return fmt.Errorf("postgres: UpdateMLModel: %w", err)
	}
	return nil
}

func (d *DB) UpsertMLModelVersions(ctx context.Context, orgID, actorID string, in []mlstudio.ModelVersionInput) (mlstudio.SyncCounts, error) {
	var counts mlstudio.SyncCounts
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return counts, fmt.Errorf("postgres: UpsertMLModelVersions begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, v := range in {
		modelID, ok, err := resolveMLID(ctx, tx, "ml_models", orgID, v.ModelMLflowID)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLModelVersions resolve model: %w", err)
		}
		if !ok {
			return counts, fmt.Errorf("%w: model %q", mlstudio.ErrParentNotFound, v.ModelMLflowID)
		}
		var runID any
		if v.RunMLflowID != nil && *v.RunMLflowID != "" {
			id, found, err := resolveMLID(ctx, tx, "ml_runs", orgID, *v.RunMLflowID)
			if err != nil {
				return counts, fmt.Errorf("postgres: UpsertMLModelVersions resolve run: %w", err)
			}
			if found {
				runID = id
			}
		}
		actor := actorID
		if v.ActorID != "" {
			actor = v.ActorID
		}
		_, existed, err := resolveMLID(ctx, tx, "ml_model_versions", orgID, v.MLflowID)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLModelVersions existing: %w", err)
		}
		if existed {
			counts.Updated++
		}
		if !existed {
			counts.Created++
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ml_model_versions (org_id, mlflow_id, model_id, version, description, run_id, mlflow_created_at, source, synced_at, created_by, updated_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,'mlflow',NOW(),$8,$8)
			ON CONFLICT (org_id, mlflow_id) DO UPDATE SET
				model_id=EXCLUDED.model_id, version=EXCLUDED.version, description=EXCLUDED.description,
				run_id=COALESCE(EXCLUDED.run_id, ml_model_versions.run_id),
				mlflow_created_at=EXCLUDED.mlflow_created_at,
				deleted_at=NULL, deleted_by=NULL,
				synced_at=NOW(), updated_by=EXCLUDED.updated_by, updated_at=NOW()`,
			orgID, v.MLflowID, modelID, v.Version, v.Description, runID, v.CreatedAt, actor)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLModelVersions upsert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return counts, fmt.Errorf("postgres: UpsertMLModelVersions commit: %w", err)
	}
	return counts, nil
}

func (d *DB) UpsertMLExperiments(ctx context.Context, orgID, actorID string, in []mlstudio.ExperimentInput) (mlstudio.SyncCounts, error) {
	var counts mlstudio.SyncCounts
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return counts, fmt.Errorf("postgres: UpsertMLExperiments begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, e := range in {
		projectID, err := resolveProjectID(ctx, tx, orgID, e.ProjectName)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLExperiments resolve project: %w", err)
		}
		tags := e.Tags
		if tags == nil {
			tags = []string{}
		}
		actor := actorID
		if e.ActorID != "" {
			actor = e.ActorID
		}
		_, existed, err := resolveMLID(ctx, tx, "ml_experiments", orgID, e.MLflowID)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLExperiments existing: %w", err)
		}
		if existed {
			counts.Updated++
		}
		if !existed {
			counts.Created++
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ml_experiments (org_id, mlflow_id, project_id, name, description, status, tags, source, synced_at, created_by, updated_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,'mlflow',NOW(),$8,$8)
			ON CONFLICT (org_id, mlflow_id) DO UPDATE SET
				project_id=COALESCE(EXCLUDED.project_id, ml_experiments.project_id),
				name=EXCLUDED.name, description=EXCLUDED.description, status=EXCLUDED.status, tags=EXCLUDED.tags,
				deleted_at=NULL, deleted_by=NULL,
				synced_at=NOW(), updated_by=EXCLUDED.updated_by, updated_at=NOW()`,
			orgID, e.MLflowID, projectID, e.Name, e.Description, e.Status, pq.Array(tags), actor)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLExperiments upsert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return counts, fmt.Errorf("postgres: UpsertMLExperiments commit: %w", err)
	}
	return counts, nil
}

func (d *DB) UpsertMLRuns(ctx context.Context, orgID, actorID string, in []mlstudio.RunInput) (mlstudio.SyncCounts, error) {
	var counts mlstudio.SyncCounts
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return counts, fmt.Errorf("postgres: UpsertMLRuns begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, run := range in {
		experimentID, ok, err := resolveMLID(ctx, tx, "ml_experiments", orgID, run.ExperimentMLflowID)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLRuns resolve experiment: %w", err)
		}
		if !ok {
			return counts, fmt.Errorf("%w: experiment %q", mlstudio.ErrParentNotFound, run.ExperimentMLflowID)
		}
		var datasetID any
		if run.DatasetMLflowID != nil && *run.DatasetMLflowID != "" {
			var id string
			err := tx.QueryRowContext(ctx, `SELECT id FROM ml_datasets WHERE org_id=$1 AND experiment_id=$2 AND mlflow_id=$3 AND deleted_at IS NULL`, orgID, experimentID, *run.DatasetMLflowID).Scan(&id)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return counts, fmt.Errorf("postgres: UpsertMLRuns resolve dataset: %w", err)
			}
			if err == nil {
				datasetID = id
			}
		}
		tags := run.Tags
		if tags == nil {
			tags = []string{}
		}
		actor := actorID
		if run.ActorID != "" {
			actor = run.ActorID
		}
		_, existed, err := resolveMLID(ctx, tx, "ml_runs", orgID, run.MLflowID)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLRuns existing: %w", err)
		}
		if existed {
			counts.Updated++
		}
		if !existed {
			counts.Created++
		}
		var runID string
		err = tx.QueryRowContext(ctx, `
			INSERT INTO ml_runs (org_id, mlflow_id, experiment_id, name, status, started_at, ended_at, notes, tags, dataset_id, source, synced_at, created_by, updated_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'mlflow',NOW(),$11,$11)
			ON CONFLICT (org_id, mlflow_id) DO UPDATE SET
				experiment_id=EXCLUDED.experiment_id, name=EXCLUDED.name, status=EXCLUDED.status,
				started_at=EXCLUDED.started_at, ended_at=EXCLUDED.ended_at,
				notes=EXCLUDED.notes, tags=EXCLUDED.tags,
				dataset_id=COALESCE(EXCLUDED.dataset_id, ml_runs.dataset_id),
				deleted_at=NULL, deleted_by=NULL,
				synced_at=NOW(), updated_by=EXCLUDED.updated_by, updated_at=NOW()
			RETURNING id`,
			orgID, run.MLflowID, experimentID, run.Name, run.Status, run.StartedAt, run.EndedAt, run.Notes, pq.Array(tags), datasetID, actor).Scan(&runID)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLRuns upsert: %w", err)
		}
		if err := replaceMLParams(ctx, tx, "ml_runs_params", "run_id", runID, run.Parameters); err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLRuns parameters: %w", err)
		}
		if err := replaceMLMetrics(ctx, tx, "ml_runs_metrics", "run_id", runID, run.Metrics); err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLRuns metrics: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return counts, fmt.Errorf("postgres: UpsertMLRuns commit: %w", err)
	}
	return counts, nil
}

func (d *DB) UpsertMLArtifacts(ctx context.Context, orgID, actorID string, in []mlstudio.ArtifactInput) (mlstudio.SyncCounts, error) {
	var counts mlstudio.SyncCounts
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return counts, fmt.Errorf("postgres: UpsertMLArtifacts begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, a := range in {
		runID, ok, err := resolveMLID(ctx, tx, "ml_runs", orgID, a.RunMLflowID)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLArtifacts resolve run: %w", err)
		}
		if !ok {
			return counts, fmt.Errorf("%w: run %q", mlstudio.ErrParentNotFound, a.RunMLflowID)
		}
		actor := actorID
		if a.ActorID != "" {
			actor = a.ActorID
		}
		_, existed, err := resolveMLID(ctx, tx, "ml_artifacts", orgID, a.MLflowID)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLArtifacts existing: %w", err)
		}
		if existed {
			counts.Updated++
		}
		if !existed {
			counts.Created++
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ml_artifacts (org_id, mlflow_id, run_id, name, type, uri, download_uri, size, format, source, synced_at, created_by, updated_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'mlflow',NOW(),$10,$10)
			ON CONFLICT (org_id, mlflow_id) DO UPDATE SET
				run_id=EXCLUDED.run_id, name=EXCLUDED.name, type=EXCLUDED.type,
				uri=EXCLUDED.uri, download_uri=EXCLUDED.download_uri, size=EXCLUDED.size, format=EXCLUDED.format,
				deleted_at=NULL, deleted_by=NULL,
				synced_at=NOW(), updated_by=EXCLUDED.updated_by, updated_at=NOW()`,
			orgID, a.MLflowID, runID, a.Name, a.Type, a.URI, a.DownloadURI, a.Size, a.Format, actor)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLArtifacts upsert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return counts, fmt.Errorf("postgres: UpsertMLArtifacts commit: %w", err)
	}
	return counts, nil
}

func (d *DB) UpsertMLDatasets(ctx context.Context, orgID, actorID string, in []mlstudio.DatasetInput) (mlstudio.SyncCounts, error) {
	var counts mlstudio.SyncCounts
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return counts, fmt.Errorf("postgres: UpsertMLDatasets begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, ds := range in {
		experimentID, ok, err := resolveMLID(ctx, tx, "ml_experiments", orgID, ds.ExperimentMLflowID)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLDatasets resolve experiment: %w", err)
		}
		if !ok {
			return counts, fmt.Errorf("%w: experiment %q", mlstudio.ErrParentNotFound, ds.ExperimentMLflowID)
		}
		schema := ds.Schema
		if schema == nil {
			schema = []mlstudio.SchemaField{}
		}
		schemaJSON, err := jsonBytes(schema)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLDatasets marshal schema: %w", err)
		}
		tags := ds.Tags
		if tags == nil {
			tags = []string{}
		}
		actor := actorID
		if ds.ActorID != "" {
			actor = ds.ActorID
		}
		var existing string
		err = tx.QueryRowContext(ctx, `SELECT id FROM ml_datasets WHERE org_id=$1 AND experiment_id=$2 AND mlflow_id=$3 AND deleted_at IS NULL`, orgID, experimentID, ds.MLflowID).Scan(&existing)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return counts, fmt.Errorf("postgres: UpsertMLDatasets existing: %w", err)
		}
		if err == nil {
			counts.Updated++
		}
		if errors.Is(err, sql.ErrNoRows) {
			counts.Created++
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ml_datasets (org_id, experiment_id, mlflow_id, name, digest, source, source_type, context, row_count, schema, tags, origin, synced_at, created_by, updated_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'mlflow',NOW(),$12,$12)
			ON CONFLICT (org_id, experiment_id, mlflow_id) DO UPDATE SET
				name=EXCLUDED.name, digest=EXCLUDED.digest, source=EXCLUDED.source, source_type=EXCLUDED.source_type,
				context=EXCLUDED.context, row_count=EXCLUDED.row_count, schema=EXCLUDED.schema, tags=EXCLUDED.tags,
				deleted_at=NULL, deleted_by=NULL,
				synced_at=NOW(), updated_by=EXCLUDED.updated_by, updated_at=NOW()`,
			orgID, experimentID, ds.MLflowID, ds.Name, ds.Digest, ds.Source, ds.SourceType, ds.Context, ds.RowCount, schemaJSON, pq.Array(tags), actor)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLDatasets upsert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return counts, fmt.Errorf("postgres: UpsertMLDatasets commit: %w", err)
	}
	return counts, nil
}

func (d *DB) UpsertMLEvaluations(ctx context.Context, orgID, actorID string, in []mlstudio.EvaluationInput) (mlstudio.SyncCounts, error) {
	var counts mlstudio.SyncCounts
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return counts, fmt.Errorf("postgres: UpsertMLEvaluations begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, e := range in {
		versionID, ok, err := resolveMLID(ctx, tx, "ml_model_versions", orgID, e.VersionMLflowID)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLEvaluations resolve version: %w", err)
		}
		if !ok {
			return counts, fmt.Errorf("%w: version %q", mlstudio.ErrParentNotFound, e.VersionMLflowID)
		}
		experimentID, ok, err := resolveMLID(ctx, tx, "ml_experiments", orgID, e.ExperimentMLflowID)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLEvaluations resolve experiment: %w", err)
		}
		if !ok {
			return counts, fmt.Errorf("%w: experiment %q", mlstudio.ErrParentNotFound, e.ExperimentMLflowID)
		}
		datasetID, err := resolveOptional(ctx, tx, "ml_datasets", orgID, e.DatasetMLflowID)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLEvaluations resolve dataset: %w", err)
		}
		tags := e.Tags
		if tags == nil {
			tags = []string{}
		}
		actor := actorID
		if e.ActorID != "" {
			actor = e.ActorID
		}
		_, existed, err := resolveMLID(ctx, tx, "ml_evaluations", orgID, e.MLflowID)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLEvaluations existing: %w", err)
		}
		if existed {
			counts.Updated++
		}
		if !existed {
			counts.Created++
		}
		var evalID string
		err = tx.QueryRowContext(ctx, `
			INSERT INTO ml_evaluations (org_id, mlflow_id, version_id, experiment_id, dataset_id, name, type, description, summary, started_at, ended_at, evaluator, tags, source, synced_at, created_by, updated_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'mlflow',NOW(),$14,$14)
			ON CONFLICT (org_id, mlflow_id) DO UPDATE SET
				version_id=EXCLUDED.version_id, experiment_id=EXCLUDED.experiment_id,
				dataset_id=COALESCE(EXCLUDED.dataset_id, ml_evaluations.dataset_id),
				name=EXCLUDED.name, type=EXCLUDED.type, description=EXCLUDED.description, summary=EXCLUDED.summary,
				started_at=EXCLUDED.started_at, ended_at=EXCLUDED.ended_at, evaluator=EXCLUDED.evaluator,
				tags=EXCLUDED.tags,
				deleted_at=NULL, deleted_by=NULL,
				synced_at=NOW(), updated_by=EXCLUDED.updated_by, updated_at=NOW()
			RETURNING id`,
			orgID, e.MLflowID, versionID, experimentID, datasetID, e.Name, e.Type, e.Description, e.Summary, e.StartedAt, e.EndedAt, e.Evaluator, pq.Array(tags), actor).Scan(&evalID)
		if err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLEvaluations upsert: %w", err)
		}
		if err := replaceMLParams(ctx, tx, "ml_evaluations_params", "eval_id", evalID, e.Parameters); err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLEvaluations parameters: %w", err)
		}
		if err := replaceMLMetrics(ctx, tx, "ml_evaluations_metrics", "eval_id", evalID, e.Metrics); err != nil {
			return counts, fmt.Errorf("postgres: UpsertMLEvaluations metrics: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return counts, fmt.Errorf("postgres: UpsertMLEvaluations commit: %w", err)
	}
	return counts, nil
}

func (d *DB) PruneMLRuns(ctx context.Context, orgID, actorID string, in []mlstudio.RunPruneInput) (int, error) {
	deleted := 0
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("postgres: PruneMLRuns begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, r := range in {
		runID, ok, err := resolveMLID(ctx, tx, "ml_runs", orgID, r.MLflowID)
		if err != nil {
			return 0, fmt.Errorf("postgres: PruneMLRuns resolve run: %w", err)
		}
		if !ok {
			continue
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE ml_artifacts SET deleted_at=NOW(), deleted_by=$1, updated_by=$1, updated_at=NOW()
			WHERE org_id=$2 AND run_id=$3 AND deleted_at IS NULL`, actorID, orgID, runID)
		if err != nil {
			return 0, fmt.Errorf("postgres: PruneMLRuns artifacts: %w", err)
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE ml_runs SET deleted_at=NOW(), deleted_by=$1, updated_by=$1, updated_at=NOW()
			WHERE org_id=$2 AND id=$3 AND deleted_at IS NULL`, actorID, orgID, runID)
		if err != nil {
			return 0, fmt.Errorf("postgres: PruneMLRuns run: %w", err)
		}
		deleted++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("postgres: PruneMLRuns commit: %w", err)
	}
	return deleted, nil
}

func (d *DB) PruneMLModelVersions(ctx context.Context, orgID, actorID string, in mlstudio.VersionPruneInput) (int, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("postgres: PruneMLModelVersions begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	modelID, ok, err := resolveMLID(ctx, tx, "ml_models", orgID, in.ModelMLflowID)
	if err != nil {
		return 0, fmt.Errorf("postgres: PruneMLModelVersions resolve model: %w", err)
	}
	if !ok {
		return 0, fmt.Errorf("%w: model %q", mlstudio.ErrParentNotFound, in.ModelMLflowID)
	}

	keep := in.Keep
	if keep == nil {
		keep = []string{}
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM ml_model_versions
		WHERE org_id=$1 AND model_id=$2 AND deleted_at IS NULL AND mlflow_id <> ALL($3)`,
		orgID, modelID, pq.Array(keep))
	if err != nil {
		return 0, fmt.Errorf("postgres: PruneMLModelVersions select: %w", err)
	}
	var stale []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("postgres: PruneMLModelVersions scan: %w", err)
		}
		stale = append(stale, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, fmt.Errorf("postgres: PruneMLModelVersions rows: %w", err)
	}
	_ = rows.Close()
	if len(stale) == 0 {
		if err := tx.Commit(); err != nil {
			return 0, fmt.Errorf("postgres: PruneMLModelVersions commit: %w", err)
		}
		return 0, nil
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE ml_evaluations SET deleted_at=NOW(), deleted_by=$1, updated_by=$1, updated_at=NOW()
		WHERE org_id=$2 AND version_id = ANY($3) AND deleted_at IS NULL`, actorID, orgID, pq.Array(stale))
	if err != nil {
		return 0, fmt.Errorf("postgres: PruneMLModelVersions evaluations: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE ml_models SET production_version_id=NULL, updated_by=$1, updated_at=NOW()
		WHERE org_id=$2 AND production_version_id = ANY($3)`, actorID, orgID, pq.Array(stale))
	if err != nil {
		return 0, fmt.Errorf("postgres: PruneMLModelVersions production version: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE ml_model_versions SET deleted_at=NOW(), deleted_by=$1, updated_by=$1, updated_at=NOW()
		WHERE org_id=$2 AND id = ANY($3) AND deleted_at IS NULL`, actorID, orgID, pq.Array(stale))
	if err != nil {
		return 0, fmt.Errorf("postgres: PruneMLModelVersions versions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("postgres: PruneMLModelVersions commit: %w", err)
	}
	return len(stale), nil
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
