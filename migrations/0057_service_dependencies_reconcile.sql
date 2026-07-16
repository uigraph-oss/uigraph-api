DROP TABLE IF EXISTS service_dependency_operations;
DROP TABLE IF EXISTS service_dependency_api_endpoints;
DROP TABLE IF EXISTS service_dependencies;

CREATE TABLE service_dependencies (
    id                      UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    source_service_id       UUID        NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    org_id                  UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name                    TEXT        NOT NULL,
    provider_service_name   TEXT        NOT NULL,
    type                    TEXT        CHECK (type IS NULL OR type IN ('http','graphql','grpc','database')),
    criticality             TEXT        NOT NULL CHECK (criticality IN ('hard','soft')),
    description             TEXT        NOT NULL DEFAULT '',
    api_group_name          TEXT,
    database_name           TEXT,
    created_by              UUID        NOT NULL,
    updated_by              UUID,
    created_by_commit_hash  TEXT,
    updated_by_commit_hash  TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at              TIMESTAMPTZ,
    deleted_by              UUID,
    UNIQUE (source_service_id, name)
);

CREATE INDEX idx_service_dependencies_source ON service_dependencies(source_service_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_service_dependencies_provider ON service_dependencies(org_id, provider_service_name) WHERE deleted_at IS NULL;

CREATE TABLE service_dependency_api_endpoints (
    dependency_id UUID        NOT NULL REFERENCES service_dependencies(id) ON DELETE CASCADE,
    name          TEXT        NOT NULL,
    PRIMARY KEY (dependency_id, name)
);
