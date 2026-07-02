-- ============================================================
-- UIGraph Service Account Scopes — v1
-- PostgreSQL 15+
-- Depends on: 0004_service_accounts.sql
-- ============================================================
-- Replace the coarse org-level role on service accounts with an explicit
-- list of named permission scopes (e.g. 'diagrams:create'). Scopes are
-- enforced per request for service-account tokens.

ALTER TABLE service_accounts DROP COLUMN role;
ALTER TABLE service_accounts ADD COLUMN scopes TEXT[] NOT NULL DEFAULT '{}';
