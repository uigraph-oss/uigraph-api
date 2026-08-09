
DELETE FROM auth_providers;

ALTER TABLE auth_providers ADD COLUMN IF NOT EXISTS slug TEXT NOT NULL;

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
