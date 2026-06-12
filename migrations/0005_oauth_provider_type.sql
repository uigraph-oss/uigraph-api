-- ============================================================
-- UIGraph Auth — OAuth provider type + display name
-- ============================================================
-- provider_name stays the per-org unique instance slug used in login URLs.
-- type drives endpoint derivation (generic = explicit URLs; entra/okta derived
-- from api_url at save time). display_name is the label shown on the login page.

ALTER TABLE oauth_provider_config
    ADD COLUMN type         TEXT NOT NULL DEFAULT 'generic',
    ADD COLUMN display_name TEXT NOT NULL DEFAULT '';
