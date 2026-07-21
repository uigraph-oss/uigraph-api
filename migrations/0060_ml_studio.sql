CREATE TABLE ml_models (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id             UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    mlflow_id          TEXT        NOT NULL,
    name               TEXT        NOT NULL,
    description        TEXT        NOT NULL DEFAULT '',
    domain             TEXT        NOT NULL DEFAULT '',
    problem_type       TEXT        NOT NULL DEFAULT 'other' CHECK (problem_type IN ('classification','regression','ranking','generation','embedding','other')),
    tags               TEXT[]      NOT NULL DEFAULT '{}',
    production_version_id UUID,
    mlflow_created_at  TIMESTAMPTZ,
    mlflow_updated_at  TIMESTAMPTZ,
    synced_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by         UUID        NOT NULL,
    updated_by         UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at         TIMESTAMPTZ,
    UNIQUE (org_id, mlflow_id)
);
CREATE INDEX idx_ml_models_org ON ml_models(org_id) WHERE deleted_at IS NULL;

CREATE TABLE ml_model_versions (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id             UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    mlflow_id          TEXT        NOT NULL,
    model_id           UUID        NOT NULL REFERENCES ml_models(id) ON DELETE CASCADE,
    version            TEXT        NOT NULL,
    description        TEXT        NOT NULL DEFAULT '',
    status             TEXT        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','released','deprecated')),
    stage              TEXT        NOT NULL DEFAULT 'candidate' CHECK (stage IN ('candidate','staging','production','retired')),
    run_id             UUID,
    mlflow_created_at  TIMESTAMPTZ,
    synced_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by         UUID        NOT NULL,
    updated_by         UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at         TIMESTAMPTZ,
    UNIQUE (org_id, mlflow_id)
);
CREATE INDEX idx_ml_model_versions_model ON ml_model_versions(model_id) WHERE deleted_at IS NULL;

CREATE TABLE ml_experiments (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id             UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    mlflow_id          TEXT        NOT NULL,
    name               TEXT        NOT NULL,
    description        TEXT        NOT NULL DEFAULT '',
    status             TEXT        NOT NULL DEFAULT 'active' CHECK (status IN ('active','concluded','archived')),
    started_at         TIMESTAMPTZ,
    synced_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by         UUID        NOT NULL,
    updated_by         UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at         TIMESTAMPTZ,
    UNIQUE (org_id, mlflow_id)
);
CREATE INDEX idx_ml_experiments_org ON ml_experiments(org_id) WHERE deleted_at IS NULL;

CREATE TABLE ml_runs (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id             UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    mlflow_id          TEXT        NOT NULL,
    experiment_id      UUID        NOT NULL REFERENCES ml_experiments(id) ON DELETE CASCADE,
    name               TEXT        NOT NULL DEFAULT '',
    status             TEXT        NOT NULL DEFAULT 'running' CHECK (status IN ('running','completed','failed','cancelled')),
    started_at         TIMESTAMPTZ,
    ended_at           TIMESTAMPTZ,
    duration           TEXT        NOT NULL DEFAULT '',
    notes              TEXT        NOT NULL DEFAULT '',
    parameters         JSONB       NOT NULL DEFAULT '{}',
    metrics            JSONB       NOT NULL DEFAULT '{}',
    dataset_id         UUID,
    synced_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by         UUID        NOT NULL,
    updated_by         UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at         TIMESTAMPTZ,
    UNIQUE (org_id, mlflow_id)
);
CREATE INDEX idx_ml_runs_experiment ON ml_runs(experiment_id) WHERE deleted_at IS NULL;

CREATE TABLE ml_run_metric_points (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    run_id             UUID        NOT NULL REFERENCES ml_runs(id) ON DELETE CASCADE,
    key                TEXT        NOT NULL,
    step               BIGINT      NOT NULL,
    value              DOUBLE PRECISION NOT NULL,
    ts                 TIMESTAMPTZ,
    UNIQUE (run_id, key, step)
);
CREATE INDEX idx_ml_run_metric_points_run ON ml_run_metric_points(run_id);

CREATE TABLE ml_artifacts (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id             UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    mlflow_id          TEXT        NOT NULL,
    run_id             UUID        NOT NULL REFERENCES ml_runs(id) ON DELETE CASCADE,
    name               TEXT        NOT NULL,
    type               TEXT        NOT NULL CHECK (type IN ('Model checkpoint','Confusion matrix','Notebook','Plot','ONNX','GGUF')),
    uri                TEXT        NOT NULL DEFAULT '',
    size               TEXT        NOT NULL DEFAULT '',
    format             TEXT        NOT NULL DEFAULT '',
    synced_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by         UUID        NOT NULL,
    updated_by         UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at         TIMESTAMPTZ,
    UNIQUE (org_id, mlflow_id)
);
CREATE INDEX idx_ml_artifacts_run ON ml_artifacts(run_id) WHERE deleted_at IS NULL;

CREATE TABLE ml_datasets (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id             UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    mlflow_id          TEXT        NOT NULL,
    name               TEXT        NOT NULL,
    source             TEXT        NOT NULL DEFAULT '',
    type               TEXT        NOT NULL DEFAULT 'tabular' CHECK (type IN ('tabular','text','image','audio','multimodal')),
    row_count          BIGINT      NOT NULL DEFAULT 0,
    schema             JSONB       NOT NULL DEFAULT '[]',
    synced_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by         UUID        NOT NULL,
    updated_by         UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at         TIMESTAMPTZ,
    UNIQUE (org_id, mlflow_id)
);
CREATE INDEX idx_ml_datasets_org ON ml_datasets(org_id) WHERE deleted_at IS NULL;

CREATE TABLE ml_evaluations (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id             UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    mlflow_id          TEXT        NOT NULL,
    version_id         UUID        NOT NULL REFERENCES ml_model_versions(id) ON DELETE CASCADE,
    dataset_id         UUID,
    name               TEXT        NOT NULL,
    type               TEXT        NOT NULL CHECK (type IN ('Offline Benchmark','Online A/B Test','Human Review','Production Monitoring')),
    description        TEXT        NOT NULL DEFAULT '',
    summary            TEXT        NOT NULL DEFAULT '',
    evaluated_at       TIMESTAMPTZ,
    evaluator          TEXT        NOT NULL DEFAULT '',
    synced_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by         UUID        NOT NULL,
    updated_by         UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at         TIMESTAMPTZ,
    UNIQUE (org_id, mlflow_id)
);
CREATE INDEX idx_ml_evaluations_version ON ml_evaluations(version_id) WHERE deleted_at IS NULL;

CREATE TABLE ml_evaluation_metrics (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    evaluation_id      UUID        NOT NULL REFERENCES ml_evaluations(id) ON DELETE CASCADE,
    name               TEXT        NOT NULL,
    value              DOUBLE PRECISION NOT NULL,
    unit               TEXT        NOT NULL DEFAULT '',
    direction          TEXT        NOT NULL CHECK (direction IN ('higher-is-better','lower-is-better')),
    category           TEXT        NOT NULL CHECK (category IN ('quality','performance','cost','business')),
    measured_at        TIMESTAMPTZ
);
CREATE INDEX idx_ml_evaluation_metrics_eval ON ml_evaluation_metrics(evaluation_id);

CREATE TABLE ml_deployments (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id             UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    model_id           UUID        NOT NULL REFERENCES ml_models(id) ON DELETE CASCADE,
    version_id         UUID        NOT NULL REFERENCES ml_model_versions(id) ON DELETE CASCADE,
    name               TEXT        NOT NULL,
    environment        TEXT        NOT NULL DEFAULT '',
    status             TEXT        NOT NULL DEFAULT 'live' CHECK (status IN ('live','rolling-out','rolled-back','stopped')),
    endpoint           TEXT        NOT NULL DEFAULT '',
    region             TEXT        NOT NULL DEFAULT '',
    deployed_at        TIMESTAMPTZ,
    rolled_back_at     TIMESTAMPTZ,
    created_by         UUID        NOT NULL,
    updated_by         UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at         TIMESTAMPTZ,
    deleted_by         UUID
);
CREATE INDEX idx_ml_deployments_org ON ml_deployments(org_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_ml_deployments_version ON ml_deployments(version_id) WHERE deleted_at IS NULL;

CREATE TABLE ml_findings (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id             UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    model_id           UUID        NOT NULL REFERENCES ml_models(id) ON DELETE CASCADE,
    version_id         UUID        REFERENCES ml_model_versions(id) ON DELETE SET NULL,
    title              TEXT        NOT NULL,
    summary            TEXT        NOT NULL DEFAULT '',
    description        TEXT        NOT NULL DEFAULT '',
    created_by         UUID        NOT NULL,
    updated_by         UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at         TIMESTAMPTZ,
    deleted_by         UUID
);
CREATE INDEX idx_ml_findings_org ON ml_findings(org_id) WHERE deleted_at IS NULL;

CREATE TABLE ml_finding_runs (
    finding_id         UUID        NOT NULL REFERENCES ml_findings(id) ON DELETE CASCADE,
    run_id             UUID        NOT NULL REFERENCES ml_runs(id) ON DELETE CASCADE,
    PRIMARY KEY (finding_id, run_id)
);

ALTER TABLE ml_models        ADD CONSTRAINT ml_models_production_version_fk FOREIGN KEY (production_version_id) REFERENCES ml_model_versions(id) ON DELETE SET NULL;
ALTER TABLE ml_model_versions ADD CONSTRAINT ml_model_versions_run_fk       FOREIGN KEY (run_id)               REFERENCES ml_runs(id)           ON DELETE SET NULL;
ALTER TABLE ml_runs          ADD CONSTRAINT ml_runs_dataset_fk             FOREIGN KEY (dataset_id)           REFERENCES ml_datasets(id)       ON DELETE SET NULL;
ALTER TABLE ml_evaluations   ADD CONSTRAINT ml_evaluations_dataset_fk      FOREIGN KEY (dataset_id)           REFERENCES ml_datasets(id)       ON DELETE SET NULL;
