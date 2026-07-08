-- Adds git commit-hash provenance columns to saved_queries, populated only by
-- CLI sync writes (source='ci'). created_by_commit_hash mirrors created_by;
-- updated_by_commit_hash mirrors updated_by. UI-created rows leave these NULL.

ALTER TABLE saved_queries ADD COLUMN IF NOT EXISTS created_by_commit_hash TEXT;
ALTER TABLE saved_queries ADD COLUMN IF NOT EXISTS updated_by_commit_hash TEXT;
