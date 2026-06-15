CREATE TABLE service_dbs (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    service_id      UUID        NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    org_id          UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    db_name         TEXT        NOT NULL,
    db_type         TEXT        NOT NULL DEFAULT '',
    dialect         TEXT        NOT NULL DEFAULT '',
    schema_json     JSONB       NOT NULL DEFAULT '{}',
    source          TEXT,
    source_ts       TIMESTAMPTZ,
    created_by      UUID        NOT NULL,
    updated_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    deleted_by      UUID
);

CREATE INDEX idx_service_dbs_service ON service_dbs(service_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_service_dbs_org ON service_dbs(org_id) WHERE deleted_at IS NULL;

CREATE TABLE service_db_versions (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    service_db_id   UUID        NOT NULL REFERENCES service_dbs(id) ON DELETE CASCADE,
    version_number  INT         NOT NULL,
    label           TEXT,
    schema_json     JSONB       NOT NULL DEFAULT '{}',
    source          TEXT,
    source_ts       TIMESTAMPTZ,
    is_auto_version BOOLEAN     NOT NULL DEFAULT FALSE,
    created_by      UUID        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (service_db_id, version_number)
);

CREATE INDEX idx_service_db_versions_db ON service_db_versions(service_db_id);
