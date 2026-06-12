-- ============================================================
-- UIGraph Auth — track the active org per session
-- ============================================================
-- org_id stays as the login/home org (audit); active_org_id drives request
-- scope and is updated when the user switches organization. Backfilled to org_id
-- for existing sessions.

ALTER TABLE user_sessions
    ADD COLUMN active_org_id UUID REFERENCES orgs(id) ON DELETE SET NULL;

UPDATE user_sessions SET active_org_id = org_id WHERE active_org_id IS NULL;
