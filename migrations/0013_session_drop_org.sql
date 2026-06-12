-- ============================================================
-- UIGraph Auth — decouple sessions from organizations
-- ============================================================
-- A session authenticates a user, not an org. Request scope comes from the
-- {orgID} path param + membership checks, and the SPA tracks the active org
-- client-side. Drop the now-unused active_org_id and login/home org_id columns.
-- (0012 already dropped the org_id FK and NOT NULL constraint.)

ALTER TABLE user_sessions
    DROP COLUMN IF EXISTS active_org_id,
    DROP COLUMN IF EXISTS org_id;
