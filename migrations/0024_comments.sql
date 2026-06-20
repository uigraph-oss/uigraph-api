-- Comments are org-scoped annotations attached to a resource (a diagram or map
-- node, identified by an opaque resource_id). Threads are modelled via a
-- self-referential parent_comment_id. Soft-deleted via deleted_at/deleted_by.
CREATE TABLE comments (
    id                UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id            UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    resource_id       TEXT        NOT NULL,
    parent_comment_id UUID        REFERENCES comments(id) ON DELETE CASCADE,
    text              TEXT        NOT NULL DEFAULT '',
    created_by        UUID        NOT NULL,
    updated_by        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ,
    deleted_by        UUID
);

CREATE INDEX idx_comments_resource ON comments(org_id, resource_id) WHERE deleted_at IS NULL;
