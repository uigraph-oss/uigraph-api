CREATE UNIQUE INDEX IF NOT EXISTS idx_service_dbs_service_name ON service_dbs(service_id, db_name) WHERE deleted_at IS NULL;
