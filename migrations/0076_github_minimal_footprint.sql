ALTER TABLE repository_imports
    ADD COLUMN github_owner_id BIGINT,
    ADD COLUMN github_repo TEXT;

UPDATE repository_imports imp
SET github_owner_id = i.account_id, github_repo = r.name
FROM github_repositories r
JOIN github_installations i ON i.id = r.installation_id
WHERE r.org_id = imp.org_id AND r.id = imp.repository_id;

DELETE FROM repository_imports WHERE github_owner_id IS NULL OR github_repo IS NULL;

DROP INDEX repository_imports_repo_idx;

ALTER TABLE repository_imports
    ALTER COLUMN github_owner_id SET NOT NULL,
    ALTER COLUMN github_repo SET NOT NULL,
    DROP COLUMN repository_id,
    DROP COLUMN team_name;

CREATE INDEX repository_imports_repo_idx ON repository_imports(org_id, github_owner_id, github_repo, created_at DESC);

DROP TABLE github_repositories;

ALTER TABLE github_installations
    DROP CONSTRAINT github_installations_org_id_key,
    DROP COLUMN id,
    DROP COLUMN account_id,
    DROP COLUMN account_login,
    DROP COLUMN account_type,
    DROP COLUMN target_type,
    DROP COLUMN status,
    DROP COLUMN suspended_at,
    DROP COLUMN updated_at;

ALTER TABLE github_installations ADD PRIMARY KEY (org_id);
