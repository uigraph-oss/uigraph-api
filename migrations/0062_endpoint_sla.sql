-- Endpoint-level SLAs: pass/fail thresholds defined once on an API endpoint
-- (e.g. "P95 latency <= 300ms"), reused by any load test run that targets
-- that endpoint, instead of being redeclared per test pack.

ALTER TABLE api_endpoints ADD COLUMN sla JSONB;
