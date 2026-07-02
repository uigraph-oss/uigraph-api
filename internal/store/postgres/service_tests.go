package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/uigraph/app/internal/catalog"
)

// ── Test packs ────────────────────────────────────────────────────────────────

func (d *DB) CreateTestPack(ctx context.Context, p catalog.TestPack) error {
	const q = `
		INSERT INTO test_packs
			(id, service_id, org_id, name, type, created_by, updated_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = now
	}
	_, err := d.db.ExecContext(ctx, q,
		p.ID, p.ServiceID, p.OrgID, p.Name, p.Type,
		p.CreatedBy, p.UpdatedBy, p.CreatedAt, p.UpdatedAt,
	)
	return wrapErr("CreateTestPack", err)
}

func (d *DB) GetTestPack(ctx context.Context, id string) (*catalog.TestPack, error) {
	const q = `
		SELECT id, service_id, org_id, name, type,
		       created_by, updated_by, deleted_by, created_at, updated_at, deleted_at
		FROM test_packs WHERE id=$1`
	p, err := scanTestPack(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetTestPack: %w", err)
	}
	return &p, nil
}

func (d *DB) ListTestPacks(ctx context.Context, serviceID string) ([]catalog.TestPack, error) {
	const q = `
		SELECT id, service_id, org_id, name, type,
		       created_by, updated_by, deleted_by, created_at, updated_at, deleted_at
		FROM test_packs
		WHERE service_id=$1 AND deleted_at IS NULL
		ORDER BY created_at ASC`
	rows, err := d.db.QueryContext(ctx, q, serviceID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListTestPacks: %w", err)
	}
	defer rows.Close()
	var out []catalog.TestPack
	for rows.Next() {
		p, scanErr := scanTestPack(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("postgres: ListTestPacks scan: %w", scanErr)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (d *DB) UpdateTestPack(ctx context.Context, p catalog.TestPack) error {
	const q = `
		UPDATE test_packs
		SET name=$1, type=$2, updated_by=$3, updated_at=$4
		WHERE id=$5 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, p.Name, p.Type, p.UpdatedBy, time.Now().UTC(), p.ID)
	return wrapErr("UpdateTestPack", err)
}

func (d *DB) SoftDeleteTestPack(ctx context.Context, id, deletedBy string) error {
	const q = `UPDATE test_packs SET deleted_at=$1, deleted_by=$2 WHERE id=$3 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, id)
	return wrapErr("SoftDeleteTestPack", err)
}

func scanTestPack(row interface{ Scan(...any) error }) (catalog.TestPack, error) {
	var p catalog.TestPack
	err := row.Scan(
		&p.ID, &p.ServiceID, &p.OrgID, &p.Name, &p.Type,
		&p.CreatedBy, &p.UpdatedBy, &p.DeletedBy, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt,
	)
	return p, err
}

// ── Test cases ────────────────────────────────────────────────────────────────

func (d *DB) CreateTestCase(ctx context.Context, tc catalog.TestCase) error {
	const q = `
		INSERT INTO test_cases
			(id, test_pack_id, service_id, org_id, title, ord, type, description, priority,
			 labels, linked_ticket, estimated_duration_mins, test_owner, linked_map_node_id,
			 is_critical, evidence_required, manual_payload, api_payload, graphql_payload,
			 database_payload, grpc_payload, status, version, baseline_run_result_id, dependencies,
			 created_by, updated_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,
		        $10,$11,$12,$13,$14,
		        $15,$16,$17,$18,$19,
		        $20,$21,$22,$23,$24,$25,
		        $26,$27,$28,$29)`
	now := time.Now().UTC()
	if tc.CreatedAt.IsZero() {
		tc.CreatedAt = now
	}
	if tc.UpdatedAt.IsZero() {
		tc.UpdatedAt = now
	}
	if tc.Status == "" {
		tc.Status = "active"
	}
	if tc.Version == 0 {
		tc.Version = 1
	}
	labels := tc.Labels
	if labels == nil {
		labels = []string{}
	}
	deps := tc.Dependencies
	if deps == nil {
		deps = []string{}
	}
	manualJSON, _ := json.Marshal(tc.Manual)
	apiJSON, _ := json.Marshal(tc.API)
	graphqlJSON, _ := json.Marshal(tc.GraphQL)
	dbJSON, _ := json.Marshal(tc.Database)
	grpcJSON, _ := json.Marshal(tc.GRPC)
	_, err := d.db.ExecContext(ctx, q,
		tc.ID, tc.TestPackID, tc.ServiceID, tc.OrgID, tc.Title, tc.Order, tc.Type, tc.Description, tc.Priority,
		pq.Array(labels), tc.LinkedTicket, tc.EstimatedDurationMins, tc.TestOwner, tc.LinkedMapNodeID,
		tc.IsCritical, tc.EvidenceRequired, nullableJSON(manualJSON), nullableJSON(apiJSON), nullableJSON(graphqlJSON),
		nullableJSON(dbJSON), nullableJSON(grpcJSON), tc.Status, tc.Version, tc.BaselineRunResultID, pq.Array(deps),
		tc.CreatedBy, tc.UpdatedBy, tc.CreatedAt, tc.UpdatedAt,
	)
	return wrapErr("CreateTestCase", err)
}

func (d *DB) GetTestCase(ctx context.Context, id string) (*catalog.TestCase, error) {
	const q = `
		SELECT id, test_pack_id, service_id, org_id, title, ord, type, description, priority,
		       labels, linked_ticket, estimated_duration_mins, test_owner, linked_map_node_id,
		       is_critical, evidence_required, manual_payload, api_payload, graphql_payload,
		       database_payload, grpc_payload, status, version, baseline_run_result_id, dependencies,
		       created_by, updated_by, deleted_by, created_at, updated_at, deleted_at
		FROM test_cases WHERE id=$1`
	tc, err := scanTestCase(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetTestCase: %w", err)
	}
	return &tc, nil
}

func (d *DB) ListTestCases(ctx context.Context, serviceID string, testPackID *string) ([]catalog.TestCase, error) {
	q := `
		SELECT id, test_pack_id, service_id, org_id, title, ord, type, description, priority,
		       labels, linked_ticket, estimated_duration_mins, test_owner, linked_map_node_id,
		       is_critical, evidence_required, manual_payload, api_payload, graphql_payload,
		       database_payload, grpc_payload, status, version, baseline_run_result_id, dependencies,
		       created_by, updated_by, deleted_by, created_at, updated_at, deleted_at
		FROM test_cases
		WHERE service_id=$1 AND deleted_at IS NULL`
	args := []any{serviceID}
	if testPackID != nil {
		args = append(args, *testPackID)
		q += fmt.Sprintf(" AND test_pack_id = $%d", len(args))
	}
	q += " ORDER BY ord ASC, created_at ASC"
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListTestCases: %w", err)
	}
	defer rows.Close()
	var out []catalog.TestCase
	for rows.Next() {
		tc, scanErr := scanTestCase(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("postgres: ListTestCases scan: %w", scanErr)
		}
		out = append(out, tc)
	}
	return out, rows.Err()
}

func (d *DB) UpdateTestCase(ctx context.Context, tc catalog.TestCase) error {
	const q = `
		UPDATE test_cases
		SET test_pack_id=$1, title=$2, ord=$3, type=$4, description=$5, priority=$6,
		    labels=$7, linked_ticket=$8, estimated_duration_mins=$9, test_owner=$10, linked_map_node_id=$11,
		    is_critical=$12, evidence_required=$13, manual_payload=$14, api_payload=$15, graphql_payload=$16,
		    database_payload=$17, grpc_payload=$18, status=$19, version=$20, baseline_run_result_id=$21, dependencies=$22,
		    updated_by=$23, updated_at=$24
		WHERE id=$25 AND deleted_at IS NULL`
	manualJSON, _ := json.Marshal(tc.Manual)
	apiJSON, _ := json.Marshal(tc.API)
	graphqlJSON, _ := json.Marshal(tc.GraphQL)
	dbJSON, _ := json.Marshal(tc.Database)
	grpcJSON, _ := json.Marshal(tc.GRPC)
	labels := tc.Labels
	if labels == nil {
		labels = []string{}
	}
	deps := tc.Dependencies
	if deps == nil {
		deps = []string{}
	}
	_, err := d.db.ExecContext(ctx, q,
		tc.TestPackID, tc.Title, tc.Order, tc.Type, tc.Description, tc.Priority,
		pq.Array(labels), tc.LinkedTicket, tc.EstimatedDurationMins, tc.TestOwner, tc.LinkedMapNodeID,
		tc.IsCritical, tc.EvidenceRequired, nullableJSON(manualJSON), nullableJSON(apiJSON), nullableJSON(graphqlJSON),
		nullableJSON(dbJSON), nullableJSON(grpcJSON), tc.Status, tc.Version, tc.BaselineRunResultID, pq.Array(deps),
		tc.UpdatedBy, time.Now().UTC(), tc.ID,
	)
	return wrapErr("UpdateTestCase", err)
}

func (d *DB) SoftDeleteTestCase(ctx context.Context, id, deletedBy string) error {
	const q = `UPDATE test_cases SET deleted_at=$1, deleted_by=$2 WHERE id=$3 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, id)
	return wrapErr("SoftDeleteTestCase", err)
}

func scanTestCase(row interface{ Scan(...any) error }) (catalog.TestCase, error) {
	var tc catalog.TestCase
	var labels pq.StringArray
	var deps pq.StringArray
	var manualPayload, apiPayload, gqlPayload, dbPayload, grpcPayload []byte
	err := row.Scan(
		&tc.ID, &tc.TestPackID, &tc.ServiceID, &tc.OrgID, &tc.Title, &tc.Order, &tc.Type, &tc.Description, &tc.Priority,
		&labels, &tc.LinkedTicket, &tc.EstimatedDurationMins, &tc.TestOwner, &tc.LinkedMapNodeID,
		&tc.IsCritical, &tc.EvidenceRequired, &manualPayload, &apiPayload, &gqlPayload,
		&dbPayload, &grpcPayload, &tc.Status, &tc.Version, &tc.BaselineRunResultID, &deps,
		&tc.CreatedBy, &tc.UpdatedBy, &tc.DeletedBy, &tc.CreatedAt, &tc.UpdatedAt, &tc.DeletedAt,
	)
	if err != nil {
		return tc, err
	}
	tc.Labels = []string(labels)
	tc.Dependencies = []string(deps)
	tc.Manual = decodeJSONPtr[catalog.ManualTestCase](manualPayload)
	tc.API = decodeJSONPtr[catalog.APITestCase](apiPayload)
	tc.GraphQL = decodeJSONPtr[catalog.GraphQLTestCase](gqlPayload)
	tc.Database = decodeJSONPtr[catalog.DatabaseTestCase](dbPayload)
	tc.GRPC = decodeJSONPtr[catalog.GRPCTestCase](grpcPayload)
	return tc, nil
}

// ── Test runs ─────────────────────────────────────────────────────────────────

func (d *DB) CreateTestRun(ctx context.Context, tr catalog.TestRun) error {
	const q = `
		INSERT INTO test_runs
			(id, test_pack_id, service_id, org_id, environment, release_label, started_at, completed_at,
			 status, started_by, executed_by, executed_at, overall_status, created_by, updated_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`
	now := time.Now().UTC()
	if tr.CreatedAt.IsZero() {
		tr.CreatedAt = now
	}
	if tr.UpdatedAt.IsZero() {
		tr.UpdatedAt = now
	}
	if tr.ExecutedAt.IsZero() {
		tr.ExecutedAt = now
	}
	if tr.Status == "" {
		tr.Status = "running"
	}
	_, err := d.db.ExecContext(ctx, q,
		tr.ID, tr.TestPackID, tr.ServiceID, tr.OrgID, tr.Environment, tr.ReleaseLabel, tr.StartedAt, tr.CompletedAt,
		tr.Status, tr.StartedBy, tr.ExecutedBy, tr.ExecutedAt, tr.OverallStatus, tr.CreatedBy, tr.UpdatedBy, tr.CreatedAt, tr.UpdatedAt,
	)
	return wrapErr("CreateTestRun", err)
}

func (d *DB) GetTestRun(ctx context.Context, id string) (*catalog.TestRun, error) {
	const q = `
		SELECT id, test_pack_id, service_id, org_id, environment, release_label, started_at, completed_at,
		       status, started_by, executed_by, executed_at, overall_status, created_by, updated_by, created_at, updated_at, deleted_at
		FROM test_runs WHERE id=$1`
	tr, err := scanTestRun(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetTestRun: %w", err)
	}
	return &tr, nil
}

func (d *DB) ListTestRuns(ctx context.Context, serviceID string, testPackID *string) ([]catalog.TestRun, error) {
	q := `
		SELECT id, test_pack_id, service_id, org_id, environment, release_label, started_at, completed_at,
		       status, started_by, executed_by, executed_at, overall_status, created_by, updated_by, created_at, updated_at, deleted_at
		FROM test_runs
		WHERE service_id=$1 AND deleted_at IS NULL`
	args := []any{serviceID}
	if testPackID != nil {
		args = append(args, *testPackID)
		q += fmt.Sprintf(" AND test_pack_id = $%d", len(args))
	}
	q += " ORDER BY executed_at DESC, created_at DESC"
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListTestRuns: %w", err)
	}
	defer rows.Close()
	var out []catalog.TestRun
	for rows.Next() {
		tr, scanErr := scanTestRun(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("postgres: ListTestRuns scan: %w", scanErr)
		}
		out = append(out, tr)
	}
	return out, rows.Err()
}

func (d *DB) ListTestRunsSummary(ctx context.Context, serviceID string, filter catalog.TestRunSummaryFilter) ([]catalog.TestRunSummary, error) {
	q := `
		SELECT tr.id, tr.test_pack_id, tr.service_id, tr.environment, tr.release_label, tr.started_at, tr.completed_at,
		       tr.status, tr.started_by, tr.executed_by, tr.executed_at, tr.overall_status,
		       COALESCE(SUM(CASE WHEN trr.status = 'passed' THEN 1 ELSE 0 END), 0) AS passed_count,
		       COALESCE(SUM(CASE WHEN trr.status = 'failed' THEN 1 ELSE 0 END), 0) AS failed_count,
		       COALESCE(SUM(CASE WHEN trr.status = 'skipped' THEN 1 ELSE 0 END), 0) AS skipped_count,
		       COALESCE(SUM(CASE WHEN trr.status = 'blocked' THEN 1 ELSE 0 END), 0) AS blocked_count
		FROM test_runs tr
		LEFT JOIN test_run_results trr ON trr.test_run_id = tr.id AND trr.deleted_at IS NULL
		WHERE tr.service_id = $1 AND tr.deleted_at IS NULL`
	args := []any{serviceID}
	if filter.TestPackID != nil {
		args = append(args, *filter.TestPackID)
		q += fmt.Sprintf(" AND tr.test_pack_id = $%d", len(args))
	}
	if filter.Environment != nil {
		args = append(args, *filter.Environment)
		q += fmt.Sprintf(" AND tr.environment = $%d", len(args))
	}
	if filter.Status != nil {
		args = append(args, *filter.Status)
		q += fmt.Sprintf(" AND tr.status = $%d", len(args))
	}
	if filter.ExecutedBy != nil {
		args = append(args, *filter.ExecutedBy)
		q += fmt.Sprintf(" AND tr.executed_by = $%d", len(args))
	}
	if filter.FromDate != nil {
		args = append(args, *filter.FromDate)
		q += fmt.Sprintf(" AND tr.executed_at >= $%d", len(args))
	}
	if filter.ToDate != nil {
		args = append(args, *filter.ToDate)
		q += fmt.Sprintf(" AND tr.executed_at <= $%d", len(args))
	}
	q += `
		GROUP BY tr.id, tr.test_pack_id, tr.service_id, tr.environment, tr.release_label, tr.started_at, tr.completed_at,
		         tr.status, tr.started_by, tr.executed_by, tr.executed_at, tr.overall_status
		ORDER BY tr.executed_at DESC`
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListTestRunsSummary: %w", err)
	}
	defer rows.Close()
	var out []catalog.TestRunSummary
	for rows.Next() {
		var s catalog.TestRunSummary
		if scanErr := rows.Scan(
			&s.TestRunID, &s.TestPackID, &s.ServiceID, &s.Environment, &s.ReleaseLabel, &s.StartedAt, &s.CompletedAt,
			&s.Status, &s.StartedBy, &s.ExecutedBy, &s.ExecutedAt, &s.OverallStatus,
			&s.PassedCount, &s.FailedCount, &s.SkippedCount, &s.BlockedCount,
		); scanErr != nil {
			return nil, fmt.Errorf("postgres: ListTestRunsSummary scan: %w", scanErr)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (d *DB) UpdateTestRun(ctx context.Context, tr catalog.TestRun) error {
	const q = `
		UPDATE test_runs
		SET environment=$1, release_label=$2, started_at=$3, completed_at=$4, status=$5,
		    started_by=$6, overall_status=$7, updated_by=$8, updated_at=$9
		WHERE id=$10 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q,
		tr.Environment, tr.ReleaseLabel, tr.StartedAt, tr.CompletedAt, tr.Status,
		tr.StartedBy, tr.OverallStatus, tr.UpdatedBy, time.Now().UTC(), tr.ID,
	)
	return wrapErr("UpdateTestRun", err)
}

func scanTestRun(row interface{ Scan(...any) error }) (catalog.TestRun, error) {
	var tr catalog.TestRun
	err := row.Scan(
		&tr.ID, &tr.TestPackID, &tr.ServiceID, &tr.OrgID, &tr.Environment, &tr.ReleaseLabel, &tr.StartedAt, &tr.CompletedAt,
		&tr.Status, &tr.StartedBy, &tr.ExecutedBy, &tr.ExecutedAt, &tr.OverallStatus,
		&tr.CreatedBy, &tr.UpdatedBy, &tr.CreatedAt, &tr.UpdatedAt, &tr.DeletedAt,
	)
	return tr, err
}

// ── Test run results ──────────────────────────────────────────────────────────

func (d *DB) CreateTestRunResult(ctx context.Context, rr catalog.TestRunResult) error {
	const q = `
		INSERT INTO test_run_results
			(id, test_run_id, test_case_id, service_id, org_id, status, blocked_reason,
			 response_status, response_body, response_time_ms, notes, screenshot_urls, executed_at, executed_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`
	now := time.Now().UTC()
	if rr.ExecutedAt.IsZero() {
		rr.ExecutedAt = now
	}
	if rr.CreatedAt.IsZero() {
		rr.CreatedAt = now
	}
	if rr.UpdatedAt.IsZero() {
		rr.UpdatedAt = now
	}
	shots := rr.ScreenshotURLs
	if shots == nil {
		shots = []string{}
	}
	_, err := d.db.ExecContext(ctx, q,
		rr.ID, rr.TestRunID, rr.TestCaseID, rr.ServiceID, rr.OrgID, rr.Status, rr.BlockedReason,
		rr.ResponseStatus, rr.ResponseBody, rr.ResponseTimeMs, rr.Notes, pq.Array(shots), rr.ExecutedAt, rr.ExecutedBy, rr.CreatedAt, rr.UpdatedAt,
	)
	return wrapErr("CreateTestRunResult", err)
}

func (d *DB) GetTestRunResult(ctx context.Context, id string) (*catalog.TestRunResult, error) {
	const q = `
		SELECT id, test_run_id, test_case_id, service_id, org_id, status, blocked_reason,
		       response_status, response_body, response_time_ms, notes, screenshot_urls, executed_at, executed_by, created_at, updated_at, deleted_at
		FROM test_run_results WHERE id=$1`
	rr, err := scanTestRunResult(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetTestRunResult: %w", err)
	}
	return &rr, nil
}

func (d *DB) ListTestRunResults(ctx context.Context, serviceID, testRunID string) ([]catalog.TestRunResult, error) {
	const q = `
		SELECT id, test_run_id, test_case_id, service_id, org_id, status, blocked_reason,
		       response_status, response_body, response_time_ms, notes, screenshot_urls, executed_at, executed_by, created_at, updated_at, deleted_at
		FROM test_run_results
		WHERE service_id=$1 AND test_run_id=$2 AND deleted_at IS NULL
		ORDER BY executed_at ASC, created_at ASC`
	rows, err := d.db.QueryContext(ctx, q, serviceID, testRunID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListTestRunResults: %w", err)
	}
	defer rows.Close()
	var out []catalog.TestRunResult
	for rows.Next() {
		rr, scanErr := scanTestRunResult(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("postgres: ListTestRunResults scan: %w", scanErr)
		}
		out = append(out, rr)
	}
	return out, rows.Err()
}

func (d *DB) UpdateTestRunResult(ctx context.Context, rr catalog.TestRunResult) error {
	const q = `
		UPDATE test_run_results
		SET status=$1, blocked_reason=$2, response_status=$3, response_body=$4, response_time_ms=$5,
		    notes=$6, screenshot_urls=$7, executed_at=$8, executed_by=$9, updated_at=$10
		WHERE id=$11 AND deleted_at IS NULL`
	shots := rr.ScreenshotURLs
	if shots == nil {
		shots = []string{}
	}
	_, err := d.db.ExecContext(ctx, q,
		rr.Status, rr.BlockedReason, rr.ResponseStatus, rr.ResponseBody, rr.ResponseTimeMs,
		rr.Notes, pq.Array(shots), rr.ExecutedAt, rr.ExecutedBy, time.Now().UTC(), rr.ID,
	)
	return wrapErr("UpdateTestRunResult", err)
}

func scanTestRunResult(row interface{ Scan(...any) error }) (catalog.TestRunResult, error) {
	var rr catalog.TestRunResult
	var shots pq.StringArray
	err := row.Scan(
		&rr.ID, &rr.TestRunID, &rr.TestCaseID, &rr.ServiceID, &rr.OrgID, &rr.Status, &rr.BlockedReason,
		&rr.ResponseStatus, &rr.ResponseBody, &rr.ResponseTimeMs, &rr.Notes, &shots, &rr.ExecutedAt, &rr.ExecutedBy, &rr.CreatedAt, &rr.UpdatedAt, &rr.DeletedAt,
	)
	if err != nil {
		return rr, err
	}
	rr.ScreenshotURLs = []string(shots)
	return rr, nil
}

func nullableJSON(b []byte) any {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	return b
}

func decodeJSONPtr[T any](b []byte) *T {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return &out
}
