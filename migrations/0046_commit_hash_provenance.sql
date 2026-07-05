-- Adds git commit-hash provenance columns, populated only by CLI sync writes.
-- created_by_commit_hash mirrors created_by; updated_by_commit_hash mirrors updated_by.

ALTER TABLE services            ADD COLUMN IF NOT EXISTS created_by_commit_hash TEXT;
ALTER TABLE services            ADD COLUMN IF NOT EXISTS updated_by_commit_hash TEXT;

ALTER TABLE maps                ADD COLUMN IF NOT EXISTS created_by_commit_hash TEXT;
ALTER TABLE maps                ADD COLUMN IF NOT EXISTS updated_by_commit_hash TEXT;

ALTER TABLE frames              ADD COLUMN IF NOT EXISTS created_by_commit_hash TEXT;
ALTER TABLE frames              ADD COLUMN IF NOT EXISTS updated_by_commit_hash TEXT;

ALTER TABLE focal_points        ADD COLUMN IF NOT EXISTS created_by_commit_hash TEXT;
ALTER TABLE focal_points        ADD COLUMN IF NOT EXISTS updated_by_commit_hash TEXT;

ALTER TABLE focal_point_meta    ADD COLUMN IF NOT EXISTS created_by_commit_hash TEXT;
ALTER TABLE focal_point_meta    ADD COLUMN IF NOT EXISTS updated_by_commit_hash TEXT;

ALTER TABLE service_diagrams    ADD COLUMN IF NOT EXISTS created_by_commit_hash TEXT;
ALTER TABLE service_diagrams    ADD COLUMN IF NOT EXISTS updated_by_commit_hash TEXT;

ALTER TABLE service_dbs         ADD COLUMN IF NOT EXISTS created_by_commit_hash TEXT;
ALTER TABLE service_dbs         ADD COLUMN IF NOT EXISTS updated_by_commit_hash TEXT;

ALTER TABLE service_db_versions ADD COLUMN IF NOT EXISTS created_by_commit_hash TEXT;
ALTER TABLE service_db_versions ADD COLUMN IF NOT EXISTS updated_by_commit_hash TEXT;

ALTER TABLE service_docs        ADD COLUMN IF NOT EXISTS created_by_commit_hash TEXT;
ALTER TABLE service_docs        ADD COLUMN IF NOT EXISTS updated_by_commit_hash TEXT;

ALTER TABLE api_groups          ADD COLUMN IF NOT EXISTS created_by_commit_hash TEXT;
ALTER TABLE api_groups          ADD COLUMN IF NOT EXISTS updated_by_commit_hash TEXT;

ALTER TABLE api_group_versions  ADD COLUMN IF NOT EXISTS created_by_commit_hash TEXT;
ALTER TABLE api_group_versions  ADD COLUMN IF NOT EXISTS updated_by_commit_hash TEXT;

ALTER TABLE api_endpoints       ADD COLUMN IF NOT EXISTS created_by_commit_hash TEXT;
ALTER TABLE api_endpoints       ADD COLUMN IF NOT EXISTS updated_by_commit_hash TEXT;

ALTER TABLE test_packs          ADD COLUMN IF NOT EXISTS created_by_commit_hash TEXT;
ALTER TABLE test_packs          ADD COLUMN IF NOT EXISTS updated_by_commit_hash TEXT;

ALTER TABLE test_cases          ADD COLUMN IF NOT EXISTS created_by_commit_hash TEXT;
ALTER TABLE test_cases          ADD COLUMN IF NOT EXISTS updated_by_commit_hash TEXT;
