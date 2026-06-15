CREATE TABLE service_diagrams (
    service_id   UUID        NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    diagram_id   UUID        NOT NULL REFERENCES diagrams(id) ON DELETE CASCADE,
    org_id       UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    created_by   UUID        NOT NULL,
    updated_by   UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ,
    deleted_by   UUID,
    PRIMARY KEY (service_id, diagram_id)
);

CREATE INDEX idx_service_diagrams_service ON service_diagrams(service_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_service_diagrams_org ON service_diagrams(org_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_service_diagrams_diagram ON service_diagrams(diagram_id) WHERE deleted_at IS NULL;
