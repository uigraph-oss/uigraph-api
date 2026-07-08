ALTER TABLE mcp_usage_events
    ADD COLUMN client_name    TEXT,
    ADD COLUMN client_version TEXT;

ALTER TABLE mcp_usage_events
    ADD CONSTRAINT mcp_usage_events_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
    ADD CONSTRAINT mcp_usage_events_service_account_id_fkey
        FOREIGN KEY (service_account_id) REFERENCES service_accounts(id) ON DELETE SET NULL;
