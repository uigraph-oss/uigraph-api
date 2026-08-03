CREATE TABLE agent_sessions (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id             UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    type               TEXT        NOT NULL,
    status             TEXT        NOT NULL,
    user_id            UUID        REFERENCES users(id) ON DELETE SET NULL,
    service_account_id UUID        REFERENCES service_accounts(id) ON DELETE SET NULL,
    title              TEXT,
    model_name         TEXT,
    metadata           JSONB,
    report             TEXT,
    error              TEXT,
    started_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at       TIMESTAMPTZ
);

CREATE INDEX idx_agent_sessions_org      ON agent_sessions(org_id, started_at DESC);
CREATE INDEX idx_agent_sessions_org_type ON agent_sessions(org_id, type, started_at DESC);

CREATE TABLE agent_session_steps (
    id                   UUID          NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    session_id           UUID          NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    seq                  INT           NOT NULL,
    kind                 TEXT          NOT NULL,
    name                 TEXT,
    model_name           TEXT,
    input                JSONB,
    output               JSONB,
    text                 TEXT,
    finish_reason        TEXT,
    error                TEXT,
    input_tokens         INT,
    output_tokens        INT,
    reasoning_tokens     INT,
    cached_input_tokens  INT,
    cached_output_tokens INT,
    cost_usd             NUMERIC(16, 8),
    started_at           TIMESTAMPTZ   NOT NULL,
    completed_at         TIMESTAMPTZ   NOT NULL,
    UNIQUE (session_id, seq)
);

CREATE INDEX idx_agent_session_steps_session ON agent_session_steps(session_id, seq);
