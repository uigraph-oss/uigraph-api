package tests

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// ── Folders ───────────────────────────────────────────────────────────────────

func TestFolders_CRUD(t *testing.T) {
	name := fmt.Sprintf("services-%d", time.Now().UnixNano())

	// Create
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/folders", adminToken, M{
		"name":  name,
		"type":  "service",
		"order": 1.0,
	})
	id := str(created, "id")
	if id == "" {
		t.Fatal("expected id in create response")
	}
	if created["type"] != "service" {
		t.Fatalf("want type=service, got %v", created["type"])
	}

	// Get
	got := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/folders/"+id, adminToken, nil)
	if got["name"] != name {
		t.Fatalf("want name %q, got %v", name, got["name"])
	}
	if got["type"] != "service" {
		t.Fatalf("want type=service after get, got %v", got["type"])
	}

	// List — must contain the new folder
	body := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/folders", adminToken, nil)
	found := false
	for _, f := range list(body, "folders") {
		if str(f, "id") == id {
			found = true
		}
	}
	if !found {
		t.Fatal("created folder not in list response")
	}

	// Update
	updated := mustDo(t, "PUT", "/api/v1/orgs/"+orgID+"/folders/"+id, adminToken, M{
		"name": name + "-updated",
	})
	if updated["name"] != name+"-updated" {
		t.Fatalf("want updated name, got %v", updated["name"])
	}

	// Delete
	r := do("DELETE", "/api/v1/orgs/"+orgID+"/folders/"+id, adminToken, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204 on delete, got %d", r.StatusCode)
	}

	// Confirm soft-deleted: get returns 404
	r = do("GET", "/api/v1/orgs/"+orgID+"/folders/"+id, adminToken, nil)
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 after delete, got %d", r.StatusCode)
	}

	// Confirm soft-deleted: not in list
	body = mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/folders", adminToken, nil)
	for _, f := range list(body, "folders") {
		if str(f, "id") == id {
			t.Fatal("deleted folder should not appear in list")
		}
	}
}

func TestFolders_TypeFilter(t *testing.T) {
	prefix := fmt.Sprintf("%d", time.Now().UnixNano())

	svcFolder := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/folders", adminToken, M{
		"name": prefix + "-svc", "type": "service",
	})
	diagFolder := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/folders", adminToken, M{
		"name": prefix + "-diag", "type": "diagram",
	})
	mapFolder := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/folders", adminToken, M{
		"name": prefix + "-map", "type": "map",
	})
	svcID := str(svcFolder, "id")
	diagID := str(diagFolder, "id")
	mapID := str(mapFolder, "id")

	check := func(filterType, expectID string, forbidIDs ...string) {
		t.Helper()
		body := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/folders?type="+filterType, adminToken, nil)
		found := false
		for _, f := range list(body, "folders") {
			fid := str(f, "id")
			if fid == expectID {
				found = true
			}
			for _, fid2 := range forbidIDs {
				if fid == fid2 {
					t.Fatalf("folder %s should not appear in type=%s filter", fid, filterType)
				}
			}
		}
		if !found {
			t.Fatalf("folder %s not found when filtering type=%s", expectID, filterType)
		}
	}

	check("service", svcID, diagID, mapID)
	check("diagram", diagID, svcID, mapID)
	check("map", mapID, svcID, diagID)

	// Cleanup
	for _, id := range []string{svcID, diagID, mapID} {
		do("DELETE", "/api/v1/orgs/"+orgID+"/folders/"+id, adminToken, nil)
	}
}

func TestFolders_Nested(t *testing.T) {
	prefix := fmt.Sprintf("%d", time.Now().UnixNano())

	parent := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/folders", adminToken, M{
		"name": prefix + "-parent", "type": "service",
	})
	parentID := str(parent, "id")

	child := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/folders", adminToken, M{
		"name": prefix + "-child", "type": "service", "parentId": parentID,
	})
	childID := str(child, "id")

	// Child must reference the parent
	got := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/folders/"+childID, adminToken, nil)
	if got["parentId"] != parentID {
		t.Fatalf("want parentId=%q, got %v", parentID, got["parentId"])
	}

	do("DELETE", "/api/v1/orgs/"+orgID+"/folders/"+childID, adminToken, nil)
	do("DELETE", "/api/v1/orgs/"+orgID+"/folders/"+parentID, adminToken, nil)
}

// ── Diagrams ──────────────────────────────────────────────────────────────────

const sampleContent = `{"nodes":[{"id":"1","type":"rectangle","data":{"label":"API Gateway"}}],"edges":[]}`
const updatedContent = `{"nodes":[{"id":"1","type":"rectangle","data":{"label":"API Gateway"}},{"id":"2","type":"rectangle","data":{"label":"Auth"}}],"edges":[{"id":"e1","source":"1","target":"2"}]}`

func TestDiagrams_Create(t *testing.T) {
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/diagrams", adminToken, M{
		"name":    "System Architecture",
		"content": sampleContent,
	})
	id := str(created, "id")
	if id == "" {
		t.Fatal("expected id in create response")
	}
	if created["name"] != "System Architecture" {
		t.Fatalf("want name 'System Architecture', got %v", created["name"])
	}
	// Content must NOT appear in the metadata response.
	if _, ok := created["content"]; ok {
		t.Fatal("content should not be in diagram metadata response")
	}

	do("DELETE", "/api/v1/orgs/"+orgID+"/diagrams/"+id, adminToken, nil)
}

func TestDiagrams_GetMetadata(t *testing.T) {
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/diagrams", adminToken, M{
		"name": "Meta Only", "content": sampleContent,
	})
	id := str(created, "id")

	got := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/diagrams/"+id, adminToken, nil)
	if got["id"] != id {
		t.Fatalf("want id %q, got %v", id, got["id"])
	}
	if got["name"] != "Meta Only" {
		t.Fatalf("want name 'Meta Only', got %v", got["name"])
	}
	if _, ok := got["content"]; ok {
		t.Fatal("content must not be present in metadata response")
	}

	do("DELETE", "/api/v1/orgs/"+orgID+"/diagrams/"+id, adminToken, nil)
}

func TestDiagrams_GetContent(t *testing.T) {
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/diagrams", adminToken, M{
		"name": "Content Test", "content": sampleContent,
	})
	id := str(created, "id")

	body := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/diagrams/"+id+"/content", adminToken, nil)
	if body["content"] != sampleContent {
		t.Fatalf("want exact content back, got %v", body["content"])
	}
	if body["diagramId"] != id {
		t.Fatalf("want diagramId=%q in content response, got %v", id, body["diagramId"])
	}

	do("DELETE", "/api/v1/orgs/"+orgID+"/diagrams/"+id, adminToken, nil)
}

func TestDiagrams_List(t *testing.T) {
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/diagrams", adminToken, M{
		"name": fmt.Sprintf("list-test-%d", time.Now().UnixNano()), "content": sampleContent,
	})
	id := str(created, "id")

	body := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/diagrams", adminToken, nil)
	found := false
	for _, d := range list(body, "diagrams") {
		if str(d, "id") == id {
			found = true
			if _, ok := d["content"]; ok {
				t.Fatal("content must not be present in list response")
			}
		}
	}
	if !found {
		t.Fatal("created diagram not in list")
	}

	do("DELETE", "/api/v1/orgs/"+orgID+"/diagrams/"+id, adminToken, nil)
}

func TestDiagrams_Update_NameOnly(t *testing.T) {
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/diagrams", adminToken, M{
		"name": "Original Name", "content": sampleContent,
	})
	id := str(created, "id")

	// Update name without touching content.
	updated := mustDo(t, "PUT", "/api/v1/orgs/"+orgID+"/diagrams/"+id, adminToken, M{
		"name": "Renamed",
	})
	if updated["name"] != "Renamed" {
		t.Fatalf("want name 'Renamed', got %v", updated["name"])
	}

	// Version count must still be 1 (no content change).
	vBody := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/diagrams/"+id+"/versions", adminToken, nil)
	if n := len(list(vBody, "versions")); n != 1 {
		t.Fatalf("want 1 version after name-only update, got %d", n)
	}

	do("DELETE", "/api/v1/orgs/"+orgID+"/diagrams/"+id, adminToken, nil)
}

func TestDiagrams_Update_ContentChanged(t *testing.T) {
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/diagrams", adminToken, M{
		"name": "Update With Content", "content": sampleContent,
	})
	id := str(created, "id")

	// Update with different content.
	mustDo(t, "PUT", "/api/v1/orgs/"+orgID+"/diagrams/"+id, adminToken, M{
		"content": updatedContent,
	})

	// Content must reflect the update.
	cBody := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/diagrams/"+id+"/content", adminToken, nil)
	if cBody["content"] != updatedContent {
		t.Fatalf("want updatedContent, got %v", cBody["content"])
	}

	// A new auto-version should have been created (total = 2).
	vBody := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/diagrams/"+id+"/versions", adminToken, nil)
	versions := list(vBody, "versions")
	if len(versions) != 2 {
		t.Fatalf("want 2 versions after content update, got %d", len(versions))
	}
	// The second version must be auto.
	foundAuto := false
	for _, v := range versions {
		if vn, _ := v["versionNumber"].(float64); vn == 2 {
			if v["isAutoVersion"] != true {
				t.Fatal("version 2 must be an auto-version")
			}
			foundAuto = true
		}
	}
	if !foundAuto {
		t.Fatal("version 2 not found")
	}

	do("DELETE", "/api/v1/orgs/"+orgID+"/diagrams/"+id, adminToken, nil)
}

func TestDiagrams_Update_ContentUnchanged(t *testing.T) {
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/diagrams", adminToken, M{
		"name": "No New Version", "content": sampleContent,
	})
	id := str(created, "id")

	// Update with the identical content.
	mustDo(t, "PUT", "/api/v1/orgs/"+orgID+"/diagrams/"+id, adminToken, M{
		"content": sampleContent,
	})

	// Version count stays 1.
	vBody := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/diagrams/"+id+"/versions", adminToken, nil)
	if n := len(list(vBody, "versions")); n != 1 {
		t.Fatalf("want 1 version when content is unchanged, got %d", n)
	}

	do("DELETE", "/api/v1/orgs/"+orgID+"/diagrams/"+id, adminToken, nil)
}

func TestDiagrams_Delete(t *testing.T) {
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/diagrams", adminToken, M{
		"name": "To Be Deleted", "content": sampleContent,
	})
	id := str(created, "id")

	r := do("DELETE", "/api/v1/orgs/"+orgID+"/diagrams/"+id, adminToken, nil)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", r.StatusCode)
	}

	// Metadata endpoint returns 404.
	r = do("GET", "/api/v1/orgs/"+orgID+"/diagrams/"+id, adminToken, nil)
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 after delete, got %d", r.StatusCode)
	}

	// Content endpoint also returns 404.
	r = do("GET", "/api/v1/orgs/"+orgID+"/diagrams/"+id+"/content", adminToken, nil)
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 on content after delete, got %d", r.StatusCode)
	}
}

// ── Versions ──────────────────────────────────────────────────────────────────

func TestDiagrams_Versions_AutoCreatedOnCreate(t *testing.T) {
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/diagrams", adminToken, M{
		"name": "Auto Version", "content": sampleContent,
	})
	id := str(created, "id")

	vBody := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/diagrams/"+id+"/versions", adminToken, nil)
	versions := list(vBody, "versions")
	if len(versions) != 1 {
		t.Fatalf("want 1 auto-version after create, got %d", len(versions))
	}
	if vn, _ := versions[0]["versionNumber"].(float64); vn != 1 {
		t.Fatalf("want versionNumber=1, got %v", versions[0]["versionNumber"])
	}

	do("DELETE", "/api/v1/orgs/"+orgID+"/diagrams/"+id, adminToken, nil)
}

func TestDiagrams_Versions_ExplicitCreate(t *testing.T) {
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/diagrams", adminToken, M{
		"name": "Explicit Version", "content": sampleContent,
	})
	id := str(created, "id")

	// Create a named explicit version.
	ver := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/diagrams/"+id+"/versions", adminToken, M{
		"label": "v1.0-release",
	})
	if ver["label"] != "v1.0-release" {
		t.Fatalf("want label 'v1.0-release', got %v", ver["label"])
	}
	if ver["isAutoVersion"] != false {
		t.Fatalf("explicit version must have isAutoVersion=false, got %v", ver["isAutoVersion"])
	}
	vn, _ := ver["versionNumber"].(float64)
	if vn != 2 {
		t.Fatalf("want versionNumber=2, got %v", ver["versionNumber"])
	}

	// Two versions total.
	vBody := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/diagrams/"+id+"/versions", adminToken, nil)
	if n := len(list(vBody, "versions")); n != 2 {
		t.Fatalf("want 2 versions, got %d", n)
	}

	do("DELETE", "/api/v1/orgs/"+orgID+"/diagrams/"+id, adminToken, nil)
}

func TestDiagrams_Versions_GetContent(t *testing.T) {
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/diagrams", adminToken, M{
		"name": "Version Content", "content": sampleContent,
	})
	id := str(created, "id")

	// Get version list to find version 1 ID.
	vBody := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/diagrams/"+id+"/versions", adminToken, nil)
	versions := list(vBody, "versions")
	if len(versions) == 0 {
		t.Fatal("expected at least one version")
	}
	versionID := str(versions[0], "id")

	cBody := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/diagrams/"+id+"/versions/"+versionID+"/content", adminToken, nil)
	if cBody["content"] != sampleContent {
		t.Fatalf("want sampleContent in version, got %v", cBody["content"])
	}
	if cBody["versionId"] != versionID {
		t.Fatalf("want versionId=%q, got %v", versionID, cBody["versionId"])
	}

	do("DELETE", "/api/v1/orgs/"+orgID+"/diagrams/"+id, adminToken, nil)
}

func TestDiagrams_Versions_Restore(t *testing.T) {
	created := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/diagrams", adminToken, M{
		"name": "Restore Test", "content": sampleContent,
	})
	id := str(created, "id")

	// Update content to v2.
	mustDo(t, "PUT", "/api/v1/orgs/"+orgID+"/diagrams/"+id, adminToken, M{
		"content": updatedContent,
	})

	// Find version 1.
	vBody := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/diagrams/"+id+"/versions", adminToken, nil)
	var v1ID string
	for _, v := range list(vBody, "versions") {
		if vn, _ := v["versionNumber"].(float64); vn == 1 {
			v1ID = str(v, "id")
		}
	}
	if v1ID == "" {
		t.Fatal("version 1 not found")
	}

	// Restore version 1.
	mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/diagrams/"+id+"/versions/"+v1ID+"/restore", adminToken, nil)

	// Current content must now match sampleContent again.
	cBody := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/diagrams/"+id+"/content", adminToken, nil)
	if cBody["content"] != sampleContent {
		t.Fatalf("after restore, want sampleContent, got %v", cBody["content"])
	}

	// A new auto-version must have been created (now 3 total: v1 auto, v2 auto, v3 restore).
	vBody = mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/diagrams/"+id+"/versions", adminToken, nil)
	if n := len(list(vBody, "versions")); n != 3 {
		t.Fatalf("want 3 versions after restore, got %d", n)
	}

	do("DELETE", "/api/v1/orgs/"+orgID+"/diagrams/"+id, adminToken, nil)
}

