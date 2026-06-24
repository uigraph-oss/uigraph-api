// Package tests — see api_test.go for shared TestMain setup, adminToken, orgID.
package tests

import "testing"

func TestMCPSavings_Summary_Blended(t *testing.T) {
	before := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/mcp/savings/summary?period=1y", adminToken, nil)
	beforeCalls := int(before["totalCalls"].(float64))
	beforeSaved := int(before["totalTokensSaved"].(float64))

	mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/mcp/usage", adminToken, M{
		"toolName": "get_service_context", "resourceIds": []string{"svc-1"},
		"modelId": "claude-sonnet-4-6", "tokensServed": 100, "tokensRawEquivalent": 600,
		"tokensSaved": 500, "responseSizeBytes": 2048,
	})

	after := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/mcp/savings/summary?period=1y", adminToken, nil)
	afterCalls := int(after["totalCalls"].(float64))
	afterSaved := int(after["totalTokensSaved"].(float64))

	if afterCalls != beforeCalls+1 {
		t.Fatalf("want totalCalls to increase by 1, before=%d after=%d", beforeCalls, afterCalls)
	}
	if afterSaved != beforeSaved+500 {
		t.Fatalf("want totalTokensSaved to increase by 500, before=%d after=%d", beforeSaved, afterSaved)
	}
	if after["modelId"] != "" {
		t.Fatalf("want blended summary to report empty modelId, got %v", after["modelId"])
	}
}

func TestMCPSavings_Summary_FilteredByModel(t *testing.T) {
	const other = "claude-haiku-4-5"
	before := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/mcp/savings/summary?period=1y&model_id="+other, adminToken, nil)
	beforeCalls := int(before["totalCalls"].(float64))

	// An event for a DIFFERENT model must not affect the filtered summary.
	mustDo(t, "POST", "/api/v1/orgs/"+orgID+"/mcp/usage", adminToken, M{
		"toolName": "get_api_spec", "resourceIds": []string{"svc-1"},
		"modelId": "gpt-4o", "tokensServed": 10, "tokensRawEquivalent": 40,
		"tokensSaved": 30, "responseSizeBytes": 256,
	})

	after := mustDo(t, "GET", "/api/v1/orgs/"+orgID+"/mcp/savings/summary?period=1y&model_id="+other, adminToken, nil)
	afterCalls := int(after["totalCalls"].(float64))

	if afterCalls != beforeCalls {
		t.Fatalf("want totalCalls unaffected by a different model's event, before=%d after=%d", beforeCalls, afterCalls)
	}
	if after["modelId"] != other {
		t.Fatalf("want modelId %q echoed back, got %v", other, after["modelId"])
	}
}
