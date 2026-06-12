-- ============================================================
-- UIGraph Schema Bootstrap — v1
-- PostgreSQL 16+
-- Run first. All subsequent migrations depend on this.
-- ============================================================

-- ─── Migration state tracker ─────────────────────────────
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT        NOT NULL PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
