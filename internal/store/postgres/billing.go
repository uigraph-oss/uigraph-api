package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/uigraph/app/internal/billing"
)

func (d *DB) CreateCloudConnection(ctx context.Context, orgID, actorID string, provider billing.Provider, displayName, encryptedPayload string) (*billing.Connection, error) {
	const q = `
		INSERT INTO cloud_connections (org_id, provider, display_name, auth_payload, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, org_id, provider, display_name, status, status_message,
		          last_verified_at, created_by, updated_by, created_at, updated_at`
	row := d.db.QueryRowContext(ctx, q, orgID, string(provider), displayName, encryptedPayload, actorID)
	return scanCloudConnection(row)
}

func (d *DB) ListCloudConnections(ctx context.Context, orgID string) ([]billing.Connection, error) {
	const q = `
		SELECT id, org_id, provider, display_name, status, status_message,
		       last_verified_at, created_by, updated_by, created_at, updated_at
		FROM cloud_connections
		WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`
	rows, err := d.db.QueryContext(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListCloudConnections: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []billing.Connection
	for rows.Next() {
		c, err := scanCloudConnection(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListCloudConnections scan: %w", err)
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (d *DB) GetCloudConnection(ctx context.Context, orgID, id string) (*billing.Connection, error) {
	const q = `
		SELECT id, org_id, provider, display_name, status, status_message,
		       last_verified_at, created_by, updated_by, created_at, updated_at
		FROM cloud_connections
		WHERE org_id = $1 AND id = $2 AND deleted_at IS NULL`
	c, err := scanCloudConnection(d.db.QueryRowContext(ctx, q, orgID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, billing.ErrConnectionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetCloudConnection: %w", err)
	}
	return c, nil
}

func (d *DB) GetCloudConnectionAuth(ctx context.Context, orgID, id string) (*billing.Connection, string, error) {
	const q = `
		SELECT id, org_id, provider, display_name, status, status_message,
		       last_verified_at, created_by, updated_by, created_at, updated_at, auth_payload
		FROM cloud_connections
		WHERE org_id = $1 AND id = $2 AND deleted_at IS NULL`
	row := d.db.QueryRowContext(ctx, q, orgID, id)
	c, encrypted, err := scanCloudConnectionWithAuth(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", billing.ErrConnectionNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("postgres: GetCloudConnectionAuth: %w", err)
	}
	return c, encrypted, nil
}

func (d *DB) UpdateCloudConnectionStatus(ctx context.Context, id string, status billing.ConnectionStatus, message string) error {
	const q = `
		UPDATE cloud_connections
		SET status = $2, status_message = $3, last_verified_at = NOW(), updated_at = NOW()
		WHERE id = $1`
	res, err := d.db.ExecContext(ctx, q, id, string(status), message)
	if err != nil {
		return fmt.Errorf("postgres: UpdateCloudConnectionStatus: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return billing.ErrConnectionNotFound
	}
	return nil
}

func (d *DB) DeleteCloudConnection(ctx context.Context, orgID, id string) error {
	const q = `UPDATE cloud_connections SET deleted_at = NOW() WHERE org_id = $1 AND id = $2 AND deleted_at IS NULL`
	res, err := d.db.ExecContext(ctx, q, orgID, id)
	if err != nil {
		return fmt.Errorf("postgres: DeleteCloudConnection: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return billing.ErrConnectionNotFound
	}
	return nil
}

func (d *DB) ListActiveCloudConnections(ctx context.Context) ([]billing.Connection, error) {
	const q = `
		SELECT id, org_id, provider, display_name, status, status_message,
		       last_verified_at, created_by, updated_by, created_at, updated_at
		FROM cloud_connections
		WHERE deleted_at IS NULL`
	rows, err := d.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListActiveCloudConnections: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []billing.Connection
	for rows.Next() {
		c, err := scanCloudConnection(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListActiveCloudConnections scan: %w", err)
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCloudConnection(row rowScanner) (*billing.Connection, error) {
	var c billing.Connection
	var provider, status string
	err := row.Scan(&c.ID, &c.OrgID, &provider, &c.DisplayName, &status, &c.StatusMessage,
		&c.LastVerifiedAt, &c.CreatedBy, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.Provider = billing.Provider(provider)
	c.Status = billing.ConnectionStatus(status)
	return &c, nil
}

func scanCloudConnectionWithAuth(row rowScanner) (*billing.Connection, string, error) {
	var c billing.Connection
	var provider, status, encrypted string
	err := row.Scan(&c.ID, &c.OrgID, &provider, &c.DisplayName, &status, &c.StatusMessage,
		&c.LastVerifiedAt, &c.CreatedBy, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt, &encrypted)
	if err != nil {
		return nil, "", err
	}
	c.Provider = billing.Provider(provider)
	c.Status = billing.ConnectionStatus(status)
	return &c, encrypted, nil
}

func (d *DB) CreateTagRule(ctx context.Context, orgID, serviceID, actorID, tagKey, tagValue string) (*billing.TagRule, error) {
	const q = `
		INSERT INTO service_cost_tag_rules (org_id, service_id, tag_key, tag_value, created_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (service_id, tag_key, tag_value) DO UPDATE SET tag_key = EXCLUDED.tag_key
		RETURNING id, org_id, service_id, tag_key, tag_value, created_by, created_at`
	var r billing.TagRule
	err := d.db.QueryRowContext(ctx, q, orgID, serviceID, tagKey, tagValue, actorID).
		Scan(&r.ID, &r.OrgID, &r.ServiceID, &r.TagKey, &r.TagValue, &r.CreatedBy, &r.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("postgres: CreateTagRule: %w", err)
	}
	return &r, nil
}

func (d *DB) ListTagRules(ctx context.Context, orgID, serviceID string) ([]billing.TagRule, error) {
	const q = `
		SELECT id, org_id, service_id, tag_key, tag_value, created_by, created_at
		FROM service_cost_tag_rules
		WHERE org_id = $1 AND service_id = $2
		ORDER BY created_at ASC`
	rows, err := d.db.QueryContext(ctx, q, orgID, serviceID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListTagRules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []billing.TagRule
	for rows.Next() {
		var r billing.TagRule
		if err := rows.Scan(&r.ID, &r.OrgID, &r.ServiceID, &r.TagKey, &r.TagValue, &r.CreatedBy, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("postgres: ListTagRules scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (d *DB) DeleteTagRule(ctx context.Context, orgID, serviceID, ruleID string) error {
	const q = `DELETE FROM service_cost_tag_rules WHERE org_id = $1 AND service_id = $2 AND id = $3`
	res, err := d.db.ExecContext(ctx, q, orgID, serviceID, ruleID)
	if err != nil {
		return fmt.Errorf("postgres: DeleteTagRule: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return billing.ErrTagRuleNotFound
	}
	return nil
}

func (d *DB) UpsertCostResources(ctx context.Context, orgID, connectionID string, resources []billing.Resource) (map[string]string, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("postgres: UpsertCostResources begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const q = `
		INSERT INTO cost_resources (
			org_id, cloud_connection_id, external_resource_id, name, resource_type,
			provider, region, environment, status, monthly_cost_usd, tags, last_synced_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
		ON CONFLICT (cloud_connection_id, external_resource_id) DO UPDATE SET
			name = EXCLUDED.name, resource_type = EXCLUDED.resource_type,
			region = EXCLUDED.region, environment = EXCLUDED.environment,
			status = EXCLUDED.status, monthly_cost_usd = EXCLUDED.monthly_cost_usd,
			tags = EXCLUDED.tags, last_synced_at = NOW(), updated_at = NOW()
		RETURNING id`

	ids := make(map[string]string, len(resources))
	for _, r := range resources {
		tagsJSON, err := json.Marshal(r.Tags)
		if err != nil {
			return nil, fmt.Errorf("postgres: UpsertCostResources marshal tags: %w", err)
		}
		var id string
		if err := tx.QueryRowContext(ctx, q, orgID, connectionID, r.ExternalResourceID, r.Name, r.ResourceType,
			string(r.Provider), r.Region, r.Environment, string(r.Status), r.MonthlyCostUSD, tagsJSON).Scan(&id); err != nil {
			return nil, fmt.Errorf("postgres: UpsertCostResources: %w", err)
		}
		ids[r.ExternalResourceID] = id
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("postgres: UpsertCostResources commit: %w", err)
	}
	return ids, nil
}

func (d *DB) UpsertCostUsageDaily(ctx context.Context, orgID, resourceID string, usageDate string, costUSD float64) error {
	const q = `
		INSERT INTO cost_usage_daily (org_id, resource_id, usage_date, cost_usd)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (resource_id, usage_date) DO UPDATE SET cost_usd = EXCLUDED.cost_usd`
	if _, err := d.db.ExecContext(ctx, q, orgID, resourceID, usageDate, costUSD); err != nil {
		return fmt.Errorf("postgres: UpsertCostUsageDaily: %w", err)
	}
	return nil
}

func (d *DB) ListResourcesForService(ctx context.Context, orgID, serviceID string) ([]billing.Resource, error) {
	const q = `
		SELECT r.id, r.org_id, r.cloud_connection_id, r.external_resource_id, r.name,
		       r.resource_type, r.provider, r.region, r.environment, r.status,
		       r.monthly_cost_usd, r.tags, r.last_synced_at
		FROM cost_resources r
		WHERE r.org_id = $1
		  AND EXISTS (
		      SELECT 1 FROM service_cost_tag_rules tr, jsonb_each_text(r.tags) rt
		      WHERE tr.org_id = r.org_id AND tr.service_id = $2
		        AND rt.key = tr.tag_key AND rt.value = tr.tag_value
		  )
		ORDER BY r.monthly_cost_usd DESC`
	rows, err := d.db.QueryContext(ctx, q, orgID, serviceID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListResourcesForService: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []billing.Resource
	for rows.Next() {
		var r billing.Resource
		var provider, status string
		var tagsJSON []byte
		if err := rows.Scan(&r.ID, &r.OrgID, &r.CloudConnectionID, &r.ExternalResourceID, &r.Name,
			&r.ResourceType, &provider, &r.Region, &r.Environment, &status,
			&r.MonthlyCostUSD, &tagsJSON, &r.LastSyncedAt); err != nil {
			return nil, fmt.Errorf("postgres: ListResourcesForService scan: %w", err)
		}
		r.Provider = billing.Provider(provider)
		r.Status = billing.ResourceStatus(status)
		if len(tagsJSON) > 0 {
			if err := json.Unmarshal(tagsJSON, &r.Tags); err != nil {
				return nil, fmt.Errorf("postgres: ListResourcesForService unmarshal tags: %w", err)
			}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (d *DB) ListTrendForService(ctx context.Context, orgID, serviceID string, days int) ([]billing.TrendPoint, error) {
	const q = `
		SELECT u.usage_date, r.provider, SUM(u.cost_usd)
		FROM cost_usage_daily u
		JOIN cost_resources r ON r.id = u.resource_id
		WHERE u.org_id = $1
		  AND u.usage_date >= (CURRENT_DATE - $3::int)
		  AND EXISTS (
		      SELECT 1 FROM service_cost_tag_rules tr, jsonb_each_text(r.tags) rt
		      WHERE tr.org_id = r.org_id AND tr.service_id = $2
		        AND rt.key = tr.tag_key AND rt.value = tr.tag_value
		  )
		GROUP BY u.usage_date, r.provider
		ORDER BY u.usage_date ASC`
	rows, err := d.db.QueryContext(ctx, q, orgID, serviceID, days)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListTrendForService: %w", err)
	}
	defer func() { _ = rows.Close() }()

	byDate := make(map[string]*billing.TrendPoint)
	var order []string
	for rows.Next() {
		var date, provider string
		var cost float64
		if err := rows.Scan(&date, &provider, &cost); err != nil {
			return nil, fmt.Errorf("postgres: ListTrendForService scan: %w", err)
		}
		p, ok := byDate[date]
		if !ok {
			p = &billing.TrendPoint{Date: date}
			byDate[date] = p
			order = append(order, date)
		}
		p.TotalUSD += cost
		switch billing.Provider(provider) {
		case billing.ProviderAWS:
			p.AWSUSD = cost
		case billing.ProviderAzure:
			p.AzureUSD = cost
		case billing.ProviderGCP:
			p.GCPUSD = cost
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]billing.TrendPoint, 0, len(order))
	for _, date := range order {
		out = append(out, *byDate[date])
	}
	return out, nil
}

func (d *DB) CreateSyncRun(ctx context.Context, connectionID string) (*billing.SyncRun, error) {
	const q = `
		INSERT INTO cost_sync_runs (cloud_connection_id)
		VALUES ($1)
		RETURNING id, cloud_connection_id, started_at, finished_at, status, resource_count, error_message`
	var r billing.SyncRun
	err := d.db.QueryRowContext(ctx, q, connectionID).
		Scan(&r.ID, &r.CloudConnectionID, &r.StartedAt, &r.FinishedAt, &r.Status, &r.ResourceCount, &r.ErrorMessage)
	if err != nil {
		return nil, fmt.Errorf("postgres: CreateSyncRun: %w", err)
	}
	return &r, nil
}

func (d *DB) FinishSyncRun(ctx context.Context, runID string, status string, resourceCount int, errMsg string) error {
	const q = `
		UPDATE cost_sync_runs
		SET finished_at = NOW(), status = $2, resource_count = $3,
		    error_message = NULLIF($4, '')
		WHERE id = $1`
	_, err := d.db.ExecContext(ctx, q, runID, status, resourceCount, errMsg)
	if err != nil {
		return fmt.Errorf("postgres: FinishSyncRun: %w", err)
	}
	return nil
}
