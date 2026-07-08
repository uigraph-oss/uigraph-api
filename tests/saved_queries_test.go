package tests

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

func createTestServiceDB(t *testing.T, serviceName string) (serviceID, dbID string) {
	t.Helper()
	team := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/teams", adminToken, M{
		"name": fmt.Sprintf("Test Team %d", time.Now().UnixNano()),
	})
	svc := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/services", adminToken, M{
		"name": serviceName, "description": "test service", "language": "Go", "teamId": str(team, "id"),
	})
	serviceID = str(svc, "id")

	db := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/dbs", adminToken, M{
		"dbName": "primary", "dbType": "PostgreSQL", "dialect": "postgresql",
	})
	dbID = str(db, "id")
	return serviceID, dbID
}

func queriesPath(serviceID, dbID, suffix string) string {
	return "/api/v1/orgs/" + orgID + "/services/" + serviceID + "/dbs/" + dbID + "/queries" + suffix
}

// ── CRUD ──────────────────────────────────────────────────────────────────────

func TestSavedQueries_CRUD(t *testing.T) {
	serviceID, dbID := createTestServiceDB(t, fmt.Sprintf("queries-crud-%d", time.Now().UnixNano()))

	created := mustDo(t, "POST", queriesPath(serviceID, dbID, ""), adminToken, M{
		"title": "Billing Query", "description": "", "queryText": "select * from billing",
		"tags": []string{"billing"}, "scope": "personal",
	})
	id := str(created, "id")
	if id == "" {
		t.Fatal("expected id in create response")
	}
	if created["scope"] != "personal" {
		t.Fatalf("want scope=personal, got %v", created["scope"])
	}

	listBody := mustDo(t, "GET", queriesPath(serviceID, dbID, "?scope=personal"), adminToken, nil)
	found := false
	for _, q := range list(listBody, "queries") {
		if str(q, "id") == id {
			found = true
		}
	}
	if !found {
		t.Fatal("created query not in personal list")
	}

	// Team-scope list must not see the personal query.
	teamList := mustDo(t, "GET", queriesPath(serviceID, dbID, "?scope=team"), adminToken, nil)
	for _, q := range list(teamList, "queries") {
		if str(q, "id") == id {
			t.Fatal("personal query leaked into team list")
		}
	}

	updated := mustDo(t, "PUT", queriesPath(serviceID, dbID, "/"+id), adminToken, M{
		"title": "Billing Query v2",
	})
	if updated["title"] != "Billing Query v2" {
		t.Fatalf("want title=Billing Query v2, got %v", updated["title"])
	}

	r := do("DELETE", queriesPath(serviceID, dbID, "/"+id), adminToken, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", r.StatusCode)
	}
	listBody = mustDo(t, "GET", queriesPath(serviceID, dbID, "?scope=personal"), adminToken, nil)
	for _, q := range list(listBody, "queries") {
		if str(q, "id") == id {
			t.Fatal("deleted query still in list")
		}
	}
}

// ── Folder delete clears member queries ─────────────────────────────────────

func TestSavedQueryFolders_DeleteClearsMemberQueries(t *testing.T) {
	serviceID, dbID := createTestServiceDB(t, fmt.Sprintf("folders-%d", time.Now().UnixNano()))

	folder := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/dbs/"+dbID+"/query-folders", adminToken, M{
		"name": "My Folder", "scope": "personal",
	})
	folderID := str(folder, "id")

	q := mustDo(t, "POST", queriesPath(serviceID, dbID, ""), adminToken, M{
		"title": "In Folder", "queryText": "select 1", "scope": "personal", "folderId": folderID,
	})
	queryID := str(q, "id")

	r := do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/dbs/"+dbID+"/query-folders/"+folderID, adminToken, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", r.StatusCode)
	}

	got := mustDo(t, "GET", queriesPath(serviceID, dbID, "?scope=personal"), adminToken, nil)
	for _, item := range list(got, "queries") {
		if str(item, "id") == queryID {
			if item["folderId"] != nil {
				t.Fatalf("want folderId cleared after folder delete, got %v", item["folderId"])
			}
		}
	}
}

// ── CI sync dedup ─────────────────────────────────────────────────────────────

func TestSavedQueries_SyncIsIdempotent(t *testing.T) {
	serviceID, dbID := createTestServiceDB(t, fmt.Sprintf("sync-dedup-%d", time.Now().UnixNano()))
	sourceRef := "top-customers"

	first := mustDo(t, "POST", queriesPath(serviceID, dbID, "/sync"), adminToken, M{
		"sourceRef": sourceRef, "title": "Top Customers", "queryText": "select 1",
	})
	if first["created"] != true {
		t.Fatalf("want created=true on first sync, got %v", first["created"])
	}
	firstID := str(first, "id")

	second := mustDo(t, "POST", queriesPath(serviceID, dbID, "/sync"), adminToken, M{
		"sourceRef": sourceRef, "title": "Top Customers v2", "queryText": "select 2",
	})
	if second["created"] != false {
		t.Fatalf("want created=false on second sync, got %v", second["created"])
	}
	if str(second, "id") != firstID {
		t.Fatalf("want same id on re-sync, got %s vs %s", str(second, "id"), firstID)
	}

	got := mustDo(t, "GET", queriesPath(serviceID, dbID, "?scope=team"), adminToken, nil)
	count := 0
	for _, q := range list(got, "queries") {
		if str(q, "id") == firstID {
			count++
			if str(q, "title") != "Top Customers v2" {
				t.Fatalf("want updated title after re-sync, got %v", q["title"])
			}
		}
	}
	if count != 1 {
		t.Fatalf("want exactly 1 row for sourceRef %q, got %d", sourceRef, count)
	}
}

// TestSavedQueries_SyncIsRaceFree fires concurrent syncs of the same sourceRef
// and asserts the Postgres unique-constraint upsert prevents duplicates — this
// is the core "no duplicate records from CLI sync" requirement.
func TestSavedQueries_SyncIsRaceFree(t *testing.T) {
	serviceID, dbID := createTestServiceDB(t, fmt.Sprintf("sync-race-%d", time.Now().UnixNano()))
	sourceRef := "concurrent-query"

	const n = 10
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp := do("POST", queriesPath(serviceID, dbID, "/sync"), adminToken, M{
				"sourceRef": sourceRef, "title": fmt.Sprintf("Concurrent %d", i), "queryText": "select 1",
			})
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("sync %d: want 200, got %d", i, resp.StatusCode)
			}
		}(i)
	}
	wg.Wait()

	// This db has no other queries, so every team-scope row belongs to sourceRef.
	got := mustDo(t, "GET", queriesPath(serviceID, dbID, "?scope=team"), adminToken, nil)
	ids := map[string]bool{}
	for _, q := range list(got, "queries") {
		ids[str(q, "id")] = true
	}
	if len(ids) != 1 {
		t.Fatalf("want exactly 1 distinct row after %d concurrent syncs of the same sourceRef, got %d", n, len(ids))
	}
}
