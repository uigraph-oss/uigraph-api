CREATE TABLE github_installations (
    org_id UUID PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    github_installation_id BIGINT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE github_installation_states (
    state_hash TEXT PRIMARY KEY,
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    github_user_id BIGINT,
    authorized_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX github_installation_states_expiry_idx ON github_installation_states(expires_at) WHERE consumed_at IS NULL;

CREATE TABLE repository_imports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    github_owner_id BIGINT NOT NULL,
    github_repo TEXT NOT NULL,
    team_id UUID NOT NULL REFERENCES teams(id),
    status TEXT NOT NULL,
    steps JSONB NOT NULL DEFAULT '[]',
    branch TEXT NOT NULL,
    run_id BIGINT,
    run_url TEXT,
    pr_url TEXT,
    missing_ai_configuration TEXT[] NOT NULL DEFAULT '{}',
    error TEXT,
    service_id UUID REFERENCES services(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    run_started_at TIMESTAMPTZ,
    run_completed_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    UNIQUE(org_id, id)
);

CREATE INDEX repository_imports_repo_idx ON repository_imports(org_id, github_owner_id, github_repo, created_at DESC);
CREATE UNIQUE INDEX repository_imports_branch_idx ON repository_imports(org_id, branch);

CREATE TABLE repository_import_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    import_id UUID NOT NULL,
    kind TEXT NOT NULL,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    lease_owner TEXT,
    lease_expires_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 8,
    last_error TEXT,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY(org_id, import_id) REFERENCES repository_imports(org_id, id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX repository_import_jobs_active_idx ON repository_import_jobs(import_id, kind) WHERE completed_at IS NULL;
CREATE INDEX repository_import_jobs_claim_idx ON repository_import_jobs(available_at, created_at) WHERE completed_at IS NULL;

CREATE TABLE github_webhook_deliveries (
    delivery_id TEXT PRIMARY KEY,
    event TEXT NOT NULL,
    action TEXT NOT NULL,
    installation_id BIGINT,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
