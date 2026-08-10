package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/uigraph/app/internal/identity"
	"github.com/uigraph/app/internal/org"
	"github.com/uigraph/app/internal/rolemap"
	"github.com/uigraph/app/internal/store"
)

const authProviderCols = `id, slug, org_id, kind, type, display_name, COALESCE(icon_asset_id,''),
	enabled, allow_sign_up, allowed_domains, default_role,
	COALESCE(client_id,''), COALESCE(client_secret,''),
	COALESCE(auth_url,''), COALESCE(token_url,''), COALESCE(userinfo_url,''), COALESCE(api_url,''),
	scopes, email_claim, name_claim, sub_claim, groups_claim,
	COALESCE(idp_metadata_url,''), COALESCE(idp_metadata_xml,''),
	COALESCE(idp_entity_id,''), COALESCE(idp_sso_url,''), COALESCE(idp_cert,''),
	COALESCE(sp_cert,''), COALESCE(sp_key,''),
	sign_requests, name_id_format,
	email_attribute, name_attribute, groups_attribute,
	created_at, updated_at`

func scanAuthProvider(row interface{ Scan(...any) error }) (identity.AuthProvider, error) {
	var p identity.AuthProvider
	err := row.Scan(
		&p.ID, &p.Slug, &p.OrgID, &p.Kind, &p.Type, &p.DisplayName, &p.IconAssetID,
		&p.Enabled, &p.AllowSignUp, &p.AllowedDomains, &p.DefaultRole,
		&p.ClientID, &p.ClientSecret,
		&p.AuthURL, &p.TokenURL, &p.UserinfoURL, &p.APIURL,
		&p.Scopes, &p.EmailClaim, &p.NameClaim, &p.SubClaim, &p.GroupsClaim,
		&p.IDPMetadataURL, &p.IDPMetadataXML,
		&p.IDPEntityID, &p.IDPSSOURL, &p.IDPCert,
		&p.SPCert, &p.SPKey,
		&p.SignRequests, &p.NameIDFormat,
		&p.EmailAttribute, &p.NameAttribute, &p.GroupsAttribute,
		&p.CreatedAt, &p.UpdatedAt,
	)
	return p, err
}

func (d *DB) CreateAuthProvider(ctx context.Context, p identity.AuthProvider) error {
	const q = `
		INSERT INTO auth_providers
		    (id, slug, org_id, kind, type, display_name, icon_asset_id,
		     enabled, allow_sign_up, allowed_domains, default_role,
		     client_id, client_secret, auth_url, token_url, userinfo_url, api_url,
		     scopes, email_claim, name_claim, sub_claim, groups_claim,
		     idp_metadata_url, idp_metadata_xml, idp_entity_id, idp_sso_url, idp_cert,
		     sp_cert, sp_key, sign_requests, name_id_format,
		     email_attribute, name_attribute, groups_attribute,
		     created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),
		        $8,$9,$10,$11,
		        NULLIF($12,''),NULLIF($13,''),NULLIF($14,''),NULLIF($15,''),NULLIF($16,''),NULLIF($17,''),
		        $18,$19,$20,$21,$22,
		        NULLIF($23,''),NULLIF($24,''),NULLIF($25,''),NULLIF($26,''),NULLIF($27,''),
		        NULLIF($28,''),NULLIF($29,''),$30,$31,
		        $32,$33,$34,
		        NOW(),NOW())`

	_, err := d.db.ExecContext(ctx, q,
		p.ID, p.Slug, p.OrgID, p.Kind, p.Type, p.DisplayName, p.IconAssetID,
		p.Enabled, p.AllowSignUp, p.AllowedDomains, string(p.DefaultRole),
		p.ClientID, p.ClientSecret, p.AuthURL, p.TokenURL, p.UserinfoURL, p.APIURL,
		p.Scopes, p.EmailClaim, p.NameClaim, p.SubClaim, p.GroupsClaim,
		p.IDPMetadataURL, p.IDPMetadataXML, p.IDPEntityID, p.IDPSSOURL, p.IDPCert,
		p.SPCert, p.SPKey, p.SignRequests, p.NameIDFormat,
		p.EmailAttribute, p.NameAttribute, p.GroupsAttribute,
	)
	if uniqueViolation(err, "auth_providers_org_slug_key") {
		return store.ErrConflict
	}
	if uniqueViolation(err, "auth_providers_org_id_display_name_key") {
		return store.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("postgres: CreateAuthProvider: %w", err)
	}
	return nil
}

func (d *DB) UpdateAuthProvider(ctx context.Context, p identity.AuthProvider) error {
	const q = `
		UPDATE auth_providers SET
		    kind             = $3,
		    type             = $4,
		    display_name     = $5,
		    enabled          = $6,
		    allow_sign_up    = $7,
		    allowed_domains  = $8,
		    default_role     = $9,
		    client_id        = NULLIF($10,''),
		    client_secret    = NULLIF($11,''),
		    auth_url         = NULLIF($12,''),
		    token_url        = NULLIF($13,''),
		    userinfo_url     = NULLIF($14,''),
		    api_url          = NULLIF($15,''),
		    scopes           = $16,
		    email_claim      = $17,
		    name_claim       = $18,
		    sub_claim        = $19,
		    groups_claim     = $20,
		    idp_metadata_url = NULLIF($21,''),
		    idp_metadata_xml = NULLIF($22,''),
		    idp_entity_id    = NULLIF($23,''),
		    idp_sso_url      = NULLIF($24,''),
		    idp_cert         = NULLIF($25,''),
		    sp_cert          = NULLIF($26,''),
		    sp_key           = NULLIF($27,''),
		    sign_requests    = $28,
		    name_id_format   = $29,
		    email_attribute  = $30,
		    name_attribute   = $31,
		    groups_attribute = $32,
		    updated_at       = NOW()
		WHERE id = $1 AND org_id = $2`

	res, err := d.db.ExecContext(ctx, q,
		p.ID, p.OrgID, p.Kind, p.Type, p.DisplayName,
		p.Enabled, p.AllowSignUp, p.AllowedDomains, string(p.DefaultRole),
		p.ClientID, p.ClientSecret, p.AuthURL, p.TokenURL, p.UserinfoURL, p.APIURL,
		p.Scopes, p.EmailClaim, p.NameClaim, p.SubClaim, p.GroupsClaim,
		p.IDPMetadataURL, p.IDPMetadataXML, p.IDPEntityID, p.IDPSSOURL, p.IDPCert,
		p.SPCert, p.SPKey, p.SignRequests, p.NameIDFormat,
		p.EmailAttribute, p.NameAttribute, p.GroupsAttribute,
	)
	if uniqueViolation(err, "auth_providers_org_id_display_name_key") {
		return store.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("postgres: UpdateAuthProvider: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("postgres: UpdateAuthProvider: %w", err)
	}
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (d *DB) GetAuthProviderBySlug(ctx context.Context, orgID, slug string) (*identity.AuthProvider, error) {
	q := "SELECT " + authProviderCols + " FROM auth_providers WHERE org_id = $1 AND slug = $2"
	p, err := scanAuthProvider(d.db.QueryRowContext(ctx, q, orgID, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetAuthProviderBySlug: %w", err)
	}
	return &p, nil
}

func (d *DB) ListAuthProviders(ctx context.Context, orgID string) ([]identity.AuthProvider, error) {
	q := "SELECT " + authProviderCols + " FROM auth_providers WHERE org_id = $1 ORDER BY display_name"
	return d.queryAuthProviders(ctx, "ListAuthProviders", q, orgID)
}

func (d *DB) ListEnabledAuthProviders(ctx context.Context, orgID string) ([]identity.AuthProvider, error) {
	q := "SELECT " + authProviderCols + " FROM auth_providers WHERE org_id = $1 AND enabled ORDER BY display_name"
	return d.queryAuthProviders(ctx, "ListEnabledAuthProviders", q, orgID)
}

func (d *DB) queryAuthProviders(ctx context.Context, method, q string, args ...any) ([]identity.AuthProvider, error) {
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: %s: %w", method, err)
	}
	defer func() { _ = rows.Close() }()

	var out []identity.AuthProvider
	for rows.Next() {
		p, err := scanAuthProvider(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: %s scan: %w", method, err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (d *DB) DeleteAuthProvider(ctx context.Context, orgID, providerID string) error {
	const q = `DELETE FROM auth_providers WHERE id = $1 AND org_id = $2`
	if _, err := d.db.ExecContext(ctx, q, providerID, orgID); err != nil {
		return fmt.Errorf("postgres: DeleteAuthProvider: %w", err)
	}
	return nil
}

func (d *DB) SetAuthProviderIcon(ctx context.Context, orgID, providerID string, assetID *string) error {
	const q = `UPDATE auth_providers SET icon_asset_id = $3, updated_at = NOW() WHERE id = $1 AND org_id = $2`
	if _, err := d.db.ExecContext(ctx, q, providerID, orgID, assetID); err != nil {
		return fmt.Errorf("postgres: SetAuthProviderIcon: %w", err)
	}
	return nil
}

func (d *DB) CreateRoleMapping(ctx context.Context, m rolemap.Rule) error {
	const q = `
		INSERT INTO auth_role_mappings
		    (id, provider_id, priority, attribute_key, operator, attribute_value, role, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())`

	_, err := d.db.ExecContext(ctx, q,
		m.ID, m.ProviderID, m.Priority, m.AttributeKey, m.Operator, m.AttributeValue, string(m.Role),
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateRoleMapping: %w", err)
	}
	return nil
}

func (d *DB) UpdateRoleMapping(ctx context.Context, m rolemap.Rule) error {
	const q = `
		UPDATE auth_role_mappings SET
		    priority        = $3,
		    attribute_key   = $4,
		    operator        = $5,
		    attribute_value = $6,
		    role            = $7,
		    updated_at      = NOW()
		WHERE id = $1 AND provider_id = $2`

	res, err := d.db.ExecContext(ctx, q,
		m.ID, m.ProviderID, m.Priority, m.AttributeKey, m.Operator, m.AttributeValue, string(m.Role),
	)
	if err != nil {
		return fmt.Errorf("postgres: UpdateRoleMapping: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("postgres: UpdateRoleMapping: %w", err)
	}
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (d *DB) ListRoleMappings(ctx context.Context, providerID string) ([]rolemap.Rule, error) {
	const q = `
		SELECT id, provider_id, priority, attribute_key, operator, attribute_value, role
		FROM   auth_role_mappings
		WHERE  provider_id = $1
		ORDER BY priority, created_at`

	rows, err := d.db.QueryContext(ctx, q, providerID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListRoleMappings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []rolemap.Rule
	for rows.Next() {
		var m rolemap.Rule
		if err := rows.Scan(&m.ID, &m.ProviderID, &m.Priority, &m.AttributeKey, &m.Operator, &m.AttributeValue, &m.Role); err != nil {
			return nil, fmt.Errorf("postgres: ListRoleMappings scan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (d *DB) DeleteRoleMapping(ctx context.Context, providerID, mappingID string) error {
	const q = `DELETE FROM auth_role_mappings WHERE id = $1 AND provider_id = $2`
	if _, err := d.db.ExecContext(ctx, q, mappingID, providerID); err != nil {
		return fmt.Errorf("postgres: DeleteRoleMapping: %w", err)
	}
	return nil
}

func (d *DB) CreateOrgDomain(ctx context.Context, dom identity.OrgDomain) error {
	const q = `
		INSERT INTO org_domains (id, org_id, domain, created_at, updated_at)
		VALUES ($1, $2, LOWER($3), NOW(), NOW())
		ON CONFLICT (org_id, domain) DO NOTHING`

	if _, err := d.db.ExecContext(ctx, q, dom.ID, dom.OrgID, dom.Domain); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23514" {
			return store.ErrConflict
		}
		return fmt.Errorf("postgres: CreateOrgDomain: %w", err)
	}
	return nil
}

func (d *DB) ListOrgDomains(ctx context.Context, orgID string) ([]identity.OrgDomain, error) {
	const q = `
		SELECT id, org_id, domain, created_at, updated_at
		FROM   org_domains
		WHERE  org_id = $1
		ORDER BY domain`

	rows, err := d.db.QueryContext(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListOrgDomains: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []identity.OrgDomain
	for rows.Next() {
		var dom identity.OrgDomain
		if err := rows.Scan(&dom.ID, &dom.OrgID, &dom.Domain, &dom.CreatedAt, &dom.UpdatedAt); err != nil {
			return nil, fmt.Errorf("postgres: ListOrgDomains scan: %w", err)
		}
		out = append(out, dom)
	}
	return out, rows.Err()
}

func (d *DB) DeleteOrgDomain(ctx context.Context, orgID, domainID string) error {
	const q = `DELETE FROM org_domains WHERE id = $1 AND org_id = $2`
	if _, err := d.db.ExecContext(ctx, q, domainID, orgID); err != nil {
		return fmt.Errorf("postgres: DeleteOrgDomain: %w", err)
	}
	return nil
}

func (d *DB) ListOrgsByDomain(ctx context.Context, domain string) ([]org.Org, error) {
	const q = `
		SELECT o.id, o.name, o.logo_asset_id, o.disabled, o.auto_join, o.onboarding_done,
		       o.created_at, o.updated_at
		FROM   orgs o
		JOIN   org_domains od ON od.org_id = o.id
		WHERE  od.domain = LOWER($1) AND NOT o.disabled
		ORDER BY o.name`

	rows, err := d.db.QueryContext(ctx, q, domain)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListOrgsByDomain: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []org.Org
	for rows.Next() {
		var o org.Org
		if err := rows.Scan(&o.ID, &o.Name, &o.LogoAssetID, &o.Disabled, &o.AutoJoin, &o.OnboardingDone, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("postgres: ListOrgsByDomain scan: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (d *DB) CreateLoginState(ctx context.Context, s identity.LoginState) error {
	const q = `
		INSERT INTO auth_login_state
		    (id, state, org_id, provider_id, nonce, saml_request_id, redirect_to, created_at, expires_at)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NOW(),$8)`

	_, err := d.db.ExecContext(ctx, q,
		s.ID, s.State, s.OrgID, s.ProviderID, s.Nonce, s.SAMLRequestID, s.RedirectTo, s.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateLoginState: %w", err)
	}
	return nil
}

func (d *DB) ConsumeLoginState(ctx context.Context, state string) (*identity.LoginState, error) {
	const q = `
		DELETE FROM auth_login_state
		WHERE  state = $1
		RETURNING id, state, org_id, provider_id,
		          COALESCE(nonce,''), COALESCE(saml_request_id,''), COALESCE(redirect_to,''),
		          created_at, expires_at`

	var s identity.LoginState
	err := d.db.QueryRowContext(ctx, q, state).Scan(
		&s.ID, &s.State, &s.OrgID, &s.ProviderID,
		&s.Nonce, &s.SAMLRequestID, &s.RedirectTo,
		&s.CreatedAt, &s.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: ConsumeLoginState: %w", err)
	}
	if time.Now().After(s.ExpiresAt) {
		return nil, nil
	}
	return &s, nil
}

func (d *DB) PurgeExpiredLoginState(ctx context.Context) error {
	const q = `DELETE FROM auth_login_state WHERE expires_at < NOW()`
	if _, err := d.db.ExecContext(ctx, q); err != nil {
		return fmt.Errorf("postgres: PurgeExpiredLoginState: %w", err)
	}
	return nil
}

// ── LDAP ──────────────────────────────────────────────────────────────────────

func (d *DB) UpsertLDAPConfig(ctx context.Context, cfg identity.LDAPConfig) error {
	// Global LDAP: single row, so we upsert by host (or just replace the only row).
	// Since the table is now global with no unique key other than PK,
	// we do the same as SAML and SCIM: one row, insert-or-replace using id conflict.
	const q = `
		INSERT INTO ldap_config
		    (host, port, use_ssl, start_tls, skip_tls_verify,
		     bind_dn, bind_password, search_base_dn, search_filter,
		     email_attribute, name_attribute, username_attribute,
		     member_of_attribute, allow_sign_up, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),$8,$9,$10,$11,$12,$13,$14,NOW(),NOW())
		ON CONFLICT (true) DO NOTHING`

	_, err := d.db.ExecContext(ctx, q,
		cfg.Host, cfg.Port, cfg.UseSSL, cfg.StartTLS, cfg.SkipTLSVerify,
		cfg.BindDN, cfg.BindPassword, cfg.SearchBaseDN, cfg.SearchFilter,
		cfg.EmailAttribute, cfg.NameAttribute, cfg.UsernameAttr,
		cfg.MemberOfAttr, cfg.AllowSignUp,
	)
	if err != nil {
		return fmt.Errorf("postgres: UpsertLDAPConfig: %w", err)
	}
	return nil
}

func (d *DB) GetLDAPConfig(ctx context.Context) (*identity.LDAPConfig, error) {
	const q = `
		SELECT id, host, port, use_ssl, start_tls, skip_tls_verify,
		       COALESCE(bind_dn,''), COALESCE(bind_password,''),
		       search_base_dn, search_filter,
		       email_attribute, name_attribute, username_attribute,
		       member_of_attribute, allow_sign_up,
		       created_at, updated_at
		FROM   ldap_config LIMIT 1`

	var c identity.LDAPConfig
	err := d.db.QueryRowContext(ctx, q).Scan(
		&c.ID, &c.Host, &c.Port, &c.UseSSL, &c.StartTLS, &c.SkipTLSVerify,
		&c.BindDN, &c.BindPassword,
		&c.SearchBaseDN, &c.SearchFilter,
		&c.EmailAttribute, &c.NameAttribute, &c.UsernameAttr,
		&c.MemberOfAttr, &c.AllowSignUp,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetLDAPConfig: %w", err)
	}
	return &c, nil
}

func (d *DB) DeleteLDAPConfig(ctx context.Context) error {
	const q = `DELETE FROM ldap_config`
	if _, err := d.db.ExecContext(ctx, q); err != nil {
		return fmt.Errorf("postgres: DeleteLDAPConfig: %w", err)
	}
	return nil
}

// ── SCIM ──────────────────────────────────────────────────────────────────────

func (d *DB) UpsertSCIMConfig(ctx context.Context, cfg identity.SCIMConfig) error {
	const q = `
		INSERT INTO scim_config
		    (enabled, bearer_token_hash, sync_users, sync_groups, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (true) DO NOTHING`

	_, err := d.db.ExecContext(ctx, q,
		cfg.Enabled, cfg.BearerTokenHash, cfg.SyncUsers, cfg.SyncGroups,
	)
	if err != nil {
		return fmt.Errorf("postgres: UpsertSCIMConfig: %w", err)
	}
	return nil
}

func (d *DB) GetSCIMConfig(ctx context.Context) (*identity.SCIMConfig, error) {
	const q = `
		SELECT id, enabled, bearer_token_hash, sync_users, sync_groups, created_at, updated_at
		FROM   scim_config LIMIT 1`

	var c identity.SCIMConfig
	err := d.db.QueryRowContext(ctx, q).Scan(
		&c.ID, &c.Enabled, &c.BearerTokenHash,
		&c.SyncUsers, &c.SyncGroups, &c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetSCIMConfig: %w", err)
	}
	return &c, nil
}

func (d *DB) RotateSCIMToken(ctx context.Context, newHash string) error {
	const q = `UPDATE scim_config SET bearer_token_hash = $1, updated_at = NOW()`
	if _, err := d.db.ExecContext(ctx, q, newHash); err != nil {
		return fmt.Errorf("postgres: RotateSCIMToken: %w", err)
	}
	return nil
}
