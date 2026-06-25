package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/uigraph/app/internal/catalog"
	"github.com/uigraph/app/internal/store"
)

// ── Services ──────────────────────────────────────────────────────────────────

// resolveTeamID maps a team name to its id within an org. It returns
// store.ErrNotFound when no team matches, so callers surface a 404.
func (d *DB) resolveTeamID(ctx context.Context, orgID, name string) (string, error) {
	const q = `SELECT id FROM teams WHERE org_id = $1 AND name = $2`
	var id string
	err := d.db.QueryRowContext(ctx, q, orgID, name).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", store.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("postgres: resolveTeamID: %w", err)
	}
	return id, nil
}

func (d *DB) CreateService(ctx context.Context, s catalog.Service) error {
	if (s.TeamID == nil || *s.TeamID == "") && s.TeamName != "" {
		id, err := d.resolveTeamID(ctx, s.OrgID, s.TeamName)
		if err != nil {
			return err
		}
		s.TeamID = &id
	}

	const q = `
		INSERT INTO services
			(id, org_id, folder_id, team_id, name, slug, description,
			 status, tier, category, language,
			 git_repo_url, jira_project_url, slack_channel_url, last_commit_sha,
			 labels, metadata, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`
	now := time.Now().UTC()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = now
	}
	meta := s.Metadata
	if meta == nil {
		meta = json.RawMessage("{}")
	}
	labels := s.Labels
	if labels == nil {
		labels = []string{}
	}
	_, err := d.db.ExecContext(ctx, q,
		s.ID, s.OrgID, s.FolderID, s.TeamID,
		s.Name, s.Slug, s.Description,
		s.Status, s.Tier, s.Category, s.Language,
		s.GitRepoURL, s.JiraProjectURL, s.SlackChannelURL, s.LastCommitSha,
		pq.Array(labels), meta,
		s.CreatedBy, s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateService: %w", err)
	}
	return nil
}

func (d *DB) GetService(ctx context.Context, id string) (*catalog.Service, error) {
	const q = `
		SELECT id, org_id, folder_id, team_id, name, slug, description,
		       status, tier, category, language,
		       git_repo_url, jira_project_url, slack_channel_url, last_commit_sha,
		       labels, metadata, created_by, updated_by,
		       created_at, updated_at, deleted_at, deleted_by
		FROM services WHERE id = $1`
	s, err := scanService(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetService: %w", err)
	}
	return &s, nil
}

func (d *DB) GetServiceBySlug(ctx context.Context, orgID, slug string) (*catalog.Service, error) {
	const q = `
		SELECT id, org_id, folder_id, team_id, name, slug, description,
		       status, tier, category, language,
		       git_repo_url, jira_project_url, slack_channel_url, last_commit_sha,
		       labels, metadata, created_by, updated_by,
		       created_at, updated_at, deleted_at, deleted_by
		FROM services WHERE org_id = $1 AND slug = $2 AND deleted_at IS NULL`
	s, err := scanService(d.db.QueryRowContext(ctx, q, orgID, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetServiceBySlug: %w", err)
	}
	return &s, nil
}

func (d *DB) ListServices(ctx context.Context, orgID string, folderID, teamID *string) ([]catalog.Service, error) {
	q := `
		SELECT id, org_id, folder_id, team_id, name, slug, description,
		       status, tier, category, language,
		       git_repo_url, jira_project_url, slack_channel_url, last_commit_sha,
		       labels, metadata, created_by, updated_by,
		       created_at, updated_at, deleted_at, deleted_by
		FROM services WHERE org_id = $1 AND deleted_at IS NULL`
	args := []any{orgID}
	if folderID != nil {
		args = append(args, *folderID)
		q += fmt.Sprintf(" AND folder_id = $%d", len(args))
	}
	if teamID != nil {
		args = append(args, *teamID)
		q += fmt.Sprintf(" AND team_id = $%d", len(args))
	}
	q += " ORDER BY name ASC"

	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListServices: %w", err)
	}
	defer rows.Close()

	var out []catalog.Service
	for rows.Next() {
		s, err := scanService(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListServices scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (d *DB) ListServiceStats(ctx context.Context, orgID string, serviceID *string) ([]catalog.ServiceStats, error) {
	q := `
		SELECT id
		FROM services
		WHERE org_id = $1 AND deleted_at IS NULL`
	args := []any{orgID}
	if serviceID != nil && *serviceID != "" {
		args = append(args, *serviceID)
		q += fmt.Sprintf(" AND id = $%d", len(args))
	}
	q += " ORDER BY name ASC"

	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListServiceStats list services: %w", err)
	}
	defer rows.Close()

	stats := make([]catalog.ServiceStats, 0)
	for rows.Next() {
		var sid string
		if err := rows.Scan(&sid); err != nil {
			return nil, fmt.Errorf("postgres: ListServiceStats scan service id: %w", err)
		}
		s, err := d.serviceStatsForID(ctx, orgID, sid)
		if err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: ListServiceStats rows: %w", err)
	}
	return stats, nil
}

func (d *DB) serviceStatsForID(ctx context.Context, orgID, serviceID string) (catalog.ServiceStats, error) {
	out := catalog.ServiceStats{ServiceID: serviceID}

	const endpointQ = `
		SELECT COUNT(*)
		FROM api_endpoints e
		JOIN api_groups g ON g.id = e.api_group_id
		WHERE e.org_id = $1
		  AND e.service_id = $2
		  AND e.deleted_at IS NULL
		  AND g.deleted_at IS NULL`
	if err := d.db.QueryRowContext(ctx, endpointQ, orgID, serviceID).Scan(&out.EndpointCount); err != nil {
		return out, fmt.Errorf("postgres: ListServiceStats endpoint count: %w", err)
	}

	diagramCount, err := d.countOptionalServiceRows(ctx, "service_diagrams", orgID, serviceID)
	if err != nil {
		return out, err
	}
	out.DiagramCount = diagramCount

	docCount, err := d.countOptionalServiceRows(ctx, "service_docs", orgID, serviceID)
	if err != nil {
		return out, err
	}
	out.DocCount = docCount

	dbTableCount, err := d.countOptionalServiceRows(ctx, "service_dbs", orgID, serviceID)
	if err != nil {
		return out, err
	}
	out.DBTableCount = dbTableCount

	testCaseCount, err := d.countOptionalTestCases(ctx, orgID, serviceID)
	if err != nil {
		return out, err
	}
	out.TestCaseCount = testCaseCount

	return out, nil
}

func (d *DB) countOptionalServiceRows(ctx context.Context, tableName, orgID, serviceID string) (int, error) {
	exists, err := d.tableExists(ctx, tableName)
	if err != nil || !exists {
		return 0, err
	}

	var clauses []string
	args := []any{serviceID}
	clauses = append(clauses, "service_id = $1")

	hasOrgID, err := d.columnExists(ctx, tableName, "org_id")
	if err != nil {
		return 0, fmt.Errorf("postgres: ListServiceStats %s org column check: %w", tableName, err)
	}
	if hasOrgID {
		args = append(args, orgID)
		clauses = append(clauses, fmt.Sprintf("org_id = $%d", len(args)))
	}

	hasDeletedAt, err := d.columnExists(ctx, tableName, "deleted_at")
	if err != nil {
		return 0, fmt.Errorf("postgres: ListServiceStats %s deleted column check: %w", tableName, err)
	}
	if hasDeletedAt {
		clauses = append(clauses, "deleted_at IS NULL")
	}

	q := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", tableName, strings.Join(clauses, " AND "))
	var count int
	if err := d.db.QueryRowContext(ctx, q, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("postgres: ListServiceStats %s count: %w", tableName, err)
	}
	return count, nil
}

func (d *DB) countOptionalTestCases(ctx context.Context, orgID, serviceID string) (int, error) {
	testCasesExists, err := d.tableExists(ctx, "test_cases")
	if err != nil || !testCasesExists {
		return 0, err
	}

	testCasesHasServiceID, err := d.columnExists(ctx, "test_cases", "service_id")
	if err != nil {
		return 0, fmt.Errorf("postgres: ListServiceStats test_cases service_id check: %w", err)
	}
	if testCasesHasServiceID {
		return d.countOptionalServiceRows(ctx, "test_cases", orgID, serviceID)
	}

	testPacksExists, err := d.tableExists(ctx, "test_packs")
	if err != nil || !testPacksExists {
		return 0, err
	}
	testCasesHasPackID, err := d.columnExists(ctx, "test_cases", "test_pack_id")
	if err != nil {
		return 0, fmt.Errorf("postgres: ListServiceStats test_cases test_pack_id check: %w", err)
	}
	testPacksHasServiceID, err := d.columnExists(ctx, "test_packs", "service_id")
	if err != nil {
		return 0, fmt.Errorf("postgres: ListServiceStats test_packs service_id check: %w", err)
	}
	if !testCasesHasPackID || !testPacksHasServiceID {
		return 0, nil
	}

	var testCasesHasDeletedAt bool
	testCasesHasDeletedAt, err = d.columnExists(ctx, "test_cases", "deleted_at")
	if err != nil {
		return 0, fmt.Errorf("postgres: ListServiceStats test_cases deleted_at check: %w", err)
	}
	var testPacksHasDeletedAt bool
	testPacksHasDeletedAt, err = d.columnExists(ctx, "test_packs", "deleted_at")
	if err != nil {
		return 0, fmt.Errorf("postgres: ListServiceStats test_packs deleted_at check: %w", err)
	}

	q := `
		SELECT COUNT(*)
		FROM test_cases tc
		JOIN test_packs tp ON tp.id = tc.test_pack_id
		WHERE tp.service_id = $1`
	args := []any{serviceID}
	if testPacksHasDeletedAt {
		q += " AND tp.deleted_at IS NULL"
	}
	if testCasesHasDeletedAt {
		q += " AND tc.deleted_at IS NULL"
	}

	testCasesHasOrgID, err := d.columnExists(ctx, "test_cases", "org_id")
	if err != nil {
		return 0, fmt.Errorf("postgres: ListServiceStats test_cases org_id check: %w", err)
	}
	testPacksHasOrgID, err := d.columnExists(ctx, "test_packs", "org_id")
	if err != nil {
		return 0, fmt.Errorf("postgres: ListServiceStats test_packs org_id check: %w", err)
	}
	if testCasesHasOrgID {
		args = append(args, orgID)
		q += fmt.Sprintf(" AND tc.org_id = $%d", len(args))
	} else if testPacksHasOrgID {
		args = append(args, orgID)
		q += fmt.Sprintf(" AND tp.org_id = $%d", len(args))
	}

	var count int
	if err := d.db.QueryRowContext(ctx, q, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("postgres: ListServiceStats test case count: %w", err)
	}
	return count, nil
}

func (d *DB) tableExists(ctx context.Context, tableName string) (bool, error) {
	var exists bool
	err := d.db.QueryRowContext(
		ctx,
		`SELECT to_regclass($1) IS NOT NULL`,
		"public."+tableName,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("postgres: tableExists %s: %w", tableName, err)
	}
	return exists, nil
}

func (d *DB) columnExists(ctx context.Context, tableName, columnName string) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = $1
			  AND column_name = $2
		)`
	var exists bool
	if err := d.db.QueryRowContext(ctx, q, tableName, columnName).Scan(&exists); err != nil {
		return false, fmt.Errorf("postgres: columnExists %s.%s: %w", tableName, columnName, err)
	}
	return exists, nil
}

func (d *DB) UpdateService(ctx context.Context, s catalog.Service) error {
	if (s.TeamID == nil || *s.TeamID == "") && s.TeamName != "" {
		id, err := d.resolveTeamID(ctx, s.OrgID, s.TeamName)
		if err != nil {
			return err
		}
		s.TeamID = &id
	}
	const q = `
		UPDATE services
		SET name=$1, slug=$2, description=$3, status=$4, tier=$5, category=$6, language=$7,
		    git_repo_url=$8, jira_project_url=$9, slack_channel_url=$10, last_commit_sha=$11,
		    labels=$12, metadata=$13, folder_id=$14, team_id=$15,
		    updated_by=$16, updated_at=$17
		WHERE id=$18 AND deleted_at IS NULL`
	meta := s.Metadata
	if meta == nil {
		meta = json.RawMessage("{}")
	}
	labels := s.Labels
	if labels == nil {
		labels = []string{}
	}
	_, err := d.db.ExecContext(ctx, q,
		s.Name, s.Slug, s.Description, s.Status, s.Tier, s.Category, s.Language,
		s.GitRepoURL, s.JiraProjectURL, s.SlackChannelURL, s.LastCommitSha,
		pq.Array(labels), meta, s.FolderID, s.TeamID,
		s.UpdatedBy, time.Now().UTC(), s.ID,
	)
	if err != nil {
		return fmt.Errorf("postgres: UpdateService: %w", err)
	}
	return nil
}

func (d *DB) SoftDeleteService(ctx context.Context, id, deletedBy string) error {
	const q = `UPDATE services SET deleted_at=$1, deleted_by=$2 WHERE id=$3 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, id)
	return wrapErr("SoftDeleteService", err)
}

func scanService(row interface{ Scan(...any) error }) (catalog.Service, error) {
	var s catalog.Service
	var labels pq.StringArray
	var meta []byte
	err := row.Scan(
		&s.ID, &s.OrgID, &s.FolderID, &s.TeamID,
		&s.Name, &s.Slug, &s.Description,
		&s.Status, &s.Tier, &s.Category, &s.Language,
		&s.GitRepoURL, &s.JiraProjectURL, &s.SlackChannelURL, &s.LastCommitSha,
		&labels, &meta,
		&s.CreatedBy, &s.UpdatedBy,
		&s.CreatedAt, &s.UpdatedAt, &s.DeletedAt, &s.DeletedBy,
	)
	if err != nil {
		return s, err
	}
	s.Labels = []string(labels)
	if s.Labels == nil {
		s.Labels = []string{}
	}
	s.Metadata = meta
	return s, nil
}

// ── API Groups ────────────────────────────────────────────────────────────────

func (d *DB) CreateAPIGroup(ctx context.Context, g catalog.APIGroup) error {
	const q = `
		INSERT INTO api_groups
			(id, service_id, org_id, name, version, label, protocol,
			 spec_key, spec_hash, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`
	now := time.Now().UTC()
	if g.CreatedAt.IsZero() {
		g.CreatedAt = now
	}
	if g.UpdatedAt.IsZero() {
		g.UpdatedAt = now
	}
	_, err := d.db.ExecContext(ctx, q,
		g.ID, g.ServiceID, g.OrgID,
		g.Name, g.Version, g.Label, g.Protocol,
		g.SpecKey, g.SpecHash,
		g.CreatedBy, g.CreatedAt, g.UpdatedAt,
	)
	return wrapErr("CreateAPIGroup", err)
}

func (d *DB) GetAPIGroup(ctx context.Context, id string) (*catalog.APIGroup, error) {
	const q = `
		SELECT id, service_id, org_id, name, version, label, protocol,
		       spec_key, spec_hash,
		       created_by, updated_by, created_at, updated_at, deleted_at, deleted_by
		FROM api_groups WHERE id = $1`
	g, err := scanAPIGroup(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetAPIGroup: %w", err)
	}
	return &g, nil
}

func (d *DB) ListAPIGroups(ctx context.Context, serviceID string) ([]catalog.APIGroup, error) {
	const q = `
		SELECT id, service_id, org_id, name, version, label, protocol,
		       spec_key, spec_hash,
		       created_by, updated_by, created_at, updated_at, deleted_at, deleted_by
		FROM api_groups WHERE service_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC`
	rows, err := d.db.QueryContext(ctx, q, serviceID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListAPIGroups: %w", err)
	}
	defer rows.Close()
	var out []catalog.APIGroup
	for rows.Next() {
		g, err := scanAPIGroup(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListAPIGroups scan: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (d *DB) UpdateAPIGroup(ctx context.Context, g catalog.APIGroup) error {
	const q = `
		UPDATE api_groups
		SET name=$1, version=$2, label=$3, protocol=$4,
		    spec_key=$5, spec_hash=$6,
		    updated_by=$7, updated_at=$8
		WHERE id=$9 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q,
		g.Name, g.Version, g.Label, g.Protocol,
		g.SpecKey, g.SpecHash,
		g.UpdatedBy, time.Now().UTC(), g.ID,
	)
	return wrapErr("UpdateAPIGroup", err)
}

func (d *DB) SoftDeleteAPIGroup(ctx context.Context, id, deletedBy string) error {
	const q = `UPDATE api_groups SET deleted_at=$1, deleted_by=$2 WHERE id=$3 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, id)
	return wrapErr("SoftDeleteAPIGroup", err)
}

func scanAPIGroup(row interface{ Scan(...any) error }) (catalog.APIGroup, error) {
	var g catalog.APIGroup
	return g, row.Scan(
		&g.ID, &g.ServiceID, &g.OrgID,
		&g.Name, &g.Version, &g.Label, &g.Protocol,
		&g.SpecKey, &g.SpecHash,
		&g.CreatedBy, &g.UpdatedBy,
		&g.CreatedAt, &g.UpdatedAt, &g.DeletedAt, &g.DeletedBy,
	)
}

// ── API Group Versions ────────────────────────────────────────────────────────

func (d *DB) CreateAPIGroupVersion(ctx context.Context, v catalog.APIGroupVersion) error {
	const q = `
		INSERT INTO api_group_versions
			(id, api_group_id, version_number, label, spec_key, spec_hash,
			 is_auto_version, created_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now().UTC()
	}
	_, err := d.db.ExecContext(ctx, q,
		v.ID, v.APIGroupID, v.VersionNumber, v.Label,
		v.SpecKey, v.SpecHash, v.IsAutoVersion,
		v.CreatedBy, v.CreatedAt,
	)
	return wrapErr("CreateAPIGroupVersion", err)
}

func (d *DB) GetAPIGroupVersion(ctx context.Context, id string) (*catalog.APIGroupVersion, error) {
	const q = `
		SELECT id, api_group_id, version_number, label, spec_key, spec_hash,
		       is_auto_version, created_by, created_at
		FROM api_group_versions WHERE id = $1`
	var v catalog.APIGroupVersion
	err := d.db.QueryRowContext(ctx, q, id).Scan(
		&v.ID, &v.APIGroupID, &v.VersionNumber, &v.Label,
		&v.SpecKey, &v.SpecHash, &v.IsAutoVersion,
		&v.CreatedBy, &v.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetAPIGroupVersion: %w", err)
	}
	return &v, nil
}

func (d *DB) ListAPIGroupVersions(ctx context.Context, apiGroupID string) ([]catalog.APIGroupVersion, error) {
	const q = `
		SELECT id, api_group_id, version_number, label, spec_key, spec_hash,
		       is_auto_version, created_by, created_at
		FROM api_group_versions WHERE api_group_id = $1
		ORDER BY version_number DESC`
	rows, err := d.db.QueryContext(ctx, q, apiGroupID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListAPIGroupVersions: %w", err)
	}
	defer rows.Close()
	var out []catalog.APIGroupVersion
	for rows.Next() {
		var v catalog.APIGroupVersion
		if err := rows.Scan(
			&v.ID, &v.APIGroupID, &v.VersionNumber, &v.Label,
			&v.SpecKey, &v.SpecHash, &v.IsAutoVersion,
			&v.CreatedBy, &v.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres: ListAPIGroupVersions scan: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (d *DB) LatestAPIGroupVersionNumber(ctx context.Context, apiGroupID string) (int, error) {
	const q = `SELECT COALESCE(MAX(version_number), 0) FROM api_group_versions WHERE api_group_id = $1`
	var n int
	return n, d.db.QueryRowContext(ctx, q, apiGroupID).Scan(&n)
}

// ── API Endpoints ─────────────────────────────────────────────────────────────

func (d *DB) CreateAPIEndpoint(ctx context.Context, e catalog.APIEndpoint) error {
	const q = `
		INSERT INTO api_endpoints
			(id, api_group_id, api_group_version_id, service_id, org_id,
			 operation_id, method, path, summary, description,
			 tags, token_count, parameters, request_body, responses, ord,
			 created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`
	now := time.Now().UTC()
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now
	}
	if e.UpdatedAt.IsZero() {
		e.UpdatedAt = now
	}
	tags := e.Tags
	if tags == nil {
		tags = []string{}
	}
	params := e.Parameters
	if params == nil {
		params = json.RawMessage("[]")
	}
	reqBody := e.RequestBody
	if reqBody == nil {
		reqBody = json.RawMessage("{}")
	}
	resps := e.Responses
	if resps == nil {
		resps = json.RawMessage("{}")
	}
	_, err := d.db.ExecContext(ctx, q,
		e.ID, e.APIGroupID, e.APIGroupVersionID, e.ServiceID, e.OrgID,
		e.OperationID, e.Method, e.Path, e.Summary, e.Description,
		pq.Array(tags), e.TokenCount, params, reqBody, resps, e.Order,
		e.CreatedBy, e.CreatedAt, e.UpdatedAt,
	)
	return wrapErr("CreateAPIEndpoint", err)
}

func (d *DB) GetAPIEndpoint(ctx context.Context, id string) (*catalog.APIEndpoint, error) {
	const q = `
		SELECT id, api_group_id, api_group_version_id, service_id, org_id,
		       operation_id, method, path, summary, description,
		       tags, token_count, parameters, request_body, responses, ord,
		       created_by, updated_by, created_at, updated_at, deleted_at, deleted_by
		FROM api_endpoints WHERE id = $1`
	e, err := scanAPIEndpoint(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetAPIEndpoint: %w", err)
	}
	return &e, nil
}

// ListAPIEndpoints returns the current working-copy endpoints (api_group_version_id IS NULL).
func (d *DB) ListAPIEndpoints(ctx context.Context, apiGroupID string) ([]catalog.APIEndpoint, error) {
	const q = `
		SELECT id, api_group_id, api_group_version_id, service_id, org_id,
		       operation_id, method, path, summary, description,
		       tags, token_count, parameters, request_body, responses, ord,
		       created_by, updated_by, created_at, updated_at, deleted_at, deleted_by
		FROM api_endpoints
		WHERE api_group_id = $1 AND api_group_version_id IS NULL AND deleted_at IS NULL
		ORDER BY ord ASC, created_at ASC`
	rows, err := d.db.QueryContext(ctx, q, apiGroupID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListAPIEndpoints: %w", err)
	}
	defer rows.Close()
	var out []catalog.APIEndpoint
	for rows.Next() {
		e, err := scanAPIEndpoint(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListAPIEndpoints scan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *DB) UpdateAPIEndpoint(ctx context.Context, e catalog.APIEndpoint) error {
	const q = `
		UPDATE api_endpoints
		SET operation_id=$1, method=$2, path=$3, summary=$4, description=$5,
		    tags=$6, token_count=$7, parameters=$8, request_body=$9, responses=$10, ord=$11,
		    updated_by=$12, updated_at=$13
		WHERE id=$14 AND deleted_at IS NULL`
	tags := e.Tags
	if tags == nil {
		tags = []string{}
	}
	_, err := d.db.ExecContext(ctx, q,
		e.OperationID, e.Method, e.Path, e.Summary, e.Description,
		pq.Array(tags), e.TokenCount, e.Parameters, e.RequestBody, e.Responses, e.Order,
		e.UpdatedBy, time.Now().UTC(), e.ID,
	)
	return wrapErr("UpdateAPIEndpoint", err)
}

func (d *DB) SoftDeleteAPIEndpoint(ctx context.Context, id, deletedBy string) error {
	const q = `UPDATE api_endpoints SET deleted_at=$1, deleted_by=$2 WHERE id=$3 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, id)
	return wrapErr("SoftDeleteAPIEndpoint", err)
}

// SoftDeleteCurrentAPIEndpoints soft-deletes only the working-copy endpoints (api_group_version_id IS NULL).
// Versioned snapshot endpoints are never touched.
func (d *DB) SoftDeleteCurrentAPIEndpoints(ctx context.Context, apiGroupID, deletedBy string) error {
	const q = `UPDATE api_endpoints SET deleted_at=$1, deleted_by=$2
	            WHERE api_group_id=$3 AND api_group_version_id IS NULL AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, apiGroupID)
	return wrapErr("SoftDeleteCurrentAPIEndpoints", err)
}

// CopyEndpointsForVersion snapshots the current working-copy endpoints into versioned rows.
func (d *DB) CopyEndpointsForVersion(ctx context.Context, apiGroupID, versionID, actorID string) error {
	const q = `
		INSERT INTO api_endpoints
			(id, api_group_id, api_group_version_id, service_id, org_id,
			 operation_id, method, path, summary, description,
			 tags, token_count, parameters, request_body, responses, ord,
			 created_by, updated_by, created_at, updated_at)
		SELECT gen_random_uuid(), api_group_id, $2, service_id, org_id,
		       operation_id, method, path, summary, description,
		       tags, token_count, parameters, request_body, responses, ord,
		       created_by, $3, created_at, NOW()
		FROM api_endpoints
		WHERE api_group_id = $1 AND api_group_version_id IS NULL AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, apiGroupID, versionID, actorID)
	return wrapErr("CopyEndpointsForVersion", err)
}

func scanAPIEndpoint(row interface{ Scan(...any) error }) (catalog.APIEndpoint, error) {
	var e catalog.APIEndpoint
	var tags pq.StringArray
	var params, reqBody, resps []byte
	err := row.Scan(
		&e.ID, &e.APIGroupID, &e.APIGroupVersionID, &e.ServiceID, &e.OrgID,
		&e.OperationID, &e.Method, &e.Path, &e.Summary, &e.Description,
		&tags, &e.TokenCount, &params, &reqBody, &resps, &e.Order,
		&e.CreatedBy, &e.UpdatedBy,
		&e.CreatedAt, &e.UpdatedAt, &e.DeletedAt, &e.DeletedBy,
	)
	if err != nil {
		return e, err
	}
	e.Tags = []string(tags)
	if e.Tags == nil {
		e.Tags = []string{}
	}
	e.Parameters = params
	e.RequestBody = reqBody
	e.Responses = resps
	return e, nil
}

// wrapErr wraps a postgres error with a method name prefix; nil passes through.
func wrapErr(method string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("postgres: %s: %w", method, err)
}
