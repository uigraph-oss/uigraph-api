package catalog

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNormalizeRequestBodyForStorage_generatesExample(t *testing.T) {
	in := map[string]interface{}{
		"required": true,
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}

	out := normalizeRequestBodyForStorage(in)
	example, ok := out.(map[string]interface{})
	if !ok {
		t.Fatalf("expected example map, got %T", out)
	}
	if example["id"] != "string" {
		t.Fatalf("expected id example string, got %v", example["id"])
	}
	if _, hasContent := example["content"]; hasContent {
		t.Fatal("expected content wrapper to be stripped")
	}
	if _, hasType := example["type"]; hasType {
		t.Fatal("expected example payload, not JSON Schema")
	}
}

func TestNormalizeResponsesForStorage_generatesExample(t *testing.T) {
	in := map[string]interface{}{
		"201": map[string]interface{}{
			"description": "Order created",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id": map[string]interface{}{"type": "string"},
						},
					},
				},
			},
		},
	}

	out := normalizeResponsesForStorage(in)
	example, ok := out.(map[string]interface{})
	if !ok {
		t.Fatalf("expected example map for single status, got %T", out)
	}
	if example["id"] != "string" {
		t.Fatalf("expected id example string, got %v", example["id"])
	}
	if _, hasSchema := example["schema"]; hasSchema {
		t.Fatal("expected example payload, not schema wrapper")
	}
}

func TestNormalizeResponsesForStorage_multipleStatuses(t *testing.T) {
	in := map[string]interface{}{
		"201": map[string]interface{}{
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id": map[string]interface{}{"type": "string"},
						},
					},
				},
			},
		},
		"400": map[string]interface{}{
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"error": map[string]interface{}{"type": "string"},
						},
					},
				},
			},
		},
	}

	out := normalizeResponsesForStorage(in)
	byStatus, ok := out.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map keyed by status, got %T", out)
	}
	if len(byStatus) != 2 {
		t.Fatalf("expected 2 status examples, got %d", len(byStatus))
	}
}

func TestParseSpecEndpoints_storesExamplePayloads(t *testing.T) {
	endpoints, err := parseSpecEndpoints(testOpenAPISpecWithRef, "group-1", "svc-1", "org-1", "user-1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}

	e := endpoints[0]

	var reqExample map[string]interface{}
	if err := json.Unmarshal(e.RequestBody, &reqExample); err != nil {
		t.Fatalf("RequestBody is not valid JSON: %v", err)
	}
	if _, hasRef := reqExample["$ref"]; hasRef {
		t.Fatal("expected requestBody $ref to be resolved into example")
	}
	if _, hasType := reqExample["type"]; hasType {
		t.Fatal("expected requestBody to be an example payload, not JSON Schema")
	}
	if reqExample["id"] != "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("expected uuid example for id, got %v", reqExample["id"])
	}

	var respExample map[string]interface{}
	if err := json.Unmarshal(e.Responses, &respExample); err != nil {
		t.Fatalf("Responses is not valid JSON: %v", err)
	}
	if _, hasSchema := respExample["schema"]; hasSchema {
		t.Fatal("expected response to be an example payload, not schema wrapper")
	}
	if respExample["id"] != "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("expected uuid example for id, got %v", respExample["id"])
	}
	if _, ok := respExample["total_amount"]; !ok {
		t.Fatal("expected resolved response example to include total_amount")
	}
}
