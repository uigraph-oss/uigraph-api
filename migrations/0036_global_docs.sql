-- Global docs: org-level documentation files, mirroring diagrams.
-- Bytes live in object storage; Postgres stores metadata + asset key.
-- service_docs is a junction (service_id, doc_id) mirroring service_diagrams.

CREATE TABLE docs (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id          UUID        NOT NULL REFERENCES orgs(id)    ON DELETE CASCADE,
    folder_id       UUID        REFERENCES folders(id)          ON DELETE SET NULL,
    team_id         UUID        REFERENCES teams(id)            ON DELETE SET NULL,
    file_asset_id   TEXT        NOT NULL,
    file_name       TEXT        NOT NULL,
    file_type       TEXT        NOT NULL DEFAULT 'application/octet-stream',
    description     TEXT        NOT NULL DEFAULT '',
    content_hash    TEXT        NOT NULL,
    doc_token_count INTEGER     NOT NULL DEFAULT 0,
    created_by      UUID        NOT NULL,
    updated_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    deleted_by      UUID
);

CREATE INDEX idx_docs_org    ON docs(org_id)    WHERE deleted_at IS NULL;
CREATE INDEX idx_docs_folder ON docs(folder_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_docs_team   ON docs(team_id)   WHERE deleted_at IS NULL;

DROP TABLE service_docs;

CREATE TABLE service_docs (
    service_id UUID        NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    doc_id     UUID        NOT NULL REFERENCES docs(id)     ON DELETE CASCADE,
    org_id     UUID        NOT NULL REFERENCES orgs(id)     ON DELETE CASCADE,
    created_by UUID        NOT NULL,
    updated_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    deleted_by UUID,
    PRIMARY KEY (service_id, doc_id)
);

CREATE INDEX idx_service_docs_service ON service_docs(service_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_service_docs_org     ON service_docs(org_id)     WHERE deleted_at IS NULL;
CREATE INDEX idx_service_docs_doc     ON service_docs(doc_id)     WHERE deleted_at IS NULL;
