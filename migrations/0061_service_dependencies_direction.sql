ALTER TABLE service_dependencies RENAME COLUMN source_service_id     TO service_id;
ALTER TABLE service_dependencies RENAME COLUMN provider_service_name TO dependency_name;

ALTER TABLE service_dependencies
    ADD COLUMN dependency_id UUID NULL REFERENCES services(id) ON DELETE SET NULL,
    ADD COLUMN direction     TEXT NOT NULL;

ALTER TABLE service_dependencies
    ADD CONSTRAINT service_dependencies_direction_check CHECK (direction IN ('upstream','downstream'));

DROP INDEX IF EXISTS idx_service_dependencies_source;
DROP INDEX IF EXISTS idx_service_dependencies_provider;

CREATE INDEX idx_service_dependencies_service    ON service_dependencies(service_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_service_dependencies_dependency ON service_dependencies(dependency_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_service_dependencies_name       ON service_dependencies(org_id, dependency_name) WHERE deleted_at IS NULL;
