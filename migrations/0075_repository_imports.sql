DELETE FROM repository_onboarding_jobs;
DELETE FROM repository_onboardings;
DELETE FROM repository_onboarding_batches;
DELETE FROM github_repositories;
DELETE FROM github_installations;

DROP TABLE repository_onboarding_batches CASCADE;

ALTER TABLE repository_onboardings DROP COLUMN batch_id;

ALTER TABLE repository_onboardings
    ADD COLUMN steps JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN run_started_at TIMESTAMPTZ,
    ADD COLUMN run_completed_at TIMESTAMPTZ;

ALTER TABLE repository_onboardings RENAME TO repository_imports;
ALTER INDEX repository_onboardings_repo_idx RENAME TO repository_imports_repo_idx;
ALTER INDEX repository_onboardings_branch_idx RENAME TO repository_imports_branch_idx;

ALTER TABLE repository_onboarding_jobs RENAME COLUMN onboarding_id TO import_id;
ALTER TABLE repository_onboarding_jobs RENAME TO repository_import_jobs;
ALTER INDEX repository_onboarding_jobs_active_idx RENAME TO repository_import_jobs_active_idx;
ALTER INDEX repository_onboarding_jobs_claim_idx RENAME TO repository_import_jobs_claim_idx;
