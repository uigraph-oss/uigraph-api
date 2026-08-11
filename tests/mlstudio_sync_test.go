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

func TestMLSync_SoftDeletedRowCountsAsUpdatedAndStaysDeleted(t *testing.T) {
	suffix := uniqueSuffix(t)
	project := "softdel-project-" + suffix
	experiment := "softdel-exp-" + suffix

	mustDo(t, "POST", mlBase()+"/projects/sync", adminToken, []M{{"name": project, "type": "training", "team": "uigraph"}})
	created, updated := syncCounts(t, "/experiments/sync", []M{
		{"mlflowId": experiment, "projectName": project, "name": "softdel", "status": "active"},
	})
	if created != 1 || updated != 0 {
		t.Fatalf("first experiment sync = created %d updated %d, want 1/0", created, updated)
	}

	_, err := testDB.DB().ExecContext(context.Background(),
		`UPDATE ml_experiments SET deleted_at=NOW() WHERE org_id=$1 AND mlflow_id=$2`, orgID, experiment)
	if err != nil {
		t.Fatalf("soft delete experiment: %v", err)
	}

	created, updated = syncCounts(t, "/experiments/sync", []M{
		{"mlflowId": experiment, "projectName": project, "name": "softdel", "status": "active"},
	})
	if created != 0 || updated != 1 {
		t.Fatalf("re-sync of a soft-deleted row = created %d updated %d, want 0/1", created, updated)
	}
	if mlDeletedAt(t, "ml_experiments", experiment) == nil {
		t.Fatal("sync resurrected a soft-deleted row, deleted_at is NULL")
	}
}
