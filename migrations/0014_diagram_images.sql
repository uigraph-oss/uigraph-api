-- Diagram images: user-uploaded images attached to a diagram. The binary lives
-- in object storage (key derived from org/diagram/file id); this row holds the
-- metadata and display order.

CREATE TABLE diagram_images (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    diagram_id  UUID        NOT NULL REFERENCES diagrams(id) ON DELETE CASCADE,
    org_id      UUID        NOT NULL REFERENCES orgs(id)     ON DELETE CASCADE,
    file_id     TEXT        NOT NULL,
    file_name   TEXT,
    "order"     INTEGER     NOT NULL DEFAULT 0,
    created_by  UUID        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_diagram_images_diagram ON diagram_images(diagram_id);
