package tests

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// ── Maps ──────────────────────────────────────────────────────────────────────

func TestMaps_CRUD(t *testing.T) {
	name := fmt.Sprintf("map-%d", time.Now().UnixNano())

	// Create
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/maps", adminToken, M{
		"name": name, "description": "test map",
	})
	id := str(created, "id")
	if id == "" {
		t.Fatal("expected id in create response")
	}
	if created["name"] != name {
		t.Fatalf("want name %q, got %v", name, created["name"])
	}
	if created["status"] != "active" {
		t.Fatalf("want status=active, got %v", created["status"])
	}

	// Get
	got := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/maps/"+id, adminToken, nil)
	if got["description"] != "test map" {
		t.Fatalf("want description 'test map', got %v", got["description"])
	}

	// List — must contain the new map
	body := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/maps", adminToken, nil)
	found := false
	for _, m := range list(body, "maps") {
		if str(m, "id") == id {
			found = true
		}
	}
	if !found {
		t.Fatal("created map not in list")
	}

	// Update
	updated := mustDo(t, "PUT", "/api/v1/orgs/"+orgID+"/maps/"+id, adminToken, M{
		"name": name + "-updated", "status": "archived",
	})
	if updated["name"] != name+"-updated" {
		t.Fatalf("want updated name, got %v", updated["name"])
	}
	if updated["status"] != "archived" {
		t.Fatalf("want status=archived, got %v", updated["status"])
	}

	// Delete
	r := do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+id, adminToken, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204 on delete, got %d", r.StatusCode)
	}

	// Confirm 404 after delete
	r = do("GET", "/api/v1/orgs/"+orgID+"/maps/"+id, adminToken, nil)
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 after delete, got %d", r.StatusCode)
	}

	// Confirm not in list
	body = mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/maps", adminToken, nil)
	for _, m := range list(body, "maps") {
		if str(m, "id") == id {
			t.Fatal("deleted map must not appear in list")
		}
	}
}

func TestMaps_FilterByFolder(t *testing.T) {
	// Create a folder to attach the map to
	folder := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/folders", adminToken, M{
		"name": fmt.Sprintf("mapfolder-%d", time.Now().UnixNano()), "type": "map",
	})
	folderID := str(folder, "id")

	m1 := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/maps", adminToken, M{
		"name": fmt.Sprintf("in-folder-%d", time.Now().UnixNano()), "folderId": folderID,
	})
	m2 := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/maps", adminToken, M{
		"name": fmt.Sprintf("no-folder-%d", time.Now().UnixNano()),
	})
	id1 := str(m1, "id")
	id2 := str(m2, "id")

	body := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/maps?folderId="+folderID, adminToken, nil)
	found1, found2 := false, false
	for _, m := range list(body, "maps") {
		if str(m, "id") == id1 {
			found1 = true
		}
		if str(m, "id") == id2 {
			found2 = true
		}
	}
	if !found1 {
		t.Fatal("map with folderId must appear in filtered list")
	}
	if found2 {
		t.Fatal("map without folderId must not appear in filtered list")
	}

	do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+id1, adminToken, nil)
	do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+id2, adminToken, nil)
	do("DELETE", "/api/v1/orgs/"+orgID+"/folders/"+folderID, adminToken, nil)
}

// ── Frames ────────────────────────────────────────────────────────────────────

func createTestMap(t *testing.T) string {
	t.Helper()
	m := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/maps", adminToken, M{
		"name": fmt.Sprintf("testmap-%d", time.Now().UnixNano()),
	})
	return str(m, "id")
}

func TestFrames_CRUD(t *testing.T) {
	mapID := createTestMap(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+mapID, adminToken, nil) })

	// Create
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames", adminToken, M{
		"name": "Login Screen", "templateType": "mobile", "order": 1.0,
	})
	id := str(created, "id")
	if id == "" {
		t.Fatal("expected id in create response")
	}
	if created["name"] != "Login Screen" {
		t.Fatalf("want name 'Login Screen', got %v", created["name"])
	}
	if created["mapId"] != mapID {
		t.Fatalf("want mapId=%q, got %v", mapID, created["mapId"])
	}

	// Get
	got := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/"+id, adminToken, nil)
	if got["templateType"] != "mobile" {
		t.Fatalf("want templateType=mobile, got %v", got["templateType"])
	}

	// List
	body := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames", adminToken, nil)
	found := false
	for _, f := range list(body, "frames") {
		if str(f, "id") == id {
			found = true
		}
	}
	if !found {
		t.Fatal("created frame not in list")
	}

	// Update
	updated := mustDo(t, "PUT", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/"+id, adminToken, M{
		"name": "Login Screen v2",
	})
	if updated["name"] != "Login Screen v2" {
		t.Fatalf("want updated name, got %v", updated["name"])
	}

	// Delete
	r := do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/"+id, adminToken, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", r.StatusCode)
	}

	r = do("GET", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/"+id, adminToken, nil)
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 after delete, got %d", r.StatusCode)
	}
}

func TestFrames_WithScreenshot(t *testing.T) {
	mapID := createTestMap(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+mapID, adminToken, nil) })

	screenshot := `<svg xmlns="http://www.w3.org/2000/svg"><rect width="100" height="100"/></svg>`

	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames", adminToken, M{
		"name":       "With Screenshot",
		"screenshot": screenshot,
	})
	id := str(created, "id")
	if str(created, "screenshotKey") == "" {
		t.Fatal("expected screenshotKey in response when screenshot provided")
	}
	if str(created, "screenshotContentHash") == "" {
		t.Fatal("expected screenshotContentHash in response")
	}

	do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/"+id, adminToken, nil)
}

func TestFrames_Nested(t *testing.T) {
	mapID := createTestMap(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+mapID, adminToken, nil) })

	parent := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames", adminToken, M{
		"name": "Parent Screen",
	})
	parentID := str(parent, "id")

	child := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames", adminToken, M{
		"name": "Child Screen", "parentFrameId": parentID,
	})
	childID := str(child, "id")
	if child["parentFrameId"] != parentID {
		t.Fatalf("want parentFrameId=%q, got %v", parentID, child["parentFrameId"])
	}

	do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/"+childID, adminToken, nil)
	do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/"+parentID, adminToken, nil)
}

// ── Frame Sync ────────────────────────────────────────────────────────────────

func TestFrames_Sync_Create(t *testing.T) {
	mapID := createTestMap(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+mapID, adminToken, nil) })

	screenshot := `<svg>v1</svg>`
	body := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/sync", adminToken, M{
		"name":         "Sync Frame",
		"templateType": "web",
		"screenshot":   screenshot,
		"source":       "cli",
	})
	id := str(body, "frameId")
	if id == "" {
		t.Fatal("expected frameId in sync create response")
	}
	if body["screenshotSaved"] != true {
		t.Fatalf("want screenshotSaved=true, got %v", body["screenshotSaved"])
	}

	got := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/"+id, adminToken, nil)
	if got["name"] != "Sync Frame" {
		t.Fatalf("want name 'Sync Frame', got %v", got["name"])
	}
	if got["source"] != "cli" {
		t.Fatalf("want source=cli, got %v", got["source"])
	}

	do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/"+id, adminToken, nil)
}

func TestFrames_Sync_Update_ScreenshotChanged(t *testing.T) {
	mapID := createTestMap(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+mapID, adminToken, nil) })

	// Create via sync
	initial := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/sync", adminToken, M{
		"name": "Sync Update Frame", "screenshot": "<svg>v1</svg>",
	})
	id := str(initial, "frameId")

	// Sync with different screenshot
	body := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/sync", adminToken, M{
		"frameId":    id,
		"name":       "Sync Update Frame",
		"screenshot": "<svg>v2</svg>",
	})
	if body["screenshotSaved"] != true {
		t.Fatalf("want screenshotSaved=true when screenshot changed, got %v", body["screenshotSaved"])
	}

	do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/"+id, adminToken, nil)
}

func TestFrames_Sync_Update_ScreenshotUnchanged(t *testing.T) {
	mapID := createTestMap(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+mapID, adminToken, nil) })

	screenshot := "<svg>same</svg>"

	// Create via sync
	initial := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/sync", adminToken, M{
		"name": "Same Screenshot Frame", "screenshot": screenshot,
	})
	id := str(initial, "frameId")

	// Sync with identical screenshot
	body := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/sync", adminToken, M{
		"frameId":    id,
		"name":       "Same Screenshot Frame",
		"screenshot": screenshot,
	})
	if body["screenshotSaved"] != false {
		t.Fatalf("want screenshotSaved=false when screenshot unchanged, got %v", body["screenshotSaved"])
	}

	do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/"+id, adminToken, nil)
}

// ── Focal Points ──────────────────────────────────────────────────────────────

func createTestFrame(t *testing.T, mapID string) string {
	t.Helper()
	f := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames", adminToken, M{
		"name": fmt.Sprintf("frame-%d", time.Now().UnixNano()),
	})
	return str(f, "id")
}

func TestFocalPoints_CRUD(t *testing.T) {
	mapID := createTestMap(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+mapID, adminToken, nil) })
	frameID := createTestFrame(t, mapID)

	// Create
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/"+frameID+"/focal-points", adminToken, M{
		"name":       "Add to Cart Button",
		"locationX":  150.5,
		"locationY":  300.0,
		"visibility": "public",
		"isActive":   true,
	})
	id := str(created, "id")
	if id == "" {
		t.Fatal("expected id in create response")
	}
	if created["name"] != "Add to Cart Button" {
		t.Fatalf("want name, got %v", created["name"])
	}

	// Get
	got := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/"+frameID+"/focal-points/"+id, adminToken, nil)
	if got["locationX"] != 150.5 {
		t.Fatalf("want locationX=150.5, got %v", got["locationX"])
	}

	// List
	body := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/"+frameID+"/focal-points", adminToken, nil)
	found := false
	for _, fp := range list(body, "focalPoints") {
		if str(fp, "id") == id {
			found = true
		}
	}
	if !found {
		t.Fatal("created focal point not in list")
	}

	// Update
	updated := mustDo(t, "PUT", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/"+frameID+"/focal-points/"+id, adminToken, M{
		"name":      "Buy Now Button",
		"locationX": 200.0,
		"isActive":  false,
	})
	if updated["name"] != "Buy Now Button" {
		t.Fatalf("want updated name, got %v", updated["name"])
	}
	if updated["isActive"] != false {
		t.Fatalf("want isActive=false, got %v", updated["isActive"])
	}

	// Delete
	r := do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/"+frameID+"/focal-points/"+id, adminToken, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", r.StatusCode)
	}

	r = do("GET", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames/"+frameID+"/focal-points/"+id, adminToken, nil)
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 after delete, got %d", r.StatusCode)
	}
}

// ── Canvas ────────────────────────────────────────────────────────────────────

func TestCanvas_DefaultEmpty(t *testing.T) {
	mapID := createTestMap(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+mapID, adminToken, nil) })

	body := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/canvas", adminToken, nil)
	if body["zoom"] != 1.0 {
		t.Fatalf("want default zoom=1.0, got %v", body["zoom"])
	}
}

func TestCanvas_UpsertAndGet(t *testing.T) {
	mapID := createTestMap(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+mapID, adminToken, nil) })

	frameID := createTestFrame(t, mapID)

	// Upsert canvas state
	mustDo(t, "PUT", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/canvas", adminToken, M{
		"zoom":        1.5,
		"navigationX": -200.0,
		"navigationY": 50.0,
		"framePositions": M{
			frameID: M{"x": 100.0, "y": 200.0},
		},
	})

	// Get and verify
	body := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/canvas", adminToken, nil)
	if body["zoom"] != 1.5 {
		t.Fatalf("want zoom=1.5, got %v", body["zoom"])
	}
	if body["navigationX"] != -200.0 {
		t.Fatalf("want navigationX=-200, got %v", body["navigationX"])
	}

	positions, _ := body["framePositions"].(map[string]any)
	pos, _ := positions[frameID].(map[string]any)
	if pos["x"] != 100.0 {
		t.Fatalf("want frame position x=100, got %v", pos["x"])
	}

	// Upsert again — only zoom changes
	mustDo(t, "PUT", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/canvas", adminToken, M{
		"zoom": 2.0,
	})
	body = mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/canvas", adminToken, nil)
	if body["zoom"] != 2.0 {
		t.Fatalf("want zoom=2.0 after second upsert, got %v", body["zoom"])
	}
}
