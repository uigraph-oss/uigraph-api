-- Replace the polymorphic component_link JSONB object with one explicit, typed id column
-- per link type. Only the column matching the row's component_id is ever set. The extra
-- ids the JSON carried (serviceId, apiGroupId) are dropped: they only existed for URL
-- building and are now resolved on demand from the leaf id.

DROP INDEX idx_focal_point_meta_component_link;
ALTER TABLE focal_point_meta DROP COLUMN component_link;

ALTER TABLE focal_point_meta ADD COLUMN component_link_diagram_id      UUID;
ALTER TABLE focal_point_meta ADD COLUMN component_link_api_endpoint_id UUID;
ALTER TABLE focal_point_meta ADD COLUMN component_link_test_pack_id    UUID;
ALTER TABLE focal_point_meta ADD COLUMN component_link_service_doc_id  UUID;

CREATE INDEX idx_focal_point_meta_diagram      ON focal_point_meta (component_link_diagram_id)      WHERE deleted_at IS NULL;
CREATE INDEX idx_focal_point_meta_api_endpoint ON focal_point_meta (component_link_api_endpoint_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_focal_point_meta_test_pack    ON focal_point_meta (component_link_test_pack_id)    WHERE deleted_at IS NULL;
CREATE INDEX idx_focal_point_meta_service_doc  ON focal_point_meta (component_link_service_doc_id)  WHERE deleted_at IS NULL;
