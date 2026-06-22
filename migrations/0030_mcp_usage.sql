CREATE TABLE mcp_usage_events (
    id                    TEXT        NOT NULL PRIMARY KEY,
    org_id                UUID        NOT NULL REFERENCES orgs(id),
    user_id               UUID,
    service_account_id    UUID,
    tool_name             TEXT        NOT NULL,
    resource_ids          TEXT[]      NOT NULL DEFAULT '{}',
    model_id              TEXT        NOT NULL,
    tokens_served         INT         NOT NULL,
    tokens_raw_equivalent INT         NOT NULL,
    tokens_saved          INT         NOT NULL,
    response_size_bytes   INT         NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX mcp_usage_events_org_id_idx ON mcp_usage_events(org_id, created_at DESC);
