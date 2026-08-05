package tests

import (
	"net/http"
	"strings"
	"testing"
)

func TestCreateUpload_RandomIDWithoutContentHash(t *testing.T) {
	path := "/api/v1/orgs/" + orgID + "/assets"

	first := mustDo(t, "POST", path, adminToken, M{})
	second := mustDo(t, "POST", path, adminToken, M{})

	firstID := str(first, "assetId")
	if !strings.HasPrefix(firstID, "file_") {
		t.Fatalf("want a file_ asset id, got %q", firstID)
	}
	if firstID == str(second, "assetId") {
		t.Fatal("two upload requests without a contentHash must not share an asset id")
	}
	if str(first, "uploadUrl") == "" {
		t.Fatal("expected an uploadUrl")
	}
}

func TestCreateUpload_DeterministicIDFromContentHash(t *testing.T) {
	path := "/api/v1/orgs/" + orgID + "/assets"
	hash := strings.Repeat("ab", 32)

	first := mustDo(t, "POST", path, adminToken, M{"contentHash": hash})
	second := mustDo(t, "POST", path, adminToken, M{"contentHash": hash})

	want := "file_" + hash
	if str(first, "assetId") != want {
		t.Fatalf("want asset id %q, got %q", want, str(first, "assetId"))
	}
	if str(second, "assetId") != want {
		t.Fatalf("re-uploading identical content must reuse the asset id, got %q", str(second, "assetId"))
	}
}

func TestCreateUpload_RejectsMalformedContentHash(t *testing.T) {
	path := "/api/v1/orgs/" + orgID + "/assets"

	for _, hash := range []string{"not-a-hash", strings.Repeat("A", 64), strings.Repeat("ab", 20)} {
		r := do("POST", path, adminToken, M{"contentHash": hash})
		if r.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400 for contentHash %q, got %d", hash, r.StatusCode)
		}
	}
}
