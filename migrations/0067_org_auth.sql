
DROP TABLE IF EXISTS sso_role_mappings;
DROP TABLE IF EXISTS oauth_provider_config;
DROP TABLE IF EXISTS saml_config;


CREATE TABLE org_domains (
    id         UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id     UUID        NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    domain     TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, domain)
);

CREATE INDEX idx_org_domains_domain ON org_domains (domain);


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
