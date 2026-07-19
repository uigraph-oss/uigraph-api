ALTER TABLE service_dependencies
    ADD COLUMN api_endpoint_names TEXT[] NOT NULL DEFAULT '{}';

UPDATE service_dependencies d
SET api_endpoint_names = COALESCE(
    (SELECT array_agg(name ORDER BY name)
     FROM service_dependency_api_endpoints
     WHERE dependency_id = d.id),
    '{}'
);

DROP TABLE service_dependency_api_endpoints;
