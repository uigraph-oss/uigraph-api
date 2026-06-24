package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/uigraph/app/internal/mcpusage"
)

func (d *DB) CreateUsageEvent(ctx context.Context, e mcpusage.UsageEvent) error {
	const q = `
		INSERT INTO mcp_usage_events
			(id, org_id, user_id, service_account_id, tool_name, resource_ids,
			 model_id, tokens_served, tokens_raw_equivalent, tokens_saved,
			 response_size_bytes, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`
	ids := e.ResourceIDs
	if ids == nil {
		ids = []string{}
	}
	_, err := d.db.ExecContext(ctx, q,
		e.ID, e.OrgID, e.UserID, e.ServiceAccountID,
		e.ToolName, pq.Array(ids),
		e.ModelID, e.TokensServed, e.TokensRawEquivalent, e.TokensSaved,
		e.ResponseSizeBytes, time.Now().UTC(),
	)
	return wrapErr("CreateUsageEvent", err)
}

func (d *DB) ListUsageEvents(ctx context.Context, orgID string, f mcpusage.Filter) ([]mcpusage.UsageEvent, error) {
	q := `
		SELECT id, org_id, user_id, service_account_id, tool_name, resource_ids,
		       model_id, tokens_served, tokens_raw_equivalent, tokens_saved,
		       response_size_bytes, created_at
		FROM mcp_usage_events
		WHERE org_id = $1`
	args := []any{orgID}
	i := 2
	if f.Tool != nil {
		q += fmt.Sprintf(" AND tool_name = $%d", i)
		args = append(args, *f.Tool)
		i++
	}
	if f.FromTS != nil {
		q += fmt.Sprintf(" AND created_at >= $%d", i)
		args = append(args, *f.FromTS)
		i++
	}
	if f.ToTS != nil {
		q += fmt.Sprintf(" AND created_at <= $%d", i)
		args = append(args, *f.ToTS)
	}
	q += " ORDER BY created_at DESC LIMIT 500"

	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListUsageEvents: %w", err)
	}
	defer rows.Close()
	var out []mcpusage.UsageEvent
	for rows.Next() {
		var e mcpusage.UsageEvent
		var ids pq.StringArray
		if scanErr := rows.Scan(
			&e.ID, &e.OrgID, &e.UserID, &e.ServiceAccountID,
			&e.ToolName, &ids,
			&e.ModelID, &e.TokensServed, &e.TokensRawEquivalent, &e.TokensSaved,
			&e.ResponseSizeBytes, &e.CreatedAt,
		); scanErr != nil {
			return nil, fmt.Errorf("postgres: ListUsageEvents scan: %w", scanErr)
		}
		e.ResourceIDs = []string(ids)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *DB) GetSavingsSummary(ctx context.Context, orgID, modelID string, since time.Time) (*mcpusage.SavingsSummary, error) {
	const q = `
		SELECT
		    COUNT(*)                                                                                AS total_calls,
		    COALESCE(SUM(e.tokens_served), 0)                                                        AS total_tokens_served,
		    COALESCE(SUM(e.tokens_saved), 0)                                                         AS total_tokens_saved,
		    COALESCE(SUM(e.tokens_served::NUMERIC / 1000000 * m.input_cost_per_million), 0)          AS cost_served_usd,
		    COALESCE(SUM(e.tokens_raw_equivalent::NUMERIC / 1000000 * m.input_cost_per_million), 0)  AS cost_raw_usd,
		    COALESCE(SUM(e.tokens_saved::NUMERIC / 1000000 * m.input_cost_per_million), 0)           AS cost_saved_usd,
		    COUNT(DISTINCT e.user_id)                                                                 AS unique_users_count
		FROM mcp_usage_events e
		LEFT JOIN llm_models m ON m.model_id = e.model_id
		WHERE e.org_id = $1
		  AND e.created_at >= $2
		  AND ($3 = '' OR e.model_id = $3)`
	var s mcpusage.SavingsSummary
	err := d.db.QueryRowContext(ctx, q, orgID, since, modelID).Scan(
		&s.TotalCalls, &s.TotalTokensServed, &s.TotalTokensSaved,
		&s.CostServedUSD, &s.CostRawUSD, &s.CostSavedUSD,
		&s.UniqueUsersCount,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: GetSavingsSummary: %w", err)
	}
	s.OrgID = orgID
	s.ModelID = modelID
	return &s, nil
}

func (d *DB) GetSavingsTimeseries(ctx context.Context, orgID, modelID string, since time.Time) ([]mcpusage.DailySavings, error) {
	const q = `
		SELECT
		    date_trunc('day', e.created_at)                                                          AS day,
		    COUNT(*)                                                                                  AS total_calls,
		    COALESCE(SUM(e.tokens_served), 0)                                                         AS total_tokens_served,
		    COALESCE(SUM(e.tokens_saved), 0)                                                          AS total_tokens_saved,
		    COALESCE(SUM(e.tokens_served::NUMERIC / 1000000 * m.input_cost_per_million), 0)           AS cost_served_usd,
		    COALESCE(SUM(e.tokens_raw_equivalent::NUMERIC / 1000000 * m.input_cost_per_million), 0)   AS cost_raw_usd,
		    COALESCE(SUM(e.tokens_saved::NUMERIC / 1000000 * m.input_cost_per_million), 0)            AS cost_saved_usd
		FROM mcp_usage_events e
		LEFT JOIN llm_models m ON m.model_id = e.model_id
		WHERE e.org_id = $1
		  AND e.created_at >= $2
		  AND ($3 = '' OR e.model_id = $3)
		GROUP BY day
		ORDER BY day ASC`
	rows, err := d.db.QueryContext(ctx, q, orgID, since, modelID)
	if err != nil {
		return nil, fmt.Errorf("postgres: GetSavingsTimeseries: %w", err)
	}
	defer rows.Close()
	var out []mcpusage.DailySavings
	for rows.Next() {
		var row mcpusage.DailySavings
		if scanErr := rows.Scan(
			&row.Date, &row.TotalCalls, &row.TotalTokensServed, &row.TotalTokensSaved,
			&row.CostServedUSD, &row.CostRawUSD, &row.CostSavedUSD,
		); scanErr != nil {
			return nil, fmt.Errorf("postgres: GetSavingsTimeseries scan: %w", scanErr)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
