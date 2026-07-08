-- Adds git commit-hash provenance columns to diagrams, populated only by sync writes.
-- created_by_commit_hash mirrors created_by; updated_by_commit_hash mirrors updated_by.

ALTER TABLE diagrams ADD COLUMN IF NOT EXISTS created_by_commit_hash TEXT;
ALTER TABLE diagrams ADD COLUMN IF NOT EXISTS updated_by_commit_hash TEXT;
