CREATE TABLE ml_projects (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id             UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name               TEXT        NOT NULL,
    type               TEXT        NOT NULL CHECK (type IN ('model','training')),
    description        TEXT        NOT NULL DEFAULT '',
    source_type        TEXT        NOT NULL DEFAULT '',
    source_url         TEXT        NOT NULL DEFAULT '',
    team               TEXT        NOT NULL DEFAULT '',
    email              TEXT        NOT NULL DEFAULT '',
    synced_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by         UUID        NOT NULL,
    updated_by         UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at         TIMESTAMPTZ,
    UNIQUE (org_id, name)
);
CREATE INDEX idx_ml_projects_org ON ml_projects(org_id) WHERE deleted_at IS NULL;

ALTER TABLE ml_models      ADD COLUMN project_id UUID REFERENCES ml_projects(id) ON DELETE SET NULL;
ALTER TABLE ml_experiments ADD COLUMN project_id UUID REFERENCES ml_projects(id) ON DELETE SET NULL;
