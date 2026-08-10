UPDATE org_domains
SET domain = LOWER(TRIM(domain));

ALTER TABLE org_domains
    ADD CONSTRAINT org_domains_domain_format_check CHECK (
        domain = LOWER(domain)
        AND LENGTH(domain) BETWEEN 3 AND 253
        AND domain ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$'
        AND domain ~ '\.[a-z0-9-]*[a-z][a-z0-9-]*$'
    );
