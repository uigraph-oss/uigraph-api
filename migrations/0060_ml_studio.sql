CREATE TABLE ml_projects (
    id            UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id        UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    team_id       UUID        REFERENCES teams(id) ON DELETE SET NULL,
    name          TEXT        NOT NULL,
    type          TEXT        NOT NULL CHECK (type IN ('model','training')),
    description   TEXT        NOT NULL DEFAULT '',
    source_type   TEXT        NOT NULL DEFAULT '',
    source_url    TEXT        NOT NULL DEFAULT '',
    synced_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by    UUID        NOT NULL,
    updated_by    UUID,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ,
    UNIQUE (org_id, name)
);
CREATE INDEX idx_ml_projects_org  ON ml_projects(org_id)  WHERE deleted_at IS NULL;
CREATE INDEX idx_ml_projects_team ON ml_projects(team_id) WHERE deleted_at IS NULL;

CREATE TABLE ml_models (
    id                    UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id                UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    project_id            UUID        REFERENCES ml_projects(id) ON DELETE SET NULL,
    mlflow_id             TEXT,
    origin                TEXT        NOT NULL DEFAULT 'mlflow' CHECK (origin IN ('mlflow','manual')),
    name                  TEXT        NOT NULL,
    description           TEXT        NOT NULL DEFAULT '',
    domain                TEXT        NOT NULL DEFAULT '',
    problem_type          TEXT        NOT NULL DEFAULT 'other' CHECK (problem_type IN ('classification','regression','ranking','generation','embedding','other')),
    tags                  TEXT[]      NOT NULL DEFAULT '{}',
    license               TEXT        NOT NULL DEFAULT '',
    reference_links       TEXT[]      NOT NULL DEFAULT '{}',
    intended_use          TEXT        NOT NULL DEFAULT '',
    limitations           TEXT        NOT NULL DEFAULT '',
    considerations        TEXT        NOT NULL DEFAULT '',
    recommendations       TEXT        NOT NULL DEFAULT '',
    production_version_id UUID,
    mlflow_created_at     TIMESTAMPTZ,
    mlflow_updated_at     TIMESTAMPTZ,
    synced_at             TIMESTAMPTZ DEFAULT NOW(),
    created_by            UUID        NOT NULL,
    updated_by            UUID,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at            TIMESTAMPTZ,
    deleted_by            UUID,
    UNIQUE (org_id, mlflow_id)
);
CREATE INDEX idx_ml_models_org ON ml_models(org_id) WHERE deleted_at IS NULL;
CREATE INDEX ml_models_tags_idx ON ml_models USING GIN (tags);

CREATE TABLE ml_model_versions (
    id                UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id            UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    model_id          UUID        NOT NULL REFERENCES ml_models(id) ON DELETE CASCADE,
    mlflow_id         TEXT,
    source            TEXT        NOT NULL DEFAULT 'mlflow' CHECK (source IN ('mlflow','manual')),
    version           TEXT        NOT NULL,
    description       TEXT        NOT NULL DEFAULT '',
    run_id            UUID,
    mlflow_created_at TIMESTAMPTZ,
    synced_at         TIMESTAMPTZ DEFAULT NOW(),
    created_by        UUID        NOT NULL,
    updated_by        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ,
    deleted_by        UUID,
    UNIQUE (org_id, mlflow_id)
);
CREATE INDEX idx_ml_model_versions_model ON ml_model_versions(model_id) WHERE deleted_at IS NULL;

CREATE TABLE ml_experiments (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id      UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    project_id  UUID        REFERENCES ml_projects(id) ON DELETE SET NULL,
    mlflow_id   TEXT,
    source      TEXT        NOT NULL DEFAULT 'mlflow' CHECK (source IN ('mlflow','manual')),
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    status      TEXT        NOT NULL DEFAULT 'active' CHECK (status IN ('active','concluded','archived')),
    tags        TEXT[]      NOT NULL DEFAULT '{}',
    synced_at   TIMESTAMPTZ DEFAULT NOW(),
    created_by  UUID        NOT NULL,
    updated_by  UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    deleted_by  UUID,
    UNIQUE (org_id, mlflow_id)
);
CREATE INDEX idx_ml_experiments_org ON ml_experiments(org_id) WHERE deleted_at IS NULL;
CREATE INDEX ml_experiments_tags_idx ON ml_experiments USING GIN (tags);

CREATE TABLE ml_datasets (
    id            UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id        UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    experiment_id UUID        NOT NULL REFERENCES ml_experiments(id) ON DELETE CASCADE,
    mlflow_id     TEXT,
    origin        TEXT        NOT NULL DEFAULT 'mlflow' CHECK (origin IN ('mlflow','manual')),
    name          TEXT        NOT NULL,
    digest        TEXT        NOT NULL DEFAULT '',
    source        TEXT        NOT NULL DEFAULT '',
    source_type   TEXT        NOT NULL DEFAULT '',
    context       TEXT        NOT NULL DEFAULT 'training' CHECK (context IN ('training','evaluation')),
    row_count     BIGINT      NOT NULL DEFAULT 0,
    schema        JSONB       NOT NULL DEFAULT '[]',
    tags          TEXT[]      NOT NULL DEFAULT '{}',
    synced_at     TIMESTAMPTZ DEFAULT NOW(),
    created_by    UUID        NOT NULL,
    updated_by    UUID,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ,
    deleted_by    UUID,
    UNIQUE (org_id, experiment_id, mlflow_id)
);
CREATE INDEX idx_ml_datasets_org_exp ON ml_datasets(org_id, experiment_id) WHERE deleted_at IS NULL;
CREATE INDEX ml_datasets_tags_idx ON ml_datasets USING GIN (tags);

CREATE TABLE ml_runs (
    id            UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id        UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    experiment_id UUID        NOT NULL REFERENCES ml_experiments(id) ON DELETE CASCADE,
    dataset_id    UUID,
    mlflow_id     TEXT,
    source        TEXT        NOT NULL DEFAULT 'mlflow' CHECK (source IN ('mlflow','manual')),
    name          TEXT        NOT NULL DEFAULT '',
    status        TEXT        NOT NULL DEFAULT 'running' CHECK (status IN ('running','completed','failed','cancelled')),
    notes         TEXT        NOT NULL DEFAULT '',
    tags          TEXT[]      NOT NULL DEFAULT '{}',
    started_at    TIMESTAMPTZ NOT NULL,
    ended_at      TIMESTAMPTZ,
    synced_at     TIMESTAMPTZ DEFAULT NOW(),
    created_by    UUID        NOT NULL,
    updated_by    UUID,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ,
    deleted_by    UUID,
    UNIQUE (org_id, mlflow_id)
);
CREATE INDEX idx_ml_runs_experiment ON ml_runs(experiment_id) WHERE deleted_at IS NULL;
CREATE INDEX ml_runs_tags_idx ON ml_runs USING GIN (tags);

CREATE TABLE ml_runs_params (
    id     UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    run_id UUID NOT NULL REFERENCES ml_runs(id) ON DELETE CASCADE,
    key    TEXT NOT NULL,
    value  TEXT NOT NULL,
    UNIQUE (run_id, key)
);
CREATE INDEX idx_ml_runs_params_run ON ml_runs_params(run_id);

CREATE TABLE ml_runs_metrics (
    id     UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    run_id UUID NOT NULL REFERENCES ml_runs(id) ON DELETE CASCADE,
    key    TEXT NOT NULL,
    value  DOUBLE PRECISION NOT NULL,
    UNIQUE (run_id, key)
);
CREATE INDEX idx_ml_runs_metrics_run ON ml_runs_metrics(run_id);

CREATE TABLE ml_artifacts (
    id           UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id       UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    run_id       UUID        NOT NULL REFERENCES ml_runs(id) ON DELETE CASCADE,
    mlflow_id    TEXT,
    source       TEXT        NOT NULL DEFAULT 'mlflow' CHECK (source IN ('mlflow','manual')),
    name         TEXT        NOT NULL,
    type         TEXT        NOT NULL,
    uri          TEXT        NOT NULL DEFAULT '',
    download_uri TEXT        NOT NULL DEFAULT '',
    storage_key  TEXT        NOT NULL DEFAULT '',
    mime_type    TEXT        NOT NULL DEFAULT '',
    size         TEXT        NOT NULL DEFAULT '',
    size_bytes   BIGINT,
    format       TEXT        NOT NULL DEFAULT '',
    synced_at    TIMESTAMPTZ DEFAULT NOW(),
    created_by   UUID        NOT NULL,
    updated_by   UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ,
    deleted_by   UUID,
    UNIQUE (org_id, mlflow_id)
);
CREATE INDEX idx_ml_artifacts_run ON ml_artifacts(run_id) WHERE deleted_at IS NULL;

CREATE TABLE ml_evaluations (
    id            UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id        UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    experiment_id UUID        NOT NULL REFERENCES ml_experiments(id) ON DELETE CASCADE,
    version_id    UUID        NOT NULL REFERENCES ml_model_versions(id) ON DELETE CASCADE,
    dataset_id    UUID,
    mlflow_id     TEXT,
    source        TEXT        NOT NULL DEFAULT 'mlflow' CHECK (source IN ('mlflow','manual')),
    name          TEXT        NOT NULL,
    type          TEXT        NOT NULL CHECK (type IN ('Offline Benchmark','Online A/B Test','Human Review','Production Monitoring')),
    description   TEXT        NOT NULL DEFAULT '',
    summary       TEXT        NOT NULL DEFAULT '',
    evaluator     TEXT        NOT NULL DEFAULT '',
    tags          TEXT[]      NOT NULL DEFAULT '{}',
    started_at    TIMESTAMPTZ NOT NULL,
    ended_at      TIMESTAMPTZ,
    synced_at     TIMESTAMPTZ DEFAULT NOW(),
    created_by    UUID        NOT NULL,
    updated_by    UUID,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ,
    deleted_by    UUID,
    UNIQUE (org_id, mlflow_id)
);
CREATE INDEX idx_ml_evaluations_version    ON ml_evaluations(version_id)    WHERE deleted_at IS NULL;
CREATE INDEX idx_ml_evaluations_experiment ON ml_evaluations(experiment_id) WHERE deleted_at IS NULL;
CREATE INDEX ml_evaluations_tags_idx ON ml_evaluations USING GIN (tags);

CREATE TABLE ml_evaluations_params (
    id      UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    eval_id UUID NOT NULL REFERENCES ml_evaluations(id) ON DELETE CASCADE,
    key     TEXT NOT NULL,
    value   TEXT NOT NULL,
    UNIQUE (eval_id, key)
);
CREATE INDEX idx_ml_evaluations_params_eval ON ml_evaluations_params(eval_id);

CREATE TABLE ml_evaluations_metrics (
    id      UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    eval_id UUID NOT NULL REFERENCES ml_evaluations(id) ON DELETE CASCADE,
    key     TEXT NOT NULL,
    value   DOUBLE PRECISION NOT NULL,
    UNIQUE (eval_id, key)
);
CREATE INDEX idx_ml_evaluations_metrics_eval ON ml_evaluations_metrics(eval_id);

CREATE TABLE ml_version_deployments (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id      UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    version_id  UUID        NOT NULL REFERENCES ml_model_versions(id) ON DELETE CASCADE,
    from_status TEXT        CHECK (from_status IN ('candidate','staging','production','retired')),
    to_status   TEXT        NOT NULL CHECK (to_status IN ('candidate','staging','production','retired')),
    changed_by  UUID        NOT NULL,
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_ml_version_deployments_version
    ON ml_version_deployments(version_id, changed_at DESC, id DESC);

CREATE TABLE ml_deployments (
    id             UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id         UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    model_id       UUID        NOT NULL REFERENCES ml_models(id) ON DELETE CASCADE,
    version_id     UUID        NOT NULL REFERENCES ml_model_versions(id) ON DELETE CASCADE,
    name           TEXT        NOT NULL,
    environment    TEXT        NOT NULL DEFAULT '',
    status         TEXT        NOT NULL DEFAULT 'live' CHECK (status IN ('live','rolling-out','rolled-back','stopped')),
    endpoint       TEXT        NOT NULL DEFAULT '',
    region         TEXT        NOT NULL DEFAULT '',
    deployed_at    TIMESTAMPTZ,
    rolled_back_at TIMESTAMPTZ,
    created_by     UUID        NOT NULL,
    updated_by     UUID,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ,
    deleted_by     UUID
);
CREATE INDEX idx_ml_deployments_org     ON ml_deployments(org_id)     WHERE deleted_at IS NULL;
CREATE INDEX idx_ml_deployments_version ON ml_deployments(version_id) WHERE deleted_at IS NULL;

CREATE TABLE ml_findings (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id      UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    model_id    UUID        NOT NULL REFERENCES ml_models(id) ON DELETE CASCADE,
    version_id  UUID        REFERENCES ml_model_versions(id) ON DELETE SET NULL,
    title       TEXT        NOT NULL,
    summary     TEXT        NOT NULL DEFAULT '',
    description TEXT        NOT NULL DEFAULT '',
    created_by  UUID        NOT NULL,
    updated_by  UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    deleted_by  UUID
);
CREATE INDEX idx_ml_findings_org ON ml_findings(org_id) WHERE deleted_at IS NULL;

CREATE TABLE ml_finding_runs (
    finding_id UUID NOT NULL REFERENCES ml_findings(id) ON DELETE CASCADE,
    run_id     UUID NOT NULL REFERENCES ml_runs(id) ON DELETE CASCADE,
    PRIMARY KEY (finding_id, run_id)
);

CREATE TABLE ml_finding_evaluations (
    finding_id    UUID NOT NULL REFERENCES ml_findings(id) ON DELETE CASCADE,
    evaluation_id UUID NOT NULL REFERENCES ml_evaluations(id) ON DELETE CASCADE,
    PRIMARY KEY (finding_id, evaluation_id)
);
CREATE INDEX idx_ml_finding_evaluations_eval ON ml_finding_evaluations(evaluation_id);

ALTER TABLE ml_models         ADD CONSTRAINT ml_models_production_version_fk FOREIGN KEY (production_version_id) REFERENCES ml_model_versions(id) ON DELETE SET NULL;
ALTER TABLE ml_model_versions ADD CONSTRAINT ml_model_versions_run_fk       FOREIGN KEY (run_id)                REFERENCES ml_runs(id)           ON DELETE SET NULL;
ALTER TABLE ml_runs           ADD CONSTRAINT ml_runs_dataset_fk             FOREIGN KEY (dataset_id)            REFERENCES ml_datasets(id)       ON DELETE SET NULL;
ALTER TABLE ml_evaluations    ADD CONSTRAINT ml_evaluations_dataset_fk      FOREIGN KEY (dataset_id)            REFERENCES ml_datasets(id)       ON DELETE SET NULL;
