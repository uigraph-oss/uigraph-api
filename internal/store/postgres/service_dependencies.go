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
	if _, err := tx.ExecContext(ctx, `UPDATE service_dependencies SET deleted_at=$1, deleted_by=$2 WHERE source_service_id=$3 AND deleted_at IS NULL`, now, actorID, serviceID); err != nil {
		return fmt.Errorf("postgres: SyncServiceDependencies clear: %w", err)
	}
	for _, dependency := range dependencies {
		var providerID string
		err := tx.QueryRowContext(ctx, `SELECT id FROM services WHERE org_id=$1 AND name=$2 AND status='active' AND deleted_at IS NULL`, orgID, dependency.ProviderServiceName).Scan(&providerID)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("postgres: SyncServiceDependencies provider: %w", err)
		}
		if providerID == serviceID {
			return fmt.Errorf("%w: dependency must not reference its consumer service", store.ErrInvalidDependency)
		}
		var id string
		err = tx.QueryRowContext(ctx, `INSERT INTO service_dependencies (source_service_id, org_id, name, provider_service_name, type, criticality, description, api_group_name, database_name, api_endpoint_names, created_by, updated_by, created_by_commit_hash, updated_by_commit_hash, created_at, updated_at, deleted_at, deleted_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11,$12,$12,$13,$13,NULL,NULL) ON CONFLICT (source_service_id, name) DO UPDATE SET provider_service_name=EXCLUDED.provider_service_name, type=EXCLUDED.type, criticality=EXCLUDED.criticality, description=EXCLUDED.description, api_group_name=EXCLUDED.api_group_name, database_name=EXCLUDED.database_name, api_endpoint_names=EXCLUDED.api_endpoint_names, updated_by=EXCLUDED.updated_by, updated_by_commit_hash=EXCLUDED.updated_by_commit_hash, updated_at=EXCLUDED.updated_at, deleted_at=NULL, deleted_by=NULL RETURNING id`, serviceID, orgID, dependency.Name, dependency.ProviderServiceName, nullableText(dependency.Type), dependency.Criticality, dependency.Description, dependency.APIGroupName, dependency.DatabaseName, pq.Array(dependency.APIEndpointNames), actorID, commitHash, now).Scan(&id)
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
	if direction == "all" {
		upstream, err := d.ListServiceDependencies(ctx, orgID, serviceID, "upstream", criticality)
		if err != nil {
			return nil, err
		}
		downstream, err := d.ListServiceDependencies(ctx, orgID, serviceID, "downstream", criticality)
		if err != nil {
			return nil, err
		}
		return append(upstream, downstream...), nil
	}
	where := `d.org_id=$1 AND d.deleted_at IS NULL`
	args := []any{orgID}
	if serviceID != "" {
		if direction == "downstream" {
			where += ` AND p.id=$2`
			args = append(args, serviceID)
		} else {
			where += ` AND d.source_service_id=$2`
			args = append(args, serviceID)
		}
	}
	if criticality != "" {
		where += fmt.Sprintf(" AND d.criticality=$%d", len(args)+1)
		args = append(args, criticality)
	}
	q := `SELECT d.id, d.source_service_id, d.org_id, d.name, d.provider_service_name, COALESCE(d.type,''), d.criticality, d.description, d.api_group_name, d.database_name, d.created_by, d.updated_by, d.created_by_commit_hash, d.updated_by_commit_hash, d.created_at, d.updated_at, d.deleted_at, d.deleted_by, d.api_endpoint_names, CASE WHEN c.id IS NULL THEN NULL ELSE json_build_object('id', c.id, 'name', c.name, 'description', c.description, 'status', c.status, 'tier', c.tier, 'category', c.category, 'language', c.language, 'gitRepoUrl', c.git_repo_url, 'updatedAt', c.updated_at, 'metadata', c.metadata) END, CASE WHEN p.id IS NULL THEN NULL ELSE json_build_object('id', p.id, 'name', p.name, 'description', p.description, 'status', p.status, 'tier', p.tier, 'category', p.category, 'language', p.language, 'gitRepoUrl', p.git_repo_url, 'updatedAt', p.updated_at, 'metadata', p.metadata) END FROM service_dependencies d LEFT JOIN services c ON c.id=d.source_service_id AND c.deleted_at IS NULL LEFT JOIN services p ON p.org_id=d.org_id AND p.name=d.provider_service_name AND p.status='active' AND p.deleted_at IS NULL WHERE ` + where + ` ORDER BY d.name`
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListServiceDependencies: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanDependencyEdges(rows, direction)
}

func scanDependencyEdges(rows *sql.Rows, direction string) ([]catalog.ServiceDependencyEdge, error) {
	result := []catalog.ServiceDependencyEdge{}
	for rows.Next() {
		var edge catalog.ServiceDependencyEdge
		var endpointNames pq.StringArray
		var consumer, provider []byte
		err := rows.Scan(&edge.ID, &edge.SourceServiceID, &edge.OrgID, &edge.Name, &edge.ProviderServiceName, &edge.Type, &edge.Criticality, &edge.Description, &edge.APIGroupName, &edge.DatabaseName, &edge.CreatedBy, &edge.UpdatedBy, &edge.CreatedByCommitHash, &edge.UpdatedByCommitHash, &edge.CreatedAt, &edge.UpdatedAt, &edge.DeletedAt, &edge.DeletedBy, &endpointNames, &consumer, &provider)
		if err != nil {
			return nil, err
		}
		edge.APIEndpointNames = []string(endpointNames)
		if edge.APIEndpointNames == nil {
			edge.APIEndpointNames = []string{}
		}
		if len(consumer) > 0 {
			edge.Consumer = &catalog.Service{}
			if err := json.Unmarshal(consumer, edge.Consumer); err != nil {
				return nil, err
			}
		}
		if len(provider) > 0 {
			edge.Provider = &catalog.Service{}
			if err := json.Unmarshal(provider, edge.Provider); err != nil {
				return nil, err
			}
		}
		edge.Direction = direction
		result = append(result, edge)
	}
	return result, rows.Err()
}

func (d *DB) DependencyGraph(ctx context.Context, orgID, serviceID string) (catalog.DependencyGraph, error) {
	if serviceID == "" {
		edges, err := d.allDependencyEdges(ctx, orgID)
		if err != nil {
			return catalog.DependencyGraph{}, err
		}
		return graphFromEdges(edges), nil
	}
	upstream, err := d.dependencyGraph(ctx, orgID, serviceID, "upstream", 0)
	if err != nil {
		return catalog.DependencyGraph{}, err
	}
	downstream, err := d.dependencyGraph(ctx, orgID, serviceID, "downstream", 0)
	if err != nil {
		return catalog.DependencyGraph{}, err
	}
	return mergeGraphs(upstream, downstream), nil
}

func mergeGraphs(a, b catalog.DependencyGraph) catalog.DependencyGraph {
	graph := catalog.DependencyGraph{Nodes: []catalog.DependencyGraphNode{}, Edges: []catalog.ServiceDependencyEdge{}}
	seenNode := map[string]bool{}
	for _, list := range [][]catalog.DependencyGraphNode{a.Nodes, b.Nodes} {
		for _, node := range list {
			if seenNode[node.ID] {
				continue
			}
			seenNode[node.ID] = true
			graph.Nodes = append(graph.Nodes, node)
		}
	}
	seenEdge := map[string]bool{}
	for _, list := range [][]catalog.ServiceDependencyEdge{a.Edges, b.Edges} {
		for _, edge := range list {
			if seenEdge[edge.ID] {
				continue
			}
			seenEdge[edge.ID] = true
			graph.Edges = append(graph.Edges, edge)
		}
	}
	return graph
}

func (d *DB) Impact(ctx context.Context, orgID, serviceID, direction string, maxDepth int) (catalog.DependencyGraph, error) {
	return d.dependencyGraph(ctx, orgID, serviceID, direction, maxDepth)
}

func (d *DB) dependencyGraph(ctx context.Context, orgID, serviceID, direction string, maxDepth int) (catalog.DependencyGraph, error) {
	if maxDepth <= 0 {
		maxDepth = 10
	}
	cte := `WITH RECURSIVE walk(service_id, depth, path) AS (SELECT $2::uuid, 0, ARRAY[$2::uuid] UNION ALL SELECT `
	if direction == "downstream" {
		cte += `d.source_service_id, w.depth+1, w.path || d.source_service_id FROM walk w JOIN services p ON p.id=w.service_id JOIN service_dependencies d ON d.org_id=$1 AND d.provider_service_name=p.name AND d.deleted_at IS NULL WHERE w.depth < $3 AND NOT d.source_service_id = ANY(w.path)`
	} else {
		cte += `p.id, w.depth+1, w.path || p.id FROM walk w JOIN service_dependencies d ON d.source_service_id=w.service_id AND d.org_id=$1 AND d.deleted_at IS NULL JOIN services p ON p.org_id=d.org_id AND p.name=d.provider_service_name AND p.status='active' AND p.deleted_at IS NULL WHERE w.depth < $3 AND NOT p.id = ANY(w.path)`
	}
	cte += `) SELECT DISTINCT service_id FROM walk`
	rows, err := d.db.QueryContext(ctx, cte, orgID, serviceID, maxDepth)
	if err != nil {
		return catalog.DependencyGraph{}, fmt.Errorf("postgres: dependency graph walk: %w", err)
	}
	defer func() { _ = rows.Close() }()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return catalog.DependencyGraph{}, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return catalog.DependencyGraph{}, err
	}
	all, err := d.allDependencyEdges(ctx, orgID)
	if err != nil {
		return catalog.DependencyGraph{}, err
	}
	allowed := map[string]bool{}
	for _, id := range ids {
		allowed[id] = true
	}
	graph := catalog.DependencyGraph{Nodes: []catalog.DependencyGraphNode{}, Edges: []catalog.ServiceDependencyEdge{}}
	nodes := map[string]catalog.DependencyGraphNode{}
	for _, edge := range all {
		providerID := "ghost:" + edge.ProviderServiceName
		if edge.Provider != nil {
			providerID = edge.Provider.ID
		}
		if !allowed[edge.SourceServiceID] || (edge.Provider != nil && !allowed[providerID]) {
			continue
		}
		graph.Edges = append(graph.Edges, edge)
		if edge.Consumer != nil {
			nodes[edge.Consumer.ID] = catalog.DependencyGraphNode{ID: edge.Consumer.ID, Name: edge.Consumer.Name, Service: edge.Consumer}
		}
		if edge.Provider != nil {
			nodes[providerID] = catalog.DependencyGraphNode{ID: providerID, Name: edge.Provider.Name, Service: edge.Provider}
		} else {
			nodes[providerID] = catalog.DependencyGraphNode{ID: providerID, Name: edge.ProviderServiceName}
		}
	}
	for _, node := range nodes {
		graph.Nodes = append(graph.Nodes, node)
	}
	return graph, nil
}

func graphFromEdges(edges []catalog.ServiceDependencyEdge) catalog.DependencyGraph {
	graph := catalog.DependencyGraph{Edges: edges, Nodes: []catalog.DependencyGraphNode{}}
	nodes := map[string]catalog.DependencyGraphNode{}
	for _, edge := range edges {
		if edge.Consumer != nil {
			nodes[edge.Consumer.ID] = catalog.DependencyGraphNode{ID: edge.Consumer.ID, Name: edge.Consumer.Name, Service: edge.Consumer}
		}
		if edge.Provider != nil {
			nodes[edge.Provider.ID] = catalog.DependencyGraphNode{ID: edge.Provider.ID, Name: edge.Provider.Name, Service: edge.Provider}
			continue
		}
		id := "ghost:" + edge.ProviderServiceName
		nodes[id] = catalog.DependencyGraphNode{ID: id, Name: edge.ProviderServiceName}
	}
	for _, node := range nodes {
		graph.Nodes = append(graph.Nodes, node)
	}
	return graph
}

func (d *DB) allDependencyEdges(ctx context.Context, orgID string) ([]catalog.ServiceDependencyEdge, error) {
	return d.ListServiceDependencies(ctx, orgID, "", "upstream", "")
}
