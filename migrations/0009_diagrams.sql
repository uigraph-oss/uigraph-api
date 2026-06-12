-- Diagrams: metadata only. Content (ReactFlow JSON) lives in object storage.
-- content_key  = object storage key for current content
-- content_hash = SHA-256 used to skip no-op syncs from the CLI

CREATE TABLE diagrams (
    id                    UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id                UUID        NOT NULL REFERENCES orgs(id)    ON DELETE CASCADE,
    folder_id             UUID        REFERENCES folders(id)          ON DELETE SET NULL,
    team_id               UUID        REFERENCES teams(id)            ON DELETE SET NULL,
    name                  TEXT        NOT NULL,
    content_key           TEXT,
    content_hash          TEXT,
    preview_image_file_id TEXT,
    source                TEXT,
    created_by            UUID        NOT NULL,
    updated_by            UUID,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at            TIMESTAMPTZ,
    deleted_by            UUID
);

CREATE INDEX idx_diagrams_org    ON diagrams(org_id)    WHERE deleted_at IS NULL;
CREATE INDEX idx_diagrams_folder ON diagrams(folder_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_diagrams_team   ON diagrams(team_id)   WHERE deleted_at IS NULL;

-- Version snapshots. Each row points to an immutable object in storage.
CREATE TABLE diagram_versions (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    diagram_id      UUID        NOT NULL REFERENCES diagrams(id) ON DELETE CASCADE,
    version_number  INTEGER     NOT NULL,
    label           TEXT,
    content_key     TEXT        NOT NULL,
    content_hash    TEXT        NOT NULL,
    is_auto_version BOOLEAN     NOT NULL DEFAULT FALSE,
    source          TEXT,
    created_by      UUID        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (diagram_id, version_number)
);

CREATE INDEX idx_diagram_versions_diagram ON diagram_versions(diagram_id);
