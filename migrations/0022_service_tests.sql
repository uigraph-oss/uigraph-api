-- Service Tests: packs, cases, runs, and run results.

CREATE TABLE test_packs (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    service_id  UUID        NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    org_id      UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    type        TEXT        NOT NULL DEFAULT 'manual'
                            CHECK (type IN ('smoke', 'regression', 'manual')),
    created_by  UUID        NOT NULL,
    updated_by  UUID,
    deleted_by  UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_test_packs_service ON test_packs(service_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_test_packs_org ON test_packs(org_id) WHERE deleted_at IS NULL;

CREATE TABLE test_cases (
    id                       UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    test_pack_id             UUID        NOT NULL REFERENCES test_packs(id) ON DELETE CASCADE,
    service_id               UUID        NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    org_id                   UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    title                    TEXT        NOT NULL,
    ord                      DOUBLE PRECISION NOT NULL DEFAULT 0,
    type                     TEXT        NOT NULL
                                       CHECK (type IN ('manual', 'api', 'graphql', 'database', 'grpc')),
    description              TEXT,
    priority                 TEXT        CHECK (priority IN ('p0', 'p1', 'p2', 'p3')),
    labels                   TEXT[]      NOT NULL DEFAULT '{}',
    linked_ticket            TEXT,
    estimated_duration_mins  INT,
    test_owner               TEXT,
    linked_map_node_id       UUID,
    is_critical              BOOLEAN     NOT NULL DEFAULT FALSE,
    evidence_required        BOOLEAN     NOT NULL DEFAULT FALSE,
    manual_payload           JSONB,
    api_payload              JSONB,
    graphql_payload          JSONB,
    database_payload         JSONB,
    grpc_payload             JSONB,
    status                   TEXT        NOT NULL DEFAULT 'active'
                                       CHECK (status IN ('active', 'draft', 'deprecated')),
    version                  INT         NOT NULL DEFAULT 1,
    baseline_run_result_id   UUID,
    dependencies             UUID[]      NOT NULL DEFAULT '{}',
    created_by               UUID        NOT NULL,
    updated_by               UUID,
    deleted_by               UUID,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at               TIMESTAMPTZ
);

CREATE INDEX idx_test_cases_pack ON test_cases(test_pack_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_test_cases_service ON test_cases(service_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_test_cases_org ON test_cases(org_id) WHERE deleted_at IS NULL;

CREATE TABLE test_runs (
    id             UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    test_pack_id   UUID        NOT NULL REFERENCES test_packs(id) ON DELETE CASCADE,
    service_id     UUID        NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    org_id         UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    environment    TEXT        NOT NULL,
    release_label  TEXT,
    started_at     TIMESTAMPTZ,
    completed_at   TIMESTAMPTZ,
    status         TEXT        NOT NULL DEFAULT 'running'
                              CHECK (status IN ('running', 'completed', 'aborted')),
    started_by     UUID,
    executed_by    UUID        NOT NULL,
    executed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    overall_status TEXT        NOT NULL
                              CHECK (overall_status IN ('passed', 'failed', 'partial')),
    created_by     UUID        NOT NULL,
    updated_by     UUID,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);

CREATE INDEX idx_test_runs_pack ON test_runs(test_pack_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_test_runs_service ON test_runs(service_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_test_runs_org ON test_runs(org_id) WHERE deleted_at IS NULL;

CREATE TABLE test_run_results (
    id               UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    test_run_id      UUID        NOT NULL REFERENCES test_runs(id) ON DELETE CASCADE,
    test_case_id     UUID        NOT NULL REFERENCES test_cases(id) ON DELETE CASCADE,
    service_id       UUID        NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    org_id           UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    status           TEXT        NOT NULL
                                CHECK (status IN ('passed', 'failed', 'skipped', 'blocked')),
    blocked_reason   TEXT,
    response_status  INT,
    response_body    TEXT,
    response_time_ms BIGINT,
    notes            TEXT,
    screenshot_urls  TEXT[]      NOT NULL DEFAULT '{}',
    executed_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    executed_by      UUID        NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ,
    UNIQUE (test_run_id, test_case_id)
);

CREATE INDEX idx_test_run_results_run ON test_run_results(test_run_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_test_run_results_case ON test_run_results(test_case_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_test_run_results_service ON test_run_results(service_id) WHERE deleted_at IS NULL;
