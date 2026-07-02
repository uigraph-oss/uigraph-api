-- Saved queries: personal or team-scoped SQL/NoSQL query snippets attached to a
-- service database, organized into folders and tags. CLI/CI-synced queries are
-- always team-scoped (scope='team', team_id=NULL) since there is no human user
-- in a CI context, and are matched on source_ref for idempotent re-sync.

CREATE TABLE saved_query_folders (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id          UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    service_db_id   UUID        NOT NULL REFERENCES service_dbs(id) ON DELETE CASCADE,
    scope           TEXT        NOT NULL CHECK (scope IN ('personal', 'team')),
    owner_user_id   UUID,
    team_id         UUID        REFERENCES teams(id) ON DELETE SET NULL,
    name            TEXT        NOT NULL,
    created_by      UUID        NOT NULL,
    updated_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    deleted_by      UUID,
    CONSTRAINT saved_query_folders_scope_owner_chk CHECK (
        (scope = 'personal' AND owner_user_id IS NOT NULL) OR
        (scope = 'team' AND owner_user_id IS NULL)
    )
);

CREATE INDEX idx_saved_query_folders_db ON saved_query_folders(service_db_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_saved_query_folders_owner ON saved_query_folders(service_db_id, owner_user_id) WHERE deleted_at IS NULL AND scope = 'personal';

CREATE TABLE saved_queries (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id          UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    service_db_id   UUID        NOT NULL REFERENCES service_dbs(id) ON DELETE CASCADE,
    folder_id       UUID        REFERENCES saved_query_folders(id) ON DELETE SET NULL,
    scope           TEXT        NOT NULL CHECK (scope IN ('personal', 'team')),
    owner_user_id   UUID,
    team_id         UUID        REFERENCES teams(id) ON DELETE SET NULL,
    title           TEXT        NOT NULL,
    description     TEXT        NOT NULL DEFAULT '',
    query_text      TEXT        NOT NULL DEFAULT '',
    tags            TEXT[]      NOT NULL DEFAULT '{}',
    source          TEXT,               -- 'ui' | 'ci'
    source_ref      TEXT,               -- stable external key, CLI-supplied only
    created_by      UUID        NOT NULL,
    updated_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    deleted_by      UUID,
    CONSTRAINT saved_queries_scope_owner_chk CHECK (
        (scope = 'personal' AND owner_user_id IS NOT NULL) OR
        (scope = 'team' AND owner_user_id IS NULL)
    )
);

CREATE INDEX idx_saved_queries_db ON saved_queries(service_db_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_saved_queries_folder ON saved_queries(folder_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_saved_queries_owner ON saved_queries(service_db_id, owner_user_id) WHERE deleted_at IS NULL AND scope = 'personal';

-- Race-free CI dedup: a real unique constraint backs the ON CONFLICT upsert used
-- by the sync path, unlike the list-then-decide pattern used elsewhere in this repo.
CREATE UNIQUE INDEX uq_saved_queries_source_ref
    ON saved_queries(service_db_id, source_ref)
    WHERE source_ref IS NOT NULL AND deleted_at IS NULL;
