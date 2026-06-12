package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uigraph/app/internal/identity"
)

// ── OAuth ─────────────────────────────────────────────────────────────────────

func (d *DB) UpsertOAuthProvider(ctx context.Context, cfg identity.OAuthProviderConfig) error {
	const q = `
		INSERT INTO oauth_provider_config
		    (provider_name, type, display_name, client_id, client_secret,
		     auth_url, token_url, userinfo_url, api_url,
		     scopes, allowed_domains, allow_sign_up,
		     email_claim, name_claim, sub_claim,
		     created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),$10,NULLIF($11,''),$12,$13,$14,$15,NOW(),NOW())
		ON CONFLICT (provider_name) DO UPDATE SET
		    type            = EXCLUDED.type,
		    display_name    = EXCLUDED.display_name,
		    client_id       = EXCLUDED.client_id,
		    client_secret   = EXCLUDED.client_secret,
		    auth_url        = EXCLUDED.auth_url,
		    token_url       = EXCLUDED.token_url,
		    userinfo_url    = EXCLUDED.userinfo_url,
		    api_url         = EXCLUDED.api_url,
		    scopes          = EXCLUDED.scopes,
		    allowed_domains = EXCLUDED.allowed_domains,
		    allow_sign_up   = EXCLUDED.allow_sign_up,
		    email_claim     = EXCLUDED.email_claim,
		    name_claim      = EXCLUDED.name_claim,
		    sub_claim       = EXCLUDED.sub_claim,
		    updated_at      = NOW()`

	_, err := d.db.ExecContext(ctx, q,
		cfg.ProviderName, cfg.Type, cfg.DisplayName, cfg.ClientID, cfg.ClientSecret,
		cfg.AuthURL, cfg.TokenURL, cfg.UserinfoURL, cfg.APIURL,
		cfg.Scopes, cfg.AllowedDomains, cfg.AllowSignUp,
		cfg.EmailClaim, cfg.NameClaim, cfg.SubClaim,
	)
	if err != nil {
		return fmt.Errorf("postgres: UpsertOAuthProvider: %w", err)
	}
	return nil
}

func scanOAuthProvider(row interface{ Scan(...any) error }) (identity.OAuthProviderConfig, error) {
	var c identity.OAuthProviderConfig
	var apiURL, allowedDomains sql.NullString
	err := row.Scan(
		&c.ID, &c.ProviderName, &c.Type, &c.DisplayName,
		&c.ClientID, &c.ClientSecret,
		&c.AuthURL, &c.TokenURL, &c.UserinfoURL, &apiURL,
		&c.Scopes, &allowedDomains, &c.AllowSignUp,
		&c.EmailClaim, &c.NameClaim, &c.SubClaim,
		&c.CreatedAt, &c.UpdatedAt,
	)
	c.APIURL = apiURL.String
	c.AllowedDomains = allowedDomains.String
	return c, err
}

const oauthCols = `id, provider_name, type, display_name, client_id, client_secret,
	auth_url, token_url, userinfo_url, api_url,
	scopes, allowed_domains, allow_sign_up,
	email_claim, name_claim, sub_claim,
	created_at, updated_at`

func (d *DB) GetOAuthProvider(ctx context.Context, provider string) (*identity.OAuthProviderConfig, error) {
	q := "SELECT " + oauthCols + " FROM oauth_provider_config WHERE provider_name = $1"
	c, err := scanOAuthProvider(d.db.QueryRowContext(ctx, q, provider))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetOAuthProvider: %w", err)
	}
	return &c, nil
}

func (d *DB) ListOAuthProviders(ctx context.Context) ([]identity.OAuthProviderConfig, error) {
	q := "SELECT " + oauthCols + " FROM oauth_provider_config ORDER BY provider_name"
	rows, err := d.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListOAuthProviders: %w", err)
	}
	defer rows.Close()

	var out []identity.OAuthProviderConfig
	for rows.Next() {
		c, err := scanOAuthProvider(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListOAuthProviders scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *DB) DeleteOAuthProvider(ctx context.Context, provider string) error {
	const q = `DELETE FROM oauth_provider_config WHERE provider_name = $1`
	if _, err := d.db.ExecContext(ctx, q, provider); err != nil {
		return fmt.Errorf("postgres: DeleteOAuthProvider: %w", err)
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

// ── SAML ──────────────────────────────────────────────────────────────────────

func (d *DB) UpsertSAMLConfig(ctx context.Context, cfg identity.SAMLConfig) error {
	const q = `
		INSERT INTO saml_config
		    (idp_metadata_url, idp_metadata_xml,
		     sp_entity_id, sign_requests, name_id_format,
		     email_attribute, name_attribute, login_attribute, groups_attribute,
		     allow_sign_up, created_at, updated_at)
		VALUES (NULLIF($1,''),NULLIF($2,''),$3,$4,$5,$6,$7,$8,NULLIF($9,''),$10,NOW(),NOW())
		ON CONFLICT (true) DO NOTHING`

	_, err := d.db.ExecContext(ctx, q,
		cfg.IDPMetadataURL, cfg.IDPMetadataXML,
		cfg.SPEntityID, cfg.SignRequests, cfg.NameIDFormat,
		cfg.EmailAttribute, cfg.NameAttribute, cfg.LoginAttribute, cfg.GroupsAttribute,
		cfg.AllowSignUp,
	)
	if err != nil {
		return fmt.Errorf("postgres: UpsertSAMLConfig: %w", err)
	}
	return nil
}

func (d *DB) GetSAMLConfig(ctx context.Context) (*identity.SAMLConfig, error) {
	const q = `
		SELECT id, COALESCE(idp_metadata_url,''), COALESCE(idp_metadata_xml,''),
		       COALESCE(idp_entity_id,''), COALESCE(idp_sso_url,''), COALESCE(idp_cert,''),
		       sp_entity_id, COALESCE(sp_cert,''), COALESCE(sp_key,''),
		       sign_requests, name_id_format,
		       email_attribute, name_attribute, login_attribute, COALESCE(groups_attribute,''),
		       allow_sign_up, created_at, updated_at
		FROM   saml_config LIMIT 1`

	var c identity.SAMLConfig
	err := d.db.QueryRowContext(ctx, q).Scan(
		&c.ID, &c.IDPMetadataURL, &c.IDPMetadataXML,
		&c.IDPEntityID, &c.IDPSSOUrl, &c.IDPCert,
		&c.SPEntityID, &c.SPCert, &c.SPKey,
		&c.SignRequests, &c.NameIDFormat,
		&c.EmailAttribute, &c.NameAttribute, &c.LoginAttribute, &c.GroupsAttribute,
		&c.AllowSignUp, &c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetSAMLConfig: %w", err)
	}
	return &c, nil
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
