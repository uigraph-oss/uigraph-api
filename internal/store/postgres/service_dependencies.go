package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/uigraph/app/internal/catalog"
	"github.com/uigraph/app/internal/store"
)

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (d *DB) SyncServiceDependencies(ctx context.Context, orgID, serviceID, actorID string, commitHash *string, dependencies []catalog.ServiceDependency) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres: SyncServiceDependencies begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `UPDATE service_dependencies SET deleted_at=$1, deleted_by=$2 WHERE service_id=$3 AND deleted_at IS NULL`, now, actorID, serviceID); err != nil {
		return fmt.Errorf("postgres: SyncServiceDependencies clear: %w", err)
	}
	for _, dependency := range dependencies {
		var dependencyID *string
		var resolved string
		err := tx.QueryRowContext(ctx, `SELECT id FROM services WHERE org_id=$1 AND name=$2 AND status='active' AND deleted_at IS NULL`, orgID, dependency.DependencyName).Scan(&resolved)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("postgres: SyncServiceDependencies resolve dependency: %w", err)
		}
		if err == nil {
			if resolved == serviceID {
				return fmt.Errorf("%w: dependency must not reference its own service", store.ErrInvalidDependency)
			}
			dependencyID = &resolved
		}
		var id string
		err = tx.QueryRowContext(ctx, `INSERT INTO service_dependencies (service_id, dependency_id, dependency_name, direction, org_id, name, type, criticality, description, api_group_name, database_name, api_endpoint_names, created_by, updated_by, created_by_commit_hash, updated_by_commit_hash, created_at, updated_at, deleted_at, deleted_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13,$14,$14,$15,$15,NULL,NULL) ON CONFLICT (service_id, name) DO UPDATE SET dependency_id=EXCLUDED.dependency_id, dependency_name=EXCLUDED.dependency_name, direction=EXCLUDED.direction, type=EXCLUDED.type, criticality=EXCLUDED.criticality, description=EXCLUDED.description, api_group_name=EXCLUDED.api_group_name, database_name=EXCLUDED.database_name, api_endpoint_names=EXCLUDED.api_endpoint_names, updated_by=EXCLUDED.updated_by, updated_by_commit_hash=EXCLUDED.updated_by_commit_hash, updated_at=EXCLUDED.updated_at, deleted_at=NULL, deleted_by=NULL RETURNING id`, serviceID, dependencyID, dependency.DependencyName, dependency.Direction, orgID, dependency.Name, nullableText(dependency.Type), dependency.Criticality, dependency.Description, dependency.APIGroupName, dependency.DatabaseName, pq.Array(dependency.APIEndpointNames), actorID, commitHash, now).Scan(&id)
		if err != nil {
			return fmt.Errorf("postgres: SyncServiceDependencies upsert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres: SyncServiceDependencies commit: %w", err)
	}
	return nil
}

func (d *DB) ListServiceDependencies(ctx context.Context, orgID, serviceID, direction, criticality string) ([]catalog.ServiceDependencyEdge, error) {
	where := `d.org_id=$1 AND d.deleted_at IS NULL`
	args := []any{orgID}
	if serviceID != "" {
		where += fmt.Sprintf(" AND d.service_id=$%d", len(args)+1)
		args = append(args, serviceID)
	}
	if direction != "" && direction != "all" {
		where += fmt.Sprintf(" AND d.direction=$%d", len(args)+1)
		args = append(args, direction)
	}
	if criticality != "" {
		where += fmt.Sprintf(" AND d.criticality=$%d", len(args)+1)
		args = append(args, criticality)
	}
	q := `SELECT d.id, d.service_id, d.dependency_id, d.dependency_name, d.direction, d.org_id, d.name, COALESCE(d.type,''), d.criticality, d.description, d.api_group_name, d.database_name, d.created_by, d.updated_by, d.created_by_commit_hash, d.updated_by_commit_hash, d.created_at, d.updated_at, d.deleted_at, d.deleted_by, d.api_endpoint_names, CASE WHEN s.id IS NULL THEN NULL ELSE json_build_object('id', s.id, 'name', s.name, 'description', s.description, 'status', s.status, 'tier', s.tier, 'category', s.category, 'language', s.language, 'gitRepoUrl', s.git_repo_url, 'updatedAt', s.updated_at, 'metadata', s.metadata) END, CASE WHEN dp.id IS NULL THEN NULL ELSE json_build_object('id', dp.id, 'name', dp.name, 'description', dp.description, 'status', dp.status, 'tier', dp.tier, 'category', dp.category, 'language', dp.language, 'gitRepoUrl', dp.git_repo_url, 'updatedAt', dp.updated_at, 'metadata', dp.metadata) END FROM service_dependencies d LEFT JOIN services s ON s.id=d.service_id AND s.deleted_at IS NULL LEFT JOIN services dp ON dp.id=d.dependency_id AND dp.status='active' AND dp.deleted_at IS NULL WHERE ` + where + ` ORDER BY d.name`
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListServiceDependencies: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanDependencyEdges(rows)
}

func scanDependencyEdges(rows *sql.Rows) ([]catalog.ServiceDependencyEdge, error) {
	result := []catalog.ServiceDependencyEdge{}
	for rows.Next() {
		var edge catalog.ServiceDependencyEdge
		var endpointNames pq.StringArray
		var service, dependency []byte
		err := rows.Scan(&edge.ID, &edge.ServiceID, &edge.DependencyID, &edge.DependencyName, &edge.Direction, &edge.OrgID, &edge.Name, &edge.Type, &edge.Criticality, &edge.Description, &edge.APIGroupName, &edge.DatabaseName, &edge.CreatedBy, &edge.UpdatedBy, &edge.CreatedByCommitHash, &edge.UpdatedByCommitHash, &edge.CreatedAt, &edge.UpdatedAt, &edge.DeletedAt, &edge.DeletedBy, &endpointNames, &service, &dependency)
		if err != nil {
			return nil, err
		}
		edge.APIEndpointNames = []string(endpointNames)
		if edge.APIEndpointNames == nil {
			edge.APIEndpointNames = []string{}
		}
		if len(service) > 0 {
			edge.Service = &catalog.Service{}
			if err := json.Unmarshal(service, edge.Service); err != nil {
				return nil, err
			}
		}
		if len(dependency) > 0 {
			edge.Dependency = &catalog.Service{}
			if err := json.Unmarshal(dependency, edge.Dependency); err != nil {
				return nil, err
			}
		}
		result = append(result, edge)
	}
	return result, rows.Err()
}

func (d *DB) DependencyGraph(ctx context.Context, orgID, serviceID string) ([]catalog.ServiceDependencyEdge, error) {
	if serviceID == "" {
		return d.allDependencyEdges(ctx, orgID)
	}
	up, err := d.dependencyGraph(ctx, orgID, serviceID, "upstream", 0)
	if err != nil {
		return nil, err
	}
	down, err := d.dependencyGraph(ctx, orgID, serviceID, "downstream", 0)
	if err != nil {
		return nil, err
	}
	return dedupeEdges(up, down), nil
}

func dedupeEdges(a, b []catalog.ServiceDependencyEdge) []catalog.ServiceDependencyEdge {
	edges := []catalog.ServiceDependencyEdge{}
	seen := map[string]bool{}
	for _, list := range [][]catalog.ServiceDependencyEdge{a, b} {
		for _, edge := range list {
			if seen[edge.ID] {
				continue
			}
			seen[edge.ID] = true
			edges = append(edges, edge)
		}
	}
	return edges
}

func (d *DB) Impact(ctx context.Context, orgID, serviceID, direction string, maxDepth int) ([]catalog.ServiceDependencyEdge, error) {
	return d.dependencyGraph(ctx, orgID, serviceID, direction, maxDepth)
}

func (d *DB) dependencyGraph(ctx context.Context, orgID, serviceID, direction string, maxDepth int) ([]catalog.ServiceDependencyEdge, error) {
	if maxDepth <= 0 {
		maxDepth = 10
	}
	var ownSide, otherSide string
	if direction == "upstream" {
		ownSide, otherSide = "downstream", "upstream"
	} else if direction == "downstream" {
		ownSide, otherSide = "upstream", "downstream"
	} else {
		return nil, fmt.Errorf("postgres: dependency graph walk: direction must be upstream or downstream, got %q", direction)
	}
	cte := `WITH RECURSIVE walk(service_id, depth, path) AS (SELECT $2::uuid, 0, ARRAY[$2::uuid] UNION ALL SELECT next.id, w.depth+1, w.path || next.id FROM walk w JOIN service_dependencies d ON d.org_id=$1 AND d.deleted_at IS NULL AND ((d.service_id=w.service_id AND d.direction='` + ownSide + `') OR (d.dependency_id=w.service_id AND d.direction='` + otherSide + `')) JOIN services next ON next.id=CASE WHEN d.service_id=w.service_id THEN d.dependency_id ELSE d.service_id END AND next.org_id=$1 AND next.status='active' AND next.deleted_at IS NULL WHERE w.depth < $3 AND NOT next.id = ANY(w.path)) SELECT DISTINCT service_id FROM walk`
	rows, err := d.db.QueryContext(ctx, cte, orgID, serviceID, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("postgres: dependency graph walk: %w", err)
	}
	defer func() { _ = rows.Close() }()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	all, err := d.allDependencyEdges(ctx, orgID)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, id := range ids {
		allowed[id] = true
	}
	edges := []catalog.ServiceDependencyEdge{}
	for _, edge := range all {
		dependencyID := "ghost:" + edge.DependencyName
		if edge.Dependency != nil {
			dependencyID = edge.Dependency.ID
		}
		if !allowed[edge.ServiceID] || (edge.Dependency != nil && !allowed[dependencyID]) {
			continue
		}
		edges = append(edges, edge)
	}
	return edges, nil
}

func (d *DB) allDependencyEdges(ctx context.Context, orgID string) ([]catalog.ServiceDependencyEdge, error) {
	return d.ListServiceDependencies(ctx, orgID, "", "all", "")
}
