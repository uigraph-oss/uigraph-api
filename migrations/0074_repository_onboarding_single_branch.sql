ALTER TABLE repository_onboardings RENAME COLUMN setup_branch TO branch;
ALTER TABLE repository_onboardings RENAME COLUMN generation_run_id TO run_id;
ALTER TABLE repository_onboardings RENAME COLUMN generation_run_url TO run_url;
ALTER TABLE repository_onboardings RENAME COLUMN artifacts_pr_url TO pr_url;

ALTER TABLE repository_onboardings
    DROP COLUMN failed_phase,
    DROP COLUMN artifacts_branch,
    DROP COLUMN artifacts_pr_number,
    DROP COLUMN setup_pr_number,
    DROP COLUMN setup_pr_url,
    DROP COLUMN sync_run_id,
    DROP COLUMN sync_run_url;

UPDATE repository_onboardings
SET branch='uigraph/onboarding/' || id
WHERE branch <> 'uigraph/onboarding/' || id;

UPDATE repository_onboardings
SET status='failed',
    error='Onboarding restarted on a single branch; retry this repository',
    updated_at=NOW()
WHERE status IN (
    'setup_pr_creating','setup_pr_open','waiting_setup_merge',
    'generation_queued','generation_running',
    'artifacts_pr_open','waiting_artifacts_merge',
    'sync_queued','sync_running');

DELETE FROM repository_onboarding_jobs
WHERE completed_at IS NULL AND kind IN ('setup_pr','check_ai','generation','find_artifacts','sync');

CREATE UNIQUE INDEX repository_onboardings_branch_idx ON repository_onboardings(org_id, branch);
