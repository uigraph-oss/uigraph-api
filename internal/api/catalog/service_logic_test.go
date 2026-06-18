package catalog

import (
	"strings"
	"testing"
)

// ── toSlug ────────────────────────────────────────────────────────────────────

func TestToSlug_lowercasesInput(t *testing.T) {
	if got := toSlug("Hello World"); got != "hello-world" {
		t.Fatalf("expected hello-world, got %q", got)
	}
}

func TestToSlug_replacesSpacesWithHyphens(t *testing.T) {
	if got := toSlug("my service name"); got != "my-service-name" {
		t.Fatalf("expected my-service-name, got %q", got)
	}
}

func TestToSlug_replacesSpecialChars(t *testing.T) {
	if got := toSlug("order-service@v2"); got != "order-service-v2" {
		t.Fatalf("expected order-service-v2, got %q", got)
	}
}

func TestToSlug_preservesDotsAndHyphens(t *testing.T) {
	if got := toSlug("api.v1-service"); got != "api.v1-service" {
		t.Fatalf("expected api.v1-service, got %q", got)
	}
}

func TestToSlug_preservesNumbers(t *testing.T) {
	if got := toSlug("service123"); got != "service123" {
		t.Fatalf("expected service123, got %q", got)
	}
}

func TestToSlug_collapsesConsecutiveHyphens(t *testing.T) {
	if got := toSlug("foo  bar"); got != "foo-bar" {
		t.Fatalf("expected foo-bar (collapsed hyphens), got %q", got)
	}
}

func TestToSlug_trimsLeadingAndTrailingHyphens(t *testing.T) {
	if got := toSlug("@service@"); got != "service" {
		t.Fatalf("expected service (trimmed), got %q", got)
	}
}

func TestToSlug_emptyStringIsEmpty(t *testing.T) {
	if got := toSlug(""); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

// ── specHash / sha256Bytes ────────────────────────────────────────────────────

func TestSpecHash_isDeterministic(t *testing.T) {
	input := `{"openapi":"3.0.0"}`
	h1 := specHash(input)
	h2 := specHash(input)
	if h1 != h2 {
		t.Fatalf("specHash is not deterministic: %q vs %q", h1, h2)
	}
}

func TestSpecHash_isHexString(t *testing.T) {
	h := specHash("anything")
	const hexChars = "0123456789abcdef"
	for _, c := range h {
		if !strings.ContainsRune(hexChars, c) {
			t.Fatalf("specHash contains non-hex character %q in %q", c, h)
		}
	}
	// SHA-256 produces 64 hex characters
	if len(h) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(h))
	}
}

func TestSpecHash_differentInputsProduceDifferentHashes(t *testing.T) {
	h1 := specHash("version: 1")
	h2 := specHash("version: 2")
	if h1 == h2 {
		t.Fatal("different inputs must not produce the same hash")
	}
}

func TestSha256Bytes_matchesSpecHashForSameContent(t *testing.T) {
	content := "hello world"
	fromString := specHash(content)
	fromBytes := sha256Bytes([]byte(content))
	if fromString != fromBytes {
		t.Fatalf("specHash and sha256Bytes disagree: %q vs %q", fromString, fromBytes)
	}
}

func TestSha256Bytes_emptyInput(t *testing.T) {
	h := sha256Bytes([]byte{})
	if len(h) != 64 {
		t.Fatalf("expected 64 hex chars for empty input, got %d", len(h))
	}
}
