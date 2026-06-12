-- ============================================================
-- UIGraph Auth Schema — v1
-- PostgreSQL 15+  |  module: github.com/uigraph/auth
-- ============================================================

-- ─── Orgs ────────────────────────────────────────────────
-- Top-level tenant. All other tables hang off org_id.
-- slug is the URL-friendly identifier used in routes and subdomains.

CREATE TABLE orgs (
    id         UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    name       TEXT        NOT NULL,
    slug       TEXT        NOT NULL UNIQUE,
    disabled   BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Users ───────────────────────────────────────────────
-- Global identity record — one row per person, not per org.
-- Org membership is tracked in org_members (0002_rbac.sql).
-- A user logging into two orgs shares this row; sessions are org-scoped.

CREATE TABLE users (
    id                   UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    email                TEXT        NOT NULL UNIQUE,
    name                 TEXT        NOT NULL,
    login                TEXT        NOT NULL UNIQUE,
    password_hash        TEXT,                          -- NULL = SSO-only account
    must_change_password BOOLEAN     NOT NULL DEFAULT FALSE,
    disabled             BOOLEAN     NOT NULL DEFAULT FALSE,
    last_seen_at         TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users (email) WHERE disabled = FALSE;

-- ─── External identity links ─────────────────────────────
-- One row per (user, provider) pair. Links local user to IdP subject.
-- No org_id — an IdP identity is global; the same sub maps to one user
-- regardless of how many orgs that user belongs to.
-- OAuth tokens stored encrypted at rest (AES-256-GCM, key from UIGRAPH_SECRET_KEY).
-- provider examples: 'generic_oauth', 'github', 'google'

CREATE TABLE user_auth (
    id               UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id          UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider         TEXT        NOT NULL,
    provider_sub     TEXT        NOT NULL,  -- stable subject ID from IdP (sub claim)
    access_token     TEXT,                  -- encrypted at rest
    refresh_token    TEXT,                  -- encrypted at rest
    id_token         TEXT,                  -- OIDC id_token; used for logout and claim re-reads
    token_type       TEXT,                  -- usually 'Bearer'
    token_expires_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_sub)
);

CREATE INDEX idx_user_auth_user ON user_auth (user_id);

-- ─── Sessions ────────────────────────────────────────────
-- One active row per browser session. Sessions are org-scoped so a user
-- logged into two orgs simultaneously has two rows.
-- Revocation = delete the row. No blocklist needed.
--
-- Rotation protocol (mirrors Grafana):
--   1. On rotation: write new token_hash, move old value to prev_token_hash,
--      set auth_token_seen = FALSE, update rotated_at.
--   2. On next request with new token: set auth_token_seen = TRUE, seen_at = NOW().
--   3. prev_token_hash is accepted only while auth_token_seen = FALSE, giving
--      concurrent tabs a grace window to catch up without a 401.

CREATE TABLE user_sessions (
    id                  UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id             UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    org_id              UUID        NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    token_hash          TEXT        NOT NULL UNIQUE,
    prev_token_hash     TEXT,
    auth_token_seen     BOOLEAN     NOT NULL DEFAULT FALSE,
    seen_at             TIMESTAMPTZ,
    user_agent          TEXT,
    client_ip           TEXT,
    rotated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at          TIMESTAMPTZ NOT NULL,
    last_active_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- SAML Single Logout: IdP logout request carries NameID + SessionIndex;
    -- we hash NameID for storage and index both to find the session quickly.
    saml_session_index  TEXT,
    saml_name_id_hash   TEXT
);

CREATE INDEX idx_user_sessions_user ON user_sessions (user_id);
CREATE INDEX idx_user_sessions_hash ON user_sessions (token_hash);

-- ─── OAuth provider config ───────────────────────────────
-- One row per (org, provider). An org can have multiple OAuth providers active.
-- provider_name: github | gitlab | google | generic_oauth | azure_ad | okta
--
-- auth_url / token_url / userinfo_url: for known providers the app pre-populates
-- these; they must be set explicitly for generic_oauth or self-hosted instances.
--
-- api_url: provider-specific base URL —
--   github/gitlab self-hosted → instance root  (e.g. https://gitlab.example.com)
--   okta            → Okta domain base         (e.g. https://company.okta.com)
--   azure_ad        → tenant base              (e.g. https://login.microsoftonline.com/{tenant})
--
-- allowed_domains: optional comma-separated list; restricts sign-in to those
-- email domains (e.g. "company.com,subsidiary.com"). Empty = unrestricted.
--
-- client_secret stored encrypted at rest (AES-256-GCM, key from UIGRAPH_SECRET_KEY).

CREATE TABLE oauth_provider_config (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id          UUID        NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    provider_name   TEXT        NOT NULL,
    client_id       TEXT        NOT NULL,
    client_secret   TEXT        NOT NULL,  -- encrypted at rest
    auth_url        TEXT        NOT NULL,
    token_url       TEXT        NOT NULL,
    userinfo_url    TEXT        NOT NULL,
    api_url         TEXT,                  -- base URL for self-hosted / tenant / domain
    scopes          TEXT        NOT NULL DEFAULT 'openid profile email',
    allowed_domains TEXT,                  -- comma-separated; empty = unrestricted
    allow_sign_up   BOOLEAN     NOT NULL DEFAULT TRUE,
    email_claim     TEXT        NOT NULL DEFAULT 'email',
    name_claim      TEXT        NOT NULL DEFAULT 'name',
    sub_claim       TEXT        NOT NULL DEFAULT 'sub',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, provider_name)
);

-- ─── LDAP config ─────────────────────────────────────────
-- One row per org. LDAP/Active Directory authentication.
-- bind_password stored encrypted at rest.
-- Anonymous bind: leave bind_dn and bind_password NULL.
--
-- search_filter uses %s as the username placeholder, e.g.:
--   OpenLDAP:        (uid=%s)
--   Active Directory: (sAMAccountName=%s)

CREATE TABLE ldap_config (
    id                  UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id              UUID        NOT NULL UNIQUE REFERENCES orgs (id) ON DELETE CASCADE,
    host                TEXT        NOT NULL,
    port                INT         NOT NULL DEFAULT 636,
    use_ssl             BOOLEAN     NOT NULL DEFAULT TRUE,
    start_tls           BOOLEAN     NOT NULL DEFAULT FALSE,
    skip_tls_verify     BOOLEAN     NOT NULL DEFAULT FALSE,
    bind_dn             TEXT,                                  -- NULL = anonymous bind
    bind_password       TEXT,                                  -- encrypted at rest
    search_base_dn      TEXT        NOT NULL,                  -- e.g. 'dc=example,dc=com'
    search_filter       TEXT        NOT NULL DEFAULT '(uid=%s)',
    email_attribute     TEXT        NOT NULL DEFAULT 'mail',
    name_attribute      TEXT        NOT NULL DEFAULT 'cn',
    username_attribute  TEXT        NOT NULL DEFAULT 'uid',
    member_of_attribute TEXT        NOT NULL DEFAULT 'memberOf',
    allow_sign_up       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── LDAP group → role mappings ──────────────────────────
-- Maps an LDAP group DN to a UIGraph role within an org.
-- Evaluated on every LDAP login (analogous to sso_role_mappings for OAuth).
-- group_dn example: 'cn=uigraph-admins,ou=groups,dc=example,dc=com'

CREATE TABLE ldap_group_mappings (
    id         UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id     UUID        NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    group_dn   TEXT        NOT NULL,
    role       TEXT        NOT NULL,  -- admin | editor | viewer
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, group_dn)
);

CREATE INDEX idx_ldap_group_mappings_org ON ldap_group_mappings (org_id);

-- ─── Login attempts ──────────────────────────────────────
-- Tracks failed login attempts for brute-force rate limiting.
-- Query pattern: count rows WHERE username = ? AND created_at > NOW() - interval.
-- Old rows are safe to purge periodically.

CREATE TABLE login_attempts (
    id         BIGSERIAL   PRIMARY KEY,
    username   TEXT        NOT NULL,
    ip_address TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_login_attempts_username ON login_attempts (username, created_at);

-- ─── Org invitations ─────────────────────────────────────
-- Pending invitations sent via email before the invitee has a users row.
-- On accept: create/lookup users row, insert org_members row, delete this row.
-- status: pending | accepted | revoked

CREATE TABLE org_invitations (
    id           UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id       UUID        NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    email        TEXT        NOT NULL,
    role         TEXT        NOT NULL,
    code         TEXT        NOT NULL UNIQUE,  -- random token sent in the invite email
    invited_by   UUID        REFERENCES users (id) ON DELETE SET NULL,
    status       TEXT        NOT NULL DEFAULT 'pending',
    expires_at   TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, email)
);

CREATE INDEX idx_org_invitations_code ON org_invitations (code);

-- ─── SAML config ─────────────────────────────────────────
-- One row per org. SP-initiated SSO via SAML 2.0.
--
-- IdP metadata: set idp_metadata_url (fetched at login) or paste
-- idp_metadata_xml directly when the IdP has no public metadata URL.
-- idp_entity_id / idp_sso_url / idp_cert are parsed from metadata on save
-- so the login path never needs to re-parse XML.
--
-- sp_key is the SP's private signing key, encrypted at rest.
-- sign_requests = TRUE requires the IdP to validate SP request signatures.
--
-- groups_attribute: SAML attribute whose value is a list of group names;
-- fed into sso_role_mappings exactly like OAuth claim groups — no separate
-- role-mapping table needed.

CREATE TABLE saml_config (
    id               UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id           UUID        NOT NULL UNIQUE REFERENCES orgs (id) ON DELETE CASCADE,
    idp_metadata_url TEXT,
    idp_metadata_xml TEXT,
    idp_entity_id    TEXT,
    idp_sso_url      TEXT,
    idp_cert         TEXT,
    sp_entity_id     TEXT        NOT NULL,
    sp_cert          TEXT,
    sp_key           TEXT,                  -- encrypted at rest
    sign_requests    BOOLEAN     NOT NULL DEFAULT FALSE,
    name_id_format   TEXT        NOT NULL DEFAULT 'urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress',
    email_attribute  TEXT        NOT NULL DEFAULT 'email',
    name_attribute   TEXT        NOT NULL DEFAULT 'displayName',
    login_attribute  TEXT        NOT NULL DEFAULT 'uid',
    groups_attribute TEXT,
    allow_sign_up    BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── SCIM config ─────────────────────────────────────────
-- One row per org. Enables IdP-driven user/group provisioning.
--
-- The IdP (Okta, Entra ID, etc.) calls UIGraph's SCIM API with a Bearer
-- token; we store its SHA-256 hash here — never the plaintext.
--
-- sync_users  = TRUE: IdP can create/update/deactivate users in this org.
-- sync_groups = TRUE: IdP can create/update/delete groups; each SCIM group
--   maps to a teams row (tracked via teams.external_id).
--
-- SCIM-provisioned users get a user_auth row with provider = 'scim' and
-- provider_sub = the SCIM externalId — no additional table needed.

CREATE TABLE scim_config (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id             UUID        NOT NULL UNIQUE REFERENCES orgs (id) ON DELETE CASCADE,
    enabled            BOOLEAN     NOT NULL DEFAULT TRUE,
    bearer_token_hash  TEXT        NOT NULL,  -- SHA-256 of the token the IdP sends
    sync_users         BOOLEAN     NOT NULL DEFAULT TRUE,
    sync_groups        BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
