-- ============================================================
-- Hidden service accounts — built-in "System Service" per org
-- Depends on: 0004_service_accounts.sql
-- ============================================================

ALTER TABLE service_accounts ADD COLUMN hidden BOOLEAN NOT NULL DEFAULT FALSE;
