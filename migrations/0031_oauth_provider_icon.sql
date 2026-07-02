ALTER TABLE oauth_provider_config
    ADD COLUMN IF NOT EXISTS icon_url TEXT NOT NULL DEFAULT '';
