-- Pasted references to docs that live outside UIGraph (Confluence, Notion,
-- Google Docs, one-pagers) — a label + URL per link, no file upload
-- involved. Distinct from the existing docs/service_docs tables, which
-- model uploaded file assets.

ALTER TABLE services ADD COLUMN doc_links JSONB;
