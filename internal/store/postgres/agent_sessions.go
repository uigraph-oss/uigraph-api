package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uigraph/app/internal/agentsession"
	"github.com/uigraph/app/internal/store"
)

const agentSessionColumns = `
	s.id, s.org_id, s.type, s.status, s.user_id, s.service_account_id,
	s.title, s.model_name, s.metadata, s.report, s.error,
	s.started_at, s.updated_at, s.completed_at,
	t.step_count, t.input_tokens, t.output_tokens, t.reasoning_tokens,
	t.cached_input_tokens, t.cached_output_tokens, t.cost_usd, t.unpriced_steps, t.step_duration_ms`

const agentSessionTotalsJoin = `
	LEFT JOIN LATERAL (
	    SELECT COUNT(*)                                AS step_count,
	           COALESCE(SUM(input_tokens),         0)  AS input_tokens,
	           COALESCE(SUM(output_tokens),        0)  AS output_tokens,
	           COALESCE(SUM(reasoning_tokens),     0)  AS reasoning_tokens,
	           COALESCE(SUM(cached_input_tokens),  0)  AS cached_input_tokens,
	           COALESCE(SUM(cached_output_tokens), 0)  AS cached_output_tokens,
	           SUM(cost_usd)::float8                   AS cost_usd,
	           COUNT(*) FILTER (WHERE kind = 'llm' AND cost_usd IS NULL) AS unpriced_steps,
	           COALESCE(SUM(EXTRACT(EPOCH FROM (completed_at - started_at)) * 1000), 0)::bigint AS step_duration_ms
	    FROM agent_session_steps
	    WHERE session_id = s.id
	) t ON true`

func (d *DB) CreateAgentSession(ctx context.Context, s agentsession.Session) error {
	const q = `
		INSERT INTO agent_sessions
			(id, org_id, type, status, user_id, service_account_id,
			 title, model_name, metadata, started_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`
	var metadata any
	if len(s.Metadata) > 0 {
		metadata = []byte(s.Metadata)
	}
	_, err := d.db.ExecContext(ctx, q,
		s.ID, s.OrgID, s.Type, s.Status, s.UserID, s.ServiceAccountID,
		s.Title, s.ModelName, metadata, s.StartedAt, s.UpdatedAt,
	)
	return wrapErr("CreateAgentSession", err)
}

func (d *DB) GetAgentSession(ctx context.Context, id string) (*agentsession.Session, error) {
	q := `SELECT ` + agentSessionColumns + ` FROM agent_sessions s ` + agentSessionTotalsJoin + ` WHERE s.id = $1`
	sess, err := scanAgentSession(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetAgentSession: %w", err)
	}
	return &sess, nil
}

func (d *DB) ListAgentSessions(ctx context.Context, orgID string, f agentsession.SessionFilter) ([]agentsession.Session, int, error) {
	where := ` WHERE s.org_id = $1 AND s.started_at >= $2`
	args := []any{orgID, f.Since}
	i := 3
	if f.Type != nil {
		where += fmt.Sprintf(" AND s.type = $%d", i)
		args = append(args, *f.Type)
		i++
	}
	if f.Status != nil {
		where += fmt.Sprintf(" AND s.status = $%d", i)
		args = append(args, *f.Status)
		i++
	}

	var total int
	if err := d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_sessions s`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres: ListAgentSessions count: %w", err)
	}

	q := `SELECT ` + agentSessionColumns + ` FROM agent_sessions s ` + agentSessionTotalsJoin + where +
		fmt.Sprintf(" ORDER BY s.started_at DESC LIMIT $%d OFFSET $%d", i, i+1)
	args = append(args, f.Limit, f.Offset)

	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: ListAgentSessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []agentsession.Session
	for rows.Next() {
		sess, scanErr := scanAgentSession(rows)
		if scanErr != nil {
			return nil, 0, fmt.Errorf("postgres: ListAgentSessions scan: %w", scanErr)
		}
		out = append(out, sess)
	}
	return out, total, rows.Err()
}

func (d *DB) FinishAgentSession(ctx context.Context, id, status string, report, errMsg *string, completedAt time.Time) error {
	const q = `
		UPDATE agent_sessions
		SET status=$1, report=$2, error=$3, completed_at=$4, updated_at=$4
		WHERE id=$5 AND status='running'`
	res, err := d.db.ExecContext(ctx, q, status, report, errMsg, completedAt, id)
	if err != nil {
		return wrapErr("FinishAgentSession", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return wrapErr("FinishAgentSession", err)
	}
	if n == 0 {
		return fmt.Errorf("postgres: FinishAgentSession: %w", store.ErrConflict)
	}
	return nil
}

func (d *DB) CreateAgentSessionStep(ctx context.Context, st agentsession.Step) (int, error) {
	const q = `
		INSERT INTO agent_session_steps
			(id, session_id, seq, kind, name, model_name, input, output, text, finish_reason, error,
			 input_tokens, output_tokens, reasoning_tokens, cached_input_tokens, cached_output_tokens,
			 cost_usd, started_at, completed_at)
		VALUES ($1, $2,
		        (SELECT COALESCE(MAX(seq), 0) + 1 FROM agent_session_steps WHERE session_id = $2),
		        $3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		RETURNING seq`
	var input any
	if len(st.Input) > 0 {
		input = []byte(st.Input)
	}
	var output any
	if len(st.Output) > 0 {
		output = []byte(st.Output)
	}
	var seq int
	err := d.db.QueryRowContext(ctx, q,
		st.ID, st.SessionID, st.Kind, st.Name, st.ModelName, input, output, st.Text, st.FinishReason, st.Error,
		st.InputTokens, st.OutputTokens, st.ReasoningTokens, st.CachedInputTokens, st.CachedOutputTokens,
		st.CostUSD, st.StartedAt, st.CompletedAt,
	).Scan(&seq)
	if err != nil {
		return 0, fmt.Errorf("postgres: CreateAgentSessionStep: %w", err)
	}
	if _, err := d.db.ExecContext(ctx,
		`UPDATE agent_sessions SET updated_at=$1 WHERE id=$2`,
		st.CompletedAt, st.SessionID,
	); err != nil {
		return 0, wrapErr("CreateAgentSessionStep(bump session)", err)
	}
	return seq, nil
}

func (d *DB) ListAgentSessionSteps(ctx context.Context, sessionID string) ([]agentsession.Step, error) {
	const q = `
		SELECT id, session_id, seq, kind, name, model_name, input, output, text, finish_reason, error,
		       input_tokens, output_tokens, reasoning_tokens, cached_input_tokens, cached_output_tokens,
		       cost_usd::float8, started_at, completed_at
		FROM agent_session_steps
		WHERE session_id = $1
		ORDER BY seq ASC`
	rows, err := d.db.QueryContext(ctx, q, sessionID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListAgentSessionSteps: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []agentsession.Step
	for rows.Next() {
		var st agentsession.Step
		var input, output []byte
		if scanErr := rows.Scan(
			&st.ID, &st.SessionID, &st.Seq, &st.Kind, &st.Name, &st.ModelName, &input, &output,
			&st.Text, &st.FinishReason, &st.Error,
			&st.InputTokens, &st.OutputTokens, &st.ReasoningTokens, &st.CachedInputTokens, &st.CachedOutputTokens,
			&st.CostUSD, &st.StartedAt, &st.CompletedAt,
		); scanErr != nil {
			return nil, fmt.Errorf("postgres: ListAgentSessionSteps scan: %w", scanErr)
		}
		st.Input = input
		st.Output = output
		out = append(out, st)
	}
	return out, rows.Err()
}

func (d *DB) GetAgentSessionSummary(ctx context.Context, orgID string, since time.Time, sessionType *string) (*agentsession.Summary, error) {
	where := ` WHERE s.org_id = $1 AND s.started_at >= $2`
	args := []any{orgID, since}
	if sessionType != nil {
		where += " AND s.type = $3"
		args = append(args, *sessionType)
	}

	const aggregates = `
	    COUNT(*)                                                  AS total_sessions,
	    COUNT(*) FILTER (WHERE s.status = 'completed')            AS completed_sessions,
	    COUNT(*) FILTER (WHERE s.status = 'failed')               AS failed_sessions,
	    COUNT(*) FILTER (WHERE s.status = 'running')              AS running_sessions,
	    COALESCE(SUM(EXTRACT(EPOCH FROM (s.completed_at - s.started_at)) * 1000), 0)::bigint AS total_duration_ms,
	    COALESCE(SUM(t.step_count),           0)                  AS step_count,
	    COALESCE(SUM(t.input_tokens),         0)                  AS input_tokens,
	    COALESCE(SUM(t.output_tokens),        0)                  AS output_tokens,
	    COALESCE(SUM(t.reasoning_tokens),     0)                  AS reasoning_tokens,
	    COALESCE(SUM(t.cached_input_tokens),  0)                  AS cached_input_tokens,
	    COALESCE(SUM(t.cached_output_tokens), 0)                  AS cached_output_tokens,
	    SUM(t.cost_usd)::float8                                   AS cost_usd,
	    COALESCE(SUM(t.unpriced_steps),       0)                  AS unpriced_steps,
	    COALESCE(SUM(t.step_duration_ms),     0)                  AS step_duration_ms`

	var sum agentsession.Summary
	row := d.db.QueryRowContext(ctx,
		`SELECT`+aggregates+` FROM agent_sessions s `+agentSessionTotalsJoin+where, args...)
	if err := row.Scan(
		&sum.TotalSessions, &sum.CompletedSessions, &sum.FailedSessions, &sum.RunningSessions,
		&sum.TotalDurationMs,
		&sum.Totals.StepCount, &sum.Totals.InputTokens, &sum.Totals.OutputTokens, &sum.Totals.ReasoningTokens,
		&sum.Totals.CachedInputTokens, &sum.Totals.CachedOutputTokens, &sum.Totals.CostUSD,
		&sum.Totals.UnpricedSteps, &sum.Totals.StepDurationMs,
	); err != nil {
		return nil, fmt.Errorf("postgres: GetAgentSessionSummary: %w", err)
	}

	rows, err := d.db.QueryContext(ctx,
		`SELECT s.type,`+aggregates+` FROM agent_sessions s `+agentSessionTotalsJoin+where+
			` GROUP BY s.type ORDER BY total_sessions DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: GetAgentSessionSummary byType: %w", err)
	}
	defer func() { _ = rows.Close() }()

	sum.ByType = []agentsession.TypeSummary{}
	for rows.Next() {
		var ts agentsession.TypeSummary
		if scanErr := rows.Scan(
			&ts.Type,
			&ts.TotalSessions, &ts.CompletedSessions, &ts.FailedSessions, &ts.RunningSessions,
			&ts.TotalDurationMs,
			&ts.Totals.StepCount, &ts.Totals.InputTokens, &ts.Totals.OutputTokens, &ts.Totals.ReasoningTokens,
			&ts.Totals.CachedInputTokens, &ts.Totals.CachedOutputTokens, &ts.Totals.CostUSD,
			&ts.Totals.UnpricedSteps, &ts.Totals.StepDurationMs,
		); scanErr != nil {
			return nil, fmt.Errorf("postgres: GetAgentSessionSummary byType scan: %w", scanErr)
		}
		sum.ByType = append(sum.ByType, ts)
	}
	return &sum, rows.Err()
}

func scanAgentSession(row interface{ Scan(...any) error }) (agentsession.Session, error) {
	var s agentsession.Session
	var metadata []byte
	err := row.Scan(
		&s.ID, &s.OrgID, &s.Type, &s.Status, &s.UserID, &s.ServiceAccountID,
		&s.Title, &s.ModelName, &metadata, &s.Report, &s.Error,
		&s.StartedAt, &s.UpdatedAt, &s.CompletedAt,
		&s.Totals.StepCount, &s.Totals.InputTokens, &s.Totals.OutputTokens, &s.Totals.ReasoningTokens,
		&s.Totals.CachedInputTokens, &s.Totals.CachedOutputTokens, &s.Totals.CostUSD,
		&s.Totals.UnpricedSteps, &s.Totals.StepDurationMs,
	)
	s.Metadata = metadata
	return s, err
}
