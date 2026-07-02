CREATE UNIQUE INDEX idx_services_org_name ON services(org_id, name) WHERE deleted_at IS NULL;
