ALTER TABLE oauth_provider_config DROP COLUMN IF EXISTS icon_url;
ALTER TABLE oauth_provider_config ADD COLUMN IF NOT EXISTS icon_asset_id TEXT NOT NULL DEFAULT '';
