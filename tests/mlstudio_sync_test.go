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
	if created != 0 || updated != 0 {
		t.Fatalf("identical project re-sync = created %d updated %d, want 0/0", created, updated)
	}

	created, updated = syncCounts(t, "/projects/sync", []M{{"name": project, "type": "training", "description": "changed", "team": "uigraph"}})
	if created != 0 || updated != 1 {
		t.Fatalf("changed project re-sync = created %d updated %d, want 0/1", created, updated)
	}

	created, updated = syncCounts(t, "/experiments/sync", []M{
		{"mlflowId": experiment, "projectName": project, "name": "counts", "status": "active"},
	})
	if created != 1 || updated != 0 {
		t.Fatalf("first experiment sync = created %d updated %d, want 1/0", created, updated)
	}

	created, updated = syncCounts(t, "/experiments/sync", []M{
		{"mlflowId": experiment, "projectName": project, "name": "counts", "status": "active"},
	})
	if created != 0 || updated != 0 {
		t.Fatalf("identical experiment re-sync = created %d updated %d, want 0/0", created, updated)
	}

	created, updated = syncCounts(t, "/experiments/sync", []M{
		{"mlflowId": experiment, "projectName": project, "name": "counts renamed", "status": "active"},
	})
	if created != 0 || updated != 1 {
		t.Fatalf("renamed experiment re-sync = created %d updated %d, want 0/1", created, updated)
	}
}

func TestMLSync_RunMetricChangeCountsAsUpdated(t *testing.T) {
	suffix := uniqueSuffix(t)
	project := "metric-project-" + suffix
	experiment := "metric-exp-" + suffix
	run := "metric-run-" + suffix

	mustDo(t, "POST", mlBase()+"/projects/sync", adminToken, []M{{"name": project, "type": "training", "team": "uigraph"}})
	mustDo(t, "POST", mlBase()+"/experiments/sync", adminToken, []M{
		{"mlflowId": experiment, "projectName": project, "name": "metric", "status": "active"},
	})

	runBody := func(accuracy float64) []M {
		return []M{{
			"mlflowId":           run,
			"experimentMlflowId": experiment,
			"name":               "metric-run",
			"status":             "completed",
			"metrics":            M{"accuracy": accuracy},
		}}
	}

	created, updated := syncCounts(t, "/runs/sync", runBody(0.9))
	if created != 1 || updated != 0 {
		t.Fatalf("first run sync = created %d updated %d, want 1/0", created, updated)
	}

	created, updated = syncCounts(t, "/runs/sync", runBody(0.9))
	if created != 0 || updated != 0 {
		t.Fatalf("identical run re-sync = created %d updated %d, want 0/0", created, updated)
	}

	created, updated = syncCounts(t, "/runs/sync", runBody(0.95))
	if created != 0 || updated != 1 {
		t.Fatalf("run re-sync with a changed metric = created %d updated %d, want 0/1", created, updated)
	}
}

func TestMLSync_SoftDeletedRowStaysDeleted(t *testing.T) {
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
	if created != 0 || updated != 0 {
		t.Fatalf("re-sync of a soft-deleted row = created %d updated %d, want 0/0", created, updated)
	}
	if mlDeletedAt(t, "ml_experiments", experiment) == nil {
		t.Fatal("sync resurrected a soft-deleted row, deleted_at is NULL")
	}
}

func mlProjectNames(t *testing.T, query string) map[string]M {
	t.Helper()
	res := mustDo(t, "GET", mlBase()+"/projects"+query, adminToken, nil)
	out := map[string]M{}
	list, ok := res["projects"].([]any)
	if !ok {
		return out
	}
	for _, item := range list {
		p, ok := item.(M)
		if !ok {
			t.Fatalf("project entry is %T, want an object", item)
		}
		out[str(p, "name")] = p
	}
	return out
}

func TestListProjects_IncludeDeletedExposesSoftDeletedProjects(t *testing.T) {
	project := "listdel-project-" + uniqueSuffix(t)

	mustDo(t, "POST", mlBase()+"/projects/sync", adminToken, []M{{"name": project, "type": "training", "team": "uigraph"}})
	if _, ok := mlProjectNames(t, "")[project]; !ok {
		t.Fatalf("project %q missing from the default listing", project)
	}

	_, err := testDB.DB().ExecContext(context.Background(),
		`UPDATE ml_projects SET deleted_at=NOW() WHERE org_id=$1 AND name=$2`, orgID, project)
	if err != nil {
		t.Fatalf("soft delete project: %v", err)
	}

	if _, ok := mlProjectNames(t, "")[project]; ok {
		t.Fatalf("soft-deleted project %q still in the default listing", project)
	}
	if _, ok := mlProjectNames(t, "?includeDeleted=false")[project]; ok {
		t.Fatalf("soft-deleted project %q returned with includeDeleted=false", project)
	}

	found, ok := mlProjectNames(t, "?includeDeleted=true")[project]
	if !ok {
		t.Fatalf("soft-deleted project %q missing with includeDeleted=true", project)
	}
	if str(found, "deletedAt") == "" {
		t.Fatalf("project %q returned without deletedAt: %v", project, found)
	}
}

func TestListProjects_RejectsInvalidIncludeDeleted(t *testing.T) {
	resp := do("GET", mlBase()+"/projects?includeDeleted=garbage", adminToken, nil)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 400 {
		t.Fatalf("includeDeleted=garbage → %d, want 400", resp.StatusCode)
	}
}

func TestMLProjects_RestoreBringsBackADeletedProject(t *testing.T) {
	suffix := uniqueSuffix(t)
	project := "restore-project-" + suffix
	experiment := "restore-exp-" + suffix

	mustDo(t, "POST", mlBase()+"/projects/sync", adminToken, []M{{"name": project, "type": "training", "team": "uigraph"}})
	_, err := testDB.DB().ExecContext(context.Background(),
		`UPDATE ml_projects SET deleted_at=NOW() WHERE org_id=$1 AND name=$2`, orgID, project)
	if err != nil {
		t.Fatalf("soft delete project: %v", err)
	}

	resp := mustDo(t, "POST", mlBase()+"/projects/restore", adminToken, []M{{"name": project}})
	if restored, _ := resp["restored"].(float64); restored != 1 {
		t.Fatalf("restore = %v, want restored 1", resp)
	}

	found, ok := mlProjectNames(t, "")[project]
	if !ok {
		t.Fatalf("restored project %q missing from the default listing", project)
	}
	if str(found, "deletedAt") != "" {
		t.Fatalf("restored project %q still has deletedAt: %v", project, found)
	}

	created, updated := syncCounts(t, "/experiments/sync", []M{
		{"mlflowId": experiment, "projectName": project, "name": "restore", "status": "active"},
	})
	if created != 1 || updated != 0 {
		t.Fatalf("experiment sync under a restored project = created %d updated %d, want 1/0", created, updated)
	}

	resp = mustDo(t, "POST", mlBase()+"/projects/restore", adminToken, []M{{"name": project}})
	if restored, _ := resp["restored"].(float64); restored != 0 {
		t.Fatalf("restoring a live project = %v, want restored 0", resp)
	}
}

func TestMLProjects_RestoreRejectsAnEmptyName(t *testing.T) {
	resp := do("POST", mlBase()+"/projects/restore", adminToken, []M{{"name": ""}})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 400 {
		t.Fatalf("restore with an empty name → %d, want 400", resp.StatusCode)
	}
}

func TestMLSync_ChildOfUnknownProjectIsRejected(t *testing.T) {
	project := "nosuch-project-" + uniqueSuffix(t)
	experiment := "nosuch-exp-" + uniqueSuffix(t)

	resp := do("POST", mlBase()+"/experiments/sync", adminToken, []M{
		{"mlflowId": experiment, "projectName": project, "name": "orphan", "status": "active"},
	})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 400 {
		t.Fatalf("experiment under an unknown project → %d, want 400", resp.StatusCode)
	}
}

func TestMLSync_EvaluationDatasetResolvesWithinItsOwnExperiment(t *testing.T) {
	suffix := uniqueSuffix(t)
	project := "evalds-project-" + suffix
	experimentA := "evalds-exp-a-" + suffix
	experimentB := "evalds-exp-b-" + suffix
	digest := "evalds-digest-" + suffix
	model := "evalds-model-" + suffix
	version := model + "/1"
	evaluation := version + "/evalds-run-" + suffix

	mustDo(t, "POST", mlBase()+"/projects/sync", adminToken, []M{{"name": project, "type": "training", "team": "uigraph"}})
	mustDo(t, "POST", mlBase()+"/experiments/sync", adminToken, []M{
		{"mlflowId": experimentA, "projectName": project, "name": "a", "status": "active"},
		{"mlflowId": experimentB, "projectName": project, "name": "b", "status": "active"},
	})

	mustDo(t, "POST", mlBase()+"/datasets/sync", adminToken, []M{
		{"mlflowId": digest, "experimentMlflowId": experimentA, "name": "ds-a", "digest": digest, "context": "training"},
	})
	mustDo(t, "POST", mlBase()+"/datasets/sync", adminToken, []M{
		{"mlflowId": digest, "experimentMlflowId": experimentB, "name": "ds-b", "digest": digest, "context": "evaluation"},
	})

	mustDo(t, "POST", mlBase()+"/models/sync", adminToken, []M{
		{"mlflowId": model, "projectName": project, "name": model},
	})
	mustDo(t, "POST", mlBase()+"/versions/sync", adminToken, []M{
		{"mlflowId": version, "modelMlflowId": model, "version": "1"},
	})

	mustDo(t, "POST", mlBase()+"/evaluations/sync", adminToken, []M{
		{
			"mlflowId":           evaluation,
			"versionMlflowId":    version,
			"experimentMlflowId": experimentB,
			"datasetMlflowId":    digest,
			"name":               "eval",
			"type":               "Offline Benchmark",
		},
	})

	var datasetName *string
	err := testDB.DB().QueryRowContext(context.Background(),
		`SELECT d.name FROM ml_evaluations e JOIN ml_datasets d ON d.id = e.dataset_id
		 WHERE e.org_id=$1 AND e.mlflow_id=$2`, orgID, evaluation).Scan(&datasetName)
	if err != nil {
		t.Fatalf("read evaluation dataset: %v", err)
	}
	if datasetName == nil || *datasetName != "ds-b" {
		t.Fatalf("evaluation linked to dataset %v, want ds-b (same experiment as the evaluation)", datasetName)
	}
}
