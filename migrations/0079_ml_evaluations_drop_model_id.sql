-- Older databases carry a legacy NOT NULL model_id on ml_evaluations that the
-- current migrations no longer create. version_id is the source of truth for
-- which model an evaluation belongs to, so the column is dropped where present.

ALTER TABLE ml_evaluations DROP COLUMN IF EXISTS model_id;
