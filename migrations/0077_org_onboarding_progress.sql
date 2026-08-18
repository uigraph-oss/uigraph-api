CREATE TABLE org_onboarding_progress (
    org_id     UUID PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    step       TEXT NOT NULL,
    team_id    UUID REFERENCES teams(id) ON DELETE SET NULL,
    runner     TEXT,
    repo_owner TEXT,
    repo_name  TEXT,
    import_id  UUID REFERENCES repository_imports(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
