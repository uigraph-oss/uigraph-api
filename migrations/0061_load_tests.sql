-- Load test support: a new 'load' test pack type, per-pack load config
-- (target endpoints + pass/fail thresholds) with an optional pinned
-- baseline run, and per-run load metrics (throughput/latency/error-rate,
-- optional time series and per-endpoint breakdown).

ALTER TABLE test_packs DROP CONSTRAINT test_packs_type_check;
ALTER TABLE test_packs ADD CONSTRAINT test_packs_type_check
    CHECK (type IN ('smoke', 'regression', 'manual', 'load'));

ALTER TABLE test_packs ADD COLUMN load_config JSONB;
ALTER TABLE test_packs ADD COLUMN baseline_run_id UUID;

ALTER TABLE test_runs ADD COLUMN load_metrics JSONB;
