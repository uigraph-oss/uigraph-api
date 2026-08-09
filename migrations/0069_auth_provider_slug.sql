-- ============================================================
-- UIGraph Auth Provider Slugs — v69
--
-- Providers are addressed in URLs by a slug scoped to their org, never
-- by their UUID. The UUID stays the primary key and the FK target for
-- role mappings and login state; it just never appears in a URL.
--
-- The slug is typed by the admin and validated in Go. Nothing derives
-- it from the display name, so existing rows cannot be backfilled and
-- are removed: the feature is unreleased and its providers must be
-- re-created with an explicit slug.
-- ============================================================

DELETE FROM auth_providers;

ALTER TABLE auth_providers ADD COLUMN IF NOT EXISTS slug TEXT NOT NULL;

-- sp_entity_id stored the SP metadata URL, which embedded the provider
-- UUID and so goes stale with this change. It is derived from the org
-- and slug at read time instead.
ALTER TABLE auth_providers DROP COLUMN IF EXISTS sp_entity_id;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'auth_providers_org_slug_key'
    ) THEN
        ALTER TABLE auth_providers
            ADD CONSTRAINT auth_providers_org_slug_key UNIQUE (org_id, slug);
    END IF;
END $$;
