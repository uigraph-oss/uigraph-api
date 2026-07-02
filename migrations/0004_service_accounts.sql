-- ============================================================
-- UIGraph Service Accounts Schema — v1
-- PostgreSQL 15+  |  module: github.com/uigraph/auth
-- Depends on: 0001_auth.sql, 0002_rbac.sql
-- ============================================================

-- ─── Service accounts ────────────────────────────────────
-- Non-human identities for CLI, CI/CD, and MCP clients.
-- Soft-deleted via deleted_at so token FK rows remain auditable.

CREATE TABLE service_accounts (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id      UUID        NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    description TEXT,
    role        TEXT        NOT NULL DEFAULT 'viewer',
    disabled    BOOLEAN     NOT NULL DEFAULT FALSE,
    created_by  UUID        REFERENCES users (id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    UNIQUE (org_id, name)
);

CREATE INDEX idx_service_accounts_org ON service_accounts (org_id) WHERE deleted_at IS NULL;

-- ─── Service account tokens ──────────────────────────────
-- Multiple tokens may belong to one service account.
-- prefix  — first 12 chars of plaintext (including "uig_"), indexed for fast lookup.
-- hash    — lower-case hex SHA-256 digest of the full plaintext; never the secret itself.
-- The raw token is shown exactly once at creation time.

CREATE TABLE service_account_tokens (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    service_account_id UUID        NOT NULL REFERENCES service_accounts (id) ON DELETE CASCADE,
    name               TEXT        NOT NULL,
    prefix             TEXT        NOT NULL,
    hash               TEXT        NOT NULL,
    expires_at         TIMESTAMPTZ,
    last_used_at       TIMESTAMPTZ,
    revoked            BOOLEAN     NOT NULL DEFAULT FALSE,
    created_by         UUID        REFERENCES users (id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (service_account_id, name),
    UNIQUE (prefix)
);

CREATE INDEX idx_sat_service_account ON service_account_tokens (service_account_id);
