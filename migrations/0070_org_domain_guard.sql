WITH ranked_domains AS (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY org_id ORDER BY created_at, id) AS row_number
    FROM org_domains
)
DELETE FROM org_domains
WHERE id IN (
    SELECT id
    FROM ranked_domains
    WHERE row_number > 1
);

UPDATE org_domains
SET domain = LOWER(TRIM(domain));

ALTER TABLE org_domains
    ADD CONSTRAINT org_domains_domain_format_check CHECK (
        domain = LOWER(domain)
        AND LENGTH(domain) BETWEEN 3 AND 253
        AND domain ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$'
        AND domain ~ '\.[a-z0-9-]*[a-z][a-z0-9-]*$'
    );

CREATE OR REPLACE FUNCTION enforce_single_org_domain()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended(NEW.org_id::TEXT, 0));

    IF EXISTS (
        SELECT 1
        FROM org_domains
        WHERE org_id = NEW.org_id
          AND id <> NEW.id
    ) THEN
        RAISE EXCEPTION 'organization can currently have only one domain'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER enforce_single_org_domain_trigger
BEFORE INSERT OR UPDATE OF org_id ON org_domains
FOR EACH ROW
EXECUTE FUNCTION enforce_single_org_domain();
