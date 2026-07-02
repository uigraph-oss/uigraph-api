ALTER TABLE api_endpoints ADD COLUMN token_count        INT NOT NULL DEFAULT 0;
ALTER TABLE service_dbs   ADD COLUMN schema_token_count INT NOT NULL DEFAULT 0;
ALTER TABLE service_docs  ADD COLUMN doc_token_count    INT NOT NULL DEFAULT 0;
ALTER TABLE diagrams      ADD COLUMN content_token_count INT NOT NULL DEFAULT 0;
