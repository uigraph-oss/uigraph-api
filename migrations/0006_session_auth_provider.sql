-- ============================================================
-- UIGraph Auth — record which provider created each session
-- ============================================================
-- 'password' for email/password logins; otherwise the OAuth provider instance
-- name (oauth_provider_config.provider_name) used to sign in.

ALTER TABLE user_sessions
    ADD COLUMN auth_provider TEXT NOT NULL DEFAULT 'password';
