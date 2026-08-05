-- source_ref is the stable external key the CLI repo-scan supplies so a
-- re-scan updates an event in place instead of appending a duplicate every
-- run: "adr:docs/adr/0007-use-postgres.md", "incident:docs/postmortems/
-- 2026-01-04-checkout-outage.md", "release:v1.2.3". Path/version based rather
-- than title based, so renaming an ADR's title still resolves to the same row.
-- Mirrors saved_queries.source_ref (0042_saved_queries.sql), including the
-- partial unique index: manually-created events leave it NULL and any number
-- of them may coexist.
ALTER TABLE timeline_events ADD COLUMN source_ref TEXT;

CREATE UNIQUE INDEX uq_timeline_events_source_ref
    ON timeline_events(service_id, source_ref)
    WHERE source_ref IS NOT NULL;
