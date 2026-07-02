-- Tighten the API-group working-copy invariant.
--
-- Each API group keeps exactly one live working-copy row per (method, path)
-- with api_group_version_id IS NULL. Versioned snapshot rows
-- (api_group_version_id IS NOT NULL) are unconstrained — a method+path repeats
-- across versions by design. This stops the working copy from accumulating
-- duplicate endpoints if a spec is imported more than once.

CREATE UNIQUE INDEX idx_api_endpoints_working_copy_unique
    ON api_endpoints (api_group_id, method, path)
    WHERE api_group_version_id IS NULL AND deleted_at IS NULL;
