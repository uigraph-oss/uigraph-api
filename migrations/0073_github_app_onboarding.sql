CREATE TABLE github_installations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL UNIQUE REFERENCES orgs(id) ON DELETE CASCADE,
    github_installation_id BIGINT NOT NULL UNIQUE,
    account_id BIGINT NOT NULL,
    account_login TEXT NOT NULL,
    account_type TEXT NOT NULL,
    target_type TEXT NOT NULL,
    status TEXT NOT NULL,
    suspended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
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

CREATE TABLE github_repositories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    installation_id UUID NOT NULL REFERENCES github_installations(id) ON DELETE CASCADE,
    github_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    full_name TEXT NOT NULL,
    html_url TEXT NOT NULL,
    default_branch TEXT NOT NULL,
    private BOOLEAN NOT NULL DEFAULT FALSE,
    archived BOOLEAN NOT NULL DEFAULT FALSE,
    selected BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, github_id),
    UNIQUE(org_id, id)
);

CREATE TABLE repository_onboarding_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    team_id UUID NOT NULL REFERENCES teams(id),
    team_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'running',
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, id)
);

CREATE TABLE repository_onboardings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    batch_id UUID NOT NULL,
    repository_id UUID NOT NULL,
    team_id UUID NOT NULL REFERENCES teams(id),
    team_name TEXT NOT NULL,
    status TEXT NOT NULL,
    failed_phase TEXT,
    setup_branch TEXT NOT NULL,
    setup_pr_number INTEGER,
    setup_pr_url TEXT,
    generation_run_id BIGINT,
    generation_run_url TEXT,
    artifacts_branch TEXT NOT NULL,
    artifacts_pr_number INTEGER,
    artifacts_pr_url TEXT,
    sync_run_id BIGINT,
    sync_run_url TEXT,
    missing_ai_configuration TEXT[] NOT NULL DEFAULT '{}',
    error TEXT,
    service_id UUID REFERENCES services(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    FOREIGN KEY(org_id, batch_id) REFERENCES repository_onboarding_batches(org_id, id) ON DELETE CASCADE,
    FOREIGN KEY(org_id, repository_id) REFERENCES github_repositories(org_id, id),
    UNIQUE(batch_id, repository_id),
    UNIQUE(org_id, id)
);

CREATE INDEX repository_onboardings_repo_idx ON repository_onboardings(org_id, repository_id, created_at DESC);

CREATE TABLE repository_onboarding_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    onboarding_id UUID NOT NULL,
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
    FOREIGN KEY(org_id, onboarding_id) REFERENCES repository_onboardings(org_id, id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX repository_onboarding_jobs_active_idx ON repository_onboarding_jobs(onboarding_id, kind) WHERE completed_at IS NULL;
CREATE INDEX repository_onboarding_jobs_claim_idx ON repository_onboarding_jobs(available_at, created_at) WHERE completed_at IS NULL;

CREATE TABLE github_webhook_deliveries (
    delivery_id TEXT PRIMARY KEY,
    event TEXT NOT NULL,
    action TEXT NOT NULL,
    installation_id BIGINT,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
