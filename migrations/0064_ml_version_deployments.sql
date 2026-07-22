CREATE TABLE ml_version_deployments (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id      UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    version_id  UUID        NOT NULL REFERENCES ml_model_versions(id) ON DELETE CASCADE,
    from_status TEXT        CHECK (from_status IN ('candidate','staging','production','retired')),
    to_status   TEXT        NOT NULL CHECK (to_status IN ('candidate','staging','production','retired')),
    changed_by  UUID        NOT NULL,
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_ml_version_deployments_version
    ON ml_version_deployments(version_id, changed_at DESC, id DESC);

INSERT INTO ml_version_deployments (id, org_id, version_id, from_status, to_status, changed_by, changed_at)
SELECT gen_random_uuid(), org_id, id, NULL, stage, created_by, created_at
FROM ml_model_versions
WHERE deleted_at IS NULL;

ALTER TABLE ml_model_versions DROP COLUMN status, DROP COLUMN stage;
