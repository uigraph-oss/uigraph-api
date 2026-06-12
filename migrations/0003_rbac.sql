-- ============================================================
-- UIGraph RBAC Schema — v1
-- PostgreSQL 15+  |  module: github.com/uigraph/auth
-- Depends on: 0001_auth.sql
-- ============================================================

-- ─── Org-level membership ────────────────────────────────
-- One row per (user, org) pair. Baseline role for every user in the org.
-- role: admin | editor | viewer
-- source: manual | sso

CREATE TABLE org_members (
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    org_id     UUID        NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    role       TEXT        NOT NULL,
    source     TEXT        NOT NULL DEFAULT 'manual',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, org_id)
);

CREATE INDEX idx_org_members_org ON org_members (org_id);

-- ─── Per-resource permission overrides ───────────────────
-- One row per (user, org, resource_type, resource_id) tuple.
-- Supersedes the org-level role for that specific resource.
-- resource_type: diagram | service | project

CREATE TABLE resource_permissions (
    id            UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id       UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    org_id        UUID        NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    resource_type TEXT        NOT NULL,
    resource_id   TEXT        NOT NULL,
    role          TEXT        NOT NULL,
    source        TEXT        NOT NULL DEFAULT 'manual',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, org_id, resource_type, resource_id)
);

CREATE INDEX idx_resource_permissions_user_org ON resource_permissions (user_id, org_id);
CREATE INDEX idx_resource_permissions_resource  ON resource_permissions (resource_type, resource_id);

-- ─── SSO claim → role mappings ───────────────────────────
-- Self-hosting teams configure which IdP claim key/value maps to which
-- UIGraph role. Evaluated on every SSO login.
-- scope: org | resource

CREATE TABLE sso_role_mappings (
    id            UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id        UUID        NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    claim_key     TEXT        NOT NULL,  -- e.g. 'groups', 'uigraph_role', 'roles'
    claim_value   TEXT        NOT NULL,  -- e.g. 'uigraph-admin'
    role          TEXT        NOT NULL,
    scope         TEXT        NOT NULL,  -- 'org' | 'resource'
    resource_type TEXT,                  -- required when scope = 'resource'
    resource_id   TEXT,                  -- NULL = all resources of that type
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, claim_key, claim_value)
);

CREATE INDEX idx_sso_role_mappings_org ON sso_role_mappings (org_id, claim_key);

-- ─── Teams ───────────────────────────────────────────────
-- Named groups within an org. Teams can be granted resource permissions
-- in bulk and can be synced from IdP groups via team_sso_groups.

CREATE TABLE teams (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id      UUID        NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    email       TEXT,
    external_id TEXT,       -- SCIM externalId; set when the team is provisioned by an IdP
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, name)
);

CREATE INDEX idx_teams_org ON teams (org_id);

-- ─── Team members ────────────────────────────────────────
-- permission: member | admin  (team-level admin can manage team membership,
-- not to be confused with org admin role in org_members)

CREATE TABLE team_members (
    team_id    UUID        NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    org_id     UUID        NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    permission TEXT        NOT NULL DEFAULT 'member',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (team_id, user_id)
);

CREATE INDEX idx_team_members_user ON team_members (user_id, org_id);

-- ─── Team SSO group sync ─────────────────────────────────
-- Maps an IdP group claim value to a UIGraph team.
-- On SSO login, if the user's groups claim contains group_id,
-- they are automatically added to the team.

CREATE TABLE team_sso_groups (
    id         UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    team_id    UUID        NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
    org_id     UUID        NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    group_id   TEXT        NOT NULL,  -- IdP group claim value, e.g. 'uigraph-designers'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (team_id, group_id)
);

CREATE INDEX idx_team_sso_groups_org ON team_sso_groups (org_id);
