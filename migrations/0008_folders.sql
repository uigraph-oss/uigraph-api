-- Folders: org-scoped containers for services, diagrams, maps, and docs.
-- Supports nesting via parent_id (self-referencing FK).

CREATE TABLE folders (
    id         UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id     UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    parent_id  UUID        REFERENCES folders(id) ON DELETE CASCADE,
    type       TEXT        NOT NULL CHECK (type IN ('service','diagram','map','doc')),
    name       TEXT        NOT NULL,
    ord        DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_by UUID        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_folders_org_type ON folders(org_id, type) WHERE deleted_at IS NULL;
CREATE INDEX idx_folders_parent    ON folders(parent_id)   WHERE deleted_at IS NULL;
