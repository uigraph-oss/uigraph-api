-- User-editable request/response samples, separate from parser-owned schema fields.
ALTER TABLE api_endpoints
    ADD COLUMN IF NOT EXISTS example_requests  JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN IF NOT EXISTS example_responses JSONB NOT NULL DEFAULT '[]';

