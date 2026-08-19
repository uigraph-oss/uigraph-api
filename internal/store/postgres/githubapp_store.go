package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/uigraph/app/internal/authz"
	"github.com/uigraph/app/internal/githubapp"
	"github.com/uigraph/app/internal/identity"
	"github.com/uigraph/app/internal/store"
)

func (d *DB) GetOrgName(ctx context.Context, orgID string) (string, error) {
	var name string
	if err := d.db.QueryRowContext(ctx, `SELECT name FROM orgs WHERE id=$1 AND disabled=FALSE`, orgID).Scan(&name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", store.ErrNotFound
		}
		return "", fmt.Errorf("postgres: GetOrgName: %w", err)
	}
	return name, nil
}

func (d *DB) CreateInstallState(ctx context.Context, orgID, userID, stateHash string, expiresAt time.Time) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO github_installation_states(state_hash, org_id, user_id, expires_at)
		VALUES($1,$2,$3,$4)`, stateHash, orgID, userID, expiresAt)
	if err != nil {
		return fmt.Errorf("postgres: CreateInstallState: %w", err)
	}
	return nil
}

func (d *DB) AuthorizeInstallState(ctx context.Context, stateHash string, githubUserID int64) (string, error) {
	var orgID string
	err := d.db.QueryRowContext(ctx, `
		UPDATE github_installation_states
		SET github_user_id=$2, authorized_at=NOW()
		WHERE state_hash=$1 AND consumed_at IS NULL AND expires_at>NOW() AND authorized_at IS NULL
		RETURNING org_id`, stateHash, githubUserID).Scan(&orgID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", store.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("postgres: AuthorizeInstallState: %w", err)
	}
	return orgID, nil
}

func (d *DB) ConsumeInstallState(ctx context.Context, stateHash string, githubUserID int64) (string, error) {
	var orgID string
	err := d.db.QueryRowContext(ctx, `
		UPDATE github_installation_states
		SET consumed_at=NOW()
		WHERE state_hash=$1 AND consumed_at IS NULL AND expires_at>NOW() AND authorized_at IS NOT NULL AND github_user_id=$2
		RETURNING org_id`, stateHash, githubUserID).Scan(&orgID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", store.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("postgres: ConsumeInstallState: %w", err)
	}
	return orgID, nil
}

func (d *DB) UpsertInstallation(ctx context.Context, orgID string, installationID int64) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO github_installations(org_id, github_installation_id) VALUES($1,$2)
		ON CONFLICT(org_id) DO UPDATE SET github_installation_id=EXCLUDED.github_installation_id`, orgID, installationID)
	if err != nil {
		return fmt.Errorf("postgres: UpsertInstallation: %w", err)
	}
	return nil
}

func (d *DB) GetInstallation(ctx context.Context, orgID string) (*githubapp.Installation, error) {
	var installation githubapp.Installation
	err := d.db.QueryRowContext(ctx, `
		SELECT org_id, github_installation_id, created_at FROM github_installations WHERE org_id=$1`, orgID).Scan(
		&installation.OrgID, &installation.GitHubInstallationID, &installation.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetInstallation: %w", err)
	}
	return &installation, nil
}

func (d *DB) DeleteInstallation(ctx context.Context, orgID string) error {
	result, err := d.db.ExecContext(ctx, `DELETE FROM github_installations WHERE org_id=$1`, orgID)
	if err != nil {
		return fmt.Errorf("postgres: DeleteInstallation: %w", err)
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (d *DB) CreateImport(ctx context.Context, orgID, teamID string, ownerID int64, repo string) (*githubapp.Import, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var teamName string
	if err = tx.QueryRowContext(ctx, `SELECT name FROM teams WHERE id=$1 AND org_id=$2`, teamID, orgID).Scan(&teamName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrTeamNotFound
		}
		return nil, err
	}
	importID := uuid.NewString()
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO repository_imports(id, org_id, github_owner_id, github_repo, team_id, status, branch)
		VALUES($1,$2,$3,$4,$5,$6,$7)`, importID, orgID, ownerID, repo, teamID,
		githubapp.StateSelected, githubapp.Branch(importID)); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO repository_import_jobs(org_id, import_id, kind) VALUES($1,$2,$3)`, orgID, importID, githubapp.JobStart); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return d.GetImport(ctx, orgID, importID)
}

const importSelect = `SELECT imp.id, imp.org_id, imp.github_owner_id, imp.github_repo, imp.team_id, t.name, imp.status,
	imp.steps, imp.branch, COALESCE(imp.run_id,0), COALESCE(imp.run_url,''), COALESCE(imp.pr_url,''),
	imp.missing_ai_configuration, COALESCE(imp.error,''), COALESCE(imp.service_id::text,''),
	imp.created_at, imp.updated_at, imp.run_started_at, imp.run_completed_at, imp.completed_at
	FROM repository_imports imp JOIN teams t ON t.id=imp.team_id`

func scanImport(row interface{ Scan(...any) error }) (*githubapp.Import, error) {
	var value githubapp.Import
	var status string
	var steps []byte
	err := row.Scan(&value.ID, &value.OrgID, &value.GitHubOwnerID, &value.GitHubRepo,
		&value.TeamID, &value.TeamName, &status, &steps, &value.Branch,
		&value.RunID, &value.RunURL, &value.PRURL,
		pq.Array(&value.MissingAIConfiguration), &value.Error, &value.ServiceID,
		&value.CreatedAt, &value.UpdatedAt, &value.RunStartedAt, &value.RunCompletedAt, &value.CompletedAt)
	if err != nil {
		return nil, err
	}
	value.Status = githubapp.State(status)
	if err := value.Status.Validate(); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(steps, &value.Steps); err != nil {
		return nil, fmt.Errorf("postgres: decode import steps: %w", err)
	}
	return &value, nil
}

func (d *DB) GetImport(ctx context.Context, orgID, importID string) (*githubapp.Import, error) {
	value, err := scanImport(d.db.QueryRowContext(ctx, importSelect+` WHERE imp.org_id=$1 AND imp.id=$2`, orgID, importID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetImport: %w", err)
	}
	return value, nil
}

func (d *DB) GetLatestImport(ctx context.Context, orgID string) (*githubapp.Import, error) {
	value, err := scanImport(d.db.QueryRowContext(ctx, importSelect+` WHERE imp.org_id=$1 ORDER BY imp.created_at DESC LIMIT 1`, orgID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetLatestImport: %w", err)
	}
	return value, nil
}

func (d *DB) SetImportStatus(ctx context.Context, orgID, importID string, status githubapp.State, missingAIConfiguration []string) error {
	if err := status.Validate(); err != nil {
		return err
	}
	_, err := d.db.ExecContext(ctx, `
		UPDATE repository_imports SET status=$3, missing_ai_configuration=COALESCE($4::text[],'{}'), updated_at=NOW()
		WHERE org_id=$1 AND id=$2`, orgID, importID, status, pq.Array(missingAIConfiguration))
	if err != nil {
		return fmt.Errorf("postgres: SetImportStatus: %w", err)
	}
	return nil
}

func (d *DB) SetImportRunQueued(ctx context.Context, orgID, importID string) error {
	_, err := d.db.ExecContext(ctx, `
		UPDATE repository_imports SET status=$3, missing_ai_configuration='{}', updated_at=NOW()
		WHERE org_id=$1 AND id=$2 AND status=$4`, orgID, importID, githubapp.StateRunQueued, githubapp.StateCheckingAI)
	if err != nil {
		return fmt.Errorf("postgres: SetImportRunQueued: %w", err)
	}
	return nil
}

func (d *DB) SetImportPullRequest(ctx context.Context, orgID, importID, url string) error {
	_, err := d.db.ExecContext(ctx, `
		UPDATE repository_imports SET pr_url=NULLIF($3,''), updated_at=NOW()
		WHERE org_id=$1 AND id=$2`, orgID, importID, url)
	if err != nil {
		return fmt.Errorf("postgres: SetImportPullRequest: %w", err)
	}
	return nil
}

func (d *DB) RetryImport(ctx context.Context, orgID, importID string, recheck bool) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM repository_imports WHERE org_id=$1 AND id=$2 FOR UPDATE`, orgID, importID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return store.ErrNotFound
	}
	if err != nil {
		return err
	}
	if recheck && status != string(githubapp.StateWaitingAI) {
		return store.ErrConflict
	}
	if !recheck && status != string(githubapp.StateWaitingAI) && status != string(githubapp.StateFailed) {
		return store.ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE repository_imports SET status=$3, error=NULL, run_id=NULL, run_url=NULL, steps='[]',
		run_started_at=NULL, run_completed_at=NULL, completed_at=NULL, updated_at=NOW()
		WHERE org_id=$1 AND id=$2`, orgID, importID, githubapp.StateCheckingAI); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO repository_import_jobs(org_id,import_id,kind) VALUES($1,$2,$3)
		ON CONFLICT(import_id,kind) WHERE completed_at IS NULL DO UPDATE SET available_at=NOW(), attempts=0, lease_owner=NULL, lease_expires_at=NULL, last_error=NULL`, orgID, importID, githubapp.JobStart)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (d *DB) ClaimJob(ctx context.Context, owner string, lease time.Duration) (*githubapp.Job, error) {
	var job githubapp.Job
	err := d.db.QueryRowContext(ctx, `
		WITH candidate AS (
			SELECT id FROM repository_import_jobs
			WHERE completed_at IS NULL AND available_at<=NOW() AND (lease_expires_at IS NULL OR lease_expires_at<NOW())
			ORDER BY available_at, created_at FOR UPDATE SKIP LOCKED LIMIT 1
		)
		UPDATE repository_import_jobs j SET lease_owner=$1, lease_expires_at=NOW()+$2::interval,
		attempts=attempts+1, updated_at=NOW() FROM candidate WHERE j.id=candidate.id
		RETURNING j.id,j.org_id,j.import_id,j.kind,j.attempts,j.max_attempts`, owner, fmt.Sprintf("%f seconds", lease.Seconds())).Scan(
		&job.ID, &job.OrgID, &job.ImportID, &job.Kind, &job.Attempts, &job.MaxAttempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: ClaimJob: %w", err)
	}
	return &job, nil
}

func (d *DB) CompleteJob(ctx context.Context, jobID, owner string) error {
	_, err := d.db.ExecContext(ctx, `UPDATE repository_import_jobs SET completed_at=NOW(), lease_owner=NULL, lease_expires_at=NULL, updated_at=NOW() WHERE id=$1 AND lease_owner=$2`, jobID, owner)
	return err
}

func (d *DB) RetryJob(ctx context.Context, job githubapp.Job, owner string, next time.Time, message string) error {
	if job.Attempts >= job.MaxAttempts {
		tx, err := d.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }()
		if _, err = tx.ExecContext(ctx, `UPDATE repository_import_jobs SET completed_at=NOW(), last_error=$3, lease_owner=NULL, lease_expires_at=NULL, updated_at=NOW() WHERE id=$1 AND lease_owner=$2`, job.ID, owner, message); err != nil {
			return err
		}
		if job.Kind != githubapp.JobStart {
			return tx.Commit()
		}
		_, err = tx.ExecContext(ctx, `UPDATE repository_imports SET status=$3, error=$4, updated_at=NOW() WHERE org_id=$1 AND id=$2`, job.OrgID, job.ImportID, githubapp.StateFailed, message)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	_, err := d.db.ExecContext(ctx, `UPDATE repository_import_jobs SET available_at=$3, last_error=$4, lease_owner=NULL, lease_expires_at=NULL, updated_at=NOW() WHERE id=$1 AND lease_owner=$2`, job.ID, owner, next, message)
	return err
}

func (d *DB) RecordWebhook(ctx context.Context, deliveryID, event, action string, installationID int64) (bool, error) {
	result, err := d.db.ExecContext(ctx, `INSERT INTO github_webhook_deliveries(delivery_id,event,action,installation_id) VALUES($1,$2,$3,NULLIF($4,0)) ON CONFLICT DO NOTHING`, deliveryID, event, action, installationID)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (d *DB) ApplyWorkflowRunEvent(ctx context.Context, run githubapp.WorkflowRun, repositoryURL string) error {
	if run.Event != "push" || run.Path != githubapp.OnboardingWorkflowPath {
		return nil
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var orgID, importID string
	var currentRunID int64
	err = tx.QueryRowContext(ctx, activeImportSelect+` FOR UPDATE`, run.HeadBranch).Scan(
		&orgID, &importID, &currentRunID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if run.ID < currentRunID {
		return nil
	}
	if run.Status != "completed" {
		_, err = tx.ExecContext(ctx, `UPDATE repository_imports SET status=$3,run_id=$4,run_url=$5,run_started_at=COALESCE(run_started_at,NOW()),updated_at=NOW() WHERE org_id=$1 AND id=$2`, orgID, importID, githubapp.StateRunRunning, run.ID, run.HTMLURL)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	if run.Conclusion != "success" {
		message := "UiGraph import run failed; verify the repository AI settings and review the run logs"
		_, err = tx.ExecContext(ctx, `UPDATE repository_imports SET status=$3,error=$4,run_id=$5,run_url=$6,run_completed_at=NOW(),updated_at=NOW() WHERE org_id=$1 AND id=$2`, orgID, importID, githubapp.StateFailed, message, run.ID, run.HTMLURL)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	var serviceID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM services WHERE org_id=$1 AND deleted_at IS NULL AND REGEXP_REPLACE(LOWER(TRIM(TRAILING '/' FROM git_repo_url)), '\.git$', '')=REGEXP_REPLACE(LOWER(TRIM(TRAILING '/' FROM $2)), '\.git$', '') ORDER BY created_at LIMIT 1`, orgID, repositoryURL).Scan(&serviceID)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `UPDATE repository_imports SET status=$3,error='Sync succeeded but no service matched the canonical repository URL',run_id=$4,run_url=$5,run_completed_at=NOW(),updated_at=NOW() WHERE org_id=$1 AND id=$2`, orgID, importID, githubapp.StateFailed, run.ID, run.HTMLURL)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE repository_imports SET status=$3,service_id=$4,run_id=$5,run_url=$6,run_completed_at=NOW(),completed_at=NOW(),updated_at=NOW() WHERE org_id=$1 AND id=$2`, orgID, importID, githubapp.StateCompleted, serviceID, run.ID, run.HTMLURL); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO repository_import_jobs(org_id,import_id,kind) VALUES($1,$2,$3) ON CONFLICT(import_id,kind) WHERE completed_at IS NULL DO NOTHING`, orgID, importID, githubapp.JobOpenPR); err != nil {
		return err
	}
	return tx.Commit()
}

// activeImportSelect resolves the branch UiGraph created to the import that owns
// it, and only while that import is still running. The branch embeds the import
// id, so it is unique on its own. Terminal imports match nothing, so runs the
// repository triggers afterwards — including every later uigraph-sync run — are
// ignored outright.
const activeImportSelect = `
	SELECT org_id,id,COALESCE(run_id,0) FROM repository_imports
	WHERE branch=$1 AND status NOT IN ('completed','failed')`

func (d *DB) ApplyWorkflowJobEvent(ctx context.Context, branch string, runID int64, steps []githubapp.Step) error {
	if len(steps) == 0 {
		return nil
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var orgID, importID string
	var currentRunID int64
	err = tx.QueryRowContext(ctx, activeImportSelect+` FOR UPDATE`, branch).Scan(
		&orgID, &importID, &currentRunID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if runID < currentRunID {
		return nil
	}
	var existing []byte
	if err = tx.QueryRowContext(ctx, `SELECT steps FROM repository_imports WHERE org_id=$1 AND id=$2`, orgID, importID).Scan(&existing); err != nil {
		return err
	}
	var current []githubapp.Step
	if err = json.Unmarshal(existing, &current); err != nil {
		return fmt.Errorf("postgres: decode import steps: %w", err)
	}
	incoming := make(map[int64]bool, len(steps))
	for _, step := range steps {
		incoming[step.JobID] = true
	}
	merged := make([]githubapp.Step, 0, len(current)+len(steps))
	for _, step := range current {
		if incoming[step.JobID] {
			continue
		}
		merged = append(merged, step)
	}
	merged = append(merged, steps...)
	sort.Slice(merged, func(a, b int) bool {
		if merged[a].JobID != merged[b].JobID {
			return merged[a].JobID < merged[b].JobID
		}
		return merged[a].Number < merged[b].Number
	})
	encoded, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE repository_imports SET steps=$3, run_id=$4, run_started_at=COALESCE(run_started_at,NOW()), updated_at=NOW()
		WHERE org_id=$1 AND id=$2`, orgID, importID, encoded, runID)
	if err != nil {
		return fmt.Errorf("postgres: ApplyWorkflowJobEvent: %w", err)
	}
	return tx.Commit()
}

const importServiceAccountName = "GitHub Import"

func (d *DB) CreateImportToken(ctx context.Context, orgID, repository string, expiresAt time.Time) (string, error) {
	var serviceAccountID string
	err := d.db.QueryRowContext(ctx, `SELECT id FROM service_accounts WHERE org_id=$1 AND name=$2 AND is_internal=FALSE AND deleted_at IS NULL ORDER BY created_at LIMIT 1`, orgID, importServiceAccountName).Scan(&serviceAccountID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("postgres: CreateImportToken: %w", err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		roleScopes := authz.ScopesForRole(authz.RoleEditor)
		scopes := make([]string, len(roleScopes))
		for index, scope := range roleScopes {
			scopes[index] = string(scope)
		}
		serviceAccountID = uuid.NewString()
		account := identity.ServiceAccount{
			ID:          serviceAccountID,
			OrgID:       orgID,
			Name:        importServiceAccountName,
			Description: "Token used by repository import artifact and sync workflows",
			Scopes:      scopes,
		}
		if err := d.CreateServiceAccount(ctx, account); err != nil {
			return "", err
		}
	}
	tokens, err := d.ListTokens(ctx, serviceAccountID)
	if err != nil {
		return "", err
	}
	for _, token := range tokens {
		if token.Name != repository || token.Revoked {
			continue
		}
		if err := d.RevokeToken(ctx, token.ID); err != nil {
			return "", err
		}
	}
	id, plaintext, hash, err := identity.Generate()
	if err != nil {
		return "", err
	}
	token := identity.Token{
		ID:               id,
		ServiceAccountID: serviceAccountID,
		Name:             repository,
		Prefix:           identity.Prefix(plaintext),
		Hash:             hash,
		ExpiresAt:        &expiresAt,
	}
	if err := d.CreateToken(ctx, token); err != nil {
		return "", err
	}
	return plaintext, nil
}
