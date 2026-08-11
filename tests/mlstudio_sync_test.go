package tests

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

func mlBase() string {
	return "/api/v1/orgs/" + orgID + "/ml"
}

func uniqueSuffix(t *testing.T) string {
	t.Helper()
	return uuid.NewString()[:8]
}

func syncCounts(t *testing.T, path string, body any) (int, int) {
	t.Helper()
	res := mustDo(t, "POST", mlBase()+path, adminToken, body)
	created, _ := res["created"].(float64)
	updated, _ := res["updated"].(float64)
	return int(created), int(updated)
}

func mlDeletedAt(t *testing.T, table, mlflowID string) *string {
	t.Helper()
	var deletedAt *string
	err := testDB.DB().QueryRowContext(context.Background(),
		fmt.Sprintf(`SELECT deleted_at::text FROM %s WHERE org_id=$1 AND mlflow_id=$2`, table),
		orgID, mlflowID).Scan(&deletedAt)
	if err != nil {
		t.Fatalf("read %s %s: %v", table, mlflowID, err)
	}
	return deletedAt
}

func TestMLSync_CountsCreatedThenUpdated(t *testing.T) {
	project := "counts-project-" + uniqueSuffix(t)
	experiment := "counts-exp-" + uniqueSuffix(t)

	created, updated := syncCounts(t, "/projects/sync", []M{{"name": project, "type": "training", "team": "uigraph"}})
	if created != 1 || updated != 0 {
		t.Fatalf("first project sync = created %d updated %d, want 1/0", created, updated)
	}

	created, updated = syncCounts(t, "/projects/sync", []M{{"name": project, "type": "training", "team": "uigraph"}})
	if created != 0 || updated != 1 {
		t.Fatalf("second project sync = created %d updated %d, want 0/1", created, updated)
	}

	created, updated = syncCounts(t, "/experiments/sync", []M{
		{"mlflowId": experiment, "projectName": project, "name": "counts", "status": "active"},
	})
	if created != 1 || updated != 0 {
		t.Fatalf("first experiment sync = created %d updated %d, want 1/0", created, updated)
	}

	created, updated = syncCounts(t, "/experiments/sync", []M{
		{"mlflowId": experiment, "projectName": project, "name": "counts renamed", "status": "active"},
	})
	if created != 0 || updated != 1 {
		t.Fatalf("second experiment sync = created %d updated %d, want 0/1", created, updated)
	}
}

func TestMLSync_PruneRunSoftDeletesArtifactsAndRevives(t *testing.T) {
	suffix := uniqueSuffix(t)
	project := "prune-project-" + suffix
	experiment := "prune-exp-" + suffix
	run := "prune-run-" + suffix
	artifact := "prune-artifact-" + suffix

	mustDo(t, "POST", mlBase()+"/projects/sync", adminToken, []M{{"name": project, "type": "training", "team": "uigraph"}})
	mustDo(t, "POST", mlBase()+"/experiments/sync", adminToken, []M{
		{"mlflowId": experiment, "projectName": project, "name": "prune", "status": "active"},
	})
	mustDo(t, "POST", mlBase()+"/runs/sync", adminToken, []M{
		{"mlflowId": run, "experimentMlflowId": experiment, "name": "r1", "status": "finished"},
	})
	mustDo(t, "POST", mlBase()+"/artifacts/sync", adminToken, []M{
		{"mlflowId": artifact, "runMlflowId": run, "name": "model.pkl", "type": "model"},
	})

	res := mustDo(t, "POST", mlBase()+"/runs/prune", adminToken, []M{{"mlflowId": run}})
	if deleted, _ := res["deleted"].(float64); deleted != 1 {
		t.Fatalf("prune deleted = %v, want 1", res["deleted"])
	}
	if mlDeletedAt(t, "ml_runs", run) == nil {
		t.Fatal("run deleted_at is NULL after prune")
	}
	if mlDeletedAt(t, "ml_artifacts", artifact) == nil {
		t.Fatal("artifact deleted_at is NULL after prune, soft delete did not cascade")
	}

	res = mustDo(t, "POST", mlBase()+"/runs/prune", adminToken, []M{{"mlflowId": run}})
	if deleted, _ := res["deleted"].(float64); deleted != 0 {
		t.Fatalf("second prune deleted = %v, want 0", res["deleted"])
	}

	created, updated := syncCounts(t, "/runs/sync", []M{
		{"mlflowId": run, "experimentMlflowId": experiment, "name": "r1", "status": "finished"},
	})
	if created != 1 || updated != 0 {
		t.Fatalf("revived run = created %d updated %d, want 1/0", created, updated)
	}
	if at := mlDeletedAt(t, "ml_runs", run); at != nil {
		t.Fatalf("revived run deleted_at = %v, want NULL", *at)
	}
}

func TestMLSync_PruneVersionsHonoursKeep(t *testing.T) {
	suffix := uniqueSuffix(t)
	project := "vprune-project-" + suffix
	model := "vprune-model-" + suffix
	keptID := model + "/1"
	staleID := model + "/2"

	mustDo(t, "POST", mlBase()+"/projects/sync", adminToken, []M{{"name": project, "type": "model", "team": "uigraph"}})
	mustDo(t, "POST", mlBase()+"/models/sync", adminToken, []M{
		{"mlflowId": model, "projectName": project, "name": model},
	})
	mustDo(t, "POST", mlBase()+"/versions/sync", adminToken, []M{
		{"mlflowId": keptID, "modelMlflowId": model, "version": "1"},
	})
	mustDo(t, "POST", mlBase()+"/versions/sync", adminToken, []M{
		{"mlflowId": staleID, "modelMlflowId": model, "version": "2"},
	})
	mustDo(t, "POST", mlBase()+"/models/sync", adminToken, []M{
		{"mlflowId": model, "projectName": project, "name": model, "productionVersionMlflowId": staleID},
	})

	res := mustDo(t, "POST", mlBase()+"/versions/prune", adminToken, M{
		"modelMlflowId": model, "keep": []string{keptID},
	})
	if deleted, _ := res["deleted"].(float64); deleted != 1 {
		t.Fatalf("prune deleted = %v, want 1", res["deleted"])
	}
	if mlDeletedAt(t, "ml_model_versions", staleID) == nil {
		t.Fatal("stale version deleted_at is NULL after prune")
	}
	if at := mlDeletedAt(t, "ml_model_versions", keptID); at != nil {
		t.Fatalf("kept version deleted_at = %v, want NULL", *at)
	}

	var production *string
	err := testDB.DB().QueryRowContext(context.Background(),
		`SELECT production_version_id::text FROM ml_models WHERE org_id=$1 AND mlflow_id=$2`, orgID, model).Scan(&production)
	if err != nil {
		t.Fatalf("read model: %v", err)
	}
	if production != nil {
		t.Fatalf("production_version_id = %v, want NULL after its version was pruned", *production)
	}
}

func TestMLSync_PruneVersionsAcceptsEmptyKeep(t *testing.T) {
	suffix := uniqueSuffix(t)
	project := "vempty-project-" + suffix
	model := "vempty-model-" + suffix
	versionID := model + "/1"

	mustDo(t, "POST", mlBase()+"/projects/sync", adminToken, []M{{"name": project, "type": "model", "team": "uigraph"}})
	mustDo(t, "POST", mlBase()+"/models/sync", adminToken, []M{
		{"mlflowId": model, "projectName": project, "name": model},
	})
	mustDo(t, "POST", mlBase()+"/versions/sync", adminToken, []M{
		{"mlflowId": versionID, "modelMlflowId": model, "version": "1"},
	})

	res := mustDo(t, "POST", mlBase()+"/versions/prune", adminToken, M{
		"modelMlflowId": model, "keep": []string{},
	})
	if deleted, _ := res["deleted"].(float64); deleted != 1 {
		t.Fatalf("prune deleted = %v, want 1 (an empty keep list prunes everything)", res["deleted"])
	}
	if mlDeletedAt(t, "ml_model_versions", versionID) == nil {
		t.Fatal("version deleted_at is NULL after prune with an empty keep list")
	}
}
