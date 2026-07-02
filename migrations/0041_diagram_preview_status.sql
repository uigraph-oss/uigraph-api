-- Track the lifecycle of a diagram's preview screenshot generation.
-- pending  = a screenshot job has been enqueued and not yet completed
-- success  = the worker rendered and stored the thumbnail
-- failed   = the worker failed to render or store the thumbnail
ALTER TABLE diagrams ADD COLUMN preview_status TEXT NOT NULL DEFAULT 'pending';

-- Backfill: diagrams that already have a thumbnail are considered successful.
UPDATE diagrams SET preview_status = 'success' WHERE preview_asset_id IS NOT NULL;
