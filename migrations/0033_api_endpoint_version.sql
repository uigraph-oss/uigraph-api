ALTER TABLE api_endpoints
    ADD COLUMN api_group_version_id UUID REFERENCES api_group_versions(id) ON DELETE CASCADE;

CREATE INDEX idx_api_endpoints_version ON api_endpoints(api_group_version_id) WHERE deleted_at IS NULL;
