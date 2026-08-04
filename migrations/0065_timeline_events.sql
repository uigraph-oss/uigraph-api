-- Timeline events per service: releases, decisions (ADRs), and incident
-- postmortems. This migration only supports manually-created events
-- (origin = 'manual'); origin = 'auto' is reserved for a future CLI
-- repo-scan sync and is not written by anything yet. touches is a JSONB
-- array of {id, label, kind} — free-text node/service references, not
-- foreign keys, since there's no generic node/service registry to link
-- against yet (mirrors the cost_resources.tags JSONB idiom).

CREATE TABLE timeline_events (
    id                     UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id                 UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    service_id             UUID        NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    type                   TEXT        NOT NULL CHECK (type IN ('release', 'decision', 'incident')),
    title                  TEXT        NOT NULL,
    summary                TEXT        NOT NULL DEFAULT '',
    event_date             TIMESTAMPTZ NOT NULL,
    version                TEXT,
    adr_number             TEXT,
    decision_status        TEXT        CHECK (decision_status IN ('proposed', 'accepted', 'superseded', 'deprecated')),
    source_label           TEXT,
    source_url             TEXT,
    is_agent_summarized    BOOLEAN     NOT NULL DEFAULT false,
    origin                 TEXT        NOT NULL DEFAULT 'manual' CHECK (origin IN ('auto', 'manual')),
    touches                JSONB       NOT NULL DEFAULT '[]',
    attachment_asset_id    TEXT,
    attachment_file_name   TEXT,
    attachment_file_type   TEXT,
    created_by             UUID        NOT NULL,
    updated_by             UUID,
    created_by_commit_hash TEXT,
    updated_by_commit_hash TEXT,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_timeline_events_service_date ON timeline_events(service_id, event_date DESC);
