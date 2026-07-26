-- An evaluation is meaningless without the version it evaluates, so version_id
-- is required. Older databases may still allow NULL there; any orphaned rows
-- are removed before the constraint is enforced.

DELETE FROM ml_evaluations WHERE version_id IS NULL;

ALTER TABLE ml_evaluations ALTER COLUMN version_id SET NOT NULL;
