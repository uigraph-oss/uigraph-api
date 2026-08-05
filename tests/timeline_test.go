package tests

import (
	"net/http"
	"testing"
	"time"
)

func TestTimelineEvents_CRUD(t *testing.T) {
	serviceID := createTestService(t)
	base := "/api/v1/orgs/" + orgID + "/services/" + serviceID + "/timeline"

	// create a release event
	created := mustDo(t, "POST", base, adminToken, M{
		"type":      "release",
		"title":     "Add phone-based MFA",
		"summary":   "Adds a new /auth/mfa endpoint.",
		"eventDate": time.Now().UTC().Format(time.RFC3339),
		"version":   "v2.4.0",
		"touches": []M{
			{"id": "user-service", "label": "User Service", "kind": "service"},
		},
	})
	eventID := str(created, "id")
	if eventID == "" {
		t.Fatal("expected id in response")
	}
	if created["origin"] != "manual" {
		t.Fatalf("want origin=manual, got %v", created["origin"])
	}
	if created["version"] != "v2.4.0" {
		t.Fatalf("want version v2.4.0, got %v", created["version"])
	}

	// list — should contain the new event
	body := mustDo(t, "GET", base, adminToken, nil)
	found := false
	for _, e := range list(body, "events") {
		if str(e, "id") == eventID {
			found = true
		}
	}
	if !found {
		t.Fatal("new event not found in list")
	}

	// update
	updated := mustDo(t, "PUT", base+"/"+eventID, adminToken, M{
		"type":      "release",
		"title":     "Add phone-based MFA to login flow",
		"summary":   "Updated summary.",
		"eventDate": time.Now().UTC().Format(time.RFC3339),
		"version":   "v2.4.1",
	})
	if updated["title"] != "Add phone-based MFA to login flow" {
		t.Fatalf("want updated title, got %v", updated["title"])
	}
	if updated["version"] != "v2.4.1" {
		t.Fatalf("want updated version, got %v", updated["version"])
	}
	if updated["origin"] != "manual" {
		t.Fatalf("origin should be preserved across update, got %v", updated["origin"])
	}

	// delete
	r := do("DELETE", base+"/"+eventID, adminToken, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", r.StatusCode)
	}

	// confirm gone
	body = mustDo(t, "GET", base, adminToken, nil)
	for _, e := range list(body, "events") {
		if str(e, "id") == eventID {
			t.Fatal("deleted event still present in list")
		}
	}
}

func TestTimelineEvents_DecisionAndIncident(t *testing.T) {
	serviceID := createTestService(t)
	base := "/api/v1/orgs/" + orgID + "/services/" + serviceID + "/timeline"

	decision := mustDo(t, "POST", base, adminToken, M{
		"type":           "decision",
		"title":          "Use event sourcing for order state transitions",
		"summary":        "Order status changes become an append-only log.",
		"eventDate":      time.Now().UTC().Format(time.RFC3339),
		"adrNumber":      "ADR-021",
		"decisionStatus": "accepted",
	})
	if decision["adrNumber"] != "ADR-021" {
		t.Fatalf("want adrNumber ADR-021, got %v", decision["adrNumber"])
	}
	if decision["decisionStatus"] != "accepted" {
		t.Fatalf("want decisionStatus accepted, got %v", decision["decisionStatus"])
	}

	incident := mustDo(t, "POST", base, adminToken, M{
		"type":      "incident",
		"title":     "Payment gateway retried failed charges twice",
		"summary":   "Duplicate charges due to a retry bug.",
		"eventDate": time.Now().UTC().Format(time.RFC3339),
	})
	if str(incident, "id") == "" {
		t.Fatal("expected id in response")
	}
}

func TestTimelineEvents_SyncBySourceRef(t *testing.T) {
	serviceID := createTestService(t)
	base := "/api/v1/orgs/" + orgID + "/services/" + serviceID + "/timeline"
	eventDate := time.Now().UTC().Format(time.RFC3339)

	manual := mustDo(t, "POST", base, adminToken, M{
		"type":      "incident",
		"title":     "Hand-written incident",
		"eventDate": eventDate,
	})
	manualID := str(manual, "id")

	first := mustDo(t, "POST", base+"/sync", adminToken, M{
		"sourceRef":      "adr:docs/adr/0007-use-postgres.md",
		"type":           "decision",
		"title":          "Use Postgres",
		"summary":        "Chose Postgres over DynamoDB.",
		"eventDate":      eventDate,
		"adrNumber":      "0007",
		"decisionStatus": "proposed",
		"commitHash":     "abc123",
	})
	if first["created"] != true {
		t.Fatalf("want created=true on first sync, got %v", first["created"])
	}
	firstEvent := obj(first, "event")
	syncedID := str(firstEvent, "id")
	if firstEvent["origin"] != "auto" {
		t.Fatalf("want origin=auto, got %v", firstEvent["origin"])
	}
	if firstEvent["sourceRef"] != "adr:docs/adr/0007-use-postgres.md" {
		t.Fatalf("want sourceRef echoed back, got %v", firstEvent["sourceRef"])
	}

	// same sourceRef, changed title: updates in place rather than duplicating
	second := mustDo(t, "POST", base+"/sync", adminToken, M{
		"sourceRef":      "adr:docs/adr/0007-use-postgres.md",
		"type":           "decision",
		"title":          "Use Postgres for the primary store",
		"eventDate":      eventDate,
		"decisionStatus": "accepted",
	})
	if second["created"] != false {
		t.Fatalf("want created=false on re-sync, got %v", second["created"])
	}
	secondEvent := obj(second, "event")
	if str(secondEvent, "id") != syncedID {
		t.Fatalf("re-sync should reuse the same row: %v vs %v", str(secondEvent, "id"), syncedID)
	}
	if secondEvent["title"] != "Use Postgres for the primary store" {
		t.Fatalf("want updated title, got %v", secondEvent["title"])
	}
	if secondEvent["decisionStatus"] != "accepted" {
		t.Fatalf("want updated decisionStatus, got %v", secondEvent["decisionStatus"])
	}

	// a different sourceRef is a different event
	third := mustDo(t, "POST", base+"/sync", adminToken, M{
		"sourceRef": "incident:docs/postmortems/2026-01-04-checkout.md",
		"type":      "incident",
		"title":     "Checkout outage",
		"eventDate": eventDate,
	})
	if third["created"] != true {
		t.Fatalf("want created=true for a new sourceRef, got %v", third["created"])
	}

	events := list(mustDo(t, "GET", base, adminToken, nil), "events")
	if len(events) != 3 {
		t.Fatalf("want 3 events (1 manual + 2 synced), got %d", len(events))
	}
	for _, e := range events {
		if str(e, "id") == manualID && e["origin"] != "manual" {
			t.Fatalf("sync must not touch the manual event, origin is now %v", e["origin"])
		}
	}
}

func TestTimelineEvents_SyncRequiresSourceRef(t *testing.T) {
	serviceID := createTestService(t)
	base := "/api/v1/orgs/" + orgID + "/services/" + serviceID + "/timeline"

	r := do("POST", base+"/sync", adminToken, M{
		"type":      "decision",
		"title":     "No source ref",
		"eventDate": time.Now().UTC().Format(time.RFC3339),
	})
	if r.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 for missing sourceRef, got %d", r.StatusCode)
	}
}

func TestTimelineEvents_RequiresTitle(t *testing.T) {
	serviceID := createTestService(t)
	base := "/api/v1/orgs/" + orgID + "/services/" + serviceID + "/timeline"

	r := do("POST", base, adminToken, M{
		"type":      "incident",
		"summary":   "missing a title",
		"eventDate": time.Now().UTC().Format(time.RFC3339),
	})
	if r.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 for missing title, got %d", r.StatusCode)
	}
}

func TestTimelineEvents_UpdateMissing_404(t *testing.T) {
	serviceID := createTestService(t)
	base := "/api/v1/orgs/" + orgID + "/services/" + serviceID + "/timeline"

	r := do("PUT", base+"/00000000-0000-0000-0000-000000000000", adminToken, M{
		"type":      "incident",
		"title":     "does not exist",
		"summary":   "x",
		"eventDate": time.Now().UTC().Format(time.RFC3339),
	})
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", r.StatusCode)
	}
}
