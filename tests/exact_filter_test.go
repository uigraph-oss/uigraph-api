package tests

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func exactList(t *testing.T, path, key string) []M {
	t.Helper()
	return list(mustDo(t, "GET", path, adminToken, nil), key)
}

func exactOnly(t *testing.T, items []M, field, want string) {
	t.Helper()
	if len(items) != 1 {
		t.Fatalf("want exactly 1 row, got %d", len(items))
	}
	if got := str(items[0], field); got != want {
		t.Fatalf("want %s=%q, got %q", field, want, got)
	}
}

func exactHas(t *testing.T, items []M, field string, want ...string) {
	t.Helper()
	for _, w := range want {
		found := false
		for _, it := range items {
			if str(it, field) == w {
				found = true
			}
		}
		if !found {
			t.Fatalf("want %s=%q in unfiltered list of %d rows", field, w, len(items))
		}
	}
}

func exactNested(items []M, key string) []M {
	out := make([]M, len(items))
	for i, it := range items {
		out[i] = obj(it, key)
	}
	return out
}

func TestServices_ExactName(t *testing.T) {
	ts := time.Now().UnixNano()
	alpha := fmt.Sprintf("alpha-%d", ts)
	beta := fmt.Sprintf("beta-%d", ts)

	a := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/services", adminToken, M{"name": alpha})
	b := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/services", adminToken, M{"name": beta})
	t.Cleanup(func() {
		do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+str(a, "id"), adminToken, nil)
		do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+str(b, "id"), adminToken, nil)
	})

	base := "/api/v1/orgs/" + orgID + "/services"
	exactOnly(t, exactList(t, base+"?exact_name="+alpha, "services"), "name", alpha)
	exactOnly(t, exactList(t, base+"?exact_name="+strings.ToUpper(alpha), "services"), "name", alpha)
	exactHas(t, exactList(t, fmt.Sprintf("%s?limit=100&search=%d", base, ts), "services"), "name", alpha, beta)
}

func TestServices_ExactNamePrefersExactCase(t *testing.T) {
	ts := time.Now().UnixNano()
	lower := fmt.Sprintf("zeta-%d", ts)
	upper := fmt.Sprintf("Zeta-%d", ts)

	a := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/services", adminToken, M{"name": upper})
	b := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/services", adminToken, M{"name": lower})
	t.Cleanup(func() {
		do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+str(a, "id"), adminToken, nil)
		do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+str(b, "id"), adminToken, nil)
	})

	svcs := exactList(t, "/api/v1/orgs/"+orgID+"/services?exact_name="+lower, "services")
	if len(svcs) != 2 {
		t.Fatalf("want 2 case-insensitive matches, got %d", len(svcs))
	}
	if got := str(svcs[0], "name"); got != lower {
		t.Fatalf("want exactly-cased %q first, got %q", lower, got)
	}
}

func TestMaps_ExactName(t *testing.T) {
	ts := time.Now().UnixNano()
	alpha := fmt.Sprintf("alpha-%d", ts)
	beta := fmt.Sprintf("beta-%d", ts)

	a := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/maps", adminToken, M{"name": alpha})
	b := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/maps", adminToken, M{"name": beta})
	t.Cleanup(func() {
		do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+str(a, "id"), adminToken, nil)
		do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+str(b, "id"), adminToken, nil)
	})

	base := "/api/v1/orgs/" + orgID + "/maps"
	exactOnly(t, exactList(t, base+"?exact_name="+alpha, "maps"), "name", alpha)
	exactOnly(t, exactList(t, base+"?exact_name="+strings.ToUpper(alpha), "maps"), "name", alpha)
	exactHas(t, exactList(t, fmt.Sprintf("%s?limit=100&search=%d", base, ts), "maps"), "name", alpha, beta)
}

func TestFrames_ExactName(t *testing.T) {
	mapID := createTestMap(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+mapID, adminToken, nil) })

	ts := time.Now().UnixNano()
	alpha := fmt.Sprintf("alpha-%d", ts)
	beta := fmt.Sprintf("beta-%d", ts)
	mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames", adminToken, M{"name": alpha})
	mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/maps/"+mapID+"/frames", adminToken, M{"name": beta})

	base := "/api/v1/orgs/" + orgID + "/maps/" + mapID + "/frames"
	exactOnly(t, exactList(t, base+"?exact_name="+alpha, "frames"), "name", alpha)
	exactOnly(t, exactList(t, base+"?exact_name="+strings.ToUpper(alpha), "frames"), "name", alpha)
	exactHas(t, exactList(t, base, "frames"), "name", alpha, beta)
}

func TestFocalPoints_ExactName(t *testing.T) {
	mapID := createTestMap(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/maps/"+mapID, adminToken, nil) })
	frameID := createTestFrame(t, mapID)

	ts := time.Now().UnixNano()
	alpha := fmt.Sprintf("alpha-%d", ts)
	beta := fmt.Sprintf("beta-%d", ts)
	base := "/api/v1/orgs/" + orgID + "/maps/" + mapID + "/frames/" + frameID + "/focal-points"
	mustDo(t, "POST", base, adminToken, M{"name": alpha, "locationX": 1.0, "locationY": 2.0})
	mustDo(t, "POST", base, adminToken, M{"name": beta, "locationX": 3.0, "locationY": 4.0})

	exactOnly(t, exactList(t, base+"?exact_name="+alpha, "focalPoints"), "name", alpha)
	exactOnly(t, exactList(t, base+"?exact_name="+strings.ToUpper(alpha), "focalPoints"), "name", alpha)
	exactHas(t, exactList(t, base, "focalPoints"), "name", alpha, beta)
}

func TestAPIGroups_ExactName(t *testing.T) {
	serviceID := createTestService(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+serviceID, adminToken, nil) })

	ts := time.Now().UnixNano()
	alpha := fmt.Sprintf("alpha-%d", ts)
	beta := fmt.Sprintf("beta-%d", ts)
	base := "/api/v1/orgs/" + orgID + "/services/" + serviceID + "/api-groups"
	mustDo(t, "POST", base, adminToken, M{"name": alpha, "protocol": "REST"})
	mustDo(t, "POST", base, adminToken, M{"name": beta, "protocol": "REST"})

	exactOnly(t, exactList(t, base+"?exact_name="+alpha, "apiGroups"), "name", alpha)
	exactOnly(t, exactList(t, base+"?exact_name="+strings.ToUpper(alpha), "apiGroups"), "name", alpha)
	exactHas(t, exactList(t, base, "apiGroups"), "name", alpha, beta)
}

func TestAPIEndpoints_ExactOperationId(t *testing.T) {
	serviceID := createTestService(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+serviceID, adminToken, nil) })
	apiGroupID := createTestAPIGroup(t, serviceID)

	base := "/api/v1/orgs/" + orgID + "/services/" + serviceID + "/api-groups/" + apiGroupID + "/endpoints"
	mustDo(t, "POST", base, adminToken, M{"operationId": "listPayments", "method": "GET", "path": "/payments"})
	mustDo(t, "POST", base, adminToken, M{"operationId": "getPayment", "method": "GET", "path": "/payments/{id}"})

	exactOnly(t, exactList(t, base+"?exact_operation_id=listPayments", "endpoints"), "operationId", "listPayments")
	if n := len(exactList(t, base+"?exact_operation_id=LISTPAYMENTS", "endpoints")); n != 0 {
		t.Fatalf("exact_operation_id is case-sensitive, want 0 rows, got %d", n)
	}
	exactHas(t, exactList(t, base, "endpoints"), "operationId", "listPayments", "getPayment")
}

func TestAPIEndpoints_ExactOperationIdWithVersionId(t *testing.T) {
	serviceID := createTestService(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+serviceID, adminToken, nil) })

	g := mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/api-groups", adminToken, M{
		"name": fmt.Sprintf("api-%d", time.Now().UnixNano()), "protocol": "REST", "spec": updatedSpec,
	})
	apiGroupID := str(g, "id")

	versions := exactList(t, "/api/v1/orgs/"+orgID+"/services/"+serviceID+"/api-groups/"+apiGroupID+"/versions", "versions")
	if len(versions) != 1 {
		t.Fatalf("want 1 auto-version after create, got %d", len(versions))
	}
	versionID := str(versions[0], "id")

	base := "/api/v1/orgs/" + orgID + "/services/" + serviceID + "/api-groups/" + apiGroupID + "/endpoints?versionId=" + versionID
	exactHas(t, exactList(t, base, "endpoints"), "operationId", "listPayments", "getPayment")
	exactOnly(t, exactList(t, base+"&exact_operation_id=getPayment", "endpoints"), "operationId", "getPayment")
	if n := len(exactList(t, base+"&exact_operation_id=GETPAYMENT", "endpoints")); n != 0 {
		t.Fatalf("exact_operation_id is case-sensitive, want 0 rows, got %d", n)
	}
}

func TestServiceDBs_ExactDBName(t *testing.T) {
	serviceID := createTestService(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+serviceID, adminToken, nil) })

	ts := time.Now().UnixNano()
	alpha := fmt.Sprintf("alpha-%d", ts)
	beta := fmt.Sprintf("beta-%d", ts)
	base := "/api/v1/orgs/" + orgID + "/services/" + serviceID + "/dbs"
	mustDo(t, "POST", base, adminToken, M{"dbName": alpha, "dbType": "PostgreSQL", "dialect": "postgresql"})
	mustDo(t, "POST", base, adminToken, M{"dbName": beta, "dbType": "PostgreSQL", "dialect": "postgresql"})

	exactOnly(t, exactList(t, base+"?exact_db_name="+alpha, "dbs"), "dbName", alpha)
	exactOnly(t, exactList(t, base+"?exact_db_name="+strings.ToUpper(alpha), "dbs"), "dbName", alpha)
	exactHas(t, exactList(t, base, "dbs"), "dbName", alpha, beta)
}

func TestTestPacks_ExactName(t *testing.T) {
	serviceID := createTestService(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+serviceID, adminToken, nil) })

	ts := time.Now().UnixNano()
	alpha := fmt.Sprintf("alpha-%d", ts)
	beta := fmt.Sprintf("beta-%d", ts)
	svcBase := "/api/v1/orgs/" + orgID + "/services/" + serviceID
	mustDo(t, "POST", svcBase+"/test-pack", adminToken, M{"name": alpha, "type": "manual"})
	mustDo(t, "POST", svcBase+"/test-pack", adminToken, M{"name": beta, "type": "manual"})

	exactOnly(t, exactList(t, svcBase+"/test-packs?exact_name="+alpha, "testPacks"), "name", alpha)
	exactOnly(t, exactList(t, svcBase+"/test-packs?exact_name="+strings.ToUpper(alpha), "testPacks"), "name", alpha)
	exactHas(t, exactList(t, svcBase+"/test-packs", "testPacks"), "name", alpha, beta)
}

func TestTestCases_ExactTitle(t *testing.T) {
	serviceID := createTestService(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+serviceID, adminToken, nil) })

	ts := time.Now().UnixNano()
	alpha := fmt.Sprintf("alpha-%d", ts)
	beta := fmt.Sprintf("beta-%d", ts)
	svcBase := "/api/v1/orgs/" + orgID + "/services/" + serviceID
	pack := mustDo(t, "POST", svcBase+"/test-pack", adminToken, M{"name": fmt.Sprintf("pack-%d", ts), "type": "manual"})
	packID := str(pack, "id")
	mustDo(t, "POST", svcBase+"/test-case", adminToken, M{"testPackId": packID, "title": alpha, "type": "manual"})
	mustDo(t, "POST", svcBase+"/test-case", adminToken, M{"testPackId": packID, "title": beta, "type": "manual"})

	exactOnly(t, exactList(t, svcBase+"/test-cases?exact_title="+alpha, "testCases"), "title", alpha)
	exactOnly(t, exactList(t, svcBase+"/test-cases?exact_title="+strings.ToUpper(alpha), "testCases"), "title", alpha)
	exactHas(t, exactList(t, svcBase+"/test-cases", "testCases"), "title", alpha, beta)
}

func TestServiceDocs_ExactFileName(t *testing.T) {
	serviceID := createTestService(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+serviceID, adminToken, nil) })

	ts := time.Now().UnixNano()
	alpha := fmt.Sprintf("alpha-%d.md", ts)
	beta := fmt.Sprintf("beta-%d.md", ts)
	base := "/api/v1/orgs/" + orgID + "/services/" + serviceID + "/docs"
	mustDo(t, "POST", base, adminToken, M{"fileName": alpha, "fileType": "text/markdown", "contentBase64": "aGVsbG8="})
	mustDo(t, "POST", base, adminToken, M{"fileName": beta, "fileType": "text/markdown", "contentBase64": "aGVsbG8="})

	exactOnly(t, exactNested(exactList(t, base+"?exact_file_name="+alpha, "docs"), "doc"), "fileName", alpha)
	exactOnly(t, exactNested(exactList(t, base+"?exact_file_name="+strings.ToUpper(alpha), "docs"), "doc"), "fileName", alpha)
	exactHas(t, exactNested(exactList(t, base, "docs"), "doc"), "fileName", alpha, beta)
}

func TestServiceDiagrams_ExactName(t *testing.T) {
	serviceID := createTestService(t)
	t.Cleanup(func() { do("DELETE", "/api/v1/orgs/"+orgID+"/services/"+serviceID, adminToken, nil) })

	ts := time.Now().UnixNano()
	alpha := fmt.Sprintf("alpha-%d", ts)
	beta := fmt.Sprintf("beta-%d", ts)
	base := "/api/v1/orgs/" + orgID + "/services/" + serviceID + "/diagrams"
	mustDo(t, "POST", base, adminToken, M{"name": alpha, "content": "{}"})
	mustDo(t, "POST", base, adminToken, M{"name": beta, "content": "{}"})

	exactOnly(t, exactNested(exactList(t, base+"?exact_name="+alpha, "diagrams"), "diagram"), "name", alpha)
	exactOnly(t, exactNested(exactList(t, base+"?exact_name="+strings.ToUpper(alpha), "diagrams"), "diagram"), "name", alpha)
	exactHas(t, exactNested(exactList(t, base, "diagrams"), "diagram"), "name", alpha, beta)
}
