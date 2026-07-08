package tests

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// ── Services ──────────────────────────────────────────────────────────────────

func TestServices_CRUD(t *testing.T) {
	name := fmt.Sprintf("payments-%d", time.Now().UnixNano())

	// Create
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/services", adminToken, M{
		"name":        name,
		"description": "handles payments",
		"tier":        "tier1",
		"language":    "Go",
		"labels":      []string{"payments", "critical"},
	})
	id := str(created, "id")
	if id == "" {
		t.Fatal("expected id in create response")
	}
	if created["status"] != "active" {
		t.Fatalf("want status=active, got %v", created["status"])
	}
	if created["tier"] != "tier1" {
		t.Fatalf("want tier=tier1, got %v", created["tier"])
	}

	// Get
	got := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/services/"+id, adminToken, nil)
	if got["language"] != "Go" {
		t.Fatalf("want language=Go, got %v", got["language"])
	}

	// List
	body := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/services", adminToken, nil)
	found := false
	for _, s := range list(body, "services") {
		if str(s, "id") == id {
			found = true
		}
	}
	if !found {
		t.Fatal("created service not in list")
	}

	// Update
	updated := mustDo(t, "PUT", "/api/v1/orgs/"+orgID+"/services/"+id, adminToken, M{
		"status":        "deprecated",
		"gitRepoUrl":    "https://github.com/example/payments",
		"lastCommitSha": "abc123",
	})
	if updated["status"] != "deprecated" {
		t.Fatalf("want status=deprecated, got %v", updated["status"])
	}
	if updated["gitRepoUrl"] != "https://github.com/example/payments" {
		t.Fatalf("want gitRepoUrl, got %v", updated["gitRepoUrl"])
	}

	// Delete
	r := do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+id, adminToken, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", r.StatusCode)
	}

	r = do("GET", "/api/v1/orgs/"+orgID+"/services/"+id, adminToken, nil)
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 after delete, got %d", r.StatusCode)
	}
}

func TestServices_Labels(t *testing.T) {
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/services", adminToken, M{
		"name":   fmt.Sprintf("labeled-%d", time.Now().UnixNano()),
		"labels": []string{"auth", "core", "internal"},
	})
	id := str(created, "id")

	got := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/services/"+id, adminToken, nil)
	rawLabels, _ := got["labels"].([]any)
	if len(rawLabels) != 3 {
		t.Fatalf("want 3 labels, got %v", rawLabels)
	}

	do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+id, adminToken, nil)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func createTestService(t *testing.T) string {
	t.Helper()
	s := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/services", adminToken, M{
		"name": fmt.Sprintf("svc-%d", time.Now().UnixNano()),
	})
	return str(s, "id")
}

// ── API Groups ────────────────────────────────────────────────────────────────

const sampleSpec = `openapi: "3.0.0"
info:
  title: Payments API
  version: v1
paths:
  /payments:
    get:
      operationId: listPayments
      summary: List payments
      responses:
        "200":
          description: OK`

const updatedSpec = `openapi: "3.0.0"
info:
  title: Payments API
  version: v2
paths:
  /payments:
    get:
      operationId: listPayments
      summary: List payments
      responses:
        "200":
          description: OK
  /payments/{id}:
    get:
      operationId: getPayment
      summary: Get a payment
      responses:
        "200":
          description: OK`

func TestAPIGroups_CRUD(t *testing.T) {
	serviceID := createTestService(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+serviceID, adminToken, nil) })

	// Create with spec
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/api-groups", adminToken, M{
		"name":     "payments-api",
		"version":  "v1",
		"protocol": "REST",
		"spec":     sampleSpec,
	})
	id := str(created, "id")
	if id == "" {
		t.Fatal("expected id in create response")
	}
	if created["protocol"] != "REST" {
		t.Fatalf("want protocol=REST, got %v", created["protocol"])
	}
	if str(created, "specHash") == "" {
		t.Fatal("expected specHash when spec provided")
	}

	// Get
	got := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/api-groups/"+id, adminToken, nil)
	if got["name"] != "payments-api" {
		t.Fatalf("want name=payments-api, got %v", got["name"])
	}

	// List
	body := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/api-groups", adminToken, nil)
	found := false
	for _, g := range list(body, "apiGroups") {
		if str(g, "id") == id {
			found = true
		}
	}
	if !found {
		t.Fatal("created api group not in list")
	}

	// Auto-version created on create
	vBody := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/api-groups/"+id+"/versions", adminToken, nil)
	if n := len(list(vBody, "versions")); n != 1 {
		t.Fatalf("want 1 auto-version after create, got %d", n)
	}

	// Update with new spec → auto-version 2
	mustDo(t, "PUT", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/api-groups/"+id, adminToken, M{
		"spec": updatedSpec,
	})
	vBody = mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/api-groups/"+id+"/versions", adminToken, nil)
	if n := len(list(vBody, "versions")); n != 2 {
		t.Fatalf("want 2 versions after spec update, got %d", n)
	}

	// Delete
	r := do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/api-groups/"+id, adminToken, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", r.StatusCode)
	}
}

// ── API Endpoints ─────────────────────────────────────────────────────────────

func createTestAPIGroup(t *testing.T, serviceID string) string {
	t.Helper()
	g := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/api-groups", adminToken, M{
		"name": fmt.Sprintf("api-%d", time.Now().UnixNano()), "protocol": "REST",
	})
	return str(g, "id")
}

func TestAPIEndpoints_CRUD(t *testing.T) {
	serviceID := createTestService(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+serviceID, adminToken, nil) })
	apiGroupID := createTestAPIGroup(t, serviceID)

	// Create
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/api-groups/"+apiGroupID+"/endpoints", adminToken, M{
		"operationId": "listPayments",
		"method":      "GET",
		"path":        "/payments",
		"summary":     "List all payments",
		"tags":        []string{"payments"},
	})
	id := str(created, "id")
	if id == "" {
		t.Fatal("expected id in create response")
	}
	if created["method"] != "GET" {
		t.Fatalf("want method=GET, got %v", created["method"])
	}
	if created["path"] != "/payments" {
		t.Fatalf("want path=/payments, got %v", created["path"])
	}

	// Get
	got := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/api-groups/"+apiGroupID+"/endpoints/"+id, adminToken, nil)
	if got["operationId"] != "listPayments" {
		t.Fatalf("want operationId=listPayments, got %v", got["operationId"])
	}

	// List
	body := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/api-groups/"+apiGroupID+"/endpoints", adminToken, nil)
	found := false
	for _, e := range list(body, "endpoints") {
		if str(e, "id") == id {
			found = true
		}
	}
	if !found {
		t.Fatal("created endpoint not in list")
	}

	// Update
	updated := mustDo(t, "PUT", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/api-groups/"+apiGroupID+"/endpoints/"+id, adminToken, M{
		"summary": "List all payment records",
		"tags":    []string{"payments", "read"},
	})
	if updated["summary"] != "List all payment records" {
		t.Fatalf("want updated summary, got %v", updated["summary"])
	}

	// Delete
	r := do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/api-groups/"+apiGroupID+"/endpoints/"+id, adminToken, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", r.StatusCode)
	}

	r = do("GET", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/api-groups/"+apiGroupID+"/endpoints/"+id, adminToken, nil)
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 after delete, got %d", r.StatusCode)
	}
}

func TestAPIEndpoints_MultipleHTTPMethods(t *testing.T) {
	serviceID := createTestService(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+serviceID, adminToken, nil) })
	apiGroupID := createTestAPIGroup(t, serviceID)

	methods := []string{"GET", "POST", "PUT", "DELETE"}
	for i, m := range methods {
		mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/api-groups/"+apiGroupID+"/endpoints", adminToken, M{
			"method":  m,
			"path":    fmt.Sprintf("/resources/%d", i),
			"summary": m + " resource",
			"order":   float64(i),
		})
	}

	body := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/api-groups/"+apiGroupID+"/endpoints", adminToken, nil)
	if n := len(list(body, "endpoints")); n != len(methods) {
		t.Fatalf("want %d endpoints, got %d", len(methods), n)
	}
}
