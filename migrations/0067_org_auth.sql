-- ============================================================
-- UIGraph Per-Org Authentication — v67
-- Replaces instance-global SSO (0012_global_auth.sql) with per-org
-- OIDC/SAML providers, email-domain org discovery, and per-provider
-- role mapping rules.
--
-- Text columns hold the discriminators (kind, operator, role) and are
-- validated in Go; no Postgres enums, per AGENTS.md.
-- ============================================================

DROP TABLE IF EXISTS sso_role_mappings;
DROP TABLE IF EXISTS oauth_provider_config;
DROP TABLE IF EXISTS saml_config;

-- ─── Org email domains ───────────────────────────────────
-- Maps an email domain to the orgs that claim it, driving the org list
-- shown after the user enters their email.
-- Intentionally NOT unique on domain alone: one domain may belong to
-- several orgs. Domains are stored lowercased, without the '@'.

CREATE TABLE org_domains (
    id         UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id     UUID        NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    domain     TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, domain)
);

CREATE INDEX idx_org_domains_domain ON org_domains (domain);

-- ─── Auth providers ──────────────────────────────────────
-- One row per provider per org. An org may have many providers of either
-- kind. kind: 'oidc' | 'saml'. type (oidc only): 'generic' | 'entra' | 'okta'.
--
-- The kind-specific column groups are nullable; Go validates that the set
-- required by kind is present on write.
--
-- client_secret and sp_key are encrypted at rest (AES-256-GCM via
-- internal/crypto, key from UIGRAPH_SECRET_KEY).
--
-- default_role is applied when no role mapping rule matches.
--
-- allowed_domains is a comma-separated allowlist restricting which email
-- domains may authenticate through this provider; empty means no restriction.
-- It is distinct from org_domains, which drives org discovery.

CREATE TABLE auth_providers (
    id             UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id         UUID        NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    kind           TEXT        NOT NULL,
    type           TEXT        NOT NULL DEFAULT 'generic',
    display_name   TEXT        NOT NULL,
    icon_asset_id  TEXT,
    enabled        BOOLEAN     NOT NULL DEFAULT TRUE,
    allow_sign_up  BOOLEAN     NOT NULL DEFAULT TRUE,
    default_role   TEXT        NOT NULL DEFAULT 'viewer',
    allowed_domains TEXT       NOT NULL DEFAULT '',

    -- OIDC
    client_id      TEXT,
    client_secret  TEXT,
    auth_url       TEXT,
    token_url      TEXT,
    userinfo_url   TEXT,
    api_url        TEXT,
    scopes         TEXT        NOT NULL DEFAULT 'openid profile email',
    email_claim    TEXT        NOT NULL DEFAULT 'email',
    name_claim     TEXT        NOT NULL DEFAULT 'name',
    sub_claim      TEXT        NOT NULL DEFAULT 'sub',
    groups_claim   TEXT        NOT NULL DEFAULT 'groups',

    -- SAML
    idp_metadata_url TEXT,
    idp_metadata_xml TEXT,
    idp_entity_id    TEXT,
    idp_sso_url      TEXT,
    idp_cert         TEXT,
    sp_entity_id     TEXT,
    sp_cert          TEXT,
    sp_key           TEXT,
    sign_requests    BOOLEAN     NOT NULL DEFAULT FALSE,
    name_id_format   TEXT        NOT NULL DEFAULT 'urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress',
    email_attribute  TEXT        NOT NULL DEFAULT 'email',
    name_attribute   TEXT        NOT NULL DEFAULT 'displayName',
    groups_attribute TEXT        NOT NULL DEFAULT 'groups',

    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, display_name)
);

CREATE INDEX idx_auth_providers_org ON auth_providers (org_id);

-- ─── Role mapping rules ──────────────────────────────────
-- Ordered per provider. On login the rules are evaluated in ascending
-- priority and the first match decides the org role; if none match the
-- provider's default_role applies.
--
-- attribute_key addresses an OIDC claim or a SAML attribute, and supports
-- dot notation for nested claims (e.g. 'user.role').
-- operator: equals | notEquals | contains | notContains | startsWith |
--           endsWith | exists | notExists | regex
-- attribute_value is unused by the exists / notExists operators.
-- role: admin | editor | viewer

CREATE TABLE auth_role_mappings (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    provider_id     UUID        NOT NULL REFERENCES auth_providers (id) ON DELETE CASCADE,
    priority        INT         NOT NULL DEFAULT 0,
    attribute_key   TEXT        NOT NULL,
    operator        TEXT        NOT NULL,
    attribute_value TEXT        NOT NULL DEFAULT '',
    role            TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_auth_role_mappings_provider ON auth_role_mappings (provider_id, priority);

-- ─── Login handshake state ───────────────────────────────
-- Correlates a login redirect with the org and provider it started from.
-- Replaces the previous uigraph_oauth_state cookie for two reasons:
--   1. The SAML ACS endpoint is a cross-site POST from the IdP, and the
--      state cookie is SameSite=Lax, so it would not be sent. RelayState
--      plus this server-side row is the only correct correlation.
--   2. The chosen org must survive the round trip, which the CSRF cookie
--      alone cannot carry.
--
-- state doubles as the OIDC state parameter and the SAML RelayState.
-- Rows are single-use: consumed (deleted) on callback. Expired rows are
-- purged opportunistically on write.

CREATE TABLE auth_login_state (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    state           TEXT        NOT NULL UNIQUE,
    org_id          UUID        NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    provider_id     UUID        NOT NULL REFERENCES auth_providers (id) ON DELETE CASCADE,
    nonce           TEXT,
    saml_request_id TEXT,
    redirect_to     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_auth_login_state_expires ON auth_login_state (expires_at);
