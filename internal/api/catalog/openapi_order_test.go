package catalog

import (
	"testing"
	"time"
)

func TestParseOpenAPIDocumentOrder_preservesPathAndMethodOrder(t *testing.T) {
	spec := `{
  "openapi": "3.0.0",
  "paths": {
    "/checkout": {
      "post": { "operationId": "checkout" }
    },
    "/addresses": {
      "get": { "operationId": "listAddresses" },
      "post": { "operationId": "createAddress" }
    },
    "/cart": {
      "get": { "operationId": "getCart" }
    }
  }
}`

	paths, methodsByPath, err := parseOpenAPIDocumentOrder(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantPaths := []string{"/checkout", "/addresses", "/cart"}
	if len(paths) != len(wantPaths) {
		t.Fatalf("paths len = %d, want %d", len(paths), len(wantPaths))
	}
	for i, want := range wantPaths {
		if paths[i] != want {
			t.Fatalf("paths[%d] = %q, want %q", i, paths[i], want)
		}
	}

	wantMethods := map[string][]string{
		"/checkout":  {"post"},
		"/addresses": {"get", "post"},
		"/cart":      {"get"},
	}
	for path, want := range wantMethods {
		got := methodsByPath[path]
		if len(got) != len(want) {
			t.Fatalf("methods for %s = %v, want %v", path, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("methods for %s = %v, want %v", path, got, want)
			}
		}
	}
}

func TestParseSpecEndpoints_preservesSpecOrder(t *testing.T) {
	spec := `{
  "openapi": "3.0.0",
  "paths": {
    "/checkout": {
      "post": { "operationId": "checkout" }
    },
    "/addresses": {
      "get": { "operationId": "listAddresses" },
      "post": { "operationId": "createAddress" }
    },
    "/cart": {
      "get": { "operationId": "getCart" }
    }
  }
}`

	endpoints, err := parseSpecEndpoints(spec, "group-1", "svc-1", "org-1", "user-1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(endpoints) != 4 {
		t.Fatalf("expected 4 endpoints, got %d", len(endpoints))
	}

	want := []struct {
		method string
		path   string
		order  float64
	}{
		{"POST", "/checkout", 0},
		{"GET", "/addresses", 1},
		{"POST", "/addresses", 2},
		{"GET", "/cart", 3},
	}

	for i, w := range want {
		got := endpoints[i]
		if got.Method != w.method || got.Path != w.path || got.Order != w.order {
			t.Fatalf("endpoints[%d] = {%s %s order=%v}, want {%s %s order=%v}",
				i, got.Method, got.Path, got.Order, w.method, w.path, w.order)
		}
	}
}

func TestParseSpecEndpoints_specOrderIsStableAcrossRuns(t *testing.T) {
	spec := `{
  "openapi": "3.0.0",
  "paths": {
    "/z-last": { "get": {} },
    "/a-first": { "get": {} },
    "/m-middle": { "get": {} }
  }
}`

	first, err := parseSpecEndpoints(spec, "g", "s", "o", "u", time.Now())
	if err != nil {
		t.Fatal(err)
	}

	for run := 0; run < 20; run++ {
		next, err := parseSpecEndpoints(spec, "g", "s", "o", "u", time.Now())
		if err != nil {
			t.Fatal(err)
		}
		for i := range first {
			if first[i].Path != next[i].Path {
				t.Fatalf("run %d path order changed: got %q want %q", run, next[i].Path, first[i].Path)
			}
		}
	}

	wantPaths := []string{"/z-last", "/a-first", "/m-middle"}
	for i, path := range wantPaths {
		if first[i].Path != path {
			t.Fatalf("endpoints[%d].Path = %q, want %q", i, first[i].Path, path)
		}
	}
}
