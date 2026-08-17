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

func (d *DB) UpsertInstallation(ctx context.Context, installation githubapp.Installation, repositories []githubapp.Repository) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var installationID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO github_installations(org_id, github_installation_id, account_id, account_login, account_type, target_type, status, suspended_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT(org_id) DO UPDATE SET github_installation_id=EXCLUDED.github_installation_id,
		account_id=EXCLUDED.account_id, account_login=EXCLUDED.account_login, account_type=EXCLUDED.account_type,
		target_type=EXCLUDED.target_type, status=EXCLUDED.status, suspended_at=EXCLUDED.suspended_at, updated_at=NOW()
		RETURNING id`, installation.OrgID, installation.GitHubInstallationID, installation.AccountID,
		installation.AccountLogin, installation.AccountType, installation.TargetType, installation.Status, installation.SuspendedAt).Scan(&installationID)
	if err != nil {
		return fmt.Errorf("postgres: UpsertInstallation: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE github_repositories SET selected=FALSE, updated_at=NOW() WHERE org_id=$1`, installation.OrgID); err != nil {
		return err
	}
	for _, repository := range repositories {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO github_repositories(org_id, installation_id, github_id, name, full_name, html_url, default_branch, private, archived, selected)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,TRUE)
			ON CONFLICT(org_id, github_id) DO UPDATE SET installation_id=EXCLUDED.installation_id, name=EXCLUDED.name,
			full_name=EXCLUDED.full_name, html_url=EXCLUDED.html_url, default_branch=EXCLUDED.default_branch,
			private=EXCLUDED.private, archived=EXCLUDED.archived, selected=TRUE, updated_at=NOW()`,
			installation.OrgID, installationID, repository.GitHubID, repository.Name, repository.FullName,
			repository.URL, repository.DefaultBranch, repository.Private, repository.Archived)
		if err != nil {
			return fmt.Errorf("postgres: UpsertInstallation repository: %w", err)
		}
	}
	return tx.Commit()
}

func scanInstallation(row interface{ Scan(...any) error }) (*githubapp.Installation, error) {
	var installation githubapp.Installation
	err := row.Scan(&installation.ID, &installation.OrgID, &installation.GitHubInstallationID, &installation.AccountID,
		&installation.AccountLogin, &installation.AccountType, &installation.TargetType, &installation.Status,
		&installation.SuspendedAt, &installation.CreatedAt, &installation.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &installation, err
}

func (d *DB) GetInstallation(ctx context.Context, orgID string) (*githubapp.Installation, error) {
	installation, err := scanInstallation(d.db.QueryRowContext(ctx, `
		SELECT id, org_id, github_installation_id, account_id, account_login, account_type, target_type,
		status, suspended_at, created_at, updated_at FROM github_installations WHERE org_id=$1`, orgID))
	if err != nil {
		return nil, fmt.Errorf("postgres: GetInstallation: %w", err)
	}
	return installation, nil
}

// DisconnectInstallation marks the connection as disconnected instead of
// deleting it. Deleting cascades into github_repositories, which import history
// references, and the services produced by past imports must outlive the
// connection that created them.
func (d *DB) DisconnectInstallation(ctx context.Context, orgID string) error {
	result, err := d.db.ExecContext(ctx, `UPDATE github_installations SET status='disconnected', updated_at=NOW() WHERE org_id=$1`, orgID)
	if err != nil {
		return fmt.Errorf("postgres: DisconnectInstallation: %w", err)
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return store.ErrNotFound
	}
	return nil
}

func scanRepository(row interface{ Scan(...any) error }) (githubapp.Repository, error) {
	var repository githubapp.Repository
	err := row.Scan(&repository.ID, &repository.OrgID, &repository.InstallationID, &repository.GitHubID,
		&repository.Name, &repository.FullName, &repository.URL, &repository.DefaultBranch, &repository.Private,
		&repository.Archived, &repository.Selected, &repository.UpdatedAt)
	return repository, err
}

func (d *DB) ListRepositories(ctx context.Context, orgID string) ([]githubapp.Repository, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, org_id, installation_id, github_id, name, full_name, html_url, default_branch, private, archived, selected, updated_at
		FROM github_repositories WHERE org_id=$1 AND selected=TRUE ORDER BY full_name`, orgID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListRepositories: %w", err)
	}
	defer rows.Close()
	var repositories []githubapp.Repository
	for rows.Next() {
		repository, err := scanRepository(rows)
		if err != nil {
			return nil, err
		}
		repositories = append(repositories, repository)
	}
	return repositories, rows.Err()
}

func (d *DB) CreateImport(ctx context.Context, orgID, teamID, createdBy, repositoryID string) (*githubapp.Import, error) {
	tx, err := d.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var teamName string
	if err = tx.QueryRowContext(ctx, `SELECT name FROM teams WHERE id=$1 AND org_id=$2`, teamID, orgID).Scan(&teamName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrTeamNotFound
		}
		return nil, err
	}
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM github_repositories WHERE org_id=$1 AND id=$2 AND selected=TRUE AND archived=FALSE)`, orgID, repositoryID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, store.ErrNotFound
	}
	importID := uuid.NewString()
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO repository_imports(id, org_id, repository_id, team_id, team_name, status, branch)
		VALUES($1,$2,$3,$4,$5,$6,$7)`, importID, orgID, repositoryID, teamID, teamName,
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

const importSelect = `SELECT o.id, o.org_id, o.repository_id, o.team_id, o.team_name, o.status,
	o.steps, o.branch, COALESCE(o.run_id,0), COALESCE(o.run_url,''), COALESCE(o.pr_url,''),
	o.missing_ai_configuration, COALESCE(o.error,''), COALESCE(o.service_id::text,''),
	o.created_at, o.updated_at, o.run_started_at, o.run_completed_at, o.completed_at,
	r.id, r.org_id, r.installation_id, r.github_id, r.name, r.full_name, r.html_url, r.default_branch,
	r.private, r.archived, r.selected, r.updated_at
	FROM repository_imports o JOIN github_repositories r ON r.org_id=o.org_id AND r.id=o.repository_id`

func scanImport(row interface{ Scan(...any) error }) (*githubapp.Import, error) {
	var value githubapp.Import
	var status string
	var steps []byte
	err := row.Scan(&value.ID, &value.OrgID, &value.RepositoryID,
		&value.TeamID, &value.TeamName, &status, &steps, &value.Branch,
		&value.RunID, &value.RunURL, &value.PRURL,
		pq.Array(&value.MissingAIConfiguration), &value.Error, &value.ServiceID,
		&value.CreatedAt, &value.UpdatedAt, &value.RunStartedAt, &value.RunCompletedAt, &value.CompletedAt,
		&value.Repository.ID, &value.Repository.OrgID, &value.Repository.InstallationID,
		&value.Repository.GitHubID, &value.Repository.Name, &value.Repository.FullName,
		&value.Repository.URL, &value.Repository.DefaultBranch, &value.Repository.Private,
		&value.Repository.Archived, &value.Repository.Selected, &value.Repository.UpdatedAt)
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
	value, err := scanImport(d.db.QueryRowContext(ctx, importSelect+` WHERE o.org_id=$1 AND o.id=$2`, orgID, importID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetImport: %w", err)
	}
	return value, nil
}

func (d *DB) GetLatestImport(ctx context.Context, orgID string) (*githubapp.Import, error) {
	value, err := scanImport(d.db.QueryRowContext(ctx, importSelect+` WHERE o.org_id=$1 ORDER BY o.created_at DESC LIMIT 1`, orgID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetLatestImport: %w", err)
	}
	return value, nil
}

func (d *DB) ListImports(ctx context.Context, orgID string) ([]githubapp.Import, error) {
	rows, err := d.db.QueryContext(ctx, importSelect+` WHERE o.org_id=$1 ORDER BY o.created_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListImports: %w", err)
	}
	defer rows.Close()
	imports := []githubapp.Import{}
	for rows.Next() {
		value, err := scanImport(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListImports: %w", err)
		}
		imports = append(imports, *value)
	}
	return imports, rows.Err()
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
	defer tx.Rollback()
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
		defer tx.Rollback()
		if _, err = tx.ExecContext(ctx, `UPDATE repository_import_jobs SET completed_at=NOW(), last_error=$3, lease_owner=NULL, lease_expires_at=NULL, updated_at=NOW() WHERE id=$1 AND lease_owner=$2`, job.ID, owner, message); err != nil {
			return err
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

func (d *DB) ApplyWorkflowRunEvent(ctx context.Context, installationID, repositoryID, runID int64, event, status, conclusion, headBranch, htmlURL, path string) error {
	if event != "push" {
		return nil
	}
	if path != githubapp.ArtifactWorkflowPath {
		return nil
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var orgID, importID, repositoryURL string
	var currentRunID int64
	err = tx.QueryRowContext(ctx, activeImportSelect+` FOR UPDATE OF o`, installationID, repositoryID, headBranch).Scan(
		&orgID, &importID, &currentRunID, &repositoryURL)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if runID < currentRunID {
		return nil
	}
	if status != "completed" {
		_, err = tx.ExecContext(ctx, `UPDATE repository_imports SET status=$3,run_id=$4,run_url=$5,run_started_at=COALESCE(run_started_at,NOW()),updated_at=NOW() WHERE org_id=$1 AND id=$2`, orgID, importID, githubapp.StateRunRunning, runID, htmlURL)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	if conclusion != "success" {
		message := "UiGraph import run failed; verify the repository AI settings and review the run logs"
		_, err = tx.ExecContext(ctx, `UPDATE repository_imports SET status=$3,error=$4,run_id=$5,run_url=$6,run_completed_at=NOW(),updated_at=NOW() WHERE org_id=$1 AND id=$2`, orgID, importID, githubapp.StateFailed, message, runID, htmlURL)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	var serviceID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM services WHERE org_id=$1 AND deleted_at IS NULL AND REGEXP_REPLACE(LOWER(TRIM(TRAILING '/' FROM git_repo_url)), '\.git$', '')=REGEXP_REPLACE(LOWER(TRIM(TRAILING '/' FROM $2)), '\.git$', '') ORDER BY created_at LIMIT 1`, orgID, repositoryURL).Scan(&serviceID)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `UPDATE repository_imports SET status=$3,error='Sync succeeded but no service matched the canonical repository URL',run_id=$4,run_url=$5,run_completed_at=NOW(),updated_at=NOW() WHERE org_id=$1 AND id=$2`, orgID, importID, githubapp.StateFailed, runID, htmlURL)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE repository_imports SET status=$3,service_id=$4,run_id=$5,run_url=$6,run_completed_at=NOW(),completed_at=NOW(),updated_at=NOW() WHERE org_id=$1 AND id=$2`, orgID, importID, githubapp.StateCompleted, serviceID, runID, htmlURL); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO repository_import_jobs(org_id,import_id,kind) VALUES($1,$2,$3) ON CONFLICT(import_id,kind) WHERE completed_at IS NULL DO NOTHING`, orgID, importID, githubapp.JobOpenPR); err != nil {
		return err
	}
	return tx.Commit()
}

// activeImportSelect resolves an installation + repository + branch to the
// import that owns it, and only while that import is still running. Terminal
// imports match nothing, so runs the repository triggers afterwards — including
// every later uigraph-sync run — are ignored outright.
const activeImportSelect = `
	SELECT o.org_id,o.id,COALESCE(o.run_id,0),r.html_url FROM repository_imports o
	JOIN github_repositories r ON r.id=o.repository_id AND r.org_id=o.org_id
	JOIN github_installations i ON i.id=r.installation_id
	WHERE i.github_installation_id=$1 AND r.github_id=$2 AND o.branch=$3
	AND o.status NOT IN ('completed','failed','cancelled')`

func (d *DB) ApplyWorkflowJobEvent(ctx context.Context, installationID, repositoryID, runID int64, headBranch string, steps []githubapp.Step) error {
	if len(steps) == 0 {
		return nil
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var orgID, importID, repositoryURL string
	var currentRunID int64
	err = tx.QueryRowContext(ctx, activeImportSelect+` FOR UPDATE OF o`, installationID, repositoryID, headBranch).Scan(
		&orgID, &importID, &currentRunID, &repositoryURL)
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
	// A run has several jobs and each webhook carries one of them, so replace
	// only the steps of the job that reported and keep the rest.
	merged := make([]githubapp.Step, 0, len(current)+len(steps))
	for _, step := range current {
		if step.JobID == steps[0].JobID {
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

func (d *DB) CreateOnboardingToken(ctx context.Context, orgID string, expiresAt time.Time) (string, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var serviceAccountID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM service_accounts WHERE org_id=$1 AND name='GitHub Onboarding' AND is_internal=TRUE AND deleted_at IS NULL FOR UPDATE`, orgID).Scan(&serviceAccountID)
	if errors.Is(err, sql.ErrNoRows) {
		serviceAccountID = uuid.NewString()
		scopes := make([]string, len(authz.AllScopes))
		for index, scope := range authz.AllScopes {
			scopes[index] = string(scope)
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO service_accounts(id,org_id,name,description,scopes,is_internal) VALUES($1,$2,'GitHub Onboarding','Token used by repository onboarding artifact and sync workflows',$3,TRUE)`, serviceAccountID, orgID, pq.Array(scopes))
	}
	if err != nil {
		return "", err
	}
	id, plaintext, hash, err := identity.Generate()
	if err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO service_account_tokens(id,service_account_id,name,prefix,hash,expires_at) VALUES($1,$2,$3,$4,$5,$6)`, id, serviceAccountID, "GitHub onboarding sync "+id, identity.Prefix(plaintext), hash, expiresAt); err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return plaintext, nil
}
