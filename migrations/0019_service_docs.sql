CREATE TABLE service_docs (
    id           UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    service_id   UUID        NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    org_id       UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    file_key     TEXT        NOT NULL,
    file_name    TEXT        NOT NULL,
    file_type    TEXT        NOT NULL DEFAULT 'application/octet-stream',
    description  TEXT        NOT NULL DEFAULT '',
    content_hash TEXT        NOT NULL,
    created_by   UUID        NOT NULL,
    updated_by   UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX idx_service_docs_service ON service_docs(service_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_service_docs_org ON service_docs(org_id) WHERE deleted_at IS NULL;
