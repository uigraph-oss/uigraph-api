-- Chat sessions: per-user AI assistant conversations, scoped to an org. Each
-- session owns an ordered list of chat_messages. Sessions are personal to the
-- user who created them (owner_user_id). Messages are never edited; the assistant
-- reply is written by uigraph-gateway after streaming completes.

CREATE TABLE chat_sessions (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id          UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    owner_user_id   UUID        NOT NULL,
    title           TEXT        NOT NULL,
    is_pinned       BOOLEAN     NOT NULL DEFAULT false,
    created_by      UUID        NOT NULL,
    updated_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    deleted_by      UUID
);

CREATE INDEX idx_chat_sessions_owner ON chat_sessions(org_id, owner_user_id) WHERE deleted_at IS NULL;

CREATE TABLE chat_messages (
    id                UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id            UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    chat_session_id   UUID        NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    role              TEXT        NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    content           TEXT        NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_chat_messages_session ON chat_messages(chat_session_id, created_at);
