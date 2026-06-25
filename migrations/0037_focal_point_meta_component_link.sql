-- Replace the opaque colon-delimited component_link_id string (and the now-folded
-- component_flow_diagram column) with a structured component_link JSONB object whose
-- shape is chosen by the row's component_id. component_images is dropped: it was never
-- written or rendered.

ALTER TABLE focal_point_meta DROP COLUMN component_link_id;
ALTER TABLE focal_point_meta DROP COLUMN component_flow_diagram;
ALTER TABLE focal_point_meta DROP COLUMN component_images;
ALTER TABLE focal_point_meta ADD COLUMN component_link JSONB;

CREATE INDEX idx_focal_point_meta_component_link
    ON focal_point_meta USING GIN (component_link)
    WHERE deleted_at IS NULL;
