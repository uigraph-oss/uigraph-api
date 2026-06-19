-- Custom components are org-scoped catalog entries created by users. They reuse
-- the components table but carry a free-text category (category_text) instead of
-- a foreign-key category_id, so category_id becomes nullable for them.
ALTER TABLE components ALTER COLUMN category_id DROP NOT NULL;
ALTER TABLE components ADD COLUMN category_text TEXT;
