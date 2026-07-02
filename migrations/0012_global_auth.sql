-- ============================================================
-- UIGraph Global Auth Migration — v12
-- Drops per-org SSO config, makes sessions global, adds server-admin role.
-- Dev-reset is expected: all per-org SSO rows are deleted.
-- ============================================================

-- 1. Global user role — server-admin vs regular user
ALTER TABLE users
    ADD COLUMN role VARCHAR(20) NOT NULL DEFAULT 'user';

-- 2. Wipe per-org SSO data (re-seeded globally via seed/ later)
DELETE FROM oauth_provider_config;
DELETE FROM ldap_config;
DELETE FROM saml_config;
DELETE FROM scim_config;
DELETE FROM ldap_group_mappings;
DELETE FROM sso_role_mappings;

-- 3. OAuth provider config: one per instance, not per org
ALTER TABLE oauth_provider_config
    DROP CONSTRAINT IF EXISTS oauth_provider_config_org_id_fkey,
    DROP CONSTRAINT IF EXISTS oauth_provider_config_org_id_provider_name_key,
    DROP COLUMN org_id,
    ADD UNIQUE (provider_name);

 -- 4. Sessions: global, not tied to a single org at creation
ALTER TABLE user_sessions
    DROP CONSTRAINT IF EXISTS user_sessions_org_id_fkey,
    ALTER COLUMN org_id DROP NOT NULL;

-- 5. LDAP: one global config, no org constraint
ALTER TABLE ldap_config
    DROP CONSTRAINT IF EXISTS ldap_config_org_id_fkey,
    DROP CONSTRAINT IF EXISTS ldap_config_org_id_key,
    DROP COLUMN org_id;

-- 6. LDAP group mappings: global, no org constraint
ALTER TABLE ldap_group_mappings
    DROP CONSTRAINT IF EXISTS ldap_group_mappings_org_id_fkey,
    DROP CONSTRAINT IF EXISTS ldap_group_mappings_org_id_group_id_key,
    DROP COLUMN org_id,
    ADD UNIQUE (group_dn);

-- 7. SAML: one global config, no org constraint
ALTER TABLE saml_config
    DROP CONSTRAINT IF EXISTS saml_config_org_id_fkey,
    DROP CONSTRAINT IF EXISTS saml_config_org_id_key,
    DROP COLUMN org_id;

-- 8. SCIM: one global config, no org constraint
ALTER TABLE scim_config
    DROP CONSTRAINT IF EXISTS scim_config_org_id_fkey,
    DROP CONSTRAINT IF EXISTS scim_config_org_id_key,
    DROP COLUMN org_id;

-- 9. SSO role mappings: nullable org_id so a mapping can target either
--    the server-admin role (org_id = NULL) or a specific org+role.
ALTER TABLE sso_role_mappings
    DROP CONSTRAINT IF EXISTS sso_role_mappings_org_id_fkey,
    DROP CONSTRAINT IF EXISTS sso_role_mappings_org_id_claim_key_claim_value_key,
    ALTER COLUMN org_id DROP NOT NULL,
    ADD UNIQUE (claim_key, claim_value);
