-- ============================================================
-- UIGraph Per-Org Authentication — fixup for v67
--
-- 0067 was applied to some databases before its column set was final,
-- and schema_migrations makes it unrepeatable. This re-asserts the
-- intended shape idempotently: on a database created by the final 0067
-- every statement here is a no-op.
-- ============================================================

CREATE TABLE IF NOT EXISTS org_domains (
    id         UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id     UUID        NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    domain     TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, domain)
);

CREATE INDEX IF NOT EXISTS idx_org_domains_domain ON org_domains (domain);

CREATE TABLE IF NOT EXISTS auth_providers (
    id             UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id         UUID        NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    kind           TEXT        NOT NULL,
    display_name   TEXT        NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, display_name)
);

ALTER TABLE auth_providers
    ADD COLUMN IF NOT EXISTS type             TEXT    NOT NULL DEFAULT 'generic',
    ADD COLUMN IF NOT EXISTS icon_asset_id    TEXT,
    ADD COLUMN IF NOT EXISTS enabled          BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS allow_sign_up    BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS default_role     TEXT    NOT NULL DEFAULT 'viewer',
    ADD COLUMN IF NOT EXISTS allowed_domains  TEXT    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS client_id        TEXT,
    ADD COLUMN IF NOT EXISTS client_secret    TEXT,
    ADD COLUMN IF NOT EXISTS auth_url         TEXT,
    ADD COLUMN IF NOT EXISTS token_url        TEXT,
    ADD COLUMN IF NOT EXISTS userinfo_url     TEXT,
    ADD COLUMN IF NOT EXISTS api_url          TEXT,
    ADD COLUMN IF NOT EXISTS scopes           TEXT    NOT NULL DEFAULT 'openid profile email',
    ADD COLUMN IF NOT EXISTS email_claim      TEXT    NOT NULL DEFAULT 'email',
    ADD COLUMN IF NOT EXISTS name_claim       TEXT    NOT NULL DEFAULT 'name',
    ADD COLUMN IF NOT EXISTS sub_claim        TEXT    NOT NULL DEFAULT 'sub',
    ADD COLUMN IF NOT EXISTS groups_claim     TEXT    NOT NULL DEFAULT 'groups',
    ADD COLUMN IF NOT EXISTS idp_metadata_url TEXT,
    ADD COLUMN IF NOT EXISTS idp_metadata_xml TEXT,
    ADD COLUMN IF NOT EXISTS idp_entity_id    TEXT,
    ADD COLUMN IF NOT EXISTS idp_sso_url      TEXT,
    ADD COLUMN IF NOT EXISTS idp_cert         TEXT,
    ADD COLUMN IF NOT EXISTS sp_entity_id     TEXT,
    ADD COLUMN IF NOT EXISTS sp_cert          TEXT,
    ADD COLUMN IF NOT EXISTS sp_key           TEXT,
    ADD COLUMN IF NOT EXISTS sign_requests    BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS name_id_format   TEXT    NOT NULL DEFAULT 'urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress',
    ADD COLUMN IF NOT EXISTS email_attribute  TEXT    NOT NULL DEFAULT 'email',
    ADD COLUMN IF NOT EXISTS name_attribute   TEXT    NOT NULL DEFAULT 'displayName',
    ADD COLUMN IF NOT EXISTS groups_attribute TEXT    NOT NULL DEFAULT 'groups';

CREATE INDEX IF NOT EXISTS idx_auth_providers_org ON auth_providers (org_id);

CREATE TABLE IF NOT EXISTS auth_role_mappings (
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

ALTER TABLE auth_role_mappings
    ADD COLUMN IF NOT EXISTS priority        INT  NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS attribute_value TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_auth_role_mappings_provider ON auth_role_mappings (provider_id, priority);

CREATE TABLE IF NOT EXISTS auth_login_state (
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

ALTER TABLE auth_login_state
    ADD COLUMN IF NOT EXISTS nonce           TEXT,
    ADD COLUMN IF NOT EXISTS saml_request_id TEXT,
    ADD COLUMN IF NOT EXISTS redirect_to     TEXT;

CREATE INDEX IF NOT EXISTS idx_auth_login_state_expires ON auth_login_state (expires_at);
