package catalog

import (
	"encoding/json"
	"testing"
)

func TestNormalizeStoredJSON_unwrapsDoubleEncodedString(t *testing.T) {
	in := json.RawMessage(`"{\"query\":\"mutation { trackEvent { id } }\"}"`)
	out := normalizeStoredJSON(in)
	want := `{"query":"mutation { trackEvent { id } }"}`
	if string(out) != want {
		t.Fatalf("got %s, want %s", out, want)
	}
}

func TestNormalizeStoredJSON_passesThroughObject(t *testing.T) {
	in := json.RawMessage(`{"query":"hello"}`)
	out := normalizeStoredJSON(in)
	if string(out) != string(in) {
		t.Fatalf("object should pass through unchanged, got %s", out)
	}
}
