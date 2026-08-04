-- The sync-now endpoint runs asynchronously (a full multi-region sync can
-- take well past a typical HTTP client's timeout), so the connection now
-- has a visible in-flight state between "connected" and the next
-- success/error transition.
ALTER TABLE cloud_connections DROP CONSTRAINT cloud_connections_status_check;
ALTER TABLE cloud_connections ADD CONSTRAINT cloud_connections_status_check
    CHECK (status IN ('pending', 'connected', 'syncing', 'error'));
