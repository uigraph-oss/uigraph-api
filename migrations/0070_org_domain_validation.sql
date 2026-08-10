DROP TRIGGER IF EXISTS enforce_single_org_domain_trigger ON org_domains;
DROP FUNCTION IF EXISTS enforce_single_org_domain();

UPDATE org_domains
SET domain = LOWER(TRIM(domain));

ALTER TABLE org_domains
    DROP CONSTRAINT IF EXISTS org_domains_domain_format_check;

ALTER TABLE org_domains
    ADD CONSTRAINT org_domains_domain_format_check CHECK (
        domain = LOWER(domain)
        AND LENGTH(domain) BETWEEN 3 AND 253
        AND domain ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$'
        AND domain ~ '\.[a-z0-9-]*[a-z][a-z0-9-]*$'
    );
