-- Cloud billing integration: org-scoped connections to AWS/Azure/GCP billing
-- accounts, service-level tag rules that determine which discovered cloud
-- resources roll up into a service's cost dashboard, and the resource/cost
-- data synced from each provider.

-- One row per connected cloud billing account. auth_payload is the
-- AES-256-GCM-encrypted JSON blob for that provider (AWS: role ARN +
-- external ID; GCP: service-account key; Azure: service-principal secret +
-- tenant/subscription IDs) — same crypto.Cipher used for Figma tokens.
CREATE TABLE cloud_connections (
    id                UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id            UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    provider          TEXT        NOT NULL CHECK (provider IN ('aws', 'azure', 'gcp')),
    display_name      TEXT        NOT NULL,
    auth_payload      TEXT        NOT NULL,
    status            TEXT        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'connected', 'syncing', 'error')),
    status_message     TEXT        NOT NULL DEFAULT '',
    last_verified_at   TIMESTAMPTZ,
    created_by        UUID        NOT NULL,
    updated_by        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ,
    UNIQUE (org_id, display_name)
);
CREATE INDEX idx_cloud_connections_org ON cloud_connections(org_id) WHERE deleted_at IS NULL;

-- A service is linked to real cloud resources by one or more tag rules
-- (key=value). A resource matches a service if any of that service's rules
-- matches one of the resource's real cloud tags. Kept separate from the
-- existing services.labels column, which is a free-text categorization
-- field the UI lets users edit already (configure-service-modal.tsx) and is
-- not meant to carry cloud-provider tag semantics.
CREATE TABLE service_cost_tag_rules (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id      UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    service_id  UUID        NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    tag_key     TEXT        NOT NULL,
    tag_value   TEXT        NOT NULL,
    created_by  UUID        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (service_id, tag_key, tag_value)
);
CREATE INDEX idx_service_cost_tag_rules_service ON service_cost_tag_rules(service_id);

-- Discovered cloud resources, synced from each cloud_connection. tags is
-- the resource's real provider tags as a JSON key/value map (matched
-- against service_cost_tag_rules to compute which service a resource
-- belongs to, and which of its tags did the matching).
CREATE TABLE cost_resources (
    id                    UUID          NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id                UUID          NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    cloud_connection_id   UUID          NOT NULL REFERENCES cloud_connections(id) ON DELETE CASCADE,
    external_resource_id  TEXT          NOT NULL,
    name                  TEXT          NOT NULL,
    resource_type         TEXT          NOT NULL,
    provider              TEXT          NOT NULL CHECK (provider IN ('aws', 'azure', 'gcp')),
    region                TEXT          NOT NULL DEFAULT '',
    environment           TEXT          NOT NULL DEFAULT '',
    status                TEXT          NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'stopped', 'degraded')),
    monthly_cost_usd      NUMERIC(14,4) NOT NULL DEFAULT 0,
    tags                  JSONB         NOT NULL DEFAULT '{}',
    last_synced_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    created_at            TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    UNIQUE (cloud_connection_id, external_resource_id)
);
CREATE INDEX idx_cost_resources_org ON cost_resources(org_id);
CREATE INDEX idx_cost_resources_connection ON cost_resources(cloud_connection_id);
CREATE INDEX idx_cost_resources_tags ON cost_resources USING GIN (tags);

-- Daily cost time series per resource, backing the trend chart. Kept at
-- resource grain (rather than pre-aggregated per service) so re-mapping a
-- service's tag rules recomputes history instead of requiring a re-sync.
CREATE TABLE cost_usage_daily (
    id          UUID          NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id      UUID          NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    resource_id UUID          NOT NULL REFERENCES cost_resources(id) ON DELETE CASCADE,
    usage_date  DATE          NOT NULL,
    cost_usd    NUMERIC(14,4) NOT NULL DEFAULT 0,
    UNIQUE (resource_id, usage_date)
);
CREATE INDEX idx_cost_usage_daily_org_date ON cost_usage_daily(org_id, usage_date);

-- Sync job run log, for surfacing last-sync status/errors per connection.
CREATE TABLE cost_sync_runs (
    id                  UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    cloud_connection_id UUID        NOT NULL REFERENCES cloud_connections(id) ON DELETE CASCADE,
    started_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at         TIMESTAMPTZ,
    status              TEXT        NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'success', 'error')),
    resource_count       INT,
    error_message        TEXT
);
CREATE INDEX idx_cost_sync_runs_connection ON cost_sync_runs(cloud_connection_id, started_at DESC);
