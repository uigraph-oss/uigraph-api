-- Unify browser-facing blobs under a single public asset id.
-- Every public blob (frame screenshot, diagram thumbnail, diagram image) is
-- stored in object storage under "assets/<assetId>" and read directly by the
-- browser. Records carry only the asset id; the frontend builds the URL as
-- ${ASSETS_URL}/<assetId>.

-- Frames: the screenshot is addressed by a deterministic asset id (frame_<id>),
-- so the storage key is derivable and the old explicit key column is dropped.
ALTER TABLE frames DROP COLUMN screenshot_key;
ALTER TABLE frames ADD COLUMN screenshot_asset_id TEXT;

-- Diagrams: the thumbnail asset id is deterministic (diagram_<id>); keep a
-- content hash for cache-busting the public URL.
ALTER TABLE diagrams RENAME COLUMN preview_image_file_id TO preview_asset_id;
ALTER TABLE diagrams ADD COLUMN preview_content_hash TEXT;

-- Diagram images: random asset id (file_<uuid>) per image.
ALTER TABLE diagram_images RENAME COLUMN file_id TO asset_id;
