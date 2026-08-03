-- Load tests, endpoint SLAs, test-case screenshots / linked map node type
-- fix, and external service doc links.

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

-- Endpoint-level SLAs: pass/fail thresholds defined once on an API endpoint
-- (e.g. "P95 latency <= 300ms"), reused by any load test run that targets
-- that endpoint, instead of being redeclared per test pack.
ALTER TABLE api_endpoints ADD COLUMN sla JSONB;

-- Reference screenshots attached to a test case's own definition (distinct
-- from TestRunResult.screenshot_urls, which captures evidence from
-- *executing* a test case). Stores asset IDs, resolved to signed URLs via
-- the existing asset-url query.
ALTER TABLE test_cases ADD COLUMN screenshot_urls TEXT[];

-- linked_map_node_id was typed as a plain UUID, but the UI Map Node picker
-- submits a composite "mapId:screenId:focalPointId" reference — not a
-- single UUID. The application layer already treats this column as an
-- opaque string (Go: *string), so this is purely a column-type correction.
ALTER TABLE test_cases ALTER COLUMN linked_map_node_id TYPE TEXT;

-- Pasted references to docs that live outside UIGraph (Confluence, Notion,
-- Google Docs, one-pagers) — a label + URL per link, no file upload
-- involved. Distinct from the existing docs/service_docs tables, which
-- model uploaded file assets.
ALTER TABLE services ADD COLUMN doc_links JSONB;
