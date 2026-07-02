-- Service Catalog: services, API groups (with versioning), and API endpoints.

CREATE TABLE services (
    id                UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id            UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    folder_id         UUID        REFERENCES folders(id) ON DELETE SET NULL,
    team_id           UUID        REFERENCES teams(id)   ON DELETE SET NULL,
    name              TEXT        NOT NULL,
    slug              TEXT        NOT NULL,
    description       TEXT        NOT NULL DEFAULT '',
    status            TEXT        NOT NULL DEFAULT 'active'
                                  CHECK (status IN ('active','inactive','deprecated','development','maintenance')),
    tier              TEXT        NOT NULL DEFAULT 'tier3'
                                  CHECK (tier IN ('tier1','tier2','tier3','tier4')),
    category          TEXT        NOT NULL DEFAULT '',
    language          TEXT        NOT NULL DEFAULT '',
    git_repo_url      TEXT,
    jira_project_url  TEXT,
    slack_channel_url TEXT,
    last_commit_sha   TEXT,
    labels            TEXT[]      NOT NULL DEFAULT '{}',
    metadata          JSONB       NOT NULL DEFAULT '{}',
    created_by        UUID        NOT NULL,
    updated_by        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ,
    deleted_by        UUID,
    UNIQUE (org_id, slug)
);

CREATE INDEX idx_services_org    ON services(org_id)    WHERE deleted_at IS NULL;
CREATE INDEX idx_services_folder ON services(folder_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_services_team   ON services(team_id)   WHERE deleted_at IS NULL;

-- API groups: a versioned collection of endpoints for a service.
CREATE TABLE api_groups (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    service_id  UUID        NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    org_id      UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    version     TEXT        NOT NULL DEFAULT 'v1',
    label       TEXT,
    protocol    TEXT        NOT NULL DEFAULT 'REST'
                            CHECK (protocol IN ('REST','GraphQL','gRPC')),
    spec_key    TEXT,
    spec_hash   TEXT,
    created_by  UUID        NOT NULL,
    updated_by  UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    deleted_by  UUID
);

CREATE INDEX idx_api_groups_service ON api_groups(service_id) WHERE deleted_at IS NULL;

-- API group versions: immutable spec snapshots.
CREATE TABLE api_group_versions (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    api_group_id    UUID        NOT NULL REFERENCES api_groups(id) ON DELETE CASCADE,
    version_number  INT         NOT NULL,
    label           TEXT,
    spec_key        TEXT        NOT NULL,
    spec_hash       TEXT        NOT NULL,
    is_auto_version BOOLEAN     NOT NULL DEFAULT FALSE,
    created_by      UUID        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (api_group_id, version_number)
);

CREATE INDEX idx_api_group_versions_group ON api_group_versions(api_group_id);

-- API endpoints: individual operations within an API group.
CREATE TABLE api_endpoints (
    id           UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    api_group_id UUID        NOT NULL REFERENCES api_groups(id) ON DELETE CASCADE,
    service_id   UUID        NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    org_id       UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    operation_id TEXT        NOT NULL DEFAULT '',
    method       TEXT        NOT NULL DEFAULT '',
    path         TEXT        NOT NULL DEFAULT '',
    summary      TEXT        NOT NULL DEFAULT '',
    description  TEXT        NOT NULL DEFAULT '',
    tags         TEXT[]      NOT NULL DEFAULT '{}',
    parameters   JSONB       NOT NULL DEFAULT '[]',
    request_body JSONB       NOT NULL DEFAULT '{}',
    responses    JSONB       NOT NULL DEFAULT '{}',
    ord          DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_by   UUID        NOT NULL,
    updated_by   UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ,
    deleted_by   UUID
);

CREATE INDEX idx_api_endpoints_group   ON api_endpoints(api_group_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_api_endpoints_service ON api_endpoints(service_id)   WHERE deleted_at IS NULL;
